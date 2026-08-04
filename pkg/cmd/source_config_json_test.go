package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildSourceConfigFromJSONString verifies that --config (JSON string) parses
// into a config map with exact values preserved.
func TestBuildSourceConfigFromJSONString(t *testing.T) {
	t.Run("simple config JSON with exact values", func(t *testing.T) {
		input := `{"allowed_http_methods":["POST","PUT"],"custom_response":{"content_type":"json","body":"{\"ok\":true}"}}`
		config, err := buildSourceConfigFromFlags(input, "", nil, "WEBHOOK")
		require.NoError(t, err)
		require.NotNil(t, config)

		methods, ok := config["allowed_http_methods"].([]interface{})
		require.True(t, ok, "allowed_http_methods should be an array, got %T", config["allowed_http_methods"])
		assert.Equal(t, []interface{}{"POST", "PUT"}, methods)

		customResp, ok := config["custom_response"].(map[string]interface{})
		require.True(t, ok, "custom_response should be a map, got %T", config["custom_response"])
		assert.Equal(t, "json", customResp["content_type"])
		assert.Equal(t, `{"ok":true}`, customResp["body"])
	})

	t.Run("auth config JSON with exact values", func(t *testing.T) {
		input := `{"webhook_secret":"whsec_test_abc123"}`
		config, err := buildSourceConfigFromFlags(input, "", nil, "STRIPE")
		require.NoError(t, err)
		require.NotNil(t, config)

		// normalizeSourceConfigAuth may transform this, but the value should be preserved
		// Check that the secret value is present somewhere in the config
		auth, hasAuth := config["auth"].(map[string]interface{})
		if hasAuth {
			assert.Equal(t, "whsec_test_abc123", auth["webhook_secret_key"],
				"webhook_secret_key should be exactly 'whsec_test_abc123'")
		} else {
			// If not normalized, original key should be present
			assert.Equal(t, "whsec_test_abc123", config["webhook_secret"],
				"webhook_secret should be exactly 'whsec_test_abc123'")
		}
	})

	t.Run("nested config JSON preserves structure", func(t *testing.T) {
		input := `{"auth":{"webhook_secret_key":"whsec_nested_123"},"custom_response":{"content_type":"xml","body":"<ok/>"}}`
		config, err := buildSourceConfigFromFlags(input, "", nil, "STRIPE")
		require.NoError(t, err)
		require.NotNil(t, config)

		auth, ok := config["auth"].(map[string]interface{})
		require.True(t, ok, "auth should be a map, got %T", config["auth"])
		assert.Equal(t, "whsec_nested_123", auth["webhook_secret_key"])

		customResp, ok := config["custom_response"].(map[string]interface{})
		require.True(t, ok, "custom_response should be a map")
		assert.Equal(t, "xml", customResp["content_type"])
		assert.Equal(t, "<ok/>", customResp["body"])
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		_, err := buildSourceConfigFromFlags(`{invalid`, "", nil, "WEBHOOK")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--config")
	})
}

// TestBuildSourceConfigFromJSONFile verifies that --config-file reads a JSON file
// and produces a config map with exact values preserved.
func TestBuildSourceConfigFromJSONFile(t *testing.T) {
	t.Run("file config JSON with exact values", func(t *testing.T) {
		content := `{"allowed_http_methods":["GET","POST"],"custom_response":{"content_type":"text","body":"received"}}`
		tmpFile := filepath.Join(t.TempDir(), "source-config.json")
		require.NoError(t, os.WriteFile(tmpFile, []byte(content), 0644))

		config, err := buildSourceConfigFromFlags("", tmpFile, nil, "WEBHOOK")
		require.NoError(t, err)
		require.NotNil(t, config)

		methods, ok := config["allowed_http_methods"].([]interface{})
		require.True(t, ok, "allowed_http_methods should be an array")
		assert.Equal(t, []interface{}{"GET", "POST"}, methods)

		customResp, ok := config["custom_response"].(map[string]interface{})
		require.True(t, ok, "custom_response should be a map")
		assert.Equal(t, "text", customResp["content_type"])
		assert.Equal(t, "received", customResp["body"])
	})

	t.Run("file with auth config preserves exact values", func(t *testing.T) {
		content := `{"webhook_secret":"whsec_file_test_456"}`
		tmpFile := filepath.Join(t.TempDir(), "source-auth-config.json")
		require.NoError(t, os.WriteFile(tmpFile, []byte(content), 0644))

		config, err := buildSourceConfigFromFlags("", tmpFile, nil, "STRIPE")
		require.NoError(t, err)
		require.NotNil(t, config)

		// After normalization, secret should be preserved
		auth, hasAuth := config["auth"].(map[string]interface{})
		if hasAuth {
			assert.Equal(t, "whsec_file_test_456", auth["webhook_secret_key"])
		} else {
			assert.Equal(t, "whsec_file_test_456", config["webhook_secret"])
		}
	})

	t.Run("nonexistent file returns error", func(t *testing.T) {
		_, err := buildSourceConfigFromFlags("", "/nonexistent/path.json", nil, "WEBHOOK")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--config-file")
	})

	t.Run("invalid JSON file returns error", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "bad.json")
		require.NoError(t, os.WriteFile(tmpFile, []byte(`{not json`), 0644))

		_, err := buildSourceConfigFromFlags("", tmpFile, nil, "WEBHOOK")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "config file")
	})
}
