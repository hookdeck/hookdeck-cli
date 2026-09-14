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

	// Route to the correct events metrics endpoint based on measures/dimensions.
	// Each route accepts a different set of filters, so the ones it would ignore
	// are refused here rather than silently dropped by the API.
	var result hookdeck.MetricsResponse
	switch {
	case containsAny(params.Measures, "queue_depth", "max_depth", "max_age"):
		if err := rejectFilters(params, hookdeck.QueueDepthRouteFilters, "queue depth metrics"); err != nil {
			return ErrorResult(err.Error()), nil
		}
		result, err = client.QueryQueueDepth(ctx, params)
	case containsAny(params.Measures, "pending") && params.Granularity != "":
		if err := rejectFilters(params, hookdeck.PendingTimeseriesRouteFilters, "pending event metrics (measures: pending)"); err != nil {
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
		if err := rejectFilters(params, hookdeck.EventsByIssueRouteFilters, "per-issue event metrics"); err != nil {
			return ErrorResult(err.Error()), nil
		}
		result, err = client.QueryEventsByIssue(ctx, params)
	default:
		if err := rejectFilters(params, hookdeck.DefaultEventRouteFilters, "event metrics"); err != nil {
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
	result, err := client.QueryTransformationMetrics(ctx, params)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResultEnvelopeForClient(result, client)
}
