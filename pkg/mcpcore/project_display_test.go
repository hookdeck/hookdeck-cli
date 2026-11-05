package mcpcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stalledAPI answers nothing until the request is abandoned, which is what a
// slow API looks like to a client with no deadline of its own.
func stalledAPI(t *testing.T) (*hookdeck.Client, *atomic.Int64) {
	t.Helper()
	var requests atomic.Int64
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(func() {
		close(release)
		server.Close()
	})
	baseURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	return &hookdeck.Client{BaseURL: baseURL, APIKey: "k", ProjectID: "tm_1"}, &requests
}

func shortenProjectDisplayLookup(t *testing.T, d time.Duration) {
	t.Helper()
	previous := projectDisplayLookupTimeout
	projectDisplayLookupTimeout = d
	t.Cleanup(func() { projectDisplayLookupTimeout = previous })
}

// TestFillProjectDisplayName_GivesUpOnAStalledAPI: the lookup is cosmetic, so a
// slow API must cost at most the timeout, not however long the API takes.
func TestFillProjectDisplayName_GivesUpOnAStalledAPI(t *testing.T) {
	shortenProjectDisplayLookup(t, 200*time.Millisecond)
	client, requests := stalledAPI(t)

	start := time.Now()
	FillProjectDisplayNameIfNeeded(context.Background(), client, client)

	assert.Less(t, time.Since(start), 3*time.Second, "the lookup must give up at its timeout")
	assert.GreaterOrEqual(t, requests.Load(), int64(1), "the lookup should have been attempted")
	assert.Empty(t, client.ProjectName, "a lookup that gave up leaves the name for the next call to fill")
	assert.Empty(t, client.ProjectOrg)
}

// TestToolCallIsNotHeldByAStalledDisplayNameLookup is the case the acceptance
// suite kept hitting: a tool that never needs the API (here, one standing in for
// a write refused in read-only mode) answered nothing, because the lookup ahead
// of it waited on the API with no deadline.
func TestToolCallIsNotHeldByAStalledDisplayNameLookup(t *testing.T) {
	shortenProjectDisplayLookup(t, 200*time.Millisecond)
	client, _ := stalledAPI(t)
	s := &Server{client: client, accountClient: client}

	refusal := mcpsdk.ToolHandler(func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		return &mcpsdk.CallToolResult{IsError: true}, nil
	})

	done := make(chan *mcpsdk.CallToolResult, 1)
	go func() {
		result, _ := s.wrapWithTelemetry("gateway_sources_read", refusal)(context.Background(), newCallToolRequest(`{"action":"delete"}`))
		done <- result
	}()

	select {
	case result := <-done:
		require.NotNil(t, result)
		assert.True(t, result.IsError)
	case <-time.After(5 * time.Second):
		t.Fatal("the tool call is still waiting on the display-name lookup")
	}
}

// TestFillProjectDisplayName_StopsWhenTheCallIsCancelled: a client that cancels
// the tool call must not leave the lookup running to its own timeout.
func TestFillProjectDisplayName_StopsWhenTheCallIsCancelled(t *testing.T) {
	shortenProjectDisplayLookup(t, time.Minute)
	client, _ := stalledAPI(t)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	FillProjectDisplayNameIfNeeded(ctx, client, client)

	assert.Less(t, time.Since(start), 3*time.Second, "cancelling the call must end the lookup")
	assert.Empty(t, client.ProjectName)
}

func TestFillProjectDisplayNameIfNeeded_SetsNameFromAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2026-09-01/projects" {
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
	FillProjectDisplayNameIfNeeded(context.Background(), client, client)
	require.Equal(t, "Acme", client.ProjectOrg)
	require.Equal(t, "production", client.ProjectName)
}

func TestFillProjectDisplayNameIfNeeded_NoOpWhenNameSet(t *testing.T) {
	client := &hookdeck.Client{ProjectID: "p", ProjectName: "already"}
	FillProjectDisplayNameIfNeeded(context.Background(), client, client)
	require.Equal(t, "already", client.ProjectName)
}

// A product API served from its own host cannot answer the project list, so the
// lookup has to go to the account API while the product client is the one
// updated.
func TestFillProjectDisplayNameIfNeeded_LooksUpThroughTheAccountClient(t *testing.T) {
	accountAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2026-09-01/projects" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "proj_x", "name": "[Acme] production", "type": "outpost"},
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

	FillProjectDisplayNameIfNeeded(context.Background(), account, product)
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
		case strings.HasSuffix(r.URL.Path, "/projects"):
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

	FillProjectDisplayNameIfNeeded(context.Background(), client, client)

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
		case strings.HasSuffix(r.URL.Path, "/projects"):
			_, _ = w.Write([]byte(`[{"id":"tm_1","name":"[Acme] the active one","type":"outpost"}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	baseURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	client := &hookdeck.Client{BaseURL: baseURL, APIKey: "user-key", ProjectID: "tm_1"}

	FillProjectDisplayNameIfNeeded(context.Background(), client, client)

	assert.Equal(t, "the active one", client.ProjectName, "the key's own project must not be used when it is not the active one")
}
