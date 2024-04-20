package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/tui"
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

		tui.ConnectionFull(&Config, connection)

		return nil
	},
}

func init() {
	connectionCmd.AddCommand(connectionRetrieveCmd)
}
