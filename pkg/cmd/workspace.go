package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type workspaceCmd struct {
	cmd *cobra.Command
}

func newWorkspaceCmd() *workspaceCmd {
	lc := &workspaceCmd{}

	lc.cmd = &cobra.Command{
		Use:   "workspace",
		Args:  validators.NoArgs,
		Short: "Manage your workspaces",
	}

	lc.cmd.AddCommand(newWorkspaceListCmd().cmd)
	lc.cmd.AddCommand(newWorkspaceUseCmd().cmd)

	return lc
}
