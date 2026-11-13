//go:build source

package acceptance

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// postSigned POSTs body to url with the given headers and returns the status code.
func postSigned(t *testing.T, url, body string, headers map[string]string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

func hmacSHA256Hex(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

// verification is what the API recorded for the request whose body is body.
type verification struct {
	Verified       bool
	RejectionCause string
}

// waitForRequestVerification polls the source's requests until one carrying marker in its raw
// body appears, and returns how the API judged its signature.
func waitForRequestVerification(t *testing.T, cli *CLIRunner, sourceID, marker string) verification {
	t.Helper()
	for i := 0; i < propagationAttempts; i++ {
		time.Sleep(propagationInterval)
		var list struct {
			Models []map[string]interface{} `json:"models"`
		}
		require.NoError(t, cli.RunJSON(&list, "gateway", "request", "list", "--source-id", sourceID, "--limit", "20"))
		for _, m := range list.Models {
			id, _ := m["id"].(string)
			stdout := cli.RunExpectSuccess("gateway", "request", "raw-body", id)
			if !strings.Contains(stdout, marker) {
				continue
			}
			var req map[string]interface{}
			require.NoError(t, cli.RunJSON(&req, "gateway", "request", "get", id))
			v, _ := req["verified"].(bool)
			cause, _ := req["rejection_cause"].(string)
			return verification{Verified: v, RejectionCause: cause}
		}
	}
	require.Failf(t, "request not found", "no request on source %s carried %q", sourceID, marker)
	return verification{}
}

// TestSourceCreateVAPIWebhookSecretVerifiesProviderSignatures is the regression for --webhook-secret
// sending a generic header_key. The API lays the stored config.auth over the provider's defaults,
// so the generic x-webhook-signature header replaced VAPI's x-signature and every real VAPI
// request was rejected. The request is signed the way VAPI signs it (HMAC-SHA256, hex, in
// x-signature), and the negative controls show verification is really happening.
func TestSourceCreateVAPIWebhookSecretVerifiesProviderSignatures(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	secret := "vapi_secret_" + generateTimestamp()

	var src Source
	require.NoError(t, cli.RunJSON(&src, "gateway", "source", "create",
		"--name", "test-vapi-auth-"+generateTimestamp(), "--type", "VAPI", "--webhook-secret", secret))
	require.NotEmpty(t, src.ID)
	require.NotEmpty(t, src.URL)
	t.Cleanup(func() { deleteSource(t, cli, src.ID) })

	// The stored auth is the bare secret: nothing the CLI added can override VAPI's header.
	var got map[string]interface{}
	require.NoError(t, cli.RunJSON(&got, "gateway", "source", "get", src.ID, "--include-auth"))
	config, _ := got["config"].(map[string]interface{})
	auth, _ := config["auth"].(map[string]interface{})
	assert.NotContains(t, auth, "header_key", "the CLI must not override VAPI's own header")

	send := func(marker string, headers map[string]string) {
		body := fmt.Sprintf(`{"marker":%q}`, marker)
		for k, v := range headers {
			if v == "sign" {
				headers[k] = hmacSHA256Hex(secret, body)
			}
			if v == "sign-wrong" {
				headers[k] = hmacSHA256Hex(secret+"x", body)
			}
		}
		require.Less(t, postSigned(t, src.URL, body, headers), 500)
	}

	good := "good-" + generateTimestamp()
	send(good, map[string]string{"x-signature": "sign"})
	bad := "bad-" + generateTimestamp()
	send(bad, map[string]string{"x-signature": "sign-wrong"})
	generic := "generic-" + generateTimestamp()
	send(generic, map[string]string{"x-webhook-signature": "sign"})

	v := waitForRequestVerification(t, cli, src.ID, good)
	assert.True(t, v.Verified, "a request signed in x-signature must verify; rejection_cause=%q", v.RejectionCause)

	v = waitForRequestVerification(t, cli, src.ID, bad)
	assert.False(t, v.Verified, "a wrong signature must not verify")
	assert.Equal(t, "VERIFICATION_FAILED", v.RejectionCause)

	v = waitForRequestVerification(t, cli, src.ID, generic)
	assert.False(t, v.Verified, "the generic header must not be what VAPI sources verify")
}

// TestSourceCreateChoiceOfMethodTypesWithAuthFlags: GITLAB offers a choice of method, and the
// generic shape --webhook-secret used to send fitted none of them, so the API answered 422
// "config.auth is not allowed". Flags that no method of a type can express are refused locally.
func TestSourceCreateChoiceOfMethodTypesWithAuthFlags(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	authType := func(id string) string {
		t.Helper()
		var got map[string]interface{}
		require.NoError(t, cli.RunJSON(&got, "gateway", "source", "get", id, "--include-auth"))
		config, _ := got["config"].(map[string]interface{})
		v, _ := config["auth_type"].(string)
		return v
	}

	var secretSrc Source
	require.NoError(t, cli.RunJSON(&secretSrc, "gateway", "source", "create",
		"--name", "test-gitlab-secret-"+generateTimestamp(), "--type", "GITLAB", "--webhook-secret", "whsec_acceptance"))
	require.NotEmpty(t, secretSrc.ID)
	t.Cleanup(func() { deleteSource(t, cli, secretSrc.ID) })
	assert.Equal(t, "STANDARD_WEBHOOKS", authType(secretSrc.ID))

	var keySrc Source
	require.NoError(t, cli.RunJSON(&keySrc, "gateway", "source", "create",
		"--name", "test-gitlab-key-"+generateTimestamp(), "--type", "GITLAB", "--api-key", "glpat_acceptance"))
	require.NotEmpty(t, keySrc.ID)
	t.Cleanup(func() { deleteSource(t, cli, keySrc.ID) })
	assert.Equal(t, "GITLAB", authType(keySrc.ID))

	// A type whose API key needs its own header name has no flag for it: refused, nothing created.
	name := "test-ordinal-key-" + generateTimestamp()
	stdout, stderr, err := cli.Run("gateway", "source", "create", "--name", name, "--type", "ORDINAL", "--api-key", "k")
	require.Error(t, err)
	assert.Contains(t, stdout+stderr, "header_key")
	assert.Contains(t, stdout+stderr, "--config")
}
