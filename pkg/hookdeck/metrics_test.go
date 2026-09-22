package hookdeck

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildMetricsQueryIncludesDeliveryGroup(t *testing.T) {
	query, err := url.ParseQuery(buildMetricsQuery(MetricsQueryParams{
		Start:         "2026-09-01T00:00:00Z",
		End:           "2026-09-02T00:00:00Z",
		DeliveryGroup: "cus_priority",
		Dimensions:    []string{"delivery_group"},
	}))
	require.NoError(t, err)
	require.Equal(t, "cus_priority", query.Get("filters[delivery_group]"))
	require.Equal(t, []string{"delivery_group"}, query["dimensions[]"])
}

// TestDefaultEventRouteHonoursEveryFilterExceptIssueID pins the invariant that
// makes the default events route need no filter gate of its own.
//
// `metrics events` routes on measures, then the issue_id dimension, then the
// issue filter, and only then falls through to the default route - so a set
// IssueID never reaches that fallback. IssueID is also the only filter
// DefaultEventRouteFilters withholds from the union the callers advertise,
// which leaves nothing for a default-route gate to catch; the gate that used to
// sit there could not fire.
//
// If a new filter is added that /metrics/events does not honour, this test
// fails and says so: the default route then needs its gate back.
func TestDefaultEventRouteHonoursEveryFilterExceptIssueID(t *testing.T) {
	want := EventMetricsFilters
	want.IssueID = false
	assert.Equal(t, want, DefaultEventRouteFilters,
		"the default events route must honour every advertised filter except issue_id; "+
			"a filter it drops needs a gate in queryEventMetricsConsolidated and metricsEvents")

	// The union is what --help and the tool schema offer, so a filter missing
	// from it is one no caller can pass.
	assert.Equal(t, MetricsFilters{SourceID: true, DestinationID: true, ConnectionID: true, Status: true, IssueID: true, DeliveryGroup: true},
		EventMetricsFilters)
}
