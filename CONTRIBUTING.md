# Contributing to the Hookdeck CLI

## Reporting a bug or asking for a feature

Open an issue. That is what the issue tracker is for, and a report with the exact
command, its output and what you expected instead is the fastest one to act on.

If what you found could expose credentials, data or another user's access, do not
open an issue: see [SECURITY.md](SECURITY.md).

## Opening a pull request

Pull requests are welcome. [AGENTS.md](AGENTS.md) describes the project layout,
CLI conventions and how to run the tests; it is written for agents but reads as
a contributor guide.

## Maintainers: where a finding goes

The issue tracker is public, and it is the first thing someone evaluating the CLI
reads. An issue opened and closed within the hour tells a watcher nothing the pull
request did not, and forty open bugs filed by the maintainers in a week read as
instability rather than diligence. So a finding goes to the narrowest place that
does its job:

| The finding is | It goes in |
|---|---|
| Being fixed now | The pull request description. No issue. |
| One of several from a single QA or review pass | One branch, one commit per finding, one pull request. Anything deferred goes in a **single** tracking issue with a checklist, not one issue each. |
| A user-visible bug that is not being fixed yet | A public issue: a user who hits it will search for it. |
| Internal only: CI, test flakiness, agent tooling | The team's internal tracker, not this repository. |
| Security-relevant: credentials, file permissions, secrets in logs, access across projects or organisations | A private security advisory ([SECURITY.md](SECURITY.md)). **Never** a public issue or a pull request title that describes the exposure before a release carries the fix. |

Before opening an issue, search for an existing one; a duplicate costs every
watcher two notifications.

Agents follow the same table, and do not open issues at all unless asked to.
