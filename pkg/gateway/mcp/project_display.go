package mcp

import (
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/project"
)

// fillProjectDisplayNameIfNeeded sets client.ProjectOrg and client.ProjectName from
// ListProjects when the client has an API key and project id but no cached org/name
// (the profile on disk stores only project_id, so every process starts blank).
// Fails silently on API errors.
// Stdio MCP invokes tools sequentially, so this is safe without locking.
func fillProjectDisplayNameIfNeeded(client *hookdeck.Client) {
	if client == nil || client.APIKey == "" || client.ProjectID == "" {
		return
	}
	if client.ProjectName != "" || client.ProjectOrg != "" {
		return
	}
	if fillFromProjectList(client) {
		return
	}
	fillFromValidate(client)
}

// fillFromProjectList resolves the active project's org/name from GET /projects.
// Reports whether the active project was found.
func fillFromProjectList(client *hookdeck.Client) bool {
	projects, err := client.ListProjects()
	if err != nil {
		return false
	}
	items := project.NormalizeProjects(projects, client.ProjectID)
	for i := range items {
		if items[i].Id != client.ProjectID {
			continue
		}
		client.ProjectOrg = items[i].Org
		client.ProjectName = items[i].Project
		return true
	}
	return false
}

// fillFromValidate resolves the org/name from /cli-auth/validate, which reports the
// project the API key is bound to. Project-scoped credentials (hookdeck ci keys,
// dashboard API keys) cannot list projects at all, so this is the only source of a
// display name for them. The names are only applied when the key's project matches
// the active one — otherwise the meta block would name the wrong project.
func fillFromValidate(client *hookdeck.Client) {
	response, err := client.ValidateAPIKey()
	if err != nil || response == nil {
		return
	}
	if response.ProjectID != client.ProjectID {
		return
	}
	client.ProjectOrg = response.OrganizationName
	client.ProjectName = response.ProjectName
}
