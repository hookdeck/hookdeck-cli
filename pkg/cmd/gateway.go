package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type gatewayCmd struct {
	cmd *cobra.Command
}

func newGatewayCmd() *gatewayCmd {
	g := &gatewayCmd{}

	g.cmd = &cobra.Command{
		Use:   "gateway",
		Args:  validators.NoArgs,
		Short: "Manage Hookdeck Event Gateway resources",
		Long: `Commands for managing Event Gateway sources, destinations, connections,
transformations, events, requests, and metrics.

The gateway command group provides full access to all Event Gateway resources.

Examples:
  # List connections
  hookdeck gateway connection list

  # Create a source
  hookdeck gateway source create --name my-source --type WEBHOOK

  # Query event metrics
  hookdeck gateway metrics events --start 2026-01-01T00:00:00Z --end 2026-02-01T00:00:00Z`,
	}

	// Register resource subcommands (same factory as root backward-compat registration)
	addConnectionCmdTo(g.cmd)
	addSourceCmdTo(g.cmd)
	addDestinationCmdTo(g.cmd)
	addTransformationCmdTo(g.cmd)

	return g
}
