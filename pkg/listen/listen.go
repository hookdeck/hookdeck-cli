/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package listen

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/login"
	"github.com/hookdeck/hookdeck-cli/pkg/proxy"
	hookdecksdk "github.com/hookdeck/hookdeck-go-sdk"
	log "github.com/sirupsen/logrus"
)

type Flags struct {
	NoWSS          bool
	Path           string
	MaxConnections int
	Output         string
}

// listenCmd represents the listen command
func Listen(URL *url.URL, sourceQuery string, connectionFilterString string, flags Flags, config *config.Config) error {
	var err error
	var guestURL string

	sourceAliases, err := parseSourceQuery(sourceQuery)
	if err != nil {
		return err
	}

	isMultiSource := len(sourceAliases) > 1 || (len(sourceAliases) == 1 && sourceAliases[0] == "*")

	if flags.Path != "" {
		if isMultiSource {
			return errors.New("Can only set a CLI path when listening to a single source")
		}

		flagIsPath, err := isPath(flags.Path)
		if err != nil {
			return err
		}
		if !flagIsPath {
			return errors.New("The path must be in a valid format")
		}
	}

	if config.Profile.APIKey == "" {
		guestURL, err = login.GuestLogin(config)
		if guestURL == "" {
			return err
		}
	} else if config.Profile.GuestURL != "" && config.Profile.APIKey != "" {
		// User is logged in with a guest account (has both GuestURL and APIKey)
		guestURL = config.Profile.GuestURL
	}
	// If user has permanent account (APIKey but no GuestURL), guestURL remains empty

	sdkClient := config.GetClient()

	// Prepare data

	sources, err := getSources(sdkClient, sourceAliases)
	if err != nil {
		return err
	}

	connections, err := getConnections(sdkClient, sources, connectionFilterString, isMultiSource, flags.Path)
	if err != nil {
		return err
	}

	if len(flags.Path) != 0 && len(connections) > 1 {
		return errors.New(fmt.Errorf(`Multiple CLI destinations found. Cannot set the path on multiple destinations.
Specify a single destination to update the path. For example, pass a connection name:
			
  hookdeck listen %s %s %s --path %s`, URL.String(), sources[0].Name, "<connection>", flags.Path).Error())
	}

	// If the "--path" flag has been passed and the destination has a current cli path value but it's different, update destination path
	if len(flags.Path) != 0 &&
		len(connections) == 1 &&
		*connections[0].Destination.CliPath != "" &&
		*connections[0].Destination.CliPath != flags.Path {

		updateMsg := fmt.Sprintf("Updating destination CLI path from \"%s\" to \"%s\"", *connections[0].Destination.CliPath, flags.Path)
		log.Debug(updateMsg)

		path := flags.Path
		_, err := sdkClient.Destination.Update(context.Background(), connections[0].Destination.Id, &hookdecksdk.DestinationUpdateRequest{
			CliPath: hookdecksdk.Optional(path),
		})

		if err != nil {
			return err
		}

		connections[0].Destination.CliPath = &path
	}

	sources = getRelevantSources(sources, connections)

	if err := validateData(sources, connections); err != nil {
		return err
	}

	// Start proxy
	// For non-interactive modes, print connection info before starting
	if flags.Output == "compact" || flags.Output == "quiet" {
		fmt.Println()
		printSourcesWithConnections(config, sources, connections, URL, guestURL)
		fmt.Println()
	}
	// For interactive mode, connection info will be shown in TUI

	// Create proxy config
	proxyCfg := &proxy.Config{
		DeviceName:       config.DeviceName,
		Key:              config.Profile.APIKey,
		ProjectID:        config.Profile.ProjectId,
		ProjectMode:      config.Profile.ProjectMode,
		APIBaseURL:       config.APIBaseURL,
		DashboardBaseURL: config.DashboardBaseURL,
		ConsoleBaseURL:   config.ConsoleBaseURL,
		WSBaseURL:        config.WSBaseURL,
		NoWSS:            flags.NoWSS,
		URL:              URL,
		Log:              log.StandardLogger(),
		Insecure:         config.Insecure,
		Output:           flags.Output,
		GuestURL:         guestURL,
		MaxConnections:   flags.MaxConnections,
	}

	// Create renderer based on output mode
	rendererCfg := &proxy.RendererConfig{
		DeviceName:       config.DeviceName,
		APIKey:           config.Profile.APIKey,
		APIBaseURL:       config.APIBaseURL,
		DashboardBaseURL: config.DashboardBaseURL,
		ConsoleBaseURL:   config.ConsoleBaseURL,
		ProjectMode:      config.Profile.ProjectMode,
		ProjectID:        config.Profile.ProjectId,
		GuestURL:         guestURL,
		TargetURL:        URL,
		Output:           flags.Output,
		Sources:          sources,
		Connections:      connections,
	}

	renderer := proxy.NewRenderer(rendererCfg)

	// Create and run proxy with renderer
	p := proxy.New(proxyCfg, connections, renderer)

	err = p.Run(context.Background())
	if err != nil {
		// Renderer is already cleaned up, safe to print error
		fmt.Fprintf(os.Stderr, "\n%s\n", err)
		os.Exit(1)
	}

	return nil
}

func parseSourceQuery(sourceQuery string) ([]string, error) {
	var sourceAliases []string
	if sourceQuery == "" {
		sourceAliases = []string{}
	} else if strings.Contains(sourceQuery, ",") {
		sourceAliases = strings.Split(sourceQuery, ",")
	} else if strings.Contains(sourceQuery, " ") {
		sourceAliases = strings.Split(sourceQuery, " ")
	} else {
		sourceAliases = append(sourceAliases, sourceQuery)
	}

	for i := range sourceAliases {
		sourceAliases[i] = strings.TrimSpace(sourceAliases[i])
	}

	// TODO: remove once we can support better limit
	if len(sourceAliases) > 10 {
		return []string{}, errors.New("max 10 sources supported")
	}

	return sourceAliases, nil
}

func isPath(value string) (bool, error) {
	is_path, err := regexp.MatchString(`^(\/)+([/a-zA-Z0-9-_%\.\-\_\~\!\$\&\'\(\)\*\+\,\;\=\:\@]*)$`, value)
	return is_path, err
}

func validateData(sources []*hookdecksdk.Source, connections []*hookdecksdk.Connection) error {
	if len(connections) == 0 {
		return errors.New("no connections provided")
	}

	return nil
}

func getRelevantSources(sources []*hookdecksdk.Source, connections []*hookdecksdk.Connection) []*hookdecksdk.Source {
	relevantSourceId := map[string]bool{}

	for _, connection := range connections {
		relevantSourceId[connection.Source.Id] = true
	}

	relevantSources := []*hookdecksdk.Source{}

	for _, source := range sources {
		if relevantSourceId[source.Id] {
			relevantSources = append(relevantSources, source)
		}
	}

	return relevantSources
}
