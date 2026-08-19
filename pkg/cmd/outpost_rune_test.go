package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
)

// Unit coverage for the outpost commands' RunE against a stub Outpost API.
//
// The acceptance suite runs against a shared project, so the destructive paths
// there are limited to what is safe to do to it — `outpost config set` in
// particular is only ever exercised with --dry-run, because the config it would
// change belongs to every other test in the file. That left the code that
// builds the PATCH body untested, including how --unset is encoded, which is
// the part most likely to be wrong and the most damaging if it is: sending the
// wrong shape here changes delivery for a whole project.
//
// These drive the real cobra commands, so flag parsing, PreRunE validation and
// the request body are all covered without touching a real project.

// stubRequest is a request the command under test sent.
type stubRequest struct {
	method string
	path   string
	query  string
	body   []byte
}

// stubOutpostAPI points the global Config at an httptest server, and restores
// the previous state when the test ends.
//
// It records every request, so a test can assert both what was sent and that
// nothing was sent at all — which is the whole point of --dry-run.
func stubOutpostAPI(t *testing.T, handlers map[string]http.HandlerFunc) *[]stubRequest {
	t.Helper()

	var requests []stubRequest

	mux := http.NewServeMux()
	for pattern, handler := range handlers {
		pattern, handler := pattern, handler
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			requests = append(requests, stubRequest{r.Method, r.URL.Path, r.URL.RawQuery, body})
			r.Body = io.NopCloser(bytes.NewReader(body))
			handler(w, r)
		})
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests = append(requests, stubRequest{r.Method, r.URL.Path, r.URL.RawQuery, body})
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	previous := Config
	Config = config.Config{}
	Config.OutpostAPIBaseURL = server.URL
	Config.Profile.APIKey = "test-key"
	Config.Profile.ProjectId = "proj_outpost"
	Config.Profile.ProjectType = config.ProjectTypeOutpost
	// Executing any cobra command runs the initializer registered in root.go,
	// which reads the config file and validates the log level. Point it at a
	// throwaway file so a test never reads — or creates — the developer's real
	// one, and give it a level it accepts. Everything set above survives:
	// InitConfig coalesces, so a value already present wins over the file.
	Config.LogLevel = "info"
	Config.ConfigFileFlag = filepath.Join(t.TempDir(), "config.toml")
	config.ResetAPIClientForTesting()

	t.Cleanup(func() {
		Config = previous
		config.ResetAPIClientForTesting()
	})

	return &requests
}

// jsonResponse replies with status and body.
func jsonResponse(status int, body any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}
}

// runCommand executes a command with the given arguments, capturing stdout.
func runCommand(t *testing.T, cmd interface {
	SetArgs([]string)
	Execute() error
}, args ...string) (stdout string, err error) {
	t.Helper()

	original := os.Stdout
	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)
	os.Stdout = w

	cmd.SetArgs(args)
	err = cmd.Execute()

	require.NoError(t, w.Close())
	os.Stdout = original

	out, readErr := io.ReadAll(r)
	require.NoError(t, readErr)
	return string(out), err
}

// decodeConfigBody decodes a config PATCH body, keeping nulls distinguishable
// from missing keys — the difference between "clear this" and "leave it alone".
func decodeConfigBody(t *testing.T, raw []byte) map[string]*string {
	t.Helper()
	var body map[string]*string
	require.NoError(t, json.Unmarshal(raw, &body), "body was not a JSON object: %s", raw)
	return body
}

// ---------------------------------------------------------------------------
// outpost config set
// ---------------------------------------------------------------------------

func TestOutpostConfigSetSendsThePatch(t *testing.T) {
	requests := stubOutpostAPI(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/config": jsonResponse(http.StatusOK, map[string]string{
			"TOPICS": "user.created", "MAX_RETRY_LIMIT": "3",
		}),
		"PATCH /2025-07-01/config": jsonResponse(http.StatusOK, map[string]string{
			"TOPICS": "user.created,user.updated",
		}),
	})

	cmd := newOutpostConfigSetCmd().cmd
	stdout, err := runCommand(t, cmd,
		"TOPICS=user.created,user.updated", "--unset", "MAX_RETRY_LIMIT")
	require.NoError(t, err)

	require.Len(t, *requests, 2, "the current config is read first so a diff can be shown")
	patch := (*requests)[1]
	assert.Equal(t, http.MethodPatch, patch.method)
	assert.Equal(t, "/2025-07-01/config", patch.path)

	body := decodeConfigBody(t, patch.body)
	require.Contains(t, body, "TOPICS")
	require.NotNil(t, body["TOPICS"])
	assert.Equal(t, "user.created,user.updated", *body["TOPICS"])

	// An unset key must be present with a null value. Omitting it would leave
	// the key set, and sending "" would set it to an empty string — neither
	// returns it to its default.
	require.Contains(t, body, "MAX_RETRY_LIMIT")
	assert.Nil(t, body["MAX_RETRY_LIMIT"])

	assert.Contains(t, stdout, "Updated 2 configuration value(s)")
	assert.Contains(t, stdout, "hookdeck outpost status", "the change is asynchronous, so say where to check")
}

func TestOutpostConfigSetDryRunSendsNoPatch(t *testing.T) {
	requests := stubOutpostAPI(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/config": jsonResponse(http.StatusOK, map[string]string{
			"TOPICS": "user.created", "MAX_RETRY_LIMIT": "3",
		}),
	})

	cmd := newOutpostConfigSetCmd().cmd
	stdout, err := runCommand(t, cmd, "TOPICS=nope", "--unset", "MAX_RETRY_LIMIT", "--dry-run")
	require.NoError(t, err)

	for _, r := range *requests {
		assert.Equal(t, http.MethodGet, r.method, "--dry-run must not write anything")
	}

	assert.Contains(t, stdout, "Dry run")
	assert.Contains(t, stdout, "before: user.created")
	assert.Contains(t, stdout, "after:  nope")
	// An unset shows what it reverts to, which is not the same as an empty value.
	assert.Contains(t, stdout, "after:  (default)")
	assert.Contains(t, stdout, "Re-run without --dry-run to apply.")
}

func TestOutpostConfigSetFromAFile(t *testing.T) {
	requests := stubOutpostAPI(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/config":   jsonResponse(http.StatusOK, map[string]string{}),
		"PATCH /2025-07-01/config": jsonResponse(http.StatusOK, map[string]string{"TOPICS": "a"}),
	})

	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"TOPICS":"a","MAX_RETRY_LIMIT":null}`), 0o600))

	cmd := newOutpostConfigSetCmd().cmd
	_, err := runCommand(t, cmd, "--config-file", path)
	require.NoError(t, err)

	body := decodeConfigBody(t, (*requests)[1].body)
	require.NotNil(t, body["TOPICS"])
	assert.Equal(t, "a", *body["TOPICS"])
	// A null in the file means the same as --unset.
	require.Contains(t, body, "MAX_RETRY_LIMIT")
	assert.Nil(t, body["MAX_RETRY_LIMIT"])
}

func TestOutpostConfigSetValidation(t *testing.T) {
	t.Run("nothing to change", func(t *testing.T) {
		cmd := newOutpostConfigSetCmd().cmd
		_, err := runCommand(t, cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nothing to change")
	})

	t.Run("arguments and a file conflict", func(t *testing.T) {
		cmd := newOutpostConfigSetCmd().cmd
		_, err := runCommand(t, cmd, "TOPICS=a", "--config-file", "x.json")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be used together")
	})

	t.Run("an argument without = is rejected", func(t *testing.T) {
		stubOutpostAPI(t, nil)
		cmd := newOutpostConfigSetCmd().cmd
		_, err := runCommand(t, cmd, "TOPICS")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be in KEY=VALUE form")
	})
}

func TestOutpostConfigGet(t *testing.T) {
	stubOutpostAPI(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/config": jsonResponse(http.StatusOK, map[string]any{
			"TOPICS": "user.created", "UNSET_KEY": nil,
		}),
	})

	t.Run("one key prints just the value", func(t *testing.T) {
		stdout, err := runCommand(t, newOutpostConfigGetCmd().cmd, "TOPICS")
		require.NoError(t, err)
		assert.Equal(t, "user.created\n", stdout, "scripts consume this, so it must be the bare value")
	})

	t.Run("an unknown key is named in the error", func(t *testing.T) {
		_, err := runCommand(t, newOutpostConfigGetCmd().cmd, "NOPE")
		require.Error(t, err)
		assert.Contains(t, err.Error(), `no configuration key named "NOPE"`)
	})

	t.Run("keys with no value are omitted from the listing", func(t *testing.T) {
		stdout, err := runCommand(t, newOutpostConfigGetCmd().cmd)
		require.NoError(t, err)
		assert.Contains(t, stdout, "TOPICS")
		assert.NotContains(t, stdout, "UNSET_KEY")
	})
}

// ---------------------------------------------------------------------------
// outpost config custom-domain
// ---------------------------------------------------------------------------

func TestOutpostCustomDomain(t *testing.T) {
	t.Run("get reports a configured domain and its DNS records", func(t *testing.T) {
		stubOutpostAPI(t, map[string]http.HandlerFunc{
			"GET /2025-07-01/config/custom_domain": jsonResponse(http.StatusOK, map[string]any{
				"hostname": "portal.example.com",
				"status":   "pending",
				"verification": []map[string]any{
					{"type": "CNAME", "name": "portal", "value": "outpost.hookdeck.com"},
				},
			}),
		})

		stdout, err := runCommand(t, newOutpostCustomDomainGetCmd().cmd)
		require.NoError(t, err)
		assert.Contains(t, stdout, "portal.example.com")
		assert.Contains(t, stdout, "Status: pending")
		assert.Contains(t, stdout, "Create these DNS records:")
		assert.Contains(t, stdout, "CNAME")
	})

	t.Run("get says how to add one when there is none", func(t *testing.T) {
		stubOutpostAPI(t, map[string]http.HandlerFunc{
			"GET /2025-07-01/config/custom_domain": jsonResponse(http.StatusOK, map[string]any{}),
		})

		stdout, err := runCommand(t, newOutpostCustomDomainGetCmd().cmd)
		require.NoError(t, err)
		assert.Contains(t, stdout, "No custom domain is configured.")
		assert.Contains(t, stdout, "custom-domain set")
	})

	t.Run("set posts the hostname and echoes the DNS records", func(t *testing.T) {
		requests := stubOutpostAPI(t, map[string]http.HandlerFunc{
			"POST /2025-07-01/config/custom_domain": jsonResponse(http.StatusCreated, map[string]any{
				"hostname": "portal.example.com",
				"verification": []map[string]any{
					{"type": "CNAME", "name": "portal", "value": "outpost.hookdeck.com"},
				},
			}),
		})

		stdout, err := runCommand(t, newOutpostCustomDomainSetCmd().cmd, "portal.example.com")
		require.NoError(t, err)

		require.Len(t, *requests, 1)
		assert.Equal(t, http.MethodPost, (*requests)[0].method)
		assert.JSONEq(t, `{"hostname":"portal.example.com"}`, string((*requests)[0].body))
		assert.Contains(t, stdout, "portal.example.com")
		assert.Contains(t, stdout, "CNAME", "without the records the domain never verifies")
	})

	t.Run("delete --force skips the prompt", func(t *testing.T) {
		requests := stubOutpostAPI(t, map[string]http.HandlerFunc{
			"DELETE /2025-07-01/config/custom_domain": jsonResponse(http.StatusOK, nil),
		})

		stdout, err := runCommand(t, newOutpostCustomDomainDeleteCmd().cmd, "--force")
		require.NoError(t, err)

		require.Len(t, *requests, 1)
		assert.Equal(t, http.MethodDelete, (*requests)[0].method)
		assert.Equal(t, "/2025-07-01/config/custom_domain", (*requests)[0].path)
		assert.Contains(t, stdout, "Custom domain removed")
	})
}

// A destructive command without --force must prompt. The prompt cannot be
// answered from a non-interactive test, so this asserts the command declines to
// proceed rather than deleting anything — the failure mode that matters is a
// delete going through unprompted in a script.
func TestOutpostCustomDomainDeleteWithoutForceDoesNotDelete(t *testing.T) {
	requests := stubOutpostAPI(t, map[string]http.HandlerFunc{
		"DELETE /2025-07-01/config/custom_domain": func(w http.ResponseWriter, r *http.Request) {
			t.Error("the domain was deleted without confirmation")
		},
	})

	_, _ = runCommand(t, newOutpostCustomDomainDeleteCmd().cmd)
	assert.Empty(t, *requests, "no request may be made until the deletion is confirmed")
}

// ---------------------------------------------------------------------------
// outpost tenant portal
// ---------------------------------------------------------------------------

func TestOutpostTenantPortal(t *testing.T) {
	t.Run("prints the URL and passes the theme through", func(t *testing.T) {
		requests := stubOutpostAPI(t, map[string]http.HandlerFunc{
			"GET /2025-07-01/tenants/acme/portal": jsonResponse(http.StatusOK, map[string]any{
				"redirect_url": "https://portal.example.com/s/abc",
				"tenant_id":    "acme",
			}),
		})

		stdout, err := runCommand(t, newOutpostTenantPortalCmd().cmd, "acme", "--theme", "dark")
		require.NoError(t, err)

		require.Len(t, *requests, 1)
		assert.Equal(t, "theme=dark", (*requests)[0].query)
		// The URL is a credential and scripts pipe it, so it must be the only
		// thing on stdout.
		assert.Equal(t, "https://portal.example.com/s/abc\n", stdout)
	})

	// The endpoint answers 404 whenever the project has no portal, which reads
	// as a missing tenant unless the CLI says otherwise. The precondition is
	// documented in the command's help, so the error has to name it too.
	t.Run("a project with no portal is explained, not dumped", func(t *testing.T) {
		stubOutpostAPI(t, map[string]http.HandlerFunc{
			"GET /2025-07-01/tenants/acme/portal": jsonResponse(http.StatusNotFound, map[string]any{
				"data": map[string]any{"message": "Portal not configured for this project"},
				"code": "NOT_FOUND",
			}),
		})

		_, err := runCommand(t, newOutpostTenantPortalCmd().cmd, "acme")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no tenant portal")
		assert.Contains(t, err.Error(), "custom-domain set", "the error has to name the fix")
		assert.NotContains(t, err.Error(), `"code"`, "the raw response body is not an error message")
	})

	t.Run("an invalid theme is rejected before the request", func(t *testing.T) {
		requests := stubOutpostAPI(t, nil)
		_, err := runCommand(t, newOutpostTenantPortalCmd().cmd, "acme", "--theme", "neon")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--theme must be either light or dark")
		assert.Empty(t, *requests)
	})
}

// ---------------------------------------------------------------------------
// --metadata on destinations (parity with tenant upsert and the MCP tool)
// ---------------------------------------------------------------------------

func TestOutpostDestinationCreateSendsMetadata(t *testing.T) {
	requests := stubOutpostAPI(t, map[string]http.HandlerFunc{
		"GET /2025-07-01/destination-types": jsonResponse(http.StatusOK, []map[string]any{{
			"type":          "webhook",
			"config_fields": []map[string]any{{"key": "url", "type": "text", "required": true}},
		}}),
		"POST /2025-07-01/tenants/acme/destinations": jsonResponse(http.StatusCreated, map[string]any{
			"id": "des_1", "type": "webhook", "topics": "*",
		}),
	})

	parent := newOutpostDestinationCmd()
	_, err := runCommand(t, parent.cmd, "create",
		"--tenant-id", "acme", "--type", "webhook",
		"--config", "url=https://example.com/hooks",
		"--metadata", "owner=platform", "--metadata", "tier=pro")
	require.NoError(t, err)

	var body map[string]any
	for _, r := range *requests {
		if r.method == http.MethodPost {
			require.NoError(t, json.Unmarshal(r.body, &body))
		}
	}
	require.NotNil(t, body, "no create request was sent")
	assert.Equal(t, map[string]any{"owner": "platform", "tier": "pro"}, body["metadata"])
}

func TestOutpostDestinationUpdateSendsMetadataFromAFile(t *testing.T) {
	requests := stubOutpostAPI(t, map[string]http.HandlerFunc{
		"PATCH /2025-07-01/tenants/acme/destinations/des_1": jsonResponse(http.StatusOK, map[string]any{
			"id": "des_1", "type": "webhook", "topics": "*",
		}),
	})

	path := filepath.Join(t.TempDir(), "metadata.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"owner":"platform"}`), 0o600))

	parent := newOutpostDestinationCmd()
	_, err := runCommand(t, parent.cmd, "update", "des_1",
		"--tenant-id", "acme", "--metadata-file", path)
	require.NoError(t, err)

	require.Len(t, *requests, 1, "metadata alone needs no destination-type lookup")
	var body map[string]any
	require.NoError(t, json.Unmarshal((*requests)[0].body, &body))
	assert.Equal(t, map[string]any{"owner": "platform"}, body["metadata"])
	// Nothing else was passed, so nothing else may be sent: this endpoint
	// merge-patches, and an empty config would clear the destination's config.
	assert.NotContains(t, body, "config")
	assert.NotContains(t, body, "topics")
}

func TestOutpostDestinationMetadataValidation(t *testing.T) {
	t.Run("--metadata and --metadata-file conflict", func(t *testing.T) {
		stubOutpostAPI(t, nil)
		parent := newOutpostDestinationCmd()
		_, err := runCommand(t, parent.cmd, "update", "des_1",
			"--tenant-id", "acme", "--metadata", "a=b", "--metadata-file", "x.json")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--metadata and --metadata-file cannot be used together")
	})

	t.Run("a pair without = is rejected", func(t *testing.T) {
		got, err := resolveOutpostMetadata([]string{"justakey"}, "")
		require.Error(t, err)
		assert.Nil(t, got)
		assert.Contains(t, err.Error(), "must be in key=value form")
	})

	t.Run("metadata alone satisfies update's has-anything check", func(t *testing.T) {
		fields := outpostDestinationFieldFlags{metadata: []string{"a=b"}}
		assert.True(t, fields.hasAny(),
			"--metadata on its own must be a valid update, not 'nothing to update'")
	})

	t.Run("no metadata flags means leave metadata alone", func(t *testing.T) {
		got, err := resolveOutpostMetadata(nil, "")
		require.NoError(t, err)
		assert.Nil(t, got, "nil, not an empty map, or the update would clear the metadata")
	})
}

// ---------------------------------------------------------------------------
// Empty-value rejection on outpost flags
// ---------------------------------------------------------------------------

// An unexported shell variable expands to an empty string, so `--tenant-id
// "$TENANT"` would otherwise address a different path rather than fail.
func TestOutpostFlagsRejectEmptyValues(t *testing.T) {
	cases := []struct {
		name string
		args []string
		flag string
	}{
		{"tenant-id on destination list", []string{"list", "--tenant-id", ""}, "--tenant-id"},
		{"type on destination create", []string{"create", "--tenant-id", "acme", "--type", ""}, "--type"},
		{"config on destination create", []string{"create", "--tenant-id", "acme", "--type", "webhook", "--config", ""}, "--config"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requests := stubOutpostAPI(t, nil)
			parent := newOutpostDestinationCmd()
			_, err := runCommand(t, parent.cmd, tc.args...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.flag)
			assert.Empty(t, *requests, "an empty value must fail before any request")
		})
	}

	t.Run("theme on tenant portal", func(t *testing.T) {
		requests := stubOutpostAPI(t, nil)
		_, err := runCommand(t, newOutpostTenantPortalCmd().cmd, "acme", "--theme", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--theme")
		assert.Empty(t, *requests)
	})
}
