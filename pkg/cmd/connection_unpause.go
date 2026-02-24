package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type connectionUnpauseCmd struct {
	cmd *cobra.Command
}

func newConnectionUnpauseCmd() *connectionUnpauseCmd {
	cc := &connectionUnpauseCmd{}

	cc.cmd = &cobra.Command{
		Use:   "unpause <connection-id>",
		Args:  validators.ExactArgs(1),
		Short: "Resume a paused connection",
		Long: `Resume a paused connection.

The connection will start processing queued events.`,
		RunE: cc.runConnectionUnpauseCmd,
	}

	return cc
}

func (cc *connectionUnpauseCmd) runConnectionUnpauseCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	client := Config.GetAPIClient()
	ctx := context.Background()

	conn, err := client.UnpauseConnection(ctx, args[0])
	if err != nil {
		return fmt.Errorf("failed to unpause connection: %w", err)
	}

	name := "unnamed"
	if conn.Name != nil {
		name = *conn.Name
	}

	fmt.Printf(SuccessCheck+" Connection unpaused: %s (%s)\n", name, conn.ID)
	return nil
}
