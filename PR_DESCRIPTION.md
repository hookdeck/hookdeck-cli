# PR Title

`v2.0.0: MCP server, telemetry instrumentation, issues CLI, metrics consolidation, and Go SDK removal`

# PR Description

Use the content below to update the PR description on GitHub.

---

## Summary

This is the **v2.0.0** release branch. It combines several feature branches into a single release that includes breaking changes, new functionality, and internal improvements.

### What's New

- **MCP Server** (`hookdeck gateway mcp`) — A Model Context Protocol server exposing Hookdeck's API as LLM-callable tools, compatible with Claude Desktop, Cursor, and other MCP clients.
- **Issues CLI** (`hookdeck gateway issue`) — Full CRUD commands for managing Hookdeck issues (delivery failures, transformation errors, backpressure).
- **Telemetry Instrumentation** — Per-request telemetry headers tracking CLI/MCP source, environment, command path, and invocation ID, with opt-out support.
- **`hookdeck telemetry` command** — New command to enable/disable telemetry (`hookdeck telemetry disable` / `hookdeck telemetry enable`).

### Breaking Changes

- **Global `--config` flag renamed to `--hookdeck-config`** — The global flag for specifying the CLI config file path has been renamed to avoid conflicts with `--config` flags on source/destination commands that accept JSON objects. Users must update scripts using `hookdeck --config /path` to `hookdeck --hookdeck-config /path`.
- **Metrics subcommands consolidated** — Seven metrics subcommands have been reduced to four resource-aligned commands (`events`, `requests`, `attempts`, `transformations`). The removed commands (`pending`, `queue-depth`, `events-by-issue`) are now served by `hookdeck gateway metrics events` with appropriate `--measures` and flag combinations. See migration table below.
- **Deprecated Hookdeck Go SDK removed** — The vendored `hookdeck-go-sdk` dependency (pinned to API version `2024-03-01`) has been removed. The `listen` command and all internal API calls now use the direct HTTP client against API version `2025-07-01`. This changes the underlying API version for `hookdeck listen` operations.
- **`pkg/config/sdkclient.go` and `pkg/hookdeck/sdkclient.go` removed** — The SDK client wrapper files have been deleted entirely.

### Metrics Migration Guide

| Old Command | New Command |
|---|---|
| `hookdeck gateway metrics pending --measures count --granularity 1h` | `hookdeck gateway metrics events --measures pending --granularity 1h` |
| `hookdeck gateway metrics queue-depth --measures max_depth,max_age` | `hookdeck gateway metrics events --measures max_depth,max_age` |
| `hookdeck gateway metrics events-by-issue <issue-id> --measures count` | `hookdeck gateway metrics events --measures count --issue-id <issue-id>` |

---

## Detailed Changes

### MCP Server (`hookdeck gateway mcp`)

A stdio JSON-RPC MCP server that exposes Hookdeck's API for use by AI agents and LLM-powered tools.

**Tools (11 total):**

| Tool | Description |
|------|-------------|
| `hookdeck_help` | Get tool overview or detailed help for a specific tool |
| `hookdeck_sources` | Inspect inbound HTTP endpoints (list, get) |
| `hookdeck_destinations` | Inspect delivery targets — HTTP, CLI, MOCK (list, get) |
| `hookdeck_connections` | Inspect and control routes linking sources to destinations (list, get, pause, unpause) |
| `hookdeck_requests` | Query raw inbound HTTP requests before routing (list, get, raw_body, events, ignored_events) |
| `hookdeck_events` | Query processed events routed through connections (list, get, raw_body) |
| `hookdeck_attempts` | Query delivery attempts and retry history (list, get) |
| `hookdeck_transformations` | Inspect JavaScript payload transformations (list, get) |
| `hookdeck_issues` | Inspect aggregated failure signals (list, get) |
| `hookdeck_metrics` | Query aggregate metrics with time ranges and grouping dimensions (events, requests, attempts, transformations) |
| `hookdeck_projects` | List available projects or switch active project (list, use) |
| `hookdeck_login` | Browser-based device authentication (available when unauthenticated) |

**Key features:**
- Stdio transport compatible with Claude Desktop, Cursor, and other MCP clients
- Structured error translation mapping HTTP status codes to MCP error codes
- In-band browser-based authentication — if unauthenticated, only `hookdeck_login` is available; after auth, all resource tools become available without reconnection
- Comprehensive test coverage (94+ tests in `server_test.go` and `telemetry_test.go`)

**Configuration example (Cursor / Claude Desktop):**
```json
{
  "mcpServers": {
    "hookdeck": {
      "command": "hookdeck",
      "args": ["gateway", "mcp"]
    }
  }
}
```

### Issues CLI (`hookdeck gateway issue`)

New subcommand for managing Hookdeck issues with full CRUD support.

| Command | Description |
|---------|-------------|
| `issue list` | List and filter issues by type, status, trigger ID with pagination |
| `issue get <id>` | Get detailed issue information |
| `issue count` | Count issues with optional filters |
| `issue update <id> --status <status>` | Update issue status (OPENED, IGNORED, ACKNOWLEDGED, RESOLVED) |
| `issue dismiss <id>` | Dismiss an issue (with `--force` to skip confirmation) |

All commands support `--output json` for machine-readable output.

### Telemetry Instrumentation

Per-request telemetry headers (`X-Hookdeck-CLI-Telemetry`) are now sent with API requests, tracking:

| Dimension | Description | Example |
|-----------|-------------|---------|
| `source` | Request origin | `"cli"` or `"mcp"` |
| `environment` | Runtime environment | `"interactive"` or `"ci"` |
| `command_path` | Command or tool being executed | `"hookdeck listen"`, `"hookdeck_sources/list"` |
| `invocation_id` | Unique per-invocation identifier | `"inv_a1b2c3d4e5f6g7h8"` |
| `device_name` | Hostname | `"my-machine"` |
| `mcp_client` | MCP client identifier (MCP only) | `"cursor/0.42"` |

**Opt-out:** Users can disable telemetry via:
- `hookdeck telemetry disable` command
- `HOOKDECK_CLI_TELEMETRY_OPTOUT=1` environment variable

### Go SDK Removal

The deprecated `github.com/hookdeck/hookdeck-go-sdk v0.7.0` dependency has been removed entirely. All API calls now go through the direct HTTP client (`pkg/hookdeck/client.go`), using API version `2025-07-01` instead of the SDK's pinned `2024-03-01`.

**Files removed:**
- `pkg/config/sdkclient.go`
- `pkg/hookdeck/sdkclient.go`

**Files updated:**
- `pkg/listen/listen.go` — Now uses `config.GetAPIClient()` instead of SDK client
- `pkg/listen/source.go` — Direct API calls for source operations
- `pkg/listen/connection.go` — Direct API calls for connection operations
- `pkg/listen/proxy/` — Updated to use `hookdeck.*` types instead of `hookdecksdk.*`
- `pkg/listen/tui/` — Updated type references

### Other Changes

- **`REFERENCE.md` updated** — Reflects new commands, renamed flags, and removed metrics subcommands
- **`REFERENCE.template.md` removed** — Template file for reference generation deleted
- **Source/destination `--config` flag** — New `--config` flag on source and destination create/update/upsert commands for passing JSON config objects directly
- **Acceptance test infrastructure** — New parallel test runner (`test/acceptance/run_parallel.sh`), expanded test helpers, and comprehensive test coverage for issues, metrics, and MCP
- **CI workflow updates** — Acceptance test workflow updated for new test structure

---

## Stats

- **100 files changed**
- **~8,900 additions**, **~610 deletions**
- **45 commits**

## Test Plan

- [ ] Acceptance tests pass for issues CLI (`test/acceptance/issue_test.go`)
- [ ] Acceptance tests pass for metrics consolidation (`test/acceptance/metrics_test.go`)
- [ ] MCP server unit tests pass (`pkg/gateway/mcp/server_test.go` — 94+ tests)
- [ ] MCP telemetry tests pass (`pkg/gateway/mcp/telemetry_test.go`)
- [ ] CLI telemetry tests pass (`pkg/cmd/telemetry_test.go`, `pkg/hookdeck/telemetry_test.go`)
- [ ] `hookdeck listen` works correctly with new direct API client
- [ ] `hookdeck gateway mcp` starts and responds to tool calls
- [ ] Telemetry opt-out works via command and environment variable
- [ ] `--hookdeck-config` flag works (and `--config` no longer conflicts)
- [ ] Verify 2.0.0-beta.0 version in build artifacts
