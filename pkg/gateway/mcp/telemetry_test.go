package mcp

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// ---------------------------------------------------------------------------
// End-to-end integration tests: MCP tool call → HTTP request → telemetry header
// These tests use the full MCP server pipeline (mockAPIWithClient) and verify
// that the telemetry header arrives at the mock API server with
// the correct content.
// ---------------------------------------------------------------------------

// headerCapture is a thread-safe collector for HTTP headers received by the
// mock API. Each incoming request appends its telemetry header.
type headerCapture struct {
	mu      sync.Mutex
	headers []string
}

func (hc *headerCapture) handler(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hc.mu.Lock()
		hc.headers = append(hc.headers, r.Header.Get(hookdeck.TelemetryHeaderName))
		hc.mu.Unlock()
		next(w, r)
	}
}

func (hc *headerCapture) last() string {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	if len(hc.headers) == 0 {
		return ""
	}
	return hc.headers[len(hc.headers)-1]
}

func (hc *headerCapture) all() []string {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	cp := make([]string, len(hc.headers))
	copy(cp, hc.headers)
	return cp
}

// parseTelemetryHeader unmarshals a telemetry header value.
func parseTelemetryHeader(t *testing.T, raw string) hookdeck.CLITelemetry {
	t.Helper()
	var tel hookdeck.CLITelemetry
	require.NoError(t, json.Unmarshal([]byte(raw), &tel))
	return tel
}

func TestMCPToolCall_TelemetryHeaderSentToAPI(t *testing.T) {
	// Ensure env-var opt-out is disabled so telemetry flows.
	t.Setenv("HOOKDECK_CLI_TELEMETRY_DISABLED", "")

	capture := &headerCapture{}

	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/sources": capture.handler(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(listResponse(
				map[string]any{"id": "src_1", "name": "webhook", "url": "https://example.com"},
			))
		}),
	})

	result := callTool(t, session, "gateway_sources", map[string]any{"action": "list"})
	require.False(t, result.IsError, "tool call should succeed")

	// Verify the telemetry header was sent.
	raw := capture.last()
	require.NotEmpty(t, raw, "telemetry header must be sent")

	tel := parseTelemetryHeader(t, raw)
	require.Equal(t, "mcp", tel.Source)
	require.Equal(t, "gateway_sources/list", tel.CommandPath)
	require.True(t, strings.HasPrefix(tel.InvocationID, "inv_"), "invocation ID must start with inv_")
	require.NotEmpty(t, tel.DeviceName)
	require.Contains(t, []string{"interactive", "ci"}, tel.Environment)
	// The in-memory MCP transport populates ClientInfo
	require.Equal(t, "test-client/0.0.1", tel.MCPClient)
}

func TestMCPToolCall_EachCallGetsUniqueInvocationID(t *testing.T) {
	t.Setenv("HOOKDECK_CLI_TELEMETRY_DISABLED", "")

	capture := &headerCapture{}

	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/sources": capture.handler(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(listResponse(
				map[string]any{"id": "src_1", "name": "webhook", "url": "https://example.com"},
			))
		}),
	})

	// Make three separate tool calls.
	for i := 0; i < 3; i++ {
		result := callTool(t, session, "gateway_sources", map[string]any{"action": "list"})
		require.False(t, result.IsError)
	}

	headers := capture.all()
	require.Len(t, headers, 3, "expected 3 API requests")

	ids := make(map[string]bool)
	for _, raw := range headers {
		tel := parseTelemetryHeader(t, raw)
		require.False(t, ids[tel.InvocationID], "duplicate invocation ID: %s", tel.InvocationID)
		ids[tel.InvocationID] = true
	}
}

func TestMCPToolCall_TelemetryHeaderReflectsAction(t *testing.T) {
	t.Setenv("HOOKDECK_CLI_TELEMETRY_DISABLED", "")

	capture := &headerCapture{}

	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/sources": capture.handler(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(listResponse(
				map[string]any{"id": "src_1", "name": "test-source", "url": "https://example.com"},
			))
		}),
		"GET /2025-07-01/sources/src_1": capture.handler(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"id": "src_1", "name": "test-source", "url": "https://example.com"})
		}),
	})

	// Call "list" action.
	result := callTool(t, session, "gateway_sources", map[string]any{"action": "list"})
	require.False(t, result.IsError)

	listTel := parseTelemetryHeader(t, capture.all()[0])
	require.Equal(t, "gateway_sources/list", listTel.CommandPath)

	// Call "get" action.
	result = callTool(t, session, "gateway_sources", map[string]any{"action": "get", "id": "src_1"})
	require.False(t, result.IsError)

	getTel := parseTelemetryHeader(t, capture.all()[1])
	require.Equal(t, "gateway_sources/get", getTel.CommandPath)
}

func TestMCPToolCall_TelemetryDisabledByConfig(t *testing.T) {
	t.Setenv("HOOKDECK_CLI_TELEMETRY_DISABLED", "")

	capture := &headerCapture{}

	api := mockAPI(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/sources": capture.handler(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(listResponse(
				map[string]any{"id": "src_1", "name": "test-source", "url": "https://example.com"},
			))
		}),
	})

	client := newTestClient(api.URL, "test-key")
	client.TelemetryDisabled = true
	session := connectInMemory(t, client)

	result := callTool(t, session, "gateway_sources", map[string]any{"action": "list"})
	require.False(t, result.IsError)

	raw := capture.last()
	require.Empty(t, raw, "telemetry header should NOT be sent when config opt-out is enabled")
}

func TestMCPToolCall_TelemetryDisabledByEnvVar(t *testing.T) {
	t.Setenv("HOOKDECK_CLI_TELEMETRY_DISABLED", "true")

	capture := &headerCapture{}

	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/sources": capture.handler(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(listResponse(
				map[string]any{"id": "src_1", "name": "test-source", "url": "https://example.com"},
			))
		}),
	})

	result := callTool(t, session, "gateway_sources", map[string]any{"action": "list"})
	require.False(t, result.IsError)

	raw := capture.last()
	require.Empty(t, raw, "telemetry header should NOT be sent when env var opt-out is enabled")
}

func TestMCPToolCall_MultipleAPICallsSameInvocation(t *testing.T) {
	// The "projects use" action makes 2 API calls (list projects, then update).
	// Both should carry the same invocation ID.
	t.Setenv("HOOKDECK_CLI_TELEMETRY_DISABLED", "")

	capture := &headerCapture{}

	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/teams": capture.handler(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": "proj_abc", "name": "My Project", "mode": "console"},
			})
		}),
	})

	result := callTool(t, session, "hookdeck_projects", map[string]any{
		"action":     "use",
		"project_id": "proj_abc",
	})
	require.False(t, result.IsError)

	headers := capture.all()
	require.GreaterOrEqual(t, len(headers), 1, "expected at least 1 API request")

	// All requests from a single tool invocation should share the same invocation ID.
	firstTel := parseTelemetryHeader(t, headers[0])
	for i, raw := range headers[1:] {
		tel := parseTelemetryHeader(t, raw)
		require.Equal(t, firstTel.InvocationID, tel.InvocationID,
			"request %d should have same invocation ID as first request", i+1)
	}
}
