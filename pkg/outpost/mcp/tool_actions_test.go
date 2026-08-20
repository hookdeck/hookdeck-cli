package mcp

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

// Every action of every Outpost tool, called successfully at least once.
//
// The write actions previously had only their read-only refusal covered, so the
// first successful invocation of a delete or a create would have happened in a
// user's project. The reads that no test called at all are here for the same
// reason.
//
// These assert the request that goes on the wire — method, path, query and body
// — rather than only that the call did not error. Every Outpost defect found
// during development was a wire-shape bug, which a "no error" assertion misses
// entirely because the stub server answers whatever it is asked.

// captured records the request a handler actually sent.
type captured struct {
	method string
	path   string
	query  string
	body   []byte
}

// recordJSON captures the incoming request and replies with response.
func recordJSON(into *captured, status int, response any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		*into = captured{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, body: body}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if response != nil {
			_ = json.NewEncoder(w).Encode(response)
		}
	}
}

// decodeBody unmarshals the captured request body.
func (c captured) decodeBody(t *testing.T) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(c.body, &body), "request body was not a JSON object: %s", c.body)
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

// ---------------------------------------------------------------------------
// outpost_tenants — the write actions
// ---------------------------------------------------------------------------

func TestTenantsGet(t *testing.T) {
	var got captured
	api := mockAPI(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/tenants/acme": recordJSON(&got, http.StatusOK, map[string]any{
			"id": "acme", "destinations_count": 2, "topics": []string{"user.created"},
		}),
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

	result := callTool(t, session, "outpost_tenants", map[string]any{"action": "get", "id": "acme"})
	require.False(t, result.IsError, resultText(t, result))

	assert.Equal(t, "/2025-07-01/tenants/acme", got.path)
	assert.Contains(t, string(envelopeData(t, resultText(t, result))), `"acme"`)
}

func TestTenantsUpsertSendsMetadata(t *testing.T) {
	var got captured
	api := mockAPI(t, map[string]http.HandlerFunc{
		"PUT /2025-07-01/tenants/acme": recordJSON(&got, http.StatusOK, map[string]any{"id": "acme"}),
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

	result := callTool(t, session, "outpost_tenants", map[string]any{
		"action":   "upsert",
		"id":       "acme",
		"metadata": map[string]any{"plan": "pro", "region": "eu"},
	})
	require.False(t, result.IsError, resultText(t, result))

	assert.Equal(t, http.MethodPut, got.method)
	assert.Equal(t, "/2025-07-01/tenants/acme", got.path)
	assert.Equal(t, map[string]any{"plan": "pro", "region": "eu"}, got.decodeBody(t)["metadata"])
}

func TestTenantsDelete(t *testing.T) {
	var got captured
	api := mockAPI(t, map[string]http.HandlerFunc{
		"DELETE /2025-07-01/tenants/acme": recordJSON(&got, http.StatusOK, map[string]any{"success": true}),
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

	result := callTool(t, session, "outpost_tenants", map[string]any{"action": "delete", "id": "acme"})
	require.False(t, result.IsError, resultText(t, result))

	assert.Equal(t, http.MethodDelete, got.method)
	assert.Equal(t, "/2025-07-01/tenants/acme", got.path)
	// The API returns no useful body, so the tool has to say what happened.
	assert.JSONEq(t, `{"tenant_id":"acme","status":"deleted"}`, string(envelopeData(t, resultText(t, result))))
}

func TestTenantsDeleteRequiresAnID(t *testing.T) {
	api := mockAPI(t, nil)
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

	result := callTool(t, session, "outpost_tenants", map[string]any{"action": "delete"})
	require.True(t, result.IsError)
	assert.Contains(t, resultText(t, result), "id is required")
}

func TestTenantsToken(t *testing.T) {
	var got captured
	api := mockAPI(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/tenants/acme/token": recordJSON(&got, http.StatusOK, map[string]any{
			"token": "header.payload.signature", "tenant_id": "acme",
		}),
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

	result := callTool(t, session, "outpost_tenants", map[string]any{"action": "token", "id": "acme"})
	require.False(t, result.IsError, resultText(t, result))

	assert.Equal(t, "/2025-07-01/tenants/acme/token", got.path)
	assert.Contains(t, string(envelopeData(t, resultText(t, result))), "header.payload.signature")
}

func TestTenantsPortal(t *testing.T) {
	var got captured
	api := mockAPI(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/tenants/acme/portal": recordJSON(&got, http.StatusOK, map[string]any{
			"redirect_url": "https://portal.example.com/s/abc", "tenant_id": "acme",
		}),
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

	result := callTool(t, session, "outpost_tenants", map[string]any{
		"action": "portal", "id": "acme", "theme": "dark",
	})
	require.False(t, result.IsError, resultText(t, result))

	assert.Equal(t, "/2025-07-01/tenants/acme/portal", got.path)
	assert.Equal(t, "theme=dark", got.query, "the theme has to reach the API or the flag does nothing")
	assert.Contains(t, string(envelopeData(t, resultText(t, result))), "https://portal.example.com/s/abc")
}

// ---------------------------------------------------------------------------
// outpost_destinations — every action
// ---------------------------------------------------------------------------

func TestDestinationsGet(t *testing.T) {
	var got captured
	api := mockAPI(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/tenants/acme/destinations/des_1": recordJSON(&got, http.StatusOK, map[string]any{
			"id": "des_1", "type": "webhook", "topics": "*",
		}),
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

	result := callTool(t, session, "outpost_destinations", map[string]any{
		"action": "get", "tenant_id": "acme", "id": "des_1",
	})
	require.False(t, result.IsError, resultText(t, result))
	assert.Equal(t, "/2025-07-01/tenants/acme/destinations/des_1", got.path)
}

func TestDestinationsCreate(t *testing.T) {
	var got captured
	api := mockAPI(t, map[string]http.HandlerFunc{
		"POST /2025-07-01/tenants/acme/destinations": recordJSON(&got, http.StatusCreated, map[string]any{
			"id": "des_1", "type": "webhook", "topics": []string{"user.created"},
		}),
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

	result := callTool(t, session, "outpost_destinations", map[string]any{
		"action":      "create",
		"tenant_id":   "acme",
		"type":        "webhook",
		"topics":      []any{"user.created"},
		"config":      map[string]any{"url": "https://example.com/hooks"},
		"credentials": map[string]any{"secret": "shh"},
		"filter":      map[string]any{"data": map[string]any{"tier": "pro"}},
		"metadata":    map[string]any{"owner": "platform"},
	})
	require.False(t, result.IsError, resultText(t, result))

	assert.Equal(t, http.MethodPost, got.method)
	assert.Equal(t, "/2025-07-01/tenants/acme/destinations", got.path)

	body := got.decodeBody(t)
	assert.Equal(t, "webhook", body["type"])
	assert.Equal(t, []any{"user.created"}, body["topics"])
	assert.Equal(t, map[string]any{"url": "https://example.com/hooks"}, body["config"])
	assert.Equal(t, map[string]any{"secret": "shh"}, body["credentials"])
	assert.Equal(t, map[string]any{"data": map[string]any{"tier": "pro"}}, body["filter"])
	assert.Equal(t, map[string]any{"owner": "platform"}, body["metadata"])
}

// The API documents subscribing to everything as the bare string "*", not
// ["*"], and rejects the array form. The encoding lives in OutpostTopics, so
// this asserts it survives the round trip from an MCP argument.
func TestDestinationsCreateSendsTheWildcardAsAString(t *testing.T) {
	var got captured
	api := mockAPI(t, map[string]http.HandlerFunc{
		"POST /2025-07-01/tenants/acme/destinations": recordJSON(&got, http.StatusCreated, map[string]any{
			"id": "des_1", "type": "webhook", "topics": "*",
		}),
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

	result := callTool(t, session, "outpost_destinations", map[string]any{
		"action": "create", "tenant_id": "acme", "type": "webhook",
		"topics": []any{"*"},
		"config": map[string]any{"url": "https://example.com/hooks"},
	})
	require.False(t, result.IsError, resultText(t, result))
	assert.Equal(t, "*", got.decodeBody(t)["topics"])
}

func TestDestinationsCreateRequiresAType(t *testing.T) {
	api := mockAPI(t, nil)
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

	result := callTool(t, session, "outpost_destinations", map[string]any{
		"action": "create", "tenant_id": "acme",
	})
	require.True(t, result.IsError)
	assert.Contains(t, resultText(t, result), "type is required")
}

func TestDestinationsUpdateIsAPatch(t *testing.T) {
	var got captured
	api := mockAPI(t, map[string]http.HandlerFunc{
		"PATCH /2025-07-01/tenants/acme/destinations/des_1": recordJSON(&got, http.StatusOK, map[string]any{
			"id": "des_1", "type": "webhook", "topics": "*",
		}),
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

	result := callTool(t, session, "outpost_destinations", map[string]any{
		"action": "update", "tenant_id": "acme", "id": "des_1",
		"config": map[string]any{"url": "https://example.com/new"},
	})
	require.False(t, result.IsError, resultText(t, result))

	// A PUT here would replace the destination rather than merge into it.
	assert.Equal(t, http.MethodPatch, got.method)
	assert.Equal(t, "/2025-07-01/tenants/acme/destinations/des_1", got.path)

	body := got.decodeBody(t)
	assert.Equal(t, map[string]any{"url": "https://example.com/new"}, body["config"])
	// Fields the caller did not mention must not be sent, or the merge-patch
	// clears them.
	assert.NotContains(t, body, "topics")
	assert.NotContains(t, body, "credentials")
	assert.NotContains(t, body, "metadata")
}

func TestDestinationsDelete(t *testing.T) {
	var got captured
	api := mockAPI(t, map[string]http.HandlerFunc{
		"DELETE /2025-07-01/tenants/acme/destinations/des_1": recordJSON(&got, http.StatusOK, nil),
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

	result := callTool(t, session, "outpost_destinations", map[string]any{
		"action": "delete", "tenant_id": "acme", "id": "des_1",
	})
	require.False(t, result.IsError, resultText(t, result))

	assert.Equal(t, http.MethodDelete, got.method)
	assert.Equal(t, "/2025-07-01/tenants/acme/destinations/des_1", got.path)
	assert.JSONEq(t, `{"tenant_id":"acme","destination_id":"des_1","status":"deleted"}`,
		string(envelopeData(t, resultText(t, result))))
}

func TestDestinationsEnableAndDisable(t *testing.T) {
	for _, action := range []string{"enable", "disable"} {
		t.Run(action, func(t *testing.T) {
			var got captured
			api := mockAPI(t, map[string]http.HandlerFunc{
				"PUT /2025-07-01/tenants/acme/destinations/des_1/" + action: recordJSON(&got, http.StatusOK, map[string]any{
					"id": "des_1", "type": "webhook", "topics": "*",
				}),
			})
			session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

			result := callTool(t, session, "outpost_destinations", map[string]any{
				"action": action, "tenant_id": "acme", "id": "des_1",
			})
			require.False(t, result.IsError, resultText(t, result))

			assert.Equal(t, http.MethodPut, got.method)
			assert.Equal(t, "/2025-07-01/tenants/acme/destinations/des_1/"+action, got.path)
		})
	}
}

func TestDestinationsWriteActionsRequireADestinationID(t *testing.T) {
	api := mockAPI(t, nil)
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

	for _, action := range []string{"update", "delete", "enable", "disable"} {
		t.Run(action, func(t *testing.T) {
			result := callTool(t, session, "outpost_destinations", map[string]any{
				"action": action, "tenant_id": "acme",
			})
			require.True(t, result.IsError)
			assert.Contains(t, resultText(t, result), "id is required")
		})
	}
}

// ---------------------------------------------------------------------------
// outpost_config — the reads, and the custom-domain writes
// ---------------------------------------------------------------------------

func TestConfigGet(t *testing.T) {
	var got captured
	api := mockAPI(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/config": recordJSON(&got, http.StatusOK, map[string]any{
			"TOPICS": "user.created", "MAX_RETRY_LIMIT": "5",
		}),
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

	t.Run("everything", func(t *testing.T) {
		result := callTool(t, session, "outpost_config", map[string]any{"action": "get"})
		require.False(t, result.IsError, resultText(t, result))
		assert.Equal(t, "/2025-07-01/config", got.path)

		data := envelopeData(t, resultText(t, result))
		assert.Contains(t, string(data), "TOPICS")
		assert.Contains(t, string(data), "MAX_RETRY_LIMIT")
	})

	t.Run("one key", func(t *testing.T) {
		result := callTool(t, session, "outpost_config", map[string]any{"action": "get", "key": "TOPICS"})
		require.False(t, result.IsError, resultText(t, result))
		assert.JSONEq(t, `{"TOPICS":"user.created"}`, string(envelopeData(t, resultText(t, result))))
	})

	t.Run("an unknown key is named in the error", func(t *testing.T) {
		result := callTool(t, session, "outpost_config", map[string]any{"action": "get", "key": "NOPE"})
		require.True(t, result.IsError)
		assert.Contains(t, resultText(t, result), `no configuration key named "NOPE"`)
	})
}

func TestConfigCustomDomainGet(t *testing.T) {
	var got captured
	api := mockAPI(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/config/custom_domain": recordJSON(&got, http.StatusOK, map[string]any{
			"hostname": "portal.example.com", "status": "pending",
		}),
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

	result := callTool(t, session, "outpost_config", map[string]any{"action": "custom_domain_get"})
	require.False(t, result.IsError, resultText(t, result))

	assert.Equal(t, "/2025-07-01/config/custom_domain", got.path)
	assert.Contains(t, string(envelopeData(t, resultText(t, result))), "portal.example.com")
}

func TestConfigCustomDomainSet(t *testing.T) {
	var got captured
	api := mockAPI(t, map[string]http.HandlerFunc{
		"POST /2025-07-01/config/custom_domain": recordJSON(&got, http.StatusCreated, map[string]any{
			"hostname": "portal.example.com",
			"status":   "pending",
			"verification": []map[string]any{
				{"type": "CNAME", "name": "portal", "value": "outpost.hookdeck.com"},
			},
		}),
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

	result := callTool(t, session, "outpost_config", map[string]any{
		"action": "custom_domain_set", "hostname": "portal.example.com",
	})
	require.False(t, result.IsError, resultText(t, result))

	assert.Equal(t, http.MethodPost, got.method)
	assert.Equal(t, "/2025-07-01/config/custom_domain", got.path)
	assert.Equal(t, "portal.example.com", got.decodeBody(t)["hostname"])
	// The DNS records are the only actionable part of the response; dropping
	// them would leave the domain permanently unverified.
	assert.Contains(t, string(envelopeData(t, resultText(t, result))), "CNAME")
}

func TestConfigCustomDomainSetRequiresAHostname(t *testing.T) {
	api := mockAPI(t, nil)
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

	result := callTool(t, session, "outpost_config", map[string]any{"action": "custom_domain_set"})
	require.True(t, result.IsError)
	assert.Contains(t, resultText(t, result), "hostname is required")
}

func TestConfigCustomDomainDelete(t *testing.T) {
	var got captured
	api := mockAPI(t, map[string]http.HandlerFunc{
		"DELETE /2025-07-01/config/custom_domain": recordJSON(&got, http.StatusOK, nil),
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

	result := callTool(t, session, "outpost_config", map[string]any{"action": "custom_domain_delete"})
	require.False(t, result.IsError, resultText(t, result))

	assert.Equal(t, http.MethodDelete, got.method)
	assert.Equal(t, "/2025-07-01/config/custom_domain", got.path)
	assert.JSONEq(t, `{"status":"deleted"}`, string(envelopeData(t, resultText(t, result))))
}

// ---------------------------------------------------------------------------
// outpost_attempts — no test called this tool with any action
// ---------------------------------------------------------------------------

func TestAttemptsList(t *testing.T) {
	var got captured
	api := mockAPI(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/attempts": recordJSON(&got, http.StatusOK, map[string]any{
			"models":     []map[string]any{{"id": "att_1", "status": "failed", "code": "500"}},
			"pagination": map[string]any{"limit": 10},
		}),
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

	result := callTool(t, session, "outpost_attempts", map[string]any{
		"action":      "list",
		"event_id":    "evt_1",
		"status":      "failed",
		"topic":       "user.created,user.updated",
		"include":     []any{"event"},
		"time_after":  "2026-08-01T00:00:00Z",
		"time_before": "2026-08-14T00:00:00Z",
		"limit":       10,
	})
	require.False(t, result.IsError, resultText(t, result))

	assert.Equal(t, "/2025-07-01/attempts", got.path)
	// Repeated values use indexed brackets; repeating the bare key is not
	// equivalent for this API.
	assert.Contains(t, got.query, "event_id%5B0%5D=evt_1")
	assert.Contains(t, got.query, "topic%5B0%5D=user.created")
	assert.Contains(t, got.query, "topic%5B1%5D=user.updated")
	assert.Contains(t, got.query, "include%5B0%5D=event")
	assert.Contains(t, got.query, "status=failed")
	assert.Contains(t, got.query, "time%5Bgte%5D=2026-08-01T00%3A00%3A00Z")
	assert.Contains(t, got.query, "time%5Blte%5D=2026-08-14T00%3A00%3A00Z")
	assert.Contains(t, string(envelopeData(t, resultText(t, result))), "att_1")
}

// Models routinely quote numbers. limit: "5" used to fall through to the
// default of 0, so the tool silently returned the API's page size instead of
// the one that was asked for.
func TestListsAcceptLimitAsAString(t *testing.T) {
	for _, tc := range []struct {
		tool string
		path string
		args map[string]any
	}{
		{"outpost_attempts", "/2025-07-01/attempts", map[string]any{}},
		{"outpost_events", "/2025-07-01/events", map[string]any{}},
		{"outpost_tenants", "/2025-07-01/tenants", map[string]any{}},
	} {
		t.Run(tc.tool, func(t *testing.T) {
			var got captured
			api := mockAPI(t, map[string]http.HandlerFunc{
				"GET " + tc.path: recordJSON(&got, http.StatusOK, map[string]any{
					"models": []map[string]any{{"id": "x_1"}},
				}),
			})
			session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

			args := map[string]any{"action": "list", "limit": "5"}
			for k, v := range tc.args {
				args[k] = v
			}
			result := callTool(t, session, tc.tool, args)
			require.False(t, result.IsError, resultText(t, result))

			assert.Contains(t, got.query, "limit=5",
				"a quoted limit must not fall back to the API's page size")
		})
	}
}

// eligible_for_retry is a *bool so that "not supplied" stays distinct from
// false. A quoted "false" was read as not supplied, so the event was published
// with the API's default rather than the caller's choice.
func TestPublishAcceptsEligibleForRetryAsAString(t *testing.T) {
	var body map[string]any
	api := mockAPI(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/tenants/acme": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "acme"})
		},
		"POST /2025-07-01/publish": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&body)
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "evt_1", "destination_ids": []string{"des_1"}})
		},
	})
	session := connect(t, ServerOptions{
		Client: newTestClient(t, api.URL), WriteEnabled: true, PublishAPIKey: "project-api-key",
	})

	result := callTool(t, session, "outpost_publish", map[string]any{
		"action": "publish", "tenant_id": "acme", "topic": "user.created",
		"data": map[string]any{"user_id": "123"}, "eligible_for_retry": "false",
	})
	require.False(t, result.IsError, resultText(t, result))
	assert.Equal(t, false, body["eligible_for_retry"],
		"a quoted false must reach the API rather than being read as unset")
}

// A single tenant and destination address the nested route; anything else has
// to fall back to the global one, because the nested path cannot express two.
func TestAttemptsListUsesTheTenantScopedRoute(t *testing.T) {
	var got captured
	api := mockAPI(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/tenants/acme/destinations/des_1/attempts": recordJSON(&got, http.StatusOK, map[string]any{
			"models": []map[string]any{{"id": "att_1"}},
		}),
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

	result := callTool(t, session, "outpost_attempts", map[string]any{
		"action": "list", "tenant_id": "acme", "destination_id": "des_1",
	})
	require.False(t, result.IsError, resultText(t, result))
	assert.Equal(t, "/2025-07-01/tenants/acme/destinations/des_1/attempts", got.path)
}

func TestAttemptsGet(t *testing.T) {
	var got captured
	api := mockAPI(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/attempts/att_1": recordJSON(&got, http.StatusOK, map[string]any{
			"id": "att_1", "status": "failed", "code": "500",
			"response_data": map[string]any{"body": "boom"},
		}),
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

	result := callTool(t, session, "outpost_attempts", map[string]any{
		"action": "get", "id": "att_1", "include": []any{"destination"},
	})
	require.False(t, result.IsError, resultText(t, result))

	assert.Equal(t, "/2025-07-01/attempts/att_1", got.path)
	assert.Contains(t, got.query, "include%5B0%5D=destination")
	// The response body is why anyone looks at an attempt.
	assert.Contains(t, string(envelopeData(t, resultText(t, result))), "boom")
}

func TestAttemptsGetRequiresAnID(t *testing.T) {
	api := mockAPI(t, nil)
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

	result := callTool(t, session, "outpost_attempts", map[string]any{"action": "get"})
	require.True(t, result.IsError)
	assert.Contains(t, resultText(t, result), "id is required")
}

// ---------------------------------------------------------------------------
// outpost_events — the reads
// ---------------------------------------------------------------------------

func TestEventsList(t *testing.T) {
	var got captured
	api := mockAPI(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/events": recordJSON(&got, http.StatusOK, map[string]any{
			"models":     []map[string]any{{"id": "evt_1", "topic": "user.created"}},
			"pagination": map[string]any{"limit": 5, "next": "cursor_2"},
		}),
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

	result := callTool(t, session, "outpost_events", map[string]any{
		"action": "list", "tenant_id": "acme", "topic": "user.created",
		"limit": 5, "dir": "desc", "next": "cursor_1",
	})
	require.False(t, result.IsError, resultText(t, result))

	assert.Equal(t, "/2025-07-01/events", got.path)
	assert.Contains(t, got.query, "tenant_id%5B0%5D=acme")
	assert.Contains(t, got.query, "topic%5B0%5D=user.created")
	assert.Contains(t, got.query, "limit=5")
	assert.Contains(t, got.query, "dir=desc")
	// Pagination is only usable if the cursor is actually forwarded.
	assert.Contains(t, got.query, "next=cursor_1")
	assert.Contains(t, string(envelopeData(t, resultText(t, result))), "cursor_2")
}

func TestEventsGet(t *testing.T) {
	var got captured
	api := mockAPI(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/events/evt_1": recordJSON(&got, http.StatusOK, map[string]any{
			"id": "evt_1", "topic": "user.created", "data": map[string]any{"user_id": "123"},
		}),
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

	result := callTool(t, session, "outpost_events", map[string]any{
		"action": "get", "id": "evt_1", "tenant_id": "acme",
	})
	require.False(t, result.IsError, resultText(t, result))

	assert.Equal(t, "/2025-07-01/events/evt_1", got.path)
	assert.Equal(t, "tenant_id=acme", got.query)
	assert.Contains(t, string(envelopeData(t, resultText(t, result))), "user_id")
}

func TestEventsGetRequiresAnID(t *testing.T) {
	api := mockAPI(t, nil)
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

	result := callTool(t, session, "outpost_events", map[string]any{"action": "get"})
	require.True(t, result.IsError)
	assert.Contains(t, resultText(t, result), "id is required")
}

// ---------------------------------------------------------------------------
// The read-only catalogues
// ---------------------------------------------------------------------------

func TestDestinationTypesGet(t *testing.T) {
	var got captured
	api := mockAPI(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/destination-types/webhook": recordJSON(&got, http.StatusOK, map[string]any{
			"type":         "webhook",
			"label":        "Webhook",
			"icon":         "<svg>icon</svg>",
			"instructions": "a very long setup guide",
			"config_fields": []map[string]any{
				{"key": "url", "type": "text", "required": true},
			},
		}),
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

	result := callTool(t, session, "outpost_destination_types", map[string]any{
		"action": "get", "type": "webhook",
	})
	require.False(t, result.IsError, resultText(t, result))

	assert.Equal(t, "/2025-07-01/destination-types/webhook", got.path)
	text := resultText(t, result)
	assert.Contains(t, text, "url", "the config fields are the reason to call this")
	assert.NotContains(t, text, "a very long setup guide", "setup docs are opt-in on get too")

	verbose := callTool(t, session, "outpost_destination_types", map[string]any{
		"action": "get", "type": "webhook", "include_setup_docs": true,
	})
	assert.Contains(t, resultText(t, verbose), "a very long setup guide")
}

func TestDestinationTypesGetRequiresAType(t *testing.T) {
	api := mockAPI(t, nil)
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

	result := callTool(t, session, "outpost_destination_types", map[string]any{"action": "get"})
	require.True(t, result.IsError)
	assert.Contains(t, resultText(t, result), "type is required")
}

func TestTopicsList(t *testing.T) {
	var got captured
	api := mockAPI(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/topics": recordJSON(&got, http.StatusOK, []string{"user.created", "user.deleted"}),
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

	result := callTool(t, session, "outpost_topics", map[string]any{"action": "list"})
	require.False(t, result.IsError, resultText(t, result))

	assert.Equal(t, "/2025-07-01/topics", got.path)
	assert.JSONEq(t, `{"topics":["user.created","user.deleted"]}`,
		string(envelopeData(t, resultText(t, result))))
}

func TestStatusGet(t *testing.T) {
	var got captured
	api := mockAPI(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/status": recordJSON(&got, http.StatusOK, map[string]any{
			"status": "ready", "version": "1.2.3", "portal_hostname": "portal.example.com",
		}),
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

	result := callTool(t, session, "outpost_status", map[string]any{"action": "get"})
	require.False(t, result.IsError, resultText(t, result))

	assert.Equal(t, "/2025-07-01/status", got.path)
	assert.Contains(t, string(envelopeData(t, resultText(t, result))), "portal.example.com")
}

// ---------------------------------------------------------------------------
// outpost_metrics — the events action was never called
// ---------------------------------------------------------------------------

func TestMetricsEvents(t *testing.T) {
	var got captured
	api := mockAPI(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/metrics/events": recordJSON(&got, http.StatusOK, map[string]any{
			"data": []map[string]any{
				{"dimensions": map[string]string{"topic": "user.created"}, "metrics": map[string]any{"count": 42}},
			},
			"metadata": map[string]any{"row_count": 1},
		}),
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

	result := callTool(t, session, "outpost_metrics", map[string]any{
		"action":      "events",
		"start":       "2026-08-01T00:00:00Z",
		"end":         "2026-08-14T00:00:00Z",
		"granularity": "1d",
		"measures":    []any{"count"},
		"dimensions":  []any{"topic"},
	})
	require.False(t, result.IsError, resultText(t, result))

	assert.Equal(t, "/2025-07-01/metrics/events", got.path)
	// The range uses bracketed keys, not start/end.
	assert.Contains(t, got.query, "time%5Bstart%5D=2026-08-01T00%3A00%3A00Z")
	assert.Contains(t, got.query, "time%5Bend%5D=2026-08-14T00%3A00%3A00Z")
	assert.Contains(t, got.query, "granularity=1d")
	assert.Contains(t, got.query, "measures%5B0%5D=count")
	assert.Contains(t, got.query, "dimensions%5B0%5D=topic")
	assert.Contains(t, string(envelopeData(t, resultText(t, result))), "42")
}

// ---------------------------------------------------------------------------
// Coverage gate
// ---------------------------------------------------------------------------

// TestEveryActionHasBeenCalledSuccessfully is a checklist rather than a
// behaviour test. It fails when an action is added without a successful call
// being written for it, which is the gap this file was created to close: a
// destructive action whose only coverage is its refusal has never been proven
// to work at all.
func TestEveryActionHasBeenCalledSuccessfully(t *testing.T) {
	// Actions with a test above that calls them and asserts the request sent.
	covered := map[string]map[string]bool{
		"tenants": {
			"list": true, "get": true, "upsert": true,
			"delete": true, "token": true, "portal": true,
		},
		"destinations": {
			"list": true, "get": true, "create": true, "update": true,
			"delete": true, "enable": true, "disable": true,
		},
		"events":            {"list": true, "get": true, "retry": true},
		"attempts":          {"list": true, "get": true},
		"topics":            {"list": true},
		"destination_types": {"list": true, "get": true},
		"metrics":           {"events": true, "attempts": true},
		"config": {
			"get": true, "set": true, "custom_domain_get": true,
			"custom_domain_set": true, "custom_domain_delete": true,
		},
		"status": {"get": true},
	}

	specs := map[string]mcpcore.ActionSet{
		"tenants":           tenantsActions,
		"destinations":      destinationsActions,
		"events":            eventsActions,
		"attempts":          attemptsActions,
		"topics":            topicsActions,
		"destination_types": destinationTypesActions,
		"metrics":           metricsActions,
		"config":            configActions,
		"status":            statusActions,
	}

	for resource, actions := range specs {
		for _, a := range actions {
			assert.True(t, covered[resource][a.Name],
				"outpost_%s action %q has no test making a successful call; "+
					"a refusal test alone does not prove the action works",
				resource, a.Name)
		}
	}
}
