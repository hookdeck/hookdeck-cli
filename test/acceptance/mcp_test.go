//go:build mcp

package acceptance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mcpInitializeJSON = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","clientInfo":{"name":"test","version":"1.0"},"capabilities":{}}}`

func firstJSONRPCMessageLine(t *testing.T, stdout string) map[string]any {
	t.Helper()
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var msg map[string]any
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if _, ok := msg["jsonrpc"]; ok {
			return msg
		}
	}
	t.Fatalf("no JSON-RPC line in stdout: %q", stdout)
	return nil
}

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
