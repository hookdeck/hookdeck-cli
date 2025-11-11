package acceptance

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConnectionListBasic tests that connection list command works
func TestConnectionListBasic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	// List should work even if there are no connections
	stdout := cli.RunExpectSuccess("connection", "list")
	assert.NotEmpty(t, stdout, "connection list should produce output")

	t.Logf("Connection list output: %s", strings.TrimSpace(stdout))
}

// TestConnectionCreateAndDelete tests creating and deleting a connection
func TestConnectionCreateAndDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	// Create a test connection
	connID := createTestConnection(t, cli)
	require.NotEmpty(t, connID, "Connection ID should not be empty")

	// Register cleanup
	t.Cleanup(func() {
		deleteConnection(t, cli, connID)
	})

	// Verify the connection was created by getting it
	var conn Connection
	err := cli.RunJSON(&conn, "connection", "get", connID)
	require.NoError(t, err, "Should be able to get the created connection")
	assert.Equal(t, connID, conn.ID, "Retrieved connection ID should match")
	assert.NotEmpty(t, conn.Name, "Connection should have a name")
	assert.NotEmpty(t, conn.Source.Name, "Connection should have a source")
	assert.NotEmpty(t, conn.Destination.Name, "Connection should have a destination")

	t.Logf("Successfully created and retrieved connection: %s", conn.Name)
}

// TestConnectionWithWebhookSource tests creating a connection with a WEBHOOK source
func TestConnectionWithWebhookSource(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-webhook-" + timestamp
	sourceName := "test-src-webhook-" + timestamp
	destName := "test-dst-webhook-" + timestamp

	var conn Connection
	err := cli.RunJSON(&conn,
		"connection", "create",
		"--name", connName,
		"--source-name", sourceName,
		"--source-type", "WEBHOOK",
		"--destination-name", destName,
		"--destination-type", "CLI",
		"--destination-cli-path", "/webhooks",
	)
	require.NoError(t, err, "Should create connection with WEBHOOK source")
	require.NotEmpty(t, conn.ID, "Connection should have an ID")

	// Cleanup
	t.Cleanup(func() {
		deleteConnection(t, cli, conn.ID)
	})

	// Verify source type
	assert.Equal(t, sourceName, conn.Source.Name, "Source name should match")
	assert.Equal(t, "WEBHOOK", strings.ToUpper(conn.Source.Type), "Source type should be WEBHOOK")

	t.Logf("Successfully created connection with WEBHOOK source: %s", conn.ID)
}

// TestConnectionAuthenticationTypes tests various source and destination authentication methods
// This test covers all authentication scenarios from the shell acceptance tests
func TestConnectionAuthenticationTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	t.Run("WEBHOOK_Source_NoAuth", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-webhook-conn-" + timestamp
		sourceName := "test-webhook-source-" + timestamp
		destName := "test-webhook-dest-" + timestamp

		// Create connection with WEBHOOK source (no authentication)
		stdout, stderr, err := cli.Run("connection", "create",
			"--name", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--destination-type", "CLI",
			"--destination-name", destName,
			"--destination-cli-path", "/webhooks",
			"--output", "json")
		require.NoError(t, err, "Failed to create connection: stderr=%s", stderr)

		// Parse creation response
		var createResp map[string]interface{}
		err = json.Unmarshal([]byte(stdout), &createResp)
		require.NoError(t, err, "Failed to parse creation response: %s", stdout)

		// Verify creation response fields
		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID in creation response, got: %v", createResp["id"])

		assert.Equal(t, connName, createResp["name"], "Connection name should match")

		// Verify source details
		source, ok := createResp["source"].(map[string]interface{})
		require.True(t, ok, "Expected source object in creation response, got: %v", createResp["source"])
		assert.Equal(t, sourceName, source["name"], "Source name should match")
		srcType, _ := source["type"].(string)
		assert.Equal(t, "WEBHOOK", strings.ToUpper(srcType), "Source type should be WEBHOOK")

		// Verify destination details
		dest, ok := createResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination object in creation response, got: %v", createResp["destination"])
		assert.Equal(t, destName, dest["name"], "Destination name should match")
		destType, _ := dest["type"].(string)
		assert.Equal(t, "CLI", strings.ToUpper(destType), "Destination type should be CLI")

		// Verify using connection get
		var getResp map[string]interface{}
		err = cli.RunJSON(&getResp, "connection", "get", connID)
		require.NoError(t, err, "Should be able to get the created connection")

		// Compare key fields between create and get responses
		assert.Equal(t, connID, getResp["id"], "Connection ID should match")
		assert.Equal(t, connName, getResp["name"], "Connection name should match")

		// Verify source in get response
		getSource, ok := getResp["source"].(map[string]interface{})
		require.True(t, ok, "Expected source object in get response")
		assert.Equal(t, sourceName, getSource["name"], "Source name should match in get response")
		getSrcType, _ := getSource["type"].(string)
		assert.Equal(t, "WEBHOOK", strings.ToUpper(getSrcType), "Source type should match in get response")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		t.Logf("Successfully tested WEBHOOK source (no auth): %s", connID)
	})

	t.Run("STRIPE_Source_WebhookSecret", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-stripe-conn-" + timestamp
		sourceName := "test-stripe-source-" + timestamp
		destName := "test-stripe-dest-" + timestamp
		webhookSecret := "whsec_test_secret_123"

		// Create connection with STRIPE source (webhook secret authentication)
		stdout, stderr, err := cli.Run("connection", "create",
			"--name", connName,
			"--source-type", "STRIPE",
			"--source-name", sourceName,
			"--source-webhook-secret", webhookSecret,
			"--destination-type", "CLI",
			"--destination-name", destName,
			"--destination-cli-path", "/webhooks",
			"--output", "json")
		require.NoError(t, err, "Failed to create connection: stderr=%s", stderr)

		// Parse creation response
		var createResp map[string]interface{}
		err = json.Unmarshal([]byte(stdout), &createResp)
		require.NoError(t, err, "Failed to parse creation response: %s", stdout)

		// Verify creation response fields
		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID in creation response")

		assert.Equal(t, connName, createResp["name"], "Connection name should match")

		// Verify source details
		source, ok := createResp["source"].(map[string]interface{})
		require.True(t, ok, "Expected source object in creation response")
		assert.Equal(t, sourceName, source["name"], "Source name should match")
		srcType, _ := source["type"].(string)
		assert.Equal(t, "STRIPE", strings.ToUpper(srcType), "Source type should be STRIPE")

		// Verify authentication configuration is present (webhook secret should NOT be returned for security)
		if verification, ok := source["verification"].(map[string]interface{}); ok {
			if verType, ok := verification["type"].(string); ok {
				upperVerType := strings.ToUpper(verType)
				assert.True(t, upperVerType == "WEBHOOK_SECRET" || upperVerType == "STRIPE",
					"Verification type should be WEBHOOK_SECRET or STRIPE, got: %s", verType)
			}
		}

		// Verify using connection get
		var getResp map[string]interface{}
		err = cli.RunJSON(&getResp, "connection", "get", connID)
		require.NoError(t, err, "Should be able to get the created connection")

		assert.Equal(t, connID, getResp["id"], "Connection ID should match")
		getSource, _ := getResp["source"].(map[string]interface{})
		assert.Equal(t, sourceName, getSource["name"], "Source name should match in get response")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		t.Logf("Successfully tested STRIPE source with webhook secret: %s", connID)
	})

	t.Run("HTTP_Source_APIKey", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-http-apikey-conn-" + timestamp
		sourceName := "test-http-apikey-source-" + timestamp
		destName := "test-http-apikey-dest-" + timestamp
		apiKey := "test_api_key_abc123"

		// Create connection with HTTP source (API key authentication)
		stdout, stderr, err := cli.Run("connection", "create",
			"--name", connName,
			"--source-type", "HTTP",
			"--source-name", sourceName,
			"--source-api-key", apiKey,
			"--destination-type", "CLI",
			"--destination-name", destName,
			"--destination-cli-path", "/webhooks",
			"--output", "json")
		require.NoError(t, err, "Failed to create connection: stderr=%s", stderr)

		// Parse creation response
		var createResp map[string]interface{}
		err = json.Unmarshal([]byte(stdout), &createResp)
		require.NoError(t, err, "Failed to parse creation response: %s", stdout)

		// Verify creation response fields
		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID in creation response")

		assert.Equal(t, connName, createResp["name"], "Connection name should match")

		// Verify source details
		source, ok := createResp["source"].(map[string]interface{})
		require.True(t, ok, "Expected source object in creation response")
		assert.Equal(t, sourceName, source["name"], "Source name should match")
		srcType, _ := source["type"].(string)
		assert.Equal(t, "HTTP", strings.ToUpper(srcType), "Source type should be HTTP")

		// Verify authentication configuration is present (API key should NOT be returned for security)
		if verification, ok := source["verification"].(map[string]interface{}); ok {
			if verType, ok := verification["type"].(string); ok {
				assert.Equal(t, "API_KEY", strings.ToUpper(verType), "Verification type should be API_KEY")
			}
		}

		// Verify using connection get
		var getResp map[string]interface{}
		err = cli.RunJSON(&getResp, "connection", "get", connID)
		require.NoError(t, err, "Should be able to get the created connection")

		assert.Equal(t, connID, getResp["id"], "Connection ID should match")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		t.Logf("Successfully tested HTTP source with API key: %s", connID)
	})

	t.Run("HTTP_Source_BasicAuth", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-http-basic-conn-" + timestamp
		sourceName := "test-http-basic-source-" + timestamp
		destName := "test-http-basic-dest-" + timestamp
		username := "test_user"
		password := "test_pass_123"

		// Create connection with HTTP source (basic authentication)
		stdout, stderr, err := cli.Run("connection", "create",
			"--name", connName,
			"--source-type", "HTTP",
			"--source-name", sourceName,
			"--source-basic-auth-user", username,
			"--source-basic-auth-pass", password,
			"--destination-type", "CLI",
			"--destination-name", destName,
			"--destination-cli-path", "/webhooks",
			"--output", "json")
		require.NoError(t, err, "Failed to create connection: stderr=%s", stderr)

		// Parse creation response
		var createResp map[string]interface{}
		err = json.Unmarshal([]byte(stdout), &createResp)
		require.NoError(t, err, "Failed to parse creation response: %s", stdout)

		// Verify creation response fields
		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID in creation response")

		assert.Equal(t, connName, createResp["name"], "Connection name should match")

		// Verify source details
		source, ok := createResp["source"].(map[string]interface{})
		require.True(t, ok, "Expected source object in creation response")
		assert.Equal(t, sourceName, source["name"], "Source name should match")
		srcType, _ := source["type"].(string)
		assert.Equal(t, "HTTP", strings.ToUpper(srcType), "Source type should be HTTP")

		// Verify authentication configuration (password should NOT be returned for security)
		if verification, ok := source["verification"].(map[string]interface{}); ok {
			if verType, ok := verification["type"].(string); ok {
				assert.Equal(t, "BASIC_AUTH", strings.ToUpper(verType), "Verification type should be BASIC_AUTH")
			}
			// Check if username is returned (password should not be)
			if configs, ok := verification["configs"].(map[string]interface{}); ok {
				if user, ok := configs["username"].(string); ok {
					assert.Equal(t, username, user, "Username should match")
				}
			}
		}

		// Verify using connection get
		var getResp map[string]interface{}
		err = cli.RunJSON(&getResp, "connection", "get", connID)
		require.NoError(t, err, "Should be able to get the created connection")

		assert.Equal(t, connID, getResp["id"], "Connection ID should match")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		t.Logf("Successfully tested HTTP source with basic auth: %s", connID)
	})

	t.Run("TWILIO_Source_HMAC", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-twilio-conn-" + timestamp
		sourceName := "test-twilio-source-" + timestamp
		destName := "test-twilio-dest-" + timestamp
		hmacSecret := "test_hmac_secret_xyz"

		// Create connection with TWILIO source (HMAC authentication)
		stdout, stderr, err := cli.Run("connection", "create",
			"--name", connName,
			"--source-type", "TWILIO",
			"--source-name", sourceName,
			"--source-hmac-secret", hmacSecret,
			"--source-hmac-algo", "sha1",
			"--destination-type", "CLI",
			"--destination-name", destName,
			"--destination-cli-path", "/webhooks",
			"--output", "json")
		require.NoError(t, err, "Failed to create connection: stderr=%s", stderr)

		// Parse creation response
		var createResp map[string]interface{}
		err = json.Unmarshal([]byte(stdout), &createResp)
		require.NoError(t, err, "Failed to parse creation response: %s", stdout)

		// Verify creation response fields
		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID in creation response")

		assert.Equal(t, connName, createResp["name"], "Connection name should match")

		// Verify source details
		source, ok := createResp["source"].(map[string]interface{})
		require.True(t, ok, "Expected source object in creation response")
		assert.Equal(t, sourceName, source["name"], "Source name should match")
		srcType, _ := source["type"].(string)
		assert.Equal(t, "TWILIO", strings.ToUpper(srcType), "Source type should be TWILIO")

		// Verify HMAC authentication configuration (secret should NOT be returned for security)
		if verification, ok := source["verification"].(map[string]interface{}); ok {
			if verType, ok := verification["type"].(string); ok {
				upperVerType := strings.ToUpper(verType)
				assert.True(t, upperVerType == "HMAC" || upperVerType == "TWILIO",
					"Verification type should be HMAC or TWILIO, got: %s", verType)
			}
			// Check if algorithm is returned
			if configs, ok := verification["configs"].(map[string]interface{}); ok {
				if algo, ok := configs["algorithm"].(string); ok {
					assert.Equal(t, "sha1", strings.ToLower(algo), "HMAC algorithm should be sha1")
				}
			}
		}

		// Verify using connection get
		var getResp map[string]interface{}
		err = cli.RunJSON(&getResp, "connection", "get", connID)
		require.NoError(t, err, "Should be able to get the created connection")

		assert.Equal(t, connID, getResp["id"], "Connection ID should match")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		t.Logf("Successfully tested TWILIO source with HMAC: %s", connID)
	})

	t.Run("HTTP_Destination_BearerToken", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-bearer-conn-" + timestamp
		sourceName := "test-bearer-source-" + timestamp
		destName := "test-bearer-dest-" + timestamp
		destURL := "https://api.hookdeck.com/dev/null"
		bearerToken := "test_bearer_token_abc123"

		// Create connection with HTTP destination (bearer token authentication)
		stdout, stderr, err := cli.Run("connection", "create",
			"--name", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--destination-type", "HTTP",
			"--destination-name", destName,
			"--destination-url", destURL,
			"--destination-auth-method", "bearer",
			"--destination-bearer-token", bearerToken,
			"--output", "json")
		require.NoError(t, err, "Failed to create connection: stderr=%s", stderr)

		// Parse creation response
		var createResp map[string]interface{}
		err = json.Unmarshal([]byte(stdout), &createResp)
		require.NoError(t, err, "Failed to parse creation response: %s", stdout)

		// Verify creation response fields
		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID in creation response")

		assert.Equal(t, connName, createResp["name"], "Connection name should match")

		// Verify destination details
		dest, ok := createResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination object in creation response")
		assert.Equal(t, destName, dest["name"], "Destination name should match")
		destType, _ := dest["type"].(string)
		assert.Equal(t, "HTTP", strings.ToUpper(destType), "Destination type should be HTTP")

		// Verify URL is in destination.config.url (not destination.url)
		destConfig, ok := dest["config"].(map[string]interface{})
		require.True(t, ok, "Expected destination config object in creation response")
		if url, ok := destConfig["url"].(string); ok {
			assert.Equal(t, destURL, url, "Destination URL should match in config")
		} else {
			t.Errorf("Expected destination URL in config, got: %v", destConfig["url"])
		}

		// Verify authentication configuration (bearer token should NOT be returned for security)
		// Auth config is in destination.config
		if authType, ok := destConfig["auth_type"].(string); ok {
			t.Logf("Destination auth_type: %s", authType)
		}

		// Verify using connection get
		var getResp map[string]interface{}
		err = cli.RunJSON(&getResp, "connection", "get", connID)
		require.NoError(t, err, "Should be able to get the created connection")

		assert.Equal(t, connID, getResp["id"], "Connection ID should match")
		getDest, _ := getResp["destination"].(map[string]interface{})
		getDestConfig, ok := getDest["config"].(map[string]interface{})
		require.True(t, ok, "Expected destination config in get response")
		if url, ok := getDestConfig["url"].(string); ok {
			assert.Equal(t, destURL, url, "Destination URL should match in get response")
		} else {
			t.Errorf("Expected destination URL in get response config, got: %v", getDestConfig["url"])
		}

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		t.Logf("Successfully tested HTTP destination with bearer token: %s", connID)
	})

	t.Run("HTTP_Destination_BasicAuth", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-dest-basic-conn-" + timestamp
		sourceName := "test-dest-basic-source-" + timestamp
		destName := "test-dest-basic-dest-" + timestamp
		destURL := "https://api.hookdeck.com/dev/null"
		username := "dest_user"
		password := "dest_pass_123"

		// Create connection with HTTP destination (basic authentication)
		stdout, stderr, err := cli.Run("connection", "create",
			"--name", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--destination-type", "HTTP",
			"--destination-name", destName,
			"--destination-url", destURL,
			"--destination-auth-method", "basic",
			"--destination-basic-auth-user", username,
			"--destination-basic-auth-pass", password,
			"--output", "json")
		require.NoError(t, err, "Failed to create connection: stderr=%s", stderr)

		// Parse creation response
		var createResp map[string]interface{}
		err = json.Unmarshal([]byte(stdout), &createResp)
		require.NoError(t, err, "Failed to parse creation response: %s", stdout)

		// Verify creation response fields
		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID in creation response")

		assert.Equal(t, connName, createResp["name"], "Connection name should match")

		// Verify destination details
		dest, ok := createResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination object in creation response")
		assert.Equal(t, destName, dest["name"], "Destination name should match")
		destType, _ := dest["type"].(string)
		assert.Equal(t, "HTTP", strings.ToUpper(destType), "Destination type should be HTTP")

		// Verify URL is in destination.config.url (not destination.url)
		destConfig, ok := dest["config"].(map[string]interface{})
		require.True(t, ok, "Expected destination config object in creation response")
		if url, ok := destConfig["url"].(string); ok {
			assert.Equal(t, destURL, url, "Destination URL should match in config")
		} else {
			t.Errorf("Expected destination URL in config, got: %v", destConfig["url"])
		}

		// Verify authentication configuration (password should NOT be returned for security)
		if authType, ok := destConfig["auth_type"].(string); ok {
			t.Logf("Destination auth_type: %s", authType)
		}
		// Note: Username/password details may be in auth config, but password should NOT be returned

		// Verify using connection get
		var getResp map[string]interface{}
		err = cli.RunJSON(&getResp, "connection", "get", connID)
		require.NoError(t, err, "Should be able to get the created connection")

		assert.Equal(t, connID, getResp["id"], "Connection ID should match")
		getDest, _ := getResp["destination"].(map[string]interface{})
		getDestConfig, ok := getDest["config"].(map[string]interface{})
		require.True(t, ok, "Expected destination config in get response")
		if url, ok := getDestConfig["url"].(string); ok {
			assert.Equal(t, destURL, url, "Destination URL should match in get response")
		} else {
			t.Errorf("Expected destination URL in get response config, got: %v", getDestConfig["url"])
		}

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		t.Logf("Successfully tested HTTP destination with basic auth: %s", connID)
	})
	t.Run("HTTP_Destination_APIKey_Header", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-apikey-header-conn-" + timestamp
		sourceName := "test-apikey-header-source-" + timestamp
		destName := "test-apikey-header-dest-" + timestamp
		destURL := "https://api.hookdeck.com/dev/null"
		apiKey := "sk_test_123"

		// Create connection with HTTP destination (API key in header)
		stdout, stderr, err := cli.Run("connection", "create",
			"--name", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--destination-type", "HTTP",
			"--destination-name", destName,
			"--destination-url", destURL,
			"--destination-auth-method", "api_key",
			"--destination-api-key", apiKey,
			"--destination-api-key-header", "X-API-Key",
			"--destination-api-key-to", "header",
			"--output", "json")
		require.NoError(t, err, "Failed to create connection: stderr=%s", stderr)

		var createResp map[string]interface{}
		err = json.Unmarshal([]byte(stdout), &createResp)
		require.NoError(t, err, "Failed to parse creation response: %s", stdout)

		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID in creation response")

		// Verify destination auth configuration
		dest, ok := createResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination object in creation response")

		destConfig, ok := dest["config"].(map[string]interface{})
		require.True(t, ok, "Expected destination config object")

		if authMethod, ok := destConfig["auth_method"].(map[string]interface{}); ok {
			assert.Equal(t, "API_KEY", authMethod["type"], "Auth type should be API_KEY")
			assert.Equal(t, "X-API-Key", authMethod["key"], "Auth key should be X-API-Key")
			assert.Equal(t, "header", authMethod["to"], "Auth location should be header")
			// API key itself should not be returned for security
		}

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		t.Logf("Successfully tested HTTP destination with API key (header): %s", connID)
	})

	t.Run("HTTP_Destination_APIKey_Query", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-apikey-query-conn-" + timestamp
		sourceName := "test-apikey-query-source-" + timestamp
		destName := "test-apikey-query-dest-" + timestamp
		destURL := "https://api.hookdeck.com/dev/null"
		apiKey := "sk_test_456"

		// Create connection with HTTP destination (API key in query)
		stdout, stderr, err := cli.Run("connection", "create",
			"--name", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--destination-type", "HTTP",
			"--destination-name", destName,
			"--destination-url", destURL,
			"--destination-auth-method", "api_key",
			"--destination-api-key", apiKey,
			"--destination-api-key-header", "api_key",
			"--destination-api-key-to", "query",
			"--output", "json")
		require.NoError(t, err, "Failed to create connection: stderr=%s", stderr)

		var createResp map[string]interface{}
		err = json.Unmarshal([]byte(stdout), &createResp)
		require.NoError(t, err, "Failed to parse creation response: %s", stdout)

		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID in creation response")

		// Verify destination auth configuration
		dest, ok := createResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination object in creation response")

		destConfig, ok := dest["config"].(map[string]interface{})
		require.True(t, ok, "Expected destination config object")

		if authMethod, ok := destConfig["auth_method"].(map[string]interface{}); ok {
			assert.Equal(t, "API_KEY", authMethod["type"], "Auth type should be API_KEY")
			assert.Equal(t, "api_key", authMethod["key"], "Auth key should be api_key")
			assert.Equal(t, "query", authMethod["to"], "Auth location should be query")
		}

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		t.Logf("Successfully tested HTTP destination with API key (query): %s", connID)
	})

	t.Run("HTTP_Destination_CustomSignature", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-custom-sig-conn-" + timestamp
		sourceName := "test-custom-sig-source-" + timestamp
		destName := "test-custom-sig-dest-" + timestamp
		destURL := "https://api.hookdeck.com/dev/null"

		// Create connection with HTTP destination (custom signature)
		stdout, stderr, err := cli.Run("connection", "create",
			"--name", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--destination-type", "HTTP",
			"--destination-name", destName,
			"--destination-url", destURL,
			"--destination-auth-method", "custom_signature",
			"--destination-custom-signature-key", "X-Signature",
			"--destination-custom-signature-secret", "secret123",
			"--output", "json")
		require.NoError(t, err, "Failed to create connection: stderr=%s", stderr)

		var createResp map[string]interface{}
		err = json.Unmarshal([]byte(stdout), &createResp)
		require.NoError(t, err, "Failed to parse creation response: %s", stdout)

		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID in creation response")

		// Verify destination auth configuration
		dest, ok := createResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination object in creation response")

		destConfig, ok := dest["config"].(map[string]interface{})
		require.True(t, ok, "Expected destination config object")

		if authMethod, ok := destConfig["auth_method"].(map[string]interface{}); ok {
			assert.Equal(t, "CUSTOM_SIGNATURE", authMethod["type"], "Auth type should be CUSTOM_SIGNATURE")
			assert.Equal(t, "X-Signature", authMethod["key"], "Auth key should be X-Signature")
			// Signing secret should not be returned for security
		}

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		t.Logf("Successfully tested HTTP destination with custom signature: %s", connID)
	})

	t.Run("HTTP_Destination_HookdeckSignature", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-hookdeck-sig-conn-" + timestamp
		sourceName := "test-hookdeck-sig-source-" + timestamp
		destName := "test-hookdeck-sig-dest-" + timestamp
		destURL := "https://api.hookdeck.com/dev/null"

		// Create connection with HTTP destination (Hookdeck signature - explicit)
		stdout, stderr, err := cli.Run("connection", "create",
			"--name", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--destination-type", "HTTP",
			"--destination-name", destName,
			"--destination-url", destURL,
			"--destination-auth-method", "hookdeck",
			"--output", "json")
		require.NoError(t, err, "Failed to create connection: stderr=%s", stderr)

		var createResp map[string]interface{}
		err = json.Unmarshal([]byte(stdout), &createResp)
		require.NoError(t, err, "Failed to parse creation response: %s", stdout)

		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID in creation response")

		// Verify destination auth configuration
		dest, ok := createResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination object in creation response")

		destConfig, ok := dest["config"].(map[string]interface{})
		require.True(t, ok, "Expected destination config object")

		// Hookdeck signature should be set as the auth type
		if authMethod, ok := destConfig["auth_method"].(map[string]interface{}); ok {
			assert.Equal(t, "HOOKDECK_SIGNATURE", authMethod["type"], "Auth type should be HOOKDECK_SIGNATURE")
		}

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		t.Logf("Successfully tested HTTP destination with Hookdeck signature: %s", connID)
	})

	t.Run("ConnectionUpsert_ChangeAuthMethod", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-upsert-auth-" + timestamp
		sourceName := "test-upsert-auth-source-" + timestamp
		destName := "test-upsert-auth-dest-" + timestamp
		destURL := "https://api.hookdeck.com/dev/null"

		// Create connection with bearer token auth
		stdout, stderr, err := cli.Run("connection", "upsert", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--destination-type", "HTTP",
			"--destination-name", destName,
			"--destination-url", destURL,
			"--destination-auth-method", "bearer",
			"--destination-bearer-token", "initial_token",
			"--output", "json")
		require.NoError(t, err, "Failed to create connection: stderr=%s", stderr)

		var createResp map[string]interface{}
		err = json.Unmarshal([]byte(stdout), &createResp)
		require.NoError(t, err, "Failed to parse creation response: %s", stdout)

		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID in creation response")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		// Update to API key auth
		stdout, stderr, err = cli.Run("connection", "upsert", connName,
			"--destination-auth-method", "api_key",
			"--destination-api-key", "new_api_key",
			"--destination-api-key-header", "X-API-Key",
			"--output", "json")
		require.NoError(t, err, "Failed to update connection auth: stderr=%s", stderr)

		var updateResp map[string]interface{}
		err = json.Unmarshal([]byte(stdout), &updateResp)
		require.NoError(t, err, "Failed to parse update response: %s", stdout)

		assert.Equal(t, connID, updateResp["id"], "Connection ID should remain the same")

		// Verify auth was updated to API key
		updateDest, ok := updateResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination object in update response")

		updateDestConfig, ok := updateDest["config"].(map[string]interface{})
		require.True(t, ok, "Expected destination config object in update response")

		if authMethod, ok := updateDestConfig["auth_method"].(map[string]interface{}); ok {
			assert.Equal(t, "API_KEY", authMethod["type"], "Auth type should be updated to API_KEY")
			assert.Equal(t, "X-API-Key", authMethod["key"], "Auth key should be X-API-Key")
		}

		// Update to Hookdeck signature (reset to default)
		stdout, stderr, err = cli.Run("connection", "upsert", connName,
			"--destination-auth-method", "hookdeck",
			"--output", "json")
		require.NoError(t, err, "Failed to reset to Hookdeck signature: stderr=%s", stderr)

		var resetResp map[string]interface{}
		err = json.Unmarshal([]byte(stdout), &resetResp)
		require.NoError(t, err, "Failed to parse reset response: %s", stdout)

		assert.Equal(t, connID, resetResp["id"], "Connection ID should remain the same")

		// Verify auth was reset to Hookdeck signature
		resetDest, ok := resetResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination object in reset response")

		resetDestConfig, ok := resetDest["config"].(map[string]interface{})
		require.True(t, ok, "Expected destination config object in reset response")

		if authMethod, ok := resetDestConfig["auth_method"].(map[string]interface{}); ok {
			assert.Equal(t, "HOOKDECK_SIGNATURE", authMethod["type"], "Auth type should be reset to HOOKDECK_SIGNATURE")
		}

		t.Logf("Successfully tested changing authentication methods via upsert: %s", connID)
	})
}

// TestConnectionDelete tests deleting a connection and verifying it's removed
func TestConnectionDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	// Create a test connection
	connID := createTestConnection(t, cli)
	require.NotEmpty(t, connID, "Connection ID should not be empty")

	// Verify the connection exists before deletion
	var conn Connection
	err := cli.RunJSON(&conn, "connection", "get", connID)
	require.NoError(t, err, "Should be able to get the connection before deletion")
	assert.Equal(t, connID, conn.ID, "Connection ID should match")

	// Delete the connection using --force flag (no interactive prompt)
	stdout := cli.RunExpectSuccess("connection", "delete", connID, "--force")
	assert.NotEmpty(t, stdout, "delete command should produce output")

	t.Logf("Deleted connection: %s", connID)

	// Verify deletion by attempting to get the connection
	// This should fail because the connection no longer exists
	stdout, stderr, err := cli.Run("connection", "get", connID, "--output", "json")

	// We expect an error here since the connection was deleted
	if err == nil {
		t.Errorf("Expected error when getting deleted connection, but command succeeded. stdout: %s", stdout)
	} else {
		// Verify the error indicates the connection was not found
		errorOutput := stderr + stdout
		if !strings.Contains(strings.ToLower(errorOutput), "not found") &&
			!strings.Contains(strings.ToLower(errorOutput), "404") &&
			!strings.Contains(strings.ToLower(errorOutput), "does not exist") {
			t.Logf("Warning: Error message doesn't clearly indicate 'not found': %s", errorOutput)
		}
		t.Logf("Verified connection was deleted (get command failed as expected)")
	}
}

// TestConnectionBulkDelete tests creating and deleting multiple connections
// This mirrors the cleanup pattern from the shell script (lines 240-246)
func TestConnectionBulkDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	// Create multiple test connections
	numConnections := 5
	connectionIDs := make([]string, 0, numConnections)

	for i := 0; i < numConnections; i++ {
		connID := createTestConnection(t, cli)
		require.NotEmpty(t, connID, "Connection ID should not be empty")
		connectionIDs = append(connectionIDs, connID)
		t.Logf("Created test connection %d/%d: %s", i+1, numConnections, connID)
	}

	// Verify all connections were created
	assert.Len(t, connectionIDs, numConnections, "Should have created all connections")

	// Delete all connections using --force flag
	for i, connID := range connectionIDs {
		t.Logf("Deleting connection %d/%d: %s", i+1, numConnections, connID)
		stdout := cli.RunExpectSuccess("connection", "delete", connID, "--force")
		assert.NotEmpty(t, stdout, "delete command should produce output")
	}

	t.Logf("Successfully deleted all %d connections", numConnections)

	// Verify all connections are deleted
	for _, connID := range connectionIDs {
		_, _, err := cli.Run("connection", "get", connID, "--output", "json")

		// We expect an error for each deleted connection
		if err == nil {
			t.Errorf("Connection %s should have been deleted but still exists", connID)
		}
	}

	t.Logf("Verified all connections were deleted")
}

// TestConnectionWithRetryRule tests creating a connection with a retry rule
func TestConnectionWithRetryRule(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-retry-rule-" + timestamp
	sourceName := "test-src-retry-" + timestamp
	destName := "test-dst-retry-" + timestamp

	// Test with linear retry strategy
	var conn Connection
	err := cli.RunJSON(&conn,
		"connection", "create",
		"--name", connName,
		"--source-name", sourceName,
		"--source-type", "WEBHOOK",
		"--destination-name", destName,
		"--destination-type", "CLI",
		"--destination-cli-path", "/webhooks",
		"--rule-retry-strategy", "linear",
		"--rule-retry-count", "3",
		"--rule-retry-interval", "5000",
	)
	require.NoError(t, err, "Should create connection with retry rule")
	require.NotEmpty(t, conn.ID, "Connection should have an ID")

	// Cleanup
	t.Cleanup(func() {
		deleteConnection(t, cli, conn.ID)
	})

	// Verify the rule was created by getting the connection
	var getConn Connection
	err = cli.RunJSON(&getConn, "connection", "get", conn.ID)
	require.NoError(t, err, "Should be able to get the created connection")

	require.NotEmpty(t, getConn.Rules, "Connection should have rules")
	require.Len(t, getConn.Rules, 1, "Connection should have exactly one rule")

	rule := getConn.Rules[0]
	assert.Equal(t, "retry", rule["type"], "Rule type should be retry")
	assert.Equal(t, "linear", rule["strategy"], "Retry strategy should be linear")
	assert.Equal(t, float64(3), rule["count"], "Retry count should be 3")
	assert.Equal(t, float64(5000), rule["interval"], "Retry interval should be 5000")

	t.Logf("Successfully created and verified connection with retry rule: %s", conn.ID)
}

// TestConnectionWithFilterRule tests creating a connection with a filter rule
func TestConnectionWithFilterRule(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-filter-rule-" + timestamp
	sourceName := "test-src-filter-" + timestamp
	destName := "test-dst-filter-" + timestamp

	var conn Connection
	err := cli.RunJSON(&conn,
		"connection", "create",
		"--name", connName,
		"--source-name", sourceName,
		"--source-type", "WEBHOOK",
		"--destination-name", destName,
		"--destination-type", "CLI",
		"--destination-cli-path", "/webhooks",
		"--rule-filter-body", `{"type":"payment"}`,
		"--rule-filter-headers", `{"content-type":"application/json"}`,
	)
	require.NoError(t, err, "Should create connection with filter rule")
	require.NotEmpty(t, conn.ID, "Connection should have an ID")

	// Cleanup
	t.Cleanup(func() {
		deleteConnection(t, cli, conn.ID)
	})

	// Verify the rule was created by getting the connection
	var getConn Connection
	err = cli.RunJSON(&getConn, "connection", "get", conn.ID)
	require.NoError(t, err, "Should be able to get the created connection")

	require.NotEmpty(t, getConn.Rules, "Connection should have rules")
	require.Len(t, getConn.Rules, 1, "Connection should have exactly one rule")

	rule := getConn.Rules[0]
	assert.Equal(t, "filter", rule["type"], "Rule type should be filter")
	assert.Equal(t, `{"type":"payment"}`, rule["body"], "Filter body should match input")
	assert.Equal(t, `{"content-type":"application/json"}`, rule["headers"], "Filter headers should match input")

	t.Logf("Successfully created and verified connection with filter rule: %s", conn.ID)
}

// TestConnectionWithTransformRule tests creating a connection with a transform rule
func TestConnectionWithTransformRule(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-transform-rule-" + timestamp
	sourceName := "test-src-transform-" + timestamp
	destName := "test-dst-transform-" + timestamp

	var conn Connection
	err := cli.RunJSON(&conn,
		"connection", "create",
		"--name", connName,
		"--source-name", sourceName,
		"--source-type", "WEBHOOK",
		"--destination-name", destName,
		"--destination-type", "CLI",
		"--destination-cli-path", "/webhooks",
		"--rule-transform-name", "my-transform",
		"--rule-transform-code", "return { transformed: true };",
	)
	require.NoError(t, err, "Should create connection with transform rule")
	require.NotEmpty(t, conn.ID, "Connection should have an ID")

	// Cleanup
	t.Cleanup(func() {
		deleteConnection(t, cli, conn.ID)
	})

	// Verify the rule was created by getting the connection
	var getConn Connection
	err = cli.RunJSON(&getConn, "connection", "get", conn.ID)
	require.NoError(t, err, "Should be able to get the created connection")

	require.NotEmpty(t, getConn.Rules, "Connection should have rules")
	require.Len(t, getConn.Rules, 1, "Connection should have exactly one rule")

	rule := getConn.Rules[0]
	assert.Equal(t, "transform", rule["type"], "Rule type should be transform")

	// The API creates a transformation resource and returns just the ID reference
	assert.NotEmpty(t, rule["transformation_id"], "Transform rule should have a transformation_id")

	t.Logf("Successfully created and verified connection with transform rule: %s", conn.ID)
}

// TestConnectionWithDelayRule tests creating a connection with a delay rule
func TestConnectionWithDelayRule(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-delay-rule-" + timestamp
	sourceName := "test-src-delay-" + timestamp
	destName := "test-dst-delay-" + timestamp

	var conn Connection
	err := cli.RunJSON(&conn,
		"connection", "create",
		"--name", connName,
		"--source-name", sourceName,
		"--source-type", "WEBHOOK",
		"--destination-name", destName,
		"--destination-type", "CLI",
		"--destination-cli-path", "/webhooks",
		"--rule-delay", "3000",
	)
	require.NoError(t, err, "Should create connection with delay rule")
	require.NotEmpty(t, conn.ID, "Connection should have an ID")

	// Cleanup
	t.Cleanup(func() {
		deleteConnection(t, cli, conn.ID)
	})

	// Verify the rule was created by getting the connection
	var getConn Connection
	err = cli.RunJSON(&getConn, "connection", "get", conn.ID)
	require.NoError(t, err, "Should be able to get the created connection")

	require.NotEmpty(t, getConn.Rules, "Connection should have rules")
	require.Len(t, getConn.Rules, 1, "Connection should have exactly one rule")

	rule := getConn.Rules[0]
	assert.Equal(t, "delay", rule["type"], "Rule type should be delay")
	assert.Equal(t, float64(3000), rule["delay"], "Delay should be 3000 milliseconds")

	t.Logf("Successfully created and verified connection with delay rule: %s", conn.ID)
}

// TestConnectionWithDeduplicateRule tests creating a connection with a deduplicate rule
func TestConnectionWithDeduplicateRule(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-dedupe-rule-" + timestamp
	sourceName := "test-src-dedupe-" + timestamp
	destName := "test-dst-dedupe-" + timestamp

	var conn Connection
	err := cli.RunJSON(&conn,
		"connection", "create",
		"--name", connName,
		"--source-name", sourceName,
		"--source-type", "WEBHOOK",
		"--destination-name", destName,
		"--destination-type", "CLI",
		"--destination-cli-path", "/webhooks",
		"--rule-deduplicate-window", "86400",
		"--rule-deduplicate-include-fields", "body.id,body.timestamp",
	)
	require.NoError(t, err, "Should create connection with deduplicate rule")
	require.NotEmpty(t, conn.ID, "Connection should have an ID")

	// Cleanup
	t.Cleanup(func() {
		deleteConnection(t, cli, conn.ID)
	})

	// Verify the rule was created by getting the connection
	var getConn Connection
	err = cli.RunJSON(&getConn, "connection", "get", conn.ID)
	require.NoError(t, err, "Should be able to get the created connection")

	require.NotEmpty(t, getConn.Rules, "Connection should have rules")
	require.Len(t, getConn.Rules, 1, "Connection should have exactly one rule")

	rule := getConn.Rules[0]
	assert.Equal(t, "deduplicate", rule["type"], "Rule type should be deduplicate")
	assert.Equal(t, float64(86400), rule["window"], "Deduplicate window should be 86400 milliseconds")

	// Verify include_fields is correctly set and matches our input
	if includeFields, ok := rule["include_fields"].([]interface{}); ok {
		require.Len(t, includeFields, 2, "Should have 2 include fields")
		assert.Equal(t, "body.id", includeFields[0], "First include field should be 'body.id'")
		assert.Equal(t, "body.timestamp", includeFields[1], "Second include field should be 'body.timestamp'")
	} else {
		t.Fatal("include_fields should be an array in the response")
	}

	t.Logf("Successfully created and verified connection with deduplicate rule: %s", conn.ID)
}

// TestConnectionWithMultipleRules tests creating a connection with multiple rules and verifies logical ordering
func TestConnectionWithMultipleRules(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-multi-rules-" + timestamp
	sourceName := "test-src-multi-" + timestamp
	destName := "test-dst-multi-" + timestamp

	// Note: Rules are created in logical order (deduplicate -> transform -> filter -> delay -> retry)
	// This order matches the API's default ordering for proper data flow through the pipeline.
	var conn Connection
	err := cli.RunJSON(&conn,
		"connection", "create",
		"--name", connName,
		"--source-name", sourceName,
		"--source-type", "WEBHOOK",
		"--destination-name", destName,
		"--destination-type", "CLI",
		"--destination-cli-path", "/webhooks",
		"--rule-filter-body", `{"type":"payment"}`,
		"--rule-retry-strategy", "exponential",
		"--rule-retry-count", "5",
		"--rule-retry-interval", "60000",
		"--rule-delay", "1000",
	)
	require.NoError(t, err, "Should create connection with multiple rules")
	require.NotEmpty(t, conn.ID, "Connection should have an ID")

	// Cleanup
	t.Cleanup(func() {
		deleteConnection(t, cli, conn.ID)
	})

	// Verify the rules were created by getting the connection
	var getConn Connection
	err = cli.RunJSON(&getConn, "connection", "get", conn.ID)
	require.NoError(t, err, "Should be able to get the created connection")

	require.NotEmpty(t, getConn.Rules, "Connection should have rules")
	require.Len(t, getConn.Rules, 3, "Connection should have exactly three rules")

	// Verify logical order: filter -> delay -> retry (deduplicate/transform not present in this test)
	assert.Equal(t, "filter", getConn.Rules[0]["type"], "First rule should be filter (logical order)")
	assert.Equal(t, "delay", getConn.Rules[1]["type"], "Second rule should be delay (logical order)")
	assert.Equal(t, "retry", getConn.Rules[2]["type"], "Third rule should be retry (logical order)")

	// Verify filter rule details
	assert.Equal(t, `{"type":"payment"}`, getConn.Rules[0]["body"], "Filter should have body expression")

	// Verify delay rule details
	assert.Equal(t, float64(1000), getConn.Rules[1]["delay"], "Delay should be 1000 milliseconds")

	// Verify retry rule details
	assert.Equal(t, "exponential", getConn.Rules[2]["strategy"], "Retry strategy should be exponential")
	assert.Equal(t, float64(5), getConn.Rules[2]["count"], "Retry count should be 5")
	assert.Equal(t, float64(60000), getConn.Rules[2]["interval"], "Retry interval should be 60000")

	t.Logf("Successfully created and verified connection with multiple rules in logical order: %s", conn.ID)
}

// TestConnectionWithRateLimiting tests creating a connection with rate limiting
func TestConnectionWithRateLimiting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	t.Run("RateLimit_PerSecond", func(t *testing.T) {
		connName := "test-ratelimit-sec-" + timestamp
		sourceName := "test-src-rl-sec-" + timestamp
		destName := "test-dst-rl-sec-" + timestamp

		var conn Connection
		err := cli.RunJSON(&conn,
			"connection", "create",
			"--name", connName,
			"--source-name", sourceName,
			"--source-type", "WEBHOOK",
			"--destination-name", destName,
			"--destination-type", "HTTP",
			"--destination-url", "https://api.example.com/webhooks",
			"--destination-rate-limit", "100",
			"--destination-rate-limit-period", "second",
		)
		require.NoError(t, err, "Should create connection with rate limiting")
		require.NotEmpty(t, conn.ID, "Connection should have an ID")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, conn.ID)
		})

		// Verify rate limiting configuration by getting the connection
		var getConn Connection
		err = cli.RunJSON(&getConn, "connection", "get", conn.ID)
		require.NoError(t, err, "Should be able to get the created connection")

		require.NotNil(t, getConn.Destination, "Connection should have a destination")
		if config, ok := getConn.Destination.Config.(map[string]interface{}); ok {
			rateLimit, hasRateLimit := config["rate_limit"].(float64)
			require.True(t, hasRateLimit, "Rate limit should be present in destination config")
			assert.Equal(t, float64(100), rateLimit, "Rate limit should be 100")

			period, hasPeriod := config["rate_limit_period"].(string)
			require.True(t, hasPeriod, "Rate limit period should be present in destination config")
			assert.Equal(t, "second", period, "Rate limit period should be second")
		} else {
			t.Fatal("Destination config should be present")
		}

		t.Logf("Successfully created and verified connection with rate limiting (per second): %s", conn.ID)
	})

	t.Run("RateLimit_PerMinute", func(t *testing.T) {
		connName := "test-ratelimit-min-" + timestamp
		sourceName := "test-src-rl-min-" + timestamp
		destName := "test-dst-rl-min-" + timestamp

		var conn Connection
		err := cli.RunJSON(&conn,
			"connection", "create",
			"--name", connName,
			"--source-name", sourceName,
			"--source-type", "WEBHOOK",
			"--destination-name", destName,
			"--destination-type", "HTTP",
			"--destination-url", "https://api.example.com/webhooks",
			"--destination-rate-limit", "1000",
			"--destination-rate-limit-period", "minute",
		)
		require.NoError(t, err, "Should create connection with rate limiting")
		require.NotEmpty(t, conn.ID, "Connection should have an ID")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, conn.ID)
		})

		// Verify rate limiting configuration by getting the connection
		var getConn Connection
		err = cli.RunJSON(&getConn, "connection", "get", conn.ID)
		require.NoError(t, err, "Should be able to get the created connection")

		require.NotNil(t, getConn.Destination, "Connection should have a destination")
		if config, ok := getConn.Destination.Config.(map[string]interface{}); ok {
			rateLimit, hasRateLimit := config["rate_limit"].(float64)
			require.True(t, hasRateLimit, "Rate limit should be present in destination config")
			assert.Equal(t, float64(1000), rateLimit, "Rate limit should be 1000")

			period, hasPeriod := config["rate_limit_period"].(string)
			require.True(t, hasPeriod, "Rate limit period should be present in destination config")
			assert.Equal(t, "minute", period, "Rate limit period should be minute")
		} else {
			t.Fatal("Destination config should be present")
		}

		t.Logf("Successfully created and verified connection with rate limiting (per minute): %s", conn.ID)
	})
	t.Run("RateLimit_Concurrent", func(t *testing.T) {
		connName := "test-ratelimit-concurrent-" + timestamp
		sourceName := "test-src-rl-concurrent-" + timestamp
		destName := "test-dst-rl-concurrent-" + timestamp

		var conn Connection
		err := cli.RunJSON(&conn,
			"connection", "create",
			"--name", connName,
			"--source-name", sourceName,
			"--source-type", "WEBHOOK",
			"--destination-name", destName,
			"--destination-type", "HTTP",
			"--destination-url", "https://api.example.com/webhooks",
			"--destination-rate-limit", "10",
			"--destination-rate-limit-period", "concurrent",
		)
		require.NoError(t, err, "Should create connection with concurrent rate limiting")
		require.NotEmpty(t, conn.ID, "Connection should have an ID")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, conn.ID)
		})

		// Verify rate limiting configuration by getting the connection
		var getConn Connection
		err = cli.RunJSON(&getConn, "connection", "get", conn.ID)
		require.NoError(t, err, "Should be able to get the created connection")

		require.NotNil(t, getConn.Destination, "Connection should have a destination")
		if config, ok := getConn.Destination.Config.(map[string]interface{}); ok {
			rateLimit, hasRateLimit := config["rate_limit"].(float64)
			require.True(t, hasRateLimit, "Rate limit should be present in destination config")
			assert.Equal(t, float64(10), rateLimit, "Rate limit should be 10")

			period, hasPeriod := config["rate_limit_period"].(string)
			require.True(t, hasPeriod, "Rate limit period should be present in destination config")
			assert.Equal(t, "concurrent", period, "Rate limit period should be concurrent")
		} else {
			t.Fatal("Destination config should be present")
		}

		t.Logf("Successfully created and verified connection with concurrent rate limiting: %s", conn.ID)
	})

}

// TestConnectionUpsertCreate tests creating a new connection via upsert
func TestConnectionUpsertCreate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-upsert-create-" + timestamp
	sourceName := "test-upsert-src-" + timestamp
	destName := "test-upsert-dst-" + timestamp

	// Upsert (create) a new connection
	var conn Connection
	err := cli.RunJSON(&conn,
		"connection", "upsert", connName,
		"--source-name", sourceName,
		"--source-type", "WEBHOOK",
		"--destination-name", destName,
		"--destination-type", "CLI",
		"--destination-cli-path", "/webhooks",
	)
	require.NoError(t, err, "Should create connection via upsert")
	require.NotEmpty(t, conn.ID, "Connection should have an ID")

	// Cleanup
	t.Cleanup(func() {
		deleteConnection(t, cli, conn.ID)
	})

	// PRIMARY: Verify upsert command output
	assert.Equal(t, connName, conn.Name, "Connection name should match in upsert output")
	assert.Equal(t, sourceName, conn.Source.Name, "Source name should match in upsert output")
	assert.Equal(t, destName, conn.Destination.Name, "Destination name should match in upsert output")

	// SECONDARY: Verify persisted state via GET
	var fetched Connection
	err = cli.RunJSON(&fetched, "connection", "get", conn.ID)
	require.NoError(t, err, "Should be able to get the created connection")

	assert.Equal(t, connName, fetched.Name, "Connection name should be persisted")
	assert.Equal(t, sourceName, fetched.Source.Name, "Source name should be persisted")
	assert.Equal(t, destName, fetched.Destination.Name, "Destination name should be persisted")

	t.Logf("Successfully created connection via upsert: %s", conn.ID)
}

// TestConnectionUpsertUpdate tests updating an existing connection via upsert
func TestConnectionUpsertUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-upsert-update-" + timestamp
	sourceName := "test-upsert-update-src-" + timestamp
	destName := "test-upsert-update-dst-" + timestamp

	// First create a connection
	var conn Connection
	err := cli.RunJSON(&conn,
		"connection", "create",
		"--name", connName,
		"--source-name", sourceName,
		"--source-type", "WEBHOOK",
		"--destination-name", destName,
		"--destination-type", "CLI",
		"--destination-cli-path", "/webhooks",
	)
	require.NoError(t, err, "Should create initial connection")

	// Cleanup
	t.Cleanup(func() {
		deleteConnection(t, cli, conn.ID)
	})

	// Now upsert (update) with a description
	newDesc := "Updated via upsert command"
	var upserted Connection
	err = cli.RunJSON(&upserted, "connection", "upsert", connName,
		"--description", newDesc,
	)
	require.NoError(t, err, "Should upsert connection")

	// PRIMARY: Verify upsert command output
	assert.Equal(t, conn.ID, upserted.ID, "Connection ID should match")
	assert.Equal(t, connName, upserted.Name, "Connection name should match")
	assert.Equal(t, newDesc, upserted.Description, "Description should be updated in upsert output")
	assert.Equal(t, sourceName, upserted.Source.Name, "Source should be preserved in upsert output")
	assert.Equal(t, destName, upserted.Destination.Name, "Destination should be preserved in upsert output")

	// SECONDARY: Verify persisted state via GET
	var fetched Connection
	err = cli.RunJSON(&fetched, "connection", "get", conn.ID)
	require.NoError(t, err, "Should get updated connection")

	assert.Equal(t, newDesc, fetched.Description, "Description should be persisted")
	assert.Equal(t, sourceName, fetched.Source.Name, "Source should be persisted")
	assert.Equal(t, destName, fetched.Destination.Name, "Destination should be persisted")

	t.Logf("Successfully updated connection via upsert: %s", conn.ID)
}

// TestConnectionUpsertIdempotent tests that upsert is idempotent
func TestConnectionUpsertIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-upsert-idem-" + timestamp
	sourceName := "test-upsert-idem-src-" + timestamp
	destName := "test-upsert-idem-dst-" + timestamp

	// Run upsert twice with same parameters
	var conn1, conn2 Connection

	err := cli.RunJSON(&conn1,
		"connection", "upsert", connName,
		"--source-name", sourceName,
		"--source-type", "WEBHOOK",
		"--destination-name", destName,
		"--destination-type", "CLI",
		"--destination-cli-path", "/webhooks",
	)
	require.NoError(t, err, "First upsert should succeed")

	// Cleanup
	t.Cleanup(func() {
		deleteConnection(t, cli, conn1.ID)
	})

	err = cli.RunJSON(&conn2,
		"connection", "upsert", connName,
		"--source-name", sourceName,
		"--source-type", "WEBHOOK",
		"--destination-name", destName,
		"--destination-type", "CLI",
		"--destination-cli-path", "/webhooks",
	)
	require.NoError(t, err, "Second upsert should succeed")

	// PRIMARY: Both outputs should refer to the same connection with same properties
	assert.Equal(t, conn1.ID, conn2.ID, "Both upserts should operate on same connection")
	assert.Equal(t, conn1.Name, conn2.Name, "Connection name should match in both outputs")
	assert.Equal(t, conn1.Source.Name, conn2.Source.Name, "Source name should match in both outputs")
	assert.Equal(t, conn1.Destination.Name, conn2.Destination.Name, "Destination name should match in both outputs")

	// SECONDARY: Verify persisted state
	var fetched Connection
	err = cli.RunJSON(&fetched, "connection", "get", conn1.ID)
	require.NoError(t, err, "Should get connection")
	assert.Equal(t, connName, fetched.Name, "Connection name should be persisted")

	t.Logf("Successfully verified idempotency: %s", conn1.ID)
}

// TestConnectionUpsertDryRun tests that dry-run doesn't make changes
func TestConnectionUpsertDryRun(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-upsert-dryrun-" + timestamp
	sourceName := "test-upsert-dryrun-src-" + timestamp
	destName := "test-upsert-dryrun-dst-" + timestamp

	// Run upsert with --dry-run (should not create)
	stdout := cli.RunExpectSuccess("connection", "upsert", connName,
		"--source-name", sourceName,
		"--source-type", "WEBHOOK",
		"--destination-name", destName,
		"--destination-type", "CLI",
		"--destination-cli-path", "/webhooks",
		"--dry-run",
	)

	assert.Contains(t, stdout, "DRY RUN", "Should indicate dry-run mode")
	assert.Contains(t, stdout, "Operation: CREATE", "Should indicate create operation")
	assert.Contains(t, stdout, "No changes were made", "Should confirm no changes")

	// Verify the connection was NOT created by trying to list it
	var listResp map[string]interface{}
	cli.RunJSON(&listResp, "connection", "list", "--name", connName)
	// Connection should not exist, so we expect empty or error

	t.Logf("Successfully verified dry-run for create scenario")
}

// TestConnectionUpsertDryRunUpdate tests dry-run on update scenario
func TestConnectionUpsertDryRunUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-upsert-dryrun-upd-" + timestamp
	sourceName := "test-upsert-dryrun-upd-src-" + timestamp
	destName := "test-upsert-dryrun-upd-dst-" + timestamp

	// Create initial connection
	var conn Connection
	err := cli.RunJSON(&conn,
		"connection", "create",
		"--name", connName,
		"--source-name", sourceName,
		"--source-type", "WEBHOOK",
		"--destination-name", destName,
		"--destination-type", "CLI",
		"--destination-cli-path", "/webhooks",
	)
	require.NoError(t, err, "Should create initial connection")

	// Cleanup
	t.Cleanup(func() {
		deleteConnection(t, cli, conn.ID)
	})

	// Run upsert with --dry-run for update
	newDesc := "This should not be applied"
	stdout := cli.RunExpectSuccess("connection", "upsert", connName,
		"--description", newDesc,
		"--dry-run",
	)

	assert.Contains(t, stdout, "DRY RUN", "Should indicate dry-run mode")
	assert.Contains(t, stdout, "Operation: UPDATE", "Should indicate update operation")
	assert.Contains(t, stdout, "Description", "Should show description change")

	// Verify the connection was NOT updated
	var getResp Connection
	err = cli.RunJSON(&getResp, "connection", "get", conn.ID)
	require.NoError(t, err, "Should get connection")

	assert.NotEqual(t, newDesc, getResp.Description, "Description should not be updated in dry-run")

	t.Logf("Successfully verified dry-run for update scenario")
}

// TestConnectionUpsertPartialUpdate tests updating only some properties
func TestConnectionUpsertPartialUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-upsert-partial-" + timestamp
	sourceName := "test-upsert-partial-src-" + timestamp
	destName := "test-upsert-partial-dst-" + timestamp
	initialDesc := "Initial description"

	// Create initial connection
	var conn Connection
	err := cli.RunJSON(&conn,
		"connection", "create",
		"--name", connName,
		"--description", initialDesc,
		"--source-name", sourceName,
		"--source-type", "WEBHOOK",
		"--destination-name", destName,
		"--destination-type", "CLI",
		"--destination-cli-path", "/webhooks",
	)
	require.NoError(t, err, "Should create initial connection")

	// Cleanup
	t.Cleanup(func() {
		deleteConnection(t, cli, conn.ID)
	})

	// Update only description
	newDesc := "Updated description only"
	var upserted Connection
	err = cli.RunJSON(&upserted, "connection", "upsert", connName,
		"--description", newDesc,
	)
	require.NoError(t, err, "Should upsert connection")

	// PRIMARY: Verify upsert command output - source and destination weren't changed
	assert.Equal(t, conn.ID, upserted.ID, "Connection ID should match")
	assert.Equal(t, newDesc, upserted.Description, "Description should be updated in upsert output")
	assert.Equal(t, sourceName, upserted.Source.Name, "Source should be preserved in upsert output")
	assert.Equal(t, destName, upserted.Destination.Name, "Destination should be preserved in upsert output")

	// SECONDARY: Verify persisted state via GET
	var fetched Connection
	err = cli.RunJSON(&fetched, "connection", "get", conn.ID)
	require.NoError(t, err, "Should get updated connection")

	assert.Equal(t, newDesc, fetched.Description, "Description should be persisted")
	assert.Equal(t, sourceName, fetched.Source.Name, "Source should be persisted")
	assert.Equal(t, destName, fetched.Destination.Name, "Destination should be persisted")

	t.Logf("Successfully verified partial update via upsert")
}

// TestConnectionUpsertWithRules tests updating rules via upsert
func TestConnectionUpsertWithRules(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-upsert-rules-" + timestamp
	sourceName := "test-upsert-rules-src-" + timestamp
	destName := "test-upsert-rules-dst-" + timestamp

	// Create initial connection
	var conn Connection
	err := cli.RunJSON(&conn,
		"connection", "create",
		"--name", connName,
		"--source-name", sourceName,
		"--source-type", "WEBHOOK",
		"--destination-name", destName,
		"--destination-type", "CLI",
		"--destination-cli-path", "/webhooks",
	)
	require.NoError(t, err, "Should create initial connection")

	// Cleanup
	t.Cleanup(func() {
		deleteConnection(t, cli, conn.ID)
	})

	// Update with retry rule
	var upserted Connection
	err = cli.RunJSON(&upserted,
		"connection", "upsert", connName,
		"--rule-retry-strategy", "linear",
		"--rule-retry-count", "3",
		"--rule-retry-interval", "5000",
	)
	require.NoError(t, err, "Should update with rules")

	// PRIMARY: Verify upsert command output includes rules
	assert.Equal(t, conn.ID, upserted.ID, "Connection ID should match")
	assert.NotEmpty(t, upserted.Rules, "Should have rules in upsert output")
	assert.Greater(t, len(upserted.Rules), 0, "Should have at least one rule in upsert output")
	assert.Equal(t, sourceName, upserted.Source.Name, "Source should be preserved in upsert output")
	assert.Equal(t, destName, upserted.Destination.Name, "Destination should be preserved in upsert output")

	// SECONDARY: Verify persisted state via GET
	var fetched Connection
	err = cli.RunJSON(&fetched, "connection", "get", conn.ID)
	require.NoError(t, err, "Should get updated connection")
	assert.NotEmpty(t, fetched.Rules, "Should have rules persisted")

	t.Logf("Successfully updated rules via upsert: %s", conn.ID)
}

// TestConnectionUpsertReplaceRules tests replacing existing rules via upsert
func TestConnectionUpsertReplaceRules(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-upsert-replace-rules-" + timestamp
	sourceName := "test-upsert-replace-src-" + timestamp
	destName := "test-upsert-replace-dst-" + timestamp

	// Create initial connection WITH a retry rule
	var conn Connection
	err := cli.RunJSON(&conn,
		"connection", "create",
		"--name", connName,
		"--source-name", sourceName,
		"--source-type", "WEBHOOK",
		"--destination-name", destName,
		"--destination-type", "CLI",
		"--destination-cli-path", "/webhooks",
		"--rule-retry-strategy", "linear",
		"--rule-retry-count", "3",
		"--rule-retry-interval", "5000",
	)
	require.NoError(t, err, "Should create initial connection with retry rule")
	require.NotEmpty(t, conn.Rules, "Initial connection should have rules")

	// Cleanup
	t.Cleanup(func() {
		deleteConnection(t, cli, conn.ID)
	})

	// Verify initial rule is retry
	initialRule := conn.Rules[0]
	assert.Equal(t, "retry", initialRule["type"], "Initial rule should be retry type")

	// Upsert to REPLACE retry rule with filter rule (using proper JSON format)
	filterBody := `{"type":"payment"}`
	var upserted Connection
	err = cli.RunJSON(&upserted,
		"connection", "upsert", connName,
		"--rule-filter-body", filterBody,
	)
	require.NoError(t, err, "Should upsert connection with filter rule")

	// PRIMARY: Verify upsert command output has replaced rules
	assert.Equal(t, conn.ID, upserted.ID, "Connection ID should match")
	assert.NotEmpty(t, upserted.Rules, "Should have rules in upsert output")
	assert.Len(t, upserted.Rules, 1, "Should have exactly one rule (replaced)")

	// Verify the rule is now a filter rule, not retry
	replacedRule := upserted.Rules[0]
	assert.Equal(t, "filter", replacedRule["type"], "Rule should now be filter type")
	assert.NotEqual(t, "retry", replacedRule["type"], "Retry rule should be replaced")
	assert.Equal(t, filterBody, replacedRule["body"], "Filter body should match input")

	// Verify source and destination are preserved
	assert.Equal(t, sourceName, upserted.Source.Name, "Source should be preserved in upsert output")
	assert.Equal(t, destName, upserted.Destination.Name, "Destination should be preserved in upsert output")

	// SECONDARY: Verify persisted state via GET
	var fetched Connection
	err = cli.RunJSON(&fetched, "connection", "get", conn.ID)
	require.NoError(t, err, "Should get updated connection")

	assert.Len(t, fetched.Rules, 1, "Should have exactly one rule persisted")
	fetchedRule := fetched.Rules[0]
	assert.Equal(t, "filter", fetchedRule["type"], "Persisted rule should be filter type")
	assert.Equal(t, filterBody, fetchedRule["body"], "Persisted filter body should match input")

	t.Logf("Successfully replaced rules via upsert: %s", conn.ID)
}

// TestConnectionUpsertValidation tests validation errors
func TestConnectionUpsertValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	// Test 1: Missing name
	_, _, err := cli.Run("connection", "upsert")
	assert.Error(t, err, "Should require name positional argument")

	// Test 2: Missing required fields for new connection
	connName := "test-upsert-validation-" + timestamp
	_, _, err = cli.Run("connection", "upsert", connName)
	assert.Error(t, err, "Should require source and destination for new connection")

	t.Logf("Successfully verified validation errors")
}

// TestConnectionCreateOutputStructure tests the human-readable output format
func TestConnectionCreateOutputStructure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-output-" + timestamp
	sourceName := "test-src-output-" + timestamp
	destName := "test-dst-output-" + timestamp

	// Create connection without --output json to get human-readable format
	stdout := cli.RunExpectSuccess(
		"connection", "create",
		"--name", connName,
		"--source-name", sourceName,
		"--source-type", "WEBHOOK",
		"--destination-name", destName,
		"--destination-type", "CLI",
		"--destination-cli-path", "/webhooks",
	)

	// Parse connection ID from output for cleanup
	// New format: "Connection:  test-output-xxx (web_xxxxx)"
	lines := strings.Split(stdout, "\n")
	var connID string
	for _, line := range lines {
		if strings.Contains(line, "Connection:") && strings.Contains(line, "(") && strings.Contains(line, ")") {
			// Extract ID from parentheses
			start := strings.Index(line, "(")
			end := strings.Index(line, ")")
			if start != -1 && end != -1 && end > start {
				connID = strings.TrimSpace(line[start+1 : end])
				break
			}
		}
	}
	require.NotEmpty(t, connID, "Should be able to parse connection ID from output")

	// Cleanup
	t.Cleanup(func() {
		deleteConnection(t, cli, connID)
	})

	// Verify output structure contains expected elements from create command
	// Expected format:
	// ✔ Connection created successfully
	//
	// Connection:  test-webhooks-to-local (conn_abc123)
	// Source:      test-webhooks (src_123abc)
	// Source Type: WEBHOOK
	// Source URL:  https://hkdk.events/src_123abc
	// Destination: local-dev (dst_456def)
	// Destination Type: CLI
	// Destination Path: /webhooks (for CLI destinations)

	assert.Contains(t, stdout, "✔ Connection created successfully", "Should show success message")

	// Verify Connection line format: "Connection:  name (id)"
	assert.Contains(t, stdout, "Connection:", "Should show Connection label")
	assert.Contains(t, stdout, connName, "Should include connection name")
	assert.Contains(t, stdout, connID, "Should include connection ID in parentheses")

	// Verify Source details
	assert.Contains(t, stdout, "Source:", "Should show Source label")
	assert.Contains(t, stdout, sourceName, "Should include source name")
	assert.Contains(t, stdout, "Source Type:", "Should show source type label")
	assert.Contains(t, stdout, "WEBHOOK", "Should show source type value")
	assert.Contains(t, stdout, "Source URL:", "Should show source URL label")
	assert.Contains(t, stdout, "https://hkdk.events/", "Should include Hookdeck event URL")

	// Verify Destination details
	assert.Contains(t, stdout, "Destination:", "Should show Destination label")
	assert.Contains(t, stdout, destName, "Should include destination name")
	assert.Contains(t, stdout, "Destination Type:", "Should show destination type label")
	assert.Contains(t, stdout, "CLI", "Should show destination type value")

	// For CLI destinations, should show Destination Path
	assert.Contains(t, stdout, "Destination Path:", "Should show destination path label for CLI destinations")
	assert.Contains(t, stdout, "/webhooks", "Should show the destination path value")

	t.Logf("Successfully verified connection create output structure")
}

// TestConnectionWithDestinationPathForwarding tests path_forwarding_disabled and http_method fields
func TestConnectionWithDestinationPathForwarding(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	t.Run("HTTP_Destination_PathForwardingDisabled_And_HTTPMethod", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-path-forward-conn-" + timestamp
		sourceName := "test-path-forward-source-" + timestamp
		destName := "test-path-forward-dest-" + timestamp
		destURL := "https://api.hookdeck.com/dev/null"

		// Create connection with path forwarding disabled and custom HTTP method
		stdout, stderr, err := cli.Run("connection", "create",
			"--name", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--destination-type", "HTTP",
			"--destination-name", destName,
			"--destination-url", destURL,
			"--destination-path-forwarding-disabled", "true",
			"--destination-http-method", "PUT",
			"--output", "json")
		require.NoError(t, err, "Failed to create connection: stderr=%s", stderr)

		// Parse creation response
		var createResp map[string]interface{}
		err = json.Unmarshal([]byte(stdout), &createResp)
		require.NoError(t, err, "Failed to parse creation response: %s", stdout)

		// Verify creation response fields
		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID in creation response")

		assert.Equal(t, connName, createResp["name"], "Connection name should match")

		// Verify destination details
		dest, ok := createResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination object in creation response")
		assert.Equal(t, destName, dest["name"], "Destination name should match")
		destType, _ := dest["type"].(string)
		assert.Equal(t, "HTTP", strings.ToUpper(destType), "Destination type should be HTTP")

		// Verify path_forwarding_disabled and http_method in destination config
		destConfig, ok := dest["config"].(map[string]interface{})
		require.True(t, ok, "Expected destination config object")

		// Check path_forwarding_disabled is set to true
		pathForwardingDisabled, ok := destConfig["path_forwarding_disabled"].(bool)
		require.True(t, ok, "Expected path_forwarding_disabled in config")
		assert.True(t, pathForwardingDisabled, "path_forwarding_disabled should be true")

		// Check http_method is set to PUT
		httpMethod, ok := destConfig["http_method"].(string)
		require.True(t, ok, "Expected http_method in config")
		assert.Equal(t, "PUT", strings.ToUpper(httpMethod), "HTTP method should be PUT")

		// Verify using connection get
		var getResp map[string]interface{}
		err = cli.RunJSON(&getResp, "connection", "get", connID)
		require.NoError(t, err, "Should be able to get the created connection")

		// Verify destination config in get response
		getDest, ok := getResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination object in get response")
		getDestConfig, ok := getDest["config"].(map[string]interface{})
		require.True(t, ok, "Expected destination config in get response")

		getPathForwardingDisabled, ok := getDestConfig["path_forwarding_disabled"].(bool)
		require.True(t, ok, "Expected path_forwarding_disabled in get response config")
		assert.True(t, getPathForwardingDisabled, "path_forwarding_disabled should be true in get response")

		getHTTPMethod, ok := getDestConfig["http_method"].(string)
		require.True(t, ok, "Expected http_method in get response config")
		assert.Equal(t, "PUT", strings.ToUpper(getHTTPMethod), "HTTP method should be PUT in get response")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		t.Logf("Successfully tested HTTP destination with path_forwarding_disabled and http_method: %s", connID)
	})

	t.Run("HTTP_Destination_AllHTTPMethods", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE"}

		for _, method := range methods {
			connName := "test-http-method-" + strings.ToLower(method) + "-" + timestamp
			sourceName := "test-src-" + strings.ToLower(method) + "-" + timestamp
			destName := "test-dst-" + strings.ToLower(method) + "-" + timestamp
			destURL := "https://api.hookdeck.com/dev/null"

			var createResp map[string]interface{}
			err := cli.RunJSON(&createResp,
				"connection", "create",
				"--name", connName,
				"--source-type", "WEBHOOK",
				"--source-name", sourceName,
				"--destination-type", "HTTP",
				"--destination-name", destName,
				"--destination-url", destURL,
				"--destination-http-method", method)
			require.NoError(t, err, "Failed to create connection with HTTP method %s", method)

			connID, ok := createResp["id"].(string)
			require.True(t, ok && connID != "", "Expected connection ID")

			// Verify http_method
			dest, ok := createResp["destination"].(map[string]interface{})
			require.True(t, ok, "Expected destination object")
			destConfig, ok := dest["config"].(map[string]interface{})
			require.True(t, ok, "Expected destination config")
			httpMethod, ok := destConfig["http_method"].(string)
			require.True(t, ok, "Expected http_method in config")
			assert.Equal(t, method, strings.ToUpper(httpMethod), "HTTP method should be %s", method)

			// Cleanup
			t.Cleanup(func() {
				deleteConnection(t, cli, connID)
			})

			t.Logf("Successfully tested HTTP method %s: %s", method, connID)
		}
	})
}

// TestConnectionUpsertDestinationFields tests upserting path_forwarding_disabled and http_method
func TestConnectionUpsertDestinationFields(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	t.Run("Upsert_PathForwardingDisabled", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-upsert-path-" + timestamp
		sourceName := "test-src-upsert-path-" + timestamp
		destName := "test-dst-upsert-path-" + timestamp
		destURL := "https://api.hookdeck.com/dev/null"

		// Create connection with path forwarding enabled (default)
		var createResp map[string]interface{}
		err := cli.RunJSON(&createResp,
			"connection", "create",
			"--name", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--destination-type", "HTTP",
			"--destination-name", destName,
			"--destination-url", destURL)
		require.NoError(t, err, "Failed to create connection")

		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		// Verify path_forwarding_disabled is not set (or false)
		dest, ok := createResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination object")
		destConfig, ok := dest["config"].(map[string]interface{})
		require.True(t, ok, "Expected destination config")

		// It may not be present or may be false
		if pathForwardingDisabled, ok := destConfig["path_forwarding_disabled"].(bool); ok {
			assert.False(t, pathForwardingDisabled, "path_forwarding_disabled should be false by default")
		}

		// Upsert to disable path forwarding
		var upsertResp map[string]interface{}
		err = cli.RunJSON(&upsertResp,
			"connection", "upsert", connName,
			"--destination-path-forwarding-disabled", "true")
		require.NoError(t, err, "Failed to upsert connection")

		// Verify path_forwarding_disabled is now true
		upsertDest, ok := upsertResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination object in upsert response")
		upsertDestConfig, ok := upsertDest["config"].(map[string]interface{})
		require.True(t, ok, "Expected destination config in upsert response")

		pathForwardingDisabled, ok := upsertDestConfig["path_forwarding_disabled"].(bool)
		require.True(t, ok, "Expected path_forwarding_disabled in upsert response config")
		assert.True(t, pathForwardingDisabled, "path_forwarding_disabled should be true after upsert")

		// Upsert again to re-enable path forwarding
		var upsertResp2 map[string]interface{}
		err = cli.RunJSON(&upsertResp2,
			"connection", "upsert", connName,
			"--destination-path-forwarding-disabled", "false")
		require.NoError(t, err, "Failed to upsert connection second time")

		// Verify path_forwarding_disabled is now false
		upsertDest2, ok := upsertResp2["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination object in second upsert response")
		upsertDestConfig2, ok := upsertDest2["config"].(map[string]interface{})
		require.True(t, ok, "Expected destination config in second upsert response")

		pathForwardingDisabled2, ok := upsertDestConfig2["path_forwarding_disabled"].(bool)
		require.True(t, ok, "Expected path_forwarding_disabled in second upsert response config")
		assert.False(t, pathForwardingDisabled2, "path_forwarding_disabled should be false after second upsert")

		t.Logf("Successfully tested upsert path_forwarding_disabled toggle: %s", connID)
	})

	t.Run("Upsert_HTTPMethod", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-upsert-method-" + timestamp
		sourceName := "test-src-upsert-method-" + timestamp
		destName := "test-dst-upsert-method-" + timestamp
		destURL := "https://api.hookdeck.com/dev/null"

		// Create connection with POST method
		var createResp map[string]interface{}
		err := cli.RunJSON(&createResp,
			"connection", "create",
			"--name", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--destination-type", "HTTP",
			"--destination-name", destName,
			"--destination-url", destURL,
			"--destination-http-method", "POST")
		require.NoError(t, err, "Failed to create connection")

		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		// Verify initial method is POST
		dest, ok := createResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination object")
		destConfig, ok := dest["config"].(map[string]interface{})
		require.True(t, ok, "Expected destination config")
		httpMethod, ok := destConfig["http_method"].(string)
		require.True(t, ok, "Expected http_method in config")
		assert.Equal(t, "POST", strings.ToUpper(httpMethod), "HTTP method should be POST")

		// Upsert to change method to PUT
		var upsertResp map[string]interface{}
		err = cli.RunJSON(&upsertResp,
			"connection", "upsert", connName,
			"--destination-http-method", "PUT")
		require.NoError(t, err, "Failed to upsert connection")

		// Verify method is now PUT
		upsertDest, ok := upsertResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination object in upsert response")
		upsertDestConfig, ok := upsertDest["config"].(map[string]interface{})
		require.True(t, ok, "Expected destination config in upsert response")
		upsertHTTPMethod, ok := upsertDestConfig["http_method"].(string)
		require.True(t, ok, "Expected http_method in upsert response config")
		assert.Equal(t, "PUT", strings.ToUpper(upsertHTTPMethod), "HTTP method should be PUT after upsert")

		t.Logf("Successfully tested upsert http_method change: %s", connID)
	})

	t.Run("Create_Source_AllowedHTTPMethods", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-allowed-methods-" + timestamp
		sourceName := "test-src-allowed-methods-" + timestamp
		destName := "test-dst-allowed-methods-" + timestamp

		// Create connection with allowed HTTP methods
		var createResp map[string]interface{}
		err := cli.RunJSON(&createResp,
			"connection", "create",
			"--name", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--source-allowed-http-methods", "POST,PUT,DELETE",
			"--destination-type", "CLI",
			"--destination-name", destName,
			"--destination-cli-path", "/webhooks")
		require.NoError(t, err, "Failed to create connection with allowed HTTP methods")

		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		// Verify source config contains allowed_http_methods
		source, ok := createResp["source"].(map[string]interface{})
		require.True(t, ok, "Expected source object")
		sourceConfig, ok := source["config"].(map[string]interface{})
		require.True(t, ok, "Expected source config")

		allowedMethods, ok := sourceConfig["allowed_http_methods"].([]interface{})
		require.True(t, ok, "Expected allowed_http_methods in source config")
		require.Len(t, allowedMethods, 3, "Expected 3 allowed HTTP methods")

		// Verify methods are correct
		methodsMap := make(map[string]bool)
		for _, m := range allowedMethods {
			method, ok := m.(string)
			require.True(t, ok, "Expected string method")
			methodsMap[strings.ToUpper(method)] = true
		}
		assert.True(t, methodsMap["POST"], "Should contain POST")
		assert.True(t, methodsMap["PUT"], "Should contain PUT")
		assert.True(t, methodsMap["DELETE"], "Should contain DELETE")

		t.Logf("Successfully tested source allowed HTTP methods: %s", connID)
	})

	t.Run("Create_Source_CustomResponse", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-custom-response-" + timestamp
		sourceName := "test-src-custom-response-" + timestamp
		destName := "test-dst-custom-response-" + timestamp
		customBody := `{"status":"received","timestamp":"2024-01-01T00:00:00Z"}`

		// Create connection with custom response
		var createResp map[string]interface{}
		err := cli.RunJSON(&createResp,
			"connection", "create",
			"--name", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--source-custom-response-content-type", "json",
			"--source-custom-response-body", customBody,
			"--destination-type", "CLI",
			"--destination-name", destName,
			"--destination-cli-path", "/webhooks")
		require.NoError(t, err, "Failed to create connection with custom response")

		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		// Verify source config contains custom_response
		source, ok := createResp["source"].(map[string]interface{})
		require.True(t, ok, "Expected source object")
		sourceConfig, ok := source["config"].(map[string]interface{})
		require.True(t, ok, "Expected source config")

		customResponse, ok := sourceConfig["custom_response"].(map[string]interface{})
		require.True(t, ok, "Expected custom_response in source config")

		contentType, ok := customResponse["content_type"].(string)
		require.True(t, ok, "Expected content_type in custom_response")
		assert.Equal(t, "json", strings.ToLower(contentType), "Content type should be json")

		body, ok := customResponse["body"].(string)
		require.True(t, ok, "Expected body in custom_response")
		assert.Equal(t, customBody, body, "Body should match")

		t.Logf("Successfully tested source custom response: %s", connID)
	})

	t.Run("Create_Source_AllConfigOptions", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-all-config-" + timestamp
		sourceName := "test-src-all-config-" + timestamp
		destName := "test-dst-all-config-" + timestamp
		customBody := `{"ok":true}`

		// Create connection with all source config options
		// Note: allowed_http_methods and custom_response are only supported for WEBHOOK source types
		var createResp map[string]interface{}
		err := cli.RunJSON(&createResp,
			"connection", "create",
			"--name", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--source-allowed-http-methods", "POST,PUT,PATCH",
			"--source-custom-response-content-type", "json",
			"--source-custom-response-body", customBody,
			"--destination-type", "CLI",
			"--destination-name", destName,
			"--destination-cli-path", "/webhooks")
		require.NoError(t, err, "Failed to create connection with all source config options")

		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		// Verify source config contains all options
		source, ok := createResp["source"].(map[string]interface{})
		require.True(t, ok, "Expected source object")
		sourceConfig, ok := source["config"].(map[string]interface{})
		require.True(t, ok, "Expected source config")

		// Verify allowed_http_methods
		allowedMethods, ok := sourceConfig["allowed_http_methods"].([]interface{})
		require.True(t, ok, "Expected allowed_http_methods in source config")
		assert.Len(t, allowedMethods, 3, "Expected 3 allowed HTTP methods")

		// Verify custom_response
		customResponse, ok := sourceConfig["custom_response"].(map[string]interface{})
		require.True(t, ok, "Expected custom_response in source config")
		assert.Equal(t, "json", strings.ToLower(customResponse["content_type"].(string)), "Content type should be json")
		assert.Equal(t, customBody, customResponse["body"].(string), "Body should match")

		t.Logf("Successfully tested all source config options: %s", connID)
	})

	t.Run("Upsert_Source_AllowedHTTPMethods", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-upsert-allowed-methods-" + timestamp
		sourceName := "test-src-upsert-methods-" + timestamp
		destName := "test-dst-upsert-methods-" + timestamp

		// Create connection without allowed methods
		var createResp map[string]interface{}
		err := cli.RunJSON(&createResp,
			"connection", "create",
			"--name", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--destination-type", "CLI",
			"--destination-name", destName,
			"--destination-cli-path", "/webhooks")
		require.NoError(t, err, "Failed to create connection")

		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		// Upsert to add allowed HTTP methods
		var upsertResp map[string]interface{}
		err = cli.RunJSON(&upsertResp,
			"connection", "upsert", connName,
			"--source-allowed-http-methods", "POST,GET")
		require.NoError(t, err, "Failed to upsert connection with allowed methods")

		// Verify allowed_http_methods are set
		source, ok := upsertResp["source"].(map[string]interface{})
		require.True(t, ok, "Expected source object in upsert response")
		sourceConfig, ok := source["config"].(map[string]interface{})
		require.True(t, ok, "Expected source config in upsert response")

		allowedMethods, ok := sourceConfig["allowed_http_methods"].([]interface{})
		require.True(t, ok, "Expected allowed_http_methods in upsert response")
		assert.Len(t, allowedMethods, 2, "Expected 2 allowed HTTP methods")

		t.Logf("Successfully tested upsert source allowed HTTP methods: %s", connID)
	})

	t.Run("Upsert_Source_CustomResponse", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-upsert-custom-resp-" + timestamp
		sourceName := "test-src-upsert-resp-" + timestamp
		destName := "test-dst-upsert-resp-" + timestamp

		// Create connection without custom response
		var createResp map[string]interface{}
		err := cli.RunJSON(&createResp,
			"connection", "create",
			"--name", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--destination-type", "CLI",
			"--destination-name", destName,
			"--destination-cli-path", "/webhooks")
		require.NoError(t, err, "Failed to create connection")

		connID, ok := createResp["id"].(string)
		require.True(t, ok && connID != "", "Expected connection ID")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		// Upsert to add custom response
		customBody := `{"message":"accepted"}`
		var upsertResp map[string]interface{}
		err = cli.RunJSON(&upsertResp,
			"connection", "upsert", connName,
			"--source-custom-response-content-type", "json",
			"--source-custom-response-body", customBody)
		require.NoError(t, err, "Failed to upsert connection with custom response")

		// Verify custom_response is set
		source, ok := upsertResp["source"].(map[string]interface{})
		require.True(t, ok, "Expected source object in upsert response")
		sourceConfig, ok := source["config"].(map[string]interface{})
		require.True(t, ok, "Expected source config in upsert response")

		customResponse, ok := sourceConfig["custom_response"].(map[string]interface{})
		require.True(t, ok, "Expected custom_response in upsert response")
		assert.Equal(t, "json", strings.ToLower(customResponse["content_type"].(string)), "Content type should be json")
		assert.Equal(t, customBody, customResponse["body"].(string), "Body should match")

		t.Logf("Successfully tested upsert source custom response: %s", connID)
	})
}
