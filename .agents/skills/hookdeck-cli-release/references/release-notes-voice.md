# How these release notes are written

Structure is in [release-notes-template.md](release-notes-template.md). This file is the prose.

Read [v2.6.0](https://github.com/hookdeck/hookdeck-cli/releases/tag/v2.6.0) in full before
drafting. It is the specification; this is the summary.

Draft wherever you like — `plans/*-release-notes.md` is gitignored for this — but **do not commit
the notes.** The GitHub Release is the record. A copy in the repo drifts the moment anyone edits
the Release, and `CHANGELOG.md` already points at Releases for exactly this reason.

## Length

Measured across the last three GA releases:

| release | bullets | median words per bullet | whole note |
|---|---|---|---|
| v2.4.0 | 4 | 40 | 445 words |
| v2.5.0 | 9 | 51 | 902 words |
| v2.6.0 | 36 | 31 | 1,438 words |

**Targets: a median of 30–50 words per bullet, and a whole note under about 1,500 words.** v2.6.0
covered thirty-six fixes in 1,438 words. If a draft runs to three thousand, it is an article, not a
release note — cut it, do not justify it.

A bullet is **two to three sentences**: what changed, then the consequence that made it matter. The
longest bullet in any recent release is about 120 words, and that is the ceiling, not the aim.

**Link out for the rest.** The reader who wants the mechanism follows the issue or PR.

Re-measure before publishing:

```sh
gh release view v2.6.0 --json body -q .body | wc -w    # ~1438
wc -w <your-draft.md>
```

## Spelling

British in prose, literal for identifiers — `--color` stays `--color` while the colour it prints is
spelled the British way. The rule and the reasoning are in **AGENTS.md § 6**; recent notes are
inconsistent because it was not written down until now.

## Structure

`## Summary`, then only the sections with content: `New features`, `Fixes`,
`Improvements / behavior changes`, `Internal / reliability / infrastructure`. Omit empty headings.

Group a long `Fixes` section with `###` by command or area, as v2.6.0 does.

Always end with the compare link:

`**Full Changelog**: https://github.com/hookdeck/hookdeck-cli/compare/<prev>...<new>`

## Summary

Two to four short paragraphs. Lead with the capability or the theme, then the shape of the rest.
**Name the shared shape when fixes rhyme** — often one defect wearing different clothes, and saying
so is worth more than the list:

> Alongside that is a long list of fixes sharing one shape — the CLI reported success while doing
> something other than what you asked.

## Entries

**Second person.** "your project", "your scripts" — never "the user".

**Lead with the fix, bolded, in the reader's terms.** Then the consequence. Then the link.

> - **`hookdeck ci --local` and `hookdeck login --local` no longer rewrite your global config.**
>   `--local` added a second write rather than redirecting the first, so it silently switched the
>   active project for every other `hookdeck` command on the machine — the opposite of what the flag
>   is for. ([#332](https://github.com/hookdeck/hookdeck-cli/issues/332))

That entry is 52 words and explains a subtle bug completely. Match that density.

**Make the damage concrete, and bold it where it is the point.**

> returned **unfiltered totals formatted as if filtered**

**Show a command only when it earns the space** — a new flag people will copy, or output that makes
a failure obvious. Not one per entry.

**Link issues and PRs in full markdown**, in parentheses, at the end:
`([#376](https://github.com/hookdeck/hookdeck-cli/issues/376))`. Bare `#376` does not link.

## Scope: GA to GA

Notes compare the last GA to this one. **A defect in surface that did not exist in the previous GA
is not a fix** — it is part of shipping the feature, and belongs in New features or nowhere.

Betas do not count as shipped. Check the previous GA *tag*, not the commit log: a long-lived branch
means reachability does not imply the change is new. Build the tag if that is what it takes.

**Name a fixed thing by the name the reader has, not the one it is getting.** In a release that
renames things, a fix entry describes a defect in the *old* surface, so the old name is what a
reader recognises — give the new name after it:

> **`hookdeck_connections` accepted a `disabled` filter it could not honour** — now
> `gateway_connections_read`. …

Getting this backwards sends someone looking for a tool they have never had, in a bug report about
software they have been running for weeks.

## Cutting to length

**Cut padding. Never cut a qualification, or the example a claim rests on.** Where the word target
and accuracy conflict, accuracy wins and the entry is long.

Compressing the v3.0.0 draft broke three things, each a qualification that looked like padding: the
three tools that keep the `hookdeck_` prefix (losing it made the rename claim false), the name of
the `--allow-write` flag, and the example under "and the next command disagreed", without which the
phrase means nothing.

## Verify every claim against the built binary, not the commit log

The v3.0.0 notes said `gateway_metrics_read` now accepted `"measures": "count"`.
It did not. The fix was real (`7dae336`), and a later merge reverted it
(`b0b5710`) — so the commit log said "fixed" while the shipped artifact said
`measures is required`. The claim was published, and a user hit it the first
time they used the release.

**A commit is evidence that a fix was written. Only the binary is evidence that
it shipped.** Before a bullet goes in, exercise the behaviour on a build of the
tag:

```sh
go build -o /tmp/hd-check . && /tmp/hd-check <the command the bullet describes>
```

For MCP behaviour, drive the built server over stdio and call the tool. It takes
a minute per bullet and it is the difference between a release note and a guess.

This matters most for a **long-lived release branch that has taken merges from
`main`** — exactly the shape where a fix gets quietly undone. `git log -S` will
not show you the revert; it skips merges. Use `git log --full-history -m -S`.

A note that claims a fix the release does not contain is worse than omitting it.
The reader stops looking for a workaround.

## What this voice avoids

- **Verbosity.** See the table. This is the failure mode.
- **Marketing register.** No "we're excited", "powerful", "seamless".
- **Hedging.** "may", "should" — say what it does.
- **Labels that categorise the reader** ("who it reaches", "for advanced users").
- **Bare negatives.** "Nothing was removed" gives the reader nothing to do. v2.5.0 turns the same
  thought into: "Nothing was removed or renamed, but if you have automation that depended on the
  old behaviour, these are the ones to check."
- **Tables in a release note.** No recent release uses one. (This file is not a release note.)
- **Restating the commit.** The reader does not care which function moved.
