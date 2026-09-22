package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type orgGetCmd struct {
	cmd    *cobra.Command
	output string
}

func newOrgGetCmd() *orgGetCmd {
	gc := &orgGetCmd{}

	gc.cmd = &cobra.Command{
		Use:   "get",
		Args:  validators.NoArgs,
		Short: "Show the current organization",
		Long:  `Show the Hookdeck organization your credentials belong to.`,
		Example: `$ hookdeck org get
$ hookdeck org get --output json`,
		RunE: gc.runOrgGetCmd,
	}

	gc.cmd.Flags().StringVar(&gc.output, "output", "", "Output format: json")

	return gc
}

func (gc *orgGetCmd) runOrgGetCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	org, err := Config.GetAPIClient().GetOrganization(context.Background())
	if err != nil {
		return err
	}

	if gc.output == "json" {
		out, err := json.MarshalIndent(org, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	}

	fmt.Printf("Organization: %s\n", org.Name)
	fmt.Printf("ID:           %s\n", org.ID)
	return nil
}
