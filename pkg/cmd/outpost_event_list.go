package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type outpostEventListCmd struct {
	cmd *cobra.Command

	ids            string
	tenantIDs      string
	destinationIDs string
	topics         string
	timeAfter      string
	timeBefore     string
	limit          int
	orderBy        string
	dir            string
	next           string
	prev           string
	output         string
}

func newOutpostEventListCmd() *outpostEventListCmd {
	ec := &outpostEventListCmd{}

	ec.cmd = &cobra.Command{
		Use:   "list",
		Args:  validators.NoArgs,
		Short: ShortList(ResourceEvent),
		Long: `List published events, most recent first.

Filters are combined with AND. Time bounds are ISO 8601 datetimes.`,
		PreRunE: ec.validateFlags,
		RunE:    ec.run,
		Example: `  # Recent events
  hookdeck outpost event list --limit 10

  # For one tenant, on one topic
  hookdeck outpost event list --tenant-id acme --topic user.created

  # Within a time window
  hookdeck outpost event list --time-after 2026-08-01T00:00:00Z --time-before 2026-08-14T00:00:00Z`,
	}

	ec.cmd.Flags().StringVar(&ec.ids, "id", "", "Filter by event ID(s), comma-separated")
	ec.cmd.Flags().StringVar(&ec.tenantIDs, "tenant-id", "", "Filter by tenant ID(s), comma-separated")
	ec.cmd.Flags().StringVar(&ec.destinationIDs, "destination-id", "", "Filter by matched destination ID(s), comma-separated")
	ec.cmd.Flags().StringVar(&ec.topics, "topic", "", "Filter by topic(s), comma-separated")
	ec.cmd.Flags().StringVar(&ec.timeAfter, "time-after", "", "Only events at or after this ISO 8601 datetime")
	ec.cmd.Flags().StringVar(&ec.timeBefore, "time-before", "", "Only events at or before this ISO 8601 datetime")
	ec.cmd.Flags().IntVar(&ec.limit, "limit", 0, "Limit number of results")
	ec.cmd.Flags().StringVar(&ec.orderBy, "order-by", "", "Field to sort by (time)")
	ec.cmd.Flags().StringVar(&ec.dir, "dir", "", "Sort direction (asc, desc)")
	ec.cmd.Flags().StringVar(&ec.next, "next", "", "Next page cursor")
	ec.cmd.Flags().StringVar(&ec.prev, "prev", "", "Previous page cursor")
	ec.cmd.Flags().StringVar(&ec.output, "output", "", "Output format (json)")

	return ec
}

func (ec *outpostEventListCmd) validateFlags(cmd *cobra.Command, args []string) error {
	return rejectEmptyFlags(cmd)
}

func (ec *outpostEventListCmd) run(cmd *cobra.Command, args []string) error {
	client := Config.GetOutpostAPIClient()

	resp, err := client.ListOutpostEvents(context.Background(), hookdeck.OutpostEventListParams{
		IDs:            splitCommaList(ec.ids),
		TenantIDs:      splitCommaList(ec.tenantIDs),
		DestinationIDs: splitCommaList(ec.destinationIDs),
		Topics:         splitCommaList(ec.topics),
		TimeAfter:      ec.timeAfter,
		TimeBefore:     ec.timeBefore,
		Limit:          ec.limit,
		OrderBy:        ec.orderBy,
		Dir:            ec.dir,
		Next:           ec.next,
		Prev:           ec.prev,
	})
	if err != nil {
		return fmt.Errorf("failed to list events: %w", err)
	}

	if ec.output == "json" {
		jsonBytes, err := marshalListResponseWithPagination(resp.Models, resp.Pagination)
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
	fmt.Printf("\nFound %d event(s):\n\n", len(resp.Models))
	for _, event := range resp.Models {
		fmt.Printf("%s\n", color.Green(event.ID))
		fmt.Printf("  Topic: %s\n", event.Topic)
		fmt.Printf("  Tenant: %s\n", event.TenantID)
		if len(event.MatchedDestinationIDs) > 0 {
			fmt.Printf("  Destinations: %s\n", strings.Join(event.MatchedDestinationIDs, ", "))
		}
		fmt.Printf("  Time: %s\n", event.Time.Format("2006-01-02 15:04:05"))
		fmt.Println()
	}

	printPaginationInfo(resp.Pagination, "hookdeck outpost event list")

	return nil
}
