package cmd

import (
	"errors" // Add this line
	"fmt"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/project" // Added
)

type projectCreateCmd struct {
	cmd *cobra.Command
}

func newProjectCreateCmd() *projectCreateCmd {
	lc := &projectCreateCmd{}
	lc.cmd = &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new project and set it as active",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("requires a project name argument")
			}
			if len(args) > 1 {
				return errors.New("invalid extra arguments provided, please provide only a project name")
			}
			return nil
		},
		RunE: lc.runProjectCreateCmd,
	}
	return lc
}

func (lc *projectCreateCmd) runProjectCreateCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	projectName := args[0]

	// Instantiate the API client
	parsedBaseURL, err := url.Parse(Config.APIBaseURL)
	if err != nil {
		return fmt.Errorf("error parsing API base URL: %w", err)
	}
	apiClient := &hookdeck.Client{
		BaseURL: parsedBaseURL,
		APIKey:  Config.Profile.APIKey,
	}

	// Fetch existing projects to get OrganizationID
	// project.ListProjects uses its own client initialization based on Config.
	// We can use that directly.
	existingProjects, err := project.ListProjects(&Config)
	if err != nil {
		return fmt.Errorf("error fetching existing projects: %w", err)
	}

	if len(existingProjects) == 0 {
		// This case implies we cannot determine the OrganizationID.
		// The API for creating a project requires an organization_id.
		// If the user has no projects, they might need to create one via the dashboard first,
		// or the CLI needs a way to create an initial project without a pre-existing org ID,
		// or obtain an org ID through other means (e.g. a dedicated user/org info endpoint).
		// For now, we'll error out.
		return fmt.Errorf("error: no existing projects found. Cannot determine Organization ID to create a new project. Please ensure you have at least one project or create one via the dashboard")
	}
	// Assuming all projects under this API key belong to the same organization.
	organizationID := existingProjects[0].OrganizationID
	if organizationID == "" {
		// This would indicate an issue with the data from the API or the struct mapping
		return fmt.Errorf("error: could not retrieve Organization ID from existing projects")
	}

	// Use cmd.Context() for the context
	createdProject, err := apiClient.CreateProject(cmd.Context(), projectName, organizationID, false) // isPrivate defaults to false
	if err != nil {
		return fmt.Errorf("error creating project: %w", err)
	}

	err = Config.UseProject(createdProject.Id, createdProject.Mode)
	if err != nil {
		return fmt.Errorf("error setting project '%s' as active: %w", createdProject.Name, err)
	}

	fmt.Printf("Project '%s' created successfully and set as active.\n", createdProject.Name)
	return nil
}
