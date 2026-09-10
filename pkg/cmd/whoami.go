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

	projectName, orgName, apiProjectType, note := resolveActiveProject(response, Config.Profile.ProjectId, func() ([]hookdeck.Project, error) {
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

	projectType := Config.Profile.ResolveProjectType()
	if projectType == "" {
		projectType = config.NormalizeProjectType(apiProjectType)
	}
	if label := config.TypeLabel(projectType); label != "" {
		fmt.Printf("Project type: %s\n", label)
	}

	return nil
}

// resolveActiveProject returns the project name, organization name, and project
// type to display. /cli-auth/validate resolves the project from the API key's
// bound team and ignores the profile's active project_id, so when the two
// differ the active project is looked up via listProjects. A non-empty note is
// returned when the active project could not be resolved and the key-bound
// values are shown instead.
func resolveActiveProject(response *hookdeck.ValidateAPIKeyResponse, activeProjectID string, listProjects func() ([]hookdeck.Project, error)) (projectName, orgName, apiProjectType, note string) {
	projectName = response.ProjectName
	orgName = response.OrganizationName
	// Newest field first: team_type, then the short-lived team_product, then team_mode.
	apiProjectType = firstKnownProjectType(response.ProjectType, response.ProjectProduct, response.ProjectMode)

	if activeProjectID == "" || activeProjectID == response.ProjectID {
		return projectName, orgName, apiProjectType, ""
	}

	projects, err := listProjects()
	if err != nil {
		note = fmt.Sprintf("Warning: could not look up the active project (%s); showing the project associated with your API key.", activeProjectID)
		return projectName, orgName, apiProjectType, note
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
		return proj, org, p.Type, ""
	}

	note = fmt.Sprintf("Warning: the active project (%s) was not found; showing the project associated with your API key. Run 'hookdeck project use' to select a project.", activeProjectID)
	return projectName, orgName, apiProjectType, note
}

// firstKnownProjectType returns the first value that resolves to a known API
// project type. The auth endpoints renamed this field twice, so a response can
// carry any one of team_type, team_product or team_mode depending on how far the
// API has been rolled out.
func firstKnownProjectType(values ...string) string {
	for _, v := range values {
		if t := config.NormalizeProjectType(v); t != "" {
			return t
		}
	}
	return ""
}
