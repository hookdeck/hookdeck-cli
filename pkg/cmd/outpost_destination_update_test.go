package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
)

// webhookSchema is a type whose config has a required field, which is the shape
// that made partial updates impossible.
func webhookSchema() []map[string]any {
	return []map[string]any{{
		"type":  "webhook",
		"label": "Webhook",
		"config_fields": []map[string]any{
			{"key": "url", "type": "text", "label": "URL", "required": true},
			{"key": "timeout", "type": "text", "label": "Timeout"},
		},
		"credential_fields": []map[string]any{
			{"key": "secret", "type": "text", "label": "Secret"},
		},
	}}
}

// updateHarness points the CLI at a mock Outpost API and returns the body of
// the PATCH it receives, if any.
func updateHarness(t *testing.T) *struct {
	patched   bool
	patchBody map[string]any
} {
	t.Helper()

	got := &struct {
		patched   bool
		patchBody map[string]any
	}{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/2025-07-01/destination-types":
			_ = json.NewEncoder(w).Encode(webhookSchema())
		case r.Method == http.MethodGet && r.URL.Path == "/2025-07-01/tenants/acme/destinations/des_1":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "des_1", "type": "webhook"})
		case r.Method == http.MethodPatch && r.URL.Path == "/2025-07-01/tenants/acme/destinations/des_1":
			got.patched = true
			_ = json.NewDecoder(r.Body).Decode(&got.patchBody)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "des_1", "type": "webhook"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	// The schema cache is keyed on host and project and lives under the temp
	// dir, so a private temp dir guarantees this test fetches from its own mock.
	t.Setenv("TMPDIR", t.TempDir())

	previous := Config
	t.Cleanup(func() {
		Config = previous
		config.ResetAPIClientForTesting()
	})

	Config = config.Config{OutpostAPIBaseURL: server.URL, APIBaseURL: server.URL}
	Config.Profile.APIKey = "sk_test"
	Config.Profile.ProjectId = "proj_1"
	config.ResetAPIClientForTesting()

	return got
}

// Update is a merge patch, so create-time required-field rules do not apply to
// it. Enforcing them made credential rotation impossible: the command failed
// with "--config url=<value> is required" before any request was sent, while
// the MCP path, which does not validate, worked.
func TestOutpostDestinationUpdateAcceptsAPartialChange(t *testing.T) {
	t.Run("rotating a credential does not require the config", func(t *testing.T) {
		got := updateHarness(t)

		dc := newOutpostDestinationUpdateCmd(&outpostDestinationCmd{tenantID: "acme"})
		dc.fields.credential = []string{"secret=new"}

		require.NoError(t, dc.runOutpostDestinationUpdateCmd(dc.cmd, []string{"des_1"}))
		require.True(t, got.patched, "the update must reach the API")

		assert.Equal(t, map[string]any{"secret": "new"}, got.patchBody["credentials"])
		// Unchanged fields must be absent rather than sent empty.
		assert.NotContains(t, got.patchBody, "config")
		assert.NotContains(t, got.patchBody, "topics")
	})

	t.Run("changing one optional config field does not require the rest", func(t *testing.T) {
		got := updateHarness(t)

		dc := newOutpostDestinationUpdateCmd(&outpostDestinationCmd{tenantID: "acme"})
		dc.fields.config = []string{"timeout=30"}

		require.NoError(t, dc.runOutpostDestinationUpdateCmd(dc.cmd, []string{"des_1"}))
		require.True(t, got.patched)
		assert.Equal(t, map[string]any{"timeout": "30"}, got.patchBody["config"])
	})

	t.Run("the required field is still accepted when supplied", func(t *testing.T) {
		got := updateHarness(t)

		dc := newOutpostDestinationUpdateCmd(&outpostDestinationCmd{tenantID: "acme"})
		dc.fields.config = []string{"url=https://example.com/new"}

		require.NoError(t, dc.runOutpostDestinationUpdateCmd(dc.cmd, []string{"des_1"}))
		require.True(t, got.patched)
		assert.Equal(t, map[string]any{"url": "https://example.com/new"}, got.patchBody["config"])
	})
}

// Dropping the required-field rule must not drop the rest: a misspelled key is
// still a mistake, and the API's message is less useful than this one.
func TestOutpostDestinationUpdateStillRejectsAnUnknownKey(t *testing.T) {
	got := updateHarness(t)

	dc := newOutpostDestinationUpdateCmd(&outpostDestinationCmd{tenantID: "acme"})
	dc.fields.config = []string{"ur1=https://example.com"}

	err := dc.runOutpostDestinationUpdateCmd(dc.cmd, []string{"des_1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"ur1" is not a valid config field`)
	assert.False(t, got.patched, "a request known to be wrong should not be sent")
}

// Clearing a required field is not a partial update, so an explicitly empty
// value is still reported.
func TestOutpostDestinationUpdateRejectsAnEmptyRequiredValue(t *testing.T) {
	got := updateHarness(t)

	dc := newOutpostDestinationUpdateCmd(&outpostDestinationCmd{tenantID: "acme"})
	dc.fields.config = []string{"url=  "}

	err := dc.runOutpostDestinationUpdateCmd(dc.cmd, []string{"des_1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--config url=<value> is required")
	assert.False(t, got.patched)
}
