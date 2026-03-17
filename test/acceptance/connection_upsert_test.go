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

	// Regression test for https://github.com/hookdeck/hookdeck-cli/issues/192:
	// --rule-filter-headers should store JSON as a parsed object with exact values preserved.
	t.Run("FilterHeadersJSONExactValues", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-filter-headers-" + timestamp
		sourceName := "test-filter-src-" + timestamp
		destName := "test-filter-dst-" + timestamp

		var createResp map[string]interface{}
		err := cli.RunJSON(&createResp,
			"gateway", "connection", "upsert", connName,
			"--source-name", sourceName,
			"--source-type", "WEBHOOK",
			"--destination-name", destName,
			"--destination-type", "HTTP",
			"--destination-url", "https://example.com/webhook",
			"--rule-filter-headers", `{"x-shopify-topic":{"$startsWith":"order/"},"content-type":"application/json"}`,
		)
		require.NoError(t, err, "Should create connection with --rule-filter-headers JSON")

		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID in response")

		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		source, ok := createResp["source"].(map[string]interface{})
		require.True(t, ok, "Expected source object in response")
		assert.Equal(t, sourceName, source["name"], "Source name should match")

		dest, ok := createResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination object in response")
		assert.Equal(t, destName, dest["name"], "Destination name should match")

		rules, ok := createResp["rules"].([]interface{})
		require.True(t, ok, "Expected rules array in response")

		foundFilter := false
		for _, r := range rules {
			rule, ok := r.(map[string]interface{})
			if !ok || rule["type"] != "filter" {
				continue
			}
			foundFilter = true

			headersMap, isMap := rule["headers"].(map[string]interface{})
			require.True(t, isMap,
				"headers should be a JSON object, got %T: %v", rule["headers"], rule["headers"])

			// Verify exact nested values
			topicVal, ok := headersMap["x-shopify-topic"].(map[string]interface{})
			require.True(t, ok, "x-shopify-topic should be a nested object, got %T", headersMap["x-shopify-topic"])
			assert.Equal(t, "order/", topicVal["$startsWith"],
				"$startsWith value should be exactly 'order/'")

			assert.Equal(t, "application/json", headersMap["content-type"],
				"content-type value should be exactly 'application/json'")
			break
		}
		assert.True(t, foundFilter, "Should have a filter rule")
	})

	// --rule-filter-body should store JSON as a parsed object with exact values preserved.
	t.Run("FilterBodyJSONExactValues", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-filter-body-" + timestamp
		sourceName := "test-filter-body-src-" + timestamp
		destName := "test-filter-body-dst-" + timestamp

		var createResp map[string]interface{}
		err := cli.RunJSON(&createResp,
			"gateway", "connection", "upsert", connName,
			"--source-name", sourceName,
			"--source-type", "WEBHOOK",
			"--destination-name", destName,
			"--destination-type", "HTTP",
			"--destination-url", "https://example.com/webhook",
			"--rule-filter-body", `{"event_type":"payment","amount":{"$gte":100}}`,
		)
		require.NoError(t, err, "Should create connection with --rule-filter-body JSON")

		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID in response")

		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		rules, ok := createResp["rules"].([]interface{})
		require.True(t, ok, "Expected rules array in response")

		foundFilter := false
		for _, r := range rules {
			rule, ok := r.(map[string]interface{})
			if !ok || rule["type"] != "filter" {
				continue
			}
			foundFilter = true

			bodyMap, isMap := rule["body"].(map[string]interface{})
			require.True(t, isMap, "body should be a JSON object, got %T: %v", rule["body"], rule["body"])

			assert.Equal(t, "payment", bodyMap["event_type"],
				"event_type should be exactly 'payment'")

			amountMap, ok := bodyMap["amount"].(map[string]interface{})
			require.True(t, ok, "amount should be a nested object, got %T", bodyMap["amount"])
			assert.Equal(t, float64(100), amountMap["$gte"],
				"$gte value should be exactly 100")
			break
		}
		assert.True(t, foundFilter, "Should have a filter rule")
	})

	// --rule-filter-query should store JSON as a parsed object with exact values preserved.
	t.Run("FilterQueryJSONExactValues", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-filter-query-" + timestamp
		sourceName := "test-filter-query-src-" + timestamp
		destName := "test-filter-query-dst-" + timestamp

		var createResp map[string]interface{}
		err := cli.RunJSON(&createResp,
			"gateway", "connection", "upsert", connName,
			"--source-name", sourceName,
			"--source-type", "WEBHOOK",
			"--destination-name", destName,
			"--destination-type", "HTTP",
			"--destination-url", "https://example.com/webhook",
			"--rule-filter-query", `{"status":"active","page":{"$gte":1}}`,
		)
		require.NoError(t, err, "Should create connection with --rule-filter-query JSON")

		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID in response")

		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		rules, ok := createResp["rules"].([]interface{})
		require.True(t, ok, "Expected rules array in response")

		foundFilter := false
		for _, r := range rules {
			rule, ok := r.(map[string]interface{})
			if !ok || rule["type"] != "filter" {
				continue
			}
			foundFilter = true

			queryMap, isMap := rule["query"].(map[string]interface{})
			require.True(t, isMap, "query should be a JSON object, got %T: %v", rule["query"], rule["query"])

			assert.Equal(t, "active", queryMap["status"],
				"status should be exactly 'active'")

			pageMap, ok := queryMap["page"].(map[string]interface{})
			require.True(t, ok, "page should be a nested object, got %T", queryMap["page"])
			assert.Equal(t, float64(1), pageMap["$gte"],
				"$gte value should be exactly 1")
			break
		}
		assert.True(t, foundFilter, "Should have a filter rule")
	})

	// --rule-filter-path should store JSON as a parsed object with exact values preserved.
	t.Run("FilterPathJSONExactValues", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-filter-path-" + timestamp
		sourceName := "test-filter-path-src-" + timestamp
		destName := "test-filter-path-dst-" + timestamp

		var createResp map[string]interface{}
		err := cli.RunJSON(&createResp,
			"gateway", "connection", "upsert", connName,
			"--source-name", sourceName,
			"--source-type", "WEBHOOK",
			"--destination-name", destName,
			"--destination-type", "HTTP",
			"--destination-url", "https://example.com/webhook",
			"--rule-filter-path", `{"$contains":"/webhooks/"}`,
		)
		require.NoError(t, err, "Should create connection with --rule-filter-path JSON")

		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID in response")

		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		rules, ok := createResp["rules"].([]interface{})
		require.True(t, ok, "Expected rules array in response")

		foundFilter := false
		for _, r := range rules {
			rule, ok := r.(map[string]interface{})
			if !ok || rule["type"] != "filter" {
				continue
			}
			foundFilter = true

			pathMap, isMap := rule["path"].(map[string]interface{})
			require.True(t, isMap, "path should be a JSON object, got %T: %v", rule["path"], rule["path"])

			assert.Equal(t, "/webhooks/", pathMap["$contains"],
				"$contains value should be exactly '/webhooks/'")
			break
		}
		assert.True(t, foundFilter, "Should have a filter rule")
	})

	// All four filter flags combined should produce a single filter rule with exact values.
	t.Run("AllFilterFlagsCombinedExactValues", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-filter-all-" + timestamp
		sourceName := "test-filter-all-src-" + timestamp
		destName := "test-filter-all-dst-" + timestamp

		var createResp map[string]interface{}
		err := cli.RunJSON(&createResp,
			"gateway", "connection", "upsert", connName,
			"--source-name", sourceName,
			"--source-type", "WEBHOOK",
			"--destination-name", destName,
			"--destination-type", "HTTP",
			"--destination-url", "https://example.com/webhook",
			"--rule-filter-headers", `{"content-type":"application/json"}`,
			"--rule-filter-body", `{"action":"created"}`,
			"--rule-filter-query", `{"verbose":"true"}`,
			"--rule-filter-path", `{"$startsWith":"/api/v1"}`,
		)
		require.NoError(t, err, "Should create connection with all four filter flags")

		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID in response")

		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		rules, ok := createResp["rules"].([]interface{})
		require.True(t, ok, "Expected rules array in response")

		foundFilter := false
		for _, r := range rules {
			rule, ok := r.(map[string]interface{})
			if !ok || rule["type"] != "filter" {
				continue
			}
			foundFilter = true

			// Verify headers
			headersMap, ok := rule["headers"].(map[string]interface{})
			require.True(t, ok, "headers should be a JSON object, got %T", rule["headers"])
			assert.Equal(t, "application/json", headersMap["content-type"])

			// Verify body
			bodyMap, ok := rule["body"].(map[string]interface{})
			require.True(t, ok, "body should be a JSON object, got %T", rule["body"])
			assert.Equal(t, "created", bodyMap["action"])

			// Verify query
			queryMap, ok := rule["query"].(map[string]interface{})
			require.True(t, ok, "query should be a JSON object, got %T", rule["query"])
			assert.Equal(t, "true", queryMap["verbose"])

			// Verify path
			pathMap, ok := rule["path"].(map[string]interface{})
			require.True(t, ok, "path should be a JSON object, got %T", rule["path"])
			assert.Equal(t, "/api/v1", pathMap["$startsWith"])
			break
		}
		assert.True(t, foundFilter, "Should have a filter rule")
	})

	// Verify retry status codes are returned as integer array with exact values.
	t.Run("RetryStatusCodesExactValues", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-retry-codes-" + timestamp
		sourceName := "test-retry-codes-src-" + timestamp
		destName := "test-retry-codes-dst-" + timestamp

		var createResp map[string]interface{}
		err := cli.RunJSON(&createResp,
			"gateway", "connection", "upsert", connName,
			"--source-name", sourceName,
			"--source-type", "WEBHOOK",
			"--destination-name", destName,
			"--destination-type", "HTTP",
			"--destination-url", "https://example.com/webhook",
			"--rule-retry-strategy", "linear",
			"--rule-retry-count", "5",
			"--rule-retry-interval", "10000",
			"--rule-retry-response-status-codes", "500,502,503,504",
		)
		require.NoError(t, err, "Should create connection with retry status codes")

		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID in response")

		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		rules, ok := createResp["rules"].([]interface{})
		require.True(t, ok, "Expected rules array in response")

		foundRetry := false
		for _, r := range rules {
			rule, ok := r.(map[string]interface{})
			if !ok || rule["type"] != "retry" {
				continue
			}
			foundRetry = true

			assert.Equal(t, "linear", rule["strategy"], "strategy should be 'linear'")
			assert.Equal(t, float64(5), rule["count"], "count should be 5")
			assert.Equal(t, float64(10000), rule["interval"], "interval should be 10000")

			statusCodes, ok := rule["response_status_codes"].([]interface{})
			require.True(t, ok, "response_status_codes should be an array, got %T: %v",
				rule["response_status_codes"], rule["response_status_codes"])
			require.Len(t, statusCodes, 4, "Should have exactly 4 status codes")

			expectedCodes := []float64{500, 502, 503, 504}
			for i, expected := range expectedCodes {
				actual, ok := statusCodes[i].(float64)
				require.True(t, ok, "status code [%d] should be a number, got %T", i, statusCodes[i])
				assert.Equal(t, expected, actual, "status code [%d] should be %v", i, expected)
			}
			break
		}
		assert.True(t, foundRetry, "Should have a retry rule")
	})
}
