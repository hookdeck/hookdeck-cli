package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

type telemetryCmd struct {
	cmd *cobra.Command
}

func newTelemetryCmd() *telemetryCmd {
	tc := &telemetryCmd{}

	tc.cmd = &cobra.Command{
		Use:   "telemetry [enable|disable]",
		Short: "Manage anonymous telemetry settings",
		Long:  "Enable or disable anonymous telemetry that helps improve the Hookdeck CLI. Telemetry is enabled by default. You can also set the HOOKDECK_CLI_TELEMETRY_OPTOUT environment variable to 1 or true.",
		Example: `  $ hookdeck telemetry disable
  $ hookdeck telemetry enable`,
		Args: cobra.ExactArgs(1),
		ValidArgs: []string{"enable", "disable"},
		RunE: tc.runTelemetryCmd,
	}

	return tc
}

func (tc *telemetryCmd) runTelemetryCmd(cmd *cobra.Command, args []string) error {
	switch args[0] {
	case "disable":
		if err := Config.SetTelemetryDisabled(true); err != nil {
			return fmt.Errorf("failed to disable telemetry: %w", err)
		}
		fmt.Println("Telemetry has been disabled.")
	case "enable":
		if err := Config.SetTelemetryDisabled(false); err != nil {
			return fmt.Errorf("failed to enable telemetry: %w", err)
		}
		fmt.Println("Telemetry has been enabled.")
	default:
		return fmt.Errorf("invalid argument %q: must be \"enable\" or \"disable\"", args[0])
	}
	return nil
}
