package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type connectionPauseCmd struct {
	cmd *cobra.Command
}

func newConnectionPauseCmd() *connectionPauseCmd {
	cc := &connectionPauseCmd{}

	cc.cmd = &cobra.Command{
		Use:   "pause <connection-id>",
		Args:  validators.ExactArgs(1),
		Short: "Pause a connection temporarily",
		Long: `Pause a connection temporarily.

The connection will queue incoming events until unpaused.`,
		RunE: cc.runConnectionPauseCmd,
	}

	return cc
}

func (cc *connectionPauseCmd) runConnectionPauseCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	client := Config.GetAPIClient()
	ctx := context.Background()

	conn, err := client.PauseConnection(ctx, args[0])
	if err != nil {
		return fmt.Errorf("failed to pause connection: %w", err)
	}

	name := "unnamed"
	if conn.Name != nil {
		name = *conn.Name
	}

	fmt.Printf(SuccessCheck+" Connection paused: %s (%s)\n", name, conn.ID)
	return nil
}
