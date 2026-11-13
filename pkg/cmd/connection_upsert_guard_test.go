package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// stubCall is one call the stub API received.
type stubCall struct {
	Method string
	Path   string
	Body   map[string]interface{}
}

// stubConnectionAPI records every request, so a test can assert which writes
// did or did not happen.
type stubConnectionAPI struct {
	mu       sync.Mutex
	requests []stubCall
}

// writes returns the recorded requests that are not GETs.
func (s *stubConnectionAPI) writes() []stubCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []stubCall
	for _, r := range s.requests {
		if r.Method != http.MethodGet {
			out = append(out, r)
		}
	}
	return out
}

// runConnectionCommand runs a gateway command against a stub whose
// GET /connections returns listed whatever the name filter says (standing in
// for a loose filter) and whose GET /connections/{id} returns each listed
// connection by ID. Writes are answered with the first listed connection. It
// returns the stub and the command's error.
func runConnectionCommand(t *testing.T, listed []hookdeck.Connection, args ...string) (*stubConnectionAPI, error) {
	t.Helper()

	stub := &stubConnectionAPI{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		rec := stubCall{Method: r.Method, Path: r.URL.Path}
		if r.Body != nil && r.Method != http.MethodGet {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &rec.Body)
		}
		stub.mu.Lock()
		stub.requests = append(stub.requests, rec)
		stub.mu.Unlock()

		switch {
		case r.URL.Path == hookdeck.APIPathPrefix+"/cli-auth/validate":
			_ = json.NewEncoder(w).Encode(hookdeck.ValidateAPIKeyResponse{ProjectID: "proj_1", ProjectMode: "inbound"})
		case r.URL.Path == hookdeck.APIPathPrefix+"/connections" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(hookdeck.ConnectionListResponse{Models: listed})
		case r.Method == http.MethodGet:
			for _, c := range listed {
				if r.URL.Path == hookdeck.APIPathPrefix+"/connections/"+c.ID {
					_ = json.NewEncoder(w).Encode(c)
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"not found"}`))
		default:
			if len(listed) > 0 {
				_ = json.NewEncoder(w).Encode(listed[0])
			} else {
				_, _ = w.Write([]byte(`{"id":"web_new"}`))
			}
		}
	}))
	t.Cleanup(server.Close)

	config.ResetAPIClientForTesting()
	t.Cleanup(config.ResetAPIClientForTesting)

	old := Config
	t.Cleanup(func() { Config = old })
	Config = config.Config{APIBaseURL: server.URL, LogLevel: "info"}
	Config.Profile.APIKey = "sk_test_123456789012"
	Config.Profile.ProjectId = "proj_1"
	Config.Profile.ProjectType = config.ProjectTypeEventGateway

	root := &cobra.Command{Use: "hookdeck", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(newGatewayCmd().cmd)
	root.SetArgs(args)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	return stub, root.Execute()
}

// TestConnectionUpsertIgnoresLooseNameMatch: the stub answers the name filter
// with a different connection. Taking it unchecked made upsert carry that
// connection's source and destination over to the one being upserted.
func TestConnectionUpsertIgnoresLooseNameMatch(t *testing.T) {
	loose := storedUpsertConnection()
	loose.Name = strPtr("my-conn-extra")

	stub, err := runConnectionCommand(t, []hookdeck.Connection{*loose},
		"gateway", "connection", "upsert", "my-conn", "--description", "d")
	require.NoError(t, err)

	puts := stub.writes()
	require.Len(t, puts, 1)
	require.NotContains(t, puts[0].Body, "source_id", "the loose match's source must not be carried over")
	require.NotContains(t, puts[0].Body, "destination_id", "the loose match's destination must not be carried over")
}
