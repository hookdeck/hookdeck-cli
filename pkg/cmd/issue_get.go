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

type issueGetCmd struct {
	cmd    *cobra.Command
	output string
}

func newIssueGetCmd() *issueGetCmd {
	ic := &issueGetCmd{}

	ic.cmd = &cobra.Command{
		Use:   "get <issue-id>",
		Args:  validators.ExactArgs(1),
		Short: ShortGet(ResourceIssue),
		Long: `Get detailed information about a specific issue.

Examples:
  hookdeck gateway issue get iss_abc123
  hookdeck gateway issue get iss_abc123 --output json`,
		RunE: ic.runIssueGetCmd,
	}

	ic.cmd.Flags().StringVar(&ic.output, "output", "", "Output format (json)")

	return ic
}

func (ic *issueGetCmd) runIssueGetCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	issueID := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	iss, err := client.GetIssue(ctx, issueID)
	if err != nil {
		return fmt.Errorf("failed to get issue: %w", err)
	}

	if ic.output == "json" {
		jsonBytes, err := json.MarshalIndent(iss, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal issue to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	color := ansi.Color(os.Stdout)
	statusColor := issueStatusColor(color, string(iss.Status))
	fmt.Printf("\n%s\n", color.Bold(iss.ID))
	fmt.Printf("  Type:       %s\n", string(iss.Type))
	fmt.Printf("  Status:     %s\n", statusColor)
	fmt.Printf("  First seen: %s\n", iss.FirstSeenAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Last seen:  %s\n", iss.LastSeenAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Opened at:  %s\n", iss.OpenedAt.Format("2006-01-02 15:04:05"))
	if iss.DismissedAt != nil {
		fmt.Printf("  Dismissed:  %s\n", iss.DismissedAt.Format("2006-01-02 15:04:05"))
	}
	fmt.Printf("  Created:    %s\n", iss.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Updated:    %s\n", iss.UpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()

	return nil
}
