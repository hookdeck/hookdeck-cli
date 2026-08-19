package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

// attempts is read-only: a delivery attempt is a record of something that
// already happened. Retrying is an action on the event, not on the attempt.
var attemptsActions = mcpcore.ActionSet{
	{Name: "list", Desc: "list delivery attempts"},
	{Name: "get", Desc: "get one attempt, including the response data"},
}

var attemptsSpec = mcpcore.ToolSpec{
	Resource: "attempts",
	Summary:  "Query delivery attempts (each HTTP request made to deliver an event to its destination). Filter by event to see retry history, response status codes, and error details.",
	Actions:  attemptsActions,
	Props: map[string]mcpcore.Prop{
		"id":       {Type: "string", Desc: "Attempt ID (required for get)"},
		"event_id": {Type: "string", Desc: "Filter by event (list)"},
		"limit":    {Type: "integer", Desc: "Max results (list)"},
		"order_by": {Type: "string", Desc: "Sort field (list)"},
		"dir":      {Type: "string", Desc: "Sort direction: asc or desc (list)"},
		"next":     {Type: "string", Desc: "Next page cursor"},
		"prev":     {Type: "string", Desc: "Previous page cursor"},
	},
	Handler: handleAttempts,
}

func handleAttempts(srv *mcpcore.Server) mcpsdk.ToolHandler {
	client := srv.Client()
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if r := srv.RequireAuth(); r != nil {
			return r, nil
		}

		in, err := mcpcore.ParseInput(req.Params.Arguments)
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}

		action, blocked := mcpcore.DispatchWithDefault(srv, attemptsActions, in.String("action"), "list")
		if blocked != nil {
			return blocked, nil
		}

		if action == "list" {
			return attemptsList(ctx, client, in)
		}
		return attemptsGet(ctx, client, in)
	}
}

func attemptsList(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	params := make(map[string]string)
	mcpcore.SetIfNonEmpty(params, "event_id", in.String("event_id"))
	mcpcore.SetInt(params, "limit", in.Int("limit", 0))
	mcpcore.SetIfNonEmpty(params, "order_by", in.String("order_by"))
	mcpcore.SetIfNonEmpty(params, "dir", in.String("dir"))
	mcpcore.SetIfNonEmpty(params, "next", in.String("next"))
	mcpcore.SetIfNonEmpty(params, "prev", in.String("prev"))

	result, err := client.ListAttempts(ctx, params)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(result, client)
}

func attemptsGet(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return mcpcore.ErrorResult("id is required for the get action"), nil
	}
	attempt, err := client.GetAttempt(ctx, id)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(attempt, client)
}
