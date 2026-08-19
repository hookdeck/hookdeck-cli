package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

var metricsActions = mcpcore.ActionSet{
	{Name: "events", Desc: "aggregated publish metrics"},
	{Name: "attempts", Desc: "aggregated delivery metrics"},
}

var metricsSpec = mcpcore.ToolSpec{
	Resource: "metrics",
	Summary: "Query aggregate metrics over a time range. " +
		"Event measures: count, rate; dimensions: tenant_id, topic, destination_id. " +
		"Attempt measures: count, successful_count, failed_count, error_rate, first_attempt_count, retry_count, manual_retry_count, avg_attempt_number, rate, successful_rate, failed_rate; dimensions: tenant_id, destination_id, destination_type, topic, status, code, manual, attempt_number. " +
		"Omit granularity for a single total over the whole range.",
	Actions:  metricsActions,
	Required: []string{"start", "end", "measures"},
	Props: map[string]mcpcore.Prop{
		"start":       {Type: "string", Desc: "Start of the range (ISO 8601 datetime, required)."},
		"end":         {Type: "string", Desc: "End of the range (ISO 8601 datetime, required)."},
		"granularity": {Type: "string", Desc: "Time bucket size, e.g. 1h, 5m, 1d. Omit for one total over the whole range."},
		"measures":    {Type: "array", Desc: "Measures to return (required). See the tool description for the measures each action supports.", Items: &mcpcore.Prop{Type: "string"}},
		"dimensions":  {Type: "array", Desc: "Dimensions to group by.", Items: &mcpcore.Prop{Type: "string"}},
		"filters":     {Type: "object", Desc: `Filter by dimension, e.g. {"topic": "user.created"} or {"status": ["failed"]}.`},
	},
	Handler: handleMetrics,
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

		params, err := metricsParams(in)
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}

		var result *hookdeck.OutpostMetricsResponse
		if action == "events" {
			result, err = client.GetOutpostEventMetrics(ctx, params)
		} else {
			result, err = client.GetOutpostAttemptMetrics(ctx, params)
		}
		if err != nil {
			return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
		}
		return mcpcore.JSONResultEnvelopeForClient(result, client)
	}
}

func metricsParams(in mcpcore.Input) (hookdeck.OutpostMetricsParams, error) {
	start := in.String("start")
	end := in.String("end")
	if start == "" || end == "" {
		return hookdeck.OutpostMetricsParams{}, fmt.Errorf("start and end are required (ISO 8601 datetimes)")
	}
	measures := mcpcore.StringList(in, "measures")
	if len(measures) == 0 {
		return hookdeck.OutpostMetricsParams{}, fmt.Errorf(`measures is required, e.g. ["count"]`)
	}

	filters, err := metricsFilters(in)
	if err != nil {
		return hookdeck.OutpostMetricsParams{}, err
	}

	return hookdeck.OutpostMetricsParams{
		Start:       start,
		End:         end,
		Granularity: in.String("granularity"),
		Measures:    measures,
		Dimensions:  mcpcore.StringList(in, "dimensions"),
		Filters:     filters,
	}, nil
}

// metricsFilters reads the filters object, accepting a single value or an array
// per dimension.
func metricsFilters(in mcpcore.Input) (map[string][]string, error) {
	raw, err := mcpcore.Object(in, "filters")
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}

	filters := make(map[string][]string, len(raw))
	for dimension, value := range raw {
		switch v := value.(type) {
		case string:
			filters[dimension] = []string{v}
		case []interface{}:
			for _, item := range v {
				s, ok := item.(string)
				if !ok {
					return nil, fmt.Errorf("filters.%s must contain only strings", dimension)
				}
				filters[dimension] = append(filters[dimension], s)
			}
		default:
			return nil, fmt.Errorf("filters.%s must be a string or an array of strings", dimension)
		}
	}
	return filters, nil
}
