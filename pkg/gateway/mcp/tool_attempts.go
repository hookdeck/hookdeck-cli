package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

func handleAttempts(client *hookdeck.Client) mcpsdk.ToolHandler {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if r := mcpcore.RequireAuth(client, loginToolName); r != nil {
			return r, nil
		}

		in, err := mcpcore.ParseInput(req.Params.Arguments)
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}

		action := in.String("action")
		switch action {
		case "list", "":
			return attemptsList(ctx, client, in)
		case "get":
			return attemptsGet(ctx, client, in)
		default:
			return mcpcore.ErrorResult(fmt.Sprintf("unknown action %q; expected list or get", action)), nil
		}
	}
}

func attemptsList(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	params := make(map[string]string)
	mcpcore.SetIfNonEmpty(params, "event_id", in.String("event_id"))
	mcpcore.SetInt(params, "limit", in.Int("limit", 0))
	mcpcore.SetIfNonEmpty(params, "order_by", in.String("order_by"))
	mcpcore.SetIfNonEmpty(params, "dir", in.String("dir"))
	mcpcore.SetIfNonEmpty(params, "next", in.String("next"))
	mcpcore.SetIfNonEmpty(params, "prev", in.String("prev"))

	result, err := client.ListAttempts(ctx, params)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(result, client)
}

func attemptsGet(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return mcpcore.ErrorResult("id is required for the get action"), nil
	}
	attempt, err := client.GetAttempt(ctx, id)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(attempt, client)
}
