package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

type metricsPendingCmd struct {
	cmd  *cobra.Command
	flags metricsCommonFlags
}

func newMetricsPendingCmd() *metricsPendingCmd {
	c := &metricsPendingCmd{}
	c.cmd = &cobra.Command{
		Use:   "pending",
		Args:  cobra.NoArgs,
		Short: "Query events pending timeseries",
		Long:  `Query events pending over time (timeseries). Measures: count.`,
		RunE:  c.runE,
	}
	addMetricsCommonFlags(c.cmd, &c.flags)
	return c
}

func (c *metricsPendingCmd) runE(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	params := metricsParamsFromFlags(&c.flags)
	data, err := Config.GetAPIClient().QueryEventsPendingTimeseries(context.Background(), params)
	if err != nil {
		return fmt.Errorf("query events pending: %w", err)
	}
	return printMetricsResponse(data, c.flags.output)
}
