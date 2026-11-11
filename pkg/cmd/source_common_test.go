package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookSecretShorthandDefaults(t *testing.T) {
	t.Run("provider types send only the secret so API defaults apply", func(t *testing.T) {
		for _, typ := range []string{"VAPI", "vapi", "STRIPE", "GITHUB"} {
			cfg, err := buildSourceConfigFromIndividualFlags(&sourceConfigFlags{WebhookSecret: "s3cret"}, "", typ)
			require.NoError(t, err)
			assert.Equal(t, map[string]interface{}{"webhook_secret_key": "s3cret"}, cfg["auth"], typ)
		}
	})
	t.Run("generic types keep conventional HMAC defaults", func(t *testing.T) {
		for _, typ := range []string{"WEBHOOK", "HTTP", ""} {
			cfg, err := buildSourceConfigFromIndividualFlags(&sourceConfigFlags{WebhookSecret: "s3cret"}, "", typ)
			require.NoError(t, err)
			assert.Equal(t, map[string]interface{}{
				"algorithm": "sha256", "encoding": "hex",
				"header_key": "x-webhook-signature", "webhook_secret_key": "s3cret",
			}, cfg["auth"], typ)
		}
	})
	t.Run("legacy flat webhook_secret in JSON follows the same rule", func(t *testing.T) {
		cfg, err := buildSourceConfigFromFlags(`{"webhook_secret":"s3cret"}`, "", nil, "VAPI")
		require.NoError(t, err)
		assert.Equal(t, map[string]interface{}{"webhook_secret_key": "s3cret"}, cfg["auth"])
		assert.NotContains(t, cfg, "webhook_secret")
	})
}

func TestValidateSourceConfigAuthShape(t *testing.T) {
	t.Run("flat VAPI auth keys are rejected with corrected JSON, secret redacted", func(t *testing.T) {
		_, err := buildSourceConfigFromFlags(
			`{"webhook_secret_key":"topsecret","algorithm":"sha256","encoding":"hex","header_key":"x-signature","trim_payload":false}`,
			"", nil, "VAPI")
		require.Error(t, err)
		msg := err.Error()
		for _, k := range []string{"webhook_secret_key", "algorithm", "encoding", "header_key", "trim_payload"} {
			assert.Contains(t, msg, k)
		}
		assert.Contains(t, msg, `"auth": {`)
		assert.Contains(t, msg, "<webhook_secret_key>")
		assert.NotContains(t, msg, "topsecret")
	})
	t.Run("correct shape passes", func(t *testing.T) {
		cfg, err := buildSourceConfigFromFlags(`{"auth":{"webhook_secret_key":"s","header_key":"x-signature"}}`, "", nil, "VAPI")
		require.NoError(t, err)
		assert.Contains(t, cfg, "auth")
	})
	t.Run("misplaced keys alongside an existing auth object are merged in the suggestion", func(t *testing.T) {
		err := validateSourceConfigAuthShape(map[string]interface{}{
			"auth":      map[string]interface{}{"webhook_secret_key": "s"},
			"algorithm": "sha1",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "algorithm")
		assert.True(t, strings.Contains(err.Error(), `"auth"`))
	})
	t.Run("non-auth keys are untouched", func(t *testing.T) {
		assert.NoError(t, validateSourceConfigAuthShape(map[string]interface{}{
			"allowed_http_methods": []interface{}{"POST"},
		}))
	})
	t.Run("connection --source-config path rejects flat keys too", func(t *testing.T) {
		cc := &connectionCreateCmd{}
		cc.sourceType = "VAPI"
		cc.SourceConfig = `{"webhook_secret_key":"s","header_key":"x-signature"}`
		_, err := cc.buildSourceConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "header_key")
	})
}
