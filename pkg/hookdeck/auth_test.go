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
			ProjectMode:      "gateway",
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
}
