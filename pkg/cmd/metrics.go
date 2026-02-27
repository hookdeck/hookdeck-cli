package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

// printMetricsResponse prints data as JSON or a human-readable table.
func printMetricsResponse(data hookdeck.MetricsResponse, output string) error {
	if output == "json" {
		bytes, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal metrics: %w", err)
		}
		fmt.Println(string(bytes))
		return nil
	}
	if len(data) == 0 {
		fmt.Println("No data points.")
		return nil
	}
	for i, pt := range data {
		tb := "<none>"
		if pt.TimeBucket != nil {
			tb = *pt.TimeBucket
		}
		fmt.Printf("time_bucket: %s\n", tb)
		if len(pt.Dimensions) > 0 {
			for k, v := range pt.Dimensions {
				fmt.Printf("  %s: %v\n", k, v)
			}
		}
		if len(pt.Metrics) > 0 {
			for k, v := range pt.Metrics {
				fmt.Printf("  %s: %v\n", k, v)
			}
		}
		if i < len(data)-1 {
			fmt.Println("---")
		}
	}
	return nil
}

const granularityHelp = `Time bucket size. Format: <number><unit> (e.g. 1h, 5m, 1d).
Units: s (seconds), m (minutes), h (hours), d (days), w (weeks), M (months).`

// metricsCommonFlags holds the common flags for all metrics subcommands.
// Used by addMetricsCommonFlags and to build hookdeck.MetricsQueryParams.
type metricsCommonFlags struct {
	start         string
	end           string
	granularity   string
	measures      string
	dimensions    string
	sourceID      string
	destinationID string
	connectionID  string
	status        string
	issueID       string
	output        string
}

// addMetricsCommonFlags adds common metrics flags to cmd and binds them to f.
// For subcommands that take a required resource id as an argument (e.g. events-by-issue <issue-id>),
// pass skipIssueID true so --issue-id is not added as a flag.
func addMetricsCommonFlags(cmd *cobra.Command, f *metricsCommonFlags) {
	addMetricsCommonFlagsEx(cmd, f, false)
}

func addMetricsCommonFlagsEx(cmd *cobra.Command, f *metricsCommonFlags, skipIssueID bool) {
	cmd.Flags().StringVar(&f.start, "start", "", "Start of time range (ISO 8601 date-time, required)")
	cmd.Flags().StringVar(&f.end, "end", "", "End of time range (ISO 8601 date-time, required)")
	cmd.Flags().StringVar(&f.granularity, "granularity", "", granularityHelp)
	cmd.Flags().StringVar(&f.measures, "measures", "", "Comma-separated list of measures to return")
	cmd.Flags().StringVar(&f.dimensions, "dimensions", "", "Comma-separated dimensions to group by (e.g. connection_id, source_id, destination_id, status)")
	cmd.Flags().StringVar(&f.sourceID, "source-id", "", "Filter by source ID")
	cmd.Flags().StringVar(&f.destinationID, "destination-id", "", "Filter by destination ID")
	cmd.Flags().StringVar(&f.connectionID, "connection-id", "", "Filter by connection ID")
	cmd.Flags().StringVar(&f.status, "status", "", "Filter by status (e.g. SUCCESSFUL, FAILED)")
	if !skipIssueID {
		cmd.Flags().StringVar(&f.issueID, "issue-id", "", "Filter by issue ID")
	}
	cmd.Flags().StringVar(&f.output, "output", "", "Output format (json)")
	_ = cmd.MarkFlagRequired("start")
	_ = cmd.MarkFlagRequired("end")
}

// metricsParamsFromFlags builds hookdeck.MetricsQueryParams from common flags.
// Measures and dimensions are split from comma-separated strings.
func metricsParamsFromFlags(f *metricsCommonFlags) hookdeck.MetricsQueryParams {
	var measures, dimensions []string
	if f.measures != "" {
		for _, s := range strings.Split(f.measures, ",") {
			if t := strings.TrimSpace(s); t != "" {
				measures = append(measures, t)
			}
		}
	}
	if f.dimensions != "" {
		for _, s := range strings.Split(f.dimensions, ",") {
			if t := strings.TrimSpace(s); t != "" {
				// API expects webhook_id for connection dimension; CLI accepts connection_id/connection-id for consistency.
				if t == "connection_id" || t == "connection-id" {
					t = "webhook_id"
				}
				dimensions = append(dimensions, t)
			}
		}
	}
	return hookdeck.MetricsQueryParams{
		Start:         f.start,
		End:           f.end,
		Granularity:   f.granularity,
		Measures:      measures,
		Dimensions:    dimensions,
		SourceID:      f.sourceID,
		DestinationID: f.destinationID,
		ConnectionID:  f.connectionID,
		Status:        f.status,
		IssueID:       f.issueID,
	}
}

type metricsCmd struct {
	cmd *cobra.Command
}

func newMetricsCmd() *metricsCmd {
	mc := &metricsCmd{}

	mc.cmd = &cobra.Command{
		Use:   "metrics",
		Args:  validators.NoArgs,
		Short: ShortBeta("Query Event Gateway metrics"),
		Long: LongBeta(`Query metrics for events, requests, attempts, queue depth, pending events, events by issue, and transformations.
Requires --start and --end (ISO 8601 date-time). Use subcommands to choose the metric type.`),
	}

	mc.cmd.AddCommand(newMetricsEventsCmd().cmd)
	mc.cmd.AddCommand(newMetricsRequestsCmd().cmd)
	mc.cmd.AddCommand(newMetricsAttemptsCmd().cmd)
	mc.cmd.AddCommand(newMetricsQueueDepthCmd().cmd)
	mc.cmd.AddCommand(newMetricsPendingCmd().cmd)
	mc.cmd.AddCommand(newMetricsEventsByIssueCmd().cmd)
	mc.cmd.AddCommand(newMetricsTransformationsCmd().cmd)

	return mc
}

func addMetricsCmdTo(parent *cobra.Command) {
	parent.AddCommand(newMetricsCmd().cmd)
}
