package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func handleIssues(client *hookdeck.Client) mcpsdk.ToolHandler {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		in, err := parseInput(req.Params.Arguments)
		if err != nil {
			return ErrorResult(err.Error()), nil
		}

		action := in.String("action")
		switch action {
		case "list", "":
			return issuesList(ctx, client, in)
		case "get":
			return issuesGet(ctx, client, in)
		case "update":
			return issuesUpdate(ctx, client, in)
		case "dismiss":
			return issuesDismiss(ctx, client, in)
		default:
			return ErrorResult(fmt.Sprintf("unknown action %q; expected list, get, update, or dismiss", action)), nil
		}
	}
}

func issuesList(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	params := make(map[string]string)
	setIfNonEmpty(params, "type", in.String("type"))
	setIfNonEmpty(params, "status", in.String("filter_status"))
	setIfNonEmpty(params, "issue_trigger_id", in.String("issue_trigger_id"))
	setIfNonEmpty(params, "order_by", in.String("order_by"))
	setIfNonEmpty(params, "dir", in.String("dir"))
	setInt(params, "limit", in.Int("limit", 0))
	setIfNonEmpty(params, "next", in.String("next"))
	setIfNonEmpty(params, "prev", in.String("prev"))

	result, err := client.ListIssues(ctx, params)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResult(result)
}

func issuesGet(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return ErrorResult("id is required for the get action"), nil
	}
	issue, err := client.GetIssue(ctx, id)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResult(issue)
}

func issuesUpdate(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return ErrorResult("id is required for the update action"), nil
	}
	status := in.String("status")
	if status == "" {
		return ErrorResult("status is required for the update action (OPENED, IGNORED, ACKNOWLEDGED, RESOLVED)"), nil
	}
	issue, err := client.UpdateIssue(ctx, id, &hookdeck.IssueUpdateRequest{
		Status: hookdeck.IssueStatus(status),
	})
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResult(issue)
}

func issuesDismiss(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return ErrorResult("id is required for the dismiss action"), nil
	}
	_, err := client.DismissIssue(ctx, id)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResult(map[string]string{
		"status":   "ok",
		"action":   "dismiss",
		"issue_id": id,
	})
}
