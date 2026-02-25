package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

const metricsQueueDepthMeasures = "max_depth, max_age"
const metricsQueueDepthDimensions = "destination_id"

type metricsQueueDepthCmd struct {
	cmd  *cobra.Command
	flags metricsCommonFlags
}

func newMetricsQueueDepthCmd() *metricsQueueDepthCmd {
	c := &metricsQueueDepthCmd{}
	c.cmd = &cobra.Command{
		Use:   "queue-depth",
		Args:  cobra.NoArgs,
		Short: "Query queue depth metrics",
		Long:  `Query queue depth metrics. Measures: ` + metricsQueueDepthMeasures + `. Dimensions: ` + metricsQueueDepthDimensions + `.`,
		RunE:  c.runE,
	}
	addMetricsCommonFlags(c.cmd, &c.flags)
	return c
}

func (c *metricsQueueDepthCmd) runE(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	params := metricsParamsFromFlags(&c.flags)
	data, err := Config.GetAPIClient().QueryQueueDepth(context.Background(), params)
	if err != nil {
		return fmt.Errorf("query queue depth: %w", err)
	}
	return printMetricsResponse(data, c.flags.output)
}
