package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func handleDestinations(client *hookdeck.Client) mcpsdk.ToolHandler {
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
			return destinationsList(ctx, client, in)
		case "get":
			return destinationsGet(ctx, client, in)
		default:
			return ErrorResult(fmt.Sprintf("unknown action %q; expected list or get", action)), nil
		}
	}
}

func destinationsList(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	params := make(map[string]string)
	setIfNonEmpty(params, "name", in.String("name"))
	setInt(params, "limit", in.Int("limit", 0))
	setIfNonEmpty(params, "next", in.String("next"))
	setIfNonEmpty(params, "prev", in.String("prev"))

	result, err := client.ListDestinations(ctx, params)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResult(result)
}

func destinationsGet(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return ErrorResult("id is required for the get action"), nil
	}
	dest, err := client.GetDestination(ctx, id, nil)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResult(dest)
}
