package acceptance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDestinationCreateWithConfigJSONExactValues verifies that destination --config (JSON string)
// sends the correct structure to the API and the returned resource preserves exact values.
func TestDestinationCreateWithConfigJSONExactValues(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	t.Run("HTTP destination with url and http_method config", func(t *testing.T) {
		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()
		name := "test-dst-cfg-json-" + timestamp

		var resp map[string]interface{}
		err := cli.RunJSON(&resp, "gateway", "destination", "create",
			"--name", name,
			"--type", "HTTP",
			"--config", `{"url":"https://api.example.com/webhooks","http_method":"PUT"}`,
		)
		require.NoError(t, err, "Should create destination with --config JSON")

		dstID, ok := resp["id"].(string)
		require.True(t, ok && dstID != "", "Expected destination ID")
		t.Cleanup(func() { deleteDestination(t, cli, dstID) })

		assert.Equal(t, name, resp["name"], "Destination name should match exactly")
		assert.Equal(t, "HTTP", resp["type"], "Destination type should be HTTP")

		config, ok := resp["config"].(map[string]interface{})
		require.True(t, ok, "Expected config object in response")
		assert.Equal(t, "https://api.example.com/webhooks", config["url"],
			"URL should match exactly")
		assert.Equal(t, "PUT", config["http_method"],
			"http_method should be 'PUT'")
	})

	t.Run("HTTP destination with rate limit config", func(t *testing.T) {
		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()
		name := "test-dst-cfg-rate-" + timestamp

		var resp map[string]interface{}
		err := cli.RunJSON(&resp, "gateway", "destination", "create",
			"--name", name,
			"--type", "HTTP",
			"--config", `{"url":"https://api.example.com/hooks","rate_limit":100,"rate_limit_period":"second"}`,
		)
		require.NoError(t, err, "Should create destination with rate limit config")

		dstID, ok := resp["id"].(string)
		require.True(t, ok && dstID != "", "Expected destination ID")
		t.Cleanup(func() { deleteDestination(t, cli, dstID) })

		config, ok := resp["config"].(map[string]interface{})
		require.True(t, ok, "Expected config object in response")
		assert.Equal(t, "https://api.example.com/hooks", config["url"])
		assert.Equal(t, float64(100), config["rate_limit"],
			"rate_limit should be exactly 100")
		assert.Equal(t, "second", config["rate_limit_period"],
			"rate_limit_period should be 'second'")
	})
}

// TestDestinationCreateWithConfigFileExactValues verifies that destination --config-file
// reads JSON from a file and the returned resource preserves exact values.
func TestDestinationCreateWithConfigFileExactValues(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()
	name := "test-dst-cfg-file-" + timestamp

	configContent := `{"url":"https://file-config.example.com/hooks","http_method":"PATCH"}`
	tmpFile := filepath.Join(t.TempDir(), "dest-config.json")
	require.NoError(t, os.WriteFile(tmpFile, []byte(configContent), 0644))

	var resp map[string]interface{}
	err := cli.RunJSON(&resp, "gateway", "destination", "create",
		"--name", name,
		"--type", "HTTP",
		"--config-file", tmpFile,
	)
	require.NoError(t, err, "Should create destination with --config-file")

	dstID, ok := resp["id"].(string)
	require.True(t, ok && dstID != "", "Expected destination ID")
	t.Cleanup(func() { deleteDestination(t, cli, dstID) })

	config, ok := resp["config"].(map[string]interface{})
	require.True(t, ok, "Expected config object in response")
	assert.Equal(t, "https://file-config.example.com/hooks", config["url"])
	assert.Equal(t, "PATCH", config["http_method"])
}

// TestDestinationUpsertWithConfigJSONExactValues verifies that destination upsert --config (JSON)
// creates/updates with exact values preserved in the API response.
func TestDestinationUpsertWithConfigJSONExactValues(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()
	name := "test-dst-upsert-cfg-" + timestamp

	// Create via upsert
	var resp map[string]interface{}
	err := cli.RunJSON(&resp, "gateway", "destination", "upsert", name,
		"--type", "HTTP",
		"--config", `{"url":"https://upsert-config.example.com/v1","http_method":"POST"}`,
	)
	require.NoError(t, err, "Should upsert destination with --config JSON")

	dstID, ok := resp["id"].(string)
	require.True(t, ok && dstID != "", "Expected destination ID")
	t.Cleanup(func() { deleteDestination(t, cli, dstID) })

	config, ok := resp["config"].(map[string]interface{})
	require.True(t, ok, "Expected config in response")
	assert.Equal(t, "https://upsert-config.example.com/v1", config["url"])
	assert.Equal(t, "POST", config["http_method"])

	// Update via upsert with new config
	var resp2 map[string]interface{}
	err = cli.RunJSON(&resp2, "gateway", "destination", "upsert", name,
		"--config", `{"url":"https://upsert-config.example.com/v2","http_method":"PUT"}`,
	)
	require.NoError(t, err, "Should upsert destination with updated --config JSON")

	config2, ok := resp2["config"].(map[string]interface{})
	require.True(t, ok, "Expected config in upsert update response")
	assert.Equal(t, "https://upsert-config.example.com/v2", config2["url"],
		"URL should be updated to v2")
	assert.Equal(t, "PUT", config2["http_method"],
		"http_method should be updated to PUT")
}
