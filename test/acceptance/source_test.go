package acceptance

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSourceList(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess("gateway", "source", "list")
	assert.NotEmpty(t, stdout)
}

func TestSourceCreateAndDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	sourceID := createTestSource(t, cli)
	t.Cleanup(func() { deleteSource(t, cli, sourceID) })

	stdout := cli.RunExpectSuccess("gateway", "source", "get", sourceID)
	assert.Contains(t, stdout, sourceID)
	assert.Contains(t, stdout, "WEBHOOK")
}

func TestSourceGetByName(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()
	name := "test-src-get-" + timestamp

	var src Source
	err := cli.RunJSON(&src, "gateway", "source", "create", "--name", name, "--type", "WEBHOOK")
	require.NoError(t, err)
	t.Cleanup(func() { deleteSource(t, cli, src.ID) })

	stdout := cli.RunExpectSuccess("gateway", "source", "get", name)
	assert.Contains(t, stdout, src.ID)
	assert.Contains(t, stdout, name)
}

func TestSourceCreateWithDescription(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()
	name := "test-src-desc-" + timestamp
	desc := "Test source description"

	var src Source
	err := cli.RunJSON(&src, "gateway", "source", "create", "--name", name, "--type", "WEBHOOK", "--description", desc)
	require.NoError(t, err)
	t.Cleanup(func() { deleteSource(t, cli, src.ID) })

	stdout := cli.RunExpectSuccess("gateway", "source", "get", src.ID)
	assert.Contains(t, stdout, desc)
}

func TestSourceUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	sourceID := createTestSource(t, cli)
	t.Cleanup(func() { deleteSource(t, cli, sourceID) })

	newName := "test-src-updated-" + generateTimestamp()
	cli.RunExpectSuccess("gateway", "source", "update", sourceID, "--name", newName)

	stdout := cli.RunExpectSuccess("gateway", "source", "get", sourceID)
	assert.Contains(t, stdout, newName)
}

func TestSourceUpsertCreate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	name := "test-src-upsert-create-" + generateTimestamp()

	var src Source
	err := cli.RunJSON(&src, "gateway", "source", "upsert", name, "--type", "WEBHOOK")
	require.NoError(t, err)
	require.NotEmpty(t, src.ID)
	assert.Equal(t, name, src.Name)
	t.Cleanup(func() { deleteSource(t, cli, src.ID) })
}

func TestSourceUpsertUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	name := "test-src-upsert-upd-" + generateTimestamp()

	var src Source
	err := cli.RunJSON(&src, "gateway", "source", "upsert", name, "--type", "WEBHOOK")
	require.NoError(t, err)
	t.Cleanup(func() { deleteSource(t, cli, src.ID) })

	newDesc := "Updated via upsert"
	err = cli.RunJSON(&src, "gateway", "source", "upsert", name, "--description", newDesc)
	require.NoError(t, err)

	stdout := cli.RunExpectSuccess("gateway", "source", "get", src.ID)
	assert.Contains(t, stdout, newDesc)
}

func TestSourceEnableDisable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	sourceID := createTestSource(t, cli)
	t.Cleanup(func() { deleteSource(t, cli, sourceID) })

	cli.RunExpectSuccess("gateway", "source", "disable", sourceID)
	stdout := cli.RunExpectSuccess("gateway", "source", "get", sourceID)
	assert.Contains(t, stdout, "disabled")

	cli.RunExpectSuccess("gateway", "source", "enable", sourceID)
	stdout = cli.RunExpectSuccess("gateway", "source", "get", sourceID)
	assert.Contains(t, stdout, "active")
}

func TestSourceCount(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess("gateway", "source", "count")
	stdout = strings.TrimSpace(stdout)
	assert.NotEmpty(t, stdout)
	assert.Regexp(t, `^\d+$`, stdout)
}

func TestSourceListFilterByName(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	sourceID := createTestSource(t, cli)
	t.Cleanup(func() { deleteSource(t, cli, sourceID) })

	// Get name from get output or create with known name
	var src Source
	err := cli.RunJSON(&src, "gateway", "source", "get", sourceID)
	require.NoError(t, err)

	stdout := cli.RunExpectSuccess("gateway", "source", "list", "--name", src.Name)
	assert.Contains(t, stdout, src.ID)
	assert.Contains(t, stdout, src.Name)
}

func TestSourceListFilterByType(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess("gateway", "source", "list", "--type", "WEBHOOK", "--limit", "5")
	// May be empty or have entries
	assert.NotContains(t, stdout, "failed")
}

func TestSourceDeleteForce(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	sourceID := createTestSource(t, cli)

	cli.RunExpectSuccess("gateway", "source", "delete", sourceID, "--force")

	_, _, err := cli.Run("gateway", "source", "get", sourceID)
	require.Error(t, err)
}

func TestSourceUpsertDryRun(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	name := "test-src-dryrun-" + generateTimestamp()
	stdout := cli.RunExpectSuccess("gateway", "source", "upsert", name, "--type", "WEBHOOK", "--dry-run")
	assert.Contains(t, stdout, "Dry Run")
	assert.Contains(t, stdout, "CREATE")
}

func TestSourceGetOutputJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	sourceID := createTestSource(t, cli)
	t.Cleanup(func() { deleteSource(t, cli, sourceID) })

	var src Source
	err := cli.RunJSON(&src, "gateway", "source", "get", sourceID, "--output", "json")
	require.NoError(t, err)
	assert.Equal(t, sourceID, src.ID)
	assert.Equal(t, "WEBHOOK", src.Type)
}

// TestStandaloneSourceThenConnection creates a standalone source via `source create`,
// then creates a connection that uses that source via --source-id.
func TestStandaloneSourceThenConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	// Create standalone source first
	sourceName := "test-standalone-src-" + timestamp
	var src Source
	err := cli.RunJSON(&src, "gateway", "source", "create", "--name", sourceName, "--type", "WEBHOOK")
	require.NoError(t, err, "Failed to create standalone source")
	require.NotEmpty(t, src.ID, "Source ID should not be empty")

	t.Cleanup(func() { deleteSource(t, cli, src.ID) })

	// Create connection using the standalone source
	connName := "test-conn-standalone-src-" + timestamp
	destName := "test-dst-standalone-" + timestamp
	var conn Connection
	err = cli.RunJSON(&conn,
		"gateway", "connection", "create",
		"--name", connName,
		"--source-id", src.ID,
		"--destination-name", destName,
		"--destination-type", "CLI",
		"--destination-cli-path", "/webhooks",
	)
	require.NoError(t, err, "Failed to create connection with standalone source")
	require.NotEmpty(t, conn.ID, "Connection ID should not be empty")

	t.Cleanup(func() { deleteConnection(t, cli, conn.ID) })

	// Connection should use the same standalone source
	assert.Equal(t, src.ID, conn.Source.ID, "Connection should use the standalone source ID")
	assert.Equal(t, sourceName, conn.Source.Name, "Connection should use the standalone source name")
	assert.Equal(t, "WEBHOOK", conn.Source.Type)
	assert.Equal(t, destName, conn.Destination.Name)
}
