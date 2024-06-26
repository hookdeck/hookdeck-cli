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
	"fmt"
	"net/url"
	"regexp"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/login"
	"github.com/hookdeck/hookdeck-cli/pkg/proxy"
	log "github.com/sirupsen/logrus"
)

type Flags struct {
	NoWSS bool
}

// listenCmd represents the listen command
func Listen(URL *url.URL, source_alias string, connectionQuery string, flags Flags, config *config.Config) error {
	var err error
	var guest_url string

	if config.Profile.APIKey == "" {
		guest_url, err = login.GuestLogin(config)
		if guest_url == "" {
			return err
		}
	}

	sdkClient := config.GetClient()

	source, err := getSource(sdkClient, source_alias)
	if err != nil {
		return err
	}

	connections, err := getConnections(sdkClient, source, connectionQuery)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println(ansi.Bold("Dashboard"))
	if guest_url != "" {
		fmt.Println("👤 Console URL: " + guest_url)
		fmt.Println("Sign up in the Console to make your webhook URL permanent.")
		fmt.Println()
	} else {
		var url = config.DashboardBaseURL
		if config.Profile.TeamID != "" {
			url += "?team_id=" + config.Profile.TeamID
		}
		if config.Profile.TeamMode == "console" {
			url = config.ConsoleBaseURL + "?source_id=" + source.Id
		}
		fmt.Println("👉 Inspect and replay events: " + url)
		fmt.Println()
	}

	fmt.Println(ansi.Bold(source.Name + " Source"))
	fmt.Println("🔌 Event URL: " + source.Url)
	fmt.Println()

	fmt.Println(ansi.Bold("Connections"))
	for _, connection := range connections {
		var connectionName string
		if connection.Name != nil {
			connectionName = *connection.Name
		} else {
			connectionName = connection.Destination.Name
		}
		fmt.Println(connectionName + " forwarding to " + *connection.Destination.CliPath)
	}
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
	}, source, connections)

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
