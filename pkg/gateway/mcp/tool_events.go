package mcp

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

// gateway_events is the collection half of the events pair: it searches for
// events and hands back their ids. Everything you can do to one event lives on
// gateway_event — see tool_event.go.
var eventsActions = mcpcore.ActionSet{
	{Name: "list", Desc: "search events by filter, most recent first; returns event IDs. Pass request_id to list only the events one request produced"},
	{Name: "list_ignored", Desc: "list the events a request produced that a connection filter dropped; requires request_id. Takes id, paging and ordering only — the route declares no other filters"},
}

var eventsSpec = mcpcore.ToolSpec{
	Resource: "events",
	Summary: "SEARCH MANY events — plural, collection only. Find events (processed deliveries routed through connections to destinations) matching filters and get back their IDs. " +
		"List supports the same filters as `hookdeck gateway event list` (metadata, date range, payload search, sort). " +
		"To act on a specific event you already have an ID for, use " + eventToolName + " (singular). This tool only searches. " +
		"To see the events one request produced, pass request_id — this tool queries the request's own events route, which takes these same filters. " +
		"Results are scoped to the active project — call the projects tool first if the user has specified a project.",
	Actions: eventsActions,
	Props: map[string]mcpcore.Prop{
		"request_id":          {Type: "string", Desc: "Request ID (req_...) to scope to — lists the events that request produced. Required for list_ignored. This is the only way from a request to its events: GET /events declares no request_id filter, so the tool queries the request's own events route, which honours this same filter set. Not to be confused with id, which filters by event ID."},
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
		"body":                {Type: "string", JSONValue: true, Desc: "Filter by event payload body. " + descJSONFilter},
		"headers":             {Type: "string", JSONValue: true, Desc: "Filter by event headers. " + descJSONFilter},
		"parsed_query":        {Type: "string", JSONValue: true, Desc: "Filter by parsed query as JSON. " + descJSONFilter},
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
  The API offers one traversal direction only, and this tool is both ends of it.
  From a request to its events: pass request_id with action list. GET /events itself declares no
  request_id filter, so the tool queries GET /requests/{id}/events instead — it takes the same
  filters, so every argument here still applies. Use action list_ignored for the events a
  connection filter dropped; that route takes paging and ordering only.
  Going the other way, an event carries request_id: read it from the event and pass it to
  ` + requestToolName + ` with action get to see the raw inbound request.
  Requests cannot be filtered by event_id — there is no such filter, so do not look for one.`,
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

		action, blocked := mcpcore.DispatchWithDefault(srv, eventsActions, in.String("action"), "list", "To act on one event you already have an ID for, use "+eventToolName+" (singular)")
		if blocked != nil {
			return blocked, nil
		}

		if action == "list_ignored" {
			return eventsListIgnored(ctx, client, in)
		}
		return eventsList(ctx, client, in)
	}
}

// canonicalEventsStatus returns the status to send, in the API's own spelling.
//
// Ported from main, which found it on the pre-v3.0.0 tool shape: the
// request-scoped listing canonicalised and the collection listing forwarded the
// raw string, so `status: "failed"` worked one way and came back a 422 the
// other — the same enum, reached by two routes. Both are this tool now, so the
// canonicalisation belongs here once.
func canonicalEventsStatus(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if canonical, ok := hookdeck.CanonicalStatusValue(hookdeck.EventStatusValueList, value); ok {
		return canonical, nil
	}
	msg := fmt.Sprintf("status %q is not supported by %s; it filters by %s",
		value, eventsToolName, hookdeck.ValueList(hookdeck.EventStatusValueList))
	// The request log's vocabulary is the one a caller reaches for by mistake,
	// and the API's 422 would only ever name the enum it was sent to.
	if _, ok := hookdeck.CanonicalStatusValue(hookdeck.RequestLogStatusValueList, value); ok {
		msg += fmt.Sprintf(". It is a request status, which %s filters by: %s",
			requestsToolName, hookdeck.RequestLogStatusValues)
	}
	return "", errors.New(msg)
}

func eventsList(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	return eventsListing(ctx, client, in, false)
}

func eventsListIgnored(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	return eventsListing(ctx, client, in, true)
}

// ignoredEventsQueryParams is every query parameter GET
// /requests/{id}/ignored_events declares, verified against the 2026-09-01
// OpenAPI document. It is a far smaller set than its sibling
// GET /requests/{id}/events, which declares all 30 of the /events filters.
//
// The asymmetry is easy to assume away — the two routes sit beside each other
// and return the same model — and assuming it is how a caller ends up with an
// unfiltered list that reads as a filtered one.
var ignoredEventsQueryParams = map[string]bool{
	"dir": true, "id": true, "limit": true, "next": true, "order_by": true, "prev": true,
}

// refuseFiltersIgnoredEventsDrops rejects the filters this tool advertises that
// the ignored-events route does not honour, rather than sending them to be
// dropped in silence.
func refuseFiltersIgnoredEventsDrops(in mcpcore.Input) *mcpsdk.CallToolResult {
	var refused []string
	// Unknown arguments never reach here: rejectUnknownArgs wraps this handler
	// and has already refused anything the tool does not declare, with a better
	// message than this one would give.
	for name, value := range in {
		if name == "action" || name == "request_id" || !mcpcore.ArgIsSet(value) {
			continue
		}
		if ignoredEventsQueryParams[name] {
			continue
		}
		refused = append(refused, name)
	}
	if len(refused) == 0 {
		return nil
	}
	sort.Strings(refused)
	allowed := make([]string, 0, len(ignoredEventsQueryParams))
	for name := range ignoredEventsQueryParams {
		allowed = append(allowed, name)
	}
	sort.Strings(allowed)
	return mcpcore.ErrorResult(fmt.Sprintf(
		"%s cannot be used with action list_ignored: GET /requests/{id}/ignored_events does not declare %s, "+
			"so the API would ignore them and return an unfiltered list. It accepts only: %s. "+
			"To filter the events a request produced, use action list with the same request_id.",
		strings.Join(refused, ", "),
		map[bool]string{true: "it", false: "them"}[len(refused) == 1],
		strings.Join(allowed, ", "),
	))
}

// eventsListing builds the filter set once and picks the route.
//
// GET /events and GET /requests/{id}/events declare the same query parameters,
// so for those two the params map is route-independent and only the call at the
// end differs. GET /requests/{id}/ignored_events does NOT: it declares six, and
// refuseFiltersIgnoredEventsDrops above has already rejected the rest before
// this runs. Do not "simplify" by dropping that guard on the strength of this
// comment — see ignoredEventsQueryParams for the list.
func eventsListing(ctx context.Context, client *hookdeck.Client, in mcpcore.Input, ignored bool) (*mcpsdk.CallToolResult, error) {
	requestID := in.String("request_id")
	if ignored && requestID == "" {
		return mcpcore.ErrorResult("request_id is required for the list_ignored action: ignored events exist only in the context of the request that produced them, so there is no collection to list without one"), nil
	}
	if ignored {
		if refused := refuseFiltersIgnoredEventsDrops(in); refused != nil {
			return refused, nil
		}
	}

	status, err := canonicalEventsStatus(in.String("status"))
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}

	params := make(map[string]string)
	mcpcore.SetIfNonEmpty(params, "id", in.String("id"))
	// connection_id maps to webhook_id in the API
	mcpcore.SetIfNonEmpty(params, "webhook_id", in.String("connection_id"))
	mcpcore.SetIfNonEmpty(params, "source_id", in.String("source_id"))
	mcpcore.SetIfNonEmpty(params, "destination_id", in.String("destination_id"))
	mcpcore.SetIfNonEmpty(params, "status", status)
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

	var result *hookdeck.EventListResponse
	switch {
	case ignored:
		result, err = client.GetRequestIgnoredEvents(ctx, requestID, params)
	case requestID != "":
		result, err = client.GetRequestEvents(ctx, requestID, params)
	default:
		result, err = client.ListEvents(ctx, params)
	}
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(result, client)
}
