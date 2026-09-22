package mcpcore

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// organizationActions is the platform organization surface.
//
// There is no list and no id: every route is /organizations/current, and the
// API offers no way to name another organization. A caller holding several
// organizations reaches them by using a credential that belongs to one.
var organizationActions = ActionSet{
	{Name: "get", Desc: "get the organization this credential belongs to"},
	{Name: "update", Desc: "rename the organization", Write: true},
}

// OrganizationSpec returns the platform organization spec for this server.
//
// Deliberately absent: API key management. /organizations/current/api-keys
// mints credentials, and an agent able to create one could grant itself scopes
// past every other boundary in this server — a read-only session that could
// mint a write-scoped key would have defeated the read/write split entirely.
// Key management belongs on the CLI, the API or the dashboard, where a human
// is the one holding the credential. See plans/mcp_read_write_tool_split.md.
func (s *Server) OrganizationSpec() ToolSpec {
	return ToolSpec{
		Resource: "organization",
		Platform: true,
		Summary: "Read or rename the Hookdeck organization this credential belongs to. " +
			"REQUIRES AN ORGANIZATION API KEY — a CLI session from hookdeck login can list projects " +
			"and work inside one, but cannot read the organization, and the API answers a bare 401 that " +
			"reads as a bad key rather than the wrong kind. " +
			"There is no organization ID: the API acts on the current organization only, " +
			"determined by the credential in use. To act on a different organization, sign in with one of its credentials.",
		Actions: organizationActions,
		Props: map[string]Prop{
			"name": {Type: "string", Desc: "New organization name (update).", Only: []string{GroupWrite}},
		},
		Notes: `Organizations and projects:
  An organization contains projects. This tool acts on the organization itself; use the projects
  tools to list, read, switch, create or change the projects inside it.

API keys are not available here:
  Creating, rolling and deleting API keys is deliberately not exposed to MCP. A key is a
  credential, and an agent that can mint one can grant itself access this server would otherwise
  refuse. Use the CLI or the Hookdeck dashboard.`,
		Handler: handleOrganization,
	}
}

func handleOrganization(srv *Server) mcpsdk.ToolHandler {
	client := srv.client
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if r := srv.RequireAuth(); r != nil {
			return r, nil
		}

		in, err := ParseInput(req.Params.Arguments)
		if err != nil {
			return ErrorResult(err.Error()), nil
		}

		action, blocked := DispatchWithDefault(srv, organizationActions, in.String("action"), "get")
		if blocked != nil {
			return blocked, nil
		}

		if action == "get" {
			org, err := srv.AccountClient().GetOrganization(ctx)
			if err != nil {
				return ErrorResult(organizationFailureMessage(err)), nil
			}
			return JSONResultEnvelopeForClient(org, client)
		}

		name := OptionalStringPtr(in, "name")
		if name == nil {
			// An empty PUT succeeds and changes nothing, which reads as success.
			return ErrorResult("name is required to update the organization"), nil
		}
		org, err := srv.AccountClient().UpdateOrganization(ctx, &hookdeck.OrganizationUpdateRequest{Name: name})
		if err != nil {
			return ErrorResult(organizationFailureMessage(err)), nil
		}
		return JSONResultEnvelopeForClient(org, client)
	}
}
