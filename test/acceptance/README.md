# Hookdeck CLI Acceptance Tests

This directory contains Go-based acceptance tests for the Hookdeck CLI. These tests verify end-to-end functionality by executing the CLI and validating outputs.

## Test Categories

Tests are divided into two categories:

### 1. Automated Tests (CI-Compatible)
These tests run automatically in CI using API keys from `hookdeck ci`. They don't require human interaction.

**Files:** All test files without build tags (e.g., `basic_test.go`, `connection_test.go`, `project_use_test.go`)

### 2. Manual Tests (Require Human Interaction)
These tests require browser-based authentication via `hookdeck login` and must be run manually by developers.

**Files:** Test files with `//go:build manual` tag (e.g., `project_use_manual_test.go`)

**Why Manual?** These tests access endpoints (like `/teams`) that require CLI authentication keys obtained through interactive browser login, which aren't available to CI service accounts.

## Setup

### Local Development

For local testing, create a `.env` file in this directory:

```bash
# test/acceptance/.env
HOOKDECK_CLI_TESTING_API_KEY=your_api_key_here
```

The `.env` file is automatically loaded when tests run. **This file is git-ignored and should never be committed.**

### CI/CD

In CI environments (GitHub Actions), set the `HOOKDECK_CLI_TESTING_API_KEY` environment variable directly in your workflow configuration or repository secrets.

## Running Tests

### Run all automated (CI) tests:
```bash
go test ./test/acceptance/... -v
```

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
go test ./test/acceptance/... -short
```

All acceptance tests are skipped when `-short` flag is used, allowing fast unit test runs.

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

1. **Skip in short mode:**
   ```go
   if testing.Short() {
       t.Skip("Skipping acceptance test in short mode")
   }
   ```

2. **Use cleanup for resources:**
   ```go
   t.Cleanup(func() {
       deleteConnection(t, cli, connID)
   })
   ```

3. **Use descriptive names:**
   ```go
   func TestConnectionWithStripeSource(t *testing.T) { ... }
   ```

4. **Log important information:**
   ```go
   t.Logf("Created connection: %s (ID: %s)", name, id)
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
Error: HOOKDECK_CLI_TESTING_API_KEY environment variable must be set
```
**Solution:** Create a `.env` file in `test/acceptance/` with your API key.

### Command Execution Failures
If commands fail to execute, ensure you're running from the project root or that the working directory is set correctly.

### Resource Cleanup
Tests use `t.Cleanup()` to ensure resources are deleted even if tests fail. If you see orphaned resources, check the cleanup logic in your test.