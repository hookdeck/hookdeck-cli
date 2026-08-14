package cmd

import (
	"context"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	outpostmcp "github.com/hookdeck/hookdeck-cli/pkg/outpost/mcp"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

// allowWriteEnvVar enables write actions without a flag, for MCP clients whose
// config makes environment variables easier to set than arguments.
const allowWriteEnvVar = "HOOKDECK_MCP_ALLOW_WRITE"

type outpostMCPCmd struct {
	cmd *cobra.Command

	allowWrite bool
	readOnly   bool
	apiKey     string
}

func newOutpostMCPCmd() *outpostMCPCmd {
	mc := &outpostMCPCmd{}
	mc.cmd = &cobra.Command{
		Use:   "mcp",
		Args:  validators.NoArgs,
		Short: ShortBeta("Start an MCP server for AI agent access to Outpost"),
		Long: LongBeta(`Starts a Model Context Protocol (MCP) server over stdio.

The server exposes Hookdeck Outpost resources — tenants, destinations, events,
attempts, topics, metrics and project configuration — as MCP tools that AI
agents and LLM-based clients can invoke. Tools are prefixed outpost_, so this
server and 'hookdeck gateway mcp' can be configured in the same client.

The server starts read-only: tools advertise only the actions that read data,
so an agent is never offered an action it cannot perform. Pass --allow-write to
enable creating, changing and deleting. Two reads count as writes and are also
gated, because both return a reusable credential: 'outpost_tenants token' mints
a tenant-scoped access token, and 'outpost_tenants portal' returns a URL
granting access to a tenant's portal.

Publishing needs a Hookdeck Project API key, which the credentials stored by
'hookdeck login' cannot substitute for. Without one the publish tool is not
registered at all; pass --api-key or set HOOKDECK_API_KEY to enable it.

If the CLI is already authenticated, all tools are available immediately. If
not, the server still starts and outpost_login initiates browser-based sign-in.
Protocol traffic uses stdout only (JSON-RPC); status and errors from the CLI
before the server runs go to stderr.`),
		Example: `  # Start the MCP server, read-only (stdio transport)
  hookdeck outpost mcp

  # Allow tools that change data
  hookdeck outpost mcp --allow-write

  # Allow writes, including publishing events
  hookdeck outpost mcp --allow-write --api-key $HOOKDECK_API_KEY

  # Pipe a JSON-RPC initialize request for testing
  echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","clientInfo":{"name":"test","version":"1.0"},"capabilities":{}}}' | hookdeck outpost mcp`,
		RunE: mc.runOutpostMCPCmd,
	}

	mc.cmd.Flags().BoolVar(&mc.allowWrite, "allow-write", false, "Enable tools that create, change or delete data, and that return tenant credentials. Also read from "+allowWriteEnvVar+"; the flag wins.")
	// Users arriving from other MCP servers type --read-only reflexively. It is
	// already the default, so accept it rather than failing on an unknown flag.
	mc.cmd.Flags().BoolVar(&mc.readOnly, "read-only", false, "Run without write actions. This is the default; the flag is accepted so it can be passed explicitly, and wins over --allow-write.")
	// The env var is read at run time rather than used as the flag default, so a
	// key that is already in the environment is not printed back out by --help.
	mc.cmd.Flags().StringVar(&mc.apiKey, "api-key", "", "Hookdeck Project API key, required by the publish tool. Read from HOOKDECK_API_KEY when not provided.")

	return mc
}

func addOutpostMCPCmdTo(parent *cobra.Command) {
	parent.AddCommand(newOutpostMCPCmd().cmd)
}

// resolveAllowWrite decides whether write actions are enabled.
//
// --read-only wins over everything so an explicit request for a safe session is
// never overridden; otherwise --allow-write wins over the environment variable,
// which is the more distant and easier-to-forget setting.
func resolveAllowWrite(allowWriteFlag, allowWriteFlagSet, readOnly bool, envValue string) bool {
	if readOnly {
		return false
	}
	if allowWriteFlagSet {
		return allowWriteFlag
	}
	enabled, err := strconv.ParseBool(envValue)
	if err != nil {
		return false
	}
	return enabled
}

func (mc *outpostMCPCmd) runOutpostMCPCmd(cmd *cobra.Command, args []string) error {
	// Always build the client — it may have an empty APIKey if the CLI is not
	// yet authenticated. The server handles that by registering outpost_login
	// rather than failing to start.
	//
	// This must be the Outpost client: the projects and login tools set the
	// project on the client they are given, and setting it on the Gateway client
	// would leave every Outpost call pointed at the previous project.
	client := Config.GetOutpostAPIClient()

	publishAPIKey := mc.apiKey
	if publishAPIKey == "" {
		publishAPIKey = os.Getenv("HOOKDECK_API_KEY")
	}

	writeEnabled := resolveAllowWrite(
		mc.allowWrite,
		cmd.Flags().Changed("allow-write"),
		mc.readOnly,
		os.Getenv(allowWriteEnvVar),
	)

	srv := outpostmcp.NewServer(outpostmcp.ServerOptions{
		Client: client,
		// Listing projects and validating credentials are account-level calls
		// that the Outpost host does not serve, so they go to the main API.
		AccountClient: Config.GetAPIClient(),
		Config:        &Config,
		WriteEnabled:  writeEnabled,
		PublishAPIKey: publishAPIKey,
	})
	return srv.RunStdio(context.Background())
}
