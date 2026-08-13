package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type destinationDeleteCmd struct {
	cmd   *cobra.Command
	force bool
}

func newDestinationDeleteCmd() *destinationDeleteCmd {
	dc := &destinationDeleteCmd{}

	dc.cmd = &cobra.Command{
		Use:   "delete <destination-id>",
		Args:  validators.ExactArgs(1),
		Short: ShortDelete(ResourceDestination),
		Long: LongDeleteIntro(ResourceDestination) + `

Examples:
  hookdeck gateway destination delete des_abc123
  hookdeck gateway destination delete des_abc123 --force`,
		PreRunE: dc.validateFlags,
		RunE:    dc.runDestinationDeleteCmd,
	}

	dc.cmd.Flags().BoolVar(&dc.force, "force", false, "Force delete without confirmation")

	return dc
}

func (dc *destinationDeleteCmd) validateFlags(cmd *cobra.Command, args []string) error {
	return Config.Profile.ValidateAPIKey()
}

func (dc *destinationDeleteCmd) runDestinationDeleteCmd(cmd *cobra.Command, args []string) error {
	destID := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	dst, err := client.GetDestination(ctx, destID, nil)
	if err != nil {
		return fmt.Errorf("failed to get destination: %w", err)
	}

	if !dc.force {
		proceed, err := confirmDestructiveAction(
			fmt.Sprintf("\nAre you sure you want to delete destination '%s' (%s)?", dst.Name, destID),
			"Deletion cancelled.",
			"force",
		)
		if err != nil {
			return err
		}
		if !proceed {
			return nil
		}
	}

	if err := client.DeleteDestination(ctx, destID); err != nil {
		return fmt.Errorf("failed to delete destination: %w", err)
	}

	fmt.Printf(SuccessCheck+" Destination deleted: %s (%s)\n", dst.Name, destID)
	return nil
}
