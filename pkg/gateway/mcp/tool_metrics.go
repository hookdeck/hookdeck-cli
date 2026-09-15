package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func handleMetrics(client *hookdeck.Client) mcpsdk.ToolHandler {
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
		case "events":
			return metricsEvents(ctx, client, in)
		case "requests":
			return metricsRequests(ctx, client, in)
		case "attempts":
			return metricsAttempts(ctx, client, in)
		case "transformations":
			return metricsTransformations(ctx, client, in)
		default:
			return ErrorResult(fmt.Sprintf("unknown action %q; expected events, requests, attempts, or transformations", action)), nil
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
func mapDimensions(dimensions []string) []string {
	if len(dimensions) == 0 {
		return dimensions
	}
	out := make([]string, 0, len(dimensions))
	for _, d := range dimensions {
		if d == "connection_id" {
			d = "webhook_id"
		}
		out = append(out, d)
	}
	return out
}

func buildMetricsParams(in input) (hookdeck.MetricsQueryParams, error) {
	start := in.String("start")
	end := in.String("end")
	if start == "" || end == "" {
		return hookdeck.MetricsQueryParams{}, fmt.Errorf("start and end are required (ISO 8601 datetime)")
	}
	measures := in.StringSlice("measures")
	if len(measures) == 0 {
		return hookdeck.MetricsQueryParams{}, fmt.Errorf("measures is required (e.g. [\"count\"], [\"successful_count\", \"failed_count\"])")
	}

	return hookdeck.MetricsQueryParams{
		Start:         start,
		End:           end,
		Granularity:   in.String("granularity"),
		Measures:      measures,
		Dimensions:    mapDimensions(in.StringSlice("dimensions")),
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

func metricsEvents(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	params, err := buildMetricsParams(in)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	// Only one endpoint is called, so parts of the query naming different ones
	// cannot all be answered. The routing below is ordered - measures, then the
	// issue_id dimension, then the issue filter - and first match wins, so a
	// queue-depth measure silently shadowed a per-issue question (#407) rather
	// than answering it. Shared with the CLI so the two cannot drift; this call
	// subsumes RejectMixedMeasureRoutes and must not be paired with it.
	if err := hookdeck.RejectCrossRouteEventQuery(params, "measures", "dimensions", hookdeck.MCPFilterNames); err != nil {
		return ErrorResult(err.Error()), nil
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
			return ErrorResult(err.Error()), nil
		}
		if err := rejectDimensions(params, hookdeck.QueueDepthRouteDimensions, hookdeck.EventRouteQueueDepth); err != nil {
			return ErrorResult(err.Error()), nil
		}
		// The endpoint accepts max_depth and max_age only; "queue_depth" is our
		// own spelling for the route, so translate it as the CLI does.
		queueParams := params
		queueParams.Measures = hookdeck.TranslateQueueDepthMeasures(params.Measures)
		result, err = client.QueryQueueDepth(ctx, queueParams)
	case measureRoute == hookdeck.EventRoutePending:
		if err := rejectFilters(params, hookdeck.PendingTimeseriesRouteFilters, hookdeck.EventRoutePending); err != nil {
			return ErrorResult(err.Error()), nil
		}
		if err := rejectDimensions(params, hookdeck.PendingTimeseriesRouteDimensions, hookdeck.EventRoutePending); err != nil {
			return ErrorResult(err.Error()), nil
		}
		// The API expects measures[]=count here; "pending" only selects the
		// route. Without this the request carries a measure the endpoint does
		// not define - the CLI has always rewritten it, MCP did not.
		pendingParams := params
		pendingParams.Measures = []string{"count"}
		result, err = client.QueryEventsPendingTimeseries(ctx, pendingParams)
	case containsAny(params.Dimensions, "issue_id") || params.IssueID != "":
		if params.IssueID == "" {
			return ErrorResult("per-issue metrics require issue_id (required when using dimensions: issue_id)"), nil
		}
		if err := rejectFilters(params, hookdeck.EventsByIssueRouteFilters, hookdeck.EventRouteByIssue); err != nil {
			return ErrorResult(err.Error()), nil
		}
		if err := rejectDimensions(params, hookdeck.EventsByIssueRouteDimensions, hookdeck.EventRouteByIssue); err != nil {
			return ErrorResult(err.Error()), nil
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
			return ErrorResult(err.Error()), nil
		}
		result, err = client.QueryEventMetrics(ctx, params)
	}

	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResultEnvelopeForClient(result, client)
}

func metricsRequests(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	params, err := buildMetricsParams(in)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	if err := rejectFilters(params, hookdeck.RequestMetricsFilters, "request metrics"); err != nil {
		return ErrorResult(err.Error()), nil
	}
	if err := rejectDimensions(params, hookdeck.RequestMetricsDimensionValues, "request metrics"); err != nil {
		return ErrorResult(err.Error()), nil
	}
	result, err := client.QueryRequestMetrics(ctx, params)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResultEnvelopeForClient(result, client)
}

func metricsAttempts(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	params, err := buildMetricsParams(in)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	if err := rejectFilters(params, hookdeck.AttemptMetricsFilters, "attempt metrics"); err != nil {
		return ErrorResult(err.Error()), nil
	}
	if err := rejectDimensions(params, hookdeck.AttemptMetricsDimensionValues, "attempt metrics"); err != nil {
		return ErrorResult(err.Error()), nil
	}
	result, err := client.QueryAttemptMetrics(ctx, params)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResultEnvelopeForClient(result, client)
}

func metricsTransformations(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	params, err := buildMetricsParams(in)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	if err := rejectFilters(params, hookdeck.TransformationMetricsFilters, "transformation metrics"); err != nil {
		return ErrorResult(err.Error()), nil
	}
	if err := rejectDimensions(params, hookdeck.TransformationMetricsDimensionValues, "transformation metrics"); err != nil {
		return ErrorResult(err.Error()), nil
	}
	result, err := client.QueryTransformationMetrics(ctx, params)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResultEnvelopeForClient(result, client)
}
