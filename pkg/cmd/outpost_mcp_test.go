package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveAllowWrite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		allowWrite    bool
		allowWriteSet bool
		readOnly      bool
		env           string
		want          bool
	}{
		{name: "default is read-only", want: false},
		{name: "flag enables writes", allowWrite: true, allowWriteSet: true, want: true},
		{name: "env var enables writes", env: "true", want: true},
		{name: "env var accepts 1", env: "1", want: true},
		{name: "env var off", env: "false", want: false},
		{name: "unparseable env var is ignored", env: "yes please", want: false},
		{
			name:          "flag wins over the env var",
			allowWrite:    false,
			allowWriteSet: true,
			env:           "true",
			want:          false,
		},
		{
			name:          "read-only wins over the flag",
			allowWrite:    true,
			allowWriteSet: true,
			readOnly:      true,
			want:          false,
		},
		{name: "read-only wins over the env var", readOnly: true, env: "true", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resolveAllowWrite(tt.allowWrite, tt.allowWriteSet, tt.readOnly, tt.env)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOutpostMCPCommandIsRegistered(t *testing.T) {
	t.Parallel()

	cmd, _, err := RootCmd().Find([]string{"outpost", "mcp"})
	require.NoError(t, err)
	assert.Equal(t, "mcp", cmd.Name())
	require.True(t, isOutpostMCPLeafCommand(cmd), "the project gate must let MCP start unauthenticated")

	for _, name := range []string{"allow-write", "read-only", "publish-api-key"} {
		assert.NotNil(t, cmd.Flags().Lookup(name), "missing --%s", name)
	}

	// The credential is publish-specific on purpose. A generic --api-key would
	// read as the server's own authentication, which is the stored CLI login,
	// and publishing is the one action here that cannot be undone.
	assert.Nil(t, cmd.Flags().Lookup("api-key"), "the publish credential must not be named as if it authenticated the server")

	// Read-only is the default, so its help must not promise otherwise.
	assert.Equal(t, "false", cmd.Flags().Lookup("allow-write").DefValue)
	assert.Contains(t, cmd.Long, "read-only")
}
