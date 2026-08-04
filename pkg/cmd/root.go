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
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		initTelemetry(cmd)
	},
}

// initTelemetry populates the process-wide telemetry singleton before any
// command runs. Commands that override PersistentPreRun (e.g. connection)
// must call this explicitly — Cobra does not chain PersistentPreRun.
func initTelemetry(cmd *cobra.Command) {
	tel := hookdeck.GetTelemetryInstance()
	tel.SetDisabled(Config.TelemetryDisabled)
	tel.SetSource("cli")
	tel.SetEnvironment(hookdeck.DetectEnvironment())
	tel.SetCommandContext(cmd)
	tel.SetCommandFlagsFromCobra(cmd)
	tel.SetDeviceName(Config.DeviceName)
	if tel.InvocationID == "" {
		tel.SetInvocationID(hookdeck.NewInvocationID())
	}
}

// RootCmd returns the root command for use by tools (e.g. generate-reference).
func RootCmd() *cobra.Command {
	return rootCmd
}

// addConnectionCmdTo registers the connection command tree on a parent so that
// "connection" (and alias "connections") is available there. Call twice to expose
// the same subcommands under both gateway and root (backward compat).
// Command definitions live only in newConnectionCmd(); this just registers the result.
func addConnectionCmdTo(parent *cobra.Command) {
	parent.AddCommand(newConnectionCmd().cmd)
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	gatewayMCP := argvContainsGatewayMCP(os.Args)
	if err := rootCmd.Execute(); err != nil {
		errString := err.Error()
		isLoginRequiredError := errString == validators.ErrAPIKeyNotConfigured.Error() || errString == validators.ErrDeviceNameNotConfigured.Error()

		switch {
		case isLoginRequiredError:
			errRunes := []rune(errString)
			errRunes[0] = unicode.ToUpper(errRunes[0])
			capitalized := string(errRunes)

			if gatewayMCP {
				// MCP uses JSON-RPC on stdout; do not run interactive login or print recovery text there.
				fmt.Fprintf(os.Stderr, "%s. Use hookdeck_login in the MCP session (or run `hookdeck login` in a terminal).\n", capitalized)
				os.Exit(1)
			}

			fmt.Printf("%s. Running `hookdeck login`...\n", capitalized)
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

			msg := fmt.Sprintf("Unknown command \"%s\" for \"%s\".%s"+
				"ee \"hookdeck --help\" for a list of available commands.",
				os.Args[1], rootCmd.CommandPath(), suggStr)
			if gatewayMCP {
				fmt.Fprintln(os.Stderr, msg)
			} else {
				fmt.Println(msg)
			}

		default:
			if hookdeck.IsUnauthorizedError(err) {
				msg := "Authentication failed: your API key is invalid or expired.\n\n" +
					"Sign in again: run `hookdeck login` (browser sign-in), or `hookdeck login -i` / `hookdeck --api-key <key> login`.\n\n" +
					"MCP: use hookdeck_login with reauth: true."
				if gatewayMCP {
					fmt.Fprintln(os.Stderr, msg)
				} else {
					fmt.Println(msg)
				}
			} else if gatewayMCP {
				fmt.Fprintln(os.Stderr, err)
			} else {
				fmt.Println(err)
			}
		}

		os.Exit(1)
	}
}

// argvContainsGatewayMCP reports whether argv invokes `hookdeck gateway mcp`, ignoring
// global flags and flag values (e.g. --profile name, -p name) so detection stays accurate.
func argvContainsGatewayMCP(argv []string) bool {
	if len(argv) < 3 {
		return false
	}
	pos := globalPositionalArgs(argv[1:])
	for i := 0; i < len(pos)-1; i++ {
		if pos[i] == "gateway" && pos[i+1] == "mcp" {
			return true
		}
	}
	return false
}

// flagNeedsNextArg lists global flags that consume the next argv token as their value.
// Keep in sync with the PersistentFlags registered in init() below.
var flagNeedsNextArg = map[string]bool{
	"profile":         true,
	"p":               true,
	"cli-key":         true,
	"api-key":         true,
	"hookdeck-config": true,
	"device-name":     true,
	"log-level":       true,
	"color":           true,
	"api-base":        true,
	"dashboard-base":  true,
	"console-base":    true,
	"ws-base":         true,
}

// globalPositionalArgs returns argv arguments that are not global flags or flag values,
// stopping at `--` (which ends flag parsing; remaining tokens are positional).
func globalPositionalArgs(args []string) []string {
	var out []string
	i := 0
	for i < len(args) {
		a := args[i]
		if a == "--" {
			return append(out, args[i+1:]...)
		}
		if !strings.HasPrefix(a, "-") {
			return append(out, args[i:]...)
		}
		if strings.HasPrefix(a, "--") {
			body := strings.TrimPrefix(a, "--")
			name := body
			hasEq := false
			if j := strings.IndexByte(body, '='); j >= 0 {
				name = body[:j]
				hasEq = true
			}
			i++
			if flagNeedsNextArg[name] && !hasEq {
				if i < len(args) && !strings.HasPrefix(args[i], "-") {
					i++
				}
			}
			continue
		}
		// Short flags: support -p <profile> only; other shorts consume one token.
		if a == "-p" {
			i++
			if i < len(args) && !strings.HasPrefix(args[i], "-") {
				i++
			}
			continue
		}
		i++
	}
	return out
}

func init() {
	cobra.OnInitialize(Config.InitConfig)

	rootCmd.PersistentFlags().StringVarP(&Config.Profile.Name, "profile", "p", "", fmt.Sprintf("profile name (default \"%s\")", hookdeck.DefaultProfileName))

	rootCmd.PersistentFlags().StringVar(&Config.Profile.APIKey, "cli-key", "", "(deprecated) Your API key to use for the command")
	rootCmd.PersistentFlags().MarkHidden("cli-key")

	rootCmd.PersistentFlags().StringVar(&Config.Profile.APIKey, "api-key", "", "Your API key to use for the command")
	rootCmd.PersistentFlags().MarkHidden("api-key")

	rootCmd.PersistentFlags().StringVar(&Config.Color, "color", "", "turn on/off color output (on, off, auto)")

	rootCmd.PersistentFlags().StringVar(&Config.ConfigFileFlag, "hookdeck-config", "", "path to CLI config file (default is $HOME/.config/hookdeck/config.toml)")

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
	rootCmd.AddCommand(newGatewayCmd().cmd)
	rootCmd.AddCommand(newTelemetryCmd().cmd)
	// Backward compat: same connection command tree also at root (single definition in newConnectionCmd)
	addConnectionCmdTo(rootCmd)
}
