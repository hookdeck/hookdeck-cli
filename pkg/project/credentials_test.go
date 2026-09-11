package project

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	configpkg "github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/stretchr/testify/require"
)

func TestEnsureUserAssociatedCredentials_rejectsProjectScopedKey(t *testing.T) {
	configpkg.ResetAPIClientForTesting()
	t.Cleanup(configpkg.ResetAPIClientForTesting)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, hookdeck.APIPathPrefix+"/cli-auth/validate", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(hookdeck.ValidateAPIKeyResponse{
			ProjectID:        "tm_ci",
			ProjectType:      "event_gateway",
			OrganizationName: "Org",
			OrganizationID:   "org_1",
			ProjectName:      "CI Project",
		})
	}))
	t.Cleanup(ts.Close)

	cfg := &configpkg.Config{
		APIBaseURL:        ts.URL,
		LogLevel:          "error",
		TelemetryDisabled: true,
	}
	cfg.Profile = configpkg.Profile{
		Name:   "default",
		APIKey: "hk_test_ci_key_abcdefghij",
		Config: cfg,
	}

	err := EnsureUserAssociatedCredentials(cfg)
	require.ErrorIs(t, err, ErrProjectScopedCredentials)
}
