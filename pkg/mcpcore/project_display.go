package mcpcore

import (
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/project"
)

// FillProjectDisplayNameIfNeeded sets target.ProjectOrg and target.ProjectName
// from the project list when target has an API key and project id but no cached
// org/name (typical after loading the profile from disk). Fails silently on API
// errors. Stdio MCP invokes tools sequentially, so this is safe without locking.
//
// lookup is the client the project list is fetched from, which is not always
// target: a product API served from its own host does not answer account-level
// requests, so the lookup has to go to the account API.
func FillProjectDisplayNameIfNeeded(lookup, target *hookdeck.Client) {
	if lookup == nil || target == nil || target.APIKey == "" || target.ProjectID == "" {
		return
	}
	if target.ProjectName != "" || target.ProjectOrg != "" {
		return
	}
	projects, err := lookup.ListProjects()
	if err != nil {
		return
	}
	items := project.NormalizeProjects(projects, target.ProjectID)
	for i := range items {
		if items[i].Id != target.ProjectID {
			continue
		}
		target.ProjectOrg = items[i].Org
		target.ProjectName = items[i].Project
		return
	}
}
