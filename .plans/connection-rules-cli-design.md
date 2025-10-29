# Connection Rule Configuration CLI Design

This document outlines the design for command-line flags to configure ordered rules for Hookdeck connections.

## 1. Guiding Principles

1.  **Clarity and Predictability**: The mapping from flags to API rules should be intuitive. Users should be able to predict the outcome based on the flags they provide.
2.  **Order Matters**: The CLI must respect the order of flags to determine the execution order of the rules. The first rule-related flag encountered sets the position of that rule in the execution chain.
3.  **Consistency**: Flag names should follow the existing conventions outlined in `AGENTS.md`, using a `rule-<type>-<property>` pattern.
4.  **Atomicity for Complex Rules**: For rules with multiple properties (like `filter`), the first flag of that rule type determines its order. Subsequent flags for the same rule type will amend the existing rule object rather than creating a new one.
5.  **Escape Hatch**: A JSON-based fallback mechanism (`--rules` and `--rules-file`) will be provided for complex configurations that are difficult to express with flags.


## 2. Flag Naming Convention

All rule-related flags will follow a consistent pattern: `--rule-<type>-<property>`. This convention is clear, avoids conflicts, and aligns with the `AGENTS.md` guidelines for flattening nested API objects.

-   `<type>`: The type of the rule (e.g., `filter`, `retry`, `transform`).
-   `<property>`: The specific attribute of the rule to be configured (e.g., `body`, `strategy`, `name`).

**Example:**
-   `--rule-retry-strategy=linear`
-   `--rule-filter-body='{"$.type":"payment"}'`
-   `--rule-transform-name="my-transform"`

## 3. Rule Ordering Mechanism

The execution order of the rules is determined by the order in which the rule-related flags appear in the command.

-   The **first occurrence of a flag for a specific rule type** (e.g., the first `--rule-filter-*` flag) establishes that rule's position in the execution order.
-   Subsequent flags for the **same rule type** will modify the existing rule object without changing its position.
-   Only **one rule of each type** is supported, as per the API constraints. If multiple flags for the same property are provided (e.g., two `--rule-retry-count` flags), the command will fail validation.

**Example:**
```bash
hookdeck connection create \
  --rule-filter-body '{"$.type":"payment"}' \  # 1. Filter rule is first
  --rule-transform-name "my-transform" \      # 2. Transform rule is second
  --rule-filter-headers '{"$.auth":"present"}' # Modifies the first rule
```
This command will generate a `rules` array where the `filter` rule is at index 0 and the `transform` rule is at index 1.

## 4. Complete Flag List by Rule Type

### 4.1 Retry Rule

| Flag | Type | Description |
|---|---|---|
| `--rule-retry-strategy` | `string` | The retry strategy. Accepted values: `linear`, `exponential`. |
| `--rule-retry-count` | `integer` | The number of retry attempts. |
| `--rule-retry-interval` | `integer` | The interval between retries in milliseconds. |
| `--rule-retry-response-status-codes` | `string` | Comma-separated list of HTTP status codes to retry on (e.g., "500-599,!401,404"). |

### 4.2 Filter Rule

| Flag | Type | Description |
|---|---|---|
| `--rule-filter-body` | `string` | A JQ expression to filter on the request body. |
| `--rule-filter-headers` | `string` | A JQ expression to filter on the request headers. |
| `--rule-filter-query` | `string` | A JQ expression to filter on the request query parameters. |
| `--rule-filter-path` | `string` | A JQ expression to filter on the request path. |

### 4.3 Transform Rule

| Flag | Type | Description |
|---|---|---|
| `--rule-transform-name` | `string` | The name or ID of the transformation to apply. |
| `--rule-transform-code` | `string` | The transformation code (if creating inline). |
| `--rule-transform-env` | `string` | A JSON string representing environment variables for the transformation. |

### 4.4 Delay Rule

| Flag | Type | Description |
|---|---|---|
| `--rule-delay-delay` | `integer` | The delay in milliseconds. |

### 4.5 Deduplicate Rule

| Flag | Type | Description |
|---|---|---|
| `--rule-deduplicate-window` | `integer` | Time window in milliseconds for deduplication. |
| `--rule-deduplicate-include-fields` | `string` | Comma-separated list of fields to include for deduplication. |
| `--rule-deduplicate-exclude-fields` | `string` | Comma-separated list of fields to exclude for deduplication. |


## 5. Validation Rules

Validation will be performed in the `PreRunE` phase of the Cobra command to ensure that the combination of flags is valid before making an API call.

### 5.1 General Rules
- If any `--rule-*` flag is used, the corresponding rule object will be constructed.
- A maximum of one rule per type is allowed. The CLI will fail if, for example, a retry rule is defined using both flags and the JSON fallback.

### 5.2 Per-Type Validation

-   **Retry Rule**:
    -   If any `--rule-retry-*` flag is provided, `--rule-retry-strategy` is required.
    -   `--rule-retry-strategy` must be one of `linear` or `exponential`.
    -   `--rule-retry-count` and `--rule-retry-interval` must be positive integers.

-   **Filter Rule**:
    -   If any `--rule-filter-*` flag is provided, at least one of `--rule-filter-body`, `--rule-filter-headers`, `--rule-filter-query`, or `--rule-filter-path` must not be empty.

-   **Transform Rule**:
    -   If any `--rule-transform-*` flag is provided, `--rule-transform-name` is required.
    -   `--rule-transform-env` must be a valid JSON string if provided.

-   **Delay Rule**:
    -   `--rule-delay-delay` must be a positive integer.

-   **Deduplicate Rule**:
    -   If any `--rule-deduplicate-*` flag is provided, `--rule-deduplicate-window` is required.
    -   `--rule-deduplicate-window` must be a positive integer.

## 6. JSON Fallback Strategy

For complex configurations or for users who prefer to manage rules as code, a JSON fallback mechanism will be provided. This aligns with the existing strategy for source and destination configurations.

| Flag | Type | Description |
|---|---|---|
| `--rules` | `string` | A JSON string representing the entire `rules` array. |
| `--rules-file` | `string` | Path to a JSON file containing the `rules` array. |

**Behavior:**
- If `--rules` or `--rules-file` is used, all individual `--rule-*` flags will be ignored.
- The JSON provided must be a valid array of rule objects, conforming to the Hookdeck API schema.
- The order of the rules in the JSON array will be preserved.


## 7. Usage Examples

### Example 1: Simple Retry and Filter
This command creates a connection that first retries failed webhooks, then filters them.
```bash
hookdeck connection create \
  --source-name "my-source" \
  --destination-name "my-destination" \
  --rule-retry-strategy="exponential" \
  --rule-retry-count=3 \
  --rule-filter-body='{"$.event_type":"user.created"}'
```
**Resulting `rules` array:**
```json
[
  {"type": "retry", "strategy": "exponential", "count": 3},
  {"type": "filter", "body": "{\"$.event_type\":\"user.created\"}"}
]
```

### Example 2: Filter with Multiple Properties
The filter rule is defined by the first `--rule-filter-*` flag. Subsequent flags for the same rule amend the original rule object.
```bash
hookdeck connection create \
  --source-name "my-source" \
  --destination-name "my-destination" \
  --rule-filter-headers='{"$.content-type":"application/json"}' \
  --rule-transform-name="my-transform" \
  --rule-filter-body='{"$.status":"completed"}'
```
**Resulting `rules` array:**
```json
[
  {
    "type": "filter",
    "headers": "{\"$.content-type\":\"application/json\"}",
    "body": "{\"$.status\":\"completed\"}"
  },
  {"type": "transform", "transformation_id": "my-transform"}
]
```

### Example 3: Using the JSON Fallback
This example uses a JSON file to define the rules, which is useful for complex configurations.
```bash
# rules.json
# [
#   {"type": "delay", "delay": 5000},
#   {"type": "retry", "strategy": "linear", "count": 5, "interval": 10000}
# ]

hookdeck connection create \
  --source-name "my-source" \
  --destination-name "my-destination" \
  --rules-file="rules.json"
```
**Resulting `rules` array:**
```json
[
  {"type": "delay", "delay": 5000},
  {"type": "retry", "strategy": "linear", "count": 5, "interval": 10000}
]
```

## 8. Edge Cases

-   **Duplicate Rule Types**: If a user specifies a rule type with both flags and the JSON fallback (e.g., `--rule-retry-strategy` and `--rules` containing a retry rule), the CLI will exit with an error, as the JSON fallback takes precedence and all individual flags are ignored.
-   **Conflicting Properties**: If a user provides the same flag twice (e.g., `--rule-retry-count=3 --rule-retry-count=5`), Cobra will automatically use the last value provided. Our validation will not need to handle this explicitly.
-   **Empty Rule Properties**: If a flag is provided with an empty value (e.g., `--rule-filter-body=""`), it will be treated as an empty string and passed to the API. The API will then perform its own validation.

## 9. Implementation Guidance

This section provides guidance for the `code` mode to implement this design.

### 9.1 Flag Parsing and Rule Construction
-   Use a custom `RuleSet` struct to manage the rules as they are parsed. This struct should maintain the order of the rules and the properties of each rule.
-   Iterate through the command's flags in the `PreRunE` function. When a `--rule-*` flag is encountered, add the rule to the `RuleSet`.
-   The `RuleSet` should have a method to convert the rules into the JSON format expected by the API.

### 9.2 Cobra Flag Definition
-   Define all `--rule-*` flags in the `connection_create.go` and `connection_update.go` files.
-   Use `pflag` to access the flags in the order they were set on the command line. This is crucial for preserving the rule order.

### 9.3 Validation Logic
-   Implement the validation rules defined in Section 5 within the `PreRunE` function.
-   Provide clear and actionable error messages to the user if validation fails, as per `AGENTS.md`.

### 9.4 API Client
-   Update the `ConnectionCreate` and `ConnectionUpdate` functions in the API client to accept the `rules` array.
-   Ensure that the API client correctly serializes the `rules` array into the request body.
