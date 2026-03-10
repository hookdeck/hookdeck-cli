package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type telemetryEnableCmd struct {
	cmd *cobra.Command
}

func newTelemetryEnableCmd() *telemetryEnableCmd {
	tc := &telemetryEnableCmd{}

	tc.cmd = &cobra.Command{
		Use:     "enable",
		Args:    validators.NoArgs,
		Short:   "Enable anonymous telemetry",
		Long:    "Enable anonymous telemetry collection. This persists the setting in your config file.",
		Example: "  $ hookdeck telemetry enable",
		RunE:    tc.runTelemetryEnableCmd,
	}

	return tc
}

func (tc *telemetryEnableCmd) runTelemetryEnableCmd(cmd *cobra.Command, args []string) error {
	if err := Config.SetTelemetryDisabled(false); err != nil {
		return fmt.Errorf("failed to enable telemetry: %w", err)
	}
	fmt.Println("Telemetry has been enabled.")
	return nil
}
