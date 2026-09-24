package mcp

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// TestMetricsToolRejectsFiltersTheEndpointIgnores is the MCP counterpart of
// TestMetricsFlagsMatchTheEndpointSchemas. Both read the same matrix, so the
// layers cannot drift - which is what happened when the CLI fix landed and this
// one was not touched. Expectations are hardcoded; see the pkg/cmd note.
func TestMetricsToolRejectsFiltersTheEndpointIgnores(t *testing.T) {
	tests := []struct {
		name     string
		action   string
		filter   string
		endpoint string
	}{
		{"requests ignores destination", "requests", "destination_id", "/metrics/requests"},
		{"requests ignores connection", "requests", "connection_id", "/metrics/requests"},
		{"requests ignores delivery group", "requests", "delivery_group", "/metrics/requests"},
		{"attempts ignores source", "attempts", "source_id", "/metrics/attempts"},
		{"attempts ignores connection", "attempts", "connection_id", "/metrics/attempts"},
		{"transformations ignores source", "transformations", "source_id", "/metrics/transformations"},
		{"transformations ignores status", "transformations", "status", "/metrics/transformations"},
		{"transformations ignores delivery group", "transformations", "delivery_group", "/metrics/transformations"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := mockAPIWithClient(t, map[string]http.HandlerFunc{
				hookdeck.APIPathPrefix + tt.endpoint: func(w http.ResponseWriter, r *http.Request) {
					t.Fatalf("must not call the API with a filter it would ignore (%s)", tt.filter)
				},
			})

			result := callTool(t, session, "gateway_metrics_read", map[string]any{
				"action":   tt.action,
				"start":    "2025-01-01T00:00:00Z",
				"end":      "2025-01-02T00:00:00Z",
				"measures": []any{"count"},
				tt.filter:  "x_bogus",
			})

			assert.True(t, result.IsError, "%s with %s must be refused", tt.action, tt.filter)
			body := textContent(t, result)
			assert.Contains(t, body, tt.filter)
			assert.Contains(t, body, "unfiltered", "the message should say why it matters")
		})
	}
}

// TestMetricsToolAcceptsFiltersTheEndpointHonours is the other half: a filter
// the schema does declare must still reach the API.
func TestMetricsToolAcceptsFiltersTheEndpointHonours(t *testing.T) {
	var sawFilter string
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/metrics/attempts": func(w http.ResponseWriter, r *http.Request) {
			sawFilter = r.URL.Query().Get("filters[destination_id]")
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		},
	})

	result := callTool(t, session, "gateway_metrics_read", map[string]any{
		"action":         "attempts",
		"start":          "2025-01-01T00:00:00Z",
		"end":            "2025-01-02T00:00:00Z",
		"measures":       []any{"count"},
		"destination_id": "des_123",
	})

	assert.False(t, result.IsError)
	assert.Equal(t, "des_123", sawFilter)
}

// TestMetricsToolMapsConnectionDimension covers the tool schema's own claim that
// connection_id "maps to webhook_id". That was true of the filter and not of the
// dimension, so a caller grouping by connection sent a dimension the API does
// not define.
func TestMetricsToolMapsConnectionDimension(t *testing.T) {
	var sawDimensions []string
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/metrics/events": func(w http.ResponseWriter, r *http.Request) {
			sawDimensions = r.URL.Query()["dimensions[]"]
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		},
	})

	result := callTool(t, session, "gateway_metrics_read", map[string]any{
		"action":     "events",
		"start":      "2025-01-01T00:00:00Z",
		"end":        "2025-01-02T00:00:00Z",
		"measures":   []any{"count"},
		"dimensions": []any{"connection_id"},
	})

	assert.False(t, result.IsError)
	assert.Equal(t, []string{"webhook_id"}, sawDimensions)
}

// TestMetricsToolRejectsFiltersTheEventsRouteIgnores is the events half of the
// gate above, and the one that matters most: `action: events` fans out over
// four endpoints that each honour a different set of filters, so the filter the
// caller passed is dropped by the API and unfiltered totals come back looking
// like an answer. That is the same silent-wrong-answer family as the
// delivery_group bug this release exists for.
//
// Each case names a filter the selected route does not declare, without naming
// a second route: the cross-route guard would otherwise refuse the call first
// and this gate would never be reached.
func TestMetricsToolRejectsFiltersTheEventsRouteIgnores(t *testing.T) {
	tests := []struct {
		name     string
		measures []any
		filter   string
		extra    map[string]any
		contains []string
	}{
		{
			name: "queue depth does not filter by source", measures: []any{"queue_depth"},
			filter: "source_id", contains: []string{"source_id", "queue depth metrics"},
		},
		{
			name: "queue depth does not filter by status", measures: []any{"queue_depth"},
			filter: "status", contains: []string{"status", "queue depth metrics"},
		},
		{
			name: "queue depth does not filter by connection", measures: []any{"max_age"},
			filter: "connection_id", contains: []string{"connection_id", "queue depth metrics"},
		},
		{
			name: "pending filters by destination only", measures: []any{"pending"},
			filter: "source_id", contains: []string{"source_id", "pending event metrics"},
		},
		{
			name: "pending does not filter by delivery group", measures: []any{"pending"},
			filter: "delivery_group", contains: []string{"delivery_group", "pending event metrics"},
		},
		{
			name: "per-issue does not filter by status", measures: []any{"count"},
			filter: "status", extra: map[string]any{"issue_id": "iss_1"},
			contains: []string{"status", "per-issue event metrics"},
		},
		{
			name: "per-issue does not filter by delivery group", measures: []any{"count"},
			filter: "delivery_group", extra: map[string]any{"issue_id": "iss_1"},
			contains: []string{"delivery_group", "per-issue event metrics"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fail := func(w http.ResponseWriter, r *http.Request) {
				t.Fatalf("must not call %s with a filter it would ignore (%s)", r.URL.Path, tt.filter)
			}
			session := mockAPIWithClient(t, map[string]http.HandlerFunc{
				hookdeck.APIPathPrefix + "/metrics/events":                    fail,
				hookdeck.APIPathPrefix + "/metrics/events-by-issue":           fail,
				hookdeck.APIPathPrefix + "/metrics/events-pending-timeseries": fail,
				hookdeck.APIPathPrefix + "/metrics/queue-depth":               fail,
			})

			args := map[string]any{
				"action":   "events",
				"start":    "2025-01-01T00:00:00Z",
				"end":      "2025-01-02T00:00:00Z",
				"measures": tt.measures,
				tt.filter:  "x_bogus",
			}
			for k, v := range tt.extra {
				args[k] = v
			}

			result := callTool(t, session, "gateway_metrics_read", args)

			assert.True(t, result.IsError, "events with %s must be refused", tt.filter)
			body := textContent(t, result)
			for _, want := range tt.contains {
				assert.Contains(t, body, want)
			}
			assert.Contains(t, body, "unfiltered", "the message should say why it matters")
		})
	}
}

// TestMetricsToolKeepsFiltersTheEventsRouteHonours is the other half: the
// filters each route does declare must still reach it, so the gate above cannot
// be satisfied by refusing everything.
func TestMetricsToolKeepsFiltersTheEventsRouteHonours(t *testing.T) {
	tests := []struct {
		name     string
		measures []any
		args     map[string]any
		endpoint string
		param    string
		want     string
	}{
		{
			name: "queue depth filters by destination", measures: []any{"queue_depth"},
			args:     map[string]any{"destination_id": "des_1"},
			endpoint: "/metrics/queue-depth", param: "filters[destination_id]", want: "des_1",
		},
		{
			name: "pending filters by destination", measures: []any{"pending"},
			args:     map[string]any{"destination_id": "des_1"},
			endpoint: "/metrics/events-pending-timeseries", param: "filters[destination_id]", want: "des_1",
		},
		{
			name: "per-issue filters by source", measures: []any{"count"},
			args:     map[string]any{"issue_id": "iss_1", "source_id": "src_1"},
			endpoint: "/metrics/events-by-issue", param: "filters[source_id]", want: "src_1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			session := mockAPIWithClient(t, map[string]http.HandlerFunc{
				hookdeck.APIPathPrefix + tt.endpoint: func(w http.ResponseWriter, r *http.Request) {
					got = r.URL.Query().Get(tt.param)
					_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
				},
			})

			args := map[string]any{
				"action":   "events",
				"start":    "2025-01-01T00:00:00Z",
				"end":      "2025-01-02T00:00:00Z",
				"measures": tt.measures,
			}
			for k, v := range tt.args {
				args[k] = v
			}

			result := callTool(t, session, "gateway_metrics_read", args)
			assert.False(t, result.IsError, textContent(t, result))
			assert.Equal(t, tt.want, got)
		})
	}
}

// The helper tests in pkg/hookdeck cover the mapping itself; this covers the
// tool calling it. Without a test at this level, removing the call from the
// handler leaves those unit tests green while the agent-facing result goes back
// to being keyed with an internal name the caller never used (#442).
func TestMetricsToolAnswersWithTheDimensionNameTheCallerAsked(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/metrics/events": func(w http.ResponseWriter, r *http.Request) {
			// The API answers in its own spelling, whatever we asked with.
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{
				map[string]any{
					"time_bucket": "2025-01-01T00:00:00.000Z",
					"dimensions":  map[string]any{"webhook_id": "web_123"},
					"metrics":     map[string]any{"count": 7},
				},
			}})
		},
	})

	result := callTool(t, session, "gateway_metrics_read", map[string]any{
		"action":     "events",
		"start":      "2025-01-01T00:00:00Z",
		"end":        "2025-01-02T00:00:00Z",
		"measures":   []any{"count"},
		"dimensions": []any{"connection_id"},
	})

	assert.False(t, result.IsError)
	body := textContent(t, result)
	assert.Contains(t, body, "connection_id",
		"the result must be keyed with the name the caller asked with")
	assert.Contains(t, body, "web_123", "the value itself is unchanged")
	assert.NotContains(t, body, "webhook_id",
		"the API's internal spelling must not reach the caller")
}
