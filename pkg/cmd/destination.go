package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type destinationCmd struct {
	cmd *cobra.Command
}

func newDestinationCmd() *destinationCmd {
	dc := &destinationCmd{}

	dc.cmd = &cobra.Command{
		Use:     "destination",
		Aliases: []string{"destinations"},
		Args:    validators.NoArgs,
		Short:   ShortBeta("Manage your destinations"),
		Long: LongBeta(`Manage webhook and event destinations.

Destinations define where Hookdeck forwards events. Create destinations with a type (HTTP, CLI, MOCK_API),
optional URL and authentication, then connect them to sources via connections.`),
	}

	dc.cmd.AddCommand(newDestinationListCmd().cmd)
	dc.cmd.AddCommand(newDestinationGetCmd().cmd)
	dc.cmd.AddCommand(newDestinationCreateCmd().cmd)
	dc.cmd.AddCommand(newDestinationUpsertCmd().cmd)
	dc.cmd.AddCommand(newDestinationUpdateCmd().cmd)
	dc.cmd.AddCommand(newDestinationDeleteCmd().cmd)
	dc.cmd.AddCommand(newDestinationEnableCmd().cmd)
	dc.cmd.AddCommand(newDestinationDisableCmd().cmd)
	dc.cmd.AddCommand(newDestinationCountCmd().cmd)

	return dc
}

// addDestinationCmdTo registers the destination command tree on the given parent (e.g. gateway).
func addDestinationCmdTo(parent *cobra.Command) {
	parent.AddCommand(newDestinationCmd().cmd)
}
