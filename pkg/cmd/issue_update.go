package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type issueUpdateCmd struct {
	cmd *cobra.Command

	status string
	output string
}

func newIssueUpdateCmd() *issueUpdateCmd {
	ic := &issueUpdateCmd{}

	ic.cmd = &cobra.Command{
		Use:   "update <issue-id>",
		Args:  validators.ExactArgs(1),
		Short: ShortUpdate(ResourceIssue),
		Long: LongUpdateIntro(ResourceIssue) + `

The --status flag is required. Valid statuses: OPENED, IGNORED, ACKNOWLEDGED, RESOLVED.

Examples:
  hookdeck gateway issue update iss_abc123 --status ACKNOWLEDGED
  hookdeck gateway issue update iss_abc123 --status RESOLVED`,
		PreRunE: ic.validateFlags,
		RunE:    ic.runIssueUpdateCmd,
	}

	ic.cmd.Flags().StringVar(&ic.status, "status", "", "New issue status (OPENED, IGNORED, ACKNOWLEDGED, RESOLVED) [required]")
	ic.cmd.MarkFlagRequired("status")
	ic.cmd.Flags().StringVar(&ic.output, "output", "", "Output format (json)")

	return ic
}

var validIssueStatuses = map[string]bool{
	"OPENED":       true,
	"IGNORED":      true,
	"ACKNOWLEDGED": true,
	"RESOLVED":     true,
}

func (ic *issueUpdateCmd) validateFlags(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	upper := strings.ToUpper(ic.status)
	if !validIssueStatuses[upper] {
		return fmt.Errorf("invalid status %q; must be one of: OPENED, IGNORED, ACKNOWLEDGED, RESOLVED", ic.status)
	}
	ic.status = upper
	return nil
}

func (ic *issueUpdateCmd) runIssueUpdateCmd(cmd *cobra.Command, args []string) error {
	issueID := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	req := &hookdeck.IssueUpdateRequest{
		Status: hookdeck.IssueStatus(ic.status),
	}

	iss, err := client.UpdateIssue(ctx, issueID, req)
	if err != nil {
		return fmt.Errorf("failed to update issue: %w", err)
	}

	if ic.output == "json" {
		jsonBytes, err := json.MarshalIndent(iss, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal issue to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	fmt.Printf(SuccessCheck+" Issue %s updated to %s\n", iss.ID, iss.Status)
	return nil
}
