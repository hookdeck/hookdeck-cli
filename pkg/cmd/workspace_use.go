package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
	"github.com/hookdeck/hookdeck-cli/pkg/workspace"
)

type workspaceUseCmd struct {
	cmd *cobra.Command
}

func newWorkspaceUseCmd() *workspaceUseCmd {
	lc := &workspaceUseCmd{}

	lc.cmd = &cobra.Command{
		Use:   "use",
		Args:  validators.MaximumNArgs(1),
		Short: "Select an active workspace for upcoming commands",
		RunE:  lc.runWorkspaceUseCmd,
	}

	return lc
}

func (lc *workspaceUseCmd) runWorkspaceUseCmd(cmd *cobra.Command, args []string) error {	
	workspaces, err := workspace.ListWorkspaces(&Config)
	if err != nil {
		return err
	}

	workspaceOptions := make([]string, len(workspaces))

	for i := range workspaceOptions {
		workspaceOptions[i] = workspaces[i].Id + " : " + workspaces[i].Name
	}

	prompt := promptui.Select{
		Label: "Select Workspace",
		Items: workspaceOptions,
	}

	_, result, err := prompt.Run()
	if err != nil {
		return err
	}

	color := ansi.Color(os.Stdout)
	splittedResult := strings.Split(result, " : ")
	id := splittedResult[0]
	name := splittedResult[1]

	fmt.Printf("Selecting workspace %s\n", color.Green(name))
	return Config.UseWorkspace(id)
}
