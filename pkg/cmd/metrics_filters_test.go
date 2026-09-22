package cmd

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// TestMetricsFlagsMatchTheEndpointSchemas guards against offering a filter the
// endpoint ignores, which returns unfiltered totals that look filtered.
//
// Expectations are hardcoded, so an API-side change will NOT fail this test; it
// catches the CLI and MCP layers drifting apart. Re-check the matrix against
// https://api.hookdeck.com/2026-09-01/openapi when the version moves.
func TestMetricsFlagsMatchTheEndpointSchemas(t *testing.T) {
	all := []string{"source-id", "destination-id", "connection-id", "status", "issue-id", "delivery-group"}

	tests := []struct {
		name    string
		cmd     *cobra.Command
		offered []string
	}{
		{"requests", newMetricsRequestsCmd().cmd, []string{"source-id", "status"}},
		{"attempts", newMetricsAttemptsCmd().cmd, []string{"destination-id", "status", "delivery-group"}},
		{"transformations", newMetricsTransformationsCmd().cmd, []string{"connection-id", "issue-id"}},
		// events fans out over four endpoints, so it offers the union and
		// validates per route at run time.
		{"events", newMetricsEventsCmd().cmd, all},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offered := map[string]bool{}
			for _, f := range tt.offered {
				offered[f] = true
			}
			for _, flag := range all {
				got := tt.cmd.Flags().Lookup(flag) != nil
				if offered[flag] {
					assert.True(t, got, "%s honours %s and should offer the flag", tt.name, flag)
				} else {
					assert.False(t, got, "%s ignores %s; offering it would imply a filter that does nothing", tt.name, flag)
				}
			}
		})
	}
}

// TestEventMetricsRejectFiltersTheRouteIgnores covers the routes `metrics events`
// can take. The flag set cannot be decided when the command is built, so each
// route has to refuse the filters its endpoint would drop.
func TestEventMetricsRejectFiltersTheRouteIgnores(t *testing.T) {
	tests := []struct {
		name    string
		params  hookdeck.MetricsQueryParams
		wantErr string
	}{
		{
			name:    "queue depth ignores source",
			params:  hookdeck.MetricsQueryParams{Measures: []string{"queue_depth"}, SourceID: "src_1"},
			wantErr: "--source-id",
		},
		{
			name:    "pending timeseries ignores delivery group",
			params:  hookdeck.MetricsQueryParams{Measures: []string{"pending"}, Granularity: "1h", DeliveryGroup: "dg_1"},
			wantErr: "--delivery-group",
		},
		{
			name:    "pending timeseries ignores status",
			params:  hookdeck.MetricsQueryParams{Measures: []string{"pending"}, Granularity: "1h", Status: "SUCCESSFUL"},
			wantErr: "--status",
		},
		{
			name:    "per-issue ignores delivery group",
			params:  hookdeck.MetricsQueryParams{Dimensions: []string{"issue_id"}, IssueID: "iss_1", DeliveryGroup: "dg_1"},
			wantErr: "--delivery-group",
		},
		{
			name:    "per-issue ignores status",
			params:  hookdeck.MetricsQueryParams{Dimensions: []string{"issue_id"}, IssueID: "iss_1", Status: "FAILED"},
			wantErr: "--status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := queryEventMetricsConsolidated(context.Background(), nil, tt.params)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Contains(t, err.Error(), "unfiltered",
				"the message should say why it matters, not just that the flag is unsupported")
		})
	}
}

// TestPerIssueStillReportsTheMissingIssueIDFirst keeps the more useful error
// ahead of the filter validation.
func TestPerIssueStillReportsTheMissingIssueIDFirst(t *testing.T) {
	_, err := queryEventMetricsConsolidated(context.Background(), nil, hookdeck.MetricsQueryParams{
		Dimensions:    []string{"issue_id"},
		DeliveryGroup: "dg_1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--issue-id")
}

// TestRejectUnsupportedFiltersAllowsWhatTheEndpointHonours is the other half:
// a filter the schema declares must pass through untouched.
func TestRejectUnsupportedFiltersAllowsWhatTheEndpointHonours(t *testing.T) {
	err := rejectUnsupportedFilters(hookdeck.MetricsQueryParams{
		SourceID:      "src_1",
		DestinationID: "des_1",
		ConnectionID:  "web_1",
		Status:        "SUCCESSFUL",
		DeliveryGroup: "dg_1",
	}, hookdeck.DefaultEventRouteFilters, "event metrics")
	assert.NoError(t, err)

	err = rejectUnsupportedFilters(hookdeck.MetricsQueryParams{
		DestinationID: "des_1",
		DeliveryGroup: "dg_1",
	}, hookdeck.QueueDepthRouteFilters, "queue depth metrics")
	assert.NoError(t, err)

	// Nothing set is always fine.
	assert.NoError(t, rejectUnsupportedFilters(hookdeck.MetricsQueryParams{}, hookdeck.PendingTimeseriesRouteFilters, "pending"))
}
