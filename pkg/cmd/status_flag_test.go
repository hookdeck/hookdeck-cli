package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// logStatusStub serves any log collection and records the query it was asked
// for.
func logStatusStub(t *testing.T, query *url.Values) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*query = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(hookdeck.EventListResponse{})
	}))
	t.Cleanup(server.Close)
	return server
}

// pointCommandAt aims the process-wide API client singleton at the stub for the
// duration of one test.
func pointCommandAt(t *testing.T, server *httptest.Server) {
	t.Helper()
	old := Config
	t.Cleanup(func() { Config = old })
	config.ResetAPIClientForTesting()
	t.Cleanup(config.ResetAPIClientForTesting)

	Config = config.Config{}
	Config.APIBaseURL = server.URL
	Config.Profile.APIKey = "sk_test_123456789012"
	Config.Profile.ProjectId = "proj_1"
}

// TestCLIStatusIsCanonicalisedPerCommand pins the CLI half of a contract that
// only MCP was honouring.
//
// pkg/hookdeck/status.go exists so both layers can name which vocabulary they
// mean, and only the MCP tools called it: in the same release
// `hookdeck_requests {action:"list", status:"ACCEPTED"}` succeeded while
// `hookdeck gateway request list --status ACCEPTED` came back a 422, because
// the request log's enum is lower case and the API checks the case.
func TestCLIStatusIsCanonicalisedPerCommand(t *testing.T) {
	// run invokes one command with --status set and returns what reached the
	// API, or the error that stopped it.
	type runner struct {
		name string
		path string
		run  func(t *testing.T, value string) error
	}
	runners := []runner{
		{
			name: "gateway request list",
			path: hookdeck.APIPathPrefix + "/requests",
			run: func(t *testing.T, value string) error {
				rc := newRequestListCmd()
				require.NoError(t, rc.cmd.Flags().Set("status", value))
				return rc.runRequestListCmd(rc.cmd, nil)
			},
		},
		{
			name: "gateway event list",
			path: hookdeck.APIPathPrefix + "/events",
			run: func(t *testing.T, value string) error {
				ec := newEventListCmd()
				require.NoError(t, ec.cmd.Flags().Set("status", value))
				return ec.runEventListCmd(ec.cmd, nil)
			},
		},
		{
			name: "gateway request events",
			path: hookdeck.APIPathPrefix + "/requests/req_1/events",
			run: func(t *testing.T, value string) error {
				rc := newRequestEventsCmd()
				require.NoError(t, rc.cmd.Flags().Set("status", value))
				return rc.runRequestEventsCmd(rc.cmd, []string{"req_1"})
			},
		},
	}

	// forwards[i] is keyed by the runner name: the value the user types and the
	// spelling the API has to be sent.
	forwards := map[string][][2]string{
		"gateway request list": {
			{"accepted", "accepted"},
			{"ACCEPTED", "accepted"},
			{"Rejected", "rejected"},
		},
		"gateway event list": {
			{"SUCCESSFUL", "SUCCESSFUL"},
			{"failed", "FAILED"},
			{"Cancelled", "CANCELLED"},
		},
		"gateway request events": {
			{"SUCCESSFUL", "SUCCESSFUL"},
			{"failed", "FAILED"},
		},
	}

	// rejects[i] is a value from the sibling collection's vocabulary, which the
	// API would answer with a 422 naming only the enum it was sent.
	rejects := map[string]struct {
		value     string
		elsewhere string
	}{
		"gateway request list":   {"SUCCESSFUL", "gateway event list"},
		"gateway event list":     {"accepted", "gateway request list"},
		"gateway request events": {"rejected", "gateway request list"},
	}

	for _, r := range runners {
		for _, pair := range forwards[r.name] {
			t.Run(r.name+" sends "+pair[0]+" as "+pair[1], func(t *testing.T) {
				var query url.Values
				server := logStatusStub(t, &query)
				pointCommandAt(t, server)

				require.NoError(t, r.run(t, pair[0]))
				assert.Equal(t, pair[1], query.Get("status"),
					"the API checks the case of its enum, so --status has to be canonicalised")
			})
		}

		t.Run(r.name+" refuses the sibling vocabulary", func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatalf("must not call %s with a status from the other collection", r.URL.Path)
			}))
			t.Cleanup(server.Close)
			pointCommandAt(t, server)

			tt := rejects[r.name]
			err := r.run(t, tt.value)
			require.Error(t, err, "%q belongs to another collection", tt.value)
			assert.Contains(t, err.Error(), tt.value)
			assert.Contains(t, err.Error(), tt.elsewhere,
				"the error should name the command that does take it")
		})
	}
}

// TestStatusFlagUsageNamesTheVocabularyItAccepts keeps --help and the check in
// step. The event commands spelled the enum out by hand and `request list`
// named no vocabulary at all, which is how "--status ACCEPTED" looked like a
// reasonable thing to type.
func TestStatusFlagUsageNamesTheVocabularyItAccepts(t *testing.T) {
	assert.Contains(t, newRequestListCmd().cmd.Flags().Lookup("status").Usage,
		hookdeck.RequestLogStatusValues)
	assert.Contains(t, newEventListCmd().cmd.Flags().Lookup("status").Usage,
		hookdeck.EventStatusValues)
	assert.Contains(t, newRequestEventsCmd().cmd.Flags().Lookup("status").Usage,
		hookdeck.EventStatusValues)
}
