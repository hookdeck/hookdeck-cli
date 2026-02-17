package cmd

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type transformationCountCmd struct {
	cmd   *cobra.Command
	name  string
	output string
}

func newTransformationCountCmd() *transformationCountCmd {
	tc := &transformationCountCmd{}

	tc.cmd = &cobra.Command{
		Use:   "count",
		Args:  validators.NoArgs,
		Short: "Count transformations",
		Long: `Count transformations matching optional filters.

Examples:
  hookdeck gateway transformation count
  hookdeck gateway transformation count --name my-transform`,
		RunE: tc.runTransformationCountCmd,
	}

	tc.cmd.Flags().StringVar(&tc.name, "name", "", "Filter by transformation name")
	tc.cmd.Flags().StringVar(&tc.output, "output", "", "Output format (json)")

	return tc
}

func (tc *transformationCountCmd) runTransformationCountCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	client := Config.GetAPIClient()
	params := make(map[string]string)
	if tc.name != "" {
		params["name"] = tc.name
	}

	resp, err := client.CountTransformations(context.Background(), params)
	if err != nil {
		return fmt.Errorf("failed to count transformations: %w", err)
	}

	if tc.output == "json" {
		fmt.Printf(`{"count":%d}`+"\n", resp.Count)
		return nil
	}

	fmt.Println(strconv.Itoa(resp.Count))
	return nil
}
