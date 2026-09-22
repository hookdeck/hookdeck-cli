package mcp

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// TestMetricsToolRejectsDimensionsTheRouteIgnores is the dimension counterpart
// of TestMetricsToolRejectsFiltersTheEndpointIgnores.
//
// Filters were gated per route and dimensions were not gated at all, so a
// dimension the route does not define reached the API as a raw 422 for
// something the flat schema appeared to offer. Both layers read the same matrix
// in pkg/hookdeck so they cannot drift.
func TestMetricsToolRejectsDimensionsTheRouteIgnores(t *testing.T) {
	tests := []struct {
		name      string
		action    string
		measures  []any
		dimension string
		// extra carries the arguments a route needs to be selected at all -
		// the by-issue route is chosen by issue_id, not by a measure.
		extra    map[string]any
		contains []string
	}{
		// 422 dimensions[0] must be [destination_id]
		{
			name: "pending timeseries groups by destination only", action: "events",
			measures: []any{"pending"}, dimension: "status",
			contains: []string{"status", "pending event metrics", "destination_id"},
		},
		{
			name: "queue depth has no status dimension", action: "events",
			measures: []any{"queue_depth"}, dimension: "status",
			contains: []string{"queue depth metrics", "destination_id, delivery_group"},
		},
		{
			name: "default event route has no issue_id dimension", action: "events",
			measures: []any{"count"}, dimension: "rejection_cause",
			contains: []string{"rejection_cause", "event metrics"},
		},
		{
			name: "per-issue route has a narrower set", action: "events",
			measures: []any{"count"}, dimension: "status",
			extra:    map[string]any{"issue_id": "iss_1"},
			contains: []string{"status", "per-issue event metrics", "issue_id, source_id, destination_id, connection_id"},
		},
		{
			name: "requests do not group by destination", action: "requests",
			measures: []any{"count"}, dimension: "destination_id",
			contains: []string{"destination_id", "request metrics"},
		},
		{
			name: "attempts do not group by source", action: "attempts",
			measures: []any{"count"}, dimension: "source_id",
			contains: []string{"source_id", "attempt metrics"},
		},
		{
			name: "transformations do not group by status", action: "transformations",
			measures: []any{"count"}, dimension: "status",
			contains: []string{"status", "transformation metrics"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fail := func(w http.ResponseWriter, r *http.Request) {
				t.Fatalf("must not call %s with a dimension it does not define (%s)", r.URL.Path, tt.dimension)
			}
			session := mockAPIWithClient(t, map[string]http.HandlerFunc{
				hookdeck.APIPathPrefix + "/metrics/events":                    fail,
				hookdeck.APIPathPrefix + "/metrics/events-by-issue":           fail,
				hookdeck.APIPathPrefix + "/metrics/events-pending-timeseries": fail,
				hookdeck.APIPathPrefix + "/metrics/queue-depth":               fail,
				hookdeck.APIPathPrefix + "/metrics/requests":                  fail,
				hookdeck.APIPathPrefix + "/metrics/attempts":                  fail,
				hookdeck.APIPathPrefix + "/metrics/transformations":           fail,
			})

			args := map[string]any{
				"action":     tt.action,
				"start":      "2025-01-01T00:00:00Z",
				"end":        "2025-01-02T00:00:00Z",
				"measures":   tt.measures,
				"dimensions": []any{tt.dimension},
			}
			for k, v := range tt.extra {
				args[k] = v
			}

			result := callTool(t, session, "gateway_metrics", args)

			assert.True(t, result.IsError, "%s grouped by %s must be refused", tt.action, tt.dimension)
			body := textContent(t, result)
			for _, want := range tt.contains {
				assert.Contains(t, body, want)
			}
		})
	}
}

// TestMetricsToolRejectsDeliveryGroupDimensionWithoutDestination covers the
// API's one cross-field rule, and the worst of these to leave unguarded: this
// is the delivery-group grouping the release exists for, appearing usable and
// answering "422 The delivery_group dimension requires a filters.destination_id
// filter". The message has to name the fix, because adding destination_id works.
func TestMetricsToolRejectsDeliveryGroupDimensionWithoutDestination(t *testing.T) {
	for _, action := range []string{"events", "attempts"} {
		t.Run(action, func(t *testing.T) {
			fail := func(w http.ResponseWriter, r *http.Request) {
				t.Fatalf("must not call %s: the API rejects delivery_group grouping without destination_id", r.URL.Path)
			}
			session := mockAPIWithClient(t, map[string]http.HandlerFunc{
				hookdeck.APIPathPrefix + "/metrics/events":   fail,
				hookdeck.APIPathPrefix + "/metrics/attempts": fail,
			})

			result := callTool(t, session, "gateway_metrics", map[string]any{
				"action":     action,
				"start":      "2025-01-01T00:00:00Z",
				"end":        "2025-01-02T00:00:00Z",
				"measures":   []any{"count"},
				"dimensions": []any{"delivery_group"},
			})

			assert.True(t, result.IsError)
			body := textContent(t, result)
			assert.Contains(t, body, "delivery_group")
			assert.Contains(t, body, "destination_id", "the message must name the filter that fixes it")
		})
	}
}

// TestMetricsToolAcceptsDimensionsTheRouteHonours is the other half. Grouping
// by delivery_group with a destination filter is the combination that works
// against the live API, so it must still reach it.
func TestMetricsToolAcceptsDimensionsTheRouteHonours(t *testing.T) {
	var sawDimensions []string
	var sawDestination string
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/metrics/events": func(w http.ResponseWriter, r *http.Request) {
			sawDimensions = r.URL.Query()["dimensions[]"]
			sawDestination = r.URL.Query().Get("filters[destination_id]")
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		},
	})

	result := callTool(t, session, "gateway_metrics", map[string]any{
		"action":         "events",
		"start":          "2025-01-01T00:00:00Z",
		"end":            "2025-01-02T00:00:00Z",
		"measures":       []any{"count"},
		"dimensions":     []any{"delivery_group"},
		"destination_id": "des_123",
	})

	assert.False(t, result.IsError, textContent(t, result))
	assert.Equal(t, []string{"delivery_group"}, sawDimensions)
	assert.Equal(t, "des_123", sawDestination)
}

// TestMetricsToolMapsConnectionDimensionBeforeGating makes sure the caller's
// connection_id spelling is translated before it is checked, so the alias the
// schema documents is not refused by the new gate.
func TestMetricsToolMapsConnectionDimensionBeforeGating(t *testing.T) {
	var sawDimensions []string
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/metrics/events": func(w http.ResponseWriter, r *http.Request) {
			sawDimensions = r.URL.Query()["dimensions[]"]
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		},
	})

	result := callTool(t, session, "gateway_metrics", map[string]any{
		"action":     "events",
		"start":      "2025-01-01T00:00:00Z",
		"end":        "2025-01-02T00:00:00Z",
		"measures":   []any{"count"},
		"dimensions": []any{"connection_id"},
	})

	assert.False(t, result.IsError, textContent(t, result))
	assert.Equal(t, []string{"webhook_id"}, sawDimensions)
}
