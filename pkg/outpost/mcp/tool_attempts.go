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
		"id":               {Type: "string", Desc: "Attempt ID (required for get).", Actions: []string{"get"}},
		"tenant_id":        {Type: "string", Desc: "Filter by tenant on list; scopes the lookup to one tenant on get. " + descListValue, Actions: []string{"list", "get"}},
		"destination_id":   {Type: "string", Desc: "Filter by destination on list; scopes the lookup to one destination on get. " + descListValue, Actions: []string{"list", "get"}},
		"event_id":         {Type: "string", Desc: "Filter by event — use this to see an event's full retry history (list). " + descListValue, Actions: []string{"list"}},
		"destination_type": {Type: "string", Desc: "Filter by destination type (list). " + descListValue, Actions: []string{"list"}},
		"topic":            {Type: "string", Desc: "Filter by topic(s) (list). " + descListValue, Actions: []string{"list"}},
		"status":           {Type: "string", Desc: "Filter by outcome: success or failed (list).", Enum: []string{"success", "failed"}, Actions: []string{"list"}},
		"include":          {Type: "array", Desc: `Embed related records in the response: "event", "destination".`, Items: &mcpcore.Prop{Type: "string"}, Actions: []string{"list", "get"}},
		"time_after":       {Type: "string", Desc: descTimeAfter + " (list)", Actions: []string{"list"}},
		"time_before":      {Type: "string", Desc: descTimeBefore + " (list)", Actions: []string{"list"}},
		"limit":            {Type: "integer", Desc: "Max results (list)", Actions: []string{"list"}},
		"order_by":         {Type: "string", Desc: "Sort field: time (list)", Actions: []string{"list"}},
		"dir":              {Type: "string", Desc: "Sort direction: asc or desc (list)", Actions: []string{"list"}},
		"next":             {Type: "string", Desc: "Next page cursor (list)", Actions: []string{"list"}},
		"prev":             {Type: "string", Desc: "Previous page cursor (list)", Actions: []string{"list"}},
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
