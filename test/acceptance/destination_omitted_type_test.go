//go:build destination

package acceptance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// configString reads a string field from a destination's decoded config.
func configString(t *testing.T, dst Destination, key string) string {
	t.Helper()
	cfg, ok := dst.Config.(map[string]interface{})
	require.True(t, ok, "destination config should be an object, got %T", dst.Config)
	v, _ := cfg[key].(string)
	return v
}

// TestDestinationUpdateAppliesTypeSpecificFlagsWithoutType covers #406: --type
// is normally omitted on update because the type is already stored, and
// --url / --cli-path used to be dropped from the request while the command
// exited 0. Every case reads the destination back, because the exit code and
// the success message were the part that was already "right".
func TestDestinationUpdateAppliesTypeSpecificFlagsWithoutType(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	t.Run("update --url on an HTTP destination", func(t *testing.T) {
		name := "test-dst-406-http-" + generateTimestamp()
		var dst Destination
		require.NoError(t, cli.RunJSON(&dst, "gateway", "destination", "create",
			"--name", name, "--type", "HTTP", "--url", "https://example.com/old"))
		t.Cleanup(func() { deleteDestination(t, cli, dst.ID) })

		// Negative control: a change with no --url must leave the URL alone, so a
		// changed URL below can only come from --url.
		cli.RunExpectSuccess("gateway", "destination", "update", dst.ID, "--description", "no url flag")
		var untouched Destination
		require.NoError(t, cli.RunJSON(&untouched, "gateway", "destination", "get", dst.ID))
		require.Equal(t, "https://example.com/old", configString(t, untouched, "url"))

		cli.RunExpectSuccess("gateway", "destination", "update", dst.ID, "--url", "https://example.com/new")

		var after Destination
		require.NoError(t, cli.RunJSON(&after, "gateway", "destination", "get", dst.ID))
		assert.Equal(t, "https://example.com/new", configString(t, after, "url"), "--url without --type must change the URL")
		assert.Equal(t, "HTTP", after.Type, "the type must not be changed by resolving it")
	})

	t.Run("update --cli-path on a CLI destination", func(t *testing.T) {
		name := "test-dst-406-cli-" + generateTimestamp()
		var dst Destination
		require.NoError(t, cli.RunJSON(&dst, "gateway", "destination", "create",
			"--name", name, "--type", "CLI", "--cli-path", "/old"))
		t.Cleanup(func() { deleteDestination(t, cli, dst.ID) })

		cli.RunExpectSuccess("gateway", "destination", "update", dst.ID, "--cli-path", "/new")

		var after Destination
		require.NoError(t, cli.RunJSON(&after, "gateway", "destination", "get", dst.ID))
		assert.Equal(t, "/new", configString(t, after, "path"), "--cli-path without --type must change the path")
	})

	t.Run("update --url on a CLI destination is refused, not dropped", func(t *testing.T) {
		name := "test-dst-406-mismatch-" + generateTimestamp()
		var dst Destination
		require.NoError(t, cli.RunJSON(&dst, "gateway", "destination", "create",
			"--name", name, "--type", "CLI", "--cli-path", "/keep"))
		t.Cleanup(func() { deleteDestination(t, cli, dst.ID) })

		stdout, stderr, err := cli.Run("gateway", "destination", "update", dst.ID, "--url", "https://example.com/x")
		require.Error(t, err, "a flag the stored type has no field for must not be silently ignored\nstdout: %s", stdout)
		assert.Contains(t, stderr+stdout, "--url")
	})

	t.Run("upsert --url on an existing HTTP destination", func(t *testing.T) {
		name := "test-dst-406-upsert-" + generateTimestamp()
		var dst Destination
		require.NoError(t, cli.RunJSON(&dst, "gateway", "destination", "create",
			"--name", name, "--type", "HTTP", "--url", "https://example.com/old"))
		t.Cleanup(func() { deleteDestination(t, cli, dst.ID) })

		cli.RunExpectSuccess("gateway", "destination", "upsert", name, "--url", "https://example.com/new")

		var after Destination
		require.NoError(t, cli.RunJSON(&after, "gateway", "destination", "get", dst.ID))
		assert.Equal(t, "https://example.com/new", configString(t, after, "url"), "upsert --url without --type must change the URL")
	})
}
