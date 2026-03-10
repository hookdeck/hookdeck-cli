package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type telemetryCmd struct {
	cmd *cobra.Command
}

func newTelemetryCmd() *telemetryCmd {
	tc := &telemetryCmd{}

	tc.cmd = &cobra.Command{
		Use:   "telemetry",
		Args:  validators.NoArgs,
		Short: "Manage anonymous telemetry settings",
		Long:  "Enable or disable anonymous telemetry that helps improve the Hookdeck CLI. Telemetry is enabled by default. You can also set the HOOKDECK_CLI_TELEMETRY_OPTOUT environment variable to 1 or true.",
	}

	tc.cmd.AddCommand(newTelemetryEnableCmd().cmd)
	tc.cmd.AddCommand(newTelemetryDisableCmd().cmd)

	return tc
}
