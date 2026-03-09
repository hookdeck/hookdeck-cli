package cmd

import (
	"context"

	gatewaymcp "github.com/hookdeck/hookdeck-cli/pkg/gateway/mcp"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
	"github.com/spf13/cobra"
)

type mcpCmd struct {
	cmd *cobra.Command
}

func newMCPCmd() *mcpCmd {
	mc := &mcpCmd{}
	mc.cmd = &cobra.Command{
		Use:   "mcp",
		Args:  validators.NoArgs,
		Short: "Start an MCP server for AI agent access to Hookdeck",
		Long: `Starts a Model Context Protocol (MCP) server over stdio.

The server exposes Hookdeck Event Gateway resources — connections, sources,
destinations, events, requests, and more — as MCP tools that AI agents and
LLM-based clients can invoke.

If the CLI is already authenticated, all tools are available immediately.
If not, a hookdeck_login tool is provided that initiates browser-based
authentication so the user can log in without leaving the MCP session.`,
		Example: `  # Start the MCP server (stdio transport)
  hookdeck gateway mcp

  # Pipe a JSON-RPC initialize request for testing
  echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","clientInfo":{"name":"test","version":"1.0"},"capabilities":{}}}' | hookdeck gateway mcp`,
		RunE: mc.runMCPCmd,
	}
	return mc
}

func addMCPCmdTo(parent *cobra.Command) {
	parent.AddCommand(newMCPCmd().cmd)
}

func (mc *mcpCmd) runMCPCmd(cmd *cobra.Command, args []string) error {
	// Always build the client — it may have an empty APIKey if the CLI is
	// not yet authenticated. The MCP server handles this gracefully by
	// registering a hookdeck_login tool instead of crashing.
	client := Config.GetAPIClient()
	srv := gatewaymcp.NewServer(client, &Config)
	return srv.RunStdio(context.Background())
}
