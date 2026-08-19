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

	// The types are fetched from the deployment rather than hardcoded, so an
	// unknown one has to be answered with the list that is actually available —
	// otherwise the only way to find a valid type is to guess.
	t.Run("an unknown type lists the available ones", func(t *testing.T) {
		stdout, _, err := cli.Run("outpost", "destination-type", "get", "banana")
		require.Error(t, err)
		assert.Contains(t, stdout, `unknown destination type "banana"`)
		assert.Contains(t, stdout, "webhook", "the error should list the types that are valid")
	})
}

// TestOutpostTenantPagination walks a cursor rather than trusting that the flag
// is wired up. --next and --prev were accepted by every list command but no
// test had ever sent one, so a dropped cursor would have looked like a short
// page instead of a bug.
func TestOutpostTenantPagination(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewOutpostCLIRunner(t)

	// Three tenants guarantee at least two pages at a limit of one, whatever
	// else is already in the shared project.
	created := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		created = append(created, createTestTenant(t, cli))
	}

	type page struct {
		Models []struct {
			ID string `json:"id"`
		} `json:"models"`
		Pagination struct {
			Next *string `json:"next"`
			Prev *string `json:"prev"`
		} `json:"pagination"`
	}

	readPage := func(args ...string) page {
		t.Helper()
		var p page
		stdout := cli.RunExpectSuccess(append([]string{"outpost", "tenant", "list", "--limit", "1", "--output", "json"}, args...)...)
		require.NoError(t, json.Unmarshal([]byte(stdout), &p))
		return p
	}

	first := readPage()
	require.Len(t, first.Models, 1, "--limit was not applied")
	require.NotNil(t, first.Pagination.Next, "a next cursor is required to page forward")

	second := readPage("--next", *first.Pagination.Next)
	require.Len(t, second.Models, 1)
	assert.NotEqual(t, first.Models[0].ID, second.Models[0].ID,
		"--next returned the same record, so the cursor was not sent")

	// Paging back from the second page must return the first one. This is the
	// assertion that a cursor is being used rather than silently ignored.
	require.NotNil(t, second.Pagination.Prev)
	back := readPage("--prev", *second.Pagination.Prev)
	require.Len(t, back.Models, 1)
	assert.Equal(t, first.Models[0].ID, back.Models[0].ID)

	assert.Len(t, created, 3)
}

// TestOutpostTenantPortalAndCustomDomain covers the tenant portal and the
// custom-domain commands, which between them had no automated coverage at all.
//
// They have to be tested together: a portal URL only exists once the project
// has a portal custom domain, so proving `tenant portal` works means configuring
// one.
//
// This mutates a project-wide setting rather than being gated behind an opt-in
// env var. An opt-in would never run in CI, which is the same silent
// non-execution this work set out to remove — and these are the commands most
// in need of a real run, since nothing else exercises them. The blast radius is
// contained instead: the prior state is read first and restored in t.Cleanup,
// whether the test passes or fails, and the hostname is unique per run.
//
// The hostname is a subdomain of a domain Hookdeck owns because the API rejects
// the reserved test TLDs (.test, .invalid, .example) and example.com. No DNS
// record is ever created, so the domain stays unverified and is removed at the
// end of the test.
func TestOutpostTenantPortalAndCustomDomain(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewOutpostCLIRunner(t)

	readCustomDomain := func() string {
		t.Helper()
		stdout := cli.RunExpectSuccess("outpost", "config", "custom-domain", "get", "--output", "json")
		var domain struct {
			Hostname string `json:"hostname"`
		}
		require.NoError(t, json.Unmarshal([]byte(stdout), &domain))
		return domain.Hostname
	}

	hostname := fmt.Sprintf("cli-acceptance-%d.hookdeck.com", time.Now().UnixNano())

	before := readCustomDomain()
	t.Cleanup(func() {
		// Only remove the domain this test set. The happy path already deleted
		// it, and a concurrent run against the same project may have configured
		// its own by now — deleting that would fail the other run rather than
		// this one.
		if readCustomDomain() == hostname {
			if _, _, err := cli.Run("outpost", "config", "custom-domain", "delete", "--force"); err != nil {
				t.Errorf("cleanup: could not remove the test custom domain: %v", err)
			}
		}
		if before != "" {
			if _, _, err := cli.Run("outpost", "config", "custom-domain", "set", before); err != nil {
				t.Errorf("cleanup: could not restore the project's custom domain %q: %v", before, err)
			}
		}
	})

	if before != "" {
		cli.RunExpectSuccess("outpost", "config", "custom-domain", "delete", "--force")
	}

	stdout := cli.RunExpectSuccess("outpost", "config", "custom-domain", "set", hostname)
	assert.Contains(t, stdout, hostname)

	stdout = cli.RunExpectSuccess("outpost", "config", "custom-domain", "get")
	assert.Contains(t, stdout, hostname)
	assert.Contains(t, stdout, "Status:", "a domain that is not yet verified has to say so")

	tenantID := createTestTenant(t, cli)

	// Configuration changes reach the deployment asynchronously, so the portal
	// 404s for a short while after the domain is set. Polling here is the
	// difference between testing the command and testing the propagation delay.
	//
	// The hostname is asserted above, on the fast path; this only waits for a
	// URL to exist. Requiring our hostname here as well would make the test
	// fail whenever a concurrent run against the same project has replaced the
	// domain in the meantime — a collision between runs, not a CLI defect.
	var portalURL string
	require.Eventually(t, func() bool {
		out, _, err := cli.Run("outpost", "tenant", "portal", tenantID)
		if err != nil {
			return false
		}
		portalURL = strings.TrimSpace(out)
		return strings.Contains(portalURL, "token=")
	}, 90*time.Second, 5*time.Second, "the portal URL never became available after setting a custom domain")

	// The URL is a credential and scripts pipe it, so it must be the only thing
	// printed.
	assert.Equal(t, 1, len(strings.Split(portalURL, "\n")), "the portal URL must be printed on its own")
	assert.Contains(t, portalURL, "token=", "the URL carries the tenant's portal session")

	t.Run("theme is passed through", func(t *testing.T) {
		for _, theme := range []string{"light", "dark"} {
			out := cli.RunExpectSuccess("outpost", "tenant", "portal", tenantID, "--theme", theme, "--output", "json")
			var portal struct {
				RedirectURL string `json:"redirect_url"`
				TenantID    string `json:"tenant_id"`
			}
			require.NoError(t, json.Unmarshal([]byte(out), &portal))
			assert.Equal(t, tenantID, portal.TenantID)
			assert.Contains(t, portal.RedirectURL, "theme="+theme)
		}
	})

	t.Run("an invalid theme is rejected", func(t *testing.T) {
		stdout, _, err := cli.Run("outpost", "tenant", "portal", tenantID, "--theme", "neon")
		require.Error(t, err)
		assert.Contains(t, stdout, "--theme must be either light or dark")
	})

	// --open launches a browser, so it is never exercised here; this only
	// checks it is still offered, since the flag is otherwise unreferenced.
	t.Run("--open is offered", func(t *testing.T) {
		help := cli.RunExpectSuccess("outpost", "tenant", "portal", "--help")
		assert.Contains(t, help, "--open")
	})

	cli.RunExpectSuccess("outpost", "config", "custom-domain", "delete", "--force")
	assert.Empty(t, readCustomDomain(), "the domain should be gone after delete")
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
