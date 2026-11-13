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

// These run the real commands against a stub API and assert on the request body sent, so they
// pin each command's wiring - which type the config is built for, what is merged from the
// stored resource - not just the shared builders.

type recordedRequest struct {
	Method string
	Path   string
	Body   map[string]interface{}
}

// stubStatus makes runGatewayAgainstStub answer with a non-200 status.
type stubStatus struct {
	Code int
	Body interface{}
}

// runGatewayAgainstStub runs `hookdeck gateway <args>` against a stub API. respond returns the
// JSON body for a request (nil means `{}`, a stubStatus sets the status code). Every request
// except the credential check is recorded and returned.
func runGatewayAgainstStub(t *testing.T, respond func(r *http.Request) interface{}, args ...string) ([]recordedRequest, error) {
	t.Helper()

	var requests []recordedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == hookdeck.APIPathPrefix+"/cli-auth/validate" {
			_ = json.NewEncoder(w).Encode(hookdeck.ValidateAPIKeyResponse{ProjectID: "proj_1", ProjectMode: "inbound"})
			return
		}
		rec := recordedRequest{Method: r.Method, Path: r.URL.Path}
		if data, _ := io.ReadAll(r.Body); len(data) > 0 {
			require.NoError(t, json.Unmarshal(data, &rec.Body))
		}
		requests = append(requests, rec)

		body := respond(r)
		if st, ok := body.(stubStatus); ok {
			w.WriteHeader(st.Code)
			body = st.Body
		}
		if body == nil {
			body = map[string]interface{}{}
		}
		_ = json.NewEncoder(w).Encode(body)
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
	root.SetArgs(append([]string{"gateway"}, args...))
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	err := root.Execute()
	return requests, err
}

func findRequest(t *testing.T, requests []recordedRequest, method, path string) recordedRequest {
	t.Helper()
	for _, r := range requests {
		if r.Method == method && r.Path == path {
			return r
		}
	}
	require.Failf(t, "request not sent", "%s %s; sent: %v", method, path, requests)
	return recordedRequest{}
}

func bodyMap(t *testing.T, v interface{}, key string) map[string]interface{} {
	t.Helper()
	m, ok := v.(map[string]interface{})
	require.True(t, ok, "expected an object, got %T", v)
	inner, ok := m[key].(map[string]interface{})
	require.True(t, ok, "expected %q to be an object in %v", key, m)
	return inner
}

// `source upsert` without --type: the API upserts as WEBHOOK, so auth must carry auth_type.
func TestSourceUpsertWithoutTypeSendsAuthType(t *testing.T) {
	requests, err := runGatewayAgainstStub(t,
		func(r *http.Request) interface{} {
			if r.Method == http.MethodGet { // lookup by name
				return map[string]interface{}{"models": []interface{}{}}
			}
			return map[string]interface{}{"id": "src_1", "name": "s", "type": "WEBHOOK"}
		},
		"source", "upsert", "s", "--api-key", "KEY",
	)
	require.NoError(t, err)

	put := findRequest(t, requests, http.MethodPut, hookdeck.APIPathPrefix+"/sources")
	cfg := bodyMap(t, put.Body, "config")
	assert.Equal(t, "API_KEY", cfg["auth_type"])
	assert.Equal(t, map[string]interface{}{"header_key": "x-api-key", "api_key": "KEY"}, cfg["auth"])
}

// `source update <id>` without --type: config needs a type beside it, and the auth is shaped and
// typed for the stored type.
func TestSourceUpdateWithoutTypeUsesStoredType(t *testing.T) {
	tests := []struct {
		name       string
		storedType string
		args       []string
		wantConfig map[string]interface{}
	}{
		{
			name:       "WEBHOOK api key",
			storedType: "WEBHOOK",
			args:       []string{"--api-key", "KEY"},
			wantConfig: map[string]interface{}{
				"auth_type": "API_KEY",
				"auth":      map[string]interface{}{"header_key": "x-api-key", "api_key": "KEY"},
			},
		},
		{
			name:       "STRIPE webhook secret is Stripe-shaped, no auth_type",
			storedType: "STRIPE",
			args:       []string{"--webhook-secret", "whsec_1"},
			wantConfig: map[string]interface{}{
				"auth": map[string]interface{}{"webhook_secret_key": "whsec_1"},
			},
		},
		{
			name:       "non-auth config still sends the stored type",
			storedType: "WEBHOOK",
			args:       []string{"--allowed-http-methods", "POST"},
			wantConfig: map[string]interface{}{"allowed_http_methods": []interface{}{"POST"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stored := map[string]interface{}{"id": "src_1", "name": "s", "type": tt.storedType}
			requests, err := runGatewayAgainstStub(t,
				func(r *http.Request) interface{} { return stored },
				append([]string{"source", "update", "src_1"}, tt.args...)...,
			)
			require.NoError(t, err)

			put := findRequest(t, requests, http.MethodPut, hookdeck.APIPathPrefix+"/sources/src_1")
			assert.Equal(t, tt.storedType, put.Body["type"])
			assert.Equal(t, tt.wantConfig, put.Body["config"])
		})
	}
}

func TestSourceUpdateNameOnlyMakesNoLookup(t *testing.T) {
	requests, err := runGatewayAgainstStub(t,
		func(r *http.Request) interface{} {
			return map[string]interface{}{"id": "src_1", "name": "n", "type": "WEBHOOK"}
		},
		"source", "update", "src_1", "--name", "n",
	)
	require.NoError(t, err)
	require.Len(t, requests, 1, "a name-only update needs no type and no GET")
	assert.NotContains(t, requests[0].Body, "type")
}

// existingConnection is a connection whose WEBHOOK source verifies with HMAC. The API returns
// the method as config.auth_type but not the credentials.
func existingConnection(sourceConfig map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"models": []interface{}{map[string]interface{}{
			"id":   "web_1",
			"name": "conn",
			"source": map[string]interface{}{
				"id": "src_1", "name": "src", "type": "WEBHOOK", "config": sourceConfig,
			},
			"destination": map[string]interface{}{
				"id": "des_1", "name": "dst", "type": "MOCK_API", "config": map[string]interface{}{},
			},
		}},
	}
}

// `connection upsert conn --source-*` on an existing connection merges the flags into the stored
// source. The stored auth_type must not survive beside new auth, nor be sent without auth.
func TestConnectionUpsertPartialSourceUpdate(t *testing.T) {
	stored := map[string]interface{}{"auth_type": "HMAC", "allowed_http_methods": []interface{}{"POST"}}

	tests := []struct {
		name       string
		args       []string
		wantConfig map[string]interface{}
	}{
		{
			name: "new api key replaces the stored method",
			args: []string{"--source-api-key", "NEWKEY"},
			wantConfig: map[string]interface{}{
				"auth_type":            "API_KEY",
				"auth":                 map[string]interface{}{"header_key": "x-api-key", "api_key": "NEWKEY"},
				"allowed_http_methods": []interface{}{"POST"},
			},
		},
		{
			// No method fits {api_key} alone, so nothing is inferred; the stored HMAC must not
			// be sent as the method for these credentials either.
			name: "auth that fits no method does not inherit the stored method",
			args: []string{"--source-config", `{"auth":{"api_key":"NEWKEY"}}`},
			wantConfig: map[string]interface{}{
				"auth":                 map[string]interface{}{"api_key": "NEWKEY"},
				"allowed_http_methods": []interface{}{"POST"},
			},
		},
		{
			name: "non-auth change drops the credential-less auth_type",
			args: []string{"--source-allowed-http-methods", "POST,PUT"},
			wantConfig: map[string]interface{}{
				"allowed_http_methods": []interface{}{"POST", "PUT"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests, err := runGatewayAgainstStub(t,
				func(r *http.Request) interface{} {
					if r.Method == http.MethodGet {
						return existingConnection(stored)
					}
					return existingConnection(stored)["models"].([]interface{})[0]
				},
				append([]string{"connection", "upsert", "conn"}, tt.args...)...,
			)
			require.NoError(t, err)

			put := findRequest(t, requests, http.MethodPut, hookdeck.APIPathPrefix+"/connections")
			source := bodyMap(t, put.Body, "source")
			assert.Equal(t, "WEBHOOK", source["type"])
			assert.Equal(t, tt.wantConfig, source["config"])
		})
	}
	assert.Equal(t, "HMAC", stored["auth_type"], "the stored config is not mutated")
}

// A new connection with an inline source and no --source-type is sent as a WEBHOOK source.
func TestConnectionUpsertNewInlineSourceWithoutType(t *testing.T) {
	requests, err := runGatewayAgainstStub(t,
		func(r *http.Request) interface{} {
			if r.Method == http.MethodGet {
				return map[string]interface{}{"models": []interface{}{}}
			}
			return existingConnection(nil)["models"].([]interface{})[0]
		},
		"connection", "upsert", "conn",
		"--source-name", "src", "--source-api-key", "KEY",
		"--destination-name", "dst", "--destination-type", "MOCK_API",
	)
	require.NoError(t, err)

	put := findRequest(t, requests, http.MethodPut, hookdeck.APIPathPrefix+"/connections")
	source := bodyMap(t, put.Body, "source")
	assert.Equal(t, "WEBHOOK", source["type"], "the API rejects an empty source.type")
	assert.Equal(t, "API_KEY", bodyMap(t, source, "config")["auth_type"])
}
