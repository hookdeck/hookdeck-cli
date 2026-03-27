package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

const maxRawBodyBytes = 100 * 1024 // 100 KB

func handleRequests(client *hookdeck.Client) mcpsdk.ToolHandler {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if r := requireAuth(client); r != nil {
			return r, nil
		}

		in, err := parseInput(req.Params.Arguments)
		if err != nil {
			return ErrorResult(err.Error()), nil
		}

		action := in.String("action")
		switch action {
		case "list", "":
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
			return ErrorResult(fmt.Sprintf("unknown action %q; expected list, get, raw_body, events, or ignored_events", action)), nil
		}
	}
}

func requestsList(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	params := make(map[string]string)
	setIfNonEmpty(params, "source_id", in.String("source_id"))
	setIfNonEmpty(params, "status", in.String("status"))
	setIfNonEmpty(params, "rejection_cause", in.String("rejection_cause"))
	setInt(params, "limit", in.Int("limit", 0))
	setIfNonEmpty(params, "next", in.String("next"))
	setIfNonEmpty(params, "prev", in.String("prev"))

	if bp := in.BoolPtr("verified"); bp != nil {
		if *bp {
			params["verified"] = "true"
		} else {
			params["verified"] = "false"
		}
	}

	result, err := client.ListRequests(ctx, params)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResultEnvelopeForClient(result, client)
}

func requestsGet(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return ErrorResult("id is required for the get action"), nil
	}
	r, err := client.GetRequest(ctx, id, nil)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResultEnvelopeForClient(r, client)
}

func requestsRawBody(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return ErrorResult("id is required for the raw_body action"), nil
	}
	body, err := client.GetRequestRawBody(ctx, id)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	text := string(body)
	if len(body) > maxRawBodyBytes {
		text = string(body[:maxRawBodyBytes]) + "\n... [truncated]"
	}
	return JSONResultEnvelopeForClient(map[string]string{"raw_body": text}, client)
}

func requestsEvents(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return ErrorResult("id is required for the events action"), nil
	}
	result, err := client.GetRequestEvents(ctx, id, nil)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResultEnvelopeForClient(result, client)
}

func requestsIgnoredEvents(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return ErrorResult("id is required for the ignored_events action"), nil
	}
	result, err := client.GetRequestIgnoredEvents(ctx, id, nil)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResultEnvelopeForClient(result, client)
}

