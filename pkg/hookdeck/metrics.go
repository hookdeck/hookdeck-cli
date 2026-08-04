package hookdeck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
)

// MetricDataPoint is a single metric data point with time bucket, dimensions, and metrics.
// All metrics endpoints return an array of MetricDataPoint.
type MetricDataPoint struct {
	TimeBucket *string                `json:"time_bucket,omitempty"`
	Dimensions map[string]interface{} `json:"dimensions,omitempty"`
	Metrics    map[string]float64     `json:"metrics,omitempty"`
}

// MetricsResponse is the response from any of the metrics GET endpoints.
type MetricsResponse = []MetricDataPoint

// MetricsQueryParams holds shared query parameters for all metrics endpoints.
// Start and End are required (ISO 8601 date-time).
// ConnectionID is mapped to API webhook_id in the CLI layer.
type MetricsQueryParams struct {
	Start         string // required, ISO 8601
	End           string // required, ISO 8601
	Granularity   string // e.g. 1h, 5m, 1d (pattern: \d+(s|m|h|d|w|M))
	Measures      []string
	Dimensions    []string
	SourceID      string
	DestinationID string
	ConnectionID  string // sent as filters[webhook_id]
	Status        string // e.g. SUCCESSFUL, FAILED
	IssueID       string // sent as filters[issue_id]; required for events-by-issue
}

// buildMetricsQuery builds the query string for metrics endpoints.
// Uses bracket notation: date_range[start], date_range[end], filters[webhook_id], etc.
func buildMetricsQuery(p MetricsQueryParams) string {
	q := url.Values{}
	q.Set("date_range[start]", p.Start)
	q.Set("date_range[end]", p.End)
	if p.Granularity != "" {
		q.Set("granularity", p.Granularity)
	}
	for _, m := range p.Measures {
		q.Add("measures[]", m)
	}
	for _, d := range p.Dimensions {
		q.Add("dimensions[]", d)
	}
	if p.SourceID != "" {
		q.Set("filters[source_id]", p.SourceID)
	}
	if p.DestinationID != "" {
		q.Set("filters[destination_id]", p.DestinationID)
	}
	if p.ConnectionID != "" {
		q.Set("filters[webhook_id]", p.ConnectionID)
	}
	if p.Status != "" {
		q.Set("filters[status]", p.Status)
	}
	if p.IssueID != "" {
		q.Set("filters[issue_id]", p.IssueID)
	}
	return q.Encode()
}

// metricsResponseWrapper is used when the API returns an object with a "data" array instead of a raw array.
type metricsResponseWrapper struct {
	Data MetricsResponse `json:"data"`
}

func (c *Client) queryMetrics(ctx context.Context, path string, params MetricsQueryParams) (MetricsResponse, error) {
	queryStr := buildMetricsQuery(params)
	resp, err := c.Get(ctx, APIPathPrefix+path, queryStr, nil)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read metrics response: %w", err)
	}
	// Try as array first (most endpoints return []MetricDataPoint).
	var result MetricsResponse
	if err := json.Unmarshal(body, &result); err == nil {
		return result, nil
	}
	// Some endpoints may return {"data": [...]}.
	var wrapped metricsResponseWrapper
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&wrapped); err != nil {
		return nil, fmt.Errorf("failed to parse metrics response: %w", err)
	}
	return wrapped.Data, nil
}

// QueryEventMetrics returns event metrics (GET /metrics/events).
func (c *Client) QueryEventMetrics(ctx context.Context, params MetricsQueryParams) (MetricsResponse, error) {
	return c.queryMetrics(ctx, "/metrics/events", params)
}

// QueryRequestMetrics returns request metrics (GET /metrics/requests).
func (c *Client) QueryRequestMetrics(ctx context.Context, params MetricsQueryParams) (MetricsResponse, error) {
	return c.queryMetrics(ctx, "/metrics/requests", params)
}

// QueryAttemptMetrics returns attempt metrics (GET /metrics/attempts).
func (c *Client) QueryAttemptMetrics(ctx context.Context, params MetricsQueryParams) (MetricsResponse, error) {
	return c.queryMetrics(ctx, "/metrics/attempts", params)
}

// QueryQueueDepth returns queue depth metrics (GET /metrics/queue-depth).
func (c *Client) QueryQueueDepth(ctx context.Context, params MetricsQueryParams) (MetricsResponse, error) {
	return c.queryMetrics(ctx, "/metrics/queue-depth", params)
}

// QueryEventsPendingTimeseries returns events pending timeseries (GET /metrics/events-pending-timeseries).
func (c *Client) QueryEventsPendingTimeseries(ctx context.Context, params MetricsQueryParams) (MetricsResponse, error) {
	return c.queryMetrics(ctx, "/metrics/events-pending-timeseries", params)
}

// QueryEventsByIssue returns events grouped by issue (GET /metrics/events-by-issue).
func (c *Client) QueryEventsByIssue(ctx context.Context, params MetricsQueryParams) (MetricsResponse, error) {
	return c.queryMetrics(ctx, "/metrics/events-by-issue", params)
}

// QueryTransformationMetrics returns transformation metrics (GET /metrics/transformations).
func (c *Client) QueryTransformationMetrics(ctx context.Context, params MetricsQueryParams) (MetricsResponse, error) {
	return c.queryMetrics(ctx, "/metrics/transformations", params)
}
