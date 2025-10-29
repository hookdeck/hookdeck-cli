package acceptance

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConnectionOAuth2AWSAuthentication tests OAuth2 and AWS authentication types
func TestConnectionOAuth2AWSAuthentication(t *testing.T) {
	t.Run("HTTP_Destination_OAuth2_ClientCredentials", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-oauth2-cc-conn-" + timestamp
		sourceName := "test-oauth2-cc-source-" + timestamp
		destName := "test-oauth2-cc-dest-" + timestamp
		destURL := "https://api.hookdeck.com/dev/null"

		// Create connection with HTTP destination (OAuth2 Client Credentials)
		stdout, stderr, err := cli.Run("connection", "create",
			"--name", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--destination-type", "HTTP",
			"--destination-name", destName,
			"--destination-url", destURL,
			"--destination-auth-method", "oauth2_client_credentials",
			"--destination-oauth2-auth-server", "https://auth.example.com/oauth/token",
			"--destination-oauth2-client-id", "client_123",
			"--destination-oauth2-client-secret", "secret_456",
			"--destination-oauth2-scopes", "read,write",
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
			assert.Equal(t, "OAUTH2_CLIENT_CREDENTIALS", authMethod["type"], "Auth type should be OAUTH2_CLIENT_CREDENTIALS")
			assert.Equal(t, "https://auth.example.com/oauth/token", authMethod["auth_server"], "Auth server should match")
			assert.Equal(t, "client_123", authMethod["client_id"], "Client ID should match")
			// Client secret and scopes may or may not be returned depending on API
		}

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		t.Logf("Successfully tested HTTP destination with OAuth2 Client Credentials: %s", connID)
	})

	t.Run("HTTP_Destination_OAuth2_AuthorizationCode", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-oauth2-ac-conn-" + timestamp
		sourceName := "test-oauth2-ac-source-" + timestamp
		destName := "test-oauth2-ac-dest-" + timestamp
		destURL := "https://api.hookdeck.com/dev/null"

		// Create connection with HTTP destination (OAuth2 Authorization Code)
		stdout, stderr, err := cli.Run("connection", "create",
			"--name", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--destination-type", "HTTP",
			"--destination-name", destName,
			"--destination-url", destURL,
			"--destination-auth-method", "oauth2_authorization_code",
			"--destination-oauth2-auth-server", "https://auth.example.com/oauth/token",
			"--destination-oauth2-client-id", "client_789",
			"--destination-oauth2-client-secret", "secret_abc",
			"--destination-oauth2-refresh-token", "refresh_xyz",
			"--destination-oauth2-scopes", "profile,email",
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
			assert.Equal(t, "OAUTH2_AUTHORIZATION_CODE", authMethod["type"], "Auth type should be OAUTH2_AUTHORIZATION_CODE")
			assert.Equal(t, "https://auth.example.com/oauth/token", authMethod["auth_server"], "Auth server should match")
			assert.Equal(t, "client_789", authMethod["client_id"], "Client ID should match")
			// Sensitive fields like client_secret, refresh_token may not be returned
		}

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		t.Logf("Successfully tested HTTP destination with OAuth2 Authorization Code: %s", connID)
	})

	t.Run("HTTP_Destination_AWS_Signature", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-aws-sig-conn-" + timestamp
		sourceName := "test-aws-sig-source-" + timestamp
		destName := "test-aws-sig-dest-" + timestamp
		destURL := "https://api.hookdeck.com/dev/null"

		// Create connection with HTTP destination (AWS Signature)
		stdout, stderr, err := cli.Run("connection", "create",
			"--name", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--destination-type", "HTTP",
			"--destination-name", destName,
			"--destination-url", destURL,
			"--destination-auth-method", "aws",
			"--destination-aws-access-key-id", "AKIAIOSFODNN7EXAMPLE",
			"--destination-aws-secret-access-key", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			"--destination-aws-region", "us-east-1",
			"--destination-aws-service", "execute-api",
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
			assert.Equal(t, "AWS_SIGNATURE", authMethod["type"], "Auth type should be AWS_SIGNATURE")
			assert.Equal(t, "us-east-1", authMethod["region"], "AWS region should match")
			assert.Equal(t, "execute-api", authMethod["service"], "AWS service should match")
			// Access key may be returned but secret key should not be for security
		}

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		t.Logf("Successfully tested HTTP destination with AWS Signature: %s", connID)
	})
}
