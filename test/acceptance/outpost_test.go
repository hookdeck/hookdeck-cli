//go:build outpost

package acceptance

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// uniqueTenantID keeps runs independent. The test project is shared between
// local runs and CI, and a failed run can leave data behind, so nothing may
// assume it starts empty.
func uniqueTenantID(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("cli-at-%d", time.Now().UnixNano())
}

// createTestTenant creates a tenant and removes it when the test ends, whether
// or not the test passed.
func createTestTenant(t *testing.T, cli *CLIRunner) string {
	t.Helper()

	tenantID := uniqueTenantID(t)
	cli.RunExpectSuccess("outpost", "tenant", "upsert", tenantID)
	t.Cleanup(func() {
		if _, _, err := cli.Run("outpost", "tenant", "delete", tenantID, "--force"); err != nil {
			t.Logf("cleanup: could not delete tenant %s: %v", tenantID, err)
		}
	})

	return tenantID
}

func TestOutpostStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewOutpostCLIRunner(t)
	stdout := cli.RunExpectSuccess("outpost", "status")
	assert.Contains(t, stdout, "Status:")
}

func TestOutpostTopicList(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewOutpostCLIRunner(t)
	stdout := cli.RunExpectSuccess("outpost", "topic", "list")

	// A project with no topics cannot deliver anything, so most of the coverage
	// below would be meaningless. Fail here with the fix rather than further in.
	require.NotContains(t, stdout, "No topics configured",
		"the Outpost test project needs topics: hookdeck outpost config set TOPICS=user.created")
}

func TestOutpostDestinationTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewOutpostCLIRunner(t)

	stdout := cli.RunExpectSuccess("outpost", "destination-type", "list")
	assert.Contains(t, stdout, "webhook")

	stdout = cli.RunExpectSuccess("outpost", "destination-type", "get", "webhook")
	assert.Contains(t, stdout, "--config fields:")
	assert.Contains(t, stdout, "url")
	assert.Contains(t, stdout, "required")
}

func TestOutpostTenantLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewOutpostCLIRunner(t)
	tenantID := uniqueTenantID(t)

	stdout := cli.RunExpectSuccess("outpost", "tenant", "upsert", tenantID, "--metadata", "plan=pro")
	assert.Contains(t, stdout, tenantID)

	// Upsert is idempotent, so running it again must succeed rather than
	// conflict — that is the only way to create a tenant.
	cli.RunExpectSuccess("outpost", "tenant", "upsert", tenantID, "--metadata", "plan=enterprise")

	stdout = cli.RunExpectSuccess("outpost", "tenant", "get", tenantID, "--output", "json")
	var tenant struct {
		ID       string            `json:"id"`
		Metadata map[string]string `json:"metadata"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &tenant))
	assert.Equal(t, tenantID, tenant.ID)
	assert.Equal(t, "enterprise", tenant.Metadata["plan"], "the second upsert should have replaced the metadata")

	stdout = cli.RunExpectSuccess("outpost", "tenant", "list", "--id", tenantID, "--output", "json")
	assert.Contains(t, stdout, tenantID)

	// The token is a real credential scoped to this tenant, so assert its shape
	// rather than its contents: three dot-separated JWT segments, nothing logged.
	stdout = cli.RunExpectSuccess("outpost", "tenant", "token", tenantID)
	token := strings.TrimSpace(stdout)
	assert.Len(t, strings.Split(token, "."), 3, "expected a JWT")
	assert.NotContains(t, token, tenantID, "the raw tenant id should not be readable in the token")

	cli.RunExpectSuccess("outpost", "tenant", "delete", tenantID, "--force")

	_, _, err := cli.Run("outpost", "tenant", "get", tenantID)
	assert.Error(t, err, "the tenant should be gone after delete")
}

func TestOutpostDestinationLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewOutpostCLIRunner(t)
	tenantID := createTestTenant(t, cli)

	stdout := cli.RunExpectSuccess("outpost", "destination", "create",
		"--tenant-id", tenantID,
		"--type", "webhook",
		"--config", "url=https://example.com/acceptance",
		"--topics", "*",
		"--output", "json")

	var created struct {
		ID     string                 `json:"id"`
		Type   string                 `json:"type"`
		Topics interface{}            `json:"topics"`
		Config map[string]interface{} `json:"config"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &created))
	require.NotEmpty(t, created.ID)
	assert.Equal(t, "webhook", created.Type)
	assert.Equal(t, "https://example.com/acceptance", created.Config["url"])
	// "*" comes back as a bare string rather than an array; decoding it is the
	// point of this assertion.
	assert.Equal(t, "*", created.Topics)

	stdout = cli.RunExpectSuccess("outpost", "destination", "get", created.ID, "--tenant-id", tenantID)
	assert.Contains(t, stdout, created.ID)

	stdout = cli.RunExpectSuccess("outpost", "destination", "list", "--tenant-id", tenantID)
	assert.Contains(t, stdout, created.ID)

	stdout = cli.RunExpectSuccess("outpost", "destination", "update", created.ID,
		"--tenant-id", tenantID, "--config", "url=https://example.com/updated", "--output", "json")
	assert.Contains(t, stdout, "https://example.com/updated")

	stdout = cli.RunExpectSuccess("outpost", "destination", "disable", created.ID, "--tenant-id", tenantID)
	assert.Contains(t, strings.ToLower(stdout), "disabled")

	stdout = cli.RunExpectSuccess("outpost", "destination", "enable", created.ID, "--tenant-id", tenantID)
	assert.Contains(t, strings.ToLower(stdout), "enabled")

	cli.RunExpectSuccess("outpost", "destination", "delete", created.ID, "--tenant-id", tenantID, "--force")
}

func TestOutpostDestinationValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewOutpostCLIRunner(t)
	tenantID := createTestTenant(t, cli)

	// Error text goes to stdout today, not stderr — see #340, which tracks
	// moving it. These assert current behaviour so they fail loudly if it moves,
	// rather than silently checking the wrong stream.
	t.Run("an unknown config field is rejected with the valid ones", func(t *testing.T) {
		stdout, _, err := cli.Run("outpost", "destination", "create",
			"--tenant-id", tenantID, "--type", "webhook", "--config", "nope=x")
		require.Error(t, err)
		assert.Contains(t, stdout, "not a valid config field")
	})

	t.Run("an unknown type lists the available types", func(t *testing.T) {
		stdout, _, err := cli.Run("outpost", "destination", "create",
			"--tenant-id", tenantID, "--type", "banana", "--config", "url=https://example.com")
		require.Error(t, err)
		assert.Contains(t, stdout, "unknown destination type")
		assert.Contains(t, stdout, "webhook", "the error should list the types that are valid")
	})

	t.Run("a missing tenant is reported before the request", func(t *testing.T) {
		stdout, _, err := cli.Run("outpost", "destination", "list")
		require.Error(t, err)
		assert.Contains(t, stdout, "--tenant-id is required")
	})
}

func TestOutpostPublishAndInspect(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewOutpostCLIRunner(t)
	tenantID := createTestTenant(t, cli)

	topics := cli.RunExpectSuccess("outpost", "topic", "list", "--output", "json")
	var available []string
	require.NoError(t, json.Unmarshal([]byte(topics), &available))
	require.NotEmpty(t, available, "the test project needs at least one topic")
	topic := available[0]

	cli.RunExpectSuccess("outpost", "destination", "create",
		"--tenant-id", tenantID, "--type", "webhook",
		"--config", "url=https://example.com/publish", "--topics", topic)

	// Publish needs the Project API key directly; the stored CLI key is not
	// accepted by this endpoint.
	stdout := cli.RunExpectSuccess("outpost", "publish",
		"--tenant-id", tenantID, "--topic", topic,
		"--data", `{"source":"acceptance"}`,
		"--api-key", cli.apiKey)
	assert.Contains(t, stdout, "accepted")

	// Publishing is asynchronous, so poll rather than asserting immediately.
	require.Eventually(t, func() bool {
		out, _, err := cli.Run("outpost", "event", "list", "--tenant-id", tenantID, "--output", "json")
		if err != nil {
			return false
		}
		var events struct {
			Models []struct {
				Topic string `json:"topic"`
			} `json:"models"`
		}
		return json.Unmarshal([]byte(out), &events) == nil && len(events.Models) > 0
	}, 30*time.Second, 2*time.Second, "the published event never appeared")

	// Fetch one event by id, using an id from the list above rather than
	// assuming the publish response id is queryable yet.
	listed := cli.RunExpectSuccess("outpost", "event", "list", "--tenant-id", tenantID, "--output", "json")
	var events struct {
		Models []struct {
			ID    string `json:"id"`
			Topic string `json:"topic"`
		} `json:"models"`
	}
	require.NoError(t, json.Unmarshal([]byte(listed), &events))
	require.NotEmpty(t, events.Models)

	stdout = cli.RunExpectSuccess("outpost", "event", "get", events.Models[0].ID, "--tenant-id", tenantID, "--output", "json")
	assert.Contains(t, stdout, events.Models[0].ID)
	assert.Contains(t, stdout, "acceptance", "the published payload should come back")

	// Delivery to example.com fails, but a failed attempt still exercises the
	// read path, which is what is being checked here.
	var attempts struct {
		Models []struct {
			ID string `json:"id"`
		} `json:"models"`
	}
	require.Eventually(t, func() bool {
		out, _, err := cli.Run("outpost", "attempt", "list", "--tenant-id", tenantID, "--output", "json")
		if err != nil {
			return false
		}
		return json.Unmarshal([]byte(out), &attempts) == nil && len(attempts.Models) > 0
	}, 60*time.Second, 3*time.Second, "no delivery attempt was recorded")

	stdout = cli.RunExpectSuccess("outpost", "attempt", "get", attempts.Models[0].ID, "--tenant-id", tenantID)
	assert.Contains(t, stdout, attempts.Models[0].ID)

	t.Run("retry queues another attempt", func(t *testing.T) {
		destinations := cli.RunExpectSuccess("outpost", "destination", "list", "--tenant-id", tenantID, "--output", "json")
		var dests []struct {
			ID string `json:"id"`
		}
		require.NoError(t, json.Unmarshal([]byte(destinations), &dests))
		require.NotEmpty(t, dests)

		out := cli.RunExpectSuccess("outpost", "event", "retry",
			"--event-id", events.Models[0].ID, "--destination-id", dests[0].ID)
		assert.Contains(t, out, "Retry accepted")
	})
}

func TestOutpostPublishRequiresProjectAPIKey(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewOutpostCLIRunner(t)

	// The stored credentials are deliberately not enough here, and the error has
	// to explain that rather than surfacing a bare 401.
	stdout, _, err := cli.RunWithEnv(map[string]string{"HOOKDECK_API_KEY": ""},
		"outpost", "publish", "--tenant-id", "whoever", "--topic", "user.created")
	require.Error(t, err)
	assert.Contains(t, stdout, "Project API key")
	assert.Contains(t, stdout, "--api-key", "the error should name the flag that fixes it")
}

func TestOutpostMetrics(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewOutpostCLIRunner(t)

	end := time.Now().UTC().Format(time.RFC3339)
	start := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)

	cli.RunExpectSuccess("outpost", "metrics", "events", "--start", start, "--end", end, "--measures", "count")
	cli.RunExpectSuccess("outpost", "metrics", "attempts", "--start", start, "--end", end, "--measures", "count")

	t.Run("required parameters are enforced", func(t *testing.T) {
		_, _, err := cli.Run("outpost", "metrics", "events", "--start", start, "--end", end)
		assert.Error(t, err, "--measures is required")
	})
}

func TestOutpostConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewOutpostCLIRunner(t)

	stdout := cli.RunExpectSuccess("outpost", "config", "get", "TOPICS")
	require.NotEmpty(t, strings.TrimSpace(stdout))

	// Dry run must not change anything — this config is shared with every other
	// test in this file, so an accidental write would be disruptive.
	before := cli.RunExpectSuccess("outpost", "config", "get", "TOPICS")
	stdout = cli.RunExpectSuccess("outpost", "config", "set", "TOPICS=should.not.apply", "--dry-run")
	assert.Contains(t, stdout, "Dry run")
	after := cli.RunExpectSuccess("outpost", "config", "get", "TOPICS")
	assert.Equal(t, before, after, "--dry-run must not apply the change")
}
