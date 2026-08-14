package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type outpostAttemptCmd struct {
	cmd *cobra.Command
}

func newOutpostAttemptCmd() *outpostAttemptCmd {
	ac := &outpostAttemptCmd{}

	ac.cmd = &cobra.Command{
		Use:     "attempt",
		Aliases: []string{"attempts"},
		Args:    validators.NoArgs,
		Short:   ShortBeta("Inspect delivery attempts"),
		Long: LongBeta(`Inspect delivery attempts — each try at delivering an event to a destination.

This is where to look when a destination is not receiving events: attempts carry
the response code and body the destination returned.`),
	}

	ac.cmd.AddCommand(newOutpostAttemptListCmd().cmd)
	ac.cmd.AddCommand(newOutpostAttemptGetCmd().cmd)

	return ac
}

// printOutpostAttempt renders one attempt, colouring the outcome so a failure is
// obvious in a long list.
func printOutpostAttempt(attempt *hookdeck.OutpostAttempt) {
	color := ansi.Color(os.Stdout)

	fmt.Printf("%s\n", color.Green(attempt.ID))
	if attempt.Succeeded() {
		fmt.Printf("  Status: %s\n", color.Green(attempt.Status))
	} else {
		fmt.Printf("  Status: %s\n", color.Red(attempt.Status))
	}
	if attempt.Code != "" {
		fmt.Printf("  Code: %s\n", attempt.Code)
	}
	fmt.Printf("  Event: %s\n", attempt.EventID)
	fmt.Printf("  Destination: %s\n", attempt.DestinationID)
	fmt.Printf("  Attempt: %d", attempt.AttemptNumber)
	if attempt.Manual {
		fmt.Printf(" (manual retry)")
	}
	fmt.Println()
	fmt.Printf("  Time: %s\n", attempt.Time.Format("2006-01-02 15:04:05"))
}

type outpostAttemptListCmd struct {
	cmd *cobra.Command

	tenantID        string
	destinationID   string
	eventIDs        string
	destinationType string
	status          string
	topics          string
	timeAfter       string
	timeBefore      string
	include         string
	limit           int
	orderBy         string
	dir             string
	next            string
	prev            string
	output          string
}

func newOutpostAttemptListCmd() *outpostAttemptListCmd {
	ac := &outpostAttemptListCmd{}

	ac.cmd = &cobra.Command{
		Use:   "list",
		Args:  validators.NoArgs,
		Short: ShortList(ResourceAttempt),
		Long: `List delivery attempts, most recent first.

Passing both --tenant-id and --destination-id narrows to that destination
specifically; the filters and results are otherwise the same.`,
		PreRunE: ac.validateFlags,
		RunE:    ac.run,
		Example: `  # Recent failures
  hookdeck outpost attempt list --status failed --limit 20

  # Every attempt for one event
  hookdeck outpost attempt list --event-id evt_abc123

  # Include the response body the destination returned
  hookdeck outpost attempt list --event-id evt_abc123 --include response_data --output json`,
	}

	ac.cmd.Flags().StringVar(&ac.tenantID, "tenant-id", "", "Filter by tenant ID(s), comma-separated")
	ac.cmd.Flags().StringVar(&ac.destinationID, "destination-id", "", "Filter by destination ID(s), comma-separated")
	ac.cmd.Flags().StringVar(&ac.eventIDs, "event-id", "", "Filter by event ID(s), comma-separated")
	ac.cmd.Flags().StringVar(&ac.destinationType, "destination-type", "", "Filter by destination type(s), comma-separated")
	ac.cmd.Flags().StringVar(&ac.status, "status", "", "Filter by status (success, failed)")
	ac.cmd.Flags().StringVar(&ac.topics, "topic", "", "Filter by topic(s), comma-separated")
	ac.cmd.Flags().StringVar(&ac.timeAfter, "time-after", "", "Only attempts at or after this ISO 8601 datetime")
	ac.cmd.Flags().StringVar(&ac.timeBefore, "time-before", "", "Only attempts at or before this ISO 8601 datetime")
	ac.cmd.Flags().StringVar(&ac.include, "include", "", "Include related data, comma-separated (event, event.data, response_data, destination)")
	ac.cmd.Flags().IntVar(&ac.limit, "limit", 0, "Limit number of results")
	ac.cmd.Flags().StringVar(&ac.orderBy, "order-by", "", "Field to sort by")
	ac.cmd.Flags().StringVar(&ac.dir, "dir", "", "Sort direction (asc, desc)")
	ac.cmd.Flags().StringVar(&ac.next, "next", "", "Next page cursor")
	ac.cmd.Flags().StringVar(&ac.prev, "prev", "", "Previous page cursor")
	ac.cmd.Flags().StringVar(&ac.output, "output", "", "Output format (json)")

	return ac
}

func (ac *outpostAttemptListCmd) validateFlags(cmd *cobra.Command, args []string) error {
	return rejectEmptyFlags(cmd)
}

func (ac *outpostAttemptListCmd) run(cmd *cobra.Command, args []string) error {
	client := Config.GetOutpostAPIClient()

	params := hookdeck.OutpostAttemptListParams{
		EventIDs:        splitCommaList(ac.eventIDs),
		DestinationType: splitCommaList(ac.destinationType),
		Topics:          splitCommaList(ac.topics),
		Status:          ac.status,
		TimeAfter:       ac.timeAfter,
		TimeBefore:      ac.timeBefore,
		Include:         splitCommaList(ac.include),
		Limit:           ac.limit,
		OrderBy:         ac.orderBy,
		Dir:             ac.dir,
		Next:            ac.next,
		Prev:            ac.prev,
	}

	// A single tenant and destination can use the tenant-scoped route; anything
	// else has to go through the filters on the general one.
	tenants := splitCommaList(ac.tenantID)
	destinations := splitCommaList(ac.destinationID)
	if len(tenants) == 1 && len(destinations) == 1 {
		params.TenantID, params.DestinationID = tenants[0], destinations[0]
	} else {
		params.TenantIDs, params.DestinationIDs = tenants, destinations
	}

	resp, err := client.ListOutpostAttempts(context.Background(), params)
	if err != nil {
		return fmt.Errorf("failed to list attempts: %w", err)
	}

	if ac.output == "json" {
		jsonBytes, err := marshalListResponseWithPagination(resp.Models, resp.Pagination)
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

	fmt.Printf("\nFound %d attempt(s):\n\n", len(resp.Models))
	for i := range resp.Models {
		printOutpostAttempt(&resp.Models[i])
		fmt.Println()
	}

	printPaginationInfo(resp.Pagination, "hookdeck outpost attempt list")

	return nil
}

type outpostAttemptGetCmd struct {
	cmd *cobra.Command

	tenantID      string
	destinationID string
	include       string
	output        string
}

func newOutpostAttemptGetCmd() *outpostAttemptGetCmd {
	ac := &outpostAttemptGetCmd{}

	ac.cmd = &cobra.Command{
		Use:     "get <attempt-id>",
		Args:    validators.ExactArgs(1),
		Short:   ShortGet(ResourceAttempt),
		Long:    `Get a delivery attempt, including the destination's response.`,
		PreRunE: ac.validateFlags,
		RunE:    ac.run,
		Example: `  # Get an attempt with the response body
  hookdeck outpost attempt get att_abc123 --include response_data --output json`,
		Annotations: map[string]string{
			"cli.arguments": `[
				{"name":"attempt-id","type":"string","description":"The ID of the delivery attempt.","required":true}
			]`,
		},
	}

	ac.cmd.Flags().StringVar(&ac.tenantID, "tenant-id", "", "Tenant the attempt belongs to")
	ac.cmd.Flags().StringVar(&ac.destinationID, "destination-id", "", "Destination the attempt targeted")
	ac.cmd.Flags().StringVar(&ac.include, "include", "", "Include related data, comma-separated (event, event.data, response_data, destination)")
	ac.cmd.Flags().StringVar(&ac.output, "output", "", "Output format (json)")

	return ac
}

func (ac *outpostAttemptGetCmd) validateFlags(cmd *cobra.Command, args []string) error {
	return rejectEmptyFlags(cmd)
}

func (ac *outpostAttemptGetCmd) run(cmd *cobra.Command, args []string) error {
	client := Config.GetOutpostAPIClient()

	attempt, err := client.GetOutpostAttempt(context.Background(), args[0], hookdeck.OutpostAttemptGetParams{
		TenantID:      ac.tenantID,
		DestinationID: ac.destinationID,
		Include:       splitCommaList(ac.include),
	})
	if err != nil {
		return fmt.Errorf("failed to get attempt: %w", err)
	}

	if ac.output == "json" {
		return printJSONIndented(attempt)
	}

	fmt.Println()
	printOutpostAttempt(attempt)
	fmt.Println()

	return nil
}
