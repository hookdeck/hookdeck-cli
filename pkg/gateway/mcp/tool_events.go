package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func handleEvents(client *hookdeck.Client) mcpsdk.ToolHandler {
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
			return eventsList(ctx, client, in)
		case "get":
			return eventsGet(ctx, client, in)
		case "raw_body":
			return eventsRawBody(ctx, client, in)
		default:
			return ErrorResult(fmt.Sprintf("unknown action %q; expected list, get, or raw_body", action)), nil
		}
	}
}

func eventsList(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	params := make(map[string]string)
	setIfNonEmpty(params, "id", in.String("id"))
	// connection_id maps to webhook_id in the API
	setIfNonEmpty(params, "webhook_id", in.String("connection_id"))
	setIfNonEmpty(params, "source_id", in.String("source_id"))
	setIfNonEmpty(params, "destination_id", in.String("destination_id"))
	setIfNonEmpty(params, "status", in.String("status"))
	setIfNonEmpty(params, "attempts", in.String("attempts"))
	setIfNonEmpty(params, "issue_id", in.String("issue_id"))
	setIfNonEmpty(params, "error_code", in.String("error_code"))
	setIfNonEmpty(params, "response_status", in.String("response_status"))
	setIfNonEmpty(params, "cli_id", in.String("cli_id"))
	setIfNonEmpty(params, "created_at[gte]", in.String("created_after"))
	setIfNonEmpty(params, "created_at[lte]", in.String("created_before"))
	setIfNonEmpty(params, "successful_at[gte]", in.String("successful_after"))
	setIfNonEmpty(params, "successful_at[lte]", in.String("successful_before"))
	setIfNonEmpty(params, "last_attempt_at[gte]", in.String("last_attempt_after"))
	setIfNonEmpty(params, "last_attempt_at[lte]", in.String("last_attempt_before"))
	setInt(params, "limit", in.Int("limit", 0))
	setIfNonEmpty(params, "order_by", in.String("order_by"))
	setIfNonEmpty(params, "dir", in.String("dir"))
	setIfNonEmpty(params, "next", in.String("next"))
	setIfNonEmpty(params, "prev", in.String("prev"))
	if err := setPayloadSearchFilters(params, in); err != nil {
		return ErrorResult(err.Error()), nil
	}

	result, err := client.ListEvents(ctx, params)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResultEnvelopeForClient(result, client)
}

func eventsGet(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return ErrorResult("id is required for the get action"), nil
	}
	event, err := client.GetEvent(ctx, id, nil)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResultEnvelopeForClient(event, client)
}

func eventsRawBody(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return ErrorResult("id is required for the raw_body action"), nil
	}
	body, err := client.GetEventRawBody(ctx, id)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	text := string(body)
	if len(body) > maxRawBodyBytes {
		text = string(body[:maxRawBodyBytes]) + "\n... [truncated]"
	}
	return JSONResultEnvelopeForClient(map[string]string{"raw_body": text}, client)
}

