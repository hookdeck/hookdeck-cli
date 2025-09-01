# Hookdeck CLI Reference

The Hookdeck CLI provides authentication, basic project management, and local webhook forwarding for your webhook infrastructure. This reference covers all available commands and their usage.

## Table of Contents

### Current Functionality ✅
- [Global Options](#global-options)
- [Authentication](#authentication)
- [Projects](#projects) (list and use only)
- [Local Development](#local-development)
- [CI/CD Integration](#cicd-integration)
- [Utilities](#utilities)
- [Current Limitations](#current-limitations)

### Planned Functionality 🚧
- [Advanced Project Management](#advanced-project-management)
- [Sources](#sources)
- [Destinations](#destinations)
- [Connections](#connections)
- [Transformations](#transformations)
- [Events & Monitoring](#events--monitoring)
- [Issue Triggers](#issue-triggers)
- [Attempts](#attempts)
- [Bookmarks](#bookmarks)
- [Integrations](#integrations)
- [Issues](#issues)
- [Requests](#requests)
- [Bulk Operations](#bulk-operations)
- [Implementation Status](#implementation-status)

## Global Options

All commands support these global options based on the current implementation:

### ✅ Current Global Options
```bash
--profile, -p string     Profile name (default "default")
--api-key string         Your API key to use for the command
--color string           Turn on/off color output (on, off, auto)
--config string          Config file (default is $HOME/.config/hookdeck/config.toml)
--device-name string     Device name for this CLI instance
--log-level string       Log level: debug, info, warn, error (default "info")
--insecure              Allow invalid TLS certificates
--version, -v           Show version information
--help, -h              Show help information
```

### 🚧 Planned Global Options
```bash
--project string         Project ID to use (overrides profile)
--format string          Output format: table, json, yaml (default "table")
```

## Authentication

### ✅ Login
```bash
# Interactive login with prompts
hookdeck login

# Login with API key directly
hookdeck login --api-key your_api_key

# Use different profile
hookdeck login --profile production
```

### ✅ Logout
```bash
# Logout current profile
hookdeck logout

# Logout all profiles
hookdeck logout --all

# Logout specific profile
hookdeck logout --profile production
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

Projects are top-level containers for your webhook infrastructure. Currently, only basic project management is available.

### ✅ List projects
```bash
# List all projects you have access to
hookdeck project list

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

### ✅ Listen for webhooks
```bash
# Start webhook forwarding to localhost
hookdeck listen

# Forward to specific path
hookdeck listen --path /webhooks

# Listen without WebSocket (polling mode)
hookdeck listen --no-wss
```

The `listen` command forwards webhooks from Hookdeck to your local development server, allowing you to test webhook integrations locally.

## CI/CD Integration

### ✅ CI command
```bash
# Run in CI/CD environments
hookdeck ci
```

This command provides CI/CD specific functionality for automated deployments and testing.

## Utilities

### ✅ Shell completion
```bash
# Generate completion for bash
hookdeck completion bash

# Generate completion for zsh  
hookdeck completion zsh

# Generate completion for fish
hookdeck completion fish

# Generate completion for PowerShell
hookdeck completion powershell
```

### ✅ Version information
```bash
hookdeck version

# Short version
hookdeck --version
```

## Current Limitations

The Hookdeck CLI is currently focused on authentication, basic project management, and local development. The following functionality is planned but not yet implemented:

- ❌ **No structured output formats** - Only plain text with ANSI colors
- ❌ **No `--format` flag** - Cannot output JSON, YAML, or tables
- ❌ **No resource management** - Cannot manage sources, destinations, or connections
- ❌ **No transformation management** - Cannot create or manage JavaScript transformations
- ❌ **No event monitoring** - Cannot view or retry webhook events
- ❌ **No bulk operations** - Cannot perform batch operations on resources
- ❌ **No advanced filtering** - Limited query capabilities
- ❌ **No project creation** - Cannot create, update, or delete projects via CLI

---

# 🚧 Planned Functionality

*The following sections document planned functionality that is not yet implemented. This serves as a specification for future development.*

## Implementation Status

| Command Category | Status | Available Commands | Planned Commands |
|------------------|--------|-------------------|------------------|
| Authentication | ✅ **Current** | `login`, `logout`, `whoami` | *None needed* |
| Project Management | 🔄 **Partial** | `list`, `use` | `create`, `get`, `update`, `delete` |
| Local Development | ✅ **Current** | `listen` | *Enhancements planned* |
| CI/CD | ✅ **Current** | `ci` | *Enhancements planned* |
| Source Management | 🚧 **Planned** | *None* | Full CRUD operations |
| Destination Management | 🚧 **Planned** | *None* | Full CRUD operations |
| Connection Management | 🚧 **Planned** | *None* | Full CRUD operations |
| Transformation Management | 🚧 **Planned** | *None* | Full CRUD operations |
| Event Management | 🚧 **Planned** | *None* | List, retry, monitor |
| Issue Trigger Management | 🚧 **Planned** | *None* | Full CRUD, enable/disable |
| Attempt Management | 🚧 **Planned** | *None* | List, get, retry |
| Bookmark Management | 🚧 **Planned** | *None* | Full CRUD, trigger |
| Integration Management | 🚧 **Planned** | *None* | Full CRUD, attach/detach |
| Issue Management | 🚧 **Planned** | *None* | List, get, update, dismiss |
| Request Management | 🚧 **Planned** | *None* | List, get, retry, raw access |
| Bulk Operations | 🚧 **Planned** | *None* | Bulk retry, enable/disable, delete |
| Output Formatting | 🚧 **Planned** | Basic text only | JSON, YAML, table, CSV |

## Advanced Project Management

🚧 **PLANNED FUNCTIONALITY** - Not yet implemented

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

🚧 **PLANNED FUNCTIONALITY** - Not yet implemented

Sources represent the webhook providers that send webhooks to Hookdeck.

### List sources
```bash
# List all sources
hookdeck source list

# Filter by type
hookdeck source list --type STRIPE

# Filter by name pattern
hookdeck source list --name "*prod*"

# Include disabled sources
hookdeck source list --include-disabled

# Output as JSON
hookdeck source list --format json
```

### Get source details
```bash
# Get source by ID
hookdeck source get src_123

# Get source by name
hookdeck source get stripe-webhooks

# Show authentication details
hookdeck source get src_123 --include-auth
```

### Create a source

#### Interactive creation
```bash
# Create with interactive prompts
hookdeck source create
```

#### Platform-specific sources with webhook secrets

##### 1. Stripe - Payment webhooks
```bash
# Create Stripe source with webhook secret
hookdeck source create \
  --name "stripe-prod" \
  --type STRIPE \
  --description "Production Stripe payment webhooks" \
  --webhook-secret "whsec_1a2b3c4d5e6f7g8h9i0j..."

# Use case: Receive payment confirmation, subscription updates, and dispute events
```

##### 2. GitHub - Repository webhooks  
```bash
# Create GitHub source with webhook secret
hookdeck source create \
  --name "github-repo" \
  --type GITHUB \
  --description "Repository event webhooks" \
  --webhook-secret "your_github_webhook_secret"

# Use case: CI/CD triggers, issue tracking, pull request automation
```

##### 3. Shopify - E-commerce webhooks
```bash
# Create Shopify source with webhook secret
hookdeck source create \
  --name "shopify-store" \
  --type SHOPIFY \
  --description "Store order and inventory webhooks" \
  --webhook-secret "your_shopify_webhook_secret"

# Use case: Order processing, inventory sync, customer management
```

##### 4. Slack - Workspace events
```bash
# Create Slack source with signing secret
hookdeck source create \
  --name "slack-workspace" \
  --type SLACK \
  --description "Slack workspace event webhooks" \
  --webhook-secret "your_slack_signing_secret"

# Use case: Bot interactions, message events, workspace activity
```

##### 5. Twilio - Communication webhooks
```bash
# Create Twilio source with auth token
hookdeck source create \
  --name "twilio-sms" \
  --type TWILIO \
  --description "SMS and voice event webhooks" \
  --webhook-secret "your_twilio_auth_token"

# Use case: SMS delivery status, voice call events, messaging analytics
```

#### Cloud service sources

##### 6. AWS SNS - Minimal configuration
```bash
# Create AWS SNS source
hookdeck source create \
  --name "aws-sns-notifications" \
  --type AWS_SNS \
  --description "AWS SNS topic notifications"

# Use case: CloudWatch alerts, S3 events, EC2 state changes
# Note: SNS automatically handles subscription confirmation
```

#### Generic webhook sources with authentication

##### 7. WEBHOOK with HMAC authentication
```bash
# Create webhook source with HMAC signature verification
hookdeck source create \
  --name "api-webhooks" \
  --type WEBHOOK \
  --description "Third-party API webhooks" \
  --auth-type HMAC \
  --auth-secret "your_hmac_secret" \
  --auth-header "X-Signature"

# Use case: Custom API integrations requiring HMAC verification
```

##### 8. WEBHOOK with API Key authentication
```bash
# Create webhook source with API key authentication
hookdeck source create \
  --name "secure-webhooks" \
  --type WEBHOOK \
  --description "API key protected webhooks" \
  --auth-type API_KEY \
  --auth-key "your_api_key" \
  --auth-header "X-API-Key"

# Alternative: API key in query parameter
hookdeck source create \
  --name "query-auth-webhooks" \
  --type WEBHOOK \
  --auth-type API_KEY \
  --auth-key "your_api_key" \
  --auth-query-param "api_key"

# Use case: Internal services requiring API key validation
```

##### 9. WEBHOOK with Basic Authentication
```bash
# Create webhook source with Basic Auth
hookdeck source create \
  --name "basic-auth-webhooks" \
  --type WEBHOOK \
  --description "Basic auth protected webhooks" \
  --auth-type BASIC \
  --auth-username "webhook_user" \
  --auth-password "secure_password"

# Use case: Legacy systems using username/password authentication
```

#### Additional source types

##### 10. HTTP - Generic HTTP source
```bash
# Create generic HTTP source with custom configuration
hookdeck source create \
  --name "http-events" \
  --type HTTP \
  --description "Generic HTTP event receiver" \
  --allowed-methods "POST,PUT,PATCH" \
  --custom-response-body '{"status": "received"}' \
  --custom-response-status 200

# Use case: Custom integrations not covered by specific source types
```

##### 11. Discord - Bot webhooks with public key
```bash
# Create Discord source with public key verification
hookdeck source create \
  --name "discord-bot" \
  --type DISCORD \
  --description "Discord bot interaction webhooks" \
  --public-key "your_discord_public_key"

# Use case: Discord slash commands, button interactions, bot events
```

##### 12. Telnyx - Communication platform
```bash
# Create Telnyx source with public key verification
hookdeck source create \
  --name "telnyx-comms" \
  --type TELNYX \
  --description "Telnyx communication webhooks" \
  --public-key "your_telnyx_public_key"

# Use case: SMS/MMS delivery, voice events, number management
```

#### Advanced configurations

```bash
# Create source with multiple HTTP methods
hookdeck source create \
  --name "flexible-webhooks" \
  --type WEBHOOK \
  --allowed-methods "GET,POST,PUT,DELETE" \
  --description "Multi-method webhook endpoint"

# Create source with custom response
hookdeck source create \
  --name "custom-response" \
  --type WEBHOOK \
  --custom-response-status 201 \
  --custom-response-body '{"message": "Event processed"}' \
  --custom-response-headers "Content-Type=application/json"
```

### Update a source
```bash
# Update interactively
hookdeck source update src_123

# Update name
hookdeck source update src_123 --name "new-name"

# Update webhook secret
hookdeck source update src_123 --webhook-secret "new_secret"
```

### Delete a source
```bash
# Delete with confirmation
hookdeck source delete src_123

# Force delete
hookdeck source delete src_123 --force
```

### Enable/Disable sources
```bash
# Disable source
hookdeck source disable src_123

# Enable source
hookdeck source enable src_123
```

## Destinations

🚧 **PLANNED FUNCTIONALITY** - Not yet implemented

Destinations are the endpoints where webhooks are delivered.

### List destinations
```bash
# List all destinations
hookdeck destination list

# Filter by type
hookdeck destination list --type HTTP

# Output as JSON
hookdeck destination list --format json
```

### Create a destination
```bash
# Create with interactive prompts
hookdeck destination create

# Create HTTP destination
hookdeck destination create \
  --name "my-api" \
  --type HTTP \
  --url "https://api.example.com/webhooks"

# Create with authentication
hookdeck destination create \
  --name "secure-api" \
  --type HTTP \
  --url "https://api.example.com/webhooks" \
  --auth-type BEARER_TOKEN \
  --auth-token "your_token"

# Create CLI destination for local development
hookdeck destination create \
  --name "local-dev" \
  --type CLI \
  --path "/webhooks"

# Create MOCK_API destination for testing
hookdeck destination create \
  --name "test-api" \
  --type MOCK_API \
  --description "Mock API for testing webhooks"

# Create HTTP destination with basic authentication
hookdeck destination create \
  --name "basic-auth-api" \
  --type HTTP \
  --url "https://api.example.com/webhooks" \
  --auth-type BASIC_AUTH \
  --auth-username "api_user" \
  --auth-password "secure_password"

# Create HTTP destination with API key authentication
hookdeck destination create \
  --name "api-key-endpoint" \
  --type HTTP \
  --url "https://api.example.com/webhooks" \
  --auth-type API_KEY \
  --auth-key "your_api_key" \
  --auth-header "X-API-Key"

# Create HTTP destination with custom headers
hookdeck destination create \
  --name "custom-headers-api" \
  --type HTTP \
  --url "https://api.example.com/webhooks" \
  --headers "Content-Type=application/json,X-Custom-Header=value"

# Create HTTP destination with rate limiting
hookdeck destination create \
  --name "rate-limited-api" \
  --type HTTP \
  --url "https://api.example.com/webhooks" \
  --rate-limit 100 \
  --rate-limit-period minute

# Create HTTP destination with OAuth2
hookdeck destination create \
  --name "oauth2-api" \
  --type HTTP \
  --url "https://api.example.com/webhooks" \
  --auth-type OAUTH2_CLIENT_CREDENTIALS \
  --auth-server "https://auth.example.com/token" \
  --client-id "your_client_id" \
  --client-secret "your_client_secret"
```

## Connections

🚧 **PLANNED FUNCTIONALITY** - Not yet implemented

Connections link sources to destinations and define processing rules.

### List connections
```bash
# List all connections
hookdeck connection list

# Filter by source
hookdeck connection list --source stripe-prod

# Filter by destination
hookdeck connection list --destination my-api
```

### Create a connection
```bash
# Create with interactive prompts
hookdeck connection create

# Create connection using existing source and destination IDs
hookdeck connection create \
  --name "stripe-to-api" \
  --source-id src_123 \
  --destination-id dest_456

# Create connection with inline source and destination creation
hookdeck connection create \
  --name "stripe-to-api" \
  --source-name "stripe-prod" \
  --source-type STRIPE \
  --source-description "Production Stripe webhooks" \
  --source-webhook-secret "whsec_abc123" \
  --destination-name "my-api" \
  --destination-type HTTP \
  --destination-url "https://api.example.com/webhooks" \
  --destination-auth-type BEARER_TOKEN \
  --destination-auth-token "your_token"

# Create connection with existing source, new destination
hookdeck connection create \
  --name "stripe-to-new-api" \
  --source-id src_123 \
  --destination-name "new-endpoint" \
  --destination-type HTTP \
  --destination-url "https://new-api.example.com/hooks"

# Create connection with new source, existing destination  
hookdeck connection create \
  --name "github-to-existing" \
  --source-name "github-repo" \
  --source-type GITHUB \
  --source-webhook-secret "github_secret_123" \
  --destination-id dest_456

# Create connection with retry rules
hookdeck connection create \
  --name "reliable-connection" \
  --source-id src_123 \
  --destination-id dest_456 \
  --retry-strategy exponential \
  --retry-count 5 \
  --retry-interval 1000

# Create connection with filter rules
hookdeck connection create \
  --name "filtered-webhooks" \
  --source-id src_123 \
  --destination-id dest_456 \
  --filter-headers '{"X-Event-Type": "payment.*"}' \
  --filter-body '{"type": ["invoice.payment_succeeded", "invoice.payment_failed"]}'

# Create connection with transformation
hookdeck connection create \
  --name "transformed-connection" \
  --source-id src_123 \
  --destination-id dest_456 \
  --transformation "stripe-formatter" \
  --transformation-env "API_URL=https://api.example.com"

# Create connection with delay rule
hookdeck connection create \
  --name "delayed-processing" \
  --source-id src_123 \
  --destination-id dest_456 \
  --delay 30000

# Create connection with deduplication
hookdeck connection create \
  --name "deduplicated-events" \
  --source-id src_123 \
  --destination-id dest_456 \
  --deduplicate-window 300000 \
  --deduplicate-fields "id,type,created"

# Create complex connection with inline resources and multiple rules
hookdeck connection create \
  --name "complex-connection" \
  --source-name "shopify-store" \
  --source-type SHOPIFY \
  --source-webhook-secret "shopify_secret" \
  --destination-name "webhook-processor" \
  --destination-type HTTP \
  --destination-url "https://processor.example.com/webhooks" \
  --destination-auth-type API_KEY \
  --destination-auth-key "api_key_123" \
  --destination-auth-header "X-API-Key" \
  --filter-body '{"type": "order.*"}' \
  --transformation "order-formatter" \
  --retry-strategy exponential \
  --retry-count 3 \
  --delay 5000
```

### Connection lifecycle management
```bash
# Disable connection
hookdeck connection disable conn_123

# Enable connection
hookdeck connection enable conn_123

# Pause connection (temporary)
hookdeck connection pause conn_123

# Unpause connection
hookdeck connection unpause conn_123

# Check connection status
hookdeck connection get conn_123 --format json | jq '{disabled_at, paused_at}'
```

## Transformations

🚧 **PLANNED FUNCTIONALITY** - Not yet implemented

Transformations allow you to modify webhook payloads using JavaScript.

### List transformations
```bash
# List all transformations
hookdeck transformation list

# Filter by name
hookdeck transformation list --name "*stripe*"
```

### Create a transformation
```bash
# Create with interactive prompts
hookdeck transformation create

# Create from file
hookdeck transformation create \
  --name "stripe-formatter" \
  --code-file "./transformations/stripe.js"

# Create with environment variables
hookdeck transformation create \
  --name "api-enricher" \
  --code-file "./transformations/enrich.js" \
  --env API_KEY=your_key

# Create inline transformation with JavaScript code
hookdeck transformation create \
  --name "payment-formatter" \
  --code 'export default function(request) {
    // Add timestamp
    request.body.processed_at = new Date().toISOString();
    
    // Normalize Stripe event data
    if (request.body.type) {
      request.body.event_type = request.body.type;
      request.body.webhook_source = "stripe";
    }
    
    // Add custom headers
    request.headers["X-Processed"] = "true";
    
    return request;
  }'

# Create transformation with environment variables
hookdeck transformation create \
  --name "slack-notifier" \
  --env SLACK_WEBHOOK_URL=https://hooks.slack.com/... \
  --code 'export default function(request) {
    const { SLACK_WEBHOOK_URL } = process.env;
    
    // Format webhook for Slack
    const slackMessage = {
      text: `Webhook received: ${request.body.type}`,
      blocks: [{
        type: "section",
        text: {
          type: "mrkdwn",
          text: `*Event:* ${request.body.type}\n*ID:* ${request.body.id}`
        }
      }]
    };
    
    // Send to Slack (example - actual HTTP call would use fetch)
    console.log("Sending to Slack:", slackMessage);
    
    return request;
  }'

# Create transformation for data validation
hookdeck transformation create \
  --name "data-validator" \
  --code 'export default function(request) {
    // Validate required fields
    const required = ["id", "type", "created"];
    const missing = required.filter(field => !request.body[field]);
    
    if (missing.length > 0) {
      throw new Error(`Missing required fields: ${missing.join(", ")}`);
    }
    
    // Add validation metadata
    request.body.validated_at = new Date().toISOString();
    request.body.validation_status = "passed";
    
    return request;
  }'
```

### Update a transformation
```bash
# Update transformation code
hookdeck transformation update trans_123 \
  --code-file "./updated-transformation.js"

# Update environment variables
hookdeck transformation update trans_123 \
  --env API_KEY=new_key,DEBUG=true
```

### Test a transformation
```bash
# Test with sample data
hookdeck transformation test trans_123 \
  --input-file "./sample-webhook.json"

# Test with inline JSON
hookdeck transformation test trans_123 \
  --input '{"event": "test", "data": {"user_id": 123}}'

# Test with connection context
hookdeck transformation test trans_123 \
  --connection conn_123 \
  --input-file "./webhook-payload.json"

# Test and save output
hookdeck transformation test trans_123 \
  --input-file "./test-payload.json" \
  --output-file "./test-result.json"
```

## Events & Monitoring

🚧 **PLANNED FUNCTIONALITY** - Not yet implemented

### List events
```bash
# List recent events
hookdeck event list

# Filter by connection
hookdeck event list --connection stripe-to-api

# Filter by status
hookdeck event list --status FAILED

# Filter by date range
hookdeck event list --created-after "2023-01-01" --created-before "2023-12-31"

# Filter by source and destination
hookdeck event list --source stripe-prod --destination my-api

# Filter by response status
hookdeck event list --response-status 500

# Filter by error code
hookdeck event list --error-code TIMEOUT

# Complex filtering
hookdeck event list \
  --status FAILED \
  --connection stripe-to-api \
  --response-status 500 \
  --created-after "2023-12-01"

# Output as JSON with full event data
hookdeck event list --format json --include-data

# Search by event content
hookdeck event list --search "payment_intent"

# Filter by number of attempts
hookdeck event list --attempts ">3"
```

### Get event details
```bash
# Get event by ID
hookdeck event get evt_123

# Get with full payload data
hookdeck event get evt_123 --include-data

# Get raw event body
hookdeck event get evt_123 --raw-body

# Export event as curl command
hookdeck event get evt_123 --export-curl
```

### Retry events
```bash
# Retry single event
hookdeck event retry evt_123

# Retry with different destination
hookdeck event retry evt_123 --destination new-endpoint

# Bulk retry failed events
hookdeck event retry --status FAILED --connection stripe-to-api

# Bulk retry with date filter
hookdeck event retry \
  --status FAILED \
  --created-after "2023-12-01" \
  --dry-run

# Bulk retry with progress tracking
hookdeck event retry \
  --status FAILED \
  --connection stripe-to-api \
  --progress \
  --batch-size 100
```

### Event monitoring
```bash
# Watch events in real-time
hookdeck event watch

# Watch specific connection
hookdeck event watch --connection stripe-to-api

# Watch with filtering
hookdeck event watch --status FAILED --response-status 5xx

# Get event statistics
hookdeck event stats --connection stripe-to-api

# Get delivery rate
hookdeck event stats --connection stripe-to-api --metric delivery-rate

# Alert on failure rate
hookdeck event monitor \
  --connection stripe-to-api \
  --failure-threshold 10 \
  --window 5m \
  --alert-webhook https://alerts.example.com
```

## Issue Triggers

🚧 **PLANNED FUNCTIONALITY** - Not yet implemented

Issue triggers automatically detect and create issues when specific conditions are met on webhook events.

### List issue triggers
```bash
# List all issue triggers
hookdeck issue-trigger list

# Filter by type
hookdeck issue-trigger list --type delivery_failure

# Filter by connection
hookdeck issue-trigger list --connection stripe-to-api

# Include disabled triggers
hookdeck issue-trigger list --include-disabled

# Output as JSON
hookdeck issue-trigger list --format json
```

### Get issue trigger details
```bash
# Get trigger by ID
hookdeck issue-trigger get isst_123

# Get with configuration details
hookdeck issue-trigger get isst_123 --include-config
```

### Create an issue trigger
```bash
# Create with interactive prompts
hookdeck issue-trigger create

# Create delivery trigger with Slack notification
hookdeck issue-trigger create \
  --name "payment-delivery-failures" \
  --type delivery \
  --strategy final_attempt \
  --connections "conn_stripe_to_api,conn_payment_processor" \
  --slack-channel "#alerts"

# Create transformation trigger with email notification
hookdeck issue-trigger create \
  --name "transform-errors" \
  --type transformation \
  --log-level error \
  --transformations "*payment*" \
  --email

# Create backpressure trigger for high delays
hookdeck issue-trigger create \
  --name "high-latency-alerts" \
  --type backpressure \
  --delay 300000 \
  --destinations "dest_api_endpoint,dest_webhook_handler" \
  --slack-channel "#ops" \
  --pagerduty

# Create trigger for all connections (wildcard pattern)
hookdeck issue-trigger create \
  --name "all-delivery-failures" \
  --type delivery \
  --strategy first_attempt \
  --connections "*" \
  --email \
  --slack-channel "#critical"

# Create trigger with multiple notification channels
hookdeck issue-trigger create \
  --name "critical-failures" \
  --type delivery \
  --strategy final_attempt \
  --connections "payment-*" \
  --slack-channel "#alerts" \
  --email \
  --pagerduty \
  --opsgenie
```

### Update issue trigger
```bash
# Update interactively
hookdeck issue-trigger update isst_123

# Update threshold
hookdeck issue-trigger update isst_123 --threshold 10

# Update time window
hookdeck issue-trigger update isst_123 --window-minutes 120

# Update filters
hookdeck issue-trigger update isst_123 --filter "severity=high"
```

### Delete issue trigger
```bash
# Delete with confirmation
hookdeck issue-trigger delete isst_123

# Force delete without confirmation
hookdeck issue-trigger delete isst_123 --force
```

### Enable/Disable issue triggers
```bash
# Disable trigger
hookdeck issue-trigger disable isst_123

# Enable trigger
hookdeck issue-trigger enable isst_123

# Check trigger status
hookdeck issue-trigger get isst_123 --format json | jq '{disabled_at, enabled}'
```

### Issue trigger output examples
```bash
$ hookdeck issue-trigger list
ID             NAME                    TYPE               CONNECTION       THRESHOLD  WINDOW   STATUS
isst_ABC123    payment-failures        delivery_failure   stripe-to-api    5          60min    enabled
isst_DEF456    transform-errors        transformation_error data-processor 3          30min    enabled
isst_GHI789    volume-spike           high_volume        api-events       1000       5min     disabled

$ hookdeck issue-trigger get isst_ABC123 --format json
{
  "id": "isst_ABC123",
  "name": "payment-failures", 
  "type": "delivery_failure",
  "connection_id": "conn_stripe123",
  "threshold": 5,
  "window_minutes": 60,
  "filters": ["event_type=payment.failed"],
  "enabled": true,
  "created_at": "2023-12-01T10:00:00Z",
  "updated_at": "2023-12-01T10:00:00Z"
}
```

## Attempts

🚧 **PLANNED FUNCTIONALITY** - Not yet implemented

Attempts represent individual delivery attempts for webhook events, including retry attempts.

### List attempts
```bash
# List all attempts
hookdeck attempt list

# Filter by event
hookdeck attempt list --event evt_123

# Filter by connection
hookdeck attempt list --connection stripe-to-api

# Filter by status
hookdeck attempt list --status FAILED

# Filter by date range
hookdeck attempt list --from "2023-12-01" --to "2023-12-31"

# Limit results
hookdeck attempt list --limit 50

# Output as JSON
hookdeck attempt list --format json
```

### Get attempt details
```bash
# Get attempt by ID
hookdeck attempt get atmpt_123

# Get with request/response details
hookdeck attempt get atmpt_123 --include-details

# Get with headers
hookdeck attempt get atmpt_123 --include-headers
```

### Retry attempts
```bash
# Retry specific attempt
hookdeck attempt retry atmpt_123

# Retry all failed attempts for an event
hookdeck attempt retry --event evt_123 --status FAILED

# Retry with custom headers
hookdeck attempt retry atmpt_123 --header "X-Retry-Reason=manual"
```

### Attempt filtering options
```bash
# Filter by HTTP status code
hookdeck attempt list --status-code 500

# Filter by response time
hookdeck attempt list --response-time-min 5000

# Filter by attempt number
hookdeck attempt list --attempt-number 3

# Complex filtering
hookdeck attempt list \
  --connection payment-api \
  --status FAILED \
  --status-code 500 \
  --from "2023-12-01" \
  --limit 100
```

### Attempt output examples
```bash
$ hookdeck attempt list --event evt_123
ID              EVENT_ID    ATTEMPT#  STATUS     STATUS_CODE  RESPONSE_TIME  CREATED_AT
atmpt_ABC123    evt_123     1         SUCCEEDED  200          245ms          2023-12-01T10:00:00Z
atmpt_DEF456    evt_123     2         FAILED     500          5.2s           2023-12-01T10:01:00Z
atmpt_GHI789    evt_123     3         SUCCEEDED  200          180ms          2023-12-01T10:02:00Z

$ hookdeck attempt get atmpt_DEF456 --format json
{
  "id": "atmpt_DEF456",
  "event_id": "evt_123",
  "attempt_number": 2,
  "status": "FAILED",
  "status_code": 500,
  "response_time_ms": 5200,
  "error_code": "DESTINATION_TIMEOUT",
  "created_at": "2023-12-01T10:01:00Z",
  "request": {
    "method": "POST",
    "url": "https://api.example.com/webhooks",
    "headers": {...},
    "body": "..."
  },
  "response": {
    "status_code": 500,
    "headers": {...},
    "body": "Internal Server Error"
  }
}
```

## Bookmarks

🚧 **PLANNED FUNCTIONALITY** - Not yet implemented

Bookmarks allow you to save and quickly replay webhook events for testing and debugging.

### List bookmarks
```bash
# List all bookmarks
hookdeck bookmark list

# Filter by name pattern
hookdeck bookmark list --name "*test*"

# Filter by event type
hookdeck bookmark list --event-type "payment.succeeded"

# Filter by source
hookdeck bookmark list --source stripe-prod

# Output as JSON
hookdeck bookmark list --format json
```

### Get bookmark details
```bash
# Get bookmark by ID
hookdeck bookmark get bmk_123

# Get bookmark by name
hookdeck bookmark get test-payment

# Get with full event data
hookdeck bookmark get bmk_123 --include-data
```

### Create a bookmark
```bash
# Create from existing event
hookdeck bookmark create --event evt_123 --name "failed-payment-test"

# Create with description
hookdeck bookmark create \
  --event evt_123 \
  --name "stripe-subscription-cancel" \
  --description "Test case for subscription cancellation flow"

# Create with tags
hookdeck bookmark create \
  --event evt_123 \
  --name "high-value-transaction" \
  --tags "payment,testing,edge-case"

# Create from JSON payload
hookdeck bookmark create \
  --name "custom-test-event" \
  --payload-file "./test-payloads/payment.json" \
  --headers "Content-Type=application/json" \
  --source stripe-prod
```

### Update bookmark
```bash
# Update interactively
hookdeck bookmark update bmk_123

# Update name
hookdeck bookmark update bmk_123 --name "new-bookmark-name"

# Update description
hookdeck bookmark update bmk_123 --description "Updated test description"

# Update tags
hookdeck bookmark update bmk_123 --tags "payment,updated,regression"

# Update payload
hookdeck bookmark update bmk_123 --payload-file "./updated-payload.json"
```

### Delete bookmark
```bash
# Delete with confirmation
hookdeck bookmark delete bmk_123

# Force delete without confirmation
hookdeck bookmark delete bmk_123 --force

# Delete by name
hookdeck bookmark delete test-payment
```

### Trigger bookmark
```bash
# Trigger bookmark to connection
hookdeck bookmark trigger bmk_123 --connection stripe-to-api

# Trigger to specific destination
hookdeck bookmark trigger bmk_123 --destination local-dev

# Trigger with modified headers
hookdeck bookmark trigger bmk_123 \
  --connection stripe-to-api \
  --header "X-Test-Mode=true"

# Trigger multiple times
hookdeck bookmark trigger bmk_123 \
  --connection stripe-to-api \
  --count 5 \
  --interval 1000ms
```

### Bookmark output examples
```bash
$ hookdeck bookmark list
ID           NAME                      EVENT_TYPE           SOURCE      TAGS             CREATED_AT
bmk_ABC123   failed-payment-test      payment.failed       stripe-prod payment,test     2023-12-01T10:00:00Z
bmk_DEF456   subscription-cancel      customer.subscription stripe-prod subscription     2023-12-01T11:00:00Z
bmk_GHI789   high-value-transaction   payment.succeeded     stripe-prod payment,edge     2023-12-01T12:00:00Z

$ hookdeck bookmark get bmk_ABC123 --format json
{
  "id": "bmk_ABC123",
  "name": "failed-payment-test",
  "description": "Test case for failed payment handling",
  "event_type": "payment.failed",
  "source_id": "src_stripe123",
  "tags": ["payment", "test"],
  "headers": {...},
  "payload": {...},
  "created_at": "2023-12-01T10:00:00Z",
  "updated_at": "2023-12-01T10:00:00Z"
}
```

## Integrations

🚧 **PLANNED FUNCTIONALITY** - Not yet implemented

Integrations connect platform-specific services and can be attached to sources for enhanced functionality.

### List integrations
```bash
# List all integrations
hookdeck integration list

# Filter by provider
hookdeck integration list --provider SLACK

# Filter by type
hookdeck integration list --type notification

# Include disabled integrations
hookdeck integration list --include-disabled

# Output as JSON
hookdeck integration list --format json
```

### Get integration details
```bash
# Get integration by ID
hookdeck integration get int_123

# Get with configuration details
hookdeck integration get int_123 --include-config
```

### Create integrations

#### Slack notification integration
```bash
# Create Slack integration
hookdeck integration create \
  --name "team-notifications" \
  --provider SLACK \
  --type notification \
  --webhook-url "https://hooks.slack.com/services/..." \
  --channel "#webhooks" \
  --username "Hookdeck Bot"

# Create with custom message template
hookdeck integration create \
  --name "alert-notifications" \
  --provider SLACK \
  --type notification \
  --webhook-url "https://hooks.slack.com/services/..." \
  --channel "#alerts" \
  --template "🚨 Webhook failed: {{event.type}} from {{source.name}}"
```

#### Email notification integration
```bash
# Create email integration
hookdeck integration create \
  --name "ops-email-alerts" \
  --provider EMAIL \
  --type notification \
  --recipients "ops@company.com,alerts@company.com" \
  --subject-template "Webhook Alert: {{event.type}}"
```

#### PagerDuty integration
```bash
# Create PagerDuty integration
hookdeck integration create \
  --name "critical-alerts" \
  --provider PAGERDUTY \
  --type alert \
  --routing-key "your_pagerduty_routing_key" \
  --severity "critical"
```

#### Datadog monitoring integration
```bash
# Create Datadog integration
hookdeck integration create \
  --name "webhook-metrics" \
  --provider DATADOG \
  --type monitoring \
  --api-key "your_datadog_api_key" \
  --site "datadoghq.com" \
  --tags "service:webhooks,env:production"
```

### Update integration
```bash
# Update interactively
hookdeck integration update int_123

# Update configuration
hookdeck integration update int_123 --webhook-url "https://new-webhook-url.com"

# Update notification settings
hookdeck integration update int_123 --channel "#new-channel"
```

### Delete integration
```bash
# Delete with confirmation
hookdeck integration delete int_123

# Force delete without confirmation
hookdeck integration delete int_123 --force
```

### Enable/Disable integrations
```bash
# Disable integration
hookdeck integration disable int_123

# Enable integration
hookdeck integration enable int_123
```

### Attach integration to source
```bash
# Attach integration to source
hookdeck source attach-integration src_123 int_456

# Attach with specific trigger conditions
hookdeck source attach-integration src_123 int_456 \
  --trigger-on "delivery_failure,transformation_error" \
  --threshold 3

# Detach integration from source
hookdeck source detach-integration src_123 int_456
```

### Integration output examples
```bash
$ hookdeck integration list
ID           NAME                 PROVIDER    TYPE          STATUS    CREATED_AT
int_ABC123   team-notifications   SLACK       notification  enabled   2023-12-01T10:00:00Z
int_DEF456   critical-alerts      PAGERDUTY   alert         enabled   2023-12-01T11:00:00Z
int_GHI789   webhook-metrics      DATADOG     monitoring    disabled  2023-12-01T12:00:00Z

$ hookdeck integration get int_ABC123 --format json
{
  "id": "int_ABC123",
  "name": "team-notifications",
  "provider": "SLACK",
  "type": "notification",
  "enabled": true,
  "config": {
    "webhook_url": "https://hooks.slack.com/services/...",
    "channel": "#webhooks",
    "username": "Hookdeck Bot"
  },
  "created_at": "2023-12-01T10:00:00Z",
  "updated_at": "2023-12-01T10:00:00Z"
}
```

## Issues

🚧 **PLANNED FUNCTIONALITY** - Not yet implemented

Issues represent problems detected in your webhook infrastructure, automatically created by issue triggers.

### List issues
```bash
# List all issues
hookdeck issue list

# Filter by status
hookdeck issue list --status OPEN

# Filter by type
hookdeck issue list --type delivery_failure

# Filter by connection
hookdeck issue list --connection stripe-to-api

# Filter by date range
hookdeck issue list --from "2023-12-01" --to "2023-12-31"

# Sort by severity
hookdeck issue list --sort severity --order desc

# Output as JSON
hookdeck issue list --format json
```

### Get issue details
```bash
# Get issue by ID
hookdeck issue get iss_123

# Get with related events
hookdeck issue get iss_123 --include-events

# Get with resolution history
hookdeck issue get iss_123 --include-history
```

### Update issue
```bash
# Update issue status
hookdeck issue update iss_123 --status RESOLVED

# Add note to issue
hookdeck issue update iss_123 --note "Fixed by deploying v2.1.3"

# Update severity
hookdeck issue update iss_123 --severity LOW

# Assign issue
hookdeck issue update iss_123 --assignee "ops@company.com"
```

### Dismiss issues
```bash
# Dismiss single issue
hookdeck issue dismiss iss_123 --reason "False positive"

# Dismiss multiple issues
hookdeck issue dismiss iss_123 iss_456 iss_789 --reason "Resolved in bulk"

# Dismiss all issues of a type
hookdeck issue dismiss --type delivery_failure --connection old-api --reason "Service deprecated"

# Auto-dismiss after resolution
hookdeck issue update iss_123 --status RESOLVED --auto-dismiss
```

### Issue filtering and sorting
```bash
# Filter by severity
hookdeck issue list --severity CRITICAL,HIGH

# Filter by trigger
hookdeck issue list --trigger isst_123

# Complex filtering
hookdeck issue list \
  --status OPEN \
  --severity HIGH,CRITICAL \
  --from "2023-12-01" \
  --connection payment-api \
  --sort created_at \
  --order desc \
  --limit 50
```

### Issue output examples
```bash
$ hookdeck issue list
ID           TYPE                CONNECTION      SEVERITY  STATUS  EVENTS  CREATED_AT
iss_ABC123   delivery_failure    stripe-to-api  HIGH      OPEN    15      2023-12-01T10:00:00Z  
iss_DEF456   transformation_error data-processor MEDIUM    OPEN    8       2023-12-01T11:00:00Z
iss_GHI789   high_volume         api-events      LOW       RESOLVED 1       2023-12-01T12:00:00Z

$ hookdeck issue get iss_ABC123 --format json
{
  "id": "iss_ABC123",
  "type": "delivery_failure",
  "connection_id": "conn_stripe123",
  "trigger_id": "isst_payment_failures",
  "severity": "HIGH",
  "status": "OPEN",
  "title": "High delivery failure rate detected",
  "description": "15 events failed delivery in the last 60 minutes",
  "event_count": 15,
  "first_event_at": "2023-12-01T09:00:00Z",
  "last_event_at": "2023-12-01T10:00:00Z",
  "created_at": "2023-12-01T10:00:00Z",
  "updated_at": "2023-12-01T10:00:00Z",
  "assignee": null,
  "notes": []
}
```

## Requests

🚧 **PLANNED FUNCTIONALITY** - Not yet implemented

Requests represent the raw HTTP requests received by Hookdeck sources before they become events.

### List requests
```bash
# List all requests
hookdeck request list

# Filter by source
hookdeck request list --source stripe-prod

# Filter by status
hookdeck request list --status PROCESSED

# Filter by date range
hookdeck request list --from "2023-12-01T10:00:00Z" --to "2023-12-01T11:00:00Z"

# Filter by HTTP method
hookdeck request list --method POST

# Filter by status code
hookdeck request list --status-code 200

# Output as JSON
hookdeck request list --format json
```

### Get request details
```bash
# Get request by ID
hookdeck request get req_123

# Get with full headers
hookdeck request get req_123 --include-headers

# Get with raw body
hookdeck request get req_123 --include-body

# Get associated events
hookdeck request get req_123 --include-events
```

### Get raw request data
```bash
# Get raw request headers
hookdeck request raw req_123 --headers-only

# Get raw request body
hookdeck request raw req_123 --body-only

# Get complete raw request
hookdeck request raw req_123 --complete

# Save raw request to file
hookdeck request raw req_123 --output request-123.json

# Get curl equivalent
hookdeck request raw req_123 --as-curl
```

### Retry requests
```bash
# Retry request (creates new event)
hookdeck request retry req_123

# Retry with different destination
hookdeck request retry req_123 --destination local-dev

# Retry with modified headers
hookdeck request retry req_123 --header "X-Retry-Source=cli"

# Retry multiple requests
hookdeck request retry req_123 req_456 req_789

# Bulk retry filtered requests
hookdeck request retry \
  --source stripe-prod \
  --status FAILED \
  --from "2023-12-01T10:00:00Z" \
  --limit 100
```

### Request filtering options
```bash
# Filter by content type
hookdeck request list --content-type "application/json"

# Filter by user agent
hookdeck request list --user-agent "*Stripe*"

# Filter by IP address
hookdeck request list --ip "192.168.1.100"

# Filter by payload size
hookdeck request list --size-min 1024 --size-max 10240

# Complex filtering
hookdeck request list \
  --source payment-api \
  --method POST \
  --status PROCESSED \
  --from "2023-12-01" \
  --content-type "application/json" \
  --limit 50
```

### Request output examples
```bash
$ hookdeck request list --source stripe-prod --limit 5
ID           SOURCE        METHOD  STATUS      STATUS_CODE  SIZE    CREATED_AT
req_ABC123   stripe-prod   POST    PROCESSED   200          2.1KB   2023-12-01T10:00:00Z
req_DEF456   stripe-prod   POST    PROCESSED   200          1.8KB   2023-12-01T10:01:00Z
req_GHI789   stripe-prod   POST    FAILED      400          0.5KB   2023-12-01T10:02:00Z
req_JKL012   stripe-prod   POST    PROCESSED   200          3.2KB   2023-12-01T10:03:00Z
req_MNO345   stripe-prod   POST    PROCESSED   200          1.9KB   2023-12-01T10:04:00Z

$ hookdeck request get req_ABC123 --format json
{
  "id": "req_ABC123",
  "source_id": "src_stripe123",
  "method": "POST",
  "path": "/webhook",
  "query_string": "",
  "status": "PROCESSED",
  "status_code": 200,
  "content_type": "application/json",
  "user_agent": "Stripe/1.0",
  "ip_address": "3.18.12.63",
  "size_bytes": 2150,
  "created_at": "2023-12-01T10:00:00Z",
  "processed_at": "2023-12-01T10:00:01Z",
  "event_count": 1,
  "headers": {...},
  "body_preview": "{"id": "evt_...", "object": "event"...}"
}

$ hookdeck request raw req_ABC123 --as-curl
curl -X POST \
  -H "Content-Type: application/json" \
  -H "User-Agent: Stripe/1.0" \
  -H "Stripe-Signature: t=1234567890,v1=abc123..." \
  -d '{"id":"evt_123","object":"event","type":"payment_intent.succeeded",...}' \
  https://events.hookdeck.com/e/src_stripe123/webhook
```

## Bulk Operations

🚧 **PLANNED FUNCTIONALITY** - Not yet implemented

Bulk operations allow you to perform actions on multiple resources at once, with filtering and confirmation options.

### Bulk retry for events
```bash
# Retry all failed events
hookdeck event bulk-retry --status FAILED --confirm

# Retry failed events for specific connection
hookdeck event bulk-retry \
  --status FAILED \
  --connection stripe-to-api \
  --from "2023-12-01" \
  --confirm

# Retry events with specific error codes
hookdeck event bulk-retry \
  --status FAILED \
  --error-code "DESTINATION_TIMEOUT,CONNECTION_REFUSED" \
  --limit 100 \
  --confirm

# Dry run to see what would be retried
hookdeck event bulk-retry --status FAILED --dry-run

# Retry with custom delay between attempts
hookdeck event bulk-retry \
  --status FAILED \
  --connection payment-api \
  --delay 5s \
  --confirm
```

### Bulk retry for ignored events
```bash
# Retry all ignored events
hookdeck ignored-event bulk-retry --confirm

# Retry ignored events for specific connection
hookdeck ignored-event bulk-retry \
  --connection stripe-to-api \
  --from "2023-12-01" \
  --confirm

# Retry ignored events by filter pattern
hookdeck ignored-event bulk-retry \
  --filter "event_type=payment.failed" \
  --limit 50 \
  --confirm

# Dry run to preview ignored events
hookdeck ignored-event bulk-retry --dry-run
```

### Bulk retry for requests
```bash
# Retry all failed requests
hookdeck request bulk-retry --status FAILED --confirm

# Retry requests by source
hookdeck request bulk-retry \
  --source stripe-prod \
  --status FAILED \
  --from "2023-12-01T10:00:00Z" \
  --to "2023-12-01T11:00:00Z" \
  --confirm

# Retry requests with specific HTTP status codes
hookdeck request bulk-retry \
  --status-code 500,502,503,504 \
  --limit 200 \
  --confirm

# Retry requests to different destination
hookdeck request bulk-retry \
  --source api-events \
  --status FAILED \
  --destination backup-processor \
  --confirm
```

### Bulk enable/disable resources
```bash
# Disable all sources of a type
hookdeck source bulk-disable --type WEBHOOK --confirm

# Enable all destinations matching pattern
hookdeck destination bulk-enable --name "*staging*" --confirm

# Disable connections by source
hookdeck connection bulk-disable --source deprecated-api --confirm

# Enable all disabled issue triggers
hookdeck issue-trigger bulk-enable --disabled --confirm

# Disable integrations by provider
hookdeck integration bulk-disable --provider SLACK --confirm
```

### Bulk delete operations
```bash
# Delete all disabled sources
hookdeck source bulk-delete --disabled --confirm

# Delete all test connections
hookdeck connection bulk-delete --name "*test*" --confirm

# Delete old events (with retention policy)
hookdeck event bulk-delete \
  --status SUCCEEDED \
  --before "2023-11-01" \
  --confirm

# Delete bookmarks by tag
hookdeck bookmark bulk-delete --tag "deprecated" --confirm

# Delete resolved issues older than 30 days
hookdeck issue bulk-delete \
  --status RESOLVED \
  --before "30 days ago" \
  --confirm
```

### Bulk update operations
```bash
# Update connection retry policies
hookdeck connection bulk-update \
  --source stripe-prod \
  --retry-count 5 \
  --retry-strategy exponential \
  --confirm

# Update destination timeouts
hookdeck destination bulk-update \
  --type HTTP \
  --timeout 30000 \
  --confirm

# Update source authentication
hookdeck source bulk-update \
  --type WEBHOOK \
  --auth-type HMAC \
  --confirm

# Assign multiple issues to user
hookdeck issue bulk-update \
  --status OPEN \
  --connection payment-api \
  --assignee "ops@company.com" \
  --confirm
```

### Bulk filtering options
```bash
# Filter by date ranges
--from "2023-12-01"
--to "2023-12-31" 
--before "30 days ago"
--after "2023-11-01T10:00:00Z"

# Filter by status and type
--status FAILED,TIMEOUT
--type delivery_failure
--error-code CONNECTION_REFUSED

# Filter by naming patterns
--name "*test*"
--name-regex "^prod-.*"
--tag "deprecated,testing"

# Filter by relationships
--connection stripe-to-api
--source payment-webhooks
--destination local-dev
--trigger isst_123

# Limit and ordering
--limit 1000
--sort created_at
--order desc
```

### Bulk operation safety features
```bash
# Always confirm destructive operations
--confirm             # Required for delete/disable operations

# Preview operations without executing
--dry-run            # Show what would be affected

# Limit scope to prevent accidents
--limit 100          # Maximum number of resources to affect

# Progress tracking for large operations
--progress           # Show progress bar for bulk operations

# Force operations (bypass some confirmations)
--force              # Use with extreme caution
```

### Bulk operation output examples
```bash
$ hookdeck event bulk-retry --status FAILED --connection stripe-to-api --dry-run
Found 45 failed events matching criteria:
- Connection: stripe-to-api
- Status: FAILED  
- Date range: Last 24 hours

Events would be retried:
evt_ABC123  payment.failed      2023-12-01T10:00:00Z
evt_DEF456  invoice.updated     2023-12-01T10:05:00Z
evt_GHI789  customer.updated    2023-12-01T10:10:00Z
...

Run with --confirm to execute this bulk retry operation.

$ hookdeck source bulk-disable --type WEBHOOK --confirm
Disabling 12 webhook sources...
✓ src_ABC123 (webhook-1) disabled
✓ src_DEF456 (webhook-2) disabled  
✓ src_GHI789 (webhook-3) disabled
...
Successfully disabled 12 webhook sources.

$ hookdeck connection bulk-delete --name "*test*" --confirm
Deleting 8 test connections...
✓ conn_test_123 deleted
✓ conn_test_456 deleted
✗ conn_test_789 failed (connection has active events)
...
Successfully deleted 7 of 8 connections. 1 failed.
```

## Advanced Usage (Planned)

🚧 **PLANNED FUNCTIONALITY** - Not yet implemented

### Output formats
```bash
# JSON output
hookdeck source list --format json

# YAML output
hookdeck destination list --format yaml

# CSV output
hookdeck connection list --format csv
```

### Environment variables
```bash
# Set API key
export HOOKDECK_API_KEY="your_api_key"

# Set default project
export HOOKDECK_PROJECT_ID="proj_123"

# Set output format
export HOOKDECK_FORMAT="json"
```

---

## Getting Help

```bash
# General help
hookdeck --help

# Command-specific help
hookdeck project --help
hookdeck listen --help

# Version information
hookdeck version
```

## Migration from Current to Planned CLI

When the planned functionality is implemented, existing commands will continue to work, but will gain additional capabilities:

1. **Enhanced output** - Current text output will be supplemented with `--format` options
2. **Extended project management** - Current `list` and `use` will be joined by `create`, `update`, `delete`
3. **New resource types** - Sources, destinations, connections, and transformations will be added
4. **Backward compatibility** - All current commands and flags will continue to work

This reference serves both as documentation for current users and as a specification for future development.