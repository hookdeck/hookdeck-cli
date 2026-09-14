package cmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// routeCapture records the path and query of the request the routing actually
// produced, so these tests pin the wiring rather than a pure helper.
func routeCapture(t *testing.T) (*hookdeck.Client, *string, *url.Values) {
	t.Helper()
	var gotPath string
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(srv.Close)

	baseURL, err := url.Parse(srv.URL)
	require.NoError(t, err)
	return &hookdeck.Client{BaseURL: baseURL, APIKey: "k"}, &gotPath, &gotQuery
}

// TestPendingRoutesWithoutGranularity pins the routing fix: "pending" selects
// the pending-timeseries endpoint on the measure alone. Granularity is optional
// on that route, and gating on it sent the request to the default events
// endpoint, which does not define the measure.
func TestPendingRoutesWithoutGranularity(t *testing.T) {
	client, path, query := routeCapture(t)

	_, err := queryEventMetricsConsolidated(context.Background(), client,
		hookdeck.MetricsQueryParams{Measures: []string{"pending"}})
	require.NoError(t, err)

	assert.Contains(t, *path, "events-pending-timeseries",
		"pending must not fall through to the default events route")
	// "pending" only selects the route; the API expects measures[]=count.
	assert.Equal(t, []string{"count"}, (*query)["measures[]"])
}

// TestPendingStillRoutesWithGranularity guards the case that already worked.
func TestPendingStillRoutesWithGranularity(t *testing.T) {
	client, path, _ := routeCapture(t)

	_, err := queryEventMetricsConsolidated(context.Background(), client,
		hookdeck.MetricsQueryParams{Measures: []string{"pending"}, Granularity: "1h"})
	require.NoError(t, err)
	assert.Contains(t, *path, "events-pending-timeseries")
}

// TestQueueDepthMeasureIsTranslatedOnTheWire pins the translation wiring, not
// just the helper: the endpoint accepts max_depth and max_age only, so sending
// our own "queue_depth" spelling is rejected by the API.
func TestQueueDepthMeasureIsTranslatedOnTheWire(t *testing.T) {
	client, path, query := routeCapture(t)

	_, err := queryEventMetricsConsolidated(context.Background(), client,
		hookdeck.MetricsQueryParams{Measures: []string{"queue_depth"}})
	require.NoError(t, err)

	assert.Contains(t, *path, "queue-depth")
	assert.Equal(t, []string{"max_depth"}, (*query)["measures[]"],
		"queue_depth must reach the API as max_depth")
}

// TestTranslateQueueDepthMeasures covers the helper's edge cases.
func TestTranslateQueueDepthMeasures(t *testing.T) {
	assert.Equal(t, []string{"max_depth"}, hookdeck.TranslateQueueDepthMeasures([]string{"queue_depth"}))
	assert.Equal(t, []string{"max_age"}, hookdeck.TranslateQueueDepthMeasures([]string{"max_age"}))
	// Both spellings collapse to one measure rather than being sent twice.
	assert.Equal(t, []string{"max_depth"}, hookdeck.TranslateQueueDepthMeasures([]string{"queue_depth", "max_depth"}))
	assert.Equal(t, []string{"max_depth", "max_age"}, hookdeck.TranslateQueueDepthMeasures([]string{"queue_depth", "max_age"}))
	assert.Empty(t, hookdeck.TranslateQueueDepthMeasures(nil))
}

// TestMixedMeasureRoutesAreRejected pins the guard against a measure list that
// spans more than one endpoint. Only one endpoint is called, so the surplus
// measures were either dropped (pending replaces the list with "count") or
// rewritten into a 422 (queue_depth becomes max_depth). Both looked like a
// successful answer to a question that was never asked.
func TestMixedMeasureRoutesAreRejected(t *testing.T) {
	tests := []struct {
		name     string
		measures []string
		contains []string
	}{
		{
			name:     "pending with a default-route measure",
			measures: []string{"pending", "failed_count"},
			contains: []string{"--measures", `"pending"`, `"failed_count"`, "pending event metrics", "event metrics"},
		},
		{
			name:     "default-route measure with queue depth",
			measures: []string{"count", "queue_depth"},
			contains: []string{`"count"`, `"queue_depth"`, "queue depth metrics"},
		},
		{
			name:     "pending with queue depth",
			measures: []string{"max_age", "pending"},
			contains: []string{`"max_age"`, `"pending"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, path, _ := routeCapture(t)

			_, err := queryEventMetricsConsolidated(context.Background(), client,
				hookdeck.MetricsQueryParams{Measures: tt.measures})
			require.Error(t, err, "a cross-route measure list must not be answered from one endpoint")
			for _, want := range tt.contains {
				assert.Contains(t, err.Error(), want)
			}
			assert.Empty(t, *path, "the API must not be called at all")
		})
	}
}

// TestSingleRouteMeasureCombinationsStillWork is the other half: measures that
// all belong to one endpoint must still be sent together.
func TestSingleRouteMeasureCombinationsStillWork(t *testing.T) {
	t.Run("default route", func(t *testing.T) {
		client, path, query := routeCapture(t)
		_, err := queryEventMetricsConsolidated(context.Background(), client,
			hookdeck.MetricsQueryParams{Measures: []string{"count", "failed_count", "error_rate"}})
		require.NoError(t, err)
		assert.Contains(t, *path, "metrics/events")
		assert.Equal(t, []string{"count", "failed_count", "error_rate"}, (*query)["measures[]"])
	})

	t.Run("queue depth route", func(t *testing.T) {
		client, path, query := routeCapture(t)
		_, err := queryEventMetricsConsolidated(context.Background(), client,
			hookdeck.MetricsQueryParams{Measures: []string{"queue_depth", "max_age"}})
		require.NoError(t, err)
		assert.Contains(t, *path, "queue-depth")
		assert.Equal(t, []string{"max_depth", "max_age"}, (*query)["measures[]"])
	})

	t.Run("a measure this package does not know does not trigger the guard", func(t *testing.T) {
		client, path, _ := routeCapture(t)
		_, err := queryEventMetricsConsolidated(context.Background(), client,
			hookdeck.MetricsQueryParams{Measures: []string{"count", "not_a_measure"}})
		require.NoError(t, err, "an unknown measure is the API's to reject, with a better message")
		assert.Contains(t, *path, "metrics/events")
	})
}

// TestCrossRouteMeasureAndDimensionIsRejected pins #407. The routing conditions
// are ordered and the first match wins, so a queue-depth measure decided the
// endpoint and the issue_id dimension went to /metrics/queue-depth, which does
// not group by issue: the command exited 0 having answered a different
// question. "pending" shadowed it in exactly the same way.
func TestCrossRouteMeasureAndDimensionIsRejected(t *testing.T) {
	tests := []struct {
		name     string
		params   hookdeck.MetricsQueryParams
		contains []string
	}{
		{
			name: "queue depth measure with the issue_id dimension",
			params: hookdeck.MetricsQueryParams{
				Measures:   []string{"queue_depth"},
				Dimensions: []string{"issue_id"},
				IssueID:    "iss_1",
			},
			contains: []string{"--measures", `"queue_depth"`, "queue depth metrics", "--dimensions", `"issue_id"`, "per-issue event metrics"},
		},
		{
			name: "the dimension conflicts even without the filter",
			params: hookdeck.MetricsQueryParams{
				Measures:   []string{"max_age"},
				Dimensions: []string{"issue_id"},
			},
			contains: []string{`"max_age"`, "queue depth metrics", "per-issue event metrics"},
		},
		{
			name: "the --issue-id filter selects the route on its own",
			params: hookdeck.MetricsQueryParams{
				Measures: []string{"queue_depth"},
				IssueID:  "iss_1",
			},
			contains: []string{`"queue_depth"`, "queue depth metrics", "--issue-id", "per-issue event metrics"},
		},
		{
			name: "pending shadows the issue route the same way",
			params: hookdeck.MetricsQueryParams{
				Measures:   []string{"pending"},
				Dimensions: []string{"issue_id"},
				IssueID:    "iss_1",
			},
			contains: []string{`"pending"`, "pending event metrics", "per-issue event metrics"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, path, _ := routeCapture(t)

			_, err := queryEventMetricsConsolidated(context.Background(), client, tt.params)
			require.Error(t, err, "one route's numbers must not be returned under another route's question")
			for _, want := range tt.contains {
				assert.Contains(t, err.Error(), want)
			}
			assert.Empty(t, *path, "the API must not be called at all")
		})
	}
}

// TestCompatibleMeasureAndDimensionStillRoute is the other half: the default
// route is what the by-issue endpoint refines, not what it contradicts, and a
// dimension that selects no route of its own must not trip the guard.
func TestCompatibleMeasureAndDimensionStillRoute(t *testing.T) {
	t.Run("a default-route measure grouped by issue goes to the issue endpoint", func(t *testing.T) {
		client, path, query := routeCapture(t)
		_, err := queryEventMetricsConsolidated(context.Background(), client, hookdeck.MetricsQueryParams{
			Measures:   []string{"count"},
			Dimensions: []string{"issue_id"},
			IssueID:    "iss_1",
		})
		require.NoError(t, err)
		assert.Contains(t, *path, "events-by-issue")
		assert.Equal(t, []string{"count"}, (*query)["measures[]"])
	})

	t.Run("queue depth grouped by a dimension that selects no route still routes", func(t *testing.T) {
		client, path, _ := routeCapture(t)
		_, err := queryEventMetricsConsolidated(context.Background(), client, hookdeck.MetricsQueryParams{
			Measures:   []string{"queue_depth"},
			Dimensions: []string{"destination_id"},
		})
		require.NoError(t, err)
		assert.Contains(t, *path, "queue-depth")
	})

	t.Run("an unknown measure leaves the issue route to the dimension", func(t *testing.T) {
		client, path, _ := routeCapture(t)
		_, err := queryEventMetricsConsolidated(context.Background(), client, hookdeck.MetricsQueryParams{
			Measures:   []string{"not_a_measure"},
			Dimensions: []string{"issue_id"},
			IssueID:    "iss_1",
		})
		require.NoError(t, err, "a measure this package does not know must not decide the route")
		assert.Contains(t, *path, "events-by-issue")
	})
}
