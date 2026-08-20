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
- [Metrics](#metrics)
- [Outpost](#outpost)
- [Utilities](#utilities)
<!-- GENERATE_END -->
## Global Options

All commands support these global options:

<!-- GENERATE_GLOBAL_FLAGS:START -->
| Flag | Type | Description |
|------|------|-------------|
| `--color` | `string` | turn on/off color output (on, off, auto) |
| `--device-name` | `string` | device name |
| `--hookdeck-config` | `string` | path to CLI config file (default is $HOME/.config/hookdeck/config.toml) |
| `--insecure` | `bool` | Allow invalid TLS certificates |
| `--log-level` | `string` | log level (debug, info, warn, error) (default "info") |
| `-p, --profile` | `string` | profile name (default "default") |
| `-v, --version` | `bool` | Get the version of the Hookdeck CLI |

<!-- GENERATE_END -->
## Authentication

<!-- GENERATE:login|logout|whoami:START -->
- [Login](#login)
- [Logout](#logout)
- [Whoami](#whoami)

## Login

Login to your Hookdeck account to setup the CLI.

With a guest Console profile (after hookdeck listen), hookdeck login opens the browser to sign you
up and keep your sandbox data.

Use `--cli-key` with a claimed CLI client key from the Hookdeck product (for example Event Gateway
onboarding or Console CLI authorization). The CLI validates the key and saves your config, replacing
a guest profile when present. Device login (hookdeck login without `--cli-key`) is required for guest
upgrade and sandbox retention.

**Usage:**

```bash
hookdeck login [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--cli-key` | `string` | CLI key from Hookdeck dashboard onboarding |
| `-i, --interactive` | `bool` | Run interactive configuration mode if you cannot open a browser |
| `--local` | `bool` | Save credentials to current directory (.hookdeck/config.toml) |

**Examples:**

```bash
$ hookdeck login
$ hookdeck login --cli-key <key>
$ hookdeck logout && hookdeck login  # existing Platform account
$ hookdeck login -i  # interactive mode (no browser)
$ hookdeck login --local  # save credentials to .hookdeck/config.toml
```
## Logout

Logout of your Hookdeck account to setup the CLI

**Usage:**

```bash
hookdeck logout [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `-a, --all` | `bool` | Clear credentials for all projects you are currently logged into. |

**Examples:**

```bash
$ hookdeck logout
$ hookdeck logout -a  # clear all projects
```
## Whoami

Show the logged-in user

**Usage:**

```bash
hookdeck whoami
```

**Examples:**

```bash
$ hookdeck whoami
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
hookdeck project list [<organization_substring>] [<project_substring>] [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--output` | `string` | Output format: json |
| `--type` | `string` | Filter by project type: gateway, outpost, console |

**Examples:**

```bash
$ hookdeck project list
Acme / Ecommerce Production (current) | Gateway
Acme / Ecommerce Staging | Gateway
$ hookdeck project list --output json
$ hookdeck project list --type gateway
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

**Examples:**

```bash
$ hookdeck project use
Use the arrow keys to navigate: ↓ ↑ → ←
? Select Project:
▸ Acme / Ecommerce Production (current) | Gateway
Acme / Ecommerce Staging | Gateway

$ hookdeck project use --local
Pinning project to current directory
```
<!-- GENERATE_END -->
## Local Development

<!-- GENERATE:listen:START -->
## Listen

Forward events for one or more sources to your local server.

You can listen to a single source, a comma-separated list of sources, or
"*" to listen to all of your sources at once.

This command will create a new Hookdeck Source if it doesn't exist (single
source only).

By default the Hookdeck Destination will be named "{source}-cli", and the
Destination CLI path will be "/". To set the CLI path, use the "`--path`" flag.

Authentication order: "`--cli-key`", then stored credentials from "hookdeck login"
or "hookdeck ci", then HOOKDECK_API_KEY. Setting HOOKDECK_API_KEY to a Project
API key is enough to run in CI — the CLI exchanges it for CLI credentials and
saves them. With none of these, a temporary guest account is created, which has
no delivery history, retries, or issue triggers.

One exception: HOOKDECK_API_KEY does take precedence over a stored *guest*
profile, so a machine that once ran "listen" without credentials still uses your
project when the variable is set. Replacing a guest profile is announced, and
discards the link to that sandbox — unset HOOKDECK_API_KEY to keep it.

Without a terminal (CI, Docker, nohup, an AI agent) the interactive UI cannot
run, so "`--output`" falls back to "compact" automatically.

**Usage:**

```bash
hookdeck listen [port or forwarding URL] [source(s)] [connection] [flags]
```

**Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `port or forwarding URL` | `string` | **Required.** Port (e.g. 3000) or full URL (e.g. http://localhost:3000) to forward events to. The forward URL will be http://localhost:$PORT/$DESTINATION_PATH or http://domain/$DESTINATION_PATH. Only one of port or domain is required. |
| `source` | `string` | **Optional.** The name of a source to listen to, a comma-separated list of source names, or '*' (with quotes) to listen to all. If empty, the CLI prompts you to choose. |
| `connection` | `string` | **Optional.** Filter connections by connection name or path. |

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--cli-key` | `string` | Hookdeck CLI key used to authenticate this command, e.g. the key shown in the Hookdeck Console |
| `--filter-body` | `string` | Filter events by request body using Hookdeck filter syntax (JSON) |
| `--filter-headers` | `string` | Filter events by request headers using Hookdeck filter syntax (JSON) |
| `--filter-path` | `string` | Filter events by request path using Hookdeck filter syntax (JSON) |
| `--filter-query` | `string` | Filter events by query parameters using Hookdeck filter syntax (JSON) |
| `--max-connections` | `int` | Maximum concurrent connections to local endpoint (default: 50, increase for high-volume testing) (default "50") |
| `--no-healthcheck` | `bool` | Disable periodic health checks of the local server |
| `--output` | `string` | Output mode: interactive (full UI), compact (simple logs), quiet (errors and warnings only). Falls back to compact automatically when there is no terminal. (default "interactive") |
| `--path` | `string` | Sets the path to which events are forwarded e.g., /webhooks or /api/stripe |
<!-- GENERATE_END -->
## Gateway

<!-- GENERATE:gateway:START -->
## Gateway

Commands for managing Event Gateway sources, destinations, connections,
transformations, events, requests, metrics, and MCP server.

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

# Start the MCP server for AI agent access
hookdeck gateway mcp
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
hookdeck gateway connection list

# Filter by connection name
hookdeck gateway connection list --name my-connection

# Filter by source ID
hookdeck gateway connection list --source-id src_abc123

# Filter by destination ID
hookdeck gateway connection list --destination-id dst_def456

# Include disabled connections
hookdeck gateway connection list --disabled

# Limit results
hookdeck gateway connection list --limit 10
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
| `--rule-filter-body` | `string` | Filter on request body using Hookdeck filter syntax (JSON) |
| `--rule-filter-headers` | `string` | Filter on request headers using Hookdeck filter syntax (JSON) |
| `--rule-filter-path` | `string` | Filter on request path using Hookdeck filter syntax (JSON) |
| `--rule-filter-query` | `string` | Filter on request query parameters using Hookdeck filter syntax (JSON) |
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
hookdeck gateway connection create \
--name "test-webhooks-to-local" \
--source-type WEBHOOK --source-name "test-webhooks" \
--destination-type CLI --destination-name "local-dev"

# Create with existing resources
hookdeck gateway connection create \
--name "github-to-api" \
--source-id src_abc123 \
--destination-id dst_def456

# Create with source configuration options
hookdeck gateway connection create \
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

**Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `connection-id-or-name` | `string` | **Required.** Connection ID or name |

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--include-destination-auth` | `bool` | Include destination authentication credentials in the response |
| `--include-source-auth` | `bool` | Include source authentication credentials in the response |
| `--output` | `string` | Output format (json) |

**Examples:**

```bash
# Get connection by ID
hookdeck gateway connection get conn_abc123

# Get connection by name
hookdeck gateway connection get my-connection
```
### hookdeck gateway connection update

Update an existing connection by its ID.

Unlike upsert (which uses name as identifier), update takes a connection ID
and allows changing any field including the connection name.

**Usage:**

```bash
hookdeck gateway connection update <connection-id> [flags]
```

**Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `connection-id` | `string` | **Required.** Connection ID |

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
| `--rule-filter-body` | `string` | Filter on request body using Hookdeck filter syntax (JSON) |
| `--rule-filter-headers` | `string` | Filter on request headers using Hookdeck filter syntax (JSON) |
| `--rule-filter-path` | `string` | Filter on request path using Hookdeck filter syntax (JSON) |
| `--rule-filter-query` | `string` | Filter on request query parameters using Hookdeck filter syntax (JSON) |
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

**Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `connection-id` | `string` | **Required.** Connection ID |

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--force` | `bool` | Force delete without confirmation |

**Examples:**

```bash
# Delete a connection (with confirmation)
hookdeck gateway connection delete conn_abc123

# Force delete without confirmation
hookdeck gateway connection delete conn_abc123 --force
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

**Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `name` | `string` | **Required.** Connection name (create or update by name) |

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
| `--destination-cli-path` | `string` | CLI path for CLI destinations (default: / for new connections) |
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
| `--rule-filter-body` | `string` | Filter on request body using Hookdeck filter syntax (JSON) |
| `--rule-filter-headers` | `string` | Filter on request headers using Hookdeck filter syntax (JSON) |
| `--rule-filter-path` | `string` | Filter on request path using Hookdeck filter syntax (JSON) |
| `--rule-filter-query` | `string` | Filter on request query parameters using Hookdeck filter syntax (JSON) |
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
hookdeck gateway connection upsert "my-connection" \
--source-name "stripe-prod" --source-type STRIPE \
--destination-name "my-api" --destination-type HTTP --destination-url https://api.example.com

# Update just the rate limit on an existing connection
hookdeck gateway connection upsert my-connection \
--destination-rate-limit 100 --destination-rate-limit-period minute

# Update source configuration options
hookdeck gateway connection upsert my-connection \
--source-allowed-http-methods "POST,PUT,DELETE" \
--source-custom-response-content-type "json" \
--source-custom-response-body '{"status":"received"}'

# Preview changes without applying them
hookdeck gateway connection upsert my-connection \
--destination-rate-limit 200 --destination-rate-limit-period hour \
--dry-run
```
### hookdeck gateway connection enable

Enable a disabled connection.

**Usage:**

```bash
hookdeck gateway connection enable <connection-id>
```

**Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `connection-id` | `string` | **Required.** Connection ID |
### hookdeck gateway connection disable

Disable an active connection. It will stop receiving new events until re-enabled.

**Usage:**

```bash
hookdeck gateway connection disable <connection-id>
```

**Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `connection-id` | `string` | **Required.** Connection ID |
### hookdeck gateway connection pause

Pause a connection temporarily.

The connection will queue incoming events until unpaused.

**Usage:**

```bash
hookdeck gateway connection pause <connection-id-or-name>
```

**Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `connection-id-or-name` | `string` | **Required.** Connection ID or name |

**Examples:**

```bash
# Pause by connection ID
hookdeck gateway connection pause web_abc123

# Pause by connection name
hookdeck gateway connection pause my-connection
```
### hookdeck gateway connection unpause

Resume a paused connection.

The connection will start processing queued events.

**Usage:**

```bash
hookdeck gateway connection unpause <connection-id-or-name>
```

**Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `connection-id-or-name` | `string` | **Required.** Connection ID or name |

**Examples:**

```bash
# Unpause by connection ID
hookdeck gateway connection unpause web_abc123

# Unpause by connection name
hookdeck gateway connection unpause my-connection
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
| `--api-key` | `string` | API key for source authentication |
| `--basic-auth-pass` | `string` | Password for Basic authentication |
| `--basic-auth-user` | `string` | Username for Basic authentication |
| `--config` | `string` | JSON object for source config (overrides individual flags if set) |
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
| `--api-key` | `string` | API key for source authentication |
| `--basic-auth-pass` | `string` | Password for Basic authentication |
| `--basic-auth-user` | `string` | Username for Basic authentication |
| `--config` | `string` | JSON object for source config (overrides individual flags if set) |
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
| `--api-key` | `string` | API key for source authentication |
| `--basic-auth-pass` | `string` | Password for Basic authentication |
| `--basic-auth-user` | `string` | Username for Basic authentication |
| `--config` | `string` | JSON object for source config (overrides individual flags if set) |
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
| `--api-key` | `string` | API key for destination auth |
| `--api-key-header` | `string` | Header/key name for API key |
| `--api-key-to` | `string` | Where to send API key (header or query) (default "header") |
| `--auth-method` | `string` | Auth method (hookdeck, bearer, basic, api_key, custom_signature) |
| `--basic-auth-pass` | `string` | Password for Basic auth |
| `--basic-auth-user` | `string` | Username for Basic auth |
| `--bearer-token` | `string` | Bearer token for destination auth |
| `--cli-path` | `string` | Path for CLI destinations (default "/") |
| `--config` | `string` | JSON object for destination config (overrides individual flags if set) |
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
| `--api-key` | `string` | API key for destination auth |
| `--api-key-header` | `string` | Header/key name for API key |
| `--api-key-to` | `string` | Where to send API key (header or query) (default "header") |
| `--auth-method` | `string` | Auth method (hookdeck, bearer, basic, api_key, custom_signature) |
| `--basic-auth-pass` | `string` | Password for Basic auth |
| `--basic-auth-user` | `string` | Username for Basic auth |
| `--bearer-token` | `string` | Bearer token for destination auth |
| `--cli-path` | `string` | Path for CLI destinations |
| `--config` | `string` | JSON object for destination config (overrides individual flags if set) |
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
| `--api-key` | `string` | API key for destination auth |
| `--api-key-header` | `string` | Header/key name for API key |
| `--api-key-to` | `string` | Where to send API key (header or query) (default "header") |
| `--auth-method` | `string` | Auth method (hookdeck, bearer, basic, api_key, custom_signature) |
| `--basic-auth-pass` | `string` | Password for Basic auth |
| `--basic-auth-user` | `string` | Username for Basic auth |
| `--bearer-token` | `string` | Bearer token for destination auth |
| `--cli-path` | `string` | Path for CLI destinations |
| `--config` | `string` | JSON object for destination config (overrides individual flags if set) |
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

Use `--search-term` to match a value partially against the body, headers, parsed query or path
at once, when you know the value but not which field carries it.

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
| `--delivery-group` | `string` | Filter by delivery group (comma-separated) |
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
| `--next-attempt-at-after` | `string` | Filter by next_attempt_at after (ISO date-time) |
| `--next-attempt-at-before` | `string` | Filter by next_attempt_at before (ISO date-time) |
| `--order-by` | `string` | Sort key (e.g. created_at) |
| `--output` | `string` | Output format (json) |
| `--parsed-query` | `string` | Filter by parsed query (JSON string) |
| `--path` | `string` | Filter by path |
| `--prev` | `string` | Pagination cursor for previous page |
| `--response-status` | `string` | Filter by HTTP response status (e.g. 200, 500) |
| `--search-term` | `string` | Partial match against body, headers, parsed query or path (min 3 characters) |
| `--source-id` | `string` | Filter by source ID |
| `--status` | `string` | Filter by status (SCHEDULED, QUEUED, HOLD, SUCCESSFUL, FAILED, CANCELLED) |
| `--successful-at-after` | `string` | Filter by successful_at after (ISO date-time) |
| `--successful-at-before` | `string` | Filter by successful_at before (ISO date-time) |

**Examples:**

```bash
hookdeck gateway event list
hookdeck gateway event list --connection-id web_abc123
hookdeck gateway event list --status FAILED --limit 20
hookdeck gateway event list --search-term cus_1234
hookdeck gateway event list --status QUEUED --next-attempt-at-before 2026-01-01T00:00:00Z
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

Use `--search-term` to match a value partially against the body, headers, parsed query or path
at once. `--events-count` 0 finds requests that produced no events, which is the usual reason a
webhook appears to have gone missing.

**Usage:**

```bash
hookdeck gateway request list [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--body` | `string` | Filter by body (JSON string) |
| `--cli-events-count` | `string` | Filter by number of CLI events (integer or operators) |
| `--created-after` | `string` | Filter requests created after (ISO date-time) |
| `--created-before` | `string` | Filter requests created before (ISO date-time) |
| `--dir` | `string` | Sort direction (asc, desc) |
| `--events-count` | `string` | Filter by number of events produced (integer or operators) |
| `--headers` | `string` | Filter by headers (JSON string) |
| `--id` | `string` | Filter by request ID(s) (comma-separated) |
| `--ignored-count` | `string` | Filter by number of ignored events (integer or operators) |
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
| `--search-term` | `string` | Partial match against body, headers, parsed query or path (min 3 characters) |
| `--source-id` | `string` | Filter by source ID |
| `--status` | `string` | Filter by status |
| `--verified` | `string` | Filter by verified (true/false) |

**Examples:**

```bash
hookdeck gateway request list
hookdeck gateway request list --source-id src_abc123 --limit 20
hookdeck gateway request list --search-term cus_1234
hookdeck gateway request list --events-count 0
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
## Metrics

Query Event Gateway metrics (events, requests, attempts, queue depth, pending events, events by issue, transformations). All metrics commands require `--start` and `--end` (ISO 8601 date-time).

**Use cases and examples:**

| Use case | Example command |
|----------|-----------------|
| Event volume and failure rate over time | `hookdeck gateway metrics events --start 2026-02-01T00:00:00Z --end 2026-02-25T00:00:00Z --granularity 1d --measures count,failed_count,error_rate` |
| Request acceptance vs rejection | `hookdeck gateway metrics requests --start 2026-02-01T00:00:00Z --end 2026-02-25T00:00:00Z --measures count,accepted_count,rejected_count` |
| Delivery latency (attempts) | `hookdeck gateway metrics attempts --start 2026-02-01T00:00:00Z --end 2026-02-25T00:00:00Z --measures response_latency_avg,response_latency_p95` |
| Queue backlog per destination | `hookdeck gateway metrics queue-depth --start 2026-02-01T00:00:00Z --end 2026-02-25T00:00:00Z --measures max_depth,max_age --destination-id dest_xxx` |
| Pending events over time | `hookdeck gateway metrics pending --start 2026-02-01T00:00:00Z --end 2026-02-25T00:00:00Z --granularity 1h --measures count` |
| Events grouped by issue (debugging) | `hookdeck gateway metrics events-by-issue iss_xxx --start 2026-02-01T00:00:00Z --end 2026-02-25T00:00:00Z --measures count` |
| Transformation errors | `hookdeck gateway metrics transformations --start 2026-02-01T00:00:00Z --end 2026-02-25T00:00:00Z --measures count,failed_count,error_rate` |

**Common flags (all metrics subcommands):** `--start`, `--end` (required), `--granularity` (e.g. 1h, 5m, 1d), `--measures`, `--dimensions`, `--source-id`, `--destination-id`, `--connection-id`, `--status`, `--output` (json).

## Outpost

Manage Hookdeck Outpost — tenants, their destinations, and the events delivered to them. These commands require an Outpost project; use `hookdeck project use` to switch.

Config and credential fields differ per destination type and are defined by the Outpost deployment rather than the CLI, so they are passed as repeatable `key=value` pairs rather than individual flags:

```sh
hookdeck outpost destination create --tenant-id acme --type webhook \
  --config url=https://example.com/hooks
```

Run `hookdeck outpost destination-type get <type>` to see the fields a type accepts, or add `--type <type>` to `--help`:

```sh
hookdeck outpost destination create --type kafka --help
```

Nested values, should a type need them, use dotted paths (`--config a.b=c`), and `--config-file` accepts a JSON object.

**`outpost publish` needs a Hookdeck Project API key.** It is the one command that does not accept the credentials stored by `hookdeck login`; pass `--api-key` or set `HOOKDECK_API_KEY`. Create a Project API key in the Hookdeck dashboard under your project's settings.

<!-- GENERATE:outpost tenant list|outpost tenant get|outpost tenant upsert|outpost tenant delete|outpost tenant token|outpost tenant portal|outpost destination list|outpost destination get|outpost destination create|outpost destination update|outpost destination delete|outpost destination enable|outpost destination disable|outpost destination-type list|outpost destination-type get|outpost event list|outpost event get|outpost event retry|outpost attempt list|outpost attempt get|outpost publish|outpost topic list|outpost metrics events|outpost metrics attempts|outpost config get|outpost config set|outpost config custom-domain get|outpost config custom-domain set|outpost config custom-domain delete|outpost status:START -->
- [hookdeck outpost tenant list](#hookdeck-outpost-tenant-list)
- [hookdeck outpost tenant get](#hookdeck-outpost-tenant-get)
- [hookdeck outpost tenant upsert](#hookdeck-outpost-tenant-upsert)
- [hookdeck outpost tenant delete](#hookdeck-outpost-tenant-delete)
- [hookdeck outpost tenant token](#hookdeck-outpost-tenant-token)
- [hookdeck outpost tenant portal](#hookdeck-outpost-tenant-portal)
- [hookdeck outpost destination list](#hookdeck-outpost-destination-list)
- [hookdeck outpost destination get](#hookdeck-outpost-destination-get)
- [hookdeck outpost destination create](#hookdeck-outpost-destination-create)
- [hookdeck outpost destination update](#hookdeck-outpost-destination-update)
- [hookdeck outpost destination delete](#hookdeck-outpost-destination-delete)
- [hookdeck outpost destination enable](#hookdeck-outpost-destination-enable)
- [hookdeck outpost destination disable](#hookdeck-outpost-destination-disable)
- [hookdeck outpost destination-type list](#hookdeck-outpost-destination-type-list)
- [hookdeck outpost destination-type get](#hookdeck-outpost-destination-type-get)
- [hookdeck outpost event list](#hookdeck-outpost-event-list)
- [hookdeck outpost event get](#hookdeck-outpost-event-get)
- [hookdeck outpost event retry](#hookdeck-outpost-event-retry)
- [hookdeck outpost attempt list](#hookdeck-outpost-attempt-list)
- [hookdeck outpost attempt get](#hookdeck-outpost-attempt-get)
- [hookdeck outpost publish](#hookdeck-outpost-publish)
- [hookdeck outpost topic list](#hookdeck-outpost-topic-list)
- [hookdeck outpost metrics events](#hookdeck-outpost-metrics-events)
- [hookdeck outpost metrics attempts](#hookdeck-outpost-metrics-attempts)
- [hookdeck outpost config get](#hookdeck-outpost-config-get)
- [hookdeck outpost config set](#hookdeck-outpost-config-set)
- [hookdeck outpost config custom-domain get](#hookdeck-outpost-config-custom-domain-get)
- [hookdeck outpost config custom-domain set](#hookdeck-outpost-config-custom-domain-set)
- [hookdeck outpost config custom-domain delete](#hookdeck-outpost-config-custom-domain-delete)
- [hookdeck outpost status](#hookdeck-outpost-status)

### hookdeck outpost tenant list

List tenants in the current Outpost project.

**Usage:**

```bash
hookdeck outpost tenant list [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--dir` | `string` | Sort direction (asc, desc) |
| `--id` | `string` | Filter by tenant ID(s), comma-separated |
| `--limit` | `int` | Limit number of results (1-100) (default "0") |
| `--next` | `string` | Next page cursor |
| `--output` | `string` | Output format (json) |
| `--prev` | `string` | Previous page cursor |

**Examples:**

```bash
# List tenants
hookdeck outpost tenant list

# Fetch specific tenants by ID
hookdeck outpost tenant list --id acme,globex

# Page through results
hookdeck outpost tenant list --limit 20 --next <cursor>
```
### hookdeck outpost tenant get

Get details for a tenant, including how many destinations it has.

**Usage:**

```bash
hookdeck outpost tenant get <tenant-id> [flags]
```

**Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `tenant-id` | `string` | **Required.** The ID of the tenant. |

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--output` | `string` | Output format (json) |

**Examples:**

```bash
# Get a tenant
hookdeck outpost tenant get acme

# As JSON
hookdeck outpost tenant get acme --output json
```
### hookdeck outpost tenant upsert

Create a new tenant or update an existing one by name (idempotent).

Tenant IDs are chosen by you, not generated, so this is the only way to create one.
Re-running with the same ID updates the tenant's metadata rather than failing.

Metadata is replaced wholesale, not merged: pass every key you want to keep.

**Usage:**

```bash
hookdeck outpost tenant upsert <tenant-id> [flags]
```

**Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `tenant-id` | `string` | **Required.** The ID of the tenant to create or update. |

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--metadata` | `stringArray` | Metadata as key=value (repeatable) (default "[]") |
| `--metadata-file` | `string` | Path to a JSON file of metadata key/value pairs |
| `--output` | `string` | Output format (json) |

**Examples:**

```bash
# Create or update a tenant
hookdeck outpost tenant upsert acme

# With metadata
hookdeck outpost tenant upsert acme --metadata plan=pro --metadata region=eu

# Metadata from a JSON file
hookdeck outpost tenant upsert acme --metadata-file ./tenant.json
```
### hookdeck outpost tenant delete

Delete a tenant.

Deleting a tenant also removes its destinations, so events will stop being
delivered on its behalf. This cannot be undone.

**Usage:**

```bash
hookdeck outpost tenant delete <tenant-id> [flags]
```

**Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `tenant-id` | `string` | **Required.** The ID of the tenant to delete. |

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--force` | `bool` | Delete without confirmation |

**Examples:**

```bash
# Delete a tenant, with a confirmation prompt
hookdeck outpost tenant delete acme

# Skip the prompt (for scripts and CI)
hookdeck outpost tenant delete acme --force
```
### hookdeck outpost tenant token

Mint a short-lived JWT scoped to a single tenant.

The token grants access to that tenant's data and is valid for 24 hours. Treat it
as a credential: it is intended for your own backend to hand to a tenant's session,
not to be pasted into a shell history or shared.

[BETA] This feature is in beta. Please share bugs and feedback via:
https://github.com/hookdeck/hookdeck-cli/issues

**Usage:**

```bash
hookdeck outpost tenant token <tenant-id> [flags]
```

**Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `tenant-id` | `string` | **Required.** The ID of the tenant to mint a token for. |

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--output` | `string` | Output format (json) |

**Examples:**

```bash
# Mint a token for a tenant
hookdeck outpost tenant token acme

# As JSON, for piping into another tool
hookdeck outpost tenant token acme --output json
```
### hookdeck outpost tenant portal

Get a redirect URL for a tenant's portal, where they manage their own destinations.

The URL grants access to that tenant's portal session, so treat it as a credential.

This requires a portal custom domain to be configured for the project; see
'hookdeck outpost config custom-domain'.

[BETA] This feature is in beta. Please share bugs and feedback via:
https://github.com/hookdeck/hookdeck-cli/issues

**Usage:**

```bash
hookdeck outpost tenant portal <tenant-id> [flags]
```

**Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `tenant-id` | `string` | **Required.** The ID of the tenant whose portal URL to fetch. |

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--open` | `bool` | Open the portal URL in your browser |
| `--output` | `string` | Output format (json) |
| `--theme` | `string` | Portal theme (light, dark) |

**Examples:**

```bash
# Print the portal URL
hookdeck outpost tenant portal acme

# Open it in a browser
hookdeck outpost tenant portal acme --open

# Request the dark theme
hookdeck outpost tenant portal acme --theme dark
```
### hookdeck outpost destination list

List a tenant's destinations.

This endpoint is not paginated: every destination for the tenant is returned.

**Usage:**

```bash
hookdeck outpost destination list [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--output` | `string` | Output format (json) |
| `--topics` | `string` | Filter by topic(s), comma-separated |
| `--type` | `string` | Filter by destination type(s), comma-separated |

**Examples:**

```bash
# List a tenant's destinations
hookdeck outpost destination list --tenant-id acme

# Filter by type or topic
hookdeck outpost destination list --tenant-id acme --type webhook
hookdeck outpost destination list --tenant-id acme --topics user.created
```
### hookdeck outpost destination get

Get details for a destination, including its config and topics.

**Usage:**

```bash
hookdeck outpost destination get <destination-id> [flags]
```

**Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `destination-id` | `string` | **Required.** The ID of the destination. |

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--output` | `string` | Output format (json) |

**Examples:**

```bash
# Get a destination
hookdeck outpost destination get des_abc123 --tenant-id acme
```
### hookdeck outpost destination create

Create a destination for a tenant.

Config and credential fields depend on `--type`. Pass them as repeatable key=value
pairs; run 'hookdeck outpost destination-type list' to see the available types and
'hookdeck outpost destination-type get <type>' to see the fields one accepts.

Topics default to all ("*") when `--topics` is omitted.

**Usage:**

```bash
hookdeck outpost destination create [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--config` | `stringArray` | Config field as key=value (repeatable), e.g. `--config` url=https://example.com (default "[]") |
| `--config-file` | `string` | Path to a JSON file of config fields |
| `--credential` | `stringArray` | Credential field as key=value (repeatable) (default "[]") |
| `--credentials-file` | `string` | Path to a JSON file of credential fields |
| `--filter` | `string` | Event filter as a JSON object |
| `--filter-file` | `string` | Path to a JSON file containing an event filter |
| `--metadata` | `stringArray` | Metadata as key=value (repeatable) (default "[]") |
| `--metadata-file` | `string` | Path to a JSON file of metadata key/value pairs |
| `--output` | `string` | Output format (json) |
| `--topics` | `string` | Topics to subscribe to, comma-separated, or "*" for all |
| `--type` | `string` | Destination type (required) |

**Examples:**

```bash
# A webhook destination subscribed to everything
hookdeck outpost destination create --tenant-id acme --type webhook \
--config url=https://example.com/hooks

# Subscribed to specific topics
hookdeck outpost destination create --tenant-id acme --type webhook \
--config url=https://example.com/hooks --topics user.created,user.updated

# With credentials and a filter
hookdeck outpost destination create --tenant-id acme --type aws_sqs \
--config queue_url=https://sqs.eu-west-2.amazonaws.com/1/q \
--credential key=AKIA... --credential secret=... \
--filter '{"data":{"tier":"pro"}}'

# With metadata of your own to correlate against your systems
hookdeck outpost destination create --tenant-id acme --type webhook \
--config url=https://example.com/hooks \
--metadata owner=platform --metadata tier=pro
```
### hookdeck outpost destination update

Update an existing destination by its ID.

Only the fields you pass are changed; omitted fields are left alone.

`--filter` and `--metadata` are the exceptions: the API replaces each wholesale
rather than merging into it, so pass the complete value you want.

**Usage:**

```bash
hookdeck outpost destination update <destination-id> [flags]
```

**Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `destination-id` | `string` | **Required.** The ID of the destination to update. |

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--config` | `stringArray` | Config field as key=value (repeatable), e.g. `--config` url=https://example.com (default "[]") |
| `--config-file` | `string` | Path to a JSON file of config fields |
| `--credential` | `stringArray` | Credential field as key=value (repeatable) (default "[]") |
| `--credentials-file` | `string` | Path to a JSON file of credential fields |
| `--filter` | `string` | Event filter as a JSON object |
| `--filter-file` | `string` | Path to a JSON file containing an event filter |
| `--metadata` | `stringArray` | Metadata as key=value (repeatable) (default "[]") |
| `--metadata-file` | `string` | Path to a JSON file of metadata key/value pairs |
| `--output` | `string` | Output format (json) |
| `--topics` | `string` | Topics to subscribe to, comma-separated, or "*" for all |

**Examples:**

```bash
# Point a destination at a new URL
hookdeck outpost destination update des_abc123 --tenant-id acme \
--config url=https://example.com/new

# Change which topics it receives
hookdeck outpost destination update des_abc123 --tenant-id acme --topics "*"

# Replace the metadata
hookdeck outpost destination update des_abc123 --tenant-id acme \
--metadata owner=platform --metadata tier=pro
```
### hookdeck outpost destination delete

Delete a destination.

Events will stop being delivered to it. To stop delivery temporarily and keep the
destination, use 'disable' instead.

**Usage:**

```bash
hookdeck outpost destination delete <destination-id> [flags]
```

**Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `destination-id` | `string` | **Required.** The ID of the destination to delete. |

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--force` | `bool` | Delete without confirmation |

**Examples:**

```bash
# Delete a destination, with a confirmation prompt
hookdeck outpost destination delete des_abc123 --tenant-id acme

# Skip the prompt (for scripts and CI)
hookdeck outpost destination delete des_abc123 --tenant-id acme --force
```
### hookdeck outpost destination enable

Enable a disabled destination.

**Usage:**

```bash
hookdeck outpost destination enable <destination-id> [flags]
```

**Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `destination-id` | `string` | **Required.** The ID of the destination to enable. |

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--output` | `string` | Output format (json) |

**Examples:**

```bash
# Resume delivery to a destination
hookdeck outpost destination enable des_abc123 --tenant-id acme
```
### hookdeck outpost destination disable

Disable an active destination. It will stop receiving new events until re-enabled.

The destination and its configuration are kept, so 'enable' resumes delivery.

**Usage:**

```bash
hookdeck outpost destination disable <destination-id> [flags]
```

**Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `destination-id` | `string` | **Required.** The ID of the destination to disable. |

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--output` | `string` | Output format (json) |

**Examples:**

```bash
# Pause delivery to a destination
hookdeck outpost destination disable des_abc123 --tenant-id acme
```
### hookdeck outpost destination-type list

List the destination types available in this project.

**Usage:**

```bash
hookdeck outpost destination-type list [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--output` | `string` | Output format (json) |

**Examples:**

```bash
# List available destination types
hookdeck outpost destination-type list
```
### hookdeck outpost destination-type get

Show the config and credential fields a destination type accepts.

Each field lists whether it is required, whether it is sensitive, and any values
or format the schema constrains it to.

**Usage:**

```bash
hookdeck outpost destination-type get <type> [flags]
```

**Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `type` | `string` | **Required.** The destination type to describe (e.g. webhook, aws_sqs). |

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--output` | `string` | Output format (json) |

**Examples:**

```bash
# Show the fields a webhook destination accepts
hookdeck outpost destination-type get webhook
```
### hookdeck outpost event list

List published events, most recent first.

Filters are combined with AND. Time bounds are ISO 8601 datetimes.

**Usage:**

```bash
hookdeck outpost event list [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--destination-id` | `string` | Filter by matched destination ID(s), comma-separated |
| `--dir` | `string` | Sort direction (asc, desc) |
| `--id` | `string` | Filter by event ID(s), comma-separated |
| `--limit` | `int` | Limit number of results (default "0") |
| `--next` | `string` | Next page cursor |
| `--order-by` | `string` | Field to sort by (time) |
| `--output` | `string` | Output format (json) |
| `--prev` | `string` | Previous page cursor |
| `--tenant-id` | `string` | Filter by tenant ID(s), comma-separated |
| `--time-after` | `string` | Only events at or after this ISO 8601 datetime |
| `--time-before` | `string` | Only events at or before this ISO 8601 datetime |
| `--topic` | `string` | Filter by topic(s), comma-separated |

**Examples:**

```bash
# Recent events
hookdeck outpost event list --limit 10

# For one tenant, on one topic
hookdeck outpost event list --tenant-id acme --topic user.created

# Within a time window
hookdeck outpost event list --time-after 2026-08-01T00:00:00Z --time-before 2026-08-14T00:00:00Z
```
### hookdeck outpost event get

Get an event, including the payload that was published.

**Usage:**

```bash
hookdeck outpost event get <event-id> [flags]
```

**Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `event-id` | `string` | **Required.** The ID of the event. |

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--output` | `string` | Output format (json) |
| `--tenant-id` | `string` | Tenant the event belongs to |

**Examples:**

```bash
# Get an event
hookdeck outpost event get evt_abc123

# Get the payload alone
hookdeck outpost event get evt_abc123 --output json | jq .data
```
### hookdeck outpost event retry

Deliver an event to a destination again.

The retry is queued rather than performed inline, so a successful response means
it was accepted, not that it has been delivered. Use 'hookdeck outpost attempt
list' to see the outcome.

The destination must be enabled and must subscribe to the event's topic.

[BETA] This feature is in beta. Please share bugs and feedback via:
https://github.com/hookdeck/hookdeck-cli/issues

**Usage:**

```bash
hookdeck outpost event retry [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--destination-id` | `string` | The destination to deliver to (required) |
| `--event-id` | `string` | The event to retry (required) |
| `--output` | `string` | Output format (json) |

**Examples:**

```bash
# Retry one delivery
hookdeck outpost event retry --event-id evt_abc123 --destination-id des_abc123
```
### hookdeck outpost attempt list

List delivery attempts, most recent first.

Passing both `--tenant-id` and `--destination-id` narrows to that destination
specifically; the filters and results are otherwise the same.

**Usage:**

```bash
hookdeck outpost attempt list [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--destination-id` | `string` | Filter by destination ID(s), comma-separated |
| `--destination-type` | `string` | Filter by destination type(s), comma-separated |
| `--dir` | `string` | Sort direction (asc, desc) |
| `--event-id` | `string` | Filter by event ID(s), comma-separated |
| `--include` | `string` | Include related data, comma-separated (event, event.data, response_data, destination) |
| `--limit` | `int` | Limit number of results (default "0") |
| `--next` | `string` | Next page cursor |
| `--order-by` | `string` | Field to sort by |
| `--output` | `string` | Output format (json) |
| `--prev` | `string` | Previous page cursor |
| `--status` | `string` | Filter by status (success, failed) |
| `--tenant-id` | `string` | Filter by tenant ID(s), comma-separated |
| `--time-after` | `string` | Only attempts at or after this ISO 8601 datetime |
| `--time-before` | `string` | Only attempts at or before this ISO 8601 datetime |
| `--topic` | `string` | Filter by topic(s), comma-separated |

**Examples:**

```bash
# Recent failures
hookdeck outpost attempt list --status failed --limit 20

# Every attempt for one event
hookdeck outpost attempt list --event-id evt_abc123

# Include the response body the destination returned
hookdeck outpost attempt list --event-id evt_abc123 --include response_data --output json
```
### hookdeck outpost attempt get

Get a delivery attempt, including the destination's response.

**Usage:**

```bash
hookdeck outpost attempt get <attempt-id> [flags]
```

**Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `attempt-id` | `string` | **Required.** The ID of the delivery attempt. |

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--destination-id` | `string` | Destination the attempt targeted |
| `--include` | `string` | Include related data, comma-separated (event, event.data, response_data, destination) |
| `--output` | `string` | Output format (json) |
| `--tenant-id` | `string` | Tenant the attempt belongs to |

**Examples:**

```bash
# Get an attempt with the response body
hookdeck outpost attempt get att_abc123 --include response_data --output json
```
### hookdeck outpost publish

Publish an event to a topic, for delivery to a tenant's matching destinations.

Publishing is asynchronous: a successful response means the event was accepted,
not that it has been delivered.

This command needs a Hookdeck Project API key, which is different from every
other outpost command. The credentials stored by 'hookdeck login' are not
accepted by the publish API, so pass `--api-key` or set HOOKDECK_API_KEY. You can
create a Project API key in the Hookdeck dashboard under project settings.

[BETA] This feature is in beta. Please share bugs and feedback via:
https://github.com/hookdeck/hookdeck-cli/issues

**Usage:**

```bash
hookdeck outpost publish [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--api-key` | `string` | Hookdeck Project API key. Read from HOOKDECK_API_KEY when not provided. |
| `--data` | `string` | Event payload as a JSON object |
| `--data-file` | `string` | Path to a JSON file containing the event payload |
| `--destination-id` | `string` | Deliver only to this destination |
| `--eligible-for-retry` | `bool` | Whether failed deliveries should be retried (default "true") |
| `--event-id` | `string` | Event ID, for idempotent publishing |
| `--metadata` | `stringArray` | Metadata as key=value (repeatable) (default "[]") |
| `--output` | `string` | Output format (json) |
| `--tenant-id` | `string` | Tenant to publish for (required) |
| `--topic` | `string` | Topic to publish to (required) |

**Examples:**

```bash
# Publish an event
hookdeck outpost publish --tenant-id acme --topic user.created \
--data '{"user_id":"123"}' --api-key $HOOKDECK_API_KEY

# Publish to one specific destination
hookdeck outpost publish --tenant-id acme --topic user.created \
--data '{"user_id":"123"}' --destination-id des_abc123

# Idempotent publish: repeating the same --event-id will not duplicate
hookdeck outpost publish --tenant-id acme --topic user.created \
--event-id my-unique-id --data-file ./payload.json
```
### hookdeck outpost topic list

List the topics configured for this project.

**Usage:**

```bash
hookdeck outpost topic list [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--output` | `string` | Output format (json) |

**Examples:**

```bash
# List topics
hookdeck outpost topic list
```
### hookdeck outpost metrics events

Aggregated event publish metrics.

Measures: count, rate

Dimensions: tenant_id, topic, destination_id

Omit `--granularity` for a single total over the whole range; set it (1h, 5m, 1d)
to bucket the results over time.

[BETA] This feature is in beta. Please share bugs and feedback via:
https://github.com/hookdeck/hookdeck-cli/issues

**Usage:**

```bash
hookdeck outpost metrics events [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--dimensions` | `string` | Dimensions to group by, comma-separated |
| `--end` | `string` | End of the range, ISO 8601 (required) |
| `--filter` | `stringArray` | Filter as dimension=value (repeatable) (default "[]") |
| `--granularity` | `string` | Bucket size (e.g. 5m, 1h, 1d) |
| `--measures` | `string` | Measures to compute, comma-separated (required) |
| `--output` | `string` | Output format (json) |
| `--start` | `string` | Start of the range, ISO 8601 (required) |

**Examples:**

```bash
# Total over the last week
hookdeck outpost metrics events --start 2026-08-07T00:00:00Z --end 2026-08-14T00:00:00Z --measures count

# Bucketed hourly and grouped by topic
hookdeck outpost metrics events --start 2026-08-13T00:00:00Z --end 2026-08-14T00:00:00Z \
--measures count --granularity 1h --dimensions topic
```
### hookdeck outpost metrics attempts

Aggregated delivery attempt metrics.

Measures: count, successful_count, failed_count, error_rate, first_attempt_count, retry_count, manual_retry_count, avg_attempt_number, rate, successful_rate, failed_rate

Dimensions: tenant_id, destination_id, destination_type, topic, status, code, manual, attempt_number

Omit `--granularity` for a single total over the whole range; set it (1h, 5m, 1d)
to bucket the results over time.

[BETA] This feature is in beta. Please share bugs and feedback via:
https://github.com/hookdeck/hookdeck-cli/issues

**Usage:**

```bash
hookdeck outpost metrics attempts [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--dimensions` | `string` | Dimensions to group by, comma-separated |
| `--end` | `string` | End of the range, ISO 8601 (required) |
| `--filter` | `stringArray` | Filter as dimension=value (repeatable) (default "[]") |
| `--granularity` | `string` | Bucket size (e.g. 5m, 1h, 1d) |
| `--measures` | `string` | Measures to compute, comma-separated (required) |
| `--output` | `string` | Output format (json) |
| `--start` | `string` | Start of the range, ISO 8601 (required) |

**Examples:**

```bash
# Total over the last week
hookdeck outpost metrics attempts --start 2026-08-07T00:00:00Z --end 2026-08-14T00:00:00Z --measures count

# Bucketed hourly and grouped by topic
hookdeck outpost metrics attempts --start 2026-08-13T00:00:00Z --end 2026-08-14T00:00:00Z \
--measures count --granularity 1h --dimensions topic
```
### hookdeck outpost config get

Show this project's Outpost configuration.

Pass a key to print just that value, which is convenient in scripts. Unset keys
are omitted unless you ask for one by name.

[BETA] This feature is in beta. Please share bugs and feedback via:
https://github.com/hookdeck/hookdeck-cli/issues

**Usage:**

```bash
hookdeck outpost config get [key] [flags]
```

**Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `key` | `string` | **Optional.** A single configuration key to print. |

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--output` | `string` | Output format (json) |

**Examples:**

```bash
# Show everything that is set
hookdeck outpost config get

# Show one value
hookdeck outpost config get TOPICS
```
### hookdeck outpost config set

Change this project's Outpost configuration.

Only the keys you pass are changed. `--unset` returns a key to its default.

This affects delivery for every tenant in the project, so use `--dry-run` first to
see exactly what would change.

Some keys are managed for you and are rejected if set directly; the API says
which when that happens.

[BETA] This feature is in beta. Please share bugs and feedback via:
https://github.com/hookdeck/hookdeck-cli/issues

**Usage:**

```bash
hookdeck outpost config set [KEY=VALUE ...] [flags]
```

**Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `KEY=VALUE` | `string` | **Optional.** Configuration values to set. Repeatable. |

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--config-file` | `string` | Path to a JSON file of configuration values |
| `--dry-run` | `bool` | Show what would change without applying it |
| `--output` | `string` | Output format (json) |
| `--unset` | `stringArray` | Return a key to its default (repeatable) (default "[]") |

**Examples:**

```bash
# Set the topics destinations can subscribe to
hookdeck outpost config set TOPICS=user.created,user.updated

# Preview a change without applying it
hookdeck outpost config set MAX_RETRY_LIMIT=5 --dry-run

# Return a key to its default
hookdeck outpost config set --unset MAX_RETRY_LIMIT
```
### hookdeck outpost config custom-domain get

Show the custom domain configured for the tenant portal, if any.

[BETA] This feature is in beta. Please share bugs and feedback via:
https://github.com/hookdeck/hookdeck-cli/issues

**Usage:**

```bash
hookdeck outpost config custom-domain get [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--output` | `string` | Output format (json) |

**Examples:**

```bash
# Show the configured custom domain
hookdeck outpost config custom-domain get
```
### hookdeck outpost config custom-domain set

Configure a custom hostname for the tenant portal.

The response includes the DNS records to create. The domain is not usable until
they have propagated and been verified.

[BETA] This feature is in beta. Please share bugs and feedback via:
https://github.com/hookdeck/hookdeck-cli/issues

**Usage:**

```bash
hookdeck outpost config custom-domain set <hostname> [flags]
```

**Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `hostname` | `string` | **Required.** The hostname to serve the tenant portal from. |

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--output` | `string` | Output format (json) |

**Examples:**

```bash
# Configure a custom domain
hookdeck outpost config custom-domain set portal.example.com
```
### hookdeck outpost config custom-domain delete

Remove the tenant portal's custom domain.

Tenant portal URLs stop working until another domain is configured.

[BETA] This feature is in beta. Please share bugs and feedback via:
https://github.com/hookdeck/hookdeck-cli/issues

**Usage:**

```bash
hookdeck outpost config custom-domain delete [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--force` | `bool` | Delete without confirmation |

**Examples:**

```bash
# Remove the custom domain, with a confirmation prompt
hookdeck outpost config custom-domain delete

# Skip the prompt (for scripts and CI)
hookdeck outpost config custom-domain delete --force
```
### hookdeck outpost status

Show the status of this project's Outpost deployment.

Worth checking first when something is not behaving: configuration changes take
a short while to reach the deployment, and the status reports when it is still
being applied.

[BETA] This feature is in beta. Please share bugs and feedback via:
https://github.com/hookdeck/hookdeck-cli/issues

**Usage:**

```bash
hookdeck outpost status [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--output` | `string` | Output format (json) |

**Examples:**

```bash
# Check deployment status
hookdeck outpost status
```
<!-- GENERATE_END -->
### Outpost MCP server

`hookdeck outpost mcp` exposes the Outpost resources above as MCP tools, prefixed `outpost_` so it can be configured alongside `hookdeck gateway mcp` in the same client.

It starts **read-only**: each tool advertises only the actions that read data, so an agent is never offered an action it cannot perform. `--allow-write` enables the rest. Two reads are gated with the writes because both return a reusable credential — `outpost_tenants token` mints a tenant-scoped access token, and `outpost_tenants portal` returns a URL granting access to a tenant's portal.

The publish tool is only registered when a Hookdeck Project API key is available, since the publish API does not accept the credentials stored by `hookdeck login`.

<!-- GENERATE:outpost mcp:START -->
### hookdeck outpost mcp

Starts a Model Context Protocol (MCP) server over stdio.

The server exposes Hookdeck Outpost resources — tenants, destinations, events,
attempts, topics, metrics and project configuration — as MCP tools that AI
agents and LLM-based clients can invoke. Tools are prefixed outpost_, so this
server and 'hookdeck gateway mcp' can be configured in the same client.

The server starts read-only: tools advertise only the actions that read data,
so an agent is never offered an action it cannot perform. Pass `--allow-write` to
enable creating, changing and deleting. Two reads count as writes and are also
gated, because both return a reusable credential: 'outpost_tenants token' mints
a tenant-scoped access token, and 'outpost_tenants portal' returns a URL
granting access to a tenant's portal.

Publishing needs a Hookdeck Project API key, which the credentials stored by
'hookdeck login' cannot substitute for. Without one the publish tool is not
registered at all; pass `--publish-api-key` or set HOOKDECK_OUTPOST_PUBLISH_API_KEY.

This deliberately does not read HOOKDECK_API_KEY, which elsewhere in the CLI
means "a key to exchange for CLI credentials". Publishing sends real events to
real destinations and cannot be undone, so it should not be switched on by a
variable that happens to be exported for something else.

If the CLI is already authenticated, all tools are available immediately. If
not, the server still starts and hookdeck_login initiates browser-based sign-in.
Signing in is a Hookdeck operation rather than an Outpost one, so it keeps the
hookdeck_ prefix here as it does in 'hookdeck gateway mcp'.
Protocol traffic uses stdout only (JSON-RPC); status and errors from the CLI
before the server runs go to stderr.

[BETA] This feature is in beta. Please share bugs and feedback via:
https://github.com/hookdeck/hookdeck-cli/issues

**Usage:**

```bash
hookdeck outpost mcp [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--allow-write` | `bool` | Enable tools that create, change or delete data, and that return tenant credentials. Also read from HOOKDECK_MCP_ALLOW_WRITE; the flag wins. |
| `--publish-api-key` | `string` | Hookdeck Project API key, required by the publish tool. Also read from HOOKDECK_OUTPOST_PUBLISH_API_KEY. HOOKDECK_API_KEY is deliberately not used here. |
| `--read-only` | `bool` | Run without write actions. This is the default; the flag is accepted so it can be passed explicitly, and wins over `--allow-write`. |

**Examples:**

```bash
# Start the MCP server, read-only (stdio transport)
hookdeck outpost mcp

# Allow tools that change data
hookdeck outpost mcp --allow-write

# Allow writes, including publishing events
hookdeck outpost mcp --allow-write --publish-api-key $HOOKDECK_OUTPOST_PUBLISH_API_KEY

# Pipe a JSON-RPC initialize request for testing
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","clientInfo":{"name":"test","version":"1.0"},"capabilities":{}}}' | hookdeck outpost mcp
```
<!-- GENERATE_END -->
## Utilities

<!-- GENERATE:completion|ci:START -->
- [Completion](#completion)
- [CI](#ci)

## Completion

Generate a shell completion script for hookdeck.

The script is written to standard output. To enable completions in the
current shell session, source the output:

  $ source <(hookdeck completion `--shell` bash)
  $ source <(hookdeck completion `--shell` zsh)

To install completions permanently, redirect the output to your shell's
completion directory:

  bash:  hookdeck completion `--shell` bash > /usr/local/etc/bash_completion.d/hookdeck
  zsh:   hookdeck completion `--shell` zsh > "${fpath[1]}/_hookdeck"

When installed via Homebrew or Scoop, completions are installed automatically.

**Usage:**

```bash
hookdeck completion [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--shell` | `string` | The shell to generate completion commands for. Supports "bash" or "zsh" |

**Examples:**

```bash
$ hookdeck completion --shell zsh
$ hookdeck completion --shell bash
```
## CI

If you want to use Hookdeck in CI for tests or any other purposes, you can use your HOOKDECK_API_KEY to authenticate and start forwarding events.

**Usage:**

```bash
hookdeck ci [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--api-key` | `string` | Your Hookdeck Project API key. The CLI reads from HOOKDECK_API_KEY if not provided. |
| `--local` | `bool` | Save credentials to current directory (.hookdeck/config.toml) |
| `--name` | `string` | Name of the CI run (ex: GITHUB_REF) for identification in the dashboard |

**Examples:**

```bash
$ hookdeck ci --api-key $HOOKDECK_API_KEY
Done! The Hookdeck CLI is configured in project MyProject

$ hookdeck listen 3000 shopify orders

●── HOOKDECK CLI ──●

Listening on 1 source • 1 connection • [i] Collapse

Shopify Source
│  Requests to → https://hkdk.events/src_DAjaFWyyZXsFdZrTOKpuHnOH
└─ Forwards to → http://localhost:3000/webhooks/shopify/orders (Orders Service)

💡 View dashboard to inspect, retry & bookmark events: https://dashboard.hookdeck.com/events/cli?team_id=...

Events • [↑↓] Navigate ──────────────────────────────────────────────────────────

> 2025-10-12 14:42:55 [200] POST http://localhost:3000/webhooks/shopify/orders (34ms) → https://dashboard.hookdeck.com/events/evt_...

───────────────────────────────────────────────────────────────────────────────
> ✓ Last event succeeded with status 200 | [r] Retry • [o] Open in dashboard • [d] Show data
```
<!-- GENERATE_END -->