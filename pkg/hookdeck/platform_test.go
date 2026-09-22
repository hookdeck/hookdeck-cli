package hookdeck

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The routes each platform call reaches. apiPath prepends the version prefix,
// so a nested route built by joining two apiPath results produces
// /2026-09-01/projects/x/2026-09-01/custom_domains/y — which compiles, and
// fails only against the API.
func TestPlatformCallsHitTheRightPaths(t *testing.T) {
	var sawMethod, sawPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawMethod, sawPath = r.Method, r.URL.Path
		// The custom-domain listing is the one array response here.
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/custom_domains") {
			_, _ = w.Write([]byte(`[{"id":"dom_1","hostname":"portal.example.test"}]`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"x","name":"n"}`))
	}))
	defer server.Close()

	base, err := url.Parse(server.URL)
	require.NoError(t, err)
	client := &Client{BaseURL: base, APIKey: "k"}
	ctx := context.Background()

	cases := []struct {
		name   string
		call   func() error
		method string
		path   string
	}{
		{"get organization", func() error { _, e := client.GetOrganization(ctx); return e },
			http.MethodGet, APIPathPrefix + "/organizations/current"},
		{"update organization", func() error {
			n := "Acme"
			_, e := client.UpdateOrganization(ctx, &OrganizationUpdateRequest{Name: &n})
			return e
		}, http.MethodPut, APIPathPrefix + "/organizations/current"},
		{"get project", func() error { _, e := client.GetProject(ctx, "tm_1"); return e },
			http.MethodGet, APIPathPrefix + "/projects/tm_1"},
		{"create project", func() error { _, e := client.CreateProject(ctx, &ProjectCreateRequest{}); return e },
			http.MethodPost, APIPathPrefix + "/projects"},
		{"update project", func() error { _, e := client.UpdateProject(ctx, "tm_1", &ProjectUpdateRequest{}); return e },
			http.MethodPut, APIPathPrefix + "/projects/tm_1"},
		{"delete project", func() error { return client.DeleteProject(ctx, "tm_1") },
			http.MethodDelete, APIPathPrefix + "/projects/tm_1"},
		{"list custom domains", func() error { _, e := client.ListCustomDomains(ctx, "tm_1"); return e },
			http.MethodGet, APIPathPrefix + "/projects/tm_1/custom_domains"},
		{"add custom domain", func() error { _, e := client.AddCustomDomain(ctx, "tm_1", "portal.example.test"); return e },
			http.MethodPost, APIPathPrefix + "/projects/tm_1/custom_domains"},
		{"delete custom domain", func() error { return client.DeleteCustomDomain(ctx, "tm_1", "dom_1") },
			http.MethodDelete, APIPathPrefix + "/projects/tm_1/custom_domains/dom_1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sawMethod, sawPath = "", ""
			require.NoError(t, tc.call())
			assert.Equal(t, tc.method, sawMethod)
			assert.Equal(t, tc.path, sawPath, "the version prefix must appear exactly once")
		})
	}
}

// An id with a slash or a space must not be able to reach a different route.
func TestPlatformPathsEscapeTheirIDs(t *testing.T) {
	var sawPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.EscapedPath()
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "x"})
	}))
	defer server.Close()

	base, err := url.Parse(server.URL)
	require.NoError(t, err)
	client := &Client{BaseURL: base, APIKey: "k"}

	_, err = client.GetProject(context.Background(), "tm 1/../../evil")
	if err == nil {
		assert.NotContains(t, sawPath, "/../", "a traversal must not survive into the path")
	}
}
