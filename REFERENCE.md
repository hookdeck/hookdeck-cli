# Hookdeck CLI Reference

<!-- generated at 2026-02-19 -->

The Hookdeck CLI provides comprehensive webhook infrastructure management including authentication, project management, resource management, event and attempt querying, and local development tools. This reference covers all available commands and their usage.

## Table of Contents

<!-- GENERATE_TOC:START -->
- [Global Options](#global-options)
- [Authentication](#authentication)
- [Projects](#projects)
- [Local Development](#local-development)
- [Gateway](#gateway)
- [Connections](#connections)
- [Sources](#sources)
- [Destinations](#destinations)
- [Transformations](#transformations)
- [Events](#events)
- [Requests](#requests)
- [Attempts](#attempts)
- [Utilities](#utilities)
<!-- GENERATE_END -->
## Global Options

All commands support these global options:

<!-- GENERATE_GLOBAL_FLAGS:START -->
| Flag | Type | Description |
|------|------|-------------|
| `--color` | `string` | turn on/off color output (on, off, auto) |
| `--config` | `string` | config file (default is $HOME/.config/hookdeck/config.toml) |
| `--device-name` | `string` | device name |
| `--insecure` | `bool` | Allow invalid TLS certificates |
| `--log-level` | `string` | log level (debug, info, warn, error) (default "info") |
| `-p, --profile` | `string` | profile name (default "default") |
| `-v, --version` | `bool` | Get the version of the Hookdeck CLI |

<!-- GENERATE_END -->
## Authentication

<!-- GENERATE:login|logout|whoami:START -->
- [hookdeck login](#hookdeck-login)
- [hookdeck logout](#hookdeck-logout)
- [hookdeck whoami](#hookdeck-whoami)

### hookdeck login

Login to your Hookdeck account to setup the CLI

**Usage:**

```bash
hookdeck login [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `-i, --interactive` | `bool` | Run interactive configuration mode if you cannot open a browser |
### hookdeck logout

Logout of your Hookdeck account to setup the CLI

**Usage:**

```bash
hookdeck logout [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `-a, --all` | `bool` | Clear credentials for all projects you are currently logged into. |
### hookdeck whoami

Show the logged-in user

**Usage:**

```bash
hookdeck whoami
```
<!-- GENERATE_END -->
## Projects

<!-- GENERATE:project list|project use:START -->
- [hookdeck project list](#hookdeck-project-list)
- [hookdeck project use](#hookdeck-project-use)

### hookdeck project list

List and filter projects by organization and project name substrings

**Usage:**

```bash
hookdeck project list [<organization_substring>] [<project_substring>]
```
### hookdeck project use

Set the active project for future commands

**Usage:**

```bash
hookdeck project use [<organization_name> [<project_name>]] [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--local` | `bool` | Save project to current directory (.hookdeck/config.toml) |
<!-- GENERATE_END -->
## Local Development

<!-- GENERATE:listen:START -->
### hookdeck listen

Forward events for a source to your local server.

This command will create a new Hookdeck Source if it doesn't exist.

By default the Hookdeck Destination will be named "{source}-cli", and the
Destination CLI path will be "/". To set the CLI path, use the "`--path`" flag.

**Usage:**

```bash
hookdeck listen [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--filter-body` | `string` | Filter events by request body using Hookdeck filter syntax (JSON) |
| `--filter-headers` | `string` | Filter events by request headers using Hookdeck filter syntax (JSON) |
| `--filter-path` | `string` | Filter events by request path using Hookdeck filter syntax (JSON) |
| `--filter-query` | `string` | Filter events by query parameters using Hookdeck filter syntax (JSON) |
| `--max-connections` | `int` | Maximum concurrent connections to local endpoint (default: 50, increase for high-volume testing) (default "50") |
| `--no-healthcheck` | `bool` | Disable periodic health checks of the local server |
| `--output` | `string` | Output mode: interactive (full UI), compact (simple logs), quiet (errors and warnings only) (default "interactive") |
| `--path` | `string` | Sets the path to which events are forwarded e.g., /webhooks or /api/stripe |
<!-- GENERATE_END -->
## Gateway

<!-- GENERATE:gateway:START -->
### hookdeck gateway

Commands for managing Event Gateway sources, destinations, connections,
transformations, events, requests, and metrics.

The gateway command group provides full access to all Event Gateway resources.

**Usage:**

```bash
hookdeck gateway
```

**Examples:**

```bash
# List connections
hookdeck gateway connection list

# Create a source
hookdeck gateway source create --name my-source --type WEBHOOK

# Query event metrics
hookdeck gateway metrics events --start 2026-01-01T00:00:00Z --end 2026-02-01T00:00:00Z
```
<!-- GENERATE_END -->
## Connections

<!-- GENERATE:gateway connection list|gateway connection create|gateway connection get|gateway connection update|gateway connection delete|gateway connection upsert|gateway connection enable|gateway connection disable|gateway connection pause|gateway connection unpause:START -->
- [hookdeck gateway connection list](#hookdeck-gateway-connection-list)
- [hookdeck gateway connection create](#hookdeck-gateway-connection-create)
- [hookdeck gateway connection get](#hookdeck-gateway-connection-get)
- [hookdeck gateway connection update](#hookdeck-gateway-connection-update)
- [hookdeck gateway connection delete](#hookdeck-gateway-connection-delete)
- [hookdeck gateway connection upsert](#hookdeck-gateway-connection-upsert)
- [hookdeck gateway connection enable](#hookdeck-gateway-connection-enable)
- [hookdeck gateway connection disable](#hookdeck-gateway-connection-disable)
- [hookdeck gateway connection pause](#hookdeck-gateway-connection-pause)
- [hookdeck gateway connection unpause](#hookdeck-gateway-connection-unpause)

### hookdeck gateway connection list

List all connections or filter by source/destination.

**Usage:**

```bash
hookdeck gateway connection list [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--destination-id` | `string` | Filter by destination ID |
| `--disabled` | `bool` | Include disabled connections |
| `--limit` | `int` | Limit number of results (default "100") |
| `--name` | `string` | Filter by connection name |
| `--output` | `string` | Output format (json) |
| `--source-id` | `string` | Filter by source ID |

**Examples:**

```bash
# List all connections
hookdeck connection list

# Filter by connection name
hookdeck connection list --name my-connection

# Filter by source ID
hookdeck connection list --source-id src_abc123

# Filter by destination ID
hookdeck connection list --destination-id dst_def456

# Include disabled connections
hookdeck connection list --disabled

# Limit results
hookdeck connection list --limit 10
```
### hookdeck gateway connection create

Create a connection between a source and destination.
	
	You can either reference existing resources by ID or create them inline.

**Usage:**

```bash
hookdeck gateway connection create [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--description` | `string` | Connection description |
| `--destination-api-key` | `string` | API key for destination authentication |
| `--destination-api-key-header` | `string` | Key/header name for API key authentication |
| `--destination-api-key-to` | `string` | Where to send API key: 'header' or 'query' (default "header") |
| `--destination-auth-method` | `string` | Authentication method for HTTP destinations (hookdeck, bearer, basic, api_key, custom_signature, oauth2_client_credentials, oauth2_authorization_code, aws, gcp) |
| `--destination-aws-access-key-id` | `string` | AWS access key ID |
| `--destination-aws-region` | `string` | AWS region |
| `--destination-aws-secret-access-key` | `string` | AWS secret access key |
| `--destination-aws-service` | `string` | AWS service name |
| `--destination-basic-auth-pass` | `string` | Password for destination Basic authentication |
| `--destination-basic-auth-user` | `string` | Username for destination Basic authentication |
| `--destination-bearer-token` | `string` | Bearer token for destination authentication |
| `--destination-cli-path` | `string` | CLI path for CLI destinations (default: /) (default "/") |
| `--destination-custom-signature-key` | `string` | Key/header name for custom signature |
| `--destination-custom-signature-secret` | `string` | Signing secret for custom signature |
| `--destination-description` | `string` | Destination description |
| `--destination-gcp-scope` | `string` | GCP scope for service account authentication |
| `--destination-gcp-service-account-key` | `string` | GCP service account key JSON for destination authentication |
| `--destination-http-method` | `string` | HTTP method for HTTP destinations (GET, POST, PUT, PATCH, DELETE) |
| `--destination-id` | `string` | Use existing destination by ID |
| `--destination-name` | `string` | Destination name for inline creation |
| `--destination-oauth2-auth-server` | `string` | OAuth2 authorization server URL |
| `--destination-oauth2-auth-type` | `string` | OAuth2 Client Credentials authentication type: 'basic', 'bearer', or 'x-www-form-urlencoded' (default "basic") |
| `--destination-oauth2-client-id` | `string` | OAuth2 client ID |
| `--destination-oauth2-client-secret` | `string` | OAuth2 client secret |
| `--destination-oauth2-refresh-token` | `string` | OAuth2 refresh token (required for Authorization Code flow) |
| `--destination-oauth2-scopes` | `string` | OAuth2 scopes (comma-separated) |
| `--destination-path-forwarding-disabled` | `string` | Disable path forwarding for HTTP destinations (true/false) |
| `--destination-rate-limit` | `int` | Rate limit for destination (requests per period) (default "0") |
| `--destination-rate-limit-period` | `string` | Rate limit period (second, minute, hour, concurrent) |
| `--destination-type` | `string` | Destination type (CLI, HTTP, MOCK) |
| `--destination-url` | `string` | URL for HTTP destinations |
| `--name` | `string` | Connection name (required) |
| `--output` | `string` | Output format (json) |
| `--rule-deduplicate-exclude-fields` | `string` | Comma-separated list of fields to exclude for deduplication |
| `--rule-deduplicate-include-fields` | `string` | Comma-separated list of fields to include for deduplication |
| `--rule-deduplicate-window` | `int` | Time window in seconds for deduplication (default "0") |
| `--rule-delay` | `int` | Delay in milliseconds (default "0") |
| `--rule-filter-body` | `string` | JQ expression to filter on request body |
| `--rule-filter-headers` | `string` | JQ expression to filter on request headers |
| `--rule-filter-path` | `string` | JQ expression to filter on request path |
| `--rule-filter-query` | `string` | JQ expression to filter on request query parameters |
| `--rule-retry-count` | `int` | Number of retry attempts (default "0") |
| `--rule-retry-interval` | `int` | Interval between retries in milliseconds (default "0") |
| `--rule-retry-response-status-codes` | `string` | Comma-separated HTTP status codes to retry on |
| `--rule-retry-strategy` | `string` | Retry strategy (linear, exponential) |
| `--rule-transform-code` | `string` | Transformation code (if creating inline) |
| `--rule-transform-env` | `string` | JSON string representing environment variables for transformation |
| `--rule-transform-name` | `string` | Name or ID of the transformation to apply |
| `--rules` | `string` | JSON string representing the entire rules array |
| `--rules-file` | `string` | Path to a JSON file containing the rules array |
| `--source-allowed-http-methods` | `string` | Comma-separated list of allowed HTTP methods (GET, POST, PUT, PATCH, DELETE) |
| `--source-api-key` | `string` | API key for source authentication |
| `--source-basic-auth-pass` | `string` | Password for Basic authentication |
| `--source-basic-auth-user` | `string` | Username for Basic authentication |
| `--source-config` | `string` | JSON string for source authentication config |
| `--source-config-file` | `string` | Path to a JSON file for source authentication config |
| `--source-custom-response-body` | `string` | Custom response body (max 1000 chars) |
| `--source-custom-response-content-type` | `string` | Custom response content type (json, text, xml) |
| `--source-description` | `string` | Source description |
| `--source-hmac-algo` | `string` | HMAC algorithm (SHA256, etc.) |
| `--source-hmac-secret` | `string` | HMAC secret for signature verification |
| `--source-id` | `string` | Use existing source by ID |
| `--source-name` | `string` | Source name for inline creation |
| `--source-type` | `string` | Source type (WEBHOOK, STRIPE, etc.) |
| `--source-webhook-secret` | `string` | Webhook secret for source verification (e.g., Stripe) |

**Examples:**

```bash
# Create with inline source and destination
hookdeck connection create \
--name "test-webhooks-to-local" \
--source-type WEBHOOK --source-name "test-webhooks" \
--destination-type CLI --destination-name "local-dev"

# Create with existing resources
hookdeck connection create \
--name "github-to-api" \
--source-id src_abc123 \
--destination-id dst_def456

# Create with source configuration options
hookdeck connection create \
--name "api-webhooks" \
--source-type WEBHOOK --source-name "api-source" \
--source-allowed-http-methods "POST,PUT,PATCH" \
--source-custom-response-content-type "json" \
--source-custom-response-body '{"status":"received"}' \
--destination-type CLI --destination-name "local-dev"
```
### hookdeck gateway connection get

Get detailed information about a specific connection.

You can specify either a connection ID or name.

**Usage:**

```bash
hookdeck gateway connection get <connection-id-or-name> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--include-destination-auth` | `bool` | Include destination authentication credentials in the response |
| `--include-source-auth` | `bool` | Include source authentication credentials in the response |
| `--output` | `string` | Output format (json) |

**Examples:**

```bash
# Get connection by ID
hookdeck connection get conn_abc123

# Get connection by name
hookdeck connection get my-connection
```
### hookdeck gateway connection update

Update an existing connection by its ID.

Unlike upsert (which uses name as identifier), update takes a connection ID
and allows changing any field including the connection name.

**Usage:**

```bash
hookdeck gateway connection update <connection-id> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--description` | `string` | Connection description |
| `--destination-id` | `string` | Update destination by ID |
| `--name` | `string` | New connection name |
| `--output` | `string` | Output format (json) |
| `--rule-deduplicate-exclude-fields` | `string` | Comma-separated list of fields to exclude for deduplication |
| `--rule-deduplicate-include-fields` | `string` | Comma-separated list of fields to include for deduplication |
| `--rule-deduplicate-window` | `int` | Time window in seconds for deduplication (default "0") |
| `--rule-delay` | `int` | Delay in milliseconds (default "0") |
| `--rule-filter-body` | `string` | JQ expression to filter on request body |
| `--rule-filter-headers` | `string` | JQ expression to filter on request headers |
| `--rule-filter-path` | `string` | JQ expression to filter on request path |
| `--rule-filter-query` | `string` | JQ expression to filter on request query parameters |
| `--rule-retry-count` | `int` | Number of retry attempts (default "0") |
| `--rule-retry-interval` | `int` | Interval between retries in milliseconds (default "0") |
| `--rule-retry-response-status-codes` | `string` | Comma-separated HTTP status codes to retry on |
| `--rule-retry-strategy` | `string` | Retry strategy (linear, exponential) |
| `--rule-transform-code` | `string` | Transformation code (if creating inline) |
| `--rule-transform-env` | `string` | JSON string representing environment variables for transformation |
| `--rule-transform-name` | `string` | Name or ID of the transformation to apply |
| `--rules` | `string` | JSON string representing the entire rules array |
| `--rules-file` | `string` | Path to a JSON file containing the rules array |
| `--source-id` | `string` | Update source by ID |

**Examples:**

```bash
# Rename a connection
hookdeck gateway connection update web_abc123 --name "new-name"

# Update description
hookdeck gateway connection update web_abc123 --description "Updated description"

# Change the source on a connection
hookdeck gateway connection update web_abc123 --source-id src_def456

# Update rules
hookdeck gateway connection update web_abc123 \
--rule-retry-strategy linear --rule-retry-count 5

# Update with JSON output
hookdeck gateway connection update web_abc123 --name "new-name" --output json
```
### hookdeck gateway connection delete

Delete a connection.

**Usage:**

```bash
hookdeck gateway connection delete <connection-id> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--force` | `bool` | Force delete without confirmation |

**Examples:**

```bash
# Delete a connection (with confirmation)
hookdeck connection delete conn_abc123

# Force delete without confirmation
hookdeck connection delete conn_abc123 --force
```
### hookdeck gateway connection upsert

Create a new connection or update an existing one by name (idempotent).

	This command is idempotent - it can be safely run multiple times with the same arguments.
	
	When the connection doesn't exist:
		 - Creates a new connection with the provided properties
		 - Requires source and destination to be specified
	
	When the connection exists:
		 - Updates the connection with the provided properties
		 - Only updates properties that are explicitly provided
		 - Preserves existing properties that aren't specified
	
	Use `--dry-run` to preview changes without applying them.

**Usage:**

```bash
hookdeck gateway connection upsert <name> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--description` | `string` | Connection description |
| `--destination-api-key` | `string` | API key for destination authentication |
| `--destination-api-key-header` | `string` | Key/header name for API key authentication |
| `--destination-api-key-to` | `string` | Where to send API key: 'header' or 'query' (default "header") |
| `--destination-auth-method` | `string` | Authentication method for HTTP destinations (hookdeck, bearer, basic, api_key, custom_signature, oauth2_client_credentials, oauth2_authorization_code, aws, gcp) |
| `--destination-aws-access-key-id` | `string` | AWS access key ID |
| `--destination-aws-region` | `string` | AWS region |
| `--destination-aws-secret-access-key` | `string` | AWS secret access key |
| `--destination-aws-service` | `string` | AWS service name |
| `--destination-basic-auth-pass` | `string` | Password for destination Basic authentication |
| `--destination-basic-auth-user` | `string` | Username for destination Basic authentication |
| `--destination-bearer-token` | `string` | Bearer token for destination authentication |
| `--destination-cli-path` | `string` | CLI path for CLI destinations (default: /) (default "/") |
| `--destination-custom-signature-key` | `string` | Key/header name for custom signature |
| `--destination-custom-signature-secret` | `string` | Signing secret for custom signature |
| `--destination-description` | `string` | Destination description |
| `--destination-gcp-scope` | `string` | GCP scope for service account authentication |
| `--destination-gcp-service-account-key` | `string` | GCP service account key JSON for destination authentication |
| `--destination-http-method` | `string` | HTTP method for HTTP destinations (GET, POST, PUT, PATCH, DELETE) |
| `--destination-id` | `string` | Use existing destination by ID |
| `--destination-name` | `string` | Destination name for inline creation |
| `--destination-oauth2-auth-server` | `string` | OAuth2 authorization server URL |
| `--destination-oauth2-auth-type` | `string` | OAuth2 Client Credentials authentication type: 'basic', 'bearer', or 'x-www-form-urlencoded' (default "basic") |
| `--destination-oauth2-client-id` | `string` | OAuth2 client ID |
| `--destination-oauth2-client-secret` | `string` | OAuth2 client secret |
| `--destination-oauth2-refresh-token` | `string` | OAuth2 refresh token (required for Authorization Code flow) |
| `--destination-oauth2-scopes` | `string` | OAuth2 scopes (comma-separated) |
| `--destination-path-forwarding-disabled` | `string` | Disable path forwarding for HTTP destinations (true/false) |
| `--destination-rate-limit` | `int` | Rate limit for destination (requests per period) (default "0") |
| `--destination-rate-limit-period` | `string` | Rate limit period (second, minute, hour, concurrent) |
| `--destination-type` | `string` | Destination type (CLI, HTTP, MOCK) |
| `--destination-url` | `string` | URL for HTTP destinations |
| `--dry-run` | `bool` | Preview changes without applying them |
| `--output` | `string` | Output format (json) |
| `--rule-deduplicate-exclude-fields` | `string` | Comma-separated list of fields to exclude for deduplication |
| `--rule-deduplicate-include-fields` | `string` | Comma-separated list of fields to include for deduplication |
| `--rule-deduplicate-window` | `int` | Time window in seconds for deduplication (default "0") |
| `--rule-delay` | `int` | Delay in milliseconds (default "0") |
| `--rule-filter-body` | `string` | JQ expression to filter on request body |
| `--rule-filter-headers` | `string` | JQ expression to filter on request headers |
| `--rule-filter-path` | `string` | JQ expression to filter on request path |
| `--rule-filter-query` | `string` | JQ expression to filter on request query parameters |
| `--rule-retry-count` | `int` | Number of retry attempts (default "0") |
| `--rule-retry-interval` | `int` | Interval between retries in milliseconds (default "0") |
| `--rule-retry-response-status-codes` | `string` | Comma-separated HTTP status codes to retry on |
| `--rule-retry-strategy` | `string` | Retry strategy (linear, exponential) |
| `--rule-transform-code` | `string` | Transformation code (if creating inline) |
| `--rule-transform-env` | `string` | JSON string representing environment variables for transformation |
| `--rule-transform-name` | `string` | Name or ID of the transformation to apply |
| `--rules` | `string` | JSON string representing the entire rules array |
| `--rules-file` | `string` | Path to a JSON file containing the rules array |
| `--source-allowed-http-methods` | `string` | Comma-separated list of allowed HTTP methods (GET, POST, PUT, PATCH, DELETE) |
| `--source-api-key` | `string` | API key for source authentication |
| `--source-basic-auth-pass` | `string` | Password for Basic authentication |
| `--source-basic-auth-user` | `string` | Username for Basic authentication |
| `--source-config` | `string` | JSON string for source authentication config |
| `--source-config-file` | `string` | Path to a JSON file for source authentication config |
| `--source-custom-response-body` | `string` | Custom response body (max 1000 chars) |
| `--source-custom-response-content-type` | `string` | Custom response content type (json, text, xml) |
| `--source-description` | `string` | Source description |
| `--source-hmac-algo` | `string` | HMAC algorithm (SHA256, etc.) |
| `--source-hmac-secret` | `string` | HMAC secret for signature verification |
| `--source-id` | `string` | Use existing source by ID |
| `--source-name` | `string` | Source name for inline creation |
| `--source-type` | `string` | Source type (WEBHOOK, STRIPE, etc.) |
| `--source-webhook-secret` | `string` | Webhook secret for source verification (e.g., Stripe) |

**Examples:**

```bash
# Create or update a connection with inline source and destination
hookdeck connection upsert "my-connection" \
--source-name "stripe-prod" --source-type STRIPE \
--destination-name "my-api" --destination-type HTTP --destination-url https://api.example.com

# Update just the rate limit on an existing connection
hookdeck connection upsert my-connection \
--destination-rate-limit 100 --destination-rate-limit-period minute

# Update source configuration options
hookdeck connection upsert my-connection \
--source-allowed-http-methods "POST,PUT,DELETE" \
--source-custom-response-content-type "json" \
--source-custom-response-body '{"status":"received"}'

# Preview changes without applying them
hookdeck connection upsert my-connection \
--destination-rate-limit 200 --destination-rate-limit-period hour \
--dry-run
```
### hookdeck gateway connection enable

Enable a disabled connection.

**Usage:**

```bash
hookdeck gateway connection enable <connection-id>
```
### hookdeck gateway connection disable

Disable an active connection. It will stop receiving new events until re-enabled.

**Usage:**

```bash
hookdeck gateway connection disable <connection-id>
```
### hookdeck gateway connection pause

Pause a connection temporarily.

The connection will queue incoming events until unpaused.

**Usage:**

```bash
hookdeck gateway connection pause <connection-id>
```
### hookdeck gateway connection unpause

Resume a paused connection.

The connection will start processing queued events.

**Usage:**

```bash
hookdeck gateway connection unpause <connection-id>
```
<!-- GENERATE_END -->
## Sources

<!-- GENERATE:gateway source list|gateway source create|gateway source get|gateway source update|gateway source delete|gateway source upsert|gateway source enable|gateway source disable|gateway source count:START -->
- [hookdeck gateway source list](#hookdeck-gateway-source-list)
- [hookdeck gateway source create](#hookdeck-gateway-source-create)
- [hookdeck gateway source get](#hookdeck-gateway-source-get)
- [hookdeck gateway source update](#hookdeck-gateway-source-update)
- [hookdeck gateway source delete](#hookdeck-gateway-source-delete)
- [hookdeck gateway source upsert](#hookdeck-gateway-source-upsert)
- [hookdeck gateway source enable](#hookdeck-gateway-source-enable)
- [hookdeck gateway source disable](#hookdeck-gateway-source-disable)
- [hookdeck gateway source count](#hookdeck-gateway-source-count)

### hookdeck gateway source list

List all sources or filter by name or type.

**Usage:**

```bash
hookdeck gateway source list [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--disabled` | `bool` | Include disabled sources |
| `--limit` | `int` | Limit number of results (default "100") |
| `--name` | `string` | Filter by source name |
| `--output` | `string` | Output format (json) |
| `--type` | `string` | Filter by source type (e.g. WEBHOOK, STRIPE) |

**Examples:**

```bash
hookdeck gateway source list
hookdeck gateway source list --name my-source
hookdeck gateway source list --type WEBHOOK
hookdeck gateway source list --disabled
hookdeck gateway source list --limit 10
```
### hookdeck gateway source create

Create a new source.

Requires `--name` and `--type`. Use `--config` or `--config-file` for authentication (e.g. webhook_secret, api_key).

**Usage:**

```bash
hookdeck gateway source create [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--allowed-http-methods` | `string` | Comma-separated allowed HTTP methods (GET, POST, PUT, PATCH, DELETE) |
| `--basic-auth-pass` | `string` | Password for Basic authentication |
| `--basic-auth-user` | `string` | Username for Basic authentication |
| `--config-file` | `string` | Path to JSON file for source config (overrides individual flags if set) |
| `--custom-response-body` | `string` | Custom response body (max 1000 chars) |
| `--custom-response-content-type` | `string` | Custom response content type (json, text, xml) |
| `--description` | `string` | Source description |
| `--hmac-algo` | `string` | HMAC algorithm (SHA256, etc.) |
| `--hmac-secret` | `string` | HMAC secret for signature verification |
| `--name` | `string` | Source name (required) |
| `--output` | `string` | Output format (json) |
| `--type` | `string` | Source type (e.g. WEBHOOK, STRIPE) (required) |
| `--webhook-secret` | `string` | Webhook secret for source verification (e.g., Stripe) |

**Examples:**

```bash
hookdeck gateway source create --name my-webhook --type WEBHOOK
hookdeck gateway source create --name stripe-prod --type STRIPE --config '{"webhook_secret":"whsec_xxx"}'
```
### hookdeck gateway source get

Get detailed information about a specific source.

You can specify either a source ID or name.

**Usage:**

```bash
hookdeck gateway source get <source-id-or-name> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--include-auth` | `bool` | Include source authentication credentials in the response |
| `--output` | `string` | Output format (json) |

**Examples:**

```bash
hookdeck gateway source get src_abc123
hookdeck gateway source get my-source --include-auth
```
### hookdeck gateway source update

Update an existing source by its ID.

**Usage:**

```bash
hookdeck gateway source update <source-id> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--allowed-http-methods` | `string` | Comma-separated allowed HTTP methods (GET, POST, PUT, PATCH, DELETE) |
| `--basic-auth-pass` | `string` | Password for Basic authentication |
| `--basic-auth-user` | `string` | Username for Basic authentication |
| `--config-file` | `string` | Path to JSON file for source config (overrides individual flags if set) |
| `--custom-response-body` | `string` | Custom response body (max 1000 chars) |
| `--custom-response-content-type` | `string` | Custom response content type (json, text, xml) |
| `--description` | `string` | New source description |
| `--hmac-algo` | `string` | HMAC algorithm (SHA256, etc.) |
| `--hmac-secret` | `string` | HMAC secret for signature verification |
| `--name` | `string` | New source name |
| `--output` | `string` | Output format (json) |
| `--type` | `string` | Source type (e.g. WEBHOOK, STRIPE) |
| `--webhook-secret` | `string` | Webhook secret for source verification (e.g., Stripe) |

**Examples:**

```bash
hookdeck gateway source update src_abc123 --name new-name
hookdeck gateway source update src_abc123 --description "Updated"
hookdeck gateway source update src_abc123 --config '{"webhook_secret":"whsec_new"}'
```
### hookdeck gateway source delete

Delete a source.

**Usage:**

```bash
hookdeck gateway source delete <source-id> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--force` | `bool` | Force delete without confirmation |

**Examples:**

```bash
hookdeck gateway source delete src_abc123
hookdeck gateway source delete src_abc123 --force
```
### hookdeck gateway source upsert

Create a new source or update an existing one by name (idempotent).

**Usage:**

```bash
hookdeck gateway source upsert <name> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--allowed-http-methods` | `string` | Comma-separated allowed HTTP methods (GET, POST, PUT, PATCH, DELETE) |
| `--basic-auth-pass` | `string` | Password for Basic authentication |
| `--basic-auth-user` | `string` | Username for Basic authentication |
| `--config-file` | `string` | Path to JSON file for source config (overrides individual flags if set) |
| `--custom-response-body` | `string` | Custom response body (max 1000 chars) |
| `--custom-response-content-type` | `string` | Custom response content type (json, text, xml) |
| `--description` | `string` | Source description |
| `--dry-run` | `bool` | Preview changes without applying |
| `--hmac-algo` | `string` | HMAC algorithm (SHA256, etc.) |
| `--hmac-secret` | `string` | HMAC secret for signature verification |
| `--output` | `string` | Output format (json) |
| `--type` | `string` | Source type (e.g. WEBHOOK, STRIPE) |
| `--webhook-secret` | `string` | Webhook secret for source verification (e.g., Stripe) |

**Examples:**

```bash
hookdeck gateway source upsert my-webhook --type WEBHOOK
hookdeck gateway source upsert stripe-prod --type STRIPE --config '{"webhook_secret":"whsec_xxx"}'
hookdeck gateway source upsert my-webhook --description "Updated" --dry-run
```
### hookdeck gateway source enable

Enable a disabled source.

**Usage:**

```bash
hookdeck gateway source enable <source-id>
```
### hookdeck gateway source disable

Disable an active source. It will stop receiving new events until re-enabled.

**Usage:**

```bash
hookdeck gateway source disable <source-id>
```
### hookdeck gateway source count

Count sources matching optional filters.

**Usage:**

```bash
hookdeck gateway source count [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--disabled` | `bool` | Count disabled sources only (when set with other filters) |
| `--name` | `string` | Filter by source name |
| `--type` | `string` | Filter by source type |

**Examples:**

```bash
hookdeck gateway source count
hookdeck gateway source count --type WEBHOOK
hookdeck gateway source count --disabled
```
<!-- GENERATE_END -->
## Destinations

<!-- GENERATE:gateway destination list|gateway destination create|gateway destination get|gateway destination update|gateway destination delete|gateway destination upsert|gateway destination count|gateway destination enable|gateway destination disable:START -->
- [hookdeck gateway destination list](#hookdeck-gateway-destination-list)
- [hookdeck gateway destination create](#hookdeck-gateway-destination-create)
- [hookdeck gateway destination get](#hookdeck-gateway-destination-get)
- [hookdeck gateway destination update](#hookdeck-gateway-destination-update)
- [hookdeck gateway destination delete](#hookdeck-gateway-destination-delete)
- [hookdeck gateway destination upsert](#hookdeck-gateway-destination-upsert)
- [hookdeck gateway destination count](#hookdeck-gateway-destination-count)
- [hookdeck gateway destination enable](#hookdeck-gateway-destination-enable)
- [hookdeck gateway destination disable](#hookdeck-gateway-destination-disable)

### hookdeck gateway destination list

List all destinations or filter by name or type.

**Usage:**

```bash
hookdeck gateway destination list [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--disabled` | `bool` | Include disabled destinations |
| `--limit` | `int` | Limit number of results (default "100") |
| `--name` | `string` | Filter by destination name |
| `--output` | `string` | Output format (json) |
| `--type` | `string` | Filter by destination type (HTTP, CLI, MOCK_API) |

**Examples:**

```bash
hookdeck gateway destination list
hookdeck gateway destination list --name my-destination
hookdeck gateway destination list --type HTTP
hookdeck gateway destination list --disabled
hookdeck gateway destination list --limit 10
```
### hookdeck gateway destination create

Create a new destination.

Requires `--name` and `--type`. For HTTP destinations, `--url` is required. Use `--config` or `--config-file` for auth and rate limiting.

**Usage:**

```bash
hookdeck gateway destination create [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--api-key-header` | `string` | Header/key name for API key |
| `--api-key-to` | `string` | Where to send API key (header or query) (default "header") |
| `--auth-method` | `string` | Auth method (hookdeck, bearer, basic, api_key, custom_signature) |
| `--basic-auth-pass` | `string` | Password for Basic auth |
| `--basic-auth-user` | `string` | Username for Basic auth |
| `--bearer-token` | `string` | Bearer token for destination auth |
| `--cli-path` | `string` | Path for CLI destinations (default "/") |
| `--config-file` | `string` | Path to JSON file for destination config (overrides individual flags if set) |
| `--custom-signature-key` | `string` | Key/header name for custom signature |
| `--custom-signature-secret` | `string` | Signing secret for custom signature |
| `--description` | `string` | Destination description |
| `--http-method` | `string` | HTTP method for HTTP destinations (GET, POST, PUT, PATCH, DELETE) |
| `--name` | `string` | Destination name (required) |
| `--output` | `string` | Output format (json) |
| `--rate-limit` | `int` | Rate limit (requests per period) (default "0") |
| `--rate-limit-period` | `string` | Rate limit period (second, minute, hour, concurrent) |
| `--type` | `string` | Destination type (HTTP, CLI, MOCK_API) (required) |
| `--url` | `string` | URL for HTTP destinations (required for type HTTP) |

**Examples:**

```bash
hookdeck gateway destination create --name my-api --type HTTP --url https://api.example.com/webhooks
hookdeck gateway destination create --name local-cli --type CLI --cli-path /webhooks
hookdeck gateway destination create --name my-api --type HTTP --url https://api.example.com --bearer-token token123
```
### hookdeck gateway destination get

Get detailed information about a specific destination.

You can specify either a destination ID or name.

**Usage:**

```bash
hookdeck gateway destination get <destination-id-or-name> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--include-auth` | `bool` | Include authentication credentials in the response |
| `--output` | `string` | Output format (json) |

**Examples:**

```bash
hookdeck gateway destination get des_abc123
hookdeck gateway destination get my-destination --include-auth
```
### hookdeck gateway destination update

Update an existing destination by its ID.

**Usage:**

```bash
hookdeck gateway destination update <destination-id> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--api-key-header` | `string` | Header/key name for API key |
| `--api-key-to` | `string` | Where to send API key (header or query) (default "header") |
| `--auth-method` | `string` | Auth method (hookdeck, bearer, basic, api_key, custom_signature) |
| `--basic-auth-pass` | `string` | Password for Basic auth |
| `--basic-auth-user` | `string` | Username for Basic auth |
| `--bearer-token` | `string` | Bearer token for destination auth |
| `--cli-path` | `string` | Path for CLI destinations |
| `--config-file` | `string` | Path to JSON file for destination config (overrides individual flags if set) |
| `--custom-signature-key` | `string` | Key/header name for custom signature |
| `--custom-signature-secret` | `string` | Signing secret for custom signature |
| `--description` | `string` | New destination description |
| `--http-method` | `string` | HTTP method for HTTP destinations |
| `--name` | `string` | New destination name |
| `--output` | `string` | Output format (json) |
| `--rate-limit` | `int` | Rate limit (requests per period) (default "0") |
| `--rate-limit-period` | `string` | Rate limit period (second, minute, hour, concurrent) |
| `--type` | `string` | Destination type (HTTP, CLI, MOCK_API) |
| `--url` | `string` | URL for HTTP destinations |

**Examples:**

```bash
hookdeck gateway destination update des_abc123 --name new-name
hookdeck gateway destination update des_abc123 --description "Updated"
hookdeck gateway destination update des_abc123 --url https://api.example.com/new
```
### hookdeck gateway destination delete

Delete a destination.

**Usage:**

```bash
hookdeck gateway destination delete <destination-id> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--force` | `bool` | Force delete without confirmation |

**Examples:**

```bash
hookdeck gateway destination delete des_abc123
hookdeck gateway destination delete des_abc123 --force
```
### hookdeck gateway destination upsert

Create a new destination or update an existing one by name (idempotent).

**Usage:**

```bash
hookdeck gateway destination upsert <name> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--api-key-header` | `string` | Header/key name for API key |
| `--api-key-to` | `string` | Where to send API key (header or query) (default "header") |
| `--auth-method` | `string` | Auth method (hookdeck, bearer, basic, api_key, custom_signature) |
| `--basic-auth-pass` | `string` | Password for Basic auth |
| `--basic-auth-user` | `string` | Username for Basic auth |
| `--bearer-token` | `string` | Bearer token for destination auth |
| `--cli-path` | `string` | Path for CLI destinations |
| `--config-file` | `string` | Path to JSON file for destination config (overrides individual flags if set) |
| `--custom-signature-key` | `string` | Key/header name for custom signature |
| `--custom-signature-secret` | `string` | Signing secret for custom signature |
| `--description` | `string` | Destination description |
| `--dry-run` | `bool` | Preview changes without applying |
| `--http-method` | `string` | HTTP method for HTTP destinations |
| `--output` | `string` | Output format (json) |
| `--rate-limit` | `int` | Rate limit (requests per period) (default "0") |
| `--rate-limit-period` | `string` | Rate limit period (second, minute, hour, concurrent) |
| `--type` | `string` | Destination type (HTTP, CLI, MOCK_API) |
| `--url` | `string` | URL for HTTP destinations |

**Examples:**

```bash
hookdeck gateway destination upsert my-api --type HTTP --url https://api.example.com/webhooks
hookdeck gateway destination upsert local-cli --type CLI --cli-path /webhooks
hookdeck gateway destination upsert my-api --description "Updated" --dry-run
```
### hookdeck gateway destination count

Count destinations matching optional filters.

**Usage:**

```bash
hookdeck gateway destination count [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--disabled` | `bool` | Count disabled destinations only (when set with other filters) |
| `--name` | `string` | Filter by destination name |
| `--type` | `string` | Filter by destination type (HTTP, CLI, MOCK_API) |

**Examples:**

```bash
hookdeck gateway destination count
hookdeck gateway destination count --type HTTP
hookdeck gateway destination count --disabled
```
### hookdeck gateway destination enable

Enable a disabled destination.

**Usage:**

```bash
hookdeck gateway destination enable <destination-id>
```
### hookdeck gateway destination disable

Disable an active destination. It will stop receiving new events until re-enabled.

**Usage:**

```bash
hookdeck gateway destination disable <destination-id>
```
<!-- GENERATE_END -->
## Transformations

<!-- GENERATE:gateway transformation list|gateway transformation create|gateway transformation get|gateway transformation update|gateway transformation delete|gateway transformation upsert|gateway transformation run|gateway transformation count|gateway transformation executions|gateway transformation executions list|gateway transformation executions get:START -->
- [hookdeck gateway transformation list](#hookdeck-gateway-transformation-list)
- [hookdeck gateway transformation create](#hookdeck-gateway-transformation-create)
- [hookdeck gateway transformation get](#hookdeck-gateway-transformation-get)
- [hookdeck gateway transformation update](#hookdeck-gateway-transformation-update)
- [hookdeck gateway transformation delete](#hookdeck-gateway-transformation-delete)
- [hookdeck gateway transformation upsert](#hookdeck-gateway-transformation-upsert)
- [hookdeck gateway transformation run](#hookdeck-gateway-transformation-run)
- [hookdeck gateway transformation count](#hookdeck-gateway-transformation-count)
- [hookdeck gateway transformation executions list](#hookdeck-gateway-transformation-executions-list)
- [hookdeck gateway transformation executions get](#hookdeck-gateway-transformation-executions-get)

### hookdeck gateway transformation list

List all transformations or filter by name or id.

**Usage:**

```bash
hookdeck gateway transformation list [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--dir` | `string` | Sort direction (asc, desc) |
| `--id` | `string` | Filter by transformation ID(s) |
| `--limit` | `int` | Limit number of results (default "100") |
| `--name` | `string` | Filter by transformation name |
| `--next` | `string` | Pagination cursor for next page |
| `--order-by` | `string` | Sort key (name, created_at, updated_at) |
| `--output` | `string` | Output format (json) |
| `--prev` | `string` | Pagination cursor for previous page |

**Examples:**

```bash
hookdeck gateway transformation list
hookdeck gateway transformation list --name my-transform
hookdeck gateway transformation list --order-by created_at --dir desc
hookdeck gateway transformation list --limit 10
```
### hookdeck gateway transformation create

Create a new transformation.

Requires `--name` and `--code` (or `--code-file`). Use `--env` for key-value environment variables.

**Usage:**

```bash
hookdeck gateway transformation create [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--code` | `string` | JavaScript code string (required if `--code-file` not set) |
| `--code-file` | `string` | Path to JavaScript file (required if `--code` not set) |
| `--env` | `string` | Environment variables as KEY=value,KEY2=value2 |
| `--name` | `string` | Transformation name (required) |
| `--output` | `string` | Output format (json) |

**Examples:**

```bash
hookdeck gateway transformation create --name my-transform --code "addHandler(\"transform\", (request, context) => { return request; });"
hookdeck gateway transformation create --name my-transform --code-file ./transform.js --env FOO=bar,BAZ=qux
```
### hookdeck gateway transformation get

Get detailed information about a specific transformation.

You can specify either a transformation ID or name.

**Usage:**

```bash
hookdeck gateway transformation get <transformation-id-or-name> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--output` | `string` | Output format (json) |

**Examples:**

```bash
hookdeck gateway transformation get trn_abc123
hookdeck gateway transformation get my-transform
```
### hookdeck gateway transformation update

Update an existing transformation by its ID.

**Usage:**

```bash
hookdeck gateway transformation update <transformation-id-or-name> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--code` | `string` | New JavaScript code string |
| `--code-file` | `string` | Path to JavaScript file |
| `--env` | `string` | Environment variables as KEY=value,KEY2=value2 |
| `--name` | `string` | New transformation name |
| `--output` | `string` | Output format (json) |

**Examples:**

```bash
hookdeck gateway transformation update trn_abc123 --name new-name
hookdeck gateway transformation update my-transform --code-file ./transform.js
hookdeck gateway transformation update trn_abc123 --env FOO=bar
```
### hookdeck gateway transformation delete

Delete a transformation.

**Usage:**

```bash
hookdeck gateway transformation delete <transformation-id-or-name> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--force` | `bool` | Force delete without confirmation |

**Examples:**

```bash
hookdeck gateway transformation delete trn_abc123
hookdeck gateway transformation delete trn_abc123 --force
```
### hookdeck gateway transformation upsert

Create a new transformation or update an existing one by name (idempotent).

**Usage:**

```bash
hookdeck gateway transformation upsert <name> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--code` | `string` | JavaScript code string |
| `--code-file` | `string` | Path to JavaScript file |
| `--dry-run` | `bool` | Preview changes without applying |
| `--env` | `string` | Environment variables as KEY=value,KEY2=value2 |
| `--output` | `string` | Output format (json) |

**Examples:**

```bash
hookdeck gateway transformation upsert my-transform --code "addHandler(\"transform\", (request, context) => { return request; });"
hookdeck gateway transformation upsert my-transform --code-file ./transform.js --env FOO=bar
hookdeck gateway transformation upsert my-transform --code "addHandler(\"transform\", (request, context) => { return request; });" --dry-run
```
### hookdeck gateway transformation run

Test run transformation code against a sample request.

Provide either inline `--code`/`--code-file` or `--id` to use an existing transformation.
The `--request` or `--request-file` must be JSON with at least "headers" (can be {}). Optional: body, path, query.

**Usage:**

```bash
hookdeck gateway transformation run [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--code` | `string` | JavaScript code string to run |
| `--code-file` | `string` | Path to JavaScript file |
| `--connection-id` | `string` | Connection ID for execution context |
| `--env` | `string` | Environment variables as KEY=value,KEY2=value2 |
| `--id` | `string` | Use existing transformation by ID |
| `--output` | `string` | Output format (json) |
| `--request` | `string` | Request JSON (must include headers, e.g. {"headers":{}}) |
| `--request-file` | `string` | Path to request JSON file |

**Examples:**

```bash
hookdeck gateway transformation run --id trs_abc123 --request '{"headers":{}}'
hookdeck gateway transformation run --code "addHandler(\"transform\", (request, context) => { return request; });" --request-file ./sample.json
hookdeck gateway transformation run --id trs_abc123 --request '{"headers":{},"body":{"foo":"bar"}}' --connection-id web_xxx
```
### hookdeck gateway transformation count

Count transformations matching optional filters.

**Usage:**

```bash
hookdeck gateway transformation count [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--name` | `string` | Filter by transformation name |
| `--output` | `string` | Output format (json) |

**Examples:**

```bash
hookdeck gateway transformation count
hookdeck gateway transformation count --name my-transform
```
### hookdeck gateway transformation executions list

List executions for a transformation.

**Usage:**

```bash
hookdeck gateway transformation executions list <transformation-id-or-name> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--connection-id` | `string` | Filter by connection ID |
| `--created-at` | `string` | Filter by created_at (ISO date or operator) |
| `--dir` | `string` | Sort direction (asc, desc) |
| `--issue-id` | `string` | Filter by issue ID |
| `--limit` | `int` | Limit number of results (default "100") |
| `--next` | `string` | Pagination cursor for next page |
| `--order-by` | `string` | Sort key (created_at) |
| `--output` | `string` | Output format (json) |
| `--prev` | `string` | Pagination cursor for previous page |
### hookdeck gateway transformation executions get

Get a single execution by transformation ID and execution ID.

**Usage:**

```bash
hookdeck gateway transformation executions get <transformation-id-or-name> <execution-id> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--output` | `string` | Output format (json) |
<!-- GENERATE_END -->
## Events

<!-- GENERATE:gateway event list|gateway event get|gateway event retry|gateway event cancel|gateway event mute|gateway event raw-body:START -->
- [hookdeck gateway event list](#hookdeck-gateway-event-list)
- [hookdeck gateway event get](#hookdeck-gateway-event-get)
- [hookdeck gateway event retry](#hookdeck-gateway-event-retry)
- [hookdeck gateway event cancel](#hookdeck-gateway-event-cancel)
- [hookdeck gateway event mute](#hookdeck-gateway-event-mute)
- [hookdeck gateway event raw-body](#hookdeck-gateway-event-raw-body)

### hookdeck gateway event list

List events (processed webhook deliveries). Filter by connection ID, source, destination, or status.

**Usage:**

```bash
hookdeck gateway event list [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--attempts` | `string` | Filter by number of attempts (integer or operators) |
| `--body` | `string` | Filter by body (JSON string) |
| `--cli-id` | `string` | Filter by CLI ID |
| `--connection-id` | `string` | Filter by connection ID |
| `--created-after` | `string` | Filter events created after (ISO date-time) |
| `--created-before` | `string` | Filter events created before (ISO date-time) |
| `--destination-id` | `string` | Filter by destination ID |
| `--dir` | `string` | Sort direction (asc, desc) |
| `--error-code` | `string` | Filter by error code |
| `--headers` | `string` | Filter by headers (JSON string) |
| `--id` | `string` | Filter by event ID(s) (comma-separated) |
| `--issue-id` | `string` | Filter by issue ID |
| `--last-attempt-at-after` | `string` | Filter by last_attempt_at after (ISO date-time) |
| `--last-attempt-at-before` | `string` | Filter by last_attempt_at before (ISO date-time) |
| `--limit` | `int` | Limit number of results (default "100") |
| `--next` | `string` | Pagination cursor for next page |
| `--order-by` | `string` | Sort key (e.g. created_at) |
| `--output` | `string` | Output format (json) |
| `--parsed-query` | `string` | Filter by parsed query (JSON string) |
| `--path` | `string` | Filter by path |
| `--prev` | `string` | Pagination cursor for previous page |
| `--response-status` | `string` | Filter by HTTP response status (e.g. 200, 500) |
| `--source-id` | `string` | Filter by source ID |
| `--status` | `string` | Filter by status (SCHEDULED, QUEUED, HOLD, SUCCESSFUL, FAILED, CANCELLED) |
| `--successful-at-after` | `string` | Filter by successful_at after (ISO date-time) |
| `--successful-at-before` | `string` | Filter by successful_at before (ISO date-time) |

**Examples:**

```bash
hookdeck gateway event list
hookdeck gateway event list --connection-id web_abc123
hookdeck gateway event list --status FAILED --limit 20
```
### hookdeck gateway event get

Get detailed information about an event by ID.

**Usage:**

```bash
hookdeck gateway event get <event-id> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--output` | `string` | Output format (json) |

**Examples:**

```bash
hookdeck gateway event get evt_abc123
```
### hookdeck gateway event retry

Retry delivery for an event by ID.

**Usage:**

```bash
hookdeck gateway event retry <event-id>
```

**Examples:**

```bash
hookdeck gateway event retry evt_abc123
```
### hookdeck gateway event cancel

Cancel an event by ID. Cancelled events will not be retried.

**Usage:**

```bash
hookdeck gateway event cancel <event-id>
```

**Examples:**

```bash
hookdeck gateway event cancel evt_abc123
```
### hookdeck gateway event mute

Mute an event by ID. Muted events will not trigger alerts or retries.

**Usage:**

```bash
hookdeck gateway event mute <event-id>
```

**Examples:**

```bash
hookdeck gateway event mute evt_abc123
```
### hookdeck gateway event raw-body

Output the raw request body of an event by ID.

**Usage:**

```bash
hookdeck gateway event raw-body <event-id>
```

**Examples:**

```bash
hookdeck gateway event raw-body evt_abc123
```
<!-- GENERATE_END -->
## Requests

<!-- GENERATE:gateway request list|gateway request get|gateway request retry|gateway request events|gateway request ignored-events|gateway request raw-body:START -->
- [hookdeck gateway request list](#hookdeck-gateway-request-list)
- [hookdeck gateway request get](#hookdeck-gateway-request-get)
- [hookdeck gateway request retry](#hookdeck-gateway-request-retry)
- [hookdeck gateway request events](#hookdeck-gateway-request-events)
- [hookdeck gateway request ignored-events](#hookdeck-gateway-request-ignored-events)
- [hookdeck gateway request raw-body](#hookdeck-gateway-request-raw-body)

### hookdeck gateway request list

List requests (raw inbound webhooks). Filter by source ID.

**Usage:**

```bash
hookdeck gateway request list [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--body` | `string` | Filter by body (JSON string) |
| `--created-after` | `string` | Filter requests created after (ISO date-time) |
| `--created-before` | `string` | Filter requests created before (ISO date-time) |
| `--dir` | `string` | Sort direction (asc, desc) |
| `--headers` | `string` | Filter by headers (JSON string) |
| `--id` | `string` | Filter by request ID(s) (comma-separated) |
| `--ingested-at-after` | `string` | Filter by ingested_at after (ISO date-time) |
| `--ingested-at-before` | `string` | Filter by ingested_at before (ISO date-time) |
| `--limit` | `int` | Limit number of results (default "100") |
| `--next` | `string` | Pagination cursor for next page |
| `--order-by` | `string` | Sort key (e.g. created_at) |
| `--output` | `string` | Output format (json) |
| `--parsed-query` | `string` | Filter by parsed query (JSON string) |
| `--path` | `string` | Filter by path |
| `--prev` | `string` | Pagination cursor for previous page |
| `--rejection-cause` | `string` | Filter by rejection cause |
| `--source-id` | `string` | Filter by source ID |
| `--status` | `string` | Filter by status |
| `--verified` | `string` | Filter by verified (true/false) |

**Examples:**

```bash
hookdeck gateway request list
hookdeck gateway request list --source-id src_abc123 --limit 20
```
### hookdeck gateway request get

Get detailed information about a request by ID.

**Usage:**

```bash
hookdeck gateway request get <request-id> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--output` | `string` | Output format (json) |

**Examples:**

```bash
hookdeck gateway request get req_abc123
```
### hookdeck gateway request retry

Retry a request by ID. By default retries on all connections. Use `--connection-ids` to retry only for specific connections.

**Usage:**

```bash
hookdeck gateway request retry <request-id> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--connection-ids` | `string` | Comma-separated connection IDs to retry (omit to retry all) |

**Examples:**

```bash
hookdeck gateway request retry req_abc123
hookdeck gateway request retry req_abc123 --connection-ids web_1,web_2
```
### hookdeck gateway request events

List events (deliveries) created from a request.

**Usage:**

```bash
hookdeck gateway request events <request-id> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--limit` | `int` | Limit number of results (default "100") |
| `--next` | `string` | Pagination cursor for next page |
| `--output` | `string` | Output format (json) |
| `--prev` | `string` | Pagination cursor for previous page |

**Examples:**

```bash
hookdeck gateway request events req_abc123
```
### hookdeck gateway request ignored-events

List ignored events for a request (e.g. filtered out or deduplicated).

**Usage:**

```bash
hookdeck gateway request ignored-events <request-id> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--limit` | `int` | Limit number of results (default "100") |
| `--next` | `string` | Pagination cursor for next page |
| `--output` | `string` | Output format (json) |
| `--prev` | `string` | Pagination cursor for previous page |

**Examples:**

```bash
hookdeck gateway request ignored-events req_abc123
```
### hookdeck gateway request raw-body

Output the raw request body of a request by ID.

**Usage:**

```bash
hookdeck gateway request raw-body <request-id>
```

**Examples:**

```bash
hookdeck gateway request raw-body req_abc123
```
<!-- GENERATE_END -->
## Attempts

<!-- GENERATE:gateway attempt list|gateway attempt get:START -->
- [hookdeck gateway attempt list](#hookdeck-gateway-attempt-list)
- [hookdeck gateway attempt get](#hookdeck-gateway-attempt-get)

### hookdeck gateway attempt list

List attempts for an event. Requires `--event-id`.

**Usage:**

```bash
hookdeck gateway attempt list [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--dir` | `string` | Sort direction (asc, desc) |
| `--event-id` | `string` | Filter by event ID (required) |
| `--limit` | `int` | Limit number of results (default "100") |
| `--next` | `string` | Pagination cursor for next page |
| `--order-by` | `string` | Sort key |
| `--output` | `string` | Output format (json) |
| `--prev` | `string` | Pagination cursor for previous page |

**Examples:**

```bash
hookdeck gateway attempt list --event-id evt_abc123
```
### hookdeck gateway attempt get

Get detailed information about an attempt by ID.

**Usage:**

```bash
hookdeck gateway attempt get <attempt-id> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--output` | `string` | Output format (json) |

**Examples:**

```bash
hookdeck gateway attempt get atm_abc123
```
<!-- GENERATE_END -->
## Utilities

<!-- GENERATE:completion|ci:START -->
- [hookdeck completion](#hookdeck-completion)
- [hookdeck ci](#hookdeck-ci)

### hookdeck completion

Generate bash and zsh completion scripts

**Usage:**

```bash
hookdeck completion [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--shell` | `string` | The shell to generate completion commands for. Supports "bash" or "zsh" |
### hookdeck ci

Login to your Hookdeck project to forward events in CI

**Usage:**

```bash
hookdeck ci [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--name` | `string` | Your CI name (ex: $GITHUB_REF) |
<!-- GENERATE_END -->