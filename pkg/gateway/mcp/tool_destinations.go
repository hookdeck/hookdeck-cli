package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

var destinationsActions = mcpcore.ActionSet{
	{Name: "list", Desc: "list destinations"},
	{Name: "get", Desc: "get one destination"},
	{Name: "create", Desc: "create a destination", Write: true},
	{Name: "upsert", Desc: "create a destination or update the existing one with the same name", Write: true},
	{Name: "update", Desc: "update a destination", Write: true},
	{Name: "delete", Desc: "delete a destination", Write: true, Destructive: true},
	{Name: "enable", Desc: "enable a disabled destination", Write: true},
	{Name: "disable", Desc: "disable a destination so delivery stops", Write: true},
}

var destinationsSpec = mcpcore.ToolSpec{
	Resource: "destinations",
	Summary:  "Inspect and manage delivery destinations where events are sent. Destination types include HTTP endpoints, CLI (local development), and MOCK_API (testing). Configuration covers the URL, authentication, and rate limiting.",
	Actions:  destinationsActions,
	Props: map[string]mcpcore.Prop{
		"id":          {Type: "string", Desc: "Destination ID. Required for get/update/delete/enable/disable."},
		"name":        {Type: "string", Desc: "Destination name. Filters on list; required on create/upsert."},
		"type":        {Type: "string", Desc: "Destination type, e.g. HTTP, CLI, MOCK_API (create/upsert/update)", Write: true},
		"description": {Type: "string", Desc: "Destination description (create/upsert/update)", Write: true},
		"config":      {Type: "object", Desc: "Type-specific configuration: url, auth, rate limiting (create/upsert/update). Replaces the stored config.", Write: true},
		"limit":       {Type: "integer", Desc: "Max results (list)"},
		"next":        {Type: "string", Desc: "Next page cursor"},
		"prev":        {Type: "string", Desc: "Previous page cursor"},
	},
	Handler: handleDestinations,
}

func handleDestinations(srv *mcpcore.Server) mcpsdk.ToolHandler {
	client := srv.Client()
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if r := srv.RequireAuth(); r != nil {
			return r, nil
		}

		in, err := mcpcore.ParseInput(req.Params.Arguments)
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}

		action, blocked := mcpcore.DispatchWithDefault(srv, destinationsActions, in.String("action"), "list")
		if blocked != nil {
			return blocked, nil
		}

		switch action {
		case "list":
			return destinationsList(ctx, client, in)
		case "get":
			return destinationsGet(ctx, client, in)
		case "create", "upsert":
			return destinationsWrite(ctx, client, in, action)
		case "update":
			return destinationsUpdate(ctx, client, in)
		case "delete":
			return destinationsDelete(ctx, client, in)
		case "enable":
			return destinationsSetEnabled(ctx, client, in, "enable")
		default:
			return destinationsSetEnabled(ctx, client, in, "disable")
		}
	}
}

// destinationsWrite handles create and upsert, which share a request body. Both
// require a name: the API keys upsert on it.
func destinationsWrite(ctx context.Context, client *hookdeck.Client, in mcpcore.Input, action string) (*mcpsdk.CallToolResult, error) {
	name, err := mcpcore.RequireString(in, "name", action)
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	config, err := mcpcore.Object(in, "config")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	req := &hookdeck.DestinationCreateRequest{
		Name:        name,
		Type:        in.String("type"),
		Description: mcpcore.OptionalStringPtr(in, "description"),
		Config:      config,
	}

	call := client.CreateDestination
	if action == "upsert" {
		call = client.UpsertDestination
	}
	dest, err := call(ctx, req)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(dest, client)
}

func destinationsUpdate(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id, err := mcpcore.RequireString(in, "id", "update")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	config, err := mcpcore.Object(in, "config")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	dest, err := client.UpdateDestination(ctx, id, &hookdeck.DestinationUpdateRequest{
		Name:        in.String("name"),
		Type:        in.String("type"),
		Description: mcpcore.OptionalStringPtr(in, "description"),
		Config:      config,
	})
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(dest, client)
}

func destinationsDelete(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id, err := mcpcore.RequireString(in, "id", "delete")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	if err := client.DeleteDestination(ctx, id); err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(map[string]string{
		"destination_id": id,
		"status":         "deleted",
	}, client)
}

func destinationsSetEnabled(ctx context.Context, client *hookdeck.Client, in mcpcore.Input, action string) (*mcpsdk.CallToolResult, error) {
	id, err := mcpcore.RequireString(in, "id", action)
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	call := client.EnableDestination
	if action == "disable" {
		call = client.DisableDestination
	}
	dest, err := call(ctx, id)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(dest, client)
}

func destinationsList(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	params := make(map[string]string)
	mcpcore.SetIfNonEmpty(params, "name", in.String("name"))
	mcpcore.SetInt(params, "limit", in.Int("limit", 0))
	mcpcore.SetIfNonEmpty(params, "next", in.String("next"))
	mcpcore.SetIfNonEmpty(params, "prev", in.String("prev"))

	result, err := client.ListDestinations(ctx, params)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(result, client)
}

func destinationsGet(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return mcpcore.ErrorResult("id is required for the get action"), nil
	}
	dest, err := client.GetDestination(ctx, id, nil)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(dest, client)
}
