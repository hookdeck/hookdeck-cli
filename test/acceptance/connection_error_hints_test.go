package acceptance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConnectionCreateWithNonExistentSourceID tests error hints when source ID doesn't exist
func TestConnectionCreateWithNonExistentSourceID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-bad-src-" + timestamp
	destName := "test-dst-" + timestamp
	fakeSourceID := "src_nonexistent123"

	// Try to create connection with non-existent source ID
	stdout, stderr, err := cli.Run("gateway", "connection", "create",
		"--name", connName,
		"--source-id", fakeSourceID,
		"--destination-name", destName,
		"--destination-type", "CLI",
		"--destination-cli-path", "/webhooks",
	)

	require.Error(t, err, "Should fail when source ID doesn't exist")
	combinedOutput := stdout + stderr

	// Verify error message contains helpful hints
	assert.Contains(t, combinedOutput, "failed to create connection", "Should indicate connection creation failed")
	assert.Contains(t, combinedOutput, "Hints:", "Should contain hints section")
	assert.Contains(t, combinedOutput, "--source-id", "Hint should mention --source-id flag")
	assert.Contains(t, combinedOutput, fakeSourceID, "Hint should include the provided source ID")
	assert.Contains(t, combinedOutput, "src_", "Hint should mention source ID prefix format")

	t.Logf("Successfully verified error hints for non-existent source ID")
}

// TestConnectionCreateWithNonExistentDestinationID tests error hints when destination ID doesn't exist
func TestConnectionCreateWithNonExistentDestinationID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-bad-dst-" + timestamp
	sourceName := "test-src-" + timestamp
	fakeDestinationID := "des_nonexistent123"

	// Try to create connection with non-existent destination ID
	stdout, stderr, err := cli.Run("gateway", "connection", "create",
		"--name", connName,
		"--source-name", sourceName,
		"--source-type", "WEBHOOK",
		"--destination-id", fakeDestinationID,
	)

	require.Error(t, err, "Should fail when destination ID doesn't exist")
	combinedOutput := stdout + stderr

	// Verify error message contains helpful hints
	assert.Contains(t, combinedOutput, "failed to create connection", "Should indicate connection creation failed")
	assert.Contains(t, combinedOutput, "Hints:", "Should contain hints section")
	assert.Contains(t, combinedOutput, "--destination-id", "Hint should mention --destination-id flag")
	assert.Contains(t, combinedOutput, fakeDestinationID, "Hint should include the provided destination ID")
	assert.Contains(t, combinedOutput, "des_", "Hint should mention destination ID prefix format")

	t.Logf("Successfully verified error hints for non-existent destination ID")
}

// TestConnectionCreateWithWrongIDType tests error hints when wrong ID type is provided
// This reproduces the bug from issue #204 where a connection ID was passed as source ID
func TestConnectionCreateWithWrongIDType(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-wrong-id-type-" + timestamp
	destName := "test-dst-" + timestamp
	// Using a connection ID format (web_) instead of source ID format (src_)
	wrongIDType := "web_y0A7nz0tRxZy"

	// Try to create connection with wrong ID type
	stdout, stderr, err := cli.Run("gateway", "connection", "create",
		"--name", connName,
		"--source-id", wrongIDType,
		"--destination-name", destName,
		"--destination-type", "HTTP",
		"--destination-url", "https://example.com/webhooks",
	)

	require.Error(t, err, "Should fail when wrong ID type is provided")
	combinedOutput := stdout + stderr

	// Verify error message contains helpful hints about correct ID format
	assert.Contains(t, combinedOutput, "failed to create connection", "Should indicate connection creation failed")
	assert.Contains(t, combinedOutput, "Hints:", "Should contain hints section")
	assert.Contains(t, combinedOutput, "--source-id", "Hint should mention --source-id flag")
	assert.Contains(t, combinedOutput, wrongIDType, "Hint should include the provided ID")
	assert.Contains(t, combinedOutput, "src_", "Hint should mention correct source ID prefix")
	assert.Contains(t, combinedOutput, "verify the resource IDs", "Should suggest verifying resource IDs")

	t.Logf("Successfully verified error hints for wrong ID type (issue #204 scenario)")
}

// TestConnectionUpsertWithNonExistentSourceID tests error hints for upsert with non-existent source ID
func TestConnectionUpsertWithNonExistentSourceID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-upsert-bad-src-" + timestamp
	destName := "test-dst-" + timestamp
	fakeSourceID := "src_nonexistent456"

	// Try to upsert connection with non-existent source ID
	stdout, stderr, err := cli.Run("gateway", "connection", "upsert", connName,
		"--source-id", fakeSourceID,
		"--destination-name", destName,
		"--destination-type", "CLI",
		"--destination-cli-path", "/webhooks",
	)

	require.Error(t, err, "Should fail when source ID doesn't exist")
	combinedOutput := stdout + stderr

	// Verify error message contains helpful hints
	assert.Contains(t, combinedOutput, "failed to upsert connection", "Should indicate connection upsert failed")
	assert.Contains(t, combinedOutput, "Hints:", "Should contain hints section")
	assert.Contains(t, combinedOutput, "--source-id", "Hint should mention --source-id flag")
	assert.Contains(t, combinedOutput, fakeSourceID, "Hint should include the provided source ID")
	assert.Contains(t, combinedOutput, "src_", "Hint should mention source ID prefix format")

	t.Logf("Successfully verified error hints for upsert with non-existent source ID")
}

// TestConnectionUpsertWithNonExistentDestinationID tests error hints for upsert with non-existent destination ID
func TestConnectionUpsertWithNonExistentDestinationID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-upsert-bad-dst-" + timestamp
	sourceName := "test-src-" + timestamp
	fakeDestinationID := "des_nonexistent456"

	// Try to upsert connection with non-existent destination ID
	stdout, stderr, err := cli.Run("gateway", "connection", "upsert", connName,
		"--source-name", sourceName,
		"--source-type", "WEBHOOK",
		"--destination-id", fakeDestinationID,
	)

	require.Error(t, err, "Should fail when destination ID doesn't exist")
	combinedOutput := stdout + stderr

	// Verify error message contains helpful hints
	assert.Contains(t, combinedOutput, "failed to upsert connection", "Should indicate connection upsert failed")
	assert.Contains(t, combinedOutput, "Hints:", "Should contain hints section")
	assert.Contains(t, combinedOutput, "--destination-id", "Hint should mention --destination-id flag")
	assert.Contains(t, combinedOutput, fakeDestinationID, "Hint should include the provided destination ID")
	assert.Contains(t, combinedOutput, "des_", "Hint should mention destination ID prefix format")

	t.Logf("Successfully verified error hints for upsert with non-existent destination ID")
}

// TestConnectionCreateWithExistingSourceID tests that create works correctly with a valid existing source ID
// This is a positive test to ensure the --source-id flag works when the source exists
func TestConnectionCreateWithExistingSourceID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	// First, create a connection to get a source we can reuse
	initialConnName := "test-initial-" + timestamp
	sourceName := "test-reusable-src-" + timestamp
	initialDestName := "test-initial-dst-" + timestamp

	var initialConn Connection
	err := cli.RunJSON(&initialConn,
		"gateway", "connection", "create",
		"--name", initialConnName,
		"--source-name", sourceName,
		"--source-type", "WEBHOOK",
		"--destination-name", initialDestName,
		"--destination-type", "CLI",
		"--destination-cli-path", "/initial",
	)
	require.NoError(t, err, "Should create initial connection")
	require.NotEmpty(t, initialConn.ID, "Initial connection should have ID")

	// Get the source ID from the created connection
	var connDetails map[string]interface{}
	err = cli.RunJSON(&connDetails, "gateway", "connection", "get", initialConn.ID)
	require.NoError(t, err, "Should get connection details")

	source, ok := connDetails["source"].(map[string]interface{})
	require.True(t, ok, "Should have source in connection")
	sourceID, ok := source["id"].(string)
	require.True(t, ok && sourceID != "", "Should have source ID")

	t.Logf("Created initial connection with source ID: %s", sourceID)

	// Cleanup initial connection
	t.Cleanup(func() {
		deleteConnection(t, cli, initialConn.ID)
	})

	// Now create a new connection using the existing source ID
	newConnName := "test-with-src-id-" + timestamp
	newDestName := "test-new-dst-" + timestamp

	var newConn Connection
	err = cli.RunJSON(&newConn,
		"gateway", "connection", "create",
		"--name", newConnName,
		"--source-id", sourceID,
		"--destination-name", newDestName,
		"--destination-type", "CLI",
		"--destination-cli-path", "/new",
	)
	require.NoError(t, err, "Should create connection with existing source ID")
	require.NotEmpty(t, newConn.ID, "New connection should have ID")

	// Cleanup new connection
	t.Cleanup(func() {
		deleteConnection(t, cli, newConn.ID)
	})

	// Verify the connection uses the same source
	assert.Equal(t, sourceName, newConn.Source.Name, "Should use the existing source")
	assert.Equal(t, newDestName, newConn.Destination.Name, "Should have new destination")

	t.Logf("Successfully created connection %s using existing source ID %s", newConn.ID, sourceID)
}
