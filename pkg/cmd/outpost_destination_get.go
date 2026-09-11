package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type outpostDestinationGetCmd struct {
	cmd    *cobra.Command
	parent *outpostDestinationCmd

	output string
}

func newOutpostDestinationGetCmd(parent *outpostDestinationCmd) *outpostDestinationGetCmd {
	dc := &outpostDestinationGetCmd{parent: parent}

	dc.cmd = &cobra.Command{
		Use:     "get <destination-id>",
		Args:    validators.ExactArgs(1),
		Short:   ShortGet(ResourceDestination),
		Long:    `Get details for a destination, including its config and topics.`,
		PreRunE: dc.validateFlags,
		RunE:    dc.runOutpostDestinationGetCmd,
		Example: `  # Get a destination
  hookdeck outpost destination get des_abc123 --tenant-id acme`,
		Annotations: map[string]string{
			"cli.arguments": `[
				{"name":"destination-id","type":"string","description":"The ID of the destination.","required":true}
			]`,
		},
	}

	dc.cmd.Flags().StringVar(&dc.output, "output", "", "Output format (json)")

	return dc
}

func (dc *outpostDestinationGetCmd) validateFlags(cmd *cobra.Command, args []string) error {
	if err := rejectEmptyFlags(cmd); err != nil {
		return err
	}
	return dc.parent.requireTenantID()
}

func (dc *outpostDestinationGetCmd) runOutpostDestinationGetCmd(cmd *cobra.Command, args []string) error {
	client := Config.GetOutpostAPIClient()

	destination, err := client.GetOutpostDestination(context.Background(), dc.parent.tenantID, args[0])
	if err != nil {
		return fmt.Errorf("failed to get destination: %w", err)
	}

	if dc.output == "json" {
		return printJSONIndented(destination)
	}

	fmt.Println()
	printOutpostDestination(destination, "")
	fmt.Println()

	return nil
}
