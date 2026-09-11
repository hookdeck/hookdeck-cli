//go:build mcp

package acceptance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertGatewayMCPStdioHygiene(t *testing.T, stdout, stderr string) {
	t.Helper()
	assert.NotContains(t, stdout, "Running `hookdeck login`")
	assert.NotContains(t, stdout, "You aren't")
	assert.NotContains(t, stderr, "Running `hookdeck login`")
}

// --- Help ---

func TestMCPHelp(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess("gateway", "mcp", "--help")
	assert.Contains(t, stdout, "Model Context Protocol")
	assert.Contains(t, stdout, "stdio")
	assert.Contains(t, stdout, "hookdeck gateway mcp")
	assert.Contains(t, stdout, "--allow-write")
	assert.Contains(t, stdout, "read-only")
}

func TestGatewayHelpListsMCP(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess("gateway", "--help")
	assert.Contains(t, stdout, "mcp", "gateway --help should list 'mcp' subcommand")
}

// --- Stdio / auth-aware gateway MCP (subprocess) ---

func TestGatewayMCPStdio_UnauthenticatedInitialize(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err)
	cfgPath := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("profile = \"default\"\n\n[default]\n"), 0644))

	extra := map[string]string{
		"HOOKDECK_CLI_TESTING_API_KEY":   "",
		"HOOKDECK_CLI_TESTING_API_KEY_2": "",
		"HOOKDECK_CLI_TESTING_API_KEY_3": "",
	}
	stdout, stderr, _ := RunGatewayMCPSubprocess(t, projectRoot, cfgPath, extra, mcpInitializeJSON+"\n", 4*time.Second)
	msg := firstJSONRPCMessageLine(t, stdout)
	assert.Equal(t, "2.0", msg["jsonrpc"])
	assertGatewayMCPStdioHygiene(t, stdout, stderr)
}

func TestGatewayMCPStdio_AuthenticatedInitialize(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout, stderr, _ := RunGatewayMCPSubprocess(t, cli.projectRoot, cli.configPath, nil, mcpInitializeJSON+"\n", 4*time.Second)
	msg := firstJSONRPCMessageLine(t, stdout)
	assert.Equal(t, "2.0", msg["jsonrpc"])
	assertGatewayMCPStdioHygiene(t, stdout, stderr)
}

func TestGatewayMCPStdio_NoProjectExitsWithStderr(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err)
	apiKey := os.Getenv("HOOKDECK_CLI_TESTING_API_KEY")
	require.NotEmpty(t, apiKey, "HOOKDECK_CLI_TESTING_API_KEY required")
	cfgPath := filepath.Join(t.TempDir(), "config.toml")
	toml := fmt.Sprintf("profile = \"default\"\n\n[default]\napi_key = %q\n", apiKey)
	require.NoError(t, os.WriteFile(cfgPath, []byte(toml), 0644))

	stdout, stderr, waitErr := RunGatewayMCPSubprocess(t, projectRoot, cfgPath, nil, "", 4*time.Second)
	assert.Error(t, waitErr)
	lower := strings.ToLower(stderr)
	assert.True(t, strings.Contains(lower, "project"), "stderr=%q", stderr)
	if strings.TrimSpace(stdout) != "" {
		assertGatewayMCPStdioHygiene(t, stdout, stderr)
	}
}

func TestMCPEventsList_DateRangeAndBodyFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	result := CallGatewayMCPTool(t, cli.projectRoot, cli.configPath, "gateway_events", map[string]any{
		"action":         "list",
		"created_after":  "2020-01-01T00:00:00Z",
		"created_before": "2030-01-01T00:00:00Z",
		"body":           map[string]any{},
		"limit":          5,
	}, 20*time.Second)
	assert.False(t, result.IsError, "tool error: %s", result.Text)
	assert.Contains(t, result.Text, `"data"`)
	assert.True(t, strings.Contains(result.Text, `"models"`) || strings.Contains(result.Text, `"count"`),
		"expected list payload in %s", result.Text)
}

func TestMCPRequestsList_DateRangeAndBodyFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	result := CallGatewayMCPTool(t, cli.projectRoot, cli.configPath, "gateway_requests", map[string]any{
		"action":         "list",
		"ingested_after": "2020-01-01T00:00:00Z",
		"created_before": "2030-01-01T00:00:00Z",
		"body":           map[string]any{},
		"limit":          5,
	}, 20*time.Second)
	assert.False(t, result.IsError, "tool error: %s", result.Text)
	assert.Contains(t, result.Text, `"data"`)
	assert.True(t, strings.Contains(result.Text, `"models"`) || strings.Contains(result.Text, `"count"`),
		"expected list payload in %s", result.Text)
}

// The singular tools are the ones an agent reaches for once it has an id, so
// they have to be reachable end to end. A missing record fails at the API,
// which proves the call got that far; an unknown action or tool would not.
func TestMCPSingularToolsAreReachable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)

	cases := []struct{ tool, id string }{
		{"gateway_event", "evt_does_not_exist"},
		{"gateway_request", "req_does_not_exist"},
	}

	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			result := CallGatewayMCPTool(t, cli.projectRoot, cli.configPath, tc.tool, map[string]any{
				"action": "get",
				"id":     tc.id,
			}, 20*time.Second)
			assert.NotContains(t, result.Text, "unknown action")
			assert.NotContains(t, result.Text, "Unknown tool")
		})
	}
}

func TestGatewayMCPStdio_OutpostProjectRejected(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err)
	apiKey := os.Getenv("HOOKDECK_CLI_TESTING_API_KEY")
	require.NotEmpty(t, apiKey)
	cfgPath := filepath.Join(t.TempDir(), "config.toml")
	toml := fmt.Sprintf("profile = \"default\"\n\n[default]\napi_key = %q\nproject_id = \"proj_outpost_fake\"\nproject_type = \"Outpost\"\n", apiKey)
	require.NoError(t, os.WriteFile(cfgPath, []byte(toml), 0644))

	stdout, stderr, waitErr := RunGatewayMCPSubprocess(t, projectRoot, cfgPath, nil, "", 4*time.Second)
	assert.Error(t, waitErr)
	assert.Contains(t, strings.ToLower(stderr), "gateway")
	if strings.TrimSpace(stdout) != "" {
		assertGatewayMCPStdioHygiene(t, stdout, stderr)
	}
}

// --- Write mode (--allow-write) ---

var gatewayMCPCommand = []string{"gateway", "mcp"}

func TestGatewayMCPStdio_ReadOnlyByDefault(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)

	tools, stdout, stderr := ListMCPTools(t, cli.projectRoot, cli.configPath, gatewayMCPCommand, 10*time.Second)
	assertGatewayMCPStdioHygiene(t, stdout, stderr)

	// Product tools take the gateway_ prefix; the platform tools keep
	// hookdeck_, because you log in to Hookdeck and switch a Hookdeck project
	// whichever product's server you are in.
	for _, name := range []string{
		"hookdeck_projects", "hookdeck_login",
		"gateway_help", "gateway_connections", "gateway_sources",
		"gateway_destinations", "gateway_transformations",
		"gateway_requests", "gateway_request",
		"gateway_events", "gateway_event",
		"gateway_attempts", "gateway_issues", "gateway_metrics",
	} {
		assert.Contains(t, tools, name)
	}
	for _, name := range []string{"gateway_login", "gateway_projects"} {
		assert.NotContains(t, tools, name, "platform tools must not carry the product prefix")
	}
	for _, name := range []string{"hookdeck_connections", "hookdeck_events", "hookdeck_help"} {
		assert.NotContains(t, tools, name, "product tools were renamed to gateway_ in v3")
	}

	// Nothing that creates, changes or deletes.
	assert.Equal(t, []string{"list", "get", "pause", "unpause"},
		MCPToolActionEnum(t, tools["gateway_connections"]))
	assert.Equal(t, []string{"list", "get"}, MCPToolActionEnum(t, tools["gateway_sources"]))
	// Events and requests are split plural/singular: the plural tools search,
	// the singular ones act on one record by id.
	assert.Equal(t, []string{"list"}, MCPToolActionEnum(t, tools["gateway_events"]))
	assert.Equal(t, []string{"get", "raw_body"}, MCPToolActionEnum(t, tools["gateway_event"]))
	assert.Equal(t, []string{"list"}, MCPToolActionEnum(t, tools["gateway_requests"]))
	assert.Equal(t, []string{"get", "raw_body", "events", "ignored_events"},
		MCPToolActionEnum(t, tools["gateway_request"]))
}

func TestGatewayMCPStdio_AllowWriteAddsWriteActions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)

	command := append(append([]string{}, gatewayMCPCommand...), "--allow-write")
	tools, stdout, stderr := ListMCPTools(t, cli.projectRoot, cli.configPath, command, 10*time.Second)
	assertGatewayMCPStdioHygiene(t, stdout, stderr)

	// retry lives on the singular tools, and stays off the plural ones.
	assert.Contains(t, MCPToolActionEnum(t, tools["gateway_event"]), "retry")
	assert.Contains(t, MCPToolActionEnum(t, tools["gateway_request"]), "retry")
	assert.Equal(t, []string{"list"}, MCPToolActionEnum(t, tools["gateway_events"]))
	assert.Equal(t, []string{"list"}, MCPToolActionEnum(t, tools["gateway_requests"]))
	for _, want := range []string{"create", "upsert", "update", "delete", "enable", "disable"} {
		assert.Contains(t, MCPToolActionEnum(t, tools["gateway_connections"]), want)
		assert.Contains(t, MCPToolActionEnum(t, tools["gateway_sources"]), want)
	}
	assert.Contains(t, MCPToolActionEnum(t, tools["gateway_issues"]), "dismiss")

	// Read-only tools stay read-only in write mode.
	assert.Equal(t, []string{"list", "get"}, MCPToolActionEnum(t, tools["gateway_attempts"]))
}

func TestGatewayMCPStdio_ReadOnlyRefusesWriteAction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)

	result := CallMCPTool(t, cli.projectRoot, cli.configPath, gatewayMCPCommand, "gateway_sources", map[string]any{
		"action": "delete",
		"id":     "src_does_not_exist",
	}, 20*time.Second)

	require.True(t, result.IsError, "a read-only server must refuse delete: %s", result.Text)
	assert.Contains(t, result.Text, "read-only mode")
	assert.Contains(t, result.Text, "--allow-write")
}

// TestGatewayMCPStdio_PauseStaysAvailableReadOnly is the acceptance-level
// counterpart to the unit test: pause is a mutation that deliberately remains
// offered in read-only mode, because read-only is the mode incidents get
// investigated in.
func TestGatewayMCPStdio_PauseStaysAvailableReadOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)

	result := CallMCPTool(t, cli.projectRoot, cli.configPath, gatewayMCPCommand, "gateway_connections", map[string]any{
		"action": "pause",
		"id":     "conn_does_not_exist",
	}, 20*time.Second)

	// It fails because the connection does not exist, not because the action
	// was gated. The distinction is the whole point of the test.
	assert.NotContains(t, result.Text, "read-only mode")
	assert.NotContains(t, result.Text, "--allow-write")
}

func TestGatewayMCPTool_HelpReportsMode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)

	readOnly := CallMCPTool(t, cli.projectRoot, cli.configPath, gatewayMCPCommand,
		"gateway_help", map[string]any{}, 20*time.Second)
	assert.Contains(t, readOnly.Text, "Mode: read-only")
	assert.Contains(t, readOnly.Text, "--allow-write")

	write := CallMCPTool(t, cli.projectRoot, cli.configPath,
		append(append([]string{}, gatewayMCPCommand...), "--allow-write"),
		"gateway_help", map[string]any{}, 20*time.Second)
	assert.Contains(t, write.Text, "Mode: write enabled")
}
