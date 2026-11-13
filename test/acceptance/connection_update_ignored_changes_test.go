//go:build connection_update

package acceptance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConnectionUpdateRejectsChangedSides covers #466: PUT /connections/{id}
// discards source_id and destination_id, so update used to print success and the
// old destination for a change that never happened.
func TestConnectionUpdateRejectsChangedSides(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	connID := createTestConnection(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })

	var conn Connection
	require.NoError(t, cli.RunJSON(&conn, "gateway", "connection", "get", connID))

	otherDestID := createTestDestination(t, cli)
	t.Cleanup(func() { deleteDestination(t, cli, otherDestID) })
	otherSourceID := createTestSource(t, cli)
	t.Cleanup(func() { deleteSource(t, cli, otherSourceID) })

	t.Run("a different destination ID is rejected", func(t *testing.T) {
		stdout, stderr, err := cli.Run("gateway", "connection", "update", connID, "--destination-id", otherDestID)
		require.Error(t, err, "stdout: %s", stdout)
		assert.Contains(t, stderr+stdout, "cannot be changed")

		var after Connection
		require.NoError(t, cli.RunJSON(&after, "gateway", "connection", "get", connID))
		assert.Equal(t, conn.Destination.ID, after.Destination.ID)
	})

	t.Run("a different source ID is rejected", func(t *testing.T) {
		stdout, stderr, err := cli.Run("gateway", "connection", "update", connID, "--source-id", otherSourceID)
		require.Error(t, err, "stdout: %s", stdout)
		assert.Contains(t, stderr+stdout, "cannot be changed")

		var after Connection
		require.NoError(t, cli.RunJSON(&after, "gateway", "connection", "get", connID))
		assert.Equal(t, conn.Source.ID, after.Source.ID)
	})

	t.Run("the stored IDs are accepted and other changes still apply", func(t *testing.T) {
		// Negative control: the guard rejects a difference, not the flags.
		var updated Connection
		require.NoError(t, cli.RunJSON(&updated, "gateway", "connection", "update", connID,
			"--destination-id", conn.Destination.ID, "--source-id", conn.Source.ID,
			"--description", "same sides"))
		assert.Equal(t, "same sides", updated.Description)
		assert.Equal(t, conn.Destination.ID, updated.Destination.ID)
	})
}
