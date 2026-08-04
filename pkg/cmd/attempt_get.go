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

type attemptGetCmd struct {
	cmd    *cobra.Command
	output string
}

func newAttemptGetCmd() *attemptGetCmd {
	ac := &attemptGetCmd{}

	ac.cmd = &cobra.Command{
		Use:   "get <attempt-id>",
		Args:  validators.ExactArgs(1),
		Short: ShortGet(ResourceAttempt),
		Long: `Get detailed information about an attempt by ID.

Examples:
  hookdeck gateway attempt get atm_abc123`,
		RunE: ac.runAttemptGetCmd,
	}

	ac.cmd.Flags().StringVar(&ac.output, "output", "", "Output format (json)")

	return ac
}

func (ac *attemptGetCmd) runAttemptGetCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	attemptID := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	attempt, err := client.GetAttempt(ctx, attemptID)
	if err != nil {
		return fmt.Errorf("failed to get attempt: %w", err)
	}

	if ac.output == "json" {
		jsonBytes, err := json.MarshalIndent(attempt, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal attempt to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	color := ansi.Color(os.Stdout)
	fmt.Printf("\n%s\n", color.Green(attempt.ID))
	fmt.Printf("  Event ID:       %s\n", attempt.EventID)
	fmt.Printf("  Destination ID: %s\n", attempt.DestinationID)
	fmt.Printf("  Attempt #:      %d\n", attempt.AttemptNumber)
	fmt.Printf("  Status:         %s\n", attempt.Status)
	fmt.Printf("  Trigger:        %s\n", attempt.Trigger)
	if attempt.ResponseStatus != nil {
		fmt.Printf("  Response:       %d\n", *attempt.ResponseStatus)
	}
	if attempt.ErrorCode != nil {
		fmt.Printf("  Error code:     %s\n", *attempt.ErrorCode)
	}
	fmt.Printf("  Method:         %s\n", attempt.HTTPMethod)
	fmt.Printf("  URL:            %s\n", attempt.RequestedURL)
	fmt.Println()
	return nil
}
