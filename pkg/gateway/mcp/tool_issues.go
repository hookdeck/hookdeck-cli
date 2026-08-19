package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

var issuesActions = mcpcore.ActionSet{
	{Name: "list", Desc: "list issues"},
	{Name: "get", Desc: "get one issue"},
	{Name: "update", Desc: "set an issue's status", Write: true},
	{Name: "dismiss", Desc: "dismiss an issue, closing it without resolving the cause", Write: true, Destructive: true},
}

var issuesSpec = mcpcore.ToolSpec{
	Resource: "issues",
	Summary:  "Inspect and triage Hookdeck issues — aggregated failure signals such as repeated delivery failures, transformation errors, and backpressure alerts. Use this to identify systemic problems across your event pipeline. Results are scoped to the active project — call the projects tool first if the user has specified a project.",
	Actions:  issuesActions,
	Props: map[string]mcpcore.Prop{
		"id":               {Type: "string", Desc: "Issue ID. Required for get/update/dismiss."},
		"status":           {Type: "string", Desc: "New status for update: OPENED, IGNORED, ACKNOWLEDGED or RESOLVED", Enum: []string{"OPENED", "IGNORED", "ACKNOWLEDGED", "RESOLVED"}},
		"type":             {Type: "string", Desc: "Filter: delivery, transformation, or backpressure (list)"},
		"filter_status":    {Type: "string", Desc: "Filter by status (list)"},
		"issue_trigger_id": {Type: "string", Desc: "Filter by trigger (list)"},
		"order_by":         {Type: "string", Desc: "Sort field (list)"},
		"dir":              {Type: "string", Desc: "Sort direction: asc or desc (list)"},
		"limit":            {Type: "integer", Desc: "Max results (list)"},
		"next":             {Type: "string", Desc: "Next page cursor"},
		"prev":             {Type: "string", Desc: "Previous page cursor"},
	},
	Handler: handleIssues,
}

func handleIssues(srv *mcpcore.Server) mcpsdk.ToolHandler {
	client := srv.Client()
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if r := srv.RequireAuth(); r != nil {
			return r, nil
		}

		in, err := mcpcore.ParseInput(req.Params.Arguments)
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}

		action, blocked := mcpcore.DispatchWithDefault(srv, issuesActions, in.String("action"), "list")
		if blocked != nil {
			return blocked, nil
		}

		switch action {
		case "list":
			return issuesList(ctx, client, in)
		case "get":
			return issuesGet(ctx, client, in)
		case "update":
			return issuesUpdate(ctx, client, in)
		default:
			return issuesDismiss(ctx, client, in)
		}
	}
}

func issuesUpdate(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id, err := mcpcore.RequireString(in, "id", "update")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	status, err := mcpcore.RequireString(in, "status", "update")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	issue, err := client.UpdateIssue(ctx, id, &hookdeck.IssueUpdateRequest{
		Status: hookdeck.IssueStatus(status),
	})
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(issue, client)
}

func issuesDismiss(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id, err := mcpcore.RequireString(in, "id", "dismiss")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	issue, err := client.DismissIssue(ctx, id)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(issue, client)
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
