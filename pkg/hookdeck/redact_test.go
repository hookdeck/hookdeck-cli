package hookdeck

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// The API key the CLI sends is base64("<key>:") in a Basic credential, so a
// leaked header is a leaked key.
const testAPIKey = "hkdk_test_secret_value_do_not_log"

func TestRedactHeadersHidesCredentialsKeepsScheme(t *testing.T) {
	header := http.Header{}
	header.Set("Content-Type", "application/json")
	header.Set("X-Team-Id", "tm_visible")
	header.Add("Cookie", "session="+testAPIKey)
	header.Set("X-Api-Key", testAPIKey)

	req := &http.Request{Header: header}
	req.SetBasicAuth(testAPIKey, "")

	redacted := redactHeaders(req.Header)

	if got := redacted.Get("Authorization"); got != "Basic "+redactedPlaceholder {
		t.Errorf("Authorization = %q, want %q", got, "Basic "+redactedPlaceholder)
	}
	if got := redacted.Get("X-Api-Key"); got != redactedPlaceholder {
		t.Errorf("X-Api-Key = %q, want %q", got, redactedPlaceholder)
	}
	if got := redacted.Get("Cookie"); got != redactedPlaceholder {
		t.Errorf("Cookie = %q, want %q", got, redactedPlaceholder)
	}

	// Non-sensitive headers must survive — they are the diagnostic value.
	if got := redacted.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want it preserved", got)
	}
	if got := redacted.Get("X-Team-Id"); got != "tm_visible" {
		t.Errorf("X-Team-Id = %q, want it preserved", got)
	}

	// The original header must be untouched — it still authenticates the request.
	if !strings.HasPrefix(req.Header.Get("Authorization"), "Basic ") ||
		req.Header.Get("Authorization") == "Basic "+redactedPlaceholder {
		t.Error("redactHeaders mutated the original header; the request would lose its credentials")
	}
}

func TestRedactHeadersBearerAndUnknownScheme(t *testing.T) {
	header := http.Header{}
	header.Set("Authorization", "Bearer "+testAPIKey)
	if got := redactHeaders(header).Get("Authorization"); got != "Bearer "+redactedPlaceholder {
		t.Errorf("Authorization = %q, want %q", got, "Bearer "+redactedPlaceholder)
	}

	// A bare credential with no scheme must be fully redacted, not passed through.
	header.Set("Authorization", testAPIKey)
	if got := redactHeaders(header).Get("Authorization"); got != redactedPlaceholder {
		t.Errorf("Authorization = %q, want %q", got, redactedPlaceholder)
	}
}

func TestRedactURLHidesCredentialQueryParams(t *testing.T) {
	// The login poll endpoint carries the API key in the query string, and URLs
	// are logged at error level — visible without --log-level debug.
	u, err := url.Parse("https://api.hookdeck.com/2025-07-01/cli-auth/poll?key=" + testAPIKey + "&project_id=tm_visible")
	if err != nil {
		t.Fatalf("failed to parse url: %v", err)
	}

	got := redactURL(u)

	if strings.Contains(got, testAPIKey) {
		t.Errorf("redactURL leaked the key: %s", got)
	}
	if !strings.Contains(got, "key="+url.QueryEscape(redactedPlaceholder)) {
		t.Errorf("redactURL = %q, want a redacted key parameter", got)
	}
	// Endpoint and non-sensitive params stay, so the log still says what was called.
	if !strings.Contains(got, "/cli-auth/poll") || !strings.Contains(got, "project_id=tm_visible") {
		t.Errorf("redactURL = %q, want path and non-sensitive params preserved", got)
	}
}

func TestRedactURLLeavesCleanURLsAlone(t *testing.T) {
	raw := "https://api.hookdeck.com/2025-07-01/sources?limit=100"
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("failed to parse url: %v", err)
	}

	if got := redactURL(u); got != raw {
		t.Errorf("redactURL = %q, want it unchanged (%q)", got, raw)
	}
}

func TestRedactBodyHidesCredentialFieldsAtAnyDepth(t *testing.T) {
	// Shaped like the login poll response (api_key) and a destination create
	// request (nested auth secrets).
	body := []byte(`{
		"api_key": "` + testAPIKey + `",
		"name": "my-destination",
		"config": {
			"url": "https://example.com",
			"auth": {"password": "` + testAPIKey + `", "username": "visible-user"}
		},
		"sources": [{"webhook_secret": "` + testAPIKey + `", "id": "src_visible"}]
	}`)

	got := redactBody(body)

	if strings.Contains(got, testAPIKey) {
		t.Fatalf("redactBody leaked the secret: %s", got)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("redactBody produced invalid JSON: %v", err)
	}

	if parsed["api_key"] != redactedPlaceholder {
		t.Errorf("api_key = %v, want %q", parsed["api_key"], redactedPlaceholder)
	}
	// Everything that isn't a credential must survive for diagnosis.
	if parsed["name"] != "my-destination" {
		t.Errorf("name = %v, want it preserved", parsed["name"])
	}
	config := parsed["config"].(map[string]interface{})
	if config["url"] != "https://example.com" {
		t.Errorf("config.url = %v, want it preserved", config["url"])
	}
	auth := config["auth"].(map[string]interface{})
	if auth["password"] != redactedPlaceholder {
		t.Errorf("config.auth.password = %v, want %q", auth["password"], redactedPlaceholder)
	}
	if auth["username"] != "visible-user" {
		t.Errorf("config.auth.username = %v, want it preserved", auth["username"])
	}
	source := parsed["sources"].([]interface{})[0].(map[string]interface{})
	if source["webhook_secret"] != redactedPlaceholder {
		t.Errorf("sources[0].webhook_secret = %v, want %q", source["webhook_secret"], redactedPlaceholder)
	}
	if source["id"] != "src_visible" {
		t.Errorf("sources[0].id = %v, want it preserved", source["id"])
	}
}

func TestRedactBodyPassesThroughNonJSON(t *testing.T) {
	if got := redactBody([]byte("not json at all")); got != "not json at all" {
		t.Errorf("redactBody = %q, want the raw body", got)
	}
	if got := redactBody(nil); got != "" {
		t.Errorf("redactBody(nil) = %q, want empty", got)
	}
}
