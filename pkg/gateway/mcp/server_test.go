package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/internal/speccheck"
	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// newTestClient creates a hookdeck.Client pointing at the given base URL.
func newTestClient(baseURL string, apiKey string) *hookdeck.Client {
	u, _ := url.Parse(baseURL)
	return &hookdeck.Client{
		BaseURL:   u,
		APIKey:    apiKey,
		ProjectID: "proj_test123",
	}
}

// connectInMemory creates an MCP server+client pair connected via in-memory
// transport and returns the client session. The server runs in a background
// goroutine and is torn down when the test ends.
func connectInMemory(t *testing.T, client *hookdeck.Client) *mcpsdk.ClientSession {
	t.Helper()
	return connectInMemoryWithMode(t, client, false)
}

// connectInMemoryWriteEnabled is connectInMemory with --allow-write, for the
// tests that exercise the actions a read-only server does not offer.
func connectInMemoryWriteEnabled(t *testing.T, client *hookdeck.Client) *mcpsdk.ClientSession {
	t.Helper()
	return connectInMemoryWithMode(t, client, true)
}

func connectInMemoryWithMode(t *testing.T, client *hookdeck.Client, writeEnabled bool) *mcpsdk.ClientSession {
	t.Helper()
	cfg := &config.Config{}
	if client != nil && client.BaseURL != nil {
		cfg.APIBaseURL = client.BaseURL.String()
	}
	srv := NewServer(ServerOptions{Client: client, Config: cfg, WriteEnabled: writeEnabled})

	serverTransport, clientTransport := mcpsdk.NewInMemoryTransports()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() {
		_ = srv.Run(ctx, serverTransport)
	}()

	mcpClient := mcpsdk.NewClient(&mcpsdk.Implementation{
		Name:    "test-client",
		Version: "0.0.1",
	}, nil)

	session, err := mcpClient.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

// textContent extracts the text from the first content block of a CallToolResult.
func textContent(t *testing.T, result *mcpsdk.CallToolResult) string {
	t.Helper()
	require.NotEmpty(t, result.Content, "expected at least one content block")
	tc, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok, "expected TextContent, got %T", result.Content[0])
	return tc.Text
}

// callTool is a convenience wrapper.
func callTool(t *testing.T, session *mcpsdk.ClientSession, name string, args map[string]any) *mcpsdk.CallToolResult {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	require.NoError(t, err)
	return result
}

// listResponse returns a standard paginated API response.
func listResponse(models ...map[string]any) map[string]any {
	return map[string]any{
		"models": models,
		// limit must be non-zero so connection name resolution (ListConnections) is not treated as empty
		"pagination": map[string]any{"limit": 100},
	}
}

// mockAPI creates an httptest server that handles specific API paths.

// specGuard fails a test whose code sends a query parameter the pinned OpenAPI
// document does not declare for that route.
//
// Without it a mock answers whatever it is asked, so a test asserting "this
// filter reached the API" passes for a filter the endpoint would ignore. That
// is how list_ignored came to forward the full events filter set to
// GET /requests/{id}/ignored_events, which declares six parameters — the mock
// recorded all of them arriving and the test went green.
func specGuard(t *testing.T, next http.HandlerFunc) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if bad := speccheck.Undeclared(r.URL.Path, r.Method, r.URL.Query()); len(bad) > 0 {
			sort.Strings(bad)
			t.Errorf("%s %s sends query parameter(s) the OpenAPI document does not declare for this route: %s.\n"+
				"The API would ignore them, so the result would look filtered without being filtered.",
				r.Method, r.URL.Path, strings.Join(bad, ", "))
		}
		next(w, r)
	}
}

func mockAPI(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	if handlers == nil {
		handlers = map[string]http.HandlerFunc{}
	}
	if _, ok := handlers[hookdeck.APIPathPrefix+"/cli-auth/validate"]; !ok {
		handlers[hookdeck.APIPathPrefix+"/cli-auth/validate"] = func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{
				"user_id":           "usr_test",
				"user_name":         "Test User",
				"user_email":        "u@example.com",
				"organization_name": "Test Org",
				"organization_id":   "org_test",
				"team_id":           "proj_test123",
				"team_name_no_org":  "Production",
				"team_type":         "console",
			})
		}
	}
	mux := http.NewServeMux()
	for pattern, handler := range handlers {
		mux.HandleFunc(pattern, specGuard(t, handler))
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t.Logf("unhandled request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{"message": "not found: " + r.URL.Path})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// mockAPIWithClientWriteEnabled is mockAPIWithClient with --allow-write on.
func mockAPIWithClientWriteEnabled(t *testing.T, handlers map[string]http.HandlerFunc) *mcpsdk.ClientSession {
	t.Helper()
	api := mockAPI(t, handlers)
	client := newTestClient(api.URL, "test-key")
	client.SuppressRateLimitErrors = true
	return connectInMemoryWriteEnabled(t, client)
}

// mockAPIWithClient creates a mock API and returns both the server and a connected MCP session.
func mockAPIWithClient(t *testing.T, handlers map[string]http.HandlerFunc) *mcpsdk.ClientSession {
	t.Helper()
	api := mockAPI(t, handlers)
	client := newTestClient(api.URL, "test-key")
	client.SuppressRateLimitErrors = true
	return connectInMemory(t, client)
}

// ---------------------------------------------------------------------------
// Server initialization and tool listing
// ---------------------------------------------------------------------------

func TestListTools_Authenticated(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-api-key")
	session := connectInMemory(t, client)

	result, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)

	toolNames := make([]string, len(result.Tools))
	for i, tool := range result.Tools {
		toolNames[i] = tool.Name
	}

	assert.Contains(t, toolNames, "hookdeck_login")

	expectedTools := []string{
		"hookdeck_projects_read", "gateway_connections_read", "gateway_sources_read",
		"gateway_destinations_read", "gateway_transformations_read",
		"gateway_requests_read", "gateway_request_read",
		"gateway_events_read", "gateway_event_read", "gateway_attempts_read", "gateway_issues_read",
		"gateway_metrics_read", "gateway_help",
	}
	for _, name := range expectedTools {
		assert.Contains(t, toolNames, name)
	}
}

func TestListTools_Unauthenticated(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "")
	session := connectInMemory(t, client)

	result, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)

	toolNames := make([]string, len(result.Tools))
	for i, tool := range result.Tools {
		toolNames[i] = tool.Name
	}

	assert.Contains(t, toolNames, "hookdeck_login")
	assert.Contains(t, toolNames, "gateway_help")
	assert.Contains(t, toolNames, "gateway_events_read")
	assert.Contains(t, toolNames, "gateway_event_read")
}

// schemaPropertyNames returns the parameter names a tool advertises.
func schemaPropertyNames(t *testing.T, tool *mcpsdk.Tool) []string {
	t.Helper()
	require.NotNil(t, tool)
	// The SDK hands the schema back as decoded JSON, so re-encode rather than
	// assuming a concrete type.
	raw, err := json.Marshal(tool.InputSchema)
	require.NoError(t, err)

	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	require.NoError(t, json.Unmarshal(raw, &schema))

	names := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Filters belong to the plural search tools. The whole point of splitting the
// pairs is that a caller holding an id is not shown twenty filters it cannot
// use, so the singular tools have to stay at their id (plus connection_ids for
// a retry) however many filters the plural side grows.
func TestFiltersLiveOnThePluralToolsOnly(t *testing.T) {
	session := connectInMemory(t, newTestClient("https://api.hookdeck.com", "test-key"))
	tools := listTools(t, session)

	t.Run("gateway_events carries the list filters", func(t *testing.T) {
		props := schemaPropertyNames(t, tools["gateway_events_read"])
		for _, name := range []string{
			"search_term", "delivery_group", "next_attempt_after", "next_attempt_before",
		} {
			assert.Contains(t, props, name)
		}
	})

	t.Run("gateway_requests carries the list filters", func(t *testing.T) {
		props := schemaPropertyNames(t, tools["gateway_requests_read"])
		for _, name := range []string{
			"search_term", "events_count", "ignored_count", "cli_events_count",
		} {
			assert.Contains(t, props, name)
		}
	})

	t.Run("gateway_event takes only an id", func(t *testing.T) {
		assert.Equal(t, []string{"action", "id"}, schemaPropertyNames(t, tools["gateway_event_read"]))
	})

	// connection_ids belongs to retry, which read-only mode does not offer, so
	// it is not advertised there either. Offering a parameter for an action the
	// session cannot reach is the same problem as naming the action in prose.
	t.Run("gateway_request takes only an id in read-only mode", func(t *testing.T) {
		assert.Equal(t, []string{"action", "id"}, schemaPropertyNames(t, tools["gateway_request_read"]))
	})

	// connection_ids belongs to retry, which lives on the write tool. The read
	// tool must not grow it when --allow-write is on: a read tool that changes
	// shape with the mode is the thing the split exists to prevent.
	t.Run("the retry targets live on the write tool, not the read one", func(t *testing.T) {
		writeSession := connectInMemoryWriteEnabled(t, newTestClient("https://api.hookdeck.com", "test-key"))
		tools := listTools(t, writeSession)
		assert.Equal(t, []string{"action", "id"},
			schemaPropertyNames(t, tools["gateway_request_read"]))
		assert.Equal(t, []string{"action", "connection_ids", "id"},
			schemaPropertyNames(t, tools["gateway_request_write"]))
	})
}

// Read-only mode filters the action enum; it must filter everything else that
// describes those actions too. A session offered `config` or `rules` has been
// shown an affordance it cannot use, and a description naming "retry, cancel or
// mute" is more persuasive to a model than the enum that contradicts it.
func TestReadOnlyModeHidesWriteOnlyPropsAndProse(t *testing.T) {
	session := connectInMemory(t, newTestClient("https://api.hookdeck.com", "test-key"))
	tools := listTools(t, session)

	t.Run("write-only properties are absent", func(t *testing.T) {
		for tool, hidden := range map[string][]string{
			"gateway_sources_read":      {"config", "description", "type"},
			"gateway_destinations_read": {"config", "description", "type"},
			"gateway_connections_read":  {"description", "rules"},
			"gateway_issues_read":       {"status"},
		} {
			props := schemaPropertyNames(t, tools[tool])
			for _, name := range hidden {
				assert.NotContains(t, props, name,
					"%s must not advertise %q when the actions using it are hidden", tool, name)
			}
		}
	})

	t.Run("descriptions do not name unavailable actions", func(t *testing.T) {
		for tool, absent := range map[string][]string{
			"gateway_event_read":    {"cancel", "mute"},
			"gateway_request_read":  {"retry"},
			"gateway_events_read":   {"cancel", "mute"},
			"gateway_requests_read": {"retry it"},
		} {
			description := tools[tool].Description
			for _, word := range absent {
				assert.NotContains(t, description, word,
					"%s describes %q, which read-only mode does not offer", tool, word)
			}
		}
	})
}

// The API spec also documents parameters marked x-docs-hide: Hookdeck keeps
// them out of its public documentation deliberately, so the CLI must not
// surface them either. Reading the spec without that context makes them look
// like filters we simply forgot, which is exactly how they would get added —
// this pins the omission as intentional.
func TestPluralToolsOmitParametersHiddenFromTheAPIDocs(t *testing.T) {
	session := connectInMemory(t, newTestClient("https://api.hookdeck.com", "test-key"))
	tools := listTools(t, session)

	hidden := map[string][]string{
		"gateway_events_read": {
			"bulk_retry_id", "include", "progressive", "event_data_id", "cli_user_id",
		},
		"gateway_requests_read": {"bulk_retry_id", "include", "progressive"},
	}

	for tool, names := range hidden {
		t.Run(tool, func(t *testing.T) {
			props := schemaPropertyNames(t, tools[tool])
			for _, name := range names {
				assert.NotContains(t, props, name,
					"%s must not expose %q: it carries x-docs-hide in the API spec", tool, name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Help tool
// ---------------------------------------------------------------------------

func TestHelpTool_Overview(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	result := callTool(t, session, "gateway_help", map[string]any{})
	assert.False(t, result.IsError)

	text := textContent(t, result)
	assert.Contains(t, text, "gateway_events_read")
	assert.Contains(t, text, "gateway_connections_read")
	assert.Contains(t, text, "gateway_sources_read")
	assert.Contains(t, text, "proj_test123") // current project
}

func TestHelpTool_SpecificTopic(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	result := callTool(t, session, "gateway_help", map[string]any{"topic": "gateway_event_read"})
	assert.False(t, result.IsError)
	text := textContent(t, result)
	assert.Contains(t, text, "get")
	assert.Contains(t, text, "raw_body")
}

// The plural/singular split is only usable if help says which tool does what,
// so each topic has to point at its counterpart by name.
func TestHelpTopics_PointAtTheirCounterpart(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	cases := []struct{ topic, wants string }{
		{"gateway_events_read", "gateway_event_read"},
		{"gateway_event_read", "gateway_events_read"},
		{"gateway_requests_read", "gateway_request_read"},
		{"gateway_request_read", "gateway_requests_read"},
	}

	for _, tc := range cases {
		t.Run(tc.topic, func(t *testing.T) {
			result := callTool(t, session, "gateway_help", map[string]any{"topic": tc.topic})
			assert.False(t, result.IsError)
			assert.Contains(t, textContent(t, result), tc.wants)
		})
	}
}

// The only relationship traversal the API supports is request → events. An
// agent told nothing will look for a request_id filter that does not exist, so
// both event topics have to say where the traversal lives.
func TestHelpTopics_DocumentTheOneTraversalDirection(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	for _, topic := range []string{"gateway_events_read", "gateway_event_read"} {
		t.Run(topic, func(t *testing.T) {
			text := textContent(t, callTool(t, session, "gateway_help", map[string]any{"topic": topic}))
			assert.Contains(t, text, "request_id")
			assert.Contains(t, text, "gateway_request_read")
		})
	}

	t.Run("gateway_requests states there is no event_id filter", func(t *testing.T) {
		text := textContent(t, callTool(t, session, "gateway_help", map[string]any{"topic": "gateway_requests_read"}))
		assert.Contains(t, text, "event_id")
	})
}

func TestHelpEventsTopic_DocumentsDateRangeFilters(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	result := callTool(t, session, "gateway_help", map[string]any{"topic": "gateway_events_read"})
	assert.False(t, result.IsError)
	text := textContent(t, result)
	assert.Contains(t, text, "Date range filters")
	assert.Contains(t, text, "created_after")
	assert.Contains(t, text, "successful_after")
	assert.Contains(t, text, "last_attempt_after")
	assert.Contains(t, text, "ISO 8601")
	assert.Contains(t, text, "created_at[gte]")
}

func TestHelpRequestsTopic_DocumentsDateRangeFilters(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	result := callTool(t, session, "gateway_help", map[string]any{"topic": "gateway_requests_read"})
	assert.False(t, result.IsError)
	text := textContent(t, result)
	assert.Contains(t, text, "Date range filters")
	assert.Contains(t, text, "created_after")
	assert.Contains(t, text, "ingested_after")
	assert.Contains(t, text, "ISO 8601")
	assert.Contains(t, text, "ingested_at[gte]")
}

func TestHelpTool_ShortTopicName(t *testing.T) {
	// "events" should resolve to "gateway_events_read"
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	result := callTool(t, session, "gateway_help", map[string]any{"topic": "events"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "gateway_events_read")
}

func TestHelpTool_UnknownTopic(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	result := callTool(t, session, "gateway_help", map[string]any{"topic": "nonexistent_tool"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "No help found")
}

// ---------------------------------------------------------------------------
// Auth guard on resource tools
// ---------------------------------------------------------------------------

func TestAuthGuard_UnauthenticatedReturnsError(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "")
	session := connectInMemory(t, client)

	resourceTools := map[string]string{
		"gateway_sources_read":         "list",
		"gateway_destinations_read":    "list",
		"gateway_connections_read":     "list",
		"gateway_events_read":          "list",
		"gateway_event_read":           "get",
		"gateway_requests_read":        "list",
		"gateway_request_read":         "get",
		"gateway_attempts_read":        "list",
		"gateway_issues_read":          "list",
		"gateway_transformations_read": "list",
		"gateway_metrics_read":         "events",
		"hookdeck_projects_read":       "list",
	}

	for toolName, action := range resourceTools {
		t.Run(toolName, func(t *testing.T) {
			result := callTool(t, session, toolName, map[string]any{"action": action, "id": "res_1"})
			assert.True(t, result.IsError, "expected IsError=true for unauthenticated %s", toolName)
			assert.Contains(t, textContent(t, result), "hookdeck_login")
		})
	}
}

// ---------------------------------------------------------------------------
// Error translation
// ---------------------------------------------------------------------------

func TestSourcesList_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/sources": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "src_123", "name": "my-source"}))
		},
	})

	result := callTool(t, session, "gateway_sources_read", map[string]any{"action": "list"})
	assert.False(t, result.IsError)
	text := textContent(t, result)
	assert.Contains(t, text, "src_123")
	assert.Contains(t, text, "my-source")
}

func TestSourcesGet_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/sources/src_123": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"id": "src_123", "name": "github-webhooks"})
		},
	})

	result := callTool(t, session, "gateway_sources_read", map[string]any{"action": "get", "id": "src_123"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "github-webhooks")
}

func TestSourcesGet_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "gateway_sources_read", map[string]any{"action": "get"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id is required")
}

func TestSourcesTool_UnknownAction(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "gateway_sources_read", map[string]any{"action": "frobnicate"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "unknown action")
}

// ---------------------------------------------------------------------------
// Destinations tool
// ---------------------------------------------------------------------------

func TestDestinationsList_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/destinations": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "des_456", "name": "my-backend"}))
		},
	})

	result := callTool(t, session, "gateway_destinations_read", map[string]any{"action": "list"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "des_456")
}

func TestDestinationsGet_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/destinations/des_456": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"id": "des_456", "name": "my-backend"})
		},
	})

	result := callTool(t, session, "gateway_destinations_read", map[string]any{"action": "get", "id": "des_456"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "des_456")
}

func TestDestinationsGet_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "gateway_destinations_read", map[string]any{"action": "get"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id is required")
}

func TestDestinationsTool_UnknownAction(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "gateway_destinations_read", map[string]any{"action": "frobnicate"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "unknown action")
}

// ---------------------------------------------------------------------------
// Connections tool
// ---------------------------------------------------------------------------

func TestConnectionsList_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/connections": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "web_conn1", "name": "stripe-to-backend"}))
		},
	})

	result := callTool(t, session, "gateway_connections_read", map[string]any{"action": "list"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "stripe-to-backend")
}

func TestConnectionsGet_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/connections/web_conn1": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"id": "web_conn1", "name": "stripe-to-backend"})
		},
	})

	result := callTool(t, session, "gateway_connections_read", map[string]any{"action": "get", "id": "web_conn1"})
	assert.False(t, result.IsError)
	text := textContent(t, result)
	assert.Contains(t, text, "web_conn1")
	assert.Contains(t, text, `"data"`)
	assert.Contains(t, text, `"meta"`)
	assert.Contains(t, text, `"active_project_id"`)
}

func TestConnectionsGet_ByName(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/connections": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "stripe-to-backend", r.URL.Query().Get("name"))
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "web_conn1", "name": "stripe-to-backend"}))
		},
		hookdeck.APIPathPrefix + "/connections/web_conn1": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			json.NewEncoder(w).Encode(map[string]any{"id": "web_conn1", "name": "stripe-to-backend"})
		},
	})

	result := callTool(t, session, "gateway_connections_read", map[string]any{"action": "get", "id": "stripe-to-backend"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "web_conn1")
}

func TestConnectionsGet_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "gateway_connections_read", map[string]any{"action": "get"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id or name is required")
}

func TestConnectionsPause_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/connections/web_conn1": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			json.NewEncoder(w).Encode(map[string]any{"id": "web_conn1", "name": "stripe-to-backend"})
		},
		hookdeck.APIPathPrefix + "/connections/web_conn1/pause": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "PUT", r.Method)
			json.NewEncoder(w).Encode(map[string]any{"id": "web_conn1", "paused_at": "2025-01-01T00:00:00Z"})
		},
	})

	result := callTool(t, session, "gateway_connections_pause", map[string]any{"action": "pause", "id": "web_conn1"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "web_conn1")
}

func TestConnectionsPause_ByName(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/connections": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "stripe-to-backend", r.URL.Query().Get("name"))
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "web_conn1", "name": "stripe-to-backend"}))
		},
		hookdeck.APIPathPrefix + "/connections/web_conn1/pause": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "PUT", r.Method)
			json.NewEncoder(w).Encode(map[string]any{"id": "web_conn1", "paused_at": "2025-01-01T00:00:00Z"})
		},
	})

	result := callTool(t, session, "gateway_connections_pause", map[string]any{"action": "pause", "id": "stripe-to-backend"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "web_conn1")
}

func TestConnectionsPause_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "gateway_connections_pause", map[string]any{"action": "pause"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id or name is required")
}

func TestConnectionsUnpause_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/connections/web_conn1": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			json.NewEncoder(w).Encode(map[string]any{"id": "web_conn1", "name": "stripe-to-backend"})
		},
		hookdeck.APIPathPrefix + "/connections/web_conn1/unpause": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "PUT", r.Method)
			json.NewEncoder(w).Encode(map[string]any{"id": "web_conn1"})
		},
	})

	result := callTool(t, session, "gateway_connections_pause", map[string]any{"action": "unpause", "id": "web_conn1"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "web_conn1")
}

func TestConnectionsUnpause_ByName(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/connections": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "stripe-to-backend", r.URL.Query().Get("name"))
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "web_conn1", "name": "stripe-to-backend"}))
		},
		hookdeck.APIPathPrefix + "/connections/web_conn1/unpause": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "PUT", r.Method)
			json.NewEncoder(w).Encode(map[string]any{"id": "web_conn1"})
		},
	})

	result := callTool(t, session, "gateway_connections_pause", map[string]any{"action": "unpause", "id": "stripe-to-backend"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "web_conn1")
}

func TestConnectionsUnpause_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "gateway_connections_pause", map[string]any{"action": "unpause"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id or name is required")
}

func TestConnectionsTool_UnknownAction(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "gateway_connections_read", map[string]any{"action": "frobnicate"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "unknown action")
}

func TestConnectionsList_DisabledFilter(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/connections": func(w http.ResponseWriter, r *http.Request) {
			// Verify disabled_at[any]=true is sent when disabled=true
			assert.Equal(t, "true", r.URL.Query().Get("disabled_at[any]"))
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "web_1"}))
		},
	})

	result := callTool(t, session, "gateway_connections_read", map[string]any{"action": "list", "disabled": true})
	assert.False(t, result.IsError)
}

// ---------------------------------------------------------------------------
// Transformations tool
// ---------------------------------------------------------------------------

func TestTransformationsList_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/transformations": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "trn_789", "name": "enrich-payload"}))
		},
	})

	result := callTool(t, session, "gateway_transformations_read", map[string]any{"action": "list"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "trn_789")
}

func TestTransformationsGet_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/transformations/trn_789": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"id": "trn_789", "name": "enrich-payload", "code": "module.exports = (req) => req"})
		},
	})

	result := callTool(t, session, "gateway_transformations_read", map[string]any{"action": "get", "id": "trn_789"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "enrich-payload")
}

func TestTransformationsGet_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "gateway_transformations_read", map[string]any{"action": "get"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id is required")
}

func TestTransformationsTool_UnknownAction(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "gateway_transformations_read", map[string]any{"action": "frobnicate"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "unknown action")
}

// ---------------------------------------------------------------------------
// Attempts tool
// ---------------------------------------------------------------------------

func TestAttemptsList_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/attempts": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "atm_001", "status": "SUCCESSFUL", "response_status": 200}))
		},
	})

	result := callTool(t, session, "gateway_attempts_read", map[string]any{"action": "list"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "atm_001")
}

func TestAttemptsGet_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/attempts/atm_001": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"id": "atm_001", "response_status": 200})
		},
	})

	result := callTool(t, session, "gateway_attempts_read", map[string]any{"action": "get", "id": "atm_001"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "atm_001")
}

func TestAttemptsGet_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "gateway_attempts_read", map[string]any{"action": "get"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id is required")
}

func TestAttemptsTool_UnknownAction(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "gateway_attempts_read", map[string]any{"action": "retry"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "unknown action")
}

// ---------------------------------------------------------------------------
// Events tool
// ---------------------------------------------------------------------------

func TestEventsList_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/events": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "evt_abc", "status": "SUCCESSFUL"}))
		},
	})

	result := callTool(t, session, "gateway_events_read", map[string]any{"action": "list"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "evt_abc")
}

func TestEventGet_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/events/evt_abc": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"id": "evt_abc", "status": "SUCCESSFUL"})
		},
	})

	result := callTool(t, session, "gateway_event_read", map[string]any{"action": "get", "id": "evt_abc"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "evt_abc")
}

func TestEventGet_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "gateway_event_read", map[string]any{"action": "get"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id is required")
}

func TestEventRawBody_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/events/evt_abc/raw_body": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"key":"value"}`))
		},
	})

	result := callTool(t, session, "gateway_event_read", map[string]any{"action": "raw_body", "id": "evt_abc"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "raw_body")
}

func TestEventRawBody_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "gateway_event_read", map[string]any{"action": "raw_body"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id is required")
}

func TestEventRawBody_Truncation(t *testing.T) {
	// Generate a body larger than 100KB
	largeBody := strings.Repeat("x", 150*1024)
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/events/evt_big/raw_body": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(largeBody))
		},
	})

	result := callTool(t, session, "gateway_event_read", map[string]any{"action": "raw_body", "id": "evt_big"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "truncated")
}

func TestEventsTool_UnknownAction(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "gateway_events_read", map[string]any{"action": "delete"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "unknown action")
}

// The by-id actions moved to gateway_event, so asking the plural tool for one
// is now a wrong-tool mistake. The error has to name the tool that can do it,
// because an agent that lands here needs redirecting, not just refusing.
func TestEventsTool_ByIDActionsAreNotOnThePluralTool(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	for _, action := range []string{"get", "raw_body", "retry", "cancel", "mute"} {
		t.Run(action, func(t *testing.T) {
			result := callTool(t, session, "gateway_events_read", map[string]any{"action": action, "id": "evt_abc"})
			assert.True(t, result.IsError)
			assert.Contains(t, textContent(t, result), "unknown action")
		})
	}
}

func TestEventTool_ListIsNotOnTheSingularTool(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	result := callTool(t, session, "gateway_event_read", map[string]any{"action": "list"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "unknown action")
}

func TestEventsList_ConnectionIDMapsToWebhookID(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/events": func(w http.ResponseWriter, r *http.Request) {
			// Verify connection_id is mapped to webhook_id
			assert.Equal(t, "web_123", r.URL.Query().Get("webhook_id"))
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "evt_1"}))
		},
	})

	result := callTool(t, session, "gateway_events_read", map[string]any{"action": "list", "connection_id": "web_123"})
	assert.False(t, result.IsError)
}

func TestEventsList_BodyFilter(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/events": func(w http.ResponseWriter, r *http.Request) {
			assert.JSONEq(t, `{"type":"payment"}`, r.URL.Query().Get("body"))
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "evt_1"}))
		},
	})

	result := callTool(t, session, "gateway_events_read", map[string]any{
		"action": "list",
		"body":   map[string]any{"type": "payment"},
	})
	assert.False(t, result.IsError)
}

func TestEventsList_PayloadFilters(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/events": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, `{"x-test":"1"}`, r.URL.Query().Get("headers"))
			assert.JSONEq(t, `{"q":"search"}`, r.URL.Query().Get("parsed_query"))
			assert.Equal(t, "/webhooks", r.URL.Query().Get("path"))
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "evt_1"}))
		},
	})

	result := callTool(t, session, "gateway_events_read", map[string]any{
		"action":       "list",
		"headers":      `{"x-test":"1"}`,
		"parsed_query": map[string]any{"q": "search"},
		"path":         "/webhooks",
	})
	assert.False(t, result.IsError)
}

func TestEventsList_MetadataFilters(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/events": func(w http.ResponseWriter, r *http.Request) {
			// This assertion used to be `"evt_1,evt_2" == Get("id")` -- the
			// comma-joined scalar. That is the wire format of bug #411: sent to
			// the live API it matches nothing and returns zero rows, where the
			// repeated id[] form returns both. So the test was certifying the
			// bug rather than guarding against it. Verified against the real API
			// on event, request and transformation list before changing it.
			assert.Equal(t, []string{"evt_1", "evt_2"}, r.URL.Query()["id[]"])
			assert.Empty(t, r.URL.Query().Get("id"), "the comma-joined scalar matches nothing")
			assert.Equal(t, "3", r.URL.Query().Get("attempts"))
			assert.Equal(t, "cli_abc", r.URL.Query().Get("cli_id"))
			assert.Equal(t, "cus_123", r.URL.Query().Get("delivery_group"))
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "evt_1"}))
		},
	})

	result := callTool(t, session, "gateway_events_read", map[string]any{
		"action":         "list",
		"id":             "evt_1,evt_2",
		"attempts":       "3",
		"cli_id":         "cli_abc",
		"delivery_group": "cus_123",
	})
	assert.False(t, result.IsError)
}

func TestEventsList_InvalidBodyFilter(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "gateway_events_read", map[string]any{"action": "list", "body": 42})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "body must be a JSON string or object")
}

func TestEventsList_CreatedAtDateRange(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/events": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "2026-06-01T00:00:00Z", r.URL.Query().Get("created_at[gte]"))
			assert.Equal(t, "2026-06-09T23:59:59Z", r.URL.Query().Get("created_at[lte]"))
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "evt_1"}))
		},
	})

	result := callTool(t, session, "gateway_events_read", map[string]any{
		"action":         "list",
		"created_after":  "2026-06-01T00:00:00Z",
		"created_before": "2026-06-09T23:59:59Z",
	})
	assert.False(t, result.IsError)
}

func TestEventsList_SuccessfulAtDateRange(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/events": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "2026-06-01T00:00:00Z", r.URL.Query().Get("successful_at[gte]"))
			assert.Equal(t, "2026-06-09T23:59:59Z", r.URL.Query().Get("successful_at[lte]"))
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "evt_1"}))
		},
	})

	result := callTool(t, session, "gateway_events_read", map[string]any{
		"action":            "list",
		"successful_after":  "2026-06-01T00:00:00Z",
		"successful_before": "2026-06-09T23:59:59Z",
	})
	assert.False(t, result.IsError)
}

func TestEventsList_LastAttemptAtDateRange(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/events": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "2026-06-01T00:00:00Z", r.URL.Query().Get("last_attempt_at[gte]"))
			assert.Equal(t, "2026-06-09T23:59:59Z", r.URL.Query().Get("last_attempt_at[lte]"))
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "evt_1"}))
		},
	})

	result := callTool(t, session, "gateway_events_read", map[string]any{
		"action":              "list",
		"last_attempt_after":  "2026-06-01T00:00:00Z",
		"last_attempt_before": "2026-06-09T23:59:59Z",
	})
	assert.False(t, result.IsError)
}

// ---------------------------------------------------------------------------
// Requests tool
// ---------------------------------------------------------------------------

func TestRequestsList_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/requests": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "req_001", "source_id": "src_123"}))
		},
	})

	result := callTool(t, session, "gateway_requests_read", map[string]any{"action": "list"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "req_001")
}

func TestRequestGet_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/requests/req_001": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"id": "req_001"})
		},
	})

	result := callTool(t, session, "gateway_request_read", map[string]any{"action": "get", "id": "req_001"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "req_001")
}

func TestRequestGet_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "gateway_request_read", map[string]any{"action": "get"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id is required")
}

func TestRequestRawBody_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/requests/req_001/raw_body": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"payload":"data"}`))
		},
	})

	result := callTool(t, session, "gateway_request_read", map[string]any{"action": "raw_body", "id": "req_001"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "raw_body")
}

func TestRequestRawBody_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "gateway_request_read", map[string]any{"action": "raw_body"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id is required")
}

func TestRequestRawBody_Truncation(t *testing.T) {
	largeBody := strings.Repeat("y", 150*1024)
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/requests/req_big/raw_body": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(largeBody))
		},
	})

	result := callTool(t, session, "gateway_request_read", map[string]any{"action": "raw_body", "id": "req_big"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "truncated")
}

// The request-scoped event listings moved from gateway_request to
// gateway_events in v3.0.0, because the filters they honour are the events
// filter set and that is where it already lived. See
// plans/mcp_read_write_tool_split.md.
func TestEventsScopedToARequest_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/requests/req_001/events": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "cus_123", r.URL.Query().Get("delivery_group"))
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "evt_from_req"}))
		},
	})

	result := callTool(t, session, "gateway_events_read", map[string]any{
		"action": "list", "request_id": "req_001", "delivery_group": "cus_123",
	})
	assert.False(t, result.IsError, textContent(t, result))
	assert.Contains(t, textContent(t, result), "evt_from_req")
}

func TestEventsIgnoredScopedToARequest_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/requests/req_001/ignored_events": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "ign_evt_001"}))
		},
	})

	result := callTool(t, session, "gateway_events_read", map[string]any{
		"action": "list_ignored", "request_id": "req_001",
	})
	assert.False(t, result.IsError, textContent(t, result))
	assert.Contains(t, textContent(t, result), "ign_evt_001")
}

// The singular request tool no longer carries these actions at all, so a caller
// that still reaches for them is told which tool does.
func TestRequestToolNoLongerCarriesEventListings(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	for _, action := range []string{"events", "ignored_events"} {
		t.Run(action, func(t *testing.T) {
			result := callTool(t, session, "gateway_request_read", map[string]any{"action": action, "id": "req_001"})
			require.True(t, result.IsError)
			body := textContent(t, result)
			assert.Contains(t, body, "unknown action")
			assert.Contains(t, body, requestsToolName, "the error should name a tool that can help")
		})
	}
}

func TestRequestsTool_UnknownAction(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "gateway_requests_read", map[string]any{"action": "delete"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "unknown action")
}

// The by-id actions moved to gateway_request; the plural tool only searches.
func TestRequestsTool_ByIDActionsAreNotOnThePluralTool(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	for _, action := range []string{"get", "raw_body", "retry"} {
		t.Run(action, func(t *testing.T) {
			result := callTool(t, session, "gateway_requests_read", map[string]any{"action": action, "id": "req_001"})
			assert.True(t, result.IsError)
			assert.Contains(t, textContent(t, result), "unknown action")
		})
	}
}

func TestRequestTool_ListIsNotOnTheSingularTool(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	result := callTool(t, session, "gateway_request_read", map[string]any{"action": "list"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "unknown action")
}

func TestRequestsList_VerifiedFilter(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/requests": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "true", r.URL.Query().Get("verified"))
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "req_v"}))
		},
	})

	result := callTool(t, session, "gateway_requests_read", map[string]any{"action": "list", "verified": true})
	assert.False(t, result.IsError)
}

func TestRequestsList_BodyFilter(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/requests": func(w http.ResponseWriter, r *http.Request) {
			assert.JSONEq(t, `{"event":"test"}`, r.URL.Query().Get("body"))
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "req_1"}))
		},
	})

	result := callTool(t, session, "gateway_requests_read", map[string]any{
		"action": "list",
		"body":   map[string]any{"event": "test"},
	})
	assert.False(t, result.IsError)
}

func TestRequestsList_CreatedAtDateRange(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/requests": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "2026-06-01T00:00:00Z", r.URL.Query().Get("created_at[gte]"))
			assert.Equal(t, "2026-06-09T23:59:59Z", r.URL.Query().Get("created_at[lte]"))
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "req_1"}))
		},
	})

	result := callTool(t, session, "gateway_requests_read", map[string]any{
		"action":         "list",
		"created_after":  "2026-06-01T00:00:00Z",
		"created_before": "2026-06-09T23:59:59Z",
	})
	assert.False(t, result.IsError)
}

func TestRequestsList_IngestedAtDateRange(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/requests": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "2026-06-01T00:00:00Z", r.URL.Query().Get("ingested_at[gte]"))
			assert.Equal(t, "2026-06-09T23:59:59Z", r.URL.Query().Get("ingested_at[lte]"))
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "req_1"}))
		},
	})

	result := callTool(t, session, "gateway_requests_read", map[string]any{
		"action":          "list",
		"ingested_after":  "2026-06-01T00:00:00Z",
		"ingested_before": "2026-06-09T23:59:59Z",
	})
	assert.False(t, result.IsError)
}

func TestRequestsList_OrderByAndDir(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/requests": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "created_at", r.URL.Query().Get("order_by"))
			assert.Equal(t, "desc", r.URL.Query().Get("dir"))
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "req_1"}))
		},
	})

	result := callTool(t, session, "gateway_requests_read", map[string]any{
		"action":   "list",
		"order_by": "created_at",
		"dir":      "desc",
	})
	assert.False(t, result.IsError)
}

// ---------------------------------------------------------------------------
// Issues tool
// ---------------------------------------------------------------------------

func TestIssuesList_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/issues": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "iss_001", "type": "delivery", "status": "OPENED"}))
		},
	})

	result := callTool(t, session, "gateway_issues_read", map[string]any{"action": "list"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "iss_001")
}

func TestIssuesGet_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/issues/iss_001": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"id": "iss_001", "type": "delivery"})
		},
	})

	result := callTool(t, session, "gateway_issues_read", map[string]any{"action": "get", "id": "iss_001"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "iss_001")
}

func TestIssuesGet_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "gateway_issues_read", map[string]any{"action": "get"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id is required")
}

func TestIssuesTool_UnknownAction(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "gateway_issues_read", map[string]any{"action": "close"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "unknown action")
}

// ---------------------------------------------------------------------------
// Projects tool
// ---------------------------------------------------------------------------

func TestProjectsList_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/projects": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": "proj_test123", "name": "Production", "type": "console"},
				{"id": "proj_other", "name": "Staging", "type": "console"},
			})
		},
	})

	result := callTool(t, session, "hookdeck_projects_read", map[string]any{"action": "list"})
	assert.False(t, result.IsError)
	text := textContent(t, result)
	assert.Contains(t, text, `"data"`)
	assert.Contains(t, text, `"meta"`)
	assert.Contains(t, text, `"projects"`)
	assert.Contains(t, text, "Production")
	assert.Contains(t, text, "Staging")
	// Current project should be marked
	assert.Contains(t, text, "proj_test123")
	// newTestClient sets ProjectID — scope lives in meta.active_project_*
	assert.Contains(t, text, `"active_project_id"`)
}

func TestProjectsList_ForbiddenIncludesReauthHint(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/projects": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]any{"message": "not allowed"})
		},
	})

	result := callTool(t, session, "hookdeck_projects_read", map[string]any{"action": "list"})
	assert.True(t, result.IsError)
	text := textContent(t, result)
	assert.Contains(t, strings.ToLower(text), "reauth")
	assert.Contains(t, text, "hookdeck_login")
}

func TestProjectsUse_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/projects": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": "proj_test123", "name": "Production", "type": "console"},
				{"id": "proj_new", "name": "Staging", "type": "console"},
			})
		},
	})

	result := callTool(t, session, "hookdeck_projects_use", map[string]any{"action": "use", "project_id": "proj_new"})
	assert.False(t, result.IsError)
	text := textContent(t, result)
	assert.Contains(t, text, "proj_new")
	assert.Contains(t, text, "Staging")
	assert.Contains(t, text, "ok")
	assert.Contains(t, text, `"active_project_id"`)
}

func TestProjectsUse_MissingProjectID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "hookdeck_projects_use", map[string]any{"action": "use"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "project_id is required")
}

func TestProjectsUse_ProjectNotFound(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/projects": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": "proj_test123", "name": "Production", "type": "console"},
			})
		},
	})

	result := callTool(t, session, "hookdeck_projects_use", map[string]any{"action": "use", "project_id": "proj_nonexistent"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "not found")
}

func TestProjectsTool_UnknownAction(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	// The example has to be something that is genuinely not an action on any
	// projects tool — otherwise the test asserts nothing.
	result := callTool(t, session, "hookdeck_projects_read", map[string]any{"action": "archive"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "unknown action")
}

// ---------------------------------------------------------------------------
// Metrics tool
// ---------------------------------------------------------------------------

func TestMetricsTool_MissingStartEnd(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "gateway_metrics_read", map[string]any{"action": "events"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "required")
}

func TestMetricsTool_MissingMeasures(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "gateway_metrics_read", map[string]any{
		"action": "events",
		"start":  "2025-01-01T00:00:00Z",
		"end":    "2025-01-02T00:00:00Z",
	})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "measures")
}

func TestMetricsEvents_DefaultRoute(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/metrics/events": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "cus_123", r.URL.Query().Get("filters[delivery_group]"))
			json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "granularity": "1h"})
		},
	})

	result := callTool(t, session, "gateway_metrics_read", map[string]any{
		"action":         "events",
		"start":          "2025-01-01T00:00:00Z",
		"end":            "2025-01-02T00:00:00Z",
		"measures":       []any{"count"},
		"delivery_group": "cus_123",
	})
	assert.False(t, result.IsError)
}

func TestMetricsEvents_QueueDepthRoute(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/metrics/queue-depth": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		},
	})

	result := callTool(t, session, "gateway_metrics_read", map[string]any{
		"action":   "events",
		"start":    "2025-01-01T00:00:00Z",
		"end":      "2025-01-02T00:00:00Z",
		"measures": []any{"queue_depth"},
	})
	assert.False(t, result.IsError)
}

func TestMetricsEvents_PendingTimeseriesRoute(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/metrics/events-pending-timeseries": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		},
	})

	result := callTool(t, session, "gateway_metrics_read", map[string]any{
		"action":      "events",
		"start":       "2025-01-01T00:00:00Z",
		"end":         "2025-01-02T00:00:00Z",
		"measures":    []any{"pending"},
		"granularity": "1h",
	})
	assert.False(t, result.IsError)
}

func TestMetricsEvents_ByIssueRoute(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/metrics/events-by-issue": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		},
	})

	result := callTool(t, session, "gateway_metrics_read", map[string]any{
		"action":     "events",
		"start":      "2025-01-01T00:00:00Z",
		"end":        "2025-01-02T00:00:00Z",
		"measures":   []any{"count"},
		"dimensions": []any{"issue_id"},
		// The endpoint filters on issue_id, so the route is meaningless without
		// one. This argument used to be absent here and the call still counted
		// as a success, which is the behaviour the CLI has always rejected.
		"issue_id": "iss_123",
	})
	assert.False(t, result.IsError)
}

// TestMetricsEvents_ByIssueRequiresIssueID pins the other half: routing to
// events-by-issue without an issue_id is a caller mistake, not a query. The CLI
// has always said so; MCP used to send the request anyway.
func TestMetricsEvents_ByIssueRequiresIssueID(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/metrics/events-by-issue": func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("must not reach the API without an issue_id")
		},
	})

	result := callTool(t, session, "gateway_metrics_read", map[string]any{
		"action":     "events",
		"start":      "2025-01-01T00:00:00Z",
		"end":        "2025-01-02T00:00:00Z",
		"measures":   []any{"count"},
		"dimensions": []any{"issue_id"},
	})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "issue_id")
}

func TestMetricsRequests_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/metrics/requests": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		},
	})

	result := callTool(t, session, "gateway_metrics_read", map[string]any{
		"action":   "requests",
		"start":    "2025-01-01T00:00:00Z",
		"end":      "2025-01-02T00:00:00Z",
		"measures": []any{"count"},
	})
	assert.False(t, result.IsError)
}

func TestMetricsAttempts_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/metrics/attempts": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		},
	})

	result := callTool(t, session, "gateway_metrics_read", map[string]any{
		"action":   "attempts",
		"start":    "2025-01-01T00:00:00Z",
		"end":      "2025-01-02T00:00:00Z",
		"measures": []any{"count"},
	})
	assert.False(t, result.IsError)
}

func TestMetricsTransformations_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/metrics/transformations": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		},
	})

	result := callTool(t, session, "gateway_metrics_read", map[string]any{
		"action":   "transformations",
		"start":    "2025-01-01T00:00:00Z",
		"end":      "2025-01-02T00:00:00Z",
		"measures": []any{"count"},
	})
	assert.False(t, result.IsError)
}

func TestMetricsTool_UnknownAction(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "gateway_metrics_read", map[string]any{
		"action":   "invalid",
		"start":    "2025-01-01T00:00:00Z",
		"end":      "2025-01-02T00:00:00Z",
		"measures": []any{"count"},
	})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "unknown action")
}

// ---------------------------------------------------------------------------
// Login tool
// ---------------------------------------------------------------------------

func TestLoginTool_AlreadyAuthenticated(t *testing.T) {
	api := mockAPI(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/cli-auth/validate": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{
				"user_id":           "usr_1",
				"user_name":         "Test User",
				"user_email":        "u@example.com",
				"organization_name": "Org",
				"organization_id":   "org_1",
				"team_id":           "tm_1",
				"team_name_no_org":  "Proj",
				"team_type":         "event_gateway",
			})
		},
	})
	client := newTestClient(api.URL, "test-key")
	session := connectInMemory(t, client)

	result := callTool(t, session, "hookdeck_login", map[string]any{})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "Already authenticated")
}

func TestLoginTool_CIScopedKeyStartsLogin(t *testing.T) {
	api := mockAPI(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/cli-auth/validate": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{
				"organization_name": "Org",
				"organization_id":   "org_1",
				"team_id":           "tm_ci",
				"team_name_no_org":  "CI Project",
				"team_type":         "event_gateway",
			})
		},
		hookdeck.APIPathPrefix + "/cli-auth": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{
				"browser_url": "https://hookdeck.com/auth?code=ci-upgrade",
				"poll_url":    "http://" + r.Host + hookdeck.APIPathPrefix + "/cli-auth/poll?key=ci-upgrade",
			})
		},
		hookdeck.APIPathPrefix + "/cli-auth/poll": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"claimed": false})
		},
	})
	client := newTestClient(api.URL, "test-key")
	session := connectInMemory(t, client)

	result := callTool(t, session, "hookdeck_login", map[string]any{})
	assert.False(t, result.IsError)
	text := textContent(t, result)
	assert.NotContains(t, text, "Already authenticated")
	assert.Contains(t, text, "Login initiated")
	assert.Contains(t, text, "scoped to one project")
}

func TestLoginTool_UnauthorizedKeyNoScopedPrefix(t *testing.T) {
	api := mockAPI(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/cli-auth/validate": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("Unauthorized"))
		},
		hookdeck.APIPathPrefix + "/cli-auth": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{
				"browser_url": "https://hookdeck.com/auth?code=revoked",
				"poll_url":    "http://" + r.Host + hookdeck.APIPathPrefix + "/cli-auth/poll?key=revoked",
			})
		},
		hookdeck.APIPathPrefix + "/cli-auth/poll": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"claimed": false})
		},
	})
	client := newTestClient(api.URL, "test-key")
	session := connectInMemory(t, client)

	result := callTool(t, session, "hookdeck_login", map[string]any{})
	assert.False(t, result.IsError)
	text := textContent(t, result)
	assert.Contains(t, text, "Login initiated")
	assert.NotContains(t, text, "scoped to one project")
}

func TestLoginTool_ReauthStartsFreshLogin(t *testing.T) {
	api := mockAPI(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/cli-auth": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{
				"browser_url": "https://hookdeck.com/auth?code=reauth",
				"poll_url":    "http://" + r.Host + hookdeck.APIPathPrefix + "/cli-auth/poll?key=reauth",
			})
		},
		hookdeck.APIPathPrefix + "/cli-auth/poll": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"claimed": false})
		},
	})

	client := newTestClient(api.URL, "sk_test_123456789012")
	cfg := &config.Config{APIBaseURL: api.URL}
	srv := NewServer(ServerOptions{Client: client, Config: cfg})

	serverTransport, clientTransport := mcpsdk.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = srv.Run(ctx, serverTransport) }()

	mcpClient := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "0.0.1"}, nil)
	session, err := mcpClient.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })

	result := callTool(t, session, "hookdeck_login", map[string]any{"reauth": true})
	assert.False(t, result.IsError)
	text := textContent(t, result)
	assert.Contains(t, text, "https://hookdeck.com/auth?code=reauth")
	assert.Empty(t, client.APIKey)
}

func TestLoginTool_ReturnsURLImmediately(t *testing.T) {
	// Mock the /cli-auth endpoint to return a browser URL and a poll URL
	// that never completes (simulates user not yet opening browser).
	authCalled := false
	api := mockAPI(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/cli-auth": func(w http.ResponseWriter, r *http.Request) {
			authCalled = true
			json.NewEncoder(w).Encode(map[string]any{
				"browser_url": "https://hookdeck.com/auth?code=abc123",
				"poll_url":    "http://" + r.Host + hookdeck.APIPathPrefix + "/cli-auth/poll?key=abc123",
			})
		},
		hookdeck.APIPathPrefix + "/cli-auth/poll": func(w http.ResponseWriter, r *http.Request) {
			// Never claimed — user hasn't opened the browser yet.
			json.NewEncoder(w).Encode(map[string]any{"claimed": false})
		},
	})

	unauthClient := newTestClient(api.URL, "")
	cfg := &config.Config{APIBaseURL: api.URL}
	srv := NewServer(ServerOptions{Client: unauthClient, Config: cfg})

	serverTransport, clientTransport := mcpsdk.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = srv.Run(ctx, serverTransport) }()

	mcpClient := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "0.0.1"}, nil)
	session, err := mcpClient.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })

	// The call should return immediately (not block for 4 minutes).
	result := callTool(t, session, "hookdeck_login", map[string]any{})
	assert.True(t, authCalled, "should have called /cli-auth")
	assert.False(t, result.IsError)
	text := textContent(t, result)
	assert.Contains(t, text, "https://hookdeck.com/auth?code=abc123")
	assert.Contains(t, text, "browser")
}

func TestLoginTool_InProgressShowsURL(t *testing.T) {
	api := mockAPI(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/cli-auth": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{
				"browser_url": "https://hookdeck.com/auth?code=xyz",
				"poll_url":    "http://" + r.Host + hookdeck.APIPathPrefix + "/cli-auth/poll?key=xyz",
			})
		},
		hookdeck.APIPathPrefix + "/cli-auth/poll": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"claimed": false})
		},
	})

	unauthClient := newTestClient(api.URL, "")
	cfg := &config.Config{APIBaseURL: api.URL}
	srv := NewServer(ServerOptions{Client: unauthClient, Config: cfg})

	serverTransport, clientTransport := mcpsdk.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = srv.Run(ctx, serverTransport) }()

	mcpClient := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "0.0.1"}, nil)
	session, err := mcpClient.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })

	// First call starts the flow.
	_ = callTool(t, session, "hookdeck_login", map[string]any{})

	// Second call should report "in progress" with the URL.
	result := callTool(t, session, "hookdeck_login", map[string]any{})
	assert.False(t, result.IsError)
	text := textContent(t, result)
	assert.Contains(t, text, "already in progress")
	assert.Contains(t, text, "https://hookdeck.com/auth?code=xyz")
}

func TestLoginTool_PollSurvivesAcrossToolCalls(t *testing.T) {
	// Regression: the login polling goroutine must use the session-level
	// context, not the per-request ctx (which is cancelled when the handler
	// returns). If the goroutine selected on per-request ctx, it would be
	// cancelled immediately and the second hookdeck_login call would see a
	// "login cancelled" error instead of "Already authenticated".
	pollCount := 0
	api := mockAPI(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/cli-auth": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{
				"browser_url": "https://hookdeck.com/auth?code=survive",
				"poll_url":    "http://" + r.Host + hookdeck.APIPathPrefix + "/cli-auth/poll?key=survive",
			})
		},
		hookdeck.APIPathPrefix + "/cli-auth/poll": func(w http.ResponseWriter, r *http.Request) {
			pollCount++
			if pollCount >= 2 {
				// Simulate user completing browser auth on 2nd poll.
				json.NewEncoder(w).Encode(map[string]any{
					"claimed":           true,
					"key":               "sk_test_survive12345",
					"team_id":           "proj_survive",
					"team_name":         "Survive Project",
					"team_type":         "console",
					"user_name":         "test-user",
					"organization_name": "test-org",
				})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"claimed": false})
		},
	})

	unauthClient := newTestClient(api.URL, "")
	cfg := &config.Config{APIBaseURL: api.URL}
	srv := NewServer(ServerOptions{Client: unauthClient, Config: cfg})

	serverTransport, clientTransport := mcpsdk.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = srv.Run(ctx, serverTransport) }()

	mcpClient := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "0.0.1"}, nil)
	session, err := mcpClient.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })

	// First call initiates the flow — handler returns immediately.
	result := callTool(t, session, "hookdeck_login", map[string]any{})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "https://hookdeck.com/auth?code=survive")

	// Wait for the background poll loop: first unclaimed response is followed by
	// mcpcore.LoginPollInterval sleep inside pollForAPIKey before the second poll succeeds.
	time.Sleep(mcpcore.LoginPollInterval + 300*time.Millisecond)

	// Second call — if the goroutine survived, the client is now authenticated.
	result2 := callTool(t, session, "hookdeck_login", map[string]any{})
	assert.False(t, result2.IsError)
	text := textContent(t, result2)
	assert.Contains(t, text, "Already authenticated")
	assert.Equal(t, "sk_test_survive12345", unauthClient.APIKey)
}

// ---------------------------------------------------------------------------
// API error scenarios (shared across tools)
// ---------------------------------------------------------------------------

func TestSourcesList_404Error(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/sources": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{"message": "workspace not found"})
		},
	})

	result := callTool(t, session, "gateway_sources_read", map[string]any{"action": "list"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "not found")
}

func TestSourcesList_422ValidationError(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/sources": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(map[string]any{"message": "invalid parameter: limit must be positive"})
		},
	})

	result := callTool(t, session, "gateway_sources_read", map[string]any{"action": "list"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "invalid parameter")
}

func TestSourcesList_429RateLimitError(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/sources": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]any{"message": "rate limited"})
		},
	})

	result := callTool(t, session, "gateway_sources_read", map[string]any{"action": "list"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "Rate limited")
}

func TestEventGet_APIError(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/events/evt_nope": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{"message": "event not found"})
		},
	})

	result := callTool(t, session, "gateway_event_read", map[string]any{"action": "get", "id": "evt_nope"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "not found")
}

// ---------------------------------------------------------------------------
// Input parsing edge cases
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Server instructions
// ---------------------------------------------------------------------------

func TestServerInfo_NameAndVersion(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	info := session.InitializeResult()
	require.NotNil(t, info)
	assert.Equal(t, "hookdeck-gateway", info.ServerInfo.Name)
	assert.NotEmpty(t, info.ServerInfo.Version)
}

// ---------------------------------------------------------------------------
// Help tool: all topics return valid content
// ---------------------------------------------------------------------------

func TestHelpTool_AllTopics(t *testing.T) {
	topics := []struct {
		name           string
		expectContains string
	}{
		{"hookdeck_projects_read", "list"},
		{"gateway_connections_read", "list"},
		// pause moved to its own tool, so it has its own topic.
		{"gateway_connections_pause", "unpause"},
		{"gateway_sources_read", "list"},
		{"gateway_destinations_read", "HTTP"},
		{"gateway_transformations_read", "JavaScript"},
		{"gateway_requests_read", "list"},
		{"gateway_request_read", "raw_body"},
		{"gateway_events_read", "list"},
		{"gateway_event_read", "raw_body"},
		{"gateway_attempts_read", "event_id"},
		{"gateway_issues_read", "delivery"},
		{"gateway_metrics_read", "granularity"},
		{"gateway_help", "topic"},
	}

	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	for _, tt := range topics {
		t.Run(tt.name, func(t *testing.T) {
			result := callTool(t, session, "gateway_help", map[string]any{"topic": tt.name})
			assert.False(t, result.IsError, "help for %s should not be an error", tt.name)
			text := textContent(t, result)
			assert.Contains(t, text, tt.expectContains,
				"help for %s should mention %q", tt.name, tt.expectContains)
		})
	}
}

func TestHelpTool_ShortNames(t *testing.T) {
	shortNames := []string{
		"projects", "connections", "sources", "destinations",
		"transformations", "requests", "request", "events", "event",
		"attempts", "issues", "metrics", "help",
	}

	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	for _, name := range shortNames {
		t.Run(name, func(t *testing.T) {
			result := callTool(t, session, "gateway_help", map[string]any{"topic": name})
			assert.False(t, result.IsError, "short name %q should resolve", name)
			// Logging in and switching project are platform operations and keep
			// the hookdeck_ prefix; every product tool takes gateway_.
			want := "gateway_" + name
			if name == "projects" || name == "login" {
				want = "hookdeck_" + name
			}
			assert.Contains(t, textContent(t, result), want)
		})
	}
}

func TestHelpTool_OverviewListsAllTools(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	result := callTool(t, session, "gateway_help", map[string]any{})
	assert.False(t, result.IsError)
	text := textContent(t, result)

	expectedTools := []string{
		"hookdeck_projects_read", "gateway_connections_read", "gateway_sources_read",
		"gateway_destinations_read", "gateway_transformations_read",
		"gateway_requests_read", "gateway_request_read",
		"gateway_events_read", "gateway_event_read", "gateway_attempts_read", "gateway_issues_read",
		"gateway_metrics_read", "gateway_help",
	}
	for _, tool := range expectedTools {
		assert.Contains(t, text, tool, "overview should list %s", tool)
	}
}

func TestHelpTool_OverviewShowsProjectNotSet(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	client.ProjectID = "" // no project set
	session := connectInMemory(t, client)

	result := callTool(t, session, "gateway_help", map[string]any{})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "not set")
}

func TestHelpTool_UnknownTopicListsAvailable(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	result := callTool(t, session, "gateway_help", map[string]any{"topic": "bogus"})
	assert.True(t, result.IsError)
	text := textContent(t, result)
	assert.Contains(t, text, "No help found")
	assert.Contains(t, text, "gateway_events_read") // lists available tools
}

// ---------------------------------------------------------------------------
// Error feedback: 500 server error through HTTP flow
// ---------------------------------------------------------------------------

func TestDestinationsGet_500ServerError(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/destinations/des_fail": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]any{"message": "internal server error"})
		},
	})

	result := callTool(t, session, "gateway_destinations_read", map[string]any{"action": "get", "id": "des_fail"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "Hookdeck API error")
}

func TestConnectionsGet_401UnauthorizedError(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/connections/web_bad": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{"message": "invalid api key"})
		},
	})

	result := callTool(t, session, "gateway_connections_read", map[string]any{"action": "get", "id": "web_bad"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "Authentication failed")
}

func TestIssuesList_422ValidationError(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/issues": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(map[string]any{"message": "invalid filter: bad_field"})
		},
	})

	result := callTool(t, session, "gateway_issues_read", map[string]any{"action": "list"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "invalid filter")
}

func TestAttemptsList_429RateLimitError(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/attempts": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]any{"message": "too many requests"})
		},
	})

	result := callTool(t, session, "gateway_attempts_read", map[string]any{"action": "list"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "Rate limited")
}

// ---------------------------------------------------------------------------
// Error translation: additional cases
// ---------------------------------------------------------------------------

// TestArgumentValuesThatCannotBeUsedAreRejected covers the second half of what
// rejectUnknownArgs is for. A name the tool does not have is one way to get a
// wrong answer that reads as a right one; a name it does have carrying a value
// it cannot use is the other. The input helpers discarded those values, so
// {"measures":["count",5]} queried one measure and reported success.
func TestArgumentValuesThatCannotBeUsedAreRejected(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	rejected := []struct {
		name string
		tool string
		args map[string]any
		want string
	}{
		{
			name: "a non-string inside a string array",
			tool: "gateway_metrics_read",
			args: map[string]any{"action": "events", "measures": []any{"count", 5}},
			want: "measures[1] must be a string",
		},
		{
			name: "an array where a single value belongs",
			tool: "gateway_events_read",
			args: map[string]any{"action": "list", "status": []any{"FAILED"}},
			want: "status takes a single value, not an array",
		},
		{
			name: "a scalar where a JSON filter belongs",
			tool: "gateway_events_read",
			args: map[string]any{"action": "list", "body": 42},
			want: "body must be a JSON string or object",
		},
	}

	for _, tt := range rejected {
		t.Run(tt.name, func(t *testing.T) {
			result := callTool(t, session, tt.tool, tt.args)
			assert.True(t, result.IsError, "the call must be reported as an error")
			assert.Contains(t, textContent(t, result), tt.want)
		})
	}
}

// The conversions the input helpers make on purpose must survive the check
// above: rejecting these would turn a deliberate convenience into an error.
func TestDeliberateArgumentConversionsStillWork(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	accepted := []struct {
		name string
		tool string
		args map[string]any
	}{
		{"a comma-separated string for an array", "gateway_metrics_read",
			map[string]any{"action": "events", "measures": "count,failed_count"}},
		{"a JSON filter given as an object", "gateway_events_read",
			map[string]any{"action": "list", "body": map[string]any{"type": "charge.succeeded"}}},
		{"a JSON filter given as a string", "gateway_events_read",
			map[string]any{"action": "list", "body": `{"type":"charge.succeeded"}`}},
	}

	for _, tt := range accepted {
		t.Run(tt.name, func(t *testing.T) {
			result := callTool(t, session, tt.tool, tt.args)
			// The call reaches the API (and fails there, unauthenticated); what
			// matters is that it was not rejected by argument validation.
			assert.NotContains(t, textContent(t, result), "must be")
			assert.NotContains(t, textContent(t, result), "takes a single value")
		})
	}
}
