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
