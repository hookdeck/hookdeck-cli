package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

var destinationsActions = mcpcore.ActionSet{
	{Name: "list", Desc: "list a tenant's destinations"},
	{Name: "get", Desc: "get one destination"},
	{Name: "create", Desc: "create a destination for a tenant", Write: true},
	{Name: "update", Desc: "update a destination", Write: true},
	{Name: "delete", Desc: "delete a destination", Write: true, Destructive: true},
	{Name: "enable", Desc: "resume delivery to a destination", Write: true},
	{Name: "disable", Desc: "stop delivery to a destination without deleting it", Write: true},
}

var destinationsSpec = mcpcore.ToolSpec{
	Resource: "destinations",
	Summary:  "Inspect and manage the destinations events are delivered to. Every destination belongs to a tenant, so tenant_id is always required. Config and credentials are specific to the destination type — call outpost_destination_types to see the fields a type accepts before creating or updating one. Destinations have no name: identify one to a human by its type and target (for example \"webhook -> https://example.com/hooks\"), not by its id, which means nothing on its own.",
	Actions:  destinationsActions,
	Required: []string{"tenant_id"},
	Props: map[string]mcpcore.Prop{
		"tenant_id":   {Type: "string", Desc: "Tenant the destination belongs to (required for every action)."},
		"id":          {Type: "string", Desc: "Destination ID. Required for get/update/delete/enable/disable."},
		"type":        {Type: "string", Desc: "Destination type, e.g. webhook (required for create). On list, filters by type(s). " + descListValue},
		"topics":      {Type: "array", Desc: `Topics to subscribe to, or ["*"] for all. On list, filters by topic(s).`, Items: &mcpcore.Prop{Type: "string"}},
		"config":      {Type: "object", Desc: "Type-specific configuration, e.g. {\"url\": \"https://example.com/hooks\"} for a webhook (create/update)."},
		"credentials": {Type: "object", Desc: "Type-specific credentials (create/update). Values are write-only; the API does not return them."},
		"filter":      {Type: "object", Desc: "Delivery filter (create/update). Replaced wholesale on update, not merged."},
		"metadata":    {Type: "object", Desc: "Destination metadata as a JSON object of string values (create/update)."},
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

		action, blocked := mcpcore.Dispatch(srv, destinationsActions, in.String("action"))
		if blocked != nil {
			return blocked, nil
		}

		tenantID, err := mcpcore.RequireString(in, "tenant_id", action)
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}

		switch action {
		case "list":
			return destinationsList(ctx, client, in, tenantID)
		case "create":
			return destinationsCreate(ctx, client, in, tenantID)
		}

		id, err := mcpcore.RequireString(in, "id", action)
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}

		switch action {
		case "get":
			return destinationResult(client)(client.GetOutpostDestination(ctx, tenantID, id))
		case "update":
			return destinationsUpdate(ctx, client, in, tenantID, id)
		case "enable":
			return destinationResult(client)(client.EnableOutpostDestination(ctx, tenantID, id))
		case "disable":
			return destinationResult(client)(client.DisableOutpostDestination(ctx, tenantID, id))
		default:
			if err := client.DeleteOutpostDestination(ctx, tenantID, id); err != nil {
				return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
			}
			return mcpcore.JSONResultEnvelopeForClient(map[string]string{
				"tenant_id":      tenantID,
				"destination_id": id,
				"status":         "deleted",
			}, client)
		}
	}
}

// destinationResult adapts the client's (destination, error) returns into a
// tool result, so the single-destination actions do not each repeat it.
func destinationResult(client *hookdeck.Client) func(*hookdeck.OutpostDestination, error) (*mcpsdk.CallToolResult, error) {
	return func(destination *hookdeck.OutpostDestination, err error) (*mcpsdk.CallToolResult, error) {
		if err != nil {
			return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
		}
		return mcpcore.JSONResultEnvelopeForClient(destination, client)
	}
}

func destinationsList(ctx context.Context, client *hookdeck.Client, in mcpcore.Input, tenantID string) (*mcpsdk.CallToolResult, error) {
	destinations, err := client.ListOutpostDestinations(ctx, tenantID, mcpcore.StringList(in, "type"), mcpcore.StringList(in, "topics"))
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(destinations, client)
}

func destinationsCreate(ctx context.Context, client *hookdeck.Client, in mcpcore.Input, tenantID string) (*mcpsdk.CallToolResult, error) {
	destinationType, err := mcpcore.RequireString(in, "type", "create")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	cfg, credentials, filter, metadata, err := destinationPayload(in)
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	return destinationResult(client)(client.CreateOutpostDestination(ctx, tenantID, &hookdeck.OutpostDestinationCreateRequest{
		Type:        destinationType,
		Topics:      hookdeck.OutpostTopics(mcpcore.StringList(in, "topics")),
		Config:      cfg,
		Credentials: credentials,
		Filter:      filter,
		Metadata:    metadata,
	}))
}

func destinationsUpdate(ctx context.Context, client *hookdeck.Client, in mcpcore.Input, tenantID, id string) (*mcpsdk.CallToolResult, error) {
	cfg, credentials, filter, metadata, err := destinationPayload(in)
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	return destinationResult(client)(client.UpdateOutpostDestination(ctx, tenantID, id, &hookdeck.OutpostDestinationUpdateRequest{
		Topics:      hookdeck.OutpostTopics(mcpcore.StringList(in, "topics")),
		Config:      cfg,
		Credentials: credentials,
		Filter:      filter,
		Metadata:    metadata,
	}))
}

// destinationPayload reads the object arguments shared by create and update.
func destinationPayload(in mcpcore.Input) (cfg, credentials, filter map[string]interface{}, metadata map[string]string, err error) {
	if cfg, err = mcpcore.Object(in, "config"); err != nil {
		return nil, nil, nil, nil, err
	}
	if credentials, err = mcpcore.Object(in, "credentials"); err != nil {
		return nil, nil, nil, nil, err
	}
	if filter, err = mcpcore.Object(in, "filter"); err != nil {
		return nil, nil, nil, nil, err
	}
	if metadata, err = mcpcore.StringMap(in, "metadata"); err != nil {
		return nil, nil, nil, nil, err
	}
	return cfg, credentials, filter, metadata, nil
}
