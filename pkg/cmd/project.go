package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type projectCmd struct {
	cmd *cobra.Command
}

func newProjectCmd() *projectCmd {
	lc := &projectCmd{}

	lc.cmd = &cobra.Command{
		Use:   "project",
		Args:  validators.NoArgs,
		Short: "Manage your projects",
	}

	lc.cmd.AddCommand(newProjectListCmd().cmd)
	lc.cmd.AddCommand(newProjectCreateCmd().cmd) // Added
	lc.cmd.AddCommand(newProjectUseCmd().cmd)

	return lc
}
