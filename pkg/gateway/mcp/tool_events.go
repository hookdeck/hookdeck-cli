package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

func handleEvents(client *hookdeck.Client) mcpsdk.ToolHandler {
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
			return eventsList(ctx, client, in)
		case "get":
			return eventsGet(ctx, client, in)
		case "raw_body":
			return eventsRawBody(ctx, client, in)
		default:
			return mcpcore.ErrorResult(fmt.Sprintf("unknown action %q; expected list, get, or raw_body", action)), nil
		}
	}
}

func eventsList(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	params := make(map[string]string)
	mcpcore.SetIfNonEmpty(params, "id", in.String("id"))
	// connection_id maps to webhook_id in the API
	mcpcore.SetIfNonEmpty(params, "webhook_id", in.String("connection_id"))
	mcpcore.SetIfNonEmpty(params, "source_id", in.String("source_id"))
	mcpcore.SetIfNonEmpty(params, "destination_id", in.String("destination_id"))
	mcpcore.SetIfNonEmpty(params, "status", in.String("status"))
	mcpcore.SetIfNonEmpty(params, "attempts", in.String("attempts"))
	mcpcore.SetIfNonEmpty(params, "issue_id", in.String("issue_id"))
	mcpcore.SetIfNonEmpty(params, "error_code", in.String("error_code"))
	mcpcore.SetIfNonEmpty(params, "response_status", in.String("response_status"))
	mcpcore.SetIfNonEmpty(params, "cli_id", in.String("cli_id"))
	mcpcore.SetIfNonEmpty(params, "created_at[gte]", in.String("created_after"))
	mcpcore.SetIfNonEmpty(params, "created_at[lte]", in.String("created_before"))
	mcpcore.SetIfNonEmpty(params, "successful_at[gte]", in.String("successful_after"))
	mcpcore.SetIfNonEmpty(params, "successful_at[lte]", in.String("successful_before"))
	mcpcore.SetIfNonEmpty(params, "last_attempt_at[gte]", in.String("last_attempt_after"))
	mcpcore.SetIfNonEmpty(params, "last_attempt_at[lte]", in.String("last_attempt_before"))
	mcpcore.SetInt(params, "limit", in.Int("limit", 0))
	mcpcore.SetIfNonEmpty(params, "order_by", in.String("order_by"))
	mcpcore.SetIfNonEmpty(params, "dir", in.String("dir"))
	mcpcore.SetIfNonEmpty(params, "next", in.String("next"))
	mcpcore.SetIfNonEmpty(params, "prev", in.String("prev"))
	if err := mcpcore.SetPayloadSearchFilters(params, in); err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}

	result, err := client.ListEvents(ctx, params)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(result, client)
}

func eventsGet(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return mcpcore.ErrorResult("id is required for the get action"), nil
	}
	event, err := client.GetEvent(ctx, id, nil)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(event, client)
}

func eventsRawBody(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return mcpcore.ErrorResult("id is required for the raw_body action"), nil
	}
	body, err := client.GetEventRawBody(ctx, id)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	text := string(body)
	if len(body) > maxRawBodyBytes {
		text = string(body[:maxRawBodyBytes]) + "\n... [truncated]"
	}
	return mcpcore.JSONResultEnvelopeForClient(map[string]string{"raw_body": text}, client)
}
