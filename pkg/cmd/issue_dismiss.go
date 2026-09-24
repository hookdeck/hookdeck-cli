package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type issueDismissCmd struct {
	cmd    *cobra.Command
	force  bool
	output string
}

func newIssueDismissCmd() *issueDismissCmd {
	ic := &issueDismissCmd{}

	ic.cmd = &cobra.Command{
		Use:   "dismiss <issue-id>",
		Args:  validators.ExactArgs(1),
		Short: "Dismiss an issue",
		Long: `Dismiss an issue. This sends a DELETE request to the API.

Examples:
  hookdeck gateway issue dismiss iss_abc123
  hookdeck gateway issue dismiss iss_abc123 --force`,
		PreRunE: ic.validateFlags,
		RunE:    ic.runIssueDismissCmd,
	}

	ic.cmd.Flags().BoolVar(&ic.force, "force", false, "Dismiss without confirmation")
	ic.cmd.Flags().StringVar(&ic.output, "output", "", "Output format (json)")

	return ic
}

func (ic *issueDismissCmd) validateFlags(cmd *cobra.Command, args []string) error {
	return Config.Profile.ValidateAPIKey()
}

func (ic *issueDismissCmd) runIssueDismissCmd(cmd *cobra.Command, args []string) error {
	issueID := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	if !ic.force {
		proceed, err := confirmDestructiveAction(
			fmt.Sprintf("Are you sure you want to dismiss issue %s?", issueID),
			"Dismiss cancelled.",
			"force",
		)
		if err != nil {
			return err
		}
		if !proceed {
			return nil
		}
	}

	iss, err := client.DismissIssue(ctx, issueID)
	if err != nil {
		return fmt.Errorf("failed to dismiss issue: %w", err)
	}

	if ic.output == "json" {
		jsonBytes, err := json.MarshalIndent(iss, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal issue to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	// Report dismissed_at rather than just the id. Dismissing does not change
	// status, so "Issue dismissed" followed by a get still showing OPENED reads
	// like a failure; naming the field that did change says what happened.
	if iss != nil && iss.DismissedAt != nil {
		fmt.Printf(SuccessCheck+" Issue dismissed: %s (dismissed_at %s; status is unchanged at %s)\n",
			issueID, iss.DismissedAt.Format(time.RFC3339), iss.Status)
		return nil
	}
	fmt.Printf(SuccessCheck+" Issue dismissed: %s\n", issueID)
	return nil
}
