package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func handleConnections(client *hookdeck.Client) mcpsdk.ToolHandler {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		in, err := parseInput(req.Params.Arguments)
		if err != nil {
			return ErrorResult(err.Error()), nil
		}

		action := in.String("action")
		switch action {
		case "list", "":
			return connectionsList(ctx, client, in)
		case "get":
			return connectionsGet(ctx, client, in)
		case "pause":
			return connectionsPause(ctx, client, in)
		case "unpause":
			return connectionsUnpause(ctx, client, in)
		default:
			return ErrorResult(fmt.Sprintf("unknown action %q; expected list, get, pause, or unpause", action)), nil
		}
	}
}

func connectionsList(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	params := make(map[string]string)
	setIfNonEmpty(params, "name", in.String("name"))
	setIfNonEmpty(params, "source_id", in.String("source_id"))
	setIfNonEmpty(params, "destination_id", in.String("destination_id"))
	setInt(params, "limit", in.Int("limit", 0))
	setIfNonEmpty(params, "next", in.String("next"))
	setIfNonEmpty(params, "prev", in.String("prev"))

	if bp := in.BoolPtr("disabled"); bp != nil {
		if *bp {
			params["disabled_at[any]"] = "true"
		}
	}

	result, err := client.ListConnections(ctx, params)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResult(result)
}

func connectionsGet(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return ErrorResult("id is required for the get action"), nil
	}
	conn, err := client.GetConnection(ctx, id)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResult(conn)
}

func connectionsPause(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return ErrorResult("id is required for the pause action"), nil
	}
	conn, err := client.PauseConnection(ctx, id)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResult(conn)
}

func connectionsUnpause(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return ErrorResult("id is required for the unpause action"), nil
	}
	conn, err := client.UnpauseConnection(ctx, id)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResult(conn)
}
