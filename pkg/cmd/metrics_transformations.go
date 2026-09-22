package cmd

import (
	"context"
	"fmt"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"

	"github.com/spf13/cobra"
)

type metricsTransformationsCmd struct {
	cmd   *cobra.Command
	flags metricsCommonFlags
}

func newMetricsTransformationsCmd() *metricsTransformationsCmd {
	c := &metricsTransformationsCmd{}
	c.cmd = &cobra.Command{
		Use:   "transformations",
		Args:  cobra.NoArgs,
		Short: ShortBeta("Query transformation metrics"),
		Long:  LongBeta(`Query metrics for transformations. Measures: ` + hookdeck.TransformationMetricsMeasures + `.`),
		RunE:  c.runE,
	}
	addMetricsCommonFlags(c.cmd, &c.flags, hookdeck.TransformationMetricsFilters, hookdeck.TransformationMetricsDimensions, hookdeck.TransformationStatusValues)
	return c
}

func (c *metricsTransformationsCmd) runE(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	params := metricsParamsFromFlags(&c.flags)
	if err := rejectUnsupportedDimensions(params, hookdeck.TransformationMetricsDimensionValues, "transformation metrics"); err != nil {
		return err
	}
	data, err := Config.GetAPIClient().QueryTransformationMetrics(context.Background(), params)
	if err != nil {
		return fmt.Errorf("query transformation metrics: %w", err)
	}
	return printMetricsResponse(data, c.flags.output)
}
