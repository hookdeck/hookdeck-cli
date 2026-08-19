package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

var eventsActions = mcpcore.ActionSet{
	{Name: "list", Desc: "list published events, most recent first"},
	{Name: "get", Desc: "get one event, including its payload"},
	{Name: "retry", Desc: "queue another delivery of an event to a destination", Write: true},
}

var eventsSpec = mcpcore.ToolSpec{
	Resource: "events",
	Summary:  "Query published events. An event is one publish, fanned out to every destination whose topic subscription matched it. Use outpost_attempts to see how delivery of an event actually went.",
	Actions:  eventsActions,
	Props: map[string]mcpcore.Prop{
		"id":             {Type: "string", Desc: "Event ID. Required for get/retry. On list, filters by event ID(s). " + descListValue},
		"tenant_id":      {Type: "string", Desc: "Tenant ID. Filters on list; optional on get. " + descListValue},
		"destination_id": {Type: "string", Desc: "Destination to deliver to (required for retry). On list, filters by matched destination(s). " + descListValue},
		"topic":          {Type: "string", Desc: "Filter by topic(s) (list). " + descListValue},
		"time_after":     {Type: "string", Desc: descTimeAfter + " (list)"},
		"time_before":    {Type: "string", Desc: descTimeBefore + " (list)"},
		"limit":          {Type: "integer", Desc: "Max results (list)"},
		"order_by":       {Type: "string", Desc: "Sort field: time (list)"},
		"dir":            {Type: "string", Desc: "Sort direction: asc or desc (list)"},
		"next":           {Type: "string", Desc: "Next page cursor (list)"},
		"prev":           {Type: "string", Desc: "Previous page cursor (list)"},
	},
	Handler: handleEvents,
}

func handleEvents(srv *mcpcore.Server) mcpsdk.ToolHandler {
	client := srv.Client()
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if r := srv.RequireAuth(); r != nil {
			return r, nil
		}
		in, err := mcpcore.ParseInput(req.Params.Arguments)
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}

		action, blocked := mcpcore.Dispatch(srv, eventsActions, in.String("action"))
		if blocked != nil {
			return blocked, nil
		}

		switch action {
		case "list":
			return eventsList(ctx, client, in)
		case "get":
			return eventsGet(ctx, client, in)
		default:
			return eventsRetry(ctx, client, in)
		}
	}
}

func eventsList(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	result, err := client.ListOutpostEvents(ctx, hookdeck.OutpostEventListParams{
		IDs:            stringList(in, "id"),
		TenantIDs:      stringList(in, "tenant_id"),
		DestinationIDs: stringList(in, "destination_id"),
		Topics:         stringList(in, "topic"),
		TimeAfter:      in.String("time_after"),
		TimeBefore:     in.String("time_before"),
		Limit:          in.Int("limit", 0),
		OrderBy:        in.String("order_by"),
		Dir:            in.String("dir"),
		Next:           in.String("next"),
		Prev:           in.String("prev"),
	})
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(result, client)
}

func eventsGet(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id, err := requireString(in, "id", "get")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	event, err := client.GetOutpostEvent(ctx, id, in.String("tenant_id"))
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(event, client)
}

func eventsRetry(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id, err := requireString(in, "id", "retry")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	destinationID, err := requireString(in, "destination_id", "retry")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	result, err := client.RetryOutpostEvent(ctx, &hookdeck.OutpostRetryRequest{
		EventID:       id,
		DestinationID: destinationID,
	})
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	// The retry is queued, not performed inline, so report acceptance rather
	// than delivery.
	return mcpcore.JSONResultEnvelopeForClient(map[string]any{
		"event_id":       id,
		"destination_id": destinationID,
		"accepted":       result.Success,
		"status":         "queued",
	}, client)
}
