package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type telemetryDisableCmd struct {
	cmd *cobra.Command
}

func newTelemetryDisableCmd() *telemetryDisableCmd {
	tc := &telemetryDisableCmd{}

	tc.cmd = &cobra.Command{
		Use:     "disable",
		Args:    validators.NoArgs,
		Short:   "Disable anonymous telemetry",
		Long:    "Disable anonymous telemetry collection. This persists the setting in your config file.",
		Example: "  $ hookdeck telemetry disable",
		RunE:    tc.runTelemetryDisableCmd,
	}

	return tc
}

func (tc *telemetryDisableCmd) runTelemetryDisableCmd(cmd *cobra.Command, args []string) error {
	if err := Config.SetTelemetryDisabled(true); err != nil {
		return fmt.Errorf("failed to disable telemetry: %w", err)
	}
	fmt.Println("Telemetry has been disabled.")
	return nil
}
