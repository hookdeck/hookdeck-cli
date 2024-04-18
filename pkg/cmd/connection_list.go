package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
	hookdeck "github.com/hookdeck/hookdeck-go-sdk"
)

var connectionListCmd = &cobra.Command{
	Use:   "list",
	Args:  validators.NoArgs,
	Short: "List your connections",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := Config.Profile.ValidateAPIKey(); err != nil {
			return err
		}

		client := Config.GetClient()

		connections, err := client.Connection.List(context.Background(), &hookdeck.ConnectionListRequest{})
		if err != nil {
			return err
		}

		for _, connection := range connections.Models {
			if connection.Name != nil {
				fmt.Printf("%s %s (%s)\n", connection.Id, *connection.FullName, *connection.Name)
			} else {
				fmt.Printf("%s %s\n", connection.Id, *connection.FullName)
			}
		}

		return nil
	},
}

func init() {
	connectionCmd.AddCommand(connectionListCmd)
}
