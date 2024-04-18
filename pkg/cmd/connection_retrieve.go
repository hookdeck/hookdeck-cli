package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

var connectionRetrieveCmd = &cobra.Command{
	Use:   "retrieve",
	Args:  validators.ExactArgs(1),
	Short: "Retrieve your connection",
	RunE: func(cmd *cobra.Command, args []string) error {
		connectionId := args[0]

		if err := Config.Profile.ValidateAPIKey(); err != nil {
			return err
		}

		client := Config.GetClient()

		connection, err := client.Connection.Retrieve(context.Background(), connectionId)
		if err != nil {
			return err
		}

		fmt.Printf("%s %s (%s)\n", connection.Id, *connection.FullName, *connection.Name)

		return nil
	},
}

func init() {
	connectionCmd.AddCommand(connectionRetrieveCmd)
}
