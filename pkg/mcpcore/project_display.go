package mcpcore

import (
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/project"
)

// FillProjectDisplayNameIfNeeded sets target.ProjectOrg and target.ProjectName
// when target has an API key and project id but no cached org/name (typical
// after loading the profile from disk). Fails silently on API errors. Stdio MCP
// invokes tools sequentially, so this is safe without locking.
//
// lookup is the client account-level requests are made from, which is not always
// target: a product API served from its own host does not answer them, so the
// lookup has to go to the account API.
//
// Resolution order matters. Validating the key is tried first because it works
// for every credential and returns the name of the key's own project. Listing
// projects only works for a user-associated key, so a project-scoped key from
// `hookdeck ci` — a common way to configure an MCP server — would otherwise
// leave the name empty and callers with nothing but an opaque id to show.
func FillProjectDisplayNameIfNeeded(lookup, target *hookdeck.Client) {
	if lookup == nil || target == nil || target.APIKey == "" || target.ProjectID == "" {
		return
	}
	if target.ProjectName != "" || target.ProjectOrg != "" {
		return
	}

	// The key's own project, available whatever kind of key it is.
	if response, err := lookup.ValidateAPIKey(); err == nil && response.ProjectID == target.ProjectID {
		target.ProjectName = response.ProjectName
		target.ProjectOrg = response.OrganizationName
		if target.ProjectName != "" || target.ProjectOrg != "" {
			return
		}
	}

	// The active project differs from the key's, so it has to be looked up.
	// Only a user-associated key can do this.
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
