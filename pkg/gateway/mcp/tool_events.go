package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

// gateway_events is the collection half of the events pair: it searches for
// events and hands back their ids. Everything you can do to one event lives on
// gateway_event — see tool_event.go.
var eventsActions = mcpcore.ActionSet{
	{Name: "list", Desc: "search events by filter, most recent first; returns event IDs"},
}

var eventsSpec = mcpcore.ToolSpec{
	Resource: "events",
	Summary: "SEARCH MANY events — plural, collection only. Find events (processed deliveries routed through connections to destinations) matching filters and get back their IDs. " +
		"List supports the same filters as `hookdeck gateway event list` (metadata, date range, payload search, sort). " +
		"To act on a specific event you already have an ID for, use " + eventToolName + " (singular). This tool only searches. " +
		"There is no request_id filter here: to see the events one request produced, call " + requestToolName + " with action events. " +
		"Results are scoped to the active project — call the projects tool first if the user has specified a project.",
	Actions: eventsActions,
	Props: map[string]mcpcore.Prop{
		"id":                  {Type: "string", Desc: "Filter by event ID(s), comma-separated. To fetch or act on one event by ID, use " + eventToolName + " instead."},
		"connection_id":       {Type: "string", Desc: "Filter by connection (maps to webhook_id)"},
		"source_id":           {Type: "string", Desc: "Filter by source"},
		"destination_id":      {Type: "string", Desc: "Filter by destination"},
		"status":              {Type: "string", Desc: "Event status: SCHEDULED, QUEUED, HOLD, SUCCESSFUL, FAILED, CANCELLED"},
		"attempts":            {Type: "string", Desc: "Filter by attempt count. " + descCountFilter},
		"issue_id":            {Type: "string", Desc: "Filter by issue"},
		"error_code":          {Type: "string", Desc: "Filter by error code"},
		"response_status":     {Type: "string", Desc: "Filter by HTTP response status"},
		"cli_id":              {Type: "string", Desc: "Filter by CLI listen session ID"},
		"delivery_group":      {Type: "string", Desc: descDeliveryGroup},
		"created_after":       {Type: "string", Desc: "created_at lower bound. " + descDateAfter},
		"created_before":      {Type: "string", Desc: "created_at upper bound. " + descDateBefore},
		"successful_after":    {Type: "string", Desc: "successful_at lower bound. " + descDateAfter},
		"successful_before":   {Type: "string", Desc: "successful_at upper bound. " + descDateBefore},
		"last_attempt_after":  {Type: "string", Desc: "last_attempt_at lower bound. " + descDateAfter},
		"last_attempt_before": {Type: "string", Desc: "last_attempt_at upper bound. " + descDateBefore},
		"next_attempt_after":  {Type: "string", Desc: "next_attempt_at lower bound — the next scheduled retry. " + descDateAfter},
		"next_attempt_before": {Type: "string", Desc: "next_attempt_at upper bound — the next scheduled retry. " + descDateBefore},
		"body":                {Type: "string", Desc: "Filter by event payload body. " + descJSONFilter},
		"headers":             {Type: "string", Desc: "Filter by event headers. " + descJSONFilter},
		"parsed_query":        {Type: "string", Desc: "Filter by parsed query as JSON. " + descJSONFilter},
		"path":                {Type: "string", Desc: descPathFilter},
		"search_term":         {Type: "string", Desc: descSearchTerm},
		"limit":               {Type: "integer", Desc: "Max results"},
		"order_by":            {Type: "string", Desc: "Sort field"},
		"dir":                 {Type: "string", Desc: "Sort direction: asc or desc"},
		"next":                {Type: "string", Desc: "Next page cursor"},
		"prev":                {Type: "string", Desc: "Previous page cursor"},
	},
	Notes: `Plural vs singular — which of the two event tools to use:
  ` + eventsToolName + ` (this tool, plural) — you have filters and want to find matching events.
  ` + eventToolName + `  (singular)         — you already have an event ID and want to read or act on it
                            (get, raw_body, retry, cancel, mute).
  The usual flow is ` + eventsToolName + ` to find an ID, then ` + eventToolName + ` with that ID.

Date range filters:
  Use *_after / *_before with ISO 8601 datetimes. Do not pass API bracket keys in MCP args.
  created_after       → created_at[gte]
  created_before      → created_at[lte]
  successful_after    → successful_at[gte]
  successful_before   → successful_at[lte]
  last_attempt_after  → last_attempt_at[gte]
  last_attempt_before → last_attempt_at[lte]
  next_attempt_after  → next_attempt_at[gte]
  next_attempt_before → next_attempt_at[lte]
  Example: {"action":"list","status":"FAILED","last_attempt_after":"2026-06-08T00:00:00Z"}
  next_attempt_at is the next scheduled retry, so it looks forward rather than back:
  {"action":"list","status":"QUEUED","next_attempt_before":"2026-06-09T00:00:00Z"} finds
  events due to be retried before that time.

Payload search:
  body, headers, parsed_query — Hookdeck JSON filter syntax (object or string)
  path — partial URL path match
  search_term — matches a COMPLETE value across body, headers, parsed_query and path at once (min 3 chars).
                Not a substring: a field holding "pat@example.test" matches that exact string, not "example".
  Example: {"action":"list","body":{"type":"charge.succeeded"}}
  Example: {"action":"list","search_term":"cus_1234"}

Delivery group:
  delivery_group filters by the group an event was delivered in; comma-separate several.
  The API's own schema is nullable and documents null as "matches events without a delivery
  group", but that null cannot survive a query string: sending the string "null" is read as a
  group of that name and returns nothing. There is no filter for "has no delivery group".

Requests and events:
  The API offers one traversal direction only. Events cannot be filtered by request_id — there is
  no such filter, so do not look for one. To get the events a request produced, call
  ` + requestToolName + ` with action events (or ignored_events for the ones filtered out).
  Going the other way, an event carries request_id: read it from the event and pass it to
  ` + requestToolName + ` with action get.`,
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

		if _, blocked := mcpcore.DispatchWithDefault(srv, eventsActions, in.String("action"), "list", "To act on one event you already have an ID for, use "+eventToolName+" (singular)"); blocked != nil {
			return blocked, nil
		}

		return eventsList(ctx, client, in)
	}
}

func eventsList(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	params := make(map[string]string)
	mcpcore.SetIfNonEmpty(params, "id", in.String("id"))
	// connection_id maps to webhook_id in the API
	mcpcore.SetIfNonEmpty(params, "webhook_id", in.String("connection_id"))
	mcpcore.SetIfNonEmpty(params, "source_id", in.String("source_id"))
	mcpcore.SetIfNonEmpty(params, "destination_id", in.String("destination_id"))
	mcpcore.SetIfNonEmpty(params, "status", in.String("status"))
	mcpcore.SetIfNonEmpty(params, "attempts", in.NumberOrString("attempts"))
	mcpcore.SetIfNonEmpty(params, "issue_id", in.String("issue_id"))
	mcpcore.SetIfNonEmpty(params, "error_code", in.String("error_code"))
	mcpcore.SetIfNonEmpty(params, "response_status", in.NumberOrString("response_status"))
	mcpcore.SetIfNonEmpty(params, "cli_id", in.String("cli_id"))
	mcpcore.SetIfNonEmpty(params, "delivery_group", in.String("delivery_group"))
	mcpcore.SetIfNonEmpty(params, "search_term", in.String("search_term"))
	mcpcore.SetIfNonEmpty(params, "created_at[gte]", in.String("created_after"))
	mcpcore.SetIfNonEmpty(params, "created_at[lte]", in.String("created_before"))
	mcpcore.SetIfNonEmpty(params, "successful_at[gte]", in.String("successful_after"))
	mcpcore.SetIfNonEmpty(params, "successful_at[lte]", in.String("successful_before"))
	mcpcore.SetIfNonEmpty(params, "last_attempt_at[gte]", in.String("last_attempt_after"))
	mcpcore.SetIfNonEmpty(params, "last_attempt_at[lte]", in.String("last_attempt_before"))
	mcpcore.SetIfNonEmpty(params, "next_attempt_at[gte]", in.String("next_attempt_after"))
	mcpcore.SetIfNonEmpty(params, "next_attempt_at[lte]", in.String("next_attempt_before"))
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
