# Connection Commands Implementation - Orchestration Plan

## Overview

Complete migration of connection management from deprecated SDK to internal API client, plus add missing lifecycle and utility commands.

## Current Status

**✅ Phase 1 Complete:** Internal client methods created
- `pkg/hookdeck/connections.go` - 12 methods, all types
- `pkg/hookdeck/sources.go` - Source types and CreateSource()
- `pkg/hookdeck/destinations.go` - Destination types and CreateDestination()

**🔄 Phases 2-4 Pending:** Command rewrites, new commands, testing

## Orchestration Tasks

### Task 1: Rewrite connection_create.go
**Mode:** Code  
**Description:** Rewrite connection create command to use internal client instead of deprecated SDK  
**Input Files:**
- `pkg/cmd/connection_create.go` (current implementation)
- `pkg/hookdeck/connections.go` (new client methods)
- `pkg/hookdeck/sources.go` (source creation)
- `pkg/hookdeck/destinations.go` (destination creation)

**Requirements:**
1. Remove SDK imports (`hookdecksdk`, `hookdeckclient`)
2. Use `Config.GetAPIClient()` instead of `Config.GetClient()`
3. Replace SDK calls with internal client methods
4. Handle inline source/destination creation using new API
5. Preserve all existing flags and validation logic
6. Maintain output formatting

**Success Criteria:**
- Compiles without SDK dependencies
- All flags work correctly
- Inline resource creation works
- Output matches previous format

---

### Task 2: Rewrite connection_list.go
**Mode:** Code  
**Description:** Rewrite connection list command to use internal client

**Input Files:**
- `pkg/cmd/connection_list.go`
- `pkg/hookdeck/connections.go`

**Requirements:**
1. Replace SDK ListConnections call with `client.ListConnections()`
2. Update response parsing for internal client types
3. Preserve filtering logic (name, source-id, destination-id, disabled, paused)
4. Maintain color-coded status display

**Success Criteria:**
- Compiles without SDK dependencies
- Filtering works correctly
- Status display preserved

---

### Task 3: Rewrite connection_get.go
**Mode:** Code  
**Description:** Rewrite connection get command to use internal client

**Input Files:**
- `pkg/cmd/connection_get.go`
- `pkg/hookdeck/connections.go`

**Requirements:**
1. Replace SDK GetConnection call with `client.GetConnection()`
2. Update response parsing
3. Preserve detailed output format

**Success Criteria:**
- Compiles without SDK dependencies
- Shows all connection details correctly

---

### Task 4: Rewrite connection_update.go
**Mode:** Code  
**Description:** Rewrite connection update command to use internal client

**Input Files:**
- `pkg/cmd/connection_update.go`
- `pkg/hookdeck/connections.go`

**Requirements:**
1. Replace SDK UpdateConnection call with `client.UpdateConnection()`
2. Update request structure
3. Preserve validation (at least one flag required)

**Success Criteria:**
- Compiles without SDK dependencies
- Update functionality works

---

### Task 5: Rewrite connection_delete.go
**Mode:** Code  
**Description:** Rewrite connection delete command to use internal client

**Input Files:**
- `pkg/cmd/connection_delete.go`
- `pkg/hookdeck/connections.go`

**Requirements:**
1. Replace SDK DeleteConnection call with `client.DeleteConnection()`
2. Preserve confirmation prompt
3. Maintain --force flag behavior

**Success Criteria:**
- Compiles without SDK dependencies
- Confirmation works correctly
- Force flag bypasses confirmation

---

### Task 6: Add connection lifecycle commands
**Mode:** Code  
**Description:** Create 6 new lifecycle management commands

**New Files to Create:**
1. `pkg/cmd/connection_enable.go` - Enable connection
2. `pkg/cmd/connection_disable.go` - Disable connection
3. `pkg/cmd/connection_pause.go` - Pause connection
4. `pkg/cmd/connection_unpause.go` - Unpause connection
5. `pkg/cmd/connection_archive.go` - Archive connection
6. `pkg/cmd/connection_unarchive.go` - Unarchive connection

**Pattern for Each:**
```go
package cmd

import (
    "context"
    "fmt"
    "github.com/spf13/cobra"
    "github.com/hookdeck/hookdeck-cli/pkg/config"
    "github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type connectionEnableCmd struct {
    cmd *cobra.Command
}

func newConnectionEnableCmd() *connectionEnableCmd {
    cc := &connectionEnableCmd{}
    cc.cmd = &cobra.Command{
        Use:   "enable <connection-id>",
        Args:  validators.ExactArgs(1),
        Short: "Enable a connection",
        RunE:  cc.runConnectionEnableCmd,
    }
    return cc
}

func (cc *connectionEnableCmd) runConnectionEnableCmd(cmd *cobra.Command, args []string) error {
    client := config.Config.GetAPIClient()
    ctx := context.Background()
    
    conn, err := client.EnableConnection(ctx, args[0])
    if err != nil {
        return fmt.Errorf("failed to enable connection: %w", err)
    }
    
    fmt.Printf("✓ Connection enabled: %s (%s)\n", *conn.Name, conn.ID)
    return nil
}
```

**Success Criteria:**
- All 6 commands created
- All compile successfully
- Registered with connection command group
- Proper error handling

---

### Task 7: Add connection count command
**Mode:** Code  
**Description:** Create count command for connections

**New File:** `pkg/cmd/connection_count.go`

**Requirements:**
1. Accept same filters as list command
2. Use `client.CountConnections()`
3. Display count result

**Success Criteria:**
- Command created and registered
- Filtering works correctly
- Count displayed properly

---

### Task 8: Update command registration
**Mode:** Code  
**Description:** Register all new commands with connection command group

**Files to Modify:**
- `pkg/cmd/connection.go`

**Requirements:**
1. Add all 7 new subcommands to connection command group
2. Ensure proper command hierarchy
3. Update help text if needed

**Success Criteria:**
- All commands show in `hookdeck connection --help`
- Commands are accessible

---

### Task 9: Update documentation
**Mode:** Code  
**Description:** Update REFERENCE.md with new capabilities

**Files to Modify:**
- `REFERENCE.md`

**Requirements:**
1. Mark lifecycle commands as ✅ implemented
2. Mark count command as ✅ implemented
3. Update examples with real command usage
4. Remove 🚧 planned markers for completed features

**Success Criteria:**
- Documentation reflects actual implementation
- Examples are accurate

---

### Task 10: Comprehensive testing
**Mode:** Code  
**Description:** Test all commands with actual Hookdeck API

**Test Scenarios:**
1. Create connection with inline source/destination
2. Create connection with existing resources
3. List connections with various filters
4. Get connection details
5. Update connection
6. Delete connection (with and without --force)
7. Enable/disable connection
8. Pause/unpause connection
9. Archive/unarchive connection
10. Count connections

**Success Criteria:**
- All commands work with real API
- No SDK-related errors
- Output is correct and formatted properly
- Error handling works as expected

---

## Execution Order

**Sequential execution recommended:**
1. Tasks 1-5: Rewrite existing commands (parallel safe)
2. Tasks 6-7: Add new commands (parallel safe)
3. Task 8: Register commands (after 6-7)
4. Task 9: Update docs (after 8)
5. Task 10: Test everything (final verification)

## Deliverables

✅ **Phase 1 Complete:**
- Internal client methods in `pkg/hookdeck/`

⏳ **Phase 2-4 Pending:**
- 5 rewritten commands using internal client
- 7 new lifecycle/utility commands
- Updated command registration
- Updated documentation
- Comprehensive test results

## Time Estimates

- **Tasks 1-5** (Rewrites): 2-3 hours
- **Tasks 6-7** (New commands): 1-2 hours
- **Tasks 8-9** (Registration & docs): 30 mins
- **Task 10** (Testing): 1 hour
- **Total**: 4.5-6.5 hours

## Resources

- **Migration Plan**: `.plans/sdk-to-internal-client-migration.md`
- **OpenAPI Spec**: `.plans/hookdeck-openapi-2025-07-01.json`
- **Schemas**: `.plans/connection-*-schema.json`, `.plans/rule-schema.json`