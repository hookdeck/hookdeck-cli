package cmd

import (
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
	"github.com/hookdeck/hookdeck-cli/pkg/workspace"
)

type workspaceUseCmd struct {
	cmd   *cobra.Command
	local bool
}

func newWorkspaceUseCmd() *workspaceUseCmd {
	lc := &workspaceUseCmd{}

	lc.cmd = &cobra.Command{
		Use:   "use",
		Args:  validators.MaximumNArgs(1),
		Short: "Select your active workspace for future commands",
		RunE:  lc.runWorkspaceUseCmd,
	}
	lc.cmd.Flags().BoolVar(&lc.local, "local", false, "Pin active workspace to the current directory")

	return lc
}

func (lc *workspaceUseCmd) runWorkspaceUseCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	workspaces, err := workspace.ListWorkspaces(&Config)
	if err != nil {
		return err
	}

	selectedTemplate := "Selecting workspace {{ .Name | green }}"
	if lc.local {
		selectedTemplate = "Pinning workspace {{ .Name | green }} to current directory"
	}

	templates := &promptui.SelectTemplates{
		Active:   "▸ {{ .Name | green }}",
		Inactive: "  {{ .Name }}",
		Selected: selectedTemplate,
	}

	prompt := promptui.Select{
		Label:     "Select Workspace",
		Items:     workspaces,
		Templates: templates,
	}

	i, _, err := prompt.Run()
	if err != nil {
		return err
	}

	workspace := workspaces[i]
	return Config.UseWorkspace(lc.local, workspace.Id, workspace.Mode)
}
