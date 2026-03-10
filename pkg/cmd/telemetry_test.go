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
