package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

const metricsAttemptsMeasures = "count, successful_count, failed_count, delivered_count, error_rate, response_latency_avg, response_latency_max, response_latency_p95, response_latency_p99, delivery_latency_avg"

type metricsAttemptsCmd struct {
	cmd  *cobra.Command
	flags metricsCommonFlags
}

func newMetricsAttemptsCmd() *metricsAttemptsCmd {
	c := &metricsAttemptsCmd{}
	c.cmd = &cobra.Command{
		Use:   "attempts",
		Args:  cobra.NoArgs,
		Short: "Query attempt metrics",
		Long:  `Query metrics for delivery attempts (latency, success/failure). Measures: ` + metricsAttemptsMeasures + `.`,
		RunE:  c.runE,
	}
	addMetricsCommonFlags(c.cmd, &c.flags)
	return c
}

func (c *metricsAttemptsCmd) runE(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	params := metricsParamsFromFlags(&c.flags)
	data, err := Config.GetAPIClient().QueryAttemptMetrics(context.Background(), params)
	if err != nil {
		return fmt.Errorf("query attempt metrics: %w", err)
	}
	return printMetricsResponse(data, c.flags.output)
}
