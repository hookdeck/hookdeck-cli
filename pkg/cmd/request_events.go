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

type requestEventsCmd struct {
	cmd    *cobra.Command
	limit  int
	next   string
	prev   string
	output string

	connectionID      string
	sourceID          string
	destinationID     string
	deliveryGroup     string
	status            string
	attempts          string
	responseStatus    string
	errorCode         string
	cliID             string
	issueID           string
	createdAfter      string
	createdBefore     string
	successfulAfter   string
	successfulBefore  string
	lastAttemptAfter  string
	lastAttemptBefore string
	headers           string
	body              string
	path              string
	parsedQuery       string
	orderBy           string
	dir               string
}

func newRequestEventsCmd() *requestEventsCmd {
	rc := &requestEventsCmd{}

	rc.cmd = &cobra.Command{
		Use:   "events <request-id>",
		Args:  validators.ExactArgs(1),
		Short: "List events for a request",
		Long: `List events (deliveries) created from a request.

Filters match ` + "`hookdeck gateway event list`" + `: this command queries the same event
collection, narrowed to one request.

Examples:
  hookdeck gateway request events req_abc123
  hookdeck gateway request events req_abc123 --status FAILED
  hookdeck gateway request events req_abc123 --destination-id des_abc123`,
		RunE: rc.runRequestEventsCmd,
	}

	// GET /requests/{id}/events declares the same query parameters as GET
	// /events, so these are the flags of `gateway event list`, spelled and
	// worded the same way - the two commands are learnt together, and a
	// filter that exists on one and not the other reads as unsupported.
	//
	// `event list --id` is the one flag deliberately left off: this command
	// already takes the request ID as its argument, so a second --id meaning
	// "event IDs" right beside it would be read as the request's.
	rc.cmd.Flags().StringVar(&rc.connectionID, "connection-id", "", "Filter by connection ID")
	rc.cmd.Flags().StringVar(&rc.sourceID, "source-id", "", "Filter by source ID")
	rc.cmd.Flags().StringVar(&rc.destinationID, "destination-id", "", "Filter by destination ID")
	rc.cmd.Flags().StringVar(&rc.deliveryGroup, "delivery-group", "", "Filter by delivery group")
	rc.cmd.Flags().StringVar(&rc.status, "status", "", eventStatusFlag.usage())
	rc.cmd.Flags().StringVar(&rc.attempts, "attempts", "", "Filter by number of attempts (integer or operators)")
	rc.cmd.Flags().StringVar(&rc.responseStatus, "response-status", "", "Filter by HTTP response status (e.g. 200, 500)")
	rc.cmd.Flags().StringVar(&rc.errorCode, "error-code", "", "Filter by error code")
	rc.cmd.Flags().StringVar(&rc.cliID, "cli-id", "", "Filter by CLI ID")
	rc.cmd.Flags().StringVar(&rc.issueID, "issue-id", "", "Filter by issue ID")
	rc.cmd.Flags().StringVar(&rc.createdAfter, "created-after", "", "Filter events created after (ISO date-time)")
	rc.cmd.Flags().StringVar(&rc.createdBefore, "created-before", "", "Filter events created before (ISO date-time)")
	rc.cmd.Flags().StringVar(&rc.successfulAfter, "successful-at-after", "", "Filter by successful_at after (ISO date-time)")
	rc.cmd.Flags().StringVar(&rc.successfulBefore, "successful-at-before", "", "Filter by successful_at before (ISO date-time)")
	rc.cmd.Flags().StringVar(&rc.lastAttemptAfter, "last-attempt-at-after", "", "Filter by last_attempt_at after (ISO date-time)")
	rc.cmd.Flags().StringVar(&rc.lastAttemptBefore, "last-attempt-at-before", "", "Filter by last_attempt_at before (ISO date-time)")
	rc.cmd.Flags().StringVar(&rc.headers, "headers", "", "Filter by headers (JSON string)")
	rc.cmd.Flags().StringVar(&rc.body, "body", "", "Filter by body (JSON string)")
	rc.cmd.Flags().StringVar(&rc.path, "path", "", "Filter by path")
	rc.cmd.Flags().StringVar(&rc.parsedQuery, "parsed-query", "", "Filter by parsed query (JSON string)")
	rc.cmd.Flags().StringVar(&rc.orderBy, "order-by", "", "Sort key (e.g. created_at)")
	rc.cmd.Flags().StringVar(&rc.dir, "dir", "", "Sort direction (asc, desc)")
	rc.cmd.Flags().IntVar(&rc.limit, "limit", 100, "Limit number of results")
	rc.cmd.Flags().StringVar(&rc.next, "next", "", "Pagination cursor for next page")
	rc.cmd.Flags().StringVar(&rc.prev, "prev", "", "Pagination cursor for previous page")
	rc.cmd.Flags().StringVar(&rc.output, "output", "", "Output format (json)")

	return rc
}

func (rc *requestEventsCmd) runRequestEventsCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	// This route shares the /events filter set, so it shares its enum and its
	// case sensitivity too.
	status, err := eventStatusFlag.canonical(rc.status)
	if err != nil {
		return err
	}

	requestID := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()
	params := map[string]string{"limit": strconv.Itoa(rc.limit)}
	if rc.next != "" {
		params["next"] = rc.next
	}
	if rc.prev != "" {
		params["prev"] = rc.prev
	}
	// Same parameter names and same date-bracket mapping as `event list`.
	if rc.connectionID != "" {
		params["webhook_id"] = rc.connectionID
	}
	if rc.sourceID != "" {
		params["source_id"] = rc.sourceID
	}
	if rc.destinationID != "" {
		params["destination_id"] = rc.destinationID
	}
	if rc.deliveryGroup != "" {
		params["delivery_group"] = rc.deliveryGroup
	}
	if status != "" {
		params["status"] = status
	}
	if rc.attempts != "" {
		params["attempts"] = rc.attempts
	}
	if rc.responseStatus != "" {
		params["response_status"] = rc.responseStatus
	}
	if rc.errorCode != "" {
		params["error_code"] = rc.errorCode
	}
	if rc.cliID != "" {
		params["cli_id"] = rc.cliID
	}
	if rc.issueID != "" {
		params["issue_id"] = rc.issueID
	}
	if rc.createdAfter != "" {
		params["created_at[gte]"] = rc.createdAfter
	}
	if rc.createdBefore != "" {
		params["created_at[lte]"] = rc.createdBefore
	}
	if rc.successfulAfter != "" {
		params["successful_at[gte]"] = rc.successfulAfter
	}
	if rc.successfulBefore != "" {
		params["successful_at[lte]"] = rc.successfulBefore
	}
	if rc.lastAttemptAfter != "" {
		params["last_attempt_at[gte]"] = rc.lastAttemptAfter
	}
	if rc.lastAttemptBefore != "" {
		params["last_attempt_at[lte]"] = rc.lastAttemptBefore
	}
	if rc.headers != "" {
		params["headers"] = rc.headers
	}
	if rc.body != "" {
		params["body"] = rc.body
	}
	if rc.path != "" {
		params["path"] = rc.path
	}
	if rc.parsedQuery != "" {
		params["parsed_query"] = rc.parsedQuery
	}
	if rc.orderBy != "" {
		params["order_by"] = rc.orderBy
	}
	if rc.dir != "" {
		params["dir"] = rc.dir
	}

	resp, err := client.GetRequestEvents(ctx, requestID, params)
	if err != nil {
		return fmt.Errorf("failed to list request events: %w", err)
	}

	if rc.output == "json" {
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
		fmt.Println("No events found for this request.")
		return nil
	}

	color := ansi.Color(os.Stdout)
	for _, e := range resp.Models {
		fmt.Printf("%s %s %s\n", color.Green(e.ID), e.Status, e.WebhookID)
	}
	return nil
}
