package cmd

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApplyCLIPath pins the precedence between --cli-path and a path supplied
// via --config. Create previously overwrote the --config value with "/".
func TestApplyCLIPath(t *testing.T) {
	t.Run("--config path survives when --cli-path is absent", func(t *testing.T) {
		config := map[string]interface{}{"path": "/from-config"}
		applyCLIPath(config, "", true)
		assert.Equal(t, "/from-config", config["path"])
	})

	t.Run("--cli-path wins over --config", func(t *testing.T) {
		config := map[string]interface{}{"path": "/from-config"}
		applyCLIPath(config, "/from-flag", true)
		assert.Equal(t, "/from-flag", config["path"])
	})

	t.Run("create defaults to / when neither is given", func(t *testing.T) {
		config := map[string]interface{}{}
		applyCLIPath(config, "", true)
		assert.Equal(t, "/", config["path"])
	})

	t.Run("upsert leaves path absent when neither is given", func(t *testing.T) {
		config := map[string]interface{}{}
		applyCLIPath(config, "", false)
		_, ok := config["path"]
		assert.False(t, ok, "upsert must not invent a path, or a partial update would reset it")
	})
}

// TestRejectDeliveryPolicyForCLI pins that delivery-policy flags are refused for
// CLI destinations. The API accepts the request and discards the policy, so
// without this the flags look applied but never take effect.
func TestRejectDeliveryPolicyForCLI(t *testing.T) {
	policy := map[string]interface{}{"rate": 100, "period": "minute"}

	t.Run("CLI destination is rejected", func(t *testing.T) {
		err := rejectDeliveryPolicyForCLI("CLI", policy, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--rate-limit")
		assert.Contains(t, err.Error(), "CLI destinations")
	})

	t.Run("connection flag prefix is named in the message", func(t *testing.T) {
		err := rejectDeliveryPolicyForCLI("cli", policy, "destination-")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--destination-rate-limit")
	})

	t.Run("HTTP and MOCK_API are unaffected", func(t *testing.T) {
		assert.NoError(t, rejectDeliveryPolicyForCLI("HTTP", policy, ""))
		assert.NoError(t, rejectDeliveryPolicyForCLI("MOCK_API", policy, ""))
	})

	t.Run("CLI without a policy is fine", func(t *testing.T) {
		assert.NoError(t, rejectDeliveryPolicyForCLI("CLI", map[string]interface{}{}, ""))
	})
}

// TestBuildDestinationConfigRejectsDeliveryPolicyForCLI covers the funnel that
// destination create/update/upsert all go through.
func TestBuildDestinationConfigRejectsDeliveryPolicyForCLI(t *testing.T) {
	flags := &destinationConfigFlags{RateLimit: 100, RateLimitPeriod: "minute"}
	_, err := buildDestinationConfigFromIndividualFlags("CLI", flags)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CLI destinations")

	// Same flags on HTTP still build a policy.
	config, err := buildDestinationConfigFromIndividualFlags("HTTP", flags)
	require.NoError(t, err)
	policy, ok := config["delivery_policy"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, 100, policy["rate"])
}

// TestCLIPathFromFlags pins the fix for --cli-path's "/" default overwriting a
// path supplied via --config. Comparing the value against "" is not enough,
// because on create the flag is never empty.
func TestCLIPathFromFlags(t *testing.T) {
	newCmd := func() *cobra.Command {
		cmd := &cobra.Command{Use: "create"}
		var cliPath string
		cmd.Flags().StringVar(&cliPath, "cli-path", "/", "Path for CLI destinations")
		return cmd
	}

	t.Run("unset flag yields no path even though it defaults to /", func(t *testing.T) {
		cmd := newCmd()
		require.NoError(t, cmd.ParseFlags([]string{}))
		assert.Equal(t, "", cliPathFromFlags(cmd, "/"))
	})

	t.Run("explicitly passed flag is returned", func(t *testing.T) {
		cmd := newCmd()
		require.NoError(t, cmd.ParseFlags([]string{"--cli-path", "/hooks"}))
		assert.Equal(t, "/hooks", cliPathFromFlags(cmd, "/hooks"))
	})

	t.Run("explicitly passing the default value still counts as set", func(t *testing.T) {
		cmd := newCmd()
		require.NoError(t, cmd.ParseFlags([]string{"--cli-path", "/"}))
		assert.Equal(t, "/", cliPathFromFlags(cmd, "/"))
	})
}

// TestCLIPathFromFlagsEndToEnd covers the combination that regressed: --config
// supplies a path, --cli-path is not passed, and the config value must survive.
func TestCLIPathFromFlagsEndToEnd(t *testing.T) {
	// --cli-path is left unset, exactly as when the user passes only --config.
	cmd := &cobra.Command{Use: "create"}
	var cliPath string
	cmd.Flags().StringVar(&cliPath, "cli-path", "/", "Path for CLI destinations")
	require.NoError(t, cmd.ParseFlags([]string{}))

	config, err := buildDestinationConfigFromFlags(`{"path":"/from-config"}`, "", "CLI", nil)
	require.NoError(t, err)
	applyCLIPath(config, cliPathFromFlags(cmd, cliPath), true)

	assert.Equal(t, "/from-config", config["path"],
		"an unset --cli-path must not overwrite the path from --config")
}

// TestConnectionDestinationRejectsDeliveryPolicyForCLI covers the two
// connection paths, which build a delivery policy separately from the
// destination commands and so need their own guard.
func TestConnectionDestinationRejectsDeliveryPolicyForCLI(t *testing.T) {
	t.Run("connection create", func(t *testing.T) {
		cc := &connectionCreateCmd{}
		cc.destinationType = "CLI"
		cc.DestinationRateLimit = 100
		cc.DestinationRateLimitPeriod = "minute"

		_, err := cc.buildDestinationConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--destination-rate-limit")
		assert.Contains(t, err.Error(), "CLI destinations")
	})

	t.Run("connection upsert against an existing CLI destination", func(t *testing.T) {
		cu := &connectionUpsertCmd{connectionCreateCmd: &connectionCreateCmd{}}
		cu.DestinationRateLimit = 100
		cu.DestinationRateLimitPeriod = "minute"

		_, err := cu.buildDestinationInputForUpdate(&hookdeck.Destination{
			ID: "des_1", Name: "local", Type: "CLI",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "CLI destinations")
	})

	t.Run("connection upsert against an HTTP destination still applies", func(t *testing.T) {
		cu := &connectionUpsertCmd{connectionCreateCmd: &connectionCreateCmd{}}
		cu.DestinationRateLimit = 100
		cu.DestinationRateLimitPeriod = "minute"

		input, err := cu.buildDestinationInputForUpdate(&hookdeck.Destination{
			ID: "des_2", Name: "web", Type: "HTTP",
		})
		require.NoError(t, err)
		policy, ok := input.Config["delivery_policy"].(map[string]interface{})
		require.True(t, ok, "HTTP destinations must still receive the policy")
		assert.Equal(t, 100, policy["rate"])
	})
}

// runCreateFlags parses args through the real `destination create` flag set and
// returns the request body the command would POST.
func runCreateFlags(t *testing.T, args ...string) (*hookdeck.DestinationCreateRequest, error) {
	t.Helper()
	dc := newDestinationCreateCmd()
	require.NoError(t, dc.cmd.ParseFlags(args))
	return dc.buildCreateRequest(dc.cmd)
}

// TestCreateRequestHonoursTheCLIPathFlagState covers the two call sites in
// `destination create` rather than cliPathFromFlags on its own. The helper was
// pinned by TestCLIPathFromFlags, but nothing checked that the command still
// called it: replacing either call with the raw flag value reinstates the
// original bug with the suite green.
func TestCreateRequestHonoursTheCLIPathFlagState(t *testing.T) {
	t.Run("--config path survives an unset --cli-path", func(t *testing.T) {
		req, err := runCreateFlags(t,
			"--name", "local-cli", "--type", "CLI", "--config", `{"path":"/webhooks"}`)
		require.NoError(t, err)
		assert.Equal(t, "/webhooks", req.Config["path"],
			"the \"/\" default of an unset --cli-path must not overwrite --config")
	})

	t.Run("an unset --cli-path is not a CLI flag on an HTTP create", func(t *testing.T) {
		req, err := runCreateFlags(t,
			"--name", "my-api", "--type", "HTTP", "--url", "https://api.example.com/webhooks")
		require.NoError(t, err,
			"an unset --cli-path must not read as a CLI flag given for an HTTP destination")
		assert.Equal(t, "https://api.example.com/webhooks", req.Config["url"])
		assert.NotContains(t, req.Config, "path")
	})

	t.Run("an explicit --cli-path still wins over --config", func(t *testing.T) {
		req, err := runCreateFlags(t,
			"--name", "local-cli", "--type", "CLI",
			"--cli-path", "/from-flag", "--config", `{"path":"/from-config"}`)
		require.NoError(t, err)
		assert.Equal(t, "/from-flag", req.Config["path"])
	})

	t.Run("a CLI create with no path at all keeps the default", func(t *testing.T) {
		req, err := runCreateFlags(t, "--name", "local-cli", "--type", "CLI")
		require.NoError(t, err)
		assert.Equal(t, "/", req.Config["path"],
			"create still supplies the \"/\" default when nothing named a path")
	})
}
