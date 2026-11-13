package cmd

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/cmd/sources"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// validateSourceAuthFromSpec reads the fixture cut from the real spec (see TestMain); the
// sources package tests pin what the parse makes of each type used here.

func TestValidateSourceAuthFromSpec(t *testing.T) {
	tests := []struct {
		name       string
		sourceType string
		configSet  bool
		auth       sourceAuthFlags
		prefix     string
		wantErr    string
	}{
		// Verification required, no auth flag.
		{name: "required secret missing", sourceType: "AIRCALL",
			wantErr: "--webhook-secret is required for source type AIRCALL"},
		{name: "connection flags are prefixed", sourceType: "AIRCALL", prefix: "source-",
			wantErr: "--source-webhook-secret is required for source type AIRCALL"},
		{name: "type in any case", sourceType: "aircall",
			wantErr: "--webhook-secret is required for source type aircall"},
		{name: "required api key missing", sourceType: "CLOUDSIGNAL",
			wantErr: "--api-key is required for source type CLOUDSIGNAL"},
		{name: "required auth with only optional fields", sourceType: "VERCEL_LOG_DRAINS",
			wantErr: "--webhook-secret is required for source type VERCEL_LOG_DRAINS"},
		{name: "required field no flag sets", sourceType: "ALIPAY",
			wantErr: `source type ALIPAY requires config.auth public_key, which no flag sets: pass them with --config '{"auth":{"public_key":"..."}}' or --config-file`},
		{name: "required fields no flag sets, connection", sourceType: "TIKTOK_SHOP", prefix: "source-",
			wantErr: `source type TIKTOK_SHOP requires config.auth app_key, webhook_secret_key, which no flag sets: pass them with --source-config '{"auth":{"app_key":"...","webhook_secret_key":"..."}}' or --source-config-file`},

		// Verification required, satisfied.
		{name: "webhook secret", sourceType: "AIRCALL", auth: sourceAuthFlags{WebhookSecret: "s"}},
		{name: "hmac secret sets the same field", sourceType: "AIRCALL", auth: sourceAuthFlags{HMACSecret: "s"}},
		{name: "api key", sourceType: "CLOUDSIGNAL", auth: sourceAuthFlags{APIKey: "k"}},
		{name: "config set by hand", sourceType: "ALIPAY", configSet: true},

		// An auth flag that does not set a required field.
		{name: "wrong flag for optional auth", sourceType: "STRIPE", auth: sourceAuthFlags{APIKey: "k"},
			wantErr: "--api-key does not set config.auth webhook_secret_key, which source type STRIPE requires: use --webhook-secret"},
		{name: "wrong flag points to basic auth", sourceType: "POSTMARK", auth: sourceAuthFlags{WebhookSecret: "s"},
			wantErr: "--webhook-secret does not set config.auth password, username, which source type POSTMARK requires: use --basic-auth-user and --basic-auth-pass"},
		{name: "flag sets only some required fields", sourceType: "TIKTOK_SHOP", auth: sourceAuthFlags{WebhookSecret: "s"},
			wantErr: `--webhook-secret does not set config.auth app_key, which source type TIKTOK_SHOP requires: pass them with --config '{"auth":{"app_key":"...","webhook_secret_key":"..."}}' or --config-file`},
		{name: "wrong flag on a handshake type", sourceType: "ONFLEET", auth: sourceAuthFlags{APIKey: "k"}, prefix: "source-",
			wantErr: "--source-api-key does not set config.auth webhook_secret_key, which source type ONFLEET requires: use --source-webhook-secret"},

		// The header name is the provider's contract and no flag sets it.
		{name: "api key for a type with a user-defined header", sourceType: "ACUMATICA", auth: sourceAuthFlags{APIKey: "k"},
			wantErr: `--api-key does not set config.auth header_key, which source type ACUMATICA requires: pass them with --config '{"auth":{"header_key":"...","api_key":"..."}}' or --config-file`},

		// Not rejected.
		{name: "optional auth omitted", sourceType: "STRIPE"},
		{name: "optional auth given", sourceType: "STRIPE", auth: sourceAuthFlags{WebhookSecret: "whsec"}},
		{name: "optional basic auth given", sourceType: "POSTMARK", auth: sourceAuthFlags{BasicAuthUser: "u", BasicAuthPass: "p"}},
		{name: "handshake type, auth with fields", sourceType: "ONFLEET"},
		{name: "handshake type, auth without fields", sourceType: "STRAVA"},
		{name: "handshake type with a choice of method", sourceType: "AZURE_EVENT_GRID"},
		{name: "choice of method", sourceType: "WEBHOOK", auth: sourceAuthFlags{APIKey: "k"}},
		{name: "type not in spec", sourceType: "NOT_A_TYPE"},
		{name: "no type", sourceType: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSourceAuthFromSpec(tt.sourceType, tt.configSet, tt.auth, tt.prefix)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Equal(t, tt.wantErr, err.Error())
		})
	}
}

func TestValidateSourceAuthFromSpecSkipsWhenSpecUnavailable(t *testing.T) {
	old := loadSourceTypes
	t.Cleanup(func() { loadSourceTypes = old })
	loadSourceTypes = func() (map[string]sources.SourceType, error) {
		return nil, errors.New("offline")
	}
	assert.NoError(t, validateSourceAuthFromSpec("AIRCALL", false, sourceAuthFlags{}, ""),
		"the API validates when the spec cannot be fetched")
}

// The commands run the check before any request: the stub sees nothing.
func TestSourceAuthValidationBlocksCommandsBeforeTheAPI(t *testing.T) {
	respond := func(r *http.Request) interface{} {
		if r.Method == http.MethodGet {
			return map[string]interface{}{"models": []interface{}{}}
		}
		return nil
	}
	tests := map[string]struct {
		args    []string
		wantErr string
	}{
		"source create": {
			args:    []string{"source", "create", "--name", "s", "--type", "AIRCALL"},
			wantErr: "--webhook-secret is required for source type AIRCALL",
		},
		"source upsert": {
			args:    []string{"source", "upsert", "s", "--type", "STRIPE", "--api-key", "k"},
			wantErr: "--api-key does not set config.auth webhook_secret_key, which source type STRIPE requires: use --webhook-secret",
		},
		"connection create": {
			args: []string{"connection", "create", "--name", "c",
				"--source-name", "s", "--source-type", "CLOUDSIGNAL",
				"--destination-name", "d", "--destination-type", "CLI", "--destination-cli-path", "/"},
			wantErr: "--source-api-key is required for source type CLOUDSIGNAL",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			requests, err := runGatewayAgainstStub(t, respond, tt.args...)
			require.Error(t, err)
			assert.Equal(t, tt.wantErr, err.Error())
			assert.Empty(t, requests)
		})
	}
}

func TestSourceCreateWithRequiredSecretReachesTheAPI(t *testing.T) {
	requests, err := runGatewayAgainstStub(t, func(r *http.Request) interface{} {
		return map[string]interface{}{"id": "src_1", "name": "s", "type": "AIRCALL"}
	}, "source", "create", "--name", "s", "--type", "AIRCALL", "--webhook-secret", "whsec")
	require.NoError(t, err)
	req := findRequest(t, requests, http.MethodPost, hookdeck.APIPathPrefix+"/sources")
	auth := bodyMap(t, bodyMap(t, req.Body, "config"), "auth")
	assert.Equal(t, "whsec", auth["webhook_secret_key"])
}
