package mcp

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A property is scoped to the actions whose handler actually reads it, so the
// shared guard can reject it everywhere else.
//
// Without the scopes, gateway_attempts_read accepted {"action":"get",
// "order_by":"created_at"} and attemptsGet never looked at order_by: the caller
// got one attempt back and no indication that the sort they asked for had been
// dropped. Every case below is the same shape — an argument the handler ignores
// making the result read as though it had been applied.
//
// Each subtest checks both halves. Rejecting the wrong action proves nothing on
// its own: a scope narrow enough to break the action that does read the
// argument would pass the first assertion and turn a working call into an
// error, which is worse than the over-permissiveness being fixed.

// gateway_attempts: order_by and dir are read by attemptsList only, and id by
// attemptsGet only.
func TestAttemptsSortOptionsBelongToList(t *testing.T) {
	t.Run("rejected on get", func(t *testing.T) {
		var got wireRequest
		session := readSession(t, map[string]http.HandlerFunc{
			"GET /2026-09-01/attempts/atm_1": ok(&got, map[string]any{"id": "atm_1"}),
		})

		result := callTool(t, session, "gateway_attempts_read", map[string]any{
			"action": "get", "id": "atm_1", "order_by": "created_at",
		})

		require.True(t, result.IsError, "get ignores order_by; the result would read as sorted")
		text := textContent(t, result)
		assert.Contains(t, text, "order_by")
		assert.Contains(t, text, "it belongs to list", "the caller needs the action that does take it")
		assert.Empty(t, got.path, "nothing should reach the API once the argument is rejected")
	})

	t.Run("accepted on list", func(t *testing.T) {
		var got wireRequest
		session := readSession(t, map[string]http.HandlerFunc{
			"GET /2026-09-01/attempts": ok(&got, listResponse(map[string]any{"id": "atm_1"})),
		})

		succeeds(t, session, "gateway_attempts_read", map[string]any{
			"action": "list", "order_by": "created_at", "dir": "desc",
		})

		assert.Contains(t, got.query, "order_by=created_at")
		assert.Contains(t, got.query, "dir=desc")
	})

	t.Run("id is rejected on list", func(t *testing.T) {
		var got wireRequest
		session := readSession(t, map[string]http.HandlerFunc{
			"GET /2026-09-01/attempts": ok(&got, listResponse(map[string]any{"id": "atm_1"})),
		})

		result := callTool(t, session, "gateway_attempts_read", map[string]any{
			"action": "list", "id": "atm_1",
		})

		require.True(t, result.IsError, "list has no id filter; it would return every attempt")
		assert.Empty(t, got.query)
	})
}

// gateway_issues: status is applied by issuesUpdate and nothing else, so a
// dismiss carrying one closed the issue with a status the caller believed had
// been set.
func TestIssuesStatusBelongsToUpdate(t *testing.T) {
	t.Run("rejected on dismiss", func(t *testing.T) {
		var got wireRequest
		session := writeSession(t, map[string]http.HandlerFunc{
			"PUT /2026-09-01/issues/iss_1/dismiss": ok(&got, map[string]any{"id": "iss_1"}),
		})

		result := callTool(t, session, "gateway_issues_write", map[string]any{
			"action": "dismiss", "id": "iss_1", "status": "RESOLVED",
		})

		require.True(t, result.IsError, "dismiss never applies status")
		assert.Contains(t, textContent(t, result), "status")
		assert.Empty(t, got.path, "the dismiss must not go out as though the status had been set")
	})

	t.Run("accepted on update", func(t *testing.T) {
		var got wireRequest
		session := writeSession(t, map[string]http.HandlerFunc{
			"PUT /2026-09-01/issues/iss_1": ok(&got, map[string]any{"id": "iss_1"}),
		})

		succeeds(t, session, "gateway_issues_write", map[string]any{
			"action": "update", "id": "iss_1", "status": "RESOLVED",
		})

		assert.Equal(t, "RESOLVED", got.decodeBody(t)["status"])
	})

	t.Run("list filters are rejected on get", func(t *testing.T) {
		var got wireRequest
		session := readSession(t, map[string]http.HandlerFunc{
			"GET /2026-09-01/issues/iss_1": ok(&got, map[string]any{"id": "iss_1"}),
		})

		result := callTool(t, session, "gateway_issues_read", map[string]any{
			"action": "get", "id": "iss_1", "filter_status": "OPENED",
		})

		require.True(t, result.IsError)
		assert.Empty(t, got.path)
	})
}

// gateway_sources: type is part of the create/upsert/update body and nothing
// reads it on delete.
func TestSourcesTypeBelongsToTheWriteBodyActions(t *testing.T) {
	t.Run("rejected on delete", func(t *testing.T) {
		var got wireRequest
		session := writeSession(t, map[string]http.HandlerFunc{
			"DELETE /2026-09-01/sources/src_1": ok(&got, nil),
		})

		result := callTool(t, session, "gateway_sources_write", map[string]any{
			"action": "delete", "id": "src_1", "type": "STRIPE",
		})

		require.True(t, result.IsError, "delete ignores type; the source is deleted whatever it is")
		assert.Contains(t, textContent(t, result), "type")
		assert.Empty(t, got.path, "a delete must not run while the caller believes it was qualified")
	})

	t.Run("accepted on create", func(t *testing.T) {
		var got wireRequest
		session := writeSession(t, map[string]http.HandlerFunc{
			"POST /2026-09-01/sources": ok(&got, map[string]any{"id": "src_1"}),
		})

		succeeds(t, session, "gateway_sources_write", map[string]any{
			"action": "create", "name": "stripe", "type": "STRIPE",
		})

		assert.Equal(t, "STRIPE", got.decodeBody(t)["type"])
	})
}

// gateway_transformations: connection_id and request are run-only, and id is
// not a list filter.
func TestTransformationRunArgumentsBelongToRun(t *testing.T) {
	t.Run("request is rejected on get", func(t *testing.T) {
		var got wireRequest
		session := readSession(t, map[string]http.HandlerFunc{
			"GET /2026-09-01/transformations/trs_1": ok(&got, map[string]any{"id": "trs_1"}),
		})

		result := callTool(t, session, "gateway_transformations_read", map[string]any{
			"action": "get", "id": "trs_1", "request": map[string]any{"headers": map[string]any{}},
		})

		require.True(t, result.IsError, "get returns the stored transformation; it runs nothing")
		text := textContent(t, result)
		assert.Contains(t, text, "request")
		assert.Contains(t, text, "it belongs to run")
		assert.Empty(t, got.path)
	})

	t.Run("accepted on run", func(t *testing.T) {
		var got wireRequest
		session := readSession(t, map[string]http.HandlerFunc{
			"PUT /2026-09-01/transformations/run": ok(&got, map[string]any{
				"request": map[string]any{"body": map[string]any{"ok": true}},
			}),
		})

		succeeds(t, session, "gateway_transformations_read", map[string]any{
			"action": "run", "code": "addHandler('transform', r => r)",
			"connection_id": "web_1",
			"request":       map[string]any{"headers": map[string]any{}},
		})

		body := got.decodeBody(t)
		assert.Equal(t, "web_1", body["webhook_id"])
		assert.NotNil(t, body["request"])
	})

	t.Run("connection_id is rejected on the write tool", func(t *testing.T) {
		var got wireRequest
		session := writeSession(t, map[string]http.HandlerFunc{
			"DELETE /2026-09-01/transformations/trs_1": ok(&got, nil),
		})

		result := callTool(t, session, "gateway_transformations_write", map[string]any{
			"action": "delete", "id": "trs_1", "connection_id": "web_1",
		})

		require.True(t, result.IsError, "delete takes no connection")
		assert.Empty(t, got.path)
	})
}

// gateway_connections: description is part of the create/upsert/update body,
// and rules replaces a ruleset — neither reaches a delete or a pause.
func TestConnectionsBodyArgumentsBelongToTheBodyActions(t *testing.T) {
	t.Run("description is rejected on delete", func(t *testing.T) {
		var got wireRequest
		session := writeSession(t, map[string]http.HandlerFunc{
			"DELETE /2026-09-01/connections/web_1": ok(&got, nil),
		})

		result := callTool(t, session, "gateway_connections_write", map[string]any{
			"action": "delete", "id": "web_1", "description": "retired",
		})

		require.True(t, result.IsError)
		assert.Contains(t, textContent(t, result), "description")
		assert.Empty(t, got.path)
	})

	t.Run("rules is not offered to pause at all", func(t *testing.T) {
		session := readSession(t, nil)
		tools := listTools(t, session)

		require.Contains(t, tools, "gateway_connections_pause")
		schema := schemaPropertyNames(t, tools["gateway_connections_pause"])
		assert.NotContains(t, schema, "rules", "pause writes no ruleset")
		assert.NotContains(t, schema, "source_id", "pause takes no source filter")
		assert.Contains(t, schema, "id", "pause addresses one connection")
	})

	t.Run("accepted on update", func(t *testing.T) {
		var got wireRequest
		session := writeSession(t, map[string]http.HandlerFunc{
			"GET /2026-09-01/connections/web_1": ok(&got, connectionBody()),
			"PUT /2026-09-01/connections/web_1": ok(&got, connectionBody()),
		})

		succeeds(t, session, "gateway_connections_write", map[string]any{
			"action": "update", "id": "web_1", "description": "retired",
		})

		assert.Equal(t, "retired", got.decodeBody(t)["description"])
	})
}
