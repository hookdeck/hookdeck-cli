package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

const metricsRequestsMeasures = "count, accepted_count, rejected_count, discarded_count, avg_events_per_request, avg_ignored_per_request"

type metricsRequestsCmd struct {
	cmd  *cobra.Command
	flags metricsCommonFlags
}

func newMetricsRequestsCmd() *metricsRequestsCmd {
	c := &metricsRequestsCmd{}
	c.cmd = &cobra.Command{
		Use:   "requests",
		Args:  cobra.NoArgs,
		Short: ShortBeta("Query request metrics"),
		Long:  LongBeta(`Query metrics for requests (acceptance, rejection, etc.). Measures: ` + metricsRequestsMeasures + `.`),
		RunE:  c.runE,
	}
	addMetricsCommonFlags(c.cmd, &c.flags)
	return c
}

func (c *metricsRequestsCmd) runE(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	params := metricsParamsFromFlags(&c.flags)
	data, err := Config.GetAPIClient().QueryRequestMetrics(context.Background(), params)
	if err != nil {
		return fmt.Errorf("query request metrics: %w", err)
	}
	return printMetricsResponse(data, c.flags.output)
}
