package sources

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The fixture (testdata/openapi_source_auth.json, cut from the 2026-09-01 spec) holds, for
// FetchSourceTypes: the SourceTypeConfig umbrella with its x-required markers, and types that
// require verification (AIRCALL, CLOUDSIGNAL, ALIPAY, TIKTOK_SHOP, VERCEL_LOG_DRAINS), types
// where it is optional (STRIPE, GITHUB, POSTMARK, ACUMATICA), types whose config is required
// for a handshake rather than verification (STRAVA, ONFLEET, AZURE_EVENT_GRID), and types
// offering a choice of method (WEBHOOK, HTTP, ...).

func loadSpecFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "openapi_source_auth.json"))
	require.NoError(t, err)
	return data
}

func parseFixtureSourceTypes(t *testing.T) map[string]SourceType {
	t.Helper()
	types, err := ParseSourceTypes(loadSpecFixture(t))
	require.NoError(t, err)
	return types
}

func lookup(t *testing.T, types map[string]SourceType, name string) SourceType {
	t.Helper()
	st, ok := SourceTypeFor(types, name)
	require.True(t, ok, "%s is a source type", name)
	return st
}

func TestParseSourceTypesListsEveryTypeConfigSchema(t *testing.T) {
	types := parseFixtureSourceTypes(t)

	assert.Len(t, types, 23, "one per SourceTypeConfig<TYPE> schema, not the umbrella")
	assert.Equal(t, "PROPERTY-FINDER", lookup(t, types, "PROPERTY-FINDER").Name,
		"named as the API spells it, although its schema name drops the hyphen")
	assert.Equal(t, "STRIPE", lookup(t, types, " stripe ").Name, "lookup ignores case and spaces")
	_, ok := SourceTypeFor(types, "NOT_A_TYPE")
	assert.False(t, ok)
}

func TestParseSourceTypesAuthRules(t *testing.T) {
	types := parseFixtureSourceTypes(t)

	tests := []struct {
		name         string
		required     bool
		fields       []string
		requiredKeys []string
	}{
		// Verification required: x-required, one fixed method with fields.
		{"AIRCALL", true, []string{"webhook_secret_key"}, []string{"webhook_secret_key"}},
		{"CLOUDSIGNAL", true, []string{"api_key"}, []string{"api_key"}},
		{"ALIPAY", true, []string{"public_key"}, []string{"public_key"}},
		{"TIKTOK_SHOP", true, []string{"app_key", "webhook_secret_key"}, []string{"app_key", "webhook_secret_key"}},
		{"VERCEL_LOG_DRAINS", true, []string{"webhook_secret_key"}, []string{}},

		// Verification optional: the fields are still required whenever auth is sent.
		{"STRIPE", false, []string{"webhook_secret_key"}, []string{"webhook_secret_key"}},
		{"GITHUB", false, []string{"webhook_secret_key"}, []string{"webhook_secret_key"}},
		{"POSTMARK", false, []string{"password", "username"}, []string{"password", "username"}},
		{"ACUMATICA", false, []string{"api_key", "header_key"}, []string{"api_key", "header_key"}},

		// x-required for a handshake: STRAVA's auth has no fields, AZURE_EVENT_GRID offers a
		// choice of method (the API accepts auth_type null with no auth).
		{"STRAVA", false, []string{}, []string{}},
		{"AZURE_EVENT_GRID", false, nil, nil},

		// A choice of method: fields depend on auth_type, which FetchSourceAuthMethods covers.
		{"WEBHOOK", false, nil, nil},
		{"HTTP", false, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := lookup(t, types, tt.name)
			assert.Equal(t, tt.required, st.AuthRequired, "AuthRequired")
			assert.Equal(t, tt.fields, st.AuthFields, "AuthFields")
			assert.Equal(t, tt.requiredKeys, st.RequiredAuthFields, "RequiredAuthFields")
		})
	}
}

// ONFLEET's config is required for its handshake, not verification, but the spec cannot tell
// the two apart for a type whose auth has fields. The cmd package corrects for it; this pins
// the limitation so a spec that starts telling them apart shows up here.
func TestParseSourceTypesReadsOnfleetAsAuthRequired(t *testing.T) {
	st := lookup(t, parseFixtureSourceTypes(t), "ONFLEET")
	assert.True(t, st.AuthRequired)
}

func TestParseSourceTypesHonoursRequiredOnTypeSchema(t *testing.T) {
	spec := `{"components":{"schemas":{
		"SourceTypeConfigACME":{"x-docs-type":"ACME","required":["auth"],"properties":{"auth":{"$ref":"#/components/schemas/AcmeAuth"}}},
		"AcmeAuth":{"properties":{"token":{"type":"string"}},"required":["token"]}
	}}}`
	types, err := ParseSourceTypes([]byte(spec))
	require.NoError(t, err)
	st := lookup(t, types, "ACME")
	assert.True(t, st.AuthRequired, "a required list naming auth counts without x-required")
	assert.Equal(t, []string{"token"}, st.RequiredAuthFields)
}

// The spec shape the previous parser read. It is gone from the API, which is how
// FetchSourceTypes came to return an empty map that the old tests, built on this shape,
// never noticed.
const legacySourceCreateRequestSpec = `{"components":{"schemas":{"SourceCreateRequest":{"properties":{
	"type":{"enum":["STRIPE","GITHUB"]},
	"verification_configs":{"oneOf":[{"properties":{"webhook_secret":{"required":["secret"]}},"required":["webhook_secret"]}]}
}}}}}`

func TestParseSourceTypesFindsNothingInLegacyShape(t *testing.T) {
	types, err := ParseSourceTypes([]byte(legacySourceCreateRequestSpec))
	require.NoError(t, err)
	assert.Empty(t, types)
}

// serveSpec points FetchSourceTypes at a test server returning body (per request, so a test
// can change it) and at a fresh cache directory. It returns the request count and cache path.
func serveSpec(t *testing.T, body *string) (*int, string) {
	t.Helper()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte(*body))
	}))
	t.Cleanup(server.Close)

	originalURL := openapiURL
	t.Cleanup(func() { openapiURL = originalURL })
	openapiURL = server.URL

	t.Setenv("TMPDIR", t.TempDir()) // os.TempDir, where the cache lives
	return &requests, filepath.Join(os.TempDir(), cacheFileName)
}

func TestFetchSourceTypesParsesAndCaches(t *testing.T) {
	body := string(loadSpecFixture(t))
	requests, cachePath := serveSpec(t, &body)

	first, err := FetchSourceTypes()
	require.NoError(t, err)
	assert.True(t, lookup(t, first, "AIRCALL").AuthRequired)
	assert.Equal(t, 1, *requests)
	_, err = os.Stat(cachePath)
	require.NoError(t, err, "a good parse is cached")

	second, err := FetchSourceTypes()
	require.NoError(t, err)
	assert.Equal(t, 1, *requests, "second call is served from the cache")
	assert.Equal(t, first, second)

	expired := time.Now().Add(-(cacheTTL + time.Hour))
	require.NoError(t, os.Chtimes(cachePath, expired, expired))
	_, err = FetchSourceTypes()
	require.NoError(t, err)
	assert.Equal(t, 2, *requests, "an expired cache is refetched")
}

func TestFetchSourceTypesDoesNotCacheEmptyOrFailedParse(t *testing.T) {
	for name, spec := range map[string]string{
		"no source type schemas": legacySourceCreateRequestSpec,
		"invalid JSON":           `{"components":`,
	} {
		t.Run(name, func(t *testing.T) {
			body := spec
			requests, cachePath := serveSpec(t, &body)

			_, err := FetchSourceTypes()
			require.Error(t, err)
			_, statErr := os.Stat(cachePath)
			assert.True(t, os.IsNotExist(statErr), "nothing is cached")

			body = string(loadSpecFixture(t))
			types, err := FetchSourceTypes()
			require.NoError(t, err, "the next call fetches again")
			assert.NotEmpty(t, types)
			assert.Equal(t, 2, *requests)
		})
	}
}

func TestFetchSourceTypesIgnoresEmptyCacheFile(t *testing.T) {
	body := string(loadSpecFixture(t))
	requests, cachePath := serveSpec(t, &body)
	// What the SourceCreateRequest parser left behind against the current spec.
	require.NoError(t, os.WriteFile(cachePath, []byte("{}"), 0o600))

	types, err := FetchSourceTypes()
	require.NoError(t, err)
	assert.NotEmpty(t, types)
	assert.Equal(t, 1, *requests, "an empty cache file is not served")
}

// Source types and auth methods are two views of one spec, so a cold start downloads it once,
// whichever is read first.
func TestSourceTypesAndAuthMethodsShareOneDownload(t *testing.T) {
	for name, first := range map[string]func() error{
		"types first":   func() error { _, err := FetchSourceTypes(); return err },
		"methods first": func() error { _, err := FetchSourceAuthMethods(); return err },
	} {
		t.Run(name, func(t *testing.T) {
			body := string(loadSpecFixture(t))
			requests, _ := serveSpec(t, &body)

			require.NoError(t, first())
			types, err := FetchSourceTypes()
			require.NoError(t, err)
			methods, err := FetchSourceAuthMethods()
			require.NoError(t, err)
			assert.NotEmpty(t, types)
			assert.NotEmpty(t, methods)
			assert.Equal(t, 1, *requests)
		})
	}
}

// A spec server that never answers must not hang a command: the download gives up.
func TestDownloadOpenAPISpecTimesOut(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	t.Cleanup(func() { close(release); server.Close() })

	originalURL, originalClient := openapiURL, specClient
	t.Cleanup(func() { openapiURL, specClient = originalURL, originalClient })
	openapiURL = server.URL
	specClient = &http.Client{Timeout: 100 * time.Millisecond}

	start := time.Now()
	_, err := downloadOpenAPISpec()
	require.Error(t, err)
	assert.Less(t, time.Since(start), 5*time.Second)
	assert.Equal(t, 10*time.Second, originalClient.Timeout, "the shipped client carries the timeout")
}
