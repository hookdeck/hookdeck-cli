package hookdeck

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestGetTelemetryInstance(t *testing.T) {
	t1 := GetTelemetryInstance()
	t2 := GetTelemetryInstance()
	require.Equal(t, t1, t2)
}

func TestSetCommandContext(t *testing.T) {
	tel := GetTelemetryInstance()
	cmd := &cobra.Command{
		Use: "foo",
	}
	tel.SetCommandContext(cmd)
	require.Equal(t, "foo", tel.CommandPath)
}

func TestTelemetryOptedOut(t *testing.T) {
	// Env var only (config disabled = false)
	require.False(t, telemetryOptedOut("", false))
	require.False(t, telemetryOptedOut("0", false))
	require.False(t, telemetryOptedOut("false", false))
	require.False(t, telemetryOptedOut("False", false))
	require.False(t, telemetryOptedOut("FALSE", false))
	require.True(t, telemetryOptedOut("1", false))
	require.True(t, telemetryOptedOut("true", false))
	require.True(t, telemetryOptedOut("True", false))
	require.True(t, telemetryOptedOut("TRUE", false))

	// Config disabled = true overrides env var
	require.True(t, telemetryOptedOut("", true))
	require.True(t, telemetryOptedOut("0", true))
	require.True(t, telemetryOptedOut("false", true))
}

func TestNewInvocationID(t *testing.T) {
	id := NewInvocationID()
	require.True(t, strings.HasPrefix(id, "inv_"), "invocation ID should have inv_ prefix")
	// "inv_" (4 chars) + 16 hex chars = 20 chars
	require.Len(t, id, 20, "invocation ID should be 20 characters")

	// IDs should be unique
	id2 := NewInvocationID()
	require.NotEqual(t, id, id2, "two invocation IDs should not be equal")
}

func TestDetectEnvironment(t *testing.T) {
	// Clear all CI vars first
	ciVars := []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "BUILDKITE", "TF_BUILD", "JENKINS_URL", "CODEBUILD_BUILD_ID"}
	for _, v := range ciVars {
		t.Setenv(v, "")
	}

	require.Equal(t, "interactive", DetectEnvironment())

	t.Setenv("CI", "true")
	require.Equal(t, "ci", DetectEnvironment())

	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "true")
	require.Equal(t, "ci", DetectEnvironment())

	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITLAB_CI", "true")
	require.Equal(t, "ci", DetectEnvironment())

	t.Setenv("GITLAB_CI", "")
	t.Setenv("JENKINS_URL", "http://jenkins.example.com")
	require.Equal(t, "ci", DetectEnvironment())

	t.Setenv("JENKINS_URL", "")
	t.Setenv("CODEBUILD_BUILD_ID", "build-123")
	require.Equal(t, "ci", DetectEnvironment())
}

func TestTelemetrySetters(t *testing.T) {
	tel := &CLITelemetry{}

	tel.SetSource("cli")
	require.Equal(t, "cli", tel.Source)

	tel.SetEnvironment("ci")
	require.Equal(t, "ci", tel.Environment)

	tel.SetInvocationID("inv_test123")
	require.Equal(t, "inv_test123", tel.InvocationID)

	tel.SetDeviceName("my-machine")
	require.Equal(t, "my-machine", tel.DeviceName)
}

func TestTelemetryJSONSerialization(t *testing.T) {
	// CLI telemetry
	tel := &CLITelemetry{
		Source:            "cli",
		Environment:       "interactive",
		CommandPath:       "hookdeck listen",
		InvocationID:      "inv_abcdef0123456789",
		DeviceName:        "macbook-pro",
		GeneratedResource: false,
	}

	b, err := json.Marshal(tel)
	require.NoError(t, err)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &parsed))
	require.Equal(t, "cli", parsed["source"])
	require.Equal(t, "interactive", parsed["environment"])
	require.Equal(t, "hookdeck listen", parsed["command_path"])
	require.Equal(t, "inv_abcdef0123456789", parsed["invocation_id"])
	require.Equal(t, "macbook-pro", parsed["device_name"])
	// generated_resource is omitempty and false, should not be present
	_, hasGenerated := parsed["generated_resource"]
	require.False(t, hasGenerated, "generated_resource=false should be omitted")
	// mcp_client is omitempty and empty, should not be present
	_, hasMCPClient := parsed["mcp_client"]
	require.False(t, hasMCPClient, "empty mcp_client should be omitted")

	// MCP telemetry
	mcpTel := &CLITelemetry{
		Source:       "mcp",
		Environment:  "interactive",
		CommandPath:  "hookdeck_events/list",
		InvocationID: "inv_1234567890abcdef",
		DeviceName:   "macbook-pro",
		MCPClient:    "claude-desktop/1.2.0",
	}

	b, err = json.Marshal(mcpTel)
	require.NoError(t, err)

	var parsedMCP map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &parsedMCP))
	require.Equal(t, "mcp", parsedMCP["source"])
	require.Equal(t, "hookdeck_events/list", parsedMCP["command_path"])
	require.Equal(t, "claude-desktop/1.2.0", parsedMCP["mcp_client"])
}

func TestTelemetryJSONWithGeneratedResource(t *testing.T) {
	tel := &CLITelemetry{
		Source:            "cli",
		Environment:       "interactive",
		CommandPath:       "hookdeck gateway source list",
		InvocationID:      "inv_abcdef0123456789",
		DeviceName:        "test",
		GeneratedResource: true,
	}

	b, err := json.Marshal(tel)
	require.NoError(t, err)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &parsed))
	require.Equal(t, true, parsed["generated_resource"])
}
