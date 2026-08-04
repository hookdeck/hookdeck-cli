package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type eventRetryCmd struct {
	cmd *cobra.Command
}

func newEventRetryCmd() *eventRetryCmd {
	ec := &eventRetryCmd{}

	ec.cmd = &cobra.Command{
		Use:   "retry <event-id>",
		Args:  validators.ExactArgs(1),
		Short: "Retry an event",
		Long: `Retry delivery for an event by ID.

Examples:
  hookdeck gateway event retry evt_abc123`,
		RunE: ec.runEventRetryCmd,
	}

	return ec
}

func (ec *eventRetryCmd) runEventRetryCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	eventID := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	if err := client.RetryEvent(ctx, eventID); err != nil {
		return fmt.Errorf("failed to retry event: %w", err)
	}
	fmt.Printf("Event %s retry requested.\n", eventID)
	return nil
}
