package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type requestListCmd struct {
	cmd *cobra.Command

	id              string
	sourceID        string
	status          string
	verified        string
	rejectionCause  string
	createdAfter    string
	createdBefore   string
	ingestedAfter   string
	ingestedBefore  string
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

func newRequestListCmd() *requestListCmd {
	rc := &requestListCmd{}

	rc.cmd = &cobra.Command{
		Use:   "list",
		Args:  validators.NoArgs,
		Short: ShortList(ResourceRequest),
		Long: `List requests (raw inbound webhooks). Filter by source ID.

Examples:
  hookdeck gateway request list
  hookdeck gateway request list --source-id src_abc123 --limit 20`,
		RunE: rc.runRequestListCmd,
	}

	rc.cmd.Flags().StringVar(&rc.id, "id", "", "Filter by request ID(s) (comma-separated)")
	rc.cmd.Flags().StringVar(&rc.sourceID, "source-id", "", "Filter by source ID")
	rc.cmd.Flags().StringVar(&rc.status, "status", "", "Filter by status")
	rc.cmd.Flags().StringVar(&rc.verified, "verified", "", "Filter by verified (true/false)")
	rc.cmd.Flags().StringVar(&rc.rejectionCause, "rejection-cause", "", "Filter by rejection cause")
	rc.cmd.Flags().StringVar(&rc.createdAfter, "created-after", "", "Filter requests created after (ISO date-time)")
	rc.cmd.Flags().StringVar(&rc.createdBefore, "created-before", "", "Filter requests created before (ISO date-time)")
	rc.cmd.Flags().StringVar(&rc.ingestedAfter, "ingested-at-after", "", "Filter by ingested_at after (ISO date-time)")
	rc.cmd.Flags().StringVar(&rc.ingestedBefore, "ingested-at-before", "", "Filter by ingested_at before (ISO date-time)")
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

func (rc *requestListCmd) runRequestListCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	client := Config.GetAPIClient()
	params := make(map[string]string)
	if rc.id != "" {
		params["id"] = rc.id
	}
	if rc.sourceID != "" {
		params["source_id"] = rc.sourceID
	}
	if rc.status != "" {
		params["status"] = rc.status
	}
	if rc.verified != "" {
		params["verified"] = rc.verified
	}
	if rc.rejectionCause != "" {
		params["rejection_cause"] = rc.rejectionCause
	}
	if rc.createdAfter != "" {
		params["created_at[gte]"] = rc.createdAfter
	}
	if rc.createdBefore != "" {
		params["created_at[lte]"] = rc.createdBefore
	}
	if rc.ingestedAfter != "" {
		params["ingested_at[gte]"] = rc.ingestedAfter
	}
	if rc.ingestedBefore != "" {
		params["ingested_at[lte]"] = rc.ingestedBefore
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
	params["limit"] = strconv.Itoa(rc.limit)
	if rc.next != "" {
		params["next"] = rc.next
	}
	if rc.prev != "" {
		params["prev"] = rc.prev
	}

	resp, err := client.ListRequests(context.Background(), params)
	if err != nil {
		return fmt.Errorf("failed to list requests: %w", err)
	}

	if rc.output == "json" {
		jsonBytes, err := marshalListResponseWithPagination(resp.Models, resp.Pagination)
		if err != nil {
			return fmt.Errorf("failed to marshal requests to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	if len(resp.Models) == 0 {
		fmt.Println("No requests found.")
		return nil
	}

	color := ansi.Color(os.Stdout)
	for _, r := range resp.Models {
		fmt.Printf("%s %s (events: %d)\n", color.Green(r.ID), r.SourceID, r.EventsCount)
	}

	// Display pagination info
	commandExample := "hookdeck gateway request list"
	printPaginationInfo(resp.Pagination, commandExample)

	return nil
}
