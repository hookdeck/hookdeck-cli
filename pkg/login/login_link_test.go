package login

import (
	"io"
	"os"
	"testing"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/stretchr/testify/require"
)

func capturePrintLoginLink(t *testing.T, session *hookdeck.LoginSession) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	oldStdout := os.Stdout
	os.Stdout = w
	printLoginLink(session)
	os.Stdout = oldStdout
	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(out)
}

func TestPrintLoginLink_printsTheCodeThePageAsksFor(t *testing.T) {
	out := capturePrintLoginLink(t, &hookdeck.LoginSession{
		BrowserURL: "https://dashboard.example.test/cli-auth/k",
		UserCode:   "WDJB-MJHT",
	})
	require.Equal(t,
		"To authenticate with Hookdeck, please go to: https://dashboard.example.test/cli-auth/k\n"+
			"When the page asks for a code, enter: WDJB-MJHT\n",
		out)
}

func TestPrintLoginLink_onlyTheURLWithoutACode(t *testing.T) {
	out := capturePrintLoginLink(t, &hookdeck.LoginSession{
		BrowserURL: "https://dashboard.example.test/cli-auth/k",
	})
	require.Equal(t, "To authenticate with Hookdeck, please go to: https://dashboard.example.test/cli-auth/k\n", out)
}
