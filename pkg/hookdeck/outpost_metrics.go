package hookdeck

import (
	"context"
	"fmt"
	"time"
)

// OutpostMetricsDataPoint is one aggregated row of a metrics query.
//
// TimeBucket is absent when the query specified no granularity, and Dimensions
// is empty when no grouping was requested.
type OutpostMetricsDataPoint struct {
	TimeBucket *time.Time             `json:"time_bucket,omitempty"`
	Dimensions map[string]string      `json:"dimensions"`
	Metrics    map[string]interface{} `json:"metrics"`
}

// OutpostMetricsMetadata describes how a metrics query was executed.
//
// Truncated reports that the row limit was hit, meaning the data is incomplete
// and should not be presented as a full picture.
type OutpostMetricsMetadata struct {
	Granularity string `json:"granularity,omitempty"`
	QueryTimeMS int    `json:"query_time_ms"`
	RowCount    int    `json:"row_count"`
	RowLimit    int    `json:"row_limit"`
	Truncated   bool   `json:"truncated"`
}

// OutpostMetricsResponse is the response from a metrics query.
type OutpostMetricsResponse struct {
	Data     []OutpostMetricsDataPoint `json:"data"`
	Metadata OutpostMetricsMetadata    `json:"metadata"`
}

// OutpostMetricsParams are the inputs to a metrics query.
//
// Start, End and Measures are required by the API. Filters holds the API's
// filters[<dimension>] parameters, keyed by dimension name.
type OutpostMetricsParams struct {
	Start       string
	End         string
	Granularity string
	Measures    []string
	Dimensions  []string
	Filters     map[string][]string
}

// GetOutpostEventMetrics returns aggregated event publish metrics.
// Supported measures are count and rate.
func (c *Client) GetOutpostEventMetrics(ctx context.Context, params OutpostMetricsParams) (*OutpostMetricsResponse, error) {
	return c.getOutpostMetrics(ctx, "events", params)
}

// GetOutpostAttemptMetrics returns aggregated delivery attempt metrics, such as
// counts, success and failure rates, and retry breakdowns.
func (c *Client) GetOutpostAttemptMetrics(ctx context.Context, params OutpostMetricsParams) (*OutpostMetricsResponse, error) {
	return c.getOutpostMetrics(ctx, "attempts", params)
}

func (c *Client) getOutpostMetrics(ctx context.Context, resource string, params OutpostMetricsParams) (*OutpostMetricsResponse, error) {
	if params.Start == "" || params.End == "" {
		return nil, fmt.Errorf("start and end are required for a metrics query")
	}
	if len(params.Measures) == 0 {
		return nil, fmt.Errorf("at least one measure is required for a metrics query")
	}

	scalar := map[string]string{
		"time[start]": params.Start,
		"time[end]":   params.End,
		"granularity": params.Granularity,
	}

	lists := map[string][]string{
		"measures":   params.Measures,
		"dimensions": params.Dimensions,
	}
	for dimension, values := range params.Filters {
		lists["filters["+dimension+"]"] = values
	}

	resp, err := c.Get(ctx, APIPathPrefix+"/metrics/"+resource, outpostQuery(scalar, lists), nil)
	if err != nil {
		return nil, err
	}

	var result OutpostMetricsResponse
	if _, err := postprocessJsonResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse %s metrics response: %w", resource, err)
	}

	return &result, nil
}
