package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type eventCmd struct {
	cmd *cobra.Command
}

func newEventCmd() *eventCmd {
	ec := &eventCmd{}

	ec.cmd = &cobra.Command{
		Use:   "event",
		Aliases: []string{"events"},
		Args:  validators.NoArgs,
		Short: "Inspect and manage events",
		Long: `List, get, retry, cancel, or mute events (processed webhook deliveries).
Filter by connection ID (--connection-id), status, source, or destination.`,
	}

	ec.cmd.AddCommand(newEventListCmd().cmd)
	ec.cmd.AddCommand(newEventGetCmd().cmd)
	ec.cmd.AddCommand(newEventRawBodyCmd().cmd)
	ec.cmd.AddCommand(newEventRetryCmd().cmd)
	ec.cmd.AddCommand(newEventCancelCmd().cmd)
	ec.cmd.AddCommand(newEventMuteCmd().cmd)

	return ec
}

func addEventCmdTo(parent *cobra.Command) {
	parent.AddCommand(newEventCmd().cmd)
}
