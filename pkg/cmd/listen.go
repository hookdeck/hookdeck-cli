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
	"net/url"
	"strconv"
	"strings"

	"github.com/hookdeck/hookdeck-cli/pkg/listen"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type listenCmd struct {
	cmd            *cobra.Command
	noWSS          bool
	path           string
	maxConnections int
	output         string
}

// Map --cli-path to --path
func normalizeCliPathFlag(f *pflag.FlagSet, name string) pflag.NormalizedName {
	switch name {
	case "cli-path":
		name = "path"
	}
	return pflag.NormalizedName(name)
}

func newListenCmd() *listenCmd {
	lc := &listenCmd{}

	lc.cmd = &cobra.Command{
		Use:   "listen",
		Short: "Forward events for a source to your local server",
		Long: `Forward events for a source to your local server.

This command will create a new Hookdeck Source if it doesn't exist.

By default the Hookdeck Destination will be named "{source}-cli", and the
Destination CLI path will be "/". To set the CLI path, use the "--path" flag.`,
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
	lc.cmd.Flags().BoolVar(&lc.noWSS, "no-wss", false, "Force unencrypted ws:// protocol instead of wss://")
	lc.cmd.Flags().MarkHidden("no-wss")

	lc.cmd.Flags().StringVar(&lc.path, "path", "", "Sets the path to which events are forwarded e.g., /webhooks or /api/stripe")
	lc.cmd.Flags().IntVar(&lc.maxConnections, "max-connections", 50, "Maximum concurrent connections to local endpoint (default: 50, increase for high-volume testing)")

	lc.cmd.Flags().StringVar(&lc.output, "output", "interactive", "Output mode: interactive (full UI), compact (simple logs), quiet (only fatal errors)")

	// --cli-path is an alias for
	lc.cmd.Flags().SetNormalizeFunc(normalizeCliPathFlag)

	usage := lc.cmd.UsageTemplate()

	usage = strings.Replace(
		usage,
		"{{.UseLine}}",
		`hookdeck listen [port or forwarding URL] [source] [connection] [flags]

Arguments:

 - [port or forwarding URL]: Required. The port or forwarding URL to forward the events to e.g., "3000" or "http://localhost:3000"
 - [source]: Required. The name of source to forward the events from e.g., "shopify", "stripe"
 - [connection]: Optional. The name of the connection linking the Source and the Destination
	`, 1)

	usage += fmt.Sprintf(`
	
Examples:

  Forward events from a Hookdeck Source named "shopify" to a local server running on port %[1]d:

    hookdeck listen %[1]d shopify
		
  Forward events to a local server running on "http://myapp.test":

    hookdeck listen %[1]d http://myapp.test
	
  Forward events to the path "/webhooks" on local server running on port %[1]d:

    hookdeck listen %[1]d --path /webhooks
		`, 3000)

	lc.cmd.SetUsageTemplate(usage)

	return lc
}

// listenCmd represents the listen command
func (lc *listenCmd) runListenCmd(cmd *cobra.Command, args []string) error {
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

	return listen.Listen(url, sourceQuery, connectionQuery, listen.Flags{
		NoWSS:          lc.noWSS,
		Path:           lc.path,
		Output:         lc.output,
		MaxConnections: lc.maxConnections,
	}, &Config)
}
