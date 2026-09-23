package hookdeck

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactHeadersForLog_redactsAuthorization(t *testing.T) {
	headers := http.Header{}
	headers.Set("Authorization", "Basic c2tfdGVzdDo=")
	headers.Set("Content-Type", "application/json")

	redacted := redactHeadersForLog(headers)
	require.Equal(t, "[redacted]", redacted.Get("Authorization"))
	require.Equal(t, "application/json", redacted.Get("Content-Type"))
	require.Equal(t, "Basic c2tfdGVzdDo=", headers.Get("Authorization"))
}

func TestRedactRequestBodyForLog_redactsGuestAPIKey(t *testing.T) {
	body := `{"device_name":"laptop","guest_api_key":"hk_secret"}`
	redacted := redactBodyForLog(body)
	require.Contains(t, redacted, `"guest_api_key":"[redacted]"`)
	require.NotContains(t, redacted, "hk_secret")
}

func TestRedactRequestBodyForLog_leavesNonJSONUnchanged(t *testing.T) {
	body := "guest_api_key=hk_secret"
	require.Equal(t, body, redactBodyForLog(body))
}

// Secrets arrive at different depths on different routes, so redaction matches
// by field name wherever it appears rather than by a fixed path.
func TestRedactBodyForLogReachesNestedSecrets(t *testing.T) {
	cases := []struct {
		name, body string
		mustNot    []string
	}{
		{
			name:    "outpost destination credentials are nested",
			body:    `{"id":"des_1","type":"webhook","config":{"url":"https://x"},"credentials":{"secret":"whsec_live","hmac":"abc"}}`,
			mustNot: []string{"whsec_live", "abc"},
		},
		{
			name:    "an api key create returns the secret as key",
			body:    `{"id":"apk_1","label":"ci","key":"hd_live_abcdef"}`,
			mustNot: []string{"hd_live_abcdef"},
		},
		{
			name:    "a tenant token response is only a token",
			body:    `{"token":"eyJhbGciOi"}`,
			mustNot: []string{"eyJhbGciOi"},
		},
		{
			name:    "inside an array of models",
			body:    `{"models":[{"id":"apk_1","key":"hd_one"},{"id":"apk_2","key":"hd_two"}]}`,
			mustNot: []string{"hd_one", "hd_two"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := redactBodyForLog(tc.body)
			for _, secret := range tc.mustNot {
				assert.NotContains(t, got, secret,
					"a debug log must not carry a reusable credential")
			}
			assert.Contains(t, got, "[redacted]")
		})
	}
}

// A body with nothing to redact is logged exactly as it was sent, so debug
// output stays faithful when it costs nothing.
func TestRedactBodyForLogLeavesCleanBodiesAlone(t *testing.T) {
	body := `{"name":"stripe-to-backend","source_id":"src_1"}`
	assert.Equal(t, body, redactBodyForLog(body))
}

func TestRedactHeadersForLogCoversEveryCredentialHeader(t *testing.T) {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer sk_live")
	headers.Set("X-Api-Key", "hd_live")
	headers.Set("Set-Cookie", "session=abc")
	headers.Set("Content-Type", "application/json")

	got := redactHeadersForLog(headers)
	for _, name := range []string{"Authorization", "X-Api-Key", "Set-Cookie"} {
		assert.Equal(t, "[redacted]", got.Get(name), "%s must be redacted", name)
	}
	assert.Equal(t, "application/json", got.Get("Content-Type"), "non-secret headers stay readable")
}
