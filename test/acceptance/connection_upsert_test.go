package acceptance

import (
	"strings"
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
			"gateway", "connection", "create",
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
			"gateway", "connection", "upsert", connName,
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
			"gateway", "connection", "create",
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
			"gateway", "connection", "upsert", connName,
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
			"gateway", "connection", "create",
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
			"gateway", "connection", "upsert", connName,
			"--destination-auth-method", "bearer",
			"--destination-bearer-token", "test_token_123",
		)
		require.NoError(t, err, "Should upsert connection with only auth-method flags")

		// Verify auth was updated
		updatedDest, ok := upsertResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination object")
		updatedDestConfig, ok := updatedDest["config"].(map[string]interface{})
		require.True(t, ok, "Expected destination config")

		assert.Equal(t, "BEARER_TOKEN", updatedDestConfig["auth_type"], "Auth type should be BEARER_TOKEN")

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
			"gateway", "connection", "create",
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
			"gateway", "connection", "upsert", connName,
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

	// Regression test for https://github.com/hookdeck/hookdeck-cli/issues/209 Bug 1:
	// Upserting with only rule flags on a connection with destination auth should NOT
	// send auth_type without credentials.
	t.Run("UpsertRulesOnlyPreservesDestinationAuth", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-upsert-rules-auth-" + timestamp
		sourceName := "test-upsert-src-ra-" + timestamp
		destName := "test-upsert-dst-ra-" + timestamp

		// Create a connection WITH destination authentication (bearer token)
		var createResp map[string]interface{}
		err := cli.RunJSON(&createResp,
			"gateway", "connection", "create",
			"--name", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--destination-type", "HTTP",
			"--destination-name", destName,
			"--destination-url", "https://api.example.com/webhook",
			"--destination-auth-method", "bearer",
			"--destination-bearer-token", "test_secret_token_123",
		)
		require.NoError(t, err, "Should create connection with bearer auth")

		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID")

		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		// Verify auth was set
		dest, ok := createResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination object")
		destConfig, ok := dest["config"].(map[string]interface{})
		require.True(t, ok, "Expected destination config")
		assert.Equal(t, "BEARER_TOKEN", destConfig["auth_type"], "Auth type should be BEARER_TOKEN after creation")

		// Upsert with ONLY rule flags (no source/destination flags)
		var upsertResp map[string]interface{}
		err = cli.RunJSON(&upsertResp,
			"gateway", "connection", "upsert", connName,
			"--rule-retry-strategy", "linear",
			"--rule-retry-count", "3",
			"--rule-retry-interval", "5000",
		)
		require.NoError(t, err, "Should upsert with only rule flags without auth_type error")

		// Verify rules were applied
		rules, ok := upsertResp["rules"].([]interface{})
		require.True(t, ok, "Expected rules array")

		foundRetry := false
		for _, r := range rules {
			rule, ok := r.(map[string]interface{})
			if ok && rule["type"] == "retry" {
				foundRetry = true
				assert.Equal(t, "linear", rule["strategy"])
				break
			}
		}
		assert.True(t, foundRetry, "Should have a retry rule")

		// Verify auth was preserved
		upsertDest, ok := upsertResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination in upsert response")
		upsertDestConfig, ok := upsertDest["config"].(map[string]interface{})
		require.True(t, ok, "Expected destination config in upsert response")
		assert.Equal(t, "BEARER_TOKEN", upsertDestConfig["auth_type"], "Auth type should still be BEARER_TOKEN")

		t.Logf("Successfully upserted connection %s with only rule flags, auth preserved", connID)
	})

	// Regression test for https://github.com/hookdeck/hookdeck-cli/issues/209 Bug 3:
	// --rule-retry-response-status-codes must be sent as an array, not a string.
	t.Run("UpsertRetryResponseStatusCodesAsArray", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-upsert-statuscodes-" + timestamp
		sourceName := "test-upsert-src-sc-" + timestamp
		destName := "test-upsert-dst-sc-" + timestamp

		// Create a connection with full source/dest so the upsert provides all required fields
		var upsertResp map[string]interface{}
		err := cli.RunJSON(&upsertResp,
			"gateway", "connection", "upsert", connName,
			"--source-name", sourceName,
			"--source-type", "WEBHOOK",
			"--destination-name", destName,
			"--destination-type", "HTTP",
			"--destination-url", "https://api.example.com/webhook",
			"--rule-retry-strategy", "linear",
			"--rule-retry-count", "3",
			"--rule-retry-interval", "5000",
			"--rule-retry-response-status-codes", "500,502,503,504",
		)
		require.NoError(t, err, "Should upsert with retry response status codes as array")

		connID, _ := upsertResp["id"].(string)
		t.Cleanup(func() {
			if connID != "" {
				deleteConnection(t, cli, connID)
			}
		})

		// Verify the retry rule has status codes as an array
		rules, ok := upsertResp["rules"].([]interface{})
		require.True(t, ok, "Expected rules array")

		foundRetry := false
		for _, r := range rules {
			rule, ok := r.(map[string]interface{})
			if ok && rule["type"] == "retry" {
				foundRetry = true

				statusCodes, ok := rule["response_status_codes"].([]interface{})
				require.True(t, ok, "response_status_codes should be array, got: %T (%v)", rule["response_status_codes"], rule["response_status_codes"])
				assert.Len(t, statusCodes, 4, "Should have 4 status codes")

				codes := make([]string, len(statusCodes))
				for i, c := range statusCodes {
					codes[i] = strings.TrimSpace(c.(string))
				}
				assert.Contains(t, codes, "500")
				assert.Contains(t, codes, "502")
				assert.Contains(t, codes, "503")
				assert.Contains(t, codes, "504")
				break
			}
		}
		assert.True(t, foundRetry, "Should have a retry rule")

		t.Logf("Successfully verified retry status codes sent as array")
	})

	// Regression test for https://github.com/hookdeck/hookdeck-cli/issues/209 Bug 2:
	// Upserting with --source-name alone (without --source-type) should work for
	// existing connections (the existing type is preserved).
	t.Run("UpsertSourceNameWithoutTypeOnExistingConnection", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-upsert-srconly-" + timestamp
		sourceName := "test-upsert-src-so-" + timestamp
		destName := "test-upsert-dst-so-" + timestamp
		newSourceName := "test-upsert-newsrc-" + timestamp

		// Create a connection first
		var createResp map[string]interface{}
		err := cli.RunJSON(&createResp,
			"gateway", "connection", "create",
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

		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		// Upsert with --source-name only (no --source-type)
		// Previously this failed with "both --source-name and --source-type are required"
		var upsertResp map[string]interface{}
		err = cli.RunJSON(&upsertResp,
			"gateway", "connection", "upsert", connName,
			"--source-name", newSourceName,
		)
		require.NoError(t, err, "Should upsert with --source-name only on existing connection")

		// Verify the source was updated
		upsertSource, ok := upsertResp["source"].(map[string]interface{})
		require.True(t, ok, "Expected source in upsert response")
		assert.Equal(t, newSourceName, upsertSource["name"], "Source name should be updated")

		t.Logf("Successfully upserted connection %s with source-name only", connID)
	})
}
