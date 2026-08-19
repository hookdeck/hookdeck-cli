package mcp

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every action of every Event Gateway tool, called successfully at least once.
//
// Most write actions previously had only their read-only refusal covered, so
// the first successful invocation of a delete or an upsert would have happened
// in a user's project. The reads no test called at all are here for the same
// reason.
//
// These assert the request that goes on the wire — method, path, query and body
// — rather than only that the call did not error. A "no error" assertion proves
// almost nothing here, because the stub server answers whatever it is asked: a
// handler sending PUT to the wrong path would still pass.

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// wireRequest records the request a handler actually sent.
type wireRequest struct {
	method string
	path   string
	query  string
	body   []byte
}

// record captures the incoming request and replies with response. A nil
// response writes no body, which is what the delete endpoints do.
func record(into *wireRequest, status int, response any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		*into = wireRequest{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, body: body}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if response != nil {
			_ = json.NewEncoder(w).Encode(response)
		}
	}
}

// ok is record with a 200 status, which is what these endpoints return.
func ok(into *wireRequest, response any) http.HandlerFunc {
	return record(into, http.StatusOK, response)
}

// decodeBody unmarshals the captured request body as a JSON object.
func (w wireRequest) decodeBody(t *testing.T) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.body, &body), "request body was not a JSON object: %s", w.body)
	return body
}

// envelopeData returns the data half of the standard result envelope.
func envelopeData(t *testing.T, text string) json.RawMessage {
	t.Helper()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(text), &envelope), "result was not an envelope: %s", text)
	return envelope.Data
}

// writeSession connects a write-enabled session to a stub API.
func writeSession(t *testing.T, handlers map[string]http.HandlerFunc) *mcpsdk.ClientSession {
	t.Helper()
	api := mockAPI(t, handlers)
	return connectInMemoryWriteEnabled(t, newTestClient(api.URL, "test-key"))
}

// readSession connects a read-only session to a stub API.
func readSession(t *testing.T, handlers map[string]http.HandlerFunc) *mcpsdk.ClientSession {
	t.Helper()
	api := mockAPI(t, handlers)
	return connectInMemory(t, newTestClient(api.URL, "test-key"))
}

// succeeds calls a tool and fails the test if the result is an error.
func succeeds(t *testing.T, session *mcpsdk.ClientSession, tool string, args map[string]any) string {
	t.Helper()
	result := callTool(t, session, tool, args)
	require.False(t, result.IsError, "unexpected error: %s", textContent(t, result))
	return textContent(t, result)
}

// connectionBody is a minimal connection as the API returns it.
func connectionBody() map[string]any {
	return map[string]any{"id": "web_1", "name": "stripe-to-backend"}
}

// ---------------------------------------------------------------------------
// gateway_connections
// ---------------------------------------------------------------------------

func TestConnectionsListSendsFilters(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/connections": ok(&got, listResponse(connectionBody())),
	})

	text := succeeds(t, session, "gateway_connections", map[string]any{
		"action": "list", "name": "stripe-to-backend", "source_id": "src_1",
		"destination_id": "des_1", "limit": 25, "next": "cursor_1",
	})

	assert.Equal(t, "/2025-07-01/connections", got.path)
	assert.Contains(t, got.query, "name=stripe-to-backend")
	assert.Contains(t, got.query, "source_id=src_1")
	assert.Contains(t, got.query, "destination_id=des_1")
	assert.Contains(t, got.query, "limit=25")
	// Pagination is only usable if the cursor is forwarded.
	assert.Contains(t, got.query, "next=cursor_1")
	assert.Contains(t, string(envelopeData(t, text)), "web_1")
}

func TestConnectionsGetByID(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/connections/web_1": ok(&got, connectionBody()),
	})

	text := succeeds(t, session, "gateway_connections", map[string]any{"action": "get", "id": "web_1"})

	assert.Equal(t, http.MethodGet, got.method)
	assert.Equal(t, "/2025-07-01/connections/web_1", got.path)
	assert.Contains(t, string(envelopeData(t, text)), "stripe-to-backend")
}

func TestConnectionsCreate(t *testing.T) {
	var got wireRequest
	session := writeSession(t, map[string]http.HandlerFunc{
		"POST /2025-07-01/connections": ok(&got, connectionBody()),
	})

	succeeds(t, session, "gateway_connections", map[string]any{
		"action":         "create",
		"name":           "stripe-to-backend",
		"description":    "routes Stripe events",
		"source_id":      "src_1",
		"destination_id": "des_1",
		"rules":          []any{map[string]any{"type": "retry", "count": 3}},
	})

	assert.Equal(t, http.MethodPost, got.method)
	assert.Equal(t, "/2025-07-01/connections", got.path)

	body := got.decodeBody(t)
	assert.Equal(t, "stripe-to-backend", body["name"])
	assert.Equal(t, "routes Stripe events", body["description"])
	assert.Equal(t, "src_1", body["source_id"])
	assert.Equal(t, "des_1", body["destination_id"])
	// The ruleset is an ordered array of rule objects; flattening it would
	// silently change the connection's behaviour.
	assert.Equal(t, []any{map[string]any{"type": "retry", "count": float64(3)}}, body["rules"])
}

// Upsert keys on the name, so it goes to the collection rather than to an id,
// and a POST here would create a duplicate on every call.
func TestConnectionsUpsertIsAPutToTheCollection(t *testing.T) {
	var got wireRequest
	session := writeSession(t, map[string]http.HandlerFunc{
		"PUT /2025-07-01/connections": ok(&got, connectionBody()),
	})

	succeeds(t, session, "gateway_connections", map[string]any{
		"action": "upsert", "name": "stripe-to-backend", "source_id": "src_1", "destination_id": "des_1",
	})

	assert.Equal(t, http.MethodPut, got.method)
	assert.Equal(t, "/2025-07-01/connections", got.path)
	assert.Equal(t, "stripe-to-backend", got.decodeBody(t)["name"])
}

func TestConnectionsUpsertRequiresAName(t *testing.T) {
	session := writeSession(t, nil)

	result := callTool(t, session, "gateway_connections", map[string]any{
		"action": "upsert", "source_id": "src_1",
	})
	require.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "name is required")
}

func TestConnectionsUpdate(t *testing.T) {
	var resolve, got wireRequest
	session := writeSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/connections/web_1": ok(&resolve, connectionBody()),
		"PUT /2025-07-01/connections/web_1": ok(&got, connectionBody()),
	})

	succeeds(t, session, "gateway_connections", map[string]any{
		"action": "update", "id": "web_1", "description": "now with retries",
		"rules": []any{map[string]any{"type": "retry"}},
	})

	assert.Equal(t, http.MethodPut, got.method)
	assert.Equal(t, "/2025-07-01/connections/web_1", got.path)

	body := got.decodeBody(t)
	assert.Equal(t, "now with retries", body["description"])
	assert.Equal(t, []any{map[string]any{"type": "retry"}}, body["rules"])
	// Fields the caller did not mention are omitted rather than sent empty,
	// which would blank them on the stored connection.
	assert.NotContains(t, body, "name")
	assert.NotContains(t, body, "source_id")
}

// The id argument accepts a name, so the write actions resolve it before
// addressing the connection. Sending the name in the path would 404.
func TestConnectionsUpdateResolvesANameToAnID(t *testing.T) {
	var lookup, got wireRequest
	session := writeSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/connections":       ok(&lookup, listResponse(connectionBody())),
		"PUT /2025-07-01/connections/web_1": ok(&got, connectionBody()),
	})

	succeeds(t, session, "gateway_connections", map[string]any{
		"action": "update", "id": "stripe-to-backend", "description": "renamed target",
	})

	assert.Equal(t, "name=stripe-to-backend", lookup.query)
	assert.Equal(t, "/2025-07-01/connections/web_1", got.path)
}

func TestConnectionsDelete(t *testing.T) {
	var resolve, got wireRequest
	session := writeSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/connections/web_1":    ok(&resolve, connectionBody()),
		"DELETE /2025-07-01/connections/web_1": ok(&got, nil),
	})

	text := succeeds(t, session, "gateway_connections", map[string]any{"action": "delete", "id": "web_1"})

	assert.Equal(t, http.MethodDelete, got.method)
	assert.Equal(t, "/2025-07-01/connections/web_1", got.path)
	// The API returns no useful body, so the tool has to say what happened.
	assert.JSONEq(t, `{"connection_id":"web_1","status":"deleted"}`, string(envelopeData(t, text)))
}

func TestConnectionsEnableAndDisable(t *testing.T) {
	for _, action := range []string{"enable", "disable"} {
		t.Run(action, func(t *testing.T) {
			var resolve, got wireRequest
			session := writeSession(t, map[string]http.HandlerFunc{
				"GET /2025-07-01/connections/web_1":           ok(&resolve, connectionBody()),
				"PUT /2025-07-01/connections/web_1/" + action: ok(&got, connectionBody()),
			})

			succeeds(t, session, "gateway_connections", map[string]any{"action": action, "id": "web_1"})

			assert.Equal(t, http.MethodPut, got.method)
			assert.Equal(t, "/2025-07-01/connections/web_1/"+action, got.path)
		})
	}
}

func TestConnectionsPauseAndUnpause(t *testing.T) {
	for _, action := range []string{"pause", "unpause"} {
		t.Run(action, func(t *testing.T) {
			var resolve, got wireRequest
			session := readSession(t, map[string]http.HandlerFunc{
				"GET /2025-07-01/connections/web_1":           ok(&resolve, connectionBody()),
				"PUT /2025-07-01/connections/web_1/" + action: ok(&got, connectionBody()),
			})

			succeeds(t, session, "gateway_connections", map[string]any{"action": action, "id": "web_1"})

			assert.Equal(t, http.MethodPut, got.method)
			assert.Equal(t, "/2025-07-01/connections/web_1/"+action, got.path)
		})
	}
}

func TestConnectionsWriteActionsRequireAnID(t *testing.T) {
	session := writeSession(t, nil)

	for _, action := range []string{"update", "delete", "enable", "disable"} {
		t.Run(action, func(t *testing.T) {
			result := callTool(t, session, "gateway_connections", map[string]any{"action": action})
			require.True(t, result.IsError)
			assert.Contains(t, textContent(t, result), "id or name is required")
		})
	}
}

// ---------------------------------------------------------------------------
// gateway_sources
// ---------------------------------------------------------------------------

func sourceBody() map[string]any {
	return map[string]any{"id": "src_1", "name": "stripe", "type": "STRIPE"}
}

func TestSourcesListSendsFilters(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/sources": ok(&got, listResponse(sourceBody())),
	})

	succeeds(t, session, "gateway_sources", map[string]any{
		"action": "list", "name": "stripe", "limit": 5, "next": "cursor_1",
	})

	assert.Equal(t, "/2025-07-01/sources", got.path)
	assert.Contains(t, got.query, "name=stripe")
	assert.Contains(t, got.query, "limit=5")
	assert.Contains(t, got.query, "next=cursor_1")
}

func TestSourcesGetByID(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/sources/src_1": ok(&got, sourceBody()),
	})

	succeeds(t, session, "gateway_sources", map[string]any{"action": "get", "id": "src_1"})
	assert.Equal(t, "/2025-07-01/sources/src_1", got.path)
}

func TestSourcesCreateAndUpsert(t *testing.T) {
	cases := []struct {
		action  string
		pattern string
		method  string
	}{
		// Create posts to the collection; upsert keys on the name and PUTs to
		// the same collection, so the two differ only in method.
		{"create", "POST /2025-07-01/sources", http.MethodPost},
		{"upsert", "PUT /2025-07-01/sources", http.MethodPut},
	}

	for _, tc := range cases {
		t.Run(tc.action, func(t *testing.T) {
			var got wireRequest
			session := writeSession(t, map[string]http.HandlerFunc{
				tc.pattern: ok(&got, sourceBody()),
			})

			succeeds(t, session, "gateway_sources", map[string]any{
				"action": tc.action, "name": "stripe", "type": "STRIPE",
				"description": "Stripe webhooks",
				"config":      map[string]any{"auth_type": "STRIPE_SIGNATURE"},
			})

			assert.Equal(t, tc.method, got.method)
			assert.Equal(t, "/2025-07-01/sources", got.path)

			body := got.decodeBody(t)
			assert.Equal(t, "stripe", body["name"])
			assert.Equal(t, "STRIPE", body["type"])
			assert.Equal(t, "Stripe webhooks", body["description"])
			assert.Equal(t, map[string]any{"auth_type": "STRIPE_SIGNATURE"}, body["config"])
		})
	}
}

func TestSourcesUpdate(t *testing.T) {
	var got wireRequest
	session := writeSession(t, map[string]http.HandlerFunc{
		"PUT /2025-07-01/sources/src_1": ok(&got, sourceBody()),
	})

	succeeds(t, session, "gateway_sources", map[string]any{
		"action": "update", "id": "src_1",
		"config": map[string]any{"allowed_http_methods": []any{"POST"}},
	})

	assert.Equal(t, http.MethodPut, got.method)
	assert.Equal(t, "/2025-07-01/sources/src_1", got.path)

	body := got.decodeBody(t)
	assert.Equal(t, map[string]any{"allowed_http_methods": []any{"POST"}}, body["config"])
	// An update that resent empty values would rename the source to "".
	assert.NotContains(t, body, "name")
	assert.NotContains(t, body, "type")
}

func TestSourcesDelete(t *testing.T) {
	var got wireRequest
	session := writeSession(t, map[string]http.HandlerFunc{
		"DELETE /2025-07-01/sources/src_1": ok(&got, nil),
	})

	text := succeeds(t, session, "gateway_sources", map[string]any{"action": "delete", "id": "src_1"})

	assert.Equal(t, http.MethodDelete, got.method)
	assert.Equal(t, "/2025-07-01/sources/src_1", got.path)
	assert.JSONEq(t, `{"source_id":"src_1","status":"deleted"}`, string(envelopeData(t, text)))
}

func TestSourcesEnableAndDisable(t *testing.T) {
	for _, action := range []string{"enable", "disable"} {
		t.Run(action, func(t *testing.T) {
			var got wireRequest
			session := writeSession(t, map[string]http.HandlerFunc{
				"PUT /2025-07-01/sources/src_1/" + action: ok(&got, sourceBody()),
			})

			succeeds(t, session, "gateway_sources", map[string]any{"action": action, "id": "src_1"})

			assert.Equal(t, http.MethodPut, got.method)
			assert.Equal(t, "/2025-07-01/sources/src_1/"+action, got.path)
		})
	}
}

func TestSourcesWriteActionsRequireAnID(t *testing.T) {
	session := writeSession(t, nil)

	for _, action := range []string{"update", "delete", "enable", "disable"} {
		t.Run(action, func(t *testing.T) {
			result := callTool(t, session, "gateway_sources", map[string]any{"action": action})
			require.True(t, result.IsError)
			assert.Contains(t, textContent(t, result), "id is required")
		})
	}
}

// ---------------------------------------------------------------------------
// gateway_destinations
// ---------------------------------------------------------------------------

func destinationBody() map[string]any {
	return map[string]any{"id": "des_1", "name": "backend", "type": "HTTP"}
}

func TestDestinationsListSendsFilters(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/destinations": ok(&got, listResponse(destinationBody())),
	})

	succeeds(t, session, "gateway_destinations", map[string]any{
		"action": "list", "name": "backend", "limit": 5, "prev": "cursor_0",
	})

	assert.Equal(t, "/2025-07-01/destinations", got.path)
	assert.Contains(t, got.query, "name=backend")
	assert.Contains(t, got.query, "limit=5")
	assert.Contains(t, got.query, "prev=cursor_0")
}

func TestDestinationsGetByID(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/destinations/des_1": ok(&got, destinationBody()),
	})

	succeeds(t, session, "gateway_destinations", map[string]any{"action": "get", "id": "des_1"})
	assert.Equal(t, "/2025-07-01/destinations/des_1", got.path)
}

func TestDestinationsCreateAndUpsert(t *testing.T) {
	cases := []struct {
		action  string
		pattern string
		method  string
	}{
		{"create", "POST /2025-07-01/destinations", http.MethodPost},
		{"upsert", "PUT /2025-07-01/destinations", http.MethodPut},
	}

	for _, tc := range cases {
		t.Run(tc.action, func(t *testing.T) {
			var got wireRequest
			session := writeSession(t, map[string]http.HandlerFunc{
				tc.pattern: ok(&got, destinationBody()),
			})

			succeeds(t, session, "gateway_destinations", map[string]any{
				"action": tc.action, "name": "backend", "type": "HTTP",
				"description": "the API",
				"config":      map[string]any{"url": "https://example.com/hooks"},
			})

			assert.Equal(t, tc.method, got.method)
			assert.Equal(t, "/2025-07-01/destinations", got.path)

			body := got.decodeBody(t)
			assert.Equal(t, "backend", body["name"])
			assert.Equal(t, "HTTP", body["type"])
			assert.Equal(t, "the API", body["description"])
			// The URL is the whole point of an HTTP destination.
			assert.Equal(t, map[string]any{"url": "https://example.com/hooks"}, body["config"])
		})
	}
}

func TestDestinationsUpdate(t *testing.T) {
	var got wireRequest
	session := writeSession(t, map[string]http.HandlerFunc{
		"PUT /2025-07-01/destinations/des_1": ok(&got, destinationBody()),
	})

	succeeds(t, session, "gateway_destinations", map[string]any{
		"action": "update", "id": "des_1",
		"config": map[string]any{"url": "https://example.com/new"},
	})

	assert.Equal(t, http.MethodPut, got.method)
	assert.Equal(t, "/2025-07-01/destinations/des_1", got.path)

	body := got.decodeBody(t)
	assert.Equal(t, map[string]any{"url": "https://example.com/new"}, body["config"])
	assert.NotContains(t, body, "name")
	assert.NotContains(t, body, "type")
}

func TestDestinationsDelete(t *testing.T) {
	var got wireRequest
	session := writeSession(t, map[string]http.HandlerFunc{
		"DELETE /2025-07-01/destinations/des_1": ok(&got, nil),
	})

	text := succeeds(t, session, "gateway_destinations", map[string]any{"action": "delete", "id": "des_1"})

	assert.Equal(t, http.MethodDelete, got.method)
	assert.Equal(t, "/2025-07-01/destinations/des_1", got.path)
	assert.JSONEq(t, `{"destination_id":"des_1","status":"deleted"}`, string(envelopeData(t, text)))
}

func TestDestinationsEnableAndDisable(t *testing.T) {
	for _, action := range []string{"enable", "disable"} {
		t.Run(action, func(t *testing.T) {
			var got wireRequest
			session := writeSession(t, map[string]http.HandlerFunc{
				"PUT /2025-07-01/destinations/des_1/" + action: ok(&got, destinationBody()),
			})

			succeeds(t, session, "gateway_destinations", map[string]any{"action": action, "id": "des_1"})

			assert.Equal(t, http.MethodPut, got.method)
			assert.Equal(t, "/2025-07-01/destinations/des_1/"+action, got.path)
		})
	}
}

func TestDestinationsWriteActionsRequireAnID(t *testing.T) {
	session := writeSession(t, nil)

	for _, action := range []string{"update", "delete", "enable", "disable"} {
		t.Run(action, func(t *testing.T) {
			result := callTool(t, session, "gateway_destinations", map[string]any{"action": action})
			require.True(t, result.IsError)
			assert.Contains(t, textContent(t, result), "id is required")
		})
	}
}

// ---------------------------------------------------------------------------
// gateway_transformations
// ---------------------------------------------------------------------------

func transformationBody() map[string]any {
	return map[string]any{"id": "trs_1", "name": "enrich", "code": "return request"}
}

func TestTransformationsListSendsFilters(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/transformations": ok(&got, listResponse(transformationBody())),
	})

	succeeds(t, session, "gateway_transformations", map[string]any{
		"action": "list", "name": "enrich", "limit": 5,
	})

	assert.Equal(t, "/2025-07-01/transformations", got.path)
	assert.Contains(t, got.query, "name=enrich")
	assert.Contains(t, got.query, "limit=5")
}

func TestTransformationsGetByID(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/transformations/trs_1": ok(&got, transformationBody()),
	})

	text := succeeds(t, session, "gateway_transformations", map[string]any{"action": "get", "id": "trs_1"})
	assert.Equal(t, "/2025-07-01/transformations/trs_1", got.path)
	// The code is the reason to fetch one.
	assert.Contains(t, string(envelopeData(t, text)), "return request")
}

func TestTransformationsCreateAndUpsert(t *testing.T) {
	cases := []struct {
		action  string
		pattern string
		method  string
	}{
		{"create", "POST /2025-07-01/transformations", http.MethodPost},
		{"upsert", "PUT /2025-07-01/transformations", http.MethodPut},
	}

	for _, tc := range cases {
		t.Run(tc.action, func(t *testing.T) {
			var got wireRequest
			session := writeSession(t, map[string]http.HandlerFunc{
				tc.pattern: ok(&got, transformationBody()),
			})

			succeeds(t, session, "gateway_transformations", map[string]any{
				"action": tc.action, "name": "enrich", "code": "return request",
				"env": map[string]any{"API_KEY": "shh"},
			})

			assert.Equal(t, tc.method, got.method)
			assert.Equal(t, "/2025-07-01/transformations", got.path)

			body := got.decodeBody(t)
			assert.Equal(t, "enrich", body["name"])
			assert.Equal(t, "return request", body["code"])
			assert.Equal(t, map[string]any{"API_KEY": "shh"}, body["env"])
		})
	}
}

func TestTransformationsUpdate(t *testing.T) {
	var got wireRequest
	session := writeSession(t, map[string]http.HandlerFunc{
		"PUT /2025-07-01/transformations/trs_1": ok(&got, transformationBody()),
	})

	succeeds(t, session, "gateway_transformations", map[string]any{
		"action": "update", "id": "trs_1", "code": "return { ...request }",
	})

	assert.Equal(t, http.MethodPut, got.method)
	assert.Equal(t, "/2025-07-01/transformations/trs_1", got.path)

	body := got.decodeBody(t)
	assert.Equal(t, "return { ...request }", body["code"])
	// Sending an empty name would rename the transformation to "".
	assert.NotContains(t, body, "name")
	assert.NotContains(t, body, "env")
}

func TestTransformationsDelete(t *testing.T) {
	var got wireRequest
	session := writeSession(t, map[string]http.HandlerFunc{
		"DELETE /2025-07-01/transformations/trs_1": ok(&got, nil),
	})

	text := succeeds(t, session, "gateway_transformations", map[string]any{"action": "delete", "id": "trs_1"})

	assert.Equal(t, http.MethodDelete, got.method)
	assert.Equal(t, "/2025-07-01/transformations/trs_1", got.path)
	assert.JSONEq(t, `{"transformation_id":"trs_1","status":"deleted"}`, string(envelopeData(t, text)))
}

func TestTransformationsRunSendsTheSampleRequest(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"PUT /2025-07-01/transformations/run": ok(&got, map[string]any{
			"request": map[string]any{"headers": map[string]any{}, "body": map[string]any{"ok": true}},
		}),
	})

	succeeds(t, session, "gateway_transformations", map[string]any{
		"action": "run", "code": "return request", "connection_id": "web_1",
		"env":     map[string]any{"API_KEY": "shh"},
		"request": map[string]any{"headers": map[string]any{"x-test": "1"}, "body": map[string]any{"id": 7}, "path": "/hooks"},
	})

	assert.Equal(t, http.MethodPut, got.method)
	assert.Equal(t, "/2025-07-01/transformations/run", got.path)

	body := got.decodeBody(t)
	assert.Equal(t, "return request", body["code"])
	// connection_id is the MCP name for the API's webhook_id.
	assert.Equal(t, "web_1", body["webhook_id"])

	request, isObject := body["request"].(map[string]any)
	require.True(t, isObject, "request must be sent as an object: %v", body["request"])
	assert.Equal(t, map[string]any{"x-test": "1"}, request["headers"])
	assert.Equal(t, map[string]any{"id": float64(7)}, request["body"])
	assert.Equal(t, "/hooks", request["path"])
}

func TestTransformationsWriteActionsRequireAnID(t *testing.T) {
	session := writeSession(t, nil)

	for _, action := range []string{"update", "delete"} {
		t.Run(action, func(t *testing.T) {
			result := callTool(t, session, "gateway_transformations", map[string]any{"action": action})
			require.True(t, result.IsError)
			assert.Contains(t, textContent(t, result), "id is required")
		})
	}
}

// ---------------------------------------------------------------------------
// gateway_events
// ---------------------------------------------------------------------------

func TestEventsListSendsFilters(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/events": ok(&got, listResponse(map[string]any{"id": "evt_1"})),
	})

	succeeds(t, session, "gateway_events", map[string]any{
		"action": "list", "connection_id": "web_1", "source_id": "src_1",
		"destination_id": "des_1", "status": "FAILED", "limit": 5,
	})

	assert.Equal(t, "/2025-07-01/events", got.path)
	// connection_id is the MCP name for the API's webhook_id.
	assert.Contains(t, got.query, "webhook_id=web_1")
	assert.Contains(t, got.query, "source_id=src_1")
	assert.Contains(t, got.query, "destination_id=des_1")
	assert.Contains(t, got.query, "status=FAILED")
}

func TestEventsGetByID(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/events/evt_1": ok(&got, map[string]any{"id": "evt_1", "status": "FAILED"}),
	})

	succeeds(t, session, "gateway_events", map[string]any{"action": "get", "id": "evt_1"})
	assert.Equal(t, "/2025-07-01/events/evt_1", got.path)
}

func TestEventsRawBody(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/events/evt_1/raw_body": func(w http.ResponseWriter, r *http.Request) {
			got = wireRequest{method: r.Method, path: r.URL.Path}
			_, _ = w.Write([]byte(`{"amount":100}`))
		},
	})

	text := succeeds(t, session, "gateway_events", map[string]any{"action": "raw_body", "id": "evt_1"})

	assert.Equal(t, "/2025-07-01/events/evt_1/raw_body", got.path)
	assert.Contains(t, string(envelopeData(t, text)), "amount")
}

// The three by-id event mutations differ in method and path, and the API
// returns no body, so the tool reports the outcome itself.
func TestEventsRetryCancelAndMute(t *testing.T) {
	cases := []struct {
		action  string
		pattern string
		method  string
		status  string
	}{
		{"retry", "POST /2025-07-01/events/evt_1/retry", http.MethodPost, "retried"},
		{"cancel", "PUT /2025-07-01/events/evt_1/cancel", http.MethodPut, "cancelled"},
		{"mute", "PUT /2025-07-01/events/evt_1/mute", http.MethodPut, "muted"},
	}

	for _, tc := range cases {
		t.Run(tc.action, func(t *testing.T) {
			var got wireRequest
			session := writeSession(t, map[string]http.HandlerFunc{
				tc.pattern: ok(&got, nil),
			})

			text := succeeds(t, session, "gateway_events", map[string]any{"action": tc.action, "id": "evt_1"})

			assert.Equal(t, tc.method, got.method)
			assert.Equal(t, "/2025-07-01/events/evt_1/"+tc.action, got.path)
			assert.JSONEq(t, `{"event_id":"evt_1","status":"`+tc.status+`"}`, string(envelopeData(t, text)))
		})
	}
}

// ---------------------------------------------------------------------------
// gateway_requests
// ---------------------------------------------------------------------------

func TestRequestsListSendsFilters(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/requests": ok(&got, listResponse(map[string]any{"id": "req_1"})),
	})

	succeeds(t, session, "gateway_requests", map[string]any{
		"action": "list", "source_id": "src_1", "status": "accepted", "limit": 5,
	})

	assert.Equal(t, "/2025-07-01/requests", got.path)
	assert.Contains(t, got.query, "source_id=src_1")
	assert.Contains(t, got.query, "status=accepted")
}

func TestRequestsGetByID(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/requests/req_1": ok(&got, map[string]any{"id": "req_1"}),
	})

	succeeds(t, session, "gateway_requests", map[string]any{"action": "get", "id": "req_1"})
	assert.Equal(t, "/2025-07-01/requests/req_1", got.path)
}

func TestRequestsRawBody(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/requests/req_1/raw_body": func(w http.ResponseWriter, r *http.Request) {
			got = wireRequest{method: r.Method, path: r.URL.Path}
			_, _ = w.Write([]byte(`{"amount":100}`))
		},
	})

	text := succeeds(t, session, "gateway_requests", map[string]any{"action": "raw_body", "id": "req_1"})

	assert.Equal(t, "/2025-07-01/requests/req_1/raw_body", got.path)
	assert.Contains(t, string(envelopeData(t, text)), "amount")
}

func TestRequestsEventsAndIgnoredEvents(t *testing.T) {
	for _, action := range []string{"events", "ignored_events"} {
		t.Run(action, func(t *testing.T) {
			var got wireRequest
			session := readSession(t, map[string]http.HandlerFunc{
				"GET /2025-07-01/requests/req_1/" + action: ok(&got, listResponse(map[string]any{"id": "evt_1"})),
			})

			succeeds(t, session, "gateway_requests", map[string]any{"action": action, "id": "req_1"})
			assert.Equal(t, "/2025-07-01/requests/req_1/"+action, got.path)
		})
	}
}

func TestRequestsRetrySendsSelectedConnections(t *testing.T) {
	var got wireRequest
	session := writeSession(t, map[string]http.HandlerFunc{
		"POST /2025-07-01/requests/req_1/retry": ok(&got, nil),
	})

	text := succeeds(t, session, "gateway_requests", map[string]any{
		"action": "retry", "id": "req_1", "connection_ids": []any{"web_1", "web_2"},
	})

	assert.Equal(t, http.MethodPost, got.method)
	assert.Equal(t, "/2025-07-01/requests/req_1/retry", got.path)
	// connection_ids is the MCP name for the API's webhook_ids; dropping it
	// would retry every connection instead of the two asked for.
	assert.Equal(t, []any{"web_1", "web_2"}, got.decodeBody(t)["webhook_ids"])
	assert.JSONEq(t, `{"request_id":"req_1","status":"retried"}`, string(envelopeData(t, text)))
}

// Omitting connection_ids must send an empty body, which the API reads as
// "every connection the request matched".
func TestRequestsRetryWithoutConnectionsSendsNoIDs(t *testing.T) {
	var got wireRequest
	session := writeSession(t, map[string]http.HandlerFunc{
		"POST /2025-07-01/requests/req_1/retry": ok(&got, nil),
	})

	succeeds(t, session, "gateway_requests", map[string]any{"action": "retry", "id": "req_1"})
	assert.NotContains(t, got.decodeBody(t), "webhook_ids")
}

// ---------------------------------------------------------------------------
// gateway_attempts
// ---------------------------------------------------------------------------

func TestAttemptsListSendsFilters(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/attempts": ok(&got, listResponse(map[string]any{"id": "atm_1"})),
	})

	succeeds(t, session, "gateway_attempts", map[string]any{
		"action": "list", "event_id": "evt_1", "limit": 5, "order_by": "created_at", "dir": "desc",
	})

	assert.Equal(t, "/2025-07-01/attempts", got.path)
	assert.Contains(t, got.query, "event_id=evt_1")
	assert.Contains(t, got.query, "order_by=created_at")
	assert.Contains(t, got.query, "dir=desc")
}

func TestAttemptsGetByID(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/attempts/atm_1": ok(&got, map[string]any{
			"id": "atm_1", "response_status": 500, "body": "boom",
		}),
	})

	text := succeeds(t, session, "gateway_attempts", map[string]any{"action": "get", "id": "atm_1"})

	assert.Equal(t, "/2025-07-01/attempts/atm_1", got.path)
	assert.Contains(t, string(envelopeData(t, text)), "atm_1")
}

// ---------------------------------------------------------------------------
// gateway_issues
// ---------------------------------------------------------------------------

func TestIssuesListSendsFilters(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/issues": ok(&got, listResponse(map[string]any{"id": "iss_1"})),
	})

	succeeds(t, session, "gateway_issues", map[string]any{
		"action": "list", "type": "delivery", "filter_status": "OPENED",
		"issue_trigger_id": "ist_1", "limit": 5,
	})

	assert.Equal(t, "/2025-07-01/issues", got.path)
	assert.Contains(t, got.query, "type=delivery")
	// filter_status is the MCP name for the list filter, so that `status`
	// stays free for the update action's new value.
	assert.Contains(t, got.query, "status=OPENED")
	assert.Contains(t, got.query, "issue_trigger_id=ist_1")
}

func TestIssuesGetByID(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/issues/iss_1": ok(&got, map[string]any{"id": "iss_1", "status": "OPENED"}),
	})

	succeeds(t, session, "gateway_issues", map[string]any{"action": "get", "id": "iss_1"})
	assert.Equal(t, "/2025-07-01/issues/iss_1", got.path)
}

func TestIssuesUpdateSendsTheNewStatus(t *testing.T) {
	var got wireRequest
	session := writeSession(t, map[string]http.HandlerFunc{
		"PUT /2025-07-01/issues/iss_1": ok(&got, map[string]any{"id": "iss_1", "status": "RESOLVED"}),
	})

	succeeds(t, session, "gateway_issues", map[string]any{
		"action": "update", "id": "iss_1", "status": "RESOLVED",
	})

	assert.Equal(t, http.MethodPut, got.method)
	assert.Equal(t, "/2025-07-01/issues/iss_1", got.path)
	assert.Equal(t, "RESOLVED", got.decodeBody(t)["status"])
}

func TestIssuesUpdateRequiresAStatus(t *testing.T) {
	session := writeSession(t, nil)

	result := callTool(t, session, "gateway_issues", map[string]any{"action": "update", "id": "iss_1"})
	require.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "status is required")
}

// Dismiss is a DELETE, and unlike the other deletes the API answers with the
// updated issue, which the tool returns rather than a synthesised status.
func TestIssuesDismissIsADelete(t *testing.T) {
	var got wireRequest
	session := writeSession(t, map[string]http.HandlerFunc{
		"DELETE /2025-07-01/issues/iss_1": ok(&got, map[string]any{"id": "iss_1", "status": "IGNORED"}),
	})

	text := succeeds(t, session, "gateway_issues", map[string]any{"action": "dismiss", "id": "iss_1"})

	assert.Equal(t, http.MethodDelete, got.method)
	assert.Equal(t, "/2025-07-01/issues/iss_1", got.path)
	assert.Contains(t, string(envelopeData(t, text)), "IGNORED")
}

// ---------------------------------------------------------------------------
// gateway_metrics
// ---------------------------------------------------------------------------

func TestMetricsActionsHitTheirOwnEndpoints(t *testing.T) {
	for _, action := range []string{"events", "requests", "attempts", "transformations"} {
		t.Run(action, func(t *testing.T) {
			var got wireRequest
			session := readSession(t, map[string]http.HandlerFunc{
				"GET /2025-07-01/metrics/" + action: ok(&got, []map[string]any{
					{
						"time_bucket": "2026-08-01T00:00:00Z",
						"dimensions":  map[string]any{"status": "FAILED"},
						"metrics":     map[string]any{"count": 42},
					},
				}),
			})

			text := succeeds(t, session, "gateway_metrics", map[string]any{
				"action":      action,
				"start":       "2026-08-01T00:00:00Z",
				"end":         "2026-08-14T00:00:00Z",
				"granularity": "1d",
				"measures":    []any{"count"},
				"dimensions":  []any{"status"},
				"source_id":   "src_1",
			})

			assert.Equal(t, "/2025-07-01/metrics/"+action, got.path)
			// The range uses bracketed keys, and repeated values use "[]"
			// suffixes; neither is interchangeable with a plain key.
			assert.Contains(t, got.query, "date_range%5Bstart%5D=2026-08-01T00%3A00%3A00Z")
			assert.Contains(t, got.query, "date_range%5Bend%5D=2026-08-14T00%3A00%3A00Z")
			assert.Contains(t, got.query, "granularity=1d")
			assert.Contains(t, got.query, "measures%5B%5D=count")
			assert.Contains(t, got.query, "dimensions%5B%5D=status")
			assert.Contains(t, got.query, "filters%5Bsource_id%5D=src_1")
			assert.Contains(t, string(envelopeData(t, text)), "42")
		})
	}
}

// ---------------------------------------------------------------------------
// Coverage gate
// ---------------------------------------------------------------------------

// TestEveryActionHasBeenCalledSuccessfully is a checklist rather than a
// behaviour test. It fails when an action — or a whole tool — is added without
// a successful call being written for it, which is the gap this file was
// created to close: a destructive action whose only coverage is its read-only
// refusal has never been proven to work at all.
//
// The action lists come from the specs themselves, so a new action shows up
// here the moment it is registered.
func TestEveryActionHasBeenCalledSuccessfully(t *testing.T) {
	covered := coveredActions()

	for _, spec := range resourceSpecs() {
		for _, action := range spec.Actions {
			assert.True(t, covered[spec.Resource][action.Name],
				"%s_%s action %q has no test making a successful call; "+
					"a refusal test alone does not prove the action works",
				toolPrefix, spec.Resource, action.Name)
		}
	}
}

// TestCoverageChecklistHasNoStaleEntries keeps the checklist honest in the
// other direction: an entry for an action that no longer exists would let a
// genuinely uncovered action hide behind a name that once matched.
func TestCoverageChecklistHasNoStaleEntries(t *testing.T) {
	actual := map[string]map[string]bool{}
	for _, spec := range resourceSpecs() {
		names := map[string]bool{}
		for _, action := range spec.Actions {
			names[action.Name] = true
		}
		actual[spec.Resource] = names
	}

	for resource, actions := range coveredActions() {
		require.Contains(t, actual, resource, "checklist covers unknown tool %q", resource)
		for name := range actions {
			assert.True(t, actual[resource][name],
				"checklist covers %s_%s action %q, which no longer exists",
				toolPrefix, resource, name)
		}
	}
}

// coveredActions lists, per tool, the actions that have a test in this file
// calling them and asserting the request that went on the wire. Adding an
// action to a spec without adding it here fails the checklist above.
func coveredActions() map[string]map[string]bool {
	return map[string]map[string]bool{
		"connections": {
			"list": true, "get": true, "pause": true, "unpause": true,
			"create": true, "upsert": true, "update": true, "delete": true,
			"enable": true, "disable": true,
		},
		"sources": {
			"list": true, "get": true, "create": true, "upsert": true,
			"update": true, "delete": true, "enable": true, "disable": true,
		},
		"destinations": {
			"list": true, "get": true, "create": true, "upsert": true,
			"update": true, "delete": true, "enable": true, "disable": true,
		},
		"transformations": {
			"list": true, "get": true, "create": true, "upsert": true,
			"update": true, "delete": true, "run": true,
		},
		"requests": {
			"list": true, "get": true, "raw_body": true, "events": true,
			"ignored_events": true, "retry": true,
		},
		"events": {
			"list": true, "get": true, "raw_body": true,
			"retry": true, "cancel": true, "mute": true,
		},
		"attempts": {"list": true, "get": true},
		"issues":   {"list": true, "get": true, "update": true, "dismiss": true},
		"metrics": {
			"events": true, "requests": true, "attempts": true, "transformations": true,
		},
	}
}
