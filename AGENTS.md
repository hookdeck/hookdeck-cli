# AGENTS Guidelines for Hookdeck CLI

This repository contains the Hookdeck CLI, a Go-based command-line tool for managing webhook infrastructure. When working with this codebase, please follow these guidelines to maintain consistency and ensure proper functionality.

## 1. Project Structure & Navigation

### Core Directories
- `pkg/cmd/` - All CLI commands (Cobra-based)
- `pkg/hookdeck/` - API client and models
- `pkg/config/` - Configuration management
- `pkg/listen/` - Local webhook forwarding functionality
- `cmd/hookdeck/` - Main entry point
- `REFERENCE.md` - Complete CLI documentation and examples

### Key Files
- `https://api.hookdeck.com/2025-07-01/openapi` - API specification (source of truth for all API interactions)
- `pkg/cmd/sources/` - Fetches and caches the OpenAPI spec for source type enum and auth rules; use for validation and help in source and connection management
- `pkg/cmd/helptext.go` - Shared Short/Long help for resource commands (sources, connections); use when adding or editing command help to avoid duplication
- `.plans/` - Implementation plans and architectural decisions
- `AGENTS.md` - This file (guidelines for AI agents)

## 2. OpenAPI to CLI Conversion Standards

When adding new CLI commands that interact with the Hookdeck API, follow these conversion patterns:

### Parameter Mapping Rules
```bash
# Nested JSON objects → Flat CLI flags
API: { "configs": { "strategy": "final_attempt" } }
CLI: --strategy final_attempt

# Arrays → Comma-separated values
API: { "connections": ["conn_1", "conn_2"] }
CLI: --connections "conn_1,conn_2"

# Boolean presence → Presence flags
API: { "channels": { "email": {} } }
CLI: --email

# Complex objects with values → Value flags
API: { "channels": { "slack": { "channel_name": "#alerts" } } }
CLI: --slack-channel "#alerts"
```

### Flag Naming Conventions
- **Resource identifiers**: Always `--name` for human-readable names
- **Type parameters**: 
  - **Individual resource commands**: Use `--type` (clear context)
    - Sources: `hookdeck source create --type STRIPE`
    - Destinations: `hookdeck destination create --type HTTP`
    - Issue Triggers: `hookdeck issue-trigger create --type delivery`
  - **Connection creation**: Use prefixed flags to avoid ambiguity when creating inline resources
    - `--source-type STRIPE` when creating source inline
    - `--destination-type HTTP` when creating destination inline
    - This prevents confusion between source and destination types in single command
- **Authentication**: Standard patterns (`--api-key`, `--webhook-secret`, `--basic-auth`)
  - **Connection creation**: Use prefixed authentication to avoid collisions
    - `--source-webhook-secret` for source authentication
    - `--destination-api-key` for destination authentication
- **Collections**: Use comma-separated values (`--connections "a,b,c"`)
- **Booleans**: Use presence flags (`--email`, `--pagerduty`, `--force`)

### Ordered Array Configurations

For API arrays where **order matters** (e.g., rules, processing steps, middleware):

**Pattern:** Use flag position to determine array order
```bash
# Flag naming: --<category>-<type>-<property>
API: { "rules": [{"type": "retry", ...}, {"type": "filter", ...}] }
CLI: --rule-retry-strategy exponential --rule-filter-body '{...}'

# Order determined by first flag of each type
--rule-filter-body '{...}' \      # Filter is first (index 0)
  --rule-transform-name "tx1" \   # Transform is second (index 1)
  --rule-filter-headers '{...}'   # Modifies first filter rule
```

**Implementation Guidelines:**
- First occurrence of `--<category>-<type>-*` flag establishes that item's position
- Subsequent flags for same type modify the existing item (don't create new one)
- Only one item of each type allowed (per API constraints)
- Provide JSON fallback for complex scenarios: `--<category>` or `--<category>-file`

**Example: Connection Rules (5 rule types)**
```bash
# Retry → Filter → Transform execution order
hookdeck connection create \
  --rule-retry-strategy exponential --rule-retry-count 3 \
  --rule-filter-body '{"event_type":"payment"}' \
  --rule-transform-name "my-transform"

# JSON fallback for complex configurations
hookdeck connection create --rules-file rules.json
```

**Validation:**
- If any `--rule-*` flag is used, corresponding rule object is constructed
- Type-specific required fields validated (e.g., `--rule-retry-strategy` required if any `--rule-retry-*` flag present)
- JSON fallback takes precedence and ignores all individual flags

### Command Structure Standards
```bash
# Standard CRUD pattern
hookdeck <resource> <action> [resource-id] [flags]

# Examples

# Individual resource creation (clear context)
hookdeck source create --type STRIPE --webhook-secret abc123
hookdeck destination create --type HTTP --url https://api.example.com

# Connection creation with inline resources (requires prefixed flags)
hookdeck connection create \
  --source-type STRIPE --source-name "stripe-prod" \
  --source-webhook-secret "whsec_abc123" \
  --destination-type HTTP --destination-name "my-api" \
  --destination-url "https://api.example.com/webhooks"
```

### Resource command naming and plural alias
For every **resource command group** (a top-level or gateway subcommand that manages a single resource type), use the **singular** as the primary `Use` and **always add the plural as an alias**. Many users type the plural (e.g. `projects`, `connections`, `sources`); supporting both keeps the CLI discoverable and consistent.

- **Primary:** singular (`source`, `connection`, `project`)
- **Alias:** plural (`sources`, `connections`, `projects`)

Example in Cobra:
```go
Use:     "source",
Aliases: []string{"sources"},
```

When adding a new resource command group (e.g. destination, transformation), add the plural alias at the same time. Existing groups: `connection`/`connections`, `project`/`projects`, `source`/`sources`.

## 3. Conditional Validation Implementation

When `--type` parameters control other valid parameters, implement progressive validation:

### Type-Driven Validation Pattern
```go
func validateResourceFlags(flags map[string]interface{}) error {
    // Handle different validation scenarios based on command context
    
    // Individual resource creation (use --type)
    if resourceType, ok := flags["type"].(string); ok {
        return validateSingleResourceType(resourceType, flags)
    }
    
    // Connection creation with inline resources (use prefixed flags)
    if sourceType, ok := flags["source_type"].(string); ok {
        if err := validateSourceType(sourceType, flags); err != nil {
            return err
        }
    }
    if destType, ok := flags["destination_type"].(string); ok {
        if err := validateDestinationType(destType, flags); err != nil {
            return err
        }
    }
    
    return nil
}

func validateTypeA(flags map[string]interface{}) error {
    // Type-specific required/forbidden parameter validation
    if flags["required_param"] == nil {
        return errors.New("--required-param is required for TYPE_A")
    }
    if flags["forbidden_param"] != nil {
        return errors.New("--forbidden-param is not supported for TYPE_A")
    }
    return nil
}
```

### Validation Philosophy
- **Prefer API feedback.** Let the API return errors for business rules and schema (invalid type, missing auth, bad payload). Avoid duplicating API validation client-side unless it clearly improves UX or you can use the cached OpenAPI spec.
- **Client-side validation is for:** (1) clear UX wins (e.g. Cobra required flags, "no updates specified" when update is run with no flags), and (2) validation driven by the **cached OpenAPI spec** (e.g. source/connection type enum and required auth from `FetchSourceTypes()`). When the cache is used, validate type and type-specific required flags; if the spec cannot be fetched, warn and let the API validate.
- **Do not** add ad-hoc client-side schema validation that duplicates or drifts from the API. When in doubt, send the request and surface the API error.

### Create vs update request shapes
- **Check the OpenAPI spec** for required request-body fields per operation: create/upsert often require an identifier (e.g. `name`); update (PUT by id) often has **no** required body fields.
- When semantics differ, use **separate request types** (e.g. `SourceCreateRequest` vs `SourceUpdateRequest`): create/upsert structs send required fields; update structs use `omitempty` on all fields so only changed fields are sent. Never send empty strings for "unchanged" fields on update.
- For update commands, if the user supplies no update flags, **fail in the CLI** with a clear message (e.g. "no updates specified (set at least one of …)") instead of sending an empty body.

### Using the cached OpenAPI spec
- Source type enum and auth rules are available via **`pkg/cmd/sources.FetchSourceTypes()`** (fetches from the API OpenAPI URL, caches under temp with TTL). Use it for **source management** (e.g. `source create`, `source upsert`) and **connection management** (e.g. `connection create` inline source) to validate `--type` and type-specific required auth flags.
- If `FetchSourceTypes()` fails (network, parse), **warn and continue**—do not block the command; let the API validate. If the given type is not in the cached enum, let the API validate.
- Prefer this over hardcoding type lists or required-auth rules so the CLI stays aligned with the API.

### Validation Layers (in order)
1. **Flag parsing validation** - Ensure flag values are correctly typed
2. **Type-specific validation** - Validate based on `--type` parameter (use cached spec when available)
3. **Cross-parameter validation** - Check relationships between parameters
4. **API** - Final authority; surface API errors to the user

### Help System Integration
Provide dynamic help text based on selected type:
```go
func getTypeSpecificHelp(command, selectedType string) string {
    // Return contextual help for the specific type
    // Show only relevant flags and their requirements
}
```

## 4. Code Organization Patterns

### Command File Structure
Each resource follows this pattern:
```
pkg/cmd/
├── resource.go              # Main command group
├── resource_list.go         # List resources with filtering
├── resource_get.go          # Get single resource details
├── resource_create.go       # Create new resources (with type validation)
├── resource_update.go       # Update existing resources
├── resource_delete.go       # Delete resources
└── resource_enable.go       # Enable/disable operations (if applicable)
```

### API Client Pattern
```
pkg/hookdeck/
├── client.go               # Base HTTP client
├── resources.go            # Resource-specific API methods
└── models.go              # API response models
```

## 5. Development Workflow

### Agent command execution (Cursor / AI)

**Strong preference:** When a command needs more than the default sandbox (network, full filesystem, real module cache, TLS to external services, `gpg-agent`, etc.), **prompt the user for permission to run that command outside the sandbox** (e.g. approve **`required_permissions: ["all"]`**, **network**, or the product’s “run outside sandbox” control)—and **execute it yourself** after approval.

- **Do not** default to “please run this in your terminal” or long copy-paste-only instructions when the environment can elevate the **same** command.
- **Do** use copy-paste or “run locally” guidance only when elevation is **refused**, **unavailable**, or the user explicitly prefers it.

This applies to **`go test`**, **`go mod download`**, **`git`**, **`gh`**, builds, and acceptance tests—not only network calls.

### Building and Testing
```bash
# Build the CLI
go build -o hookdeck cmd/hookdeck/main.go

# Run all unit tests (default packages; run from repo root)
go test ./...

# Run specific package tests
go test ./pkg/cmd/

# Run with race detection
go test -race ./...
```

### Running `go test` after code changes (required for agents)

**Whenever you change Go code in this repo, run tests from the repository root** and treat the run as failed if compilation or any test fails.

**Recommended command in Cursor (module cache + full permissions):**

```bash
cd /path/to/hookdeck-cli   # repo root containing go.mod
GOMODCACHE="$(go env GOPATH)/pkg/mod" go test ./...
```

**Prompt the user** to approve running that command **outside the sandbox** with **`required_permissions: ["all"]`** and **`["network"]`** when the module cache may need to fetch (first run or new dependencies). Plain `go test ./...` is fine on a normal developer machine where `GOMODCACHE` is unset or already populated.

**Cursor / agent environments — prompt for out-of-sandbox, fix module cache, do not skip tests**

1. **Prompt the user** for permission, then run `go test ./...` with **`required_permissions: ["all"]`** (disable sandbox for that command). Do **not** use the default sandbox for `go test` here.

2. **If you still see `no required module provides package github.com/...`**, the shell often has **`GOMODCACHE`** set to an **empty** Cursor temp directory (even with `all`). That is **not** a broken `go.mod`. **Re-run tests with an explicit cache path** (pick one):

```bash
GOMODCACHE="$(go env GOPATH)/pkg/mod" go mod download
GOMODCACHE="$(go env GOPATH)/pkg/mod" go test ./...
```

Or unset the variable so Go uses its default (`$GOPATH/pkg/mod`):

```bash
env -u GOMODCACHE go mod download
env -u GOMODCACHE go test ./...
```

Use **`go mod download`** with **`required_permissions: ["network"]` or `["all"]`** the first time the cache is cold (after **prompting** for that permission).

- **`["network"]` alone is often not enough** for `go test` because compilation still needs a **readable** module cache.
- **Prompt for out-of-sandbox / full permissions**, then re-run with **`["all"]`** and the **`GOMODCACHE`** / **`env -u`** fix above—**do not** skip tests or push the user to run commands manually unless they decline elevation.

**Acceptance tests** live under `test/acceptance/` with **build tags** (they are **not** included in plain `go test ./...`). See **`test/acceptance/README.md`** for tags, env vars, and CI parity. When you change CLI-facing behavior, **prompt for out-of-sandbox permissions** and run them the same way, with **`required_permissions: ["all"]`** (and network) when using an agent shell.

### Sandbox and permissions

Commands that need network (e.g. `git push`, `gh pr create`, `npm install`) or full system access often fail in the sandbox with **TLS/certificate errors** (e.g. `x509: OSStatus -26276`) or auth prompts. These are sandbox restrictions, not broken tooling.

**Prompt the user for permission to run outside the sandbox** (e.g. **`required_permissions: ["all"]`** or **`["network"]`**) and **re-run the same command** after they approve. See **Agent command execution** above. Only fall back to copy-pasteable “run this locally” instructions when elevation is refused or unavailable.

### Git commits (GPG)

**Always use GPG-signed commits** (`git commit -S`, or `commit.gpgsign=true` in git config). **Do not** use `--no-gpg-sign` to bypass signing.

In restricted environments, signing may fail with errors like “No agent running” or “Operation not permitted” on `~/.gnupg`. **Prompt for permission to run the commit outside the sandbox** (full permissions) so `gpg-agent` is reachable; only if that still fails, suggest the user sign from a normal local terminal. Unsigned commits should not be pushed as a shortcut.

### Linting and Formatting
```bash
# Format code
go fmt ./...

# Run linter (if available)
golangci-lint run

# Vet code
go vet ./...
```

### Local Development
```bash
# Run CLI directly during development
go run cmd/hookdeck/main.go <command>

# Example: Test login command
go run cmd/hookdeck/main.go login --help
```

### Sandbox and command execution
- **Always run tests** when changing code: **`go test ./...`** from the repo root (see **Running `go test` after code changes** above), and for CLI-facing changes, acceptance tests with the appropriate **`-tags=...`** (see `test/acceptance/README.md`). Do not skip tests to avoid failures.
- For **`go test`** in this repo, **prompt for out-of-sandbox execution** and use **`required_permissions: ["all"]`** (and **`["network"]`** when needed) so the Go module cache works (see **Running `go test` after code changes**). Same for acceptance tests that call the real API.
- When you see **TLS/certificate errors**, **`no required module provides package`** during `go test`, **permission errors** on the module cache, or similar environment-related failures, **prompt for full/out-of-sandbox permissions** and **re-run** (with **`GOMODCACHE=...`** if needed)—**do not** treat the run as passed, and **do not** default to asking the user to run the command manually unless they declined elevation.

### GitHub CLI (`gh`)
- Use the **[GitHub CLI](https://cli.github.com/) (`gh`)** to read GitHub data and perform actions from the shell: **workflow runs and job logs** (e.g. `gh run list`, `gh run view <run-id> --log-failed`, `gh run view <run-id> --job <job-id> --log`), **PRs and checks** (`gh pr view`, `gh pr checks`, `gh pr diff`), **API access** (`gh api`), and creating or updating PRs, issues, and releases.
- Install and authenticate `gh` where needed (e.g. `gh auth login`). If `gh` fails with TLS, network, or permission errors, **prompt for permission** to re-run with **network** or **all** permissions when the agent sandbox may be blocking access.

## 6. Documentation Standards

### Command help text (Short and Long)

Use the shared helpers in **`pkg/cmd/helptext.go`** for resource commands so Short and the common part of Long are defined once and stay consistent across sources, connections, and any future resources.

- **Resource constants:** `ResourceSource`, `ResourceConnection` (singular form, e.g. "source", "connection").
- **Short (one line):** Use `ShortGet(resource)`, `ShortList(resource)`, `ShortDelete(resource)`, `ShortDisable(resource)`, `ShortEnable(resource)`, `ShortUpdate(resource)`, `ShortCreate(resource)`, `ShortUpsert(resource)` instead of literal strings.
- **Long (intro paragraph):** Use `LongGetIntro(resource)`, `LongUpdateIntro(resource)`, `LongDeleteIntro(resource)`, `LongDisableIntro(resource)`, `LongEnableIntro(resource)`, `LongUpsertIntro(resource)` for the first sentence/paragraph, then append command-specific content (e.g. Examples, extra paragraphs) in the command file.

When adding a **new resource** that follows the same CRUD/get/list/delete/disable/enable/create/upsert pattern, add a new constant (e.g. `ResourceDestination`) and use the same Short/Long intro helpers; extend `helptext.go` only when you need a new *pattern* (e.g. a new verb), not for each resource. Keep command-specific wording (e.g. "Create a connection between a source and destination", list filter descriptions) in the command file.

### Cobra Example and output for website docs

CLI content is generated for the website via `tools/generate-reference`. The generator emits usage, **arguments** (if `Annotations["cli.arguments"]` is set), flags, and the command's `Example` field. Human-injected content in the website (output examples, scenario walkthroughs, behavioral notes) is **required**—it improves docs beyond what generation provides.

- **Arguments:** For commands with positional args, set `c.Annotations["cli.arguments"]` to a JSON array of `{name, type, description, required}`. The generator emits an Arguments table before Flags. See `pkg/cmd/listen.go` for an example.
- **Example (simple output):** Add to Cobra `Example` when the output is short, generic, and helps users verify success (e.g. create, list, get). Keep it representative of actual CLI output; truncate long TUI with `...` if needed.
- **Example (complex output):** For long or scenario-specific output (e.g. dry-run, multi-step flows), add it in the website mdoc as human content immediately after the command's generated section. Split GENERATE blocks so the human section sits next to that command (see website AGENTS.md).
- **Heading level for human sections:** Use `####` (h4) for human-injected sections (e.g. dry-run output, scenario addenda) so they do not appear in the sidebar TOC. Use `###` (h3) or higher only for sections that should appear in the TOC.
- **Behavioral notes:** Add short clarifications to `Long` when they apply everywhere (e.g. "Use `--dry-run` to preview changes"; "`--disabled` shows all connections, not just disabled ones"). Longer narrative goes in the website.
- **Keep in sync:** When CLI output (success messages, TUI, error text) changes, update the website examples (CI, listen, etc.) or Cobra Example so generated docs stay accurate.

### CLI Documentation
- **REFERENCE.md**: Must include all commands with examples
- Use status indicators: ✅ Current vs 🚧 Planned
- Include realistic examples with actual API responses
- Document all flag combinations and their validation rules

### Code Documentation
- Document exported functions and types
- Include usage examples for complex functions
- Explain validation logic and type relationships
- Comment on OpenAPI schema mappings where non-obvious

## 7. Error Handling Patterns

### CLI Error Messages
```go
// Good: Specific, actionable error messages
return errors.New("--webhook-secret is required for Stripe sources")

// Good: Suggest alternatives
return fmt.Errorf("unsupported source type: %s. Supported types: STRIPE, GITHUB, HTTP", sourceType)

// Avoid: Generic or unclear messages
return errors.New("invalid configuration")
```

### API Error Handling
```go
// Handle API errors gracefully
if apiErr, ok := err.(*hookdeck.APIError); ok {
    if apiErr.StatusCode == 400 {
        return fmt.Errorf("invalid request: %s", apiErr.Message)
    }
}
```

## 8. Dependencies and External Libraries

### Core Dependencies
- **Cobra**: CLI framework - follow existing patterns
- **Viper**: Configuration management
- **Go standard library**: Prefer over external dependencies when possible

### Adding New Dependencies
1. Evaluate if functionality exists in current dependencies
2. Prefer well-maintained, standard libraries
3. Update `go.mod` and commit changes
4. Document new dependency usage patterns

## 9. Testing Guidelines

- **Always run tests** when changing code. Run **`go test ./...`** from the repo root (see **§5 Running `go test` after code changes**). For CLI-facing changes, run acceptance tests with the correct **`-tags=...`** per `test/acceptance/README.md`. **Agents:** **prompt for out-of-sandbox / full permissions**, then run `go test` with **`required_permissions: ["all"]`** (and network if needed) so the module cache works—**do not** push the user to run tests manually unless elevation is refused.
- **Create tests for new functionality.** Add unit tests for validation and business logic; add acceptance tests for flows that use the CLI as a user or agent would (success and failure paths). Acceptance tests must pass or fail—no skipping to avoid failures.

### Acceptance Test Setup
Acceptance tests require a Hookdeck API key. See [`test/acceptance/README.md`](test/acceptance/README.md) for full details. Quick setup: create `test/acceptance/.env` with `HOOKDECK_CLI_TESTING_API_KEY=<key>`. The `.env` file is git-ignored and must never be committed.

### Acceptance tests and feature tags
Acceptance tests in `test/acceptance/` are partitioned by **feature build tags** so they can run in parallel (matrix slices plus a separate `acceptance-telemetry` job in CI; see [test/acceptance/README.md](test/acceptance/README.md)). Each `*_test.go` file must have exactly one feature tag (e.g. `//go:build connection`, `//go:build request`, `//go:build telemetry`). **Untagged test files are included in every `-tags=...` build**, including `-tags=telemetry` only, so non-telemetry tests would run in the telemetry job—do not add untagged `*_test.go` files. Use tags to balance and parallelize; same commands and env for local and CI.

### Unit Testing
- Test validation logic thoroughly
- Mock API calls for command tests
- Test error conditions and edge cases
- Include examples of valid/invalid flag combinations

### Integration Testing
- Test actual API interactions in isolated tests
- Use test fixtures for complex API responses
- Validate command output formats

## 10. Useful Commands Reference

| Command | Purpose |
|---------|---------|
| `go run cmd/hookdeck/main.go --help` | View CLI help |
| `go build -o hookdeck cmd/hookdeck/main.go` | Build CLI binary |
| `go test ./...` | All unit tests (repo root; agents: run with `required_permissions: ["all"]`) |
| `go test ./pkg/cmd/` | Test command implementations only |
| `go generate ./...` | Run code generation (if used) |
| `golangci-lint run` | Run comprehensive linting |

## 11. Common Patterns to Follow

### Idempotent Upsert Pattern

For resources that support declarative infrastructure-as-code workflows, provide `upsert` commands that create or update based on resource name:

**Command Signature:**
```bash
hookdeck <resource> upsert <name> [flags]
```

**Key Principles:**
1. **API-native idempotency**: Hookdeck PUT endpoints handle create-or-update natively when name is in request body
2. **Client-side checking ONLY for dry-run**: GET request only needed for `--dry-run` preview functionality
3. **Normal upsert flow**: Call PUT directly without checking existence (API handles it)
4. **Dual validation modes**:
   - Create mode: Requires source/destination (validated client-side before PUT)
   - Update mode: All flags optional, partial updates (API determines which mode applies)
5. **Dry-run support**: Add `--dry-run` flag to preview changes without applying
6. **Clear messaging**: Indicate whether CREATE or UPDATE will occur after API responds

**Example Implementation:**
```bash
# Create if doesn't exist
hookdeck connection upsert my-connection \
  --source-name "my-source" --source-type STRIPE \
  --destination-name "my-api" --destination-type HTTP \
  --destination-url "https://example.com"

# Update only rules (partial update)
hookdeck connection upsert my-connection \
  --rule-retry-strategy linear --rule-retry-count 5

# Preview changes before applying
hookdeck connection upsert my-connection \
  --description "New description" --dry-run

# No-op: connection exists, no flags provided (should not error)
hookdeck connection upsert my-connection
```

**Dry-Run Output Format:**
```
-- Dry Run: UPDATE --
Connection 'my-connection' (conn_123) will be updated with the following changes:
- Description: "New description"
- Rules: (ruleset will be replaced)
  - Filter: body contains '{"type":"payment"}'
```

**Implementation Strategy:**
```go
func runUpsertCommand(name string, flags Flags, dryRun bool) error {
    client := GetAPIClient()
    
    // DRY-RUN: GET request needed to show preview
    if dryRun {
        existing, err := client.GetResourceByName(name)
        if err != nil && !isNotFound(err) {
            return err
        }
        return previewChanges(existing, flags)
    }
    
    // NORMAL UPSERT: Call PUT directly, API handles idempotency
    req := buildUpsertRequest(name, flags)
    resource, err := client.UpsertResource(req)
    if err != nil {
        return err
    }
    
    // API response indicates whether CREATE or UPDATE occurred
    displayResult(resource)
    return nil
}
```

**Validation Strategy:**
- **Normal upsert**: Skip GET request, validate only required fields for create mode client-side
- **Dry-run mode**: Perform GET to fetch existing state, show diff preview
- **API validation**: Let PUT endpoint determine if operation is valid
- **Error handling**: API will return appropriate error if validation fails

**When to Use:**
- CI/CD pipelines managing webhook infrastructure
- Configuration-as-code scenarios
- Environments where idempotency is critical
- When you want to "ensure this configuration exists" rather than "create new" or "modify existing"

### Interactive Prompts
When required parameters are missing, prompt interactively:
```go
if flags.Type == "" {
    // Show available types and prompt for selection
    selectedType, err := promptForType()
    if err != nil {
        return err
    }
    flags.Type = selectedType
}
```

### Resource Reference Handling
```go
// Accept both names and IDs
func resolveResourceID(nameOrID string) (string, error) {
    // Try as ID first, then lookup by name
    if isValidID(nameOrID) {
        return nameOrID, nil
    }
    return lookupByName(nameOrID)
}
```

### Output Formatting
```go
// Support multiple output formats (when --format is implemented)
switch outputFormat {
case "json":
    return printJSON(resource)
case "yaml":
    return printYAML(resource)
default:
    return printTable(resource)
}
```

---

## Agent skills

- **Location:** Repo-specific agent skills live under **`skills/`** at the repository root (e.g. `skills/hookdeck-cli-release/`).
- **Cursor / Claude Code:** `.cursor/skills` and `.claude/skills` are **symlinks** to `../skills` so both tools load the same tree. Do not replace the whole `.cursor` directory with a symlink—only `skills`, so `.cursor/rules/` and similar can stay as normal files.
- **Windows:** Git must create symlinks correctly (`core.symlinks` / Developer Mode). If symlinks are missing after clone, recreate them (`mklink /D` on Windows, or copy `skills/` into `.cursor/skills` and `.claude/skills` as a fallback).
- **Releases:** For cutting GitHub releases, tags, npm/beta publish flow, and drafting release notes, use **`skills/hookdeck-cli-release/SKILL.md`**; human-facing steps remain in **README.md § Releasing**.

---

Following these guidelines ensures consistent, maintainable CLI commands that provide an excellent user experience while maintaining architectural consistency with the existing codebase.