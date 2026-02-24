package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type sourceDisableCmd struct {
	cmd *cobra.Command
}

func newSourceDisableCmd() *sourceDisableCmd {
	sc := &sourceDisableCmd{}

	sc.cmd = &cobra.Command{
		Use:   "disable <source-id>",
		Args:  validators.ExactArgs(1),
		Short: ShortDisable(ResourceSource),
		Long:  LongDisableIntro(ResourceSource),
		RunE: sc.runSourceDisableCmd,
	}

	return sc
}

func (sc *sourceDisableCmd) runSourceDisableCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	client := Config.GetAPIClient()
	ctx := context.Background()

	src, err := client.DisableSource(ctx, args[0])
	if err != nil {
		return fmt.Errorf("failed to disable source: %w", err)
	}

	fmt.Printf("✓ Source disabled: %s (%s)\n", src.Name, src.ID)
	return nil
}
