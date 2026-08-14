package mcpcore

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
	FillProjectDisplayNameIfNeeded(client, client)
	require.Equal(t, "Acme", client.ProjectOrg)
	require.Equal(t, "production", client.ProjectName)
}

func TestFillProjectDisplayNameIfNeeded_NoOpWhenNameSet(t *testing.T) {
	client := &hookdeck.Client{ProjectID: "p", ProjectName: "already"}
	FillProjectDisplayNameIfNeeded(client, client)
	require.Equal(t, "already", client.ProjectName)
}

// A product API served from its own host cannot answer the project list, so the
// lookup has to go to the account API while the product client is the one
// updated.
func TestFillProjectDisplayNameIfNeeded_LooksUpThroughTheAccountClient(t *testing.T) {
	accountAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2025-07-01/teams" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "proj_x", "name": "[Acme] production", "mode": "outpost"},
		})
	}))
	t.Cleanup(accountAPI.Close)

	productAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("the product API must not be asked for the project list: %s", r.URL.Path)
		http.NotFound(w, r)
	}))
	t.Cleanup(productAPI.Close)

	accountURL, err := url.Parse(accountAPI.URL)
	require.NoError(t, err)
	productURL, err := url.Parse(productAPI.URL)
	require.NoError(t, err)

	account := &hookdeck.Client{BaseURL: accountURL, APIKey: "k", ProjectID: "proj_x"}
	product := &hookdeck.Client{BaseURL: productURL, APIKey: "k", ProjectID: "proj_x"}

	FillProjectDisplayNameIfNeeded(account, product)
	require.Equal(t, "Acme", product.ProjectOrg)
	require.Equal(t, "production", product.ProjectName)
}
