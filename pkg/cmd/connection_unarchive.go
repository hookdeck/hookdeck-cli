package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type connectionUnarchiveCmd struct {
	cmd *cobra.Command
}

func newConnectionUnarchiveCmd() *connectionUnarchiveCmd {
	cc := &connectionUnarchiveCmd{}

	cc.cmd = &cobra.Command{
		Use:   "unarchive <connection-id>",
		Args:  validators.ExactArgs(1),
		Short: "Restore an archived connection",
		Long: `Restore an archived connection.

The connection will be unarchived and visible in active lists.`,
		RunE: cc.runConnectionUnarchiveCmd,
	}

	return cc
}

func (cc *connectionUnarchiveCmd) runConnectionUnarchiveCmd(cmd *cobra.Command, args []string) error {
	client := Config.GetAPIClient()
	ctx := context.Background()

	conn, err := client.UnarchiveConnection(ctx, args[0])
	if err != nil {
		return fmt.Errorf("failed to unarchive connection: %w", err)
	}

	name := "unnamed"
	if conn.Name != nil {
		name = *conn.Name
	}

	fmt.Printf("✓ Connection unarchived: %s (%s)\n", name, conn.ID)
	return nil
}
