package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

// metrics is read-only: every action is an aggregate query. The action here
// names the metric family rather than a verb.
var metricsActions = mcpcore.ActionSet{
	{Name: "events", Desc: "event metrics — how many events there were, and what became of them. \"How many were delivered\" is usually this one: an event is the logical delivery, and its status says whether it succeeded"},
	{Name: "requests", Desc: "aggregated inbound request metrics"},
	{Name: "attempts", Desc: "attempt metrics — the individual HTTP calls, including retries. One event can produce several attempts, so these counts run higher than the event ones and answer \"how hard did we try\" rather than \"how many got through\""},
	{Name: "transformations", Desc: "aggregated transformation execution metrics"},
}

var metricsSpec = mcpcore.ToolSpec{
	Resource: "metrics",
	Summary:  "Query aggregate metrics over a time range: counts, failure rates, error rates, queue depth and pending event data. Supports grouping by dimensions such as source, destination or connection. Filters apply only to the actions named in each argument: passing one elsewhere is rejected, because the API would ignore it and return unfiltered totals. Results are scoped to the active project — call the projects tool first if the user has specified a project.",
	Actions:  metricsActions,
	Props: map[string]mcpcore.Prop{
		"start":          {Type: "string", Desc: "Start datetime (ISO 8601, required)"},
		"end":            {Type: "string", Desc: "End datetime (ISO 8601, required)"},
		"granularity":    {Type: "string", Desc: "Time bucket size, e.g. 1h, 5m, 1d"},
		"measures":       {Type: "array", Desc: descMetricsMeasures, Items: &mcpcore.Prop{Type: "string"}},
		"dimensions":     {Type: "array", Desc: descMetricsDimensions, Items: &mcpcore.Prop{Type: "string"}},
		"source_id":      {Type: "string", Desc: "Filter by source (events, requests)"},
		"destination_id": {Type: "string", Desc: "Filter by destination (events, attempts)"},
		"delivery_group": {Type: "string", Desc: "Filter by delivery group (events, attempts)"},
		"connection_id":  {Type: "string", Desc: "Filter by connection, maps to webhook_id (events, transformations)"},
		"status":         {Type: "string", Desc: descMetricsStatus},
		"issue_id":       {Type: "string", Desc: "Filter by issue (transformations; events when grouping by issue_id)"},
	},
	Required: []string{"start", "end", "measures"},
	Handler:  handleMetrics,
}

func handleMetrics(srv *mcpcore.Server) mcpsdk.ToolHandler {
	client := srv.Client()
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if r := srv.RequireAuth(); r != nil {
			return r, nil
		}

		in, err := mcpcore.ParseInput(req.Params.Arguments)
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}

		action, blocked := mcpcore.Dispatch(srv, metricsActions, in.String("action"))
		if blocked != nil {
			return blocked, nil
		}

		switch action {
		case "events":
			return metricsEvents(ctx, client, in)
		case "requests":
			return metricsRequests(ctx, client, in)
		case "attempts":
			return metricsAttempts(ctx, client, in)
		default:
			return metricsTransformations(ctx, client, in)
		}
	}
}

// rejectFilters names filters the way an MCP client passes them.
func rejectFilters(params hookdeck.MetricsQueryParams, allowed hookdeck.MetricsFilters, route string) error {
	return hookdeck.RejectUnsupportedFilters(params, allowed, route, hookdeck.MCPFilterNames)
}

// rejectDimensions is the dimension counterpart. Filters were gated per route
// and dimensions were not, so a dimension the route does not define reached the
// API as a raw 422 - including the delivery_group grouping this release is
// about. Both layers read the same matrix so they cannot drift.
func rejectDimensions(params hookdeck.MetricsQueryParams, allowed []string, route string) error {
	return hookdeck.RejectUnsupportedDimensions(params, allowed, route, hookdeck.MCPFilterNames, "dimensions")
}

// mapDimensions rewrites connection_id to the webhook_id the API expects. The
// tool schema tells callers connection_id "maps to webhook_id", which was true
// of the filter and not of the dimension.
// mapDimensions delegates to the shared pair in pkg/hookdeck so the outbound
// rewrite and the inbound one in RestoreDimensionNames cannot drift. See #442.
func mapDimensions(dimensions []string) []string {
	return hookdeck.MapDimensionsToAPI(dimensions)
}

func buildMetricsParams(in mcpcore.Input) (hookdeck.MetricsQueryParams, error) {
	start := in.String("start")
	end := in.String("end")
	if start == "" || end == "" {
		return hookdeck.MetricsQueryParams{}, fmt.Errorf("start and end are required (ISO 8601 datetime)")
	}
	// StringList, not StringSlice: every other tool on this surface accepts a
	// comma-separated string wherever it declares an array, because models
	// routinely send one. StringSlice returns nil for anything that is not a
	// JSON array, so "count" was dropped and the caller was told the argument
	// was missing for an argument they had just supplied.
	//
	// This fix landed in 7dae336 and was reverted by the b0b5710 merge, then
	// shipped broken in v3.0.0 and v3.0.1. See #440. The tests below cover the
	// string form specifically -- the pre-existing tests all pass an array, so
	// they pass against the reverted code too and did not catch it.
	measures := mcpcore.StringList(in, "measures")
	if len(measures) == 0 {
		return hookdeck.MetricsQueryParams{}, fmt.Errorf("measures is required (e.g. [\"count\"], [\"successful_count\", \"failed_count\"])")
	}

	return hookdeck.MetricsQueryParams{
		Start:         start,
		End:           end,
		Granularity:   in.String("granularity"),
		Measures:      measures,
		Dimensions:    mapDimensions(mcpcore.StringList(in, "dimensions")),
		SourceID:      in.String("source_id"),
		DestinationID: in.String("destination_id"),
		DeliveryGroup: in.String("delivery_group"),
		ConnectionID:  in.String("connection_id"),
		Status:        in.String("status"),
		IssueID:       in.String("issue_id"),
	}, nil
}

// containsAny reports whether any of the needles appear in the haystack.
func containsAny(haystack []string, needles ...string) bool {
	for _, h := range haystack {
		for _, n := range needles {
			if h == n {
				return true
			}
		}
	}
	return false
}

func metricsEvents(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	params, err := buildMetricsParams(in)
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}

	// Only one endpoint is called, so parts of the query naming different ones
	// cannot all be answered. The routing below is ordered - measures, then the
	// issue_id dimension, then the issue filter - and first match wins, so a
	// queue-depth measure silently shadowed a per-issue question (#407) rather
	// than answering it. Shared with the CLI so the two cannot drift; this call
	// subsumes RejectMixedMeasureRoutes and must not be paired with it.
	if err := hookdeck.RejectCrossRouteEventQuery(params, "measures", "dimensions", hookdeck.MCPFilterNames); err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}

	// Route to the correct events metrics endpoint based on measures/dimensions.
	// Each route accepts a different set of filters, so the ones it would ignore
	// are refused here rather than silently dropped by the API.
	// Which measures belong to which endpoint is the shared table's to know, and
	// the route names are its constants, so this cannot drift from the refusal
	// above or from the CLI's copy of the same switch.
	measureRoute := hookdeck.RouteForMeasures(params.Measures)

	var result hookdeck.MetricsResponse
	switch {
	case measureRoute == hookdeck.EventRouteQueueDepth:
		if err := rejectFilters(params, hookdeck.QueueDepthRouteFilters, hookdeck.EventRouteQueueDepth); err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}
		if err := rejectDimensions(params, hookdeck.QueueDepthRouteDimensions, hookdeck.EventRouteQueueDepth); err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}
		// The endpoint accepts max_depth and max_age only; "queue_depth" is our
		// own spelling for the route, so translate it as the CLI does.
		queueParams := params
		queueParams.Measures = hookdeck.TranslateQueueDepthMeasures(params.Measures)
		result, err = client.QueryQueueDepth(ctx, queueParams)
	case measureRoute == hookdeck.EventRoutePending:
		if err := rejectFilters(params, hookdeck.PendingTimeseriesRouteFilters, hookdeck.EventRoutePending); err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}
		if err := rejectDimensions(params, hookdeck.PendingTimeseriesRouteDimensions, hookdeck.EventRoutePending); err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}
		// The API expects measures[]=count here; "pending" only selects the
		// route. Without this the request carries a measure the endpoint does
		// not define - the CLI has always rewritten it, MCP did not.
		pendingParams := params
		pendingParams.Measures = []string{"count"}
		result, err = client.QueryEventsPendingTimeseries(ctx, pendingParams)
	case containsAny(params.Dimensions, "issue_id") || params.IssueID != "":
		if params.IssueID == "" {
			return mcpcore.ErrorResult("per-issue metrics require issue_id (required when using dimensions: issue_id)"), nil
		}
		if err := rejectFilters(params, hookdeck.EventsByIssueRouteFilters, hookdeck.EventRouteByIssue); err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}
		if err := rejectDimensions(params, hookdeck.EventsByIssueRouteDimensions, hookdeck.EventRouteByIssue); err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}
		result, err = client.QueryEventsByIssue(ctx, params)
	default:
		// No filter gate here: the default route honours every filter the tool
		// advertises except issue_id, and a set issue_id selects the by-issue
		// route above, so nothing reaches this branch for a gate to catch. The
		// invariant is pinned by
		// hookdeck.TestDefaultEventRouteHonoursEveryFilterExceptIssueID, which
		// fails if a filter the route drops is ever added.
		if err := rejectDimensions(params, hookdeck.DefaultEventRouteDimensions, hookdeck.EventRouteDefault); err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}
		result, err = client.QueryEventMetrics(ctx, params)
	}

	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(hookdeck.RestoreDimensionNames(result), client)
}

func metricsRequests(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	params, err := buildMetricsParams(in)
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	if err := rejectFilters(params, hookdeck.RequestMetricsFilters, "request metrics"); err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	if err := rejectDimensions(params, hookdeck.RequestMetricsDimensionValues, "request metrics"); err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	result, err := client.QueryRequestMetrics(ctx, params)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(hookdeck.RestoreDimensionNames(result), client)
}

func metricsAttempts(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	params, err := buildMetricsParams(in)
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	if err := rejectFilters(params, hookdeck.AttemptMetricsFilters, "attempt metrics"); err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	if err := rejectDimensions(params, hookdeck.AttemptMetricsDimensionValues, "attempt metrics"); err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	result, err := client.QueryAttemptMetrics(ctx, params)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(hookdeck.RestoreDimensionNames(result), client)
}

func metricsTransformations(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	params, err := buildMetricsParams(in)
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	if err := rejectFilters(params, hookdeck.TransformationMetricsFilters, "transformation metrics"); err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	if err := rejectDimensions(params, hookdeck.TransformationMetricsDimensionValues, "transformation metrics"); err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	result, err := client.QueryTransformationMetrics(ctx, params)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(hookdeck.RestoreDimensionNames(result), client)
}
