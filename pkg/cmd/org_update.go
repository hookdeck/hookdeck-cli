package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type orgUpdateCmd struct {
	cmd    *cobra.Command
	name   string
	output string
}

func newOrgUpdateCmd() *orgUpdateCmd {
	uc := &orgUpdateCmd{}

	uc.cmd = &cobra.Command{
		Use:   "update",
		Args:  validators.NoArgs,
		Short: "Rename the current organization",
		Long:  `Rename the Hookdeck organization your credentials belong to. Name is the only field the API accepts.`,
		Example: `$ hookdeck org update --name "Acme Inc"
$ hookdeck org update --name "Acme Inc" --output json`,
		RunE: uc.runOrgUpdateCmd,
	}

	uc.cmd.Flags().StringVar(&uc.name, "name", "", "New organization name (required)")
	uc.cmd.Flags().StringVar(&uc.output, "output", "", "Output format: json")

	return uc
}

func (uc *orgUpdateCmd) runOrgUpdateCmd(cmd *cobra.Command, args []string) error {
	if uc.name == "" {
		// An empty PUT succeeds and changes nothing, which would report success
		// for a command that did not do anything.
		return fmt.Errorf("--name is required")
	}
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	org, err := Config.GetAPIClient().UpdateOrganization(
		context.Background(), &hookdeck.OrganizationUpdateRequest{Name: &uc.name})
	if err != nil {
		return err
	}

	if uc.output == "json" {
		out, err := json.MarshalIndent(org, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	}

	fmt.Printf("✔ Organization renamed to %s\n", org.Name)
	return nil
}
