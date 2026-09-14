package mcp

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// TestMetricsToolRejectsMixedMeasureRoutes is the MCP counterpart of
// TestMixedMeasureRoutesAreRejected. Routing picks one endpoint from the
// measures, so a list spanning two of them cannot be answered: the surplus was
// dropped or rewritten into a 422 while the call still reported success. Both
// layers call the same helper so they cannot drift.
func TestMetricsToolRejectsMixedMeasureRoutes(t *testing.T) {
	tests := []struct {
		name     string
		measures []any
		contains []string
	}{
		{
			name:     "pending with a default-route measure",
			measures: []any{"pending", "failed_count"},
			contains: []string{"measures", `"pending"`, `"failed_count"`},
		},
		{
			name:     "default-route measure with queue depth",
			measures: []any{"count", "queue_depth"},
			contains: []string{`"count"`, `"queue_depth"`, "queue depth metrics"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fail := func(w http.ResponseWriter, r *http.Request) {
				t.Fatalf("must not call %s with measures spanning two endpoints", r.URL.Path)
			}
			session := mockAPIWithClient(t, map[string]http.HandlerFunc{
				hookdeck.APIPathPrefix + "/metrics/events":                    fail,
				hookdeck.APIPathPrefix + "/metrics/queue-depth":               fail,
				hookdeck.APIPathPrefix + "/metrics/events-pending-timeseries": fail,
			})

			result := callTool(t, session, "hookdeck_metrics", map[string]any{
				"action":   "events",
				"start":    "2025-01-01T00:00:00Z",
				"end":      "2025-01-02T00:00:00Z",
				"measures": tt.measures,
			})

			assert.True(t, result.IsError, "a cross-route measure list must be refused")
			body := textContent(t, result)
			for _, want := range tt.contains {
				assert.Contains(t, body, want)
			}
		})
	}
}

// TestMetricsToolAcceptsSingleRouteMeasures is the other half: measures that all
// belong to one endpoint must still be sent together.
func TestMetricsToolAcceptsSingleRouteMeasures(t *testing.T) {
	var sawMeasures []string
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/metrics/events": func(w http.ResponseWriter, r *http.Request) {
			sawMeasures = r.URL.Query()["measures[]"]
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		},
	})

	result := callTool(t, session, "hookdeck_metrics", map[string]any{
		"action":   "events",
		"start":    "2025-01-01T00:00:00Z",
		"end":      "2025-01-02T00:00:00Z",
		"measures": []any{"count", "failed_count"},
	})

	assert.False(t, result.IsError)
	assert.Equal(t, []string{"count", "failed_count"}, sawMeasures)
}

// TestMetricsToolRejectsCrossRouteEventQuery covers #407 on the MCP surface.
//
// `events` routing is ordered - measures, then the issue_id dimension, then the
// issue filter - and first match wins. A queue-depth or pending measure
// therefore shadowed a per-issue question entirely: the call returned queue
// depth, reported success, and never mentioned that the issue_id half of the
// question had been dropped. The CLI fix landed in shared code (#407); this is
// the same guard on the tool, which has the identical ordered routing.
func TestMetricsToolRejectsCrossRouteEventQuery(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]any
		contains []string
	}{
		{
			name: "queue depth measure shadows the issue_id dimension",
			args: map[string]any{
				"measures":   []any{"queue_depth"},
				"dimensions": []any{"issue_id"},
				"issue_id":   "iss_1",
			},
			contains: []string{`"queue_depth"`, "queue depth metrics", "per-issue event metrics", "dimensions"},
		},
		{
			name: "pending measure shadows the issue_id dimension",
			args: map[string]any{
				"measures":   []any{"pending"},
				"dimensions": []any{"issue_id"},
				"issue_id":   "iss_1",
			},
			contains: []string{`"pending"`, "pending event metrics", "per-issue event metrics"},
		},
		{
			name: "queue depth measure shadows the issue filter on its own",
			args: map[string]any{
				"measures": []any{"max_depth"},
				"issue_id": "iss_1",
			},
			contains: []string{`"max_depth"`, "queue depth metrics", "issue_id", "per-issue event metrics"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fail := func(w http.ResponseWriter, r *http.Request) {
				t.Fatalf("must not call %s: the query names two routes", r.URL.Path)
			}
			session := mockAPIWithClient(t, map[string]http.HandlerFunc{
				hookdeck.APIPathPrefix + "/metrics/events":                    fail,
				hookdeck.APIPathPrefix + "/metrics/events-by-issue":           fail,
				hookdeck.APIPathPrefix + "/metrics/events-pending-timeseries": fail,
				hookdeck.APIPathPrefix + "/metrics/queue-depth":               fail,
			})

			args := map[string]any{
				"action": "events",
				"start":  "2025-01-01T00:00:00Z",
				"end":    "2025-01-02T00:00:00Z",
			}
			for k, v := range tt.args {
				args[k] = v
			}

			result := callTool(t, session, "hookdeck_metrics", args)

			assert.True(t, result.IsError, "a query naming two routes must be refused")
			body := textContent(t, result)
			for _, want := range tt.contains {
				assert.Contains(t, body, want, "the error must name both routes")
			}
		})
	}
}

// TestMetricsToolStillAnswersSingleRouteEventQueries is the other half: a
// per-issue question with a default-route measure is a per-issue count, not a
// conflict, and must still reach the by-issue endpoint.
func TestMetricsToolStillAnswersSingleRouteEventQueries(t *testing.T) {
	var called bool
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/metrics/events-by-issue": func(w http.ResponseWriter, r *http.Request) {
			called = true
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		},
	})

	result := callTool(t, session, "hookdeck_metrics", map[string]any{
		"action":     "events",
		"start":      "2025-01-01T00:00:00Z",
		"end":        "2025-01-02T00:00:00Z",
		"measures":   []any{"count"},
		"dimensions": []any{"issue_id"},
		"issue_id":   "iss_1",
	})

	assert.False(t, result.IsError, textContent(t, result))
	assert.True(t, called, "a per-issue count must still reach the by-issue endpoint")
}

// TestMetricsToolTranslatesQueueDepthMeasureOnTheWire is the MCP mirror of
// TestQueueDepthMeasureIsTranslatedOnTheWire.
//
// "queue_depth" is our own spelling for the route and the tool schema
// advertises it, but /metrics/queue-depth accepts max_depth and max_age only.
// Without the translation the tool sends a 422 for a value it told the caller
// to pass, so this pins the rewrite at the request, not in the helper.
func TestMetricsToolTranslatesQueueDepthMeasureOnTheWire(t *testing.T) {
	var sawMeasures []string
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/metrics/queue-depth": func(w http.ResponseWriter, r *http.Request) {
			sawMeasures = r.URL.Query()["measures[]"]
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		},
	})

	result := callTool(t, session, "hookdeck_metrics", map[string]any{
		"action":   "events",
		"start":    "2025-01-01T00:00:00Z",
		"end":      "2025-01-02T00:00:00Z",
		"measures": []any{"queue_depth"},
	})

	assert.False(t, result.IsError, textContent(t, result))
	assert.Equal(t, []string{"max_depth"}, sawMeasures,
		"queue_depth must reach the API as max_depth")
}
