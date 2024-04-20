package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/tui"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
	hookdeck "github.com/hookdeck/hookdeck-go-sdk"
)

var destinationCmd = &cobra.Command{
	Use:   "destination",
	Args:  validators.NoArgs,
	Short: "Manage your destinations",
}

var destinationListCmd = &cobra.Command{
	Use:   "list",
	Args:  validators.NoArgs,
	Short: "List your destinations",
	RunE:  listDestination,
}

var destinationRetrieveCmd = &cobra.Command{
	Use:   "retrieve",
	Args:  validators.ExactArgs(1),
	Short: "Retrieve your destination",
	RunE:  retrieveDestination,
}

var destinationDeleteCmd = &cobra.Command{
	Use:   "delete",
	Args:  validators.ExactArgs(1),
	Short: "Delete your destination",
	RunE:  deleteDestination,
}

func listDestination(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	client := Config.GetClient()

	destinations, err := client.Destination.List(context.Background(), &hookdeck.DestinationListRequest{})
	if err != nil {
		return err
	}

	for _, destination := range destinations.Models {
		fmt.Printf("%s %s\n", destination.Id, destination.Name)
	}

	return nil
}

func retrieveDestination(cmd *cobra.Command, args []string) error {
	destinationId := args[0]

	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	client := Config.GetClient()

	destination, err := client.Destination.Retrieve(context.Background(), destinationId)
	if err != nil {
		return err
	}

	tui.DestinationFull(&Config, destination)

	return nil
}

func deleteDestination(cmd *cobra.Command, args []string) error {
	destinationId := args[0]

	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	client := Config.GetClient()

	destination, err := client.Destination.Delete(context.Background(), destinationId)
	if err != nil {
		return err
	}

	fmt.Printf("Destination %s is deleted\n", destination.Id)

	return nil
}

func init() {
	rootCmd.AddCommand(destinationCmd)
	destinationCmd.AddCommand(destinationListCmd)
	destinationCmd.AddCommand(destinationRetrieveCmd)
	destinationCmd.AddCommand(destinationDeleteCmd)
}
