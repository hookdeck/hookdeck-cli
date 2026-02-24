package acceptance

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGatewayHelpShowsSubcommands verifies that hookdeck gateway --help lists
// the connection subcommand (and future subcommands as they are added)
func TestGatewayHelpShowsSubcommands(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	stdout := cli.RunExpectSuccess("gateway", "--help")

	// Connection should be listed as a subcommand
	assert.Contains(t, stdout, "connection", "gateway --help should list 'connection' subcommand")
	assert.Contains(t, stdout, "Commands for managing Event Gateway", "Should show gateway description")

	t.Logf("Gateway help output verified")
}

// TestGatewayConnectionListWorks verifies that hookdeck gateway connection list
// returns a successful response (same as hookdeck connection list)
func TestGatewayConnectionListWorks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	// List via gateway path
	stdout := cli.RunExpectSuccess("gateway", "connection", "list")
	assert.NotEmpty(t, stdout, "gateway connection list should produce output")

	t.Logf("Gateway connection list output: %s", stdout)
}

// TestGatewayConnectionCreateAndGet verifies full CRUD via the gateway path
func TestGatewayConnectionCreateAndGet(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-gw-conn-" + timestamp
	sourceName := "test-gw-src-" + timestamp
	destName := "test-gw-dst-" + timestamp

	// Create via gateway path
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
	require.NoError(t, err, "Should create connection via gateway path")
	require.NotEmpty(t, conn.ID, "Connection should have an ID")

	t.Cleanup(func() {
		// Delete via gateway path
		cli.Run("gateway", "connection", "delete", conn.ID, "--force")
	})

	// Get via gateway path
	var fetched Connection
	err = cli.RunJSON(&fetched, "gateway", "connection", "get", conn.ID)
	require.NoError(t, err, "Should get connection via gateway path")
	assert.Equal(t, conn.ID, fetched.ID, "Connection ID should match")
	assert.Equal(t, connName, fetched.Name, "Connection name should match")

	t.Logf("Successfully created and retrieved connection via gateway path: %s", conn.ID)
}

// TestRootConnectionAliasWorks verifies that the backward-compatible root-level
// hookdeck connection ... still works after adding the gateway namespace
func TestRootConnectionAliasWorks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-alias-conn-" + timestamp
	sourceName := "test-alias-src-" + timestamp
	destName := "test-alias-dst-" + timestamp

	// Create via root alias path (hookdeck connection create)
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
	require.NoError(t, err, "Should create connection via root alias")
	require.NotEmpty(t, conn.ID, "Connection should have an ID")

	t.Cleanup(func() {
		cli.Run("connection", "delete", conn.ID, "--force")
	})

	// Get via gateway path (cross-path access)
	var fetched Connection
	err = cli.RunJSON(&fetched, "gateway", "connection", "get", conn.ID)
	require.NoError(t, err, "Should get connection via gateway path after creating via alias")
	assert.Equal(t, conn.ID, fetched.ID, "Connection ID should match across paths")

	// List via root alias, verify JSON output
	stdout, _, err := cli.Run("connection", "list", "--output", "json")
	require.NoError(t, err, "Should list via root alias")

	var conns []Connection
	err = json.Unmarshal([]byte(stdout), &conns)
	require.NoError(t, err, "Should parse JSON list from root alias")

	found := false
	for _, c := range conns {
		if c.ID == conn.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "Connection created via alias should appear in list")

	t.Logf("Root connection alias verified: %s", conn.ID)
}

// TestGatewayConnectionUpsert verifies upsert create and update via gateway path
func TestGatewayConnectionUpsert(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-gw-upsert-" + timestamp
	sourceName := "test-gw-upsert-src-" + timestamp
	destName := "test-gw-upsert-dst-" + timestamp

	// Upsert create via gateway path
	var conn Connection
	err := cli.RunJSON(&conn,
		"gateway", "connection", "upsert", connName,
		"--source-name", sourceName,
		"--source-type", "WEBHOOK",
		"--destination-name", destName,
		"--destination-type", "CLI",
		"--destination-cli-path", "/webhooks",
	)
	require.NoError(t, err, "Should upsert create via gateway path")
	require.NotEmpty(t, conn.ID, "Connection should have an ID")

	t.Cleanup(func() {
		cli.Run("gateway", "connection", "delete", conn.ID, "--force")
	})

	// Upsert update (same name) via gateway path
	newDesc := "Updated via gateway upsert"
	var updated Connection
	err = cli.RunJSON(&updated,
		"gateway", "connection", "upsert", connName,
		"--description", newDesc,
	)
	require.NoError(t, err, "Should upsert update via gateway path")
	assert.Equal(t, conn.ID, updated.ID, "Connection ID should be unchanged")
	assert.Equal(t, newDesc, updated.Description, "Description should be updated")

	t.Logf("Gateway connection upsert verified: %s", conn.ID)
}

// TestGatewayConnectionDelete verifies delete via gateway path
func TestGatewayConnectionDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	connID := createTestConnection(t, cli)
	require.NotEmpty(t, connID, "Connection ID should not be empty")

	// Delete via gateway path
	stdout := cli.RunExpectSuccess("gateway", "connection", "delete", connID, "--force")
	assert.NotEmpty(t, stdout, "delete should produce output")

	// Verify connection is gone
	_, _, err := cli.Run("gateway", "connection", "get", connID, "--output", "json")
	require.Error(t, err, "get should fail after delete")

	t.Logf("Gateway connection delete verified: %s", connID)
}

// TestGatewayConnectionEnableDisable verifies disable and enable via gateway path
func TestGatewayConnectionEnableDisable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	connID := createTestConnection(t, cli)
	require.NotEmpty(t, connID, "Connection ID should not be empty")

	t.Cleanup(func() {
		deleteConnection(t, cli, connID)
	})

	// Disable via gateway path
	cli.RunExpectSuccess("gateway", "connection", "disable", connID)

	// Enable via gateway path
	cli.RunExpectSuccess("gateway", "connection", "enable", connID)

	t.Logf("Gateway connection enable/disable verified: %s", connID)
}

// TestGatewayConnectionGetByName verifies get by name via gateway path
func TestGatewayConnectionGetByName(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	connName := "test-gw-getbyname-" + timestamp
	sourceName := "test-gw-getbyname-src-" + timestamp
	destName := "test-gw-getbyname-dst-" + timestamp

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
	t.Cleanup(func() { cli.Run("gateway", "connection", "delete", conn.ID, "--force") })

	// Get by name via gateway path
	var byName Connection
	err = cli.RunJSON(&byName, "gateway", "connection", "get", connName)
	require.NoError(t, err, "Should get connection by name")
	assert.Equal(t, conn.ID, byName.ID, "Connection ID should match when getting by name")

	t.Logf("Gateway connection get by name verified: %s", connName)
}

// TestRootConnectionsAliasWorks verifies the plural alias "connections" works
func TestRootConnectionsAliasWorks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	// "hookdeck connections list" should be rewritten to gateway connection list
	stdout := cli.RunExpectSuccess("connections", "list")
	assert.NotEmpty(t, stdout, "connections list should produce output")

	t.Logf("Root 'connections' alias verified")
}

// TestGatewaySourcesAliasWorks verifies the plural alias "sources" works under gateway
func TestGatewaySourcesAliasWorks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	// "hookdeck gateway sources list" should behave like "gateway source list"
	stdout := cli.RunExpectSuccess("gateway", "sources", "list")
	assert.NotEmpty(t, stdout, "gateway sources list should produce output")

	// Help should show source/sources
	helpOut := cli.RunExpectSuccess("gateway", "sources", "--help")
	assert.Contains(t, helpOut, "source", "gateway sources --help should describe source commands")

	t.Logf("Gateway 'sources' alias verified")
}
