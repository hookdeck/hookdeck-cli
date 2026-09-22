package login

import (
	"os"
	"testing"

	configpkg "github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/stretchr/testify/require"
)

// TestInteractiveLogin_noTerminalExplainsItself covers #401: `hookdeck login -i`
// with stdin at /dev/null printed "Enter your CLI API key: " and then failed with
// "operation not supported by device" - the termios error from term.GetState,
// surfaced verbatim. It exited fast, so only the message was wrong.
func TestInteractiveLogin_noTerminalExplainsItself(t *testing.T) {
	oldStdinIsTerminal := stdinIsTerminal
	stdinIsTerminal = func() bool { return false }
	t.Cleanup(func() { stdinIsTerminal = oldStdinIsTerminal })

	stdoutFile, err := os.CreateTemp(t.TempDir(), "stdout")
	require.NoError(t, err)
	oldStdout := os.Stdout
	os.Stdout = stdoutFile
	t.Cleanup(func() { os.Stdout = oldStdout })

	cfg := &configpkg.Config{
		APIBaseURL:        "https://api.example.test",
		DeviceName:        "test-device",
		LogLevel:          "error",
		TelemetryDisabled: true,
	}
	cfg.Profile = configpkg.Profile{Name: "default", Config: cfg}

	err = InteractiveLogin(cfg)

	os.Stdout = oldStdout
	require.NoError(t, stdoutFile.Close())
	out, readErr := os.ReadFile(stdoutFile.Name())
	require.NoError(t, readErr)

	require.ErrorIs(t, err, ErrInteractiveLoginNoTerminal)
	require.NotContains(t, err.Error(), "not supported by device",
		"the raw termios error tells the user nothing")
	require.Contains(t, err.Error(), "--cli-key")
	require.Contains(t, err.Error(), "hookdeck ci --api-key")
	require.NotContains(t, string(out), "Enter your CLI API key",
		"do not prompt for something that cannot be typed")
}
