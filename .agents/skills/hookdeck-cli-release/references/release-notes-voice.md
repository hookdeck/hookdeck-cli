# How these release notes are written

Structure is in [release-notes-template.md](release-notes-template.md). This file is the prose:
the voice the last several releases were written in, derived from v2.4.0, v2.5.0 and v2.6.0.

Read one of those in full before drafting. They are the specification; this is the summary.

## The reader

Someone deciding whether to upgrade, and what it will cost them. Not a prospect being sold to,
and not a contributor reading a changelog. Write for a developer who has the previous version
installed and automation built against it.

## Open with a Summary that says what the release does

Two to four short paragraphs of prose. Not bullets. Lead with the capability or the theme, then
the shape of everything else.

> This release moves the CLI to API version `2026-09-01` and brings **delivery groups** to the
> command line […]
>
> Alongside that is a long list of fixes sharing one shape — the CLI reported success while doing
> something other than what you asked.

**Name the shared shape when fixes rhyme.** Several of these releases turned out to be one defect
wearing different clothes, and saying so is worth more than the list that follows. v2.5.0: "most
sharing one failure shape — the CLI would silently do something other than what you asked, and the
first symptom was missing traffic rather than an error."

## Write in second person

"you", "your project", "your scripts". Never "the user", never "users will find".

> `--local` no longer touches your global config

## Lead each entry with the fix, then explain the defect

Bold the first clause, in the reader's terms — what now happens, or what no longer happens. Then
the mechanism and why it mattered. The entry can run several sentences; these notes do not
optimise for brevity.

> - **`hookdeck listen` no longer dies at startup without a terminal.** It defaulted to a
>   full-screen UI that opens `/dev/tty`; anywhere without one it failed with
>   `could not open a new TTY` and **exited 0 having forwarded nothing** — no tunnel, no error,
>   nothing to attribute the failure to.

## State the consequence, and bold it where it is the point

The house habit is to make the damage concrete rather than describing the code.

> returned **unfiltered totals formatted as if filtered**

> events still arrived on your machine, so it looked like it worked — but they went to a throwaway
> account with no connection, delivery history, retries, or issue triggers

## Show real commands and real output

Fenced blocks with commands someone can run, and output they will actually see.

```sh
hookdeck gateway destination create --name orders \
  --delivery-group-key '$.body.tenant_id' \
  --delivery-group-rate 100 --delivery-group-rate-period minute
```

## Link issues inline, in full

At the end of the entry, in parentheses, as markdown links — not bare `#376`.

> ([#376](https://github.com/hookdeck/hookdeck-cli/issues/376))

## Do not use tables

None of the recent releases do. A table invites a matrix the reader has to decode; these notes
explain in sentences instead.

## Group with `###` when a section gets long

v2.6.0's Fixes section is grouped by command — `### listen`, `### gateway metrics`. Do it when a
flat list would run past a screen.

## Say what to check, not what did not happen

A bare negative ("nothing was removed") reads as defensive and gives the reader nothing to do.
Turn it into an instruction:

> Nothing was removed or renamed, but if you have automation that depended on the old behaviour,
> these are the ones to check.

## Attribute a finding when the story tells the reader something

Only when it carries information — usually that the failure was invisible.

> Reported by a user who concluded the websocket had failed; the tunnel was live the whole time.

## What this voice avoids

- **Marketing register.** No "we're excited", "powerful", "seamless".
- **Hedging.** "may", "should" — say what it does.
- **Labels that categorise the reader** rather than describing the change ("who it reaches", "for
  advanced users"). Say what changed and who has to act, in a sentence.
- **Restating the commit.** The reader does not care which function moved.
- **Empty sections.** Omit the heading entirely; see the template.
