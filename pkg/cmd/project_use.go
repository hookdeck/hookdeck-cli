package cmd

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/project"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type projectUseCmd struct {
	cmd *cobra.Command
	// local bool
}

func newProjectUseCmd() *projectUseCmd {
	lc := &projectUseCmd{}

	lc.cmd = &cobra.Command{
		Use:   "use",
		Args:  validators.MaximumNArgs(1),
		Short: "Select your active project for future commands",
		RunE:  lc.runProjectUseCmd,
	}

	// With the change in config management (either local or global, not both), this flag is no longer needed
	// TODO: consider remove / deprecate
	// lc.cmd.Flags().BoolVar(&lc.local, "local", false, "Pin active project to the current directory")

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

	var selectedProject hookdeck.Project
	var projectFound bool

	if len(args) > 0 {
		argProjectName := args[0]
		for _, p := range projects {
			if strings.EqualFold(p.Name, argProjectName) {
				selectedProject = p
				projectFound = true
				break
			}
		}

		if !projectFound {
			return fmt.Errorf("error: project '%s' not found", argProjectName)
		}
	} else {
		var currentProjectName string
		projectNames := make([]string, len(projects))
		for index, project := range projects {
			projectNames[index] = project.Name
			if project.Id == Config.Profile.ProjectId {
				currentProjectName = project.Name
			}
		}

		var qs = []*survey.Question{
			{
				Name: "project_name",
				Prompt: &survey.Select{
					Message: "Select Project",
					Options: projectNames,
					Default: currentProjectName,
				},
				Validate: survey.Required,
			},
		}

		answers := struct {
			ProjectName string `survey:"project_name"`
		}{}

		if err = survey.Ask(qs, &answers); err != nil {
			return err
		}

		for _, tempProject := range projects {
			if answers.ProjectName == tempProject.Name {
				selectedProject = tempProject
				projectFound = true // Set projectFound to true as project is selected via survey
				break
			}
		}
		// In interactive mode, if a project is selected, it's implicitly found.
		// If survey.Ask succeeds, a project name is selected.
		// The loop above will find it. If somehow it doesn't (e.g. projects list changed),
		// selectedProject would be zero-valued, leading to issues in UseProject.
		// However, the current logic assumes a match will be found if survey succeeds.
	}

	if !projectFound && len(args) == 0 { // This case should ideally not be hit if survey logic is correct
		return fmt.Errorf("error: no project selected interactively")
	}

	return Config.UseProject(selectedProject.Id, selectedProject.Mode)
}
