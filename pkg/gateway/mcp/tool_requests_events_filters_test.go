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

// The filters GET /events, GET /requests/{id}/events and
// GET /requests/{id}/ignored_events all declare. Carried over from main's
// coverage of the same problem, which it found on the pre-v3.0.0 tool shape.
//
// v3.0.0 moved the request-scoped listings off the singular request tool and
// onto gateway_events, because that is where this filter set already lived —
// see plans/mcp_read_write_tool_split.md. The regression this guards against is
// the one main fixed: the request-scoped route forwarding none of them.
var eventFilterForwarding = []struct {
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

// forwardingCase drives one filter against one route and reports what arrived.
func forwardingCase(t *testing.T, path string, args map[string]any, param string) string {
	t.Helper()
	var saw string
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + path: func(w http.ResponseWriter, r *http.Request) {
			saw = r.URL.Query().Get(param)
			_ = json.NewEncoder(w).Encode(listResponse())
		},
	})
	result := callTool(t, session, eventsToolName, args)
	assert.False(t, result.IsError, textContent(t, result))
	return saw
}

// TestEventsForwardsEveryFilterToTheRequestScopedRoute is the regression main
// fixed, restated for this branch's tool shape: a request-scoped listing used to
// forward nothing, so every filter passed alongside it was dropped in silence
// and the caller got an unfiltered list that read as a filtered one.
func TestEventsForwardsEveryFilterToTheRequestScopedRoute(t *testing.T) {
	for _, tt := range eventFilterForwarding {
		t.Run(tt.arg, func(t *testing.T) {
			saw := forwardingCase(t, "/requests/req_1/events",
				map[string]any{"action": "list", "request_id": "req_1", tt.arg: tt.value}, tt.param)
			assert.Equal(t, tt.want, saw, "%s must reach the API as %s", tt.arg, tt.param)
		})
	}
}

// TestEventsIgnoredForwardsOnlyWhatTheRouteDeclares.
//
// GET /requests/{id}/ignored_events is NOT its sibling: verified against the
// 2026-09-01 OpenAPI document it declares six query parameters, where
// GET /requests/{id}/events declares all thirty of the /events filters. The two
// routes sit beside each other and return the same model, so the asymmetry is
// easy to assume away — and assuming it is how a caller ends up with an
// unfiltered list that reads as a filtered one.
func TestEventsIgnoredForwardsOnlyWhatTheRouteDeclares(t *testing.T) {
	for _, tt := range []struct {
		arg   string
		value any
		param string
		want  string
	}{
		{"order_by", "created_at", "order_by", "created_at"},
		{"dir", "asc", "dir", "asc"},
		{"limit", float64(5), "limit", "5"},
		{"next", "cur_next", "next", "cur_next"},
		{"prev", "cur_prev", "prev", "cur_prev"},
	} {
		t.Run(tt.arg, func(t *testing.T) {
			saw := forwardingCase(t, "/requests/req_1/ignored_events",
				map[string]any{"action": "list_ignored", "request_id": "req_1", tt.arg: tt.value}, tt.param)
			assert.Equal(t, tt.want, saw, "%s must reach the API as %s", tt.arg, tt.param)
		})
	}
}

// TestEventsIgnoredRefusesFiltersTheRouteDrops is the other half: a filter the
// route does not declare must be refused, not forwarded to be ignored. Refusing
// is the house rule — a wrong answer that reads like a right one is the failure
// this package keeps finding.
func TestEventsIgnoredRefusesFiltersTheRouteDrops(t *testing.T) {
	for _, arg := range []string{"status", "source_id", "destination_id", "attempts", "search_term", "created_after"} {
		t.Run(arg, func(t *testing.T) {
			session := mockAPIWithClient(t, map[string]http.HandlerFunc{
				hookdeck.APIPathPrefix + "/requests/req_1/ignored_events": func(w http.ResponseWriter, r *http.Request) {
					t.Fatalf("must not query the route with a filter it does not declare: %s", r.URL.RawQuery)
				},
			})
			value := any("x")
			if arg == "created_after" {
				value = "2025-01-01T00:00:00Z"
			}
			result := callTool(t, session, eventsToolName, map[string]any{
				"action": "list_ignored", "request_id": "req_1", arg: value,
			})
			require.True(t, result.IsError, "%s must be refused on list_ignored", arg)
			body := textContent(t, result)
			assert.Contains(t, body, arg)
			assert.Contains(t, body, "list", "the message should point at the action that does filter")
		})
	}
}

// TestEventsUnscopedListForwardsEveryFilter pins that routing by request_id did
// not cost the collection route any of its filters.
func TestEventsUnscopedListForwardsEveryFilter(t *testing.T) {
	for _, tt := range eventFilterForwarding {
		t.Run(tt.arg, func(t *testing.T) {
			saw := forwardingCase(t, "/events",
				map[string]any{"action": "list", tt.arg: tt.value}, tt.param)
			assert.Equal(t, tt.want, saw, "%s must reach the API as %s", tt.arg, tt.param)
		})
	}
}

// TestEventsRouteSelection is the switch itself: request_id decides which of the
// three routes is queried, and nothing else does.
func TestEventsRouteSelection(t *testing.T) {
	routes := []struct {
		name string
		args map[string]any
		path string
	}{
		{"no request_id queries the collection", map[string]any{"action": "list"}, "/events"},
		{"request_id scopes to the request", map[string]any{"action": "list", "request_id": "req_1"}, "/requests/req_1/events"},
		{"list_ignored scopes to the request", map[string]any{"action": "list_ignored", "request_id": "req_1"}, "/requests/req_1/ignored_events"},
	}
	for _, tt := range routes {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			others := map[string]http.HandlerFunc{}
			for _, p := range []string{"/events", "/requests/req_1/events", "/requests/req_1/ignored_events"} {
				if p == tt.path {
					continue
				}
				others[hookdeck.APIPathPrefix+p] = func(w http.ResponseWriter, r *http.Request) {
					t.Errorf("wrong route queried: %s", r.URL.Path)
				}
			}
			others[hookdeck.APIPathPrefix+tt.path] = func(w http.ResponseWriter, r *http.Request) {
				called = true
				_ = json.NewEncoder(w).Encode(listResponse())
			}
			session := mockAPIWithClient(t, others)
			result := callTool(t, session, eventsToolName, tt.args)
			assert.False(t, result.IsError, textContent(t, result))
			assert.True(t, called, "expected %s to be queried", tt.path)
		})
	}
}

// TestEventsListIgnoredRequiresARequestID: there is no unscoped ignored-events
// collection, so the action cannot fall back to one.
func TestEventsListIgnoredRequiresARequestID(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/events": func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("must not fall back to the events collection")
		},
	})
	result := callTool(t, session, eventsToolName, map[string]any{"action": "list_ignored"})
	require.True(t, result.IsError, "list_ignored without request_id must be refused")
	assert.Contains(t, textContent(t, result), "request_id")
}

// TestEventsStatusIsCanonicalised is main's fix, ported. gateway_events queries
// the same collection the request-scoped routes do, so it has to accept the
// same values: the request-scoped path canonicalised and the collection path
// forwarded the raw string, so `status: "failed"` worked on one and came back a
// 422 from the other — the same enum, reached two ways.
func TestEventsStatusIsCanonicalised(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"canonical spelling is forwarded", "SUCCESSFUL", "SUCCESSFUL"},
		{"lower case is canonicalised", "failed", "FAILED"},
		{"mixed case is canonicalised", "Cancelled", "CANCELLED"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saw := forwardingCase(t, "/events",
				map[string]any{"action": "list", "status": tt.value}, "status")
			assert.Equal(t, tt.want, saw, "status must reach the API in the spelling the enum uses")
		})
	}
}

// TestEventsStatusRejectsTheRequestVocabulary: the request log's words are the
// ones a caller reaches for by mistake, and the API's 422 would only ever name
// the enum it was sent to. The error has to name the tool that does take it.
func TestEventsStatusRejectsTheRequestVocabulary(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/events": func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("must not query the API with a status from the other vocabulary")
		},
	})
	result := callTool(t, session, eventsToolName, map[string]any{"action": "list", "status": "accepted"})
	require.True(t, result.IsError, "a request-log status must be refused here")
	body := textContent(t, result)
	assert.Contains(t, body, "accepted")
	assert.Contains(t, body, requestsToolName, "the message should name the tool that takes it")
}

// TestEventsFilterSetMatchesTheRequestScopedRoute stops the three routes
// drifting. They declare the same query parameters, so one schema serves all
// three; a filter added for the collection and forgotten for the scoped routes
// is how the request-scoped listing ended up forwarding none of them.
func TestEventsFilterSetMatchesTheRequestScopedRoute(t *testing.T) {
	declared := make([]string, 0, len(eventsSpec.Props))
	for name := range eventsSpec.Props {
		declared = append(declared, name)
	}
	sort.Strings(declared)

	for _, tt := range eventFilterForwarding {
		assert.Contains(t, declared, tt.arg,
			"%s is forwarded by the handler but not declared on the schema", tt.arg)
	}
}

// TestRequestIDIsARouteSelectorNotAFilter pins the one argument on this tool
// that is not a filter.
//
// No events route declares request_id as a query parameter — not GET /events,
// not GET /requests/{id}/events, not the ignored sibling. It exists on the tool
// to choose which of them is queried, and putting it on the wire would be the
// failure this tool guards against everywhere else: a parameter the API ignores,
// producing an unfiltered list that reads as a filtered one.
//
// Before v3.0.0 request_id was the canonical example of an argument
// gateway_events did NOT have (see TestUnknownArgumentsAreRejected). It now
// exists and means something different, so the distinction is worth holding
// down rather than leaving to memory.
func TestRequestIDIsARouteSelectorNotAFilter(t *testing.T) {
	t.Run("declared on the schema", func(t *testing.T) {
		_, ok := eventsSpec.Props["request_id"]
		assert.True(t, ok, "request_id must be a declared argument, or callers get an unknown-argument error")
	})

	t.Run("never reaches the wire", func(t *testing.T) {
		for _, tc := range []struct{ action, path string }{
			{"list", "/requests/req_1/events"},
			{"list_ignored", "/requests/req_1/ignored_events"},
		} {
			t.Run(tc.action, func(t *testing.T) {
				var sawQuery string
				session := mockAPIWithClient(t, map[string]http.HandlerFunc{
					hookdeck.APIPathPrefix + tc.path: func(w http.ResponseWriter, r *http.Request) {
						sawQuery = r.URL.RawQuery
						_ = json.NewEncoder(w).Encode(listResponse())
					},
				})
				result := callTool(t, session, eventsToolName, map[string]any{
					"action": tc.action, "request_id": "req_1",
				})
				assert.False(t, result.IsError, textContent(t, result))
				assert.NotContains(t, sawQuery, "request_id",
					"request_id selects the route; the API declares no such filter, so sending it would be ignored")
			})
		}
	})

	t.Run("the unscoped route is unaffected", func(t *testing.T) {
		var sawQuery string
		session := mockAPIWithClient(t, map[string]http.HandlerFunc{
			hookdeck.APIPathPrefix + "/events": func(w http.ResponseWriter, r *http.Request) {
				sawQuery = r.URL.RawQuery
				_ = json.NewEncoder(w).Encode(listResponse())
			},
		})
		result := callTool(t, session, eventsToolName, map[string]any{"action": "list", "source_id": "src_1"})
		assert.False(t, result.IsError, textContent(t, result))
		assert.Contains(t, sawQuery, "source_id=src_1")
		assert.NotContains(t, sawQuery, "request_id")
	})
}

// TestRequestsStatusIsCanonicalised is the counterpart to
// TestEventsStatusIsCanonicalised, and exists because this half was dropped
// once already.
//
// main shipped canonicalisation for both vocabularies. The v2.6.0 merge carried
// the events half across and lost the requests half, so
// gateway_requests{status:"ACCEPTED"} — which worked in v2.6.0 — started 422ing
// against an enum the API spells in lower case. A verification sweep caught it.
// Both halves are pinned here so the asymmetry cannot come back a third time.
func TestRequestsStatusIsCanonicalised(t *testing.T) {
	for _, tt := range []struct{ name, value, want string }{
		{"canonical spelling is forwarded", "accepted", "accepted"},
		{"upper case is canonicalised", "ACCEPTED", "accepted"},
		{"mixed case is canonicalised", "Rejected", "rejected"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var saw string
			session := mockAPIWithClient(t, map[string]http.HandlerFunc{
				hookdeck.APIPathPrefix + "/requests": func(w http.ResponseWriter, r *http.Request) {
					saw = r.URL.Query().Get("status")
					_ = json.NewEncoder(w).Encode(listResponse())
				},
			})
			result := callTool(t, session, requestsToolName, map[string]any{
				"action": "list", "status": tt.value,
			})
			assert.False(t, result.IsError, textContent(t, result))
			assert.Equal(t, tt.want, saw, "status must reach the API in the spelling the enum uses")
		})
	}
}

// TestRequestsStatusRejectsTheEventVocabulary: a lifecycle status sent to the
// request log is refused with the tool that does take it, rather than the API's
// 422 naming only the enum it was sent to.
func TestRequestsStatusRejectsTheEventVocabulary(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/requests": func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("must not query the API with a status from the other vocabulary")
		},
	})
	result := callTool(t, session, requestsToolName, map[string]any{"action": "list", "status": "SUCCESSFUL"})
	require.True(t, result.IsError, "an event status must be refused here")
	body := textContent(t, result)
	assert.Contains(t, body, "SUCCESSFUL")
	assert.Contains(t, body, eventsToolName, "the message should name the tool that takes it")
}
