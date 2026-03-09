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

func buildMetricsParams(in input) (hookdeck.MetricsQueryParams, error) {
	start := in.String("start")
	end := in.String("end")
	if start == "" || end == "" {
		return hookdeck.MetricsQueryParams{}, fmt.Errorf("start and end are required (ISO 8601 datetime)")
	}

	return hookdeck.MetricsQueryParams{
		Start:         start,
		End:           end,
		Granularity:   in.String("granularity"),
		Measures:      in.StringSlice("measures"),
		Dimensions:    in.StringSlice("dimensions"),
		SourceID:      in.String("source_id"),
		DestinationID: in.String("destination_id"),
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

	// Route to the correct events metrics endpoint based on measures/dimensions
	var result hookdeck.MetricsResponse
	switch {
	case containsAny(params.Measures, "queue_depth", "max_depth", "max_age"):
		result, err = client.QueryQueueDepth(ctx, params)
	case containsAny(params.Measures, "pending") && params.Granularity != "":
		result, err = client.QueryEventsPendingTimeseries(ctx, params)
	case containsAny(params.Dimensions, "issue_id") || params.IssueID != "":
		result, err = client.QueryEventsByIssue(ctx, params)
	default:
		result, err = client.QueryEventMetrics(ctx, params)
	}

	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResult(result)
}

func metricsRequests(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	params, err := buildMetricsParams(in)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	result, err := client.QueryRequestMetrics(ctx, params)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResult(result)
}

func metricsAttempts(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	params, err := buildMetricsParams(in)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	result, err := client.QueryAttemptMetrics(ctx, params)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResult(result)
}

func metricsTransformations(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	params, err := buildMetricsParams(in)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	result, err := client.QueryTransformationMetrics(ctx, params)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResult(result)
}
