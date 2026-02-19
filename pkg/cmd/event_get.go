package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type eventGetCmd struct {
	cmd    *cobra.Command
	output string
}

func newEventGetCmd() *eventGetCmd {
	ec := &eventGetCmd{}

	ec.cmd = &cobra.Command{
		Use:   "get <event-id>",
		Args:  validators.ExactArgs(1),
		Short: ShortGet(ResourceEvent),
		Long: `Get detailed information about an event by ID.

Examples:
  hookdeck gateway event get evt_abc123`,
		RunE: ec.runEventGetCmd,
	}

	ec.cmd.Flags().StringVar(&ec.output, "output", "", "Output format (json)")

	return ec
}

func (ec *eventGetCmd) runEventGetCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	eventID := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	event, err := client.GetEvent(ctx, eventID, nil)
	if err != nil {
		return fmt.Errorf("failed to get event: %w", err)
	}

	if ec.output == "json" {
		jsonBytes, err := json.MarshalIndent(event, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal event to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	color := ansi.Color(os.Stdout)
	fmt.Printf("\n%s\n", color.Green(event.ID))
	fmt.Printf("  Status:         %s\n", event.Status)
	fmt.Printf("  Connection ID:  %s\n", event.WebhookID)
	fmt.Printf("  Source ID:     %s\n", event.SourceID)
	fmt.Printf("  Destination ID: %s\n", event.DestinationID)
	fmt.Printf("  Request ID:    %s\n", event.RequestID)
	fmt.Printf("  Attempts:      %d\n", event.Attempts)
	if event.ResponseStatus != nil {
		fmt.Printf("  Response:       %d\n", *event.ResponseStatus)
	}
	if event.ErrorCode != nil {
		fmt.Printf("  Error code:     %s\n", *event.ErrorCode)
	}
	fmt.Printf("  Created:       %s\n", event.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()
	return nil
}
