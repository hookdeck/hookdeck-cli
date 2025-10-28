package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type connectionCmd struct {
	cmd *cobra.Command
}

func newConnectionCmd() *connectionCmd {
	cc := &connectionCmd{}

	cc.cmd = &cobra.Command{
		Use:   "connection",
		Args:  validators.NoArgs,
		Short: "Manage your connections",
		Long: `Manage connections between sources and destinations.

A connection links a source to a destination and defines how webhooks are routed.
You can create connections with inline source and destination creation, or reference
existing resources.`,
	}

	cc.cmd.AddCommand(newConnectionCreateCmd().cmd)
	cc.cmd.AddCommand(newConnectionListCmd().cmd)
	cc.cmd.AddCommand(newConnectionGetCmd().cmd)
	cc.cmd.AddCommand(newConnectionUpdateCmd().cmd)
	cc.cmd.AddCommand(newConnectionDeleteCmd().cmd)
	cc.cmd.AddCommand(newConnectionEnableCmd().cmd)
	cc.cmd.AddCommand(newConnectionDisableCmd().cmd)
	cc.cmd.AddCommand(newConnectionPauseCmd().cmd)
	cc.cmd.AddCommand(newConnectionUnpauseCmd().cmd)
	cc.cmd.AddCommand(newConnectionArchiveCmd().cmd)
	cc.cmd.AddCommand(newConnectionUnarchiveCmd().cmd)

	return cc
}
