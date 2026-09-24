package hookdeck

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The API answers POST /cli-auth with a poll_url that is NOT under
// APIPathPrefix. checkResolvedPath compares every request path against that
// prefix, so it rejected the URL the server had just told the CLI to call and
// browser sign-in died between opening the browser and collecting the key:
//
//	request path rejected: "/cli-auth/poll" does not address the /2026-09-01 API
//
// The guard still has to apply -- poll_url is remote input -- so the fix pins
// it to the path the server supplied instead of switching it off. See #438.
func TestPollForAPIKeyAcceptsUnversionedPollURL(t *testing.T) {
	var polled string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		polled = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"claimed": true,
			"key":     "cli_key_from_poll",
		})
	}))
	defer srv.Close()

	// Unversioned, exactly as the real API returns it.
	got, err := pollForAPIKey(srv.URL+"/cli-auth/poll?key=pollkey", time.Millisecond, 2)

	require.NoError(t, err, "an unversioned poll_url must be polled, not rejected")
	require.Equal(t, "/cli-auth/poll", polled, "the server's own path must be the one requested")
	require.Equal(t, "cli_key_from_poll", got.APIKey)
}

// PollForAPIKeyWithKey builds its own poll URL with the prefix, so both shapes
// have to keep working.
func TestPollForAPIKeyAcceptsVersionedPollURL(t *testing.T) {
	var polled string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		polled = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"claimed": true,
			"key":     "cli_key_from_poll",
		})
	}))
	defer srv.Close()

	got, err := pollForAPIKey(srv.URL+APIPathPrefix+"/cli-auth/poll?key=pollkey", time.Millisecond, 2)

	require.NoError(t, err)
	require.Equal(t, APIPathPrefix+"/cli-auth/poll", polled)
	require.Equal(t, "cli_key_from_poll", got.APIKey)
}

// Pinning the prefix to the supplied path must not become a way to reach
// anything else on that host: the guard is still live for the poll client.
func TestPollClientStillRejectsRetargetedPath(t *testing.T) {
	base, err := url.Parse("https://api.example.test")
	require.NoError(t, err)

	c := &Client{BaseURL: base, PathPrefix: "/cli-auth/poll"}

	_, err = c.resolveRequestURL("/organizations/current")
	require.ErrorIs(t, err, ErrRequestPathRejected,
		"a path outside the pinned poll path must still be refused")
}
