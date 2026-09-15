package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
)

// PUT /transformations/run answers 200 for a throwing handler exactly as it
// does for a clean one; only the body differs. These are the bodies the live
// API returned at API version 2026-09-01, so the stub is the real contract and
// not an assumption about it.
const (
	// A handler that throws: fatal, the reason only in console, and no request
	// at all. This is #410 — the CLI printed "{}" and exited 0 for it.
	runResponseThrew = `{
		"log_level": "fatal",
		"console": [{"type": "error", "message": "Error: boom"}]
	}`

	// A handler that logs and then returns: the run completed, so the caller
	// wants the transformed request AND the diagnostics it printed on the way.
	runResponseLoggedAndReturned = `{
		"log_level": "error",
		"console": [{"type": "error", "message": "unexpected field"}],
		"request": {"headers": {"content-type": "application/json"}}
	}`

	// A clean run.
	runResponseClean = `{
		"log_level": "info",
		"request": {"headers": {"content-type": "application/json"}}
	}`
)

// TestTransformationRunFailureIsAnError covers the human output path. A
// transformation that did not complete must be an error, and the console output
// is the only place the reason appears, so it has to reach the user.
func TestTransformationRunFailureIsAnError(t *testing.T) {
	stdout, err := runTransformationRunAgainst(t, runResponseThrew,
		"--code", `addHandler("transform",(r,c)=>{ throw new Error("boom"); });`,
		"--request", `{"headers":{}}`)

	require.Error(t, err, "a handler that threw must not exit 0")
	assert.Contains(t, err.Error(), "did not complete")
	assert.Contains(t, err.Error(), "Error: boom",
		"the console output is the only report of why the code failed")
	assert.NotContains(t, stdout, "Transformation run completed",
		"a run that did not complete must not print the success tick")
}

// TestTransformationRunFailureIsAnErrorWithOutputJSON is the same failure on the
// --output json path, which has a second requirement: the payload is what a
// script consumes, so it must still be printed before the command fails.
func TestTransformationRunFailureIsAnErrorWithOutputJSON(t *testing.T) {
	stdout, err := runTransformationRunAgainst(t, runResponseThrew,
		"--code", `addHandler("transform",(r,c)=>{ throw new Error("boom"); });`,
		"--request", `{"headers":{}}`,
		"--output", "json")

	require.Error(t, err, "a handler that threw must not exit 0 with --output json either")
	assert.Contains(t, err.Error(), "did not complete")
	assert.Contains(t, stdout, `"log_level": "fatal"`,
		"the payload must still be printed - it carries the diagnosis")
	assert.Contains(t, stdout, "Error: boom")
	assert.NotEqual(t, "{}", strings.TrimSpace(stdout),
		"the empty object in #410 was the response parsing into a struct with no fields for it")
}

// TestTransformationRunSuccessStillExitsZero is the other direction. Failed()
// keys off log_level, which is the highest severity logged and not a completion
// flag, so a working transformation must keep exiting 0 and printing its result.
func TestTransformationRunSuccessStillExitsZero(t *testing.T) {
	stdout, err := runTransformationRunAgainst(t, runResponseClean,
		"--code", `addHandler("transform",(r,c)=>{ return r; });`,
		"--request", `{"headers":{}}`)

	require.NoError(t, err)
	assert.Contains(t, stdout, "Transformation run completed")
	assert.Contains(t, stdout, "content-type", "the transformed request is the point of the command")
}

// TestTransformationRunPrintsConsoleOnSuccess covers a run that completed but
// logged on the way. Printing the console only on failure hid the diagnostics
// from the case a user is most likely to be debugging: code that returns
// something, but complains while doing it.
func TestTransformationRunPrintsConsoleOnSuccess(t *testing.T) {
	stdout, err := runTransformationRunAgainst(t, runResponseLoggedAndReturned,
		"--code", `addHandler("transform",(r,c)=>{ console.error("unexpected field"); return r; });`,
		"--request", `{"headers":{}}`)

	require.NoError(t, err, "a handler that logged an error and still returned a request succeeded")
	assert.Contains(t, stdout, "Transformation run completed")
	assert.Contains(t, stdout, "unexpected field", "console output must be surfaced on a successful run")
	assert.Contains(t, stdout, "content-type", "and the transformed request must still be printed")
}

// transformationRunStub serves PUT /transformations/run with a fixed body.
func transformationRunStub(t *testing.T, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/transformations/run")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

// runTransformationRunAgainst drives the command's own RunE against a stub, so
// the flags are parsed as the user typed them and stdout is what the user sees.
func runTransformationRunAgainst(t *testing.T, responseBody string, args ...string) (string, error) {
	t.Helper()
	server := transformationRunStub(t, responseBody)

	old := Config
	t.Cleanup(func() { Config = old })
	config.ResetAPIClientForTesting()
	t.Cleanup(config.ResetAPIClientForTesting)

	Config = config.Config{}
	Config.APIBaseURL = server.URL
	Config.Profile.APIKey = "sk_test_123456789012"
	Config.Profile.ProjectId = "proj_1"

	tc := newTransformationRunCmd()
	require.NoError(t, tc.cmd.ParseFlags(args))

	oldStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)
	os.Stdout = w

	runErr := tc.runTransformationRunCmd(tc.cmd, nil)

	require.NoError(t, w.Close())
	os.Stdout = oldStdout

	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, readErr := r.Read(buf)
		sb.Write(buf[:n])
		if readErr != nil {
			break
		}
	}
	return sb.String(), runErr
}
