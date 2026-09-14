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
