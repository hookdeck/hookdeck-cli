package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type destinationEnableCmd struct {
	cmd *cobra.Command
}

func newDestinationEnableCmd() *destinationEnableCmd {
	dc := &destinationEnableCmd{}

	dc.cmd = &cobra.Command{
		Use:   "enable <destination-id>",
		Args:  validators.ExactArgs(1),
		Short: ShortEnable(ResourceDestination),
		Long:  LongEnableIntro(ResourceDestination),
		RunE:  dc.runDestinationEnableCmd,
	}

	return dc
}

func (dc *destinationEnableCmd) runDestinationEnableCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	client := Config.GetAPIClient()
	ctx := context.Background()

	dst, err := client.EnableDestination(ctx, args[0])
	if err != nil {
		return fmt.Errorf("failed to enable destination: %w", err)
	}

	fmt.Printf(SuccessCheck+" Destination enabled: %s (%s)\n", dst.Name, dst.ID)
	return nil
}
