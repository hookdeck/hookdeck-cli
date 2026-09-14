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
		if r.URL.Path != hookdeck.APIPathPrefix+"/projects" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "proj_x", "name": "[Acme] production", "type": "console"},
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

// A project-scoped credential (hookdeck ci key, dashboard API key) cannot list
// projects, so the display name has to come from /cli-auth/validate instead.
func TestFillProjectDisplayNameIfNeeded_FallsBackToValidateWhenListForbidden(t *testing.T) {
	var validateCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case hookdeck.APIPathPrefix + "/projects":
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "forbidden"})
		case hookdeck.APIPathPrefix + "/cli-auth/validate":
			validateCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"team_id":           "proj_x",
				"team_name_no_org":  "Shopify Demo",
				"team_name":         "[Demos] Shopify Demo",
				"organization_name": "Demos",
				"team_type":         "event_gateway",
			})
		default:
			http.NotFound(w, r)
		}
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
	require.Equal(t, 1, validateCalls)
	require.Equal(t, "Shopify Demo", client.ProjectName)
	require.Equal(t, "Demos", client.ProjectOrg)
}

// The key's project can differ from the profile's active project. Naming the
// key's project in that case would mislabel the project the tools act on.
func TestFillProjectDisplayNameIfNeeded_IgnoresValidateForDifferentProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case hookdeck.APIPathPrefix + "/projects":
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "forbidden"})
		case hookdeck.APIPathPrefix + "/cli-auth/validate":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"team_id":           "proj_other",
				"team_name_no_org":  "Other Project",
				"organization_name": "Demos",
			})
		default:
			http.NotFound(w, r)
		}
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
	require.Equal(t, "", client.ProjectName)
	require.Equal(t, "", client.ProjectOrg)
}

// When the project list resolves the name, /cli-auth/validate must not be called.
func TestFillProjectDisplayNameIfNeeded_SkipsValidateWhenListResolves(t *testing.T) {
	var validateCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case hookdeck.APIPathPrefix + "/projects":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "proj_x", "name": "[Acme] production", "type": "console"},
			})
		case hookdeck.APIPathPrefix + "/cli-auth/validate":
			validateCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"team_id":           "proj_x",
				"team_name_no_org":  "from-validate",
				"organization_name": "from-validate-org",
			})
		default:
			http.NotFound(w, r)
		}
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
	require.Equal(t, 0, validateCalls)
	require.Equal(t, "production", client.ProjectName)
	require.Equal(t, "Acme", client.ProjectOrg)
}

func TestFillProjectDisplayNameIfNeeded_NoOpWhenNameSet(t *testing.T) {
	client := &hookdeck.Client{ProjectID: "p", ProjectName: "already"}
	fillProjectDisplayNameIfNeeded(client)
	require.Equal(t, "already", client.ProjectName)
}
