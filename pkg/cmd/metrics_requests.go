package cmd

import (
	"context"
	"fmt"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"

	"github.com/spf13/cobra"
)

type metricsRequestsCmd struct {
	cmd   *cobra.Command
	flags metricsCommonFlags
}

func newMetricsRequestsCmd() *metricsRequestsCmd {
	c := &metricsRequestsCmd{}
	c.cmd = &cobra.Command{
		Use:   "requests",
		Args:  cobra.NoArgs,
		Short: ShortBeta("Query request metrics"),
		Long:  LongBeta(`Query metrics for requests (acceptance, rejection, etc.). Measures: ` + hookdeck.RequestMetricsMeasures + `.`),
		RunE:  c.runE,
	}
	addMetricsCommonFlags(c.cmd, &c.flags, hookdeck.RequestMetricsFilters, hookdeck.RequestMetricsDimensions, hookdeck.RequestStatusValues)
	return c
}

func (c *metricsRequestsCmd) runE(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	params := metricsParamsFromFlags(&c.flags)
	if err := rejectUnsupportedDimensions(params, hookdeck.RequestMetricsDimensionValues, "request metrics"); err != nil {
		return err
	}
	data, err := Config.GetAPIClient().QueryRequestMetrics(context.Background(), params)
	if err != nil {
		return fmt.Errorf("query request metrics: %w", err)
	}
	return printMetricsResponse(data, c.flags.output)
}
