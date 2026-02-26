package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

const metricsEventsMeasures = "count, successful_count, failed_count, scheduled_count, paused_count, error_rate, avg_attempts, scheduled_retry_count"

type metricsEventsCmd struct {
	cmd  *cobra.Command
	flags metricsCommonFlags
}

func newMetricsEventsCmd() *metricsEventsCmd {
	c := &metricsEventsCmd{}
	c.cmd = &cobra.Command{
		Use:   "events",
		Args:  cobra.NoArgs,
		Short: ShortBeta("Query event metrics"),
		Long:  LongBeta(`Query metrics for events (volume, success/failure counts, error rate, etc.). Measures: ` + metricsEventsMeasures + `.`),
		RunE:  c.runE,
	}
	addMetricsCommonFlags(c.cmd, &c.flags)
	return c
}

func (c *metricsEventsCmd) runE(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	params := metricsParamsFromFlags(&c.flags)
	data, err := Config.GetAPIClient().QueryEventMetrics(context.Background(), params)
	if err != nil {
		return fmt.Errorf("query event metrics: %w", err)
	}
	return printMetricsResponse(data, c.flags.output)
}
