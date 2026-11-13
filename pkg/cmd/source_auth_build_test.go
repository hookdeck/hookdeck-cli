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

// The API stores the config.auth keys a type's auth schema lists, and verifies a request with
// them laid over the provider's own defaults. A key the CLI adds that the provider did not ask
// for therefore changes how real requests are verified: VAPI's header_key replaces x-signature,
// for example. These tests pin the exact config.auth and auth_type each flag sends per type,
// built from the spec fixture (a cut of the 2026-09-01 spec).

type authWant struct {
	auth     map[string]interface{}
	authType string
	err      string // substring of the error, when the flag must be refused
}

func genericHMAC(algo string) map[string]interface{} {
	return map[string]interface{}{
		"algorithm": algo, "encoding": "hex",
		"header_key": "x-webhook-signature", "webhook_secret_key": "S",
	}
}

var secretOnly = map[string]interface{}{"webhook_secret_key": "S"}

func TestSourceAuthFromFlagsPerType(t *testing.T) {
	tests := []struct {
		flag       string
		sourceType string
		flags      sourceConfigFlags
		want       authWant
	}{
		// --webhook-secret
		{"webhook-secret", "WEBHOOK", sourceConfigFlags{WebhookSecret: "S"}, authWant{auth: genericHMAC("sha256"), authType: "HMAC"}},
		{"webhook-secret", "HTTP", sourceConfigFlags{WebhookSecret: "S"}, authWant{auth: genericHMAC("sha256"), authType: "HMAC"}},
		{"webhook-secret", "STRIPE", sourceConfigFlags{WebhookSecret: "S"}, authWant{auth: secretOnly}},
		{"webhook-secret", "GITHUB", sourceConfigFlags{WebhookSecret: "S"}, authWant{auth: secretOnly}},
		// VAPI and FORMSTACK take header_key, so sending the generic one would replace the header
		// the provider signs with (x-signature, x-fs-signature).
		{"webhook-secret", "VAPI", sourceConfigFlags{WebhookSecret: "S"}, authWant{auth: secretOnly}},
		{"webhook-secret", "FORMSTACK", sourceConfigFlags{WebhookSecret: "S"}, authWant{auth: secretOnly}},
		{"webhook-secret", "GITLAB", sourceConfigFlags{WebhookSecret: "S"}, authWant{auth: secretOnly, authType: "STANDARD_WEBHOOKS"}},
		{"webhook-secret", "GRAFANA", sourceConfigFlags{WebhookSecret: "S"}, authWant{auth: secretOnly, authType: "GRAFANA"}},
		{"webhook-secret", "PROMETHEUS_ALERTMANAGER", sourceConfigFlags{WebhookSecret: "S"}, authWant{err: "--webhook-secret does not apply to source type PROMETHEUS_ALERTMANAGER"}},
		{"webhook-secret", "BETTER_STACK", sourceConfigFlags{WebhookSecret: "S"}, authWant{err: "--webhook-secret does not apply to source type BETTER_STACK"}},
		{"webhook-secret", "ORDINAL", sourceConfigFlags{WebhookSecret: "S"}, authWant{err: "--webhook-secret does not apply to source type ORDINAL"}},
		// A single-method type with no secret field: the required-field check names the right flag.
		{"webhook-secret", "ACUMATICA", sourceConfigFlags{WebhookSecret: "S"}, authWant{auth: secretOnly}},

		// --hmac-secret keeps the generic shape where the type's method takes algorithm.
		{"hmac-secret", "WEBHOOK", sourceConfigFlags{HMACSecret: "S", HMACAlgo: "SHA512"}, authWant{auth: genericHMAC("sha512"), authType: "HMAC"}},
		{"hmac-secret", "HTTP", sourceConfigFlags{HMACSecret: "S"}, authWant{auth: genericHMAC("sha256"), authType: "HMAC"}},
		{"hmac-secret", "VAPI", sourceConfigFlags{HMACSecret: "S"}, authWant{auth: genericHMAC("sha256")}},
		{"hmac-secret", "STRIPE", sourceConfigFlags{HMACSecret: "S"}, authWant{auth: secretOnly}},
		{"hmac-secret", "FORMSTACK", sourceConfigFlags{HMACSecret: "S"}, authWant{auth: secretOnly}},
		{"hmac-secret", "GITLAB", sourceConfigFlags{HMACSecret: "S"}, authWant{auth: secretOnly, authType: "STANDARD_WEBHOOKS"}},
		{"hmac-secret", "GRAFANA", sourceConfigFlags{HMACSecret: "S"}, authWant{auth: secretOnly, authType: "GRAFANA"}},
		// --hmac-algo on a method with no algorithm is ignored with a warning, not refused:
		// the provider fixes its own algorithm, and refusing broke TWILIO with sha1.
		{"hmac-secret", "GITLAB", sourceConfigFlags{HMACSecret: "S", HMACAlgo: "sha512"}, authWant{auth: secretOnly, authType: "STANDARD_WEBHOOKS"}},
		{"hmac-secret", "PROMETHEUS_ALERTMANAGER", sourceConfigFlags{HMACSecret: "S"}, authWant{err: "--hmac-secret does not apply to source type PROMETHEUS_ALERTMANAGER"}},
		{"hmac-secret", "BETTER_STACK", sourceConfigFlags{HMACSecret: "S"}, authWant{err: "--hmac-secret does not apply to source type BETTER_STACK"}},
		{"hmac-secret", "ORDINAL", sourceConfigFlags{HMACSecret: "S"}, authWant{err: "--hmac-secret does not apply to source type ORDINAL"}},

		// --api-key
		{"api-key", "WEBHOOK", sourceConfigFlags{APIKey: "K"}, authWant{auth: map[string]interface{}{"header_key": "x-api-key", "api_key": "K"}, authType: "API_KEY"}},
		{"api-key", "HTTP", sourceConfigFlags{APIKey: "K"}, authWant{auth: map[string]interface{}{"header_key": "x-api-key", "api_key": "K"}, authType: "API_KEY"}},
		{"api-key", "GITLAB", sourceConfigFlags{APIKey: "K"}, authWant{auth: map[string]interface{}{"api_key": "K"}, authType: "GITLAB"}},
		{"api-key", "PROMETHEUS_ALERTMANAGER", sourceConfigFlags{APIKey: "K"}, authWant{auth: map[string]interface{}{"api_key": "K"}, authType: "PROMETHEUS_ALERTMANAGER"}},
		// The header name is the provider's contract and no flag sets it: refuse rather than guess x-api-key.
		{"api-key", "GRAFANA", sourceConfigFlags{APIKey: "K"}, authWant{err: "--api-key does not set config.auth header_key"}},
		{"api-key", "BETTER_STACK", sourceConfigFlags{APIKey: "K"}, authWant{err: "--api-key does not set config.auth header_key"}},
		{"api-key", "ORDINAL", sourceConfigFlags{APIKey: "K"}, authWant{err: "--api-key does not set config.auth header_key"}},
		{"api-key", "ACUMATICA", sourceConfigFlags{APIKey: "K"}, authWant{err: "--api-key does not set config.auth header_key"}},
		{"api-key", "STRIPE", sourceConfigFlags{APIKey: "K"}, authWant{auth: map[string]interface{}{"api_key": "K"}}},

		// --basic-auth-*
		{"basic-auth", "WEBHOOK", sourceConfigFlags{BasicAuthUser: "u", BasicAuthPass: "p"}, authWant{auth: map[string]interface{}{"username": "u", "password": "p"}, authType: "BASIC_AUTH"}},
		{"basic-auth", "GRAFANA", sourceConfigFlags{BasicAuthUser: "u", BasicAuthPass: "p"}, authWant{auth: map[string]interface{}{"username": "u", "password": "p"}, authType: "BASIC_AUTH"}},
		{"basic-auth", "GITLAB", sourceConfigFlags{BasicAuthUser: "u", BasicAuthPass: "p"}, authWant{err: "--basic-auth-user and --basic-auth-pass does not apply to source type GITLAB"}},
	}
	for _, tt := range tests {
		t.Run(tt.flag+"/"+tt.sourceType, func(t *testing.T) {
			flags := tt.flags
			auth, authType, err := sourceAuthFromFlags(&flags, "", tt.sourceType)
			if tt.want.err != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.want.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want.auth, auth)
			assert.Equal(t, tt.want.authType, authType)
		})
	}
}

// The legacy top-level keys in --config go through the same per-type builder as the flags, so
// webhook_secret on VAPI does not store a header_key that replaces VAPI's x-signature.
func TestLegacyConfigKeysUsePerTypeAuth(t *testing.T) {
	tests := []struct {
		name       string
		sourceType string
		input      string
		wantConfig map[string]interface{}
		wantErr    string
	}{
		{"WEBHOOK webhook_secret", "WEBHOOK", `{"webhook_secret":"S"}`,
			map[string]interface{}{"auth": genericHMAC("sha256"), "auth_type": "HMAC"}, ""},
		{"VAPI webhook_secret", "VAPI", `{"webhook_secret":"S"}`,
			map[string]interface{}{"auth": secretOnly}, ""},
		{"GITLAB webhook_secret", "GITLAB", `{"webhook_secret":"S"}`,
			map[string]interface{}{"auth": secretOnly, "auth_type": "STANDARD_WEBHOOKS"}, ""},
		{"WEBHOOK api_key", "WEBHOOK", `{"api_key":"K"}`,
			map[string]interface{}{"auth": map[string]interface{}{"header_key": "x-api-key", "api_key": "K"}, "auth_type": "API_KEY"}, ""},
		{"GITLAB api_key", "GITLAB", `{"api_key":"K"}`,
			map[string]interface{}{"auth": map[string]interface{}{"api_key": "K"}, "auth_type": "GITLAB"}, ""},
		{"WEBHOOK hmac", "WEBHOOK", `{"hmac":{"secret":"S","algorithm":"SHA512"}}`,
			map[string]interface{}{"auth": genericHMAC("sha512"), "auth_type": "HMAC"}, ""},
		{"VAPI hmac", "VAPI", `{"hmac":{"secret":"S"}}`,
			map[string]interface{}{"auth": genericHMAC("sha256")}, ""},
		{"GRAFANA basic_auth", "GRAFANA", `{"basic_auth":{"username":"u","password":"p"}}`,
			map[string]interface{}{"auth": map[string]interface{}{"username": "u", "password": "p"}, "auth_type": "BASIC_AUTH"}, ""},
		{"explicit auth_type is kept", "WEBHOOK", `{"auth_type":"STANDARD_WEBHOOKS","webhook_secret":"S"}`,
			map[string]interface{}{"auth": genericHMAC("sha256"), "auth_type": "STANDARD_WEBHOOKS"}, ""},
		{"refused type", "BETTER_STACK", `{"webhook_secret":"S"}`, nil, `config key "webhook_secret"`},
		{"refused header", "ORDINAL", `{"api_key":"K"}`, nil, "header_key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := buildSourceConfigFromFlags(tt.input, "", nil, tt.sourceType)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantConfig, config)
		})
	}
}

// Offline, the legacy keys still work for WEBHOOK and HTTP and give provider types the bare secret.
func TestLegacyConfigKeysWithoutSpec(t *testing.T) {
	origMethods, origTypes := loadSourceAuthMethods, loadSourceTypes
	t.Cleanup(func() { loadSourceAuthMethods, loadSourceTypes = origMethods, origTypes })
	loadSourceAuthMethods = func() (map[string][]sources.AuthMethod, error) { return nil, errors.New("offline") }
	loadSourceTypes = func() (map[string]sources.SourceType, error) { return nil, errors.New("offline") }

	config, err := buildSourceConfigFromFlags(`{"webhook_secret":"S"}`, "", nil, "WEBHOOK")
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"auth": genericHMAC("sha256"), "auth_type": "HMAC"}, config)

	config, err = buildSourceConfigFromFlags(`{"webhook_secret":"S"}`, "", nil, "VAPI")
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"auth": secretOnly}, config)
}

func TestSourceAuthFromFlagsNoFlag(t *testing.T) {
	auth, authType, err := sourceAuthFromFlags(&sourceConfigFlags{AllowedHTTPMethods: "POST"}, "", "VAPI")
	require.NoError(t, err)
	assert.Nil(t, auth)
	assert.Empty(t, authType)
}

// Connection flags carry the source- prefix; the messages must name the flag the user typed.
func TestSourceAuthFromFlagsErrorsUseFlagPrefix(t *testing.T) {
	_, _, err := sourceAuthFromFlags(&sourceConfigFlags{APIKey: "K"}, "source-", "ORDINAL")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--source-api-key")
	assert.Contains(t, err.Error(), "--source-config")

	_, _, err = sourceAuthFromFlags(&sourceConfigFlags{WebhookSecret: "S"}, "source-", "BETTER_STACK")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--source-webhook-secret")
}

// The same flags through the config builder: the auth_type travels with the auth.
func TestBuildSourceConfigFromIndividualFlagsPerType(t *testing.T) {
	config, err := buildSourceConfigFromIndividualFlags(&sourceConfigFlags{WebhookSecret: "S"}, "", "VAPI")
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"auth": secretOnly}, config)

	config, err = buildSourceConfigFromIndividualFlags(&sourceConfigFlags{WebhookSecret: "S"}, "", "GITLAB")
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"auth": secretOnly, "auth_type": "STANDARD_WEBHOOKS"}, config)

	_, err = buildSourceConfigFromIndividualFlags(&sourceConfigFlags{WebhookSecret: "S"}, "", "BETTER_STACK")
	require.Error(t, err)
}

// Without the spec, WEBHOOK and HTTP keep working, and a provider type gets only the secret: the
// provider's own defaults then apply, which is right for every type and never overrides a header.
func TestSourceAuthFromFlagsWithoutSpec(t *testing.T) {
	origMethods, origTypes := loadSourceAuthMethods, loadSourceTypes
	t.Cleanup(func() { loadSourceAuthMethods, loadSourceTypes = origMethods, origTypes })
	loadSourceAuthMethods = func() (map[string][]sources.AuthMethod, error) { return nil, errors.New("offline") }
	loadSourceTypes = func() (map[string]sources.SourceType, error) { return nil, errors.New("offline") }

	auth, authType, err := sourceAuthFromFlags(&sourceConfigFlags{WebhookSecret: "S"}, "", "WEBHOOK")
	require.NoError(t, err)
	assert.Equal(t, genericHMAC("sha256"), auth)
	assert.Equal(t, "HMAC", authType)

	auth, authType, err = sourceAuthFromFlags(&sourceConfigFlags{APIKey: "K"}, "", "http")
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"header_key": "x-api-key", "api_key": "K"}, auth)
	assert.Equal(t, "API_KEY", authType)

	for _, sourceType := range []string{"VAPI", "GITLAB", "STRIPE"} {
		auth, authType, err = sourceAuthFromFlags(&sourceConfigFlags{WebhookSecret: "S"}, "", sourceType)
		require.NoError(t, err, sourceType)
		assert.Equal(t, secretOnly, auth, sourceType)
		assert.Empty(t, authType, sourceType)
	}
}

// Command level: `source create --type VAPI --webhook-secret` must send the bare secret, since the
// API would store a generic header_key and verify VAPI's requests against it.
func TestSourceCreateSendsProviderShapedAuth(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantConfig map[string]interface{}
	}{
		{"VAPI webhook secret", []string{"--type", "VAPI", "--webhook-secret", "S"},
			map[string]interface{}{"auth": secretOnly}},
		{"FORMSTACK webhook secret", []string{"--type", "FORMSTACK", "--webhook-secret", "S"},
			map[string]interface{}{"auth": secretOnly}},
		{"GITLAB webhook secret", []string{"--type", "GITLAB", "--webhook-secret", "S"},
			map[string]interface{}{"auth": secretOnly, "auth_type": "STANDARD_WEBHOOKS"}},
		{"GRAFANA webhook secret", []string{"--type", "GRAFANA", "--webhook-secret", "S"},
			map[string]interface{}{"auth": secretOnly, "auth_type": "GRAFANA"}},
		{"GITLAB api key", []string{"--type", "GITLAB", "--api-key", "K"},
			map[string]interface{}{"auth": map[string]interface{}{"api_key": "K"}, "auth_type": "GITLAB"}},
		{"WEBHOOK webhook secret is unchanged", []string{"--type", "WEBHOOK", "--webhook-secret", "S"},
			map[string]interface{}{"auth": map[string]interface{}{
				"algorithm": "sha256", "encoding": "hex", "header_key": "x-webhook-signature", "webhook_secret_key": "S",
			}, "auth_type": "HMAC"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests, err := runGatewayAgainstStub(t,
				func(r *http.Request) interface{} {
					return map[string]interface{}{"id": "src_1", "name": "s", "type": "VAPI"}
				},
				append([]string{"source", "create", "--name", "s"}, tt.args...)...,
			)
			require.NoError(t, err)
			post := findRequest(t, requests, http.MethodPost, hookdeck.APIPathPrefix+"/sources")
			assert.Equal(t, tt.wantConfig, post.Body["config"])
		})
	}
}

// A flag the CLI cannot build for the type is refused before any request is sent.
func TestSourceCreateRefusesUnbuildableAuth(t *testing.T) {
	for _, args := range [][]string{
		{"--type", "ORDINAL", "--api-key", "K"},
		{"--type", "BETTER_STACK", "--webhook-secret", "S"},
		{"--type", "ACUMATICA", "--api-key", "K"},
	} {
		requests, err := runGatewayAgainstStub(t,
			func(r *http.Request) interface{} { return map[string]interface{}{} },
			append([]string{"source", "create", "--name", "s"}, args...)...,
		)
		require.Error(t, err, args)
		assert.Empty(t, requests, "nothing is sent for %v", args)
	}
}
