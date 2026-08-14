package mcp

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Outpost API is served from its own host and does not answer account-level
// requests, so the projects tool has to list through the main Hookdeck API while
// switching the Outpost client. Getting this wrong is invisible until the next
// Outpost call silently uses the previous project.

func accountAPI(t *testing.T) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/2025-07-01/cli-auth/validate", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"user_id":   "usr_1",
			"user_name": "Test User",
			"team_id":   "proj_outpost",
			"team_mode": "outpost",
		})
	})
	mux.HandleFunc("/2025-07-01/teams", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "proj_outpost", "name": "[Acme] outpost-project", "mode": "outpost"},
			{"id": "proj_other", "name": "[Acme] second-outpost", "mode": "outpost"},
			{"id": "proj_gateway", "name": "[Acme] gateway-project", "mode": "inbound"},
		})
	})
	return mux
}

func TestProjectsTool_UsesTheAccountAPIAndSwitchesTheOutpostClient(t *testing.T) {
	account := mockAPI(t, map[string]http.HandlerFunc{
		"/2025-07-01/cli-auth/validate": accountAPI(t).ServeHTTP,
		"/2025-07-01/teams":             accountAPI(t).ServeHTTP,
	})
	// The Outpost API must never be asked for the project list.
	outpost := mockAPI(t, map[string]http.HandlerFunc{
		"/2025-07-01/teams": func(w http.ResponseWriter, r *http.Request) {
			t.Error("the Outpost API was asked to list projects")
		},
	})

	outpostClient := newTestClient(t, outpost.URL)
	accountClient := newTestClient(t, account.URL)
	session := connect(t, ServerOptions{Client: outpostClient, AccountClient: accountClient})

	t.Run("list returns only Outpost projects", func(t *testing.T) {
		result := callTool(t, session, "outpost_projects", map[string]any{"action": "list"})
		require.False(t, result.IsError, resultText(t, result))
		text := resultText(t, result)
		assert.Contains(t, text, "outpost-project")
		assert.Contains(t, text, "second-outpost")
		assert.NotContains(t, text, "gateway-project", "this server cannot serve a Gateway project")
	})

	t.Run("use switches the Outpost client, not just the account one", func(t *testing.T) {
		result := callTool(t, session, "outpost_projects", map[string]any{
			"action":     "use",
			"project_id": "proj_other",
		})
		require.False(t, result.IsError, resultText(t, result))
		assert.Equal(t, "proj_other", outpostClient.ProjectID,
			"later Outpost calls would otherwise still hit the previous project")
		assert.Equal(t, "proj_other", accountClient.ProjectID)
		assert.Equal(t, "second-outpost", outpostClient.ProjectName)
	})

	t.Run("use refuses a Gateway project", func(t *testing.T) {
		result := callTool(t, session, "outpost_projects", map[string]any{
			"action":     "use",
			"project_id": "proj_gateway",
		})
		require.True(t, result.IsError)
		assert.Contains(t, resultText(t, result), "outpost")
		assert.Equal(t, "proj_other", outpostClient.ProjectID, "the client must not have moved")
	})
}
