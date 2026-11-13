package cmd

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/cmd/sources"
)

// The API validates config.auth on WEBHOOK and HTTP sources against the method named in
// config.auth_type, and rejects auth without it ("config.auth is not allowed"). These tests
// pin that every way of building source auth adds the matching auth_type for those types,
// and leaves provider types (which take no auth_type) alone.

func TestBuildSourceConfigFromIndividualFlagsSetsAuthType(t *testing.T) {
	tests := []struct {
		name         string
		sourceType   string
		flags        sourceConfigFlags
		wantAuthType string // "" means auth_type must be absent
	}{
		{name: "WEBHOOK api key", sourceType: "WEBHOOK", flags: sourceConfigFlags{APIKey: "k"}, wantAuthType: "API_KEY"},
		{name: "webhook lowercase api key", sourceType: "webhook", flags: sourceConfigFlags{APIKey: "k"}, wantAuthType: "API_KEY"},
		{name: "WEBHOOK basic auth", sourceType: "WEBHOOK", flags: sourceConfigFlags{BasicAuthUser: "u", BasicAuthPass: "p"}, wantAuthType: "BASIC_AUTH"},
		{name: "WEBHOOK hmac", sourceType: "WEBHOOK", flags: sourceConfigFlags{HMACSecret: "s", HMACAlgo: "sha512"}, wantAuthType: "HMAC"},
		{name: "WEBHOOK webhook secret", sourceType: "WEBHOOK", flags: sourceConfigFlags{WebhookSecret: "s"}, wantAuthType: "HMAC"},
		{name: "HTTP api key", sourceType: "HTTP", flags: sourceConfigFlags{APIKey: "k"}, wantAuthType: "API_KEY"},
		{name: "HTTP basic auth", sourceType: "HTTP", flags: sourceConfigFlags{BasicAuthUser: "u", BasicAuthPass: "p"}, wantAuthType: "BASIC_AUTH"},
		{name: "AZURE_EVENT_GRID basic auth", sourceType: "AZURE_EVENT_GRID", flags: sourceConfigFlags{BasicAuthUser: "u", BasicAuthPass: "p"}, wantAuthType: "BASIC_AUTH"},
		{name: "PROMETHEUS_ALERTMANAGER basic auth", sourceType: "PROMETHEUS_ALERTMANAGER", flags: sourceConfigFlags{BasicAuthUser: "u", BasicAuthPass: "p"}, wantAuthType: "BASIC_AUTH"},
		// --api-key sends api_key only to Prometheus, whose bearer method takes no header_key.
		{name: "PROMETHEUS_ALERTMANAGER api key", sourceType: "PROMETHEUS_ALERTMANAGER", flags: sourceConfigFlags{APIKey: "k"}, wantAuthType: "PROMETHEUS_ALERTMANAGER"},
		{name: "STRIPE webhook secret takes no auth_type", sourceType: "STRIPE", flags: sourceConfigFlags{WebhookSecret: "whsec"}},
		{name: "empty type infers nothing", sourceType: "", flags: sourceConfigFlags{APIKey: "k"}},
		{name: "WEBHOOK without auth flags", sourceType: "WEBHOOK", flags: sourceConfigFlags{AllowedHTTPMethods: "POST"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := buildSourceConfigFromIndividualFlags(&tt.flags, "", tt.sourceType)
			require.NoError(t, err)
			require.NotNil(t, config)
			if tt.wantAuthType == "" {
				assert.NotContains(t, config, "auth_type")
				return
			}
			assert.Equal(t, tt.wantAuthType, config["auth_type"])
			assert.NotNil(t, config["auth"])
		})
	}
}

// The runbook case: connection upsert/create with an inline WEBHOOK source and --source-api-key.
func TestConnectionBuildSourceConfigWebhookAPIKey(t *testing.T) {
	cc := &connectionCreateCmd{sourceType: "WEBHOOK"}
	cc.SourceAPIKey = "KEY"

	config, err := cc.buildSourceConfig()
	require.NoError(t, err)

	assert.Equal(t, map[string]interface{}{
		"auth_type": "API_KEY",
		"auth": map[string]interface{}{
			"header_key": "x-api-key",
			"api_key":    "KEY",
		},
	}, config)
}

func TestConnectionBuildSourceConfigJSONWebhookAddsAuthType(t *testing.T) {
	cc := &connectionCreateCmd{sourceType: "WEBHOOK"}
	cc.SourceConfig = `{"auth":{"header_key":"x-api-key","api_key":"KEY"}}`

	config, err := cc.buildSourceConfig()
	require.NoError(t, err)
	assert.Equal(t, "API_KEY", config["auth_type"])
}

func TestBuildSourceConfigFromFlagsJSONAuthType(t *testing.T) {
	tests := []struct {
		name         string
		sourceType   string
		input        string
		wantAuthType interface{} // nil means auth_type must be absent
	}{
		{
			name:         "explicit auth_type is kept",
			sourceType:   "WEBHOOK",
			input:        `{"auth_type":"API_KEY","auth":{"header_key":"x-api-key","api_key":"KEY"}}`,
			wantAuthType: "API_KEY",
		},
		{
			name:         "auth without auth_type gets one",
			sourceType:   "WEBHOOK",
			input:        `{"auth":{"username":"u","password":"p"}}`,
			wantAuthType: "BASIC_AUTH",
		},
		{
			name:         "legacy flat api_key is normalized and typed",
			sourceType:   "WEBHOOK",
			input:        `{"api_key":"KEY"}`,
			wantAuthType: "API_KEY",
		},
		{
			name:         "hmac-shaped secret is HMAC",
			sourceType:   "HTTP",
			input:        `{"auth":{"algorithm":"sha256","encoding":"hex","header_key":"x-sig","webhook_secret_key":"s"}}`,
			wantAuthType: "HMAC",
		},
		{
			// HMAC requires algorithm/encoding/header_key, so a bare secret can only be the
			// Standard Webhooks method on the API.
			name:         "bare secret is STANDARD_WEBHOOKS",
			sourceType:   "WEBHOOK",
			input:        `{"auth":{"webhook_secret_key":"whsec_abc"}}`,
			wantAuthType: "STANDARD_WEBHOOKS",
		},
		{
			name:         "PROMETHEUS_ALERTMANAGER bearer credentials",
			sourceType:   "PROMETHEUS_ALERTMANAGER",
			input:        `{"auth":{"api_key":"token"}}`,
			wantAuthType: "PROMETHEUS_ALERTMANAGER",
		},
		{
			name:       "provider type is left alone",
			sourceType: "GITHUB",
			input:      `{"auth":{"webhook_secret_key":"s"}}`,
		},
		{
			name:       "no auth means no auth_type",
			sourceType: "WEBHOOK",
			input:      `{"allowed_http_methods":["POST"]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := buildSourceConfigFromFlags(tt.input, "", nil, tt.sourceType)
			require.NoError(t, err)
			if tt.wantAuthType == nil {
				assert.NotContains(t, config, "auth_type")
				return
			}
			assert.Equal(t, tt.wantAuthType, config["auth_type"])
		})
	}
}

// The connection's inline source defaults to WEBHOOK, the type the API applies to a new source
// created without one, so --source-api-key works without --source-type.
func TestConnectionBuildSourceConfigOmittedTypeIsWebhook(t *testing.T) {
	cc := &connectionCreateCmd{}
	cc.SourceAPIKey = "KEY"

	config, err := cc.buildSourceConfig()
	require.NoError(t, err)
	assert.Equal(t, "API_KEY", config["auth_type"])
}

// Without the spec (offline, or a spec-shape change), WEBHOOK and HTTP still get auth_type from
// the built-in methods; other types are left for the API to validate.
func TestEnsureSourceConfigAuthTypeWithoutSpec(t *testing.T) {
	original := loadSourceAuthMethods
	t.Cleanup(func() { loadSourceAuthMethods = original })
	loadSourceAuthMethods = func() (map[string][]sources.AuthMethod, error) {
		return nil, errors.New("offline")
	}

	for sourceType, want := range map[string]interface{}{"WEBHOOK": "API_KEY", "http": "API_KEY", "ORDINAL": nil} {
		config, err := buildSourceConfigFromIndividualFlags(&sourceConfigFlags{APIKey: "k"}, "", sourceType)
		require.NoError(t, err)
		if want == nil {
			assert.NotContains(t, config, "auth_type", sourceType)
		} else {
			assert.Equal(t, want, config["auth_type"], sourceType)
		}
	}
	config, err := buildSourceConfigFromFlags(`{"auth":{"webhook_secret_key":"whsec_x"}}`, "", nil, "WEBHOOK")
	require.NoError(t, err)
	assert.Equal(t, "STANDARD_WEBHOOKS", config["auth_type"])
}
