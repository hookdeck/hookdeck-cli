package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type connectionArchiveCmd struct {
	cmd *cobra.Command
}

func newConnectionArchiveCmd() *connectionArchiveCmd {
	cc := &connectionArchiveCmd{}

	cc.cmd = &cobra.Command{
		Use:   "archive <connection-id>",
		Args:  validators.ExactArgs(1),
		Short: "Archive a connection",
		Long: `Archive a connection.

The connection will be archived and hidden from active lists.`,
		RunE: cc.runConnectionArchiveCmd,
	}

	return cc
}

func (cc *connectionArchiveCmd) runConnectionArchiveCmd(cmd *cobra.Command, args []string) error {
	client := Config.GetAPIClient()
	ctx := context.Background()

	conn, err := client.ArchiveConnection(ctx, args[0])
	if err != nil {
		return fmt.Errorf("failed to archive connection: %w", err)
	}

	name := "unnamed"
	if conn.Name != nil {
		name = *conn.Name
	}

	fmt.Printf("✓ Connection archived: %s (%s)\n", name, conn.ID)
	return nil
}
