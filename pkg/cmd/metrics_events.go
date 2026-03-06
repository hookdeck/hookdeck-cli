package cmd

import (
	"context"
	"fmt"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/spf13/cobra"
)

const metricsEventsMeasures = "count, successful_count, failed_count, scheduled_count, paused_count, error_rate, avg_attempts, scheduled_retry_count, pending, queue_depth, max_depth, max_age"
const metricsEventsDimensions = "connection_id, source_id, destination_id, issue_id"

type metricsEventsCmd struct {
	cmd   *cobra.Command
	flags metricsCommonFlags
}

func newMetricsEventsCmd() *metricsEventsCmd {
	c := &metricsEventsCmd{}
	c.cmd = &cobra.Command{
		Use:   "events",
		Args:  cobra.NoArgs,
		Short: ShortBeta("Query event metrics"),
		Long: LongBeta(`Query metrics for events (volume, success/failure counts, error rate, queue depth, pending, etc.).

Measures: ` + metricsEventsMeasures + `.
Dimensions: ` + metricsEventsDimensions + `.

Routing: measures like queue_depth/max_depth/max_age query the queue-depth endpoint;
pending with --granularity queries the pending-timeseries endpoint;
--issue-id or dimensions including issue_id query the events-by-issue endpoint;
all other combinations query the default events metrics endpoint.`),
		RunE: c.runE,
	}
	addMetricsCommonFlags(c.cmd, &c.flags)
	return c
}

// queueDepthMeasures are measures that route to the queue-depth API endpoint.
var queueDepthMeasures = map[string]bool{
	"queue_depth": true,
	"max_depth":   true,
	"max_age":     true,
}

// hasMeasure checks whether any of the requested measures match the given set.
func hasMeasure(params hookdeck.MetricsQueryParams, set map[string]bool) bool {
	for _, m := range params.Measures {
		if set[m] {
			return true
		}
	}
	return false
}

// hasDimension checks whether any of the requested dimensions match the given name.
func hasDimension(params hookdeck.MetricsQueryParams, name string) bool {
	for _, d := range params.Dimensions {
		if d == name {
			return true
		}
	}
	return false
}

// queryEventMetricsConsolidated routes to the correct underlying API endpoint
// based on the requested measures and dimensions.
func queryEventMetricsConsolidated(ctx context.Context, client *hookdeck.Client, params hookdeck.MetricsQueryParams) (hookdeck.MetricsResponse, error) {
	// Route based on measures/dimensions:
	// 1. If measures include queue_depth, max_depth, or max_age → QueryQueueDepth
	if hasMeasure(params, queueDepthMeasures) {
		return client.QueryQueueDepth(ctx, params)
	}
	// 2. If measures include "pending" with granularity → QueryEventsPendingTimeseries
	if hasMeasure(params, map[string]bool{"pending": true}) && params.Granularity != "" {
		return client.QueryEventsPendingTimeseries(ctx, params)
	}
	// 3. If dimensions include "issue_id" or IssueID filter is set → QueryEventsByIssue
	if hasDimension(params, "issue_id") || params.IssueID != "" {
		return client.QueryEventsByIssue(ctx, params)
	}
	// 4. Default → QueryEventMetrics
	return client.QueryEventMetrics(ctx, params)
}

func (c *metricsEventsCmd) runE(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	params := metricsParamsFromFlags(&c.flags)
	data, err := queryEventMetricsConsolidated(context.Background(), Config.GetAPIClient(), params)
	if err != nil {
		return fmt.Errorf("query event metrics: %w", err)
	}
	return printMetricsResponse(data, c.flags.output)
}
