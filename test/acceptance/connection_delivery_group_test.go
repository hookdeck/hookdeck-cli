//go:build connection_upsert

package acceptance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// `gateway connection` carries the same delivery-policy flags as
// `gateway destination`, under a --destination- prefix. The create and upsert
// paths build the destination config through different code, so both need
// covering: connection upsert reaches the stored destination through the
// connection, not through a destination lookup.

// connectionDestinationDeliveryGroups extracts
// destination.config.delivery_policy.groups from a connection response body.
func connectionDestinationDeliveryGroups(t *testing.T, resp map[string]interface{}) map[string]interface{} {
	t.Helper()
	dest, ok := resp["destination"].(map[string]interface{})
	require.True(t, ok, "expected destination object on connection, got %T", resp["destination"])
	config, ok := dest["config"].(map[string]interface{})
	require.True(t, ok, "expected destination config object, got %T", dest["config"])
	policy, ok := config["delivery_policy"].(map[string]interface{})
	require.True(t, ok, "expected destination config.delivery_policy object, got %v", config["delivery_policy"])
	groups, ok := policy["groups"].(map[string]interface{})
	require.True(t, ok, "expected config.delivery_policy.groups object, got %v", policy["groups"])
	return groups
}

// cleanupConnectionAndResources deletes the connection and the source and
// destination it created inline. `gateway connection delete` removes only the
// connection, so tests that create resources inline have to delete them too —
// the suite already leaks ~36k sources and destinations (#362).
func cleanupConnectionAndResources(t *testing.T, cli *CLIRunner, resp map[string]interface{}) {
	t.Helper()
	id := func(key string) string {
		nested, ok := resp[key].(map[string]interface{})
		if !ok {
			return ""
		}
		s, _ := nested["id"].(string)
		return s
	}
	connID, _ := resp["id"].(string)
	srcID, dstID := id("source"), id("destination")

	t.Cleanup(func() {
		if connID != "" {
			deleteConnection(t, cli, connID)
		}
		if dstID != "" {
			deleteDestination(t, cli, dstID)
		}
		if srcID != "" {
			deleteSource(t, cli, srcID)
		}
	})
}

// requireNoConnection asserts that no connection exists under the given name.
// Refusing the command is only half the contract; the other half is that
// nothing was created before the refusal.
func requireNoConnection(t *testing.T, cli *CLIRunner, name string) {
	t.Helper()
	stdout, stderr, err := cli.Run("gateway", "connection", "get", name)
	if err == nil {
		var resp map[string]interface{}
		if jsonErr := cli.RunJSON(&resp, "gateway", "connection", "get", name); jsonErr == nil {
			cleanupConnectionAndResources(t, cli, resp)
		}
		t.Fatalf("rejected command must not create a connection, but %q exists: %s", name, stdout)
	}
	assert.Contains(t, stdout+stderr, "connection not found",
		"expected a not-found error for %q", name)
}

// connectionSampleOverrides mirrors sampleOverrides in the destination tests;
// the two files can be compiled into the same binary, so the names differ.
func connectionSampleOverrides() map[string]interface{} {
	return map[string]interface{}{
		"cust_1": map[string]interface{}{"rate": float64(5), "rate_period": "minute"},
		"cust_2": map[string]interface{}{"rate": float64(50), "rate_period": "second"},
	}
}

const connectionSampleOverridesJSON = `{"cust_1":{"rate":5,"rate_period":"minute"},"cust_2":{"rate":50,"rate_period":"second"}}`

// TestConnectionCreateWithDestinationDeliveryGroup creates a connection whose
// inline destination carries a full delivery-group triple plus overrides, then
// reads it back and asserts the stored groups object matches exactly.
func TestConnectionCreateWithDestinationDeliveryGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()
	connName := "test-conn-dg-create-" + timestamp

	var created map[string]interface{}
	err := cli.RunJSON(&created, "gateway", "connection", "create",
		"--name", connName,
		"--source-name", "test-src-dg-"+timestamp,
		"--source-type", "WEBHOOK",
		"--destination-name", "test-dst-dg-"+timestamp,
		"--destination-type", "HTTP",
		"--destination-url", "https://example.com/webhooks",
		"--destination-rate-limit", "100",
		"--destination-rate-limit-period", "minute",
		"--destination-delivery-group-key", "body.customer_id",
		"--destination-delivery-group-rate", "10",
		"--destination-delivery-group-rate-period", "second",
		"--destination-delivery-group-overrides", connectionSampleOverridesJSON,
	)
	require.NoError(t, err, "Should create connection with a destination delivery group")
	require.NotEmpty(t, created["id"], "Expected connection ID")
	cleanupConnectionAndResources(t, cli, created)

	// Read back rather than trusting the create response.
	var fetched map[string]interface{}
	err = cli.RunJSON(&fetched, "gateway", "connection", "get", connName)
	require.NoError(t, err, "Should read the connection back")

	assert.Equal(t, map[string]interface{}{
		"key":         "body.customer_id",
		"rate":        float64(10),
		"rate_period": "second",
		"overrides":   connectionSampleOverrides(),
	}, connectionDestinationDeliveryGroups(t, fetched),
		"stored delivery_policy.groups must match the flags exactly, overrides included")

	t.Logf("Created connection %s with destination delivery group overrides", connName)
}

// TestConnectionUpsertPreservesDestinationDeliveryGroupOverrides is the #393
// regression guard on the connection path. The API replaces
// delivery_policy.groups wholesale, so bumping only the group rate — which
// still has to repeat the key and rate-period — sends a full groups object and
// used to destroy the stored overrides.
func TestConnectionUpsertPreservesDestinationDeliveryGroupOverrides(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()
	connName := "test-conn-dg-upsert-" + timestamp

	var created map[string]interface{}
	err := cli.RunJSON(&created, "gateway", "connection", "create",
		"--name", connName,
		"--source-name", "test-src-dgu-"+timestamp,
		"--source-type", "WEBHOOK",
		"--destination-name", "test-dst-dgu-"+timestamp,
		"--destination-type", "HTTP",
		"--destination-url", "https://example.com/webhooks",
		"--destination-delivery-group-key", "body.customer_id",
		"--destination-delivery-group-rate", "10",
		"--destination-delivery-group-rate-period", "second",
		"--destination-delivery-group-overrides", connectionSampleOverridesJSON,
	)
	require.NoError(t, err, "Should create connection carrying overrides")
	require.NotEmpty(t, created["id"], "Expected connection ID")
	cleanupConnectionAndResources(t, cli, created)

	require.Equal(t, connectionSampleOverrides(), connectionDestinationDeliveryGroups(t, created)["overrides"],
		"precondition: the destination must be created with overrides")

	// Change ONLY the group rate.
	var upserted map[string]interface{}
	err = cli.RunJSON(&upserted, "gateway", "connection", "upsert", connName,
		"--destination-delivery-group-key", "body.customer_id",
		"--destination-delivery-group-rate", "30",
		"--destination-delivery-group-rate-period", "second",
	)
	require.NoError(t, err, "Should upsert the group rate")

	var fetched map[string]interface{}
	err = cli.RunJSON(&fetched, "gateway", "connection", "get", connName)
	require.NoError(t, err, "Should read the connection back after upsert")

	groups := connectionDestinationDeliveryGroups(t, fetched)
	assert.Equal(t, float64(30), groups["rate"], "the requested rate change must apply")
	assert.Equal(t, "body.customer_id", groups["key"], "the group key must be unchanged")
	assert.Equal(t, "second", groups["rate_period"], "the group rate period must be unchanged")
	assert.Equal(t, connectionSampleOverrides(), groups["overrides"],
		"bumping the group rate must not destroy the stored per-group overrides (#393)")
}

// TestConnectionDestinationDeliveryPolicyRejectedForCLIType asserts the
// delivery-policy flags are refused against a CLI destination, and that no
// connection is created. A CLI destination has no delivery_policy in the API
// schema: the API accepts the request and discards the policy.
func TestConnectionDestinationDeliveryPolicyRejectedForCLIType(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cases := []struct {
		name  string
		flags []string
	}{
		{"rate limit", []string{"--destination-rate-limit", "100", "--destination-rate-limit-period", "minute"}},
		{"delivery group", []string{
			"--destination-delivery-group-key", "body.customer_id",
			"--destination-delivery-group-rate", "10",
			"--destination-delivery-group-rate-period", "second",
		}},
		{"delivery group with overrides", []string{
			"--destination-delivery-group-key", "body.customer_id",
			"--destination-delivery-group-rate", "10",
			"--destination-delivery-group-rate-period", "second",
			"--destination-delivery-group-overrides", connectionSampleOverridesJSON,
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cli := NewCLIRunner(t)
			timestamp := generateTimestamp()
			connName := "test-conn-dg-cli-" + timestamp

			args := append([]string{
				"gateway", "connection", "create",
				"--name", connName,
				"--source-name", "test-src-dgcli-" + timestamp,
				"--source-type", "WEBHOOK",
				"--destination-name", "test-dst-dgcli-" + timestamp,
				"--destination-type", "CLI",
				"--destination-cli-path", "/webhooks",
			}, tc.flags...)

			stdout, stderr, err := cli.Run(args...)
			require.Error(t, err, "delivery-policy flags must be refused for CLI destinations")
			assert.Contains(t, stdout+stderr, "CLI destinations",
				"the error should name CLI destinations as the reason")

			requireNoConnection(t, cli, connName)
		})
	}
}

// TestConnectionDestinationDeliveryPolicyPartialFlagsRejected asserts that an
// incomplete delivery-policy flag set is refused before any API call, and that
// nothing is created.
func TestConnectionDestinationDeliveryPolicyPartialFlagsRejected(t *testing.T) {
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
			flags:     []string{"--destination-delivery-group-rate", "10", "--destination-delivery-group-rate-period", "second"},
			wantError: "--destination-delivery-group-key",
		},
		{
			name:      "group rate without rate period",
			flags:     []string{"--destination-delivery-group-key", "body.customer_id", "--destination-delivery-group-rate", "10"},
			wantError: "--destination-delivery-group-rate-period",
		},
		{
			name:      "overrides without a key",
			flags:     []string{"--destination-delivery-group-overrides", connectionSampleOverridesJSON},
			wantError: "--destination-delivery-group-key",
		},
		{
			name:      "rate limit without period",
			flags:     []string{"--destination-rate-limit", "100"},
			wantError: "--destination-rate-limit-period",
		},
		{
			name:      "negative group rate",
			flags:     []string{"--destination-delivery-group-key", "body.customer_id", "--destination-delivery-group-rate=-5", "--destination-delivery-group-rate-period", "second"},
			wantError: "--destination-delivery-group-rate must be a positive integer",
		},
		{
			name:      "negative rate limit",
			flags:     []string{"--destination-rate-limit=-5", "--destination-rate-limit-period", "minute"},
			wantError: "--destination-rate-limit must be a positive integer",
		},
		{
			name:      "malformed overrides JSON",
			flags:     []string{"--destination-delivery-group-key", "body.customer_id", "--destination-delivery-group-rate", "10", "--destination-delivery-group-rate-period", "second", "--destination-delivery-group-overrides", `{"cust_1": {"rate": 5`},
			wantError: "--destination-delivery-group-overrides must be a valid JSON object",
		},
		{
			name:      "overrides as a JSON array",
			flags:     []string{"--destination-delivery-group-key", "body.customer_id", "--destination-delivery-group-rate", "10", "--destination-delivery-group-rate-period", "second", "--destination-delivery-group-overrides", `[{"rate":5}]`},
			wantError: "--destination-delivery-group-overrides must be a valid JSON object",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cli := NewCLIRunner(t)
			timestamp := generateTimestamp()
			connName := "test-conn-dg-partial-" + timestamp

			args := append([]string{
				"gateway", "connection", "create",
				"--name", connName,
				"--source-name", "test-src-dgp-" + timestamp,
				"--source-type", "WEBHOOK",
				"--destination-name", "test-dst-dgp-" + timestamp,
				"--destination-type", "HTTP",
				"--destination-url", "https://example.com/webhooks",
			}, tc.flags...)

			stdout, stderr, err := cli.Run(args...)
			require.Error(t, err, "an incomplete or invalid delivery-policy flag set must be refused")
			assert.Contains(t, stdout+stderr, tc.wantError,
				"the error should name the offending flag")

			requireNoConnection(t, cli, connName)
		})
	}
}
