package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

var tenantsActions = mcpcore.ActionSet{
	{Name: "list", Desc: "list tenants"},
	{Name: "get", Desc: "get one tenant by id"},
	{Name: "upsert", Desc: "create a tenant or update its metadata", Write: true},
	{Name: "delete", Desc: "delete a tenant and everything belonging to it", Write: true, Destructive: true},
	{Name: "token", Desc: "mint a tenant-scoped access token", Write: true},
	{Name: "portal", Desc: "get a URL granting access to the tenant portal", Write: true},
}

var tenantsSpec = mcpcore.ToolSpec{
	Resource: "tenants",
	Summary:  "Inspect and manage tenants — the end customers whose destinations events are delivered to. Tenant IDs are chosen by the operator, not generated, so upsert is the way to create one — which also means the id is usually meaningful to a human and worth quoting directly.",
	Actions:  tenantsActions,
	Props: map[string]mcpcore.Prop{
		"id":       {Type: "string", Desc: "Tenant ID. Required for get/upsert/delete/token/portal. On list, filters by tenant ID(s). " + descListValue},
		"metadata": {Type: "object", Desc: "Tenant metadata as a JSON object of string values (upsert). Replaces the stored metadata.", Write: true},
		"theme":    {Type: "string", Desc: "Portal colour scheme: light or dark (portal).", Write: true},
		"limit":    {Type: "integer", Desc: "Max results (list)"},
		"dir":      {Type: "string", Desc: "Sort direction: asc or desc (list)"},
		"next":     {Type: "string", Desc: "Next page cursor (list)"},
		"prev":     {Type: "string", Desc: "Previous page cursor (list)"},
	},
	Handler: handleTenants,
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

		action, blocked := mcpcore.Dispatch(srv, tenantsActions, in.String("action"))
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
		IDs:   mcpcore.StringList(in, "id"),
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
	id, err := mcpcore.RequireString(in, "id", "get")
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
	id, err := mcpcore.RequireString(in, "id", "upsert")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	metadata, err := mcpcore.StringMap(in, "metadata")
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
	id, err := mcpcore.RequireString(in, "id", "delete")
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
	id, err := mcpcore.RequireString(in, "id", "token")
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
	id, err := mcpcore.RequireString(in, "id", "portal")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	portal, err := client.GetOutpostTenantPortalURL(ctx, id, in.String("theme"))
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(portal, client)
}
