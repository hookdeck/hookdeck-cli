package cmd

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type issueCountCmd struct {
	cmd *cobra.Command

	issueType      string
	status         string
	issueTriggerID string
}

func newIssueCountCmd() *issueCountCmd {
	ic := &issueCountCmd{}

	ic.cmd = &cobra.Command{
		Use:   "count",
		Args:  validators.NoArgs,
		Short: "Count issues",
		Long: `Count issues matching optional filters.

Examples:
  hookdeck gateway issue count
  hookdeck gateway issue count --type delivery
  hookdeck gateway issue count --status OPENED`,
		RunE: ic.runIssueCountCmd,
	}

	ic.cmd.Flags().StringVar(&ic.issueType, "type", "", "Filter by issue type (delivery, transformation, backpressure)")
	ic.cmd.Flags().StringVar(&ic.status, "status", "", "Filter by status (OPENED, IGNORED, ACKNOWLEDGED, RESOLVED)")
	ic.cmd.Flags().StringVar(&ic.issueTriggerID, "issue-trigger-id", "", "Filter by issue trigger ID")

	return ic
}

func (ic *issueCountCmd) runIssueCountCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	client := Config.GetAPIClient()
	params := make(map[string]string)

	if ic.issueType != "" {
		params["type"] = ic.issueType
	}
	if ic.status != "" {
		params["status"] = ic.status
	}
	if ic.issueTriggerID != "" {
		params["issue_trigger_id"] = ic.issueTriggerID
	}

	resp, err := client.CountIssues(context.Background(), params)
	if err != nil {
		return fmt.Errorf("failed to count issues: %w", err)
	}

	fmt.Println(strconv.Itoa(resp.Count))
	return nil
}
