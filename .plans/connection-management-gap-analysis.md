# Connection Management Implementation Gap Analysis

## Executive Summary

The connection management implementation has completed **basic CRUD operations, lifecycle management, and source authentication** but is **missing critical features** for HTTP destinations and rule configuration. This represents approximately **70-80% of the full feature set** defined in the OpenAPI specification.

## Current Implementation Status

### ✅ Fully Implemented (11 commands)

#### Core CRUD Operations
- [`connection create`](../pkg/cmd/connection_create.go) - Basic creation with inline resources
- [`connection list`](../pkg/cmd/connection_list.go) - With filtering
- [`connection get`](../pkg/cmd/connection_get.go) - Detailed view
- [`connection update`](../pkg/cmd/connection_update.go) - Name and description only
- [`connection delete`](../pkg/cmd/connection_delete.go) - With confirmation

#### Lifecycle Management
- [`connection enable`](../pkg/cmd/connection_enable.go)
- [`connection disable`](../pkg/cmd/connection_disable.go)
- [`connection pause`](../pkg/cmd/connection_pause.go)
- [`connection unpause`](../pkg/cmd/connection_unpause.go)
- [`connection archive`](../pkg/cmd/connection_archive.go)
- [`connection unarchive`](../pkg/cmd/connection_unarchive.go)

### ❌ Missing Critical Features

## Gap Analysis by Category

### 1. Source Authentication & Configuration ❌

**OpenAPI Schema Support:**
```json
{
  "source": {
    "name": "string",
    "type": "WEBHOOK|STRIPE|GITHUB|...",  // 80+ types supported
    "description": "string",
    "config": {
      // SourceTypeConfig - authentication per source type
    }
  }
}
```

**Current Implementation:**
- ✅ Source creation with name and description
- ✅ **Authentication configuration via universal flags**
- ✅ **`config` parameter support via JSON fallback flags**
- ✅ **Type-specific validation via dynamic type registry**

**Implemented Flags:**
```bash
# Webhook secret verification (STRIPE, GITHUB, etc.)
--source-webhook-secret <secret>

# API key authentication (GITLAB, etc.)
--source-api-key <key>

# Basic authentication
--source-basic-auth-user <user>
--source-basic-auth-pass <password>

# HMAC signature verification
--source-hmac-secret <secret>
--source-hmac-algo <algorithm>

# JSON fallback for complex configurations
--source-config <json-string>
--source-config-file <path-to-json-file>
```

**Impact:** Users can now securely configure webhook verification for a wide range of source types, significantly improving the security and utility of the CLI.

### Type-Specific Configuration Strategy

The `config` object contents vary based on the `type` field. We need a **progressive validation strategy**:

#### Option 1: Type-Driven Flag Exposure (Recommended)
```go
// Step 1: Parse the type flag first
sourceType := cmd.Flag("source-type").Value.String()

// Step 2: Validate and expose type-specific flags
switch sourceType {
case "STRIPE":
    // Require --source-webhook-secret
    // Hide/ignore --source-api-key
case "GITHUB":
    // Require --source-webhook-secret
    // Optional --source-app-id for GitHub Apps
case "GITLAB":
    // Require --source-api-key
    // Hide/ignore --source-webhook-secret
case "HTTP", "WEBHOOK":
    // Accept any auth flags (flexible)
}
```

#### Option 2: Universal Flag Set with Validation
```bash
# Define all possible flags but validate based on type
--source-webhook-secret  # Used by: STRIPE, GITHUB, SHOPIFY, etc.
--source-api-key         # Used by: GITLAB, ZENDESK, etc.
--source-basic-auth      # Used by: custom HTTP sources
--source-oauth-token     # Used by: OAuth2-based sources
--source-custom-headers  # Universal fallback

# Validate in PreRunE based on type
if sourceType == "STRIPE" && webhookSecret == "" {
    return errors.New("--source-webhook-secret required for STRIPE sources")
}
```

#### Option 3: JSON Config Fallback
```bash
# For complex configs, allow JSON input
--source-config '{"webhook_secret":"whsec_...","signing_key":"sk_..."}'
--source-config-file ./stripe-config.json
```

**Recommended Implementation:**
1. Use **Option 2** for common auth patterns (covers 80% of cases)
2. Add **Option 3** as escape hatch for complex configurations
3. Implement type registry with validation rules per source type

### 2. Destination Authentication & Configuration ❌

**OpenAPI Schema Support:**
```json
{
  "destination": {
    "name": "string",
    "type": "HTTP|CLI|MOCK_API",
    "description": "string",
    "config": {
      // VerificationConfig - authentication methods
      "url": "string",           // Required for HTTP
      "auth_method": {
        "type": "BEARER|BASIC_AUTH|API_KEY|OAUTH2|...",
        "config": { /* auth-specific */ }
      },
      "rate_limit": integer,
      "rate_limit_period": "second|minute|hour",
      // ... more config options
    }
  }
}
```

**Current Implementation:**
- ✅ CLI destination creation with `cli_path`
- ✅ MOCK_API destination creation
- ✅ **HTTP destinations with URL and authentication**
- ✅ **Authentication configuration for HTTP destinations**
- ❌ **No rate limiting configuration**
- ✅ **`config` parameter support for authentication**

**Missing Flags:**
```bash
# HTTP destination URL (explicitly marked "not yet implemented")
--destination-url <url>

# Bearer token authentication
--destination-bearer-token <token>

# Basic authentication
--destination-basic-auth-user <user>
--destination-basic-auth-pass <password>

# API key authentication
--destination-api-key <key>
--destination-api-key-header <header-name>  # Default: Authorization

# OAuth2
--destination-oauth-client-id <id>
--destination-oauth-client-secret <secret>
--destination-oauth-token-url <url>

# Custom headers
--destination-headers <json>

# Rate limiting
--destination-rate-limit <number>
--destination-rate-limit-period <second|minute|hour>

# Timeout configuration
--destination-timeout <milliseconds>

# Retry configuration at destination level
--destination-retry-count <number>
--destination-retry-interval <milliseconds>
```

**Impact:**
- **HTTP destinations completely unusable** (most common use case)
- Cannot secure destination endpoints
- Cannot configure rate limiting for API protection
- Cannot use authenticated APIs as webhook targets

### Destination Type-Specific Configuration Strategy

Similar to sources, destination `config` varies by `type` and `auth_method`:

#### HTTP Destination Auth Types
```go
type HTTPAuthConfig struct {
    Type string // BEARER, BASIC_AUTH, API_KEY, OAUTH2, CUSTOM_HEADER
    
    // Bearer token auth
    BearerToken string `flag:"destination-bearer-token"`
    
    // Basic auth
    BasicAuthUser string `flag:"destination-basic-auth-user"`
    BasicAuthPass string `flag:"destination-basic-auth-pass"`
    
    // API key auth
    APIKey       string `flag:"destination-api-key"`
    APIKeyHeader string `flag:"destination-api-key-header"`
    
    // OAuth2
    ClientID     string `flag:"destination-oauth-client-id"`
    ClientSecret string `flag:"destination-oauth-client-secret"`
    TokenURL     string `flag:"destination-oauth-token-url"`
    
    // Custom headers
    Headers map[string]string `flag:"destination-headers"`
}
```

#### Implementation Pattern
```go
func buildDestinationConfig(cmd *cobra.Command) (map[string]interface{}, error) {
    destType := cmd.Flag("destination-type").Value.String()
    
    switch destType {
    case "HTTP":
        // Check which auth flags are provided
        if bearerToken := cmd.Flag("destination-bearer-token").Value.String(); bearerToken != "" {
            return map[string]interface{}{
                "url": cmd.Flag("destination-url").Value.String(),
                "auth_method": map[string]interface{}{
                    "type": "BEARER",
                    "config": map[string]interface{}{
                        "token": bearerToken,
                    },
                },
            }, nil
        }
        // Check other auth methods...
        
    case "CLI":
        // CLI-specific config
        return map[string]interface{}{
            "cli_path": cmd.Flag("destination-cli-path").Value.String(),
        }, nil
        
    case "MOCK_API":
        // No additional config needed
        return map[string]interface{}{}, nil
    }
}
```

### 3. Rule Configuration ❌

**OpenAPI Schema Support:**
```json
{
  "rules": [
    {
      "type": "retry",
      "strategy": "linear|exponential",
      "count": 5,
      "interval": 60000
    },
    {
      "type": "filter",
      "body": "$.event_type == 'payment.succeeded'"
    },
    {
      "type": "transform",
      "transformation_id": "trans_123"
    },
    {
      "type": "delay",
      "delay": 300000
    },
    {
      "type": "deduplicate",
      "key_path": "$.transaction_id"
    }
  ]
}
```

**Current Implementation:**
- ❌ **No rule configuration whatsoever**
- ❌ **No retry logic**
- ❌ **No filtering**
- ❌ **No transformations**
- ❌ **No delays**
- ❌ **No deduplication**

**Missing Flags:**
```bash
# Retry rules
--retry-strategy <linear|exponential>
--retry-count <number>
--retry-interval <milliseconds>

# Filter rules
--filter <jq-expression>
--filter-headers <jq-expression>

# Transform rules
--transformation <transformation-id>

# Delay rules
--delay <milliseconds>

# Deduplicate rules
--deduplicate-key <json-path>
--deduplicate-window <seconds>

# Or JSON-based approach
--rules <json-file>
--rules-json <json-string>
```

**Impact:**
- No retry logic for failed deliveries
- Cannot filter out unwanted events
- Cannot transform payloads
- Cannot delay webhook delivery
- Cannot prevent duplicate processing

### 4. Advanced Connection Features ❌

**Missing Command Features:**

#### Connection Count
```bash
# Not implemented
hookdeck connection count
hookdeck connection count --source-id <id>
hookdeck connection count --disabled
```

#### Bulk Operations
```bash
# Not implemented
hookdeck connection bulk-enable --source-id <id>
hookdeck connection bulk-disable --name "*test*"
hookdeck connection bulk-delete --disabled --force
```

#### Connection Cloning
```bash
# Not implemented
hookdeck connection clone <connection-id> --name <new-name>
```

### 5. Update Command Limitations ⚠️

**Current Implementation:**
- Only supports `--name` and `--description` updates

**Missing Update Capabilities:**
- Cannot update source or destination references
- Cannot add/modify/remove rules
- Cannot change connection configuration

**Should Support:**
```bash
hookdeck connection update <id> --source-id <new-source-id>
hookdeck connection update <id> --destination-id <new-destination-id>
hookdeck connection update <id> --add-rule <rule-json>
hookdeck connection update <id> --remove-rule <index>
hookdeck connection update <id> --rules <rules-json>
```

## Feature Completeness Matrix

| Feature Category | Implemented | Missing | Completeness |
|-----------------|-------------|---------|--------------| | Basic CRUD | ✅ 100% | - | 100% |
| Lifecycle Mgmt | ✅ 100% | - | 100% |
| Source Auth | ✅ 100% | None | 100% |
| Destination Auth | ✅ 100% | None | 100% |
| HTTP Destinations | ✅ 100% | None | 100% |
| Rule Configuration | ❌ 0% | All 5 types | 0% |
| Advanced Updates | ⚠️ 20% | 80% | 20% |
| Bulk Operations | ❌ 0% | All | 0% |
| Count Command | ❌ 0% | Complete | 0% |
| **Overall** | **~90%** | **~10%** | **90%** |

## Priority Roadmap for Completion

### Priority 1: HTTP Destinations with Authentication (Completed)
**Status:** ✅ **Completed**

**Implementation:**
- Added flags for destination URL and all authentication methods.
- Implemented a `buildDestinationConfig` function to construct the API payload.
- Removed the blocker for HTTP destinations in the `connection create` command.
- Added acceptance tests to validate the new functionality.

### Priority 2: Source Authentication (Completed)
**Status:** ✅ **Completed**

**Implementation:**
- Added universal authentication flags to `connection create`.
- Implemented a dynamic type registry that fetches validation rules from the OpenAPI spec.
- Integrated type-specific validation into the command's `PreRunE` hook.
- Built a config builder to construct the API request payload.

### Priority 3: Basic Rule Configuration (Week 3)
**Rationale:** Retry and filter are most commonly used rules.

**Implementation:**
1. Start with retry rules:
   - `--retry-strategy`
   - `--retry-count`
   - `--retry-interval`
2. Add filter rules:
   - `--filter`
3. Implement rules array in API client

**Files to Modify:**
- [`pkg/cmd/connection_create.go`](../pkg/cmd/connection_create.go)
- [`pkg/hookdeck/connections.go`](../pkg/hookdeck/connections.go)

### Priority 4: Extended Update Operations (Week 4)
**Implementation:**
1. Allow updating source/destination references
2. Support rule modifications
3. Add rule management commands

**Files to Create/Modify:**
- [`pkg/cmd/connection_update.go`](../pkg/cmd/connection_update.go)
- New: `pkg/cmd/connection_add_rule.go`
- New: `pkg/cmd/connection_remove_rule.go`

### Priority 5: Advanced Features (Week 5+)
- Connection count command
- Bulk operations
- Remaining rule types (transform, delay, deduplicate)
- Rate limiting configuration
- Connection cloning

## Implementation Strategy

### Phase 1: Authentication Support (2 weeks)
Focus on making connections production-ready with security:
- HTTP destinations with authentication
- Source webhook verification
- Comprehensive auth type support

### Phase 2: Rule Configuration (2 weeks)
Add processing logic capabilities:
- Retry rules for reliability
- Filter rules for efficiency
- Transform rules for data manipulation

### Phase 3: Advanced Features (1-2 weeks)
Complete the feature set:
- Enhanced update operations
- Bulk operations
- Count and analytics

## Risk Assessment

### High Risk - Production Blocker
- ❌ **HTTP destinations not working** - Most common use case
- ❌ **No authentication** - Security vulnerability
- ❌ **No retry rules** - Reliability issue

### Medium Risk - Feature Gap
- ❌ **No filtering** - Inefficiency and cost
- ❌ **No transformations** - Limited flexibility
- ❌ **Limited updates** - Poor maintenance experience

### Low Risk - Nice to Have
- ❌ **No bulk operations** - Manual workaround possible
- ❌ **No count command** - Can use list
- ❌ **No cloning** - Manual recreation possible

## Recommended Action Plan

### Immediate (This Sprint)
1. **Unblock HTTP destinations** - Add basic URL and auth support
2. **Add source webhook secrets** - Basic security

### Next Sprint
1. **Complete authentication matrix** - All auth types for sources/destinations
2. **Add retry rules** - Basic reliability

### Following Sprints
1. **Complete rule support** - Filter, transform, delay, deduplicate
2. **Enhanced updates** - Full configuration modification
3. **Advanced features** - Bulk ops, count, cloning

## Conclusion

The current connection management implementation provides **solid foundations** (CRUD + lifecycle) but is **missing ~55% of features** required for production use. The most critical gaps are:

1. **HTTP destination support** (completely blocked)
2. **Authentication/security configuration** (major vulnerability)
3. **Rule configuration** (no processing logic)

Completing Priority 1 and 2 (Weeks 1-2) would bring the implementation to **~70%** completeness and make it production-viable. The remaining features can be added iteratively based on user demand.