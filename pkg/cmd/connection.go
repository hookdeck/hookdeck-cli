package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

const connectionDeprecationNotice = "Deprecation notice: 'hookdeck connection' and 'hookdeck connections' are deprecated. In a future version please use 'hookdeck gateway connection'.\n"

type connectionCmd struct {
	cmd *cobra.Command
}

func newConnectionCmd() *connectionCmd {
	cc := &connectionCmd{}

	cc.cmd = &cobra.Command{
		Use:     "connection",
		Aliases: []string{"connections"},
		Args:    validators.NoArgs,
		Short:   ShortBeta("Manage your connections"),
		Long: LongBeta(`Manage connections between sources and destinations.

A connection links a source to a destination and defines how webhooks are routed.
You can create connections with inline source and destination creation, or reference
existing resources.`),
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if shouldShowConnectionDeprecation() {
				fmt.Fprint(os.Stderr, connectionDeprecationNotice)
			}
		},
	}

	cc.cmd.AddCommand(newConnectionCreateCmd().cmd)
	cc.cmd.AddCommand(newConnectionUpsertCmd().cmd)
	cc.cmd.AddCommand(newConnectionUpdateCmd().cmd)
	cc.cmd.AddCommand(newConnectionListCmd().cmd)
	cc.cmd.AddCommand(newConnectionGetCmd().cmd)
	cc.cmd.AddCommand(newConnectionDeleteCmd().cmd)
	cc.cmd.AddCommand(newConnectionEnableCmd().cmd)
	cc.cmd.AddCommand(newConnectionDisableCmd().cmd)
	cc.cmd.AddCommand(newConnectionPauseCmd().cmd)
	cc.cmd.AddCommand(newConnectionUnpauseCmd().cmd)

	return cc
}
