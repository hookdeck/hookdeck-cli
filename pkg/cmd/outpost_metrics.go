package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type outpostMetricsCmd struct {
	cmd *cobra.Command
}

func newOutpostMetricsCmd() *outpostMetricsCmd {
	mc := &outpostMetricsCmd{}

	mc.cmd = &cobra.Command{
		Use:   "metrics",
		Args:  validators.NoArgs,
		Short: ShortBeta("Query aggregate metrics"),
		Long: LongBeta(`Query aggregated metrics over a time range.

Both subcommands require --start, --end and at least one --measures value, and
can group results with --dimensions.`),
	}

	mc.cmd.AddCommand(newOutpostMetricsResourceCmd("events",
		"Aggregated event publish metrics.",
		"count, rate",
		"tenant_id, topic, destination_id").cmd)
	mc.cmd.AddCommand(newOutpostMetricsResourceCmd("attempts",
		"Aggregated delivery attempt metrics.",
		"count, successful_count, failed_count, error_rate, first_attempt_count, retry_count, manual_retry_count, avg_attempt_number, rate, successful_rate, failed_rate",
		"tenant_id, destination_id, destination_type, topic, status, code, manual, attempt_number").cmd)

	return mc
}

// outpostMetricsResourceCmd backs both `metrics events` and `metrics attempts`,
// which differ only in endpoint and in the measures and dimensions they accept.
type outpostMetricsResourceCmd struct {
	cmd      *cobra.Command
	resource string

	start       string
	end         string
	granularity string
	measures    string
	dimensions  string
	filters     []string
	output      string
}

func newOutpostMetricsResourceCmd(resource, summary, measures, dimensions string) *outpostMetricsResourceCmd {
	mc := &outpostMetricsResourceCmd{resource: resource}

	mc.cmd = &cobra.Command{
		Use:   resource,
		Args:  validators.NoArgs,
		Short: ShortBeta(summary),
		Long: LongBeta(fmt.Sprintf(`%s

Measures: %s

Dimensions: %s

Omit --granularity for a single total over the whole range; set it (1h, 5m, 1d)
to bucket the results over time.`, summary, measures, dimensions)),
		PreRunE: mc.validateFlags,
		RunE:    mc.run,
		Example: fmt.Sprintf(`  # Total over the last week
  hookdeck outpost metrics %s --start 2026-08-07T00:00:00Z --end 2026-08-14T00:00:00Z --measures count

  # Bucketed hourly and grouped by topic
  hookdeck outpost metrics %s --start 2026-08-13T00:00:00Z --end 2026-08-14T00:00:00Z \
    --measures count --granularity 1h --dimensions topic`, resource, resource),
	}

	mc.cmd.Flags().StringVar(&mc.start, "start", "", "Start of the range, ISO 8601 (required)")
	mc.cmd.Flags().StringVar(&mc.end, "end", "", "End of the range, ISO 8601 (required)")
	mc.cmd.Flags().StringVar(&mc.granularity, "granularity", "", "Bucket size (e.g. 5m, 1h, 1d)")
	mc.cmd.Flags().StringVar(&mc.measures, "measures", "", "Measures to compute, comma-separated (required)")
	mc.cmd.Flags().StringVar(&mc.dimensions, "dimensions", "", "Dimensions to group by, comma-separated")
	mc.cmd.Flags().StringArrayVar(&mc.filters, "filter", nil, "Filter as dimension=value (repeatable)")
	mc.cmd.Flags().StringVar(&mc.output, "output", "", "Output format (json)")

	mc.cmd.MarkFlagRequired("start")
	mc.cmd.MarkFlagRequired("end")
	mc.cmd.MarkFlagRequired("measures")

	return mc
}

func (mc *outpostMetricsResourceCmd) validateFlags(cmd *cobra.Command, args []string) error {
	return rejectEmptyFlags(cmd)
}

func (mc *outpostMetricsResourceCmd) run(cmd *cobra.Command, args []string) error {
	filters := map[string][]string{}
	for _, entry := range mc.filters {
		key, value, found := strings.Cut(entry, "=")
		key = strings.TrimSpace(key)
		if !found || key == "" {
			return fmt.Errorf("--filter %q must be in dimension=value form", entry)
		}
		filters[key] = append(filters[key], value)
	}

	params := hookdeck.OutpostMetricsParams{
		Start:       mc.start,
		End:         mc.end,
		Granularity: mc.granularity,
		Measures:    splitCommaList(mc.measures),
		Dimensions:  splitCommaList(mc.dimensions),
		Filters:     filters,
	}

	client := Config.GetOutpostAPIClient()
	ctx := context.Background()

	var (
		resp *hookdeck.OutpostMetricsResponse
		err  error
	)
	if mc.resource == "events" {
		resp, err = client.GetOutpostEventMetrics(ctx, params)
	} else {
		resp, err = client.GetOutpostAttemptMetrics(ctx, params)
	}
	if err != nil {
		return fmt.Errorf("failed to get %s metrics: %w", mc.resource, err)
	}

	if mc.output == "json" {
		return printJSONIndented(resp)
	}

	if len(resp.Data) == 0 {
		fmt.Println("No data for that range.")
		return nil
	}

	fmt.Println()
	for _, point := range resp.Data {
		var parts []string
		if point.TimeBucket != nil {
			parts = append(parts, point.TimeBucket.Format("2006-01-02 15:04"))
		}
		for _, key := range sortedStringKeys(point.Dimensions) {
			parts = append(parts, fmt.Sprintf("%s=%s", key, point.Dimensions[key]))
		}
		if len(parts) > 0 {
			fmt.Printf("%s\n", strings.Join(parts, "  "))
		}
		for _, key := range sortedKeys(point.Metrics) {
			fmt.Printf("  %s: %v\n", key, point.Metrics[key])
		}
		fmt.Println()
	}

	// Silent truncation would read as a complete picture, so say so.
	if resp.Metadata.Truncated {
		fmt.Printf("Results were truncated at the %d row limit; narrow the range or filters for a complete picture.\n",
			resp.Metadata.RowLimit)
	}

	return nil
}

func sortedStringKeys(m map[string]string) []string {
	generic := make(map[string]interface{}, len(m))
	for k, v := range m {
		generic[k] = v
	}
	return sortedKeys(generic)
}
