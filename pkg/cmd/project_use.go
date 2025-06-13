package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/project"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

// parseProjectName extracts the organization and project name from a string
// formatted as "[organization_name] project_name".
// (The API returns project names in this format as it recognizes the request coming from the CLI.)
// It returns the organization name, project name, or an error if parsing fails.
func parseProjectName(fullName string) (orgName string, projName string, err error) {
	re := regexp.MustCompile(`^\[(.*?)\]\s*(.*)$`)
	matches := re.FindStringSubmatch(fullName)

	if len(matches) == 3 {
		org := strings.TrimSpace(matches[1])
		proj := strings.TrimSpace(matches[2])
		if org == "" || proj == "" {
			return "", "", fmt.Errorf("invalid project name format: organization or project name is empty in '%s'", fullName)
		}
		return org, proj, nil
	}
	return "", "", fmt.Errorf("could not parse project name into '[organization] project' format: '%s'", fullName)
}

type projectUseCmd struct {
	cmd *cobra.Command
	// local bool
}

func newProjectUseCmd() *projectUseCmd {
	lc := &projectUseCmd{}

	lc.cmd = &cobra.Command{
		Use:   "use [<organization_name> [<project_name>]]",
		Args:  validators.MaximumNArgs(2),
		Short: "Select your active project for future commands",
		RunE:  lc.runProjectUseCmd,
	}

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
	if len(projects) == 0 {
		return fmt.Errorf("no projects found. Please create a project first using 'hookdeck project create'")
	}

	var selectedProject hookdeck.Project
	projectFound := false

	switch len(args) {
	case 0: // Interactive: select from all projects
		var currentProjectName string
		projectDisplayNames := make([]string, len(projects))
		for i, p := range projects {
			projectDisplayNames[i] = p.Name
			if p.Id == Config.Profile.ProjectId {
				currentProjectName = p.Name
			}
		}

		answers := struct {
			SelectedFullName string `survey:"selected_full_name"`
		}{}
		qs := []*survey.Question{
			{
				Name: "selected_full_name",
				Prompt: &survey.Select{
					Message: "Select Project",
					Options: projectDisplayNames,
					Default: currentProjectName,
				},
				Validate: survey.Required,
			},
		}

		if err := survey.Ask(qs, &answers); err != nil {
			return err
		}

		for _, p := range projects {
			if answers.SelectedFullName == p.Name {
				selectedProject = p
				projectFound = true
				break
			}
		}
		if !projectFound { // Should not happen if survey selection is from projectDisplayNames
			return fmt.Errorf("internal error: selected project '%s' not found in project list", answers.SelectedFullName)
		}
	case 1: // Organization name provided, select project from this org
		argOrgNameInput := args[0]
		argOrgNameLower := strings.ToLower(argOrgNameInput)
		var orgProjects []hookdeck.Project
		var orgProjectDisplayNames []string

		for _, p := range projects {
			org, _, errParser := parseProjectName(p.Name)
			if errParser != nil {
				continue // Skip projects with names that don't match the expected format
			}
			if strings.ToLower(org) == argOrgNameLower {
				orgProjects = append(orgProjects, p)
				orgProjectDisplayNames = append(orgProjectDisplayNames, p.Name)
			}
		}

		if len(orgProjects) == 0 {
			return fmt.Errorf("no projects found for organization '%s'", argOrgNameInput)
		}

		if len(orgProjects) == 1 {
			selectedProject = orgProjects[0]
			projectFound = true
		} else { // More than one project in the org, prompt user
			answers := struct {
				SelectedFullName string `survey:"selected_full_name"`
			}{}
			qs := []*survey.Question{
				{
					Name: "selected_full_name",
					Prompt: &survey.Select{
						Message: fmt.Sprintf("Select project for organization '%s'", argOrgNameInput),
						Options: orgProjectDisplayNames,
					},
					Validate: survey.Required,
				},
			}
			if err := survey.Ask(qs, &answers); err != nil {
				return err
			}
			for _, p := range orgProjects { // Search within the filtered orgProjects
				if answers.SelectedFullName == p.Name {
					selectedProject = p
					projectFound = true
					break
				}
			}
			if !projectFound { // Should not happen
				return fmt.Errorf("internal error: selected project '%s' not found in organization list", answers.SelectedFullName)
			}
		}
	case 2: // Organization and Project name provided
		argOrgNameInput := args[0]
		argProjNameInput := args[1]
		argOrgNameLower := strings.ToLower(argOrgNameInput)
		argProjNameLower := strings.ToLower(argProjNameInput)

		for _, p := range projects {
			org, proj, errParser := parseProjectName(p.Name)
			if errParser != nil {
				continue // Skip projects with names that don't match the expected format
			}
			if strings.ToLower(org) == argOrgNameLower && strings.ToLower(proj) == argProjNameLower {
				selectedProject = p
				projectFound = true
				break
			}
		}

		if !projectFound {
			return fmt.Errorf("project '%s' in organization '%s' not found", argProjNameInput, argOrgNameInput)
		}
	default: // Should not happen due to Args validation by Cobra
		return fmt.Errorf("unexpected number of arguments: %d. Expected 0, 1, or 2", len(args))
	}

	if !projectFound {
		// This case should ideally be unreachable if all paths correctly set projectFound or error out.
		// It acts as a safeguard.
		return fmt.Errorf("an active project could not be determined based on the provided arguments")
	}

	err = Config.UseProject(selectedProject.Id, selectedProject.Mode)
	if err != nil {
		return err
	}

	color := ansi.Color(os.Stdout)
	fmt.Printf("Successfully set active project to: %s\n", color.Green(selectedProject.Name))
	return nil
}
