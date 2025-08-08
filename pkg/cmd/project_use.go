package cmd

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/project"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type projectUseCmd struct {
	cmd   *cobra.Command
	local bool
}

func newProjectUseCmd() *projectUseCmd {
	lc := &projectUseCmd{}

	lc.cmd = &cobra.Command{
		Use:   "use",
		Args:  validators.MaximumNArgs(1),
		Short: "Select your active project for future commands",
		RunE:  lc.runProjectUseCmd,
	}
	lc.cmd.Flags().BoolVar(&lc.local, "local", false, "Pin active project to the current directory")

	return lc
}

func (lc *projectUseCmd) runProjectUseCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	projects, err := project.ListProjects(&Config)
	if err != nil {
		return err
	}

	var currentProjectName string
	projectNames := make([]string, len(projects))
	for index, project := range projects {
		projectNames[index] = project.Name
		if project.Id == Config.Profile.TeamID {
			currentProjectName = project.Name
		}
	}

	prompt := &survey.Select{
		Message: "Select Project",
		Options: projectNames,
	}

	if currentProjectName != "" {
		prompt.Default = currentProjectName
	}

	var qs = []*survey.Question{
		{
			Name:     "project_name",
			Prompt:   prompt,
			Validate: survey.Required,
		},
	}

	answers := struct {
		ProjectName string `survey:"project_name"`
	}{}

	if err = survey.Ask(qs, &answers); err != nil {
		return err
	}

	var project hookdeck.Project
	for _, tempProject := range projects {
		if answers.ProjectName == tempProject.Name {
			project = tempProject
		}
	}

	return Config.UseProject(lc.local, project.Id, project.Mode)
}
