package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type outpostStatusCmd struct {
	cmd *cobra.Command

	output string
}

func newOutpostStatusCmd() *outpostStatusCmd {
	sc := &outpostStatusCmd{}

	sc.cmd = &cobra.Command{
		Use:   "status",
		Args:  validators.NoArgs,
		Short: ShortBeta("Show the Outpost deployment status"),
		Long: LongBeta(`Show the status of this project's Outpost deployment.

Worth checking first when something is not behaving: configuration changes take
a short while to reach the deployment, and the status reports when it is still
being applied.`),
		RunE: sc.run,
		Example: `  # Check deployment status
  hookdeck outpost status`,
	}

	sc.cmd.Flags().StringVar(&sc.output, "output", "", "Output format (json)")

	return sc
}

func (sc *outpostStatusCmd) run(cmd *cobra.Command, args []string) error {
	client := Config.GetOutpostAPIClient()

	status, err := client.GetOutpostStatus(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	if sc.output == "json" {
		return printJSONIndented(status)
	}

	color := ansi.Color(os.Stdout)
	fmt.Println()
	if status.Status == "HEALTHY" {
		fmt.Printf("Status: %s\n", color.Green(status.Status))
	} else {
		fmt.Printf("Status: %s\n", color.Red(status.Status))
	}
	if status.Version != "" {
		fmt.Printf("Version: %s\n", status.Version)
	}
	if status.PortalHostname != "" {
		fmt.Printf("Portal hostname: %s\n", status.PortalHostname)
	}
	fmt.Println()

	return nil
}
