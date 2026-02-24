package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type transformationDeleteCmd struct {
	cmd   *cobra.Command
	force bool
}

func newTransformationDeleteCmd() *transformationDeleteCmd {
	tc := &transformationDeleteCmd{}

	tc.cmd = &cobra.Command{
		Use:   "delete <transformation-id-or-name>",
		Args:  validators.ExactArgs(1),
		Short: ShortDelete(ResourceTransformation),
		Long: LongDeleteIntro(ResourceTransformation) + `

Examples:
  hookdeck gateway transformation delete trn_abc123
  hookdeck gateway transformation delete trn_abc123 --force`,
		PreRunE: tc.validateFlags,
		RunE:    tc.runTransformationDeleteCmd,
	}

	tc.cmd.Flags().BoolVar(&tc.force, "force", false, "Force delete without confirmation")

	return tc
}

func (tc *transformationDeleteCmd) validateFlags(cmd *cobra.Command, args []string) error {
	return Config.Profile.ValidateAPIKey()
}

func (tc *transformationDeleteCmd) runTransformationDeleteCmd(cmd *cobra.Command, args []string) error {
	idOrName := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	trnID, err := resolveTransformationID(ctx, client, idOrName)
	if err != nil {
		return err
	}

	t, err := client.GetTransformation(ctx, trnID)
	if err != nil {
		return fmt.Errorf("failed to get transformation: %w", err)
	}

	if !tc.force {
		fmt.Printf("\nAre you sure you want to delete transformation '%s' (%s)? [y/N]: ", t.Name, trnID)
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Deletion cancelled.")
			return nil
		}
	}

	if err := client.DeleteTransformation(ctx, trnID); err != nil {
		return fmt.Errorf("failed to delete transformation: %w", err)
	}

	fmt.Printf(SuccessCheck+" Transformation deleted: %s (%s)\n", t.Name, trnID)
	return nil
}
