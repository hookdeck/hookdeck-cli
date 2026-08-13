package version

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNeedsToUpgrade(t *testing.T) {
	// Basic same-version checks (v-prefix normalisation).
	require.False(t, needsToUpgrade("4.2.4.2", "v4.2.4.2"))
	require.False(t, needsToUpgrade("4.2.4.2", "4.2.4.2"))

	// GA to newer GA — should upgrade.
	require.True(t, needsToUpgrade("4.2.4.2", "4.2.4.3"))
	require.True(t, needsToUpgrade("4.2.4.2", "v4.2.4.3"))
	require.True(t, needsToUpgrade("v4.2.4.2", "v4.2.4.3"))

	// Minor/major version bump where numeric comparison matters
	// (string comparison would wrongly treat 1.9.x > 1.10.x).
	require.True(t, needsToUpgrade("1.9.1", "1.10.0"))
	require.False(t, needsToUpgrade("1.10.0", "1.9.1"))

	// GA current, pre-release latest — must NOT suggest upgrade.
	require.False(t, needsToUpgrade("1.9.1", "1.10.0-beta.4"))
	require.False(t, needsToUpgrade("1.9.1", "v1.10.0-beta.4"))
	require.False(t, needsToUpgrade("1.10.0", "1.10.1-beta.1"))

	// Pre-release current, newer GA latest — should upgrade.
	require.True(t, needsToUpgrade("1.9.0-beta.4", "1.9.0"))
	require.True(t, needsToUpgrade("1.10.0-beta.4", "1.10.0"))

	// Pre-release current, older GA latest — should NOT upgrade.
	require.False(t, needsToUpgrade("1.10.0-beta.4", "1.9.1"))

	// Pre-release to newer pre-release — should upgrade.
	require.True(t, needsToUpgrade("1.9.0-beta.3", "1.9.0-beta.4"))
	require.True(t, needsToUpgrade("1.9.0-beta.9", "1.9.0-beta.10"))

	// Pre-release to older pre-release — should NOT upgrade.
	require.False(t, needsToUpgrade("1.9.0-beta.4", "1.9.0-beta.3"))

	// Same pre-release — should NOT upgrade.
	require.False(t, needsToUpgrade("1.9.0-beta.4", "1.9.0-beta.4"))
}

// TestGetLatestVersion covers the replacement of go-github with a direct GitHub
// REST call (#331). The old implementation hardcoded github.NewClient(nil) with
// no injectable transport, so this function — the only reason the dependency
// existed — had no test coverage at all.
func TestGetLatestVersion(t *testing.T) {
	originalBase := githubAPIBaseURL
	t.Cleanup(func() { githubAPIBaseURL = originalBase })

	t.Run("returns the tag name from the latest release", func(t *testing.T) {
		var gotPath, gotAccept, gotUserAgent string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotAccept = r.Header.Get("Accept")
			gotUserAgent = r.Header.Get("User-Agent")
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"tag_name":"v2.5.0","name":"v2.5.0"}`)
		}))
		defer srv.Close()

		githubAPIBaseURL = srv.URL
		assert.Equal(t, "v2.5.0", getLatestVersion())
		assert.Equal(t, "/repos/hookdeck/hookdeck-cli/releases/latest", gotPath)
		assert.Equal(t, "application/vnd.github+json", gotAccept)
		// GitHub applies rate limits per User-Agent and asks callers to identify
		// themselves; go-github used to do this for us.
		assert.Contains(t, gotUserAgent, "hookdeck-cli/",
			"the request should identify the CLI rather than send Go's default agent")
	})

	t.Run("returns empty string on a non-200 response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer srv.Close()

		githubAPIBaseURL = srv.URL
		// Rate limiting is the common case here; it must never surface to the user.
		assert.Empty(t, getLatestVersion())
	})

	t.Run("returns empty string on malformed JSON", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `not json`)
		}))
		defer srv.Close()

		githubAPIBaseURL = srv.URL
		assert.Empty(t, getLatestVersion())
	})

	t.Run("returns empty string when the host is unreachable", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		srv.Close() // closed on purpose: nothing is listening

		githubAPIBaseURL = srv.URL
		assert.Empty(t, getLatestVersion(), "an offline machine must not break the CLI")
	})

	t.Run("a missing tag_name yields empty rather than panicking", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"name":"no tag here"}`)
		}))
		defer srv.Close()

		githubAPIBaseURL = srv.URL
		// The old code dereferenced *rep.TagName, which would panic on this shape.
		assert.Empty(t, getLatestVersion())
	})
}

// TestGetLatestVersionFeedsNeedsToUpgrade ties the fetch to its only consumer.
func TestGetLatestVersionFeedsNeedsToUpgrade(t *testing.T) {
	originalBase := githubAPIBaseURL
	t.Cleanup(func() { githubAPIBaseURL = originalBase })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"v2.5.0"}`)
	}))
	defer srv.Close()
	githubAPIBaseURL = srv.URL

	latest := getLatestVersion()
	assert.True(t, needsToUpgrade("v2.4.0", latest))
	assert.False(t, needsToUpgrade("v2.5.0", latest))
	assert.False(t, needsToUpgrade("v2.6.0", latest))
}
