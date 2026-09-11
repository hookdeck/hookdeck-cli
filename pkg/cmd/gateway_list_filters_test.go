package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// captureListQuery runs a gateway list command against a stub API and returns
// the query string it sent.
//
// A filter flag is only worth having if it reaches the API under the key the
// API expects. Asserting the command exits zero proves nothing here: an
// unrecognised query parameter is ignored by the API, so a misspelled key
// silently returns the unfiltered list — the worst possible failure for a
// filter, because the answer looks right.
func captureListQuery(t *testing.T, args ...string) url.Values {
	t.Helper()

	var got url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == hookdeck.APIPathPrefix+"/cli-auth/validate" {
			_ = json.NewEncoder(w).Encode(hookdeck.ValidateAPIKeyResponse{
				ProjectID:   "proj_1",
				ProjectMode: "inbound",
			})
			return
		}

		got = r.URL.Query()
		_, _ = w.Write([]byte(`{"models":[],"pagination":{"limit":100}}`))
	}))
	t.Cleanup(server.Close)

	config.ResetAPIClientForTesting()
	t.Cleanup(config.ResetAPIClientForTesting)

	old := Config
	t.Cleanup(func() { Config = old })
	Config = config.Config{APIBaseURL: server.URL, LogLevel: "info"}
	Config.Profile.APIKey = "sk_test_123456789012"
	Config.Profile.ProjectId = "proj_1"
	Config.Profile.ProjectType = config.ProjectTypeGateway

	root := &cobra.Command{Use: "hookdeck", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(newGatewayCmd().cmd)
	root.SetArgs(args)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	require.NoError(t, root.Execute())
	require.NotNil(t, got, "the command made no list request")
	return got
}

func TestEventListSendsNewFilters(t *testing.T) {
	query := captureListQuery(t,
		"gateway", "event", "list",
		"--search-term", "cus_1234",
		"--delivery-group", "grp_1",
		"--next-attempt-at-after", "2026-06-01T00:00:00Z",
		"--next-attempt-at-before", "2026-06-30T00:00:00Z",
	)

	assert.Equal(t, "cus_1234", query.Get("search_term"))
	assert.Equal(t, "grp_1", query.Get("delivery_group"))
	// The date filters are operator keys, not plain fields: a bare
	// next_attempt_at= would be a different query the API rejects.
	assert.Equal(t, "2026-06-01T00:00:00Z", query.Get("next_attempt_at[gte]"))
	assert.Equal(t, "2026-06-30T00:00:00Z", query.Get("next_attempt_at[lte]"))
	assert.Empty(t, query.Get("next_attempt_at"))
}

func TestRequestListSendsNewFilters(t *testing.T) {
	query := captureListQuery(t,
		"gateway", "request", "list",
		"--search-term", "cus_1234",
		"--events-count", "0",
		"--ignored-count", "2",
		"--cli-events-count", "1",
	)

	assert.Equal(t, "cus_1234", query.Get("search_term"))
	// events_count=0 — requests that produced no events — is the query that
	// explains a "missing" webhook, so the zero has to survive to the wire.
	assert.Equal(t, "0", query.Get("events_count"))
	assert.Equal(t, "2", query.Get("ignored_count"))
	assert.Equal(t, "1", query.Get("cli_events_count"))
}

// Omitted filters must not be sent at all. An empty value is not the same as no
// filter to every API, and sending one narrows or widens the query by accident.
func TestListFiltersAreOmittedWhenUnset(t *testing.T) {
	t.Run("event", func(t *testing.T) {
		query := captureListQuery(t, "gateway", "event", "list")
		for _, key := range []string{
			"search_term", "delivery_group", "next_attempt_at[gte]", "next_attempt_at[lte]",
		} {
			assert.NotContains(t, query, key)
		}
	})

	t.Run("request", func(t *testing.T) {
		query := captureListQuery(t, "gateway", "request", "list")
		for _, key := range []string{
			"search_term", "events_count", "ignored_count", "cli_events_count",
		} {
			assert.NotContains(t, query, key)
		}
	})
}

// The API spec documents further parameters carrying x-docs-hide: Hookdeck
// keeps them out of its public documentation deliberately, so the CLI must not
// offer them either. Read without that context they look like filters we simply
// forgot, which is how they would get added — this pins the omission as
// intentional. The MCP side is pinned by
// TestPluralToolsOmitParametersHiddenFromTheAPIDocs.
func TestListCommandsOmitFlagsHiddenFromTheAPIDocs(t *testing.T) {
	root := &cobra.Command{Use: "hookdeck"}
	root.AddCommand(newGatewayCmd().cmd)

	cases := map[string][]string{
		"event":   {"bulk-retry-id", "include", "progressive", "event-data-id", "cli-user-id"},
		"request": {"bulk-retry-id", "include", "progressive"},
	}

	for resource, hidden := range cases {
		t.Run(resource, func(t *testing.T) {
			cmd, _, err := root.Find([]string{"gateway", resource, "list"})
			require.NoError(t, err)
			require.Equal(t, "list", cmd.Name())

			for _, name := range hidden {
				assert.Nil(t, cmd.Flags().Lookup(name),
					"gateway %s list must not offer --%s: it carries x-docs-hide in the API spec",
					resource, name)
			}
		})
	}
}
