package hookdeck

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateAPIKey_omitsTeamAndProjectHeadersWhenConfigHasProjectID(t *testing.T) {
	var sawTeamHeader bool
	var sawProjectHeader bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawTeamHeader = r.Header.Get("X-Team-ID") != ""
		sawProjectHeader = r.Header.Get("X-Project-ID") != ""
		if r.URL.Path != APIPathPrefix+"/cli-auth/validate" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ValidateAPIKeyResponse{
			UserID:           "u1",
			UserName:         "n",
			UserEmail:        "e@e",
			OrganizationName: "o",
			OrganizationID:   "o1",
			ProjectID:        "t1",
			ProjectName:      "p",
			ProjectType:      "event_gateway",
		})
	}))
	t.Cleanup(server.Close)

	baseURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	client := &Client{
		BaseURL:   baseURL,
		APIKey:    "test_key",
		ProjectID: "stale_team_should_not_be_sent",
	}

	resp, err := client.ValidateAPIKey()
	require.NoError(t, err)
	require.False(t, sawTeamHeader, "validate must not send X-Team-ID")
	require.False(t, sawProjectHeader, "validate must not send X-Project-ID")
	require.Equal(t, "t1", resp.ProjectID)
	require.Equal(t, "event_gateway", resp.ProjectType)
}

func TestStartLogin_requestsAndReturnsUserCode(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, APIPathPrefix+"/cli-auth", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"browser_url": "https://dashboard.example.test/cli-auth/k",
			"poll_url":    "https://api.example.test/cli-auth/poll?key=k",
			"user_code":   "WDJB-MJHT",
		})
	}))
	t.Cleanup(server.Close)

	baseURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	session, err := (&Client{BaseURL: baseURL}).StartLogin("laptop")
	require.NoError(t, err)
	require.Equal(t, map[string]any{"device_name": "laptop", "with_user_code": true}, body)
	require.Equal(t, "WDJB-MJHT", session.UserCode)
	require.Equal(t, "https://dashboard.example.test/cli-auth/k", session.BrowserURL)
}

func TestStartLogin_noUserCodeFromAnAPIThatIssuesNone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"browser_url": "https://dashboard.example.test/cli-auth/k",
			"poll_url":    "https://api.example.test/cli-auth/poll?key=k",
		})
	}))
	t.Cleanup(server.Close)

	baseURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	session, err := (&Client{BaseURL: baseURL}).StartLogin("laptop")
	require.NoError(t, err)
	require.Empty(t, session.UserCode)
}
