# Hosted MCP + CLI MCP: Architecture & Implementation Plan

## Context

Hookdeck wants to offer a **hosted (remote) MCP server** alongside the embedded CLI MCP (`hookdeck gateway mcp`, beta, stdio-only), with the same surface area across both. Open questions: turn the CLI into an MCP proxy? Deprecate the embedded server? Or offer both from a shared package? And how should MCP client↔server auth work — Bearer token or OAuth?

What exists today: `pkg/gateway/mcp/` is built on the official `github.com/modelcontextprotocol/go-sdk` v1.6.1, whose `Server.Run(ctx, transport)` is already transport-agnostic (`server.go:141`) — the SDK ships `NewStreamableHTTPHandler`, so a hosted HTTP transport is nearly free from the same code. 12 tools (no resources/prompts), all thin calls to the Hookdeck API via `pkg/hookdeck.Client` — no CLI subprocess exec, no tunnel/TUI coupling. The existing roadmap (`plans/hookdeck_mcp_buildout_plan_v2.md` ~line 412) already names `hookdeck gateway mcp serve` + hosted MCP as Phase 2. Auth today: long-lived CLI key from a browser poll flow, stored in a TOML profile, sent as HTTP **Basic** (`client.go:176`); project scoping via `X-Team-ID`/`X-Project-ID`; no OAuth anywhere.

## Recommendation summary

1. **Offer both, from one shared Go package** — embedded stdio (`hookdeck gateway mcp`) and hosted streamable HTTP (`hookdeck gateway mcp serve`, deployed at e.g. `mcp.hookdeck.com/mcp`). **Do not** make the CLI a proxy; **do not** deprecate the embedded server.
   - *Proxy rejected*: adds a network hop + hosted-availability dependency for local use, breaks offline, and kills local-only capabilities (`hookdeck_login` browser flow persisting to the TOML profile; roadmap Phase 3 local-listen/CLI-exec tools must run on the user's machine). Saves nothing — every tool is a pure API call either way.
   - *Deprecation rejected*: embedded is the zero-config path (inherits `hookdeck login` credentials) and the only home for local tools. Fleet drift is bounded: tools are thin veneers over the versioned API (`APIPathPrefix = "/2025-07-01"`), and one shared `tools.go` means both surfaces update in the same commit.
2. **Hosted lives in this repo** as a `serve` subcommand + `Dockerfile`, deployed from `main` on merge (decoupled from tagged CLI releases). Public repo is fine — no secrets, and self-hostable is a trust asset. A separate `cmd/hookdeck-mcp-server/main.go` is a trivial later addition if image size ever matters; don't start there.
3. **Auth: Bearer token first, OAuth 2.1 later.** Phase A static `Authorization: Bearer <project API key>` covers every dev-tool client (Claude Code, Cursor, VS Code, Windsurf, Cline) with near-zero new infra. Phase B OAuth 2.1 (auth code + PKCE + DCR + RFC 9728 metadata) is required only for the Claude.ai/ChatGPT connector directories and is ~90% dashboard/API-platform work, not this repo. The middleware is designed so Phase B slots in without restructuring; static Bearer stays supported forever.
4. **Hosted surface = CLI surface minus `hookdeck_login`** (auth comes from the transport). Everything else — including `hookdeck_projects` — registers in both, from shared definitions.

## Shared-package refactor design

Three things in `pkg/gateway/mcp` assume one process = one user:
- `NewServer(client, cfg)` closes all handlers over a single `*hookdeck.Client` (`server.go:36`, `tools.go:14`)
- `projects use` mutates `client.ProjectID/ProjectOrg/ProjectName` (`tool_projects.go:99`)
- `wrapWithTelemetry` mutates `s.client.Telemetry` per call — safe only because stdio is sequential (`server.go:90–115`)

**Design: session-scoped Server + Client.** Handlers stay closed over a client (none of the 12 `tool_*.go` files change); the client becomes per-session:

```go
type Profile int
const (
    ProfileCLI    Profile = iota // stdio: full toolset incl. hookdeck_login
    ProfileHosted                // HTTP: everything except hookdeck_login
)
type Options struct {
    Client          *hookdeck.Client // hosted: built per session from Bearer token
    Config          *config.Config   // required for ProfileCLI; nil for hosted
    Profile         Profile
    TelemetrySource string           // "mcp" | "mcp-hosted"
    DeviceName      string           // os.Hostname() for CLI; "" for hosted
}
func NewServer(opts Options) *Server
```

The go-sdk's `NewStreamableHTTPHandler(getServer func(*http.Request) *mcp.Server, opts)` calls `getServer` at session init and routes follow-ups via `Mcp-Session-Id`. Hosted flow: auth middleware validates Bearer on **every** request (401 + `WWW-Authenticate` otherwise; 401 on mid-session credential change) → `getServer` builds a fresh `hookdeck.Client{APIKey: token}` + `NewServer(Options{Profile: ProfileHosted, ...})` per session. `projects use` mutating that client is then correct per-session behavior, identical to stdio semantics.

Key mechanics:
- **Capability split**: refactor `toolDefs` (`tools.go:14`) to a named `toolDef{tool, handler, profiles}` type; gate the separately-registered `hookdeck_login` block (`server.go:57–66`) on `ProfileCLI`. Add per-profile description overrides — several descriptions, `tool_help.go` topics, `requireAuth` (`auth.go`), and `listProjectsFailureMessage` (`tool_projects_errors.go`) currently instruct agents to call `hookdeck_login`, which won't exist in hosted. Under a project-scoped key, the hosted `projects list/use` error must say "your token is project-scoped; all tools already operate on that project" so agents don't loop.
- **Concurrency**: HTTP doesn't guarantee sequential calls within a session. Add a per-`Server` `sync.Mutex` held in `wrapWithTelemetry` for the call duration — serializes within one session only (each session has its own Server), preserving today's semantics for the telemetry write and `projects use`. Threading telemetry through `context` cascades into non-ctx client methods; not worth it now.
- **Statefulness consequence**: keeping `projects use` in hosted makes sessions stateful → the deployment needs session affinity (sticky routing on `Mcp-Session-Id`). Acceptable at launch scale; dropping `use` from the hosted profile later would restore stateless horizontal scaling if needed.
- **Telemetry**: source from `Options` (`"mcp-hosted"`), empty device name for hosted (pod hostnames are noise); `mcpClientInfo` works unchanged over HTTP. Consider adding build SHA since hosted runs `main`.

## Auth design

**Phase A — static Bearer (ship first).** Middleware extracts the token; per-session client uses it as `APIKey`. Don't pre-validate per request — let the first tool call surface 401 via the existing `TranslateAPIError` path (optionally one cheap validation at session init). Document **project API keys** (dashboard) as the supported credential; user-scoped CLI keys incidentally work and make `hookdeck_projects` fully functional (advanced usage). ⚠️ **Gating verification, do first**: `client.go:176` sends the key as Basic (username). Confirm the API accepts project API keys that way; if it wants `Bearer`, add a `Client.AuthScheme` field (~5 lines, default `"basic"`).

**Phase B — OAuth 2.1 (platform-led, later).** The MCP auth spec (2025-06-18) mandates OAuth 2.1 + RFC 9728 Protected Resource Metadata for HTTP; Claude.ai/ChatGPT connector directories require it with Dynamic Client Registration. Platform work (dashboard/API repos): authorization server (or an IdP product — Auth0/WorkOS/Stytch have MCP offerings worth evaluating) with `/.well-known/oauth-authorization-server`, `/authorize` (PKCE S256), `/token` (+ refresh), `/register` (DCR); a consent screen reusing dashboard session auth **with project selection**; short-lived access tokens mapping to project-scoped credentials. This repo's share is small: serve `/.well-known/oauth-protected-resource`, extend middleware to accept OAuth tokens *in addition to* raw keys, emit the spec-compliant challenge. If connector-directory distribution is the point of hosting, co-plan Phase B with the platform team now rather than treating it as optional-someday.

**Security notes**: long-lived static keys are revocable in the dashboard — same trust model as the REST API; fine for developers. `pause`/`unpause` means not read-only → add a `--read-only` flag on `serve` (cheap via the profile mechanism); real per-token scoping arrives with OAuth scopes. A project-scoped key is itself the tenancy boundary (no confused-deputy project switching). Edge hygiene: HTTPS at the LB, verify the SDK handler's Origin/DNS-rebinding protection, per-token rate limits, never log tokens/bodies.

## Implementation phases

**Phase 1 — multi-tenancy refactor (no behavior change), all in `pkg/gateway/mcp/`:**
1. `server.go`: `Options`/`Profile`, `NewServer(opts)`, per-server mutex in `wrapWithTelemetry`, parameterized telemetry source/device name, profile-gate `hookdeck_login` registration.
2. `tools.go`: named `toolDef` with `profiles` + per-profile description overrides.
3. `tool_projects_errors.go`, `auth.go`, `tool_help.go`: profile-aware messaging (no `hookdeck_login` references in hosted output).
4. `pkg/cmd/mcp.go:53–55`: update the single call site to `Options{..., Profile: ProfileCLI}`.
5. Tests (follow `server_test.go` patterns — `NewInMemoryTransports` + `mockAPI` httptest): profile param on the test connect helper; hosted profile lists 11 tools (no login); hosted descriptions contain no `hookdeck_login` mention. Existing tests unchanged.

**Phase 2 — HTTP transport + `serve` (auth Phase A):**
1. Verify Basic-vs-Bearer project-key acceptance; add `Client.AuthScheme` to `pkg/hookdeck/client.go` if needed.
2. New `pkg/gateway/mcp/http.go`: `NewHTTPHandler(opts) http.Handler` — Bearer middleware on every request → `mcpsdk.NewStreamableHTTPHandler` with per-session Server+Client → plus `/healthz`; 401s carry `WWW-Authenticate`.
3. New `pkg/cmd/mcp_serve.go`: cobra subcommand under the existing `mcp` command (`AddCommand` — no change needed to the parent's `validators.NoArgs`; cobra resolves subcommands before arg validation). Flags: `--addr`, `--api-base`, `--read-only`.
4. New `Dockerfile` (`ENTRYPOINT ["hookdeck","gateway","mcp","serve"]`); internal deploy from `main` with sticky routing on `Mcp-Session-Id`; audit LB idle timeouts for SSE streams.
5. New `http_test.go`: `httptest.NewServer` around the handler with the repo's `mockAPI` — no/bad Bearer → 401; tools/list surface; end-to-end call asserting the mock received the right key; two concurrent sessions with different tokens don't cross-contaminate; `projects use` isolation between sessions.
6. Docs: README + hookdeck.com for both connection modes; show env-var interpolation for keys in client configs.

**Phase 3 — OAuth 2.1**: platform-led; this repo adds the metadata endpoint + token acceptance in middleware; then connector-directory submissions.

**Phase 4 — divergence by design**: roadmap Phase 3 local tools land `ProfileCLI`-only; `search_docs`/MCP resources land shared.

## Verification

- Phase 1: `go test ./pkg/gateway/mcp/...`; manual stdio smoke test (`echo '<initialize JSON-RPC>' | hookdeck gateway mcp`) confirming unchanged behavior + tool list.
- Phase 2: run `hookdeck gateway mcp serve --addr :8080` locally; connect a real client (e.g. Claude Code: `claude mcp add --transport http hookdeck http://localhost:8080/mcp --header "Authorization: Bearer $KEY"`); exercise list/get tools against a real project; confirm 401 without a key; run `http_test.go` concurrency cases.

## Risks / open questions

- **Basic-auth acceptance of project API keys** — the single gating verification; do first (needs a platform-side check, not answerable from this repo).
- **go-sdk v1.6.1 handler specifics** (exact `StreamableHTTPHandler` options; `InitializeParams` availability per HTTP request) — verify at Phase 2 start.
- **Session affinity** required while hosted keeps `projects use`.
- **SSE through Hookdeck's edge** — LB timeout audit.
- Read-only scoping is per-deployment (flag) until OAuth brings per-token scopes.

## Critical files

`pkg/gateway/mcp/server.go`, `pkg/gateway/mcp/tools.go`, `pkg/gateway/mcp/tool_projects_errors.go`, `pkg/gateway/mcp/auth.go`, `pkg/gateway/mcp/tool_help.go`, `pkg/cmd/mcp.go`, `pkg/hookdeck/client.go`; new: `pkg/gateway/mcp/http.go`, `pkg/gateway/mcp/http_test.go`, `pkg/cmd/mcp_serve.go`, `Dockerfile`.
