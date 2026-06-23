package hookdeck

import (
	"net/http"
	"testing"

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
	body := `{"device_name":"laptop","guest_user_id":"usr_1","guest_api_key":"hk_secret"}`
	redacted := redactRequestBodyForLog(body)
	require.Contains(t, redacted, `"guest_api_key":"[redacted]"`)
	require.Contains(t, redacted, `"guest_user_id":"usr_1"`)
	require.NotContains(t, redacted, "hk_secret")
}

func TestRedactRequestBodyForLog_leavesNonJSONUnchanged(t *testing.T) {
	body := "guest_api_key=hk_secret"
	require.Equal(t, body, redactRequestBodyForLog(body))
}
