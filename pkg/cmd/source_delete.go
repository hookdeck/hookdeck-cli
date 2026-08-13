package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type sourceDeleteCmd struct {
	cmd   *cobra.Command
	force bool
}

func newSourceDeleteCmd() *sourceDeleteCmd {
	sc := &sourceDeleteCmd{}

	sc.cmd = &cobra.Command{
		Use:   "delete <source-id>",
		Args:  validators.ExactArgs(1),
		Short: ShortDelete(ResourceSource),
		Long: LongDeleteIntro(ResourceSource) + `

Examples:
  hookdeck gateway source delete src_abc123
  hookdeck gateway source delete src_abc123 --force`,
		PreRunE: sc.validateFlags,
		RunE:    sc.runSourceDeleteCmd,
	}

	sc.cmd.Flags().BoolVar(&sc.force, "force", false, "Force delete without confirmation")

	return sc
}

func (sc *sourceDeleteCmd) validateFlags(cmd *cobra.Command, args []string) error {
	return Config.Profile.ValidateAPIKey()
}

func (sc *sourceDeleteCmd) runSourceDeleteCmd(cmd *cobra.Command, args []string) error {
	sourceID := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	src, err := client.GetSource(ctx, sourceID, nil)
	if err != nil {
		return fmt.Errorf("failed to get source: %w", err)
	}

	if !sc.force {
		proceed, err := confirmDestructiveAction(
			fmt.Sprintf("\nAre you sure you want to delete source '%s' (%s)?", src.Name, sourceID),
			"Deletion cancelled.",
			"force",
		)
		if err != nil {
			return err
		}
		if !proceed {
			return nil
		}
	}

	if err := client.DeleteSource(ctx, sourceID); err != nil {
		return fmt.Errorf("failed to delete source: %w", err)
	}

	fmt.Printf(SuccessCheck+" Source deleted: %s (%s)\n", src.Name, sourceID)
	return nil
}
