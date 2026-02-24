package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type eventRawBodyCmd struct {
	cmd *cobra.Command
}

func newEventRawBodyCmd() *eventRawBodyCmd {
	ec := &eventRawBodyCmd{}

	ec.cmd = &cobra.Command{
		Use:   "raw-body <event-id>",
		Args:  validators.ExactArgs(1),
		Short: "Get raw body of an event",
		Long: `Output the raw request body of an event by ID.

Examples:
  hookdeck gateway event raw-body evt_abc123`,
		RunE: ec.runEventRawBodyCmd,
	}

	return ec
}

func (ec *eventRawBodyCmd) runEventRawBodyCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	eventID := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	body, err := client.GetEventRawBody(ctx, eventID)
	if err != nil {
		return fmt.Errorf("failed to get event raw body: %w", err)
	}
	_, _ = os.Stdout.Write(body)
	return nil
}
