package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

// gateway_requests is the collection half of the requests pair: it searches for
// requests and hands back their ids. Everything you can do to one request lives
// on gateway_request — see tool_request.go.
var requestsActions = mcpcore.ActionSet{
	{Name: "list", Desc: "search inbound requests by filter, most recent first; returns request IDs"},
}

var requestsSpec = mcpcore.ToolSpec{
	Resource: "requests",
	Summary: "SEARCH MANY requests — plural, collection only. Find inbound requests (raw HTTP data received by Hookdeck before routing) matching filters and get back their IDs. " +
		"List supports the same filters as `hookdeck gateway request list` (metadata, date range, payload search, sort). " +
		"To act on a specific request you already have an ID for, use " + requestToolName + " (singular). This tool only searches. " +
		"There is no event_id filter here: to go from an event to its request, read request_id off the event and call " + requestToolName + " with action get. " +
		"Results are scoped to the active project — call the projects tool first if the user has specified a project.",
	Actions: requestsActions,
	Props: map[string]mcpcore.Prop{
		"id":              {Type: "string", Desc: "Filter by request ID(s), comma-separated. To fetch or act on one request by ID, use " + requestToolName + " instead."},
		"source_id":       {Type: "string", Desc: "Filter by source"},
		"status":          {Type: "string", Desc: "Filter by status: accepted or rejected"},
		"rejection_cause": {Type: "string", Desc: "Filter by rejection cause"},
		"verified":        {Type: "boolean", Desc: "Filter by verification status"},
		"created_after":   {Type: "string", Desc: "created_at lower bound. " + descDateAfter},
		"created_before":  {Type: "string", Desc: "created_at upper bound. " + descDateBefore},
		"ingested_after":  {Type: "string", Desc: "ingested_at lower bound. " + descDateAfter},
		"ingested_before": {Type: "string", Desc: "ingested_at upper bound. " + descDateBefore},
		"body":            {Type: "string", Desc: "Filter by request body. " + descJSONFilter},
		"headers":         {Type: "string", Desc: "Filter by request headers. " + descJSONFilter},
		"parsed_query":    {Type: "string", Desc: "Filter by parsed query string as JSON. " + descJSONFilter},
		"path":            {Type: "string", Desc: descPathFilter},
		"search_term":     {Type: "string", Desc: descSearchTerm},
		// A request's counts are how you find the ones that fanned out to many
		// events, produced none, or were filtered out entirely.
		"events_count":     {Type: "string", Desc: "Filter by count of events the request produced. " + descCountFilter},
		"ignored_count":    {Type: "string", Desc: "Filter by count of events a connection filter dropped. " + descCountFilter},
		"cli_events_count": {Type: "string", Desc: "Filter by count of events delivered to a CLI listen session. " + descCountFilter},
		"order_by":         {Type: "string", Desc: "Sort field, e.g. created_at"},
		"dir":              {Type: "string", Desc: "Sort direction: asc or desc"},
		"limit":            {Type: "integer", Desc: "Max results"},
		"next":             {Type: "string", Desc: "Next page cursor"},
		"prev":             {Type: "string", Desc: "Previous page cursor"},
	},
	Notes: `Plural vs singular — which of the two request tools to use:
  ` + requestsToolName + ` (this tool, plural) — you have filters and want to find matching requests.
  ` + requestToolName + `  (singular)          — you already have a request ID and want to read or act on it
                              (get, raw_body, events, ignored_events, retry).
  The usual flow is ` + requestsToolName + ` to find an ID, then ` + requestToolName + ` with that ID.

Date range filters:
  Use *_after / *_before with ISO 8601 datetimes (e.g. 2026-06-01T00:00:00Z). Do not pass API bracket keys like created_at[gte] in MCP args.
  created_after   → created_at[gte]   (inclusive lower bound)
  created_before  → created_at[lte]   (inclusive upper bound)
  ingested_after  → ingested_at[gte]
  ingested_before → ingested_at[lte]
  Example: {"action":"list","ingested_after":"2026-06-09T12:00:00Z","source_id":"src_abc"}

Payload search:
  body, headers, parsed_query — Hookdeck JSON filter syntax (object or string). Same as hookdeck listen --filter-body.
  path — partial URL path match (string)
  search_term — matches a COMPLETE value across body, headers, parsed_query and path at once (min 3 chars).
                Not a substring: a field holding "pat@example.test" matches that exact string, not "example".
  Example: {"action":"list","body":{"type":"charge.succeeded"}}
  Example: {"action":"list","search_term":"cus_1234"}

Count filters:
  events_count, ignored_count and cli_events_count take an integer, passed through as written
  in the same way as attempts on ` + eventsToolName + `.
  A request that produced no events is the usual reason a webhook "went missing":
  {"action":"list","events_count":"0"} finds requests that matched no connection, and
  {"action":"list","ignored_count":"1"} finds ones a connection filter dropped an event from.

Requests and events:
  The API offers one traversal direction only. Requests cannot be filtered by event_id — there is
  no such filter, so do not look for one. From an event, read its request_id and call
  ` + requestToolName + ` with action get. From a request, call ` + requestToolName + ` with action
  events to list the events it produced.`,
	Handler: handleRequests,
}

func handleRequests(srv *mcpcore.Server) mcpsdk.ToolHandler {
	client := srv.Client()
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if r := srv.RequireAuth(); r != nil {
			return r, nil
		}

		in, err := mcpcore.ParseInput(req.Params.Arguments)
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}

		if _, blocked := mcpcore.DispatchWithDefault(srv, requestsActions, in.String("action"), "list", "To act on one request you already have an ID for, use "+requestToolName+" (singular)"); blocked != nil {
			return blocked, nil
		}

		return requestsList(ctx, client, in)
	}
}

func requestsList(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	params := make(map[string]string)
	mcpcore.SetIfNonEmpty(params, "id", in.String("id"))
	mcpcore.SetIfNonEmpty(params, "source_id", in.String("source_id"))
	mcpcore.SetIfNonEmpty(params, "status", in.String("status"))
	mcpcore.SetIfNonEmpty(params, "rejection_cause", in.String("rejection_cause"))
	mcpcore.SetIfNonEmpty(params, "search_term", in.String("search_term"))
	mcpcore.SetIfNonEmpty(params, "events_count", in.NumberOrString("events_count"))
	mcpcore.SetIfNonEmpty(params, "ignored_count", in.NumberOrString("ignored_count"))
	mcpcore.SetIfNonEmpty(params, "cli_events_count", in.NumberOrString("cli_events_count"))
	mcpcore.SetIfNonEmpty(params, "created_at[gte]", in.String("created_after"))
	mcpcore.SetIfNonEmpty(params, "created_at[lte]", in.String("created_before"))
	mcpcore.SetIfNonEmpty(params, "ingested_at[gte]", in.String("ingested_after"))
	mcpcore.SetIfNonEmpty(params, "ingested_at[lte]", in.String("ingested_before"))
	mcpcore.SetInt(params, "limit", in.Int("limit", 0))
	mcpcore.SetIfNonEmpty(params, "order_by", in.String("order_by"))
	mcpcore.SetIfNonEmpty(params, "dir", in.String("dir"))
	mcpcore.SetIfNonEmpty(params, "next", in.String("next"))
	mcpcore.SetIfNonEmpty(params, "prev", in.String("prev"))
	if err := mcpcore.SetPayloadSearchFilters(params, in); err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}

	verified, err := in.BoolOrStringE("verified")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	if bp := verified; bp != nil {
		if *bp {
			params["verified"] = "true"
		} else {
			params["verified"] = "false"
		}
	}

	result, err := client.ListRequests(ctx, params)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(result, client)
}
