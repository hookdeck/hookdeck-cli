package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
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
	cfg := &config.Config{}
	srv := NewServer(client, cfg)

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
func mockAPI(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for pattern, handler := range handlers {
		mux.HandleFunc(pattern, handler)
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
		"hookdeck_projects", "hookdeck_connections", "hookdeck_sources",
		"hookdeck_destinations", "hookdeck_transformations", "hookdeck_requests",
		"hookdeck_events", "hookdeck_attempts", "hookdeck_issues",
		"hookdeck_metrics", "hookdeck_help",
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
	assert.Contains(t, toolNames, "hookdeck_help")
	assert.Contains(t, toolNames, "hookdeck_events")
}

// ---------------------------------------------------------------------------
// Help tool
// ---------------------------------------------------------------------------

func TestHelpTool_Overview(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	result := callTool(t, session, "hookdeck_help", map[string]any{})
	assert.False(t, result.IsError)

	text := textContent(t, result)
	assert.Contains(t, text, "hookdeck_events")
	assert.Contains(t, text, "hookdeck_connections")
	assert.Contains(t, text, "hookdeck_sources")
	assert.Contains(t, text, "proj_test123") // current project
}

func TestHelpTool_SpecificTopic(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	result := callTool(t, session, "hookdeck_help", map[string]any{"topic": "hookdeck_events"})
	assert.False(t, result.IsError)
	text := textContent(t, result)
	assert.Contains(t, text, "list")
	assert.Contains(t, text, "get")
	assert.Contains(t, text, "raw_body")
}

func TestHelpTool_ShortTopicName(t *testing.T) {
	// "events" should resolve to "hookdeck_events"
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	result := callTool(t, session, "hookdeck_help", map[string]any{"topic": "events"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "hookdeck_events")
}

func TestHelpTool_UnknownTopic(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	result := callTool(t, session, "hookdeck_help", map[string]any{"topic": "nonexistent_tool"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "No help found")
}

// ---------------------------------------------------------------------------
// Auth guard on resource tools
// ---------------------------------------------------------------------------

func TestAuthGuard_UnauthenticatedReturnsError(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "")
	session := connectInMemory(t, client)

	resourceTools := []string{
		"hookdeck_sources", "hookdeck_destinations", "hookdeck_connections",
		"hookdeck_events", "hookdeck_requests", "hookdeck_attempts",
		"hookdeck_issues", "hookdeck_transformations", "hookdeck_metrics",
		"hookdeck_projects",
	}

	for _, toolName := range resourceTools {
		t.Run(toolName, func(t *testing.T) {
			result := callTool(t, session, toolName, map[string]any{"action": "list"})
			assert.True(t, result.IsError, "expected IsError=true for unauthenticated %s", toolName)
			assert.Contains(t, textContent(t, result), "hookdeck_login")
		})
	}
}

// ---------------------------------------------------------------------------
// Error translation
// ---------------------------------------------------------------------------

func TestTranslateAPIError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantSubstr string
	}{
		{"401 Unauthorized", &hookdeck.APIError{StatusCode: 401, Message: "bad key"}, "Authentication failed"},
		{"404 Not Found", &hookdeck.APIError{StatusCode: 404, Message: "resource xyz"}, "Resource not found"},
		{"410 Gone", &hookdeck.APIError{StatusCode: 410, Message: "resource xyz"}, "Resource not found"},
		{"422 Validation", &hookdeck.APIError{StatusCode: 422, Message: "invalid field foo"}, "invalid field foo"},
		{"429 Rate Limit", &hookdeck.APIError{StatusCode: 429, Message: "slow down"}, "Rate limited"},
		{"500 Server Error", &hookdeck.APIError{StatusCode: 500, Message: "internal"}, "Hookdeck API error"},
		{"Non-API error", fmt.Errorf("network timeout"), "network timeout"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := TranslateAPIError(tt.err)
			assert.Contains(t, msg, tt.wantSubstr)
		})
	}
}

// ---------------------------------------------------------------------------
// Sources tool
// ---------------------------------------------------------------------------

func TestSourcesList_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/sources": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "src_123", "name": "my-source"}))
		},
	})

	result := callTool(t, session, "hookdeck_sources", map[string]any{"action": "list"})
	assert.False(t, result.IsError)
	text := textContent(t, result)
	assert.Contains(t, text, "src_123")
	assert.Contains(t, text, "my-source")
}

func TestSourcesGet_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/sources/src_123": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"id": "src_123", "name": "github-webhooks"})
		},
	})

	result := callTool(t, session, "hookdeck_sources", map[string]any{"action": "get", "id": "src_123"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "github-webhooks")
}

func TestSourcesGet_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "hookdeck_sources", map[string]any{"action": "get"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id is required")
}

func TestSourcesTool_UnknownAction(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "hookdeck_sources", map[string]any{"action": "delete"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "unknown action")
}

// ---------------------------------------------------------------------------
// Destinations tool
// ---------------------------------------------------------------------------

func TestDestinationsList_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/destinations": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "des_456", "name": "my-backend"}))
		},
	})

	result := callTool(t, session, "hookdeck_destinations", map[string]any{"action": "list"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "des_456")
}

func TestDestinationsGet_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/destinations/des_456": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"id": "des_456", "name": "my-backend"})
		},
	})

	result := callTool(t, session, "hookdeck_destinations", map[string]any{"action": "get", "id": "des_456"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "des_456")
}

func TestDestinationsGet_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "hookdeck_destinations", map[string]any{"action": "get"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id is required")
}

func TestDestinationsTool_UnknownAction(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "hookdeck_destinations", map[string]any{"action": "create"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "unknown action")
}

// ---------------------------------------------------------------------------
// Connections tool
// ---------------------------------------------------------------------------

func TestConnectionsList_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/connections": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "web_conn1", "name": "stripe-to-backend"}))
		},
	})

	result := callTool(t, session, "hookdeck_connections", map[string]any{"action": "list"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "stripe-to-backend")
}

func TestConnectionsGet_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/connections/web_conn1": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"id": "web_conn1", "name": "stripe-to-backend"})
		},
	})

	result := callTool(t, session, "hookdeck_connections", map[string]any{"action": "get", "id": "web_conn1"})
	assert.False(t, result.IsError)
	text := textContent(t, result)
	assert.Contains(t, text, "web_conn1")
	assert.Contains(t, text, `"data"`)
	assert.Contains(t, text, `"meta"`)
	assert.Contains(t, text, `"active_project_id"`)
}

func TestConnectionsGet_ByName(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/connections": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "stripe-to-backend", r.URL.Query().Get("name"))
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "web_conn1", "name": "stripe-to-backend"}))
		},
		"/2025-07-01/connections/web_conn1": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			json.NewEncoder(w).Encode(map[string]any{"id": "web_conn1", "name": "stripe-to-backend"})
		},
	})

	result := callTool(t, session, "hookdeck_connections", map[string]any{"action": "get", "id": "stripe-to-backend"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "web_conn1")
}

func TestConnectionsGet_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "hookdeck_connections", map[string]any{"action": "get"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id or name is required")
}

func TestConnectionsPause_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/connections/web_conn1": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			json.NewEncoder(w).Encode(map[string]any{"id": "web_conn1", "name": "stripe-to-backend"})
		},
		"/2025-07-01/connections/web_conn1/pause": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "PUT", r.Method)
			json.NewEncoder(w).Encode(map[string]any{"id": "web_conn1", "paused_at": "2025-01-01T00:00:00Z"})
		},
	})

	result := callTool(t, session, "hookdeck_connections", map[string]any{"action": "pause", "id": "web_conn1"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "web_conn1")
}

func TestConnectionsPause_ByName(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/connections": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "stripe-to-backend", r.URL.Query().Get("name"))
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "web_conn1", "name": "stripe-to-backend"}))
		},
		"/2025-07-01/connections/web_conn1/pause": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "PUT", r.Method)
			json.NewEncoder(w).Encode(map[string]any{"id": "web_conn1", "paused_at": "2025-01-01T00:00:00Z"})
		},
	})

	result := callTool(t, session, "hookdeck_connections", map[string]any{"action": "pause", "id": "stripe-to-backend"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "web_conn1")
}

func TestConnectionsPause_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "hookdeck_connections", map[string]any{"action": "pause"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id or name is required")
}

func TestConnectionsUnpause_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/connections/web_conn1": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			json.NewEncoder(w).Encode(map[string]any{"id": "web_conn1", "name": "stripe-to-backend"})
		},
		"/2025-07-01/connections/web_conn1/unpause": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "PUT", r.Method)
			json.NewEncoder(w).Encode(map[string]any{"id": "web_conn1"})
		},
	})

	result := callTool(t, session, "hookdeck_connections", map[string]any{"action": "unpause", "id": "web_conn1"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "web_conn1")
}

func TestConnectionsUnpause_ByName(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/connections": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "stripe-to-backend", r.URL.Query().Get("name"))
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "web_conn1", "name": "stripe-to-backend"}))
		},
		"/2025-07-01/connections/web_conn1/unpause": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "PUT", r.Method)
			json.NewEncoder(w).Encode(map[string]any{"id": "web_conn1"})
		},
	})

	result := callTool(t, session, "hookdeck_connections", map[string]any{"action": "unpause", "id": "stripe-to-backend"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "web_conn1")
}

func TestConnectionsUnpause_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "hookdeck_connections", map[string]any{"action": "unpause"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id or name is required")
}

func TestConnectionsTool_UnknownAction(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "hookdeck_connections", map[string]any{"action": "delete"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "unknown action")
}

func TestConnectionsList_DisabledFilter(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/connections": func(w http.ResponseWriter, r *http.Request) {
			// Verify disabled_at[any]=true is sent when disabled=true
			assert.Equal(t, "true", r.URL.Query().Get("disabled_at[any]"))
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "web_1"}))
		},
	})

	result := callTool(t, session, "hookdeck_connections", map[string]any{"action": "list", "disabled": true})
	assert.False(t, result.IsError)
}

// ---------------------------------------------------------------------------
// Transformations tool
// ---------------------------------------------------------------------------

func TestTransformationsList_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/transformations": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "trn_789", "name": "enrich-payload"}))
		},
	})

	result := callTool(t, session, "hookdeck_transformations", map[string]any{"action": "list"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "trn_789")
}

func TestTransformationsGet_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/transformations/trn_789": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"id": "trn_789", "name": "enrich-payload", "code": "module.exports = (req) => req"})
		},
	})

	result := callTool(t, session, "hookdeck_transformations", map[string]any{"action": "get", "id": "trn_789"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "enrich-payload")
}

func TestTransformationsGet_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "hookdeck_transformations", map[string]any{"action": "get"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id is required")
}

func TestTransformationsTool_UnknownAction(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "hookdeck_transformations", map[string]any{"action": "run"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "unknown action")
}

// ---------------------------------------------------------------------------
// Attempts tool
// ---------------------------------------------------------------------------

func TestAttemptsList_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/attempts": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "atm_001", "status": "SUCCESSFUL", "response_status": 200}))
		},
	})

	result := callTool(t, session, "hookdeck_attempts", map[string]any{"action": "list"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "atm_001")
}

func TestAttemptsGet_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/attempts/atm_001": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"id": "atm_001", "response_status": 200})
		},
	})

	result := callTool(t, session, "hookdeck_attempts", map[string]any{"action": "get", "id": "atm_001"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "atm_001")
}

func TestAttemptsGet_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "hookdeck_attempts", map[string]any{"action": "get"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id is required")
}

func TestAttemptsTool_UnknownAction(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "hookdeck_attempts", map[string]any{"action": "retry"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "unknown action")
}

// ---------------------------------------------------------------------------
// Events tool
// ---------------------------------------------------------------------------

func TestEventsList_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/events": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "evt_abc", "status": "SUCCESSFUL"}))
		},
	})

	result := callTool(t, session, "hookdeck_events", map[string]any{"action": "list"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "evt_abc")
}

func TestEventsGet_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/events/evt_abc": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"id": "evt_abc", "status": "SUCCESSFUL"})
		},
	})

	result := callTool(t, session, "hookdeck_events", map[string]any{"action": "get", "id": "evt_abc"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "evt_abc")
}

func TestEventsGet_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "hookdeck_events", map[string]any{"action": "get"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id is required")
}

func TestEventsRawBody_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/events/evt_abc/raw_body": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"key":"value"}`))
		},
	})

	result := callTool(t, session, "hookdeck_events", map[string]any{"action": "raw_body", "id": "evt_abc"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "raw_body")
}

func TestEventsRawBody_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "hookdeck_events", map[string]any{"action": "raw_body"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id is required")
}

func TestEventsRawBody_Truncation(t *testing.T) {
	// Generate a body larger than 100KB
	largeBody := strings.Repeat("x", 150*1024)
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/events/evt_big/raw_body": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(largeBody))
		},
	})

	result := callTool(t, session, "hookdeck_events", map[string]any{"action": "raw_body", "id": "evt_big"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "truncated")
}

func TestEventsTool_UnknownAction(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "hookdeck_events", map[string]any{"action": "delete"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "unknown action")
}

func TestEventsList_ConnectionIDMapsToWebhookID(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/events": func(w http.ResponseWriter, r *http.Request) {
			// Verify connection_id is mapped to webhook_id
			assert.Equal(t, "web_123", r.URL.Query().Get("webhook_id"))
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "evt_1"}))
		},
	})

	result := callTool(t, session, "hookdeck_events", map[string]any{"action": "list", "connection_id": "web_123"})
	assert.False(t, result.IsError)
}

// ---------------------------------------------------------------------------
// Requests tool
// ---------------------------------------------------------------------------

func TestRequestsList_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/requests": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "req_001", "source_id": "src_123"}))
		},
	})

	result := callTool(t, session, "hookdeck_requests", map[string]any{"action": "list"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "req_001")
}

func TestRequestsGet_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/requests/req_001": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"id": "req_001"})
		},
	})

	result := callTool(t, session, "hookdeck_requests", map[string]any{"action": "get", "id": "req_001"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "req_001")
}

func TestRequestsGet_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "hookdeck_requests", map[string]any{"action": "get"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id is required")
}

func TestRequestsRawBody_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/requests/req_001/raw_body": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"payload":"data"}`))
		},
	})

	result := callTool(t, session, "hookdeck_requests", map[string]any{"action": "raw_body", "id": "req_001"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "raw_body")
}

func TestRequestsRawBody_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "hookdeck_requests", map[string]any{"action": "raw_body"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id is required")
}

func TestRequestsRawBody_Truncation(t *testing.T) {
	largeBody := strings.Repeat("y", 150*1024)
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/requests/req_big/raw_body": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(largeBody))
		},
	})

	result := callTool(t, session, "hookdeck_requests", map[string]any{"action": "raw_body", "id": "req_big"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "truncated")
}

func TestRequestsEvents_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/requests/req_001/events": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "evt_from_req"}))
		},
	})

	result := callTool(t, session, "hookdeck_requests", map[string]any{"action": "events", "id": "req_001"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "evt_from_req")
}

func TestRequestsEvents_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "hookdeck_requests", map[string]any{"action": "events"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id is required")
}

func TestRequestsIgnoredEvents_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/requests/req_001/ignored_events": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "ign_evt_001"}))
		},
	})

	result := callTool(t, session, "hookdeck_requests", map[string]any{"action": "ignored_events", "id": "req_001"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "ign_evt_001")
}

func TestRequestsIgnoredEvents_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "hookdeck_requests", map[string]any{"action": "ignored_events"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id is required")
}

func TestRequestsTool_UnknownAction(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "hookdeck_requests", map[string]any{"action": "delete"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "unknown action")
}

func TestRequestsList_VerifiedFilter(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/requests": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "true", r.URL.Query().Get("verified"))
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "req_v"}))
		},
	})

	result := callTool(t, session, "hookdeck_requests", map[string]any{"action": "list", "verified": true})
	assert.False(t, result.IsError)
}

// ---------------------------------------------------------------------------
// Issues tool
// ---------------------------------------------------------------------------

func TestIssuesList_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/issues": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(listResponse(map[string]any{"id": "iss_001", "type": "delivery", "status": "OPENED"}))
		},
	})

	result := callTool(t, session, "hookdeck_issues", map[string]any{"action": "list"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "iss_001")
}

func TestIssuesGet_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/issues/iss_001": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"id": "iss_001", "type": "delivery"})
		},
	})

	result := callTool(t, session, "hookdeck_issues", map[string]any{"action": "get", "id": "iss_001"})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "iss_001")
}

func TestIssuesGet_MissingID(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "hookdeck_issues", map[string]any{"action": "get"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "id is required")
}

func TestIssuesTool_UnknownAction(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "hookdeck_issues", map[string]any{"action": "close"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "unknown action")
}

// ---------------------------------------------------------------------------
// Projects tool
// ---------------------------------------------------------------------------

func TestProjectsList_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/teams": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": "proj_test123", "name": "Production", "mode": "console"},
				{"id": "proj_other", "name": "Staging", "mode": "console"},
			})
		},
	})

	result := callTool(t, session, "hookdeck_projects", map[string]any{"action": "list"})
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
		"/2025-07-01/teams": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]any{"message": "not allowed"})
		},
	})

	result := callTool(t, session, "hookdeck_projects", map[string]any{"action": "list"})
	assert.True(t, result.IsError)
	text := textContent(t, result)
	assert.Contains(t, strings.ToLower(text), "reauth")
	assert.Contains(t, text, "hookdeck_login")
}

func TestProjectsUse_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/teams": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": "proj_test123", "name": "Production", "mode": "console"},
				{"id": "proj_new", "name": "Staging", "mode": "console"},
			})
		},
	})

	result := callTool(t, session, "hookdeck_projects", map[string]any{"action": "use", "project_id": "proj_new"})
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
	result := callTool(t, session, "hookdeck_projects", map[string]any{"action": "use"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "project_id is required")
}

func TestProjectsUse_ProjectNotFound(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/teams": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": "proj_test123", "name": "Production", "mode": "console"},
			})
		},
	})

	result := callTool(t, session, "hookdeck_projects", map[string]any{"action": "use", "project_id": "proj_nonexistent"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "not found")
}

func TestProjectsTool_UnknownAction(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "hookdeck_projects", map[string]any{"action": "create"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "unknown action")
}

// ---------------------------------------------------------------------------
// Metrics tool
// ---------------------------------------------------------------------------

func TestMetricsTool_MissingStartEnd(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "hookdeck_metrics", map[string]any{"action": "events"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "required")
}

func TestMetricsTool_MissingMeasures(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)
	result := callTool(t, session, "hookdeck_metrics", map[string]any{
		"action": "events",
		"start":  "2025-01-01T00:00:00Z",
		"end":    "2025-01-02T00:00:00Z",
	})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "measures")
}

func TestMetricsEvents_DefaultRoute(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/metrics/events": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "granularity": "1h"})
		},
	})

	result := callTool(t, session, "hookdeck_metrics", map[string]any{
		"action":   "events",
		"start":    "2025-01-01T00:00:00Z",
		"end":      "2025-01-02T00:00:00Z",
		"measures": []any{"count"},
	})
	assert.False(t, result.IsError)
}

func TestMetricsEvents_QueueDepthRoute(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/metrics/queue-depth": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		},
	})

	result := callTool(t, session, "hookdeck_metrics", map[string]any{
		"action":   "events",
		"start":    "2025-01-01T00:00:00Z",
		"end":      "2025-01-02T00:00:00Z",
		"measures": []any{"queue_depth"},
	})
	assert.False(t, result.IsError)
}

func TestMetricsEvents_PendingTimeseriesRoute(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/metrics/events-pending-timeseries": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		},
	})

	result := callTool(t, session, "hookdeck_metrics", map[string]any{
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
		"/2025-07-01/metrics/events-by-issue": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		},
	})

	result := callTool(t, session, "hookdeck_metrics", map[string]any{
		"action":     "events",
		"start":      "2025-01-01T00:00:00Z",
		"end":        "2025-01-02T00:00:00Z",
		"measures":   []any{"count"},
		"dimensions": []any{"issue_id"},
	})
	assert.False(t, result.IsError)
}

func TestMetricsRequests_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/metrics/requests": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		},
	})

	result := callTool(t, session, "hookdeck_metrics", map[string]any{
		"action":   "requests",
		"start":    "2025-01-01T00:00:00Z",
		"end":      "2025-01-02T00:00:00Z",
		"measures": []any{"count"},
	})
	assert.False(t, result.IsError)
}

func TestMetricsAttempts_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/metrics/attempts": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		},
	})

	result := callTool(t, session, "hookdeck_metrics", map[string]any{
		"action":   "attempts",
		"start":    "2025-01-01T00:00:00Z",
		"end":      "2025-01-02T00:00:00Z",
		"measures": []any{"count"},
	})
	assert.False(t, result.IsError)
}

func TestMetricsTransformations_Success(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/metrics/transformations": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		},
	})

	result := callTool(t, session, "hookdeck_metrics", map[string]any{
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
	result := callTool(t, session, "hookdeck_metrics", map[string]any{
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
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	result := callTool(t, session, "hookdeck_login", map[string]any{})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "Already authenticated")
}

func TestLoginTool_ReauthStartsFreshLogin(t *testing.T) {
	api := mockAPI(t, map[string]http.HandlerFunc{
		"/2025-07-01/cli-auth": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{
				"browser_url": "https://hookdeck.com/auth?code=reauth",
				"poll_url":    "http://" + r.Host + "/2025-07-01/cli-auth/poll?key=reauth",
			})
		},
		"/2025-07-01/cli-auth/poll": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"claimed": false})
		},
	})

	client := newTestClient(api.URL, "sk_test_123456789012")
	cfg := &config.Config{APIBaseURL: api.URL}
	srv := NewServer(client, cfg)

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
		"/2025-07-01/cli-auth": func(w http.ResponseWriter, r *http.Request) {
			authCalled = true
			json.NewEncoder(w).Encode(map[string]any{
				"browser_url": "https://hookdeck.com/auth?code=abc123",
				"poll_url":    "http://" + r.Host + "/2025-07-01/cli-auth/poll?key=abc123",
			})
		},
		"/2025-07-01/cli-auth/poll": func(w http.ResponseWriter, r *http.Request) {
			// Never claimed — user hasn't opened the browser yet.
			json.NewEncoder(w).Encode(map[string]any{"claimed": false})
		},
	})

	unauthClient := newTestClient(api.URL, "")
	cfg := &config.Config{APIBaseURL: api.URL}
	srv := NewServer(unauthClient, cfg)

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
		"/2025-07-01/cli-auth": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{
				"browser_url": "https://hookdeck.com/auth?code=xyz",
				"poll_url":    "http://" + r.Host + "/2025-07-01/cli-auth/poll?key=xyz",
			})
		},
		"/2025-07-01/cli-auth/poll": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"claimed": false})
		},
	})

	unauthClient := newTestClient(api.URL, "")
	cfg := &config.Config{APIBaseURL: api.URL}
	srv := NewServer(unauthClient, cfg)

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
		"/2025-07-01/cli-auth": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{
				"browser_url": "https://hookdeck.com/auth?code=survive",
				"poll_url":    "http://" + r.Host + "/2025-07-01/cli-auth/poll?key=survive",
			})
		},
		"/2025-07-01/cli-auth/poll": func(w http.ResponseWriter, r *http.Request) {
			pollCount++
			if pollCount >= 2 {
				// Simulate user completing browser auth on 2nd poll.
				json.NewEncoder(w).Encode(map[string]any{
					"claimed":           true,
					"key":               "sk_test_survive12345",
					"team_id":           "proj_survive",
					"team_name":         "Survive Project",
					"team_mode":         "console",
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
	srv := NewServer(unauthClient, cfg)

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

	// Wait briefly for the polling goroutine to complete (poll interval is 2s
	// in production, but the mock returns instantly so it completes quickly).
	time.Sleep(500 * time.Millisecond)

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
		"/2025-07-01/sources": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{"message": "workspace not found"})
		},
	})

	result := callTool(t, session, "hookdeck_sources", map[string]any{"action": "list"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "not found")
}

func TestSourcesList_422ValidationError(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/sources": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(map[string]any{"message": "invalid parameter: limit must be positive"})
		},
	})

	result := callTool(t, session, "hookdeck_sources", map[string]any{"action": "list"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "invalid parameter")
}

func TestSourcesList_429RateLimitError(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/sources": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]any{"message": "rate limited"})
		},
	})

	result := callTool(t, session, "hookdeck_sources", map[string]any{"action": "list"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "Rate limited")
}

func TestEventsGet_APIError(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/events/evt_nope": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{"message": "event not found"})
		},
	})

	result := callTool(t, session, "hookdeck_events", map[string]any{"action": "get", "id": "evt_nope"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "not found")
}

// ---------------------------------------------------------------------------
// Input parsing edge cases
// ---------------------------------------------------------------------------

func TestInput_Accessors(t *testing.T) {
	raw := json.RawMessage(`{
		"name": "test",
		"count": 42,
		"active": true,
		"tags": ["a", "b"],
		"missing_bool": null
	}`)

	in, err := parseInput(raw)
	require.NoError(t, err)

	assert.Equal(t, "test", in.String("name"))
	assert.Equal(t, "", in.String("nonexistent"))
	assert.Equal(t, 42, in.Int("count", 0))
	assert.Equal(t, 99, in.Int("nonexistent", 99))
	assert.Equal(t, true, in.Bool("active"))
	assert.Equal(t, false, in.Bool("nonexistent"))
	assert.Equal(t, []string{"a", "b"}, in.StringSlice("tags"))
	assert.Nil(t, in.StringSlice("nonexistent"))

	bp := in.BoolPtr("active")
	require.NotNil(t, bp)
	assert.True(t, *bp)
	assert.Nil(t, in.BoolPtr("nonexistent"))
}

func TestInput_EmptyArgs(t *testing.T) {
	in, err := parseInput(nil)
	require.NoError(t, err)
	assert.Equal(t, "", in.String("anything"))
}

func TestInput_InvalidJSON(t *testing.T) {
	_, err := parseInput(json.RawMessage(`{invalid`))
	assert.Error(t, err)
}

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
		{"hookdeck_projects", "list"},
		{"hookdeck_connections", "pause"},
		{"hookdeck_sources", "list"},
		{"hookdeck_destinations", "HTTP"},
		{"hookdeck_transformations", "JavaScript"},
		{"hookdeck_requests", "raw_body"},
		{"hookdeck_events", "raw_body"},
		{"hookdeck_attempts", "event_id"},
		{"hookdeck_issues", "delivery"},
		{"hookdeck_metrics", "granularity"},
		{"hookdeck_help", "topic"},
	}

	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	for _, tt := range topics {
		t.Run(tt.name, func(t *testing.T) {
			result := callTool(t, session, "hookdeck_help", map[string]any{"topic": tt.name})
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
		"transformations", "requests", "events", "attempts",
		"issues", "metrics", "help",
	}

	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	for _, name := range shortNames {
		t.Run(name, func(t *testing.T) {
			result := callTool(t, session, "hookdeck_help", map[string]any{"topic": name})
			assert.False(t, result.IsError, "short name %q should resolve", name)
			assert.Contains(t, textContent(t, result), "hookdeck_"+name)
		})
	}
}

func TestHelpTool_OverviewListsAllTools(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	result := callTool(t, session, "hookdeck_help", map[string]any{})
	assert.False(t, result.IsError)
	text := textContent(t, result)

	expectedTools := []string{
		"hookdeck_projects", "hookdeck_connections", "hookdeck_sources",
		"hookdeck_destinations", "hookdeck_transformations", "hookdeck_requests",
		"hookdeck_events", "hookdeck_attempts", "hookdeck_issues",
		"hookdeck_metrics", "hookdeck_help",
	}
	for _, tool := range expectedTools {
		assert.Contains(t, text, tool, "overview should list %s", tool)
	}
}

func TestHelpTool_OverviewShowsProjectNotSet(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	client.ProjectID = "" // no project set
	session := connectInMemory(t, client)

	result := callTool(t, session, "hookdeck_help", map[string]any{})
	assert.False(t, result.IsError)
	assert.Contains(t, textContent(t, result), "not set")
}

func TestHelpTool_UnknownTopicListsAvailable(t *testing.T) {
	client := newTestClient("https://api.hookdeck.com", "test-key")
	session := connectInMemory(t, client)

	result := callTool(t, session, "hookdeck_help", map[string]any{"topic": "bogus"})
	assert.True(t, result.IsError)
	text := textContent(t, result)
	assert.Contains(t, text, "No help found")
	assert.Contains(t, text, "hookdeck_events") // lists available tools
}

// ---------------------------------------------------------------------------
// Error feedback: 500 server error through HTTP flow
// ---------------------------------------------------------------------------

func TestDestinationsGet_500ServerError(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/destinations/des_fail": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]any{"message": "internal server error"})
		},
	})

	result := callTool(t, session, "hookdeck_destinations", map[string]any{"action": "get", "id": "des_fail"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "Hookdeck API error")
}

func TestConnectionsGet_401UnauthorizedError(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/connections/web_bad": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{"message": "invalid api key"})
		},
	})

	result := callTool(t, session, "hookdeck_connections", map[string]any{"action": "get", "id": "web_bad"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "Authentication failed")
}

func TestIssuesList_422ValidationError(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/issues": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(map[string]any{"message": "invalid filter: bad_field"})
		},
	})

	result := callTool(t, session, "hookdeck_issues", map[string]any{"action": "list"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "invalid filter")
}

func TestAttemptsList_429RateLimitError(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"/2025-07-01/attempts": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]any{"message": "too many requests"})
		},
	})

	result := callTool(t, session, "hookdeck_attempts", map[string]any{"action": "list"})
	assert.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "Rate limited")
}

// ---------------------------------------------------------------------------
// Error translation: additional cases
// ---------------------------------------------------------------------------

func TestTranslateAPIError_RetryAfterMessage(t *testing.T) {
	msg := TranslateAPIError(&hookdeck.APIError{StatusCode: 429, Message: "rate limited"})
	assert.Contains(t, msg, "Rate limited")
	assert.Contains(t, msg, "Retry after")
}

func TestTranslateAPIError_GenericClientError(t *testing.T) {
	// A 4xx status not explicitly handled should pass through the message
	msg := TranslateAPIError(&hookdeck.APIError{StatusCode: 409, Message: "conflict on resource"})
	assert.Contains(t, msg, "conflict on resource")
}

func TestTranslateAPIError_502GatewayError(t *testing.T) {
	msg := TranslateAPIError(&hookdeck.APIError{StatusCode: 502, Message: "bad gateway"})
	assert.Contains(t, msg, "Hookdeck API error")
}

func TestTranslateAPIError_503ServiceUnavailable(t *testing.T) {
	msg := TranslateAPIError(&hookdeck.APIError{StatusCode: 503, Message: "service unavailable"})
	assert.Contains(t, msg, "Hookdeck API error")
}
