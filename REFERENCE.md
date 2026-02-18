# Hookdeck CLI Reference

> [!IMPORTANT]
> This document is a work in progress and is not 100% accurate.

The Hookdeck CLI provides comprehensive webhook infrastructure management including authentication, project management, resource management, event and attempt querying, and local development tools. This reference covers all available commands and their usage.

## Table of Contents

### Current Functionality ✅
- [Global Options](#global-options)
- [Authentication](#authentication)
- [Projects](#projects) (list and use only)
- [Local Development](#local-development)
- [CI/CD Integration](#cicd-integration)
- [Utilities](#utilities)
- [Events](#events)
- [Attempts](#attempts)
- [Requests](#requests)
- [Current Limitations](#current-limitations)

### Planned Functionality 🚧
- [Advanced Project Management](#advanced-project-management)
- [Sources](#sources)
- [Destinations](#destinations)
- [Connections](#connections)
- [Transformations](#transformations)
- [Issue Triggers](#issue-triggers)
- [Bookmarks](#bookmarks)
- [Integrations](#integrations)
- [Issues](#issues)
- [Bulk Operations](#bulk-operations)
- [Notifications](#notifications)
- [Implementation Status](#implementation-status)

## Global Options

All commands support these global options:

### ✅ Current Global Options
```bash
--profile, -p string     Profile name (default "default")
--api-key string         Your API key to use for the command (hidden)
--cli-key string         CLI key for legacy auth (deprecated, hidden)
--color string           Turn on/off color output (on, off, auto)
--config string          Config file (default is $HOME/.config/hookdeck/config.toml)
--device-name string     Device name for this CLI instance
--log-level string       Log level: debug, info, warn, error (default "info")
--insecure              Allow invalid TLS certificates
--version, -v           Show version information
--help, -h              Show help information
```

### 🔄 Partially Implemented Options
```bash
--output json           Output in JSON format (available on: connection create/list/get/upsert)
                        Default: human-readable format
```

### � Planned Global Options
```bash
--project string         Project ID to use (overrides profile)
--output string          Additional output formats: table, yaml (currently only json supported)
```

## Authentication

**All Parameters:**
```bash
# Login command parameters
--api-key string       API key for direct authentication
--interactive, -i      Interactive login with prompts (boolean flag)
--profile string       Profile name to use for login

# Logout command parameters  
--all, -a             Logout all profiles (boolean flag)
--profile string      Profile name to logout

# Whoami command parameters
# (No additional parameters - uses global options only)
```

### ✅ Login
```bash
# Interactive login with prompts
hookdeck login
hookdeck login --interactive
hookdeck login -i

# Login with API key directly
hookdeck login --api-key your_api_key

# Use different profile
hookdeck login --profile production
```

### ✅ Logout
```bash
# Logout current profile
hookdeck logout

# Logout specific profile
hookdeck logout --profile production

# Logout all profiles
hookdeck logout --all
hookdeck logout -a
```

### ✅ Check authentication status
```bash
hookdeck whoami

# Example output:
# Using profile default (use -p flag to use a different config profile)
# 
# Logged in as john@example.com (John Doe) on project Production in organization Acme Corp
```

## Projects

**All Parameters:**
```bash
# Project list command parameters
[organization_substring] [project_substring]    # Positional arguments for filtering
# (No additional flag parameters)

# Project use command parameters
[project-id]           # Positional argument for specific project ID
--profile string       # Profile name to use

# Project create command parameters (planned)
--name string          # Required: Project name
--description string   # Optional: Project description

# Project get command parameters (planned)
[project-id]           # Positional argument for specific project ID

# Project update command parameters (planned)
<project-id>          # Required positional argument for project ID
--name string         # Update project name
--description string  # Update project description

# Project delete command parameters (planned)
<project-id>          # Required positional argument for project ID
--force              # Force delete without confirmation (boolean flag)
```

Projects are top-level containers for your webhook infrastructure.

### ✅ List projects
```bash
# List all projects you have access to
hookdeck project list

# Filter by organization substring
hookdeck project list acme

# Filter by organization and project substrings  
hookdeck project list acme production

# Example output:
# [Acme Corp] Production
# [Acme Corp] Staging (current)
# [Test Org] Development
```

### ✅ Use project (set as current)
```bash
# Interactive selection from available projects
hookdeck project use

# Use specific project by ID
hookdeck project use proj_123

# Use with different profile
hookdeck project use --profile production
```

## Local Development

**All Parameters:**
```bash
# Listen command parameters
[port or URL]         # Required positional argument (e.g., "3000" or "http://localhost:3000")
[source]              # Optional positional argument for source name
[connection]          # Optional positional argument for connection name
--path string         # Specific path to forward to (e.g., "/webhooks")
--no-healthcheck      # Disable periodic health checks of the local server
--no-wss             # Force unencrypted WebSocket connection (hidden flag)
```

### ✅ Listen for webhooks
```bash
# Start webhook forwarding to localhost (with interactive prompts)
hookdeck listen

# Forward to specific port
hookdeck listen 3000

# Forward to specific URL
hookdeck listen http://localhost:3000

# Forward with source and connection specified
hookdeck listen 3000 stripe-webhooks payment-connection

# Forward to specific path
hookdeck listen --path /webhooks

# Disable periodic health checks of the local server
hookdeck listen --no-healthcheck 3000

# Force unencrypted WebSocket connection (hidden flag)
hookdeck listen --no-wss

# Arguments:
# - port or URL: Required (e.g., "3000" or "http://localhost:3000")
# - source: Optional source name to forward from
# - connection: Optional connection name
```

The `listen` command forwards webhooks from Hookdeck to your local development server, allowing you to test webhook integrations locally.

## CI/CD Integration

**All Parameters:**
```bash
# CI command parameters
--api-key string      # API key (defaults to HOOKDECK_API_KEY env var)
--name string         # CI name (e.g., $GITHUB_REF for GitHub Actions)
```

### ✅ CI command
```bash
# Run in CI/CD environments
hookdeck ci

# Specify API key explicitly (defaults to HOOKDECK_API_KEY env var)
hookdeck ci --api-key <key>

# Specify CI name (e.g., for GitHub Actions)
hookdeck ci --name $GITHUB_REF
```

This command provides CI/CD specific functionality for automated deployments and testing.

## Utilities

**All Parameters:**
```bash
# Completion command parameters
[shell]               # Positional argument for shell type (bash, zsh, fish, powershell)
--shell string        # Explicit shell selection flag

# Version command parameters
# (No additional parameters - uses global options only)
```

### ✅ Shell completion
```bash
# Generate completion (auto-detects bash or zsh from $SHELL)
hookdeck completion

# Specify shell explicitly
hookdeck completion --shell bash
hookdeck completion --shell zsh

# Note: Only bash and zsh are currently supported
# The CLI auto-detects your shell from the SHELL environment variable
```

### ✅ Version information
```bash
hookdeck version

# Short version
hookdeck --version
```

## Current Limitations

The Hookdeck CLI provides comprehensive connection management capabilities. The following limitations currently exist:

- ❌ **Limited bulk operations** - Cannot perform batch operations on resources (e.g., bulk retry, bulk delete)
- ❌ **No project creation** - Cannot create, update, or delete projects via CLI (only list and use existing projects)
- ❌ **No issue management** - Cannot view or manage issues from CLI

---

# 🚧 Planned Functionality

*The following sections document planned functionality that is not yet implemented. This serves as a specification for future development.*

## Implementation Status

| Command Category | Status | Available Commands |
|------------------|--------|-------------------|
| Authentication | ✅ **Current** | `login`, `logout`, `whoami` |
| Project Management | 🔄 **Partial** | `project list`, `project use` |
| Local Development | ✅ **Current** | `listen` |
| CI/CD | ✅ **Current** | `ci` |
| Connection Management | ✅ **Current** | `connection create`, `connection list`, `connection get`, `connection upsert`, `connection delete`, `connection enable`, `connection disable`, `connection pause`, `connection unpause` |
| Shell Completion | ✅ **Current** | `completion` (bash, zsh) |
| Gateway (sources, destinations, connections, transformations, events, requests, attempts) | ✅ **Current** | `gateway source`, `gateway destination`, `gateway connection`, `gateway transformation`, `gateway event`, `gateway request`, `gateway attempt` |
| Source Management | ✅ **Current** | `gateway source list`, `get`, `create`, `upsert`, `update`, `delete`, `enable`, `disable`, `count` |
| Destination Management | ✅ **Current** | `gateway destination list`, `get`, `create`, `upsert`, `update`, `delete`, `enable`, `disable`, `count` |
| Transformation Management | ✅ **Current** | `gateway transformation list`, `get`, `create`, `upsert`, `update`, `delete`, `count`, `run`, `executions` |
| Event Querying | ✅ **Current** | `gateway event list`, `get`, `raw-body`, `retry`, `cancel`, `mute` |
| Attempt Management | ✅ **Current** | `gateway attempt list`, `get` |
| Request Management | ✅ **Current** | `gateway request list`, `get`, `raw-body`, `retry`, `events`, `ignored-events` |
| Issue Trigger Management | 🚧 **Planned** | *(Not implemented)* |
| Bookmark Management | 🚧 **Planned** | *(Not implemented)* |
| Integration Management | 🚧 **Planned** | *(Not implemented)* |
| Issue Management | 🚧 **Planned** | *(Not implemented)* |
| Bulk Operations | 🚧 **Planned** | *(Not implemented)* |

## Advanced Project Management

🚧 **PLANNED FUNCTIONALITY** - Not yet implemented

*Note: These project management commands are planned for implementation as documented in `.plans/resource-management-implementation.md` and are being developed in the `feat/project-create` branch.*

### Create a project
```bash
# Create with interactive prompts
hookdeck project create

# Create with flags
hookdeck project create --name "My Project" --description "Production webhooks"
```

### Get project details
```bash
# Get current project
hookdeck project get

# Get specific project
hookdeck project get proj_123

# Get with full details
hookdeck project get proj_123 --log-level debug
```

### Update project
```bash
# Update interactively
hookdeck project update

# Update specific project
hookdeck project update proj_123 --name "Updated Name"

# Update description
hookdeck project update proj_123 --description "New description"
```

### Delete project
```bash
# Delete with confirmation
hookdeck project delete proj_123

# Force delete without confirmation
hookdeck project delete proj_123 --force
```

## Sources

**All Parameters:**
```bash
# Source list command parameters
--name string         # Filter by name pattern (supports wildcards)
--type string         # Filter by source type (96+ types supported)
--disabled           # Include disabled sources (boolean flag)
--order-by string    # Sort by: name, created_at, updated_at
--dir string         # Sort direction: asc, desc
--limit integer      # Limit number of results (0-255)
--next string        # Next page token for pagination
--prev string        # Previous page token for pagination

# Source count command parameters
--name string         # Filter by name pattern
--disabled           # Include disabled sources (boolean flag)

# Source get command parameters
<source-id>          # Required positional argument for source ID
--include string     # Include additional data (e.g., "config.auth")

# Source create command parameters
--name string         # Required: Source name
--type string         # Required: Source type (see type-specific parameters below)
--description string  # Optional: Source description

# Type-specific parameters for source create/update/upsert:
# When --type=STRIPE, GITHUB, SHOPIFY, SLACK, TWILIO, etc.:
--webhook-secret string     # Webhook secret for signature verification

# When --type=PAYPAL:
--webhook-id string         # PayPal webhook ID (not webhook_secret)

# When --type=GITLAB, OKTA, MERAKI, etc.:
--api-key string           # API key for authentication

# When --type=BRIDGE, FIREBLOCKS, DISCORD, TELNYX, etc.:
--public-key string        # Public key for signature verification

# When --type=POSTMARK, PIPEDRIVE, etc.:
--username string          # Username for basic authentication
--password string          # Password for basic authentication

# When --type=RING_CENTRAL, etc.:
--token string             # Authentication token

# When --type=EBAY (complex multi-field authentication):
--environment string       # PRODUCTION or SANDBOX
--dev-id string           # Developer ID
--client-id string        # Client ID
--client-secret string    # Client secret
--verification-token string # Verification token

# When --type=TIKTOK_SHOP (multi-key authentication):
--webhook-secret string    # Webhook secret
--app-key string          # Application key

# When --type=FISERV:
--webhook-secret string    # Webhook secret
--store-name string       # Optional: Store name

# When --type=VERCEL_LOG_DRAINS:
--webhook-secret string       # Webhook secret
--log-drains-secret string   # Optional: Log drains secret

# When --type=HTTP (custom HTTP source):
--auth-type string        # Authentication type (HMAC, API_KEY, BASIC, etc.)
--algorithm string        # HMAC algorithm (sha256, sha1, etc.)
--encoding string         # HMAC encoding (hex, base64, etc.)
--header-key string       # Header name for signature/API key
--webhook-secret string   # Secret for HMAC verification
--auth-key string         # API key for API_KEY auth type
--auth-username string    # Username for BASIC auth type
--auth-password string    # Password for BASIC auth type
--allowed-methods string  # Comma-separated HTTP methods (GET,POST,PUT,DELETE)
--custom-response-status integer   # Custom response status code
--custom-response-body string      # Custom response body
--custom-response-headers string   # Custom response headers (key=value,key2=value2)

# Source update command parameters
<source-id>          # Required positional argument for source ID
--name string         # Update source name
--description string  # Update source description
# Plus any type-specific parameters listed above

# Source upsert command parameters (create or update by name)
--name string         # Required: Source name (used for matching existing)
--type string         # Required: Source type
# Plus any type-specific parameters listed above

# Source delete command parameters
<source-id>          # Required positional argument for source ID
--force              # Force delete without confirmation (boolean flag)

# Source enable/disable command parameters
<source-id>          # Required positional argument for source ID
```

**Type Validation Rules:**
- **webhook_secret_key types**: STRIPE, GITHUB, SHOPIFY, SLACK, TWILIO, SQUARE, WOOCOMMERCE, TEBEX, MAILCHIMP, PADDLE, TREEZOR, PRAXIS, CUSTOMERIO, EXACT_ONLINE, FACEBOOK, WHATSAPP, REPLICATE, TIKTOK, FISERV, VERCEL_LOG_DRAINS, etc.
- **webhook_id types**: PAYPAL (uses webhook_id instead of webhook_secret)
- **api_key types**: GITLAB, OKTA, MERAKI, CLOUDSIGNAL, etc.
- **public_key types**: BRIDGE, FIREBLOCKS, DISCORD, TELNYX, etc.
- **basic_auth types**: POSTMARK, PIPEDRIVE, etc.
- **token types**: RING_CENTRAL, etc.
- **complex_auth types**: EBAY (5 fields), TIKTOK_SHOP (2 fields)
- **minimal_config types**: AWS_SNS (no additional auth required)

🚧 **PLANNED FUNCTIONALITY** - Not yet implemented

Sources represent the webhook providers that send webhooks to Hookdeck. The API supports 96+ provider types with specific authentication requirements.

### List sources
```bash
# List all sources
hookdeck source list

# Filter by name pattern
hookdeck source list --name "stripe*"

# Filter by type (supports 80+ types)
hookdeck source list --type STRIPE

# Include disabled sources
hookdeck source list --disabled

# Limit results
hookdeck source list --limit 50

# Combined filtering
hookdeck source list --name "*prod*" --type GITHUB --limit 25
```

### Count sources
```bash
# Count all sources
hookdeck source count

# Count with filters
hookdeck source count --name "*stripe*" --disabled
```

### Get source details
```bash
# Get source by ID
hookdeck source get <source-id>

# Include authentication configuration
hookdeck source get <source-id> --include config.auth
```

### Create a source

#### Interactive creation
```bash
# Create with interactive prompts
hookdeck source create
```

#### Platform-specific sources (80+ supported types)

##### Payment Platforms
```bash
# Stripe - Payment webhooks
hookdeck source create --name "stripe-prod" --type STRIPE --webhook-secret "whsec_1a2b3c..."

# PayPal - Payment events (uses webhook_id not webhook_secret)
hookdeck source create --name "paypal-prod" --type PAYPAL --webhook-id "webhook_id_value"

# Square - POS and payment events
hookdeck source create --name "square-webhooks" --type SQUARE --webhook-secret "webhook_secret"
```

##### Repository and CI/CD
```bash
# GitHub - Repository webhooks
hookdeck source create --name "github-repo" --type GITHUB --webhook-secret "github_secret"

# GitLab - Repository and CI webhooks
hookdeck source create --name "gitlab-project" --type GITLAB --api-key "gitlab_token"

# Bitbucket - Repository events
hookdeck source create --name "bitbucket-repo" --type BITBUCKET --webhook-secret "webhook_secret"
```

##### E-commerce Platforms
```bash
# Shopify - Store webhooks
hookdeck source create --name "shopify-store" --type SHOPIFY --webhook-secret "shopify_secret"

# WooCommerce - WordPress e-commerce
hookdeck source create --name "woocommerce-store" --type WOOCOMMERCE --webhook-secret "webhook_secret"

# Magento - Enterprise e-commerce
hookdeck source create --name "magento-store" --type MAGENTO --webhook-secret "webhook_secret"
```

##### Communication Platforms
```bash
# Slack - Workspace events
hookdeck source create --name "slack-workspace" --type SLACK --webhook-secret "slack_signing_secret"

# Twilio - SMS and voice webhooks
hookdeck source create --name "twilio-sms" --type TWILIO --webhook-secret "twilio_auth_token"

# Discord - Bot interactions
hookdeck source create --name "discord-bot" --type DISCORD --public-key "discord_public_key"

# Teams - Microsoft Teams webhooks
hookdeck source create --name "teams-notifications" --type TEAMS --webhook-secret "teams_secret"
```

##### Cloud Services
```bash
# AWS SNS - Cloud notifications
hookdeck source create --name "aws-sns" --type AWS_SNS

# Azure Event Grid - Azure events
hookdeck source create --name "azure-events" --type AZURE_EVENT_GRID --webhook-secret "webhook_secret"

# Google Cloud Pub/Sub - GCP events
hookdeck source create --name "gcp-pubsub" --type GOOGLE_CLOUD_PUBSUB --webhook-secret "webhook_secret"
```

##### CRM and Marketing
```bash
# Salesforce - CRM events
hookdeck source create --name "salesforce-crm" --type SALESFORCE --webhook-secret "salesforce_secret"

# HubSpot - Marketing automation
hookdeck source create --name "hubspot-marketing" --type HUBSPOT --webhook-secret "hubspot_secret"

# Mailchimp - Email marketing
hookdeck source create --name "mailchimp-campaigns" --type MAILCHIMP --webhook-secret "mailchimp_secret"
```

##### Authentication and Identity
```bash
# Auth0 - Identity events
hookdeck source create --name "auth0-identity" --type AUTH0 --webhook-secret "auth0_secret"

# Okta - Identity management
hookdeck source create --name "okta-identity" --type OKTA --api-key "okta_api_key"

# Firebase Auth - Authentication events
hookdeck source create --name "firebase-auth" --type FIREBASE_AUTH --webhook-secret "firebase_secret"
```

##### Complex Authentication Examples
```bash
# eBay - Multi-field authentication
hookdeck source create --name "ebay-marketplace" --type EBAY \
  --environment PRODUCTION \
  --dev-id "dev_id" \
  --client-id "client_id" \
  --client-secret "client_secret" \
  --verification-token "verification_token"

# TikTok Shop - Multi-key authentication
hookdeck source create --name "tiktok-shop" --type TIKTOK_SHOP \
  --webhook-secret "webhook_secret" \
  --app-key "app_key"

# Custom HTTP with HMAC authentication
hookdeck source create --name "custom-api" --type HTTP \
  --auth-type HMAC \
  --algorithm sha256 \
  --encoding hex \
  --header-key "X-Signature" \
  --webhook-secret "hmac_secret"
```

### Update a source
```bash
# Update name and description
hookdeck source update <source-id> --name "new-name" --description "Updated description"

# Update webhook secret
hookdeck source update <source-id> --webhook-secret "new_secret"

# Update type-specific configuration
hookdeck source update <source-id> --api-key "new_api_key"
```

### Upsert a source (create or update by name)
```bash
# Create or update source by name
hookdeck source upsert --name "stripe-prod" --type STRIPE --webhook-secret "new_secret"
```

### Delete a source
```bash
# Delete source (with confirmation)
hookdeck source delete <source-id>

# Force delete without confirmation
hookdeck source delete <source-id> --force
```

### Enable/Disable sources
```bash
# Enable source
hookdeck source enable <source-id>

# Disable source
hookdeck source disable <source-id>
```

## Destinations

**All Parameters:**
```bash
# Destination list command parameters
--name string         # Filter by name pattern (supports wildcards)
--type string         # Filter by destination type (HTTP, CLI, MOCK_API)
--disabled           # Include disabled destinations (boolean flag)
--limit integer      # Limit number of results (default varies)

# Destination count command parameters
--name string         # Filter by name pattern
--disabled           # Include disabled destinations (boolean flag)

# Destination get command parameters
<destination-id>     # Required positional argument for destination ID
--include string     # Include additional data (e.g., "config.auth")

# Destination create command parameters
--name string         # Required: Destination name
--type string         # Optional: Destination type (HTTP, CLI, MOCK_API) - defaults to HTTP
--description string  # Optional: Destination description

# Type-specific parameters for destination create/update/upsert:
# When --type=HTTP (default):
--url string              # Required: Destination URL
--auth-type string        # Authentication type (BEARER_TOKEN, BASIC_AUTH, API_KEY, OAUTH2_CLIENT_CREDENTIALS)
--auth-token string       # Bearer token for BEARER_TOKEN auth
--auth-username string    # Username for BASIC_AUTH
--auth-password string    # Password for BASIC_AUTH
--auth-key string         # API key for API_KEY auth
--auth-header string      # Header name for API_KEY auth (e.g., "X-API-Key")
--auth-server string      # OAuth2 token server URL for OAUTH2_CLIENT_CREDENTIALS
--client-id string        # OAuth2 client ID
--client-secret string    # OAuth2 client secret
--headers string          # Custom headers (key=value,key2=value2)

# When --type=CLI:
--path string             # Optional: Path for CLI destination

# When --type=MOCK_API:
# (No additional type-specific parameters required)

# Destination update command parameters
<destination-id>     # Required positional argument for destination ID
--name string         # Update destination name
--description string  # Update destination description
--url string          # Update destination URL (for HTTP type)
# Plus any type-specific auth parameters listed above

# Destination upsert command parameters (create or update by name)
--name string         # Required: Destination name (used for matching existing)
--type string         # Optional: Destination type
# Plus any type-specific parameters listed above

# Destination delete command parameters
<destination-id>     # Required positional argument for destination ID
--force              # Force delete without confirmation (boolean flag)

# Destination enable/disable command parameters
<destination-id>     # Required positional argument for destination ID
```

**Type Validation Rules:**
- **HTTP destinations**: Require `--url`, support all authentication types
- **CLI destinations**: No URL required, optional `--path` parameter
- **MOCK_API destinations**: No additional parameters required, used for testing

**Authentication Type Combinations:**
- **BEARER_TOKEN**: Requires `--auth-token`
- **BASIC_AUTH**: Requires `--auth-username` and `--auth-password`
- **API_KEY**: Requires `--auth-key` and `--auth-header`
- **OAUTH2_CLIENT_CREDENTIALS**: Requires `--auth-server`, `--client-id`, and `--client-secret`

🚧 **PLANNED FUNCTIONALITY** - Not yet implemented

Destinations are the endpoints where webhooks are delivered.

### List destinations
```bash
# List all destinations
hookdeck destination list

# Filter by name pattern
hookdeck destination list --name "api*"

# Filter by type
hookdeck destination list --type HTTP

# Include disabled destinations
hookdeck destination list --disabled

# Limit results
hookdeck destination list --limit 50
```

### Count destinations
```bash
# Count all destinations
hookdeck destination count

# Count with filters
hookdeck destination count --name "*prod*" --disabled
```

### Get destination details
```bash
# Get destination by ID
hookdeck destination get <destination-id>

# Include authentication configuration
hookdeck destination get <destination-id> --include-auth
```

### Create a destination
```bash
# Create with interactive prompts
hookdeck destination create

# HTTP destination with URL
hookdeck destination create --name "my-api" --type HTTP --url "https://api.example.com/webhooks"

# CLI destination for local development
hookdeck destination create --name "local-dev" --type CLI

# Mock API destination for testing
hookdeck destination create --name "test-mock" --type MOCK_API

# HTTP with bearer token authentication
hookdeck destination create --name "secure-api" --type HTTP \
  --url "https://api.example.com/webhooks" \
  --auth-type BEARER_TOKEN \
  --auth-token "your_token"

# HTTP with basic authentication
hookdeck destination create --name "basic-auth-api" --type HTTP \
  --url "https://api.example.com/webhooks" \
  --auth-type BASIC_AUTH \
  --auth-username "api_user" \
  --auth-password "secure_password"

# HTTP with API key authentication
hookdeck destination create --name "api-key-endpoint" --type HTTP \
  --url "https://api.example.com/webhooks" \
  --auth-type API_KEY \
  --auth-key "your_api_key" \
  --auth-header "X-API-Key"

# HTTP with custom headers
hookdeck destination create --name "custom-headers-api" --type HTTP \
  --url "https://api.example.com/webhooks" \
  --headers "Content-Type=application/json,X-Custom-Header=value"

# HTTP with OAuth2 client credentials
hookdeck destination create --name "oauth2-api" --type HTTP \
  --url "https://api.example.com/webhooks" \
  --auth-type OAUTH2_CLIENT_CREDENTIALS \
  --auth-server "https://auth.example.com/token" \
  --client-id "your_client_id" \
  --client-secret "your_client_secret"
```

### Update a destination
```bash
# Update name and URL
hookdeck destination update <destination-id> --name "new-name" --url "https://new-api.example.com"

# Update authentication
hookdeck destination update <destination-id> --auth-token "new_token"
```

### Upsert a destination (create or update by name)
```bash
# Create or update destination by name
hookdeck destination upsert --name "my-api" --type HTTP --url "https://api.example.com"
```

### Delete a destination
```bash
# Delete destination (with confirmation)
hookdeck destination delete <destination-id>

# Force delete without confirmation  
hookdeck destination delete <destination-id> --force
```

### Enable/Disable destinations
```bash
# Enable destination
hookdeck destination enable <destination-id>

# Disable destination
hookdeck destination disable <destination-id>
```

## Connections

✅ **Fully Implemented** - Connection management provides comprehensive CRUD operations, lifecycle management, authentication, and rule configuration.

**Available Commands:**
- `connection create` - Create connections with inline source/destination creation
- `connection list` - List connections with filtering options
- `connection get` - Get detailed connection information
- `connection upsert` - Idempotent create or update operations
- `connection delete` - Delete connections with confirmation
- `connection enable/disable` - Control connection state
- `connection pause/unpause` - Pause/resume event processing

**Implementation Status:**
- ✅ Full CRUD operations
- ✅ Inline resource creation with authentication
- ✅ All 5 rule types (retry, filter, transform, delay, deduplicate)
- ✅ Rate limiting configuration
- ✅ Lifecycle management
- ✅ Idempotent upsert with dry-run
- ✅ `--output json` flag for JSON output (create, list, get, upsert commands)
- ❌ Bulk operations (planned)
- ❌ Count command (planned)

### List Connections

```bash
# List all connections
hookdeck connection list

# Filter by source ID
hookdeck connection list --source-id src_abc123

# Filter by destination ID
hookdeck connection list --destination-id des_xyz789

# Filter by connection name
hookdeck connection list --name "production-connection"

# Include disabled connections
hookdeck connection list --disabled

# Combine filters
hookdeck connection list --source-id src_abc123 --disabled

# Limit results
hookdeck connection list --limit 50

# Output as JSON
hookdeck connection list --output json
```

**Available Flags:**
- `--name <string>` - Filter by connection name
- `--source-id <string>` - Filter by source ID
- `--destination-id <string>` - Filter by destination ID
- `--disabled` - Include disabled connections
- `--limit <int>` - Limit number of results (default: 100)
- `--output json` - Output in JSON format

### Get Connection

```bash
# Get by ID
hookdeck connection get conn_abc123

# Get by name
hookdeck connection get "my-connection"

# Get as JSON
hookdeck connection get conn_abc123 --output json

# Include destination authentication credentials
hookdeck connection get conn_abc123 --include-destination-auth --output json
```

**Flags:**

- `--output json` - Output in JSON format
- `--include-destination-auth` - Include destination authentication credentials in the response (fetches via GET /destinations/{id}?include=config.auth)

### Create Connection

Create a new connection with inline source/destination creation or by referencing existing resources.

#### Basic Examples

**1. Basic HTTP Connection**
```bash
hookdeck connection create \
  --source-name "webhook-receiver" \
  --source-type HTTP \
  --destination-name "api-endpoint" \
  --destination-type HTTP \
  --destination-url "https://api.example.com/webhooks"
```

**2. Using Existing Resources**
```bash
hookdeck connection create \
  --source "existing-source-name" \
  --destination "existing-dest-name" \
  --name "new-connection" \
  --description "Connects existing resources"
```

#### Authentication Examples

**3. Stripe with Webhook Secret**
```bash
hookdeck connection create \
  --source-name "stripe-prod" \
  --source-type STRIPE \
  --source-webhook-secret "whsec_abc123xyz" \
  --destination-name "payment-processor" \
  --destination-type HTTP \
  --destination-url "https://api.example.com/stripe"
```

**4. Destination with Hookdeck Signature (Default)**
```bash
# Hookdeck automatically signs outgoing webhooks - no configuration needed
hookdeck connection create \
  --source-name "stripe-webhooks" \
  --source-type STRIPE \
  --source-webhook-secret "whsec_stripe_secret" \
  --destination-name "api-with-verification" \
  --destination-type HTTP \
  --destination-url "https://api.example.com/webhook" \
  --destination-auth-method hookdeck
```
*Note: Hookdeck Signature authentication is the default. Hookdeck automatically signs all outgoing webhooks with a signature that can be verified using Hookdeck's verification libraries. No webhook secret needs to be configured.*

**5. Destination with Bearer Token**
```bash
hookdeck connection create \
  --source-name "github-webhooks" \
  --source-type GITHUB \
  --source-webhook-secret "ghp_secret123" \
  --destination-name "ci-system" \
  --destination-type HTTP \
  --destination-url "https://ci.example.com/webhook" \
  --destination-auth-method bearer \
  --destination-bearer-token "bearer_token_xyz"
```

**6. Source with Custom Response and Allowed HTTP Methods**
```bash
hookdeck connection create \
  --source-name "api-webhooks" \
  --source-type WEBHOOK \
  --source-allowed-http-methods "POST,PUT,PATCH" \
  --source-custom-response-content-type "json" \
  --source-custom-response-body '{"status":"received","timestamp":"2024-01-01T00:00:00Z"}' \
  --destination-name "webhook-handler" \
  --destination-type HTTP \
  --destination-url "https://api.example.com/webhooks"
```

#### Rule Configuration Examples

**7. Retry Rules**
```bash
hookdeck connection create \
  --source-name "payment-webhooks" \
  --source-type STRIPE \
  --destination-name "payment-api" \
  --destination-type HTTP \
  --destination-url "https://api.example.com/payments" \
  --rule-retry-strategy exponential \
  --rule-retry-count 5 \
  --rule-retry-interval 60000
```

**8. Filter Rules**
```bash
hookdeck connection create \
  --source-name "events" \
  --source-type HTTP \
  --destination-name "processor" \
  --destination-type HTTP \
  --destination-url "https://api.example.com/process" \
  --rule-filter-body '{"event_type":"payment.succeeded"}'
```

**9. All Rule Types Combined**
```bash
hookdeck connection create \
  --source-name "shopify-webhooks" \
  --source-type SHOPIFY \
  --destination-name "order-processor" \
  --destination-type HTTP \
  --destination-url "https://api.example.com/orders" \
  --rule-filter-body '{"type":"order"}' \
  --rule-retry-strategy exponential \
  --rule-retry-count 3 \
  --rule-retry-interval 30000 \
  --rule-transform-name "order-transformer" \
  --rule-delay 5000
```

**10. Rate Limiting**
```bash
hookdeck connection create \
  --source-name "high-volume-source" \
  --source-type HTTP \
  --destination-name "rate-limited-api" \
  --destination-type HTTP \
  --destination-url "https://api.example.com/endpoint" \
  --destination-rate-limit 100 \
  --destination-rate-limit-period minute
```

**11. GCP Service Account Authentication**
```bash
hookdeck connection create \
  --source-name "webhooks" \
  --source-type HTTP \
  --destination-name "gcp-cloud-function" \
  --destination-type HTTP \
  --destination-url "https://us-central1-project-id.cloudfunctions.net/function" \
  --destination-auth-method gcp \
  --destination-gcp-service-account-key '{"type":"service_account","project_id":"project-id","private_key_id":"key-id","private_key":"-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----\n","client_email":"service-account@project-id.iam.gserviceaccount.com"}' \
  --destination-gcp-scope "https://www.googleapis.com/auth/cloud-platform"
```

#### Available Flags

**Connection Configuration:**
- `--name <string>` - Connection name (optional, auto-generated if not provided)
- `--description <string>` - Connection description

**Source (Inline Creation):**
- `--source-name <string>` - Source name (required for inline)
- `--source-type <type>` - Source type: `STRIPE`, `GITHUB`, `SHOPIFY`, `HTTP`, etc.
- `--source-description <string>` - Source description
- `--source-webhook-secret <secret>` - Webhook verification secret
- `--source-api-key <key>` - API key authentication
- `--source-basic-auth-user <user>` - Basic auth username
- `--source-basic-auth-pass <pass>` - Basic auth password
- `--source-hmac-secret <secret>` - HMAC secret
- `--source-hmac-algo <algo>` - HMAC algorithm
- `--source-allowed-http-methods <methods>` - Comma-separated list of allowed HTTP methods: `GET`, `POST`, `PUT`, `PATCH`, `DELETE`
- `--source-custom-response-content-type <type>` - Custom response content type: `json`, `text`, `xml`
- `--source-custom-response-body <body>` - Custom response body (max 1000 chars)
- `--source-config <json>` - JSON authentication config
- `--source-config-file <path>` - Path to JSON config file

**Destination (Inline Creation):**
- `--destination-name <string>` - Destination name (required for inline)
- `--destination-type <type>` - Destination type: `HTTP`, `MOCK`, etc.
- `--destination-description <string>` - Destination description
- `--destination-url <url>` - Destination URL (required for HTTP)
- `--destination-cli-path <path>` - CLI path (default: `/`)
- `--destination-path-forwarding-disabled <true|false>` - Disable path forwarding for HTTP destinations (default: false)
- `--destination-http-method <method>` - HTTP method for HTTP destinations: `GET`, `POST`, `PUT`, `PATCH`, `DELETE`
- `--destination-auth-method <method>` - Authentication method: `hookdeck`, `bearer`, `basic`, `api_key`, `custom_signature`, `oauth2_client_credentials`, `oauth2_authorization_code`, `aws`, `gcp`
- `--destination-rate-limit <number>` - Rate limit (requests per period)
- `--destination-rate-limit-period <period>` - Period: `second`, `minute`, `hour`, `day`, `month`, `year`

**Destination Authentication Options:**

*Hookdeck Signature (default):*
- `--destination-auth-method hookdeck` - Use Hookdeck signature authentication

*Bearer Token:*
- `--destination-auth-method bearer`
- `--destination-bearer-token <token>` - Bearer token

*Basic Authentication:*
- `--destination-auth-method basic`
- `--destination-basic-auth-user <user>` - Username
- `--destination-basic-auth-pass <pass>` - Password

*API Key:*
- `--destination-auth-method api_key`
- `--destination-api-key <key>` - API key
- `--destination-api-key-header <name>` - Key/header name
- `--destination-api-key-to <location>` - Location: `header` or `query` (default: `header`)

*Custom Signature (HMAC):*
- `--destination-auth-method custom_signature`
- `--destination-custom-signature-key <name>` - Key/header name
- `--destination-custom-signature-secret <secret>` - Signing secret

*OAuth2 Client Credentials:*
- `--destination-auth-method oauth2_client_credentials`
- `--destination-oauth2-auth-server <url>` - Authorization server URL
- `--destination-oauth2-client-id <id>` - Client ID
- `--destination-oauth2-client-secret <secret>` - Client secret
- `--destination-oauth2-scopes <scopes>` - Scopes (comma-separated, optional)
- `--destination-oauth2-auth-type <type>` - Auth type: `basic`, `bearer`, or `x-www-form-urlencoded` (default: `basic`)

*OAuth2 Authorization Code:*
- `--destination-auth-method oauth2_authorization_code`
- `--destination-oauth2-auth-server <url>` - Authorization server URL
- `--destination-oauth2-client-id <id>` - Client ID
- `--destination-oauth2-client-secret <secret>` - Client secret
- `--destination-oauth2-refresh-token <token>` - Refresh token
- `--destination-oauth2-scopes <scopes>` - Scopes (comma-separated, optional)

*AWS Signature:*
- `--destination-auth-method aws`
- `--destination-aws-access-key-id <id>` - AWS access key ID
- `--destination-aws-secret-access-key <key>` - AWS secret access key
- `--destination-aws-region <region>` - AWS region
- `--destination-aws-service <service>` - AWS service name

*GCP Service Account:*
- `--destination-auth-method gcp`
- `--destination-gcp-service-account-key <json>` - GCP service account key JSON
- `--destination-gcp-scope <scope>` - GCP scope (optional)

**Rules - Retry:**
- `--rule-retry-strategy <strategy>` - Strategy: `linear`, `exponential`
- `--rule-retry-count <number>` - Number of retry attempts (1-20)
- `--rule-retry-interval <ms>` - Interval in milliseconds
- `--rule-retry-response-status-codes <codes>` - Comma-separated status codes

**Rules - Filter:**
- `--rule-filter-body <json>` - Body filter (JSON format)
- `--rule-filter-headers <json>` - Header filter (JSON format)
- `--rule-filter-path <json>` - Path filter (JSON format)
- `--rule-filter-query <json>` - Query parameter filter (JSON format)

**Rules - Transform:**
- `--rule-transform-name <name>` - Name or ID of transformation

**Rules - Delay:**
- `--rule-delay <ms>` - Delay in milliseconds

**Rules - Deduplicate:**
- `--rule-deduplicate-window <seconds>` - Deduplication window
- `--rule-deduplicate-include-fields <fields>` - Comma-separated fields to include
- `--rule-deduplicate-exclude-fields <fields>` - Comma-separated fields to exclude

**Reference Existing Resources:**
- `--source <id-or-name>` - Use existing source
- `--destination <id-or-name>` - Use existing destination

**JSON Fallbacks:**
- `--rules <json>` - Complete rules array (JSON string)
- `--rules-file <path>` - Path to JSON file with rules

### Upsert Connection

Create or update a connection idempotently based on the connection name. Perfect for CI/CD and infrastructure-as-code workflows.

```bash
# Create if doesn't exist
hookdeck connection upsert my-connection \
  --source-name "stripe-prod" \
  --source-type STRIPE \
  --destination-name "api-prod" \
  --destination-type HTTP \
  --destination-url "https://api.example.com"

# Update existing (partial update)
hookdeck connection upsert my-connection \
  --description "Updated description" \
  --rule-retry-count 5

# Preview changes without applying
hookdeck connection upsert my-connection \
  --description "New description" \
  --dry-run
```

**Behavior:**
- If connection doesn't exist → Creates it (source/destination required)
- If connection exists → Updates it (all flags optional, partial updates)
- Supports all same flags as `connection create`
- Add `--dry-run` to preview CREATE or UPDATE operation

**Use Cases:**
- CI/CD pipelines
- Infrastructure-as-code
- Idempotent configuration management

### Delete Connection

```bash
# Delete with confirmation prompt
hookdeck connection delete conn_abc123

# Delete by name
hookdeck connection delete "my-connection"

# Skip confirmation
hookdeck connection delete conn_abc123 --force
```

### Lifecycle Management

Control connection state and processing behavior.

```bash
# Enable/Disable (stop receiving events)
hookdeck connection disable conn_abc123
hookdeck connection enable conn_abc123

# Pause/Unpause (queue events without forwarding)
hookdeck connection pause conn_abc123
hookdeck connection unpause conn_abc123
```

**State Differences:**
- **Disabled**: Connection stops receiving events entirely
- **Paused**: Connection queues events but doesn't forward them

### Implementation Notes

**Fully Implemented (✅):**
- Full CRUD operations (create, list, get, upsert, delete)
- Inline resource creation with authentication
- All 5 rule types (retry, filter, transform, delay, deduplicate)
- Rate limiting configuration
- Lifecycle management (enable, disable, pause, unpause)
- Idempotent upsert with dry-run support
- 21 acceptance tests, all passing

**Not Implemented (❌):**
- `connection count` command (optional)
- Bulk operations (planned)
- Connection cloning (optional)

**See Also:**
- [Connection Management Status](.plans/connection-management-status.md)

## Transformations

**All Parameters:**
```bash
# Transformation list command parameters
--name string         # Filter by name pattern (supports wildcards)
--limit integer      # Limit number of results (default varies)

# Transformation count command parameters
--name string         # Filter by name pattern

# Transformation get command parameters
<transformation-id>  # Required positional argument for transformation ID

# Transformation create command parameters
--name string         # Required: Transformation name
--code string         # Required: JavaScript code for the transformation
--description string  # Optional: Transformation description
--env string          # Optional: Environment variables (KEY=value,KEY2=value2)

# Transformation update command parameters
<transformation-id>  # Required positional argument for transformation ID
--name string         # Update transformation name
--code string         # Update JavaScript code
--description string  # Update transformation description
--env string          # Update environment variables (KEY=value,KEY2=value2)

# Transformation upsert command parameters (create or update by name)
--name string         # Required: Transformation name (used for matching existing)
--code string         # Required: JavaScript code
--description string  # Optional: Transformation description
--env string          # Optional: Environment variables

# Transformation delete command parameters
<transformation-id>  # Required positional argument for transformation ID
--force              # Force delete without confirmation (boolean flag)

# Transformation run command parameters (testing)
--code string         # Required: JavaScript code to test
--request string      # Required: Request JSON for testing

# Transformation executions command parameters
<transformation-id>  # Required positional argument for transformation ID
--limit integer      # Limit number of execution results

# Transformation execution command parameters (get single execution)
<transformation-id>  # Required positional argument for transformation ID
<execution-id>       # Required positional argument for execution ID
```

**Environment Variables Format:**
- Use comma-separated key=value pairs: `KEY1=value1,KEY2=value2`
- Supports debugging flags: `DEBUG=true,LOG_LEVEL=info`
- Can reference external services: `API_URL=https://api.example.com,API_KEY=secret`

🚧 **PLANNED FUNCTIONALITY** - Not yet implemented

Transformations allow you to modify webhook payloads using JavaScript.

### List transformations
```bash
# List all transformations
hookdeck transformation list

# Filter by name pattern
hookdeck transformation list --name "*stripe*"

# Limit results
hookdeck transformation list --limit 50
```

### Count transformations
```bash
# Count all transformations
hookdeck transformation count

# Count with filters
hookdeck transformation count --name "*formatter*"
```

### Get transformation details
```bash
# Get transformation by ID
hookdeck transformation get <transformation-id>
```

### Create a transformation
```bash
# Create with interactive prompts
hookdeck transformation create

# Create with inline code
hookdeck transformation create --name "stripe-formatter" \
  --code 'export default function(request) {
    request.body.processed_at = new Date().toISOString();
    request.body.webhook_source = "stripe";
    return request;
  }'

# Create with environment variables
hookdeck transformation create --name "api-enricher" \
  --code 'export default function(request) {
    const { API_KEY } = process.env;
    request.headers["X-API-Key"] = API_KEY;
    return request;
  }' \
  --env "API_KEY=your_key,DEBUG=true"

# Create with description
hookdeck transformation create --name "payment-processor" \
  --description "Processes payment webhooks and adds metadata" \
  --code 'export default function(request) {
    if (request.body.type?.includes("payment")) {
      request.body.category = "payment";
      request.body.priority = "high";
    }
    return request;
  }'
```

### Update a transformation
```bash
# Update transformation code
hookdeck transformation update <transformation-id> \
  --code 'export default function(request) { /* updated code */ return request; }'

# Update name and description
hookdeck transformation update <transformation-id> --name "new-name" --description "Updated description"

# Update environment variables
hookdeck transformation update <transformation-id> --env "API_KEY=new_key,DEBUG=false"
```

### Upsert a transformation (create or update by name)
```bash
# Create or update transformation by name
hookdeck transformation upsert --name "stripe-formatter" \
  --code 'export default function(request) { return request; }'
```

### Delete a transformation
```bash
# Delete transformation (with confirmation)
hookdeck transformation delete <transformation-id>

# Force delete without confirmation
hookdeck transformation delete <transformation-id> --force
```

### Test a transformation
```bash
# Test with sample request JSON
hookdeck transformation run --code 'export default function(request) { return request; }' \
  --request '{"headers": {"content-type": "application/json"}, "body": {"test": true}}'
```

### Get transformation executions
```bash
# List executions for a transformation
hookdeck transformation executions <transformation-id> --limit 50

# Get specific execution details
hookdeck transformation execution <transformation-id> <execution-id>
```

## Events

✅ **Current** — Under `hookdeck gateway event` (alias `events`).

**All Parameters:**
```bash
# Event list command parameters
--id string              # Filter by event ID(s) (comma-separated)
--connection-id string   # Filter by connection ID
--source-id string       # Filter by source ID
--destination-id string  # Filter by destination ID
--status string          # Filter by status (SCHEDULED, QUEUED, HOLD, SUCCESSFUL, FAILED, CANCELLED)
--attempts string        # Filter by number of attempts (integer or operators)
--response-status string # Filter by HTTP response status (e.g. 200, 500)
--error-code string      # Filter by error code
--cli-id string          # Filter by CLI ID
--issue-id string        # Filter by issue ID
--created-after string   # Filter events created after (ISO date-time)
--created-before string  # Filter events created before (ISO date-time)
--successful-at-after string   # Filter by successful_at after (ISO date-time)
--successful-at-before string # Filter by successful_at before (ISO date-time)
--last-attempt-at-after string  # Filter by last_attempt_at after (ISO date-time)
--last-attempt-at-before string # Filter by last_attempt_at before (ISO date-time)
--headers string         # Filter by headers (JSON string)
--body string            # Filter by body (JSON string)
--path string            # Filter by path
--parsed-query string    # Filter by parsed query (JSON string)
--order-by string        # Sort key (e.g. created_at)
--dir string             # Sort direction: asc, desc
--limit integer          # Limit number of results (default 100)
--next string            # Pagination cursor for next page
--prev string            # Pagination cursor for previous page
--output string          # Output format (json)

# Event get / raw-body / retry / cancel / mute
<event-id>             # Required positional argument for event ID
```

### List events
```bash
hookdeck gateway event list
hookdeck gateway event list --connection-id <connection-id>
hookdeck gateway event list --source-id <source-id> --status FAILED --limit 50
hookdeck gateway event list --id evt_xxx --created-after 2024-01-01T00:00:00Z
```

### Get event details
```bash
hookdeck gateway event get <event-id>
hookdeck gateway event raw-body <event-id>
```

### Retry, cancel, mute
```bash
hookdeck gateway event retry <event-id>
hookdeck gateway event cancel <event-id>
hookdeck gateway event mute <event-id>
```

## Attempts

✅ **Current** — Under `hookdeck gateway attempt` (alias `attempts`). List requires `--event-id`.

**Parameters:**
```bash
# Attempt list (--event-id required)
--event-id string     # Filter by event ID (required)
--order-by string     # Sort key
--dir string          # Sort direction: asc, desc
--limit integer       # Limit number of results (default 100)
--next string         # Pagination cursor for next page
--prev string         # Pagination cursor for previous page
--output string       # Output format (json)

# Attempt get
<attempt-id>          # Required positional argument for attempt ID
```

### List attempts
```bash
hookdeck gateway attempt list --event-id <event-id>
hookdeck gateway attempt list --event-id evt_123 --limit 10 --order-by created_at --dir desc
```

### Get attempt details
```bash
hookdeck gateway attempt get <attempt-id>
```


## Issues

🚧 **PLANNED FUNCTIONALITY** - Not yet implemented

### List issues
```bash
# List all issues
hookdeck issue list

# Filter by status
hookdeck issue list --status ACTIVE
hookdeck issue list --status DISMISSED

# Filter by type
hookdeck issue list --type DELIVERY_ISSUE
hookdeck issue list --type TRANSFORMATION_ISSUE

# Limit results
hookdeck issue list --limit 100
```

### Count issues
```bash
# Count all issues
hookdeck issue count

# Count with filters
hookdeck issue count --status ACTIVE --type DELIVERY_ISSUE
```

### Get issue details
```bash
# Get issue by ID
hookdeck issue get <issue-id>
```

## Issue Triggers

**All Parameters:**
```bash
# Issue trigger list command parameters
--name string         # Filter by name pattern (supports wildcards)
--type string         # Filter by trigger type (delivery, transformation, backpressure)
--disabled           # Include disabled triggers (boolean flag)
--limit integer      # Limit number of results (default varies)

# Issue trigger get command parameters
<trigger-id>         # Required positional argument for trigger ID

# Issue trigger create command parameters
--name string         # Optional: Unique name for the trigger
--type string         # Required: Trigger type (delivery, transformation, backpressure)
--description string  # Optional: Trigger description

# Type-specific configuration parameters:
# When --type=delivery:
--strategy string     # Required: Strategy (first_attempt, final_attempt)
--connections string  # Required: Connection patterns or IDs (comma-separated or "*")

# When --type=transformation:
--log-level string    # Required: Log level (debug, info, warn, error, fatal)
--transformations string # Required: Transformation patterns or IDs (comma-separated or "*")

# When --type=backpressure:
--delay integer       # Required: Minimum delay in milliseconds (60000-86400000)
--destinations string # Required: Destination patterns or IDs (comma-separated or "*")

# Notification channel parameters (at least one required):
--email              # Enable email notifications (boolean flag)
--slack-channel string    # Slack channel name (e.g., "#alerts")
--pagerduty          # Enable PagerDuty notifications (boolean flag)
--opsgenie           # Enable Opsgenie notifications (boolean flag)

# Issue trigger update command parameters
<trigger-id>         # Required positional argument for trigger ID
--name string         # Update trigger name
--description string  # Update trigger description
# Plus any type-specific and notification parameters listed above

# Issue trigger upsert command parameters (create or update by name)
--name string         # Required: Trigger name (used for matching existing)
--type string         # Required: Trigger type
# Plus any type-specific and notification parameters listed above

# Issue trigger delete command parameters
<trigger-id>         # Required positional argument for trigger ID
--force              # Force delete without confirmation (boolean flag)

# Issue trigger enable/disable command parameters
<trigger-id>         # Required positional argument for trigger ID
```

**Type Validation Rules:**
- **delivery type**: Requires `--strategy` and `--connections`
  - `--strategy` values: `first_attempt`, `final_attempt`
  - `--connections` accepts: connection IDs, connection name patterns, or `"*"` for all
- **transformation type**: Requires `--log-level` and `--transformations`
  - `--log-level` values: `debug`, `info`, `warn`, `error`, `fatal`
  - `--transformations` accepts: transformation IDs, transformation name patterns, or `"*"` for all
- **backpressure type**: Requires `--delay` and `--destinations`
  - `--delay` range: 60000-86400000 milliseconds (1 minute to 1 day)
  - `--destinations` accepts: destination IDs, destination name patterns, or `"*"` for all

**Notification Channel Combinations:**
- Multiple notification channels can be enabled simultaneously
- `--email` is a boolean flag (no additional configuration)
- `--slack-channel` requires a channel name (e.g., "#alerts", "#monitoring")
- `--pagerduty` and `--opsgenie` are boolean flags requiring pre-configured integrations

🚧 **PLANNED FUNCTIONALITY** - Not yet implemented

Issue triggers automatically detect and create issues when specific conditions are met.

### List issue triggers
```bash
# List all issue triggers
hookdeck issue-trigger list

# Filter by name pattern
hookdeck issue-trigger list --name "*delivery*"

# Filter by type
hookdeck issue-trigger list --type delivery
hookdeck issue-trigger list --type transformation
hookdeck issue-trigger list --type backpressure

# Include disabled triggers
hookdeck issue-trigger list --disabled

# Limit results
hookdeck issue-trigger list --limit 50
```

### Get issue trigger details
```bash
# Get issue trigger by ID
hookdeck issue-trigger get <trigger-id>
```

### Create issue triggers

#### Delivery failure trigger
```bash
# Trigger on final delivery attempt failure
hookdeck issue-trigger create --type delivery \
  --name "delivery-failures" \
  --strategy final_attempt \
  --connections "conn1,conn2" \
  --email \
  --slack-channel "#alerts"

# Trigger on first delivery attempt failure
hookdeck issue-trigger create --type delivery \
  --name "immediate-delivery-alerts" \
  --strategy first_attempt \
  --connections "*" \
  --pagerduty
```

#### Transformation error trigger
```bash
# Trigger on transformation errors
hookdeck issue-trigger create --type transformation \
  --name "transformation-errors" \
  --log-level error \
  --transformations "*" \
  --email \
  --opsgenie

# Trigger on specific transformation debug logs
hookdeck issue-trigger create --type transformation \
  --name "debug-logs" \
  --log-level debug \
  --transformations "trans1,trans2" \
  --slack-channel "#debug"
```

#### Backpressure trigger
```bash
# Trigger on destination backpressure
hookdeck issue-trigger create --type backpressure \
  --name "backpressure-alert" \
  --delay 300000 \
  --destinations "*" \
  --email \
  --pagerduty
```

### Update issue trigger
```bash
# Update trigger name and description
hookdeck issue-trigger update <trigger-id> --name "new-name" --description "Updated description"

# Update notification channels
hookdeck issue-trigger update <trigger-id> --email --slack-channel "#new-alerts"

# Update type-specific configuration
hookdeck issue-trigger update <trigger-id> --strategy first_attempt --connections "new_conn"
```

### Upsert issue trigger (create or update by name)
```bash
# Create or update issue trigger by name
hookdeck issue-trigger upsert --name "delivery-failures" --type delivery --strategy final_attempt
```

### Delete issue trigger
```bash
# Delete issue trigger (with confirmation)
hookdeck issue-trigger delete <trigger-id>

# Force delete without confirmation
hookdeck issue-trigger delete <trigger-id> --force
```

### Enable/Disable issue triggers
```bash
# Enable issue trigger
hookdeck issue-trigger enable <trigger-id>

# Disable issue trigger
hookdeck issue-trigger disable <trigger-id>
```

## Bookmarks

**All Parameters:**
```bash
# Bookmark list command parameters
--name string         # Filter by name pattern (supports wildcards)
--connection-id string # Filter by connection ID
--label string        # Filter by label
--limit integer       # Limit number of results (default varies)

# Bookmark get command parameters
<bookmark-id>         # Required positional argument for bookmark ID

# Bookmark raw-body command parameters
<bookmark-id>         # Required positional argument for bookmark ID

# Bookmark create command parameters
--event-data-id string # Required: Event data ID to bookmark
--connection-id string  # Required: Connection ID
--label string         # Required: Label for categorization
--name string          # Optional: Bookmark name

# Bookmark update command parameters
<bookmark-id>         # Required positional argument for bookmark ID
--name string          # Update bookmark name
--label string         # Update bookmark label

# Bookmark delete command parameters
<bookmark-id>         # Required positional argument for bookmark ID
--force               # Force delete without confirmation (boolean flag)

# Bookmark trigger command parameters (replay)
<bookmark-id>         # Required positional argument for bookmark ID
```

🚧 **PLANNED FUNCTIONALITY** - Not yet implemented

Bookmarks allow you to save webhook payloads for testing and replay.

### List bookmarks
```bash
# List all bookmarks
hookdeck bookmark list

# Filter by name pattern
hookdeck bookmark list --name "*test*"

# Filter by connection ID
hookdeck bookmark list --connection-id <connection-id>

# Filter by label
hookdeck bookmark list --label test_data

# Limit results
hookdeck bookmark list --limit 50
```

### Get bookmark details
```bash
# Get bookmark by ID
hookdeck bookmark get <bookmark-id>

# Get bookmark raw body
hookdeck bookmark raw-body <bookmark-id>
```

### Create a bookmark
```bash
# Create bookmark from event
hookdeck bookmark create --event-data-id <event-data-id> \
  --connection-id <connection-id> \
  --label test_payload \
  --name "stripe-payment-test"
```

### Update a bookmark
```bash
# Update bookmark properties
hookdeck bookmark update <bookmark-id> --name "new-name" --label new_label
```

### Delete a bookmark
```bash
# Delete bookmark (with confirmation)
hookdeck bookmark delete <bookmark-id>

# Force delete without confirmation
hookdeck bookmark delete <bookmark-id> --force
```

### Trigger bookmark (replay)
```bash
# Trigger bookmark to replay webhook
hookdeck bookmark trigger <bookmark-id>
```

## Integrations

🚧 **PLANNED FUNCTIONALITY** - Not yet implemented

Integrations connect third-party services to your Hookdeck workspace.

### List integrations
```bash
# List all integrations
hookdeck integration list

# Limit results
hookdeck integration list --limit 50
```

### Get integration details
```bash
# Get integration by ID
hookdeck integration get <integration-id>
```

### Create an integration
```bash
# Create integration (provider-specific configuration required)
hookdeck integration create --provider PROVIDER_NAME
```

### Update an integration
```bash
# Update integration (provider-specific configuration)
hookdeck integration update <integration-id>
```

### Delete an integration
```bash
# Delete integration (with confirmation)
hookdeck integration delete <integration-id>

# Force delete without confirmation
hookdeck integration delete <integration-id> --force
```

### Attach/Detach sources
```bash
# Attach source to integration
hookdeck integration attach <integration-id> <source-id>

# Detach source from integration
hookdeck integration detach <integration-id> <source-id>
```

## Requests

✅ **Current** — Under `hookdeck gateway request` (alias `requests`).

**All Parameters:**
```bash
# Request list
--id string              # Filter by request ID(s) (comma-separated)
--source-id string       # Filter by source ID
--status string          # Filter by status
--verified string        # Filter by verified (true/false)
--rejection-cause string # Filter by rejection cause
--created-after string   # Filter requests created after (ISO date-time)
--created-before string  # Filter requests created before (ISO date-time)
--ingested-at-after string  # Filter by ingested_at after (ISO date-time)
--ingested-at-before string # Filter by ingested_at before (ISO date-time)
--headers string         # Filter by headers (JSON string)
--body string            # Filter by body (JSON string)
--path string            # Filter by path
--parsed-query string    # Filter by parsed query (JSON string)
--order-by string        # Sort key (e.g. created_at)
--dir string             # Sort direction: asc, desc
--limit integer          # Limit number of results (default 100)
--next string            # Pagination cursor for next page
--prev string            # Pagination cursor for previous page
--output string          # Output format (json)

# Request get / raw-body / retry
<request-id>             # Required positional argument for request ID

# Request retry
--connection-ids string  # Comma-separated connection IDs to retry (omit to retry all)

# Request events / ignored-events
<request-id>             # Required positional argument for request ID
--limit integer          # Limit number of results (default 100)
--next string            # Pagination cursor for next page
--prev string            # Pagination cursor for previous page
--output string          # Output format (json)
```

### List requests
```bash
hookdeck gateway request list
hookdeck gateway request list --source-id <source-id>
hookdeck gateway request list --verified true --rejection-cause INVALID_SIGNATURE
hookdeck gateway request list --created-after 2024-01-01T00:00:00Z --limit 100
```

### Get request details and raw body
```bash
hookdeck gateway request get <request-id>
hookdeck gateway request raw-body <request-id>
```

### Retry request
```bash
hookdeck gateway request retry <request-id>
hookdeck gateway request retry <request-id> --connection-ids web_1,web_2
```

### List request events and ignored events
```bash
hookdeck gateway request events <request-id> --limit 50
hookdeck gateway request events <request-id> --limit 50 --next <cursor>
hookdeck gateway request ignored-events <request-id> --limit 50 --prev <cursor>
```

## Bulk Operations

**All Parameters:**
```bash
# Bulk event-retry command parameters
--limit integer       # Limit number of results for list operations
--query string        # JSON query for filtering resources to retry
<operation-id>        # Required positional argument for get/cancel operations

# Bulk request-retry command parameters
--limit integer       # Limit number of results for list operations
--query string        # JSON query for filtering resources to retry
<operation-id>        # Required positional argument for get/cancel operations

# Bulk ignored-event-retry command parameters
--limit integer       # Limit number of results for list operations
--query string        # JSON query for filtering resources to retry
<operation-id>        # Required positional argument for get/cancel operations
```

**Query JSON Format Examples:** (API uses `webhook_id` for connection ID)
- Event retry: `'{"status": "FAILED", "webhook_id": "conn_123"}'`
- Request retry: `'{"verified": false, "source_id": "src_123"}'`
- Ignored event retry: `'{"webhook_id": "conn_123"}'`

**Operations Available:**
- `list` - List bulk operations
- `create` - Create new bulk operation
- `plan` - Dry run to see what would be affected
- `get` - Get operation details
- `cancel` - Cancel running operation

🚧 **PLANNED FUNCTIONALITY** - Not yet implemented

Bulk operations allow you to perform actions on multiple resources at once.

### Event Bulk Retry
```bash
# List bulk event retry operations
hookdeck bulk event-retry list --limit 50

# Create bulk event retry operation
hookdeck bulk event-retry create --query '{"status": "FAILED", "webhook_id": "conn_123"}'

# Plan bulk event retry (dry run)
hookdeck bulk event-retry plan --query '{"status": "FAILED"}'

# Get bulk operation details
hookdeck bulk event-retry get <operation-id>

# Cancel bulk operation
hookdeck bulk event-retry cancel <operation-id>
```

### Request Bulk Retry
```bash
# List bulk request retry operations
hookdeck bulk request-retry list --limit 50

# Create bulk request retry operation
hookdeck bulk request-retry create --query '{"verified": false, "source_id": "src_123"}'

# Plan bulk request retry (dry run)
hookdeck bulk request-retry plan --query '{"verified": false}'

# Get bulk operation details
hookdeck bulk request-retry get <operation-id>

# Cancel bulk operation
hookdeck bulk request-retry cancel <operation-id>
```

### Ignored Events Bulk Retry
```bash
# List bulk ignored event retry operations
hookdeck bulk ignored-event-retry list --limit 50

# Create bulk ignored event retry operation
hookdeck bulk ignored-event-retry create --query '{"webhook_id": "conn_123"}'

# Plan bulk ignored event retry (dry run)
hookdeck bulk ignored-event-retry plan --query '{"webhook_id": "conn_123"}'

# Get bulk operation details
hookdeck bulk ignored-event-retry get <operation-id>

# Cancel bulk operation
hookdeck bulk ignored-event-retry cancel <operation-id>
```

## Notifications

🚧 **PLANNED FUNCTIONALITY** - Not yet implemented

### Send webhook notification
```bash
# Send webhook notification
hookdeck notification webhook --url "https://example.com/webhook" \
  --payload '{"message": "Test notification", "timestamp": "2023-12-01T10:00:00Z"}'
```

---

## Command Parameter Patterns

### Type-Driven Validation
Many commands use type-driven validation where the `--type` parameter determines which additional flags are required or valid:

- **Source creation**: `--type STRIPE` requires `--webhook-secret`, while `--type GITLAB` requires `--api-key`
- **Issue trigger creation**: `--type delivery` requires `--strategy` and `--connections`, while `--type transformation` requires `--log-level` and `--transformations`

### Collision Resolution
The `hookdeck connection create` command uses prefixed flags to avoid parameter collision when creating inline resources:

- **Individual resource commands**: Use `--type` (clear context)
- **Connection creation with inline resources**: Use `--source-type` and `--destination-type` (disambiguation)

### Parameter Conversion Patterns
- **Nested JSON → Flat flags**: `{"configs": {"strategy": "final_attempt"}}` becomes `--strategy final_attempt`
- **Arrays → Comma-separated**: `{"connections": ["conn1", "conn2"]}` becomes `--connections "conn1,conn2"`
- **Boolean presence → Presence flags**: `{"channels": {"email": {}}}` becomes `--email`
- **Complex objects → Value flags**: `{"channels": {"slack": {"channel_name": "#alerts"}}}` becomes `--slack-channel "#alerts"`

### Global Conventions
- **Resource IDs**: Use `<resource-id>` format in documentation
- **Optional parameters**: Enclosed in square brackets `[--optional-flag]`
- **Required vs optional**: Indicated by command syntax and parameter descriptions
- **Filtering**: Most list commands support filtering by name patterns, IDs, and status
- **Pagination**: All list commands support `--limit` for result limiting
- **Force operations**: Destructive operations support `--force` to skip confirmations

This comprehensive reference provides complete coverage of all Hookdeck CLI commands, including current functionality and planned features with their full parameter specifications.