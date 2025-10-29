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
		"--rule-filter-body", `{"$.type":"payment"}`,
		"--rule-filter-headers", `{"$.content-type":"application/json"}`,
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
	assert.Equal(t, `{"$.type":"payment"}`, rule["body"], "Filter body should match input")
	assert.Equal(t, `{"$.content-type":"application/json"}`, rule["headers"], "Filter headers should match input")

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
		"--rule-filter-body", `{"$.type":"payment"}`,
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
	assert.Equal(t, `{"$.type":"payment"}`, getConn.Rules[0]["body"], "Filter should have body expression")

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
}
