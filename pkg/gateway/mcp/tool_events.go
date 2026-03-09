package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func handleEvents(client *hookdeck.Client) mcpsdk.ToolHandler {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
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
		case "retry":
			return eventsRetry(ctx, client, in)
		case "cancel":
			return eventsCancel(ctx, client, in)
		case "mute":
			return eventsMute(ctx, client, in)
		default:
			return ErrorResult(fmt.Sprintf("unknown action %q; expected list, get, raw_body, retry, cancel, or mute", action)), nil
		}
	}
}

func eventsList(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	params := make(map[string]string)
	// connection_id maps to webhook_id in the API
	setIfNonEmpty(params, "webhook_id", in.String("connection_id"))
	setIfNonEmpty(params, "source_id", in.String("source_id"))
	setIfNonEmpty(params, "destination_id", in.String("destination_id"))
	setIfNonEmpty(params, "status", in.String("status"))
	setIfNonEmpty(params, "issue_id", in.String("issue_id"))
	setIfNonEmpty(params, "error_code", in.String("error_code"))
	setIfNonEmpty(params, "response_status", in.String("response_status"))
	// Date range mapping
	setIfNonEmpty(params, "created_at[gte]", in.String("created_after"))
	setIfNonEmpty(params, "created_at[lte]", in.String("created_before"))
	setInt(params, "limit", in.Int("limit", 0))
	setIfNonEmpty(params, "order_by", in.String("order_by"))
	setIfNonEmpty(params, "dir", in.String("dir"))
	setIfNonEmpty(params, "next", in.String("next"))
	setIfNonEmpty(params, "prev", in.String("prev"))

	result, err := client.ListEvents(ctx, params)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResult(result)
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
	return JSONResult(event)
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
	return JSONResult(map[string]string{"raw_body": text})
}

func eventsRetry(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return ErrorResult("id is required for the retry action"), nil
	}
	if err := client.RetryEvent(ctx, id); err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResult(map[string]string{
		"status":   "ok",
		"action":   "retry",
		"event_id": id,
	})
}

func eventsCancel(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return ErrorResult("id is required for the cancel action"), nil
	}
	if err := client.CancelEvent(ctx, id); err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResult(map[string]string{
		"status":   "ok",
		"action":   "cancel",
		"event_id": id,
	})
}

func eventsMute(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return ErrorResult("id is required for the mute action"), nil
	}
	if err := client.MuteEvent(ctx, id); err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResult(map[string]string{
		"status":   "ok",
		"action":   "mute",
		"event_id": id,
	})
}
