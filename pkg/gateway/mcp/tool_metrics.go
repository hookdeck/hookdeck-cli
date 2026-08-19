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
	{Name: "events", Desc: "aggregated event metrics"},
	{Name: "requests", Desc: "aggregated inbound request metrics"},
	{Name: "attempts", Desc: "aggregated delivery attempt metrics"},
	{Name: "transformations", Desc: "aggregated transformation execution metrics"},
}

var metricsSpec = mcpcore.ToolSpec{
	Resource: "metrics",
	Summary:  "Query aggregate metrics over a time range: counts, failure rates, error rates, queue depth and pending event data. Supports grouping by dimensions such as source, destination or connection. Results are scoped to the active project — call the projects tool first if the user has specified a project.",
	Actions:  metricsActions,
	Props: map[string]mcpcore.Prop{
		"start":          {Type: "string", Desc: "Start datetime (ISO 8601, required)"},
		"end":            {Type: "string", Desc: "End datetime (ISO 8601, required)"},
		"granularity":    {Type: "string", Desc: "Time bucket size, e.g. 1h, 5m, 1d"},
		"measures":       {Type: "array", Desc: "Metrics to retrieve (required). Common: count, successful_count, failed_count, error_count", Items: &mcpcore.Prop{Type: "string"}},
		"dimensions":     {Type: "array", Desc: "Grouping dimensions", Items: &mcpcore.Prop{Type: "string"}},
		"source_id":      {Type: "string", Desc: "Filter by source"},
		"destination_id": {Type: "string", Desc: "Filter by destination"},
		"connection_id":  {Type: "string", Desc: "Filter by connection (maps to webhook_id)"},
		"status":         {Type: "string", Desc: "Filter by status"},
		"issue_id":       {Type: "string", Desc: "Filter by issue (events only)"},
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

func buildMetricsParams(in mcpcore.Input) (hookdeck.MetricsQueryParams, error) {
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

func metricsEvents(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	params, err := buildMetricsParams(in)
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
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
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(result, client)
}

func metricsRequests(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	params, err := buildMetricsParams(in)
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	result, err := client.QueryRequestMetrics(ctx, params)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(result, client)
}

func metricsAttempts(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	params, err := buildMetricsParams(in)
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	result, err := client.QueryAttemptMetrics(ctx, params)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(result, client)
}

func metricsTransformations(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	params, err := buildMetricsParams(in)
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	result, err := client.QueryTransformationMetrics(ctx, params)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(result, client)
}
