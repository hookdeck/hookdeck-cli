//go:build basic

package acceptance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

	t.Run("WhoamiShowsProjectType", func(t *testing.T) {
		cli := NewCLIRunner(t)
		stdout := cli.RunExpectSuccess("whoami")
		// Output should include project type (Gateway, Outpost, or Console)
		assert.Contains(t, stdout, "Project type:", "whoami should show project type line")
		assert.True(t,
			strings.Contains(stdout, "Gateway") || strings.Contains(stdout, "Outpost") || strings.Contains(stdout, "Console"),
			"whoami should show one of Gateway, Outpost, Console")
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

// TestUnauthenticatedCommandFailsFastWithoutTerminal is the regression test for
// #337. Any command failing for want of credentials used to drop into
// interactive sign-in: it blocked on a prompt, opened a browser, then polled for
// ~4 minutes (maxAttemptsDefault 120 x 2s) before failing. In CI, Docker or an
// AI agent that is a hang, and the caller never learns which command fixes it.
//
// The timing assertion is the point of this test. Asserting only on the error
// message would still pass if the command took four minutes to produce it.
func TestUnauthenticatedCommandFailsFastWithoutTerminal(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	// A config file with no credentials, so the command fails for want of auth.
	emptyConfig := filepath.Join(t.TempDir(), "no-credentials.toml")
	require.NoError(t, os.WriteFile(emptyConfig, []byte(""), 0600))

	start := time.Now()
	stdout, stderr, err := cli.RunFromCwdWithEnv(
		map[string]string{"HOOKDECK_CONFIG_FILE": emptyConfig},
		"gateway", "source", "list",
	)
	elapsed := time.Since(start)

	combined := stdout + stderr
	t.Logf("elapsed=%s output=%s", elapsed, combined)

	require.Error(t, err, "an unauthenticated command must fail, not wait for a browser sign-in")

	// The old path polled for ~240s. Allow generous headroom for the go build
	// inside RunFromCwdWithEnv while still catching a reintroduced poll loop.
	assert.Less(t, elapsed, 90*time.Second,
		"unauthenticated command should fail immediately, not poll for a browser login (#337)")

	assert.Contains(t, combined, "No terminal is attached",
		"the error should explain why browser sign-in was not attempted")
	assert.Contains(t, combined, "HOOKDECK_API_KEY",
		"the error should name a way to authenticate without a terminal")
	assert.Contains(t, combined, "--cli-key",
		"the error should name the CLI key option too")

	assert.NotContains(t, combined, "Press Enter",
		"it must not prompt when there is no terminal")
	assert.NotContains(t, combined, "Waiting for confirmation",
		"it must not start the browser sign-in poll")
}
