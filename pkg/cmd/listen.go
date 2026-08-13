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
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/listen"
	"github.com/hookdeck/hookdeck-cli/pkg/login"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type listenCmd struct {
	cmd            *cobra.Command
	noWSS          bool
	noHealthcheck  bool
	path           string
	maxConnections int
	output         string
	filterBody     string
	filterHeaders  string
	filterQuery    string
	filterPath     string
}

// applyCliKey resolves the project context for a --cli-key supplied on the
// command line, and saves the key when this machine has no stored credential.
//
// A key given on the command line determines its own project. Any project read
// from the config file belongs to a different login, and sending it alongside
// this key is what previously produced "your API key is invalid or expired" for
// anyone who already had a profile. Validation is project-agnostic (see
// Client.clientForCLIAuthValidate), so it resolves the project the key really
// belongs to, and that replaces whatever was on disk for this process.
//
// Saving is separate, and only happens when there is no stored credential. That
// covers the Hookdeck Console path, where the Console hands you a
// `listen ... --cli-key <key>` command and the key would otherwise be needed on
// every later run. When a credential already exists it is left alone: someone
// forwarding a Console source for a few minutes should not silently lose the
// login they had.
//
// The key is validated before anything is written, so a typo fails here with a
// clear error rather than being persisted and confusing the next run.
func (lc *listenCmd) applyCliKey(cmd *cobra.Command) error {
	flag := cmd.Flags().Lookup("cli-key")
	if flag == nil || !flag.Changed {
		return nil
	}

	// `--cli-key=` passes the Changed check but leaves nothing to authenticate
	// with. Without this the empty value falls through InitConfig's coalesce
	// and the run fails with "your API key is invalid or expired", which
	// describes neither what happened nor how to fix it. Read the flag rather
	// than Profile.APIKey, which by now may hold a stored key instead.
	if strings.TrimSpace(flag.Value.String()) == "" {
		return errors.New("--cli-key needs a value, e.g. --cli-key <key from the Hookdeck Console>")
	}

	response, err := Config.GetAPIClient().ValidateAPIKey()
	if err != nil {
		return err
	}

	// Adopt the key's own project, discarding any stale project from config.
	Config.Profile.ApplyValidateAPIKeyResponse(response, true)
	Config.RefreshCachedAPIClient()

	if Config.HasStoredAPIKey {
		// Use the key for this run only; the existing login stays on disk.
		return nil
	}

	if err := Config.Profile.SaveProfile(); err != nil {
		return err
	}
	if err := Config.Profile.UseProfile(); err != nil {
		return err
	}
	Config.RefreshCachedAPIClient()

	// Writing credentials is a side effect of a command that otherwise only
	// forwards events, so say so rather than doing it silently.
	fmt.Printf(
		"Saved CLI key for %s. Future runs won't need --cli-key.\n",
		ansi.Bold(response.ProjectName),
	)

	return nil
}

// Map --cli-path to --path
func normalizeCliPathFlag(f *pflag.FlagSet, name string) pflag.NormalizedName {
	switch name {
	case "cli-path":
		name = "path"
	}
	return pflag.NormalizedName(name)
}

// parseFilters builds a SessionFilters object from the filter flag values
func (lc *listenCmd) parseFilters() (*hookdeck.SessionFilters, error) {
	var hasFilters bool
	filters := &hookdeck.SessionFilters{}

	if lc.filterBody != "" {
		hasFilters = true
		var rawMsg json.RawMessage
		if err := json.Unmarshal([]byte(lc.filterBody), &rawMsg); err != nil {
			return nil, fmt.Errorf("invalid JSON in --filter-body: %w", err)
		}
		filters.Body = &rawMsg
	}

	if lc.filterHeaders != "" {
		hasFilters = true
		var rawMsg json.RawMessage
		if err := json.Unmarshal([]byte(lc.filterHeaders), &rawMsg); err != nil {
			return nil, fmt.Errorf("invalid JSON in --filter-headers: %w", err)
		}
		filters.Headers = &rawMsg
	}

	if lc.filterQuery != "" {
		hasFilters = true
		var rawMsg json.RawMessage
		if err := json.Unmarshal([]byte(lc.filterQuery), &rawMsg); err != nil {
			return nil, fmt.Errorf("invalid JSON in --filter-query: %w", err)
		}
		filters.Query = &rawMsg
	}

	if lc.filterPath != "" {
		hasFilters = true
		var rawMsg json.RawMessage
		if err := json.Unmarshal([]byte(lc.filterPath), &rawMsg); err != nil {
			return nil, fmt.Errorf("invalid JSON in --filter-path: %w", err)
		}
		filters.Path = &rawMsg
	}

	if !hasFilters {
		return nil, nil
	}

	return filters, nil
}

func newListenCmd() *listenCmd {
	lc := &listenCmd{}

	lc.cmd = &cobra.Command{
		Use:   "listen [port or forwarding URL] [source(s)] [connection]",
		Short: "Forward events for one or more sources to your local server",
		Long: `Forward events for one or more sources to your local server.

You can listen to a single source, a comma-separated list of sources, or
"*" to listen to all of your sources at once.

This command will create a new Hookdeck Source if it doesn't exist (single
source only).

By default the Hookdeck Destination will be named "{source}-cli", and the
Destination CLI path will be "/". To set the CLI path, use the "--path" flag.

Authentication order: "--cli-key", then stored credentials from "hookdeck login"
or "hookdeck ci", then HOOKDECK_API_KEY. Setting HOOKDECK_API_KEY to a Project
API key is enough to run in CI — the CLI exchanges it for CLI credentials and
saves them. With none of these, a temporary guest account is created, which has
no delivery history, retries, or issue triggers.

Without a terminal (CI, Docker, nohup, an AI agent) the interactive UI cannot
run, so "--output" falls back to "compact" automatically.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("requires a port or forwarding URL to forward the events to")
			}

			_, err_port := strconv.ParseInt(args[0], 10, 64)

			var parsed_url *url.URL
			var err_url error
			if strings.HasPrefix(args[0], "http") {
				parsed_url, err_url = url.Parse(args[0])
			} else {
				parsed_url, err_url = url.Parse("http://" + args[0])
			}

			if err_port != nil && err_url != nil {
				return errors.New("argument is not a valid port or forwading URL")
			}

			if err_port != nil {
				if parsed_url.Host == "" {
					return errors.New("forwarding URL must contain a host")
				}

				if parsed_url.RawQuery != "" {
					return errors.New("forwarding URL cannot contain query params")
				}
			}

			if len(args) > 3 {
				return errors.New("invalid extra argument provided")
			}

			return nil
		},
		RunE: lc.runListenCmd,
	}
	lc.cmd.Annotations = map[string]string{
		"cli.arguments": `[
			{"name":"port or forwarding URL","type":"string","description":"Port (e.g. 3000) or full URL (e.g. http://localhost:3000) to forward events to. The forward URL will be http://localhost:$PORT/$DESTINATION_PATH or http://domain/$DESTINATION_PATH. Only one of port or domain is required.","required":true},
			{"name":"source","type":"string","description":"The name of a source to listen to, a comma-separated list of source names, or '*' (with quotes) to listen to all. If empty, the CLI prompts you to choose.","required":false},
			{"name":"connection","type":"string","description":"Filter connections by connection name or path.","required":false}
		]`,
	}
	lc.cmd.Flags().BoolVar(&lc.noWSS, "no-wss", false, "Force unencrypted ws:// protocol instead of wss://")
	lc.cmd.Flags().MarkHidden("no-wss")

	// Declared locally as well as on the root command. The root flag is hidden
	// and deprecated, but `listen --cli-key` is a documented, supported way to
	// authenticate a single run (from the Hookdeck Console, or in CI), and
	// listen's own help promotes it. Binding to the same Config field keeps the
	// behaviour identical; this only makes the flag discoverable in
	// `hookdeck listen --help`.
	lc.cmd.Flags().StringVar(&Config.Profile.APIKey, "cli-key", "", "Hookdeck CLI key used to authenticate this command, e.g. the key shown in the Hookdeck Console")

	lc.cmd.Flags().StringVar(&lc.path, "path", "", "Sets the path to which events are forwarded e.g., /webhooks or /api/stripe")
	lc.cmd.Flags().IntVar(&lc.maxConnections, "max-connections", 50, "Maximum concurrent connections to local endpoint (default: 50, increase for high-volume testing)")

	lc.cmd.Flags().StringVar(&lc.output, "output", "interactive", "Output mode: interactive (full UI), compact (simple logs), quiet (errors and warnings only). Falls back to compact automatically when there is no terminal.")

	lc.cmd.Flags().BoolVar(&lc.noHealthcheck, "no-healthcheck", false, "Disable periodic health checks of the local server")

	lc.cmd.Flags().StringVar(&lc.filterBody, "filter-body", "", "Filter events by request body using Hookdeck filter syntax (JSON)")
	lc.cmd.Flags().StringVar(&lc.filterHeaders, "filter-headers", "", "Filter events by request headers using Hookdeck filter syntax (JSON)")
	lc.cmd.Flags().StringVar(&lc.filterQuery, "filter-query", "", "Filter events by query parameters using Hookdeck filter syntax (JSON)")
	lc.cmd.Flags().StringVar(&lc.filterPath, "filter-path", "", "Filter events by request path using Hookdeck filter syntax (JSON)")

	// --cli-path is an alias for
	lc.cmd.Flags().SetNormalizeFunc(normalizeCliPathFlag)

	usage := lc.cmd.UsageTemplate()

	usage = strings.Replace(
		usage,
		"{{.UseLine}}",
		`hookdeck listen [port or forwarding URL] [source(s)] [connection] [flags]

Arguments:

 - [port or forwarding URL]: Required. The port or forwarding URL to forward the events to e.g., "3000" or "http://localhost:3000"
 - [source(s)]: Optional. One source name, a comma-separated list of source names (e.g. "shopify,stripe"), or "*" to listen to all sources. If omitted, the CLI prompts you to choose.
 - [connection]: Optional. The name of the connection linking the Source and the Destination
	`, 1)

	usage += fmt.Sprintf(`

Examples:

  Forward events from a Hookdeck Source named "shopify" to a local server running on port %[1]d:

    hookdeck listen %[1]d shopify

  Forward events from multiple sources to a local server running on port %[1]d:

    hookdeck listen %[1]d shopify,stripe

  Forward events from all of your sources:

    hookdeck listen %[1]d '*'

  Forward events to a local server running on "http://myapp.test:%[1]d":

    hookdeck listen http://myapp.test:%[1]d

  Forward events to the path "/webhooks" on local server running on port %[1]d:

    hookdeck listen %[1]d --path /webhooks

  Filter events by body content (only events with matching data):

    hookdeck listen %[1]d github --filter-body '{"action": "opened"}'

  Filter events with multiple conditions:

    hookdeck listen %[1]d stripe --filter-body '{"type": "charge.succeeded"}' --filter-headers '{"x-stripe-signature": {"$exist": true}}'

  Filter using operators (see https://hookdeck.com/docs/filters for syntax):

    hookdeck listen %[1]d api --filter-body '{"amount": {"$gte": 100}}'

  Authenticate with a specific key instead of the stored login, e.g. in CI or
  when switching accounts (--cli-key takes a user-scoped CLI key; --api-key a
  project-scoped key):

    hookdeck listen %[1]d stripe --cli-key <your-cli-key>
		`, 3000)

	lc.cmd.SetUsageTemplate(usage)

	return lc
}

// envAPIKey reads the Project API key from the environment.
// Declared as a variable so tests can substitute it.
var envAPIKey = func() string {
	return strings.TrimSpace(os.Getenv("HOOKDECK_API_KEY"))
}

// shouldExchangeEnvAPIKey reports whether listen should authenticate from
// HOOKDECK_API_KEY.
//
// Precedence is --cli-key > stored login > HOOKDECK_API_KEY > guest. The env var
// deliberately loses to a stored login: someone already signed in must not have
// their session repointed by a variable left in their shell.
func shouldExchangeEnvAPIKey(currentAPIKey, envKey string) bool {
	return currentAPIKey == "" && envKey != ""
}

// applyEnvAPIKey authenticates from HOOKDECK_API_KEY when nothing else has.
//
// HOOKDECK_API_KEY holds a *Project* API key, which the CLI-auth endpoints
// reject — GET /cli-auth/validate answers 401 for it. `hookdeck ci` works by
// exchanging it for a CLI client key via POST /cli-auth/ci, so that is what
// happens here. Assigning the env var straight to Profile.APIKey, as the
// original report suggested, would only swap one silent failure for another.
//
// Without this, listen fell through to GuestLogin and created a throwaway
// account. Traffic still arrived locally, so it looked like it worked, but none
// of it was in the caller's project: no connection, no delivery history, no
// retries (#334).
func (lc *listenCmd) applyEnvAPIKey() error {
	envKey := envAPIKey()
	if !shouldExchangeEnvAPIKey(Config.Profile.APIKey, envKey) {
		return nil
	}

	// Exchange and persist, exactly as `hookdeck ci` does, so this costs one
	// round trip per machine rather than one per listen invocation. CILogin
	// reports the project it configured.
	if err := login.CILogin(&Config, envKey, Config.DeviceName); err != nil {
		return fmt.Errorf(
			"could not authenticate with HOOKDECK_API_KEY: %w\n\n"+
				"HOOKDECK_API_KEY must be a Project API key from the Hookdeck dashboard "+
				"(Project Settings > API Keys). For a CLI key, use --cli-key instead.",
			err,
		)
	}

	Config.RefreshCachedAPIClient()

	return nil
}

// stdoutIsTerminal reports whether stdout is attached to a terminal.
// Declared as a variable so tests can substitute it.
var stdoutIsTerminal = func() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// resolveOutputMode downgrades the interactive renderer to compact when stdout is
// not a terminal, and reports a warning to show when it does.
//
// explicit reports whether the user passed --output themselves. We only warn in
// that case: someone who asked for interactive output deserves to know they did
// not get it, while the defaulted path should stay quiet so ordinary CI logs are
// not littered with a notice about a flag the caller never set.
func resolveOutputMode(requested string, explicit, isTerminal bool) (mode string, warning string) {
	if requested != "interactive" || isTerminal {
		return requested, ""
	}

	if explicit {
		return "compact", "Warning: --output interactive needs a terminal; falling back to --output compact."
	}

	return "compact", ""
}

// listenCmd represents the listen command
func (lc *listenCmd) runListenCmd(cmd *cobra.Command, args []string) error {
	if err := lc.applyCliKey(cmd); err != nil {
		return err
	}

	// Must run before listen.Listen, which falls back to a guest account when no
	// credential is present (#334).
	if err := lc.applyEnvAPIKey(); err != nil {
		return err
	}

	var sourceQuery, connectionQuery string
	if len(args) > 1 {
		sourceQuery = args[1]
	}
	if len(args) > 2 {
		connectionQuery = args[2]
	}

	// Validate output flag
	validOutputModes := map[string]bool{
		"interactive": true,
		"compact":     true,
		"quiet":       true,
	}
	if !validOutputModes[lc.output] {
		return errors.New("invalid --output mode. Must be: interactive, compact, or quiet")
	}

	// The interactive renderer opens /dev/tty itself, so without a terminal it
	// dies at startup and no events are forwarded (#333). Fall back rather than
	// fail: forwarding is the point of the command, and the caller in CI, Docker
	// or an agent has no way to know which flag to reach for.
	mode, warning := resolveOutputMode(lc.output, cmd.Flags().Changed("output"), stdoutIsTerminal())
	if warning != "" {
		fmt.Fprintln(os.Stderr, warning)
	}
	lc.output = mode

	_, err_port := strconv.ParseInt(args[0], 10, 64)
	var url *url.URL
	if err_port != nil {
		if strings.HasPrefix(args[0], "http") {
			url, _ = url.Parse(args[0])
		} else {
			url, _ = url.Parse("http://" + args[0])
		}
	} else {
		url, _ = url.Parse("http://localhost:" + args[0])
	}

	if url.Scheme == "" {
		url.Scheme = "http"
	}

	// Parse and validate filters
	filters, err := lc.parseFilters()
	if err != nil {
		return err
	}

	return listen.Listen(url, sourceQuery, connectionQuery, listen.Flags{
		NoWSS:          lc.noWSS,
		NoHealthcheck:  lc.noHealthcheck,
		Path:           lc.path,
		Output:         lc.output,
		MaxConnections: lc.maxConnections,
		Filters:        filters,
	}, &Config)
}
