package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/cmd/outposttypes"
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
		"topics":      {Type: "array", Desc: `Topics to subscribe to, or ["*"] for all. On create, defaults to ["*"] when omitted, because the API requires topics. On update, omitting it leaves the current topics unchanged. On list, filters by topic(s).`, Items: &mcpcore.Prop{Type: "string"}},
		"config":      {Type: "object", Desc: "Type-specific configuration, e.g. {\"url\": \"https://example.com/hooks\"} for a webhook (create/update).", Write: true},
		"credentials": {Type: "object", Desc: "Type-specific credentials (create/update). Values are write-only; the API does not return them.", Write: true},
		"filter":      {Type: "object", Desc: "Delivery filter (create/update). Replaced wholesale on update, not merged.", Write: true},
		"metadata":    {Type: "object", Desc: "Destination metadata as a JSON object of string values (create/update).", Write: true},
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
	// The API requires topics, and the field is omitempty, so leaving it unset
	// sent no topics at all and the create failed with a 422. Default to
	// everything rather than failing on an omitted argument — the same choice
	// 'hookdeck outpost destination create' makes.
	topics := hookdeck.OutpostTopics(mcpcore.StringList(in, "topics"))
	if len(topics) == 0 {
		topics = hookdeck.OutpostTopics{hookdeck.OutpostTopicsWildcard}
	}

	// Fill in the schema's declared defaults for config the caller omitted. The
	// API treats an absent key as unset rather than applying the default, so
	// without this a rabbitmq destination created through the MCP server stored
	// tls unset and sent its SASL credentials in the clear.
	//
	// The CLI announces applied defaults on stderr; there is no equivalent here
	// because stderr is not part of the MCP transport. The created destination
	// is returned in full, so the applied values are visible in the response.
	//
	// Create only, for the same reason as the CLI: update is a merge patch where
	// an omitted key means "leave this alone".
	cfg, _ = outposttypes.ApplyDefaultsForType(ctx, client, destinationType, cfg)

	return destinationResult(client)(client.CreateOutpostDestination(ctx, tenantID, &hookdeck.OutpostDestinationCreateRequest{
		Type:        destinationType,
		Topics:      topics,
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
		Filter:      hookdeck.OutpostObjectPatch(filter),
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
