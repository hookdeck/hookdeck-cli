package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApplyCLIPath pins the precedence between --cli-path and a path supplied
// via --config. Create previously overwrote the --config value with "/".
func TestApplyCLIPath(t *testing.T) {
	t.Run("--config path survives when --cli-path is absent", func(t *testing.T) {
		config := map[string]interface{}{"path": "/from-config"}
		applyCLIPath(config, "", true)
		assert.Equal(t, "/from-config", config["path"])
	})

	t.Run("--cli-path wins over --config", func(t *testing.T) {
		config := map[string]interface{}{"path": "/from-config"}
		applyCLIPath(config, "/from-flag", true)
		assert.Equal(t, "/from-flag", config["path"])
	})

	t.Run("create defaults to / when neither is given", func(t *testing.T) {
		config := map[string]interface{}{}
		applyCLIPath(config, "", true)
		assert.Equal(t, "/", config["path"])
	})

	t.Run("upsert leaves path absent when neither is given", func(t *testing.T) {
		config := map[string]interface{}{}
		applyCLIPath(config, "", false)
		_, ok := config["path"]
		assert.False(t, ok, "upsert must not invent a path, or a partial update would reset it")
	})
}

// TestRejectDeliveryPolicyForCLI pins that delivery-policy flags are refused for
// CLI destinations. The API accepts the request and discards the policy, so
// without this the flags look applied but never take effect.
func TestRejectDeliveryPolicyForCLI(t *testing.T) {
	policy := map[string]interface{}{"rate": 100, "period": "minute"}

	t.Run("CLI destination is rejected", func(t *testing.T) {
		err := rejectDeliveryPolicyForCLI("CLI", policy, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--rate-limit")
		assert.Contains(t, err.Error(), "CLI destinations")
	})

	t.Run("connection flag prefix is named in the message", func(t *testing.T) {
		err := rejectDeliveryPolicyForCLI("cli", policy, "destination-")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--destination-rate-limit")
	})

	t.Run("HTTP and MOCK_API are unaffected", func(t *testing.T) {
		assert.NoError(t, rejectDeliveryPolicyForCLI("HTTP", policy, ""))
		assert.NoError(t, rejectDeliveryPolicyForCLI("MOCK_API", policy, ""))
	})

	t.Run("CLI without a policy is fine", func(t *testing.T) {
		assert.NoError(t, rejectDeliveryPolicyForCLI("CLI", map[string]interface{}{}, ""))
	})
}

// TestBuildDestinationConfigRejectsDeliveryPolicyForCLI covers the funnel that
// destination create/update/upsert all go through.
func TestBuildDestinationConfigRejectsDeliveryPolicyForCLI(t *testing.T) {
	flags := &destinationConfigFlags{RateLimit: 100, RateLimitPeriod: "minute"}
	_, err := buildDestinationConfigFromIndividualFlags("CLI", flags)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CLI destinations")

	// Same flags on HTTP still build a policy.
	config, err := buildDestinationConfigFromIndividualFlags("HTTP", flags)
	require.NoError(t, err)
	policy, ok := config["delivery_policy"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, 100, policy["rate"])
}
