package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/forms"
	"github.com/hookdeck/hookdeck-cli/pkg/tui"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
	api "github.com/hookdeck/hookdeck-go-sdk"
)

var connectionCreateCmd = &cobra.Command{
	Use:   "create",
	Args:  validators.NoArgs,
	Short: "Create your connection",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := Config.Profile.ValidateAPIKey(); err != nil {
			return err
		}

		client := Config.GetClient()

		// TODO: what about pagination?
		sources, err := client.Source.List(context.Background(), &api.SourceListRequest{})
		if err != nil {
			return err
		}
		destinations, err := client.Destination.List(context.Background(), &api.DestinationListRequest{})
		if err != nil {
			return err
		}

		input, err := forms.Connection.Create(forms.ConnectionCreateFormInput{
			Sources:      sources.Models,
			Destinations: destinations.Models,
		})
		if err != nil {
			return err
		}

		connection, err := client.Connection.Create(context.Background(), input)
		if err != nil {
			return err
		}

		tui.ConnectionFull(&Config, connection)

		return nil
	},
}

func init() {
	connectionCmd.AddCommand(connectionCreateCmd)
}
