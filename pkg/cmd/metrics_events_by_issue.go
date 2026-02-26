package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type metricsEventsByIssueCmd struct {
	cmd  *cobra.Command
	flags metricsCommonFlags
}

func newMetricsEventsByIssueCmd() *metricsEventsByIssueCmd {
	c := &metricsEventsByIssueCmd{}
	c.cmd = &cobra.Command{
		Use:   "events-by-issue <issue-id>",
		Args:  validators.ExactArgs(1),
		Short: ShortBeta("Query events grouped by issue"),
		Long:  LongBeta(`Query metrics for events grouped by issue (for debugging). Requires issue ID as argument.`),
		RunE:  c.runE,
	}
	addMetricsCommonFlagsEx(c.cmd, &c.flags, true)
	return c
}

func (c *metricsEventsByIssueCmd) runE(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	params := metricsParamsFromFlags(&c.flags)
	params.IssueID = args[0]
	data, err := Config.GetAPIClient().QueryEventsByIssue(context.Background(), params)
	if err != nil {
		return fmt.Errorf("query events by issue: %w", err)
	}
	return printMetricsResponse(data, c.flags.output)
}
