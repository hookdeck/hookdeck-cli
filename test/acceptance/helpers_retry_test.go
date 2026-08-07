//go:build telemetry

package acceptance

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRunWithHTTP502RetryResetsRecordingProxy is a deterministic regression test
// for the acceptance-telemetry flake: when a CLI command hit a transient 502 and
// was retried, the recording proxy kept the failed attempt's requests (with one
// invocation_id) alongside the successful retry's (with a different invocation_id),
// so AssertTelemetryConsistent failed. The retry now resets the proxy between
// attempts, so only the final attempt's requests remain.
//
// It uses a fake upstream and a simulated CLI (no live API), so it never flakes.
func TestRunWithHTTP502RetryResetsRecordingProxy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	proxy := StartRecordingProxy(t, upstream.URL)
	defer proxy.Close()

	args := []string{"--api-base", proxy.URL(), "gateway", "connection", "list"}
	r := &CLIRunner{t: t}

	// Simulate the CLI: each attempt is a fresh process with its own invocation_id.
	// Attempt 1 makes a request then reports a transient 502; attempt 2 succeeds.
	attempt := 0
	run := func() (string, string, error) {
		attempt++
		makeProxiedTelemetryRequest(t, proxy.URL(), fmt.Sprintf("inv-%d", attempt), "hookdeck gateway connection list")
		if attempt == 1 {
			return "", "error code: 502", errors.New("exit status 1")
		}
		return "ok", "", nil
	}

	_, _, err := r.runWithHTTP502Retry("gateway connection list", args, run)
	require.NoError(t, err)
	require.Equal(t, 2, attempt, "should have retried exactly once")

	recorded := proxy.Recorded()
	require.Len(t, recorded, 1, "only the successful retry's request should remain after the reset (got %d)", len(recorded))
	// The remaining requests must be internally consistent — this is what the flake broke.
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway connection list")
}

// makeProxiedTelemetryRequest issues one request through the proxy carrying an
// X-Hookdeck-CLI-Telemetry header with the given invocation_id and command_path.
func makeProxiedTelemetryRequest(t *testing.T, proxyURL, invocationID, commandPath string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, proxyURL+"/2025-07-01/connections", nil)
	require.NoError(t, err)
	req.Header.Set("X-Hookdeck-CLI-Telemetry",
		fmt.Sprintf(`{"command_path":%q,"invocation_id":%q}`, commandPath, invocationID))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
}
