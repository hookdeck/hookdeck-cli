//go:build mcp

package acceptance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
