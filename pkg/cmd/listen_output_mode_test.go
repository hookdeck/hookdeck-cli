package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestResolveOutputMode covers #333: `hookdeck listen` defaults to the interactive
// renderer, which opens /dev/tty. Without a terminal it fails at startup and the
// command forwards nothing, so the mode must be downgraded before the renderer is
// built.
func TestResolveOutputMode(t *testing.T) {
	tests := []struct {
		name        string
		requested   string
		explicit    bool
		isTerminal  bool
		wantMode    string
		wantWarning bool
	}{
		{
			name:       "interactive on a terminal is left alone",
			requested:  "interactive",
			explicit:   false,
			isTerminal: true,
			wantMode:   "interactive",
		},
		{
			name:       "defaulted interactive without a terminal falls back quietly",
			requested:  "interactive",
			explicit:   false,
			isTerminal: false,
			wantMode:   "compact",
			// No warning: the user never asked for interactive, so telling them
			// about a flag they did not set is noise in every CI log.
			wantWarning: false,
		},
		{
			name:        "explicit interactive without a terminal falls back and warns",
			requested:   "interactive",
			explicit:    true,
			isTerminal:  false,
			wantMode:    "compact",
			wantWarning: true,
		},
		{
			name:       "explicit interactive on a terminal is honoured",
			requested:  "interactive",
			explicit:   true,
			isTerminal: true,
			wantMode:   "interactive",
		},
		{
			name:       "compact is never rewritten",
			requested:  "compact",
			explicit:   true,
			isTerminal: false,
			wantMode:   "compact",
		},
		{
			name:       "quiet is never rewritten",
			requested:  "quiet",
			explicit:   true,
			isTerminal: false,
			wantMode:   "quiet",
		},
		{
			name:       "quiet is preserved on a terminal too",
			requested:  "quiet",
			explicit:   true,
			isTerminal: true,
			wantMode:   "quiet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, warning := resolveOutputMode(tt.requested, tt.explicit, tt.isTerminal)

			assert.Equal(t, tt.wantMode, mode)
			if tt.wantWarning {
				assert.NotEmpty(t, warning, "expected a warning explaining the fallback")
				assert.Contains(t, warning, "compact", "warning should name the mode actually used")
			} else {
				assert.Empty(t, warning)
			}
		})
	}
}

// TestStdoutIsTerminalIsInjectable guards the seam the command path relies on:
// runListenCmd calls stdoutIsTerminal(), and tests (and the acceptance suite,
// which runs the CLI with piped stdout) depend on it reflecting reality.
func TestStdoutIsTerminalIsInjectable(t *testing.T) {
	original := stdoutIsTerminal
	t.Cleanup(func() { stdoutIsTerminal = original })

	stdoutIsTerminal = func() bool { return false }
	mode, _ := resolveOutputMode("interactive", false, stdoutIsTerminal())
	assert.Equal(t, "compact", mode)

	stdoutIsTerminal = func() bool { return true }
	mode, _ = resolveOutputMode("interactive", false, stdoutIsTerminal())
	assert.Equal(t, "interactive", mode)
}
