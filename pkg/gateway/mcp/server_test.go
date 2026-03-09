package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// --- helpers ---

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
	cfg := &config.Config{}
	srv := NewServer(client, cfg)

	serverTransport, clientTransport := mcpsdk.NewInMemoryTransports()

	// Run server in background.
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() {
		_ = srv.mcpServer.Run(ctx, serverTransport)
	}()

	// Connect client.
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

// --- Test: Server initialization and tool listing ---

func TestListTools_Authenticated(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-api-key")
	session := connectInMemory(t, client)

	result, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)

	toolNames := make([]string, len(result.Tools))
	for i, tool := range result.Tools {
		toolNames[i] = tool.Name
	}

	// When authenticated, hookdeck_login should NOT be present.
	assert.NotContains(t, toolNames, "hookdeck_login")

	// All 11 standard tools should be present.
	expectedTools := []string{
		"hookdeck_projects",
		"hookdeck_connections",
		"hookdeck_sources",
		"hookdeck_destinations",
		"hookdeck_transformations",
		"hookdeck_requests",
		"hookdeck_events",
		"hookdeck_attempts",
		"hookdeck_issues",
		"hookdeck_metrics",
		"hookdeck_help",
	}
	for _, name := range expectedTools {
		assert.Contains(t, toolNames, name)
	}
}

func TestListTools_Unauthenticated(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "") // no API key
	session := connectInMemory(t, client)

	result, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)

	toolNames := make([]string, len(result.Tools))
	for i, tool := range result.Tools {
		toolNames[i] = tool.Name
	}

	// When unauthenticated, hookdeck_login SHOULD be present.
	assert.Contains(t, toolNames, "hookdeck_login")

	// All 11 standard tools should still be present.
	assert.Contains(t, toolNames, "hookdeck_help")
	assert.Contains(t, toolNames, "hookdeck_events")
}

// --- Test: Help tool (no API calls) ---

func TestHelpTool_Overview(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	result, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "hookdeck_help",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := textContent(t, result)
	assert.Contains(t, text, "hookdeck_events")
	assert.Contains(t, text, "hookdeck_connections")
	assert.Contains(t, text, "hookdeck_sources")
}

func TestHelpTool_SpecificTopic(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	result, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "hookdeck_help",
		Arguments: map[string]any{"topic": "hookdeck_events"},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := textContent(t, result)
	assert.Contains(t, text, "list")
	assert.Contains(t, text, "get")
}

func TestHelpTool_UnknownTopic(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	result, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "hookdeck_help",
		Arguments: map[string]any{"topic": "nonexistent_tool"},
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "No help found")
}

// --- Test: Auth guard on resource tools ---

func TestAuthGuard_UnauthenticatedReturnsError(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "") // no API key
	session := connectInMemory(t, client)

	// All resource tools should return auth error when unauthenticated.
	resourceTools := []string{
		"hookdeck_sources",
		"hookdeck_destinations",
		"hookdeck_connections",
		"hookdeck_events",
		"hookdeck_requests",
		"hookdeck_attempts",
		"hookdeck_issues",
		"hookdeck_transformations",
		"hookdeck_metrics",
		"hookdeck_projects",
	}

	for _, toolName := range resourceTools {
		t.Run(toolName, func(t *testing.T) {
			result, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
				Name:      toolName,
				Arguments: map[string]any{"action": "list"},
			})
			require.NoError(t, err)
			assert.True(t, result.IsError, "expected IsError=true for unauthenticated %s", toolName)
			assert.Contains(t, textContent(t, result), "hookdeck_login")
		})
	}
}

// --- Test: Error translation ---

func TestTranslateAPIError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantSubstr string
	}{
		{
			name:       "401 Unauthorized",
			err:        &hookdeck.APIError{StatusCode: 401, Message: "bad key"},
			wantSubstr: "Authentication failed",
		},
		{
			name:       "404 Not Found",
			err:        &hookdeck.APIError{StatusCode: 404, Message: "resource xyz"},
			wantSubstr: "Resource not found",
		},
		{
			name:       "422 Validation",
			err:        &hookdeck.APIError{StatusCode: 422, Message: "invalid field foo"},
			wantSubstr: "invalid field foo",
		},
		{
			name:       "429 Rate Limit",
			err:        &hookdeck.APIError{StatusCode: 429, Message: "slow down"},
			wantSubstr: "Rate limited",
		},
		{
			name:       "500 Server Error",
			err:        &hookdeck.APIError{StatusCode: 500, Message: "internal"},
			wantSubstr: "Hookdeck API error",
		},
		{
			name:       "Non-API error",
			err:        fmt.Errorf("network timeout"),
			wantSubstr: "network timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := TranslateAPIError(tt.err)
			assert.Contains(t, msg, tt.wantSubstr)
		})
	}
}

// --- Test: Tool calls with mock API server ---

// mockAPI creates an httptest server that handles specific API paths.
func mockAPI(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for pattern, handler := range handlers {
		mux.HandleFunc(pattern, handler)
	}
	// Default handler for unmatched routes.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t.Logf("unhandled request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{"message": "not found: " + r.URL.Path})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestSourcesList_Success(t *testing.T) {
	apiResp := map[string]any{
		"models": []map[string]any{
			{"id": "src_123", "name": "my-source"},
		},
		"pagination": map[string]any{
			"order_by": "created_at",
			"dir":      "desc",
		},
	}

	api := mockAPI(t, map[string]http.HandlerFunc{
		"/2025-07-01/sources": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(apiResp)
		},
	})

	client := newTestClient(api.URL, "test-key")
	session := connectInMemory(t, client)

	result, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "hookdeck_sources",
		Arguments: map[string]any{"action": "list"},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := textContent(t, result)
	assert.Contains(t, text, "src_123")
	assert.Contains(t, text, "my-source")
}

func TestSourcesGet_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	result, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "hookdeck_sources",
		Arguments: map[string]any{"action": "get"},
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id is required")
}

func TestEventsList_WithMockAPI(t *testing.T) {
	apiResp := map[string]any{
		"models": []map[string]any{
			{"id": "evt_abc", "status": "SUCCESSFUL"},
		},
		"pagination": map[string]any{
			"order_by": "created_at",
			"dir":      "desc",
		},
	}

	api := mockAPI(t, map[string]http.HandlerFunc{
		"/2025-07-01/events": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(apiResp)
		},
	})

	client := newTestClient(api.URL, "test-key")
	session := connectInMemory(t, client)

	result, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "hookdeck_events",
		Arguments: map[string]any{"action": "list"},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "evt_abc")
}

func TestConnectionsList_WithMockAPI(t *testing.T) {
	apiResp := map[string]any{
		"models": []map[string]any{
			{"id": "web_conn1", "name": "stripe-to-backend"},
		},
		"pagination": map[string]any{},
	}

	api := mockAPI(t, map[string]http.HandlerFunc{
		"/2025-07-01/connections": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(apiResp)
		},
	})

	client := newTestClient(api.URL, "test-key")
	session := connectInMemory(t, client)

	result, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "hookdeck_connections",
		Arguments: map[string]any{"action": "list"},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "stripe-to-backend")
}

// --- Test: API error scenarios via mock ---

func TestSourcesList_404Error(t *testing.T) {
	api := mockAPI(t, map[string]http.HandlerFunc{
		"/2025-07-01/sources": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{"message": "workspace not found"})
		},
	})

	client := newTestClient(api.URL, "test-key")
	session := connectInMemory(t, client)

	result, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "hookdeck_sources",
		Arguments: map[string]any{"action": "list"},
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "not found")
}

func TestSourcesList_422ValidationError(t *testing.T) {
	api := mockAPI(t, map[string]http.HandlerFunc{
		"/2025-07-01/sources": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(map[string]any{"message": "invalid parameter: limit must be positive"})
		},
	})

	client := newTestClient(api.URL, "test-key")
	session := connectInMemory(t, client)

	result, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "hookdeck_sources",
		Arguments: map[string]any{"action": "list"},
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "invalid parameter")
}

func TestSourcesList_429RateLimitError(t *testing.T) {
	api := mockAPI(t, map[string]http.HandlerFunc{
		"/2025-07-01/sources": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]any{"message": "rate limited"})
		},
	})

	client := newTestClient(api.URL, "test-key")
	client.SuppressRateLimitErrors = true
	session := connectInMemory(t, client)

	result, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "hookdeck_sources",
		Arguments: map[string]any{"action": "list"},
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "Rate limited")
}

// --- Test: Invalid action ---

func TestSourcesTool_UnknownAction(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	result, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "hookdeck_sources",
		Arguments: map[string]any{"action": "delete"},
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "unknown action")
}

// --- Test: Metrics tool requires start/end/measures ---

func TestMetricsTool_MissingRequired(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	result, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "hookdeck_metrics",
		Arguments: map[string]any{"action": "events"},
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	text := textContent(t, result)
	assert.Contains(t, text, "required")
}

// --- Test: Issues tool actions ---

func TestIssuesTool_List(t *testing.T) {
	apiResp := map[string]any{
		"models": []map[string]any{
			{"id": "iss_001", "type": "delivery", "status": "OPENED"},
		},
		"pagination": map[string]any{},
	}

	api := mockAPI(t, map[string]http.HandlerFunc{
		"/2025-07-01/issues": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(apiResp)
		},
	})

	client := newTestClient(api.URL, "test-key")
	session := connectInMemory(t, client)

	result, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "hookdeck_issues",
		Arguments: map[string]any{"action": "list"},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "iss_001")
}

func TestIssuesTool_GetMissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	result, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "hookdeck_issues",
		Arguments: map[string]any{"action": "get"},
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id is required")
}
