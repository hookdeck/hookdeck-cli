package cmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// TestEventDimensionsAreGatedPerRoute is the CLI half of the dimension gate.
//
// `metrics events` fans out over four endpoints and --help advertises the
// union, so the route has to narrow it - exactly as it already does for
// filters. Both layers call the same helper over the same matrix, which is what
// stopped the filter fix from drifting and should stop this one too.
func TestEventDimensionsAreGatedPerRoute(t *testing.T) {
	tests := []struct {
		name     string
		params   hookdeck.MetricsQueryParams
		contains []string
	}{
		{
			name:     "pending groups by destination only",
			params:   hookdeck.MetricsQueryParams{Measures: []string{"pending"}, Dimensions: []string{"status"}},
			contains: []string{"--dimensions", "status", "pending event metrics", "destination_id"},
		},
		{
			name:     "queue depth has no status dimension",
			params:   hookdeck.MetricsQueryParams{Measures: []string{"queue_depth"}, Dimensions: []string{"status"}},
			contains: []string{"queue depth metrics"},
		},
		{
			name:     "per-issue route has a narrower set",
			params:   hookdeck.MetricsQueryParams{Measures: []string{"count"}, Dimensions: []string{"issue_id", "status"}, IssueID: "iss_1"},
			contains: []string{"per-issue event metrics", "status"},
		},
		{
			name:     "delivery_group grouping needs a destination filter",
			params:   hookdeck.MetricsQueryParams{Measures: []string{"count"}, Dimensions: []string{"delivery_group"}},
			contains: []string{"delivery_group", "--destination-id"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, path, _ := routeCapture(t)
			_, err := queryEventMetricsConsolidated(context.Background(), client, tt.params)
			require.Error(t, err, "a dimension the route does not define must not reach the API")
			for _, want := range tt.contains {
				assert.Contains(t, err.Error(), want)
			}
			assert.Empty(t, *path, "no request should have been sent")
		})
	}
}

// TestEventDimensionsTheRouteHonoursStillReachTheAPI is the other half: the
// combination that works against the live API must keep working.
func TestEventDimensionsTheRouteHonoursStillReachTheAPI(t *testing.T) {
	client, path, query := routeCapture(t)

	_, err := queryEventMetricsConsolidated(context.Background(), client,
		hookdeck.MetricsQueryParams{
			Measures:      []string{"count"},
			Dimensions:    []string{"delivery_group"},
			DestinationID: "des_1",
		})
	require.NoError(t, err)

	assert.Contains(t, *path, "/metrics/events")
	assert.Equal(t, []string{"delivery_group"}, (*query)["dimensions[]"])
	assert.Equal(t, "des_1", query.Get("filters[destination_id]"))
}

// TestMetricsHelpListsTheRealDimensions guards the --help text the user reads
// before composing a query. It claimed issue_id was a plain events dimension
// while omitting error_code, cli_id, attempts and response_status, so a user
// following it either grouped by something the route rejects or never learned
// about four dimensions that work.
func TestMetricsHelpListsTheRealDimensions(t *testing.T) {
	long := newMetricsEventsCmd().cmd.Long
	for _, dimension := range []string{"error_code", "cli_id", "attempts", "response_status"} {
		assert.Contains(t, long, dimension, "events --help omits the %s dimension", dimension)
	}
	// The connection dimension is connection_id to a CLI user; webhook_id is
	// the API's own spelling and is mapped on the way in.
	assert.Contains(t, long, "connection_id")
}
