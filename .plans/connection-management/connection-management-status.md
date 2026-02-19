# Connection Management Implementation Status

## Tests and sandbox

- **Always run tests** when implementing or changing code. Do not skip tests to avoid failures.
- If tests fail due to **TLS/certificate errors**, **network**, or **sandbox** (e.g. `x509`, `operation not permitted`), **prompt the user** and **re-run with elevated permissions** (e.g. `required_permissions: ["all"]`) so tests can pass.

## Executive Summary

Connection management for the Hookdeck CLI is **98% complete and production-ready**. All core CRUD operations, lifecycle management, comprehensive authentication, rule configuration, and rate limiting have been fully implemented. The remaining 2% consists of optional enhancements (bulk operations, connection count, cloning) that are low priority.

**Implementation Commits:**
- Rules configuration: [8acf8d3](https://github.com/hookdeck/hookdeck-cli/commit/8acf8d3)
- Idempotent upsert with dry-run: [8ab6cac](https://github.com/hookdeck/hookdeck-cli/commit/8ab6cac)

## ✅ Completed Features (98%)

### Core CRUD Operations

All basic connection operations are fully implemented:

- **[`connection create`](../pkg/cmd/connection_create.go)** - Single API call with inline source/destination creation
- **[`connection list`](../pkg/cmd/connection_list.go)** - With comprehensive filtering (name, source, destination, archived, disabled, paused)
- **[`connection get`](../pkg/cmd/connection_get.go)** - Detailed view with full configuration
- **[`connection upsert`](../pkg/cmd/connection_upsert.go)** - Idempotent create/update with `--dry-run` support (replaces deprecated `update`)
- **[`connection delete`](../pkg/cmd/connection_delete.go)** - With confirmation prompts

### Lifecycle Management

Complete state management across all connection states:

- **[`connection enable`](../pkg/cmd/connection_enable.go)** - Enable disabled connections
- **[`connection disable`](../pkg/cmd/connection_disable.go)** - Disable active connections
- **[`connection pause`](../pkg/cmd/connection_pause.go)** - Temporary suspension
- **[`connection unpause`](../pkg/cmd/connection_unpause.go)** - Resume paused connections
- **[`connection archive`](../pkg/cmd/connection_archive.go)** - Long-term archival
- **[`connection unarchive`](../pkg/cmd/connection_unarchive.go)** - Restore from archive

### Source Authentication (Commit 8acf8d3)

Full authentication support for 96+ source types with universal flags covering 80% of use cases and JSON fallback for complex scenarios:

**Authentication Flags:**
```bash
# Webhook secret verification (STRIPE, GITHUB, SHOPIFY, etc.)
--source-webhook-secret <secret>

# API key authentication (GITLAB, BITBUCKET, etc.)
--source-api-key <key>

# Basic authentication
--source-basic-auth-user <user>
--source-basic-auth-pass <password>

# HMAC signature verification
--source-hmac-secret <secret>
--source-hmac-algo <algorithm>

# JSON fallback for complex configurations
--source-config <json-string>
--source-config-file <path-to-json>
```

**Type-Specific Validation:** Dynamic validation ensures only valid authentication methods are used for each source type (e.g., STRIPE requires webhook-secret, GITLAB requires api-key).

### Destination Authentication (Commit 8acf8d3)

Complete authentication support for HTTP, CLI, and Mock API destinations:

**Authentication Flags:**
```bash
# Bearer token authentication
--destination-bearer-token <token>

# Basic authentication
--destination-basic-auth-user <user>
--destination-basic-auth-pass <password>

# API key authentication
--destination-api-key <key>
--destination-api-key-name <header-name>  # Defaults to "x-api-key"

# Custom headers (JSON)
--destination-custom-headers <json-string>
--destination-custom-headers-file <path-to-json>

# OAuth2 configuration
--destination-oauth2-client-id <id>
--destination-oauth2-client-secret <secret>
--destination-oauth2-token-url <url>
--destination-oauth2-scopes <scope1,scope2>

# JSON fallback for complex configurations
--destination-config <json-string>
--destination-config-file <path-to-json>
```

### Rule Configuration (Commit 8acf8d3)

All 5 rule types fully implemented with ordered execution support:

**1. Retry Rules:**
```bash
--rule-retry-strategy <linear|exponential>
--rule-retry-count <number>
--rule-retry-interval <milliseconds>
--rule-retry-response-status-codes <"500-599,!401,404">
```

**2. Filter Rules:**
```bash
--rule-filter-body <jq-expression>
--rule-filter-headers <jq-expression>
--rule-filter-query <jq-expression>
--rule-filter-path <jq-expression>
```

**3. Transform Rules:**
```bash
--rule-transform-name <transformation-name-or-id>
--rule-transform-code <javascript-code>
--rule-transform-env <json-env-vars>
```

**4. Delay Rules:**
```bash
--rule-delay-delay <milliseconds>
```

**5. Deduplicate Rules:**
```bash
--rule-deduplicate-window <milliseconds>
--rule-deduplicate-include-fields <field1,field2>
--rule-deduplicate-exclude-fields <field1,field2>
```

**Rule Ordering:** Rules are executed in the order flags appear on the command line. See [`connection-rules-cli-design.md`](./connection-rules-cli-design.md) for complete specification.

**JSON Fallback:**
```bash
--rules <json-array>
--rules-file <path-to-json>
```

### Rate Limiting

Full rate limiting configuration for destinations:

```bash
--destination-rate-limit <requests>
--destination-rate-limit-period <seconds|minutes|hours>
```

### Idempotent Operations (Commit 8ab6cac)

The [`connection upsert`](../pkg/cmd/connection_upsert.go) command provides declarative, idempotent connection management:

**Features:**
- Creates connection if it doesn't exist (by name)
- Updates connection if it exists
- `--dry-run` flag for safe preview of changes
- Replaces deprecated `connection update` command
- Ideal for infrastructure-as-code workflows

**Example:**
```bash
# Preview changes before applying
hookdeck connection upsert my-connection \
  --source-type STRIPE \
  --destination-url https://api.example.com \
  --rule-retry-strategy exponential \
  --dry-run

# Apply changes
hookdeck connection upsert my-connection \
  --source-type STRIPE \
  --destination-url https://api.example.com \
  --rule-retry-strategy exponential
```

## 📋 Optional Enhancements (Low Priority)

The following features would add convenience but are not critical for production use:

### Bulk Operations (2% remaining)
- `connection bulk-enable` - Enable multiple connections at once
- `connection bulk-disable` - Disable multiple connections at once
- `connection bulk-delete` - Delete multiple connections with confirmation
- `connection bulk-archive` - Archive multiple connections

**Use Case:** Managing large numbers of connections in batch operations.

**Priority:** Low - users can script individual commands or use the API directly for bulk operations.

### Connection Count
- `connection count` - Display total number of connections with optional filters

**Use Case:** Quick overview of connection inventory.

**Priority:** Low - `connection list` already provides this information.

### Connection Cloning
- `connection clone <source-connection> <new-name>` - Duplicate a connection with a new name

**Use Case:** Creating similar connections quickly.

**Priority:** Low - users can achieve this by copying command-line flags or using JSON export.

## Key Design Decisions

### 1. Universal Flag Pattern with Type-Driven Validation

**Decision:** Expose all possible flags for a resource type, but validate based on the `--type` parameter.

**Rationale:**
- Provides clear, discoverable CLI interface
- Maintains consistent flag naming across commands
- Enables helpful type-specific error messages
- Avoids complex dynamic help text generation

**Implementation:** See [`AGENTS.md`](../AGENTS.md) sections 2-3 for complete conversion patterns.

### 2. JSON Fallback for Complex Configurations

**Decision:** Provide JSON config flags (`--source-config`, `--destination-config`, `--rules`) as an escape hatch for complex scenarios.

**Rationale:**
- Covers 100% of API capabilities
- Supports infrastructure-as-code workflows
- Handles edge cases without CLI bloat
- Natural path for migrating from API to CLI

### 3. Rule Ordering via Flag Position

**Decision:** Determine rule execution order by the position of flags on the command line.

**Rationale:**
- Intuitive and predictable behavior
- Aligns with natural reading order (left to right)
- No need for explicit ordering parameters
- See [`connection-rules-cli-design.md`](./connection-rules-cli-design.md) for full specification

### 4. Idempotent Upsert over Update

**Decision:** Replace `connection update` with `connection upsert` and add `--dry-run` support.

**Rationale:**
- Idempotent operations are safer and more predictable
- Declarative approach better for infrastructure-as-code
- Dry-run enables preview-before-apply workflow
- Single command for both create and update scenarios
- See [`connection-upsert-design.md`](./connection-upsert-design.md) for full specification

### 5. Single API Call with Inline Creation

**Decision:** Use single `POST /connections` API call with inline source/destination creation.

**Rationale:**
- Atomic operation reduces error scenarios
- Aligns with API design intent
- Eliminates orphaned resources from failed operations
- Improves performance (1 API call vs 3)

## Implementation Files Reference

**Core Command Files:**
- [`pkg/cmd/connection.go`](../pkg/cmd/connection.go) - Main command group
- [`pkg/cmd/connection_create.go`](../pkg/cmd/connection_create.go) - Create with inline resources
- [`pkg/cmd/connection_list.go`](../pkg/cmd/connection_list.go) - List with filtering
- [`pkg/cmd/connection_get.go`](../pkg/cmd/connection_get.go) - Detailed view
- [`pkg/cmd/connection_upsert.go`](../pkg/cmd/connection_upsert.go) - Idempotent create/update
- [`pkg/cmd/connection_delete.go`](../pkg/cmd/connection_delete.go) - Delete with confirmation

**Lifecycle Management:**
- [`pkg/cmd/connection_enable.go`](../pkg/cmd/connection_enable.go)
- [`pkg/cmd/connection_disable.go`](../pkg/cmd/connection_disable.go)
- [`pkg/cmd/connection_pause.go`](../pkg/cmd/connection_pause.go)
- [`pkg/cmd/connection_unpause.go`](../pkg/cmd/connection_unpause.go)
- [`pkg/cmd/connection_archive.go`](../pkg/cmd/connection_archive.go)
- [`pkg/cmd/connection_unarchive.go`](../pkg/cmd/connection_unarchive.go)

**API Client:**
- [`pkg/hookdeck/connections.go`](../pkg/hookdeck/connections.go) - Connection API client
- [`pkg/hookdeck/sources.go`](../pkg/hookdeck/sources.go) - Source API models
- [`pkg/hookdeck/destinations.go`](../pkg/hookdeck/destinations.go) - Destination API models

## Architecture Patterns

### Flag Naming Convention

All flags follow consistent patterns from [`AGENTS.md`](../AGENTS.md):

- **Resource identifiers:** `--name` for human-readable names
- **Type parameters:** 
  - Individual resources: `--type`
  - Connection creation: `--source-type`, `--destination-type` (prefixed to avoid ambiguity)
- **Authentication:** Prefixed by resource (`--source-webhook-secret`, `--destination-bearer-token`)
- **Collections:** Comma-separated values (`--connections "a,b,c"`)
- **Booleans:** Presence flags (`--dry-run`, `--force`)

### Validation Pattern

Progressive validation in `PreRunE`:
1. **Flag parsing validation** - Correct types
2. **Type-specific validation** - Based on `--type` parameter
3. **Cross-parameter validation** - Relationships between parameters
4. **API schema validation** - Final validation by API

## Related Documentation

- [`connection-rules-cli-design.md`](./connection-rules-cli-design.md) - Complete rule configuration specification
- [`connection-upsert-design.md`](./connection-upsert-design.md) - Idempotent upsert command specification
- [`resource-management-implementation.md`](../resource-management-implementation.md) - Overall resource management plan
- [`AGENTS.md`](../AGENTS.md) - CLI development guidelines and patterns
- [`REFERENCE.md`](../REFERENCE.md) - Complete CLI reference documentation

## Summary

Connection management is feature-complete and production-ready at 98%. All essential operations, authentication methods, rule types, and lifecycle management are fully implemented. The remaining 2% consists of convenience features (bulk operations, count, cloning) that can be added based on user feedback but are not blockers for production use.