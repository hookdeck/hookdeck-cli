package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/project"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type projectListCmd struct {
	cmd *cobra.Command
}

func newProjectListCmd() *projectListCmd {
	lc := &projectListCmd{}

	lc.cmd = &cobra.Command{
		Use:     "list [<organization_substring>] [<project_substring>]",
		Args:    validators.MaximumNArgs(2),
		Short:   "List and filter projects by organization and project name substrings",
		RunE:    lc.runProjectListCmd,
		Example: `$ hookdeck project list
[Acme] Ecommerce Production (current)
[Acme] Ecommerce Staging
[Acme] Ecommerce Development`,
	}

	return lc
}

func (lc *projectListCmd) runProjectListCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	projects, err := project.ListProjects(&Config)
	if err != nil {
		return err
	}

	var filteredProjects []hookdeck.Project

	switch len(args) {
	case 0:
		filteredProjects = projects
	case 1:
		argOrgNameInput := args[0]
		argOrgNameLower := strings.ToLower(argOrgNameInput)

		for _, p := range projects {
			org, _, errParser := project.ParseProjectName(p.Name)
			if errParser != nil {
				continue
			}
			if strings.Contains(strings.ToLower(org), argOrgNameLower) {
				filteredProjects = append(filteredProjects, p)
			}
		}
	case 2:
		argOrgNameInput := args[0]
		argProjNameInput := args[1]
		argOrgNameLower := strings.ToLower(argOrgNameInput)
		argProjNameLower := strings.ToLower(argProjNameInput)

		for _, p := range projects {
			org, proj, errParser := project.ParseProjectName(p.Name)
			if errParser != nil {
				continue
			}
			if strings.Contains(strings.ToLower(org), argOrgNameLower) && strings.Contains(strings.ToLower(proj), argProjNameLower) {
				filteredProjects = append(filteredProjects, p)
			}
		}
	}

	if len(filteredProjects) == 0 {
		fmt.Println("No projects found.")
		return nil
	}

	color := ansi.Color(os.Stdout)

	for _, project := range filteredProjects {
		if project.Id == Config.Profile.ProjectId {
			fmt.Printf("%s (current)\n", color.Green(project.Name))
		} else {
			fmt.Printf("%s\n", project.Name)
		}
	}

	return nil
}
