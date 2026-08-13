package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestResolveAuthFallback covers the fix for the auto-login hang. When a command
// fails for want of credentials, Execute() used to always drop into interactive
// browser sign-in: it blocks on Enter, opens a browser, then polls for ~4 minutes
// (maxAttemptsDefault 120 x 2s). In CI, Docker or an AI agent that is a hang, and
// the caller never learns which command would have fixed it.
func TestResolveAuthFallback(t *testing.T) {
	tests := []struct {
		name             string
		gatewayMCP       bool
		interactiveStdin bool
		want             authFallback
	}{
		{
			name:             "terminal attached drops into interactive login",
			gatewayMCP:       false,
			interactiveStdin: true,
			want:             authFallbackRunLogin,
		},
		{
			name:             "no terminal reports how to authenticate instead of hanging",
			gatewayMCP:       false,
			interactiveStdin: false,
			want:             authFallbackNonInteractive,
		},
		{
			name:             "gateway mcp always reports on stderr",
			gatewayMCP:       true,
			interactiveStdin: false,
			want:             authFallbackMCP,
		},
		{
			name:             "gateway mcp wins even with a terminal attached",
			gatewayMCP:       true,
			interactiveStdin: true,
			want:             authFallbackMCP,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, resolveAuthFallback(tt.gatewayMCP, tt.interactiveStdin))
		})
	}
}

// TestNonInteractiveAuthHelpNamesEveryPath guards the message content: it is the
// only thing a non-interactive caller sees, so it has to name each way to
// authenticate without a terminal.
func TestNonInteractiveAuthHelpNamesEveryPath(t *testing.T) {
	assert.Contains(t, nonInteractiveAuthHelp, "hookdeck ci --api-key")
	assert.Contains(t, nonInteractiveAuthHelp, "HOOKDECK_API_KEY")
	assert.Contains(t, nonInteractiveAuthHelp, "hookdeck login --cli-key")
}

// TestStdinIsTerminalIsInjectable guards the seam resolveAuthFallback depends on.
func TestStdinIsTerminalIsInjectable(t *testing.T) {
	original := stdinIsTerminal
	t.Cleanup(func() { stdinIsTerminal = original })

	stdinIsTerminal = func() bool { return false }
	assert.Equal(t, authFallbackNonInteractive, resolveAuthFallback(false, stdinIsTerminal()))

	stdinIsTerminal = func() bool { return true }
	assert.Equal(t, authFallbackRunLogin, resolveAuthFallback(false, stdinIsTerminal()))
}
