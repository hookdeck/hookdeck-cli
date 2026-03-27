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
		Use:   "unpause <connection-id-or-name>",
		Args:  validators.ExactArgs(1),
		Short: "Resume a paused connection",
		Long: `Resume a paused connection.

The connection will start processing queued events.

Examples:
	 # Unpause by connection ID
	 hookdeck gateway connection unpause web_abc123

	 # Unpause by connection name
	 hookdeck gateway connection unpause my-connection`,
		RunE: cc.runConnectionUnpauseCmd,
	}
	cc.cmd.Annotations = map[string]string{
		"cli.arguments": `[{"name":"connection-id-or-name","type":"string","description":"Connection ID or name","required":true}]`,
	}

	return cc
}

func (cc *connectionUnpauseCmd) runConnectionUnpauseCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	client := Config.GetAPIClient()
	ctx := context.Background()

	id, err := resolveConnectionID(ctx, client, args[0])
	if err != nil {
		return err
	}

	conn, err := client.UnpauseConnection(ctx, id)
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
