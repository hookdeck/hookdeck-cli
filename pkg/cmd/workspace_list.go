package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
	"github.com/hookdeck/hookdeck-cli/pkg/workspace"
)

type workspaceListCmd struct {
	cmd *cobra.Command
}

func newWorkspaceListCmd() *workspaceListCmd {
	lc := &workspaceListCmd{}

	lc.cmd = &cobra.Command{
		Use:   "list",
		Args:  validators.NoArgs,
		Short: "List your workspaces",
		RunE:  lc.runWorkspaceListCmd,
	}

	return lc
}

func (lc *workspaceListCmd) runWorkspaceListCmd(cmd *cobra.Command, args []string) error {	
	// TODO: validate API key ??

	workspaces, err := workspace.ListWorkspaces(&Config)
	if err != nil {
		return err
	}

	color := ansi.Color(os.Stdout)

	for _, workspace := range workspaces {
		if workspace.Id == Config.Profile.TeamID {
			fmt.Printf("%s (current)\n", color.Green(workspace.Id + ":" + workspace.Name))
		} else {
			fmt.Printf("%s\n", workspace.Id + ":" + workspace.Name)
		}
	}

	return nil
}
