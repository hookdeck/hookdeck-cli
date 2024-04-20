package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/tui"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
	hookdeck "github.com/hookdeck/hookdeck-go-sdk"
)

var sourceCmd = &cobra.Command{
	Use:   "source",
	Args:  validators.NoArgs,
	Short: "Manage your sources",
}

var sourceListCmd = &cobra.Command{
	Use:   "list",
	Args:  validators.NoArgs,
	Short: "List your sources",
	RunE:  listSource,
}

var sourceRetrieveCmd = &cobra.Command{
	Use:   "retrieve",
	Args:  validators.ExactArgs(1),
	Short: "Retrieve your source",
	RunE:  retrieveSource,
}

var sourceDeleteCmd = &cobra.Command{
	Use:   "delete",
	Args:  validators.ExactArgs(1),
	Short: "Delete your source",
	RunE:  deleteSource,
}

func listSource(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	client := Config.GetClient()

	sources, err := client.Source.List(context.Background(), &hookdeck.SourceListRequest{})
	if err != nil {
		return err
	}

	for _, source := range sources.Models {
		fmt.Printf("%s %s\n", source.Id, source.Name)
	}

	return nil
}

func retrieveSource(cmd *cobra.Command, args []string) error {
	sourceId := args[0]

	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	client := Config.GetClient()

	source, err := client.Source.Retrieve(context.Background(), sourceId, &hookdeck.SourceRetrieveRequest{})
	if err != nil {
		return err
	}

	tui.SourceFull(&Config, source)

	return nil
}

func deleteSource(cmd *cobra.Command, args []string) error {
	sourceId := args[0]

	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	client := Config.GetClient()

	source, err := client.Source.Delete(context.Background(), sourceId)
	if err != nil {
		return err
	}

	fmt.Printf("Source %s is deleted\n", source.Id)

	return nil
}

func init() {
	rootCmd.AddCommand(sourceCmd)
	sourceCmd.AddCommand(sourceListCmd)
	sourceCmd.AddCommand(sourceRetrieveCmd)
	sourceCmd.AddCommand(sourceDeleteCmd)
}
