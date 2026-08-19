package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

var attemptsActions = mcpcore.ActionSet{
	{Name: "list", Desc: "list delivery attempts"},
	{Name: "get", Desc: "get one attempt, including the response data"},
}

var attemptsSpec = mcpcore.ToolSpec{
	Resource: "attempts",
	Summary:  "Query delivery attempts — each individual HTTP request made to deliver an event to a destination, with its status, response code and retry number. This is where to look when a customer reports a missing or failed delivery.",
	Actions:  attemptsActions,
	Props: map[string]mcpcore.Prop{
		"id":               {Type: "string", Desc: "Attempt ID (required for get)."},
		"tenant_id":        {Type: "string", Desc: "Filter by tenant. " + descListValue},
		"destination_id":   {Type: "string", Desc: "Filter by destination. " + descListValue},
		"event_id":         {Type: "string", Desc: "Filter by event — use this to see an event's full retry history. " + descListValue},
		"destination_type": {Type: "string", Desc: "Filter by destination type (list). " + descListValue},
		"topic":            {Type: "string", Desc: "Filter by topic(s) (list). " + descListValue},
		"status":           {Type: "string", Desc: "Filter by outcome: success or failed (list).", Enum: []string{"success", "failed"}},
		"include":          {Type: "array", Desc: `Embed related records in the response: "event", "destination".`, Items: &mcpcore.Prop{Type: "string"}},
		"time_after":       {Type: "string", Desc: descTimeAfter + " (list)"},
		"time_before":      {Type: "string", Desc: descTimeBefore + " (list)"},
		"limit":            {Type: "integer", Desc: "Max results (list)"},
		"order_by":         {Type: "string", Desc: "Sort field: time (list)"},
		"dir":              {Type: "string", Desc: "Sort direction: asc or desc (list)"},
		"next":             {Type: "string", Desc: "Next page cursor (list)"},
		"prev":             {Type: "string", Desc: "Previous page cursor (list)"},
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

		action, blocked := mcpcore.Dispatch(srv, attemptsActions, in.String("action"))
		if blocked != nil {
			return blocked, nil
		}

		if action == "list" {
			return attemptsList(ctx, client, in)
		}
		return attemptsGet(ctx, client, in)
	}
}

// singleOrEmpty returns the value when exactly one was supplied. The
// tenant-scoped attempts route needs one tenant and one destination; anything
// else has to go through the global route as a filter.
func singleOrEmpty(values []string) string {
	if len(values) == 1 {
		return values[0]
	}
	return ""
}

func attemptsList(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	tenantIDs := mcpcore.StringList(in, "tenant_id")
	destinationIDs := mcpcore.StringList(in, "destination_id")

	result, err := client.ListOutpostAttempts(ctx, hookdeck.OutpostAttemptListParams{
		TenantID:        singleOrEmpty(tenantIDs),
		DestinationID:   singleOrEmpty(destinationIDs),
		TenantIDs:       tenantIDs,
		EventIDs:        mcpcore.StringList(in, "event_id"),
		DestinationIDs:  destinationIDs,
		DestinationType: mcpcore.StringList(in, "destination_type"),
		Topics:          mcpcore.StringList(in, "topic"),
		Status:          in.String("status"),
		TimeAfter:       in.String("time_after"),
		TimeBefore:      in.String("time_before"),
		Include:         mcpcore.StringList(in, "include"),
		Limit:           in.Int("limit", 0),
		OrderBy:         in.String("order_by"),
		Dir:             in.String("dir"),
		Next:            in.String("next"),
		Prev:            in.String("prev"),
	})
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(result, client)
}

func attemptsGet(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id, err := mcpcore.RequireString(in, "id", "get")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	attempt, err := client.GetOutpostAttempt(ctx, id, hookdeck.OutpostAttemptGetParams{
		TenantID:      singleOrEmpty(mcpcore.StringList(in, "tenant_id")),
		DestinationID: singleOrEmpty(mcpcore.StringList(in, "destination_id")),
		Include:       mcpcore.StringList(in, "include"),
	})
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(attempt, client)
}
