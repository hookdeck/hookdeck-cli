package mcp

import (
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/project"
)

// fillProjectDisplayNameIfNeeded sets client.ProjectOrg and client.ProjectName from
// ListProjects when the client has an API key and project id but no cached org/name
// (typical after loading profile from disk). Fails silently on API errors.
// Stdio MCP invokes tools sequentially, so this is safe without locking.
func fillProjectDisplayNameIfNeeded(client *hookdeck.Client) {
	if client == nil || client.APIKey == "" || client.ProjectID == "" {
		return
	}
	if client.ProjectName != "" || client.ProjectOrg != "" {
		return
	}
	projects, err := client.ListProjects()
	if err != nil {
		return
	}
	items := project.NormalizeProjects(projects, client.ProjectID)
	for i := range items {
		if items[i].Id != client.ProjectID {
			continue
		}
		client.ProjectOrg = items[i].Org
		client.ProjectName = items[i].Project
		return
	}
}
