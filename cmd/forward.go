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
	"regexp"
	"strconv"

	"github.com/spf13/cobra"
)

// listenCmd represents the listen command
var forwardCmd = &cobra.Command{
	Use:   "forward",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("Requires a port to foward the webhooks to")
		}
		_, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return errors.New("Argument is not a valid port")
		}

		if len(args) == 3 {
			is_path, err := regexp.MatchString("^/[/.a-zA-Z0-9-]+$", args[2])
			if err != nil || !is_path {
				return errors.New("Path argument is not a valid URL path")
			}
		}

		if len(args) > 3 {
			return errors.New("Invalid extra argument provided")
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		port := args[0]
		var source_alias, destination_path string
		if len(args) > 1 {
			source_alias = args[1]
		}
		if len(args) > 2 {
			destination_path = args[2]
		}

		fmt.Println(port, source_alias, destination_path)
	},
}

func init() {
	rootCmd.AddCommand(forwardCmd)
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// listenCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// listenCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
