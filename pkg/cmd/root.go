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
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"

	"golang.org/x/term"

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

// actionableError carries recovery guidance specific to how a command failed,
// and must be shown instead of Execute's generic recovery text.
//
// Without it, a wrapped API error loses its context: IsUnauthorizedError matches
// through errors.As *and* through a "status code: 401" substring check, so any
// 401 — however specific the cause — is rewritten as the generic "your API key
// is invalid or expired" message. That is exactly wrong for a case like a
// Project API key rejected from HOOKDECK_API_KEY, where the useful part is which
// kind of key belongs in which flag.
type actionableError struct {
	err error
}

func (e *actionableError) Error() string { return e.err.Error() }

// Unwrap keeps the underlying API error inspectable. Execute checks for
// actionableError before its 401 handling, so exposing the cause here does not
// let the generic message win.
func (e *actionableError) Unwrap() error { return e.err }

// newActionableError marks an error as carrying its own recovery guidance.
func newActionableError(err error) error {
	return &actionableError{err: err}
}

// stdinIsTerminal reports whether stdin is attached to a terminal.
// Declared as a variable so tests can substitute it.
var stdinIsTerminal = func() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// authFallback describes what to do when a command fails because no credentials
// are configured.
type authFallback int

const (
	// authFallbackMCP reports the problem on stderr; stdout carries JSON-RPC.
	authFallbackMCP authFallback = iota
	// authFallbackNonInteractive reports how to authenticate without a terminal.
	authFallbackNonInteractive
	// authFallbackRunLogin drops into interactive browser sign-in.
	authFallbackRunLogin
)

// resolveAuthFallback picks the recovery path for a missing-credentials failure.
// Interactive sign-in is only viable with a terminal: it blocks on Enter, opens a
// browser, and polls for ~4 minutes, so choosing it in CI, Docker or an agent
// turns a fast, fixable error into a hang.
func resolveAuthFallback(gatewayMCP, interactiveStdin bool) authFallback {
	switch {
	case gatewayMCP:
		return authFallbackMCP
	case !interactiveStdin:
		return authFallbackNonInteractive
	default:
		return authFallbackRunLogin
	}
}

// nonInteractiveAuthHelp lists the ways to authenticate without a terminal. It is
// shown instead of dropping into browser sign-in, which cannot complete without
// one.
const nonInteractiveAuthHelp = `No terminal is attached, so browser sign-in cannot run.

Authenticate without a terminal using one of:
  hookdeck ci --api-key <project-api-key>   (or set HOOKDECK_API_KEY)
  hookdeck login --cli-key <cli-key>

Or run ` + "`hookdeck login`" + ` in an interactive terminal.`

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

			switch resolveAuthFallback(gatewayMCP, stdinIsTerminal()) {
			case authFallbackMCP:
				// MCP uses JSON-RPC on stdout; do not run interactive login or print recovery text there.
				fmt.Fprintf(os.Stderr, "%s. Use hookdeck_login in the MCP session (or run `hookdeck login` in a terminal).\n", capitalized)
				os.Exit(1)
			case authFallbackNonInteractive:
				fmt.Fprintf(os.Stderr, "%s.\n\n%s\n", capitalized, nonInteractiveAuthHelp)
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

		case errors.As(err, new(*actionableError)):
			// The command already explained what to do; do not replace it with
			// the generic recovery text below.
			if gatewayMCP {
				fmt.Fprintln(os.Stderr, err)
			} else {
				fmt.Println(err)
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
	"profile":          true,
	"p":                true,
	"cli-key":          true,
	"api-key":          true,
	"hookdeck-config":  true,
	"device-name":      true,
	"log-level":        true,
	"color":            true,
	"api-base":         true,
	"outpost-api-base": true,
	"dashboard-base":   true,
	"console-base":     true,
	"ws-base":          true,
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

	rootCmd.PersistentFlags().StringVar(&Config.Profile.APIKey, "cli-key", "", "Hookdeck CLI key (e.g. from dashboard onboarding or hookdeck login)")
	// Hidden for the same reason as --api-key below: authentication is a
	// command-specific flag (`hookdeck login --cli-key`, `hookdeck listen
	// --cli-key`, `hookdeck ci --api-key`), not a global one — see README
	// "CLI authentication keys" and AGENTS.md. The flag keeps working for
	// existing callers; it just stops being advertised as a global option in
	// generated help and REFERENCE.md.
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

	rootCmd.PersistentFlags().StringVar(&Config.OutpostAPIBaseURL, "outpost-api-base", "", fmt.Sprintf("Sets the Outpost API base URL (default \"%s\")", hookdeck.DefaultOutpostAPIBaseURL))
	rootCmd.PersistentFlags().MarkHidden("outpost-api-base")

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
