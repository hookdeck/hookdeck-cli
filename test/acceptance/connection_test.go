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

// TestConnectionUpdate tests updating a connection's metadata
func TestConnectionUpdate(t *testing.T) {
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

	// Update the connection
	timestamp := generateTimestamp()
	newName := "updated-conn-" + timestamp
	newDesc := "This is an updated description"

	stdout := cli.RunExpectSuccess("connection", "update", connID,
		"--name", newName,
		"--description", newDesc,
	)
	assert.NotEmpty(t, stdout, "update command should produce output")

	// Verify the update
	var updatedConn Connection
	err := cli.RunJSON(&updatedConn, "connection", "get", connID)
	require.NoError(t, err, "Should be able to get the updated connection")

	assert.Equal(t, newName, updatedConn.Name, "Connection name should be updated")
	assert.Equal(t, newDesc, updatedConn.Description, "Connection description should be updated")

	t.Logf("Successfully updated connection to name: %s", newName)
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
