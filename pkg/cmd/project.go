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
		Use:     "project",
		Aliases: []string{"projects"},
		Args:    validators.NoArgs,
		Short:   "Manage your projects",
		Long:    `If you are part of multiple projects, switch between them using project management commands. Projects were previously known as "Workspaces" in the Hookdeck dashboard; the CLI has been updated to use the project terminology.`,
	}

	lc.cmd.AddCommand(newProjectListCmd().cmd)
	lc.cmd.AddCommand(newProjectUseCmd().cmd)

	return lc
}
