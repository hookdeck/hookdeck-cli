package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type outpostEventCmd struct {
	cmd *cobra.Command
}

func newOutpostEventCmd() *outpostEventCmd {
	ec := &outpostEventCmd{}

	ec.cmd = &cobra.Command{
		Use:     "event",
		Aliases: []string{"events"},
		Args:    validators.NoArgs,
		Short:   ShortBeta("Inspect published events"),
		Long: LongBeta(`Inspect events published to your tenants' destinations.

Events are created by publishing, so there is no create command here. Publishing
is asynchronous, so a freshly published event can take a moment to appear.`),
	}

	ec.cmd.AddCommand(newOutpostEventListCmd().cmd)
	ec.cmd.AddCommand(newOutpostEventGetCmd().cmd)
	ec.cmd.AddCommand(newOutpostEventRetryCmd().cmd)

	return ec
}
