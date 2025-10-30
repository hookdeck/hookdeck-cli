package acceptance

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCLIBasics tests fundamental CLI operations including version, help, authentication, and whoami
func TestCLIBasics(t *testing.T) {
	// Skip in short test mode (for fast unit test runs)
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	t.Run("Version", func(t *testing.T) {
		cli := NewCLIRunner(t)

		stdout, stderr, err := cli.Run("version")
		require.NoError(t, err, "version command should succeed")
		assert.Empty(t, stderr, "version command should not produce stderr output")
		assert.NotEmpty(t, stdout, "version command should produce output")

		// Version output should contain some recognizable pattern
		// This is a basic sanity check
		t.Logf("Version output: %s", strings.TrimSpace(stdout))
	})

	t.Run("Help", func(t *testing.T) {
		cli := NewCLIRunner(t)

		stdout, _, err := cli.Run("help")
		require.NoError(t, err, "help command should succeed")
		assert.NotEmpty(t, stdout, "help command should produce output")

		// Help should mention some key commands
		assertContains(t, stdout, "Available Commands", "help output should show available commands")
		t.Logf("Help output contains %d bytes", len(stdout))
	})

	t.Run("Authentication", func(t *testing.T) {
		// NewCLIRunner already authenticates, so if we get here, auth worked
		cli := NewCLIRunner(t)

		// Verify authentication by running whoami
		stdout := cli.RunExpectSuccess("whoami")
		assert.NotEmpty(t, stdout, "whoami should produce output")

		// Whoami output should contain user information
		// The exact format may vary, but it should have some content
		t.Logf("Whoami output: %s", strings.TrimSpace(stdout))
	})

	t.Run("WhoamiAfterAuth", func(t *testing.T) {
		cli := NewCLIRunner(t)

		stdout := cli.RunExpectSuccess("whoami")
		require.NotEmpty(t, stdout, "whoami should return user information")

		// The output should contain organization or workspace information
		// This is a basic validation that the API key is working
		t.Logf("Authenticated user info: %s", strings.TrimSpace(stdout))
	})
}
