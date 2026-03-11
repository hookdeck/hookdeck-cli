package hookdeck

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithTelemetry(t *testing.T) {
	baseURL, _ := url.Parse("http://localhost")
	original := &Client{
		BaseURL:                 baseURL,
		APIKey:                  "test-key",
		ProjectID:               "proj-123",
		Verbose:                 true,
		SuppressRateLimitErrors: true,
		TelemetryDisabled:       false,
	}

	tel := &CLITelemetry{
		Source:       "mcp",
		Environment:  "interactive",
		CommandPath:  "hookdeck_events/list",
		InvocationID: "inv_test123",
		DeviceName:   "test-machine",
		MCPClient:    "test-client/1.0",
	}

	cloned := original.WithTelemetry(tel)

	// Cloned client should have the telemetry override
	require.Equal(t, tel, cloned.Telemetry)

	// Original client should NOT have telemetry set
	require.Nil(t, original.Telemetry)

	// Other fields should be copied
	require.Equal(t, original.BaseURL, cloned.BaseURL)
	require.Equal(t, original.APIKey, cloned.APIKey)
	require.Equal(t, original.ProjectID, cloned.ProjectID)
	require.Equal(t, original.Verbose, cloned.Verbose)
	require.Equal(t, original.SuppressRateLimitErrors, cloned.SuppressRateLimitErrors)
	require.Equal(t, original.TelemetryDisabled, cloned.TelemetryDisabled)
}

func TestPerformRequestUsesTelemetryOverride(t *testing.T) {
	var receivedHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get(TelemetryHeaderName)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	baseURL, _ := url.Parse(server.URL)
	client := &Client{
		BaseURL: baseURL,
		APIKey:  "test",
	}

	tel := &CLITelemetry{
		Source:       "mcp",
		Environment:  "ci",
		CommandPath:  "hookdeck_events/list",
		InvocationID: "inv_abcdef0123456789",
		DeviceName:   "test-device",
		MCPClient:    "claude-desktop/1.0",
	}
	client.Telemetry = tel

	req, err := http.NewRequest(http.MethodGet, server.URL+"/test", nil)
	require.NoError(t, err)

	// Clear opt-out env var
	t.Setenv("HOOKDECK_CLI_TELEMETRY_OPTOUT", "")

	_, err = client.PerformRequest(context.Background(), req)
	require.NoError(t, err)

	require.NotEmpty(t, receivedHeader)

	var parsed CLITelemetry
	require.NoError(t, json.Unmarshal([]byte(receivedHeader), &parsed))
	require.Equal(t, "mcp", parsed.Source)
	require.Equal(t, "ci", parsed.Environment)
	require.Equal(t, "hookdeck_events/list", parsed.CommandPath)
	require.Equal(t, "inv_abcdef0123456789", parsed.InvocationID)
	require.Equal(t, "claude-desktop/1.0", parsed.MCPClient)
}

func TestPerformRequestTelemetryDisabledByConfig(t *testing.T) {
	var receivedHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get(TelemetryHeaderName)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	baseURL, _ := url.Parse(server.URL)
	client := &Client{
		BaseURL:           baseURL,
		APIKey:            "test",
		TelemetryDisabled: true,
	}

	t.Setenv("HOOKDECK_CLI_TELEMETRY_OPTOUT", "")

	req, err := http.NewRequest(http.MethodGet, server.URL+"/test", nil)
	require.NoError(t, err)

	_, err = client.PerformRequest(context.Background(), req)
	require.NoError(t, err)

	require.Empty(t, receivedHeader, "telemetry header should be empty when config disabled")
}

func TestPerformRequestTelemetryDisabledByEnvVar(t *testing.T) {
	var receivedHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get(TelemetryHeaderName)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	baseURL, _ := url.Parse(server.URL)
	client := &Client{
		BaseURL: baseURL,
		APIKey:  "test",
	}

	t.Setenv("HOOKDECK_CLI_TELEMETRY_OPTOUT", "true")

	req, err := http.NewRequest(http.MethodGet, server.URL+"/test", nil)
	require.NoError(t, err)

	_, err = client.PerformRequest(context.Background(), req)
	require.NoError(t, err)

	require.Empty(t, receivedHeader, "telemetry header should be empty when env var opted out")
}

func TestPerformRequestFallsBackToSingleton(t *testing.T) {
	var receivedHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get(TelemetryHeaderName)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	baseURL, _ := url.Parse(server.URL)
	client := &Client{
		BaseURL: baseURL,
		APIKey:  "test",
		// Telemetry is nil, so it should fall back to the singleton
	}

	// Populate the singleton
	tel := GetTelemetryInstance()
	tel.SetSource("cli")
	tel.SetEnvironment("interactive")
	tel.CommandPath = "hookdeck listen"
	tel.SetDeviceName("test-host")
	tel.SetInvocationID("inv_singleton_test")

	t.Setenv("HOOKDECK_CLI_TELEMETRY_OPTOUT", "")

	req, err := http.NewRequest(http.MethodGet, server.URL+"/test", nil)
	require.NoError(t, err)

	_, err = client.PerformRequest(context.Background(), req)
	require.NoError(t, err)

	require.NotEmpty(t, receivedHeader)

	var parsed CLITelemetry
	require.NoError(t, json.Unmarshal([]byte(receivedHeader), &parsed))
	require.Equal(t, "cli", parsed.Source)
	require.Equal(t, "test-host", parsed.DeviceName)
	require.Equal(t, "inv_singleton_test", parsed.InvocationID)
}

func TestPerformRequestTelemetryDisabledBySingleton(t *testing.T) {
	ResetTelemetryInstanceForTesting()
	defer ResetTelemetryInstanceForTesting()

	var receivedHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get(TelemetryHeaderName)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	baseURL, _ := url.Parse(server.URL)
	// Client does NOT set TelemetryDisabled — simulates a stray client construction
	client := &Client{
		BaseURL: baseURL,
		APIKey:  "test",
	}

	// Disable telemetry via the singleton (as initTelemetry would)
	tel := GetTelemetryInstance()
	tel.SetDisabled(true)

	t.Setenv("HOOKDECK_CLI_TELEMETRY_OPTOUT", "")

	req, err := http.NewRequest(http.MethodGet, server.URL+"/test", nil)
	require.NoError(t, err)

	_, err = client.PerformRequest(context.Background(), req)
	require.NoError(t, err)

	require.Empty(t, receivedHeader, "telemetry header should be empty when singleton has Disabled=true")
}
