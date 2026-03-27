package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/stretchr/testify/require"
)

func TestFillProjectDisplayNameIfNeeded_SetsNameFromAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2025-07-01/teams" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "proj_x", "name": "[Acme] production", "mode": "console"},
		})
	}))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	client := &hookdeck.Client{
		BaseURL:   u,
		APIKey:    "k",
		ProjectID: "proj_x",
	}
	fillProjectDisplayNameIfNeeded(client)
	require.Equal(t, "Acme", client.ProjectOrg)
	require.Equal(t, "production", client.ProjectName)
}

func TestFillProjectDisplayNameIfNeeded_NoOpWhenNameSet(t *testing.T) {
	client := &hookdeck.Client{ProjectID: "p", ProjectName: "already"}
	fillProjectDisplayNameIfNeeded(client)
	require.Equal(t, "already", client.ProjectName)
}
