package mcp

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

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
