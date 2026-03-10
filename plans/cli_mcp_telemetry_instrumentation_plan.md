# Telemetry Instrumentation Plan: CLI & MCP Usage Tracking

## Problem

The `Hookdeck-CLI-Telemetry` header is sent on every API request but is always empty — `SetCommandContext()` and `SetDeviceName()` are never called. There is no way to distinguish CLI requests from MCP requests, and no way to correlate multiple API calls back to a single command or tool invocation.

## Current State

### Two HTTP client paths

| Client | Used by | Telemetry behavior |
|--------|---------|-------------------|
| `hookdeck.Client` (internal) | MCP tools, gateway commands | Header set **per-request** in `PerformRequest()` — can change dynamically |
| `hookdeckclient.Client` (Go SDK) | `listen` command | Header set **once at construction** via static HTTP headers — **cannot change per-request** |

This is the key constraint: the SDK client bakes headers in at creation time. Any per-invocation data (like an invocation ID) must be known before `Config.GetClient()` is called.

### Telemetry singleton

`CLITelemetry` is a process-wide singleton (`sync.Once`). For standard CLI commands (one command per process), this is fine. For MCP (long-lived process, many concurrent tool calls), a singleton is inadequate — we need per-request telemetry.

### What the API server can already see

- `User-Agent: Hookdeck/v1 hookdeck-cli/{VERSION}` — identifies CLI, not MCP
- `X-Hookdeck-Client-User-Agent` — OS/version info
- `Hookdeck-CLI-Telemetry` — always `{"command_path":"","device_name":"","generated_resource":false}`

## Proposed Design

### New telemetry header structure

```json
{
  "source": "cli",
  "environment": "interactive",
  "command_path": "hookdeck listen",
  "invocation_id": "inv_a1b2c3d4",
  "device_name": "macbook-pro",
  "generated_resource": false
}
```

For MCP:

```json
{
  "source": "mcp",
  "environment": "interactive",
  "command_path": "hookdeck_events/list",
  "invocation_id": "inv_e5f6g7h8",
  "device_name": "macbook-pro",
  "mcp_client": "claude-desktop/1.2.0"
}
```

For CLI in CI:

```json
{
  "source": "cli",
  "environment": "ci",
  "command_path": "hookdeck listen",
  "invocation_id": "inv_c9d0e1f2",
  "device_name": "github-runner-xyz",
  "generated_resource": false
}
```

Fields:
- **`source`**: `"cli"` or `"mcp"` — what initiated the request (the interface)
- **`environment`**: `"interactive"` or `"ci"` — where it's running (auto-detected via `CI` env var). These are orthogonal dimensions: source is about the interface, environment is about the runtime context. A CLI command can run in CI; an MCP tool could theoretically run in CI too
- **`command_path`**: For CLI: cobra command path (e.g. `"hookdeck gateway source list"`). For MCP: `"{tool_name}/{action}"` (e.g. `"hookdeck_events/list"`)
- **`invocation_id`**: Unique ID per command execution (CLI) or per tool call (MCP). This is what lets the server group multiple API requests into one logical event
- **`device_name`**: Machine hostname
- **`generated_resource`**: Existing field, kept for backward compat (CLI only)
- **`mcp_client`**: MCP client name/version from `initialize` params (MCP only)

### How invocation_id solves the multi-call problem

Example: `hookdeck listen` makes 4 API calls (list sources, create source, list connections, update destination). All 4 carry the same `invocation_id`. Server-side, PostHog receives 4 events, but they can be deduplicated/grouped into one "listen command executed" event using the invocation ID.

Same for MCP: `hookdeck_projects/use` makes 2 API calls (list projects, then update). Both share one invocation ID → one "tool used" event.

## Implementation

### Phase 1: Extend the telemetry struct and wire up CLI commands

**File: `pkg/hookdeck/telemetry.go`**

Replace the singleton pattern with a struct that can be instantiated per-invocation:

```go
type CLITelemetry struct {
    Source            string `json:"source"`
    Environment       string `json:"environment"`
    CommandPath       string `json:"command_path"`
    InvocationID      string `json:"invocation_id"`
    DeviceName        string `json:"device_name"`
    GeneratedResource bool   `json:"generated_resource,omitempty"`
    MCPClient         string `json:"mcp_client,omitempty"`
}
```

Keep `GetTelemetryInstance()` and the singleton for the CLI path — it works because CLI is one-command-per-process. Add:

```go
func (t *CLITelemetry) SetSource(source string) {
    t.Source = source
}

func (t *CLITelemetry) SetInvocationID(id string) {
    t.InvocationID = id
}
```

Generate invocation IDs with a simple helper:

```go
func NewInvocationID() string {
    b := make([]byte, 8)
    rand.Read(b)
    return "inv_" + hex.EncodeToString(b)
}
```

**File: `pkg/cmd/root.go`**

Add a `PersistentPreRun` to the root command that populates the telemetry singleton before any command runs:

```go
rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
    tel := hookdeck.GetTelemetryInstance()
    tel.SetSource("cli")
    tel.SetCommandContext(cmd)
    tel.SetDeviceName(Config.DeviceName)
    tel.SetInvocationID(hookdeck.NewInvocationID())
}
```

This fires before every command — `listen`, `gateway source list`, `mcp`, etc. One invocation ID per process lifetime, which is correct for CLI (one command = one process).

**Constraint check — SDK client**: `Config.GetClient()` is called *after* command execution starts (inside `RunE`), which is after `PersistentPreRun`. So the singleton will already be populated when `CreateSDKClient` reads `getTelemetryHeader()`. This works.

**Existing PersistentPreRun conflicts**: The `connection` command has its own `PersistentPreRun` (for deprecation warnings). Cobra does NOT chain these — a child's `PersistentPreRun` overrides the parent's. Fix: change the connection command to use `PersistentPreRunE` with an explicit call to the parent, or better, use cobra's `OnInitialize` (which does chain). Alternative: move the root telemetry setup into `cobra.OnInitialize` alongside `Config.InitConfig`.

Actually, the cleanest approach: use `PersistentPreRun` on root, and change the connection command's `PersistentPreRun` to call `rootCmd.PersistentPreRun(cmd, args)` first. Or consolidate into `OnInitialize` — but `OnInitialize` doesn't receive the `*cobra.Command`, so we can't call `SetCommandContext(cmd)` there. We'd need a two-phase approach:
1. `OnInitialize`: set source, device name, invocation ID
2. Each command's `PreRun` (or a wrapper): set command path

**Recommended approach**: Use a helper function and call it explicitly in commands that have their own `PersistentPreRun`:

```go
// pkg/cmd/root.go
func initTelemetry(cmd *cobra.Command) {
    tel := hookdeck.GetTelemetryInstance()
    tel.SetSource("cli")
    tel.SetEnvironment(hookdeck.DetectEnvironment())
    tel.SetCommandContext(cmd)
    tel.SetDeviceName(Config.DeviceName)
    tel.SetInvocationID(hookdeck.NewInvocationID())
}

// Root command
rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
    initTelemetry(cmd)
}

// Connection command (which has its own PersistentPreRun)
cmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
    initTelemetry(cmd)  // call the shared helper
    // ... existing deprecation warning logic
}
```

### Phase 2: MCP per-request telemetry

The MCP path can't use the process-wide singleton because:
1. The MCP server is long-lived (not one-command-per-process)
2. Tool calls happen sequentially but each is a different "invocation"
3. Each tool call needs its own `invocation_id` and `command_path`

**File: `pkg/hookdeck/client.go`**

Add a `Telemetry` field to `Client` that, when set, overrides the singleton:

```go
type Client struct {
    BaseURL    *url.URL
    APIKey     string
    ProjectID  string
    Verbose    bool
    SuppressRateLimitErrors bool

    // Per-request telemetry override. When non-nil, this is used instead of
    // the global telemetry singleton. Used by MCP tool handlers to set
    // per-invocation context.
    Telemetry *CLITelemetry

    httpClient *http.Client
}
```

In `PerformRequest`, change the telemetry header logic:

```go
if !telemetryOptedOut(os.Getenv("HOOKDECK_CLI_TELEMETRY_OPTOUT")) {
    var telemetryHdr string
    var err error
    if c.Telemetry != nil {
        b, e := json.Marshal(c.Telemetry)
        telemetryHdr, err = string(b), e
    } else {
        telemetryHdr, err = getTelemetryHeader()
    }
    if err == nil {
        req.Header.Set("Hookdeck-CLI-Telemetry", telemetryHdr)
    }
}
```

**Problem**: The MCP server shares ONE `Client` instance across all tool handlers. We can't set `client.Telemetry` per-call without races. Two options:

**Option A — Clone the client per tool call (recommended)**:

Add a method to clone a client with specific telemetry:

```go
func (c *Client) WithTelemetry(t *CLITelemetry) *Client {
    return &Client{
        BaseURL:    c.BaseURL,
        APIKey:     c.APIKey,
        ProjectID:  c.ProjectID,
        Verbose:    c.Verbose,
        SuppressRateLimitErrors: c.SuppressRateLimitErrors,
        Telemetry:  t,
        httpClient: c.httpClient, // share the underlying http.Client (connection pool)
    }
}
```

Then in MCP tool handlers, wrap the client before making API calls:

```go
// In tool handler
tel := &hookdeck.CLITelemetry{
    Source:       "mcp",
    Environment:  hookdeck.DetectEnvironment(),
    CommandPath:  "hookdeck_events/list",
    InvocationID: hookdeck.NewInvocationID(),
    DeviceName:   deviceName,
    MCPClient:    mcpClientName,
}
scopedClient := client.WithTelemetry(tel)
// use scopedClient for API calls
```

**Option B — Context-based telemetry**: Pass telemetry through `context.Context`. Cleaner Go idiom but requires threading context through all client methods. More invasive refactor.

**Recommendation**: Option A. Minimal changes, no refactoring of method signatures.

**File: `pkg/gateway/mcp/server.go` and tool handlers**

The tool dispatch in `tools.go` already has access to the tool name and action. The wrapping can happen in one central place rather than in every handler:

```go
// In the tool dispatch wrapper (tools.go or similar)
func wrapHandler(client *hookdeck.Client, toolName string, mcpClientInfo string, handler func(*hookdeck.Client, ...) ...) ... {
    return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
        action := extractAction(req)  // parse "action" from arguments
        tel := &hookdeck.CLITelemetry{
            Source:       "mcp",
            CommandPath:  toolName + "/" + action,
            InvocationID: hookdeck.NewInvocationID(),
            DeviceName:   deviceName,
            MCPClient:    mcpClientInfo,
        }
        scopedClient := client.WithTelemetry(tel)
        return handler(scopedClient, ...)
    }
}
```

**MCP client identification**: The MCP client name/version is available from `ServerSession.InitializeParams().ClientInfo`. However, the server currently uses `Server.Run()` (not `Server.Connect()`), so we don't directly hold a `ServerSession`. The server only has one session (stdio transport), so we can capture the client info during initialization:

Looking at the SDK, `ServerOptions` has an `OnSessionInitialized` callback or we can use middleware. Alternatively, we can read it from `server.Sessions()` after the first tool call. The simplest approach: store the MCP client info on the `Server` struct after the first session connects (via `Server.Sessions()` iterator), then use it in the telemetry wrapper.

### Phase 3: SDK client (listen command) telemetry

The SDK client (`hookdeckclient.Client`) sets headers once at construction via `hookdeckoption.WithHTTPHeader()`. Since `PersistentPreRun` runs before `RunE`, the telemetry singleton is populated before `CreateSDKClient` is called.

**This already works with Phase 1 changes** — `getTelemetryHeader()` in `CreateSDKClient` will return the correctly populated singleton. The `invocation_id` will be the same for all API calls from a single `listen` invocation, which is exactly what we want.

**Limitation**: The SDK client can't have per-request telemetry variation. For `listen`, this is fine — all calls are part of the same invocation. If a future SDK-client-based command needed per-call variation, we'd need to create multiple SDK client instances. Not a concern now.

### Phase 4: Server-side (PostHog)

Not in scope for this CLI PR, but documents the expected server-side changes:

1. Parse the `Hookdeck-CLI-Telemetry` header (already done — just reading new fields)
2. Use `source` to split CLI vs MCP events
3. Use `invocation_id` to deduplicate: group all API requests with the same invocation ID into one logical event
4. Use `command_path` as the event name / action property
5. Use `mcp_client` to break down MCP usage by AI agent

## File Change Summary

| File | Change | Phase | Complexity |
|------|--------|-------|-----------|
| `pkg/hookdeck/telemetry.go` | Add `Source`, `Environment`, `InvocationID` fields; `NewInvocationID()` generator; `DetectEnvironment()` for CI; update `telemetryOptedOut()` to accept config flag | 1, 5 | Small |
| `pkg/hookdeck/telemetry_test.go` | Update tests for new fields, opt-out with config flag, environment detection | 1, 5 | Small |
| `pkg/hookdeck/client.go` | Add `Telemetry` field + `TelemetryDisabled` field; `WithTelemetry()` clone method; update `PerformRequest` to use per-request telemetry override and config-based opt-out | 2, 5 | Small |
| `pkg/cmd/root.go` | Add `PersistentPreRun` with `initTelemetry()` helper | 1 | Small |
| `pkg/cmd/connection.go` | Call `initTelemetry()` in existing `PersistentPreRun` | 1 | Trivial |
| `pkg/gateway/mcp/server.go` | Capture MCP client info, store on Server struct | 2 | Small |
| `pkg/gateway/mcp/tools.go` | Add telemetry wrapping in tool dispatch (central `WithTelemetry` per tool call) | 2 | Medium |
| `pkg/hookdeck/sdkclient.go` | Add `TelemetryDisabled` field, thread through opt-out check | 5 | Small |
| `pkg/config/config.go` | Add `TelemetryDisabled` field, read from viper in `constructConfig()` | 5 | Small |

## Risks and Edge Cases

1. **Cobra PersistentPreRun chaining**: Cobra doesn't chain `PersistentPreRun` from parent to child. Any command with its own `PersistentPreRun` must explicitly call `initTelemetry()`. Currently only `connection` has one. Must audit for future additions.

2. **MCP session info timing**: `ServerSession.InitializeParams()` is available after handshake. Tool calls only happen after handshake, so this is safe. But if the server ever supports multiple sessions, we'd need per-session client info.

3. **Invocation ID uniqueness**: 8 random bytes = 16 hex chars. Collision probability is negligible for our use case (not a security-critical ID).

4. **SDK client static headers**: The `listen` command's SDK client gets one invocation ID baked in. If `listen` ran for days and we wanted to track "sessions," we'd need a different mechanism. Fine for now — we're tracking command invocations, not long-lived sessions.

5. **Backward compatibility**: The server must handle both old (empty) and new telemetry payloads. Since it's JSON with new fields, old servers will simply ignore unknown keys. New servers should treat missing `source` as `"cli"` for backward compat.

## Phase 5: Telemetry Opt-Out via Config

### Problem

Telemetry opt-out is currently only possible via the `HOOKDECK_CLI_TELEMETRY_OPTOUT` environment variable. This requires users to set it in their shell profile or per-invocation, which is fragile and inconvenient. Users who want telemetry permanently disabled (corporate policy, personal preference) need a persistent config-based option.

### Design

Add a **top-level** `telemetry` setting to `config.toml`. No per-profile override initially — telemetry is a user-level concern, not a project-level one. We can always add profile granularity later if needed.

**Precedence (highest to lowest):**
1. `HOOKDECK_CLI_TELEMETRY_OPTOUT` env var (existing, unchanged)
2. `telemetry` in `config.toml` (new)
3. Default: enabled

**Config format:**
```toml
# ~/.config/hookdeck/config.toml
telemetry = false   # disables telemetry globally
profile = "default"

[default]
  api_key = "..."
```

### Implementation

**File: `pkg/config/config.go`**

Add field to `Config` struct:
```go
type Config struct {
    // ... existing fields ...
    TelemetryDisabled bool
}
```

In `constructConfig()`, read the value:
```go
c.TelemetryDisabled = c.viper.GetBool("telemetry_disabled")
```

**File: `pkg/hookdeck/telemetry.go`**

Update `telemetryOptedOut` to accept a config-based flag in addition to the env var:

```go
func telemetryOptedOut(envVar string, configDisabled bool) bool {
    if configDisabled {
        return true
    }
    envVar = strings.ToLower(envVar)
    return envVar == "1" || envVar == "true"
}
```

**File: `pkg/hookdeck/client.go` and `pkg/hookdeck/sdkclient.go`**

Thread the config value through. The `Client` struct already receives config indirectly — the simplest approach is to add a `TelemetryDisabled bool` field to `Client` (set during construction in `Config.GetClient()`) and check it alongside the env var in `PerformRequest`.

**UX — setting the value:**

Users can edit `config.toml` directly, or we expose a command:
```bash
hookdeck config set telemetry_disabled true
hookdeck config set telemetry_disabled false
```

This depends on whether a generic `config set` command exists or is planned. If not, a simple `hookdeck telemetry off/on` subcommand is the alternative.

### CI/CD Environment Detection

**Context:** Some CLI tools (e.g., `npx @anthropic-ai/claude-code`) detect CI environments and disable telemetry by default. The question is whether hookdeck-cli should do the same.

**How CI detection works:** Check for well-known environment variables:
- `CI=true` (GitHub Actions, GitLab CI, CircleCI, Travis, most CI systems)
- `GITHUB_ACTIONS=true`
- `GITLAB_CI=true`
- `JENKINS_URL` is set
- `CODEBUILD_BUILD_ID` is set (AWS CodeBuild)
- `TF_BUILD=true` (Azure Pipelines)
- `BUILDKITE=true`

A simple `isCI()` function checking `os.Getenv("CI")` covers ~90% of cases.

**Arguments for disabling in CI:**
- CI runs can generate massive telemetry volume (every build, every PR, every retry)
- CI telemetry is "noisy" — it represents automation, not human decision-making
- CI environments often have strict data-exfiltration policies
- Users don't expect background analytics from build tools

**Arguments against disabling in CI:**
- Knowing that hookdeck-cli is used in CI/CD pipelines is genuinely useful product insight
- CI usage patterns differ from interactive use — that's *interesting*, not noise
- A GitHub Action (`hookdeck/hookdeck-cli-action`) is a deliberate integration — the user chose to put it in CI
- Silently disabling telemetry means you lose visibility into a growing use case

**Recommended approach — separate dimensions:**

`source` and `environment` are orthogonal:
- **`source`**: what interface initiated the request — `"cli"` (command) or `"mcp"` (tool call)
- **`environment`**: where it's running — `"interactive"` or `"ci"` (auto-detected)

This means you can cross-tabulate: "how many MCP tool calls come from CI?" is a valid query. Collapsing both into `source` would lose that.

1. **Detect CI, tag as `environment: "ci"`.** Telemetry stays enabled. The `source` field remains `"cli"` or `"mcp"` as appropriate. Server-side, PostHog can filter or segment on either dimension independently.

2. **Respect explicit opt-out.** If `HOOKDECK_CLI_TELEMETRY_OPTOUT=1` or `telemetry_disabled = true` is set, disable completely — same as interactive mode.

3. **Do NOT auto-disable.** The user made a deliberate choice to run hookdeck-cli in CI. Unlike, say, a package manager that runs implicitly, hookdeck-cli is an explicit integration. Disabling telemetry by default in CI would be overly conservative.

4. **Document it.** If `CI=true` is detected, the telemetry header includes `"environment": "ci"`. Document this behavior so CI-conscious users know what's sent.

**Implementation:**
```go
func DetectEnvironment() string {
    if os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true" {
        return "ci"
    }
    return "interactive"
}
```

This integrates cleanly with Phase 1's telemetry struct as a new `environment` field. Server-side, PostHog dashboards get two clean dimensions to slice by.

## Key Source Files

These are the files an implementer needs to read before starting:

| File | What it contains |
|------|-----------------|
| `pkg/hookdeck/telemetry.go` | `CLITelemetry` struct, singleton `GetTelemetryInstance()`, `getTelemetryHeader()`, `telemetryOptedOut()` — all telemetry logic lives here |
| `pkg/hookdeck/client.go` | Internal HTTP client, `PerformRequest()` where telemetry header is set per-request (line ~107) |
| `pkg/hookdeck/sdkclient.go` | `CreateSDKClient()` where telemetry header is baked in at construction (line ~39) |
| `pkg/config/config.go` | `Config` struct, `InitConfig()`, `constructConfig()` with viper precedence chain, `getConfigPath()` for local/global config resolution |
| `pkg/config/profile.go` | `Profile` struct, save/load/remove profile, field getter helpers |
| `pkg/cmd/root.go` | Root cobra command, `cobra.OnInitialize` wiring, where `PersistentPreRun` should be added |
| `pkg/cmd/connection.go` | Has its own `PersistentPreRun` (deprecation warning) — must be updated to call shared `initTelemetry()` |
| `pkg/gateway/mcp/server.go` | MCP server setup, `Server.Run()`, session management |
| `pkg/gateway/mcp/tools.go` | MCP tool dispatch — where per-tool-call telemetry wrapping goes |

## Testing Strategy

1. **Unit tests for telemetry struct**: Verify JSON serialization includes new fields
2. **Unit tests for `WithTelemetry`**: Verify cloned client uses override telemetry
3. **Integration test**: Wire up an MCP test with a mock API server, verify the `Hookdeck-CLI-Telemetry` header on requests contains correct `source`, `command_path`, and `invocation_id`
4. **Manual test**: Run `hookdeck listen` against a local proxy, inspect the telemetry header on outgoing requests
