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
		Short: ShortDisable(ResourceConnection),
		Long:  LongDisableIntro(ResourceConnection),
		RunE: cc.runConnectionDisableCmd,
	}

	return cc
}

func (cc *connectionDisableCmd) runConnectionDisableCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

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

	fmt.Printf(SuccessCheck+" Connection disabled: %s (%s)\n", name, conn.ID)
	return nil
}
