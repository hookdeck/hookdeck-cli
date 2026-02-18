package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type eventMuteCmd struct {
	cmd *cobra.Command
}

func newEventMuteCmd() *eventMuteCmd {
	ec := &eventMuteCmd{}

	ec.cmd = &cobra.Command{
		Use:   "mute <event-id>",
		Args:  validators.ExactArgs(1),
		Short: "Mute an event",
		Long: `Mute an event by ID. Muted events will not trigger alerts or retries.

Examples:
  hookdeck gateway event mute evt_abc123`,
		RunE: ec.runEventMuteCmd,
	}

	return ec
}

func (ec *eventMuteCmd) runEventMuteCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	eventID := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	if err := client.MuteEvent(ctx, eventID); err != nil {
		return fmt.Errorf("failed to mute event: %w", err)
	}
	fmt.Printf("Event %s muted.\n", eventID)
	return nil
}
