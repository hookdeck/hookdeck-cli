package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func TestResolveConnectionID_ByIDPrefix(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc(hookdeck.APIPathPrefix+"/connections/web_resolve1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "web_resolve1", "team_id": "tm_1"})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	baseURL, err := url.Parse(srv.URL)
	require.NoError(t, err)
	client := &hookdeck.Client{BaseURL: baseURL, APIKey: "k"}

	id, err := resolveConnectionID(context.Background(), client, "web_resolve1")
	require.NoError(t, err)
	assert.Equal(t, "web_resolve1", id)
}

func TestResolveConnectionID_ByName(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc(hookdeck.APIPathPrefix+"/connections", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		assert.Equal(t, "my-conn", r.URL.Query().Get("name"))
		resp := map[string]any{
			"models": []map[string]any{{"id": "web_named", "team_id": "tm_1"}},
			"pagination": map[string]any{
				"limit": float64(100),
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	baseURL, err := url.Parse(srv.URL)
	require.NoError(t, err)
	client := &hookdeck.Client{BaseURL: baseURL, APIKey: "k"}

	id, err := resolveConnectionID(context.Background(), client, "my-conn")
	require.NoError(t, err)
	assert.Equal(t, "web_named", id)
}

func TestResolveConnectionID_WebPrefix404FallsBackToNameLookup(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc(hookdeck.APIPathPrefix+"/connections/web_stale", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "not found"})
	})
	mux.HandleFunc(hookdeck.APIPathPrefix+"/connections", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "web_stale", r.URL.Query().Get("name"))
		resp := map[string]any{
			"models": []map[string]any{{"id": "web_real", "team_id": "tm_1"}},
			"pagination": map[string]any{
				"limit": float64(100),
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	baseURL, err := url.Parse(srv.URL)
	require.NoError(t, err)
	client := &hookdeck.Client{BaseURL: baseURL, APIKey: "k"}

	id, err := resolveConnectionID(context.Background(), client, "web_stale")
	require.NoError(t, err)
	assert.Equal(t, "web_real", id)
}
