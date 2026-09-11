package mcpcore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// projectsAPI stubs the endpoints the projects tool needs: the CLI key check and
// the project list.
func projectsAPI(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/2025-07-01/cli-auth/validate", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"user_id":   "usr_test",
			"user_name": "Test User",
			"team_id":   "proj_gateway",
			"team_mode": "inbound",
		})
	})
	mux.HandleFunc("/2025-07-01/teams", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "proj_gateway", "name": "[Acme] gateway-project", "mode": "inbound"},
			{"id": "proj_outpost", "name": "[Acme] outpost-project", "mode": "outpost"},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newProjectsServer(t *testing.T, api *httptest.Server, filter string) (*Server, *hookdeck.Client) {
	t.Helper()
	u, err := url.Parse(api.URL)
	require.NoError(t, err)
	client := &hookdeck.Client{BaseURL: u, APIKey: "test-key", ProjectID: "proj_gateway"}
	srv := NewServer(Options{
		Name:          "hookdeck-test",
		ToolPrefix:    "outpost",
		Client:        client,
		Config:        &config.Config{APIBaseURL: api.URL},
		ProjectFilter: filter,
	})
	return srv, client
}

func callProjects(t *testing.T, srv *Server, args map[string]any) (string, bool) {
	t.Helper()
	raw, err := json.Marshal(args)
	require.NoError(t, err)
	result, err := handleProjects(srv)(t.Context(), newCallToolRequest(string(raw)))
	require.NoError(t, err)
	return firstText(t, result), result.IsError
}

func TestProjectsTool_ProjectFilter(t *testing.T) {
	t.Run("list returns only projects of the server's type", func(t *testing.T) {
		api := projectsAPI(t)
		srv, _ := newProjectsServer(t, api, config.ProjectTypeOutpost)

		text, isErr := callProjects(t, srv, map[string]any{"action": "list"})
		require.False(t, isErr, text)
		assert.Contains(t, text, "outpost-project")
		assert.NotContains(t, text, "gateway-project")
	})

	t.Run("list is unfiltered when no type is configured", func(t *testing.T) {
		api := projectsAPI(t)
		srv, _ := newProjectsServer(t, api, "")

		text, isErr := callProjects(t, srv, map[string]any{"action": "list"})
		require.False(t, isErr, text)
		assert.Contains(t, text, "outpost-project")
		assert.Contains(t, text, "gateway-project")
	})

	t.Run("use switches to a project of the server's type", func(t *testing.T) {
		api := projectsAPI(t)
		srv, client := newProjectsServer(t, api, config.ProjectTypeOutpost)

		text, isErr := callProjects(t, srv, map[string]any{"action": "use", "project_id": "proj_outpost"})
		require.False(t, isErr, text)
		assert.Equal(t, "proj_outpost", client.ProjectID)
		assert.Equal(t, "outpost-project", client.ProjectName)
	})

	t.Run("use refuses a project of another type and leaves the client alone", func(t *testing.T) {
		api := projectsAPI(t)
		srv, client := newProjectsServer(t, api, config.ProjectTypeOutpost)

		text, isErr := callProjects(t, srv, map[string]any{"action": "use", "project_id": "proj_gateway"})
		assert.True(t, isErr)
		assert.Contains(t, text, "outpost")
		assert.Equal(t, "proj_gateway", client.ProjectID, "the client must not be switched")
	})
}

func TestProjectsTool_ToolNamesFollowThePrefix(t *testing.T) {
	api := projectsAPI(t)
	srv, _ := newProjectsServer(t, api, config.ProjectTypeOutpost)

	assert.Equal(t, "hookdeck_projects", srv.ProjectsToolName())
	assert.Equal(t, "hookdeck_login", srv.LoginToolName())
	assert.Equal(t, "outpost_events", srv.ToolName("events"))
	assert.Equal(t, "outpost_", srv.ToolPrefix())

	def := srv.ProjectsToolDef("desc")
	assert.Equal(t, "hookdeck_projects", def.Tool.Name)
	assert.Equal(t, "desc", def.Tool.Description)
}
