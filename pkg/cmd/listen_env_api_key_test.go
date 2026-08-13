package cmd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestShouldExchangeEnvAPIKey covers the precedence rule behind #334. listen
// ignored HOOKDECK_API_KEY entirely and fell through to a guest account, so
// traffic arrived locally but in a throwaway project. The env var must now
// authenticate — but only when nothing better already has, so an existing login
// is never repointed by a stray shell variable.
func TestShouldExchangeEnvAPIKey(t *testing.T) {
	tests := []struct {
		name          string
		currentAPIKey string
		envKey        string
		want          bool
	}{
		{
			name:          "no credential and env var set: exchange",
			currentAPIKey: "",
			envKey:        "proj_api_key",
			want:          true,
		},
		{
			name:          "no credential and no env var: leave guest fallback reachable",
			currentAPIKey: "",
			envKey:        "",
			want:          false,
		},
		{
			name:          "stored login wins over the env var",
			currentAPIKey: "cli_stored_key",
			envKey:        "proj_api_key",
			want:          false,
		},
		{
			name:          "--cli-key wins over the env var",
			currentAPIKey: "cli_key_from_flag",
			envKey:        "proj_api_key",
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, shouldExchangeEnvAPIKey(tt.currentAPIKey, tt.envKey))
		})
	}
}

// TestEnvAPIKeyReadsAndTrims guards the reader itself. A trailing newline from
// `export HOOKDECK_API_KEY=$(cat key.txt)` must not become a credential, and an
// all-whitespace value must be treated as absent rather than exchanged.
func TestEnvAPIKeyReadsAndTrims(t *testing.T) {
	t.Setenv("HOOKDECK_API_KEY", "  proj_key_with_spaces\n")
	assert.Equal(t, "proj_key_with_spaces", envAPIKey())

	t.Setenv("HOOKDECK_API_KEY", "   ")
	assert.Empty(t, envAPIKey(), "a whitespace-only value must not trigger an exchange")
	assert.False(t, shouldExchangeEnvAPIKey("", envAPIKey()))

	os.Unsetenv("HOOKDECK_API_KEY")
	assert.Empty(t, envAPIKey())
}

// TestEnvAPIKeyIsInjectable guards the seam applyEnvAPIKey depends on.
func TestEnvAPIKeyIsInjectable(t *testing.T) {
	original := envAPIKey
	t.Cleanup(func() { envAPIKey = original })

	envAPIKey = func() string { return "proj_injected" }
	assert.True(t, shouldExchangeEnvAPIKey("", envAPIKey()))
}
