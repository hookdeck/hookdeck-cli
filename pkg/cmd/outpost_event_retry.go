package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type outpostEventRetryCmd struct {
	cmd *cobra.Command

	eventID       string
	destinationID string
	output        string
}

func newOutpostEventRetryCmd() *outpostEventRetryCmd {
	ec := &outpostEventRetryCmd{}

	ec.cmd = &cobra.Command{
		Use:   "retry",
		Args:  validators.NoArgs,
		Short: ShortBeta("Retry delivering an event to a destination"),
		Long: LongBeta(`Deliver an event to a destination again.

The retry is queued rather than performed inline, so a successful response means
it was accepted, not that it has been delivered. Use 'hookdeck outpost attempt
list' to see the outcome.

The destination must be enabled and must subscribe to the event's topic.`),
		PreRunE: ec.validateFlags,
		RunE:    ec.run,
		Example: `  # Retry one delivery
  hookdeck outpost event retry --event-id evt_abc123 --destination-id des_abc123`,
	}

	ec.cmd.Flags().StringVar(&ec.eventID, "event-id", "", "The event to retry (required)")
	ec.cmd.Flags().StringVar(&ec.destinationID, "destination-id", "", "The destination to deliver to (required)")
	ec.cmd.Flags().StringVar(&ec.output, "output", "", "Output format (json)")
	ec.cmd.MarkFlagRequired("event-id")
	ec.cmd.MarkFlagRequired("destination-id")

	return ec
}

func (ec *outpostEventRetryCmd) validateFlags(cmd *cobra.Command, args []string) error {
	return rejectEmptyFlags(cmd)
}

func (ec *outpostEventRetryCmd) run(cmd *cobra.Command, args []string) error {
	client := Config.GetOutpostAPIClient()

	resp, err := client.RetryOutpostEvent(context.Background(), &hookdeck.OutpostRetryRequest{
		EventID:       ec.eventID,
		DestinationID: ec.destinationID,
	})
	if err != nil {
		return fmt.Errorf("failed to retry event: %w", err)
	}

	// The endpoint answers 200 with {"success": false} when it declines the
	// retry, so the flag is the only thing that says whether anything was
	// queued. Without this the command printed a tick and exited 0 either way.
	if ec.output == "json" {
		if err := printJSONIndented(resp); err != nil {
			return err
		}
		if !resp.Success {
			return fmt.Errorf("the API did not accept the retry")
		}
		return nil
	}

	if !resp.Success {
		return fmt.Errorf("the API did not accept the retry for event %s to destination %s",
			ec.eventID, ec.destinationID)
	}

	fmt.Printf("%s Retry accepted for event %s to destination %s\n", SuccessCheck, ec.eventID, ec.destinationID)
	fmt.Println("\nRun 'hookdeck outpost attempt list --event-id " + ec.eventID + "' to see the result.")

	return nil
}
