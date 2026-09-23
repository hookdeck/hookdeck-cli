package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

// gateway_bulk — retry, cancel and replay applied to everything matching a
// filter rather than to one record.
//
// One tool pair covers all five families because the API gives them all the
// same shape. `operation` says which family; `action` says what you are doing
// to a job. Those are the API's own words at the API's own two levels, so
// `{action:"create", operation:"events_cancel"}` starts a bulk cancellation
// and `{action:"cancel", id:...}` stops a running job, without inventing
// vocabulary that would not appear in the Hookdeck docs.
var bulkActions = mcpcore.ActionSet{
	{Name: "list", Desc: "list this operation's bulk jobs, most recent first"},
	{Name: "get", Desc: "get one bulk job by ID, including its progress"},
	{Name: "plan", Desc: "estimate how many records a bulk operation would touch, WITHOUT running it. Call this before create — it is the only way to size the blast radius, and it needs no write access"},

	{Name: "create", Desc: "start a bulk operation across everything the query matches. Call plan first", Write: true, Destructive: true},
	{Name: "cancel", Desc: "stop a pending or in-progress bulk job", Write: true},
}

var bulkSpec = mcpcore.ToolSpec{
	Resource: "bulk",
	Summary: "Run an operation across every record matching a filter, rather than one at a time. " +
		"Five operations: events_retry, events_cancel, ignored_events_retry, requests_retry, requests_replay. " +
		"Every one of them can be estimated first with action plan, which runs nothing. " +
		"Results are scoped to the active project — call the projects tool first if the user has specified a project.",
	Actions:  bulkActions,
	Required: []string{"operation"},
	Props: map[string]mcpcore.Prop{
		"operation": {Type: "string", Desc: "Which bulk operation: " + strings.Join(hookdeck.BulkFamilies(), ", ") +
			". Each accepts a different filter set — see the query description.",
			Enum: hookdeck.BulkFamilies()},
		"id": {Type: "string", Desc: "Bulk job ID (get, cancel).", Actions: []string{"get", "cancel"}},
		"query": {Type: "string", JSONValue: true, Actions: []string{"plan", "create"},
			Desc: "Filters selecting what to act on, as a JSON object. " +
				"THE FILTERS DIFFER PER OPERATION: the event operations take the full event filter set (status, connection_id, source_id, created_at, body, …); " +
				"the request operations take the request filter set; ignored_events_retry takes only cause, connection_id and transformation_id. " +
				"A filter the operation does not declare is refused here rather than sent, because the API would ignore it and run across everything the rest matched. " +
				"requests_replay also takes target, which selects where to replay: a bare source_id there replays onto EVERY active connection on that source, resolved as the replay runs."},
		"limit": {Type: "integer", Desc: "Max results (list)", Actions: []string{"list"}},
		"next":  {Type: "string", Desc: "Next page cursor (list)", Actions: []string{"list"}},
		"prev":  {Type: "string", Desc: "Previous page cursor (list)", Actions: []string{"list"}},
	},
	Notes: `Retry is not replay:
  retry re-delivers records that already exist.
  replay re-ingests the original request through the whole pipeline, creating a NEW request and
  NEW events, re-evaluated against the CURRENT configuration — transformations, filters and rules
  as they are now, not as they were. The originals are untouched. It can also target connections
  the original never went to.

Plan before you create:
  {"action":"plan","operation":"events_retry","query":{"status":"FAILED"}} returns estimated_count
  and touches nothing. There is no undo for a bulk operation that was larger than intended, and
  most of them cannot be stopped fast enough to matter.

Stopping one:
  {"action":"cancel","operation":"events_retry","id":"bar_123"} stops a pending or in-progress job.
  events_cancel is the exception: the API gives it no cancel route, so once it starts it runs to
  completion.`,
	Handler: handleBulk,
}

func handleBulk(srv *mcpcore.Server) mcpsdk.ToolHandler {
	client := srv.Client()
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if r := srv.RequireAuth(); r != nil {
			return r, nil
		}

		in, err := mcpcore.ParseInput(req.Params.Arguments)
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}

		action, blocked := mcpcore.Dispatch(srv, bulkActions, in.String("action"))
		if blocked != nil {
			return blocked, nil
		}

		family := in.String("operation")
		if family == "" {
			return mcpcore.ErrorResult("operation is required: " + strings.Join(hookdeck.BulkFamilies(), ", ")), nil
		}
		if _, ok := hookdeck.BulkFilters[family]; !ok {
			return mcpcore.ErrorResult(fmt.Sprintf("unknown operation %q; expected one of: %s",
				family, strings.Join(hookdeck.BulkFamilies(), ", "))), nil
		}

		switch action {
		case "list":
			return bulkList(ctx, client, in, family)
		case "get":
			return bulkGet(ctx, client, in, family)
		case "plan":
			return bulkPlan(ctx, client, in, family)
		case "create":
			return bulkCreate(ctx, client, in, family)
		default:
			return bulkCancel(ctx, client, in, family)
		}
	}
}

// bulkQuery reads and validates the query object against the family's declared
// filters, before anything is sent.
func bulkQuery(in mcpcore.Input, family string) (map[string]interface{}, *mcpsdk.CallToolResult) {
	params := map[string]string{}
	if err := mcpcore.SetJSONFilter(params, "query", in); err != nil {
		return nil, mcpcore.ErrorResult(err.Error())
	}
	query := map[string]interface{}{}
	if raw, ok := params["query"]; ok && raw != "" {
		if err := json.Unmarshal([]byte(raw), &query); err != nil {
			return nil, mcpcore.ErrorResult("query must be a JSON object: " + err.Error())
		}
	}
	// connection_id is what every other tool takes; webhook_id is what the API
	// calls it. Canonicalise before validating, or the refusal names a filter
	// the caller never typed.
	query = hookdeck.CanonicalBulkQuery(query)
	if err := hookdeck.RejectUnsupportedBulkFilters(family, query); err != nil {
		return nil, mcpcore.ErrorResult(err.Error())
	}
	return query, nil
}

func bulkPlan(ctx context.Context, client *hookdeck.Client, in mcpcore.Input, family string) (*mcpsdk.CallToolResult, error) {
	query, refused := bulkQuery(in, family)
	if refused != nil {
		return refused, nil
	}
	plan, err := client.PlanBulk(ctx, family, query)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(plan, client)
}

func bulkCreate(ctx context.Context, client *hookdeck.Client, in mcpcore.Input, family string) (*mcpsdk.CallToolResult, error) {
	query, refused := bulkQuery(in, family)
	if refused != nil {
		return refused, nil
	}
	job, err := client.CreateBulk(ctx, family, query)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(job, client)
}

func bulkList(ctx context.Context, client *hookdeck.Client, in mcpcore.Input, family string) (*mcpsdk.CallToolResult, error) {
	params := map[string]string{}
	mcpcore.SetInt(params, "limit", in.Int("limit", 0))
	mcpcore.SetIfNonEmpty(params, "next", in.String("next"))
	mcpcore.SetIfNonEmpty(params, "prev", in.String("prev"))

	jobs, err := client.ListBulk(ctx, family, params)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(map[string]any{"models": jobs}, client)
}

func bulkGet(ctx context.Context, client *hookdeck.Client, in mcpcore.Input, family string) (*mcpsdk.CallToolResult, error) {
	id, err := mcpcore.RequireString(in, "id", "get")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	job, apiErr := client.GetBulk(ctx, family, id)
	if apiErr != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(apiErr)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(job, client)
}

func bulkCancel(ctx context.Context, client *hookdeck.Client, in mcpcore.Input, family string) (*mcpsdk.CallToolResult, error) {
	if !hookdeck.BulkCancellable(family) {
		return mcpcore.ErrorResult(fmt.Sprintf(
			"a %s operation cannot be cancelled once started: the API gives it no cancel route, "+
				"so it runs to completion. The other bulk operations (%s) can be cancelled.",
			family, strings.Join(cancellableFamilies(), ", "))), nil
	}
	id, err := mcpcore.RequireString(in, "id", "cancel")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	job, apiErr := client.CancelBulk(ctx, family, id)
	if apiErr != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(apiErr)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(job, client)
}

func cancellableFamilies() []string {
	out := []string{}
	for _, f := range hookdeck.BulkFamilies() {
		if hookdeck.BulkCancellable(f) {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}
