package cmd

import (
	"context"
	"testing"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeliveryGroupFlagOnlyWhereTheAPIAcceptsIt covers the filter schemas the API
// actually declares: delivery_group exists on events, attempts and queue-depth,
// and not on requests or transformations. Those filters are additionalProperties:
// false, so offering the flag where it is not accepted turns a typo-level mistake
// into an opaque 422 from the server.
func TestDeliveryGroupFlagOnlyWhereTheAPIAcceptsIt(t *testing.T) {
	tests := []struct {
		name     string
		cmd      *cobra.Command
		expected bool
	}{
		{"events", newMetricsEventsCmd().cmd, true},
		{"attempts", newMetricsAttemptsCmd().cmd, true},
		{"requests", newMetricsRequestsCmd().cmd, false},
		{"transformations", newMetricsTransformationsCmd().cmd, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := tt.cmd.Flags().Lookup("delivery-group")
			if tt.expected {
				assert.NotNil(t, flag, "%s accepts delivery_group and should offer the flag", tt.name)
			} else {
				assert.Nil(t, flag, "%s rejects unknown filters; the flag must not be offered", tt.name)
			}
		})
	}
}

// TestEventMetricsRejectDeliveryGroupOnUnsupportedRoutes covers the two routes
// `metrics events` can take where the target endpoint has no delivery_group in
// its filter schema. The client must say so rather than let the API answer 422.
func TestEventMetricsRejectDeliveryGroupOnUnsupportedRoutes(t *testing.T) {
	t.Run("pending timeseries", func(t *testing.T) {
		_, err := queryEventMetricsConsolidated(context.Background(), nil, hookdeck.MetricsQueryParams{
			Measures:      []string{"pending"},
			Granularity:   "1h",
			DeliveryGroup: "dg_1",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--delivery-group")
	})

	t.Run("per-issue", func(t *testing.T) {
		_, err := queryEventMetricsConsolidated(context.Background(), nil, hookdeck.MetricsQueryParams{
			Dimensions:    []string{"issue_id"},
			IssueID:       "iss_1",
			DeliveryGroup: "dg_1",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--delivery-group")
	})

	t.Run("per-issue still reports the missing issue id first", func(t *testing.T) {
		_, err := queryEventMetricsConsolidated(context.Background(), nil, hookdeck.MetricsQueryParams{
			Dimensions:    []string{"issue_id"},
			DeliveryGroup: "dg_1",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--issue-id")
	})
}
