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
	"net/url"
	"strconv"
	"strings"

	"github.com/hookdeck/hookdeck-cli/pkg/listen"
	"github.com/spf13/cobra"
)

type listenCmd struct {
	cmd       *cobra.Command
	wsBaseURL string
	noWSS     bool
}

func newListenCmd() *listenCmd {
	lc := &listenCmd{}

	lc.cmd = &cobra.Command{
		Use:   "listen",
		Short: "Forward webhooks for a source to your local server",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("Requires a port or forwarding URL to foward the webhooks to")
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
				return errors.New("Argument is not a valid port or forwading URL")
			}

			if err_port != nil {
				if parsed_url.Host == "" {
					return errors.New("Forwarding URL must contain a host.")
				}

				if parsed_url.RawQuery != "" {
					return errors.New("Forwarding URL cannot contain query params.")
				}
			}

			if len(args) > 3 {
				return errors.New("Invalid extra argument provided")
			}

			return nil
		},
		RunE: lc.runListenCmd,
	}
	lc.cmd.Flags().BoolVar(&lc.noWSS, "no-wss", false, "Force unencrypted ws:// protocol instead of wss://")

	return lc
}

// listenCmd represents the listen command
func (lc *listenCmd) runListenCmd(cmd *cobra.Command, args []string) error {
	var source_alias, connection_query string
	if len(args) > 1 {
		source_alias = args[1]
	}
	if len(args) > 2 {
		connection_query = args[2]
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

	return listen.Listen(url, source_alias, connection_query, listen.Flags{
		NoWSS: lc.noWSS,
	}, &Config)
}
