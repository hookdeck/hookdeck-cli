package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type gatewayCmd struct {
	cmd *cobra.Command
}

// isGatewayMCPLeafCommand reports whether cmd is the gateway mcp subcommand.
// MCP uses JSON-RPC on stdout; when there is no API key yet, requireGatewayProject
// must not run so the server can start and expose hookdeck_login.
func isGatewayMCPLeafCommand(cmd *cobra.Command) bool {
	return cmd != nil && cmd.Name() == "mcp" && cmd.Parent() != nil && cmd.Parent().Name() == "gateway"
}

// gatewayPersistentPreRunE runs before gateway subcommands. MCP may start without credentials
// (JSON-RPC on stdout; hookdeck_login supplies auth). Once logged in, gateway MCP uses the
// same Gateway project rules as other gateway commands.
func gatewayPersistentPreRunE(cmd *cobra.Command, args []string) error {
	initTelemetry(cmd)
	if isGatewayMCPLeafCommand(cmd) {
		if err := Config.Profile.ValidateAPIKey(); err != nil {
			return nil
		}
	}
	return requireGatewayProject(nil)
}

// requireGatewayProject ensures the current project is a Gateway project (inbound or console).
// It runs API key validation, resolves project type from config or API, and returns an error if not Gateway.
// cfg is optional; when nil, the global Config is used (for production).
func requireGatewayProject(cfg *config.Config) error {
	if cfg == nil {
		cfg = &Config
	}
	if err := cfg.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	if cfg.Profile.ProjectId == "" {
		return fmt.Errorf("no project selected. Run 'hookdeck project use' to select a project")
	}
	projectType := cfg.Profile.ProjectType
	if projectType == "" && cfg.Profile.ProjectMode != "" {
		projectType = config.ModeToProjectType(cfg.Profile.ProjectMode)
	}
	if projectType == "" {
		// Resolve team/project/mode/type from API (authoritative for the key). Do not clear
		// guest_url here — gateway PreRun may run for users who still have a guest upgrade link.
		response, err := cfg.GetAPIClient().ValidateAPIKey()
		if err != nil {
			return err
		}
		cfg.Profile.ApplyValidateAPIKeyResponse(response, false)
		projectType = cfg.Profile.ProjectType
		_ = cfg.Profile.SaveProfile()
	}
	if !config.IsGatewayProject(projectType) {
		return fmt.Errorf("this command requires a Gateway project; current project type is %s. Use 'hookdeck project use' to switch to a Gateway project", projectType)
	}
	return nil
}

func newGatewayCmd() *gatewayCmd {
	g := &gatewayCmd{}

	g.cmd = &cobra.Command{
		Use:   "gateway",
		Args:  validators.NoArgs,
		Short: "Manage Hookdeck Event Gateway resources",
		Long: `Commands for managing Event Gateway sources, destinations, connections,
transformations, events, requests, metrics, and MCP server.

The gateway command group provides full access to all Event Gateway resources.`,
		Example: `  # List connections
  hookdeck gateway connection list

  # Create a source
  hookdeck gateway source create --name my-source --type WEBHOOK

  # Query event metrics
  hookdeck gateway metrics events --start 2026-01-01T00:00:00Z --end 2026-02-01T00:00:00Z

  # Start the MCP server for AI agent access
  hookdeck gateway mcp`,
		PersistentPreRunE: gatewayPersistentPreRunE,
	}

	// Register resource subcommands (same factory as root backward-compat registration)
	addConnectionCmdTo(g.cmd)
	addSourceCmdTo(g.cmd)
	addDestinationCmdTo(g.cmd)
	addTransformationCmdTo(g.cmd)
	addEventCmdTo(g.cmd)
	addRequestCmdTo(g.cmd)
	addAttemptCmdTo(g.cmd)
	addMetricsCmdTo(g.cmd)
	addIssueCmdTo(g.cmd)
	addMCPCmdTo(g.cmd)

	return g
}
