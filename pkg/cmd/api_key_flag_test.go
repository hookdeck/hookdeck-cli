package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// secretKey stands in for a real key exported in the caller's shell.
const secretKey = "hkdk_test_key_do_not_print"

// TestAPIKeyFlagDefaultsAreEmpty guards against the environment being read into
// a flag's default value.
//
// pflag prints a non-empty string default in --help, so
// `HOOKDECK_API_KEY=... hookdeck ci --help` would print the key back to the
// terminal — and tools/generate-reference reads the same defaults, which would
// commit it to REFERENCE.md in a public repository.
func TestAPIKeyFlagDefaultsAreEmpty(t *testing.T) {
	t.Setenv("HOOKDECK_API_KEY", secretKey)

	commands := map[string]*cobra.Command{
		"ci":              newCICmd().cmd,
		"outpost publish": newOutpostPublishCmd().cmd,
		"outpost mcp":     newOutpostMCPCmd().cmd,
	}

	for name, cmd := range commands {
		t.Run(name, func(t *testing.T) {
			usage := cmd.Flags().FlagUsages()
			assert.NotContains(t, usage, secretKey, "--help must not print the key")

			seen := 0
			cmd.Flags().VisitAll(func(f *pflag.Flag) {
				if !strings.Contains(f.Name, "api-key") {
					return
				}
				seen++
				assert.Empty(t, f.DefValue, "--%s must not default to the environment", f.Name)
			})
			assert.NotZero(t, seen, "expected an api-key flag to check")
		})
	}
}

// TestAPIKeyEnvVarIsHonouredAtRunTime is the other half: moving the lookup out
// of the flag default must not stop the variable working.
func TestAPIKeyEnvVarIsHonouredAtRunTime(t *testing.T) {
	t.Run("ci reads the environment when the flag is absent", func(t *testing.T) {
		// Short enough to be rejected by the key validator, which happens after
		// the lookup and before anything is sent, so the error names the reason.
		t.Setenv("HOOKDECK_API_KEY", "tooshort")

		lc := newCICmd()
		err := lc.runCICmd(lc.cmd, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "too short")
		assert.Equal(t, "tooshort", lc.apiKey, "the key must come from the environment")
	})

	t.Run("ci without the environment asks for a key", func(t *testing.T) {
		t.Setenv("HOOKDECK_API_KEY", "")

		lc := newCICmd()
		err := lc.runCICmd(lc.cmd, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--api-key")
	})

	t.Run("outpost publish reads the environment when the flag is absent", func(t *testing.T) {
		t.Setenv("HOOKDECK_API_KEY", secretKey)

		pc := newOutpostPublishCmd()
		require.NoError(t, pc.validateFlags(pc.cmd, nil))
		assert.Equal(t, secretKey, pc.apiKey)
	})

	t.Run("outpost publish without the environment explains what is needed", func(t *testing.T) {
		t.Setenv("HOOKDECK_API_KEY", "")

		pc := newOutpostPublishCmd()
		err := pc.validateFlags(pc.cmd, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Project API key")
	})
}
