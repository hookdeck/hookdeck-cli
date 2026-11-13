package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// captureConnectionUpsertBody runs `gateway connection upsert` against a stub
// API that holds stored as the existing connection, and returns the decoded
// PUT /connections body.
//
// The stub answers the PUT whatever it carries, so a passing run says nothing on
// its own: the real API rejects a body with no source or no destination, and
// that is what these tests assert against.
func captureConnectionUpsertBody(t *testing.T, stored *hookdeck.Connection, args ...string) map[string]interface{} {
	t.Helper()

	var body map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == hookdeck.APIPathPrefix+"/cli-auth/validate":
			_ = json.NewEncoder(w).Encode(hookdeck.ValidateAPIKeyResponse{
				ProjectID:   "proj_1",
				ProjectMode: "inbound",
			})
		case r.URL.Path == hookdeck.APIPathPrefix+"/connections" && r.Method == http.MethodGet:
			models := []hookdeck.Connection{}
			if stored != nil && r.URL.Query().Get("name") == *stored.Name {
				models = append(models, *stored)
			}
			_ = json.NewEncoder(w).Encode(hookdeck.ConnectionListResponse{Models: models})
		case r.URL.Path == hookdeck.APIPathPrefix+"/connections" && r.Method == http.MethodPut:
			raw, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(raw, &body))
			_ = json.NewEncoder(w).Encode(stored)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
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

	require.NoError(t, root.Execute())
	require.NotNil(t, body, "the command made no PUT /connections request")
	return body
}

func storedUpsertConnection() *hookdeck.Connection {
	return &hookdeck.Connection{
		ID:   "web_1",
		Name: strPtr("my-conn"),
		Source: &hookdeck.Source{
			ID: "src_1", Name: "my-src", Type: "WEBHOOK",
		},
		Destination: &hookdeck.Destination{
			ID: "des_1", Name: "my-api", Type: "HTTP",
			Config: map[string]interface{}{"url": "https://api.example.com"},
		},
	}
}

// TestConnectionUpsertInlineSourceKeepsStoredDestination is the reported case:
// an inline source (name + type + auth) with no destination flags on an existing
// connection sent no destination at all, and the API answered 422 "value must
// contain at least one of [destination_id, destination]".
func TestConnectionUpsertInlineSourceKeepsStoredDestination(t *testing.T) {
	body := captureConnectionUpsertBody(t, storedUpsertConnection(),
		"gateway", "connection", "upsert", "my-conn",
		"--source-name", "my-src",
		"--source-type", "WEBHOOK",
		"--source-hmac-secret", "s3cret",
	)

	assert.Equal(t, "des_1", body["destination_id"], "the stored destination must be carried over by ID")
	assert.NotContains(t, body, "destination", "an untouched destination is referenced, not rebuilt")

	source, ok := body["source"].(map[string]interface{})
	require.True(t, ok, "the source flags must still be sent inline")
	assert.Equal(t, "my-src", source["name"])
	assert.Equal(t, "WEBHOOK", source["type"])
}

// TestConnectionUpsertInlineDestinationKeepsStoredSource is the same defect from
// the other side: an inline destination with no source flags.
func TestConnectionUpsertInlineDestinationKeepsStoredSource(t *testing.T) {
	body := captureConnectionUpsertBody(t, storedUpsertConnection(),
		"gateway", "connection", "upsert", "my-conn",
		"--destination-name", "my-api",
		"--destination-type", "HTTP",
		"--destination-url", "https://api.example.com/v2",
	)

	assert.Equal(t, "src_1", body["source_id"], "the stored source must be carried over by ID")
	assert.NotContains(t, body, "source", "an untouched source is referenced, not rebuilt")

	destination, ok := body["destination"].(map[string]interface{})
	require.True(t, ok, "the destination flags must still be sent inline")
	assert.Equal(t, "my-api", destination["name"])
}
