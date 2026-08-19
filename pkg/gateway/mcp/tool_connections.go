package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

var connectionsActions = mcpcore.ActionSet{
	{Name: "list", Desc: "list connections"},
	{Name: "get", Desc: "get one connection by ID or name"},

	// pause and unpause deliberately stay Write: false, so they remain
	// available in read-only mode.
	//
	// Read-only is the mode people investigate incidents in, and pausing a
	// misbehaving connection is the natural end of an investigation: you find
	// the connection flooding a destination and you stop it. Requiring a server
	// restart with --allow-write at that moment would be the wrong trade. Both
	// are also fully reversible and destroy nothing — pausing buffers delivery
	// rather than dropping events.
	//
	// This is a decision, not an oversight. Every other mutation below is gated.
	{Name: "pause", Desc: "pause delivery on a connection; events are buffered, not dropped"},
	{Name: "unpause", Desc: "resume delivery on a paused connection"},

	{Name: "create", Desc: "create a connection between a source and a destination", Write: true},
	{Name: "upsert", Desc: "create a connection or update the existing one with the same name", Write: true},
	{Name: "update", Desc: "update a connection", Write: true},
	{Name: "delete", Desc: "delete a connection", Write: true, Destructive: true},
	{Name: "enable", Desc: "enable a disabled connection", Write: true},
	{Name: "disable", Desc: "disable a connection", Write: true},
}

var connectionsSpec = mcpcore.ToolSpec{
	Resource: "connections",
	Summary:  "Inspect and manage connections (routes linking sources to destinations). Results are scoped to the active project — call the projects tool first if the user has specified a project.",
	Actions:  connectionsActions,
	Props: map[string]mcpcore.Prop{
		"id":             {Type: "string", Desc: "Connection ID or name. Required for get/pause/unpause/update/delete/enable/disable."},
		"name":           {Type: "string", Desc: "Connection name. Filters on list; names the connection on create/upsert/update."},
		"description":    {Type: "string", Desc: "Connection description (create/upsert/update)"},
		"source_id":      {Type: "string", Desc: "Source ID. Filters on list; links the source on create/upsert/update."},
		"destination_id": {Type: "string", Desc: "Destination ID. Filters on list; links the destination on create/upsert/update."},
		"rules":          {Type: "array", Desc: "Ruleset applied to the connection (create/upsert/update). Array of rule objects; replaces the stored ruleset.", Items: &mcpcore.Prop{Type: "object"}},
		"disabled":       {Type: "boolean", Desc: "Filter disabled connections (list)"},
		"limit":          {Type: "integer", Desc: "Max results (list)"},
		"next":           {Type: "string", Desc: "Next page cursor"},
		"prev":           {Type: "string", Desc: "Previous page cursor"},
	},
	Handler: handleConnections,
}

func handleConnections(srv *mcpcore.Server) mcpsdk.ToolHandler {
	client := srv.Client()
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if r := srv.RequireAuth(); r != nil {
			return r, nil
		}

		in, err := mcpcore.ParseInput(req.Params.Arguments)
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}

		action, blocked := mcpcore.DispatchWithDefault(srv, connectionsActions, in.String("action"), "list")
		if blocked != nil {
			return blocked, nil
		}

		switch action {
		case "list":
			return connectionsList(ctx, client, in)
		case "get":
			return connectionsGet(ctx, client, in)
		case "pause":
			return connectionsPause(ctx, client, in)
		case "unpause":
			return connectionsUnpause(ctx, client, in)
		case "create":
			return connectionsCreate(ctx, client, in)
		case "upsert":
			return connectionsUpsert(ctx, client, in)
		case "update":
			return connectionsUpdate(ctx, client, in)
		case "delete":
			return connectionsDelete(ctx, client, in)
		case "enable":
			return connectionsSetEnabled(ctx, client, in, "enable")
		default:
			return connectionsSetEnabled(ctx, client, in, "disable")
		}
	}
}

// connectionRequest builds the create/upsert/update body from the tool input.
func connectionRequest(in mcpcore.Input) (*hookdeck.ConnectionCreateRequest, error) {
	req := &hookdeck.ConnectionCreateRequest{
		Name:          mcpcore.OptionalStringPtr(in, "name"),
		Description:   mcpcore.OptionalStringPtr(in, "description"),
		SourceID:      mcpcore.OptionalStringPtr(in, "source_id"),
		DestinationID: mcpcore.OptionalStringPtr(in, "destination_id"),
	}
	rules, err := ruleList(in, "rules")
	if err != nil {
		return nil, err
	}
	req.Rules = rules
	return req, nil
}

// ruleList reads the connection ruleset, which the API models as an ordered
// array of rule objects.
func ruleList(in mcpcore.Input, key string) ([]hookdeck.Rule, error) {
	v, ok := in[key]
	if !ok || v == nil {
		return nil, nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be an array of rule objects", key)
	}
	rules := make([]hookdeck.Rule, 0, len(arr))
	for i, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be a rule object", key, i)
		}
		rules = append(rules, hookdeck.Rule(m))
	}
	return rules, nil
}

func connectionsCreate(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	req, err := connectionRequest(in)
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	conn, err := client.CreateConnection(ctx, req)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(conn, client)
}

func connectionsUpsert(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	// Upsert matches on name, so without one the API cannot tell which
	// connection to update and would create a new one on every call.
	if _, err := mcpcore.RequireString(in, "name", "upsert"); err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	req, err := connectionRequest(in)
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	conn, err := client.UpsertConnection(ctx, req)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(conn, client)
}

func connectionsUpdate(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id, err := connectionIDFromInput(ctx, client, in, "update")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	req, err := connectionRequest(in)
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	conn, err := client.UpdateConnection(ctx, id, req)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(conn, client)
}

func connectionsDelete(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id, err := connectionIDFromInput(ctx, client, in, "delete")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	if err := client.DeleteConnection(ctx, id); err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(map[string]string{
		"connection_id": id,
		"status":        "deleted",
	}, client)
}

func connectionsSetEnabled(ctx context.Context, client *hookdeck.Client, in mcpcore.Input, action string) (*mcpsdk.CallToolResult, error) {
	id, err := connectionIDFromInput(ctx, client, in, action)
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	call := client.EnableConnection
	if action == "disable" {
		call = client.DisableConnection
	}
	conn, err := call(ctx, id)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(conn, client)
}

// connectionIDFromInput resolves the id argument, which may be a name, for the
// actions that address one existing connection.
func connectionIDFromInput(ctx context.Context, client *hookdeck.Client, in mcpcore.Input, action string) (string, error) {
	idOrName := in.String("id")
	if idOrName == "" {
		return "", fmt.Errorf("id or name is required for the %s action", action)
	}
	return resolveMCPConnectionID(ctx, client, idOrName)
}

func connectionsList(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	params := make(map[string]string)
	mcpcore.SetIfNonEmpty(params, "name", in.String("name"))
	mcpcore.SetIfNonEmpty(params, "source_id", in.String("source_id"))
	mcpcore.SetIfNonEmpty(params, "destination_id", in.String("destination_id"))
	mcpcore.SetInt(params, "limit", in.Int("limit", 0))
	mcpcore.SetIfNonEmpty(params, "next", in.String("next"))
	mcpcore.SetIfNonEmpty(params, "prev", in.String("prev"))

	if bp := in.BoolPtr("disabled"); bp != nil {
		if *bp {
			params["disabled_at[any]"] = "true"
		}
	}

	result, err := client.ListConnections(ctx, params)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(result, client)
}

func connectionsGet(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	idOrName := in.String("id")
	if idOrName == "" {
		return mcpcore.ErrorResult("id or name is required for the get action"), nil
	}
	id, err := resolveMCPConnectionID(ctx, client, idOrName)
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	conn, err := client.GetConnection(ctx, id)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(conn, client)
}

func connectionsPause(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	idOrName := in.String("id")
	if idOrName == "" {
		return mcpcore.ErrorResult("id or name is required for the pause action"), nil
	}
	id, err := resolveMCPConnectionID(ctx, client, idOrName)
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	conn, err := client.PauseConnection(ctx, id)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(conn, client)
}

func connectionsUnpause(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	idOrName := in.String("id")
	if idOrName == "" {
		return mcpcore.ErrorResult("id or name is required for the unpause action"), nil
	}
	id, err := resolveMCPConnectionID(ctx, client, idOrName)
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	conn, err := client.UnpauseConnection(ctx, id)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(conn, client)
}

// resolveMCPConnectionID resolves a connection ID or name to an ID.
// If the value looks like an ID (starts with conn_ or web_), it is returned as-is after
// verifying it exists; otherwise a name lookup is performed.
func resolveMCPConnectionID(ctx context.Context, client *hookdeck.Client, idOrName string) (string, error) {
	if strings.HasPrefix(idOrName, "conn_") || strings.HasPrefix(idOrName, "web_") {
		_, err := client.GetConnection(ctx, idOrName)
		if err == nil {
			return idOrName, nil
		}
		if !hookdeck.IsNotFoundError(err) {
			return "", errors.New(mcpcore.TranslateAPIError(err))
		}
	}

	params := map[string]string{"name": idOrName}
	result, err := client.ListConnections(ctx, params)
	if err != nil {
		return "", errors.New(mcpcore.TranslateAPIError(err))
	}
	if result.Pagination.Limit == 0 || len(result.Models) == 0 {
		return "", fmt.Errorf("connection not found: '%s'", idOrName)
	}
	if len(result.Models) > 1 {
		return "", fmt.Errorf("multiple connections found with name '%s', please use the connection ID instead", idOrName)
	}
	return result.Models[0].ID, nil
}
