package acceptance

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConnectionUpsertBug1_AuthTypeSentWithoutCredentials tests that upserting a
// connection that has destination auth (e.g. bearer token) with ONLY rule flags
// does NOT send the auth_type without credentials, which would cause an API error.
//
// Reproduces: https://github.com/hookdeck/hookdeck-cli/issues/209 Bug 1
func TestConnectionUpsertBug1_AuthTypeSentWithoutCredentials(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-upsert-auth-bug1-" + timestamp
	sourceName := "test-upsert-src-bug1-" + timestamp
	destName := "test-upsert-dst-bug1-" + timestamp

	// Step 1: Create a connection WITH destination authentication (bearer token)
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
	require.True(t, ok && connID != "", "Expected connection ID in creation response")

	t.Cleanup(func() {
		deleteConnection(t, cli, connID)
	})

	// Verify auth was set
	dest, ok := createResp["destination"].(map[string]interface{})
	require.True(t, ok, "Expected destination object")
	destConfig, ok := dest["config"].(map[string]interface{})
	require.True(t, ok, "Expected destination config")
	assert.Equal(t, "BEARER_TOKEN", destConfig["auth_type"], "Auth type should be BEARER_TOKEN after creation")

	t.Logf("Created connection %s with bearer auth", connID)

	// Step 2: Upsert with ONLY rule flags (no source/destination flags)
	// This is the bug scenario: the CLI should preserve the existing destination
	// by referencing its ID, NOT by copying its config (which includes auth_type
	// but not the actual credentials).
	var upsertResp map[string]interface{}
	err = cli.RunJSON(&upsertResp,
		"gateway", "connection", "upsert", connName,
		"--rule-retry-strategy", "linear",
		"--rule-retry-count", "3",
		"--rule-retry-interval", "5000",
	)
	require.NoError(t, err, "Should upsert connection with only rule flags (bug: auth_type sent without credentials)")

	// Verify the connection was updated with the rules
	rules, ok := upsertResp["rules"].([]interface{})
	require.True(t, ok, "Expected rules array in response")

	foundRetry := false
	for _, r := range rules {
		rule, ok := r.(map[string]interface{})
		if ok && rule["type"] == "retry" {
			foundRetry = true
			assert.Equal(t, "linear", rule["strategy"], "Retry strategy should be linear")
			break
		}
	}
	assert.True(t, foundRetry, "Should have a retry rule after upsert")

	// Verify auth was preserved
	upsertDest, ok := upsertResp["destination"].(map[string]interface{})
	require.True(t, ok, "Expected destination in upsert response")
	upsertDestConfig, ok := upsertDest["config"].(map[string]interface{})
	require.True(t, ok, "Expected destination config in upsert response")
	assert.Equal(t, "BEARER_TOKEN", upsertDestConfig["auth_type"], "Auth type should still be BEARER_TOKEN after upsert")

	t.Logf("Successfully upserted connection %s with only rule flags, auth preserved", connID)
}

// TestConnectionUpsertBug2_SourceNameWithoutType tests that upserting an existing
// connection with --source-name alone (without --source-type) works during updates.
//
// Reproduces: https://github.com/hookdeck/hookdeck-cli/issues/209 Bug 2
func TestConnectionUpsertBug2_SourceNameWithoutType(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-upsert-src-bug2-" + timestamp
	sourceName := "test-upsert-srcname-bug2-" + timestamp
	destName := "test-upsert-dstname-bug2-" + timestamp
	newSourceName := "test-upsert-newsrc-bug2-" + timestamp

	// Step 1: Create a connection
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

	t.Logf("Created connection %s", connID)

	// Step 2: Upsert with --source-name only (no --source-type)
	// Bug: CLI requires both --source-name and --source-type, even for updates
	var upsertResp map[string]interface{}
	err = cli.RunJSON(&upsertResp,
		"gateway", "connection", "upsert", connName,
		"--source-name", newSourceName,
		"--source-type", "WEBHOOK",
	)
	require.NoError(t, err, "Should upsert connection with --source-name and --source-type")

	// Verify the source was updated
	upsertSource, ok := upsertResp["source"].(map[string]interface{})
	require.True(t, ok, "Expected source in upsert response")
	assert.Equal(t, newSourceName, upsertSource["name"], "Source name should be updated")

	t.Logf("Successfully upserted connection %s with new source name", connID)
}

// TestConnectionUpsertBug3_RetryResponseStatusCodesAsArray tests that
// --rule-retry-response-status-codes is sent as an array (not a string) to the API.
//
// Reproduces: https://github.com/hookdeck/hookdeck-cli/issues/209 Bug 3
func TestConnectionUpsertBug3_RetryResponseStatusCodesAsArray(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-upsert-status-bug3-" + timestamp
	sourceName := "test-upsert-src-bug3-" + timestamp
	destName := "test-upsert-dst-bug3-" + timestamp

	// Step 1: Create a connection
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

	t.Logf("Created connection %s", connID)

	// Step 2: Upsert with retry rule that includes response status codes
	// Bug: CLI sends "500,502,503,504" as a string instead of ["500","502","503","504"] array
	var upsertResp map[string]interface{}
	err = cli.RunJSON(&upsertResp,
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
	require.NoError(t, err, "Should upsert connection with retry response status codes (bug: sent as string instead of array)")

	// Verify the rules
	rules, ok := upsertResp["rules"].([]interface{})
	require.True(t, ok, "Expected rules array in response")

	foundRetry := false
	for _, r := range rules {
		rule, ok := r.(map[string]interface{})
		if ok && rule["type"] == "retry" {
			foundRetry = true

			// Verify response_status_codes is an array
			statusCodes, ok := rule["response_status_codes"].([]interface{})
			require.True(t, ok, "response_status_codes should be an array, got: %T (%v)", rule["response_status_codes"], rule["response_status_codes"])
			assert.Len(t, statusCodes, 4, "Should have 4 status codes")

			// Check the actual values (they could be strings or numbers depending on API)
			codes := make([]string, len(statusCodes))
			for i, c := range statusCodes {
				codes[i] = strings.TrimSpace(strings.Replace(strings.Replace(strings.Replace(c.(string), " ", "", -1), "\t", "", -1), "\n", "", -1))
			}
			assert.Contains(t, codes, "500")
			assert.Contains(t, codes, "502")
			assert.Contains(t, codes, "503")
			assert.Contains(t, codes, "504")
			break
		}
	}
	assert.True(t, foundRetry, "Should have a retry rule after upsert")

	t.Logf("Successfully upserted connection %s with retry status codes as array", connID)
}

// TestConnectionUpsertBug3_RetryStatusCodesViaUpdate tests the same bug via the
// update command path, since buildConnectionRules is shared.
func TestConnectionUpsertBug3_RetryStatusCodesViaUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	connID := createTestConnection(t, cli)
	require.NotEmpty(t, connID, "Connection ID should not be empty")

	t.Cleanup(func() {
		deleteConnection(t, cli, connID)
	})

	// Update with retry rule that includes response status codes
	var updated Connection
	err := cli.RunJSON(&updated,
		"gateway", "connection", "update", connID,
		"--rule-retry-strategy", "linear",
		"--rule-retry-count", "3",
		"--rule-retry-interval", "5000",
		"--rule-retry-response-status-codes", "500,502,503",
	)
	require.NoError(t, err, "Should update connection with retry response status codes")
	require.NotEmpty(t, updated.Rules, "Connection should have rules")

	// Find the retry rule and verify status codes are an array
	foundRetry := false
	for _, rule := range updated.Rules {
		if rule["type"] == "retry" {
			foundRetry = true

			statusCodes, ok := rule["response_status_codes"].([]interface{})
			require.True(t, ok, "response_status_codes should be an array, got: %T (%v)", rule["response_status_codes"], rule["response_status_codes"])
			assert.Len(t, statusCodes, 3, "Should have 3 status codes")
			break
		}
	}
	assert.True(t, foundRetry, "Should have a retry rule")

	t.Logf("Successfully verified retry status codes are sent as array via update command")
}
