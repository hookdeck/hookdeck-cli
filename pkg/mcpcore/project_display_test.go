package mcpcore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/stretchr/testify/assert"
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

// TestFillProjectDisplayName_ProjectScopedKey covers the case that left MCP
// responses carrying a bare project id: a key from `hookdeck ci` cannot list
// projects, so name resolution has to come from validating the key instead.
func TestFillProjectDisplayName_ProjectScopedKey(t *testing.T) {
	var listCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/cli-auth/validate"):
			_, _ = w.Write([]byte(`{"team_id":"tm_1","team_name_no_org":"cli outpost testing","organization_name":"Automated Testing"}`))
		case strings.HasSuffix(r.URL.Path, "/teams"):
			listCalled = true
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"This credential is scoped to a single project"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	baseURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	client := &hookdeck.Client{BaseURL: baseURL, APIKey: "ci-key", ProjectID: "tm_1"}

	FillProjectDisplayNameIfNeeded(client, client)

	assert.Equal(t, "cli outpost testing", client.ProjectName)
	assert.Equal(t, "Automated Testing", client.ProjectOrg)
	assert.False(t, listCalled, "validating the key is enough; listing projects would fail for this credential")
}

// TestFillProjectDisplayName_FallsBackToListing covers the other direction: the
// active project is not the one the key belongs to, so only a listing can name it.
func TestFillProjectDisplayName_FallsBackToListing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/cli-auth/validate"):
			_, _ = w.Write([]byte(`{"team_id":"tm_other","team_name_no_org":"wrong one","organization_name":"Org"}`))
		case strings.HasSuffix(r.URL.Path, "/teams"):
			_, _ = w.Write([]byte(`[{"id":"tm_1","name":"[Acme] the active one","mode":"outpost"}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	baseURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	client := &hookdeck.Client{BaseURL: baseURL, APIKey: "user-key", ProjectID: "tm_1"}

	FillProjectDisplayNameIfNeeded(client, client)

	assert.Equal(t, "the active one", client.ProjectName, "the key's own project must not be used when it is not the active one")
}
