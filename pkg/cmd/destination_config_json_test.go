package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildDestinationConfigFromJSONString verifies that --config (JSON string) parses
// into a config map with exact values preserved.
func TestBuildDestinationConfigFromJSONString(t *testing.T) {
	t.Run("HTTP config JSON with exact values", func(t *testing.T) {
		input := `{"url":"https://api.example.com/hooks","http_method":"PUT","delivery_policy":{"rate":100,"period":"second"}}`
		config, err := buildDestinationConfigFromFlags(input, "", "", nil)
		require.NoError(t, err)
		require.NotNil(t, config)

		assert.Equal(t, "https://api.example.com/hooks", config["url"])
		assert.Equal(t, "PUT", config["http_method"])
		policy, ok := config["delivery_policy"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, float64(100), policy["rate"])
		assert.Equal(t, "second", policy["period"])
	})

	t.Run("config with auth JSON preserves exact values", func(t *testing.T) {
		input := `{"url":"https://api.example.com","auth_type":"BEARER_TOKEN","auth":{"bearer_token":"sk-test-token-xyz"}}`
		config, err := buildDestinationConfigFromFlags(input, "", "", nil)
		require.NoError(t, err)
		require.NotNil(t, config)

		assert.Equal(t, "https://api.example.com", config["url"])
		assert.Equal(t, "BEARER_TOKEN", config["auth_type"])

		auth, ok := config["auth"].(map[string]interface{})
		require.True(t, ok, "auth should be a map, got %T", config["auth"])
		assert.Equal(t, "sk-test-token-xyz", auth["bearer_token"],
			"bearer_token value should be exactly 'sk-test-token-xyz'")
	})

	t.Run("config with nested custom signature preserves structure", func(t *testing.T) {
		input := `{"url":"https://api.example.com","auth_type":"CUSTOM_SIGNATURE","auth":{"secret":"sig_secret_123","key":"X-Signature"}}`
		config, err := buildDestinationConfigFromFlags(input, "", "", nil)
		require.NoError(t, err)
		require.NotNil(t, config)

		auth, ok := config["auth"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "sig_secret_123", auth["secret"])
		assert.Equal(t, "X-Signature", auth["key"])
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		_, err := buildDestinationConfigFromFlags(`{broken`, "", "", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--config")
	})
}

func TestBuildDestinationConfigFromIndividualFlagsDeliveryPolicy(t *testing.T) {
	flags := &destinationConfigFlags{
		RateLimit:               100,
		RateLimitPeriod:         "minute",
		DeliveryGroupKey:        "body.customer_id",
		DeliveryGroupRate:       5,
		DeliveryGroupRatePeriod: "second",
		DeliveryGroupOverrides:  `{"cus_priority":{"rate":50,"rate_period":"second"}}`,
	}

	config, err := buildDestinationConfigFromIndividualFlags("HTTP", flags)
	require.NoError(t, err)
	assert.NotContains(t, config, "rate_limit")
	assert.NotContains(t, config, "rate_limit_period")

	policy, ok := config["delivery_policy"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, 100, policy["rate"])
	assert.Equal(t, "minute", policy["period"])

	groups, ok := policy["groups"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "body.customer_id", groups["key"])
	assert.Equal(t, 5, groups["rate"])
	assert.Equal(t, "second", groups["rate_period"])
	overrides, ok := groups["overrides"].(map[string]interface{})
	require.True(t, ok)
	priority, ok := overrides["cus_priority"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(50), priority["rate"])
	assert.Equal(t, "second", priority["rate_period"])
}

func TestBuildDestinationConfigFromIndividualFlagsDeliveryGroupValidation(t *testing.T) {
	tests := []struct {
		name      string
		flags     destinationConfigFlags
		wantError string
	}{
		{name: "missing key", flags: destinationConfigFlags{DeliveryGroupRate: 5, DeliveryGroupRatePeriod: "second"}, wantError: "--delivery-group-key"},
		{name: "missing rate", flags: destinationConfigFlags{DeliveryGroupKey: "body.customer_id", DeliveryGroupRatePeriod: "second"}, wantError: "--delivery-group-rate"},
		{name: "missing period", flags: destinationConfigFlags{DeliveryGroupKey: "body.customer_id", DeliveryGroupRate: 5}, wantError: "--delivery-group-rate-period"},
		{name: "invalid overrides", flags: destinationConfigFlags{DeliveryGroupKey: "body.customer_id", DeliveryGroupRate: 5, DeliveryGroupRatePeriod: "second", DeliveryGroupOverrides: "[]"}, wantError: "--delivery-group-overrides"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildDestinationConfigFromIndividualFlags("HTTP", &tt.flags)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
		})
	}
}

// TestBuildDestinationConfigFromJSONFile verifies that --config-file reads a JSON file
// and produces a config map with exact values preserved.
func TestBuildDestinationConfigFromJSONFile(t *testing.T) {
	t.Run("file config JSON with exact values", func(t *testing.T) {
		content := `{"url":"https://file-based.example.com/hooks","http_method":"PATCH","delivery_policy":{"rate":50,"period":"minute"}}`
		tmpFile := filepath.Join(t.TempDir(), "dest-config.json")
		require.NoError(t, os.WriteFile(tmpFile, []byte(content), 0644))

		config, err := buildDestinationConfigFromFlags("", tmpFile, "", nil)
		require.NoError(t, err)
		require.NotNil(t, config)

		assert.Equal(t, "https://file-based.example.com/hooks", config["url"])
		assert.Equal(t, "PATCH", config["http_method"])
		policy, ok := config["delivery_policy"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, float64(50), policy["rate"])
		assert.Equal(t, "minute", policy["period"])
	})

	t.Run("file with auth config preserves exact values", func(t *testing.T) {
		content := `{"url":"https://api.example.com","auth_type":"API_KEY","auth":{"api_key":"key_from_file_789","header_key":"X-API-Key","to":"header"}}`
		tmpFile := filepath.Join(t.TempDir(), "dest-auth-config.json")
		require.NoError(t, os.WriteFile(tmpFile, []byte(content), 0644))

		config, err := buildDestinationConfigFromFlags("", tmpFile, "", nil)
		require.NoError(t, err)
		require.NotNil(t, config)

		assert.Equal(t, "API_KEY", config["auth_type"])
		auth, ok := config["auth"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "key_from_file_789", auth["api_key"])
		assert.Equal(t, "X-API-Key", auth["header_key"])
		assert.Equal(t, "header", auth["to"])
	})

	t.Run("nonexistent file returns error", func(t *testing.T) {
		_, err := buildDestinationConfigFromFlags("", "/nonexistent/path.json", "", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--config-file")
	})

	t.Run("invalid JSON file returns error", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "bad.json")
		require.NoError(t, os.WriteFile(tmpFile, []byte(`{not valid`), 0644))

		_, err := buildDestinationConfigFromFlags("", tmpFile, "", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "config file")
	})
}
