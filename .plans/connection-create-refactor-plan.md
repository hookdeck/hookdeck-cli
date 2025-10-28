# Implementation Plan: Refactor `hookdeck connection create` for Single API Call

**Author:** Roo
**Date:** 2025-10-28
**Status:** Proposed

## 1. Overview

This document outlines the plan to refactor the `hookdeck connection create` command to use a single API call for creating a connection with inline sources and destinations. The current implementation makes three separate API calls, which is inefficient and does not align with the capabilities of the Hookdeck API.

The goal is to modify the command to construct a single `POST /connections` request with nested `source` and `destination` objects, streamlining the creation process and aligning the CLI with API best practices.

## 2. Analysis of the Problem

The current implementation in `pkg/cmd/connection_create.go` follows this sequence:
1.  If creating a source inline, it sends a `POST /sources` request.
2.  If creating a destination inline, it sends a `POST /destinations` request.
3.  It sends a final `POST /connections` request, referencing the newly created source and destination by their IDs.

This approach is flawed because the Hookdeck API's `POST /connections` endpoint is designed to handle the creation of all three resources in a single transaction. The existing `hookdeck.ConnectionCreateRequest` struct already supports this, but the command logic does not utilize it correctly.

## 3. Proposed Architecture

The refactoring will focus on modifying the `runConnectionCreateCmd` function to build a single, comprehensive `hookdeck.ConnectionCreateRequest` struct.

### Key Changes:
-   **API Client (`pkg/hookdeck/connections.go`):** No changes are required. The `ConnectionCreateRequest` struct already supports nested `SourceCreateInput` and `DestinationCreateInput`.
-   **Command Logic (`pkg/cmd/connection_create.go`):**
    -   The `runConnectionCreateCmd` function will be modified to populate the `Source` and `Destination` fields of the `ConnectionCreateRequest` struct instead of making separate API calls.
    -   The now-redundant calls to `client.CreateSource` and `client.CreateDestination` will be removed.
    -   The helper functions `buildSourceConfig` and `buildDestinationConfig` will be adapted to return the required `hookdeck.SourceCreateInput` and `hookdeck.DestinationCreateInput` structs.

## 4. Implementation Steps

### Step 1: Update `runConnectionCreateCmd` in `pkg/cmd/connection_create.go`

The core of the work is to refactor this function. Instead of separate API calls, we will build a single request object.

**Current Logic (Simplified):**
```go
// pkg/cmd/connection_create.go

func (cc *connectionCreateCmd) runConnectionCreateCmd(cmd *cobra.Command, args []string) error {
    // ...
    // 1. Create source if specified inline
    source, err := client.CreateSource(...)
    // ...
    
    // 2. Create destination if specified inline
    dest, err := client.CreateDestination(...)
    // ...

    // 3. Create connection using IDs
    connection, err := client.CreateConnection(context.Background(), &hookdeck.ConnectionCreateRequest{
        SourceID:      &source.ID,
        DestinationID: &dest.ID,
        // ...
    })
    // ...
}
```

**Refactored Logic (Simplified):**
```go
// pkg/cmd/connection_create.go

func (cc *connectionCreateCmd) runConnectionCreateCmd(cmd *cobra.Command, args []string) error {
    client := Config.GetAPIClient()

    req := &hookdeck.ConnectionCreateRequest{
        Name:        &cc.name,
        Description: &cc.description,
    }

    // Handle Source
    if cc.sourceID != "" {
        req.SourceID = &cc.sourceID
    } else {
        sourceInput, err := cc.buildSourceInput()
        if err != nil {
            return err
        }
        req.Source = sourceInput
    }

    // Handle Destination
    if cc.destinationID != "" {
        req.DestinationID = &cc.destinationID
    } else {
        destinationInput, err := cc.buildDestinationInput()
        if err != nil {
            return err
        }
        req.Destination = destinationInput
    }

    // Single API call to create the connection
    connection, err := client.CreateConnection(context.Background(), req)
    if err != nil {
        return fmt.Errorf("failed to create connection: %w", err)
    }

    // ... (display results)
    return nil
}
```

### Step 2: Create `buildSourceInput` and `buildDestinationInput` helpers

We will replace `buildSourceConfig` and `buildDestinationConfig` with new functions that construct the required input structs.

**New `buildSourceInput` function:**
```go
// pkg/cmd/connection_create.go

func (cc *connectionCreateCmd) buildSourceInput() (*hookdeck.SourceCreateInput, error) {
    var description *string
    if cc.sourceDescription != "" {
        description = &cc.sourceDescription
    }

    // This logic can be extracted from the old buildSourceConfig
    // and adapted to the new struct.
    sourceConfig, err := cc.buildSourceConfig() // This function will need to be adapted
    if err != nil {
        return nil, fmt.Errorf("error building source config: %w", err)
    }

    return &hookdeck.SourceCreateInput{
        Name:        cc.sourceName,
        Description: description,
        Type:        cc.sourceType,
        Config:      sourceConfig,
    }, nil
}
```

**New `buildDestinationInput` function:**
```go
// pkg/cmd/connection_create.go

func (cc *connectionCreateCmd) buildDestinationInput() (*hookdeck.DestinationCreateInput, error) {
    var description *string
    if cc.destinationDescription != "" {
        description = &cc.destinationDescription
    }

    // This logic can be extracted from the old buildDestinationConfig
    // and adapted to the new struct.
    destinationConfig, err := cc.buildDestinationConfig() // This function will need to be adapted
    if err != nil {
        return nil, fmt.Errorf("error building destination config: %w", err)
    }
    
    input := &hookdeck.DestinationCreateInput{
        Name:        cc.destinationName,
        Description: description,
        Type:        cc.destinationType,
        Config:      destinationConfig,
    }

    if cc.destinationURL != "" {
        input.URL = &cc.destinationURL
    }
    if cc.destinationCliPath != "" {
        input.CliPath = &cc.destinationCliPath
    }

    return input, nil
}
```

### Step 3: Update `hookdeck.SourceCreateInput` and `hookdeck.DestinationCreateInput`

The structs in `pkg/hookdeck/sources.go` and `pkg/hookdeck/destinations.go` may need to be updated to include all necessary fields for inline creation if they are not already present. A quick review of those files will be necessary.

Assuming they are defined as follows (if not, they should be updated):

```go
// pkg/hookdeck/sources.go
type SourceCreateInput struct {
    Name        string                 `json:"name"`
    Description *string                `json:"description,omitempty"`
    Type        string                 `json:"type"`
    Config      map[string]interface{} `json:"config,omitempty"`
}

// pkg/hookdeck/destinations.go
type DestinationCreateInput struct {
    Name        string                 `json:"name"`
    Description *string                `json:"description,omitempty"`
    Type        string                 `json:"type"`
    URL         *string                `json:"url,omitempty"`
    CliPath     *string                `json:"cli_path,omitempty"`
    Config      map[string]interface{} `json:"config,omitempty"`
}
```

## 5. Validation and Error Handling

-   The existing flag validation in `validateFlags` remains relevant and should be preserved.
-   Error handling within `runConnectionCreateCmd` will be simplified, as there is now only one API call to manage. A failure in the single `CreateConnection` call will indicate a problem with the entire operation.

## 6. Testing Strategy

-   **Unit Tests:** Update existing unit tests for `connection_create.go` to mock the single `CreateConnection` call and verify that the correct nested request is being built.
-   **Acceptance Tests:** The existing acceptance tests in `test-scripts/test-acceptance.sh` should be reviewed and updated to ensure they still pass with the new implementation. New tests should be added to cover inline creation with various authentication methods.

## 7. Conclusion

This refactoring will significantly improve the architecture of the `hookdeck connection create` command, making it more efficient and robust. By aligning the CLI with the API's capabilities, we reduce network latency and simplify the command's internal logic.