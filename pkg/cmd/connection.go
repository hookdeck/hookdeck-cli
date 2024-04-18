package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type connectionCmd struct {
	cmd *cobra.Command
}

func newConnectionCmd() *connectionCmd {
	lc := &connectionCmd{}

	lc.cmd = &cobra.Command{
		Use:   "connection",
		Args:  validators.NoArgs,
		Short: "Manage your connections",
	}

	lc.cmd.AddCommand(newConnectionListCmd().cmd)
	lc.cmd.AddCommand(newConnectionRetrieveCmd().cmd)
	lc.cmd.AddCommand(newConnectionDeleteCmd().cmd)

	return lc
}
