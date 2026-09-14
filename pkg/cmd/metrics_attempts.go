package cmd

import (
	"context"
	"fmt"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"

	"github.com/spf13/cobra"
)

type metricsAttemptsCmd struct {
	cmd   *cobra.Command
	flags metricsCommonFlags
}

func newMetricsAttemptsCmd() *metricsAttemptsCmd {
	c := &metricsAttemptsCmd{}
	c.cmd = &cobra.Command{
		Use:   "attempts",
		Args:  cobra.NoArgs,
		Short: ShortBeta("Query attempt metrics"),
		Long:  LongBeta(`Query metrics for delivery attempts (latency, success/failure). Measures: ` + hookdeck.AttemptMetricsMeasures + `.`),
		RunE:  c.runE,
	}
	addMetricsCommonFlags(c.cmd, &c.flags, hookdeck.AttemptMetricsFilters, hookdeck.AttemptMetricsDimensions, hookdeck.AttemptStatusValues)
	return c
}

func (c *metricsAttemptsCmd) runE(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	params := metricsParamsFromFlags(&c.flags)
	if err := rejectUnsupportedDimensions(params, hookdeck.AttemptMetricsDimensionValues, "attempt metrics"); err != nil {
		return err
	}
	data, err := Config.GetAPIClient().QueryAttemptMetrics(context.Background(), params)
	if err != nil {
		return fmt.Errorf("query attempt metrics: %w", err)
	}
	return printMetricsResponse(data, c.flags.output)
}
