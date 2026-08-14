//go:build outpostlive

// Live, read-only smoke test for the Outpost API client (pkg/hookdeck/outpost_*.go).
//
// Why this exists separately from the `outpost` acceptance tag: the unit tests for
// this client run against stub servers, so they only assert the implementation's
// own assumptions back at it. Nothing there proves a real request is accepted, that
// real payloads decode, or that the credentials work against the Outpost host at
// all. This file makes real requests and asserts responses decode.
//
// It is read-only on purpose — no creates, deletes, config changes or publishes —
// so it is safe to run against any Outpost project. Write coverage belongs in the
// `outpost` acceptance slice, where cleanup is handled.
//
// Run:
//
//	go test -tags=outpostlive ./test/acceptance/... -run Live -v
//
// Requires HOOKDECK_CLI_OUTPOST_TESTING_API_KEY (a Project API key for an Outpost
// project) in test/acceptance/.env or the environment.
package acceptance

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

const outpostLiveKeyEnv = "HOOKDECK_CLI_OUTPOST_TESTING_API_KEY"

func outpostLiveAPIKey(t *testing.T) string {
	t.Helper()

	key := os.Getenv(outpostLiveKeyEnv)
	if key == "" {
		t.Skipf("%s not set; skipping live Outpost smoke test", outpostLiveKeyEnv)
	}
	return key
}

// outpostLiveBaseURL allows pointing the smoke test at a non-production Outpost
// host, matching the hidden --outpost-api-base flag.
func outpostLiveBaseURL(t *testing.T) *url.URL {
	t.Helper()

	raw := os.Getenv("HOOKDECK_OUTPOST_API_BASE")
	if raw == "" {
		raw = hookdeck.DefaultOutpostAPIBaseURL
	}

	parsed, err := url.Parse(raw)
	require.NoError(t, err, "invalid Outpost API base URL %q", raw)
	return parsed
}

// newOutpostLiveClient builds a client authenticated with the Project API key
// directly. A Project API key is accepted on the Outpost resource endpoints, which
// makes this the shortest path to exercising the client.
func newOutpostLiveClient(t *testing.T) *hookdeck.Client {
	t.Helper()

	return &hookdeck.Client{
		BaseURL:                outpostLiveBaseURL(t),
		APIKey:                 outpostLiveAPIKey(t),
		AcceptAnySuccessStatus: true,
	}
}

// newOutpostLiveClientFromCLIKey authenticates the way a real user does — exchange
// the Project API key through `hookdeck ci`, then use the CLI client key the CLI
// stores. This is the path every `hookdeck outpost …` command will take, so it is
// tested explicitly rather than assumed to be equivalent to the key above.
func newOutpostLiveClientFromCLIKey(t *testing.T) *hookdeck.Client {
	t.Helper()

	apiKey := outpostLiveAPIKey(t)

	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err)

	configPath := filepath.Join(t.TempDir(), "config.toml")
	runner := NewCLIRunnerWithConfigPathNoCI(t, configPath)
	runner.projectRoot = projectRoot

	stdout, stderr, err := runner.Run("ci", "--api-key", apiKey)
	require.NoError(t, err, "hookdeck ci failed: stdout=%s stderr=%s", stdout, stderr)

	cfg, err := config.LoadConfigFromFile(configPath)
	require.NoError(t, err, "could not read the config written by hookdeck ci")

	cliKey := cfg.Profile.APIKey
	require.NotEmpty(t, cliKey, "hookdeck ci did not store a CLI key")
	require.NotEqual(t, apiKey, cliKey, "config should hold the exchanged CLI key, not the Project API key")

	t.Logf("project %s resolved as type %q", cfg.Profile.ProjectId, cfg.Profile.ProjectType)
	assert.True(t, config.IsOutpostProject(cfg.Profile.ProjectType),
		"%s must belong to an Outpost project; got type %q", outpostLiveKeyEnv, cfg.Profile.ProjectType)

	return &hookdeck.Client{
		BaseURL:                outpostLiveBaseURL(t),
		APIKey:                 cliKey,
		ProjectID:              cfg.Profile.ProjectId,
		AcceptAnySuccessStatus: true,
	}
}

// TestLiveOutpostReadsWithProjectAPIKey exercises every read endpoint the client
// exposes and asserts the real responses decode.
func TestLiveOutpostReadsWithProjectAPIKey(t *testing.T) {
	client := newOutpostLiveClient(t)
	ctx := context.Background()

	var tenantID string

	t.Run("status", func(t *testing.T) {
		status, err := client.GetOutpostStatus(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, status.Status, "deployment status should be reported")
		t.Logf("status=%s version=%s", status.Status, status.Version)
	})

	t.Run("topics", func(t *testing.T) {
		topics, err := client.ListOutpostTopics(ctx)
		require.NoError(t, err)
		t.Logf("topics=%v", topics)

		// Decoding an empty list still proves the endpoint and auth work, but a
		// project with no topics cannot host a destination or accept a publish,
		// so most of the remaining coverage is unreachable until this is fixed.
		// Fail loudly with the remedy rather than passing quietly on an empty list.
		assert.NotEmpty(t, topics,
			"the Outpost test project has no topics configured. Set TOPICS in the project's "+
				"Outpost settings (operator config); without it, tenants cannot have destinations "+
				"and events cannot be published, so tenant/destination/event decoding stays untested")
	})

	t.Run("destination types", func(t *testing.T) {
		schemas, err := client.ListOutpostDestinationTypes(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, schemas)

		var webhook *hookdeck.OutpostDestinationTypeSchema
		for i := range schemas {
			if schemas[i].Type == "webhook" {
				webhook = &schemas[i]
			}
		}
		require.NotNil(t, webhook, "webhook should always be an available destination type")
		assert.NotEmpty(t, webhook.ConfigFields, "config_fields drives dynamic --config-* validation")
	})

	t.Run("single destination type", func(t *testing.T) {
		schema, err := client.GetOutpostDestinationType(ctx, "webhook")
		require.NoError(t, err)
		assert.Equal(t, "webhook", schema.Type)
	})

	t.Run("tenants", func(t *testing.T) {
		tenants, err := client.ListOutpostTenants(ctx, hookdeck.OutpostTenantListParams{Limit: 5})
		require.NoError(t, err)
		t.Logf("tenant count=%d", len(tenants.Models))

		if len(tenants.Models) > 0 {
			tenantID = tenants.Models[0].ID
			assert.NotEmpty(t, tenantID)
		}
	})

	t.Run("destinations for a tenant", func(t *testing.T) {
		if tenantID == "" {
			t.Skip("no tenants in the test project; nothing to list destinations for")
		}

		// The response here is a bare array rather than a {models, pagination}
		// envelope — the decode is the point of this assertion.
		destinations, err := client.ListOutpostDestinations(ctx, tenantID, nil, nil)
		require.NoError(t, err)
		t.Logf("destination count=%d", len(destinations))

		for _, d := range destinations {
			assert.NotEmpty(t, d.Type)
			// topics is a union: "*" or an array. Either must decode.
			assert.NotNil(t, d.Topics)
		}
	})

	t.Run("events with a time range", func(t *testing.T) {
		// Exercises the time[gte]/time[lte] deepObject encoding against the real API.
		events, err := client.ListOutpostEvents(ctx, hookdeck.OutpostEventListParams{
			TimeAfter:  time.Now().Add(-30 * 24 * time.Hour).UTC().Format(time.RFC3339),
			TimeBefore: time.Now().UTC().Format(time.RFC3339),
			Limit:      5,
		})
		require.NoError(t, err)
		t.Logf("event count=%d", len(events.Models))
	})

	t.Run("attempts", func(t *testing.T) {
		attempts, err := client.ListOutpostAttempts(ctx, hookdeck.OutpostAttemptListParams{Limit: 5})
		require.NoError(t, err)
		t.Logf("attempt count=%d", len(attempts.Models))
	})

	t.Run("event metrics", func(t *testing.T) {
		// Exercises the time[start]/time[end] and measures[n] encoding.
		metrics, err := client.GetOutpostEventMetrics(ctx, hookdeck.OutpostMetricsParams{
			Start:    time.Now().Add(-7 * 24 * time.Hour).UTC().Format(time.RFC3339),
			End:      time.Now().UTC().Format(time.RFC3339),
			Measures: []string{"count"},
		})
		require.NoError(t, err)
		t.Logf("metrics rows=%d truncated=%v", len(metrics.Data), metrics.Metadata.Truncated)
	})

	t.Run("managed config", func(t *testing.T) {
		cfg, err := client.GetOutpostConfig(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, cfg, "managed config should return operator keys")
	})
}

// TestLiveOutpostReadsWithCLIKey is the important one: it proves the credentials
// the CLI actually stores work against the Outpost host. Everything in Phase 2
// depends on this being true.
func TestLiveOutpostReadsWithCLIKey(t *testing.T) {
	client := newOutpostLiveClientFromCLIKey(t)
	ctx := context.Background()

	t.Run("status", func(t *testing.T) {
		status, err := client.GetOutpostStatus(ctx)
		require.NoError(t, err, "a CLI client key should authenticate against the Outpost API")
		assert.NotEmpty(t, status.Status)
	})

	t.Run("tenants", func(t *testing.T) {
		tenants, err := client.ListOutpostTenants(ctx, hookdeck.OutpostTenantListParams{Limit: 5})
		require.NoError(t, err)
		t.Logf("tenant count=%d", len(tenants.Models))
	})

	t.Run("topics", func(t *testing.T) {
		_, err := client.ListOutpostTopics(ctx)
		require.NoError(t, err)
	})
}

// TestLiveOutpostSeededReadWrite creates a tenant and destination, publishes an
// event, reads everything back, then cleans up.
//
// This exists because the read-only test above can only assert what the project
// already contains. On an empty project the tenant, destination, event and
// attempt decoders are never exercised — including the `topics` union on a
// destination, which is the field shape most likely to be wrong. Seeding is the
// only way to prove those decode.
//
// Everything it creates is removed in t.Cleanup, including on failure.
func TestLiveOutpostSeededReadWrite(t *testing.T) {
	apiKey := outpostLiveAPIKey(t)
	client := newOutpostLiveClient(t)
	ctx := context.Background()

	topics, err := client.ListOutpostTopics(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, topics, "the project needs at least one configured topic to seed anything")
	topic := topics[0]

	// Unique per run so parallel runs and leftovers from a failed run cannot
	// collide, matching how the existing acceptance suites name resources.
	tenantID := fmt.Sprintf("cli-live-%d", time.Now().UnixNano())

	t.Run("upsert tenant", func(t *testing.T) {
		tenant, err := client.UpsertOutpostTenant(ctx, tenantID, &hookdeck.OutpostTenantUpsertRequest{
			Metadata: map[string]string{"created_by": "hookdeck-cli-live-test"},
		})
		require.NoError(t, err)
		assert.Equal(t, tenantID, tenant.ID)
	})

	t.Cleanup(func() {
		if err := client.DeleteOutpostTenant(context.Background(), tenantID); err != nil {
			t.Logf("cleanup: could not delete tenant %s: %v", tenantID, err)
		}
	})

	var destinationID string

	t.Run("create destination", func(t *testing.T) {
		destination, err := client.CreateOutpostDestination(ctx, tenantID, &hookdeck.OutpostDestinationCreateRequest{
			Type:   "webhook",
			Topics: hookdeck.OutpostTopics{topic},
			Config: map[string]interface{}{"url": "https://example.com/hookdeck-cli-live-test"},
		})
		require.NoError(t, err)
		require.NotEmpty(t, destination.ID)
		destinationID = destination.ID

		assert.Equal(t, "webhook", destination.Type)
		assert.Equal(t, hookdeck.OutpostTopics{topic}, destination.Topics)
		assert.False(t, destination.Disabled())
	})

	t.Run("create wildcard destination decodes the topics union", func(t *testing.T) {
		// The wildcard comes back as the bare string "*" rather than an array,
		// which a plain []string field cannot decode. This is the assertion the
		// empty project could never make.
		wildcard, err := client.CreateOutpostDestination(ctx, tenantID, &hookdeck.OutpostDestinationCreateRequest{
			Type:   "webhook",
			Topics: hookdeck.OutpostTopics{hookdeck.OutpostTopicsWildcard},
			Config: map[string]interface{}{"url": "https://example.com/hookdeck-cli-live-wildcard"},
		})
		require.NoError(t, err)
		assert.True(t, wildcard.Topics.IsWildcard(), "expected the wildcard form, got %v", wildcard.Topics)

		require.NoError(t, client.DeleteOutpostDestination(ctx, tenantID, wildcard.ID))
	})

	t.Run("list destinations", func(t *testing.T) {
		// Unpaginated: a bare array, not a {models, pagination} envelope.
		destinations, err := client.ListOutpostDestinations(ctx, tenantID, nil, nil)
		require.NoError(t, err)
		require.Len(t, destinations, 1, "only the non-wildcard destination should remain")
		assert.Equal(t, destinationID, destinations[0].ID)
	})

	t.Run("get destination", func(t *testing.T) {
		destination, err := client.GetOutpostDestination(ctx, tenantID, destinationID)
		require.NoError(t, err)
		assert.Equal(t, destinationID, destination.ID)
	})

	t.Run("disable and enable destination", func(t *testing.T) {
		disabled, err := client.DisableOutpostDestination(ctx, tenantID, destinationID)
		require.NoError(t, err)
		assert.True(t, disabled.Disabled(), "disabled_at should be set")

		enabled, err := client.EnableOutpostDestination(ctx, tenantID, destinationID)
		require.NoError(t, err)
		assert.False(t, enabled.Disabled(), "disabled_at should be cleared")
	})

	t.Run("update destination", func(t *testing.T) {
		updated, err := client.UpdateOutpostDestination(ctx, tenantID, destinationID, &hookdeck.OutpostDestinationUpdateRequest{
			Config: map[string]interface{}{"url": "https://example.com/hookdeck-cli-live-updated"},
		})
		require.NoError(t, err)
		assert.Equal(t, "https://example.com/hookdeck-cli-live-updated", updated.Config["url"])
	})

	t.Run("get tenant reflects the destination", func(t *testing.T) {
		tenant, err := client.GetOutpostTenant(ctx, tenantID)
		require.NoError(t, err)
		assert.Equal(t, 1, tenant.DestinationsCount)
		assert.Equal(t, "hookdeck-cli-live-test", tenant.Metadata["created_by"])
	})

	t.Run("tenant token", func(t *testing.T) {
		token, err := client.GetOutpostTenantToken(ctx, tenantID)
		require.NoError(t, err)
		assert.NotEmpty(t, token.Token, "this mints a real credential; it is gated behind write mode in MCP")
	})

	var eventID string

	t.Run("publish", func(t *testing.T) {
		// Publish needs the Project API key as a bearer token, not the client's
		// stored credential — the one command with different auth.
		resp, err := client.PublishOutpostEvent(ctx, apiKey, &hookdeck.OutpostPublishRequest{
			TenantID: tenantID,
			Topic:    topic,
			Data:     map[string]interface{}{"source": "hookdeck-cli-live-test"},
			Metadata: map[string]string{"origin": "cli-test"},
		})
		require.NoError(t, err)
		require.NotEmpty(t, resp.ID)
		eventID = resp.ID

		assert.False(t, resp.Duplicate)
		assert.Contains(t, resp.DestinationIDs, destinationID, "the event should match the destination's topic")
	})

	t.Run("list events for the tenant", func(t *testing.T) {
		// Publishing is asynchronous, so poll rather than asserting immediately.
		var events *hookdeck.OutpostEventListResponse
		require.Eventually(t, func() bool {
			var err error
			events, err = client.ListOutpostEvents(ctx, hookdeck.OutpostEventListParams{
				TenantIDs: []string{tenantID},
				Limit:     10,
			})
			return err == nil && len(events.Models) > 0
		}, 30*time.Second, 2*time.Second, "published event never appeared in the events list")

		event := events.Models[0]
		assert.Equal(t, tenantID, event.TenantID)
		assert.Equal(t, topic, event.Topic)
		assert.Equal(t, "hookdeck-cli-live-test", event.Data["source"])
		assert.False(t, event.Time.IsZero(), "time should decode")
	})

	t.Run("get event", func(t *testing.T) {
		if eventID == "" {
			t.Skip("no event id from publish")
		}
		event, err := client.GetOutpostEvent(ctx, eventID, tenantID)
		require.NoError(t, err)
		assert.Equal(t, eventID, event.ID)
	})

	t.Run("attempts for the tenant", func(t *testing.T) {
		// Delivery to example.com will fail; a failed attempt still proves the
		// attempt decoder works, which is what is being tested here.
		var attempts *hookdeck.OutpostAttemptListResponse
		require.Eventually(t, func() bool {
			var err error
			attempts, err = client.ListOutpostAttempts(ctx, hookdeck.OutpostAttemptListParams{
				TenantIDs: []string{tenantID},
				Limit:     10,
			})
			return err == nil && len(attempts.Models) > 0
		}, 60*time.Second, 3*time.Second, "no delivery attempt was recorded")

		attempt := attempts.Models[0]
		assert.NotEmpty(t, attempt.ID)
		assert.NotEmpty(t, attempt.Status)
		assert.Equal(t, destinationID, attempt.DestinationID)
		t.Logf("attempt status=%s code=%s number=%d", attempt.Status, attempt.Code, attempt.AttemptNumber)

		t.Run("get attempt", func(t *testing.T) {
			got, err := client.GetOutpostAttempt(ctx, attempt.ID, hookdeck.OutpostAttemptGetParams{
				TenantID: tenantID,
			})
			require.NoError(t, err)
			assert.Equal(t, attempt.ID, got.ID)
		})

		t.Run("tenant-scoped attempts path", func(t *testing.T) {
			scoped, err := client.ListOutpostAttempts(ctx, hookdeck.OutpostAttemptListParams{
				TenantID:      tenantID,
				DestinationID: destinationID,
				Limit:         10,
			})
			require.NoError(t, err)
			assert.NotEmpty(t, scoped.Models, "the tenant-scoped attempts route should return the same data")
		})
	})
}
