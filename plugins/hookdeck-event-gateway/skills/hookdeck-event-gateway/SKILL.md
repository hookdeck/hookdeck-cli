---
name: hookdeck-event-gateway
description: Operate Hookdeck Event Gateway through the Hookdeck CLI and MCP server. Use when an agent needs to inspect webhook traffic, debug failed deliveries, review issues, or plan safe connection/source/destination changes.
---

# Hookdeck Event Gateway Skill

Use this Skill to turn broad webhook operations requests into safe, auditable
Hookdeck CLI or MCP workflows. Prefer read-only MCP inspection before proposing
mutating CLI commands.

## Capabilities

- Inspect sources, destinations, connections, requests, events, attempts, issues, and metrics.
- Trace a webhook from inbound request to generated events and delivery attempts.
- Review failed deliveries, transform errors, retry behavior, and backpressure.
- Plan safe connection changes with explicit human approval before mutation.
- Produce eval metadata without exposing payloads or secrets.

## Required Output

Return a concise note with these sections:

- `Scope`: the Hookdeck project, source, destination, connection, event, request, or issue under review.
- `Evidence`: MCP tools or CLI commands used, with IDs redacted when they are production-sensitive.
- `Findings`: current state, failure pattern, queue/retry status, or configuration risk.
- `Plan`: next steps, separated into read-only checks and mutating actions.
- `Approval Required`: every pause, unpause, retry, cancel, mute, create, update, or delete action.
- `Verification`: follow-up MCP checks, CLI commands, metrics, or delivery evidence.
- `Plugin Eval Metadata`: eval case id, expected pass criteria, and safe metadata events.
- `Risks`: unresolved access, missing context, payload sensitivity, or production impact.

## Workflow

1. Confirm whether the request is read-only or mutating.
2. For read-only requests, prefer `hookdeck gateway mcp` tools through the agent harness.
3. For CLI commands, prefer `hookdeck gateway <resource> <action>` and cite the command without embedding secrets.
4. Trace failures from source/request to event/attempt before recommending retries or connection changes.
5. Ask for human approval before any action that changes delivery flow or resource state.
6. Verify after changes with event, attempt, issue, or metrics checks.

## Acceptance Checks

- Identifies the relevant Hookdeck resource and environment.
- Uses read-only inspection before mutation.
- Separates evidence from recommendations.
- Requires approval before mutating actions.
- Keeps payloads, credentials, and customer data out of examples and telemetry.

## Privacy And Telemetry Boundary

Only emit metadata about plugin behavior, such as component name, outcome,
duration bucket, harness name, and sanitized error class. Do not emit prompts,
source files, webhook payloads, API keys, signing secrets, customer data, tool
arguments, or model outputs.
