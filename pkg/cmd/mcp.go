package cmd

import (
	"context"
	"os"

	gatewaymcp "github.com/hookdeck/hookdeck-cli/pkg/gateway/mcp"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
	"github.com/spf13/cobra"
)

type mcpCmd struct {
	cmd *cobra.Command

	allowWrite bool
	readOnly   bool
}

func newMCPCmd() *mcpCmd {
	mc := &mcpCmd{}
	mc.cmd = &cobra.Command{
		Use:   "mcp",
		Args:  validators.NoArgs,
		Short: ShortBeta("Start an MCP server for AI agent access to Hookdeck"),
		Long: LongBeta(`Starts a Model Context Protocol (MCP) server over stdio.

The server exposes Hookdeck Event Gateway resources — connections, sources,
destinations, events, requests, and more — as MCP tools that AI agents and
LLM-based clients can invoke.

The server starts read-only: tools advertise only the actions that read data,
so an agent is never offered an action it cannot perform. Pass --allow-write to
enable creating, changing and deleting.

Pausing and unpausing a connection are available in both modes. Stopping a
misbehaving connection is the natural end of an investigation, and both are
reversible: pausing buffers delivery rather than dropping events.

Product tools are prefixed gateway_, so this server and 'hookdeck outpost mcp'
can be configured in the same client. Signing in and switching project are
Hookdeck operations rather than Event Gateway ones, so they keep the platform
prefix: hookdeck_login and hookdeck_projects.

If the CLI is already authenticated, all tools are available immediately.
If not, gateway MCP still starts: project selection is skipped until you
authenticate, and hookdeck_login initiates browser-based sign-in. Protocol
traffic uses stdout only (JSON-RPC); status and errors from the CLI before
the server runs go to stderr.

hookdeck_login stays registered after sign-in so you can call it with reauth: true
to replace credentials (e.g. when project listing fails with a narrow API key).`),
		Example: `  # Start the MCP server, read-only (stdio transport)
  hookdeck gateway mcp

  # Allow tools that change data
  hookdeck gateway mcp --allow-write

  # Pipe a JSON-RPC initialize request for testing
  echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","clientInfo":{"name":"test","version":"1.0"},"capabilities":{}}}' | hookdeck gateway mcp`,
		RunE: mc.runMCPCmd,
	}

	addWriteModeFlags(mc.cmd, &mc.allowWrite, &mc.readOnly,
		"Enable tools that create, change or delete data.")

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

	writeEnabled := resolveAllowWrite(
		mc.allowWrite,
		cmd.Flags().Changed("allow-write"),
		mc.readOnly,
		os.Getenv(allowWriteEnvVar),
	)

	srv := gatewaymcp.NewServer(gatewaymcp.ServerOptions{
		Client:       client,
		Config:       &Config,
		WriteEnabled: writeEnabled,
	})
	return srv.RunStdio(context.Background())
}
