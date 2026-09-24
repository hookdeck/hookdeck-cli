package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type outpostDestinationDeleteCmd struct {
	cmd    *cobra.Command
	parent *outpostDestinationCmd

	force bool
}

func newOutpostDestinationDeleteCmd(parent *outpostDestinationCmd) *outpostDestinationDeleteCmd {
	dc := &outpostDestinationDeleteCmd{parent: parent}

	dc.cmd = &cobra.Command{
		Use:   "delete <destination-id>",
		Args:  validators.ExactArgs(1),
		Short: ShortDelete(ResourceDestination),
		Long: LongDeleteIntro(ResourceDestination) + `

Events will stop being delivered to it. To stop delivery temporarily and keep the
destination, use 'disable' instead.`,
		PreRunE: dc.validateFlags,
		RunE:    dc.runOutpostDestinationDeleteCmd,
		Example: `  # Delete a destination, with a confirmation prompt
  hookdeck outpost destination delete des_abc123 --tenant-id acme

  # Skip the prompt (for scripts and CI)
  hookdeck outpost destination delete des_abc123 --tenant-id acme --force`,
		Annotations: map[string]string{
			"cli.arguments": `[
				{"name":"destination-id","type":"string","description":"The ID of the destination to delete.","required":true}
			]`,
		},
	}

	dc.cmd.Flags().BoolVar(&dc.force, "force", false, "Delete without confirmation")

	return dc
}

func (dc *outpostDestinationDeleteCmd) validateFlags(cmd *cobra.Command, args []string) error {
	if err := rejectEmptyFlags(cmd); err != nil {
		return err
	}
	return dc.parent.requireTenantID()
}

func (dc *outpostDestinationDeleteCmd) runOutpostDestinationDeleteCmd(cmd *cobra.Command, args []string) error {
	destinationID := args[0]

	if !dc.force {
		proceed, err := confirmDestructiveAction(
			fmt.Sprintf("\nAre you sure you want to delete destination '%s' for tenant '%s'?", destinationID, dc.parent.tenantID),
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

	client := Config.GetOutpostAPIClient()
	if err := client.DeleteOutpostDestination(context.Background(), dc.parent.tenantID, destinationID); err != nil {
		return fmt.Errorf("failed to delete destination: %w", err)
	}

	fmt.Printf("%s Destination %s deleted\n", SuccessCheck, destinationID)

	return nil
}
