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
func Listen(URL *url.URL, source_alias string, connection_query string, flags Flags, config *config.Config) error {
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

	fmt.Println()
	fmt.Println(ansi.Bold("Dashboard"))
	if guest_url != "" {
		fmt.Println("👤 Login URL: " + guest_url)
		fmt.Println("Sign up in the dashboard to make your webhook URL permanent.")
		fmt.Println()
	}

	fmt.Printf("👉 Inspect and replay webhooks: %s/cli/events\n", config.DashboardBaseURL)
	fmt.Println()

	fmt.Println(ansi.Bold(source.Label + " Source"))
	fmt.Println("🔌 Webhook URL: " + source.Url)
	fmt.Println()

	fmt.Println(ansi.Bold("Connections"))
	for _, connection := range connections {
		fmt.Println(connection.Label + " forwarding to " + connection.Destination.CliPath)
	}
	fmt.Println()

	deviceName, err := config.Profile.GetDeviceName()
	if err != nil {
		return err
	}

	p := proxy.New(&proxy.Config{
		DeviceName:       deviceName,
		Key:              key,
		APIBaseURL:       config.APIBaseURL,
		DashboardBaseURL: config.DashboardBaseURL,
		WSBaseURL:        flags.WSBaseURL,
		NoWSS:            flags.NoWSS,
		URL:              URL,
		Log:              log.StandardLogger(),
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
