package mcp

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// TestRequestsToolRejectsArgsTheActionDrops is the hookdeck_requests counterpart
// of TestMetricsToolRejectsFiltersTheEndpointIgnores.
//
// The CLI has one flag set per subcommand, so `gateway request list` simply has
// no --delivery-group. This tool flattens five subcommands into one flat
// schema, so the filter was accepted, never forwarded, and the caller read 200
// unfiltered rows as a filtered result. Every other filter narrows to zero rows
// on a bogus value, so nothing in the response revealed the drop.
func TestRequestsToolRejectsArgsTheActionDrops(t *testing.T) {
	tests := []struct {
		name   string
		action string
		arg    string
		value  any
		// applies names an action that does honour the argument, so the error
		// can point the caller somewhere useful.
		applies string
	}{
		// The reported bug: delivery_group on list. The API has no such query
		// parameter on /requests, so it answered with unfiltered totals.
		{"list drops delivery_group", "list", "delivery_group", "dg_bogus", "events"},
		// source_id, status and created_after used to be here. They are not
		// dropped: GET /requests/{id}/events declares the /events filter set and
		// honours all three, so they are forwarded now and covered by
		// TestRequestsEventsForwardsEveryFilterTheRouteDeclares. What stays
		// refused on events is what the sub-resource genuinely has no parameter
		// for - the three fields describing the edge decision on the request
		// itself, which only /requests carries.
		{"events drops verified", "events", "verified", true, "list"},
		{"events drops rejection_cause", "events", "rejection_cause", "NO_CONNECTION", "list"},
		{"events drops ingested_after", "events", "ingested_after", "2025-01-01T00:00:00Z", "list"},
		{"ignored_events drops delivery_group", "ignored_events", "delivery_group", "dg_bogus", "events"},
		{"get drops source_id", "get", "source_id", "src_bogus", "list"},
		{"raw_body drops limit", "raw_body", "limit", float64(10), "list"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fail := func(w http.ResponseWriter, r *http.Request) {
				t.Fatalf("must not call %s with %s, which the %s action drops", r.URL.Path, tt.arg, tt.action)
			}
			session := mockAPIWithClient(t, map[string]http.HandlerFunc{
				hookdeck.APIPathPrefix + "/requests":                      fail,
				hookdeck.APIPathPrefix + "/requests/req_1":                fail,
				hookdeck.APIPathPrefix + "/requests/req_1/events":         fail,
				hookdeck.APIPathPrefix + "/requests/req_1/ignored_events": fail,
				hookdeck.APIPathPrefix + "/requests/req_1/raw_body":       fail,
			})

			result := callTool(t, session, "hookdeck_requests", map[string]any{
				"action": tt.action,
				"id":     "req_1",
				tt.arg:   tt.value,
			})

			assert.True(t, result.IsError, "%s with %s must be refused", tt.action, tt.arg)
			body := textContent(t, result)
			assert.Contains(t, body, tt.arg)
			assert.Contains(t, body, tt.action)
			assert.Contains(t, body, "unfiltered", "the message should say why it matters")
			assert.Contains(t, body, tt.applies, "the message should name an action that honours it")
		})
	}
}

// TestEventsToolRejectsArgsTheActionDrops is the same guard on hookdeck_events:
// get and raw_body address one event by id, so a list filter passed alongside
// them was silently ignored.
func TestEventsToolRejectsArgsTheActionDrops(t *testing.T) {
	tests := []struct {
		name   string
		action string
		arg    string
	}{
		{"get drops source_id", "get", "source_id"},
		{"get drops delivery_group", "get", "delivery_group"},
		{"raw_body drops status", "raw_body", "status"},
		{"raw_body drops connection_id", "raw_body", "connection_id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fail := func(w http.ResponseWriter, r *http.Request) {
				t.Fatalf("must not call %s with %s, which the %s action drops", r.URL.Path, tt.arg, tt.action)
			}
			session := mockAPIWithClient(t, map[string]http.HandlerFunc{
				hookdeck.APIPathPrefix + "/events":                fail,
				hookdeck.APIPathPrefix + "/events/evt_1":          fail,
				hookdeck.APIPathPrefix + "/events/evt_1/raw_body": fail,
			})

			result := callTool(t, session, "hookdeck_events", map[string]any{
				"action": tt.action,
				"id":     "evt_1",
				tt.arg:   "x_bogus",
			})

			assert.True(t, result.IsError, "%s with %s must be refused", tt.action, tt.arg)
			body := textContent(t, result)
			assert.Contains(t, body, tt.arg)
			assert.Contains(t, body, "unfiltered")
		})
	}
}

// TestRequestsToolForwardsArgsTheActionHonours is the other half: an argument
// the action does support must still reach the API. Refusing everything would
// pass the test above and break the tool.
func TestRequestsToolForwardsArgsTheActionHonours(t *testing.T) {
	t.Run("delivery_group on events", func(t *testing.T) {
		var saw string
		session := mockAPIWithClient(t, map[string]http.HandlerFunc{
			hookdeck.APIPathPrefix + "/requests/req_1/events": func(w http.ResponseWriter, r *http.Request) {
				saw = r.URL.Query().Get("delivery_group")
				_ = json.NewEncoder(w).Encode(listResponse())
			},
		})
		result := callTool(t, session, "hookdeck_requests", map[string]any{
			"action": "events", "id": "req_1", "delivery_group": "dg_123",
		})
		assert.False(t, result.IsError, textContent(t, result))
		assert.Equal(t, "dg_123", saw)
	})

	t.Run("source_id on list", func(t *testing.T) {
		var saw string
		session := mockAPIWithClient(t, map[string]http.HandlerFunc{
			hookdeck.APIPathPrefix + "/requests": func(w http.ResponseWriter, r *http.Request) {
				saw = r.URL.Query().Get("source_id")
				_ = json.NewEncoder(w).Encode(listResponse())
			},
		})
		result := callTool(t, session, "hookdeck_requests", map[string]any{
			"action": "list", "source_id": "src_123",
		})
		assert.False(t, result.IsError, textContent(t, result))
		assert.Equal(t, "src_123", saw)
	})
}

// TestRequestsIgnoredEventsForwardsPagination covers a quieter drop of the same
// shape: the handler passed nil params, so limit/next/prev never reached a
// route that accepts all three. A caller asking for one row got the default
// page and no cursor to move off it.
func TestRequestsIgnoredEventsForwardsPagination(t *testing.T) {
	var sawLimit, sawNext string
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/requests/req_1/ignored_events": func(w http.ResponseWriter, r *http.Request) {
			sawLimit = r.URL.Query().Get("limit")
			sawNext = r.URL.Query().Get("next")
			_ = json.NewEncoder(w).Encode(listResponse())
		},
	})

	result := callTool(t, session, "hookdeck_requests", map[string]any{
		"action": "ignored_events", "id": "req_1", "limit": 5, "next": "cur_abc",
	})

	assert.False(t, result.IsError, textContent(t, result))
	assert.Equal(t, "5", sawLimit)
	assert.Equal(t, "cur_abc", sawNext)
}

// TestActionArgsCoverEveryDeclaredProperty stops the schema and the matrix
// drifting: a property the tool advertises that no action honours is a filter
// that can only ever be dropped or refused, and one the matrix forgets is a
// filter that silently stops working.
func TestActionArgsCoverEveryDeclaredProperty(t *testing.T) {
	tests := []struct {
		tool     string
		declared map[string]prop
		byAction map[string][]string
	}{
		{"hookdeck_requests", requestsToolProperties, requestsActionArgs},
		{"hookdeck_events", eventsToolProperties, eventsActionArgs},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			honoured := map[string]bool{"action": true}
			for _, args := range tt.byAction {
				for _, a := range args {
					honoured[a] = true
					_, ok := tt.declared[a]
					require.True(t, ok, "%s: action matrix names %q, which the schema does not declare", tt.tool, a)
				}
			}
			for name := range tt.declared {
				assert.True(t, honoured[name], "%s: schema advertises %q but no action honours it", tt.tool, name)
			}
		})
	}
}

// TestActionArgsIgnoreEmptyValues keeps the guard from firing on an argument the
// caller did not really set: the handlers forward strings and numbers only when
// non-zero, so an empty one was never a dropped filter.
func TestActionArgsIgnoreEmptyValues(t *testing.T) {
	var called bool
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/requests": func(w http.ResponseWriter, r *http.Request) {
			called = true
			_ = json.NewEncoder(w).Encode(listResponse())
		},
	})

	result := callTool(t, session, "hookdeck_requests", map[string]any{
		"action": "list", "delivery_group": "", "limit": 0,
	})

	assert.False(t, result.IsError, textContent(t, result))
	assert.True(t, called, "an empty argument must not block the call")
}

// TestActionArgsIgnoreUndeclaredKeys leaves keys outside the tool's schema
// alone. They are not filters the caller expected to take effect, and MCP
// clients attach their own.
func TestActionArgsIgnoreUndeclaredKeys(t *testing.T) {
	var called bool
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/requests": func(w http.ResponseWriter, r *http.Request) {
			called = true
			_ = json.NewEncoder(w).Encode(listResponse())
		},
	})

	result := callTool(t, session, "hookdeck_requests", map[string]any{
		"action": "list", "_client_trace_id": "abc123",
	})

	assert.False(t, result.IsError, textContent(t, result))
	assert.True(t, called)
}
