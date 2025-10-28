# Connection Source Authentication Implementation Plan (Revised)

## Executive Summary
This plan outlines the implementation of comprehensive authentication support for inline source creation in the `hookdeck connection create` command. The current implementation has a critical architectural issue: it's creating sources separately instead of inline through the connection API. This plan addresses both the architectural fix and adds full authentication configuration for 96+ source types using a hybrid approach: universal flags for common patterns (80% coverage) and JSON config fallback for complex cases (20% edge cases).

## 1. Current State Analysis

### Critical Architecture Issue Found ❌
The current implementation is **incorrectly** creating sources and destinations as separate API calls, then creating connections with their IDs. This is wrong! The API supports inline creation through the `ConnectionCreateRequest`.

**Current (INCORRECT) flow:**
1. Create source separately → Get source ID
2. Create destination separately → Get destination ID  
3. Create connection with source_id and destination_id

**Correct flow (per API design):**
1. Create connection with inline `source` and `destination` objects

### Current Implementation Problems
From `pkg/cmd/connection_create.go`:
- Line 153-159: Creates source separately using `client.CreateSource()` ❌
- Line 203-206: Creates destination separately using `client.CreateDestination()` ❌
- Line 226-231: Creates connection with IDs only ❌
- Missing: Source type not passed to API
- Missing: Source config (authentication) not supported

### Correct API Structure
From `pkg/hookdeck/connections.go`:
```go
type ConnectionCreateRequest struct {
    Name          *string                 `json:"name,omitempty"`
    Description   *string                 `json:"description,omitempty"`
    SourceID      *string                 `json:"source_id,omitempty"`      // For existing source
    DestinationID *string                 `json:"destination_id,omitempty"`  // For existing destination
    Source        *SourceCreateInput      `json:"source,omitempty"`          // For inline creation
    Destination   *DestinationCreateInput `json:"destination,omitempty"`     // For inline creation
}
```

From `pkg/hookdeck/sources.go`:
```go
type SourceCreateInput struct {
    Name        string                 `json:"name"`
    Type        string                 `json:"type,omitempty"`        // Currently not used!
    Description *string                `json:"description,omitempty"`
    Config      map[string]interface{} `json:"config,omitempty"`      // Currently not used!
}
```

## 2. Required Architecture Fix

### Step 1: Refactor Connection Creation to Use Inline Resources
The entire connection creation flow needs to be rewritten to use inline creation:

```go
func (cc *connectionCreateCmd) runConnectionCreateCmd(cmd *cobra.Command, args []string) error {
    client := Config.GetAPIClient()
    
    // Build connection request
    connReq := &hookdeck.ConnectionCreateRequest{
        Name:        nilIfEmpty(cc.name),
        Description: nilIfEmpty(cc.description),
    }
    
    // Handle source - either reference existing or create inline
    if cc.sourceID != "" {
        connReq.SourceID = &cc.sourceID
    } else {
        // Build source configuration from flags
        sourceConfig, err := buildSourceConfig(cmd)
        if err != nil {
            return fmt.Errorf("failed to build source configuration: %w", err)
        }
        
        connReq.Source = &hookdeck.SourceCreateInput{
            Name:        cc.sourceName,
            Type:        cc.sourceType,  // NOW INCLUDED!
            Description: nilIfEmpty(cc.sourceDescription),
            Config:      sourceConfig,    // NOW INCLUDED!
        }
    }
    
    // Handle destination - either reference existing or create inline
    if cc.destinationID != "" {
        connReq.DestinationID = &cc.destinationID
    } else {
        // Build destination configuration
        destConfig, err := buildDestinationConfig(cmd)
        if err != nil {
            return fmt.Errorf("failed to build destination configuration: %w", err)
        }
        
        connReq.Destination = &hookdeck.DestinationCreateInput{
            Name:        cc.destinationName,
            Type:        cc.destinationType,
            Description: nilIfEmpty(cc.destinationDescription),
            Config:      destConfig,
        }
    }
    
    // Single API call to create everything
    fmt.Printf("Creating connection '%s'...\n", cc.name)
    connection, err := client.CreateConnection(context.Background(), connReq)
    if err != nil {
        return fmt.Errorf("failed to create connection: %w", err)
    }
    
    // Display results...
}
```

### Step 2: Remove Standalone Source/Destination Creation
- Remove `client.CreateSource()` calls
- Remove `client.CreateDestination()` calls  
- Remove intermediate ID tracking
- Use single `CreateConnection()` call with inline resources

## 3. Universal Flag Set Design (Option 2)

### Core Authentication Flags
Add these flags to `pkg/cmd/connection_create.go` after line 70:

```go
// Source authentication flags - covers 80% of source types
cmd.Flags().String("source-webhook-secret", "", "Webhook secret for source verification (STRIPE, GITHUB, SLACK)")
cmd.Flags().String("source-api-key", "", "API key for source authentication (SENDGRID, MAILGUN)")
cmd.Flags().String("source-basic-auth-username", "", "Basic auth username (TWILIO account SID)")
cmd.Flags().String("source-basic-auth-password", "", "Basic auth password (TWILIO auth token)")
cmd.Flags().String("source-bearer-token", "", "Bearer token for OAuth sources")
cmd.Flags().String("source-client-id", "", "OAuth client ID")
cmd.Flags().String("source-client-secret", "", "OAuth client secret")
cmd.Flags().String("source-hmac-key", "", "HMAC signing key (SHOPIFY, INTERCOM)")
cmd.Flags().String("source-signing-secret", "", "Generic signing secret")
cmd.Flags().StringSlice("source-allowed-ips", []string{}, "Comma-separated list of allowed IP addresses")
cmd.Flags().StringToString("source-custom-headers", map[string]string{}, "Custom headers in key=value format")

// JSON config fallback flags
cmd.Flags().String("source-config", "", "JSON object for complex source configuration")
cmd.Flags().String("source-config-file", "", "Path to JSON file containing source configuration")
```

### Config Building Function
Create new file: `pkg/cmd/source_config_builder.go`

```go
package cmd

import (
    "encoding/json"
    "fmt"
    "os"
    
    "github.com/spf13/cobra"
)

// buildSourceConfig builds the config map from CLI flags
func buildSourceConfig(cmd *cobra.Command) (map[string]interface{}, error) {
    // Priority: config-file > config string > universal flags
    
    // 1. Check for config file
    configFile, _ := cmd.Flags().GetString("source-config-file")
    if configFile != "" {
        data, err := os.ReadFile(configFile)
        if err != nil {
            return nil, fmt.Errorf("failed to read config file: %w", err)
        }
        
        var config map[string]interface{}
        if err := json.Unmarshal(data, &config); err != nil {
            return nil, fmt.Errorf("invalid JSON in config file: %w", err)
        }
        return config, nil
    }
    
    // 2. Check for inline JSON config
    configJSON, _ := cmd.Flags().GetString("source-config")
    if configJSON != "" {
        var config map[string]interface{}
        if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
            return nil, fmt.Errorf("invalid JSON in --source-config: %w", err)
        }
        return config, nil
    }
    
    // 3. Build from universal flags
    config := make(map[string]interface{})
    
    // Webhook secret
    if val, _ := cmd.Flags().GetString("source-webhook-secret"); val != "" {
        config["webhook_secret"] = val
    }
    
    // API key
    if val, _ := cmd.Flags().GetString("source-api-key"); val != "" {
        config["api_key"] = val
    }
    
    // Basic auth
    username, _ := cmd.Flags().GetString("source-basic-auth-username")
    password, _ := cmd.Flags().GetString("source-basic-auth-password")
    if username != "" || password != "" {
        config["basic_auth"] = map[string]string{
            "username": username,
            "password": password,
        }
    }
    
    // Bearer token
    if val, _ := cmd.Flags().GetString("source-bearer-token"); val != "" {
        config["bearer_token"] = val
    }
    
    // OAuth
    clientID, _ := cmd.Flags().GetString("source-client-id")
    clientSecret, _ := cmd.Flags().GetString("source-client-secret")
    if clientID != "" || clientSecret != "" {
        config["oauth"] = map[string]string{
            "client_id":     clientID,
            "client_secret": clientSecret,
        }
    }
    
    // HMAC key
    if val, _ := cmd.Flags().GetString("source-hmac-key"); val != "" {
        config["hmac"] = map[string]interface{}{
            "key": val,
        }
    }
    
    // Signing secret
    if val, _ := cmd.Flags().GetString("source-signing-secret"); val != "" {
        config["signing_secret"] = val
    }
    
    // IP allowlist
    if ips, _ := cmd.Flags().GetStringSlice("source-allowed-ips"); len(ips) > 0 {
        config["allowed_ips"] = ips
    }
    
    // Custom headers
    if headers, _ := cmd.Flags().GetStringToString("source-custom-headers"); len(headers) > 0 {
        config["custom_headers"] = headers
    }
    
    // Return nil if config is empty
    if len(config) == 0 {
        return nil, nil
    }
    
    return config, nil
}

// Helper function
func nilIfEmpty(s string) *string {
    if s == "" {
        return nil
    }
    return &s
}
```

## 4. Dynamic Type Registry from OpenAPI Spec

### New Approach: Runtime OpenAPI Parsing
Instead of a hardcoded map, we will parse the `hookdeck-openapi-2025-07-01.json` spec at runtime to build the source type configurations. This ensures the CLI is always in sync with the API.

### OpenAPI Parser Implementation
Create a new file: `pkg/cmd/source_types_parser.go`

```go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Simplified struct to hold parsed info
type SourceTypeInfo struct {
	Name           string
	RequiredFields []string
}

var sourceTypeRegistry map[string]SourceTypeInfo

// LoadSourceTypesFromSpec parses the OpenAPI spec and populates the registry.
func LoadSourceTypesFromSpec() error {
	// In a real implementation, embed the spec or fetch it. For now, read from file.
	specPath := filepath.Join(".plans", "connection-management", "schemas", "hookdeck-openapi-2025-07-01.json")
	data, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("could not load OpenAPI spec: %w", err)
	}

	var spec map[string]interface{}
	if err := json.Unmarshal(data, &spec); err != nil {
		return fmt.Errorf("could not parse OpenAPI spec: %w", err)
	}

	sourceTypeRegistry = make(map[string]SourceTypeInfo)
	schemas := spec["components"].(map[string]interface{})["schemas"].(map[string]interface{})

	// Find the Source schema to get the enum of types
	sourceSchema := schemas["Source"].(map[string]interface{})
	sourceProperties := sourceSchema["properties"].(map[string]interface{})
	sourceTypeEnum := sourceProperties["type"].(map[string]interface{})["enum"].([]interface{})

	for _, typeName := range sourceTypeEnum {
		typeNameStr := typeName.(string)
		configSchemaName := "SourceTypeConfig" + typeNameStr
		
		info := SourceTypeInfo{Name: typeNameStr}

		if configSchema, ok := schemas[configSchemaName]; ok {
			properties := configSchema.(map[string]interface{})["properties"].(map[string]interface{})
			if auth, ok := properties["auth"]; ok {
				authRef := auth.(map[string]interface{})["$ref"].(string)
				authSchemaName := strings.TrimPrefix(authRef, "#/components/schemas/")
				authSchema := schemas[authSchemaName].(map[string]interface{})
				if required, ok := authSchema["required"]; ok {
					for _, reqField := range required.([]interface{}) {
						info.RequiredFields = append(info.RequiredFields, reqField.(string))
					}
				}
			}
		}
		sourceTypeRegistry[typeNameStr] = info
	}

	return nil
}

func GetSourceTypeInfo(sourceType string) (SourceTypeInfo, bool) {
	if sourceTypeRegistry == nil {
		if err := LoadSourceTypesFromSpec(); err != nil {
			// Handle error appropriately, maybe panic or log fatal
			fmt.Fprintf(os.Stderr, "Error loading source types: %v\n", err)
			return SourceTypeInfo{}, false
		}
	}
	info, exists := sourceTypeRegistry[strings.ToUpper(sourceType)]
	return info, exists
}

// This function will need to be smarter to map API fields to flags
func fieldToFlag(field string) string {
    // Simple mapping for now
    switch field {
    case "webhook_secret_key":
        return "--source-webhook-secret"
    case "api_key":
        return "--source-api-key"
    // ... add other mappings
    default:
        return field
    }
}
```

## 5. Validation Implementation

### Validation Function
Create new file: `pkg/cmd/source_validation.go`

```go
package cmd

import (
    "fmt"
    "os"
    "strings"
    
    "github.com/spf13/cobra"
)

// ValidateSourceFlags validates source creation flags based on type
func ValidateSourceFlags(cmd *cobra.Command) error {
    sourceType, _ := cmd.Flags().GetString("source-type")
    if sourceType == "" {
        return nil // No type, no validation
    }
    
    // Get type configuration
    typeConfig, exists := GetSourceTypeInfo(sourceType)
    if !exists {
    	// Unknown type - warn but allow
    	fmt.Fprintf(os.Stderr, "Warning: Unknown source type '%s'. No validation applied.\n", sourceType)
    	return nil
    }
    
    // Build config from flags
    config, err := buildSourceConfig(cmd)
    if err != nil {
        return err
    }
    
    // Check required fields
    var missingFields []string
    for _, field := range typeConfig.RequiredFields {
        if !hasConfigField(config, field) {
            flag := fieldToFlag(field)
            missingFields = append(missingFields, flag)
        }
    }
    
    if len(missingFields) > 0 {
        return fmt.Errorf("source type %s requires: %s\nExample: %s", 
            sourceType, 
            strings.Join(missingFields, ", "))
    }
    
    return nil
}

func hasConfigField(config map[string]interface{}, field string) bool {
    if config == nil {
        return false
    }
    
    // Handle nested fields like "basic_auth.username"
    parts := strings.Split(field, ".")
    current := config
    
    for i, part := range parts {
        if i == len(parts)-1 {
            _, exists := current[part]
            return exists
        }
        
        // Navigate deeper
        if next, ok := current[part].(map[string]interface{}); ok {
            current = next
        } else if next, ok := current[part].(map[string]string); ok {
            if i == len(parts)-2 {
                _, exists := next[parts[i+1]]
                return exists
            }
            return false
        } else {
            return false
        }
    }
    return false
}
```

## 6. Complete Integration into connection_create.go

### Full Refactored Implementation
Update `pkg/cmd/connection_create.go`:

```go
package cmd

import (
    "context"
    "fmt"
    
    "github.com/spf13/cobra"
    
    "github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
    "github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type connectionCreateCmd struct {
    cmd *cobra.Command
    
    // Connection flags
    name        string
    description string
    
    // Source flags (inline creation)
    sourceName        string
    sourceType        string
    sourceDescription string
    
    // Destination flags (inline creation)
    destinationName        string
    destinationType        string
    destinationDescription string
    destinationURL         string  // For HTTP destinations
    destinationCliPath     string  // For CLI destinations
    
    // Reference existing resources
    sourceID      string
    destinationID string
}

func newConnectionCreateCmd() *connectionCreateCmd {
    cc := &connectionCreateCmd{}
    
    cc.cmd = &cobra.Command{
        Use:   "create",
        Args:  validators.NoArgs,
        Short: "Create a new connection",
        Long: `Create a connection between a source and destination.

You can either reference existing resources by ID or create them inline.

Source Authentication:
  When creating sources inline, use type-specific authentication flags:
  - STRIPE/GITHUB/SLACK: --source-webhook-secret
  - TWILIO: --source-basic-auth-username and --source-basic-auth-password
  - SENDGRID/MAILGUN: --source-api-key
  - SHOPIFY: --source-hmac-key
  - Complex configs: --source-config or --source-config-file

Examples:
  # Create with Stripe source authentication
  hookdeck connection create \
    --name "stripe-to-local" \
    --source-type STRIPE --source-name "stripe-webhooks" \
    --source-webhook-secret "whsec_..." \
    --destination-type CLI --destination-name "local-dev"

  # Create with existing resources
  hookdeck connection create \
    --name "github-to-api" \
    --source-id src_abc123 \
    --destination-id dst_def456`,
        PreRunE: cc.validateFlags,
        RunE:    cc.runConnectionCreateCmd,
    }
    
    // Connection flags
    cc.cmd.Flags().StringVar(&cc.name, "name", "", "Connection name (required)")
    cc.cmd.Flags().StringVar(&cc.description, "description", "", "Connection description")
    
    // Source inline creation flags
    cc.cmd.Flags().StringVar(&cc.sourceName, "source-name", "", "Source name for inline creation")
    cc.cmd.Flags().StringVar(&cc.sourceType, "source-type", "", "Source type (WEBHOOK, STRIPE, GITHUB, etc.)")
    cc.cmd.Flags().StringVar(&cc.sourceDescription, "source-description", "", "Source description")
    
    // Source authentication flags - Universal set
    cc.cmd.Flags().String("source-webhook-secret", "", "Webhook secret for source verification")
    cc.cmd.Flags().String("source-api-key", "", "API key for source authentication")
    cc.cmd.Flags().String("source-basic-auth-username", "", "Basic auth username")
    cc.cmd.Flags().String("source-basic-auth-password", "", "Basic auth password")
    cc.cmd.Flags().String("source-bearer-token", "", "Bearer token for OAuth sources")
    cc.cmd.Flags().String("source-client-id", "", "OAuth client ID")
    cc.cmd.Flags().String("source-client-secret", "", "OAuth client secret")
    cc.cmd.Flags().String("source-hmac-key", "", "HMAC signing key")
    cc.cmd.Flags().String("source-signing-secret", "", "Generic signing secret")
    cc.cmd.Flags().StringSlice("source-allowed-ips", []string{}, "Allowed IP addresses")
    cc.cmd.Flags().StringToString("source-custom-headers", map[string]string{}, "Custom headers (key=value)")
    
    // JSON config fallback
    cc.cmd.Flags().String("source-config", "", "JSON configuration for complex sources")
    cc.cmd.Flags().String("source-config-file", "", "Path to JSON configuration file")
    
    // Destination inline creation flags
    cc.cmd.Flags().StringVar(&cc.destinationName, "destination-name", "", "Destination name for inline creation")
    cc.cmd.Flags().StringVar(&cc.destinationType, "destination-type", "", "Destination type (CLI, HTTP, MOCK)")
    cc.cmd.Flags().StringVar(&cc.destinationDescription, "destination-description", "", "Destination description")
    cc.cmd.Flags().StringVar(&cc.destinationURL, "destination-url", "", "Destination URL (for HTTP type)")
    cc.cmd.Flags().StringVar(&cc.destinationCliPath, "destination-cli-path", "/", "CLI path (for CLI type)")
    
    // Reference existing resources
    cc.cmd.Flags().StringVar(&cc.sourceID, "source-id", "", "Use existing source by ID")
    cc.cmd.Flags().StringVar(&cc.destinationID, "destination-id", "", "Use existing destination by ID")
    
    cc.cmd.MarkFlagRequired("name")
    
    return cc
}

func (cc *connectionCreateCmd) validateFlags(cmd *cobra.Command, args []string) error {
    if err := Config.Profile.ValidateAPIKey(); err != nil {
        return err
    }
    
    // Check for inline vs reference mode for source
    hasInlineSource := cc.sourceName != "" || cc.sourceType != ""
    
    if hasInlineSource && cc.sourceID != "" {
        return fmt.Errorf("cannot specify both inline source creation and --source-id")
    }
    if !hasInlineSource && cc.sourceID == "" {
        return fmt.Errorf("must specify either source creation flags or --source-id")
    }
    
    // Validate inline source creation
    if hasInlineSource {
        if cc.sourceName == "" {
            return fmt.Errorf("--source-name is required when creating a source inline")
        }
        if cc.sourceType == "" {
            return fmt.Errorf("--source-type is required when creating a source inline")
        }
        
        // Validate source authentication flags
        if err := ValidateSourceFlags(cmd); err != nil {
            return err
        }
    }
    
    // Check for inline vs reference mode for destination
    hasInlineDestination := cc.destinationName != "" || cc.destinationType != ""
    
    if hasInlineDestination && cc.destinationID != "" {
        return fmt.Errorf("cannot specify both inline destination creation and --destination-id")
    }
    if !hasInlineDestination && cc.destinationID == "" {
        return fmt.Errorf("must specify either destination creation flags or --destination-id")
    }
    
    // Validate inline destination creation
    if hasInlineDestination {
        if cc.destinationName == "" {
            return fmt.Errorf("--destination-name is required when creating a destination inline")
        }
        if cc.destinationType == "" {
            return fmt.Errorf("--destination-type is required when creating a destination inline")
        }
        
        // Validate destination type-specific requirements
        if cc.destinationType == "HTTP" && cc.destinationURL == "" {
            return fmt.Errorf("--destination-url is required for HTTP destinations")
        }
    }
    
    return nil
}

func (cc *connectionCreateCmd) runConnectionCreateCmd(cmd *cobra.Command, args []string) error {
    client := Config.GetAPIClient()
    
    fmt.Printf("Creating connection '%s'...\n", cc.name)
    
    // Build connection request
    connReq := &hookdeck.ConnectionCreateRequest{
        Name:        nilIfEmpty(cc.name),
        Description: nilIfEmpty(cc.description),
    }
    
    // Handle source - either reference existing or create inline
    if cc.sourceID != "" {
        connReq.SourceID = &cc.sourceID
        fmt.Printf("Using existing source: %s\n", cc.sourceID)
    } else {
        fmt.Printf("Creating inline source '%s' (%s)...\n", cc.sourceName, cc.sourceType)
        
        // Build source configuration from flags
        sourceConfig, err := buildSourceConfig(cmd)
        if err != nil {
            return fmt.Errorf("failed to build source configuration: %w", err)
        }
        
        connReq.Source = &hookdeck.SourceCreateInput{
            Name:        cc.sourceName,
            Type:        cc.sourceType,
            Description: nilIfEmpty(cc.sourceDescription),
            Config:      sourceConfig,
        }
        
        if sourceConfig != nil && len(sourceConfig) > 0 {
            fmt.Printf("  With authentication configured\n")
        }
    }
    
    // Handle destination - either reference existing or create inline
    if cc.destinationID != "" {
        connReq.DestinationID = &cc.destinationID
        fmt.Printf("Using existing destination: %s\n", cc.destinationID)
    } else {
        fmt.Printf("Creating inline destination '%s' (%s)...\n", cc.destinationName, cc.destinationType)
        
        // Build destination configuration
        destConfig := make(map[string]interface{})
        
        switch cc.destinationType {
        case "HTTP":
            destConfig["url"] = cc.destinationURL
        case "CLI":
            if cc.destinationCliPath != "" && cc.destinationCliPath != "/" {
                destConfig["path"] = cc.destinationCliPath
            }
        case "MOCK":
            // No additional config needed
        default:
            // Allow unknown types for forward compatibility
        }
        
        connReq.Destination = &hookdeck.DestinationCreateInput{
            Name:        cc.destinationName,
            Type:        cc.destinationType,
            Description: nilIfEmpty(cc.destinationDescription),
            Config:      destConfig,
        }
    }
    
    // Single API call to create everything
    connection, err := client.CreateConnection(context.Background(), connReq)
    if err != nil {
        return fmt.Errorf("failed to create connection: %w", err)
    }
    
    // Display results
    fmt.Printf("\n✓ Connection created successfully\n\n")
    
    if connection.Name != nil {
        fmt.Printf("Connection:  %s (ID: %s)\n", *connection.Name, connection.ID)
    } else {
        fmt.Printf("Connection:  %s\n", connection.ID)
    }
    
    if connection.Source != nil {
        fmt.Printf("Source:      %s (%s)\n", connection.Source.Name, connection.Source.ID)
        fmt.Printf("  URL:       %s\n", connection.Source.URL)
        if connection.Source.Type != "" {
            fmt.Printf("  Type:      %s\n", connection.Source.Type)
        }
    }
    
    if connection.Destination != nil {
        fmt.Printf("Destination: %s (%s)\n", connection.Destination.Name, connection.Destination.ID)
        if connection.Destination.Type != "" {
            fmt.Printf("  Type:      %s\n", connection.Destination.Type)
        }
    }
    
    return nil
}
```

## 7. Implementation Phases

### Phase 1: Core Implementation (3-4 Days)
**Day 1: Architecture Refactor**
- [ ] Refactor connection_create.go to use inline creation
- [ ] Remove standalone source/destination creation calls
- [ ] Test basic inline creation works

**Day 2: Authentication Framework**
- [ ] Add universal authentication flags
- [ ] Create source_config_builder.go
- [ ] Test config building from flags

**Day 3: Type Registry & Validation**
- [ ] Create source_types.go with 8 initial types
- [ ] Create source_validation.go
- [ ] Integrate validation into PreRunE

**Day 4: Testing & Documentation**
- [ ] Write unit tests for all components
- [ ] Write integration test script
- [ ] Update REFERENCE.md

### Phase 2: Extended Coverage (Week 2)
Add 15 more provider types:
- MAILGUN, SQUARE, PAYPAL, HUBSPOT, SALESFORCE
- INTERCOM, ZENDESK, JIRA, GITLAB, BITBUCKET
- PAGERDUTY, DATADOG, NEWRELIC, SEGMENT, MIXPANEL

### Phase 3: Complete Coverage (Weeks 3-4)
- Add remaining 70+ source types
- Consider auto-generation from OpenAPI spec
- Add interactive configuration mode

## 8. Testing Strategy

### Unit Test Files
1. `pkg/cmd/source_config_builder_test.go` - Config building logic
2. `pkg/cmd/source_validation_test.go` - Validation logic
3. `pkg/cmd/source_types_test.go` - Type registry
4. `pkg/cmd/connection_create_test.go` - Integration

### Integration Test Script
```bash
#!/bin/bash
# test-scripts/test-source-auth.sh

echo "=== Testing Source Authentication ==="

# Test 1: Stripe source with webhook secret
hookdeck connection create \
  --name "test-stripe" \
  --source-name "stripe-src" \
  --source-type "STRIPE" \
  --source-webhook-secret "whsec_test" \
  --destination-name "mock" \
  --destination-type "MOCK"

# Test 2: Twilio with basic auth
hookdeck connection create \
  --name "test-twilio" \
  --source-name "twilio-src" \
  --source-type "TWILIO" \
  --source-basic-auth-username "AC..." \
  --source-basic-auth-password "..." \
  --destination-name "mock2" \
  --destination-type "MOCK"

# Test 3: Complex config with JSON
hookdeck connection create \
  --name "test-custom" \
  --source-name "custom-src" \
  --source-type "CUSTOM" \
  --source-config '{"field": "value"}' \
  --destination-name "mock3" \
  --destination-type "MOCK"

# Test 4: Validation error (should fail)
hookdeck connection create \
  --name "test-fail" \
  --source-name "stripe-fail" \
  --source-type "STRIPE" \
  --destination-name "mock4" \
  --destination-type "MOCK"
# Expected: Error about missing --source-webhook-secret
```

## 9. Success Metrics

### Critical Fix
- [x] Identified incorrect architecture (separate creation)
- [ ] Refactored to use inline creation via connection API
- [ ] Single API call creates all resources

### Authentication Implementation
- [ ] 8 source types fully configured (Phase 1)
- [ ] Universal flags cover 80% of patterns
- [ ] JSON fallback handles complex cases
- [ ] Validation provides helpful errors
- [ ] All tests pass

### Performance
- Validation completes in <50ms
- No extra API calls
- Memory usage <5MB

## 10. Files Summary

### Files to Modify
1. `pkg/cmd/connection_create.go` - Complete refactor for inline creation

### Files to Create
1. `pkg/cmd/source_types.go` - Type registry
2. `pkg/cmd/source_config_builder.go` - Config building
3. `pkg/cmd/source_validation.go` - Validation logic
4. Test files for each new component

### Files NOT to Modify
1. `pkg/hookdeck/sources.go` - SourceCreateInput already correct
2. `pkg/hookdeck/connections.go` - ConnectionCreateRequest already correct
3. `pkg/hookdeck/destinations.go` - DestinationCreateInput already correct

## Critical Next Steps

1. **IMMEDIATE**: Fix the architecture issue - stop creating sources/destinations separately
2. **THEN**: Add authentication support using the correct inline approach
3. **FINALLY**: Expand type coverage in phases

This revised plan addresses both the architectural correction needed and adds comprehensive authentication support using the proper API patterns.