package mcp

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func TestOrganizationReadAndWrite(t *testing.T) {
	var sawMethod string
	handlers := map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/organizations/current": func(w http.ResponseWriter, r *http.Request) {
			sawMethod = r.Method
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "org_1", "name": "Acme"})
		},
	}

	read := callTool(t, mockAPIWithClient(t, handlers), "hookdeck_organization_read",
		map[string]any{"action": "get"})
	assert.False(t, read.IsError, textContent(t, read))
	assert.Contains(t, textContent(t, read), "Acme")
	assert.Equal(t, http.MethodGet, sawMethod)

	write := callTool(t, mockAPIWithClientWriteEnabled(t, handlers), "hookdeck_organization_write",
		map[string]any{"action": "update", "name": "Acme Inc"})
	assert.False(t, write.IsError, textContent(t, write))
	assert.Equal(t, http.MethodPut, sawMethod)
}

// An empty PUT succeeds and changes nothing, so it must be refused rather than
// reported as a successful rename.
func TestOrganizationUpdateRefusesAnEmptyChange(t *testing.T) {
	session := mockAPIWithClientWriteEnabled(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/organizations/current": func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("an empty update must not reach the API")
		},
	})
	result := callTool(t, session, "hookdeck_organization_write", map[string]any{"action": "update"})
	require.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "name is required")
}

func TestProjectsCRUD(t *testing.T) {
	project := map[string]any{"id": "tm_1", "name": "Production", "type": "event_gateway"}
	var sawMethod string
	handlers := map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/projects/tm_1": func(w http.ResponseWriter, r *http.Request) {
			sawMethod = r.Method
			_ = json.NewEncoder(w).Encode(project)
		},
		hookdeck.APIPathPrefix + "/projects": func(w http.ResponseWriter, r *http.Request) {
			sawMethod = r.Method
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode([]any{project})
				return
			}
			_ = json.NewEncoder(w).Encode(project)
		},
	}

	get := callTool(t, mockAPIWithClient(t, handlers), "hookdeck_projects_read",
		map[string]any{"action": "get", "project_id": "tm_1"})
	assert.False(t, get.IsError, textContent(t, get))
	assert.Contains(t, textContent(t, get), "Production")

	write := mockAPIWithClientWriteEnabled(t, handlers)

	created := callTool(t, write, "hookdeck_projects_write",
		map[string]any{"action": "create", "name": "Staging"})
	assert.False(t, created.IsError, textContent(t, created))
	assert.Equal(t, http.MethodPost, sawMethod)

	updated := callTool(t, write, "hookdeck_projects_write",
		map[string]any{"action": "update", "project_id": "tm_1", "name": "Renamed"})
	assert.False(t, updated.IsError, textContent(t, updated))
	assert.Equal(t, http.MethodPut, sawMethod)

	deleted := callTool(t, write, "hookdeck_projects_write",
		map[string]any{"action": "delete", "project_id": "tm_1"})
	assert.False(t, deleted.IsError, textContent(t, deleted))
	assert.Contains(t, textContent(t, deleted), "deleted")
}

// A bare create is a valid API call that makes an unnamed project, and an empty
// update changes nothing while reporting success. Both are refused locally.
func TestProjectsWriteRefusesEmptyRequests(t *testing.T) {
	session := mockAPIWithClientWriteEnabled(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/projects": func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("an unnamed create must not reach the API")
		},
		hookdeck.APIPathPrefix + "/projects/tm_1": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPut {
				t.Fatalf("an empty update must not reach the API")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "tm_1", "type": "event_gateway"})
		},
	})

	create := callTool(t, session, "hookdeck_projects_write", map[string]any{"action": "create"})
	require.True(t, create.IsError)
	assert.Contains(t, textContent(t, create), "name is required")

	update := callTool(t, session, "hookdeck_projects_write",
		map[string]any{"action": "update", "project_id": "tm_1"})
	require.True(t, update.IsError)
	assert.Contains(t, textContent(t, update), "nothing to update")
}

// use is available without --allow-write: a read-only session stuck in one
// project cannot investigate another.
func TestProjectsUseIsReachableInReadOnlyMode(t *testing.T) {
	tools := listTools(t, connectInMemory(t, newTestClient("https://api.hookdeck.com", "k")))
	require.Contains(t, tools, "hookdeck_projects_use")
	assert.Equal(t, []string{"use"}, actionEnum(t, tools["hookdeck_projects_use"]))
}

// API key management is excluded from MCP by decision. This asserts it at the
// registration level, not just on the spec: no tool, in either mode, on either
// server, may offer one.
func TestNoAPIKeyToolIsRegistered(t *testing.T) {
	for _, writeEnabled := range []bool{false, true} {
		tools := listTools(t, connectInMemoryWithMode(t, newTestClient("https://api.hookdeck.com", "k"), writeEnabled))
		for name, tool := range tools {
			assert.NotContains(t, strings.ToLower(name), "api_key",
				"API keys must not be reachable from MCP; use the CLI or the dashboard")
			assert.NotContains(t, strings.ToLower(name), "apikey")
			for _, action := range actionEnum(t, tool) {
				assert.NotContains(t, strings.ToLower(action), "key",
					"%s offers a key-shaped action (%s); an agent able to mint a credential "+
						"can grant itself access this server would refuse", name, action)
			}
		}
	}
}
