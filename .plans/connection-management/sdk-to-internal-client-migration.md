# SDK to Internal Client Migration Plan

## Status: Phase 1 Complete ✅ | Phases 2-4 Pending

## Objective

Migrate all connection management commands from the deprecated `github.com/hookdeck/hookdeck-go-sdk` to the internal `pkg/hookdeck/client.go` wrapper.

## Quick Reference
- **Files Created**: `pkg/hookdeck/connections.go`, `pkg/hookdeck/sources.go`, `pkg/hookdeck/destinations.go`
- **Files to Rewrite**: 5 connection commands
- **New Commands to Add**: 7 lifecycle/utility commands
- **Estimated Remaining Time**: 4-6 hours

## Scope

## Progress Tracking

### Phase 1: Internal Client Methods ✅ COMPLETE
- [x] Create `pkg/hookdeck/connections.go` with types and methods
- [x] Create `pkg/hookdeck/sources.go` with types and methods
- [x] Create `pkg/hookdeck/destinations.go` with types and methods

### Phase 2: Rewrite Commands (5 commands)
- [ ] Rewrite `pkg/cmd/connection_create.go`
- [ ] Rewrite `pkg/cmd/connection_list.go`
- [ ] Rewrite `pkg/cmd/connection_get.go`
- [ ] Rewrite `pkg/cmd/connection_update.go`
- [ ] Rewrite `pkg/cmd/connection_delete.go`

### Phase 3: Add New Commands (7 commands)
- [ ] Add `pkg/cmd/connection_enable.go`
- [ ] Add `pkg/cmd/connection_disable.go`
- [ ] Add `pkg/cmd/connection_pause.go`
- [ ] Add `pkg/cmd/connection_unpause.go`
- [ ] Add `pkg/cmd/connection_archive.go`
- [ ] Add `pkg/cmd/connection_unarchive.go`
- [ ] Add `pkg/cmd/connection_count.go`

### Phase 4: Testing & Documentation
- [ ] Test all commands with actual API
- [ ] Update REFERENCE.md with new capabilities
- [ ] Update root command to register new commands

## OpenAPI Analysis

### Available Endpoints:
- `GET /connections` - List connections
- `GET /connections/count` - Count connections
- `GET /connections/{id}` - Get connection
- `POST /connections` - Create connection
- `PUT /connections/{id}` - Update connection
- `DELETE /connections/{id}` - Delete connection
- `PUT /connections/{id}/enable` - Enable connection
- `PUT /connections/{id}/disable` - Disable connection
- `PUT /connections/{id}/pause` - Pause connection
- `PUT /connections/{id}/unpause` - Unpause connection
- `PUT /connections/{id}/archive` - Archive connection
- `PUT /connections/{id}/unarchive` - Unarchive connection

### Connection Create Request Structure:
```json
{
  "name": "string",
  "description": "string",
  "source_id": "string",
  "destination_id": "string",
  "source": {
    "name": "string",
    "type": "WEBHOOK|STRIPE|GITHUB|...",
    "description": "string",
    "config": { /* SourceTypeConfig */ }
  },
  "destination": {
    "name": "string",
    "type": "HTTP|CLI|MOCK_API",
    "description": "string",
    "config": { /* VerificationConfig */ }
  },
  "rules": [
    /* RetryRule | FilterRule | TransformRule | DelayRule | DeduplicateRule */
  ]
}
```

### Supported Rules (all inline):
1. **RetryRule** - Retry configuration
2. **FilterRule** - Event filtering
3. **TransformRule** - Payload transformation
4. **DelayRule** - Delayed delivery
5. **DeduplicateRule** - Deduplication logic

## Implementation Strategy

### Phase 1: Create Internal Client Methods
1. Create `pkg/hookdeck/connections.go` with:
   - Type definitions from OpenAPI spec
   - CRUD methods using `client.Get()`, `client.Post()`, `client.Put()`, `client.Delete()`
   - Lifecycle methods (enable, disable, pause, unpause, archive, unarchive)

2. Create `pkg/hookdeck/sources.go` with:
   - Source types and request/response structs
   - Create source method

3. Create `pkg/hookdeck/destinations.go` with:
   - Destination types and request/response structs
   - Create destination method

### Phase 2: Rewrite Connection Commands
1. Update all 5 existing commands to use internal client methods
2. Remove all SDK imports (`hookdecksdk`, `hookdeckclient`, `hookdeckoption`)
3. Use internal `pkg/config.Config.GetAPIClient()` instead of `GetClient()`

### Phase 3: Add New Commands
1. Implement lifecycle commands (enable, disable, pause, unpause, archive, unarchive)
2. Implement count command
3. Update REFERENCE.md to reflect new capabilities

### Phase 4: Testing
1. Test all commands with actual API
2. Verify inline source/destination creation
3. Test lifecycle operations
4. Validate rules support

## Success Criteria
- ✅ No more `github.com/hookdeck/hookdeck-go-sdk` imports in connection commands
- ✅ All 5 basic commands working with internal client
- ✅ Lifecycle commands implemented
- ✅ Count command implemented
- ✅ Rules support working (if time permits)
- ✅ All commands tested successfully

## Timeline
- Phase 1: 2-3 hours (create internal client methods)
- Phase 2: 2-3 hours (rewrite 5 commands)
- Phase 3: 1-2 hours (add new commands)
- Phase 4: 1 hour (testing)
- **Total**: 6-9 hours