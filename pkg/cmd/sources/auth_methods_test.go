package sources

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testdata/openapi_source_auth.json is cut from the 2026-09-01 OpenAPI spec: the source types
// that offer a choice of auth method, STRIPE, VAPI and FORMSTACK (one fixed method), and two of WEBHOOK's legacy
// provider methods (AIRCALL, and PROPERTY-FINDER whose schema name drops the hyphen).
func loadFixture(t *testing.T) map[string][]AuthMethod {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "openapi_source_auth.json"))
	require.NoError(t, err)
	methods, err := ParseSourceAuthMethods(data)
	require.NoError(t, err)
	return methods
}

func methodNames(ms []AuthMethod) []string {
	names := make([]string, 0, len(ms))
	for _, m := range ms {
		names = append(names, m.Name)
	}
	return names
}

func TestParseSourceAuthMethods(t *testing.T) {
	methods := loadFixture(t)

	assert.Equal(t, []string{"HMAC", "BASIC_AUTH", "API_KEY", "STANDARD_WEBHOOKS"}, methodNames(methods["WEBHOOK"]),
		"WEBHOOK keeps its own methods and drops legacy provider methods, PROPERTY-FINDER included")
	assert.Equal(t, []string{"HMAC", "BASIC_AUTH", "API_KEY", "STANDARD_WEBHOOKS"}, methodNames(methods["HTTP"]))
	assert.Equal(t, []string{"BASIC_AUTH", "API_KEY"}, methodNames(methods["ORDINAL"]))
	assert.Equal(t, []string{"BASIC_AUTH", "PROMETHEUS_ALERTMANAGER"}, methodNames(methods["PROMETHEUS_ALERTMANAGER"]),
		"a method named after its own type is kept")
	assert.Equal(t, []string{"STANDARD_WEBHOOKS", "GITLAB"}, methodNames(methods["GITLAB"]))
	for _, tp := range []string{"AZURE_EVENT_GRID", "SENDGRID_INBOUND", "BETTER_STACK", "GRAFANA"} {
		assert.NotEmpty(t, methods[tp], tp)
	}

	for _, tp := range []string{"STRIPE", "AIRCALL", "PROPERTYFINDER"} {
		assert.NotContains(t, methods, tp, "%s has one fixed method and takes no auth_type", tp)
	}
}

func TestAuthMethodsForNormalizesTypeName(t *testing.T) {
	methods := loadFixture(t)
	assert.NotNil(t, AuthMethodsFor(methods, "webhook"))
	assert.NotNil(t, AuthMethodsFor(methods, " Prometheus_Alertmanager "))
	assert.Nil(t, AuthMethodsFor(methods, "STRIPE"))
	assert.Nil(t, AuthMethodsFor(methods, ""))
}

func TestInferAuthType(t *testing.T) {
	methods := loadFixture(t)
	hmac := []string{"algorithm", "encoding", "header_key", "webhook_secret_key"}

	tests := []struct {
		sourceType string
		keys       []string
		want       string
	}{
		{"WEBHOOK", []string{"header_key", "api_key"}, "API_KEY"},
		{"WEBHOOK", []string{"username", "password"}, "BASIC_AUTH"},
		{"WEBHOOK", hmac, "HMAC"},
		{"WEBHOOK", []string{"webhook_secret_key"}, "STANDARD_WEBHOOKS"},
		// HTTP's spec lists no required fields, so a bare secret fits HMAC too; best fit wins.
		{"HTTP", []string{"webhook_secret_key"}, "STANDARD_WEBHOOKS"},
		{"HTTP", hmac, "HMAC"},
		{"HTTP", []string{"header_key", "api_key"}, "API_KEY"},
		{"ORDINAL", []string{"header_key", "api_key"}, "API_KEY"},
		{"ORDINAL", []string{"username", "password"}, "BASIC_AUTH"},
		{"AZURE_EVENT_GRID", []string{"header_key", "api_key"}, "API_KEY"},
		{"SENDGRID_INBOUND", hmac, "HMAC"},
		{"BETTER_STACK", []string{"header_key", "api_key"}, "API_KEY"},
		{"GRAFANA", []string{"header_key", "api_key"}, "API_KEY"},
		{"GRAFANA", []string{"webhook_secret_key", "header_key"}, "GRAFANA"},
		{"PROMETHEUS_ALERTMANAGER", []string{"api_key"}, "PROMETHEUS_ALERTMANAGER"},
		{"GITLAB", []string{"api_key"}, "GITLAB"},
		{"GITLAB", []string{"webhook_secret_key"}, "STANDARD_WEBHOOKS"},
		// No method takes these keys: leave auth_type unset and let the API report it.
		{"PROMETHEUS_ALERTMANAGER", []string{"header_key", "api_key"}, ""},
		{"WEBHOOK", []string{"api_key"}, ""},
		{"WEBHOOK", []string{"bogus"}, ""},
		{"WEBHOOK", nil, ""},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s %v", tt.sourceType, tt.keys), func(t *testing.T) {
			assert.Equal(t, tt.want, InferAuthType(AuthMethodsFor(methods, tt.sourceType), tt.keys))
		})
	}
}

func TestInferAuthTypeTieIsNotGuessed(t *testing.T) {
	methods := []AuthMethod{
		{Name: "A", Properties: []string{"api_key", "header_key"}},
		{Name: "B", Properties: []string{"api_key", "header_key"}},
	}
	assert.Equal(t, "", InferAuthType(methods, []string{"api_key"}))
}

func TestFetchSourceAuthMethodsCaches(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "openapi_source_auth.json"))
	require.NoError(t, err)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write(data)
	}))
	defer server.Close()

	originalURL, originalCache := openapiURL, authMethodsCacheFileName
	defer func() { openapiURL, authMethodsCacheFileName = originalURL, originalCache }()
	openapiURL = server.URL
	authMethodsCacheFileName = "hookdeck_source_auth_methods_test_" + t.Name() + ".json"
	cachePath := filepath.Join(os.TempDir(), authMethodsCacheFileName)
	os.Remove(cachePath)
	defer os.Remove(cachePath)

	first, err := FetchSourceAuthMethods()
	require.NoError(t, err)
	second, err := FetchSourceAuthMethods()
	require.NoError(t, err)
	assert.Equal(t, 1, requests, "second call is served from the cache")
	assert.Equal(t, first, second)
}

func TestFetchSourceAuthMethodsRejectsSpecWithoutMethods(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"components":{"schemas":{}}}`))
	}))
	defer server.Close()

	originalURL, originalCache := openapiURL, authMethodsCacheFileName
	defer func() { openapiURL, authMethodsCacheFileName = originalURL, originalCache }()
	openapiURL = server.URL
	authMethodsCacheFileName = "hookdeck_source_auth_methods_test_" + t.Name() + ".json"
	cachePath := filepath.Join(os.TempDir(), authMethodsCacheFileName)
	os.Remove(cachePath)
	defer os.Remove(cachePath)

	_, err := FetchSourceAuthMethods()
	require.Error(t, err)
	_, statErr := os.Stat(cachePath)
	assert.True(t, os.IsNotExist(statErr), "an empty result must not be cached")
}
