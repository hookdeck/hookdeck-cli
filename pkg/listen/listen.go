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
	"regexp"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/login"
	"github.com/hookdeck/hookdeck-cli/pkg/proxy"
	hookdecksdk "github.com/hookdeck/hookdeck-go-sdk"
	log "github.com/sirupsen/logrus"
)

type Flags struct {
	NoWSS bool
}

// listenCmd represents the listen command
func Listen(URL *url.URL, sourceAliases []string, connectionQuery string, flags Flags, config *config.Config) error {
	var err error
	var guestURL string

	isMultiSource := len(sourceAliases) > 1 || (len(sourceAliases) == 1 && sourceAliases[0] == "*")

	if config.Profile.APIKey == "" {
		guestURL, err = login.GuestLogin(config)
		if guestURL == "" {
			return err
		}
	}

	sdkClient := config.GetClient()

	// Prepare data

	sources, err := getSources(sdkClient, sourceAliases)
	if err != nil {
		return err
	}

	connections, err := getConnections(sdkClient, sources, connectionQuery, isMultiSource)
	if err != nil {
		return err
	}

	if err := validateData(sources, connections); err != nil {
		return err
	}

	// Start proxy

	fmt.Println()
	printDashboardInformation(config, guestURL)
	fmt.Println()
	printSources(config, sources)
	fmt.Println()
	printConnections(config, connections)
	fmt.Println()

	p := proxy.New(&proxy.Config{
		DeviceName:       config.DeviceName,
		Key:              config.Profile.APIKey,
		TeamID:           config.Profile.TeamID,
		TeamMode:         config.Profile.TeamMode,
		APIBaseURL:       config.APIBaseURL,
		DashboardBaseURL: config.DashboardBaseURL,
		ConsoleBaseURL:   config.ConsoleBaseURL,
		WSBaseURL:        config.WSBaseURL,
		NoWSS:            flags.NoWSS,
		URL:              URL,
		Log:              log.StandardLogger(),
		Insecure:         config.Insecure,
	}, connections)

	err = p.Run(context.Background())
	if err != nil {
		return err
	}

	return nil
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
