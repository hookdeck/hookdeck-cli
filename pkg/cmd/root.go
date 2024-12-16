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
package cmd

import (
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
	"github.com/hookdeck/hookdeck-cli/pkg/version"
	"github.com/spf13/cobra"
)

var Config config.Config

var rootCmd = &cobra.Command{
	Use:           "hookdeck",
	SilenceUsage:  true,
	SilenceErrors: true,
	Version:       version.Version,
	Short:         "A CLI to forward events received on Hookdeck to your local server.",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		errString := err.Error()
		isLoginRequiredError := errString == validators.ErrAPIKeyNotConfigured.Error() || errString == validators.ErrDeviceNameNotConfigured.Error()

		switch {
		case isLoginRequiredError:
			// capitalize first letter of error because linter
			errRunes := []rune(errString)
			errRunes[0] = unicode.ToUpper(errRunes[0])

			fmt.Printf("%s. Running `hookdeck login`...\n", string(errRunes))
			loginCommand, _, err := rootCmd.Find([]string{"login"})

			if err != nil {
				fmt.Println(err)
			}

			err = loginCommand.RunE(&cobra.Command{}, []string{})

			if err != nil {
				fmt.Println(err)
			}

		case strings.Contains(errString, "unknown command"):
			suggStr := "\nS"

			suggestions := rootCmd.SuggestionsFor(os.Args[1])
			if len(suggestions) > 0 {
				suggStr = fmt.Sprintf(" Did you mean \"%s\"?\nIf not, s", suggestions[0])
			}

			fmt.Println(fmt.Sprintf("Unknown command \"%s\" for \"%s\".%s"+
				"ee \"hookdeck --help\" for a list of available commands.",
				os.Args[1], rootCmd.CommandPath(), suggStr))

		default:
			fmt.Println(err)
		}

		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(Config.InitConfig)

	rootCmd.PersistentFlags().StringVarP(&Config.Profile.Name, "profile", "p", "", fmt.Sprintf("profile name (default \"%s\")", hookdeck.DefaultProfileName))

	rootCmd.PersistentFlags().StringVar(&Config.Profile.APIKey, "cli-key", "", "(deprecated) Your API key to use for the command")
	rootCmd.PersistentFlags().MarkHidden("cli-key")

	rootCmd.PersistentFlags().StringVar(&Config.Profile.APIKey, "api-key", "", "Your API key to use for the command")
	rootCmd.PersistentFlags().MarkHidden("api-key")

	rootCmd.PersistentFlags().StringVar(&Config.Color, "color", "", "turn on/off color output (on, off, auto)")

	rootCmd.PersistentFlags().StringVar(&Config.LocalConfigFile, "config", "", "config file (default is $HOME/.config/hookdeck/config.toml)")

	rootCmd.PersistentFlags().StringVar(&Config.DeviceName, "device-name", "", "device name")

	rootCmd.PersistentFlags().StringVar(&Config.LogLevel, "log-level", "info", "log level (debug, info, warn, error)")

	rootCmd.PersistentFlags().BoolVar(&Config.Insecure, "insecure", false, "Allow invalid TLS certificates")

	// Hidden configuration flags, useful for dev/debugging
	rootCmd.PersistentFlags().StringVar(&Config.APIBaseURL, "api-base", "", fmt.Sprintf("Sets the API base URL (default \"%s\")", hookdeck.DefaultAPIBaseURL))
	rootCmd.PersistentFlags().MarkHidden("api-base")

	rootCmd.PersistentFlags().StringVar(&Config.DashboardBaseURL, "dashboard-base", "", fmt.Sprintf("Sets the web dashboard base URL (default \"%s\")", hookdeck.DefaultDashboardBaseURL))
	rootCmd.PersistentFlags().MarkHidden("dashboard-base")

	rootCmd.PersistentFlags().StringVar(&Config.ConsoleBaseURL, "console-base", "", fmt.Sprintf("Sets the web console base URL (default \"%s\")", hookdeck.DefaultConsoleBaseURL))
	rootCmd.PersistentFlags().MarkHidden("console-base")

	rootCmd.PersistentFlags().StringVar(&Config.WSBaseURL, "ws-base", "", fmt.Sprintf("Sets the Websocket base URL (default \"%s\")", hookdeck.DefaultWebsocektURL))
	rootCmd.PersistentFlags().MarkHidden("ws-base")

	rootCmd.Flags().BoolP("version", "v", false, "Get the version of the Hookdeck CLI")

	rootCmd.AddCommand(newCICmd().cmd)
	rootCmd.AddCommand(newLoginCmd().cmd)
	rootCmd.AddCommand(newLogoutCmd().cmd)
	rootCmd.AddCommand(newListenCmd().cmd)
	rootCmd.AddCommand(newCompletionCmd().cmd)
	rootCmd.AddCommand(newWhoamiCmd().cmd)
	rootCmd.AddCommand(newProjectCmd().cmd)
}
