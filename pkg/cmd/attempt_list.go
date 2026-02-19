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

type attemptListCmd struct {
	cmd    *cobra.Command
	eventID string
	orderBy string
	dir     string
	limit   int
	next    string
	prev    string
	output  string
}

func newAttemptListCmd() *attemptListCmd {
	ac := &attemptListCmd{}

	ac.cmd = &cobra.Command{
		Use:   "list",
		Args:  validators.NoArgs,
		Short: ShortList(ResourceAttempt),
		Long: `List attempts for an event. Requires --event-id.

Examples:
  hookdeck gateway attempt list --event-id evt_abc123`,
		RunE: ac.runAttemptListCmd,
	}

	ac.cmd.Flags().StringVar(&ac.eventID, "event-id", "", "Filter by event ID (required)")
	ac.cmd.Flags().StringVar(&ac.orderBy, "order-by", "", "Sort key")
	ac.cmd.Flags().StringVar(&ac.dir, "dir", "", "Sort direction (asc, desc)")
	ac.cmd.Flags().IntVar(&ac.limit, "limit", 100, "Limit number of results")
	ac.cmd.Flags().StringVar(&ac.next, "next", "", "Pagination cursor for next page")
	ac.cmd.Flags().StringVar(&ac.prev, "prev", "", "Pagination cursor for previous page")
	ac.cmd.Flags().StringVar(&ac.output, "output", "", "Output format (json)")

	return ac
}

func (ac *attemptListCmd) runAttemptListCmd(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	if ac.eventID == "" {
		return fmt.Errorf("--event-id is required")
	}

	client := Config.GetAPIClient()
	params := map[string]string{
		"event_id": ac.eventID,
		"limit":    strconv.Itoa(ac.limit),
	}
	if ac.orderBy != "" {
		params["order_by"] = ac.orderBy
	}
	if ac.dir != "" {
		params["dir"] = ac.dir
	}
	if ac.next != "" {
		params["next"] = ac.next
	}
	if ac.prev != "" {
		params["prev"] = ac.prev
	}

	resp, err := client.ListAttempts(context.Background(), params)
	if err != nil {
		return fmt.Errorf("failed to list attempts: %w", err)
	}

	if ac.output == "json" {
		if len(resp.Models) == 0 {
			fmt.Println("[]")
			return nil
		}
		jsonBytes, err := json.MarshalIndent(resp.Models, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal attempts to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	if len(resp.Models) == 0 {
		fmt.Println("No attempts found.")
		return nil
	}

	color := ansi.Color(os.Stdout)
	for _, a := range resp.Models {
		status := ""
		if a.ResponseStatus != nil {
			status = fmt.Sprintf(" %d", *a.ResponseStatus)
		}
		fmt.Printf("%s #%d%s %s\n", color.Green(a.ID), a.AttemptNumber, status, a.Status)
	}
	return nil
}
