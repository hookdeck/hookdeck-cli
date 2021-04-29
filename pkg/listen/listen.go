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

	box "github.com/Delta456/box-cli-maker/v2"
	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/login"
	"github.com/hookdeck/hookdeck-cli/pkg/proxy"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
	log "github.com/sirupsen/logrus"
)

type Flags struct {
	NoWSS     bool
	WSBaseURL string
}

// listenCmd represents the listen command
func Listen(port string, source_alias string, connection_query string, flags Flags, config *config.Config) error {
	var key string
	var err error
	var guest_url string

	key, err = config.Profile.GetAPIKey()
	if err != nil {
		errString := err.Error()
		if errString == validators.ErrAPIKeyNotConfigured.Error() || errString == validators.ErrDeviceNameNotConfigured.Error() {
			guest_url, _ = login.GuestLogin(config)
			if guest_url == "" {
				return err
			}

			key, err = config.Profile.GetAPIKey()
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	parsedBaseURL, err := url.Parse(config.APIBaseURL)
	if err != nil {
		return err
	}

	client := &hookdeck.Client{
		BaseURL: parsedBaseURL,
		APIKey:  key,
	}

	source, err := getSource(client, source_alias)
	if err != nil {
		return err
	}

	connections, err := getConnections(client, source, connection_query)
	if err != nil {
		return err
	}

	if guest_url != "" {
		// Print guest login URL
		fmt.Println()
		Box := box.New(box.Config{Px: 2, Py: 1, ContentAlign: "Left", Type: "Round", Color: "White", TitlePos: "Top"})
		Box.Print("Guest User Account Created", "👤 Dashboard URL: "+guest_url)
	}

	// Print sources, connections and URLs
	fmt.Println()
	Box := box.New(box.Config{Px: 2, Py: 1, ContentAlign: "Left", Type: "Round", Color: "White", TitlePos: "Top"})
	Box.Print(source.Label, "🔌 Webhook URL: "+source.Url)

	//var connection_ids []string
	for _, connection := range connections {
		fmt.Println(connection.Label + " forwarding to " + connection.Destination.CliPath)
		//connection_ids = append(connection_ids, connection.Id)
	}

	deviceName, err := config.Profile.GetDeviceName()
	if err != nil {
		return err
	}

	fmt.Println("\n👉  Inspect and replay webhooks: https://dashboard.hookdeck.io/events/cli\n")

	p := proxy.New(&proxy.Config{
		DeviceName: deviceName,
		Key:        key,
		APIBaseURL: config.APIBaseURL,
		WSBaseURL:  flags.WSBaseURL,
		NoWSS:      flags.NoWSS,
		Port:       port,
		Log:        log.StandardLogger(),
	}, source, connections)

	err = p.Run(context.Background())
	if err != nil {
		return err
	}

	return nil
}

func isPath(value string) (bool, error) {
	is_path, err := regexp.MatchString("^(/)+([/.a-zA-Z0-9-_]*)$", value)
	return is_path, err
}
