package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// mockAPI serves the given Outpost API paths and 404s anything else, so an
// unexpected call fails the test loudly rather than hanging.
func mockAPI(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for pattern, handler := range handlers {
		mux.HandleFunc(pattern, handler)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t.Logf("unhandled request: %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "not found: " + r.URL.Path})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newTestClient(t *testing.T, baseURL string) *hookdeck.Client {
	t.Helper()
	u, err := url.Parse(baseURL)
	require.NoError(t, err)
	return &hookdeck.Client{
		BaseURL:   u,
		APIKey:    "test-key",
		ProjectID: "proj_outpost",
		// Set so the server does not go looking up the display name, which is
		// not what these tests are about.
		ProjectName:            "outpost-test",
		AcceptAnySuccessStatus: true,
	}
}

// connect starts the server over an in-memory transport and returns a client
// session, exercising the same registration path as the real stdio server.
func connect(t *testing.T, opts ServerOptions) *mcpsdk.ClientSession {
	t.Helper()
	if opts.Config == nil {
		opts.Config = &config.Config{}
	}
	srv := NewServer(opts)

	serverTransport, clientTransport := mcpsdk.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = srv.Run(ctx, serverTransport) }()

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func listTools(t *testing.T, session *mcpsdk.ClientSession) map[string]*mcpsdk.Tool {
	t.Helper()
	result, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	tools := make(map[string]*mcpsdk.Tool, len(result.Tools))
	for _, tool := range result.Tools {
		tools[tool.Name] = tool
	}
	return tools
}

func callTool(t *testing.T, session *mcpsdk.ClientSession, name string, args map[string]any) *mcpsdk.CallToolResult {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{Name: name, Arguments: args})
	require.NoError(t, err)
	return result
}

func resultText(t *testing.T, result *mcpsdk.CallToolResult) string {
	t.Helper()
	require.NotEmpty(t, result.Content)
	tc, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok, "expected TextContent, got %T", result.Content[0])
	return tc.Text
}

// actionEnum returns the action enum a tool advertises.
func actionEnum(t *testing.T, tool *mcpsdk.Tool) []string {
	t.Helper()
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
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})
	tools := listTools(t, session)

	t.Run("registers every read tool", func(t *testing.T) {
		for _, name := range []string{
			"hookdeck_projects", "hookdeck_login", "outpost_help",
			"outpost_tenants", "outpost_destinations", "outpost_events",
			"outpost_attempts", "outpost_topics", "outpost_destination_types",
			"outpost_metrics", "outpost_config", "outpost_status",
		} {
			assert.Contains(t, tools, name)
		}
	})

	t.Run("omits the publish tool entirely", func(t *testing.T) {
		assert.NotContains(t, tools, "outpost_publish",
			"a tool that could only ever fail must not be advertised")
	})

	t.Run("write actions are absent from the action enum", func(t *testing.T) {
		assert.Equal(t, []string{"list", "get"}, actionEnum(t, tools["outpost_tenants"]))
		assert.Equal(t, []string{"list", "get"}, actionEnum(t, tools["outpost_destinations"]))
		assert.Equal(t, []string{"list", "get"}, actionEnum(t, tools["outpost_events"]))
		assert.Equal(t, []string{"get", "custom_domain_get"}, actionEnum(t, tools["outpost_config"]))
	})

	t.Run("write actions are absent from the description", func(t *testing.T) {
		for _, name := range []string{"outpost_tenants", "outpost_destinations", "outpost_events", "outpost_config"} {
			description := tools[name].Description
			for _, action := range []string{"upsert", "delete", "create", "retry", "set"} {
				assert.NotContains(t, description, " "+action+" (", "%s should not describe the %s action", name, action)
			}
		}
	})

	t.Run("credential-returning reads are treated as writes", func(t *testing.T) {
		enum := actionEnum(t, tools["outpost_tenants"])
		assert.NotContains(t, enum, "token", "a tenant token is a reusable credential")
		assert.NotContains(t, enum, "portal", "a portal URL grants access to tenant data")
	})

	t.Run("read tools are annotated as read-only", func(t *testing.T) {
		for _, name := range []string{"outpost_tenants", "outpost_events", "outpost_attempts", "outpost_status"} {
			require.NotNil(t, tools[name].Annotations, name)
			assert.True(t, tools[name].Annotations.ReadOnlyHint, "%s should be annotated read-only", name)
		}
	})
}

func TestListTools_WriteMode(t *testing.T) {
	api := mockAPI(t, nil)
	session := connect(t, ServerOptions{
		Client:        newTestClient(t, api.URL),
		WriteEnabled:  true,
		PublishAPIKey: "project-api-key",
	})
	tools := listTools(t, session)

	t.Run("write actions appear in the enum", func(t *testing.T) {
		assert.Equal(t, []string{"list", "get", "upsert", "delete", "token", "portal"}, actionEnum(t, tools["outpost_tenants"]))
		assert.Equal(t, []string{"list", "get", "create", "update", "delete", "enable", "disable"}, actionEnum(t, tools["outpost_destinations"]))
		assert.Equal(t, []string{"list", "get", "retry"}, actionEnum(t, tools["outpost_events"]))
	})

	t.Run("publish is registered when a Project API key is available", func(t *testing.T) {
		assert.Contains(t, tools, "outpost_publish")
	})

	t.Run("tools with writes are no longer annotated read-only", func(t *testing.T) {
		assert.False(t, tools["outpost_tenants"].Annotations.ReadOnlyHint)
		assert.True(t, tools["outpost_attempts"].Annotations.ReadOnlyHint, "attempts has no write actions in any mode")
	})

	t.Run("destructive tools carry the destructive hint", func(t *testing.T) {
		for _, name := range []string{"outpost_tenants", "outpost_destinations", "outpost_config", "outpost_publish"} {
			require.NotNil(t, tools[name].Annotations.DestructiveHint, name)
			assert.True(t, *tools[name].Annotations.DestructiveHint, "%s should be flagged destructive", name)
		}
		require.NotNil(t, tools["outpost_events"].Annotations.DestructiveHint)
		assert.False(t, *tools["outpost_events"].Annotations.DestructiveHint, "a retry does not destroy anything")
	})
}

func TestListTools_WriteModeWithoutPublishKey(t *testing.T) {
	api := mockAPI(t, nil)
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})
	tools := listTools(t, session)

	assert.NotContains(t, tools, "outpost_publish",
		"publishing needs a Project API key, which write mode alone does not supply")
	assert.Contains(t, tools, "outpost_tenants")
}

// ---------------------------------------------------------------------------
// The handler-level guard (defence in depth)
// ---------------------------------------------------------------------------

func TestWriteGuard_BlocksWriteActionsInReadOnlyMode(t *testing.T) {
	// The API is left unstubbed: a request reaching it would mean the guard
	// failed to stop the call.
	api := mockAPI(t, map[string]http.HandlerFunc{
		"/2025-07-01/tenants/acme": func(w http.ResponseWriter, r *http.Request) {
			t.Errorf("read-only server called the API: %s %s", r.Method, r.URL.Path)
		},
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

	cases := []struct {
		tool string
		args map[string]any
	}{
		{"outpost_tenants", map[string]any{"action": "upsert", "id": "acme"}},
		{"outpost_tenants", map[string]any{"action": "delete", "id": "acme"}},
		{"outpost_tenants", map[string]any{"action": "token", "id": "acme"}},
		{"outpost_tenants", map[string]any{"action": "portal", "id": "acme"}},
		{"outpost_destinations", map[string]any{"action": "delete", "tenant_id": "acme", "id": "des_1"}},
		{"outpost_events", map[string]any{"action": "retry", "id": "evt_1", "destination_id": "des_1"}},
		{"outpost_config", map[string]any{"action": "set", "values": map[string]any{"TOPICS": "a"}}},
	}

	for _, tc := range cases {
		t.Run(tc.tool+"/"+tc.args["action"].(string), func(t *testing.T) {
			result := callTool(t, session, tc.tool, tc.args)
			require.True(t, result.IsError)
			text := resultText(t, result)
			assert.Contains(t, text, "read-only mode")
			assert.Contains(t, text, "--allow-write")
		})
	}
}

func TestWriteGuard_AllowsWriteActionsInWriteMode(t *testing.T) {
	api := mockAPI(t, map[string]http.HandlerFunc{
		"PUT /2025-07-01/tenants/acme": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "acme", "topics": []string{}})
		},
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

	result := callTool(t, session, "outpost_tenants", map[string]any{
		"action":   "upsert",
		"id":       "acme",
		"metadata": map[string]any{"plan": "pro"},
	})
	require.False(t, result.IsError, resultText(t, result))
	assert.Contains(t, resultText(t, result), `"acme"`)
}

func TestUnknownAction(t *testing.T) {
	api := mockAPI(t, nil)
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

	result := callTool(t, session, "outpost_tenants", map[string]any{"action": "explode"})
	require.True(t, result.IsError)
	text := resultText(t, result)
	assert.Contains(t, text, `unknown action "explode"`)
	assert.Contains(t, text, "list, get")
	assert.NotContains(t, text, "delete", "the error must not advertise actions this session cannot use")
}

// ---------------------------------------------------------------------------
// Authentication
// ---------------------------------------------------------------------------

// TestUnauthenticated_PointsAtALoginToolThatExists guards the platform/product
// prefix split: login is a Hookdeck operation, not an Outpost one, so it keeps
// the platform prefix here and in the Gateway server. An unauthenticated tool
// must name a tool this session actually registers.
func TestUnauthenticated_PointsAtALoginToolThatExists(t *testing.T) {
	api := mockAPI(t, nil)
	client := newTestClient(t, api.URL)
	client.APIKey = ""
	session := connect(t, ServerOptions{Client: client})

	registered := map[string]bool{}
	for name := range listTools(t, session) {
		registered[name] = true
	}
	require.True(t, registered["hookdeck_login"], "login is platform-level, so it is hookdeck_login in every server")
	require.False(t, registered["outpost_login"], "the product prefix must not be used for a platform tool")

	for _, name := range []string{"outpost_tenants", "outpost_events", "outpost_status", "hookdeck_projects"} {
		t.Run(name, func(t *testing.T) {
			result := callTool(t, session, name, map[string]any{"action": "list"})
			require.True(t, result.IsError)

			text := resultText(t, result)
			assert.Contains(t, text, "hookdeck_login")
			// Naming a tool the session does not expose would send an agent
			// chasing something that cannot be called.
			assert.True(t, registered["hookdeck_login"])
		})
	}
}

// ---------------------------------------------------------------------------
// Action set construction
// ---------------------------------------------------------------------------

func TestActionSet(t *testing.T) {
	actions := mcpcore.ActionSet{
		{Name: "list"},
		{Name: "delete", Write: true, Destructive: true},
	}

	t.Run("read-only mode drops writes", func(t *testing.T) {
		assert.Equal(t, []string{"list"}, actions.Available(false).Names())
		assert.False(t, actions.Available(false).HasWrite())
		assert.False(t, actions.Available(false).HasDestructive())
	})

	t.Run("write mode keeps everything", func(t *testing.T) {
		assert.Equal(t, []string{"list", "delete"}, actions.Available(true).Names())
		assert.True(t, actions.Available(true).HasWrite())
		assert.True(t, actions.Available(true).HasDestructive())
	})
}

// ---------------------------------------------------------------------------
// Tool handlers
// ---------------------------------------------------------------------------

func TestTenantsList(t *testing.T) {
	var gotQuery string
	api := mockAPI(t, map[string]http.HandlerFunc{
		"/2025-07-01/tenants": func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models":     []map[string]any{{"id": "acme"}},
				"pagination": map[string]any{"limit": 10},
				"count":      1,
			})
		},
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

	result := callTool(t, session, "outpost_tenants", map[string]any{
		"action": "list",
		"id":     "acme,globex",
		"limit":  10,
	})
	require.False(t, result.IsError, resultText(t, result))

	assert.Contains(t, gotQuery, "id%5B0%5D=acme")
	assert.Contains(t, gotQuery, "id%5B1%5D=globex")
	assert.Contains(t, gotQuery, "limit=10")

	// Successful JSON responses use the shared data/meta envelope.
	var envelope struct {
		Data json.RawMessage `json:"data"`
		Meta struct {
			ActiveProjectID string `json:"active_project_id"`
		} `json:"meta"`
	}
	require.NoError(t, json.Unmarshal([]byte(resultText(t, result)), &envelope))
	assert.Equal(t, "proj_outpost", envelope.Meta.ActiveProjectID)
	assert.Contains(t, string(envelope.Data), "acme")
}

func TestDestinationsRequireTenantID(t *testing.T) {
	api := mockAPI(t, nil)
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

	result := callTool(t, session, "outpost_destinations", map[string]any{"action": "list"})
	require.True(t, result.IsError)
	assert.Contains(t, resultText(t, result), "tenant_id is required")
}

func TestDestinationsList(t *testing.T) {
	var gotQuery string
	api := mockAPI(t, map[string]http.HandlerFunc{
		"/2025-07-01/tenants/acme/destinations": func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "des_1", "type": "webhook", "topics": "*"}})
		},
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

	result := callTool(t, session, "outpost_destinations", map[string]any{
		"action":    "list",
		"tenant_id": "acme",
		"type":      "webhook",
		"topics":    []any{"user.created"},
	})
	require.False(t, result.IsError, resultText(t, result))
	assert.Contains(t, gotQuery, "type%5B0%5D=webhook")
	assert.Contains(t, gotQuery, "topics%5B0%5D=user.created")
	assert.Contains(t, resultText(t, result), "des_1")
}

func TestEventsRetryReportsQueued(t *testing.T) {
	api := mockAPI(t, map[string]http.HandlerFunc{
		"POST /2025-07-01/retry": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "evt_1", body["event_id"])
			assert.Equal(t, "des_1", body["destination_id"])
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		},
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

	result := callTool(t, session, "outpost_events", map[string]any{
		"action":         "retry",
		"id":             "evt_1",
		"destination_id": "des_1",
	})
	require.False(t, result.IsError, resultText(t, result))
	// A retry is queued, not delivered; the response must not imply otherwise.
	assert.Contains(t, resultText(t, result), `"status":"queued"`)
}

func TestEventsRetryRequiresDestination(t *testing.T) {
	api := mockAPI(t, nil)
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

	result := callTool(t, session, "outpost_events", map[string]any{"action": "retry", "id": "evt_1"})
	require.True(t, result.IsError)
	assert.Contains(t, resultText(t, result), "destination_id is required")
}

func TestDestinationTypesOmitSetupDocsByDefault(t *testing.T) {
	api := mockAPI(t, map[string]http.HandlerFunc{
		"/2025-07-01/destination-types": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"type":         "webhook",
				"label":        "Webhook",
				"icon":         "<svg>a very long icon</svg>",
				"instructions": "a very long setup guide",
			}})
		},
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

	result := callTool(t, session, "outpost_destination_types", map[string]any{"action": "list"})
	require.False(t, result.IsError, resultText(t, result))
	assert.NotContains(t, resultText(t, result), "a very long setup guide")

	verbose := callTool(t, session, "outpost_destination_types", map[string]any{
		"action":             "list",
		"include_setup_docs": true,
	})
	assert.Contains(t, resultText(t, verbose), "a very long setup guide")
}

func TestMetricsRequiresStartEndAndMeasures(t *testing.T) {
	api := mockAPI(t, nil)
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

	t.Run("missing range", func(t *testing.T) {
		result := callTool(t, session, "outpost_metrics", map[string]any{
			"action": "events", "measures": []any{"count"},
		})
		require.True(t, result.IsError)
		assert.Contains(t, resultText(t, result), "start and end are required")
	})

	t.Run("missing measures", func(t *testing.T) {
		result := callTool(t, session, "outpost_metrics", map[string]any{
			"action": "events",
			"start":  "2026-08-01T00:00:00Z",
			"end":    "2026-08-14T00:00:00Z",
		})
		require.True(t, result.IsError)
		assert.Contains(t, resultText(t, result), "measures is required")
	})
}

func TestMetricsFilters(t *testing.T) {
	var gotQuery string
	api := mockAPI(t, map[string]http.HandlerFunc{
		"/2025-07-01/metrics/attempts": func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "metadata": map[string]any{}})
		},
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

	result := callTool(t, session, "outpost_metrics", map[string]any{
		"action":   "attempts",
		"start":    "2026-08-01T00:00:00Z",
		"end":      "2026-08-14T00:00:00Z",
		"measures": []any{"count"},
		"filters":  map[string]any{"status": "failed", "topic": []any{"user.created"}},
	})
	require.False(t, result.IsError, resultText(t, result))
	assert.Contains(t, gotQuery, "filters%5Bstatus%5D%5B0%5D=failed")
	assert.Contains(t, gotQuery, "filters%5Btopic%5D%5B0%5D=user.created")
}

func TestConfigSetRejectsAnEmptyChange(t *testing.T) {
	api := mockAPI(t, nil)
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

	result := callTool(t, session, "outpost_config", map[string]any{"action": "set"})
	require.True(t, result.IsError)
	assert.Contains(t, resultText(t, result), "nothing to change")
}

func TestConfigSetSendsValuesAndUnsets(t *testing.T) {
	var body map[string]*string
	api := mockAPI(t, map[string]http.HandlerFunc{
		"PATCH /2025-07-01/config": func(w http.ResponseWriter, r *http.Request) {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			_ = json.NewEncoder(w).Encode(map[string]any{"TOPICS": "user.created"})
		},
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})

	result := callTool(t, session, "outpost_config", map[string]any{
		"action": "set",
		"values": map[string]any{"TOPICS": "user.created"},
		"unset":  []any{"MAX_RETRY_LIMIT"},
	})
	require.False(t, result.IsError, resultText(t, result))

	require.Contains(t, body, "TOPICS")
	require.NotNil(t, body["TOPICS"])
	assert.Equal(t, "user.created", *body["TOPICS"])
	require.Contains(t, body, "MAX_RETRY_LIMIT")
	assert.Nil(t, body["MAX_RETRY_LIMIT"], "an unset key is sent as null to clear it")
}

func TestPublishUsesTheProjectAPIKeyAsBearer(t *testing.T) {
	var authHeader string
	api := mockAPI(t, map[string]http.HandlerFunc{
		// Publishing first checks the tenant exists in the project the publish
		// credential routes to.
		"GET /2025-07-01/tenants/acme": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "acme"})
		},
		"POST /2025-07-01/publish": func(w http.ResponseWriter, r *http.Request) {
			authHeader = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "evt_1", "destination_ids": []string{"des_1"}})
		},
	})
	session := connect(t, ServerOptions{
		Client:        newTestClient(t, api.URL),
		WriteEnabled:  true,
		PublishAPIKey: "project-api-key",
	})

	result := callTool(t, session, "outpost_publish", map[string]any{
		"action":    "publish",
		"tenant_id": "acme",
		"topic":     "user.created",
		"data":      map[string]any{"user_id": "123"},
	})
	require.False(t, result.IsError, resultText(t, result))
	assert.Equal(t, "Bearer project-api-key", authHeader)
}

// ---------------------------------------------------------------------------
// API error translation
// ---------------------------------------------------------------------------

func TestScopeFailureIsReportedAsNotPermitted(t *testing.T) {
	api := mockAPI(t, map[string]http.HandlerFunc{
		"/2025-07-01/tenants": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "insufficient scope"})
		},
	})
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

	result := callTool(t, session, "outpost_tenants", map[string]any{"action": "list"})
	require.True(t, result.IsError)
	text := resultText(t, result)
	assert.Contains(t, text, "Not permitted")
	assert.NotContains(t, text, "Check your API key", "a 403 is not a bad-key problem")
}

// ---------------------------------------------------------------------------
// Help
// ---------------------------------------------------------------------------

func TestHelpOverview_ReadOnlyMode(t *testing.T) {
	api := mockAPI(t, nil)
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})

	text := resultText(t, callTool(t, session, "outpost_help", map[string]any{}))

	assert.Contains(t, text, "Mode: read-only")
	assert.Contains(t, text, "--allow-write")
	assert.Contains(t, text, "HOOKDECK_MCP_ALLOW_WRITE")
	// The credential-returning reads need explaining, or their absence looks
	// like a bug.
	assert.Contains(t, text, "token")
	assert.Contains(t, text, "portal")
	assert.Contains(t, text, "outpost_publish is not registered")
	assert.Contains(t, text, "proj_outpost")
}

func TestHelpOverview_WriteMode(t *testing.T) {
	api := mockAPI(t, nil)

	t.Run("with a publish key", func(t *testing.T) {
		session := connect(t, ServerOptions{
			Client: newTestClient(t, api.URL), WriteEnabled: true, PublishAPIKey: "k",
		})
		text := resultText(t, callTool(t, session, "outpost_help", map[string]any{}))
		assert.Contains(t, text, "Mode: write enabled")
		assert.Contains(t, text, "outpost_publish")
	})

	t.Run("without a publish key", func(t *testing.T) {
		session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})
		text := resultText(t, callTool(t, session, "outpost_help", map[string]any{}))
		assert.Contains(t, text, "Mode: write enabled")
		assert.Contains(t, text, "outpost_publish is not registered")
		assert.Contains(t, text, "HOOKDECK_OUTPOST_PUBLISH_API_KEY")
		// HOOKDECK_API_KEY means "exchange this for CLI credentials" elsewhere in
		// the CLI and is commonly exported for CI. Naming it here would suggest an
		// ambient variable is enough to start sending real events.
		assert.NotContains(t, text, "set HOOKDECK_API_KEY")
	})
}

func TestHelpTopic(t *testing.T) {
	api := mockAPI(t, nil)

	t.Run("read-only topics document only the available actions", func(t *testing.T) {
		session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})
		text := resultText(t, callTool(t, session, "outpost_help", map[string]any{"topic": "outpost_tenants"}))
		assert.Contains(t, text, "list")
		assert.NotContains(t, text, "\n  delete ")
		assert.Contains(t, text, "read-only mode")
	})

	t.Run("write topics document the write actions", func(t *testing.T) {
		session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true})
		text := resultText(t, callTool(t, session, "outpost_help", map[string]any{"topic": "outpost_tenants"}))
		assert.Contains(t, text, "delete")
		assert.Contains(t, text, "token")
	})

	t.Run("bare topic names resolve", func(t *testing.T) {
		session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})
		text := resultText(t, callTool(t, session, "outpost_help", map[string]any{"topic": "events"}))
		assert.Contains(t, text, "outpost_events")
	})

	t.Run("an unknown topic lists the available ones", func(t *testing.T) {
		session := connect(t, ServerOptions{Client: newTestClient(t, api.URL)})
		result := callTool(t, session, "outpost_help", map[string]any{"topic": "how do I retry an event"})
		assert.True(t, result.IsError)
		assert.Contains(t, resultText(t, result), "No help found")
	})

	t.Run("every registered tool has a help topic", func(t *testing.T) {
		session := connect(t, ServerOptions{
			Client: newTestClient(t, api.URL), WriteEnabled: true, PublishAPIKey: "k",
		})
		for name := range listTools(t, session) {
			result := callTool(t, session, "outpost_help", map[string]any{"topic": name})
			assert.False(t, result.IsError, "no help topic for %s", name)
		}
	})
}

// ---------------------------------------------------------------------------
// Server identity
// ---------------------------------------------------------------------------

func TestServerIdentity(t *testing.T) {
	api := mockAPI(t, nil)
	srv := NewServer(ServerOptions{Client: newTestClient(t, api.URL), Config: &config.Config{}})
	require.NotNil(t, srv)

	// The Outpost server must only ever serve Outpost projects.
	assert.Equal(t, config.ProjectTypeOutpost, srv.ProjectFilter())
	assert.Equal(t, "hookdeck_projects", srv.ProjectsToolName())
	assert.Equal(t, "hookdeck_login", srv.LoginToolName())

	var _ *mcpcore.Server = srv
}

// TestPublishRefusesATenantTheCredentialCannotSee covers the failure that
// prompted this guard: the publish credential and the active project disagreed,
// so events were accepted, delivered nowhere, and left no trace.
func TestPublishRefusesATenantTheCredentialCannotSee(t *testing.T) {
	var published bool
	api := mockAPI(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/tenants/ghost": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "tenant not found"})
		},
		"POST /2025-07-01/publish": func(w http.ResponseWriter, r *http.Request) {
			published = true
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "evt_1", "destination_ids": []string{}})
		},
	})
	session := connect(t, ServerOptions{
		Client:        newTestClient(t, api.URL),
		WriteEnabled:  true,
		PublishAPIKey: "project-api-key",
	})

	result := callTool(t, session, "outpost_publish", map[string]any{
		"action": "publish", "tenant_id": "ghost", "topic": "user.created",
	})

	require.True(t, result.IsError)
	text := resultText(t, result)
	assert.Contains(t, text, "does not exist in the project the publish credential belongs to")
	assert.False(t, published, "nothing should be published once the tenant is known to be missing")
}

// TestPublishWarnsWhenNothingMatched covers the other half: the tenant exists,
// but no destination subscribes to the topic. The API accepts it and the event
// is never delivered or recorded, so a bare success would be misleading.
func TestPublishWarnsWhenNothingMatched(t *testing.T) {
	api := mockAPI(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/tenants/acme": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "acme"})
		},
		"POST /2025-07-01/publish": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "evt_1", "destination_ids": []string{}})
		},
	})
	session := connect(t, ServerOptions{
		Client:        newTestClient(t, api.URL),
		WriteEnabled:  true,
		PublishAPIKey: "project-api-key",
	})

	result := callTool(t, session, "outpost_publish", map[string]any{
		"action": "publish", "tenant_id": "acme", "topic": "user.created",
	})

	require.False(t, result.IsError, resultText(t, result))
	assert.Contains(t, resultText(t, result), "matched no destinations")
}

// The read-only schema surface, pinned.
//
// Prop.Write is opt-in, so a write-only property added without it is exposed to
// read-only sessions and nothing complains — which is how config, credentials,
// filter, metadata, values, unset, hostname and theme were all advertised to
// sessions that could not use them. Listing the expected set means adding a
// property forces a deliberate choice here rather than defaulting to visible.
//
// If this fails after you added a property: decide whether read actions use it.
// If they do, add it below. If only write actions do, mark it Write: true.
func TestReadOnlyPropSurface(t *testing.T) {
	expected := map[string][]string{
		"tenants":           {"dir", "id", "limit", "next", "prev"},
		"destinations":      {"id", "tenant_id", "topics", "type"},
		"events":            {"destination_id", "dir", "id", "limit", "next", "order_by", "prev", "tenant_id", "time_after", "time_before", "topic"},
		"attempts":          {"destination_id", "destination_type", "dir", "event_id", "id", "include", "limit", "next", "order_by", "prev", "status", "tenant_id", "time_after", "time_before", "topic"},
		"topics":            {},
		"destination_types": {"include_setup_docs", "type"},
		"metrics":           {"dimensions", "end", "filters", "granularity", "measures", "start"},
		"config":            {"key"},
		"status":            {},
	}

	for _, spec := range resourceSpecs() {
		t.Run(spec.Resource, func(t *testing.T) {
			want, listed := expected[spec.Resource]
			require.True(t, listed, "%s is not in the expected set; add it", spec.Resource)

			got := make([]string, 0)
			for name := range spec.VisibleProps(false) {
				got = append(got, name)
			}
			sort.Strings(got)
			sort.Strings(want)
			assert.Equal(t, want, got,
				"read-only mode advertises a different property set than expected for %s", spec.Resource)
		})
	}
}

// Write-only properties must reappear once write mode is on, or the tools that
// need them cannot be called.
func TestWriteModeRestoresTheHiddenProps(t *testing.T) {
	for _, spec := range resourceSpecs() {
		readOnly := len(spec.VisibleProps(false))
		writeMode := len(spec.VisibleProps(true))
		assert.GreaterOrEqual(t, writeMode, readOnly,
			"%s must not lose properties in write mode", spec.Resource)
		assert.Equal(t, len(spec.Props), writeMode,
			"%s must advertise every property in write mode", spec.Resource)
	}
}
