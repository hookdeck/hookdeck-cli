package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

var connectionDeleteCmd = &cobra.Command{
	Use:   "delete",
	Args:  validators.ExactArgs(1),
	Short: "Delete your connection",
	RunE: func(cmd *cobra.Command, args []string) error {
		connectionId := args[0]

		if err := Config.Profile.ValidateAPIKey(); err != nil {
			return err
		}

		client := Config.GetClient()

		connection, err := client.Connection.Delete(context.Background(), connectionId)
		if err != nil {
			return err
		}

		fmt.Printf("Connection %s is deleted\n", connection.Id)

		return nil
	},
}

func init() {
	connectionCmd.AddCommand(connectionDeleteCmd)
}
