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
	// The caller's headers survive, plus the content-type the engine needs and
	// the caller did not supply.
	assert.Equal(t, map[string]any{"x-test": "1", "content-type": "application/json"}, request["headers"])
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
// gateway_events (plural: search only)
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

// The filters added after the plural/singular split. Each one is only worth
// anything if it reaches the query string under the exact key the API expects,
// so the assertions name the encoded key rather than checking the call
// succeeded — the stub answers whatever it is asked.
func TestEventsListSendsSearchTermAndDeliveryGroup(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/events": ok(&got, listResponse(map[string]any{"id": "evt_1"})),
	})

	succeeds(t, session, "gateway_events", map[string]any{
		"action": "list", "search_term": "cus_1234", "delivery_group": "grp_1",
	})

	assert.Equal(t, "/2025-07-01/events", got.path)
	assert.Contains(t, got.query, "search_term=cus_1234")
	assert.Contains(t, got.query, "delivery_group=grp_1")
}

// next_attempt_at is a date-operator filter, so the two MCP params have to
// arrive as bracketed gte/lte keys. A plain next_attempt_at= would be a
// different (and invalid) query.
func TestEventsListSendsNextAttemptBounds(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/events": ok(&got, listResponse(map[string]any{"id": "evt_1"})),
	})

	succeeds(t, session, "gateway_events", map[string]any{
		"action":              "list",
		"next_attempt_after":  "2026-06-01T00:00:00Z",
		"next_attempt_before": "2026-06-30T00:00:00Z",
	})

	assert.Contains(t, got.query, "next_attempt_at%5Bgte%5D=2026-06-01T00%3A00%3A00Z")
	assert.Contains(t, got.query, "next_attempt_at%5Blte%5D=2026-06-30T00%3A00%3A00Z")
}

// ---------------------------------------------------------------------------
// gateway_event (singular: one event by id)
// ---------------------------------------------------------------------------

func TestEventGetByID(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/events/evt_1": ok(&got, map[string]any{"id": "evt_1", "status": "FAILED"}),
	})

	text := succeeds(t, session, "gateway_event", map[string]any{"action": "get", "id": "evt_1"})

	assert.Equal(t, http.MethodGet, got.method)
	assert.Equal(t, "/2025-07-01/events/evt_1", got.path)
	// The id addresses one record; sending it as a filter would list instead.
	assert.Empty(t, got.query)
	assert.Contains(t, string(envelopeData(t, text)), "evt_1")
}

func TestEventRawBody(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/events/evt_1/raw_body": func(w http.ResponseWriter, r *http.Request) {
			got = wireRequest{method: r.Method, path: r.URL.Path}
			_, _ = w.Write([]byte(`{"amount":100}`))
		},
	})

	text := succeeds(t, session, "gateway_event", map[string]any{"action": "raw_body", "id": "evt_1"})

	assert.Equal(t, http.MethodGet, got.method)
	assert.Equal(t, "/2025-07-01/events/evt_1/raw_body", got.path)
	assert.Contains(t, string(envelopeData(t, text)), "amount")
}

// The three by-id event mutations differ in method and path, and the API
// returns no body, so the tool reports the outcome itself.
func TestEventRetryCancelAndMute(t *testing.T) {
	cases := []struct {
		action  string
		pattern string
		method  string
	}{
		{"retry", "POST /2025-07-01/events/evt_1/retry", http.MethodPost},
		{"cancel", "PUT /2025-07-01/events/evt_1/cancel", http.MethodPut},
		{"mute", "PUT /2025-07-01/events/evt_1/mute", http.MethodPut},
	}

	for _, tc := range cases {
		t.Run(tc.action, func(t *testing.T) {
			var got wireRequest
			session := writeSession(t, map[string]http.HandlerFunc{
				tc.pattern: ok(&got, map[string]any{"id": "evt_1", "status": "QUEUED"}),
			})

			text := succeeds(t, session, "gateway_event", map[string]any{"action": tc.action, "id": "evt_1"})

			assert.Equal(t, tc.method, got.method)
			assert.Equal(t, "/2025-07-01/events/evt_1/"+tc.action, got.path)
			// The event the API answered with, not a status we assumed.
			assert.Contains(t, string(envelopeData(t, text)), `"status":"QUEUED"`)
		})
	}
}

// The API answers 200 for a no-op: cancelling an already-delivered event leaves
// it SUCCESSFUL. The tool used to report {"status":"cancelled"} regardless,
// telling the caller something that had not happened. This is the test that
// would have caught it.
func TestEventMutationsReportTheRealStatusNotTheRequestedOne(t *testing.T) {
	for _, action := range []string{"cancel", "mute"} {
		t.Run(action, func(t *testing.T) {
			var got wireRequest
			session := writeSession(t, map[string]http.HandlerFunc{
				"PUT /2025-07-01/events/evt_1/" + action: ok(&got,
					map[string]any{"id": "evt_1", "status": "SUCCESSFUL"}),
			})

			text := succeeds(t, session, "gateway_event", map[string]any{"action": action, "id": "evt_1"})
			data := string(envelopeData(t, text))

			assert.Contains(t, data, `"status":"SUCCESSFUL"`,
				"the response must carry the status the API returned")
			assert.NotContains(t, data, "cancelled",
				"a no-op must not be reported as though it changed the event")
			assert.NotContains(t, data, "muted",
				"a no-op must not be reported as though it changed the event")
		})
	}
}

// A run that threw must not read as a run that worked.
//
// The endpoint answers 200 either way and log_level is the only signal, so
// dropping it meant a syntax error, a throwing handler, a handler returning
// nothing and a clean run all produced {"data":{}} — indistinguishable.
func TestTransformationsRunSurfacesAFailedRun(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"PUT /2025-07-01/transformations/run": ok(&got, map[string]any{
			"log_level": "fatal",
			"console": []map[string]any{
				{"type": "error", "message": "Error: boom-marker"},
			},
		}),
	})

	result := callTool(t, session, "gateway_transformations", map[string]any{
		"action":  "run",
		"code":    `addHandler("transform", (r, c) => { throw new Error("boom-marker"); });`,
		"request": map[string]any{"headers": map[string]any{}, "body": map[string]any{"a": 1}},
	})

	require.True(t, result.IsError, "a transformation that threw must not be reported as a success")
	text := textContent(t, result)
	assert.Contains(t, text, "did not complete")
	assert.Contains(t, text, "boom-marker", "the reason has to reach the caller")
}

// A clean run still returns the transformed request.
func TestTransformationsRunReturnsTheResultOnSuccess(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"PUT /2025-07-01/transformations/run": ok(&got, map[string]any{
			"log_level": "info",
			"request":   map[string]any{"headers": map[string]any{}, "body": map[string]any{"a": 1, "x": 1}},
		}),
	})

	text := succeeds(t, session, "gateway_transformations", map[string]any{
		"action":  "run",
		"code":    `addHandler("transform", (r, c) => { r.body.x = 1; return r; });`,
		"request": map[string]any{"headers": map[string]any{}, "body": map[string]any{"a": 1}},
	})
	assert.Contains(t, string(envelopeData(t, text)), `"x":1`)
}

// successfulRun is what the API actually returns for a run that completed: a
// log level and the transformed request. Stubbing a bare {"log_level":"info"}
// with no request describes a response the endpoint never sends — every run
// observed without a request came back "fatal" — and made these tests pass
// against a shape the success check is right to reject.
func successfulRun() map[string]any {
	return map[string]any{
		"log_level": "info",
		"request":   map[string]any{"headers": map[string]any{}, "body": map[string]any{}},
	}
}

// The transformation engine errors without a content-type, and the schema tells
// callers headers may be an empty object. The CLI has always supplied one; the
// MCP path did not, so identical code worked from one surface and not the other.
func TestTransformationsRunSuppliesAContentType(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"PUT /2025-07-01/transformations/run": ok(&got, successfulRun()),
	})

	succeeds(t, session, "gateway_transformations", map[string]any{
		"action":  "run",
		"code":    "addHandler(\"transform\", (r, c) => r);",
		"request": map[string]any{"headers": map[string]any{}},
	})

	body := got.decodeBody(t)
	request, ok := body["request"].(map[string]any)
	require.True(t, ok, "request must be sent")
	headers, ok := request["headers"].(map[string]any)
	require.True(t, ok, "headers must be sent")
	assert.Equal(t, "application/json", headers["content-type"],
		"a content-type must be supplied when the caller sent none")
}

// A caller who set their own content-type keeps it.
func TestTransformationsRunKeepsACallerContentType(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"PUT /2025-07-01/transformations/run": ok(&got, successfulRun()),
	})

	succeeds(t, session, "gateway_transformations", map[string]any{
		"action":  "run",
		"code":    "addHandler(\"transform\", (r, c) => r);",
		"request": map[string]any{"headers": map[string]any{"content-type": "text/plain"}},
	})

	request := got.decodeBody(t)["request"].(map[string]any)
	headers := request["headers"].(map[string]any)
	assert.Equal(t, "text/plain", headers["content-type"])
}

// ---------------------------------------------------------------------------
// gateway_requests (plural: search only)
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

// The count filters and search_term added after the plural/singular split.
// events_count=0 — requests that produced no events — is the query that
// explains a "missing" webhook, and it only works if the zero survives to the
// wire rather than being dropped as an empty value.
func TestRequestsListSendsSearchTermAndCounts(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/requests": ok(&got, listResponse(map[string]any{"id": "req_1"})),
	})

	succeeds(t, session, "gateway_requests", map[string]any{
		"action": "list", "search_term": "cus_1234",
		"events_count": "0", "ignored_count": "2", "cli_events_count": "1",
	})

	assert.Equal(t, "/2025-07-01/requests", got.path)
	assert.Contains(t, got.query, "search_term=cus_1234")
	assert.Contains(t, got.query, "events_count=0")
	assert.Contains(t, got.query, "ignored_count=2")
	assert.Contains(t, got.query, "cli_events_count=1")
}

// A model is at least as likely to send a count as a JSON number as a quoted
// string, since the parameter means a number. The schema types these as string
// because the API also accepts operator syntax, so the handler has to take both.
//
// Getting this wrong is worse than an error: the filter is dropped, the API
// returns every record, and the caller sees a plausible unfiltered answer with
// nothing to indicate its filter was ignored. events_count: 0 — "which requests
// arrived but delivered nothing" — is the query that breaks.
func TestRequestsListAcceptsCountsAsJSONNumbers(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/requests": ok(&got, listResponse(map[string]any{"id": "req_1"})),
	})

	succeeds(t, session, "gateway_requests", map[string]any{
		"action": "list", "events_count": 0, "ignored_count": 2, "cli_events_count": 1,
	})

	assert.Contains(t, got.query, "events_count=0",
		"a zero count sent as a JSON number must survive as a filter")
	assert.Contains(t, got.query, "ignored_count=2")
	assert.Contains(t, got.query, "cli_events_count=1")
}

// Booleans have the same problem as the counts above, and a worse consequence:
// verified: "false" was dropped, so every request came back and the caller
// reported verified requests as unverified.
func TestRequestsListAcceptsVerifiedAsAString(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value any
		want  string
	}{
		{"json false", false, "verified=false"},
		{"quoted false", "false", "verified=false"},
		{"json true", true, "verified=true"},
		{"quoted true", "true", "verified=true"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got wireRequest
			session := readSession(t, map[string]http.HandlerFunc{
				"GET /2025-07-01/requests": ok(&got, listResponse(map[string]any{"id": "req_1"})),
			})

			succeeds(t, session, "gateway_requests", map[string]any{
				"action": "list", "verified": tc.value,
			})

			assert.Contains(t, got.query, tc.want,
				"a dropped verification filter returns every request as if it matched")
		})
	}
}

// disabled: "true" is the connections equivalent — dropping it lists every
// connection, enabled ones included.
func TestConnectionsListAcceptsDisabledAsAString(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/connections": ok(&got, listResponse(connectionBody())),
	})

	succeeds(t, session, "gateway_connections", map[string]any{
		"action": "list", "disabled": "true",
	})

	assert.Contains(t, got.query, "disabled_at%5Bany%5D=true")
}

// limit: "5" quietly returned the API's default page size instead.
func TestListsAcceptLimitAsAString(t *testing.T) {
	for _, tc := range []struct {
		tool string
		path string
		body map[string]any
	}{
		{"gateway_requests", "/2025-07-01/requests", map[string]any{"id": "req_1"}},
		{"gateway_events", "/2025-07-01/events", map[string]any{"id": "evt_1"}},
		{"gateway_connections", "/2025-07-01/connections", connectionBody()},
		{"gateway_sources", "/2025-07-01/sources", map[string]any{"id": "src_1"}},
		{"gateway_destinations", "/2025-07-01/destinations", map[string]any{"id": "des_1"}},
		{"gateway_transformations", "/2025-07-01/transformations", map[string]any{"id": "trs_1"}},
		{"gateway_issues", "/2025-07-01/issues", map[string]any{"id": "iss_1"}},
	} {
		t.Run(tc.tool, func(t *testing.T) {
			var got wireRequest
			session := readSession(t, map[string]http.HandlerFunc{
				"GET " + tc.path: ok(&got, listResponse(tc.body)),
			})

			succeeds(t, session, tc.tool, map[string]any{"action": "list", "limit": "5"})

			assert.Contains(t, got.query, "limit=5",
				"a quoted limit must not fall back to the API's page size")
		})
	}
}

// Same trait on the events tool, which had it before these filters were added.
func TestEventsListAcceptsAttemptsAsAJSONNumber(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/events": ok(&got, listResponse(map[string]any{"id": "evt_1"})),
	})

	succeeds(t, session, "gateway_events", map[string]any{
		"action": "list", "attempts": 0, "response_status": 500,
	})

	assert.Contains(t, got.query, "attempts=0")
	assert.Contains(t, got.query, "response_status=500")
}

// ---------------------------------------------------------------------------
// gateway_request (singular: one request by id)
// ---------------------------------------------------------------------------

func TestRequestGetByID(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/requests/req_1": ok(&got, map[string]any{"id": "req_1"}),
	})

	text := succeeds(t, session, "gateway_request", map[string]any{"action": "get", "id": "req_1"})

	assert.Equal(t, http.MethodGet, got.method)
	assert.Equal(t, "/2025-07-01/requests/req_1", got.path)
	// The id addresses one record; sending it as a filter would list instead.
	assert.Empty(t, got.query)
	assert.Contains(t, string(envelopeData(t, text)), "req_1")
}

func TestRequestRawBody(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/requests/req_1/raw_body": func(w http.ResponseWriter, r *http.Request) {
			got = wireRequest{method: r.Method, path: r.URL.Path}
			_, _ = w.Write([]byte(`{"amount":100}`))
		},
	})

	text := succeeds(t, session, "gateway_request", map[string]any{"action": "raw_body", "id": "req_1"})

	assert.Equal(t, http.MethodGet, got.method)
	assert.Equal(t, "/2025-07-01/requests/req_1/raw_body", got.path)
	assert.Contains(t, string(envelopeData(t, text)), "amount")
}

// events and ignored_events are the only relationship traversal the API
// offers, so they have to reach their own sub-resource paths.
func TestRequestEventsAndIgnoredEvents(t *testing.T) {
	for _, action := range []string{"events", "ignored_events"} {
		t.Run(action, func(t *testing.T) {
			var got wireRequest
			session := readSession(t, map[string]http.HandlerFunc{
				"GET /2025-07-01/requests/req_1/" + action: ok(&got, listResponse(map[string]any{"id": "evt_1"})),
			})

			text := succeeds(t, session, "gateway_request", map[string]any{"action": action, "id": "req_1"})

			assert.Equal(t, http.MethodGet, got.method)
			assert.Equal(t, "/2025-07-01/requests/req_1/"+action, got.path)
			assert.Contains(t, string(envelopeData(t, text)), "evt_1")
		})
	}
}

// retriedRequest is what the retry endpoint actually returns: the request and
// the events the retry created. Stubbing an empty body described a response the
// API does not send, and hid whether a retry had produced anything.
func retriedRequest() map[string]any {
	return map[string]any{
		"request": map[string]any{"id": "req_1"},
		"events":  []any{map[string]any{"id": "evt_1"}},
	}
}

func TestRequestRetrySendsSelectedConnections(t *testing.T) {
	var got wireRequest
	session := writeSession(t, map[string]http.HandlerFunc{
		"POST /2025-07-01/requests/req_1/retry": ok(&got, retriedRequest()),
	})

	text := succeeds(t, session, "gateway_request", map[string]any{
		"action": "retry", "id": "req_1", "connection_ids": []any{"web_1", "web_2"},
	})

	assert.Equal(t, http.MethodPost, got.method)
	assert.Equal(t, "/2025-07-01/requests/req_1/retry", got.path)
	// connection_ids is the MCP name for the API's webhook_ids; dropping it
	// would retry every connection instead of the two asked for.
	assert.Equal(t, []any{"web_1", "web_2"}, got.decodeBody(t)["webhook_ids"])
	// Asserts the events the retry created, not a fixed "retried". The previous
	// version of this line pinned the hardcoded status, so it passed whether or
	// not the retry had produced anything.
	assert.JSONEq(t, `{"request_id":"req_1","events":["evt_1"],"retried":true}`, string(envelopeData(t, text)))
}

// Omitting connection_ids must send an empty body, which the API reads as
// "every connection the request matched".
func TestRequestRetryWithoutConnectionsSendsNoIDs(t *testing.T) {
	var got wireRequest
	session := writeSession(t, map[string]http.HandlerFunc{
		"POST /2025-07-01/requests/req_1/retry": ok(&got, retriedRequest()),
	})

	succeeds(t, session, "gateway_request", map[string]any{"action": "retry", "id": "req_1"})
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
		// requests/events are the plural search tools; request/event are the
		// singular by-id tools they hand IDs to.
		"requests": {"list": true},
		"request": {
			"get": true, "raw_body": true, "events": true,
			"ignored_events": true, "retry": true,
		},
		"events": {"list": true},
		"event": {
			"get": true, "raw_body": true,
			"retry": true, "cancel": true, "mute": true,
		},
		"attempts": {"list": true, "get": true},
		"issues":   {"list": true, "get": true, "update": true, "dismiss": true},
		"metrics": {
			"events": true, "requests": true, "attempts": true, "transformations": true,
		},
	}
}

// An argument the tool does not have is ignored by the API, so the call goes
// out unfiltered and the result reads as though it were filtered.
// gateway_events has no request_id filter; passing one returned every event.
func TestUnknownArgumentsAreRejected(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/events": ok(&got, listResponse(map[string]any{"id": "evt_1"})),
	})

	result := callTool(t, session, "gateway_events", map[string]any{
		"action": "list", "request_id": "req_1",
	})

	require.True(t, result.IsError, "an unknown filter must not be silently ignored")
	text := textContent(t, result)
	assert.Contains(t, text, "request_id")
	assert.Contains(t, text, "This tool accepts:", "the caller needs to know what is valid here")
	assert.Empty(t, got.query, "nothing should reach the API once the argument is rejected")
}

// The write guard owns the message for an argument that exists but is hidden by
// read-only mode. "unknown argument" would send the caller hunting for a typo
// that is not there.
func TestHiddenWriteArgumentsGetTheWriteModeMessage(t *testing.T) {
	session := readSession(t, nil)

	result := callTool(t, session, "gateway_sources", map[string]any{
		"action": "create", "name": "s", "type": "HTTP",
	})

	require.True(t, result.IsError)
	text := textContent(t, result)
	assert.Contains(t, text, "--allow-write")
	assert.NotContains(t, text, "unknown argument",
		"type exists on this tool; it is the mode that hides it")
}

// Hookdeck ids carry their type as a prefix, and splitting events and requests
// into plural and singular tools means an agent routinely holds both kinds at
// once. A req_ id passed to the event tool used to answer "Resource not found",
// so the agent reported that a request did not exist when it did.
func TestSingularToolsCatchAnIDOfTheWrongKind(t *testing.T) {
	session := readSession(t, nil)

	t.Run("a request id passed to gateway_event", func(t *testing.T) {
		result := callTool(t, session, "gateway_event",
			map[string]any{"action": "get", "id": "req_CXgN9WztKCtGplqLfKlZ"})
		require.True(t, result.IsError)
		text := textContent(t, result)
		assert.Contains(t, text, "gateway_request", "the error has to name the tool that would work")
		assert.NotContains(t, text, "not found",
			"a wrong-kind id is not a missing record, and saying so sends the caller looking for the wrong thing")
	})

	t.Run("an event id passed to gateway_request", func(t *testing.T) {
		result := callTool(t, session, "gateway_request",
			map[string]any{"action": "get", "id": "evt_90lo6Wn1vhCSa32gzE"})
		require.True(t, result.IsError)
		assert.Contains(t, textContent(t, result), "gateway_event")
	})

	t.Run("an unrecognised prefix is left to the API", func(t *testing.T) {
		// Only ids that clearly belong to another tool are caught. Anything else
		// is the API's to judge, so a new resource type does not start failing
		// here the day it ships.
		result := callTool(t, session, "gateway_event",
			map[string]any{"action": "get", "id": "future_abc123"})
		assert.NotContains(t, textContent(t, result), "is a ")
	})
}

// The unknown-action error names the sibling, so a dead end becomes a redirect.
func TestUnknownActionNamesTheSiblingTool(t *testing.T) {
	session := readSession(t, nil)

	assert.Contains(t,
		textContent(t, callTool(t, session, "gateway_event", map[string]any{"action": "list"})),
		"gateway_events", "list belongs to the plural tool; say so")

	assert.Contains(t,
		textContent(t, callTool(t, session, "gateway_events", map[string]any{"action": "raw_body"})),
		"gateway_event", "raw_body belongs to the singular tool; say so")
}

// A hidden write-only argument on a VISIBLE action must be rejected, not
// exempted.
//
// The mode exemption exists so {"action":"create","type":"HTTP"} in read-only
// mode gets "restart with --allow-write" rather than "unknown argument". But it
// originally applied whatever action was requested, so {"action":"list","type":
// "HTTP"} was exempted too — then ignored by the handler, and the caller got an
// unfiltered list that read as a filtered one. That is the failure this guard
// exists to prevent.
func TestHiddenArgOnAVisibleActionIsRejected(t *testing.T) {
	var got wireRequest
	session := readSession(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/sources": ok(&got, listResponse(map[string]any{"id": "src_1"})),
	})

	result := callTool(t, session, "gateway_sources", map[string]any{
		"action": "list", "type": "HTTP",
	})

	require.True(t, result.IsError, "list does not take type; ignoring it returns an unfiltered result")
	assert.Contains(t, textContent(t, result), "unknown argument")
	assert.Empty(t, got.query, "nothing should reach the API")
}

// The exemption still applies where it was meant to: the action itself hidden.
func TestHiddenArgOnAHiddenActionDefersToTheWriteGuard(t *testing.T) {
	session := readSession(t, nil)

	result := callTool(t, session, "gateway_sources", map[string]any{
		"action": "create", "name": "s", "type": "HTTP",
	})

	require.True(t, result.IsError)
	text := textContent(t, result)
	assert.Contains(t, text, "--allow-write")
	assert.NotContains(t, text, "unknown argument")
}

// Help and the schema must offer the same parameters. A help topic listing
// `config` for a tool whose schema does not have it gives an agent two answers,
// and the help topic is the more persuasive one.
func TestHelpAndSchemaAgreeOnParameters(t *testing.T) {
	session := readSession(t, nil)
	tools := listTools(t, session)

	for _, tool := range []string{"gateway_sources", "gateway_destinations", "gateway_connections", "gateway_issues"} {
		t.Run(tool, func(t *testing.T) {
			help := textContent(t, callTool(t, session, "gateway_help", map[string]any{"topic": tool}))
			schema := schemaPropertyNames(t, tools[tool])

			for _, hidden := range []string{"config", "rules", "description"} {
				if contains(schema, hidden) {
					continue // visible in this mode; nothing to check
				}
				assert.NotContains(t, help, "\n  "+hidden+" ",
					"help lists %q as a parameter of %s, but the schema does not offer it", hidden, tool)
			}
		})
	}
}

func contains(haystack []string, needle string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}
