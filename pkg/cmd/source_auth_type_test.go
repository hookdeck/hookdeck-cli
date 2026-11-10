package cmd

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
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
		{name: "STRIPE webhook secret takes no auth_type", sourceType: "STRIPE", flags: sourceConfigFlags{WebhookSecret: "whsec"}},
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

// source update normally omits --type; the API rejects config without type, and WEBHOOK auth
// needs auth_type. applyStoredSourceType fills both from the stored source.
func TestApplyStoredSourceType(t *testing.T) {
	stored := &hookdeck.Source{ID: "src_1", Type: "WEBHOOK"}

	t.Run("config without --type takes the stored type and auth_type", func(t *testing.T) {
		req := &hookdeck.SourceUpdateRequest{}
		config, err := buildSourceConfigFromIndividualFlags(&sourceConfigFlags{APIKey: "KEY"}, "", "")
		require.NoError(t, err)
		require.NotContains(t, config, "auth_type", "type is unknown until the lookup")

		err = applyStoredSourceType(req, config, func() (*hookdeck.Source, error) { return stored, nil })
		require.NoError(t, err)
		assert.Equal(t, "WEBHOOK", req.Type)
		assert.Equal(t, "API_KEY", config["auth_type"])
	})

	t.Run("non-auth config still sends the stored type", func(t *testing.T) {
		req := &hookdeck.SourceUpdateRequest{}
		config := map[string]interface{}{"allowed_http_methods": []string{"POST"}}
		err := applyStoredSourceType(req, config, func() (*hookdeck.Source, error) { return stored, nil })
		require.NoError(t, err)
		assert.Equal(t, "WEBHOOK", req.Type)
		assert.NotContains(t, config, "auth_type")
	})

	t.Run("no config means no lookup", func(t *testing.T) {
		req := &hookdeck.SourceUpdateRequest{Name: "renamed"}
		err := applyStoredSourceType(req, nil, func() (*hookdeck.Source, error) {
			t.Fatal("a name-only update must not spend an API call")
			return nil, nil
		})
		require.NoError(t, err)
		assert.Empty(t, req.Type)
	})

	t.Run("lookup failure is surfaced", func(t *testing.T) {
		err := applyStoredSourceType(&hookdeck.SourceUpdateRequest{}, map[string]interface{}{"auth": map[string]interface{}{"api_key": "k"}},
			func() (*hookdeck.Source, error) { return nil, errors.New("boom") })
		require.Error(t, err)
		assert.Contains(t, err.Error(), "boom")
	})
}
