package hookdeck

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMetricsDimensionsMatchTheEndpointSchemas is the dimension counterpart of
// TestMetricsFlagsMatchTheEndpointSchemas in pkg/cmd.
//
// Expectations are hardcoded, so an API-side change will NOT fail this test; it
// pins the matrix both the CLI and MCP read, so the two cannot drift. Re-check
// against the `dimensions` enum of each GET /metrics/* operation in
// https://api.hookdeck.com/2026-09-01/openapi when the version moves.
func TestMetricsDimensionsMatchTheEndpointSchemas(t *testing.T) {
	tests := []struct {
		name   string
		actual []string
		want   []string
	}{
		{"requests", RequestMetricsDimensionValues,
			[]string{"source_id", "rejection_cause", "status", "bulk_retry_ids", "events_count", "ignored_count"}},
		{"attempts", AttemptMetricsDimensionValues,
			[]string{"destination_id", "delivery_group", "event_id", "status", "error_code", "bulk_retry_id", "trigger"}},
		{"transformations", TransformationMetricsDimensionValues,
			[]string{"transformation_id", "webhook_id", "log_level", "issue_id"}},
		{"events (default route)", DefaultEventRouteDimensions,
			[]string{"source_id", "destination_id", "webhook_id", "delivery_group", "status", "error_code", "event_data_id", "cli_id", "cli_user_id", "attempts", "response_status"}},
		{"queue depth", QueueDepthRouteDimensions,
			[]string{"destination_id", "delivery_group"}},
		{"pending timeseries", PendingTimeseriesRouteDimensions,
			[]string{"destination_id"}},
		{"events by issue", EventsByIssueRouteDimensions,
			[]string{"issue_id", "source_id", "destination_id", "webhook_id"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatch(t, tt.want, tt.actual)
		})
	}
}

// TestEventMetricsDimensionsIsTheUnionOfItsRoutes guards the list `metrics
// events` advertises. It fans out over four endpoints, so it offers the union
// and narrows per route at run time - the same shape as its filters. Building
// the union by hand is how it came to claim issue_id was a plain dimension
// while omitting error_code, cli_id and the rest.
func TestEventMetricsDimensionsIsTheUnionOfItsRoutes(t *testing.T) {
	var all []string
	all = append(all, DefaultEventRouteDimensions...)
	all = append(all, QueueDepthRouteDimensions...)
	all = append(all, PendingTimeseriesRouteDimensions...)
	all = append(all, EventsByIssueRouteDimensions...)

	seen := map[string]bool{}
	for _, d := range all {
		seen[d] = true
	}
	for _, d := range EventMetricsDimensionValues {
		assert.True(t, seen[d], "%q belongs to no events route", d)
	}
	assert.Len(t, EventMetricsDimensionValues, len(seen), "the union must be complete and free of duplicates")
}

// TestRejectUnsupportedDimensions covers the gate itself, including the API's
// cross-field rule: grouping by delivery_group without a destination filter is
// a 422, and that is the release's headline feature looking broken.
func TestRejectUnsupportedDimensions(t *testing.T) {
	tests := []struct {
		name     string
		params   MetricsQueryParams
		allowed  []string
		wantErr  bool
		contains []string
	}{
		{
			name:    "no dimensions is always fine",
			params:  MetricsQueryParams{},
			allowed: PendingTimeseriesRouteDimensions,
		},
		{
			name:    "a dimension the route defines passes",
			params:  MetricsQueryParams{Dimensions: []string{"destination_id"}},
			allowed: PendingTimeseriesRouteDimensions,
		},
		{
			name:     "a dimension the route does not define is named",
			params:   MetricsQueryParams{Dimensions: []string{"status"}},
			allowed:  PendingTimeseriesRouteDimensions,
			wantErr:  true,
			contains: []string{"--dimensions", "status", "test route", "destination_id"},
		},
		{
			name:     "delivery_group needs a destination filter",
			params:   MetricsQueryParams{Dimensions: []string{"delivery_group"}},
			allowed:  DefaultEventRouteDimensions,
			wantErr:  true,
			contains: []string{"delivery_group", "--destination-id"},
		},
		{
			name:    "delivery_group with a destination filter passes",
			params:  MetricsQueryParams{Dimensions: []string{"delivery_group"}, DestinationID: "des_1"},
			allowed: DefaultEventRouteDimensions,
		},
		{
			name:    "the caller's connection_id spelling is accepted",
			params:  MetricsQueryParams{Dimensions: []string{"connection_id"}},
			allowed: DefaultEventRouteDimensions,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RejectUnsupportedDimensions(tt.params, tt.allowed, "test route", CLIFilterNames, "--dimensions")
			if !tt.wantErr {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			for _, want := range tt.contains {
				assert.Contains(t, err.Error(), want)
			}
		})
	}
}

// TestDimensionListSpellsConnectionForTheCaller covers the one rename between
// the API and both callers: the API groups by webhook_id, and the CLI and MCP
// both take connection_id and map it. An error listing the API spelling would
// send the caller to a name their own tool does not accept.
func TestDimensionListSpellsConnectionForTheCaller(t *testing.T) {
	got := DimensionList([]string{"source_id", "webhook_id"})
	assert.Equal(t, "source_id, connection_id", got)
}

// TestMetricsMeasuresMatchTheEndpointSchemas pins the per-action measure lists
// the CLI's --help and the MCP schema both read. The MCP tool replaced four
// accurate lists with one that was wrong on three of the four actions.
func TestMetricsMeasuresMatchTheEndpointSchemas(t *testing.T) {
	assert.ElementsMatch(t,
		[]string{"count", "accepted_count", "rejected_count", "discarded_count", "avg_events_per_request", "avg_ignored_per_request"},
		RequestMetricsMeasureValues)
	assert.ElementsMatch(t,
		[]string{"count", "successful_count", "failed_count", "delivered_count", "error_rate", "response_latency_avg", "response_latency_max", "response_latency_p95", "response_latency_p99", "delivery_latency_avg"},
		AttemptMetricsMeasureValues)
	assert.ElementsMatch(t,
		[]string{"count", "successful_count", "failed_count", "error_rate", "error_count", "warn_count", "info_count", "debug_count"},
		TransformationMetricsMeasureValues)

	// count is the only measure all four actions share; the old MCP schema
	// advertised three more as "common".
	for _, action := range [][]string{
		EventMetricsMeasureValues, RequestMetricsMeasureValues,
		AttemptMetricsMeasureValues, TransformationMetricsMeasureValues,
	} {
		assert.Contains(t, action, "count")
	}
	assert.NotContains(t, RequestMetricsMeasureValues, "successful_count")
	assert.NotContains(t, RequestMetricsMeasureValues, "failed_count")
	assert.NotContains(t, RequestMetricsMeasureValues, "error_count")
	assert.NotContains(t, EventMetricsMeasureValues, "error_count")
	assert.NotContains(t, AttemptMetricsMeasureValues, "error_count")
}

// TestEveryEventMeasureRoutes keeps the measure vocabulary and the routing table
// in step: a measure advertised on `events` but absent from the routing table
// goes to the default endpoint by accident rather than by decision.
func TestEveryEventMeasureRoutes(t *testing.T) {
	for _, m := range EventMetricsMeasureValues {
		_, known := eventMeasureRoutes[m]
		assert.True(t, known, "advertised measure %q has no route", m)
	}
}
