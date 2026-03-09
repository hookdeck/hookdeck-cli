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

Authentication is inherited from the CLI profile (run "hookdeck login" first).`,
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
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	client := Config.GetAPIClient()
	srv := gatewaymcp.NewServer(client)
	return srv.RunStdio(context.Background())
}
