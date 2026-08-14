package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

var tenantsActions = actionSet{
	{name: "list", desc: "list tenants"},
	{name: "get", desc: "get one tenant by id"},
	{name: "upsert", desc: "create a tenant or update its metadata", write: true},
	{name: "delete", desc: "delete a tenant and everything belonging to it", write: true, destructive: true},
	{name: "token", desc: "mint a tenant-scoped access token", write: true},
	{name: "portal", desc: "get a URL granting access to the tenant portal", write: true},
}

var tenantsSpec = toolSpec{
	resource: "tenants",
	summary:  "Inspect and manage tenants — the end customers whose destinations events are delivered to. Tenant IDs are chosen by the operator, not generated, so upsert is the way to create one.",
	actions:  tenantsActions,
	props: map[string]mcpcore.Prop{
		"id":       {Type: "string", Desc: "Tenant ID. Required for get/upsert/delete/token/portal. On list, filters by tenant ID(s). " + descListValue},
		"metadata": {Type: "object", Desc: "Tenant metadata as a JSON object of string values (upsert). Replaces the stored metadata."},
		"theme":    {Type: "string", Desc: "Portal colour scheme: light or dark (portal)."},
		"limit":    {Type: "integer", Desc: "Max results (list)"},
		"dir":      {Type: "string", Desc: "Sort direction: asc or desc (list)"},
		"next":     {Type: "string", Desc: "Next page cursor (list)"},
		"prev":     {Type: "string", Desc: "Previous page cursor (list)"},
	},
	handler: handleTenants,
}

func handleTenants(srv *mcpcore.Server) mcpsdk.ToolHandler {
	client := srv.Client()
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if r := srv.RequireAuth(); r != nil {
			return r, nil
		}
		in, err := mcpcore.ParseInput(req.Params.Arguments)
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}

		action, blocked := dispatch(srv, tenantsActions, in.String("action"))
		if blocked != nil {
			return blocked, nil
		}

		switch action {
		case "list":
			return tenantsList(ctx, client, in)
		case "get":
			return tenantsGet(ctx, client, in)
		case "upsert":
			return tenantsUpsert(ctx, client, in)
		case "delete":
			return tenantsDelete(ctx, client, in)
		case "token":
			return tenantsToken(ctx, client, in)
		default:
			return tenantsPortal(ctx, client, in)
		}
	}
}

func tenantsList(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	result, err := client.ListOutpostTenants(ctx, hookdeck.OutpostTenantListParams{
		IDs:   stringList(in, "id"),
		Limit: in.Int("limit", 0),
		Dir:   in.String("dir"),
		Next:  in.String("next"),
		Prev:  in.String("prev"),
	})
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(result, client)
}

func tenantsGet(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id, err := requireString(in, "id", "get")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	tenant, err := client.GetOutpostTenant(ctx, id)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(tenant, client)
}

func tenantsUpsert(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id, err := requireString(in, "id", "upsert")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	metadata, err := stringMap(in, "metadata")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	tenant, err := client.UpsertOutpostTenant(ctx, id, &hookdeck.OutpostTenantUpsertRequest{Metadata: metadata})
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(tenant, client)
}

func tenantsDelete(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id, err := requireString(in, "id", "delete")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	if err := client.DeleteOutpostTenant(ctx, id); err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(map[string]string{
		"tenant_id": id,
		"status":    "deleted",
	}, client)
}

func tenantsToken(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id, err := requireString(in, "id", "token")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	token, err := client.GetOutpostTenantToken(ctx, id)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(token, client)
}

func tenantsPortal(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id, err := requireString(in, "id", "portal")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	portal, err := client.GetOutpostTenantPortalURL(ctx, id, in.String("theme"))
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(portal, client)
}
