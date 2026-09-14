package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func groupsConfig(groups map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"delivery_policy": map[string]interface{}{"groups": groups},
	}
}

func storedOverrides() map[string]interface{} {
	return map[string]interface{}{
		"cust_1": map[string]interface{}{"rate": 5, "rate_period": "minute"},
	}
}

// TestPreserveDeliveryGroupOverrides pins the fix for the data loss in #393:
// the API replaces delivery_policy.groups wholesale, so an upsert that sends a
// groups object without overrides destroys the stored ones.
func TestPreserveDeliveryGroupOverrides(t *testing.T) {
	t.Run("stored overrides survive a rate bump", func(t *testing.T) {
		config := groupsConfig(map[string]interface{}{
			"key": "body.customer_id", "rate": 30, "rate_period": "second",
		})
		existing := groupsConfig(map[string]interface{}{
			"key": "body.customer_id", "rate": 10, "rate_period": "second",
			"overrides": storedOverrides(),
		})

		preserveDeliveryGroupOverrides(config, existing)

		groups, ok := nestedMap(config, "delivery_policy", "groups")
		require.True(t, ok)
		assert.Equal(t, storedOverrides(), groups["overrides"],
			"bumping the rate must not destroy per-group overrides")
		assert.Equal(t, 30, groups["rate"], "the requested change must still apply")
	})

	t.Run("explicit overrides win over stored ones", func(t *testing.T) {
		mine := map[string]interface{}{"cust_2": map[string]interface{}{"rate": 99}}
		config := groupsConfig(map[string]interface{}{"key": "k", "overrides": mine})
		preserveDeliveryGroupOverrides(config, groupsConfig(map[string]interface{}{
			"key": "k", "overrides": storedOverrides(),
		}))

		groups, _ := nestedMap(config, "delivery_policy", "groups")
		assert.Equal(t, mine, groups["overrides"])
	})

	t.Run("explicitly clearing overrides is honoured", func(t *testing.T) {
		empty := map[string]interface{}{}
		config := groupsConfig(map[string]interface{}{"key": "k", "overrides": empty})
		preserveDeliveryGroupOverrides(config, groupsConfig(map[string]interface{}{
			"key": "k", "overrides": storedOverrides(),
		}))

		groups, _ := nestedMap(config, "delivery_policy", "groups")
		assert.Equal(t, empty, groups["overrides"],
			"--delivery-group-overrides '{}' must clear, not be silently refilled")
	})

	t.Run("no stored overrides is a no-op", func(t *testing.T) {
		config := groupsConfig(map[string]interface{}{"key": "k", "rate": 30})
		preserveDeliveryGroupOverrides(config, groupsConfig(map[string]interface{}{"key": "k"}))

		groups, _ := nestedMap(config, "delivery_policy", "groups")
		_, has := groups["overrides"]
		assert.False(t, has)
	})

	t.Run("survives absent and malformed configs", func(t *testing.T) {
		assert.NotPanics(t, func() {
			preserveDeliveryGroupOverrides(nil, nil)
			preserveDeliveryGroupOverrides(map[string]interface{}{}, nil)
			preserveDeliveryGroupOverrides(
				groupsConfig(map[string]interface{}{"key": "k"}),
				map[string]interface{}{"delivery_policy": "not-a-map"},
			)
		})
	})
}

func TestDeliveryGroupsNeedOverrides(t *testing.T) {
	assert.True(t, deliveryGroupsNeedOverrides(groupsConfig(map[string]interface{}{"key": "k"})))
	assert.False(t, deliveryGroupsNeedOverrides(groupsConfig(map[string]interface{}{
		"key": "k", "overrides": map[string]interface{}{},
	})))
	// No groups object at all: a destination-level rate limit is merged safely
	// by the API, so nothing needs carrying forward.
	assert.False(t, deliveryGroupsNeedOverrides(map[string]interface{}{
		"delivery_policy": map[string]interface{}{"rate": 100, "period": "minute"},
	}))
	assert.False(t, deliveryGroupsNeedOverrides(nil))
}

// TestConnectionUpsertPreservesOverrides covers the path that already holds the
// existing destination, so no extra request is needed.
func TestConnectionUpsertPreservesOverrides(t *testing.T) {
	cu := &connectionUpsertCmd{connectionCreateCmd: &connectionCreateCmd{}}
	cu.DestinationDeliveryGroupKey = "body.customer_id"
	cu.DestinationDeliveryGroupRate = 30
	cu.DestinationDeliveryGroupRatePeriod = "second"

	input, err := cu.buildDestinationInputForUpdate(&hookdeck.Destination{
		ID: "des_1", Name: "web", Type: "HTTP",
		Config: groupsConfig(map[string]interface{}{
			"key": "body.customer_id", "rate": 10, "rate_period": "second",
			"overrides": storedOverrides(),
		}),
	})
	require.NoError(t, err)

	groups, ok := nestedMap(input.Config, "delivery_policy", "groups")
	require.True(t, ok)
	assert.Equal(t, storedOverrides(), groups["overrides"])
	assert.Equal(t, 30, groups["rate"])
}
