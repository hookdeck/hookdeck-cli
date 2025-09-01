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
- **Collections**: Use comma-separated values (`--connections "a,b,c"`)
- **Booleans**: Use presence flags (`--email`, `--pagerduty`, `--force`)

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
  --destination-type HTTP --destination-name "my-api" \
  --destination-url "https://api.example.com/webhooks"
```

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

### Validation Layers (in order)
1. **Flag parsing validation** - Ensure flag values are correctly typed
2. **Type-specific validation** - Validate based on `--type` parameter
3. **Cross-parameter validation** - Check relationships between parameters
4. **API schema validation** - Final validation against OpenAPI constraints

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

### Building and Testing
```bash
# Build the CLI
go build -o hookdeck cmd/hookdeck/main.go

# Run tests
go test ./...

# Run specific package tests
go test ./pkg/cmd/

# Run with race detection
go test -race ./...
```

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

## 6. Documentation Standards

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
| `go test ./pkg/cmd/` | Test command implementations |
| `go generate ./...` | Run code generation (if used) |
| `golangci-lint run` | Run comprehensive linting |

## 11. Common Patterns to Follow

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

Following these guidelines ensures consistent, maintainable CLI commands that provide an excellent user experience while maintaining architectural consistency with the existing codebase.