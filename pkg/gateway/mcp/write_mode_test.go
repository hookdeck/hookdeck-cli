package mcp

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// listTools returns the advertised tools keyed by name.
func listTools(t *testing.T, session *mcpsdk.ClientSession) map[string]*mcpsdk.Tool {
	t.Helper()
	result, err := session.ListTools(t.Context(), nil)
	require.NoError(t, err)

	tools := make(map[string]*mcpsdk.Tool, len(result.Tools))
	for _, tool := range result.Tools {
		tools[tool.Name] = tool
	}
	return tools
}

// actionEnum returns the action enum a tool advertises.
func actionEnum(t *testing.T, tool *mcpsdk.Tool) []string {
	t.Helper()
	require.NotNil(t, tool)
	// The SDK reports the schema back as decoded JSON, so re-encode it rather
	// than assuming a concrete type.
	raw, err := json.Marshal(tool.InputSchema)
	require.NoError(t, err)

	var schema struct {
		Properties struct {
			Action struct {
				Enum []string `json:"enum"`
			} `json:"action"`
		} `json:"properties"`
	}
	require.NoError(t, json.Unmarshal(raw, &schema))
	return schema.Properties.Action.Enum
}

// ---------------------------------------------------------------------------
// Tool registration and the write-mode gate
// ---------------------------------------------------------------------------

func TestListTools_ReadOnlyMode(t *testing.T) {
	api := mockAPI(t, nil)
	session := connectInMemory(t, newTestClient(api.URL, "test-key"))
	tools := listTools(t, session)

	t.Run("registers every tool", func(t *testing.T) {
		for _, name := range []string{
			"hookdeck_projects", "hookdeck_login", "gateway_help",
			"gateway_connections", "gateway_sources", "gateway_destinations",
			"gateway_transformations", "gateway_requests", "gateway_events",
			"gateway_attempts", "gateway_issues", "gateway_metrics",
		} {
			assert.Contains(t, tools, name)
		}
	})

	t.Run("the old hookdeck_ product names are gone", func(t *testing.T) {
		for _, name := range []string{
			"hookdeck_connections", "hookdeck_sources", "hookdeck_destinations",
			"hookdeck_transformations", "hookdeck_requests", "hookdeck_events",
			"hookdeck_attempts", "hookdeck_issues", "hookdeck_metrics", "hookdeck_help",
		} {
			assert.NotContains(t, tools, name)
		}
	})

	t.Run("write actions are absent from the action enum", func(t *testing.T) {
		assert.Equal(t, []string{"list", "get", "pause", "unpause"}, actionEnum(t, tools["gateway_connections"]))
		assert.Equal(t, []string{"list", "get"}, actionEnum(t, tools["gateway_sources"]))
		assert.Equal(t, []string{"list", "get"}, actionEnum(t, tools["gateway_destinations"]))
		assert.Equal(t, []string{"list", "get"}, actionEnum(t, tools["gateway_transformations"]))
		assert.Equal(t, []string{"list", "get", "raw_body"}, actionEnum(t, tools["gateway_events"]))
		assert.Equal(t, []string{"list", "get", "raw_body", "events", "ignored_events"}, actionEnum(t, tools["gateway_requests"]))
		assert.Equal(t, []string{"list", "get"}, actionEnum(t, tools["gateway_issues"]))
	})

	// This is the regression gate for the pause/unpause decision: they are
	// mutations, and they stay offered in the mode people investigate in.
	t.Run("pause and unpause remain available", func(t *testing.T) {
		enum := actionEnum(t, tools["gateway_connections"])
		assert.Contains(t, enum, "pause")
		assert.Contains(t, enum, "unpause")
	})

	t.Run("write actions are absent from the description", func(t *testing.T) {
		for _, name := range []string{
			"gateway_connections", "gateway_sources", "gateway_destinations",
			"gateway_transformations", "gateway_events", "gateway_requests", "gateway_issues",
		} {
			description := tools[name].Description
			for _, action := range []string{"create", "upsert", "update", "delete", "retry", "cancel", "mute", "dismiss", "run"} {
				assert.NotContains(t, description, " "+action+" (", "%s should not describe the %s action", name, action)
			}
		}
	})

	t.Run("read-only mode is stated in the description", func(t *testing.T) {
		assert.Contains(t, tools["gateway_events"].Description, "read-only mode")
		assert.Contains(t, tools["gateway_events"].Description, "gateway_help")
	})

	t.Run("tools are annotated read-only", func(t *testing.T) {
		for _, name := range []string{
			"gateway_connections", "gateway_sources", "gateway_events",
			"gateway_attempts", "gateway_metrics",
		} {
			require.NotNil(t, tools[name].Annotations, name)
			assert.True(t, tools[name].Annotations.ReadOnlyHint, "%s should be annotated read-only", name)
		}
	})
}

func TestListTools_WriteMode(t *testing.T) {
	api := mockAPI(t, nil)
	session := connectInMemoryWriteEnabled(t, newTestClient(api.URL, "test-key"))
	tools := listTools(t, session)

	t.Run("write actions appear in the enum", func(t *testing.T) {
		assert.Equal(t,
			[]string{"list", "get", "pause", "unpause", "create", "upsert", "update", "delete", "enable", "disable"},
			actionEnum(t, tools["gateway_connections"]))
		assert.Equal(t,
			[]string{"list", "get", "create", "upsert", "update", "delete", "enable", "disable"},
			actionEnum(t, tools["gateway_sources"]))
		assert.Equal(t,
			[]string{"list", "get", "create", "upsert", "update", "delete", "enable", "disable"},
			actionEnum(t, tools["gateway_destinations"]))
		assert.Equal(t,
			[]string{"list", "get", "create", "upsert", "update", "delete", "run"},
			actionEnum(t, tools["gateway_transformations"]))
		assert.Equal(t,
			[]string{"list", "get", "raw_body", "retry", "cancel", "mute"},
			actionEnum(t, tools["gateway_events"]))
		assert.Equal(t,
			[]string{"list", "get", "raw_body", "events", "ignored_events", "retry"},
			actionEnum(t, tools["gateway_requests"]))
		assert.Equal(t,
			[]string{"list", "get", "update", "dismiss"},
			actionEnum(t, tools["gateway_issues"]))
	})

	t.Run("tools with writes are no longer annotated read-only", func(t *testing.T) {
		assert.False(t, tools["gateway_connections"].Annotations.ReadOnlyHint)
		assert.False(t, tools["gateway_events"].Annotations.ReadOnlyHint)
		assert.True(t, tools["gateway_attempts"].Annotations.ReadOnlyHint,
			"attempts has no write actions in any mode")
		assert.True(t, tools["gateway_metrics"].Annotations.ReadOnlyHint,
			"metrics has no write actions in any mode")
	})

	t.Run("destructive tools carry the destructive hint", func(t *testing.T) {
		for _, name := range []string{
			"gateway_connections", "gateway_sources", "gateway_destinations",
			"gateway_transformations", "gateway_events", "gateway_issues",
		} {
			require.NotNil(t, tools[name].Annotations.DestructiveHint, name)
			assert.True(t, *tools[name].Annotations.DestructiveHint, "%s should be flagged destructive", name)
		}
		require.NotNil(t, tools["gateway_requests"].Annotations.DestructiveHint)
		assert.False(t, *tools["gateway_requests"].Annotations.DestructiveHint,
			"retrying a request destroys nothing")
	})

	t.Run("the read-only notice is gone", func(t *testing.T) {
		assert.NotContains(t, tools["gateway_events"].Description, "read-only mode")
	})
}

// ---------------------------------------------------------------------------
// The handler-level guard (defence in depth)
// ---------------------------------------------------------------------------

func TestWriteGuard_BlocksWriteActionsInReadOnlyMode(t *testing.T) {
	// Every path a blocked action would reach fails the test: a request
	// arriving here means the guard did not stop the call.
	fail := func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("read-only server called the API: %s %s", r.Method, r.URL.Path)
	}
	api := mockAPI(t, map[string]http.HandlerFunc{
		"/2025-07-01/connections":      fail,
		"/2025-07-01/connections/":     fail,
		"/2025-07-01/sources":          fail,
		"/2025-07-01/sources/":         fail,
		"/2025-07-01/destinations":     fail,
		"/2025-07-01/destinations/":    fail,
		"/2025-07-01/transformations":  fail,
		"/2025-07-01/transformations/": fail,
		"/2025-07-01/events/":          fail,
		"/2025-07-01/requests/":        fail,
		"/2025-07-01/issues/":          fail,
	})
	session := connectInMemory(t, newTestClient(api.URL, "test-key"))

	cases := []struct {
		tool string
		args map[string]any
	}{
		{"gateway_connections", map[string]any{"action": "create", "name": "c", "source_id": "src_1", "destination_id": "des_1"}},
		{"gateway_connections", map[string]any{"action": "upsert", "name": "c"}},
		{"gateway_connections", map[string]any{"action": "update", "id": "web_1"}},
		{"gateway_connections", map[string]any{"action": "delete", "id": "web_1"}},
		{"gateway_connections", map[string]any{"action": "enable", "id": "web_1"}},
		{"gateway_connections", map[string]any{"action": "disable", "id": "web_1"}},
		{"gateway_sources", map[string]any{"action": "create", "name": "s", "type": "HTTP"}},
		{"gateway_sources", map[string]any{"action": "delete", "id": "src_1"}},
		{"gateway_destinations", map[string]any{"action": "create", "name": "d", "type": "HTTP"}},
		{"gateway_destinations", map[string]any{"action": "delete", "id": "des_1"}},
		{"gateway_transformations", map[string]any{"action": "create", "name": "t", "code": "return"}},
		{"gateway_transformations", map[string]any{"action": "delete", "id": "trs_1"}},
		{"gateway_transformations", map[string]any{"action": "run", "code": "return request"}},
		{"gateway_events", map[string]any{"action": "retry", "id": "evt_1"}},
		{"gateway_events", map[string]any{"action": "cancel", "id": "evt_1"}},
		{"gateway_events", map[string]any{"action": "mute", "id": "evt_1"}},
		{"gateway_requests", map[string]any{"action": "retry", "id": "req_1"}},
		{"gateway_issues", map[string]any{"action": "update", "id": "iss_1", "status": "RESOLVED"}},
		{"gateway_issues", map[string]any{"action": "dismiss", "id": "iss_1"}},
	}

	for _, tc := range cases {
		t.Run(tc.tool+"/"+tc.args["action"].(string), func(t *testing.T) {
			result := callTool(t, session, tc.tool, tc.args)
			require.True(t, result.IsError)
			text := textContent(t, result)
			assert.Contains(t, text, "read-only mode")
			assert.Contains(t, text, "--allow-write")
		})
	}
}

// TestWriteGuard_PauseIsNotGated is the counterpart to the test above: the two
// mutations that stay available must not be blocked in read-only mode.
func TestWriteGuard_PauseIsNotGated(t *testing.T) {
	api := mockAPI(t, map[string]http.HandlerFunc{
		"/2025-07-01/connections/web_1/pause": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"id": "web_1", "paused_at": "2026-01-01T00:00:00Z"})
		},
		"/2025-07-01/connections/web_1/unpause": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"id": "web_1"})
		},
		"/2025-07-01/connections/web_1": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"id": "web_1"})
		},
	})
	session := connectInMemory(t, newTestClient(api.URL, "test-key"))

	for _, action := range []string{"pause", "unpause"} {
		t.Run(action, func(t *testing.T) {
			result := callTool(t, session, "gateway_connections", map[string]any{
				"action": action, "id": "web_1",
			})
			assert.False(t, result.IsError, "%s must stay available in read-only mode: %s",
				action, textContent(t, result))
		})
	}
}

func TestWriteGuard_AllowsWriteActionsInWriteMode(t *testing.T) {
	var seen []string
	record := func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"id": "res_1"})
	}
	api := mockAPI(t, map[string]http.HandlerFunc{
		"/2025-07-01/events/evt_1/retry":   record,
		"/2025-07-01/requests/req_1/retry": record,
		"/2025-07-01/sources":              record,
		"/2025-07-01/destinations":         record,
		"/2025-07-01/transformations":      record,
		"/2025-07-01/issues/iss_1":         record,
	})
	session := connectInMemoryWriteEnabled(t, newTestClient(api.URL, "test-key"))

	cases := []struct {
		name string
		tool string
		args map[string]any
		want string
	}{
		{"events retry", "gateway_events", map[string]any{"action": "retry", "id": "evt_1"}, "POST /2025-07-01/events/evt_1/retry"},
		{"requests retry", "gateway_requests", map[string]any{"action": "retry", "id": "req_1"}, "POST /2025-07-01/requests/req_1/retry"},
		{"sources create", "gateway_sources", map[string]any{"action": "create", "name": "s", "type": "HTTP"}, "POST /2025-07-01/sources"},
		{"sources upsert", "gateway_sources", map[string]any{"action": "upsert", "name": "s", "type": "HTTP"}, "PUT /2025-07-01/sources"},
		{"destinations create", "gateway_destinations", map[string]any{"action": "create", "name": "d", "type": "HTTP"}, "POST /2025-07-01/destinations"},
		{"transformations create", "gateway_transformations", map[string]any{"action": "create", "name": "t", "code": "return request"}, "POST /2025-07-01/transformations"},
		{"issues update", "gateway_issues", map[string]any{"action": "update", "id": "iss_1", "status": "RESOLVED"}, "PUT /2025-07-01/issues/iss_1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seen = nil
			result := callTool(t, session, tc.tool, tc.args)
			assert.False(t, result.IsError, "unexpected error: %s", textContent(t, result))
			// Assert the request the handler sends, not just that it did not
			// error: a wire-shape bug passes a "no error" assertion.
			assert.Contains(t, seen, tc.want)
		})
	}
}

// TestWriteActions_RequireAnID checks the argument validation on the write
// actions that address one existing record.
func TestWriteActions_RequireAnID(t *testing.T) {
	api := mockAPI(t, nil)
	session := connectInMemoryWriteEnabled(t, newTestClient(api.URL, "test-key"))

	cases := []struct {
		tool string
		args map[string]any
	}{
		{"gateway_events", map[string]any{"action": "retry"}},
		{"gateway_events", map[string]any{"action": "cancel"}},
		{"gateway_events", map[string]any{"action": "mute"}},
		{"gateway_requests", map[string]any{"action": "retry"}},
		{"gateway_sources", map[string]any{"action": "delete"}},
		{"gateway_destinations", map[string]any{"action": "delete"}},
		{"gateway_transformations", map[string]any{"action": "delete"}},
		{"gateway_issues", map[string]any{"action": "dismiss"}},
	}

	for _, tc := range cases {
		t.Run(tc.tool+"/"+tc.args["action"].(string), func(t *testing.T) {
			result := callTool(t, session, tc.tool, tc.args)
			require.True(t, result.IsError)
			assert.Contains(t, textContent(t, result), "id is required")
		})
	}
}

// TestHelpReportsMode checks that gateway_help states which mode the session is
// in, so an agent can find out why an action it expected is missing.
func TestHelpReportsMode(t *testing.T) {
	api := mockAPI(t, nil)

	t.Run("read-only", func(t *testing.T) {
		session := connectInMemory(t, newTestClient(api.URL, "test-key"))
		text := textContent(t, callTool(t, session, "gateway_help", map[string]any{}))
		assert.Contains(t, text, "Mode: read-only")
		assert.Contains(t, text, "--allow-write")
		assert.Contains(t, text, "Pausing and unpausing")
	})

	t.Run("write enabled", func(t *testing.T) {
		session := connectInMemoryWriteEnabled(t, newTestClient(api.URL, "test-key"))
		text := textContent(t, callTool(t, session, "gateway_help", map[string]any{}))
		assert.Contains(t, text, "Mode: write enabled")
	})

	t.Run("a topic only documents the available actions", func(t *testing.T) {
		session := connectInMemory(t, newTestClient(api.URL, "test-key"))
		text := textContent(t, callTool(t, session, "gateway_help", map[string]any{"topic": "gateway_events"}))

		// Scope the assertion to the generated Actions list. Prose further down
		// may legitimately mention what the gated actions do.
		_, rest, ok := strings.Cut(text, "Actions:\n")
		require.True(t, ok, "help topic should have an Actions section")
		actions, _, ok := strings.Cut(rest, "\n\n")
		require.True(t, ok)

		assert.Contains(t, actions, "list")
		for _, gated := range []string{"retry", "cancel", "mute"} {
			assert.NotContains(t, actions, gated,
				"read-only help must not list the %s action", gated)
		}
		assert.Contains(t, text, "unavailable in read-only mode")
	})
}
