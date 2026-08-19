package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

var sourcesActions = mcpcore.ActionSet{
	{Name: "list", Desc: "list sources"},
	{Name: "get", Desc: "get one source"},
	{Name: "create", Desc: "create a source", Write: true},
	{Name: "upsert", Desc: "create a source or update the existing one with the same name", Write: true},
	{Name: "update", Desc: "update a source", Write: true},
	{Name: "delete", Desc: "delete a source", Write: true, Destructive: true},
	{Name: "enable", Desc: "enable a disabled source", Write: true},
	{Name: "disable", Desc: "disable a source so it stops accepting requests", Write: true},
}

var sourcesSpec = mcpcore.ToolSpec{
	Resource: "sources",
	Summary:  "Inspect and manage inbound sources (HTTP endpoints that receive events). Source configuration covers the URL, verification settings, and allowed HTTP methods.",
	Actions:  sourcesActions,
	Props: map[string]mcpcore.Prop{
		"id":          {Type: "string", Desc: "Source ID. Required for get/update/delete/enable/disable."},
		"name":        {Type: "string", Desc: "Source name. Filters on list; required on create/upsert."},
		"type":        {Type: "string", Desc: "Source type, e.g. STRIPE, GITHUB, HTTP (create/upsert/update)"},
		"description": {Type: "string", Desc: "Source description (create/upsert/update)"},
		"config":      {Type: "object", Desc: "Type-specific configuration, including verification settings (create/upsert/update). Replaces the stored config."},
		"limit":       {Type: "integer", Desc: "Max results (list)"},
		"next":        {Type: "string", Desc: "Next page cursor"},
		"prev":        {Type: "string", Desc: "Previous page cursor"},
	},
	Handler: handleSources,
}

func handleSources(srv *mcpcore.Server) mcpsdk.ToolHandler {
	client := srv.Client()
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if r := srv.RequireAuth(); r != nil {
			return r, nil
		}

		in, err := mcpcore.ParseInput(req.Params.Arguments)
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}

		action, blocked := mcpcore.DispatchWithDefault(srv, sourcesActions, in.String("action"), "list")
		if blocked != nil {
			return blocked, nil
		}

		switch action {
		case "list":
			return sourcesList(ctx, client, in)
		case "get":
			return sourcesGet(ctx, client, in)
		case "create", "upsert":
			return sourcesWrite(ctx, client, in, action)
		case "update":
			return sourcesUpdate(ctx, client, in)
		case "delete":
			return sourcesDelete(ctx, client, in)
		case "enable":
			return sourcesSetEnabled(ctx, client, in, "enable")
		default:
			return sourcesSetEnabled(ctx, client, in, "disable")
		}
	}
}

// sourcesWrite handles create and upsert, which share a request body. Both
// require a name: the API keys upsert on it.
func sourcesWrite(ctx context.Context, client *hookdeck.Client, in mcpcore.Input, action string) (*mcpsdk.CallToolResult, error) {
	name, err := mcpcore.RequireString(in, "name", action)
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	config, err := mcpcore.Object(in, "config")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	req := &hookdeck.SourceCreateRequest{
		Name:        name,
		Type:        in.String("type"),
		Description: mcpcore.OptionalStringPtr(in, "description"),
		Config:      config,
	}

	call := client.CreateSource
	if action == "upsert" {
		call = client.UpsertSource
	}
	source, err := call(ctx, req)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(source, client)
}

func sourcesUpdate(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id, err := mcpcore.RequireString(in, "id", "update")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	config, err := mcpcore.Object(in, "config")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	source, err := client.UpdateSource(ctx, id, &hookdeck.SourceUpdateRequest{
		Name:        in.String("name"),
		Type:        in.String("type"),
		Description: mcpcore.OptionalStringPtr(in, "description"),
		Config:      config,
	})
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(source, client)
}

func sourcesDelete(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id, err := mcpcore.RequireString(in, "id", "delete")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	if err := client.DeleteSource(ctx, id); err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(map[string]string{
		"source_id": id,
		"status":    "deleted",
	}, client)
}

func sourcesSetEnabled(ctx context.Context, client *hookdeck.Client, in mcpcore.Input, action string) (*mcpsdk.CallToolResult, error) {
	id, err := mcpcore.RequireString(in, "id", action)
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	call := client.EnableSource
	if action == "disable" {
		call = client.DisableSource
	}
	source, err := call(ctx, id)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(source, client)
}

func sourcesList(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	params := make(map[string]string)
	mcpcore.SetIfNonEmpty(params, "name", in.String("name"))
	mcpcore.SetInt(params, "limit", in.Int("limit", 0))
	mcpcore.SetIfNonEmpty(params, "next", in.String("next"))
	mcpcore.SetIfNonEmpty(params, "prev", in.String("prev"))

	result, err := client.ListSources(ctx, params)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(result, client)
}

func sourcesGet(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return mcpcore.ErrorResult("id is required for the get action"), nil
	}
	source, err := client.GetSource(ctx, id, nil)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(source, client)
}
