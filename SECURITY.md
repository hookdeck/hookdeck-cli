# Security policy

## Reporting a vulnerability

Report it privately through GitHub:
**[Report a vulnerability](https://github.com/hookdeck/hookdeck-cli/security/advisories/new)**
(the repository's **Security** tab → **Report a vulnerability**).

Do not open a public issue or pull request for it. A public report describes the
exposure to everyone before a release carries the fix.

Include what you ran, what it exposed, and the CLI version (`hookdeck version`).

## What counts

Anything that could expose a credential, project data or access to someone who
should not have it. For this CLI that includes, for example:

- a config or credential file written with permissions other local users can read
- a key, token or project listing printed to a log, CI output or error message
- a command or MCP tool acting on a project or organisation other than the one
  selected

When unsure, report it privately; it can always be made public afterwards.

## Supported versions

Fixes are released on the latest major version. Update with your package manager
(`brew upgrade hookdeck`, `npm install -g hookdeck-cli@latest`, `scoop update hookdeck`).
