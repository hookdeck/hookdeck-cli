package hookdeck

import (
	"net/url"
	"testing"

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
