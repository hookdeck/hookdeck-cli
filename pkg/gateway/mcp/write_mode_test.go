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

	t.Run("registers every read tool", func(t *testing.T) {
		for _, name := range []string{
			"hookdeck_projects", "hookdeck_login", "gateway_help",
			"gateway_connections_read", "gateway_sources_read", "gateway_destinations_read",
			"gateway_transformations_read", "gateway_requests_read", "gateway_request_read",
			"gateway_events_read", "gateway_event_read",
			"gateway_attempts_read", "gateway_issues_read", "gateway_metrics_read",
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

	// The unsuffixed names were the v3.0.0-beta.1 shape. They are gone too: a
	// grant written against gateway_connections must fail loudly rather than
	// silently match nothing.
	t.Run("the unsuffixed resource names are gone", func(t *testing.T) {
		for _, name := range []string{
			"gateway_connections", "gateway_sources", "gateway_destinations",
			"gateway_transformations", "gateway_requests", "gateway_request",
			"gateway_events", "gateway_event", "gateway_attempts",
			"gateway_issues", "gateway_metrics",
		} {
			assert.NotContains(t, tools, name)
		}
	})

	t.Run("no write tool is registered", func(t *testing.T) {
		for _, name := range []string{
			"gateway_connections_write", "gateway_sources_write", "gateway_destinations_write",
			"gateway_transformations_write", "gateway_request_write", "gateway_event_write",
			"gateway_issues_write",
		} {
			assert.NotContains(t, tools, name)
		}
	})

	t.Run("read tools carry only read actions", func(t *testing.T) {
		assert.Equal(t, []string{"list", "get"}, actionEnum(t, tools["gateway_connections_read"]))
		assert.Equal(t, []string{"list", "get"}, actionEnum(t, tools["gateway_sources_read"]))
		assert.Equal(t, []string{"list", "get"}, actionEnum(t, tools["gateway_destinations_read"]))
		// run stays here: it is a sandbox evaluation that persists nothing, so
		// it is a read, and debugging a transformation is read-only-mode work.
		assert.Equal(t, []string{"list", "get", "run"}, actionEnum(t, tools["gateway_transformations_read"]))
		assert.Equal(t, []string{"list", "list_ignored"}, actionEnum(t, tools["gateway_events_read"]))
		assert.Equal(t, []string{"get", "raw_body"}, actionEnum(t, tools["gateway_event_read"]))
		assert.Equal(t, []string{"list"}, actionEnum(t, tools["gateway_requests_read"]))
		assert.Equal(t, []string{"get", "raw_body"}, actionEnum(t, tools["gateway_request_read"]))
		assert.Equal(t, []string{"list", "get"}, actionEnum(t, tools["gateway_issues_read"]))
	})

	// The regression gate for the pause/unpause decision: they are mutations,
	// and they stay offered in the mode people investigate in — on a tool of
	// their own, so the read tool can still be annotated read-only.
	t.Run("pause and unpause remain available, on their own tool", func(t *testing.T) {
		require.Contains(t, tools, "gateway_connections_pause")
		assert.Equal(t, []string{"pause", "unpause"}, actionEnum(t, tools["gateway_connections_pause"]))
		assert.NotContains(t, actionEnum(t, tools["gateway_connections_read"]), "pause")
	})

	t.Run("read tools do not describe write actions", func(t *testing.T) {
		for _, name := range []string{
			"gateway_connections_read", "gateway_sources_read", "gateway_destinations_read",
			"gateway_transformations_read", "gateway_event_read", "gateway_request_read",
			"gateway_issues_read",
		} {
			description := tools[name].Description
			for _, action := range []string{"create", "upsert", "update", "delete", "retry", "cancel", "mute", "dismiss"} {
				assert.NotContains(t, description, " "+action+" (", "%s should not describe the %s action", name, action)
			}
		}
	})

	t.Run("a read tool says where the write actions live", func(t *testing.T) {
		description := tools["gateway_event_read"].Description
		assert.Contains(t, description, "gateway_event_write")
		assert.Contains(t, description, "--allow-write")
		// The plural tools only search, so nothing is being withheld from them
		// and there is no counterpart to name.
		assert.NotContains(t, tools["gateway_events_read"].Description, "--allow-write")
	})

	t.Run("every read tool is annotated read-only", func(t *testing.T) {
		for _, name := range []string{
			"gateway_connections_read", "gateway_sources_read", "gateway_events_read",
			"gateway_event_read", "gateway_requests_read", "gateway_request_read",
			"gateway_attempts_read", "gateway_metrics_read", "gateway_issues_read",
		} {
			require.NotNil(t, tools[name].Annotations, name)
			assert.True(t, tools[name].Annotations.ReadOnlyHint,
				"%s must be annotated read-only, or it cannot be blanket-allowed", name)
		}
	})

	// gateway_connections_pause is the deliberate exception. It changes
	// delivery, so it must not claim to be a pure read: a client that
	// auto-approves ReadOnlyHint tools would otherwise halt production
	// delivery without asking anyone.
	t.Run("the pause tool is not annotated read-only", func(t *testing.T) {
		require.NotNil(t, tools["gateway_connections_pause"].Annotations)
		assert.False(t, tools["gateway_connections_pause"].Annotations.ReadOnlyHint,
			"pausing changes delivery; the tool must say so")
		require.NotNil(t, tools["gateway_connections_pause"].Annotations.DestructiveHint)
		assert.False(t, *tools["gateway_connections_pause"].Annotations.DestructiveHint,
			"pausing buffers rather than drops, and is reversible")
	})
}

func TestListTools_WriteMode(t *testing.T) {
	api := mockAPI(t, nil)
	session := connectInMemoryWriteEnabled(t, newTestClient(api.URL, "test-key"))
	tools := listTools(t, session)

	t.Run("write tools appear", func(t *testing.T) {
		for _, name := range []string{
			"gateway_connections_write", "gateway_sources_write", "gateway_destinations_write",
			"gateway_transformations_write", "gateway_request_write", "gateway_event_write",
			"gateway_issues_write",
		} {
			assert.Contains(t, tools, name)
		}
	})

	t.Run("write tools carry only gated actions", func(t *testing.T) {
		assert.Equal(t, []string{"create", "upsert", "update", "delete", "enable", "disable"},
			actionEnum(t, tools["gateway_connections_write"]))
		assert.Equal(t, []string{"create", "upsert", "update", "delete"},
			actionEnum(t, tools["gateway_transformations_write"]))
		assert.Equal(t, []string{"retry", "cancel", "mute"}, actionEnum(t, tools["gateway_event_write"]))
		assert.Equal(t, []string{"retry"}, actionEnum(t, tools["gateway_request_write"]))
		assert.Equal(t, []string{"update", "dismiss"}, actionEnum(t, tools["gateway_issues_write"]))
	})

	t.Run("read tools are unchanged by write mode", func(t *testing.T) {
		assert.Equal(t, []string{"list", "get"}, actionEnum(t, tools["gateway_connections_read"]))
		assert.Equal(t, []string{"list", "list_ignored"}, actionEnum(t, tools["gateway_events_read"]))
		assert.Equal(t, []string{"pause", "unpause"}, actionEnum(t, tools["gateway_connections_pause"]))
	})

	t.Run("write tools are annotated honestly", func(t *testing.T) {
		require.NotNil(t, tools["gateway_connections_write"].Annotations)
		assert.False(t, tools["gateway_connections_write"].Annotations.ReadOnlyHint)
		require.NotNil(t, tools["gateway_connections_write"].Annotations.DestructiveHint)
		assert.True(t, *tools["gateway_connections_write"].Annotations.DestructiveHint,
			"the tool carrying delete must say it is destructive")

		// retry creates new events but destroys nothing.
		require.NotNil(t, tools["gateway_request_write"].Annotations.DestructiveHint)
		assert.False(t, *tools["gateway_request_write"].Annotations.DestructiveHint)
	})
}

func TestWriteGuard_BlocksWriteActionsInReadOnlyMode(t *testing.T) {
	// Every path a blocked action would reach fails the test: a request
	// arriving here means the guard did not stop the call.
	fail := func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("read-only server called the API: %s %s", r.Method, r.URL.Path)
	}
	api := mockAPI(t, map[string]http.HandlerFunc{
		"/2026-09-01/connections":      fail,
		"/2026-09-01/connections/":     fail,
		"/2026-09-01/sources":          fail,
		"/2026-09-01/sources/":         fail,
		"/2026-09-01/destinations":     fail,
		"/2026-09-01/destinations/":    fail,
		"/2026-09-01/transformations":  fail,
		"/2026-09-01/transformations/": fail,
		"/2026-09-01/events/":          fail,
		"/2026-09-01/requests/":        fail,
		"/2026-09-01/issues/":          fail,
	})
	session := connectInMemory(t, newTestClient(api.URL, "test-key"))

	cases := []struct {
		tool string
		args map[string]any
	}{
		{"gateway_connections_read", map[string]any{"action": "create", "name": "c", "source_id": "src_1", "destination_id": "des_1"}},
		{"gateway_connections_read", map[string]any{"action": "upsert", "name": "c"}},
		{"gateway_connections_read", map[string]any{"action": "update", "id": "web_1"}},
		{"gateway_connections_read", map[string]any{"action": "delete", "id": "web_1"}},
		{"gateway_connections_read", map[string]any{"action": "enable", "id": "web_1"}},
		{"gateway_connections_read", map[string]any{"action": "disable", "id": "web_1"}},
		{"gateway_sources_read", map[string]any{"action": "create", "name": "s", "type": "HTTP"}},
		{"gateway_sources_read", map[string]any{"action": "delete", "id": "src_1"}},
		{"gateway_destinations_read", map[string]any{"action": "create", "name": "d", "type": "HTTP"}},
		{"gateway_destinations_read", map[string]any{"action": "delete", "id": "des_1"}},
		{"gateway_transformations_read", map[string]any{"action": "create", "name": "t", "code": "return"}},
		{"gateway_transformations_read", map[string]any{"action": "delete", "id": "trs_1"}},
		{"gateway_event_read", map[string]any{"action": "retry", "id": "evt_1"}},
		{"gateway_event_read", map[string]any{"action": "cancel", "id": "evt_1"}},
		{"gateway_event_read", map[string]any{"action": "mute", "id": "evt_1"}},
		{"gateway_request_read", map[string]any{"action": "retry", "id": "req_1"}},
		{"gateway_issues_read", map[string]any{"action": "update", "id": "iss_1", "status": "RESOLVED"}},
		{"gateway_issues_read", map[string]any{"action": "dismiss", "id": "iss_1"}},
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
		"/2026-09-01/connections/web_1/pause": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"id": "web_1", "paused_at": "2026-01-01T00:00:00Z"})
		},
		"/2026-09-01/connections/web_1/unpause": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"id": "web_1"})
		},
		"/2026-09-01/connections/web_1": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"id": "web_1"})
		},
	})
	session := connectInMemory(t, newTestClient(api.URL, "test-key"))

	for _, action := range []string{"pause", "unpause"} {
		t.Run(action, func(t *testing.T) {
			result := callTool(t, session, "gateway_connections_pause", map[string]any{
				"action": action, "id": "web_1",
			})
			assert.False(t, result.IsError, "%s must stay available in read-only mode: %s",
				action, textContent(t, result))
		})
	}
}

// TestWriteGuard_TransformationRunIsNotGated pins run as a read.
//
// A run is a sandbox evaluation: verified against the API, it creates no
// execution record and returns no execution id. Gating it would leave a
// read-only session able to read transformation code but unable to try it,
// which is the debugging work read-only mode is for.
func TestWriteGuard_TransformationRunIsNotGated(t *testing.T) {
	api := mockAPI(t, map[string]http.HandlerFunc{
		"/2026-09-01/transformations/run": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{
				"request": map[string]any{"headers": map[string]any{}},
			})
		},
	})
	session := connectInMemory(t, newTestClient(api.URL, "test-key"))

	result := callTool(t, session, "gateway_transformations_read", map[string]any{
		"action":  "run",
		"code":    "addHandler(\"transform\", (request, context) => { return request; });",
		"request": map[string]any{"headers": map[string]any{}},
	})
	assert.False(t, result.IsError, "run must stay available in read-only mode: %s",
		textContent(t, result))
}

func TestWriteGuard_AllowsWriteActionsInWriteMode(t *testing.T) {
	var seen []string
	record := func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"id": "res_1"})
	}
	api := mockAPI(t, map[string]http.HandlerFunc{
		"/2026-09-01/events/evt_1/retry":   record,
		"/2026-09-01/requests/req_1/retry": record,
		"/2026-09-01/sources":              record,
		"/2026-09-01/destinations":         record,
		"/2026-09-01/transformations":      record,
		"/2026-09-01/issues/iss_1":         record,
	})
	session := connectInMemoryWriteEnabled(t, newTestClient(api.URL, "test-key"))

	cases := []struct {
		name string
		tool string
		args map[string]any
		want string
	}{
		{"event retry", "gateway_event_write", map[string]any{"action": "retry", "id": "evt_1"}, "POST /2026-09-01/events/evt_1/retry"},
		{"request retry", "gateway_request_write", map[string]any{"action": "retry", "id": "req_1"}, "POST /2026-09-01/requests/req_1/retry"},
		{"sources create", "gateway_sources_write", map[string]any{"action": "create", "name": "s", "type": "HTTP"}, "POST /2026-09-01/sources"},
		{"sources upsert", "gateway_sources_write", map[string]any{"action": "upsert", "name": "s", "type": "HTTP"}, "PUT /2026-09-01/sources"},
		{"destinations create", "gateway_destinations_write", map[string]any{"action": "create", "name": "d", "type": "HTTP"}, "POST /2026-09-01/destinations"},
		{"transformations create", "gateway_transformations_write", map[string]any{"action": "create", "name": "t", "code": "return request"}, "POST /2026-09-01/transformations"},
		{"issues update", "gateway_issues_write", map[string]any{"action": "update", "id": "iss_1", "status": "RESOLVED"}, "PUT /2026-09-01/issues/iss_1"},
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
		{"gateway_event_write", map[string]any{"action": "retry"}},
		{"gateway_event_write", map[string]any{"action": "cancel"}},
		{"gateway_event_write", map[string]any{"action": "mute"}},
		{"gateway_request_write", map[string]any{"action": "retry"}},
		{"gateway_sources_write", map[string]any{"action": "delete"}},
		{"gateway_destinations_write", map[string]any{"action": "delete"}},
		{"gateway_transformations_write", map[string]any{"action": "delete"}},
		{"gateway_issues_write", map[string]any{"action": "dismiss"}},
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
		text := textContent(t, callTool(t, session, "gateway_help", map[string]any{"topic": "gateway_event"}))

		// Scope the assertion to the generated Actions list. Prose further down
		// may legitimately mention what the gated actions do.
		_, rest, ok := strings.Cut(text, "Actions:\n")
		require.True(t, ok, "help topic should have an Actions section")
		actions, _, ok := strings.Cut(rest, "\n\n")
		require.True(t, ok)

		assert.Contains(t, actions, "get")
		for _, gated := range []string{"retry", "cancel", "mute"} {
			assert.NotContains(t, actions, gated,
				"read-only help must not list the %s action", gated)
		}
		// The topic names its write counterpart and the flag that registers it.
		// It deliberately does not say "unavailable in read-only mode": a read
		// tool's help has to read identically in both modes, or a grant written
		// against it stops describing the same tool. See
		// TestReadToolsAreIdenticalInBothModes.
		assert.Contains(t, text, "gateway_event_write")
		assert.Contains(t, text, "--allow-write")
	})
}
