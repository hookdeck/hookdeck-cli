package mcp

import (
	"context"
	"encoding/json"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// newCallToolRequest creates a CallToolRequest with the given arguments JSON.
func newCallToolRequest(argsJSON string) *mcpsdk.CallToolRequest {
	return &mcpsdk.CallToolRequest{
		Params: &mcpsdk.CallToolParamsRaw{
			Arguments: json.RawMessage(argsJSON),
		},
	}
}

func TestExtractAction(t *testing.T) {
	tests := []struct {
		name     string
		req      *mcpsdk.CallToolRequest
		expected string
	}{
		{"valid action", newCallToolRequest(`{"action":"list"}`), "list"},
		{"no action field", newCallToolRequest(`{"id":"123"}`), ""},
		{"empty object", newCallToolRequest(`{}`), ""},
		{"action with other fields", newCallToolRequest(`{"action":"get","id":"evt_123"}`), "get"},
		{"nil arguments", &mcpsdk.CallToolRequest{Params: &mcpsdk.CallToolParamsRaw{}}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAction(tt.req)
			require.Equal(t, tt.expected, got)
		})
	}
}

func TestMCPClientInfoNilSession(t *testing.T) {
	req := newCallToolRequest(`{}`)
	req.Session = nil
	got := mcpClientInfo(req)
	require.Equal(t, "", got)
}

func TestWrapWithTelemetrySetsAndClears(t *testing.T) {
	client := &hookdeck.Client{}
	s := &Server{client: client}

	var capturedTelemetry *hookdeck.CLITelemetry

	innerHandler := mcpsdk.ToolHandler(func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		require.NotNil(t, s.client.Telemetry)
		require.Equal(t, "mcp", s.client.Telemetry.Source)
		require.Equal(t, "hookdeck_events/list", s.client.Telemetry.CommandPath)
		require.NotEmpty(t, s.client.Telemetry.InvocationID)
		require.NotEmpty(t, s.client.Telemetry.DeviceName)
		// Capture a copy
		cp := *s.client.Telemetry
		capturedTelemetry = &cp
		return &mcpsdk.CallToolResult{}, nil
	})

	wrapped := s.wrapWithTelemetry("hookdeck_events", innerHandler)

	req := newCallToolRequest(`{"action":"list"}`)
	result, err := wrapped(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Telemetry should have been captured inside the handler
	require.NotNil(t, capturedTelemetry)
	require.Equal(t, "mcp", capturedTelemetry.Source)
	require.Equal(t, "hookdeck_events/list", capturedTelemetry.CommandPath)

	// After the wrapper returns, telemetry should be cleared on the shared client
	require.Nil(t, s.client.Telemetry)
}

func TestWrapWithTelemetryNoAction(t *testing.T) {
	client := &hookdeck.Client{}
	s := &Server{client: client}

	var capturedPath string

	innerHandler := mcpsdk.ToolHandler(func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		capturedPath = s.client.Telemetry.CommandPath
		return &mcpsdk.CallToolResult{}, nil
	})

	wrapped := s.wrapWithTelemetry("hookdeck_help", innerHandler)

	req := newCallToolRequest(`{"topic":"hookdeck_events"}`)
	_, err := wrapped(context.Background(), req)
	require.NoError(t, err)

	// No "action" field, so command path should just be the tool name
	require.Equal(t, "hookdeck_help", capturedPath)
}

func TestWrapWithTelemetryUniqueInvocationIDs(t *testing.T) {
	client := &hookdeck.Client{}
	s := &Server{client: client}

	var ids []string

	innerHandler := mcpsdk.ToolHandler(func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		ids = append(ids, s.client.Telemetry.InvocationID)
		return &mcpsdk.CallToolResult{}, nil
	})

	wrapped := s.wrapWithTelemetry("hookdeck_events", innerHandler)

	for i := 0; i < 5; i++ {
		req := newCallToolRequest(`{"action":"list"}`)
		_, _ = wrapped(context.Background(), req)
	}

	require.Len(t, ids, 5)
	// All IDs should be unique
	seen := make(map[string]bool)
	for _, id := range ids {
		require.False(t, seen[id], "duplicate invocation ID: %s", id)
		seen[id] = true
	}
}
