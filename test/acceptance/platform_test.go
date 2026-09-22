//go:build platform

package acceptance

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The org and project platform commands added in v3.0.0-beta.2.
//
// Read paths run against the live API. Write paths are exercised only where
// they are reversible or already refused locally — creating and deleting real
// organizations or API keys from a test suite is not worth the blast radius,
// and the destructive commands are covered by their confirmation guard instead.

func TestOrgGet(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)

	stdout, stderr, err := cli.Run("org", "get")
	require.NoError(t, err, "stderr: %s", stderr)
	assert.Contains(t, stdout, "Organization:")
	assert.Contains(t, stdout, "ID:")
}

func TestOrgGetJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)

	stdout, stderr, err := cli.Run("org", "get", "--output", "json")
	require.NoError(t, err, "stderr: %s", stderr)

	var org map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &org), "stdout: %s", stdout)
	assert.NotEmpty(t, org["id"])
}

// The listing returns the secret like every other field. Printing it would put
// a live credential into a terminal, a scrollback and any CI log.
func TestAPIKeyListNeverPrintsASecret(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)

	stdout, stderr, err := cli.Run("org", "api-key", "list")
	require.NoError(t, err, "stderr: %s", stderr)

	// Hookdeck key secrets are prefixed; a fingerprint is not.
	for _, prefix := range []string{"hd_org_", "hd_proj_"} {
		assert.NotContains(t, stdout, prefix,
			"the key listing must never print a secret")
	}

	jsonOut, _, err := cli.Run("org", "api-key", "list", "--output", "json")
	require.NoError(t, err)
	for _, prefix := range []string{"hd_org_", "hd_proj_"} {
		assert.NotContains(t, jsonOut, prefix, "the JSON listing must never print a secret either")
	}
}

// Empty-change guards: the API accepts these and does nothing, so the command
// has to refuse rather than exit 0 having changed nothing.
func TestPlatformCommandsRefuseEmptyChanges(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)

	cases := []struct {
		name string
		args []string
		want string
	}{
		{"project create without a name", []string{"project", "create"}, "--name is required"},
		{"project create without a type", []string{"project", "create", "--name", "x"}, "--type is required"},
		{"org update without a name", []string{"org", "update"}, "--name is required"},
		{"api-key roll without a delay", []string{"org", "api-key", "roll", "apk_x"}, "--delay is required"},
		{"api-key update without scopes", []string{"org", "api-key", "update", "apk_x"}, "--scope is required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, err := cli.Run(tc.args...)
			require.Error(t, err, "the command should fail rather than do nothing")
			// The root error handler prints to stdout for non-MCP commands;
			// stderr carries only the exit status.
			assert.Contains(t, stdout+stderr, tc.want)
		})
	}
}

// Destructive commands refuse to proceed without a terminal rather than
// printing "cancelled" and exiting 0 — a CI job that deleted nothing must not
// look like a success.
func TestDestructiveCommandsRefuseWithoutATerminal(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)

	for _, args := range [][]string{
		{"project", "delete", "tm_does_not_exist"},
		{"org", "api-key", "delete", "apk_does_not_exist"},
		{"project", "custom-domain", "remove", "tm_x", "dom_x"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			stdout, stderr, err := cli.Run(args...)
			require.Error(t, err)
			out := stdout + stderr
			assert.Contains(t, out, "no terminal is attached",
				"a destructive command must not silently no-op without a terminal")
			assert.Contains(t, out, "--force")
		})
	}
}

// project get resolves against the live API.
func TestProjectGet(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)

	list, _, err := cli.Run("project", "list", "--output", "json")
	require.NoError(t, err)
	var projects []map[string]any
	require.NoError(t, json.Unmarshal([]byte(list), &projects))
	if len(projects) == 0 {
		t.Skip("no projects visible to this credential")
	}

	id, _ := projects[0]["id"].(string)
	require.NotEmpty(t, id)

	stdout, stderr, err := cli.Run("project", "get", id)
	require.NoError(t, err, "stderr: %s", stderr)
	assert.Contains(t, stdout, id)
}
