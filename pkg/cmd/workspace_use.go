package cmd

import (
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

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
		Short: "Select your active workspace for future commands",
		RunE:  lc.runWorkspaceUseCmd,
	}

	return lc
}

func (lc *workspaceUseCmd) runWorkspaceUseCmd(cmd *cobra.Command, args []string) error {	
	workspaces, err := workspace.ListWorkspaces(&Config)
	if err != nil {
		return err
	}

	templates := &promptui.SelectTemplates{
		Active:   "▸ {{ .Name | green }}",
		Inactive: "  {{ .Name }}",
		Selected: "Selecting workspace {{ .Name | green }}",
	}

	prompt := promptui.Select{
		Label: "Select Workspace",
		Items: workspaces,
		Templates: templates,
	}

	i, _, err := prompt.Run()
	if err != nil {
		return err
	}

	workspace := workspaces[i]
	return Config.UseWorkspace(workspace.Id, workspace.Mode)
}
