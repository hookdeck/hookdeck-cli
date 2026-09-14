package mcp

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// TestMetricsToolRejectsFiltersTheEndpointIgnores is the MCP counterpart of
// TestMetricsFlagsMatchTheEndpointSchemas in pkg/cmd. Both consult the same
// matrix in pkg/hookdeck, so a filter added API-side fails on both sides rather
// than being fixed for the CLI and left wrong for MCP - which is exactly what
// happened when the CLI-side fix landed and this layer was not touched.
//
// The bug is silent: the API drops a filter its schema does not declare and
// answers with unfiltered totals, so an agent reading these numbers has no way
// to know the filter did nothing.
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

			result := callTool(t, session, "hookdeck_metrics", map[string]any{
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

	result := callTool(t, session, "hookdeck_metrics", map[string]any{
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

	result := callTool(t, session, "hookdeck_metrics", map[string]any{
		"action":     "events",
		"start":      "2025-01-01T00:00:00Z",
		"end":        "2025-01-02T00:00:00Z",
		"measures":   []any{"count"},
		"dimensions": []any{"connection_id"},
	})

	assert.False(t, result.IsError)
	assert.Equal(t, []string{"webhook_id"}, sawDimensions)
}
