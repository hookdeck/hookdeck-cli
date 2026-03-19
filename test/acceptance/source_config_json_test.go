//go:build source

package acceptance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSourceCreateWithConfigJSONExactValues verifies that source --config (JSON string)
// sends the correct structure to the API and the returned resource preserves exact values.
func TestSourceCreateWithConfigJSONExactValues(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	t.Run("STRIPE source with webhook_secret config", func(t *testing.T) {
		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()
		name := "test-src-cfg-json-" + timestamp

		var resp map[string]interface{}
		err := cli.RunJSON(&resp, "gateway", "source", "create",
			"--name", name,
			"--type", "STRIPE",
			"--config", `{"webhook_secret":"whsec_exact_test_123"}`,
		)
		require.NoError(t, err, "Should create source with --config JSON")

		srcID, ok := resp["id"].(string)
		require.True(t, ok && srcID != "", "Expected source ID")
		t.Cleanup(func() { deleteSource(t, cli, srcID) })

		assert.Equal(t, name, resp["name"], "Source name should match exactly")
		assert.Equal(t, "STRIPE", resp["type"], "Source type should be STRIPE")
	})

	t.Run("WEBHOOK source with allowed_http_methods config", func(t *testing.T) {
		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()
		name := "test-src-cfg-methods-" + timestamp

		var resp map[string]interface{}
		err := cli.RunJSON(&resp, "gateway", "source", "create",
			"--name", name,
			"--type", "WEBHOOK",
			"--config", `{"allowed_http_methods":["POST","PUT"]}`,
		)
		require.NoError(t, err, "Should create source with allowed_http_methods config")

		srcID, ok := resp["id"].(string)
		require.True(t, ok && srcID != "", "Expected source ID")
		t.Cleanup(func() { deleteSource(t, cli, srcID) })

		config, ok := resp["config"].(map[string]interface{})
		require.True(t, ok, "Expected config object in response")

		methods, ok := config["allowed_http_methods"].([]interface{})
		require.True(t, ok, "allowed_http_methods should be an array, got %T", config["allowed_http_methods"])
		assert.Contains(t, methods, "POST", "Should contain POST")
		assert.Contains(t, methods, "PUT", "Should contain PUT")
	})
}

// TestSourceCreateWithConfigFileExactValues verifies that source --config-file
// reads JSON from a file and the returned resource preserves exact values.
func TestSourceCreateWithConfigFileExactValues(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()
	name := "test-src-cfg-file-" + timestamp

	configContent := `{"allowed_http_methods":["GET","POST","PUT"]}`
	tmpFile := filepath.Join(t.TempDir(), "source-config.json")
	require.NoError(t, os.WriteFile(tmpFile, []byte(configContent), 0644))

	var resp map[string]interface{}
	err := cli.RunJSON(&resp, "gateway", "source", "create",
		"--name", name,
		"--type", "WEBHOOK",
		"--config-file", tmpFile,
	)
	require.NoError(t, err, "Should create source with --config-file")

	srcID, ok := resp["id"].(string)
	require.True(t, ok && srcID != "", "Expected source ID")
	t.Cleanup(func() { deleteSource(t, cli, srcID) })

	config, ok := resp["config"].(map[string]interface{})
	require.True(t, ok, "Expected config object in response")

	methods, ok := config["allowed_http_methods"].([]interface{})
	require.True(t, ok, "allowed_http_methods should be an array")
	assert.Len(t, methods, 3, "Should have exactly 3 HTTP methods")
	assert.Contains(t, methods, "GET")
	assert.Contains(t, methods, "POST")
	assert.Contains(t, methods, "PUT")
}

// TestSourceUpsertWithConfigJSONExactValues verifies that source upsert --config (JSON)
// creates/updates with exact values preserved in the API response.
func TestSourceUpsertWithConfigJSONExactValues(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()
	name := "test-src-upsert-cfg-" + timestamp

	var resp map[string]interface{}
	err := cli.RunJSON(&resp, "gateway", "source", "upsert", name,
		"--type", "WEBHOOK",
		"--config", `{"allowed_http_methods":["POST"],"custom_response":{"content_type":"json","body":"{\"status\":\"ok\"}"}}`,
	)
	require.NoError(t, err, "Should upsert source with --config JSON")

	srcID, ok := resp["id"].(string)
	require.True(t, ok && srcID != "", "Expected source ID")
	t.Cleanup(func() { deleteSource(t, cli, srcID) })

	config, ok := resp["config"].(map[string]interface{})
	require.True(t, ok, "Expected config in response")

	methods, ok := config["allowed_http_methods"].([]interface{})
	require.True(t, ok, "allowed_http_methods should be an array")
	assert.Equal(t, []interface{}{"POST"}, methods)

	customResp, ok := config["custom_response"].(map[string]interface{})
	require.True(t, ok, "custom_response should be a map, got %T", config["custom_response"])
	assert.Equal(t, "json", customResp["content_type"], "content_type should be 'json'")
	assert.Equal(t, `{"status":"ok"}`, customResp["body"], "body should match exactly")
}
