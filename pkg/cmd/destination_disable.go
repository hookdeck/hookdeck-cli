package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type destinationDisableCmd struct {
	cmd *cobra.Command
}

func newDestinationDisableCmd() *destinationDisableCmd {
	dc := &destinationDisableCmd{}

	dc.cmd = &cobra.Command{
		Use:   "disable <destination-id>",
		Args:  validators.ExactArgs(1),
		Short: ShortDisable(ResourceDestination),
		Long:  LongDisableIntro(ResourceDestination),
		RunE:  dc.runDestinationDisableCmd,
	}

	return dc
}

func (dc *destinationDisableCmd) runDestinationDisableCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	client := Config.GetAPIClient()
	ctx := context.Background()

	dst, err := client.DisableDestination(ctx, args[0])
	if err != nil {
		return fmt.Errorf("failed to disable destination: %w", err)
	}

	fmt.Printf(SuccessCheck+" Destination disabled: %s (%s)\n", dst.Name, dst.ID)
	return nil
}
