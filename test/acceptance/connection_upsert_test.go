package acceptance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConnectionUpsertPartialUpdates tests that upsert works with partial config updates
// This addresses the bug where updating only destination config (e.g., --destination-url)
// without providing source/destination name/type fails with 422 error
func TestConnectionUpsertPartialUpdates(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	t.Run("UpsertDestinationURLOnly", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-upsert-url-" + timestamp
		sourceName := "test-upsert-src-" + timestamp
		destName := "test-upsert-dst-" + timestamp
		initialURL := "https://api.example.com/initial"
		updatedURL := "https://api.example.com/updated"

		// Create initial connection
		var createResp map[string]interface{}
		err := cli.RunJSON(&createResp,
			"connection", "create",
			"--name", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--destination-type", "HTTP",
			"--destination-name", destName,
			"--destination-url", initialURL,
		)
		require.NoError(t, err, "Should create connection")

		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID in creation response")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		// Verify initial URL
		dest, ok := createResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination object")
		destConfig, ok := dest["config"].(map[string]interface{})
		require.True(t, ok, "Expected destination config")
		assert.Equal(t, initialURL, destConfig["url"], "Initial URL should match")

		t.Logf("Created connection %s with initial URL: %s", connID, initialURL)

		// Update ONLY the destination URL (this is the bug scenario)
		var upsertResp map[string]interface{}
		err = cli.RunJSON(&upsertResp,
			"connection", "upsert", connName,
			"--destination-url", updatedURL,
		)
		require.NoError(t, err, "Should upsert connection with only destination-url flag")

		// Verify the URL was updated
		updatedDest, ok := upsertResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination object in upsert response")
		updatedDestConfig, ok := updatedDest["config"].(map[string]interface{})
		require.True(t, ok, "Expected destination config in upsert response")
		assert.Equal(t, updatedURL, updatedDestConfig["url"], "URL should be updated")

		// Verify source was preserved
		updatedSource, ok := upsertResp["source"].(map[string]interface{})
		require.True(t, ok, "Expected source object in upsert response")
		assert.Equal(t, sourceName, updatedSource["name"], "Source should be preserved")

		t.Logf("Successfully updated connection %s URL to: %s", connID, updatedURL)
	})

	t.Run("UpsertDestinationHTTPMethod", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-upsert-method-" + timestamp
		sourceName := "test-upsert-src-" + timestamp
		destName := "test-upsert-dst-" + timestamp

		// Create initial connection (default HTTP method is POST)
		var createResp map[string]interface{}
		err := cli.RunJSON(&createResp,
			"connection", "create",
			"--name", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--destination-type", "HTTP",
			"--destination-name", destName,
			"--destination-url", "https://api.example.com/webhook",
		)
		require.NoError(t, err, "Should create connection")

		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		// Update ONLY the HTTP method
		var upsertResp map[string]interface{}
		err = cli.RunJSON(&upsertResp,
			"connection", "upsert", connName,
			"--destination-http-method", "PUT",
		)
		require.NoError(t, err, "Should upsert connection with only http-method flag")

		// Verify the method was updated
		updatedDest, ok := upsertResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination object")
		updatedDestConfig, ok := updatedDest["config"].(map[string]interface{})
		require.True(t, ok, "Expected destination config")
		assert.Equal(t, "PUT", updatedDestConfig["http_method"], "HTTP method should be updated")

		t.Logf("Successfully updated connection %s HTTP method to PUT", connID)
	})

	t.Run("UpsertDestinationAuthMethod", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-upsert-auth-" + timestamp
		sourceName := "test-upsert-src-" + timestamp
		destName := "test-upsert-dst-" + timestamp

		// Create initial connection without auth
		var createResp map[string]interface{}
		err := cli.RunJSON(&createResp,
			"connection", "create",
			"--name", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--destination-type", "HTTP",
			"--destination-name", destName,
			"--destination-url", "https://api.example.com/webhook",
		)
		require.NoError(t, err, "Should create connection")

		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		// Update ONLY the auth method
		var upsertResp map[string]interface{}
		err = cli.RunJSON(&upsertResp,
			"connection", "upsert", connName,
			"--destination-auth-method", "bearer",
			"--destination-bearer-token", "test_token_123",
		)
		require.NoError(t, err, "Should upsert connection with only auth-method flags")

		// Verify auth was updated
		updatedDest, ok := upsertResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination object")
		updatedDestConfig, ok := updatedDest["config"].(map[string]interface{})
		require.True(t, ok, "Expected destination config")

		if authMethod, ok := updatedDestConfig["auth_method"].(map[string]interface{}); ok {
			assert.Equal(t, "BEARER", authMethod["type"], "Auth type should be BEARER")
		}

		t.Logf("Successfully updated connection %s auth method to bearer", connID)
	})

	t.Run("UpsertSourceConfigFields", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-upsert-src-config-" + timestamp
		sourceName := "test-upsert-src-" + timestamp
		destName := "test-upsert-dst-" + timestamp

		// Create initial connection
		var createResp map[string]interface{}
		err := cli.RunJSON(&createResp,
			"connection", "create",
			"--name", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--destination-type", "CLI",
			"--destination-name", destName,
			"--destination-cli-path", "/webhooks",
		)
		require.NoError(t, err, "Should create connection")

		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		// Update ONLY source config fields
		var upsertResp map[string]interface{}
		err = cli.RunJSON(&upsertResp,
			"connection", "upsert", connName,
			"--source-allowed-http-methods", "POST,PUT",
			"--source-custom-response-content-type", "json",
			"--source-custom-response-body", `{"status":"ok"}`,
		)
		require.NoError(t, err, "Should upsert connection with only source config flags")

		// Verify source config was updated
		updatedSource, ok := upsertResp["source"].(map[string]interface{})
		require.True(t, ok, "Expected source object")
		updatedSourceConfig, ok := updatedSource["config"].(map[string]interface{})
		require.True(t, ok, "Expected source config")

		if allowedMethods, ok := updatedSourceConfig["allowed_http_methods"].([]interface{}); ok {
			assert.Len(t, allowedMethods, 2, "Should have 2 allowed HTTP methods")
		}

		t.Logf("Successfully updated connection %s source config", connID)
	})
}
