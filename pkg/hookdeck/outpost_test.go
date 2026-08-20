package hookdeck

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutpostTopicsUnmarshal(t *testing.T) {
	t.Parallel()

	t.Run("wildcard string decodes to a single element", func(t *testing.T) {
		var topics OutpostTopics
		require.NoError(t, json.Unmarshal([]byte(`"*"`), &topics))
		assert.Equal(t, OutpostTopics{"*"}, topics)
		assert.True(t, topics.IsWildcard())
	})

	t.Run("array decodes verbatim", func(t *testing.T) {
		var topics OutpostTopics
		require.NoError(t, json.Unmarshal([]byte(`["user.created","order.shipped"]`), &topics))
		assert.Equal(t, OutpostTopics{"user.created", "order.shipped"}, topics)
		assert.False(t, topics.IsWildcard())
	})

	t.Run("array containing a wildcard entry is not the wildcard form", func(t *testing.T) {
		var topics OutpostTopics
		require.NoError(t, json.Unmarshal([]byte(`["user.*","order.shipped"]`), &topics))
		assert.False(t, topics.IsWildcard())
	})

	t.Run("a non-string, non-array value is rejected", func(t *testing.T) {
		var topics OutpostTopics
		err := json.Unmarshal([]byte(`42`), &topics)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `topics must be "*" or an array of strings`)
	})
}

func TestOutpostTopicsMarshal(t *testing.T) {
	t.Parallel()

	t.Run("wildcard round-trips as a bare string", func(t *testing.T) {
		data, err := json.Marshal(OutpostTopics{"*"})
		require.NoError(t, err)
		assert.JSONEq(t, `"*"`, string(data))
	})

	t.Run("multiple topics marshal as an array", func(t *testing.T) {
		data, err := json.Marshal(OutpostTopics{"user.created", "order.shipped"})
		require.NoError(t, err)
		assert.JSONEq(t, `["user.created","order.shipped"]`, string(data))
	})

	t.Run("a single non-wildcard topic stays an array", func(t *testing.T) {
		data, err := json.Marshal(OutpostTopics{"user.created"})
		require.NoError(t, err)
		assert.JSONEq(t, `["user.created"]`, string(data))
	})
}

func TestOutpostQuery(t *testing.T) {
	t.Parallel()

	t.Run("lists use indexed bracket notation", func(t *testing.T) {
		got := outpostQuery(nil, map[string][]string{"id": {"a", "b"}})
		parsed, err := url.ParseQuery(got)
		require.NoError(t, err)
		assert.Equal(t, []string{"a"}, parsed["id[0]"])
		assert.Equal(t, []string{"b"}, parsed["id[1]"])
		// The bare key must not be used; the API does not read repeated keys.
		assert.Empty(t, parsed["id"])
	})

	t.Run("empty scalars and list entries are omitted", func(t *testing.T) {
		got := outpostQuery(map[string]string{"dir": "", "limit": "10"}, map[string][]string{"topic": {"", "user.created"}})
		parsed, err := url.ParseQuery(got)
		require.NoError(t, err)
		assert.Equal(t, []string{"10"}, parsed["limit"])
		assert.Empty(t, parsed["dir"])
		// The empty entry is skipped, so the surviving value keeps its own index.
		assert.Equal(t, []string{"user.created"}, parsed["topic[1]"])
	})

	t.Run("bracketed scalar keys pass through", func(t *testing.T) {
		params := map[string]string{}
		setOutpostTimeRange(params, "time", "2026-01-01T00:00:00Z", "2026-02-01T00:00:00Z")

		parsed, err := url.ParseQuery(outpostQuery(params, nil))
		require.NoError(t, err)
		assert.Equal(t, []string{"2026-01-01T00:00:00Z"}, parsed["time[gte]"])
		assert.Equal(t, []string{"2026-02-01T00:00:00Z"}, parsed["time[lte]"])
	})

	t.Run("a one-sided time range only sets that bound", func(t *testing.T) {
		params := map[string]string{}
		setOutpostTimeRange(params, "time", "", "2026-02-01T00:00:00Z")
		assert.NotContains(t, params, "time[gte]")
		assert.Contains(t, params, "time[lte]")
	})
}

func TestListOutpostDestinationsUnpaginated(t *testing.T) {
	t.Parallel()

	// This endpoint returns a bare array rather than the {models, pagination}
	// envelope the other list endpoints use.
	var gotPath, gotQuery string
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":"des_1","type":"webhook","topics":"*","config":{"url":"https://example.com"}},
			{"id":"des_2","type":"aws_sqs","topics":["user.created"],"config":{}}
		]`))
	})
	defer server.Close()

	destinations, err := client.ListOutpostDestinations(context.Background(), "tenant_1", []string{"webhook"}, nil)
	require.NoError(t, err)
	require.Len(t, destinations, 2)

	assert.Equal(t, APIPathPrefix+"/tenants/tenant_1/destinations", gotPath)
	assert.Contains(t, gotQuery, "type%5B0%5D=webhook")

	assert.True(t, destinations[0].Topics.IsWildcard())
	assert.Equal(t, OutpostTopics{"user.created"}, destinations[1].Topics)
	assert.False(t, destinations[0].Disabled())
}

func TestListOutpostAttemptsRouting(t *testing.T) {
	t.Parallel()

	t.Run("uses the tenant-scoped path when tenant and destination are both set", func(t *testing.T) {
		var gotPath, gotQuery string
		client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
			_, _ = w.Write([]byte(`{"models":[],"pagination":{}}`))
		})
		defer server.Close()

		_, err := client.ListOutpostAttempts(context.Background(), OutpostAttemptListParams{
			TenantID:      "tenant_1",
			DestinationID: "des_1",
			EventIDs:      []string{"evt_1"},
		})
		require.NoError(t, err)

		assert.Equal(t, APIPathPrefix+"/tenants/tenant_1/destinations/des_1/attempts", gotPath)
		assert.Contains(t, gotQuery, "event_id%5B0%5D=evt_1")
		// Path already constrains these, so they must not be sent as filters too.
		assert.NotContains(t, gotQuery, "tenant_id")
		assert.NotContains(t, gotQuery, "destination_id")
	})

	t.Run("uses the global path and sends filters when only one is set", func(t *testing.T) {
		var gotPath, gotQuery string
		client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
			_, _ = w.Write([]byte(`{"models":[],"pagination":{}}`))
		})
		defer server.Close()

		_, err := client.ListOutpostAttempts(context.Background(), OutpostAttemptListParams{
			TenantIDs: []string{"tenant_1"},
		})
		require.NoError(t, err)

		assert.Equal(t, APIPathPrefix+"/attempts", gotPath)
		assert.Contains(t, gotQuery, "tenant_id%5B0%5D=tenant_1")
	})
}

func TestPublishOutpostEventUsesBearerToken(t *testing.T) {
	t.Parallel()

	t.Run("sends the supplied project key and not the stored CLI key", func(t *testing.T) {
		var gotAuth string
		var gotBasicUser string
		var hadBasic bool
		client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			gotBasicUser, _, hadBasic = r.BasicAuth()
			_, _ = w.Write([]byte(`{"id":"evt_1","duplicate":false,"destination_ids":["des_1"]}`))
		})
		defer server.Close()

		// newTestClient sets APIKey, which PerformRequest would otherwise apply
		// as basic auth and overwrite the bearer token with.
		require.Equal(t, "test-api-key", client.APIKey)

		resp, err := client.PublishOutpostEvent(context.Background(), "project-api-key", &OutpostPublishRequest{
			TenantID: "tenant_1",
			Topic:    "user.created",
		})
		require.NoError(t, err)

		assert.Equal(t, "Bearer project-api-key", gotAuth)
		assert.False(t, hadBasic, "stored CLI key must not be sent as basic auth")
		assert.Empty(t, gotBasicUser)
		assert.Equal(t, "evt_1", resp.ID)
	})

	t.Run("leaves the original client's key intact", func(t *testing.T) {
		client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"id":"evt_1"}`))
		})
		defer server.Close()

		_, err := client.PublishOutpostEvent(context.Background(), "project-api-key", &OutpostPublishRequest{
			TenantID: "tenant_1", Topic: "user.created",
		})
		require.NoError(t, err)
		assert.Equal(t, "test-api-key", client.APIKey, "publish must not mutate the shared client")
	})

	t.Run("fails fast without a key rather than sending an unauthenticated request", func(t *testing.T) {
		called := false
		client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
			called = true
		})
		defer server.Close()

		_, err := client.PublishOutpostEvent(context.Background(), "", &OutpostPublishRequest{
			TenantID: "tenant_1", Topic: "user.created",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Project API key")
		assert.False(t, called, "no request should be sent")
	})
}

func TestOutpostMetricsRequiredParams(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[],"metadata":{}}`))
	})
	defer server.Close()

	t.Run("start and end are required", func(t *testing.T) {
		_, err := client.GetOutpostEventMetrics(context.Background(), OutpostMetricsParams{
			Measures: []string{"count"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "start and end are required")
	})

	t.Run("at least one measure is required", func(t *testing.T) {
		_, err := client.GetOutpostEventMetrics(context.Background(), OutpostMetricsParams{
			Start: "2026-01-01T00:00:00Z", End: "2026-02-01T00:00:00Z",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one measure")
	})
}

func TestOutpostMetricsQueryShape(t *testing.T) {
	t.Parallel()

	var gotQuery string
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"data":[],"metadata":{"truncated":true}}`))
	})
	defer server.Close()

	result, err := client.GetOutpostAttemptMetrics(context.Background(), OutpostMetricsParams{
		Start:      "2026-01-01T00:00:00Z",
		End:        "2026-02-01T00:00:00Z",
		Measures:   []string{"count", "failed_count"},
		Dimensions: []string{"destination_id"},
		Filters:    map[string][]string{"topic": {"user.created"}},
	})
	require.NoError(t, err)

	parsed, err := url.ParseQuery(gotQuery)
	require.NoError(t, err)
	assert.Equal(t, []string{"2026-01-01T00:00:00Z"}, parsed["time[start]"])
	assert.Equal(t, []string{"count"}, parsed["measures[0]"])
	assert.Equal(t, []string{"failed_count"}, parsed["measures[1]"])
	assert.Equal(t, []string{"destination_id"}, parsed["dimensions[0]"])
	assert.Equal(t, []string{"user.created"}, parsed["filters[topic][0]"])

	assert.True(t, result.Metadata.Truncated)
}

func TestUpdateOutpostConfigRejectsEmptyUpdate(t *testing.T) {
	t.Parallel()

	called := false
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	defer server.Close()

	_, err := client.UpdateOutpostConfig(context.Background(), OutpostManagedConfig{})
	require.Error(t, err)
	assert.False(t, called, "an empty update must not reach the API")
}

func TestOutpostConfigSendsNullToClearAKey(t *testing.T) {
	t.Parallel()

	var gotBody map[string]interface{}
	var gotMethod string
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"TOPICS":"user.created"}`))
	})
	defer server.Close()

	topics := "user.created"
	_, err := client.UpdateOutpostConfig(context.Background(), OutpostManagedConfig{
		"TOPICS":                   &topics,
		"DELIVERY_TIMEOUT_SECONDS": nil,
	})
	require.NoError(t, err)

	assert.Equal(t, http.MethodPatch, gotMethod)
	assert.Equal(t, "user.created", gotBody["TOPICS"])
	require.Contains(t, gotBody, "DELIVERY_TIMEOUT_SECONDS")
	assert.Nil(t, gotBody["DELIVERY_TIMEOUT_SECONDS"], "a nil value must serialise as null, not be dropped")
}

func TestAcceptAnySuccessStatus(t *testing.T) {
	t.Parallel()

	// The Gateway API answers 200 to everything, so the client's default treats
	// anything else as an error. Outpost uses 201 on create and 202 on
	// publish/retry, which made every write fail until this was opt-in-widened.
	for _, status := range []int{http.StatusOK, http.StatusCreated, http.StatusAccepted} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"id":"tenant_1"}`))
			})
			defer server.Close()
			client.AcceptAnySuccessStatus = true

			tenant, err := client.UpsertOutpostTenant(context.Background(), "tenant_1", &OutpostTenantUpsertRequest{})
			require.NoError(t, err, "%d must be treated as success", status)
			assert.Equal(t, "tenant_1", tenant.ID)
		})
	}

	t.Run("errors are still errors", func(t *testing.T) {
		client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"message":"invalid topic"}`))
		})
		defer server.Close()
		client.AcceptAnySuccessStatus = true

		_, err := client.UpsertOutpostTenant(context.Background(), "tenant_1", &OutpostTenantUpsertRequest{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid topic")
	})

	t.Run("default client still rejects a non-200 success", func(t *testing.T) {
		client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"evt_1"}`))
		})
		defer server.Close()

		// Guards the opt-in: Gateway behaviour must be unchanged.
		_, err := client.UpsertOutpostTenant(context.Background(), "tenant_1", &OutpostTenantUpsertRequest{})
		require.Error(t, err)
	})
}
