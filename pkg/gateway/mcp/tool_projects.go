package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func handleProjects(client *hookdeck.Client) mcpsdk.ToolHandler {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
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
	Name    string `json:"name"`
	Mode    string `json:"mode"`
	Current bool   `json:"current"`
}

func projectsList(client *hookdeck.Client) (*mcpsdk.CallToolResult, error) {
	projects, err := client.ListProjects()
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}

	entries := make([]projectEntry, len(projects))
	for i, p := range projects {
		entries[i] = projectEntry{
			ID:      p.Id,
			Name:    p.Name,
			Mode:    p.Mode,
			Current: p.Id == client.ProjectID,
		}
	}
	return JSONResult(entries)
}

func projectsUse(client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	id := in.String("project_id")
	if id == "" {
		return ErrorResult("project_id is required for the use action"), nil
	}

	// Validate project exists
	projects, err := client.ListProjects()
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}

	var name string
	for _, p := range projects {
		if p.Id == id {
			name = p.Name
			break
		}
	}
	if name == "" {
		return ErrorResult(fmt.Sprintf("project %q not found", id)), nil
	}

	client.ProjectID = id

	return JSONResult(map[string]string{
		"project_id":   id,
		"project_name": name,
		"status":       "ok",
	})
}
