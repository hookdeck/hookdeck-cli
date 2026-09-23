package mcpcore

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/project"
)

// projectsActions is the platform projects surface.
//
// use is Mutates rather than Write, and takes a tool of its own. It changes
// which project every later call in the session targets — so it cannot sit on
// the read tool without costing that tool its ReadOnlyHint — but gating it
// would leave a read-only session stuck in whichever project it started in,
// which is the opposite of useful when the job is investigating another one.
// Same reasoning as connections pause/unpause.
var projectsActions = ActionSet{
	{Name: "list", Desc: "list the projects this credential can see"},

	{Name: "use", Desc: "switch the active project. Every later call in this session — events, connections, metrics, everything — is scoped to it, so calling this changes what every other tool returns", Mutates: true, Tool: "use"},
}

// ProjectsSpec returns the platform projects spec for this server. The summary
// is supplied by the product; the actions and handlers are shared.
//
// When Options.ProjectFilter is set, every action is confined to projects of
// that type: a server can only serve the product its API belongs to, so listing
// or switching to another product's project would either fail later or act
// outside what this server is for.
func (s *Server) ProjectsSpec(summary string) ToolSpec {
	return ToolSpec{
		Resource: "projects",
		Platform: true,
		Summary:  summary,
		Actions:  projectsActions,
		Props: map[string]Prop{
			"project_id": {Type: "string", Desc: "Project ID. Required for use.", Actions: []string{"use"}},
		},
		Handler: handleProjects,
	}
}

func handleProjects(srv *Server) mcpsdk.ToolHandler {
	client := srv.client
	return func(_ context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if r := srv.RequireAuth(); r != nil {
			return r, nil
		}

		in, err := ParseInput(req.Params.Arguments)
		if err != nil {
			return ErrorResult(err.Error()), nil
		}

		action, blocked := DispatchWithDefault(srv, projectsActions, in.String("action"), "list")
		if blocked != nil {
			return blocked, nil
		}

		if action == "use" {
			return projectsUse(srv, client, in)
		}
		return projectsList(srv, client)
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
