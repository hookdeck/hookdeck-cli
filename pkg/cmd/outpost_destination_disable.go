package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type outpostDestinationDisableCmd struct {
	cmd    *cobra.Command
	parent *outpostDestinationCmd

	output string
}

func newOutpostDestinationDisableCmd(parent *outpostDestinationCmd) *outpostDestinationDisableCmd {
	dc := &outpostDestinationDisableCmd{parent: parent}

	dc.cmd = &cobra.Command{
		Use:   "disable <destination-id>",
		Args:  validators.ExactArgs(1),
		Short: ShortDisable(ResourceDestination),
		Long: LongDisableIntro(ResourceDestination) + `

The destination and its configuration are kept, so 'enable' resumes delivery.`,
		PreRunE: dc.validateFlags,
		RunE:    dc.runOutpostDestinationDisableCmd,
		Example: `  # Pause delivery to a destination
  hookdeck outpost destination disable des_abc123 --tenant-id acme`,
		Annotations: map[string]string{
			"cli.arguments": `[
				{"name":"destination-id","type":"string","description":"The ID of the destination to disable.","required":true}
			]`,
		},
	}

	dc.cmd.Flags().StringVar(&dc.output, "output", "", "Output format (json)")

	return dc
}

func (dc *outpostDestinationDisableCmd) validateFlags(cmd *cobra.Command, args []string) error {
	if err := rejectEmptyFlags(cmd); err != nil {
		return err
	}
	return dc.parent.requireTenantID()
}

func (dc *outpostDestinationDisableCmd) runOutpostDestinationDisableCmd(cmd *cobra.Command, args []string) error {
	client := Config.GetOutpostAPIClient()

	destination, err := client.DisableOutpostDestination(context.Background(), dc.parent.tenantID, args[0])
	if err != nil {
		return fmt.Errorf("failed to disable destination: %w", err)
	}

	return printOutpostDestinationStateChange(destination, dc.output, "disabled")
}
