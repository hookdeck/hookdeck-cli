package acceptance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConnectionUpdateDescription tests updating a connection's description by ID
func TestConnectionUpdateDescription(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	connID := createTestConnection(t, cli)
	require.NotEmpty(t, connID, "Connection ID should not be empty")

	t.Cleanup(func() {
		deleteConnection(t, cli, connID)
	})

	// Update description via gateway path
	newDesc := "Updated via connection update test"
	var updated Connection
	err := cli.RunJSON(&updated,
		"gateway", "connection", "update", connID,
		"--description", newDesc,
	)
	require.NoError(t, err, "Should update connection description")
	assert.Equal(t, connID, updated.ID, "Connection ID should match")
	assert.Equal(t, newDesc, updated.Description, "Description should be updated")

	// Verify via GET
	var fetched Connection
	err = cli.RunJSON(&fetched, "gateway", "connection", "get", connID)
	require.NoError(t, err, "Should get updated connection")
	assert.Equal(t, newDesc, fetched.Description, "Description should be persisted")

	t.Logf("Successfully updated connection description: %s", connID)
}

// TestConnectionUpdateRename tests renaming a connection by ID
// This is the key use case for update vs upsert -- upsert uses name as identifier
// so cannot rename, but update uses ID and can change the name
func TestConnectionUpdateRename(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-rename-" + timestamp
	sourceName := "test-rename-src-" + timestamp
	destName := "test-rename-dst-" + timestamp

	// Create connection
	var conn Connection
	err := cli.RunJSON(&conn,
		"gateway", "connection", "create",
		"--name", connName,
		"--source-name", sourceName,
		"--source-type", "WEBHOOK",
		"--destination-name", destName,
		"--destination-type", "CLI",
		"--destination-cli-path", "/webhooks",
	)
	require.NoError(t, err, "Should create connection")
	require.NotEmpty(t, conn.ID, "Connection should have an ID")

	t.Cleanup(func() {
		cli.Run("gateway", "connection", "delete", conn.ID, "--force")
	})

	// Rename via update
	newName := "test-renamed-" + timestamp
	var updated Connection
	err = cli.RunJSON(&updated,
		"gateway", "connection", "update", conn.ID,
		"--name", newName,
	)
	require.NoError(t, err, "Should rename connection")
	assert.Equal(t, conn.ID, updated.ID, "Connection ID should be unchanged")
	assert.Equal(t, newName, updated.Name, "Name should be updated")

	// Verify via GET
	var fetched Connection
	err = cli.RunJSON(&fetched, "gateway", "connection", "get", conn.ID)
	require.NoError(t, err, "Should get renamed connection")
	assert.Equal(t, newName, fetched.Name, "Name should be persisted")

	// Verify source and destination are preserved
	assert.Equal(t, sourceName, fetched.Source.Name, "Source should be preserved after rename")
	assert.Equal(t, destName, fetched.Destination.Name, "Destination should be preserved after rename")

	t.Logf("Successfully renamed connection: %s -> %s", connName, newName)
}

// TestConnectionUpdateRules tests updating rules via the update command
func TestConnectionUpdateRules(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	connID := createTestConnection(t, cli)
	require.NotEmpty(t, connID, "Connection ID should not be empty")

	t.Cleanup(func() {
		deleteConnection(t, cli, connID)
	})

	// Add a retry rule via update
	var updated Connection
	err := cli.RunJSON(&updated,
		"gateway", "connection", "update", connID,
		"--rule-retry-strategy", "linear",
		"--rule-retry-count", "3",
		"--rule-retry-interval", "5000",
	)
	require.NoError(t, err, "Should update connection rules")
	assert.Equal(t, connID, updated.ID, "Connection ID should match")
	require.NotEmpty(t, updated.Rules, "Connection should have rules")

	// Verify via GET
	var fetched Connection
	err = cli.RunJSON(&fetched, "gateway", "connection", "get", connID)
	require.NoError(t, err, "Should get updated connection")
	require.NotEmpty(t, fetched.Rules, "Rules should be persisted")

	// Find the retry rule
	foundRetry := false
	for _, rule := range fetched.Rules {
		if rule["type"] == "retry" {
			foundRetry = true
			assert.Equal(t, "linear", rule["strategy"], "Retry strategy should be linear")
			assert.Equal(t, float64(3), rule["count"], "Retry count should be 3")
			assert.Equal(t, float64(5000), rule["interval"], "Retry interval should be 5000")
			break
		}
	}
	assert.True(t, foundRetry, "Should have a retry rule")

	t.Logf("Successfully updated connection rules: %s", connID)
}

// TestConnectionUpdateNotFound tests error handling when updating a non-existent connection
func TestConnectionUpdateNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	_, _, err := cli.Run("gateway", "connection", "update", "web_nonexistent123",
		"--description", "This should fail",
	)
	require.Error(t, err, "Should fail when connection ID doesn't exist")

	t.Logf("Successfully verified error for non-existent connection update")
}

// TestConnectionUpdateNoChanges tests that update with no flags shows current state
func TestConnectionUpdateNoChanges(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	connID := createTestConnection(t, cli)
	require.NotEmpty(t, connID, "Connection ID should not be empty")

	t.Cleanup(func() {
		deleteConnection(t, cli, connID)
	})

	// Update with no flags -- should show current state
	stdout := cli.RunExpectSuccess("gateway", "connection", "update", connID)
	assert.Contains(t, stdout, "No changes specified", "Should indicate no changes")
	assert.Contains(t, stdout, connID, "Should show connection ID")

	t.Logf("Successfully verified no-op update: %s", connID)
}

// TestConnectionUpdateViaRootAlias tests that update works via the root connection alias too
func TestConnectionUpdateViaRootAlias(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	connID := createTestConnection(t, cli)
	require.NotEmpty(t, connID, "Connection ID should not be empty")

	t.Cleanup(func() {
		deleteConnection(t, cli, connID)
	})

	// Update via root alias (hookdeck connection update)
	newDesc := "Updated via root alias"
	var updated Connection
	err := cli.RunJSON(&updated,
		"connection", "update", connID,
		"--description", newDesc,
	)
	require.NoError(t, err, "Should update via root connection alias")
	assert.Equal(t, newDesc, updated.Description, "Description should be updated via alias")

	t.Logf("Successfully updated connection via root alias: %s", connID)
}

// TestConnectionUpdateOutputJSON verifies update with --output json returns valid JSON
func TestConnectionUpdateOutputJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	connID := createTestConnection(t, cli)
	require.NotEmpty(t, connID, "Connection ID should not be empty")

	t.Cleanup(func() {
		deleteConnection(t, cli, connID)
	})

	var updated Connection
	err := cli.RunJSON(&updated,
		"gateway", "connection", "update", connID,
		"--description", "JSON output test",
		"--output", "json",
	)
	require.NoError(t, err, "Should update with JSON output")
	assert.Equal(t, connID, updated.ID, "Response should contain connection ID")
	assert.Equal(t, "JSON output test", updated.Description, "Description should be in response")

	t.Logf("Connection update --output json verified: %s", connID)
}

// TestConnectionUpdateFilterRule verifies update with a filter rule
func TestConnectionUpdateFilterRule(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	connID := createTestConnection(t, cli)
	require.NotEmpty(t, connID, "Connection ID should not be empty")

	t.Cleanup(func() {
		deleteConnection(t, cli, connID)
	})

	filterBody := `{"type":"payment"}`
	var updated Connection
	err := cli.RunJSON(&updated,
		"gateway", "connection", "update", connID,
		"--rule-filter-body", filterBody,
	)
	require.NoError(t, err, "Should update with filter rule")
	require.NotEmpty(t, updated.Rules, "Connection should have rules")

	foundFilter := false
	for _, rule := range updated.Rules {
		if rule["type"] == "filter" {
			foundFilter = true
			assert.Equal(t, filterBody, rule["body"], "Filter body should match")
			break
		}
	}
	assert.True(t, foundFilter, "Should have a filter rule")

	t.Logf("Connection update with filter rule verified: %s", connID)
}

// TestConnectionUpdateDelayRule verifies update with a delay rule
func TestConnectionUpdateDelayRule(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	connID := createTestConnection(t, cli)
	require.NotEmpty(t, connID, "Connection ID should not be empty")

	t.Cleanup(func() {
		deleteConnection(t, cli, connID)
	})

	var updated Connection
	err := cli.RunJSON(&updated,
		"gateway", "connection", "update", connID,
		"--rule-delay", "2000",
	)
	require.NoError(t, err, "Should update with delay rule")
	require.NotEmpty(t, updated.Rules, "Connection should have rules")

	foundDelay := false
	for _, rule := range updated.Rules {
		if rule["type"] == "delay" {
			foundDelay = true
			assert.Equal(t, float64(2000), rule["delay"], "Delay should be 2000")
			break
		}
	}
	assert.True(t, foundDelay, "Should have a delay rule")

	t.Logf("Connection update with delay rule verified: %s", connID)
}

// TestConnectionUpdateRetryResponseStatusCodes verifies that
// --rule-retry-response-status-codes is sent as an array to the API.
// Regression test for https://github.com/hookdeck/hookdeck-cli/issues/209 Bug 3.
func TestConnectionUpdateRetryResponseStatusCodes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	connID := createTestConnection(t, cli)
	require.NotEmpty(t, connID, "Connection ID should not be empty")

	t.Cleanup(func() {
		deleteConnection(t, cli, connID)
	})

	// Update with retry rule including response status codes
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

	t.Logf("Successfully verified retry status codes are sent as array via update: %s", connID)
}

// TestConnectionUpdateWithRulesJSON verifies update with --rules JSON string
func TestConnectionUpdateWithRulesJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	connID := createTestConnection(t, cli)
	require.NotEmpty(t, connID, "Connection ID should not be empty")

	t.Cleanup(func() {
		deleteConnection(t, cli, connID)
	})

	rulesJSON := `[{"type":"retry","strategy":"exponential","count":2,"interval":10000}]`
	var updated Connection
	err := cli.RunJSON(&updated,
		"gateway", "connection", "update", connID,
		"--rules", rulesJSON,
	)
	require.NoError(t, err, "Should update with --rules JSON")
	require.Len(t, updated.Rules, 1, "Should have one rule")
	assert.Equal(t, "retry", updated.Rules[0]["type"], "Rule type should be retry")
	assert.Equal(t, "exponential", updated.Rules[0]["strategy"], "Strategy should be exponential")

	t.Logf("Connection update with --rules JSON verified: %s", connID)
}
