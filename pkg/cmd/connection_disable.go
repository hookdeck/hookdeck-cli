package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type connectionDisableCmd struct {
	cmd *cobra.Command
}

func newConnectionDisableCmd() *connectionDisableCmd {
	cc := &connectionDisableCmd{}

	cc.cmd = &cobra.Command{
		Use:   "disable <connection-id>",
		Args:  validators.ExactArgs(1),
		Short: "Disable a connection",
		Long: `Disable an active connection.

The connection will stop processing events until re-enabled.`,
		RunE: cc.runConnectionDisableCmd,
	}

	return cc
}

func (cc *connectionDisableCmd) runConnectionDisableCmd(cmd *cobra.Command, args []string) error {
	client := Config.GetAPIClient()
	ctx := context.Background()

	conn, err := client.DisableConnection(ctx, args[0])
	if err != nil {
		return fmt.Errorf("failed to disable connection: %w", err)
	}

	name := "unnamed"
	if conn.Name != nil {
		name = *conn.Name
	}

	fmt.Printf("✓ Connection disabled: %s (%s)\n", name, conn.ID)
	return nil
}
