package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func handleAttempts(client *hookdeck.Client) mcpsdk.ToolHandler {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		in, err := parseInput(req.Params.Arguments)
		if err != nil {
			return ErrorResult(err.Error()), nil
		}

		action := in.String("action")
		switch action {
		case "list", "":
			return attemptsList(ctx, client, in)
		case "get":
			return attemptsGet(ctx, client, in)
		default:
			return ErrorResult(fmt.Sprintf("unknown action %q; expected list or get", action)), nil
		}
	}
}

func attemptsList(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	params := make(map[string]string)
	setIfNonEmpty(params, "event_id", in.String("event_id"))
	setInt(params, "limit", in.Int("limit", 0))
	setIfNonEmpty(params, "order_by", in.String("order_by"))
	setIfNonEmpty(params, "dir", in.String("dir"))
	setIfNonEmpty(params, "next", in.String("next"))
	setIfNonEmpty(params, "prev", in.String("prev"))

	result, err := client.ListAttempts(ctx, params)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResult(result)
}

func attemptsGet(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return ErrorResult("id is required for the get action"), nil
	}
	attempt, err := client.GetAttempt(ctx, id)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResult(attempt)
}
