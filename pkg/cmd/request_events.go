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
}

func newRequestEventsCmd() *requestEventsCmd {
	rc := &requestEventsCmd{}

	rc.cmd = &cobra.Command{
		Use:   "events <request-id>",
		Args:  validators.ExactArgs(1),
		Short: "List events for a request",
		Long: `List events (deliveries) created from a request.

Examples:
  hookdeck gateway request events req_abc123`,
		RunE: rc.runRequestEventsCmd,
	}

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
