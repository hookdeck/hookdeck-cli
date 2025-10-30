package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type connectionEnableCmd struct {
	cmd *cobra.Command
}

func newConnectionEnableCmd() *connectionEnableCmd {
	cc := &connectionEnableCmd{}

	cc.cmd = &cobra.Command{
		Use:   "enable <connection-id>",
		Args:  validators.ExactArgs(1),
		Short: "Enable a connection",
		Long: `Enable a disabled connection.

The connection will resume processing events.`,
		RunE: cc.runConnectionEnableCmd,
	}

	return cc
}

func (cc *connectionEnableCmd) runConnectionEnableCmd(cmd *cobra.Command, args []string) error {
	client := Config.GetAPIClient()
	ctx := context.Background()

	conn, err := client.EnableConnection(ctx, args[0])
	if err != nil {
		return fmt.Errorf("failed to enable connection: %w", err)
	}

	name := "unnamed"
	if conn.Name != nil {
		name = *conn.Name
	}

	fmt.Printf("✓ Connection enabled: %s (%s)\n", name, conn.ID)
	return nil
}
