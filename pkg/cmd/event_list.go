package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type eventListCmd struct {
	cmd *cobra.Command

	id              string
	connectionID    string
	sourceID        string
	destinationID   string
	status          string
	attempts        string
	responseStatus  string
	errorCode       string
	cliID           string
	issueID         string
	createdAfter    string
	createdBefore   string
	successfulAfter string
	successfulBefore string
	lastAttemptAfter  string
	lastAttemptBefore string
	headers         string
	body            string
	path            string
	parsedQuery     string
	orderBy         string
	dir             string
	limit           int
	next            string
	prev            string
	output          string
}

func newEventListCmd() *eventListCmd {
	ec := &eventListCmd{}

	ec.cmd = &cobra.Command{
		Use:   "list",
		Args:  validators.NoArgs,
		Short: ShortList(ResourceEvent),
		Long: `List events (processed webhook deliveries). Filter by connection ID, source, destination, or status.

Examples:
  hookdeck gateway event list
  hookdeck gateway event list --connection-id web_abc123
  hookdeck gateway event list --status FAILED --limit 20`,
		RunE: ec.runEventListCmd,
	}

	ec.cmd.Flags().StringVar(&ec.id, "id", "", "Filter by event ID(s) (comma-separated)")
	ec.cmd.Flags().StringVar(&ec.connectionID, "connection-id", "", "Filter by connection ID")
	ec.cmd.Flags().StringVar(&ec.sourceID, "source-id", "", "Filter by source ID")
	ec.cmd.Flags().StringVar(&ec.destinationID, "destination-id", "", "Filter by destination ID")
	ec.cmd.Flags().StringVar(&ec.status, "status", "", "Filter by status (SCHEDULED, QUEUED, HOLD, SUCCESSFUL, FAILED, CANCELLED)")
	ec.cmd.Flags().StringVar(&ec.attempts, "attempts", "", "Filter by number of attempts (integer or operators)")
	ec.cmd.Flags().StringVar(&ec.responseStatus, "response-status", "", "Filter by HTTP response status (e.g. 200, 500)")
	ec.cmd.Flags().StringVar(&ec.errorCode, "error-code", "", "Filter by error code")
	ec.cmd.Flags().StringVar(&ec.cliID, "cli-id", "", "Filter by CLI ID")
	ec.cmd.Flags().StringVar(&ec.issueID, "issue-id", "", "Filter by issue ID")
	ec.cmd.Flags().StringVar(&ec.createdAfter, "created-after", "", "Filter events created after (ISO date-time)")
	ec.cmd.Flags().StringVar(&ec.createdBefore, "created-before", "", "Filter events created before (ISO date-time)")
	ec.cmd.Flags().StringVar(&ec.successfulAfter, "successful-at-after", "", "Filter by successful_at after (ISO date-time)")
	ec.cmd.Flags().StringVar(&ec.successfulBefore, "successful-at-before", "", "Filter by successful_at before (ISO date-time)")
	ec.cmd.Flags().StringVar(&ec.lastAttemptAfter, "last-attempt-at-after", "", "Filter by last_attempt_at after (ISO date-time)")
	ec.cmd.Flags().StringVar(&ec.lastAttemptBefore, "last-attempt-at-before", "", "Filter by last_attempt_at before (ISO date-time)")
	ec.cmd.Flags().StringVar(&ec.headers, "headers", "", "Filter by headers (JSON string)")
	ec.cmd.Flags().StringVar(&ec.body, "body", "", "Filter by body (JSON string)")
	ec.cmd.Flags().StringVar(&ec.path, "path", "", "Filter by path")
	ec.cmd.Flags().StringVar(&ec.parsedQuery, "parsed-query", "", "Filter by parsed query (JSON string)")
	ec.cmd.Flags().StringVar(&ec.orderBy, "order-by", "", "Sort key (e.g. created_at)")
	ec.cmd.Flags().StringVar(&ec.dir, "dir", "", "Sort direction (asc, desc)")
	ec.cmd.Flags().IntVar(&ec.limit, "limit", 100, "Limit number of results")
	ec.cmd.Flags().StringVar(&ec.next, "next", "", "Pagination cursor for next page")
	ec.cmd.Flags().StringVar(&ec.prev, "prev", "", "Pagination cursor for previous page")
	ec.cmd.Flags().StringVar(&ec.output, "output", "", "Output format (json)")

	return ec
}

func (ec *eventListCmd) runEventListCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	client := Config.GetAPIClient()
	params := make(map[string]string)
	if ec.id != "" {
		params["id"] = ec.id
	}
	if ec.connectionID != "" {
		params["webhook_id"] = ec.connectionID
	}
	if ec.sourceID != "" {
		params["source_id"] = ec.sourceID
	}
	if ec.destinationID != "" {
		params["destination_id"] = ec.destinationID
	}
	if ec.status != "" {
		params["status"] = ec.status
	}
	if ec.attempts != "" {
		params["attempts"] = ec.attempts
	}
	if ec.responseStatus != "" {
		params["response_status"] = ec.responseStatus
	}
	if ec.errorCode != "" {
		params["error_code"] = ec.errorCode
	}
	if ec.cliID != "" {
		params["cli_id"] = ec.cliID
	}
	if ec.issueID != "" {
		params["issue_id"] = ec.issueID
	}
	if ec.createdAfter != "" {
		params["created_at[gte]"] = ec.createdAfter
	}
	if ec.createdBefore != "" {
		params["created_at[lte]"] = ec.createdBefore
	}
	if ec.successfulAfter != "" {
		params["successful_at[gte]"] = ec.successfulAfter
	}
	if ec.successfulBefore != "" {
		params["successful_at[lte]"] = ec.successfulBefore
	}
	if ec.lastAttemptAfter != "" {
		params["last_attempt_at[gte]"] = ec.lastAttemptAfter
	}
	if ec.lastAttemptBefore != "" {
		params["last_attempt_at[lte]"] = ec.lastAttemptBefore
	}
	if ec.headers != "" {
		params["headers"] = ec.headers
	}
	if ec.body != "" {
		params["body"] = ec.body
	}
	if ec.path != "" {
		params["path"] = ec.path
	}
	if ec.parsedQuery != "" {
		params["parsed_query"] = ec.parsedQuery
	}
	if ec.orderBy != "" {
		params["order_by"] = ec.orderBy
	}
	if ec.dir != "" {
		params["dir"] = ec.dir
	}
	params["limit"] = strconv.Itoa(ec.limit)
	if ec.next != "" {
		params["next"] = ec.next
	}
	if ec.prev != "" {
		params["prev"] = ec.prev
	}

	resp, err := client.ListEvents(context.Background(), params)
	if err != nil {
		return fmt.Errorf("failed to list events: %w", err)
	}

	if ec.output == "json" {
		if len(resp.Models) == 0 {
			fmt.Println("[]")
			return nil
		}
		jsonBytes, err := json.MarshalIndent(resp.Models, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal events to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	if len(resp.Models) == 0 {
		fmt.Println("No events found.")
		return nil
	}

	color := ansi.Color(os.Stdout)
	for _, e := range resp.Models {
		fmt.Printf("%s %s %s\n", color.Green(e.ID), e.Status, e.WebhookID)
	}
	return nil
}
