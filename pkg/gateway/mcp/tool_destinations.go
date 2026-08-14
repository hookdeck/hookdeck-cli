package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

func handleDestinations(client *hookdeck.Client) mcpsdk.ToolHandler {
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
			return destinationsList(ctx, client, in)
		case "get":
			return destinationsGet(ctx, client, in)
		default:
			return mcpcore.ErrorResult(fmt.Sprintf("unknown action %q; expected list or get", action)), nil
		}
	}
}

func destinationsList(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	params := make(map[string]string)
	mcpcore.SetIfNonEmpty(params, "name", in.String("name"))
	mcpcore.SetInt(params, "limit", in.Int("limit", 0))
	mcpcore.SetIfNonEmpty(params, "next", in.String("next"))
	mcpcore.SetIfNonEmpty(params, "prev", in.String("prev"))

	result, err := client.ListDestinations(ctx, params)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(result, client)
}

func destinationsGet(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return mcpcore.ErrorResult("id is required for the get action"), nil
	}
	dest, err := client.GetDestination(ctx, id, nil)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(dest, client)
}
