package sources

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const mockOpenAPISpec = `
{
  "components": {
    "schemas": {
      "SourceCreateRequest": {
        "properties": {
          "type": {
            "enum": [
              "STRIPE",
              "GITHUB",
              "TWILIO"
            ]
          },
          "verification_configs": {
            "oneOf": [
              {
                "properties": {
                  "webhook_secret": {
                    "required": ["secret"]
                  }
                },
                "required": ["webhook_secret"]
              },
              {
                "properties": {
                  "api_key": {
                    "required": ["key"]
                  }
                },
                "required": ["api_key"]
              }
            ]
          }
        }
      }
    }
  }
}
`

func TestFetchSourceTypes_Parsing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, mockOpenAPISpec)
	}))
	defer server.Close()

	// Temporarily override the openapiURL to point to the mock server
	originalURL := openapiURL
	defer func() { openapiURL = originalURL }()
	openapiURL = server.URL

	// Clear any existing cache to ensure we hit the mock server
	cachePath := filepath.Join(os.TempDir(), cacheFileName)
	os.Remove(cachePath)

	sourceTypes, err := FetchSourceTypes()

	assert.NoError(t, err)
	assert.NotNil(t, sourceTypes)
	assert.Len(t, sourceTypes, 3)

	// The parsing logic is simplified and has a manual correction for STRIPE, let's test that
	stripeType, ok := sourceTypes["STRIPE"]
	assert.True(t, ok)
	assert.Equal(t, "STRIPE", stripeType.Name)
	assert.Equal(t, "webhook_secret", stripeType.AuthScheme)
	assert.Equal(t, []string{"secret"}, stripeType.RequiredFields)

	githubType, ok := sourceTypes["GITHUB"]
	assert.True(t, ok)
	assert.Equal(t, "GITHUB", githubType.Name)
	// This assertion depends on the simplified parsing logic which assigns the first scheme found
	assert.Equal(t, "webhook_secret", githubType.AuthScheme)
}

func TestFetchSourceTypes_Caching(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		fmt.Fprint(w, mockOpenAPISpec)
	}))
	defer server.Close()

	// Temporarily override the openapiURL
	originalURL := openapiURL
	defer func() { openapiURL = originalURL }()
	openapiURL = server.URL

	cachePath := filepath.Join(os.TempDir(), cacheFileName)
	os.Remove(cachePath) // Ensure no cache from previous runs

	// 1. First call: should fetch from the server and create a cache file
	sourceTypes1, err1 := FetchSourceTypes()
	assert.NoError(t, err1)
	assert.NotNil(t, sourceTypes1)
	assert.Equal(t, 1, requestCount, "Server should be hit on the first call")

	// Verify cache file was created
	_, err := os.Stat(cachePath)
	assert.NoError(t, err, "Cache file should exist after the first call")

	// 2. Second call: should load from cache, not hit the server
	sourceTypes2, err2 := FetchSourceTypes()
	assert.NoError(t, err2)
	assert.NotNil(t, sourceTypes2)
	assert.Equal(t, 1, requestCount, "Server should not be hit on the second call")
	assert.Equal(t, sourceTypes1, sourceTypes2, "Data from cache should match original data")

	// 3. Third call: after cache expires, should hit the server again
	// Manually set the modification time of the cache file to be older than the TTL
	oldTime := time.Now().Add(-(cacheTTL + time.Hour))
	err = os.Chtimes(cachePath, oldTime, oldTime)
	assert.NoError(t, err)

	sourceTypes3, err3 := FetchSourceTypes()
	assert.NoError(t, err3)
	assert.NotNil(t, sourceTypes3)
	assert.Equal(t, 2, requestCount, "Server should be hit again after cache expires")

	// Cleanup
	os.Remove(cachePath)
}
