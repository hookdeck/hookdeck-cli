package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
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
		Use:   "use [<organization_name> [<project_name>]]",
		Args:  validators.MaximumNArgs(2),
		Short: "Set the active project for future commands",
		RunE:  lc.runProjectUseCmd,
	}

	lc.cmd.Flags().BoolVar(&lc.local, "local", false, "Save project to current directory (.hookdeck/config.toml)")

	return lc
}

func (lc *projectUseCmd) runProjectUseCmd(cmd *cobra.Command, args []string) error {
	// Validate flag compatibility
	if lc.local && Config.ConfigFileFlag != "" {
		return fmt.Errorf("Error: --local and --config flags cannot be used together\n  --local creates config at: .hookdeck/config.toml\n  --config uses custom path: %s", Config.ConfigFileFlag)
	}

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

		prompt := &survey.Select{
			Message: "Select Project",
			Options: projectDisplayNames,
		}

		if currentProjectName != "" {
			prompt.Default = currentProjectName
		}

		answers := struct {
			SelectedFullName string `survey:"selected_full_name"`
		}{}
		qs := []*survey.Question{
			{
				Name:     "selected_full_name",
				Prompt:   prompt,
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
			org, _, errParser := project.ParseProjectName(p.Name)
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
		var matchingProjects []hookdeck.Project

		for _, p := range projects {
			org, proj, errParser := project.ParseProjectName(p.Name)
			if errParser != nil {
				continue // Skip projects with names that don't match the expected format
			}
			if strings.ToLower(org) == argOrgNameLower && strings.ToLower(proj) == argProjNameLower {
				matchingProjects = append(matchingProjects, p)
			}
		}

		if len(matchingProjects) > 1 {
			return fmt.Errorf("multiple projects named '%s' found in organization '%s'. Projects must have unique names to be used with the `project use <org> <project>` command", argProjNameInput, argOrgNameInput)
		}

		if len(matchingProjects) == 1 {
			selectedProject = matchingProjects[0]
			projectFound = true
		}

		if !projectFound {
			return fmt.Errorf("project '%s' in organization '%s' not found", argProjNameInput, argOrgNameInput)
		}
	}

	if !projectFound {
		// This case should ideally be unreachable if all paths correctly set projectFound or error out.
		// It acts as a safeguard.
		return fmt.Errorf("a project could not be determined based on the provided arguments")
	}

	// Determine which config to update
	var configPath string
	var isNewConfig bool

	if lc.local {
		// User explicitly requested local config
		isNewConfig, err = Config.UseProjectLocal(selectedProject.Id, selectedProject.Mode)
		if err != nil {
			return err
		}

		workingDir, wdErr := os.Getwd()
		if wdErr != nil {
			return wdErr
		}
		configPath = filepath.Join(workingDir, ".hookdeck/config.toml")
	} else {
		// Smart default: check if local config exists
		workingDir, wdErr := os.Getwd()
		if wdErr != nil {
			return wdErr
		}

		localConfigPath := filepath.Join(workingDir, ".hookdeck/config.toml")
		localConfigExists, _ := Config.FileExists(localConfigPath)

		if localConfigExists {
			// Local config exists, update it
			isNewConfig, err = Config.UseProjectLocal(selectedProject.Id, selectedProject.Mode)
			if err != nil {
				return err
			}
			configPath = localConfigPath
		} else {
			// No local config, use global (existing behavior)
			err = Config.UseProject(selectedProject.Id, selectedProject.Mode)
			if err != nil {
				return err
			}

			// Get global config path from Config
			configPath = Config.GetConfigFile()
			isNewConfig = false
		}
	}

	color := ansi.Color(os.Stdout)
	fmt.Printf("Successfully set active project to: %s\n", color.Green(selectedProject.Name))

	// Show which config was updated
	if strings.Contains(configPath, ".hookdeck/config.toml") {
		if isNewConfig && lc.local {
			fmt.Printf("Created: %s\n", configPath)
			// Show security warning for new local configs
			fmt.Printf("\n%s\n", color.Yellow("Security:"))
			fmt.Printf("  Local config files contain credentials and should NOT be committed to source control.\n")
			fmt.Printf("  Add .hookdeck/ to your .gitignore file.\n")
		} else {
			fmt.Printf("Updated: %s\n", configPath)
		}
	} else {
		fmt.Printf("Saved to: %s\n", configPath)
	}

	return nil
}
