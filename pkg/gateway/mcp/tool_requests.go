package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

const maxRawBodyBytes = 100 * 1024 // 100 KB

var requestsActions = mcpcore.ActionSet{
	{Name: "list", Desc: "list inbound requests"},
	{Name: "get", Desc: "get one request"},
	{Name: "raw_body", Desc: "get one request's raw body"},
	{Name: "events", Desc: "list the events a request produced"},
	{Name: "ignored_events", Desc: "list the events a request produced that were filtered out"},
	{Name: "retry", Desc: "route a request through its connections again, creating new events", Write: true},
}

var requestsSpec = mcpcore.ToolSpec{
	Resource: "requests",
	Summary:  "Query inbound requests (raw HTTP data received by Hookdeck before routing). List supports the same filters as `hookdeck gateway request list` (metadata, date range, payload search, sort). Results are scoped to the active project — call the projects tool first if the user has specified a project.",
	Actions:  requestsActions,
	Props: map[string]mcpcore.Prop{
		"id":              {Type: "string", Desc: "Request ID: filter by ID(s) on list (comma-separated), or required for get/raw_body/events/ignored_events/retry"},
		"connection_ids":  {Type: "array", Desc: "Connections to re-route the request through (retry). Omit to retry every connection the request matched.", Items: &mcpcore.Prop{Type: "string"}},
		"source_id":       {Type: "string", Desc: "Filter by source (list)"},
		"status":          {Type: "string", Desc: "Filter by status: accepted or rejected (list)"},
		"rejection_cause": {Type: "string", Desc: "Filter by rejection cause (list)"},
		"verified":        {Type: "boolean", Desc: "Filter by verification status (list)"},
		"created_after":   {Type: "string", Desc: "created_at lower bound. " + descDateAfter},
		"created_before":  {Type: "string", Desc: "created_at upper bound. " + descDateBefore},
		"ingested_after":  {Type: "string", Desc: "ingested_at lower bound. " + descDateAfter},
		"ingested_before": {Type: "string", Desc: "ingested_at upper bound. " + descDateBefore},
		"body":            {Type: "string", Desc: "Filter by request body. " + descJSONFilter},
		"headers":         {Type: "string", Desc: "Filter by request headers. " + descJSONFilter},
		"parsed_query":    {Type: "string", Desc: "Filter by parsed query string as JSON. " + descJSONFilter},
		"path":            {Type: "string", Desc: descPathFilter},
		"order_by":        {Type: "string", Desc: "Sort field (list), e.g. created_at"},
		"dir":             {Type: "string", Desc: "Sort direction: asc or desc (list)"},
		"limit":           {Type: "integer", Desc: "Max results (list)"},
		"next":            {Type: "string", Desc: "Next page cursor"},
		"prev":            {Type: "string", Desc: "Previous page cursor"},
	},
	Notes: `Date range filters (list):
  Use *_after / *_before with ISO 8601 datetimes (e.g. 2026-06-01T00:00:00Z). Do not pass API bracket keys like created_at[gte] in MCP args.
  created_after   → created_at[gte]   (inclusive lower bound)
  created_before  → created_at[lte]   (inclusive upper bound)
  ingested_after  → ingested_at[gte]
  ingested_before → ingested_at[lte]
  Example: {"action":"list","ingested_after":"2026-06-09T12:00:00Z","source_id":"src_abc"}

Payload search (list):
  body, headers, parsed_query — Hookdeck JSON filter syntax (object or string). Same as hookdeck listen --filter-body.
  path — partial URL path match (string)
  Example: {"action":"list","body":{"type":"charge.succeeded"}}

Retrying (write mode):
  retry re-routes the stored request through its connections, creating new events. It does not
  modify the original request. Omit connection_ids to retry every connection the request matched.
  Example: {"action":"retry","id":"req_abc","connection_ids":["web_123"]}`,
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

		action, blocked := mcpcore.DispatchWithDefault(srv, requestsActions, in.String("action"), "list")
		if blocked != nil {
			return blocked, nil
		}

		switch action {
		case "list":
			return requestsList(ctx, client, in)
		case "get":
			return requestsGet(ctx, client, in)
		case "raw_body":
			return requestsRawBody(ctx, client, in)
		case "events":
			return requestsEvents(ctx, client, in)
		case "ignored_events":
			return requestsIgnoredEvents(ctx, client, in)
		default:
			return requestsRetry(ctx, client, in)
		}
	}
}

func requestsRetry(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id, err := mcpcore.RequireString(in, "id", "retry")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	// An empty body retries every connection the request matched, which is what
	// the API does when webhook_ids is omitted.
	var body *hookdeck.RequestRetryRequest
	if ids := mcpcore.StringList(in, "connection_ids"); len(ids) > 0 {
		body = &hookdeck.RequestRetryRequest{WebhookIDs: ids}
	}
	if err := client.RetryRequest(ctx, id, body); err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(map[string]string{
		"request_id": id,
		"status":     "retried",
	}, client)
}

func requestsList(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	params := make(map[string]string)
	mcpcore.SetIfNonEmpty(params, "id", in.String("id"))
	mcpcore.SetIfNonEmpty(params, "source_id", in.String("source_id"))
	mcpcore.SetIfNonEmpty(params, "status", in.String("status"))
	mcpcore.SetIfNonEmpty(params, "rejection_cause", in.String("rejection_cause"))
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

	if bp := in.BoolPtr("verified"); bp != nil {
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

func requestsGet(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return mcpcore.ErrorResult("id is required for the get action"), nil
	}
	r, err := client.GetRequest(ctx, id, nil)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(r, client)
}

func requestsRawBody(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return mcpcore.ErrorResult("id is required for the raw_body action"), nil
	}
	body, err := client.GetRequestRawBody(ctx, id)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	text := string(body)
	if len(body) > maxRawBodyBytes {
		text = string(body[:maxRawBodyBytes]) + "\n... [truncated]"
	}
	return mcpcore.JSONResultEnvelopeForClient(map[string]string{"raw_body": text}, client)
}

func requestsEvents(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return mcpcore.ErrorResult("id is required for the events action"), nil
	}
	result, err := client.GetRequestEvents(ctx, id, nil)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(result, client)
}

func requestsIgnoredEvents(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return mcpcore.ErrorResult("id is required for the ignored_events action"), nil
	}
	result, err := client.GetRequestIgnoredEvents(ctx, id, nil)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(result, client)
}
