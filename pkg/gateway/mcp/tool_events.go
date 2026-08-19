package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

var eventsActions = mcpcore.ActionSet{
	{Name: "list", Desc: "list events, most recent first"},
	{Name: "get", Desc: "get one event's metadata and headers"},
	{Name: "raw_body", Desc: "get one event's payload"},
	{Name: "retry", Desc: "queue another delivery attempt for an event", Write: true},
	{Name: "cancel", Desc: "stop a scheduled event from being delivered", Write: true, Destructive: true},
	{Name: "mute", Desc: "mute a failed event so it stops raising issues", Write: true, Destructive: true},
}

var eventsSpec = mcpcore.ToolSpec{
	Resource: "events",
	Summary:  "Query events (processed deliveries routed through connections to destinations). List supports the same filters as `hookdeck gateway event list` (metadata, date range, payload search, sort). Use action raw_body with the event id to get the payload directly — do not use the requests tool for the payload when you already have an event id. Results are scoped to the active project — call the projects tool first if the user has specified a project.",
	Actions:  eventsActions,
	Props: map[string]mcpcore.Prop{
		"id":                  {Type: "string", Desc: "Event ID: filter by ID(s) on list (comma-separated), or required for get/raw_body/retry/cancel/mute"},
		"connection_id":       {Type: "string", Desc: "Filter by connection (list, maps to webhook_id)"},
		"source_id":           {Type: "string", Desc: "Filter by source (list)"},
		"destination_id":      {Type: "string", Desc: "Filter by destination (list)"},
		"status":              {Type: "string", Desc: "Event status: SCHEDULED, QUEUED, HOLD, SUCCESSFUL, FAILED, CANCELLED"},
		"attempts":            {Type: "string", Desc: "Filter by attempt count (list). Integer or API operator syntax; pass through as string."},
		"issue_id":            {Type: "string", Desc: "Filter by issue (list)"},
		"error_code":          {Type: "string", Desc: "Filter by error code (list)"},
		"response_status":     {Type: "string", Desc: "Filter by HTTP response status (list)"},
		"cli_id":              {Type: "string", Desc: "Filter by CLI listen session ID (list)"},
		"created_after":       {Type: "string", Desc: "created_at lower bound. " + descDateAfter},
		"created_before":      {Type: "string", Desc: "created_at upper bound. " + descDateBefore},
		"successful_after":    {Type: "string", Desc: "successful_at lower bound. " + descDateAfter},
		"successful_before":   {Type: "string", Desc: "successful_at upper bound. " + descDateBefore},
		"last_attempt_after":  {Type: "string", Desc: "last_attempt_at lower bound. " + descDateAfter},
		"last_attempt_before": {Type: "string", Desc: "last_attempt_at upper bound. " + descDateBefore},
		"body":                {Type: "string", Desc: "Filter by event payload body. " + descJSONFilter},
		"headers":             {Type: "string", Desc: "Filter by event headers. " + descJSONFilter},
		"parsed_query":        {Type: "string", Desc: "Filter by parsed query as JSON. " + descJSONFilter},
		"path":                {Type: "string", Desc: descPathFilter},
		"limit":               {Type: "integer", Desc: "Max results (list)"},
		"order_by":            {Type: "string", Desc: "Sort field (list)"},
		"dir":                 {Type: "string", Desc: "Sort direction: asc or desc (list)"},
		"next":                {Type: "string", Desc: "Next page cursor"},
		"prev":                {Type: "string", Desc: "Previous page cursor"},
	},
	Notes: `Date range filters (list):
  Use *_after / *_before with ISO 8601 datetimes. Do not pass API bracket keys in MCP args.
  created_after       → created_at[gte]
  created_before      → created_at[lte]
  successful_after    → successful_at[gte]
  successful_before   → successful_at[lte]
  last_attempt_after  → last_attempt_at[gte]
  last_attempt_before → last_attempt_at[lte]
  Example: {"action":"list","status":"FAILED","last_attempt_after":"2026-06-08T00:00:00Z"}

Payload search (list):
  body, headers, parsed_query — Hookdeck JSON filter syntax (object or string)
  path — partial URL path match
  Example: {"action":"list","body":{"type":"charge.succeeded"}}

Getting the payload:
  get returns metadata and headers only. Use raw_body with the event id for the payload — there is
  no need to go via the requests tool when you already have an event id.

Acting on a failure (write mode):
  retry queues another delivery attempt and is the usual follow-up to investigating a failed event.
  cancel stops a scheduled event from ever being delivered; mute stops a failed event raising
  further issues without retrying it.`,
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

		action, blocked := mcpcore.DispatchWithDefault(srv, eventsActions, in.String("action"), "list")
		if blocked != nil {
			return blocked, nil
		}

		switch action {
		case "list":
			return eventsList(ctx, client, in)
		case "get":
			return eventsGet(ctx, client, in)
		case "raw_body":
			return eventsRawBody(ctx, client, in)
		case "retry":
			return eventsRetry(ctx, client, in)
		case "cancel":
			return eventsCancel(ctx, client, in)
		default:
			return eventsMute(ctx, client, in)
		}
	}
}

func eventsRetry(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	return eventAction(ctx, client, in, "retry", "retried", client.RetryEvent)
}

func eventsCancel(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	return eventAction(ctx, client, in, "cancel", "cancelled", client.CancelEvent)
}

func eventsMute(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	return eventAction(ctx, client, in, "mute", "muted", client.MuteEvent)
}

// eventAction runs one of the by-id event mutations, which all take an event id
// and return no body, and reports the outcome in a consistent shape.
func eventAction(
	ctx context.Context,
	client *hookdeck.Client,
	in mcpcore.Input,
	action, status string,
	call func(context.Context, string) error,
) (*mcpsdk.CallToolResult, error) {
	id, err := mcpcore.RequireString(in, "id", action)
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	if err := call(ctx, id); err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(map[string]string{
		"event_id": id,
		"status":   status,
	}, client)
}

func eventsList(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	params := make(map[string]string)
	mcpcore.SetIfNonEmpty(params, "id", in.String("id"))
	// connection_id maps to webhook_id in the API
	mcpcore.SetIfNonEmpty(params, "webhook_id", in.String("connection_id"))
	mcpcore.SetIfNonEmpty(params, "source_id", in.String("source_id"))
	mcpcore.SetIfNonEmpty(params, "destination_id", in.String("destination_id"))
	mcpcore.SetIfNonEmpty(params, "status", in.String("status"))
	mcpcore.SetIfNonEmpty(params, "attempts", in.String("attempts"))
	mcpcore.SetIfNonEmpty(params, "issue_id", in.String("issue_id"))
	mcpcore.SetIfNonEmpty(params, "error_code", in.String("error_code"))
	mcpcore.SetIfNonEmpty(params, "response_status", in.String("response_status"))
	mcpcore.SetIfNonEmpty(params, "cli_id", in.String("cli_id"))
	mcpcore.SetIfNonEmpty(params, "created_at[gte]", in.String("created_after"))
	mcpcore.SetIfNonEmpty(params, "created_at[lte]", in.String("created_before"))
	mcpcore.SetIfNonEmpty(params, "successful_at[gte]", in.String("successful_after"))
	mcpcore.SetIfNonEmpty(params, "successful_at[lte]", in.String("successful_before"))
	mcpcore.SetIfNonEmpty(params, "last_attempt_at[gte]", in.String("last_attempt_after"))
	mcpcore.SetIfNonEmpty(params, "last_attempt_at[lte]", in.String("last_attempt_before"))
	mcpcore.SetInt(params, "limit", in.Int("limit", 0))
	mcpcore.SetIfNonEmpty(params, "order_by", in.String("order_by"))
	mcpcore.SetIfNonEmpty(params, "dir", in.String("dir"))
	mcpcore.SetIfNonEmpty(params, "next", in.String("next"))
	mcpcore.SetIfNonEmpty(params, "prev", in.String("prev"))
	if err := mcpcore.SetPayloadSearchFilters(params, in); err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}

	result, err := client.ListEvents(ctx, params)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(result, client)
}

func eventsGet(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return mcpcore.ErrorResult("id is required for the get action"), nil
	}
	event, err := client.GetEvent(ctx, id, nil)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(event, client)
}

func eventsRawBody(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return mcpcore.ErrorResult("id is required for the raw_body action"), nil
	}
	body, err := client.GetEventRawBody(ctx, id)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	text := string(body)
	if len(body) > maxRawBodyBytes {
		text = string(body[:maxRawBodyBytes]) + "\n... [truncated]"
	}
	return mcpcore.JSONResultEnvelopeForClient(map[string]string{"raw_body": text}, client)
}
