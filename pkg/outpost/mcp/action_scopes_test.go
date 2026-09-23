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
// Without the scopes, {"action":"get","topic":"orders"} on outpost_events_read
// succeeded and eventsGet never looked at topic: the caller got the event back
// and had no way to tell the topic had been dropped rather than matched. Every
// case below is that same shape.
//
// Each subtest checks both halves — rejected on the action that ignores the
// argument, and still accepted on the action that consumes it. A scope narrow
// enough to break the working call would pass the first assertion alone.

// outpost_events: topic, time_after and the rest are eventsList filters;
// destination_id is read by list and retry but not get; tenant_id by list and
// get but not retry.
func TestEventsFiltersBelongToList(t *testing.T) {
	t.Run("topic is rejected on get", func(t *testing.T) {
		var got captured
		api := mockAPI(t, map[string]http.HandlerFunc{
			"GET /2026-09-01/events/evt_1": recordJSON(&got, http.StatusOK, map[string]any{"id": "evt_1"}),
		})
		session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

		result := callTool(t, session, "outpost_events_read", map[string]any{
			"action": "get", "id": "evt_1", "topic": "orders",
		})

		require.True(t, result.IsError, "get ignores topic; the event would read as topic-matched")
		text := resultText(t, result)
		assert.Contains(t, text, "topic")
		assert.Contains(t, text, "it belongs to list")
		assert.Empty(t, got.path, "nothing should reach the API once the argument is rejected")
	})

	t.Run("topic is accepted on list", func(t *testing.T) {
		var got captured
		api := mockAPI(t, map[string]http.HandlerFunc{
			"GET /2026-09-01/events": recordJSON(&got, http.StatusOK, map[string]any{
				"data": []any{map[string]any{"id": "evt_1"}},
			}),
		})
		session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

		result := callTool(t, session, "outpost_events_read", map[string]any{
			"action": "list", "topic": "orders",
		})

		require.False(t, result.IsError, resultText(t, result))
		assert.Contains(t, got.query, "orders")
	})

	t.Run("tenant_id is rejected on retry", func(t *testing.T) {
		var got captured
		api := mockAPI(t, map[string]http.HandlerFunc{
			"POST /2026-09-01/events/evt_1/retry": recordJSON(&got, http.StatusOK, map[string]any{"success": true}),
		})
		session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

		result := callTool(t, session, "outpost_events_write", map[string]any{
			"action": "retry", "id": "evt_1", "destination_id": "des_1", "tenant_id": "acme",
		})

		require.True(t, result.IsError, "retry is keyed on the event and destination only")
		assert.Contains(t, resultText(t, result), "tenant_id")
		assert.Empty(t, got.path, "a retry must not be queued while the caller believes it was tenant-scoped")
	})

	t.Run("tenant_id is accepted on get", func(t *testing.T) {
		var got captured
		api := mockAPI(t, map[string]http.HandlerFunc{
			"GET /2026-09-01/events/evt_1": recordJSON(&got, http.StatusOK, map[string]any{"id": "evt_1"}),
		})
		session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

		result := callTool(t, session, "outpost_events_read", map[string]any{
			"action": "get", "id": "evt_1", "tenant_id": "acme",
		})

		require.False(t, result.IsError, resultText(t, result))
		assert.Equal(t, "/2026-09-01/events/evt_1", got.path)
		assert.Contains(t, got.query, "tenant_id=acme", "get scopes the lookup to the tenant")
	})
}

// outpost_attempts: the list filters are attemptsList's, but tenant_id,
// destination_id and include are read by attemptsGet too.
func TestAttemptsFiltersBelongToList(t *testing.T) {
	t.Run("order_by is rejected on get", func(t *testing.T) {
		var got captured
		api := mockAPI(t, map[string]http.HandlerFunc{
			"GET /2026-09-01/attempts/att_1": recordJSON(&got, http.StatusOK, map[string]any{"id": "att_1"}),
		})
		session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

		result := callTool(t, session, "outpost_attempts_read", map[string]any{
			"action": "get", "id": "att_1", "order_by": "time",
		})

		require.True(t, result.IsError)
		assert.Contains(t, resultText(t, result), "order_by")
		assert.Empty(t, got.path)
	})

	t.Run("tenant_id and include are still accepted on get", func(t *testing.T) {
		var got captured
		api := mockAPI(t, map[string]http.HandlerFunc{
			"GET /2026-09-01/tenants/acme/destinations/des_1/attempts/att_1": recordJSON(
				&got, http.StatusOK, map[string]any{"id": "att_1"}),
		})
		session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

		result := callTool(t, session, "outpost_attempts_read", map[string]any{
			"action": "get", "id": "att_1",
			"tenant_id": "acme", "destination_id": "des_1", "include": []any{"event"},
		})

		require.False(t, result.IsError, resultText(t, result))
		assert.Equal(t, "/2026-09-01/tenants/acme/destinations/des_1/attempts/att_1", got.path)
		assert.Contains(t, got.query, "include")
		assert.Contains(t, got.query, "event")
	})

	t.Run("id is rejected on list", func(t *testing.T) {
		var got captured
		api := mockAPI(t, map[string]http.HandlerFunc{
			"GET /2026-09-01/attempts": recordJSON(&got, http.StatusOK, map[string]any{"data": []any{}}),
		})
		session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

		result := callTool(t, session, "outpost_attempts_read", map[string]any{
			"action": "list", "id": "att_1",
		})

		require.True(t, result.IsError, "list has no id filter; it would return every attempt")
		assert.Empty(t, got.path)
	})
}

// outpost_config: values and unset are configSet's, hostname is
// custom_domain_set's. Nothing reads any of them on custom_domain_delete.
func TestConfigArgumentsBelongToTheirAction(t *testing.T) {
	t.Run("values is rejected on custom_domain_delete", func(t *testing.T) {
		var got captured
		api := mockAPI(t, map[string]http.HandlerFunc{
			"DELETE /2026-09-01/config/custom_domain": recordJSON(&got, http.StatusOK, nil),
		})
		session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

		result := callTool(t, session, "outpost_config_write", map[string]any{
			"action": "custom_domain_delete", "values": map[string]any{"TOPICS": "orders"},
		})

		require.True(t, result.IsError, "removing the custom domain changes no config values")
		assert.Contains(t, resultText(t, result), "values")
		assert.Empty(t, got.path, "the domain must not be deleted while the caller believes config was set")
	})

	t.Run("hostname is rejected on set", func(t *testing.T) {
		var got captured
		api := mockAPI(t, map[string]http.HandlerFunc{
			"PATCH /2026-09-01/config": recordJSON(&got, http.StatusOK, map[string]any{"TOPICS": "orders"}),
		})
		session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

		result := callTool(t, session, "outpost_config_write", map[string]any{
			"action": "set", "values": map[string]any{"TOPICS": "orders"}, "hostname": "hooks.example.com",
		})

		require.True(t, result.IsError, "set does not configure the portal domain")
		assert.Contains(t, resultText(t, result), "hostname")
		assert.Empty(t, got.path)
	})

	t.Run("each is accepted on its own action", func(t *testing.T) {
		var got captured
		api := mockAPI(t, map[string]http.HandlerFunc{
			"PATCH /2026-09-01/config":              recordJSON(&got, http.StatusOK, map[string]any{"TOPICS": "orders"}),
			"POST /2026-09-01/config/custom_domain": recordJSON(&got, http.StatusCreated, map[string]any{"hostname": "hooks.example.com"}),
		})
		session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

		result := callTool(t, session, "outpost_config_write", map[string]any{
			"action": "set", "values": map[string]any{"TOPICS": "orders"},
		})
		require.False(t, result.IsError, resultText(t, result))
		assert.Equal(t, "/2026-09-01/config", got.path)

		result = callTool(t, session, "outpost_config_write", map[string]any{
			"action": "custom_domain_set", "hostname": "hooks.example.com",
		})
		require.False(t, result.IsError, resultText(t, result))
		assert.Equal(t, "/2026-09-01/config/custom_domain", got.path)
	})
}

// outpost_tenants: metadata is tenantsUpsert's and theme is tenantsPortal's.
// A token call carrying metadata minted a credential while the caller believed
// the tenant had been updated.
func TestTenantsWriteArgumentsBelongToTheirAction(t *testing.T) {
	t.Run("metadata is rejected on token", func(t *testing.T) {
		var got captured
		api := mockAPI(t, map[string]http.HandlerFunc{
			"GET /2026-09-01/tenants/acme/token": recordJSON(&got, http.StatusOK, map[string]any{"token": "tok"}),
		})
		session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

		result := callTool(t, session, "outpost_tenants_write", map[string]any{
			"action": "token", "id": "acme", "metadata": map[string]any{"plan": "pro"},
		})

		require.True(t, result.IsError)
		assert.Contains(t, resultText(t, result), "metadata")
		assert.Empty(t, got.path, "a token must not be minted while the caller believes metadata was stored")
	})

	t.Run("metadata is accepted on upsert", func(t *testing.T) {
		var got captured
		api := mockAPI(t, map[string]http.HandlerFunc{
			"PUT /2026-09-01/tenants/acme": recordJSON(&got, http.StatusOK, map[string]any{"id": "acme"}),
		})
		session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

		result := callTool(t, session, "outpost_tenants_write", map[string]any{
			"action": "upsert", "id": "acme", "metadata": map[string]any{"plan": "pro"},
		})

		require.False(t, result.IsError, resultText(t, result))
		assert.Equal(t, "pro", got.decodeBody(t)["metadata"].(map[string]any)["plan"])
	})

	t.Run("theme is rejected on upsert and accepted on portal", func(t *testing.T) {
		var got captured
		api := mockAPI(t, map[string]http.HandlerFunc{
			"PUT /2026-09-01/tenants/acme":        recordJSON(&got, http.StatusOK, map[string]any{"id": "acme"}),
			"GET /2026-09-01/tenants/acme/portal": recordJSON(&got, http.StatusOK, map[string]any{"redirect_url": "https://portal"}),
		})
		session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

		result := callTool(t, session, "outpost_tenants_write", map[string]any{
			"action": "upsert", "id": "acme", "theme": "dark",
		})
		require.True(t, result.IsError, "a tenant has no stored theme; only the portal URL takes one")
		assert.Empty(t, got.path)

		result = callTool(t, session, "outpost_tenants_write", map[string]any{
			"action": "portal", "id": "acme", "theme": "dark",
		})
		require.False(t, result.IsError, resultText(t, result))
		assert.Contains(t, got.query, "theme=dark")
	})
}

// outpost_destinations: the payload arguments are create's and update's, and
// type is create's and list's — a destination's type cannot be changed.
func TestDestinationPayloadArgumentsBelongToCreateAndUpdate(t *testing.T) {
	t.Run("credentials is rejected on delete", func(t *testing.T) {
		var got captured
		api := mockAPI(t, map[string]http.HandlerFunc{
			"DELETE /2026-09-01/tenants/acme/destinations/des_1": recordJSON(&got, http.StatusOK, nil),
		})
		session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

		result := callTool(t, session, "outpost_destinations_write", map[string]any{
			"action": "delete", "tenant_id": "acme", "id": "des_1",
			"credentials": map[string]any{"secret": "s"},
		})

		require.True(t, result.IsError)
		assert.Contains(t, resultText(t, result), "credentials")
		assert.Empty(t, got.path, "the destination must not be deleted on a call the caller misread")
	})

	t.Run("type is rejected on update", func(t *testing.T) {
		var got captured
		api := mockAPI(t, map[string]http.HandlerFunc{
			"PATCH /2026-09-01/tenants/acme/destinations/des_1": recordJSON(&got, http.StatusOK, map[string]any{"id": "des_1"}),
		})
		session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

		result := callTool(t, session, "outpost_destinations_write", map[string]any{
			"action": "update", "tenant_id": "acme", "id": "des_1", "type": "webhook",
		})

		require.True(t, result.IsError, "update never sends type; the destination would keep the old one")
		assert.Contains(t, resultText(t, result), "type")
		assert.Empty(t, got.path)
	})

	t.Run("type and credentials are accepted on create", func(t *testing.T) {
		var got captured
		api := mockAPI(t, map[string]http.HandlerFunc{
			"POST /2026-09-01/tenants/acme/destinations": recordJSON(&got, http.StatusCreated, map[string]any{"id": "des_1"}),
		})
		session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

		result := callTool(t, session, "outpost_destinations_write", map[string]any{
			"action": "create", "tenant_id": "acme", "type": "webhook",
			"config":      map[string]any{"url": "https://example.com/hooks"},
			"credentials": map[string]any{"secret": "s"},
		})

		require.False(t, result.IsError, resultText(t, result))
		assert.Equal(t, "webhook", got.decodeBody(t)["type"])
	})
}
