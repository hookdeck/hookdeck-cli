package mcpcore

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/project"
)

// ProjectsToolDef returns the projects tool for this server, named
// "<prefix>_projects". The description is supplied by the product; the list and
// use actions are shared.
//
// When Options.ProjectFilter is set, only projects of that type are listed and
// only those can be switched to — a server can only serve the product its API
// belongs to.
func (s *Server) ProjectsToolDef(description string) ToolDef {
	return ToolDef{
		Tool: &mcpsdk.Tool{
			Name:        s.ProjectsToolName(),
			Description: description,
			InputSchema: Schema(map[string]Prop{
				"action":     {Type: "string", Desc: "Action to perform: list or use", Enum: []string{"list", "use"}},
				"project_id": {Type: "string", Desc: "Project ID (required for use action)"},
			}, "action"),
		},
		Handler: handleProjects(s),
	}
}

func handleProjects(srv *Server) mcpsdk.ToolHandler {
	client := srv.client
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if r := srv.RequireAuth(); r != nil {
			return r, nil
		}

		in, err := ParseInput(req.Params.Arguments)
		if err != nil {
			return ErrorResult(err.Error()), nil
		}

		action := in.String("action")
		switch action {
		case "list", "":
			return projectsList(srv, client)
		case "use":
			return projectsUse(srv, client, in)
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

// listProjectItems fetches the projects visible to the credentials, restricted
// to the server's project type when one is configured.
//
// The list comes from the account API, not the product one: a product served
// from its own host does not answer account-level requests.
func listProjectItems(srv *Server, client *hookdeck.Client) ([]project.ProjectListItem, error) {
	accountClient := srv.AccountClient()
	if err := project.EnsureUserAssociatedClient(accountClient); err != nil {
		return nil, err
	}

	projects, err := accountClient.ListProjects()
	if err != nil {
		return nil, err
	}

	items := project.NormalizeProjects(projects, client.ProjectID)
	filter := srv.ProjectFilter()
	if filter == "" {
		return items, nil
	}

	filtered := make([]project.ProjectListItem, 0, len(items))
	for _, it := range items {
		if it.Type == filter {
			filtered = append(filtered, it)
		}
	}
	return filtered, nil
}

func projectsList(srv *Server, client *hookdeck.Client) (*mcpsdk.CallToolResult, error) {
	items, err := listProjectItems(srv, client)
	if err != nil {
		return ErrorResult(listProjectsFailureMessage(srv, err)), nil
	}

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
	return JSONResultEnvelopeForClient(map[string]any{
		"projects": entries,
	}, client)
}

func projectsUse(srv *Server, client *hookdeck.Client, in Input) (*mcpsdk.CallToolResult, error) {
	id := in.String("project_id")
	if id == "" {
		return ErrorResult("project_id is required for the use action"), nil
	}

	items, err := listProjectItems(srv, client)
	if err != nil {
		return ErrorResult(listProjectsFailureMessage(srv, err)), nil
	}

	var found *project.ProjectListItem
	for i := range items {
		if items[i].Id == id {
			found = &items[i]
			break
		}
	}
	if found == nil {
		if filter := srv.ProjectFilter(); filter != "" {
			// The project may well exist — it is just not one this server can
			// serve, and switching to it would make every later call fail.
			return ErrorResult(fmt.Sprintf(
				"project %q not found among the %s projects available to this server. Use action list to see them.",
				id, config.ProjectTypeToJSON(filter),
			)), nil
		}
		return ErrorResult(fmt.Sprintf("project %q not found", id)), nil
	}

	// Every client this server holds has to move together, or a later call would
	// still be scoped to the previous project.
	for _, c := range srv.projectClients() {
		c.ProjectID = id
		c.ProjectOrg = found.Org
		c.ProjectName = found.Project
	}

	out := map[string]string{
		"project_id":   id,
		"project_name": found.Project,
		"type":         config.ProjectTypeToJSON(found.Type),
		"status":       "ok",
	}
	if found.Org != "" {
		out["project_org"] = found.Org
	}
	return JSONResultEnvelopeForClient(out, client)
}
