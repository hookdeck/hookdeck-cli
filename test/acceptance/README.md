# Hookdeck CLI Acceptance Tests

This directory contains Go-based acceptance tests for the Hookdeck CLI. These tests verify end-to-end functionality by executing the CLI and validating outputs.

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

### Run all acceptance tests:
```bash
go test ./test/acceptance/... -v
```

### Run specific test:
```bash
go test ./test/acceptance/... -v -run TestCLIBasics
```

### Skip acceptance tests (short mode):
```bash
go test ./test/acceptance/... -short
```

All acceptance tests are skipped when `-short` flag is used, allowing fast unit test runs.

## Test Structure

### Files

- **`helpers.go`** - Test infrastructure and utilities
  - `CLIRunner` - Executes CLI commands via `go run main.go`
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