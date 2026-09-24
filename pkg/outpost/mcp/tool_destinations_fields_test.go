package mcp

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The CLI rejects a destination field the type does not declare. This tool
// accepted one: an unknown config key was stored and an unknown credential was
// dropped, both reported as success, so a misspelled optional field looked
// applied and never took effect (#447).

// webhookSchemaAPI serves one webhook schema in the API's own shape, and
// records whether a destination write reached the API.
func webhookSchemaAPI(t *testing.T, wrote *bool) string {
	t.Helper()
	api := mockAPI(t, map[string]http.HandlerFunc{
		"/2026-09-01/destination-types": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"type":  "webhook",
				"label": "Webhook",
				"config_fields": []map[string]any{
					{"key": "url", "type": "text", "required": true},
					{"key": "custom_headers", "type": "key_value_map", "required": false},
				},
				"credential_fields": []map[string]any{
					{"key": "secret", "type": "text", "required": false},
				},
			}})
		},
		"/2026-09-01/tenants/t1/destinations": func(w http.ResponseWriter, r *http.Request) {
			*wrote = true
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "des_1", "type": "webhook"})
		},
		"/2026-09-01/tenants/t1/destinations/des_1": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				*wrote = true
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "des_1", "type": "webhook"})
		},
	})
	return api.URL
}

func TestDestinationWriteRejectsFieldsTheTypeDoesNotDeclare(t *testing.T) {
	for _, tt := range []struct {
		name string
		args map[string]any
		want string
	}{
		{
			name: "create with an unknown config key",
			args: map[string]any{"action": "create", "tenant_id": "t1", "type": "webhook",
				"config": map[string]any{"url": "https://example.com/hook", "custom_header": "x"}},
			want: `config has fields the webhook type does not accept: "custom_header"`,
		},
		{
			name: "create with an unknown credential",
			args: map[string]any{"action": "create", "tenant_id": "t1", "type": "webhook",
				"config":      map[string]any{"url": "https://example.com/hook"},
				"credentials": map[string]any{"nope": "x"}},
			want: `credentials has fields the webhook type does not accept: "nope"`,
		},
		{
			name: "update with an unknown config key",
			args: map[string]any{"action": "update", "tenant_id": "t1", "id": "des_1",
				"config": map[string]any{"another_bogus": "1"}},
			want: `config has fields the webhook type does not accept: "another_bogus"`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			wrote := false
			session := connect(t, ServerOptions{Client: newTestClient(t, webhookSchemaAPI(t, &wrote)), WriteEnabled: true})

			result := callTool(t, session, "outpost_destinations_write", tt.args)

			require.True(t, result.IsError, "an unknown field must be refused, not stored or dropped")
			body := resultText(t, result)
			assert.Contains(t, body, tt.want)
			assert.Contains(t, body, "outpost_destination_types_read", "the error names the tool that lists the valid fields")
			assert.False(t, wrote, "the write must not reach the API")
		})
	}
}

func TestDestinationWriteAcceptsDeclaredFields(t *testing.T) {
	wrote := false
	session := connect(t, ServerOptions{Client: newTestClient(t, webhookSchemaAPI(t, &wrote)), WriteEnabled: true})

	result := callTool(t, session, "outpost_destinations_write", map[string]any{
		"action": "create", "tenant_id": "t1", "type": "webhook",
		"config":      map[string]any{"url": "https://example.com/hook", "custom_headers": map[string]any{"X-A": "1"}},
		"credentials": map[string]any{"secret": "s"},
	})

	require.False(t, result.IsError, resultText(t, result))
	assert.True(t, wrote, "declared fields go through to the API")
}
