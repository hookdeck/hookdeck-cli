//go:build outpost

package acceptance

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var outpostMCPCommand = []string{"outpost", "mcp"}

// assertMCPStdoutIsJSONRPCOnly checks that nothing but protocol traffic reached
// stdout. Anything else corrupts the stream and breaks the client session.
func assertMCPStdoutIsJSONRPCOnly(t *testing.T, stdout string) {
	t.Helper()
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var msg map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &msg),
			"non-JSON line on stdout: %q", line)
		require.Contains(t, msg, "jsonrpc", "non-JSON-RPC object on stdout: %q", line)
	}
}

func TestOutpostMCPHelp(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewOutpostCLIRunner(t)
	stdout := cli.RunExpectSuccess("outpost", "mcp", "--help")
	assert.Contains(t, stdout, "Model Context Protocol")
	assert.Contains(t, stdout, "stdio")
	assert.Contains(t, stdout, "--allow-write")
	assert.Contains(t, stdout, "read-only")
}

func TestOutpostHelpListsMCP(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewOutpostCLIRunner(t)
	stdout := cli.RunExpectSuccess("outpost", "--help")
	assert.Contains(t, stdout, "mcp", "outpost --help should list the 'mcp' subcommand")
}

func TestOutpostMCPStdio_InitializeIsJSONRPCOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewOutpostCLIRunner(t)

	stdout, stderr, _ := RunOutpostMCPSubprocess(t, cli.projectRoot, cli.configPath, nil, nil,
		mcpInitializeJSON+"\n", 10*time.Second)

	msg := firstJSONRPCMessageLine(t, stdout)
	assert.Equal(t, "2.0", msg["jsonrpc"])
	assertMCPStdoutIsJSONRPCOnly(t, stdout)
	assert.NotContains(t, stdout, "Running `hookdeck login`")
	assert.NotContains(t, stderr, "Running `hookdeck login`")

	result, _ := msg["result"].(map[string]any)
	require.NotNil(t, result, "initialize returned no result: %v", msg)
	serverInfo, _ := result["serverInfo"].(map[string]any)
	require.NotNil(t, serverInfo)
	assert.Equal(t, "hookdeck-outpost", serverInfo["name"])
}

func TestOutpostMCPStdio_ReadOnlyByDefault(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewOutpostCLIRunner(t)

	tools, stdout, _ := ListMCPTools(t, cli.projectRoot, cli.configPath, outpostMCPCommand, 10*time.Second)
	assertMCPStdoutIsJSONRPCOnly(t, stdout)

	// Platform tools keep the hookdeck_ prefix in every server: you log in to
	// Hookdeck and switch a Hookdeck project, whichever product you are using.
	for _, name := range []string{
		"hookdeck_projects", "hookdeck_login",
		"outpost_help", "outpost_tenants",
		"outpost_destinations", "outpost_events", "outpost_attempts",
		"outpost_topics", "outpost_destination_types", "outpost_metrics",
		"outpost_config", "outpost_status",
	} {
		assert.Contains(t, tools, name)
	}
	for _, name := range []string{"outpost_login", "outpost_projects"} {
		assert.NotContains(t, tools, name, "platform tools must not carry the product prefix")
	}

	// Nothing that changes data, and nothing that hands back a credential.
	assert.NotContains(t, tools, "outpost_publish")
	assert.Equal(t, []string{"list", "get"}, MCPToolActionEnum(t, tools["outpost_tenants"]))
	assert.Equal(t, []string{"list", "get"}, MCPToolActionEnum(t, tools["outpost_destinations"]))
	assert.Equal(t, []string{"get", "custom_domain_get"}, MCPToolActionEnum(t, tools["outpost_config"]))
}

func TestOutpostMCPStdio_AllowWriteAddsWriteActions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewOutpostCLIRunner(t)

	command := append(append([]string{}, outpostMCPCommand...), "--allow-write")
	tools, stdout, _ := ListMCPTools(t, cli.projectRoot, cli.configPath, command, 10*time.Second)
	assertMCPStdoutIsJSONRPCOnly(t, stdout)

	tenantActions := MCPToolActionEnum(t, tools["outpost_tenants"])
	for _, want := range []string{"upsert", "delete", "token", "portal"} {
		assert.Contains(t, tenantActions, want)
	}
	assert.Contains(t, MCPToolActionEnum(t, tools["outpost_events"]), "retry")
	assert.Contains(t, MCPToolActionEnum(t, tools["outpost_config"]), "set")
}

func TestOutpostMCPStdio_ReadOnlyRefusesWriteAction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewOutpostCLIRunner(t)
	tenantID := uniqueTenantID(t)

	result := CallOutpostMCPTool(t, cli.projectRoot, cli.configPath, nil, "outpost_tenants", map[string]any{
		"action": "upsert",
		"id":     tenantID,
	}, 20*time.Second)

	require.True(t, result.IsError, "a read-only server must refuse upsert: %s", result.Text)
	assert.Contains(t, result.Text, "read-only mode")
	assert.Contains(t, result.Text, "--allow-write")

	// The refusal must be real: the tenant must not exist.
	stdout, _, err := cli.Run("outpost", "tenant", "get", tenantID)
	assert.Error(t, err, "the tenant should not have been created: %s", stdout)
}

func TestOutpostMCPTool_TenantsList(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewOutpostCLIRunner(t)
	tenantID := createTestTenant(t, cli)

	result := CallOutpostMCPTool(t, cli.projectRoot, cli.configPath, nil, "outpost_tenants", map[string]any{
		"action": "list",
		"limit":  50,
	}, 20*time.Second)

	require.False(t, result.IsError, "tool error: %s", result.Text)
	assert.Contains(t, result.Text, `"data"`)
	assert.Contains(t, result.Text, `"meta"`)
	assert.Contains(t, result.Text, tenantID)
}

func TestOutpostMCPTool_TopicsAndStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewOutpostCLIRunner(t)

	topics := CallOutpostMCPTool(t, cli.projectRoot, cli.configPath, nil, "outpost_topics", map[string]any{
		"action": "list",
	}, 20*time.Second)
	require.False(t, topics.IsError, "tool error: %s", topics.Text)
	assert.Contains(t, topics.Text, `"topics"`)

	status := CallOutpostMCPTool(t, cli.projectRoot, cli.configPath, nil, "outpost_status", map[string]any{
		"action": "get",
	}, 20*time.Second)
	require.False(t, status.IsError, "tool error: %s", status.Text)
	assert.Contains(t, status.Text, `"status"`)
}

func TestOutpostMCPTool_HelpReportsMode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewOutpostCLIRunner(t)

	readOnly := CallOutpostMCPTool(t, cli.projectRoot, cli.configPath, nil, "outpost_help", map[string]any{}, 20*time.Second)
	require.False(t, readOnly.IsError, "tool error: %s", readOnly.Text)
	assert.Contains(t, readOnly.Text, "Mode: read-only")
	assert.Contains(t, readOnly.Text, "--allow-write")

	write := CallOutpostMCPTool(t, cli.projectRoot, cli.configPath, []string{"--allow-write"},
		"outpost_help", map[string]any{}, 20*time.Second)
	require.False(t, write.IsError, "tool error: %s", write.Text)
	assert.Contains(t, write.Text, "Mode: write enabled")
}
