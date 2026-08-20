package outposttypes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

const schemaBody = `[
  {
    "type": "webhook",
    "label": "Webhook",
    "config_fields": [
      {"key": "url", "type": "text", "label": "URL", "required": true, "pattern": "^https?://"}
    ],
    "credential_fields": [
      {"key": "secret", "type": "text", "label": "Secret", "sensitive": true}
    ]
  },
  {
    "type": "aws_sqs",
    "label": "AWS SQS",
    "config_fields": [
      {"key": "queue_url", "type": "text", "label": "Queue URL", "required": true},
      {"key": "region", "type": "select", "label": "Region", "options": [
        {"label": "US East 1", "value": "us-east-1"},
        {"label": "EU West 2", "value": "eu-west-2"}
      ]}
    ],
    "credential_fields": []
  }
]`

// newTestClient points a client at a stub server and isolates the on-disk cache
// so tests never read or write a real user's temp files.
func newTestClient(t *testing.T, handler http.HandlerFunc) *hookdeck.Client {
	t.Helper()

	t.Setenv("TMPDIR", t.TempDir())

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	baseURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	return &hookdeck.Client{BaseURL: baseURL, APIKey: "test-key", ProjectID: "tm_test"}
}

func TestFetchDestinationTypes(t *testing.T) {
	t.Run("fetches and returns schemas", func(t *testing.T) {
		client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, hookdeck.APIPathPrefix+"/destination-types", r.URL.Path)
			_, _ = w.Write([]byte(schemaBody))
		})

		schemas, err := FetchDestinationTypes(context.Background(), client)
		require.NoError(t, err)
		require.Len(t, schemas, 2)
		assert.Equal(t, []string{"aws_sqs", "webhook"}, TypeNames(schemas))

		// A select field's options are {label, value} objects, not bare strings.
		// The original stub encoded them as strings, so the unit tests passed
		// while decoding the real API failed — keep this assertion faithful.
		sqs, ok := Find(schemas, "aws_sqs")
		require.True(t, ok)
		var region Field
		for _, f := range sqs.ConfigFields {
			if f.Key == "region" {
				region = f
			}
		}
		require.Len(t, region.Options, 2)
		assert.Equal(t, "US East 1", region.Options[0].Label)
		assert.Equal(t, []string{"us-east-1", "eu-west-2"}, region.OptionValues())
	})

	t.Run("serves a second call from cache without hitting the API", func(t *testing.T) {
		var calls int
		client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			calls++
			_, _ = w.Write([]byte(schemaBody))
		})

		_, err := FetchDestinationTypes(context.Background(), client)
		require.NoError(t, err)
		_, err = FetchDestinationTypes(context.Background(), client)
		require.NoError(t, err)

		assert.Equal(t, 1, calls, "the second call should come from the cache")
	})

	t.Run("refetches once the cache has expired", func(t *testing.T) {
		var calls int
		client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			calls++
			_, _ = w.Write([]byte(schemaBody))
		})

		_, err := FetchDestinationTypes(context.Background(), client)
		require.NoError(t, err)

		// Backdate the cache past its TTL rather than waiting it out.
		stale := time.Now().Add(-2 * cacheTTL)
		require.NoError(t, os.Chtimes(cachePathFor(client), stale, stale))

		_, err = FetchDestinationTypes(context.Background(), client)
		require.NoError(t, err)
		assert.Equal(t, 2, calls)
	})

	t.Run("returns an error the caller can warn on rather than caching a failure", func(t *testing.T) {
		client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"boom"}`))
		})

		schemas, err := FetchDestinationTypes(context.Background(), client)
		require.Error(t, err)
		assert.Nil(t, schemas)

		_, cached := readCache(cachePathFor(client))
		assert.False(t, cached, "a failed fetch must not populate the cache")
	})

	t.Run("caches separately per project", func(t *testing.T) {
		client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(schemaBody))
		})

		first := cachePathFor(client)
		client.ProjectID = "tm_other"
		second := cachePathFor(client)

		assert.NotEqual(t, first, second, "one project's schemas must not be served to another")
	})

	t.Run("caches separately per API host", func(t *testing.T) {
		client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})

		first := cachePathFor(client)
		other, err := url.Parse("https://outpost.elsewhere.test")
		require.NoError(t, err)
		client.BaseURL = other

		assert.NotEqual(t, first, cachePathFor(client), "a different host must not reuse the cache")
	})
}

func TestFind(t *testing.T) {
	t.Parallel()

	schemas := []Schema{{Type: "webhook"}, {Type: "aws_sqs"}}

	found, ok := Find(schemas, "WEBHOOK")
	assert.True(t, ok, "type matching should be case-insensitive")
	assert.Equal(t, "webhook", found.Type)

	_, ok = Find(schemas, "kafka")
	assert.False(t, ok)
}

func TestValidateFields(t *testing.T) {
	t.Parallel()

	configFields := []Field{
		{Key: "url", Required: true, Pattern: "^https?://"},
		{Key: "region", Options: []hookdeck.OutpostDestinationTypeOption{
			{Label: "US East 1", Value: "us-east-1"},
			{Label: "EU West 2", Value: "eu-west-2"},
		}},
		{Key: "note"},
	}

	t.Run("accepts valid values", func(t *testing.T) {
		err := ValidateFields(configFields, map[string]interface{}{
			"url":    "https://example.com/hook",
			"region": "eu-west-2",
		}, "config")
		assert.NoError(t, err)
	})

	t.Run("reports a missing required field using its flag name", func(t *testing.T) {
		err := ValidateFields(configFields, map[string]interface{}{}, "config")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--config url=<value> is required")
	})

	t.Run("treats a blank required value as missing", func(t *testing.T) {
		err := ValidateFields(configFields, map[string]interface{}{"url": "  "}, "config")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--config url=<value> is required")
	})

	t.Run("rejects an unknown field", func(t *testing.T) {
		err := ValidateFields(configFields, map[string]interface{}{
			"url":     "https://example.com",
			"unknown": "x",
		}, "config")
		require.Error(t, err)
		assert.Contains(t, err.Error(), `"unknown" is not a valid config field`)
	})

	t.Run("uses the schema key verbatim, matching the key=value flag form", func(t *testing.T) {
		err := ValidateFields([]Field{{Key: "queue_url", Required: true}}, map[string]interface{}{}, "config")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--config queue_url=<value> is required")
	})

	t.Run("rejects a value outside the declared options", func(t *testing.T) {
		err := ValidateFields(configFields, map[string]interface{}{
			"url":    "https://example.com",
			"region": "mars-1",
		}, "config")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--config region must be one of: us-east-1, eu-west-2")
	})

	t.Run("rejects a value failing the declared pattern", func(t *testing.T) {
		err := ValidateFields(configFields, map[string]interface{}{"url": "ftp://example.com"}, "config")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--config url does not match the expected format")
	})

	t.Run("ignores a pattern the schema declares but Go cannot compile", func(t *testing.T) {
		// A broken schema is not the user's fault, so it must not block a command.
		err := ValidateFields([]Field{{Key: "url", Pattern: "(unclosed"}}, map[string]interface{}{
			"url": "anything",
		}, "config")
		assert.NoError(t, err)
	})

	t.Run("names the credential group when validating credentials", func(t *testing.T) {
		err := ValidateFields([]Field{{Key: "secret", Required: true}}, map[string]interface{}{}, "credential")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--credential secret=<value> is required")
	})
}

// The update endpoint is a merge patch, so a field nobody mentioned is not
// missing — it is unchanged.
func TestValidateSuppliedFields(t *testing.T) {
	t.Parallel()

	configFields := []Field{
		{Key: "url", Required: true, Pattern: "^https?://"},
		{Key: "region", Options: []hookdeck.OutpostDestinationTypeOption{
			{Label: "US East 1", Value: "us-east-1"},
			{Label: "EU West 2", Value: "eu-west-2"},
		}},
		{Key: "note"},
	}

	t.Run("an omitted required field is not an error", func(t *testing.T) {
		err := ValidateSuppliedFields(configFields, map[string]interface{}{"note": "x"}, "config")
		assert.NoError(t, err, "requiring url here makes a partial update impossible")
	})

	t.Run("nothing supplied is not an error", func(t *testing.T) {
		assert.NoError(t, ValidateSuppliedFields(configFields, map[string]interface{}{}, "config"))
	})

	t.Run("a supplied required field may not be blanked", func(t *testing.T) {
		// Clearing a required field is not a partial update.
		err := ValidateSuppliedFields(configFields, map[string]interface{}{"url": "  "}, "config")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--config url=<value> is required")
	})

	t.Run("an unknown field is still rejected", func(t *testing.T) {
		err := ValidateSuppliedFields(configFields, map[string]interface{}{"unknown": "x"}, "config")
		require.Error(t, err)
		assert.Contains(t, err.Error(), `"unknown" is not a valid config field`)
	})

	t.Run("options and patterns are still checked", func(t *testing.T) {
		err := ValidateSuppliedFields(configFields, map[string]interface{}{"region": "mars-1"}, "config")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--config region must be one of")

		err = ValidateSuppliedFields(configFields, map[string]interface{}{"url": "ftp://example.com"}, "config")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--config url does not match the expected format")
	})

	t.Run("rotating a credential does not require the others", func(t *testing.T) {
		fields := []Field{{Key: "secret", Required: true}, {Key: "key", Required: true}}
		err := ValidateSuppliedFields(fields, map[string]interface{}{"secret": "new"}, "credential")
		assert.NoError(t, err)
	})
}

// The schema publishes a default per field and the CLI prints it in help, but
// nothing was sending it and the API treats an absent key as unset. Following
// our own printed example for a rabbitmq destination therefore stored
// tls: "false" and sent SASL credentials in the clear.
func TestApplyDefaults(t *testing.T) {
	// Both spellings are real: rabbitmq's tls default is "on", kafka's is
	// "true". The API normalises both, so the schema value is passed through
	// rather than mapped.
	fields := []Field{
		{Key: "server_url", Required: true},
		{Key: "tls", Type: "checkbox", Default: "on"},
		{Key: "storage_class", Default: "STANDARD"},
		{Key: "topic"},
	}

	t.Run("fills in a default the caller omitted", func(t *testing.T) {
		values, applied := ApplyDefaults(fields, map[string]interface{}{"server_url": "r:5672"})
		assert.Equal(t, "on", values["tls"], "TLS must be on when the schema says so and the caller said nothing")
		assert.Equal(t, "STANDARD", values["storage_class"])
		assert.ElementsMatch(t, []string{"tls", "storage_class"}, applied)
	})

	t.Run("never overrides what the caller supplied", func(t *testing.T) {
		values, applied := ApplyDefaults(fields, map[string]interface{}{"tls": "false"})
		assert.Equal(t, "false", values["tls"], "an explicit opt-out must survive")
		assert.NotContains(t, applied, "tls")
	})

	t.Run("an explicitly empty value is still the caller's choice", func(t *testing.T) {
		values, applied := ApplyDefaults(fields, map[string]interface{}{"tls": ""})
		assert.Equal(t, "", values["tls"])
		assert.NotContains(t, applied, "tls")
	})

	t.Run("fields without a default are left absent", func(t *testing.T) {
		values, _ := ApplyDefaults(fields, map[string]interface{}{})
		_, present := values["topic"]
		assert.False(t, present, "a field with no default must not be invented")
	})

	t.Run("works from a nil map", func(t *testing.T) {
		values, applied := ApplyDefaults(fields, nil)
		require.NotNil(t, values)
		assert.Equal(t, "on", values["tls"])
		assert.Len(t, applied, 2)
	})

	t.Run("reports nothing when there is nothing to apply", func(t *testing.T) {
		_, applied := ApplyDefaults([]Field{{Key: "url", Required: true}}, map[string]interface{}{"url": "x"})
		assert.Empty(t, applied)
	})
}
