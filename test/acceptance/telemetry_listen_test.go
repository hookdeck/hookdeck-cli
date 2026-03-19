//go:build telemetry

package acceptance

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestTelemetryListenProxy runs "listen" with the CLI pointed at a recording proxy
// that forwards to the real API. It lets listen run for a few seconds (so it can
// perform initial API calls: get sources, get connections, create session), then
// stops it. Asserts that every API request in that run has the same invocation_id
// and command_path "hookdeck listen".
//
// Build tag telemetry: runs only in the acceptance-telemetry CI job (telemetry enabled),
// not in matrix slices where HOOKDECK_CLI_TELEMETRY_DISABLED=1.
func TestTelemetryListenProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()

	timestamp := generateTimestamp()
	sourceName := "test-telemetry-" + timestamp

	_, _, _ = cli.RunListenWithTimeout([]string{
		"--api-base", proxy.URL(),
		"listen", "9999", sourceName,
		"--output", "compact",
	}, 6*time.Second)
	// Process is killed after 6s; we don't assert on exit error

	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1,
		"expected at least one API request from listen (sources, connections, or cli-sessions)")
	for i, req := range recorded {
		t.Logf("listen API request %d: %s %s (telemetry: %s)", i+1, req.Method, req.Path, req.Telemetry)
	}
	AssertTelemetryConsistent(t, recorded, "hookdeck listen")
}
