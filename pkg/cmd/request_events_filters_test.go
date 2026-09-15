package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// requestEventsIDFlag is the one `gateway event list` flag this command does
// not offer. `gateway request events` already takes the request ID as its
// argument, so a --id meaning "event IDs" standing right next to it would be
// read as the request's. The API parameter exists; the spelling is what does
// not survive the move.
const requestEventsIDFlag = "id"

// TestRequestEventsOffersTheEventListFilters guards the flag set.
//
// GET /requests/{id}/events declares the same query parameters as GET /events,
// so this command is `gateway event list` narrowed to one request and has to
// offer the same filters under the same names. It used to offer five, so
// "which events of this request failed" had no answer here at all and the
// filters that did exist read as the complete set.
func TestRequestEventsOffersTheEventListFilters(t *testing.T) {
	eventList := newEventListCmd().cmd
	requestEvents := newRequestEventsCmd().cmd

	var missing []string
	eventList.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Name == requestEventsIDFlag {
			return
		}
		got := requestEvents.Flags().Lookup(f.Name)
		if got == nil {
			missing = append(missing, f.Name)
			return
		}
		assert.Equal(t, f.Usage, got.Usage,
			"--%s should read the same in both commands; they are learnt together", f.Name)
	})

	assert.Empty(t, missing,
		"the route honours these on /requests/{id}/events, so the command must offer them")
}

// TestRequestEventsForwardsFiltersToTheAPI is the other half: a flag that never
// reaches the request is worse than no flag, because the unfiltered rows come
// back looking filtered. The renamed parameters are where that hides -
// --connection-id is webhook_id, and the date bounds become bracket keys.
func TestRequestEventsForwardsFiltersToTheAPI(t *testing.T) {
	tests := []struct {
		flag  string
		value string
		param string
		want  string
	}{
		{"source-id", "src_123", "source_id", "src_123"},
		{"connection-id", "web_123", "webhook_id", "web_123"},
		{"destination-id", "des_123", "destination_id", "des_123"},
		{"delivery-group", "dg_123", "delivery_group", "dg_123"},
		{"status", "FAILED", "status", "FAILED"},
		{"attempts", "3", "attempts", "3"},
		{"response-status", "500", "response_status", "500"},
		{"error-code", "TIMEOUT", "error_code", "TIMEOUT"},
		{"cli-id", "cli_123", "cli_id", "cli_123"},
		{"issue-id", "iss_123", "issue_id", "iss_123"},
		{"created-after", "2025-01-01T00:00:00Z", "created_at[gte]", "2025-01-01T00:00:00Z"},
		{"created-before", "2025-02-01T00:00:00Z", "created_at[lte]", "2025-02-01T00:00:00Z"},
		{"successful-at-after", "2025-01-01T00:00:00Z", "successful_at[gte]", "2025-01-01T00:00:00Z"},
		{"successful-at-before", "2025-02-01T00:00:00Z", "successful_at[lte]", "2025-02-01T00:00:00Z"},
		{"last-attempt-at-after", "2025-01-01T00:00:00Z", "last_attempt_at[gte]", "2025-01-01T00:00:00Z"},
		{"last-attempt-at-before", "2025-02-01T00:00:00Z", "last_attempt_at[lte]", "2025-02-01T00:00:00Z"},
		{"headers", `{"x-trace":"1"}`, "headers", `{"x-trace":"1"}`},
		{"body", `{"type":"ping"}`, "body", `{"type":"ping"}`},
		{"path", "/hook", "path", "/hook"},
		{"parsed-query", `{"q":"1"}`, "parsed_query", `{"q":"1"}`},
		{"order-by", "created_at", "order_by", "created_at"},
		{"dir", "asc", "dir", "asc"},
		{"next", "cur_next", "next", "cur_next"},
		{"prev", "cur_prev", "prev", "cur_prev"},
	}

	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			var query url.Values
			var path string
			server := requestEventsStub(t, &path, &query)

			rc := newRequestEventsCmd()
			require.NoError(t, rc.cmd.Flags().Set(tt.flag, tt.value))
			require.NoError(t, runRequestEventsAgainst(t, server, rc, "req_1"))

			require.Equal(t, hookdeck.APIPathPrefix+"/requests/req_1/events", path)
			assert.Equal(t, tt.want, query.Get(tt.param),
				"--%s must reach the API as %s", tt.flag, tt.param)
		})
	}
}

// requestEventsStub serves the events sub-resource and records what it was
// asked for.
func requestEventsStub(t *testing.T, path *string, query *url.Values) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*path = r.URL.Path
		q := r.URL.Query()
		*query = q
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(hookdeck.EventListResponse{})
	}))
	t.Cleanup(server.Close)
	return server
}

// runRequestEventsAgainst points the command's client at the stub. The API
// client is a process-wide singleton, so it has to be reset around each run.
func runRequestEventsAgainst(t *testing.T, server *httptest.Server, rc *requestEventsCmd, requestID string) error {
	t.Helper()
	old := Config
	t.Cleanup(func() { Config = old })
	config.ResetAPIClientForTesting()
	t.Cleanup(config.ResetAPIClientForTesting)

	Config = config.Config{}
	Config.APIBaseURL = server.URL
	Config.Profile.APIKey = "sk_test_123456789012"
	Config.Profile.ProjectId = "proj_1"

	return rc.runRequestEventsCmd(rc.cmd, []string{requestID})
}
