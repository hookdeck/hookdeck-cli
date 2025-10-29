# Connection Upsert Command Design

## 1. Command Overview

### Purpose and Use Cases
The `hookdeck connection upsert` command provides an idempotent mechanism for creating or updating connections. It is designed for users who manage their webhook infrastructure as code, enabling them to define the desired state of a connection without worrying about its current state.

- **Primary Use Case:** Declarative connection management in CI/CD pipelines or local scripts.
- **Secondary Use Case:** A convenient way to make partial updates to a connection without fetching its full state.

### Comparison with `create`
- `hookdeck connection create`: Explicitly creates a new connection. Fails if a connection with the same name already exists. Use this when you intend to create a new resource and want to avoid accidental overwrites.
- `hookdeck connection upsert`: Creates a connection if it doesn't exist by name, or updates it if it does. Use this for idempotent operations where the end state is what matters.

## 2. Command Signature

The command will use the connection `name` as a required positional argument, making it distinct and easy to use.

```bash
hookdeck connection upsert <name> [flags]
```

## 3. Flag Definitions

The `upsert` command will reuse all flags from the `connection create` command to ensure full control over connection properties. A new `--dry-run` flag will be added to preview changes.

- **All flags will be optional.** When updating, only the provided flags will modify the connection's properties.
- **`--dry-run` (boolean):** If present, the command will not perform the final `PUT` request. Instead, it will output a summary of the intended action (create or update) and the data that would be sent.

## 4. Dry-Run Behavior

The `--dry-run` flag is critical for safely managing infrastructure.

### Implementation
1. The command will first make a `GET /connections?name=<name>` API call to check if a connection with the given name exists.
2. Based on the result, it will determine whether the operation is a **CREATE** or an **UPDATE**.
3. It will construct the full request payload just as it would for a real operation.
4. It will output a summary indicating the action and the payload.

### Output Format
The dry-run output will be a human-readable text summary, with an option for JSON output for scripting.

**Text Summary Example (Create):**
```
-- Dry Run: CREATE --
A new connection named 'my-connection' will be created with the following properties:
- Source: my-source (inline)
- Destination: my-destination (inline)
- Rules:
  - Retry: exponential, 3 attempts
```

**Text Summary Example (Update):**
```
-- Dry Run: UPDATE --
Connection 'my-connection' (conn_123) will be updated with the following changes:
- Description: "New description"
- Rules: (ruleset will be replaced)
  - Filter: body contains '{"$.type":"payment"}'
```

**JSON Output (`--output json`):**
```json
{
  "action": "CREATE",
  "connection_name": "my-connection",
  "payload": {
    "name": "my-connection",
    "source": { "...etc" },
    "destination": { "...etc" }
  }
}
```

## 5. Behavior Specification

- **When connection doesn't exist:** The command behaves like `create`. It will create a new connection using the provided flags. `source` and `destination` (either by ID or inline) will be required in this case.
- **When connection exists:** The command will perform an update. Only the properties corresponding to the provided flags will be changed.
- **When no properties provided:** If the connection exists and no flags are provided to modify it, the command will be a **no-op**. It will print a message indicating that no changes are needed. This is not an error.
- **Rule Handling:** If *any* `--rule-*` flag or `--rules-file`/`--rules` is provided, the entire existing ruleset on the connection will be **replaced** with the new rules defined in the command. This ensures declarative and idempotent rule management.

## 6. Validation Rules

Validation logic must accommodate both create and update scenarios.

1. **Initial Check:** The command will first check if the connection exists via a `GET` request.
2. **Create Scenario Validation:** If the connection does not exist:
   - `source` (via `--source-id` or inline flags) is required.
   - `destination` (via `--destination-id` or inline flags) is required.
   - All other validation from `connection create` applies.
3. **Update Scenario Validation:** If the connection exists:
   - All flags are optional.
   - Validation will only be performed for the flags that are provided.

This approach avoids complex client-side logic and leverages the API for validation where appropriate, while providing helpful early feedback to the user.

## 7. Usage Examples

```bash
# Create a new connection if 'my-connection' does not exist
hookdeck connection upsert my-connection \
  --source-name "my-source" --source-type STRIPE \
  --destination-name "my-api" --destination-type HTTP --destination-url "https://example.com"

# Update the rules for an existing connection, leaving other properties untouched
hookdeck connection upsert my-connection \
  --rule-retry-strategy linear --rule-retry-count 5

# Preview an update with --dry-run
hookdeck connection upsert my-connection \
  --description "A new description" --dry-run

# No-op: connection exists, no flags provided
hookdeck connection upsert my-connection
```

## 8. Implementation Guidance

- **Shared Code:** Create a shared `internal/connection` package or a `connection_shared.go` file to house common logic for flag definitions, payload construction, and rule parsing, to be used by both `create` and `upsert` commands.
- **API Client:** Add a new `UpsertConnection` method to the API client that makes a `PUT /connections` call. The method should accept the connection name and the upsert payload.
- **Dry-Run Logic:** The core logic for the dry run will live in the `upsert` command's `RunE` function. It will perform the initial `GET` request and then branch to either display the dry-run output or call the `UpsertConnection` method.
- **Removing `update` command:** Delete `pkg/cmd/connection_update.go` and remove its registration from the parent `connection` command.

## 9. Migration Strategy

- **Remove `update` command:** The `connection update` command will be completely removed.
- **Documentation:** Update `REFERENCE.md` and all other documentation to replace `update` with `upsert`, providing clear examples.
- **CHANGELOG:** Add an entry under "Breaking Changes" announcing the replacement of `connection update` with `connection upsert` and explaining the benefits.
- **User Communication:** Announce the change in release notes, highlighting the new idempotent capabilities and `--dry-run` support.

## 10. Testing Strategy

- **Create Behavior:** Write a test case where the connection name does not exist, and verify that it is created correctly.
- **Update Behavior:** Test that when a connection exists, providing a subset of flags only updates those properties.
- **Idempotency:** Run the same `upsert` command twice and verify that the state is the same after both runs and the second run reports a no-op.
- **Dry-Run Accuracy:** For both create and update scenarios, test that the `--dry-run` output accurately reflects the action and payload.
- **Rule Replacement:** Test that providing rule flags completely replaces the existing ruleset on a connection.