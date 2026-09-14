package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
)

// TestBuildDestinationConfigFromJSONString verifies that --config (JSON string) parses
// into a config map with exact values preserved.
func TestBuildDestinationConfigFromJSONString(t *testing.T) {
	t.Run("HTTP config JSON with exact values", func(t *testing.T) {
		input := `{"url":"https://api.example.com/hooks","http_method":"PUT","delivery_policy":{"rate":100,"period":"second"}}`
		config, err := buildDestinationConfigFromFlags(input, "", "", nil)
		require.NoError(t, err)
		require.NotNil(t, config)

		assert.Equal(t, "https://api.example.com/hooks", config["url"])
		assert.Equal(t, "PUT", config["http_method"])
		policy, ok := config["delivery_policy"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, float64(100), policy["rate"])
		assert.Equal(t, "second", policy["period"])
	})

	t.Run("config with auth JSON preserves exact values", func(t *testing.T) {
		input := `{"url":"https://api.example.com","auth_type":"BEARER_TOKEN","auth":{"bearer_token":"sk-test-token-xyz"}}`
		config, err := buildDestinationConfigFromFlags(input, "", "", nil)
		require.NoError(t, err)
		require.NotNil(t, config)

		assert.Equal(t, "https://api.example.com", config["url"])
		assert.Equal(t, "BEARER_TOKEN", config["auth_type"])

		auth, ok := config["auth"].(map[string]interface{})
		require.True(t, ok, "auth should be a map, got %T", config["auth"])
		assert.Equal(t, "sk-test-token-xyz", auth["bearer_token"],
			"bearer_token value should be exactly 'sk-test-token-xyz'")
	})

	t.Run("config with nested custom signature preserves structure", func(t *testing.T) {
		input := `{"url":"https://api.example.com","auth_type":"CUSTOM_SIGNATURE","auth":{"secret":"sig_secret_123","key":"X-Signature"}}`
		config, err := buildDestinationConfigFromFlags(input, "", "", nil)
		require.NoError(t, err)
		require.NotNil(t, config)

		auth, ok := config["auth"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "sig_secret_123", auth["secret"])
		assert.Equal(t, "X-Signature", auth["key"])
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		_, err := buildDestinationConfigFromFlags(`{broken`, "", "", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--config")
	})
}

func TestBuildDestinationConfigFromIndividualFlagsDeliveryPolicy(t *testing.T) {
	flags := &destinationConfigFlags{
		RateLimit:               100,
		RateLimitPeriod:         "minute",
		DeliveryGroupKey:        "body.customer_id",
		DeliveryGroupRate:       5,
		DeliveryGroupRatePeriod: "second",
		DeliveryGroupOverrides:  `{"cus_priority":{"rate":50,"rate_period":"second"}}`,
	}

	config, err := buildDestinationConfigFromIndividualFlags("HTTP", flags)
	require.NoError(t, err)
	assert.NotContains(t, config, "rate_limit")
	assert.NotContains(t, config, "rate_limit_period")

	policy, ok := config["delivery_policy"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, 100, policy["rate"])
	assert.Equal(t, "minute", policy["period"])

	groups, ok := policy["groups"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "body.customer_id", groups["key"])
	assert.Equal(t, 5, groups["rate"])
	assert.Equal(t, "second", groups["rate_period"])
	overrides, ok := groups["overrides"].(map[string]interface{})
	require.True(t, ok)
	priority, ok := overrides["cus_priority"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(50), priority["rate"])
	assert.Equal(t, "second", priority["rate_period"])
}

func TestBuildDestinationConfigFromIndividualFlagsDeliveryGroupValidation(t *testing.T) {
	tests := []struct {
		name      string
		flags     destinationConfigFlags
		wantError string
	}{
		{name: "missing key", flags: destinationConfigFlags{DeliveryGroupRate: 5, DeliveryGroupRatePeriod: "second"}, wantError: "--delivery-group-key"},
		{name: "missing rate", flags: destinationConfigFlags{DeliveryGroupKey: "body.customer_id", DeliveryGroupRatePeriod: "second"}, wantError: "--delivery-group-rate"},
		{name: "missing period", flags: destinationConfigFlags{DeliveryGroupKey: "body.customer_id", DeliveryGroupRate: 5}, wantError: "--delivery-group-rate-period"},
		{name: "invalid overrides", flags: destinationConfigFlags{DeliveryGroupKey: "body.customer_id", DeliveryGroupRate: 5, DeliveryGroupRatePeriod: "second", DeliveryGroupOverrides: "[]"}, wantError: "--delivery-group-overrides"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildDestinationConfigFromIndividualFlags("HTTP", &tt.flags)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
		})
	}
}

// TestBuildDestinationConfigFromJSONFile verifies that --config-file reads a JSON file
// and produces a config map with exact values preserved.
func TestBuildDestinationConfigFromJSONFile(t *testing.T) {
	t.Run("file config JSON with exact values", func(t *testing.T) {
		content := `{"url":"https://file-based.example.com/hooks","http_method":"PATCH","delivery_policy":{"rate":50,"period":"minute"}}`
		tmpFile := filepath.Join(t.TempDir(), "dest-config.json")
		require.NoError(t, os.WriteFile(tmpFile, []byte(content), 0644))

		config, err := buildDestinationConfigFromFlags("", tmpFile, "", nil)
		require.NoError(t, err)
		require.NotNil(t, config)

		assert.Equal(t, "https://file-based.example.com/hooks", config["url"])
		assert.Equal(t, "PATCH", config["http_method"])
		policy, ok := config["delivery_policy"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, float64(50), policy["rate"])
		assert.Equal(t, "minute", policy["period"])
	})

	t.Run("file with auth config preserves exact values", func(t *testing.T) {
		content := `{"url":"https://api.example.com","auth_type":"API_KEY","auth":{"api_key":"key_from_file_789","header_key":"X-API-Key","to":"header"}}`
		tmpFile := filepath.Join(t.TempDir(), "dest-auth-config.json")
		require.NoError(t, os.WriteFile(tmpFile, []byte(content), 0644))

		config, err := buildDestinationConfigFromFlags("", tmpFile, "", nil)
		require.NoError(t, err)
		require.NotNil(t, config)

		assert.Equal(t, "API_KEY", config["auth_type"])
		auth, ok := config["auth"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "key_from_file_789", auth["api_key"])
		assert.Equal(t, "X-API-Key", auth["header_key"])
		assert.Equal(t, "header", auth["to"])
	})

	t.Run("nonexistent file returns error", func(t *testing.T) {
		_, err := buildDestinationConfigFromFlags("", "/nonexistent/path.json", "", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--config-file")
	})

	t.Run("invalid JSON file returns error", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "bad.json")
		require.NoError(t, os.WriteFile(tmpFile, []byte(`{not valid`), 0644))

		_, err := buildDestinationConfigFromFlags("", tmpFile, "", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "config file")
	})
}

// withTestAPIKey gives validateFlags a key to accept, so these cases reach the
// flag checks that follow it.
func withTestAPIKey(t *testing.T) {
	t.Helper()
	old := Config
	t.Cleanup(func() { Config = old })
	Config = config.Config{}
	Config.Profile.APIKey = "sk_test_123456789012"
}

// destinationConfigCommand is one of the three commands that accept both a
// config JSON and the individual config flags, reduced to what these cases
// need: set some flags, run the validation the command runs.
type destinationConfigCommand struct {
	name     string
	cmd      *cobra.Command
	validate func(*cobra.Command, []string) error
	args     []string
}

// destinationConfigCommands returns a fresh instance of each command. pflag
// records "was this flag given" on the flag itself, so a command may only be
// used for one case.
func destinationConfigCommands() []destinationConfigCommand {
	create := newDestinationCreateCmd()
	update := newDestinationUpdateCmd()
	upsert := newDestinationUpsertCmd()
	return []destinationConfigCommand{
		{"create", create.cmd, create.validateFlags, nil},
		{"update", update.cmd, update.validateFlags, []string{"des_1"}},
		{"upsert", upsert.cmd, upsert.validateFlags, []string{"my-http"}},
	}
}

// TestDestinationConfigJSONRefusesIndividualFlags pins the contract the three
// commands could not agree on.
//
// --config was documented as overriding the individual flags and did on
// `update`; `create` and `upsert` overlaid --url back on top of it, and
// `upsert` only reached that overlay when --type was passed, because
// resolveDestinationType returns early on the --config path. So
// `upsert my-http --config '{"url":"https://old"}' --url https://new` exited 0
// having sent https://old with no mention of --url, and the same flags with
// --type HTTP sent https://new while `update` still sent https://old. Either
// input describes the whole config, so asking for both is now a conflict.
func TestDestinationConfigJSONRefusesIndividualFlags(t *testing.T) {
	conflicts := []struct {
		flag  string
		value string
	}{
		// The reported shape: silently dropped on the --config path.
		{"url", "https://new.example.com/hook"},
		{"cli-path", "/webhooks"},
		{"http-method", "PUT"},
		// Never covered by any overlay, so silently dropped on all three
		// commands whatever the type.
		{"auth-method", "bearer"},
		{"bearer-token", "tok_123"},
		{"rate-limit", "100"},
		{"delivery-group-key", "body.customer_id"},
	}

	for i, cmdUnderTest := range destinationConfigCommands() {
		for _, tt := range conflicts {
			t.Run(cmdUnderTest.name+" --config with --"+tt.flag, func(t *testing.T) {
				withTestAPIKey(t)
				c := destinationConfigCommands()[i]
				require.NoError(t, c.cmd.Flags().Set("config", `{"url":"https://old.example.com/hook"}`))
				require.NoError(t, c.cmd.Flags().Set(tt.flag, tt.value))

				err := c.validate(c.cmd, c.args)
				require.Error(t, err, "--%s alongside --config was dropped without a word", tt.flag)
				assert.Contains(t, err.Error(), "--"+tt.flag)
				assert.Contains(t, err.Error(), "--config")
			})
		}
	}
}

// TestDestinationConfigFileRefusesIndividualFlagsToo covers the other JSON
// input. --config-file reaches exactly the same builder, so the two have to
// answer the same way.
func TestDestinationConfigFileRefusesIndividualFlagsToo(t *testing.T) {
	for i, c := range destinationConfigCommands() {
		t.Run(c.name, func(t *testing.T) {
			withTestAPIKey(t)
			c := destinationConfigCommands()[i]
			require.NoError(t, c.cmd.Flags().Set("config-file", "/some/config.json"))
			require.NoError(t, c.cmd.Flags().Set("url", "https://new.example.com/hook"))

			err := c.validate(c.cmd, c.args)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "--url")
			assert.Contains(t, err.Error(), "--config-file")
		})
	}
}

// TestDestinationConfigJSONAloneIsStillAccepted is the other half: the refusal
// must key off flags the user actually typed, not off flags that carry a
// default. --api-key-to defaults to "header" on all three commands and
// --cli-path defaults to "/" on create, and neither is something the caller
// asked for.
func TestDestinationConfigJSONAloneIsStillAccepted(t *testing.T) {
	for i, c := range destinationConfigCommands() {
		t.Run(c.name, func(t *testing.T) {
			withTestAPIKey(t)
			c := destinationConfigCommands()[i]
			require.NoError(t, c.cmd.Flags().Set("config", `{"url":"https://api.example.com/hooks"}`))

			assert.NoError(t, c.validate(c.cmd, c.args),
				"a config JSON on its own is the whole point of the flag")
		})
	}
}

// TestDestinationUpsertAndUpdateAgreeOnConfigJSON states the invariant
// directly, because the two disagreeing is what made the corner hard to see:
// the same flags have to produce the same answer on both commands, with and
// without --type.
func TestDestinationUpsertAndUpdateAgreeOnConfigJSON(t *testing.T) {
	for _, declaredType := range []string{"", "HTTP"} {
		name := "without --type"
		if declaredType != "" {
			name = "with --type " + declaredType
		}
		t.Run(name, func(t *testing.T) {
			withTestAPIKey(t)

			update := newDestinationUpdateCmd()
			upsert := newDestinationUpsertCmd()
			for _, c := range []*cobra.Command{update.cmd, upsert.cmd} {
				require.NoError(t, c.Flags().Set("config", `{"url":"https://old.example.com/hook"}`))
				require.NoError(t, c.Flags().Set("url", "https://new.example.com/hook"))
				if declaredType != "" {
					require.NoError(t, c.Flags().Set("type", declaredType))
				}
			}

			updateErr := update.validateFlags(update.cmd, []string{"des_1"})
			upsertErr := upsert.validateFlags(upsert.cmd, []string{"my-http"})

			require.Error(t, updateErr)
			require.Error(t, upsertErr)
			assert.Equal(t, updateErr.Error(), upsertErr.Error(),
				"update and upsert must not resolve the same flags differently")
		})
	}
}
