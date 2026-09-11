# Hookdeck CLI Acceptance Tests

This directory contains Go-based acceptance tests for the Hookdeck CLI. These tests verify end-to-end functionality by executing the CLI and validating outputs.

## Test Categories

Tests are divided into two categories:

### 1. Automated Tests (CI-Compatible)
These tests run automatically in CI using API keys from `hookdeck ci`. They don't require human interaction.

**Files:** Test files with **feature build tags** (e.g. `//go:build connection`, `//go:build request`). Each automated test file has exactly one feature tag so tests can be split into parallel slices (see [Parallelisation](#parallelisation)).

**Login recovery (mock API, `basic` tag):** `login_auth_acceptance_test.go` runs the real CLI with `--api-base` pointing at a local server that returns **401** on `GET .../cli-auth/validate`, then completes a fake device-auth poll — this asserts `hookdeck login` continues into browser/device flow after a stale key (no human, no real Hookdeck key). The same file includes **`TestCIFailsFastWithInvalidAPIKeyAcceptance`**, which runs `hookdeck ci --api-key` with a bogus key against the real API and expects a quick failure with the friendly **Authentication failed** message, and asserts output does **not** contain browser/device-login phrases (`Press Enter to open the browser`, `To authenticate with Hookdeck`, etc.) so CI never enters the interactive `hookdeck login` flow.

**Guest login (mock API, `guest` tag):** `guest_login_acceptance_test.go` asserts `POST /cli-auth` receives guest credentials when a guest profile is present, and omits them after logout (empty profile).

### 1b. Outpost live smoke test (`outpostlive` tag)

`outpost_live_test.go` carries `//go:build outpostlive` and is **not** part of the pull-request acceptance matrix. It calls the Outpost API client directly against a real Outpost host, which is the only thing that proves the client's stored credentials, request shapes and response decoding work outside a stub server — everything under `-tags=outpost` drives the CLI, and the client's unit tests only assert the implementation's assumptions back at it.

It is kept out of the PR gate because a live-deployment problem would then fail every unrelated pull request. Instead it runs:

- **nightly**, and on demand, via `.github/workflows/outpost-live.yml` (`workflow_dispatch`);
- **before cutting a release** — run it manually if the last nightly is not recent.

```bash
# Requires HOOKDECK_CLI_OUTPOST_TESTING_API_KEY (a Project API key for an Outpost project)
go test -tags=outpostlive ./test/acceptance/... -v -timeout 12m
```

The tests skip themselves when the key is absent, which is right on a developer machine and wrong in CI, so the workflow fails fast if the secret is missing rather than reporting a green job that tested nothing.

### 2. Manual Tests (Require Human Interaction)
These tests require browser-based authentication via `hookdeck login` and must be run manually by developers.

**Files:** Test files with `//go:build manual` tag (e.g., `project_use_manual_test.go`)

**Why Manual?** These tests access endpoints (like `/teams`) that require CLI authentication keys obtained through interactive browser login, which aren't available to CI service accounts.

### Transient HTTP 502 / 500 from the API

`CLIRunner.Run`, `RunWithEnv`, and `RunFromCwd` retry the same command up to **4** times when combined stdout/stderr looks like a transient Hookdeck API **HTTP 502** or **HTTP 500** (matching CLI log text such as `status=502` / `status=500`). **503** and **504** are not treated specially. Each retry is logged with `t.Logf` (attempt number, command summary, output excerpts); if all attempts fail, a final log line notes that the run is giving up.

### Recording proxy (telemetry tests)

Some tests (e.g. `TestTelemetryGatewayConnectionListProxy` in `telemetry_test.go`, `TestTelemetryListenProxy` in `telemetry_listen_test.go`) use a **recording proxy**: the CLI is run with `--api-base` pointing at a local HTTP server that forwards every request to the real Hookdeck API and records method, path, and the `X-Hookdeck-CLI-Telemetry` header. The same `CLIRunner` and `go run main.go` flow are used as in other acceptance tests; only the API base URL is overridden so traffic goes through the proxy. This verifies that a single CLI run sends consistent telemetry (same `invocation_id` and `command_path`) on all API calls. Helpers: `StartRecordingProxy`, `AssertTelemetryConsistent`.

**Login telemetry tests** use the same proxy approach with **HOOKDECK_CLI_TESTING_CLI_KEY** (a CLI client key, not a Project API key used with `hookdeck ci`). If unset, those tests are skipped. **TestTelemetryLoginProxy** runs `hookdeck login --api-key KEY` with `--api-base` set to the proxy and asserts exactly one recorded request (GET `/2025-07-01/cli-auth/validate`) with consistent telemetry. **TestTelemetryLoginCommandFlagsProxy** additionally asserts the telemetry JSON includes **`command_flags`** containing **`api-key`** or **`cli-key`** on the wire when that flag is passed. Other telemetry tests still use the normal Project API key via `NewCLIRunner`.

See **README.md § [CLI authentication keys](../README.md#cli-authentication-keys)** for claimed vs unclaimed keys and how Project API keys relate to `hookdeck ci`.

## Setup

### Local Development

For local testing, create a `.env` file in this directory:

```bash
# test/acceptance/.env
HOOKDECK_CLI_TESTING_API_KEY=your_api_key_here
# Optional: CLI client key for project list tests only (claimed key; see README § CLI authentication keys)
# HOOKDECK_CLI_TESTING_CLI_KEY=your_cli_client_key_here
```

The `.env` file is automatically loaded when tests run. **This file is git-ignored and should never be committed.**

For **parallel local runs**, add a second and third key so each slice uses its own project:
```bash
HOOKDECK_CLI_TESTING_API_KEY=key_for_slice0
HOOKDECK_CLI_TESTING_API_KEY_2=key_for_slice1
HOOKDECK_CLI_TESTING_API_KEY_3=key_for_slice2
```

### CI/CD

CI runs **three parallel matrix jobs**, each with its own API key (`HOOKDECK_CLI_TESTING_API_KEY`, `HOOKDECK_CLI_TESTING_API_KEY_2`, `HOOKDECK_CLI_TESTING_API_KEY_3`). Those jobs set **`HOOKDECK_CLI_TELEMETRY_DISABLED=1`** so the CLI does not send telemetry during normal acceptance tests.

A **fourth job** (`acceptance-telemetry` in `.github/workflows/test-acceptance.yml`) sets **`HOOKDECK_CLI_TELEMETRY_DISABLED=0`** so the CLI sends `X-Hookdeck-CLI-Telemetry` even if the repository or organization defines `HOOKDECK_CLI_TELEMETRY_DISABLED=1` for other jobs. It runs `go test -tags=telemetry` only (all proxy tests that assert that header, including listen, live under the `telemetry` build tag). It uses `HOOKDECK_CLI_TESTING_API_KEY` and `ACCEPTANCE_SLICE=0` (same project as slice 0; tests use unique resource names).

No test-name list in the workflow—tests are partitioned by **feature tags** (see [Parallelisation](#parallelisation)).

## Running Tests

### Run all automated tests (one key)
Pass all feature tags so every automated test file is included:
```bash
go test -tags="basic guest connection source destination gateway mcp listen project_use connection_list connection_upsert connection_error_hints connection_oauth_aws connection_update request event telemetry attempt metrics issue transformation outpost" ./test/acceptance/... -v
```

### Run one slice (for CI or local)
Same commands as CI; use when debugging a subset or running in parallel:
```bash
# Slice 0 (same tags as CI job 0)
ACCEPTANCE_SLICE=0 HOOKDECK_CLI_TELEMETRY_DISABLED=1 go test -tags="basic guest connection source mcp listen project_use connection_list connection_upsert connection_error_hints connection_oauth_aws connection_update" ./test/acceptance/... -v -timeout 12m

# Slice 1 (same tags as CI job 1)
ACCEPTANCE_SLICE=1 HOOKDECK_CLI_TELEMETRY_DISABLED=1 go test -tags="request event" ./test/acceptance/... -v -timeout 12m

# Slice 3 (same tags as CI job 3) - requires HOOKDECK_CLI_OUTPOST_TESTING_API_KEY
ACCEPTANCE_SLICE=3 HOOKDECK_CLI_TELEMETRY_DISABLED=1 go test -tags="outpost" ./test/acceptance/... -v -timeout 12m

# Slice 2 (same tags as CI job 2)
ACCEPTANCE_SLICE=2 HOOKDECK_CLI_TELEMETRY_DISABLED=1 go test -tags="attempt metrics issue transformation destination gateway" ./test/acceptance/... -v -timeout 12m

# Telemetry (same as CI acceptance-telemetry: force telemetry on)
ACCEPTANCE_SLICE=0 HOOKDECK_CLI_TELEMETRY_DISABLED=0 go test -tags=telemetry ./test/acceptance/... -v -timeout 12m
```
For slice 1 set `HOOKDECK_CLI_TESTING_API_KEY_2`; for slice 2 set `HOOKDECK_CLI_TESTING_API_KEY_3` (or set `HOOKDECK_CLI_TESTING_API_KEY` to that key). For telemetry, use the slice 0 key and set `HOOKDECK_CLI_TELEMETRY_DISABLED=0` (overrides a global opt-out).

**Project list tests** (`TestProjectListShowsType`, `TestProjectListJSONOutput`, and related filters) require a **CLI client key** (`HOOKDECK_CLI_TESTING_CLI_KEY`), not the Project API key used with `hookdeck ci`. Claimed keys from dashboard onboarding or Console CLI destination setup work; unclaimed device-auth keys are not suitable. If unset, these tests are skipped. See **README.md § [CLI authentication keys](../README.md#cli-authentication-keys)**.

### Run in parallel locally (three keys)
From the **repository root**, run the script that runs three matrix slices plus telemetry in parallel (same as CI):
```bash
./test/acceptance/run_parallel.sh
```
Requires `HOOKDECK_CLI_TESTING_API_KEY`, `HOOKDECK_CLI_TESTING_API_KEY_2`, and `HOOKDECK_CLI_TESTING_API_KEY_3` in `.env` or the environment. The script sets `HOOKDECK_CLI_TELEMETRY_DISABLED=1` for matrix slices and `HOOKDECK_CLI_TELEMETRY_DISABLED=0` for the telemetry run (same as CI).

### Run manual tests (requires human authentication):
```bash
go test -tags=manual -v ./test/acceptance/
```

### Run specific manual test:
```bash
go test -tags=manual -run TestProjectUseLocalCreatesConfig -v ./test/acceptance/
```

### Skip acceptance tests (short mode):
```bash
go test -short ./test/acceptance/...
```
Use the same `-tags` as "Run all" if you want to skip the full acceptance set. All acceptance tests are skipped when `-short` is used, allowing fast unit test runs.

## Parallelisation

Tests are partitioned by **feature build tags** so CI and local runs can execute three matrix slices in parallel (each slice uses its own Hookdeck project and config file).

- **Slice 0 features:** `basic`, `guest`, `connection`, `source`, `mcp`, `listen`, `project_use`, `connection_list`, `connection_upsert`, `connection_error_hints`, `connection_oauth_aws`, `connection_update`
- **Slice 1 features:** `request`, `event`
- **Slice 2 features:** `attempt`, `metrics`, `issue`, `transformation`, `destination`, `gateway`
- **Telemetry job:** `telemetry` only — separate CI job with telemetry **not** disabled (see [CI/CD](#cicd))

The CI workflow (`.github/workflows/test-acceptance.yml`) runs three matrix jobs plus `acceptance-telemetry`. Matrix jobs set `HOOKDECK_CLI_TELEMETRY_DISABLED=1`; the telemetry job does not. No test names or regexes are listed in YAML.

**Untagged files:** A test file with **no** build tag is included in **every** `go test -tags=...` build, including **`acceptance-telemetry`** (`-tags=telemetry` only), so non-telemetry tests would run there too. **Every new acceptance test file must have exactly one feature tag** so it runs in only one matrix slice and not in the telemetry job.

## Manual Test Workflow

When you run manual tests, here's what happens:

### Example Session
```bash
$ go test -tags=manual -v ./test/acceptance/

=== RUN   TestProjectUseLocalCreatesConfig

🔐 Fresh Authentication Required
=================================
These tests require fresh CLI authentication with project access.

Step 1: Clearing existing authentication...
✅ Authentication cleared

Step 2: Starting login process...
Running: hookdeck login

[Browser opens for authentication - complete the login process]

Please complete the browser authentication if not already done.
Press Enter when you've successfully logged in and are ready to continue...

[User presses Enter]

Verifying authentication...
✅ Authenticated successfully: Logged in as user@example.com on project my-project in organization Acme Inc

--- PASS: TestProjectUseLocalCreatesConfig (15.34s)

=== RUN   TestProjectUseSmartDefault
✅ Already authenticated (from previous test)
--- PASS: TestProjectUseSmartDefault (1.12s)

...
```

### What the Helper Does

The [`RequireCLIAuthenticationOnce(t)`](helpers.go:268) helper function:

1. **Clears existing authentication** by running `hookdeck logout` and deleting config files
2. **Runs `hookdeck login`** which opens a browser for authentication
3. **Waits for you to press Enter** after completing browser authentication (gives you full control)
4. **Verifies authentication** by running `hookdeck whoami`
5. **Fails the test** if authentication doesn't succeed
6. **Runs only once per test session** - subsequent tests in the same run reuse the authentication

### Which Tests Require Manual Authentication

**Automated Tests (project_use_test.go):**
- ✅ `TestProjectUseLocalAndConfigFlagConflict` - Flag validation only, no API calls
- ✅ `TestLocalConfigHelpers` - Helper function tests, no API calls

**Manual Tests (project_use_manual_test.go):**
- 🔐 `TestProjectUseLocalCreatesConfig` - Requires `/teams` endpoint access
- 🔐 `TestProjectUseSmartDefault` - Requires `/teams` endpoint access
- 🔐 `TestProjectUseLocalCreateDirectory` - Requires `/teams` endpoint access
- 🔐 `TestProjectUseLocalSecurityWarning` - Requires `/teams` endpoint access

### Tips for Running Manual Tests

- **Run all manual tests together** to authenticate only once:
  ```bash
  go test -tags=manual -v ./test/acceptance/
  ```

- **Authentication persists** across tests in the same run (handled by `RequireCLIAuthenticationOnce`)

- **Fresh authentication each run** - existing auth is always cleared at the start

- **Be ready to authenticate** - the browser will open automatically when you run the tests

## Test Structure

### Files

- **`helpers.go`** - Test infrastructure and utilities
  - `CLIRunner` - Executes CLI commands via `go run main.go`
  - `RequireCLIAuthentication(t)` - Forces fresh CLI authentication for manual tests
  - `RequireCLIAuthenticationOnce(t)` - Authenticates once per test run
  - Helper functions for creating/deleting test resources
  - JSON parsing utilities
  - Data structures (Connection, etc.)

- **`resource_cleanup.go`** - The bookkeeping that stops tests leaking resources
  - Records what each CLI command created and deletes the leftovers when the test ends
  - `cleanupListenResources` for the source and CLI destination `hookdeck listen` creates on the fly
  - See [Resource cleanup](#resource-cleanup)
  
- **`basic_test.go`** - Basic CLI functionality tests
  - Version command
  - Help command
  - Authentication (ci mode with API key)
  - Whoami verification

- **`connection_test.go`** - Connection CRUD tests
  - List connections
  - Create and delete connections
  - Update connection metadata
  - Various source/destination types

- **`listen_test.go`** - Listen command tests
  - Basic listen command startup and termination
  - Context-based process management
  - Background process handling

- **`project_use_test.go`** - Project use automated tests (CI-compatible)
  - Flag validation tests
  - Helper function tests
  - Tests that don't require `/teams` endpoint access

- **`project_use_manual_test.go`** - Project use manual tests (requires human auth)
  - Build tag: `//go:build manual`
  - Tests that require browser-based authentication
  - Tests that access `/teams` endpoint

- **`.env`** - Local environment variables (git-ignored)

### Key Components

#### CLIRunner

The `CLIRunner` struct provides methods to execute CLI commands:

```go
cli := NewCLIRunner(t)

// Run command and get output
stdout, stderr, err := cli.Run("connection", "list")

// Run command expecting success
stdout := cli.RunExpectSuccess("connection", "list")

// Run command and parse JSON output
var conn Connection
err := cli.RunJSON(&conn, "connection", "get", connID)
```

#### Test Helpers

- `createTestConnection(t, cli)` - Creates a basic test connection
- `deleteConnection(t, cli, id)` - Deletes a connection (for cleanup)
- `generateTimestamp()` - Generates unique timestamp for resource names

## Writing Tests

All tests should:

1. **Have a feature build tag:** Every new automated test file must have exactly one `//go:build <feature>` at the top (e.g. `//go:build connection`, `//go:build request`). This assigns the file to a slice for parallel runs. Without a tag, the file runs in both slices (duplicated). See existing `*_test.go` files for examples.

2. **Skip in short mode:**
   ```go
   if testing.Short() {
       t.Skip("Skipping acceptance test in short mode")
   }
   ```

3. **Use cleanup for resources:**
   ```go
   t.Cleanup(func() {
       deleteConnection(t, cli, connID)
   })
   ```

4. **Use descriptive names:**
   ```go
   func TestConnectionWithStripeSource(t *testing.T) { ... }
   ```

5. **Log important information:**
   ```go
   t.Logf("Created connection: %s (ID: %s)", name, id)
   ```

## Resource cleanup

Deleting a connection does **not** delete the source and destination it was created with. `gateway connection create --source-name … --destination-name …` creates three resources, and a test that cleans up the connection alone leaves two behind. That is how the test projects reached ~36,000 orphaned sources and ~36,500 destinations ([#362](https://github.com/hookdeck/hookdeck-cli/issues/362)).

So `CLIRunner` keeps the books itself, in [`resource_cleanup.go`](resource_cleanup.go):

- every command that reports creating a gateway resource has that resource's id recorded — including the source and destination returned inline by a connection create;
- every command that deletes one by id has it struck off;
- whatever is still on the list when the test ends is deleted, connections first.

Two consequences worth knowing:

- **A test that cleans up after itself costs nothing.** Its resources are struck off before the sweep runs, so the sweep makes no API calls for them. Keep writing the explicit `t.Cleanup` — it deletes earlier, and it says what the test meant.
- **A test that fails half-way still cleans up.** Resources created before the failing assertion are already recorded, which is exactly the case explicit cleanup misses, because the `t.Cleanup` call is usually below the assertion that failed.

When the sweep has work to do it says so:

```
resource_cleanup.go:239: acceptance cleanup: deleted 2 resource(s) the test left behind (0 already gone, 0 failed)
```

`hookdeck listen` is the exception: it creates the source it is pointed at when that source does not exist, plus a `cli-<source>` connection and destination, and none of that goes through `CLIRunner.Run`, so no id is ever seen. Tests that start `listen` through `startListenCapturingOutput` or `RunListenWithTimeout` are covered automatically (both call `registerListenCleanup`). A test that starts the binary itself must register the cleanup by name:

```go
t.Cleanup(func() { cleanupListenResources(t, cli, sourceName) })
```

## Environment Requirements

- **Go 1.24.9+**
- **Valid Hookdeck API key** with appropriate permissions
- **Network access** to Hookdeck API

## Migration from Shell Scripts

These Go-based tests replace the shell script acceptance tests in `test-scripts/test-acceptance.sh`. The Go version provides:

- Better error handling and reporting
- Cross-platform compatibility
- Integration with Go's testing framework
- Easier maintenance and debugging
- Structured test output with `-v` flag

### Shell Script Coverage Mapping

All functionality from `test-scripts/test-acceptance.sh` has been successfully ported to Go tests:

| Shell Script Test (Line) | Go Test Location | Status |
|--------------------------|------------------|--------|
| Build CLI (33-34) | Not needed - `go run` builds automatically | ✅ N/A |
| Version command (40-41) | [`basic_test.go:TestCLIBasics/Version`](basic_test.go:18) | ✅ Ported |
| Help command (43-44) | [`basic_test.go:TestCLIBasics/Help`](basic_test.go:31) | ✅ Ported |
| CI auth (47) | [`helpers.go:NewCLIRunner`](helpers.go) | ✅ Ported |
| Whoami (49-50) | [`basic_test.go:TestCLIBasics/Authentication`](basic_test.go:43) | ✅ Ported |
| Listen command (52-70) | [`listen_test.go:TestListenCommandBasic`](listen_test.go:15) | ✅ Ported |
| Connection list (75-76) | [`connection_test.go:TestConnectionListBasic`](connection_test.go:13) | ✅ Ported |
| Connection create - WEBHOOK (124-131) | [`connection_test.go:TestConnectionAuthenticationTypes/WEBHOOK_Source_NoAuth`](connection_test.go:140) | ✅ Ported |
| Connection create - STRIPE (133-141) | [`connection_test.go:TestConnectionAuthenticationTypes/STRIPE_Source_WebhookSecret`](connection_test.go:212) | ✅ Ported |
| Connection create - HTTP API key (143-152) | [`connection_test.go:TestConnectionAuthenticationTypes/HTTP_Source_APIKey`](connection_test.go:281) | ✅ Ported |
| Connection create - HTTP basic auth (154-163) | [`connection_test.go:TestConnectionAuthenticationTypes/HTTP_Source_BasicAuth`](connection_test.go:346) | ✅ Ported |
| Connection create - TWILIO HMAC (165-174) | [`connection_test.go:TestConnectionAuthenticationTypes/TWILIO_Source_HMAC`](connection_test.go:419) | ✅ Ported |
| Connection create - HTTP dest bearer (178-187) | [`connection_test.go:TestConnectionAuthenticationTypes/HTTP_Destination_BearerToken`](connection_test.go:493) | ✅ Ported |
| Connection create - HTTP dest basic (189-199) | [`connection_test.go:TestConnectionAuthenticationTypes/HTTP_Destination_BasicAuth`](connection_test.go:576) | ✅ Ported |
| Connection update (201-238) | [`connection_test.go:TestConnectionUpdate`](connection_test.go:57) | ✅ Ported |
| Connection bulk delete (240-246) | [`connection_test.go:TestConnectionBulkDelete`](connection_test.go:707) | ✅ Ported |
| Logout (251-252) | Not needed - handled automatically by test cleanup | ✅ N/A |

**Migration Notes:**
- Build step is unnecessary in Go tests as `go run` compiles on-the-fly
- Authentication is handled centrally in `NewCLIRunner()` helper
- Logout is not required as each test gets a fresh runner instance
- Go tests provide better isolation with `t.Cleanup()` for resource management
- All authentication types and edge cases are covered with more granular tests

## Troubleshooting

### API Key Not Set
```
Error: HOOKDECK_CLI_TESTING_API_KEY (or HOOKDECK_CLI_TESTING_API_KEY_2 for slice 1) must be set
```
**Solution:** Create a `.env` file in `test/acceptance/` with `HOOKDECK_CLI_TESTING_API_KEY`. For parallel runs (or slice 1), also set `HOOKDECK_CLI_TESTING_API_KEY_2`.

### Command Execution Failures
If commands fail to execute, ensure you're running from the project root or that the working directory is set correctly.

### Resource Cleanup
Tests use `t.Cleanup()` to ensure resources are deleted even if tests fail, and `CLIRunner` sweeps up whatever they miss (see [Resource cleanup](#resource-cleanup)). If you see orphaned resources, check the log line each test prints when the sweep had work to do.