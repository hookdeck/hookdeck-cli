# MCP surface — manual QA checklist

Read [../SKILL.md](../SKILL.md) first, including the safety model.

Two servers: `hookdeck gateway mcp` and `hookdeck outpost mcp`. Drive them with
[../scripts/mcp-call.py](../scripts/mcp-call.py).

```bash
QA=.agents/skills/hookdeck-cli-manual-qa/scripts/mcp-call.py
python3 $QA --config "$HD_CONFIG" --server gateway --list
python3 $QA --config "$HD_CONFIG" --server gateway --allow-write --list
```

The MCP surface is where defects hide, because no human reads its output. A
wrong answer here is consumed directly by an agent that will act on it.

## Mode gating

- [ ] **Read-only is the default.** With no flag, `tools/list` advertises no
      write actions and the descriptions do not mention them.
- [ ] **`--allow-write` adds them**, and `HOOKDECK_MCP_ALLOW_WRITE` does the
      same. `--read-only` wins when both are given.
- [ ] **The guard is real, not cosmetic.** Call a write action without
      `--allow-write` and confirm it is refused — a tool hidden from the listing
      but still dispatchable is not gated.
- [ ] **The refusal explains how to enable it**, naming the help tool.
- [ ] **`*_help` reports the current mode** and matches the actual listing.
- [ ] **Actions that change state but stay available read-only** (connection
      `pause`/`unpause`) still work without the flag. That is deliberate —
      incident response — and its tests are the proof it was not an oversight.
- [ ] **Annotations are honest.** `readOnlyHint` false for anything that
      changes state; `destructiveHint` true for deletes, cancel, mute, dismiss.

## Per tool

Call every action of every tool, in both modes. For each:

- [ ] **Unknown arguments are rejected**, and the message distinguishes "no such
      argument" from "that argument needs write mode".
- [ ] **A failure is reported as one.** Check `isError`, not just that a
      response came back. An envelope whose body says the operation was declined
      while the result reads as success is the defect to hunt.
- [ ] **No hardcoded status fields.** Anything reporting `"status": "queued"`,
      `"deleted"`, or similar must reflect what the API returned, not what the
      handler intended.
- [ ] **Write actions are equivalent to their CLI siblings.** Create a resource
      both ways with the same inputs and diff the stored result. Schema defaults,
      required-field fallbacks and filter handling have diverged between the two
      before — a fix applied to one surface and not the other.
- [ ] **Credential masking** matches the CLI: platform-generated secrets
      returned, caller-supplied credentials masked.

## Payloads

- [ ] **Raw body tools** (`*_events raw_body`, `*_requests raw_body`) return the
      body unmodified, including non-JSON and large payloads.
- [ ] **List filters narrow the result.** Same test as the CLI: compare against
      unfiltered. An ignored filter is worse here — the agent cannot tell.
- [ ] **Pagination** is expressible and terminates.

## Cleanup

Anything created through MCP is as real as anything created through the CLI.
Delete it, and confirm the delete with an independent `get`.
