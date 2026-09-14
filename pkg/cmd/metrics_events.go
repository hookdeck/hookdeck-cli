package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/spf13/cobra"
)

var metricsEventsDimensions = hookdeck.EventMetricsDimensions

type metricsEventsCmd struct {
	cmd   *cobra.Command
	flags metricsCommonFlags
}

func newMetricsEventsCmd() *metricsEventsCmd {
	c := &metricsEventsCmd{}
	c.cmd = &cobra.Command{
		Use:   "events",
		Args:  cobra.NoArgs,
		Short: ShortBeta("Query event metrics"),
		Long: LongBeta(`Query event metrics: volume and success/failure counts, error rate, queue depth,
pending over time, or per-issue. Use --measures and --dimensions to choose what to query.
Requires --start and --end.

When querying per-issue (e.g. --dimensions issue_id), --issue-id is required.

Each query is answered by a single endpoint, so a request cannot span two of
them: queue_depth, max_depth, max_age and pending each select their own, and
none of them can be combined with per-issue (--dimensions issue_id, --issue-id).

Measures: ` + hookdeck.EventMetricsMeasures + `.
Dimensions: ` + metricsEventsDimensions + `.`),
		RunE: c.runE,
	}
	addMetricsCommonFlags(c.cmd, &c.flags, hookdeck.EventMetricsFilters, hookdeck.EventMetricsDimensions, hookdeck.EventStatusValues)
	return c
}

// hasDimension checks whether any of the requested dimensions match the given name.
func hasDimension(params hookdeck.MetricsQueryParams, name string) bool {
	for _, d := range params.Dimensions {
		if d == name {
			return true
		}
	}
	return false
}

// queryEventMetricsConsolidated routes to the correct underlying API endpoint
// based on the requested measures and dimensions.
func queryEventMetricsConsolidated(ctx context.Context, client *hookdeck.Client, params hookdeck.MetricsQueryParams) (hookdeck.MetricsResponse, error) {
	// Only one endpoint is called, so a query that names parts of two routes
	// cannot be answered in full: the surplus measures would be dropped or
	// rewritten into a 422, and the conditions below are ordered, so a
	// queue-depth or pending measure silently shadowed the issue_id dimension
	// and the --issue-id filter (#407). Refuse the combination by name rather
	// than exit 0 having answered a different question.
	if err := hookdeck.RejectCrossRouteEventQuery(params, "--measures", "--dimensions", hookdeck.CLIFilterNames); err != nil {
		return nil, err
	}
	// Which measures belong to which endpoint is the shared table's to know, and
	// the route names are its constants: a second copy here could disagree with
	// the refusal above and dispatch a query it had just accepted to the wrong
	// endpoint.
	measureRoute := hookdeck.RouteForMeasures(params.Measures)

	// Route based on measures/dimensions:
	// 1. Measures naming the queue-depth route → QueryQueueDepth
	if measureRoute == hookdeck.EventRouteQueueDepth {
		if err := rejectUnsupportedFilters(params, hookdeck.QueueDepthRouteFilters, hookdeck.EventRouteQueueDepth); err != nil {
			return nil, err
		}
		if err := rejectUnsupportedDimensions(params, hookdeck.QueueDepthRouteDimensions, hookdeck.EventRouteQueueDepth); err != nil {
			return nil, err
		}
		// The endpoint accepts max_depth and max_age only. "queue_depth" is our own
		// spelling for the route, advertised in --help, so translate it rather than
		// letting the API reject a measure we told the user to pass.
		queueParams := params
		queueParams.Measures = hookdeck.TranslateQueueDepthMeasures(params.Measures)
		return client.QueryQueueDepth(ctx, queueParams)
	}
	// 2. Measures naming the pending route → QueryEventsPendingTimeseries.
	// API expects measures[]=count; "pending" is only used for routing.
	// Granularity is optional on this route, so it must not gate the routing:
	// gating it sent "pending" to the default endpoint, which rejects the measure.
	if measureRoute == hookdeck.EventRoutePending {
		if err := rejectUnsupportedFilters(params, hookdeck.PendingTimeseriesRouteFilters, hookdeck.EventRoutePending); err != nil {
			return nil, err
		}
		if err := rejectUnsupportedDimensions(params, hookdeck.PendingTimeseriesRouteDimensions, hookdeck.EventRoutePending); err != nil {
			return nil, err
		}
		pendingParams := params
		pendingParams.Measures = []string{"count"}
		return client.QueryEventsPendingTimeseries(ctx, pendingParams)
	}
	// 3. If dimensions include "issue_id" or IssueID filter is set → QueryEventsByIssue
	// API requires filters (we send filters[issue_id]); --issue-id is required for this path.
	if hasDimension(params, "issue_id") || params.IssueID != "" {
		if params.IssueID == "" {
			return nil, errors.New("per-issue metrics require --issue-id (required when using --dimensions issue_id)")
		}
		if err := rejectUnsupportedFilters(params, hookdeck.EventsByIssueRouteFilters, hookdeck.EventRouteByIssue); err != nil {
			return nil, err
		}
		if err := rejectUnsupportedDimensions(params, hookdeck.EventsByIssueRouteDimensions, hookdeck.EventRouteByIssue); err != nil {
			return nil, err
		}
		return client.QueryEventsByIssue(ctx, params)
	}
	// 4. Default → QueryEventMetrics
	// No filter gate here: the default route honours every filter --help offers
	// except --issue-id, and a set --issue-id selects the by-issue route above,
	// so nothing reaches this fallback for a gate to catch. The invariant is
	// pinned by hookdeck.TestDefaultEventRouteHonoursEveryFilterExceptIssueID,
	// which fails if a filter the route drops is ever added.
	if err := rejectUnsupportedDimensions(params, hookdeck.DefaultEventRouteDimensions, hookdeck.EventRouteDefault); err != nil {
		return nil, err
	}
	return client.QueryEventMetrics(ctx, params)
}

func (c *metricsEventsCmd) runE(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	params := metricsParamsFromFlags(&c.flags)
	data, err := queryEventMetricsConsolidated(context.Background(), Config.GetAPIClient(), params)
	if err != nil {
		return fmt.Errorf("query event metrics: %w", err)
	}
	return printMetricsResponse(data, c.flags.output)
}
