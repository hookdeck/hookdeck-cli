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
		excludes []string
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
		{
			// Both callers rewrite connection_id to webhook_id before
			// validating, so this is what actually arrives here. Naming the
			// wire spelling back refused "webhook_id" at a caller who typed
			// connection_id, in the same sentence as an allowed list that
			// spells it connection_id.
			name:     "a refused connection dimension is named as the caller spells it",
			params:   MetricsQueryParams{Dimensions: []string{"webhook_id"}},
			allowed:  PendingTimeseriesRouteDimensions,
			wantErr:  true,
			contains: []string{"--dimensions", `"connection_id"`, "destination_id"},
			excludes: []string{"webhook_id"},
		},
		{
			name:     "and the same when the caller's own spelling arrives unmapped",
			params:   MetricsQueryParams{Dimensions: []string{"connection_id"}},
			allowed:  PendingTimeseriesRouteDimensions,
			wantErr:  true,
			contains: []string{`"connection_id"`},
			excludes: []string{"webhook_id"},
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
			for _, unwanted := range tt.excludes {
				assert.NotContains(t, err.Error(), unwanted,
					"the error must not name a token the caller never typed")
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

// TestRouteForMeasuresIsTheOneRoutingTable pins the membership both callers now
// read instead of keeping a copy.
//
// `metrics events` had three encodings of "which measures are queue depth": a
// map in the CLI, a containsAny list in MCP, and this table. They agreed, but a
// divergence between any two would refuse a mix against one and dispatch it to
// the wrong endpoint against the other - the failure mode being that the
// refusal and the dispatch disagree about what was asked for.
func TestRouteForMeasuresIsTheOneRoutingTable(t *testing.T) {
	tests := []struct {
		measures []string
		want     string
	}{
		{[]string{"queue_depth"}, EventRouteQueueDepth},
		{[]string{"max_depth"}, EventRouteQueueDepth},
		{[]string{"max_age"}, EventRouteQueueDepth},
		{[]string{"max_depth", "max_age"}, EventRouteQueueDepth},
		{[]string{"pending"}, EventRoutePending},
		{[]string{"count"}, EventRouteDefault},
		{[]string{"error_rate", "failed_count"}, EventRouteDefault},
		// A measure this package does not route on leaves the decision to the
		// dimensions, and the API to reject the measure itself.
		{[]string{"not_a_measure"}, ""},
		{[]string{"not_a_measure", "max_age"}, EventRouteQueueDepth},
		{nil, ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, RouteForMeasures(tt.measures), "measures %v", tt.measures)
	}

	// Every advertised measure has to land on a route the caller can name, or
	// the guards that print the route name print an empty string.
	for _, m := range EventMetricsMeasureValues {
		route := RouteForMeasures([]string{m})
		assert.NotEmpty(t, route, "advertised measure %q routes nowhere", m)
	}
}

// TestEventRouteNamesAreUnique is the other half of the tidy-up: every guard
// used to hand-write the route name, which produced two names for one route in
// adjacent errors ("pending event metrics" from the cross-route refusal,
// "pending event metrics (--measures pending)" from the filter guard beside
// it). One constant per route means one name per route.
func TestEventRouteNamesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, name := range []string{EventRouteDefault, EventRouteQueueDepth, EventRoutePending, EventRouteByIssue} {
		assert.NotEmpty(t, name)
		assert.False(t, seen[name], "%q names two routes", name)
		seen[name] = true
	}
}
