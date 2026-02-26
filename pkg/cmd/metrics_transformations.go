package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

const metricsTransformationsMeasures = "count, successful_count, failed_count, error_rate, error_count, warn_count, info_count, debug_count"

type metricsTransformationsCmd struct {
	cmd  *cobra.Command
	flags metricsCommonFlags
}

func newMetricsTransformationsCmd() *metricsTransformationsCmd {
	c := &metricsTransformationsCmd{}
	c.cmd = &cobra.Command{
		Use:   "transformations",
		Args:  cobra.NoArgs,
		Short: ShortBeta("Query transformation metrics"),
		Long:  LongBeta(`Query metrics for transformations. Measures: ` + metricsTransformationsMeasures + `.`),
		RunE:  c.runE,
	}
	addMetricsCommonFlags(c.cmd, &c.flags)
	return c
}

func (c *metricsTransformationsCmd) runE(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	params := metricsParamsFromFlags(&c.flags)
	data, err := Config.GetAPIClient().QueryTransformationMetrics(context.Background(), params)
	if err != nil {
		return fmt.Errorf("query transformation metrics: %w", err)
	}
	return printMetricsResponse(data, c.flags.output)
}
