//go:build destination

package acceptance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Delivery groups are the headline feature of v2.6.0. A destination carries a
// config.delivery_policy with a destination-level rate/period and a nested
// groups object (key, rate, rate_period, overrides), driven by
// --delivery-group-key / --delivery-group-rate / --delivery-group-rate-period /
// --delivery-group-overrides on `gateway destination`.
//
// These tests exercise the real API, because the defects worth guarding against
// are all about what the API ends up storing, not about what the CLI builds in
// memory.

// destinationDeliveryPolicy extracts config.delivery_policy from a destination
// response body.
func destinationDeliveryPolicy(t *testing.T, resp map[string]interface{}) map[string]interface{} {
	t.Helper()
	config, ok := resp["config"].(map[string]interface{})
	require.True(t, ok, "expected config object on destination, got %T", resp["config"])
	policy, ok := config["delivery_policy"].(map[string]interface{})
	require.True(t, ok, "expected config.delivery_policy object, got %v", config["delivery_policy"])
	return policy
}

// destinationDeliveryGroups extracts config.delivery_policy.groups.
func destinationDeliveryGroups(t *testing.T, resp map[string]interface{}) map[string]interface{} {
	t.Helper()
	groups, ok := destinationDeliveryPolicy(t, resp)["groups"].(map[string]interface{})
	require.True(t, ok, "expected config.delivery_policy.groups object")
	return groups
}

// requireNoDestination asserts that no destination exists under the given name.
// Every rejection test uses this: refusing the command is only half the
// contract, the other half is that nothing was created before the refusal.
func requireNoDestination(t *testing.T, cli *CLIRunner, name string) {
	t.Helper()
	stdout, stderr, err := cli.Run("gateway", "destination", "get", name)
	if err == nil {
		// Something was created despite the rejection. Clean it up so the
		// failure does not also leak a resource (#362), then fail.
		var dst Destination
		if jsonErr := cli.RunJSON(&dst, "gateway", "destination", "get", name); jsonErr == nil && dst.ID != "" {
			deleteDestination(t, cli, dst.ID)
		}
		t.Fatalf("rejected command must not create a destination, but %q exists: %s", name, stdout)
	}
	assert.Contains(t, stdout+stderr, "no destination found",
		"expected a not-found error for %q", name)
}

// sampleOverrides is the overrides object used across these tests. Two entries,
// so a test cannot pass by preserving only the first.
func sampleOverrides() map[string]interface{} {
	return map[string]interface{}{
		"cust_1": map[string]interface{}{"rate": float64(5), "rate_period": "minute"},
		"cust_2": map[string]interface{}{"rate": float64(50), "rate_period": "second"},
	}
}

const sampleOverridesJSON = `{"cust_1":{"rate":5,"rate_period":"minute"},"cust_2":{"rate":50,"rate_period":"second"}}`

// TestDestinationCreateWithDeliveryGroup creates a destination with a
// destination-level rate limit and a full delivery-group triple plus
// overrides, then reads it back with --output json and asserts the stored
// delivery_policy matches exactly, nested overrides included.
func TestDestinationCreateWithDeliveryGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	name := "test-dst-dg-create-" + generateTimestamp()

	var created map[string]interface{}
	err := cli.RunJSON(&created, "gateway", "destination", "create",
		"--name", name,
		"--type", "HTTP",
		"--url", "https://example.com/webhooks",
		"--rate-limit", "100",
		"--rate-limit-period", "minute",
		"--delivery-group-key", "body.customer_id",
		"--delivery-group-rate", "10",
		"--delivery-group-rate-period", "second",
		"--delivery-group-overrides", sampleOverridesJSON,
	)
	require.NoError(t, err, "Should create destination with a delivery group")

	dstID, ok := created["id"].(string)
	require.True(t, ok && dstID != "", "Expected destination ID")
	t.Cleanup(func() { deleteDestination(t, cli, dstID) })

	// Read back rather than trusting the create response.
	var fetched map[string]interface{}
	err = cli.RunJSON(&fetched, "gateway", "destination", "get", dstID)
	require.NoError(t, err, "Should read the destination back")

	policy := destinationDeliveryPolicy(t, fetched)
	assert.Equal(t, float64(100), policy["rate"], "destination-level rate should be stored")
	assert.Equal(t, "minute", policy["period"], "destination-level period should be stored")

	assert.Equal(t, map[string]interface{}{
		"key":         "body.customer_id",
		"rate":        float64(10),
		"rate_period": "second",
		"overrides":   sampleOverrides(),
	}, destinationDeliveryGroups(t, fetched),
		"stored delivery_policy.groups must match the flags exactly, overrides included")

	t.Logf("Created destination %s (ID: %s) with delivery group overrides", name, dstID)
}

// TestDestinationUpsertPreservesDeliveryGroupOverrides is the regression guard
// for #393. The API replaces delivery_policy.groups wholesale, and the CLI
// requires --delivery-group-key and --delivery-group-rate-period whenever
// --delivery-group-rate is given, so "just bump the group rate" always sends a
// full groups object. Before the fix that silently destroyed the stored
// overrides. Only unit tests covered this; nothing exercised the real API.
func TestDestinationUpsertPreservesDeliveryGroupOverrides(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	name := "test-dst-dg-upsert-" + generateTimestamp()

	var created map[string]interface{}
	err := cli.RunJSON(&created, "gateway", "destination", "create",
		"--name", name,
		"--type", "HTTP",
		"--url", "https://example.com/webhooks",
		"--delivery-group-key", "body.customer_id",
		"--delivery-group-rate", "10",
		"--delivery-group-rate-period", "second",
		"--delivery-group-overrides", sampleOverridesJSON,
	)
	require.NoError(t, err, "Should create destination carrying overrides")

	dstID, ok := created["id"].(string)
	require.True(t, ok && dstID != "", "Expected destination ID")
	t.Cleanup(func() { deleteDestination(t, cli, dstID) })

	require.Equal(t, sampleOverrides(), destinationDeliveryGroups(t, created)["overrides"],
		"precondition: the destination must be created with overrides")

	// Change ONLY the group rate. The key and rate-period have to be repeated
	// because the CLI refuses a partial group triple.
	var upserted map[string]interface{}
	err = cli.RunJSON(&upserted, "gateway", "destination", "upsert", name,
		"--delivery-group-key", "body.customer_id",
		"--delivery-group-rate", "30",
		"--delivery-group-rate-period", "second",
	)
	require.NoError(t, err, "Should upsert the group rate")

	var fetched map[string]interface{}
	err = cli.RunJSON(&fetched, "gateway", "destination", "get", dstID)
	require.NoError(t, err, "Should read the destination back after upsert")

	groups := destinationDeliveryGroups(t, fetched)
	assert.Equal(t, float64(30), groups["rate"], "the requested rate change must apply")
	assert.Equal(t, "body.customer_id", groups["key"], "the group key must be unchanged")
	assert.Equal(t, "second", groups["rate_period"], "the group rate period must be unchanged")
	assert.Equal(t, sampleOverrides(), groups["overrides"],
		"bumping the group rate must not destroy the stored per-group overrides (#393)")
}

// TestDestinationUpdatePreservesDeliveryGroupOverrides is the same #393
// contract on `gateway destination update`.
//
// KNOWN FAILURE at the time of writing. preserveDeliveryGroupOverrides is
// called from `destination upsert` and `connection upsert`, but not from
// `destination update`, so bumping only the group rate through `update` still
// destroys the stored overrides — the identical silent data loss, on a sibling
// command. Verified against the live API: the groups object comes back with
// rate 30 and no overrides key at all.
//
// The behaviour a user gets should not depend on which of two commands they
// reach for, so this asserts the same thing the upsert test does.
func TestDestinationUpdatePreservesDeliveryGroupOverrides(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	name := "test-dst-dg-update-" + generateTimestamp()

	var created map[string]interface{}
	err := cli.RunJSON(&created, "gateway", "destination", "create",
		"--name", name,
		"--type", "HTTP",
		"--url", "https://example.com/webhooks",
		"--delivery-group-key", "body.customer_id",
		"--delivery-group-rate", "10",
		"--delivery-group-rate-period", "second",
		"--delivery-group-overrides", sampleOverridesJSON,
	)
	require.NoError(t, err, "Should create destination carrying overrides")

	dstID, ok := created["id"].(string)
	require.True(t, ok && dstID != "", "Expected destination ID")
	t.Cleanup(func() { deleteDestination(t, cli, dstID) })

	require.Equal(t, sampleOverrides(), destinationDeliveryGroups(t, created)["overrides"],
		"precondition: the destination must be created with overrides")

	// Change ONLY the group rate.
	var updated map[string]interface{}
	err = cli.RunJSON(&updated, "gateway", "destination", "update", dstID,
		"--delivery-group-key", "body.customer_id",
		"--delivery-group-rate", "30",
		"--delivery-group-rate-period", "second",
	)
	require.NoError(t, err, "Should update the group rate")

	var fetched map[string]interface{}
	err = cli.RunJSON(&fetched, "gateway", "destination", "get", dstID)
	require.NoError(t, err, "Should read the destination back after update")

	groups := destinationDeliveryGroups(t, fetched)
	assert.Equal(t, float64(30), groups["rate"], "the requested rate change must apply")
	assert.Equal(t, sampleOverrides(), groups["overrides"],
		"bumping the group rate via `destination update` must not destroy the stored "+
			"per-group overrides; the #393 fix reached upsert but not update")
}

// TestDestinationDeliveryPolicyRejectedForCLIType asserts that delivery-policy
// flags are refused on a CLI destination. A CLI destination carries no
// delivery_policy in the API schema: the API accepts the request and silently
// discards the policy, so the CLI has to refuse rather than let the flags look
// applied. Nothing may be created.
func TestDestinationDeliveryPolicyRejectedForCLIType(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cases := []struct {
		name  string
		flags []string
	}{
		{"rate limit", []string{"--rate-limit", "100", "--rate-limit-period", "minute"}},
		{"delivery group", []string{
			"--delivery-group-key", "body.customer_id",
			"--delivery-group-rate", "10",
			"--delivery-group-rate-period", "second",
		}},
		{"delivery group with overrides", []string{
			"--delivery-group-key", "body.customer_id",
			"--delivery-group-rate", "10",
			"--delivery-group-rate-period", "second",
			"--delivery-group-overrides", sampleOverridesJSON,
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cli := NewCLIRunner(t)
			name := "test-dst-dg-cli-" + generateTimestamp()

			args := append([]string{
				"gateway", "destination", "create",
				"--name", name,
				"--type", "CLI",
				"--cli-path", "/webhooks",
			}, tc.flags...)

			stdout, stderr, err := cli.Run(args...)
			require.Error(t, err, "delivery-policy flags must be refused for CLI destinations")
			assert.Contains(t, stdout+stderr, "CLI destinations",
				"the error should name CLI destinations as the reason")

			requireNoDestination(t, cli, name)
		})
	}
}

// TestDestinationDeliveryPolicyPartialFlagsRejected asserts that an incomplete
// set of delivery-policy flags is refused before any API call, and that nothing
// is created. Each of these silently dropped the value at some point.
func TestDestinationDeliveryPolicyPartialFlagsRejected(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cases := []struct {
		name      string
		flags     []string
		wantError string
	}{
		{
			name:      "group rate without key",
			flags:     []string{"--delivery-group-rate", "10", "--delivery-group-rate-period", "second"},
			wantError: "--delivery-group-key",
		},
		{
			name:      "group rate without rate period",
			flags:     []string{"--delivery-group-key", "body.customer_id", "--delivery-group-rate", "10"},
			wantError: "--delivery-group-rate-period",
		},
		{
			name:      "group key without rate",
			flags:     []string{"--delivery-group-key", "body.customer_id", "--delivery-group-rate-period", "second"},
			wantError: "--delivery-group-rate",
		},
		{
			name:      "overrides without a key",
			flags:     []string{"--delivery-group-overrides", sampleOverridesJSON},
			wantError: "--delivery-group-key",
		},
		{
			name:      "rate limit without period",
			flags:     []string{"--rate-limit", "100"},
			wantError: "--rate-limit-period",
		},
		{
			name:      "rate limit period without rate",
			flags:     []string{"--rate-limit-period", "minute"},
			wantError: "--rate-limit",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cli := NewCLIRunner(t)
			name := "test-dst-dg-partial-" + generateTimestamp()

			args := append([]string{
				"gateway", "destination", "create",
				"--name", name,
				"--type", "HTTP",
				"--url", "https://example.com/webhooks",
			}, tc.flags...)

			stdout, stderr, err := cli.Run(args...)
			require.Error(t, err, "an incomplete delivery-policy flag set must be refused")
			assert.Contains(t, stdout+stderr, tc.wantError,
				"the error should name the missing flag")

			requireNoDestination(t, cli, name)
		})
	}
}

// TestDestinationDeliveryPolicyRejectsNonPositiveRates asserts that zero and
// negative rates are refused client-side rather than dropped. Testing only
// `rate > 0` once let --rate-limit=-5 fall through both guards: the command
// succeeded having quietly ignored the value.
func TestDestinationDeliveryPolicyRejectsNonPositiveRates(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cases := []struct {
		name      string
		flags     []string
		wantError string
	}{
		{
			name:      "negative rate limit",
			flags:     []string{"--rate-limit=-5", "--rate-limit-period", "minute"},
			wantError: "--rate-limit must be a positive integer",
		},
		{
			name:      "zero rate limit",
			flags:     []string{"--rate-limit", "0", "--rate-limit-period", "minute"},
			wantError: "--rate-limit must be a positive integer",
		},
		{
			name: "negative group rate",
			flags: []string{
				"--delivery-group-key", "body.customer_id",
				"--delivery-group-rate=-5",
				"--delivery-group-rate-period", "second",
			},
			wantError: "--delivery-group-rate must be a positive integer",
		},
		{
			name: "zero group rate",
			flags: []string{
				"--delivery-group-key", "body.customer_id",
				"--delivery-group-rate", "0",
				"--delivery-group-rate-period", "second",
			},
			wantError: "--delivery-group-rate must be a positive integer",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cli := NewCLIRunner(t)
			name := "test-dst-dg-rate-" + generateTimestamp()

			args := append([]string{
				"gateway", "destination", "create",
				"--name", name,
				"--type", "HTTP",
				"--url", "https://example.com/webhooks",
			}, tc.flags...)

			stdout, stderr, err := cli.Run(args...)
			require.Error(t, err, "a non-positive rate must be refused, not silently dropped")
			assert.Contains(t, stdout+stderr, tc.wantError)

			requireNoDestination(t, cli, name)
		})
	}
}

// TestDestinationDeliveryGroupOverridesMustBeJSONObject asserts that
// --delivery-group-overrides is parsed and rejected when it is not a JSON
// object. A JSON array parses as valid JSON but not as the object the API
// expects, so it needs its own case.
func TestDestinationDeliveryGroupOverridesMustBeJSONObject(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cases := []struct {
		name      string
		overrides string
	}{
		{"malformed JSON", `{"cust_1": {"rate": 5`},
		{"not JSON at all", `cust_1=5`},
		{"JSON array instead of object", `[{"rate":5,"rate_period":"minute"}]`},
		{"JSON null", `null`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cli := NewCLIRunner(t)
			name := "test-dst-dg-badjson-" + generateTimestamp()

			stdout, stderr, err := cli.Run(
				"gateway", "destination", "create",
				"--name", name,
				"--type", "HTTP",
				"--url", "https://example.com/webhooks",
				"--delivery-group-key", "body.customer_id",
				"--delivery-group-rate", "10",
				"--delivery-group-rate-period", "second",
				"--delivery-group-overrides", tc.overrides,
			)
			require.Error(t, err, "invalid overrides JSON must be refused")
			assert.Contains(t, stdout+stderr, "--delivery-group-overrides must be a valid JSON object")

			requireNoDestination(t, cli, name)
		})
	}
}
