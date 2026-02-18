package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type requestCmd struct {
	cmd *cobra.Command
}

func newRequestCmd() *requestCmd {
	rc := &requestCmd{}

	rc.cmd = &cobra.Command{
		Use:     "request",
		Aliases: []string{"requests"},
		Args:    validators.NoArgs,
		Short:   "Inspect and manage requests",
		Long: `List, get, and retry requests (raw inbound webhooks). View events or ignored events for a request.`,
	}

	rc.cmd.AddCommand(newRequestListCmd().cmd)
	rc.cmd.AddCommand(newRequestGetCmd().cmd)
	rc.cmd.AddCommand(newRequestRawBodyCmd().cmd)
	rc.cmd.AddCommand(newRequestRetryCmd().cmd)
	rc.cmd.AddCommand(newRequestEventsCmd().cmd)
	rc.cmd.AddCommand(newRequestIgnoredEventsCmd().cmd)

	return rc
}

func addRequestCmdTo(parent *cobra.Command) {
	parent.AddCommand(newRequestCmd().cmd)
}
