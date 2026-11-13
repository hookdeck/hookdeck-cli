//go:build connection_upsert

package acceptance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// listDestinationsByName returns the destinations the API lists for an exact
// name filter.
func listDestinationsByName(t *testing.T, cli *CLIRunner, name string) []Destination {
	t.Helper()
	var resp struct {
		Models []Destination `json:"models"`
	}
	require.NoError(t, cli.RunJSON(&resp, "gateway", "destination", "list", "--name", name))
	return resp.Models
}

// TestConnectionUpsertRejectsChangedDestination covers #466: the API never
// changes an existing connection's destination, so upsert has to fail rather
// than report success, and it has to fail before the inline destination is
// upserted, or that destination is left behind attached to nothing.
func TestConnectionUpsertRejectsChangedDestination(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	connID := createTestConnection(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })

	var conn Connection
	require.NoError(t, cli.RunJSON(&conn, "gateway", "connection", "get", connID))

	t.Run("inline destination with a different name creates no orphan", func(t *testing.T) {
		orphanName := "test-orphan-dst-" + generateTimestamp()

		stdout, stderr, err := cli.Run("gateway", "connection", "upsert", conn.Name,
			"--source-name", conn.Source.Name, "--source-type", "WEBHOOK",
			"--destination-name", orphanName, "--destination-type", "CLI",
			"--destination-cli-path", "/elsewhere")
		require.Error(t, err, "changing the destination must fail\nstdout: %s", stdout)
		assert.Contains(t, stderr+stdout, "cannot be changed")

		// The destination was not created ...
		assert.Empty(t, listDestinationsByName(t, cli, orphanName), "no orphan destination may be created")
		// ... and the filter does find a destination that exists, so the empty
		// result above is not the filter matching nothing.
		assert.Len(t, listDestinationsByName(t, cli, conn.Destination.Name), 1,
			"negative control: the stored destination must be listed by the same filter")

		var after Connection
		require.NoError(t, cli.RunJSON(&after, "gateway", "connection", "get", connID))
		assert.Equal(t, conn.Destination.ID, after.Destination.ID, "destination must be unchanged")
	})

	t.Run("a different destination ID is rejected", func(t *testing.T) {
		otherID := createTestDestination(t, cli)
		t.Cleanup(func() { deleteDestination(t, cli, otherID) })

		stdout, stderr, err := cli.Run("gateway", "connection", "upsert", conn.Name,
			"--source-id", conn.Source.ID, "--destination-id", otherID)
		require.Error(t, err, "changing the destination must fail\nstdout: %s", stdout)
		assert.Contains(t, stderr+stdout, "cannot be changed")

		var after Connection
		require.NoError(t, cli.RunJSON(&after, "gateway", "connection", "get", connID))
		assert.Equal(t, conn.Destination.ID, after.Destination.ID, "destination must be unchanged")
	})

	t.Run("a different destination ID alone is rejected", func(t *testing.T) {
		otherID := createTestDestination(t, cli)
		t.Cleanup(func() { deleteDestination(t, cli, otherID) })

		// Before the fix this reached the API without a source and came back as a
		// 422 about source_id.
		stdout, stderr, err := cli.Run("gateway", "connection", "upsert", conn.Name, "--destination-id", otherID)
		require.Error(t, err, "stdout: %s", stdout)
		assert.Contains(t, stderr+stdout, "cannot be changed")
	})

	t.Run("dry-run reports the problem too", func(t *testing.T) {
		otherID := createTestDestination(t, cli)
		t.Cleanup(func() { deleteDestination(t, cli, otherID) })

		stdout, stderr, err := cli.Run("gateway", "connection", "upsert", conn.Name,
			"--source-id", conn.Source.ID, "--destination-id", otherID, "--dry-run")
		require.Error(t, err, "stdout: %s", stdout)
		assert.Contains(t, stderr+stdout, "cannot be changed")
	})

	t.Run("the stored destination is still accepted", func(t *testing.T) {
		// Negative control: the guard rejects a difference, not every upsert that
		// names a destination.
		var upserted Connection
		require.NoError(t, cli.RunJSON(&upserted, "gateway", "connection", "upsert", conn.Name,
			"--source-id", conn.Source.ID, "--destination-id", conn.Destination.ID,
			"--description", "same destination"))
		assert.Equal(t, conn.Destination.ID, upserted.Destination.ID)

		var byName Connection
		require.NoError(t, cli.RunJSON(&byName, "gateway", "connection", "upsert", conn.Name,
			"--source-name", conn.Source.Name, "--source-type", "WEBHOOK",
			"--destination-name", conn.Destination.Name, "--destination-type", "CLI",
			"--destination-cli-path", "/updated"))
		assert.Equal(t, conn.Destination.ID, byName.Destination.ID)
	})
}
