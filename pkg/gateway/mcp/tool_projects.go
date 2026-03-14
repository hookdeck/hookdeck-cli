package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/project"
)

func handleProjects(client *hookdeck.Client) mcpsdk.ToolHandler {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if r := requireAuth(client); r != nil {
			return r, nil
		}

		in, err := parseInput(req.Params.Arguments)
		if err != nil {
			return ErrorResult(err.Error()), nil
		}

		action := in.String("action")
		switch action {
		case "list", "":
			return projectsList(client)
		case "use":
			return projectsUse(client, in)
		default:
			return ErrorResult(fmt.Sprintf("unknown action %q; expected list or use", action)), nil
		}
	}
}

type projectEntry struct {
	ID      string `json:"id"`
	Org     string `json:"org"`
	Project string `json:"project"`
	Type    string `json:"type"` // lowercase: gateway, outpost, console
	Current bool   `json:"current"`
}

func projectsList(client *hookdeck.Client) (*mcpsdk.CallToolResult, error) {
	projects, err := client.ListProjects()
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}

	items := project.NormalizeProjects(projects, client.ProjectID)

	entries := make([]projectEntry, len(items))
	for i, it := range items {
		entries[i] = projectEntry{
			ID:      it.Id,
			Org:     it.Org,
			Project: it.Project,
			Type:    config.ProjectTypeToJSON(it.Type),
			Current: it.Current,
		}
	}
	return JSONResult(entries)
}

func projectsUse(client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	id := in.String("project_id")
	if id == "" {
		return ErrorResult("project_id is required for the use action"), nil
	}

	projects, err := client.ListProjects()
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}

	items := project.NormalizeProjects(projects, client.ProjectID)
	var found *project.ProjectListItem
	for i := range items {
		if items[i].Id == id {
			found = &items[i]
			break
		}
	}
	if found == nil {
		return ErrorResult(fmt.Sprintf("project %q not found", id)), nil
	}

	client.ProjectID = id

	displayName := found.Project
	if found.Org != "" {
		displayName = found.Org + " / " + found.Project
	}
	return JSONResult(map[string]string{
		"project_id":   id,
		"project_name": displayName,
		"type":         config.ProjectTypeToJSON(found.Type),
		"status":       "ok",
	})
}
