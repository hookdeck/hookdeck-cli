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

// The API spells the connection dimension webhook_id. "connection" is the
// user-facing noun everywhere else in this CLI, so both surfaces accept
// connection_id and translate it on the way out to the API.
//
// The translation used to be one-way: a caller asked for connection_id and got
// results keyed webhook_id, so they had to know an internal name they never
// used in order to read their own answer. For an agent that is worse than
// untidy -- it looks for the key it asked for, does not find it, and cannot
// learn that webhook_id means the same thing. See #442.
//
// Both directions live here, next to each other and shared by the MCP tool and
// the CLI, so neither half can drift from the other.
const (
	ConnectionDimension    = "connection_id"
	APIConnectionDimension = "webhook_id"
)

// MapDimensionsToAPI rewrites caller-facing dimension names to the API's.
func MapDimensionsToAPI(dimensions []string) []string {
	if len(dimensions) == 0 {
		return dimensions
	}
	out := make([]string, 0, len(dimensions))
	for _, d := range dimensions {
		if d == ConnectionDimension {
			d = APIConnectionDimension
		}
		out = append(out, d)
	}
	return out
}

// RestoreDimensionNames rewrites the API's dimension keys back to the ones the
// caller used, in place, so a response answers in the vocabulary of its query.
func RestoreDimensionNames(points MetricsResponse) MetricsResponse {
	for i := range points {
		d := points[i].Dimensions
		if d == nil {
			continue
		}
		v, ok := d[APIConnectionDimension]
		if !ok {
			continue
		}
		// Never clobber a connection_id the API itself returned.
		if _, exists := d[ConnectionDimension]; exists {
			continue
		}
		d[ConnectionDimension] = v
		delete(d, APIConnectionDimension)
	}
	return points
}

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
	DeliveryGroup string // sent as filters[delivery_group]
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
	if p.DeliveryGroup != "" {
		q.Set("filters[delivery_group]", p.DeliveryGroup)
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
