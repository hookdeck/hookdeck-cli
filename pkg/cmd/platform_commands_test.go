package cmd

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func sub(t *testing.T, parent *cobra.Command, name string) *cobra.Command {
	t.Helper()
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	t.Fatalf("%s has no subcommand %q", parent.Name(), name)
	return nil
}

// The shape decided in plans/mcp_read_write_tool_split.md: one api-key family
// under org, because one endpoint serves both key kinds and one listing returns
// both. A project-parented family could not express an organization key.
func TestPlatformCommandTree(t *testing.T) {
	org := newOrgCmd().cmd
	assert.ElementsMatch(t, []string{"get", "update", "api-key"}, names(org))
	assert.ElementsMatch(t, []string{"list", "create", "update", "roll", "delete"},
		names(sub(t, org, "api-key")))

	project := newProjectCmd().cmd
	assert.Subset(t, names(project),
		[]string{"list", "use", "get", "create", "update", "delete", "custom-domain"})
	assert.ElementsMatch(t, []string{"list", "add", "remove"},
		names(sub(t, project, "custom-domain")))

	// There is deliberately no api-key under project.
	assert.NotContains(t, names(project), "api-key",
		"API keys live under org: one endpoint serves both kinds, and a project-parented "+
			"command cannot express an organization key")
}

func names(c *cobra.Command) []string {
	out := []string{}
	for _, s := range c.Commands() {
		out = append(out, s.Name())
	}
	return out
}

// Every destructive command must reach the confirmation path, which fails
// without a terminal rather than printing "cancelled" and exiting 0.
func TestDestructivePlatformCommandsHaveForce(t *testing.T) {
	org := newOrgCmd().cmd
	project := newProjectCmd().cmd

	for _, c := range []*cobra.Command{
		sub(t, sub(t, org, "api-key"), "delete"),
		sub(t, project, "delete"),
		sub(t, sub(t, project, "custom-domain"), "remove"),
	} {
		t.Run(c.Name(), func(t *testing.T) {
			assert.NotNil(t, c.Flags().Lookup("force"),
				"a destructive command needs --force, or it cannot run without a terminal")
		})
	}
}

// An empty request body succeeds against the API and changes nothing, so these
// have to be refused locally or the command exits 0 having done nothing.
func TestPlatformCommandsRefuseEmptyChanges(t *testing.T) {
	cases := []struct {
		name string
		run  func() error
		want string
	}{
		{"project create without a name",
			func() error { return newProjectCreateCmd().run(newProjectCreateCmd().cmd, nil) }, "--name is required"},
		{"org update without a name",
			func() error { c := newOrgUpdateCmd(); return c.runOrgUpdateCmd(c.cmd, nil) }, "--name is required"},
		{"api-key update without scopes",
			func() error { c := newAPIKeyUpdateCmd(); return c.run(c.cmd, []string{"apk_1"}) }, "--scope is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// The listing endpoint returns the secret like every other field. Rendering it
// would put a live credential into a terminal, a scrollback and any CI log.
func TestKeyListingNeverPrintsTheSecret(t *testing.T) {
	fingerprint := "a1b2c3d4"
	team := "tm_1"
	keys := []hookdeck.APIKey{{
		ID: "apk_1", Label: "CI", Key: "hd_org_SUPERSECRET",
		KeyFingerprint: &fingerprint, TeamID: &team,
	}}

	for _, output := range []string{"", "json"} {
		t.Run("output="+output, func(t *testing.T) {
			out := captureStdout(t, func() { require.NoError(t, renderKeys(keys, output)) })
			assert.NotContains(t, out, "SUPERSECRET", "the listing must never print a key secret")
			assert.Contains(t, out, fingerprint, "the fingerprint is what identifies a key")
		})
	}
}

// The create and roll paths are the one place the secret is shown, and the only
// chance the caller has to copy it.
func TestCreateShowsTheSecretOnceAndSaysSo(t *testing.T) {
	key := &hookdeck.APIKey{ID: "apk_1", Label: "CI", Key: "hd_org_SUPERSECRET"}
	out := captureStdout(t, func() { require.NoError(t, printSecretOnce(key, "")) })
	assert.Contains(t, out, "hd_org_SUPERSECRET")
	assert.True(t, strings.Contains(out, "only time"),
		"the caller has to be told this will not be shown again")
}

// captureStdout runs fn with os.Stdout redirected and returns what it wrote.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	done := make(chan string, 1)
	go func() {
		var b bytes.Buffer
		_, _ = io.Copy(&b, r)
		done <- b.String()
	}()

	fn()
	require.NoError(t, w.Close())
	return <-done
}

// The org auth hint has to survive Execute's error handling.
//
// Execute rewrites any 401 into "your API key is invalid or expired" unless the
// error says it carries its own guidance. The first version of orgAuthError
// returned a plain wrapped error, so the hint was built and then thrown away —
// what a user actually saw was the generic message, which is the opposite of
// what the hint exists to prevent.
func TestOrgAuthErrorIsActionable(t *testing.T) {
	wrapped := orgAuthError(&hookdeck.APIError{StatusCode: 401, Message: "Unauthorized"})

	var actionable *actionableError
	require.True(t, errors.As(wrapped, &actionable),
		"the hint must be marked actionable, or Execute replaces it with the generic 401 text")

	assert.Contains(t, wrapped.Error(), "organization API key")
	assert.Contains(t, wrapped.Error(), "--api-key")

	// Still recognisably a 401 underneath, so nothing else stops matching it.
	assert.True(t, hookdeck.IsUnauthorizedError(wrapped))

	// A non-auth failure is passed through untouched: the hint would be wrong.
	plain := orgAuthError(&hookdeck.APIError{StatusCode: 500, Message: "boom"})
	assert.False(t, errors.As(plain, &actionable))
	assert.NotContains(t, plain.Error(), "organization API key")

	// So is a 403. The credential was accepted; the operation is not permitted
	// for it — "API keys cannot create organization API keys" needs an admin
	// session, not a different key, and the API says so precisely. Adding
	// "wrong kind of credential" on top would contradict it.
	forbidden := orgAuthError(&hookdeck.APIError{
		StatusCode: 403,
		Message:    "API keys cannot create organization API keys",
	})
	assert.False(t, errors.As(forbidden, &actionable),
		"a 403 carries its own accurate reason; the hint must not override it")
	assert.NotContains(t, forbidden.Error(), "wrong kind of credential")
}
