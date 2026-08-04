package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type eventCancelCmd struct {
	cmd *cobra.Command
}

func newEventCancelCmd() *eventCancelCmd {
	ec := &eventCancelCmd{}

	ec.cmd = &cobra.Command{
		Use:   "cancel <event-id>",
		Args:  validators.ExactArgs(1),
		Short: "Cancel an event",
		Long: `Cancel an event by ID. Cancelled events will not be retried.

Examples:
  hookdeck gateway event cancel evt_abc123`,
		RunE: ec.runEventCancelCmd,
	}

	return ec
}

func (ec *eventCancelCmd) runEventCancelCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	eventID := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	if err := client.CancelEvent(ctx, eventID); err != nil {
		return fmt.Errorf("failed to cancel event: %w", err)
	}
	fmt.Printf("Event %s cancelled.\n", eventID)
	return nil
}
