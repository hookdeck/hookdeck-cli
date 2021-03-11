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
	ws "github.com/hookdeck/hookdeck-cli/pkg/websocket"

	"github.com/spf13/cobra"
)

// listenCmd represents the listen command
var listenCmd = &cobra.Command{
	Use:   "listen",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		ws.Listen()

		// 	reqBody := ioutil.NopCloser(strings.NewReader(`
		// 	{
		// 		"name":"test",
		// 		"salary":"123",
		// 		"age":"23"
		// 	}
		// `))

		// reqURL, _ := url.Parse("http://localhost:9001")

		// req := &http.Request{
		// 	Method: "POST",
		// 	URL:    reqURL,
		// 	Header: map[string][]string{
		// 		"Content-Type": {"application/json; charset=UTF-8"},
		// 	},
		// 	Body: reqBody,
		// }

		// res, err := http.DefaultClient.Do(req)

		// if err != nil {
		// 	log.Fatal("Error:", err)
		// }

		// data, _ := ioutil.ReadAll(res.Body)

		// res.Body.Close()

		// fmt.Printf("status: %d\n", res.StatusCode)
		// fmt.p("body: %s\n", data)
	},
}

func init() {
	rootCmd.AddCommand(listenCmd)
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// listenCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// listenCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
