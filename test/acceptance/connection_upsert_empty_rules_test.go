//go:build connection_upsert

package acceptance

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConnectionUpsertEmptyRulesClearsRules covers #472: --rules '[]' has to
// reach the API as an empty array, as it does for `connection update`.
func TestConnectionUpsertEmptyRulesClearsRules(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	connID := createTestConnection(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })

	var conn Connection
	require.NoError(t, cli.RunJSON(&conn, "gateway", "connection", "get", connID))

	rulesJSON := `[{"type":"retry","strategy":"linear","count":2,"interval":60000}]`
	require.NoError(t, cli.RunJSON(&Connection{}, "gateway", "connection", "upsert", conn.Name, "--rules", rulesJSON))

	var withRules Connection
	require.NoError(t, cli.RunJSON(&withRules, "gateway", "connection", "get", connID))
	require.Len(t, withRules.Rules, 1, "setup: the connection must have a rule to clear")

	t.Run("dry-run says the rules will be cleared and changes nothing", func(t *testing.T) {
		stdout := cli.RunExpectSuccess("gateway", "connection", "upsert", conn.Name, "--rules", "[]", "--dry-run")
		assert.Contains(t, stdout, "will be cleared")

		var after Connection
		require.NoError(t, cli.RunJSON(&after, "gateway", "connection", "get", connID))
		assert.Len(t, after.Rules, 1, "a dry run must not clear the rules")
	})

	t.Run("an upsert without a rules flag leaves the rules alone", func(t *testing.T) {
		// Negative control: only an explicit empty array clears.
		require.NoError(t, cli.RunJSON(&Connection{}, "gateway", "connection", "upsert", conn.Name, "--description", "no rules flag"))

		var after Connection
		require.NoError(t, cli.RunJSON(&after, "gateway", "connection", "get", connID))
		assert.Len(t, after.Rules, 1, "omitting --rules must keep the stored rules")
	})

	t.Run("--rules '[]' clears them", func(t *testing.T) {
		require.NoError(t, cli.RunJSON(&Connection{}, "gateway", "connection", "upsert", conn.Name, "--rules", "[]"))

		var after Connection
		require.NoError(t, cli.RunJSON(&after, "gateway", "connection", "get", connID))
		assert.Empty(t, after.Rules, "the empty array must clear the ruleset")
	})

	t.Run("--rules-file with an empty array clears them", func(t *testing.T) {
		require.NoError(t, cli.RunJSON(&Connection{}, "gateway", "connection", "upsert", conn.Name, "--rules", rulesJSON))
		var restored Connection
		require.NoError(t, cli.RunJSON(&restored, "gateway", "connection", "get", connID))
		require.Len(t, restored.Rules, 1, "setup: the rule must be back")

		path := t.TempDir() + "/rules.json"
		require.NoError(t, os.WriteFile(path, []byte("[]"), 0644))
		require.NoError(t, cli.RunJSON(&Connection{}, "gateway", "connection", "upsert", conn.Name, "--rules-file", path))

		var after Connection
		require.NoError(t, cli.RunJSON(&after, "gateway", "connection", "get", connID))
		assert.Empty(t, after.Rules, fmt.Sprintf("--rules-file with [] must clear the ruleset: %+v", after.Rules))
	})
}
