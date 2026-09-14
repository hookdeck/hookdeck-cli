package cmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
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

// metricsStub serves any metrics endpoint and records the path it was asked
// for, so a dimension the endpoint does not define can be caught reaching it.
func metricsStub(t *testing.T, path *string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)
	return server
}

// metricsSubcommand is a metrics command and its RunE, so a test can drive the
// command's own wiring rather than the shared helper it calls.
type metricsSubcommand struct {
	cmd  *cobra.Command
	runE func(*cobra.Command, []string) error
}

// runMetricsSubcommandAgainst parses the flags as the user typed them and runs
// the command against the stub, so the gate is exercised where the command
// wires it up.
func runMetricsSubcommandAgainst(t *testing.T, server *httptest.Server, sub metricsSubcommand, args ...string) error {
	t.Helper()
	old := Config
	t.Cleanup(func() { Config = old })
	config.ResetAPIClientForTesting()
	t.Cleanup(config.ResetAPIClientForTesting)

	Config = config.Config{}
	Config.APIBaseURL = server.URL
	Config.Profile.APIKey = "sk_test_123456789012"
	Config.Profile.ProjectId = "proj_1"

	require.NoError(t, sub.cmd.ParseFlags(append([]string{
		"--start", "2025-01-01T00:00:00Z",
		"--end", "2025-01-02T00:00:00Z",
		"--measures", "count",
	}, args...)))
	return sub.runE(sub.cmd, nil)
}

func attemptsSubcommand() metricsSubcommand {
	c := newMetricsAttemptsCmd()
	return metricsSubcommand{cmd: c.cmd, runE: c.runE}
}

func requestsSubcommand() metricsSubcommand {
	c := newMetricsRequestsCmd()
	return metricsSubcommand{cmd: c.cmd, runE: c.runE}
}

func transformationsSubcommand() metricsSubcommand {
	c := newMetricsTransformationsCmd()
	return metricsSubcommand{cmd: c.cmd, runE: c.runE}
}

// TestSiblingMetricsCommandsGateTheirDimensions covers the three non-events
// metrics commands. Their dimension vocabularies differ sharply - attempts has
// no source_id, requests has no destination_id, transformations has no status -
// so a dimension borrowed from a sibling endpoint is an API 422 for something
// --help appears to offer. Only the events routes were pinned.
func TestSiblingMetricsCommandsGateTheirDimensions(t *testing.T) {
	tests := []struct {
		name      string
		sub       func() metricsSubcommand
		dimension string
		contains  []string
	}{
		{
			name:      "attempts do not group by source",
			sub:       attemptsSubcommand,
			dimension: "source_id",
			contains:  []string{"--dimensions", "source_id", "attempt metrics", "destination_id"},
		},
		{
			name:      "requests do not group by destination",
			sub:       requestsSubcommand,
			dimension: "destination_id",
			contains:  []string{"--dimensions", "destination_id", "request metrics", "source_id"},
		},
		{
			name:      "transformations do not group by status",
			sub:       transformationsSubcommand,
			dimension: "status",
			contains:  []string{"--dimensions", "status", "transformation metrics", "transformation_id"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var path string
			server := metricsStub(t, &path)

			err := runMetricsSubcommandAgainst(t, server, tt.sub(), "--dimensions", tt.dimension)
			require.Error(t, err, "a dimension the endpoint does not define must not reach the API")
			for _, want := range tt.contains {
				assert.Contains(t, err.Error(), want)
			}
			assert.Empty(t, path, "no request should have been sent")
		})
	}
}

// TestSiblingMetricsCommandsKeepTheirOwnDimensions is the other half: each
// command's own dimensions must still reach its endpoint.
func TestSiblingMetricsCommandsKeepTheirOwnDimensions(t *testing.T) {
	tests := []struct {
		name      string
		sub       func() metricsSubcommand
		dimension string
		path      string
	}{
		{"attempts group by destination", attemptsSubcommand, "destination_id", "/metrics/attempts"},
		{"requests group by source", requestsSubcommand, "source_id", "/metrics/requests"},
		{"transformations group by log level", transformationsSubcommand, "log_level", "/metrics/transformations"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var path string
			server := metricsStub(t, &path)

			require.NoError(t, runMetricsSubcommandAgainst(t, server, tt.sub(), "--dimensions", tt.dimension))
			assert.Contains(t, path, tt.path)
		})
	}
}
