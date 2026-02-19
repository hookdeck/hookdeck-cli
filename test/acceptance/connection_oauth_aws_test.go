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
		stdout, stderr, err := cli.Run("gateway", "connection", "create",
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

		authType, ok := destConfig["auth_type"].(string)
		require.True(t, ok, "Expected auth_type string in destination config, got config: %v", destConfig)
		assert.Equal(t, "OAUTH2_CLIENT_CREDENTIALS", authType, "Auth type should be OAUTH2_CLIENT_CREDENTIALS")

		// Fetch connection with --include-destination-auth to verify credentials were stored
		getStdout, getStderr, getErr := cli.Run("gateway", "connection", "get", connID,
			"--include-destination-auth",
			"--output", "json")
		require.NoError(t, getErr, "Failed to get connection: stderr=%s", getStderr)

		var getResp map[string]interface{}
		err = json.Unmarshal([]byte(getStdout), &getResp)
		require.NoError(t, err, "Failed to parse get response: %s", getStdout)

		getDest, ok := getResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination in get response")
		getConfig, ok := getDest["config"].(map[string]interface{})
		require.True(t, ok, "Expected config in get response destination")

		getAuthType, ok := getConfig["auth_type"].(string)
		require.True(t, ok, "Expected auth_type in get response config: %v", getConfig)
		assert.Equal(t, "OAUTH2_CLIENT_CREDENTIALS", getAuthType, "Auth type should match on get")

		getAuth, ok := getConfig["auth"].(map[string]interface{})
		require.True(t, ok, "Expected auth object in get response config: %v", getConfig)
		assert.Equal(t, "https://auth.example.com/oauth/token", getAuth["auth_server"], "Auth server should match")
		assert.Equal(t, "client_123", getAuth["client_id"], "Client ID should match")
		assert.Equal(t, "secret_456", getAuth["client_secret"], "Client secret should match with --include-destination-auth")

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
		stdout, stderr, err := cli.Run("gateway", "connection", "create",
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

		authType, ok := destConfig["auth_type"].(string)
		require.True(t, ok, "Expected auth_type string in destination config, got config: %v", destConfig)
		assert.Equal(t, "OAUTH2_AUTHORIZATION_CODE", authType, "Auth type should be OAUTH2_AUTHORIZATION_CODE")

		// Fetch connection with --include-destination-auth to verify credentials were stored
		getStdout, getStderr, getErr := cli.Run("gateway", "connection", "get", connID,
			"--include-destination-auth",
			"--output", "json")
		require.NoError(t, getErr, "Failed to get connection: stderr=%s", getStderr)

		var getResp map[string]interface{}
		err = json.Unmarshal([]byte(getStdout), &getResp)
		require.NoError(t, err, "Failed to parse get response: %s", getStdout)

		getDest, ok := getResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination in get response")
		getConfig, ok := getDest["config"].(map[string]interface{})
		require.True(t, ok, "Expected config in get response destination")

		getAuthType, ok := getConfig["auth_type"].(string)
		require.True(t, ok, "Expected auth_type in get response config: %v", getConfig)
		assert.Equal(t, "OAUTH2_AUTHORIZATION_CODE", getAuthType, "Auth type should match on get")

		getAuth, ok := getConfig["auth"].(map[string]interface{})
		require.True(t, ok, "Expected auth object in get response config: %v", getConfig)
		assert.Equal(t, "https://auth.example.com/oauth/token", getAuth["auth_server"], "Auth server should match")
		assert.Equal(t, "client_789", getAuth["client_id"], "Client ID should match")
		assert.Equal(t, "secret_abc", getAuth["client_secret"], "Client secret should match with --include-destination-auth")

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
		stdout, stderr, err := cli.Run("gateway", "connection", "create",
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

		authType, ok := destConfig["auth_type"].(string)
		require.True(t, ok, "Expected auth_type string in destination config, got config: %v", destConfig)
		assert.Equal(t, "AWS_SIGNATURE", authType, "Auth type should be AWS_SIGNATURE")

		// Fetch connection with --include-destination-auth to verify credentials were stored
		getStdout, getStderr, getErr := cli.Run("gateway", "connection", "get", connID,
			"--include-destination-auth",
			"--output", "json")
		require.NoError(t, getErr, "Failed to get connection: stderr=%s", getStderr)

		var getResp map[string]interface{}
		err = json.Unmarshal([]byte(getStdout), &getResp)
		require.NoError(t, err, "Failed to parse get response: %s", getStdout)

		getDest, ok := getResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination in get response")
		getConfig, ok := getDest["config"].(map[string]interface{})
		require.True(t, ok, "Expected config in get response destination")

		getAuthType, ok := getConfig["auth_type"].(string)
		require.True(t, ok, "Expected auth_type in get response config: %v", getConfig)
		assert.Equal(t, "AWS_SIGNATURE", getAuthType, "Auth type should match on get")

		getAuth, ok := getConfig["auth"].(map[string]interface{})
		require.True(t, ok, "Expected auth object in get response config: %v", getConfig)
		assert.Equal(t, "AKIAIOSFODNN7EXAMPLE", getAuth["access_key_id"], "AWS access key ID should match")
		assert.Equal(t, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", getAuth["secret_access_key"], "AWS secret access key should match")
		assert.Equal(t, "us-east-1", getAuth["region"], "AWS region should match")
		assert.Equal(t, "execute-api", getAuth["service"], "AWS service should match")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		t.Logf("Successfully tested HTTP destination with AWS Signature: %s", connID)
	})

	t.Run("HTTP_Destination_GCP_ServiceAccount", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-gcp-sa-conn-" + timestamp
		sourceName := "test-gcp-sa-source-" + timestamp
		destName := "test-gcp-sa-dest-" + timestamp
		destURL := "https://api.hookdeck.com/dev/null"

		// Create connection with HTTP destination (GCP Service Account)
		// Using a minimal but valid JSON structure for service account key
		serviceAccountKey := `{"type":"service_account","project_id":"test-project","private_key_id":"test-key-id","private_key":"-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC\n-----END PRIVATE KEY-----\n","client_email":"test@test-project.iam.gserviceaccount.com","client_id":"123456789","auth_uri":"https://accounts.google.com/o/oauth2/auth","token_uri":"https://oauth2.googleapis.com/token"}`

		stdout, stderr, err := cli.Run("gateway", "connection", "create",
			"--name", connName,
			"--source-type", "WEBHOOK",
			"--source-name", sourceName,
			"--destination-type", "HTTP",
			"--destination-name", destName,
			"--destination-url", destURL,
			"--destination-auth-method", "gcp",
			"--destination-gcp-service-account-key", serviceAccountKey,
			"--destination-gcp-scope", "https://www.googleapis.com/auth/cloud-platform",
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

		authType, ok := destConfig["auth_type"].(string)
		require.True(t, ok, "Expected auth_type string in destination config, got config: %v", destConfig)
		assert.Equal(t, "GCP_SERVICE_ACCOUNT", authType, "Auth type should be GCP_SERVICE_ACCOUNT")

		// Fetch connection with --include-destination-auth to verify credentials were stored
		getStdout, getStderr, getErr := cli.Run("gateway", "connection", "get", connID,
			"--include-destination-auth",
			"--output", "json")
		require.NoError(t, getErr, "Failed to get connection: stderr=%s", getStderr)

		var getResp map[string]interface{}
		err = json.Unmarshal([]byte(getStdout), &getResp)
		require.NoError(t, err, "Failed to parse get response: %s", getStdout)

		getDest, ok := getResp["destination"].(map[string]interface{})
		require.True(t, ok, "Expected destination in get response")
		getConfig, ok := getDest["config"].(map[string]interface{})
		require.True(t, ok, "Expected config in get response destination")

		getAuthType, ok := getConfig["auth_type"].(string)
		require.True(t, ok, "Expected auth_type in get response config: %v", getConfig)
		assert.Equal(t, "GCP_SERVICE_ACCOUNT", getAuthType, "Auth type should match on get")

		getAuth, ok := getConfig["auth"].(map[string]interface{})
		require.True(t, ok, "Expected auth object in get response config: %v", getConfig)
		assert.Equal(t, "https://www.googleapis.com/auth/cloud-platform", getAuth["scope"], "GCP scope should match")
		assert.NotEmpty(t, getAuth["service_account_key"], "Service account key should be present with --include-destination-auth")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, connID)
		})

		t.Logf("Successfully tested HTTP destination with GCP Service Account: %s", connID)
	})
}
