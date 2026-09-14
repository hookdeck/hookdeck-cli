package mcp

import (
	"encoding/json"
	"net/http"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// TestRequestsEventsForwardsEveryFilterTheRouteDeclares pins the forwarding
// half of the events action.
//
// source_id was accepted and then dropped before the request was built, so
// "the events of req_X that came from source Y" returned every event of the
// request. The API was never the problem: GET /requests/{id}/events declares
// the whole /events filter set and honours it. Every argument the action
// advertises has to arrive under the name the API knows it by - the date
// bounds and connection_id are renamed on the way out, which is precisely
// where a silent drop hides.
func TestRequestsEventsForwardsEveryFilterTheRouteDeclares(t *testing.T) {
	tests := []struct {
		arg   string
		value any
		param string
		want  string
	}{
		{"source_id", "src_123", "source_id", "src_123"},
		{"connection_id", "web_123", "webhook_id", "web_123"},
		{"destination_id", "des_123", "destination_id", "des_123"},
		{"delivery_group", "dg_123", "delivery_group", "dg_123"},
		{"status", "FAILED", "status", "FAILED"},
		{"attempts", "3", "attempts", "3"},
		{"issue_id", "iss_123", "issue_id", "iss_123"},
		{"error_code", "TIMEOUT", "error_code", "TIMEOUT"},
		{"response_status", "500", "response_status", "500"},
		{"cli_id", "cli_123", "cli_id", "cli_123"},
		{"created_after", "2025-01-01T00:00:00Z", "created_at[gte]", "2025-01-01T00:00:00Z"},
		{"created_before", "2025-02-01T00:00:00Z", "created_at[lte]", "2025-02-01T00:00:00Z"},
		{"successful_after", "2025-01-01T00:00:00Z", "successful_at[gte]", "2025-01-01T00:00:00Z"},
		{"successful_before", "2025-02-01T00:00:00Z", "successful_at[lte]", "2025-02-01T00:00:00Z"},
		{"last_attempt_after", "2025-01-01T00:00:00Z", "last_attempt_at[gte]", "2025-01-01T00:00:00Z"},
		{"last_attempt_before", "2025-02-01T00:00:00Z", "last_attempt_at[lte]", "2025-02-01T00:00:00Z"},
		{"body", `{"type":"ping"}`, "body", `{"type":"ping"}`},
		{"headers", `{"x-trace":"1"}`, "headers", `{"x-trace":"1"}`},
		{"parsed_query", `{"q":"1"}`, "parsed_query", `{"q":"1"}`},
		{"path", "/hook", "path", "/hook"},
		{"order_by", "created_at", "order_by", "created_at"},
		{"dir", "asc", "dir", "asc"},
		{"limit", float64(5), "limit", "5"},
		{"next", "cur_next", "next", "cur_next"},
		{"prev", "cur_prev", "prev", "cur_prev"},
	}

	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			var saw string
			session := mockAPIWithClient(t, map[string]http.HandlerFunc{
				hookdeck.APIPathPrefix + "/requests/req_1/events": func(w http.ResponseWriter, r *http.Request) {
					saw = r.URL.Query().Get(tt.param)
					_ = json.NewEncoder(w).Encode(listResponse())
				},
			})

			result := callTool(t, session, "hookdeck_requests", map[string]any{
				"action": "events", "id": "req_1", tt.arg: tt.value,
			})

			assert.False(t, result.IsError, textContent(t, result))
			assert.Equal(t, tt.want, saw, "%s must reach the API as %s", tt.arg, tt.param)
		})
	}
}

// TestRequestsEventsMatchesTheEventsListFilterSet stops the two from drifting.
//
// GET /requests/{id}/events and GET /events declare the same query parameters,
// so hookdeck_requests action "events" and hookdeck_events action "list" must
// offer the same arguments. A filter added to one and forgotten on the other is
// how the events action ended up with four of them in the first place.
func TestRequestsEventsMatchesTheEventsListFilterSet(t *testing.T) {
	events := append([]string{}, requestsActionArgs["events"]...)
	list := append([]string{}, eventsActionArgs["list"]...)
	sort.Strings(events)
	sort.Strings(list)

	// id is in both, but means the request on one and the event on the other;
	// the sets still match, and nothing else is exempt.
	assert.Equal(t, list, events,
		"hookdeck_requests action events and hookdeck_events action list query routes with the same filter set")
}

// TestRequestsStatusIsValidatedPerAction covers the one argument whose meaning
// changes with the action: list filters requests by what happened at the edge,
// events filters deliveries by lifecycle state. One flat schema property
// carries both vocabularies, so a value from the wrong one is refused here
// instead of coming back as the API's 422, which names only the enum of the
// route it was sent to and never mentions the action that does take it.
func TestRequestsStatusIsValidatedPerAction(t *testing.T) {
	forwards := []struct {
		name   string
		action string
		path   string
		value  string
		want   string
	}{
		{"event status on events", "events", "/requests/req_1/events", "SUCCESSFUL", "SUCCESSFUL"},
		{"event status is case-insensitive", "events", "/requests/req_1/events", "successful", "SUCCESSFUL"},
		{"request status on list", "list", "/requests", "accepted", "accepted"},
		{"request status is case-insensitive", "list", "/requests", "ACCEPTED", "accepted"},
	}
	for _, tt := range forwards {
		t.Run(tt.name, func(t *testing.T) {
			var saw string
			session := mockAPIWithClient(t, map[string]http.HandlerFunc{
				hookdeck.APIPathPrefix + tt.path: func(w http.ResponseWriter, r *http.Request) {
					saw = r.URL.Query().Get("status")
					_ = json.NewEncoder(w).Encode(listResponse())
				},
			})
			result := callTool(t, session, "hookdeck_requests", map[string]any{
				"action": tt.action, "id": "req_1", "status": tt.value,
			})
			assert.False(t, result.IsError, textContent(t, result))
			assert.Equal(t, tt.want, saw, "status must reach the API in the spelling the enum uses")
		})
	}

	rejects := []struct {
		name   string
		action string
		value  string
		// names the vocabulary the value does belong to, so the error can send
		// the caller to the action that honours it.
		elsewhere string
	}{
		{"request status on events", "events", "accepted", "list"},
		{"event status on list", "list", "SUCCESSFUL", "events"},
	}
	for _, tt := range rejects {
		t.Run(tt.name, func(t *testing.T) {
			fail := func(w http.ResponseWriter, r *http.Request) {
				t.Fatalf("must not call %s with a status from the other vocabulary", r.URL.Path)
			}
			session := mockAPIWithClient(t, map[string]http.HandlerFunc{
				hookdeck.APIPathPrefix + "/requests":              fail,
				hookdeck.APIPathPrefix + "/requests/req_1/events": fail,
			})
			result := callTool(t, session, "hookdeck_requests", map[string]any{
				"action": tt.action, "id": "req_1", "status": tt.value,
			})
			require.True(t, result.IsError, "%s must be refused on the %s action", tt.value, tt.action)
			body := textContent(t, result)
			assert.Contains(t, body, tt.value)
			assert.Contains(t, body, tt.action)
			assert.Contains(t, body, tt.elsewhere, "the message should name the action that takes it")
		})
	}
}

// TestRequestsStatusDescriptionNamesBothVocabularies is the schema half of the
// same problem. The description used to read "accepted or rejected (list)"
// while the property is also the events filter, so an MCP client planning a
// call had no way to learn the event vocabulary existed.
func TestRequestsStatusDescriptionNamesBothVocabularies(t *testing.T) {
	desc := requestsToolProperties["status"].Desc
	assert.Contains(t, desc, hookdeck.RequestLogStatusValues, "the list vocabulary must be named")
	assert.Contains(t, desc, hookdeck.EventStatusValues, "the events vocabulary must be named")
	assert.Contains(t, desc, "list")
	assert.Contains(t, desc, "events")
}
