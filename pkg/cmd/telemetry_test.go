package cmd

import (
	"testing"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestInitTelemetry(t *testing.T) {
	hookdeck.ResetTelemetryInstanceForTesting()

	cmd := &cobra.Command{
		Use: "hookdeck",
	}
	listCmd := &cobra.Command{
		Use: "list",
	}
	cmd.AddCommand(listCmd)

	Config.DeviceName = "test-machine"

	initTelemetry(listCmd)

	tel := hookdeck.GetTelemetryInstance()
	require.Equal(t, "cli", tel.Source)
	require.Equal(t, "hookdeck list", tel.CommandPath)
	require.Equal(t, "test-machine", tel.DeviceName)
	require.NotEmpty(t, tel.InvocationID)
	require.Contains(t, tel.InvocationID, "inv_")
	require.Contains(t, []string{"interactive", "ci"}, tel.Environment)
}

func TestInitTelemetryGeneratedResource(t *testing.T) {
	hookdeck.ResetTelemetryInstanceForTesting()

	cmd := &cobra.Command{
		Use: "source",
		Annotations: map[string]string{
			"generated": "operation",
		},
	}

	initTelemetry(cmd)

	tel := hookdeck.GetTelemetryInstance()
	require.True(t, tel.GeneratedResource)
}

func TestInitTelemetryNonGeneratedResource(t *testing.T) {
	hookdeck.ResetTelemetryInstanceForTesting()

	cmd := &cobra.Command{
		Use: "listen",
	}

	initTelemetry(cmd)

	tel := hookdeck.GetTelemetryInstance()
	require.False(t, tel.GeneratedResource)
}

func TestInitTelemetryResetBetweenCalls(t *testing.T) {
	// Simulate two sequential command invocations with singleton reset
	hookdeck.ResetTelemetryInstanceForTesting()

	cmd1 := &cobra.Command{Use: "listen"}
	Config.DeviceName = "device-1"
	initTelemetry(cmd1)

	tel1 := hookdeck.GetTelemetryInstance()
	id1 := tel1.InvocationID
	require.Equal(t, "listen", tel1.CommandPath)

	// Reset and reinitialize for a different command
	hookdeck.ResetTelemetryInstanceForTesting()

	cmd2 := &cobra.Command{Use: "whoami"}
	Config.DeviceName = "device-2"
	initTelemetry(cmd2)

	tel2 := hookdeck.GetTelemetryInstance()
	require.Equal(t, "whoami", tel2.CommandPath)
	require.Equal(t, "device-2", tel2.DeviceName)
	require.NotEqual(t, id1, tel2.InvocationID)
}

// TestInvocationIDPersistsAcrossMultipleInitTelemetryCalls reproduces the v2.0.0 bug: when
// initTelemetry is called multiple times in one process (e.g. root PreRun, then gateway PreRun,
// then connection PreRun), the invocation_id must stay the same so all API requests in that run
// share one ID. Without the fix, each initTelemetry call overwrites with a new ID; this test
// asserts the ID is unchanged after a second call. On v2.0.0 (repro branch) it fails.
func TestInvocationIDPersistsAcrossMultipleInitTelemetryCalls(t *testing.T) {
	hookdeck.ResetTelemetryInstanceForTesting()

	root := &cobra.Command{Use: "hookdeck"}
	gateway := &cobra.Command{Use: "gateway"}
	connection := &cobra.Command{Use: "connection"}
	root.AddCommand(gateway)
	gateway.AddCommand(connection)

	Config.DeviceName = "test-device"
	initTelemetry(root)
	tel := hookdeck.GetTelemetryInstance()
	firstID := tel.InvocationID
	require.NotEmpty(t, firstID, "first initTelemetry must set invocation_id")

	// Second call simulates gateway PreRun; must not overwrite invocation_id.
	initTelemetry(gateway)
	tel = hookdeck.GetTelemetryInstance()
	require.Equal(t, firstID, tel.InvocationID,
		"invocation_id must persist across initTelemetry calls in the same process (fix: set only when empty)")

	// Third call simulates connection PreRun; must still not overwrite.
	initTelemetry(connection)
	tel = hookdeck.GetTelemetryInstance()
	require.Equal(t, firstID, tel.InvocationID,
		"invocation_id must persist after third initTelemetry call")
}

// TestInitTelemetryWhenDisabled verifies that initTelemetry always populates the
// singleton (Source, CommandPath, etc.) even when telemetry is disabled. The
// call must happen for every command; PerformRequest later skips sending the
// header when Disabled is true.
func TestInitTelemetryWhenDisabled(t *testing.T) {
	hookdeck.ResetTelemetryInstanceForTesting()

	Config.TelemetryDisabled = true
	defer func() { Config.TelemetryDisabled = false }()

	cmd := &cobra.Command{Use: "whoami"}
	Config.DeviceName = "my-device"
	initTelemetry(cmd)

	tel := hookdeck.GetTelemetryInstance()
	require.True(t, tel.Disabled)
	require.Equal(t, "cli", tel.Source)
	require.Equal(t, "whoami", tel.CommandPath)
	require.Equal(t, "my-device", tel.DeviceName)
	require.NotEmpty(t, tel.InvocationID)
}

// TestAllCommandsWithPersistentPreRunInitTelemetry is a regression test: any
// command that defines its own PersistentPreRun or PersistentPreRunE (and thus
// replaces the root's hook under Cobra) must call initTelemetry(cmd) so that
// API requests get non-empty telemetry. Otherwise the header is sent with empty
// fields (e.g. gateway before the fix).
func TestAllCommandsWithPersistentPreRunInitTelemetry(t *testing.T) {
	root := RootCmd()
	var commandsWithPreRun []*cobra.Command
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if c.PersistentPreRun != nil || c.PersistentPreRunE != nil {
			commandsWithPreRun = append(commandsWithPreRun, c)
		}
		for _, child := range c.Commands() {
			walk(child)
		}
	}
	walk(root)

	require.NotEmpty(t, commandsWithPreRun, "expected at least root to have PersistentPreRun")

	for _, cmd := range commandsWithPreRun {
		hookdeck.ResetTelemetryInstanceForTesting()
		if cmd.PersistentPreRunE != nil {
			_ = cmd.PersistentPreRunE(cmd, nil)
		} else {
			cmd.PersistentPreRun(cmd, nil)
		}
		tel := hookdeck.GetTelemetryInstance()
		require.NotEmpty(t, tel.CommandPath,
			"command %q has PersistentPreRun(E) but did not set telemetry (CommandPath empty); it must call initTelemetry(cmd)",
			cmd.CommandPath())
	}
}
