package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/logrusorgru/aurora"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type issueListCmd struct {
	cmd *cobra.Command

	issueType      string
	status         string
	issueTriggerID string
	orderBy        string
	dir            string
	limit          int
	next           string
	prev           string
	output         string
}

func newIssueListCmd() *issueListCmd {
	ic := &issueListCmd{}

	ic.cmd = &cobra.Command{
		Use:   "list",
		Args:  validators.NoArgs,
		Short: ShortList(ResourceIssue),
		Long: `List issues or filter by type and status.

Examples:
  hookdeck gateway issue list
  hookdeck gateway issue list --type delivery
  hookdeck gateway issue list --status OPENED
  hookdeck gateway issue list --type delivery --status OPENED --limit 10`,
		RunE: ic.runIssueListCmd,
	}

	ic.cmd.Flags().StringVar(&ic.issueType, "type", "", "Filter by issue type (delivery, transformation, backpressure)")
	ic.cmd.Flags().StringVar(&ic.status, "status", "", "Filter by status (OPENED, IGNORED, ACKNOWLEDGED, RESOLVED)")
	ic.cmd.Flags().StringVar(&ic.issueTriggerID, "issue-trigger-id", "", "Filter by issue trigger ID")
	ic.cmd.Flags().StringVar(&ic.orderBy, "order-by", "", "Sort field (created_at, first_seen_at, last_seen_at, opened_at, status)")
	ic.cmd.Flags().StringVar(&ic.dir, "dir", "", "Sort direction (asc, desc)")
	ic.cmd.Flags().IntVar(&ic.limit, "limit", 100, "Limit number of results (max 250)")
	ic.cmd.Flags().StringVar(&ic.next, "next", "", "Pagination cursor for next page")
	ic.cmd.Flags().StringVar(&ic.prev, "prev", "", "Pagination cursor for previous page")
	ic.cmd.Flags().StringVar(&ic.output, "output", "", "Output format (json)")

	return ic
}

func (ic *issueListCmd) runIssueListCmd(cmd *cobra.Command, args []string) error {
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
	if ic.orderBy != "" {
		params["order_by"] = ic.orderBy
	}
	if ic.dir != "" {
		params["dir"] = ic.dir
	}
	if ic.next != "" {
		params["next"] = ic.next
	}
	if ic.prev != "" {
		params["prev"] = ic.prev
	}
	params["limit"] = strconv.Itoa(ic.limit)

	resp, err := client.ListIssues(context.Background(), params)
	if err != nil {
		return fmt.Errorf("failed to list issues: %w", err)
	}

	if ic.output == "json" {
		jsonBytes, err := marshalListResponseWithPagination(resp.Models, resp.Pagination)
		if err != nil {
			return fmt.Errorf("failed to marshal issues to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	if len(resp.Models) == 0 {
		fmt.Println("No issues found.")
		return nil
	}

	color := ansi.Color(os.Stdout)
	fmt.Printf("\nFound %d issue(s):\n\n", len(resp.Models))
	for _, iss := range resp.Models {
		statusColor := issueStatusColor(color, string(iss.Status))
		fmt.Printf("%s  %s  %s\n", color.Bold(iss.ID), statusColor, string(iss.Type))
		fmt.Printf("  First seen: %s\n", iss.FirstSeenAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Last seen:  %s\n", iss.LastSeenAt.Format("2006-01-02 15:04:05"))
		fmt.Println()
	}

	commandExample := "hookdeck gateway issue list"
	printPaginationInfo(resp.Pagination, commandExample)

	return nil
}

func issueStatusColor(color aurora.Aurora, status string) string {
	switch status {
	case "OPENED":
		return color.Sprintf(color.Red(status))
	case "ACKNOWLEDGED":
		return color.Sprintf(color.Yellow(status))
	case "RESOLVED":
		return color.Sprintf(color.Green(status))
	default:
		return status
	}
}
