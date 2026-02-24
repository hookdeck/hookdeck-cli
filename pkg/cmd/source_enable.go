package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type sourceEnableCmd struct {
	cmd *cobra.Command
}

func newSourceEnableCmd() *sourceEnableCmd {
	sc := &sourceEnableCmd{}

	sc.cmd = &cobra.Command{
		Use:   "enable <source-id>",
		Args:  validators.ExactArgs(1),
		Short: ShortEnable(ResourceSource),
		Long:  LongEnableIntro(ResourceSource),
		RunE: sc.runSourceEnableCmd,
	}

	return sc
}

func (sc *sourceEnableCmd) runSourceEnableCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	client := Config.GetAPIClient()
	ctx := context.Background()

	src, err := client.EnableSource(ctx, args[0])
	if err != nil {
		return fmt.Errorf("failed to enable source: %w", err)
	}

	fmt.Printf(SuccessCheck+" Source enabled: %s (%s)\n", src.Name, src.ID)
	return nil
}
