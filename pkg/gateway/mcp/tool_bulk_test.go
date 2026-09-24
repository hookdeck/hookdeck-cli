package mcp

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// Each family's route, so a call cannot quietly hit the wrong one.
func TestBulkActionsHitTheirOwnRoutes(t *testing.T) {
	cases := []struct {
		family string
		route  string
	}{
		{"events_retry", "/bulk/events/retry"},
		{"events_cancel", "/bulk/events/cancel"},
		{"ignored_events_retry", "/bulk/ignored-events/retry"},
		{"requests_retry", "/bulk/requests/retry"},
		{"requests_replay", "/bulk/requests/replay"},
	}

	for _, tc := range cases {
		t.Run(tc.family, func(t *testing.T) {
			var sawPlan, sawList bool
			session := mockAPIWithClient(t, map[string]http.HandlerFunc{
				hookdeck.APIPathPrefix + tc.route + "/plan": func(w http.ResponseWriter, r *http.Request) {
					sawPlan = true
					_ = json.NewEncoder(w).Encode(map[string]any{"estimated_count": 42})
				},
				hookdeck.APIPathPrefix + tc.route: func(w http.ResponseWriter, r *http.Request) {
					sawList = true
					_ = json.NewEncoder(w).Encode(map[string]any{"models": []any{map[string]any{"id": "bar_1"}}})
				},
			})

			plan := callTool(t, session, "gateway_bulk_read",
				map[string]any{"action": "plan", "operation": tc.family, "query": map[string]any{}})
			assert.False(t, plan.IsError, textContent(t, plan))
			assert.True(t, sawPlan, "plan must query the family's own plan route")
			assert.Contains(t, textContent(t, plan), "42")

			list := callTool(t, session, "gateway_bulk_read",
				map[string]any{"action": "list", "operation": tc.family})
			assert.False(t, list.IsError, textContent(t, list))
			assert.True(t, sawList)
			assert.Contains(t, textContent(t, list), "bar_1")
		})
	}
}

func TestBulkGetAndCreateAndCancel(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/bulk/events/retry/bar_1": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "bar_1", "estimated_count": 7})
		},
		hookdeck.APIPathPrefix + "/bulk/events/retry/bar_1/cancel": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "bar_1", "cancelled_at": "2026-09-22T00:00:00Z"})
		},
	})
	get := callTool(t, session, "gateway_bulk_read", map[string]any{"action": "get", "operation": "events_retry", "id": "bar_1"})
	assert.False(t, get.IsError, textContent(t, get))
	assert.Contains(t, textContent(t, get), "bar_1")

	write := mockAPIWithClientWriteEnabled(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/bulk/events/retry": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "bar_new"})
		},
		hookdeck.APIPathPrefix + "/bulk/events/retry/bar_1/cancel": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "bar_1", "cancelled_at": "2026-09-22T00:00:00Z"})
		},
	})
	created := callTool(t, write, "gateway_bulk_write",
		map[string]any{"action": "create", "operation": "events_retry", "query": map[string]any{"status": "FAILED"}})
	assert.False(t, created.IsError, textContent(t, created))
	assert.Contains(t, textContent(t, created), "bar_new")

	cancelled := callTool(t, write, "gateway_bulk_write",
		map[string]any{"action": "cancel", "operation": "events_retry", "id": "bar_1"})
	assert.False(t, cancelled.IsError, textContent(t, cancelled))
	assert.Contains(t, textContent(t, cancelled), "cancelled_at")
}

// The reason the filter matrix exists: a filter the family does not declare
// would be ignored by the API and the operation would run across everything the
// rest matched.
func TestBulkRefusesFiltersTheOperationDoesNotDeclare(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/bulk/ignored-events/retry/plan": func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("a filter the operation does not declare must not reach the API")
		},
	})
	result := callTool(t, session, "gateway_bulk_read", map[string]any{
		"action": "plan", "operation": "ignored_events_retry",
		"query": map[string]any{"status": "FAILED"},
	})
	require.True(t, result.IsError)
	body := textContent(t, result)
	assert.Contains(t, body, "status")
	assert.Contains(t, body, "cause", "the message should name what the operation does take")
}

// plan is a read, so sizing a bulk operation needs no write access at all.
func TestBulkPlanWorksWithoutWriteMode(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/bulk/events/retry/plan": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"estimated_count": 1200})
		},
	})
	result := callTool(t, session, "gateway_bulk_read",
		map[string]any{"action": "plan", "operation": "events_retry", "query": map[string]any{"status": "FAILED"}})
	assert.False(t, result.IsError, textContent(t, result))
	assert.Contains(t, textContent(t, result), "1200")

	// create is not reachable without --allow-write.
	blocked := callTool(t, session, "gateway_bulk_read",
		map[string]any{"action": "create", "operation": "events_retry", "query": map[string]any{}})
	require.True(t, blocked.IsError)
	assert.Contains(t, textContent(t, blocked), "--allow-write")
}

// events_cancel has no cancel route. Saying so beats a 404.
func TestBulkEventsCancelCannotBeCancelled(t *testing.T) {
	session := mockAPIWithClientWriteEnabled(t, nil)
	result := callTool(t, session, "gateway_bulk_write",
		map[string]any{"action": "cancel", "operation": "events_cancel", "id": "bar_1"})
	require.True(t, result.IsError)
	body := textContent(t, result)
	assert.Contains(t, body, "cannot be cancelled")
	assert.Contains(t, body, "events_retry", "name the operations that can be")
}
