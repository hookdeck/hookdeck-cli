package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

const maxRawBodyBytes = 100 * 1024 // 100 KB

// gateway_request is the single-record half of the requests pair: every action
// here addresses one request by id. Searching for requests lives on
// gateway_requests — see tool_requests.go.
var requestActions = mcpcore.ActionSet{
	{Name: "get", Desc: "get this request"},
	{Name: "raw_body", Desc: "get this request's raw body"},
	{Name: "events", Desc: "list the events this request produced"},
	{Name: "ignored_events", Desc: "list the events this request produced that were filtered out"},
	{Name: "retry", Desc: "route this request through its connections again, creating new events", Write: true},
}

var requestSpec = mcpcore.ToolSpec{
	Resource: "request",
	Summary: "ONE request by ID — singular, single-record. Use this when you already have a request ID. Takes an id and nothing else. " +
		"To find requests in the first place — by source, status, date range or payload — use " + requestsToolName + " (plural), which takes the filters and returns IDs. This tool has no filters and cannot search. " +
		"Results are scoped to the active project — call the projects tool first if the user has specified a project.",
	Actions:  requestActions,
	Required: []string{"id"},
	Props: map[string]mcpcore.Prop{
		"id":             {Type: "string", Desc: "Request ID (required, one request). Get one from " + requestsToolName + " list, or from an event's request_id."},
		"connection_ids": {Type: "array", Desc: "Connections to re-route the request through (retry action only). Omit to retry every connection the request matched.", Items: &mcpcore.Prop{Type: "string"}, Write: true},
	},
	Notes: `Plural vs singular — which of the two request tools to use:
  ` + requestToolName + `  (this tool, singular) — you already have a request ID and want to read or act on it.
  ` + requestsToolName + ` (plural)               — you have filters and want to find matching requests.
  The usual flow is ` + requestsToolName + ` to find an ID, then ` + requestToolName + ` with that ID.

Requests and events:
  events lists the events this request produced; ignored_events lists the ones a connection filter
  dropped. This is the only relationship traversal the API offers — events cannot be filtered by
  request_id, and requests cannot be filtered by event_id, so do not look for those filters.
  Coming the other way, an event carries request_id: pass it here with action get.
  Act on an individual event returned by the events action with ` + eventToolName + `.

Retrying (write mode):
  retry re-routes the stored request through its connections, creating new events. It does not
  modify the original request. Omit connection_ids to retry every connection the request matched.
  Example: {"action":"retry","id":"req_abc","connection_ids":["web_123"]}`,
	Handler: handleRequest,
}

func handleRequest(srv *mcpcore.Server) mcpsdk.ToolHandler {
	client := srv.Client()
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if r := srv.RequireAuth(); r != nil {
			return r, nil
		}

		in, err := mcpcore.ParseInput(req.Params.Arguments)
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}

		action, blocked := mcpcore.DispatchWithDefault(srv, requestActions, in.String("action"), "get",
			"To search for requests by source, status, date range or payload, use "+requestsToolName+" (plural)")
		if blocked != nil {
			return blocked, nil
		}

		if wrong := wrongIDKind(in.String("id"), "req_", requestToolName, map[string]string{
			"evt_": eventToolName,
			"web_": "gateway_connections",
			"src_": "gateway_sources",
			"des_": "gateway_destinations",
		}); wrong != nil {
			return wrong, nil
		}

		switch action {
		case "get":
			return requestGet(ctx, client, in)
		case "raw_body":
			return requestRawBody(ctx, client, in)
		case "events":
			return requestEvents(ctx, client, in)
		case "ignored_events":
			return requestIgnoredEvents(ctx, client, in)
		default:
			return requestRetry(ctx, client, in)
		}
	}
}

func requestGet(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
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

func requestRawBody(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
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

func requestEvents(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
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

func requestIgnoredEvents(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
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

func requestRetry(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
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
	result, err := client.RetryRequest(ctx, id, body)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	// Report the events the retry created rather than a fixed "retried". A
	// retry that matches no connection — including one aimed at a connection
	// the request never went through — is accepted and creates nothing, and
	// saying "retried" for that is a wrong answer that reads like a right one.
	eventIDs := make([]string, 0, len(result.Events))
	for _, event := range result.Events {
		eventIDs = append(eventIDs, event.ID)
	}
	return mcpcore.JSONResultEnvelopeForClient(map[string]any{
		"request_id": id,
		"events":     eventIDs,
		"retried":    len(eventIDs) > 0,
	}, client)
}
