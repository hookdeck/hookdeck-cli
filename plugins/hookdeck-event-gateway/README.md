# Hookdeck Event Gateway Agent Plugin

This plugin packages Hookdeck Event Gateway operating guidance for agent
workspaces such as Codex, Claude, Copilot, and other MCP-compatible harnesses.

The plugin is the installable package. A Skill is one capability inside the
plugin, usually a focused `SKILL.md` workflow guide. This first package includes
a Skill for inspecting webhook traffic through `hookdeck gateway mcp`, reviewing
failures, and planning safe connection changes.

## Install Anywhere

Configure the Hookdeck MCP server in your agent harness:

```json
{
  "mcpServers": {
    "hookdeck": {
      "command": "hookdeck",
      "args": ["gateway", "mcp"]
    }
  }
}
```

Then use the Skill in `skills/hookdeck-event-gateway/SKILL.md` to guide the
agent through read-first investigations and explicit approval gates.

## Telvine Packaging

If this plugin is published through Telvine:

```bash
npm i -g telvine
telvine login
telvine publish ./plugins/hookdeck-event-gateway
```

## Privacy Boundary

Do not emit prompts, source files, webhook payloads, API keys, signing secrets,
customer data, tool arguments, or model outputs as telemetry. Safe metadata can
include the plugin component name, sanitized outcome, harness name, duration
bucket, and sanitized error class.
