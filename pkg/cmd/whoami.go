package cmd

import (
	"fmt"
	"os"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/project"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
	"github.com/spf13/cobra"
)

type whoamiCmd struct {
	cmd *cobra.Command
}

func newWhoamiCmd() *whoamiCmd {
	lc := &whoamiCmd{}

	lc.cmd = &cobra.Command{
		Use:     "whoami",
		Args:    validators.NoArgs,
		Short:   "Show the logged-in user",
		Example: "  $ hookdeck whoami",
		RunE:    lc.runWhoamiCmd,
	}

	return lc
}

func (lc *whoamiCmd) runWhoamiCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	color := ansi.Color(os.Stdout)

	fmt.Printf("\nUsing profile %s (use -p flag to use a different config profile)\n\n", color.Bold(Config.Profile.Name))

	response, err := Config.GetAPIClient().ValidateAPIKey()
	if err != nil {
		return err
	}

	projectName, orgName, projectMode, note := resolveActiveProject(response, Config.Profile.ProjectId, func() ([]hookdeck.Project, error) {
		return Config.GetAPIClient().ListProjects()
	})

	if orgName != "" {
		fmt.Printf(
			"Logged in as %s (%s) on project %s in organization %s\n",
			color.Bold(response.UserName),
			color.Bold(response.UserEmail),
			color.Bold(projectName),
			color.Bold(orgName),
		)
	} else {
		fmt.Printf(
			"Logged in as %s (%s) on project %s\n",
			color.Bold(response.UserName),
			color.Bold(response.UserEmail),
			color.Bold(projectName),
		)
	}
	if note != "" {
		fmt.Printf("%s\n", note)
	}

	projectType := Config.Profile.ProjectType
	if projectType == "" && Config.Profile.ProjectMode != "" {
		projectType = config.ModeToProjectType(Config.Profile.ProjectMode)
	}
	if projectType == "" && projectMode != "" {
		projectType = config.ModeToProjectType(projectMode)
	}
	if projectType != "" {
		fmt.Printf("Project type: %s\n", projectType)
	}

	return nil
}

// resolveActiveProject returns the project name, organization name, and project
// mode to display. /cli-auth/validate resolves the project from the API key's
// bound team and ignores the profile's active project_id, so when the two
// differ the active project is looked up via listProjects. A non-empty note is
// returned when the active project could not be resolved and the key-bound
// values are shown instead.
func resolveActiveProject(response *hookdeck.ValidateAPIKeyResponse, activeProjectID string, listProjects func() ([]hookdeck.Project, error)) (projectName, orgName, projectMode, note string) {
	projectName = response.ProjectName
	orgName = response.OrganizationName
	projectMode = response.ProjectMode

	if activeProjectID == "" || activeProjectID == response.ProjectID {
		return projectName, orgName, projectMode, ""
	}

	projects, err := listProjects()
	if err != nil {
		note = fmt.Sprintf("Warning: could not look up the active project (%s); showing the project associated with your API key.", activeProjectID)
		return projectName, orgName, projectMode, note
	}

	for _, p := range projects {
		if p.Id != activeProjectID {
			continue
		}
		org, proj, parseErr := project.ParseProjectName(p.Name)
		if parseErr != nil {
			org = ""
			proj = p.Name
		}
		return proj, org, p.Mode, ""
	}

	note = fmt.Sprintf("Warning: the active project (%s) was not found; showing the project associated with your API key. Run 'hookdeck project use' to select a project.", activeProjectID)
	return projectName, orgName, projectMode, note
}
