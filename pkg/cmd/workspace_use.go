package cmd

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
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

	workspaceNames := make([]string, len(workspaces))
	for index, workspace := range workspaces {
		workspaceNames[index] = workspace.Name
	}

	var qs = []*survey.Question{
		{
			Name: "workspace_name",
			Prompt: &survey.Select{
				Message: "Select Workspace",
				Options: workspaceNames,
				Default: "red",
			},
			Validate: survey.Required,
		},
	}

	answers := struct {
		WorkspaceName string `survey:"workspace_name"`
	}{}

	if err = survey.Ask(qs, &answers); err != nil {
		return err
	}

	var workspace hookdeck.Workspace
	for _, tempWorkspace := range workspaces {
		if answers.WorkspaceName == tempWorkspace.Name {
			workspace = tempWorkspace
		}
	}

	return Config.UseWorkspace(lc.local, workspace.Id, workspace.Mode)
}
