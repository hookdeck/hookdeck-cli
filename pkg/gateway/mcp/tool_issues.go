package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

func handleIssues(client *hookdeck.Client) mcpsdk.ToolHandler {
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
			return issuesList(ctx, client, in)
		case "get":
			return issuesGet(ctx, client, in)
		default:
			return mcpcore.ErrorResult(fmt.Sprintf("unknown action %q; expected list or get", action)), nil
		}
	}
}

func issuesList(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	params := make(map[string]string)
	mcpcore.SetIfNonEmpty(params, "type", in.String("type"))
	mcpcore.SetIfNonEmpty(params, "status", in.String("filter_status"))
	mcpcore.SetIfNonEmpty(params, "issue_trigger_id", in.String("issue_trigger_id"))
	mcpcore.SetIfNonEmpty(params, "order_by", in.String("order_by"))
	mcpcore.SetIfNonEmpty(params, "dir", in.String("dir"))
	mcpcore.SetInt(params, "limit", in.Int("limit", 0))
	mcpcore.SetIfNonEmpty(params, "next", in.String("next"))
	mcpcore.SetIfNonEmpty(params, "prev", in.String("prev"))

	result, err := client.ListIssues(ctx, params)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(result, client)
}

func issuesGet(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return mcpcore.ErrorResult("id is required for the get action"), nil
	}
	issue, err := client.GetIssue(ctx, id)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(issue, client)
}
