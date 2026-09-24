# How these release notes are written

Structure is in [release-notes-template.md](release-notes-template.md). This file is the prose.

Read [v2.6.0](https://github.com/hookdeck/hookdeck-cli/releases/tag/v2.6.0) in full before
drafting. It is the specification; this is the summary.

## Length is the rule people get wrong

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

**Link out for the rest.** The reader who wants the mechanism follows the issue or PR. That is what
the links are for, and it is why an entry does not need to carry the full story.

Re-measure before publishing:

```sh
gh release view v2.6.0 --json body -q .body | wc -w    # ~1438
wc -w <your-draft.md>
```

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

## Cutting to length: what is safe to remove

Length is a real constraint, but three separate errors in the v3.0.0 draft came from a compression
pass, not from the first draft: the detail that three tools keep the `hookdeck_` prefix (which made
the rename claim false), the name of the `--allow-write` flag on the Outpost server, and the worked
example under "and the next command disagreed", which left the phrase meaningless.

**Cut padding. Never cut a qualification, or the example a claim rests on.** If an entry cannot
reach the word target without losing one of those, it is a long entry — take the words. Length is
the default, accuracy is the constraint.

## What this voice avoids

- **Verbosity.** See the table. This is the failure mode.
- **Marketing register.** No "we're excited", "powerful", "seamless".
- **Hedging.** "may", "should" — say what it does.
- **Labels that categorise the reader** ("who it reaches", "for advanced users").
- **Bare negatives.** "Nothing was removed" gives the reader nothing to do. v2.5.0 turns the same
  thought into: "Nothing was removed or renamed, but if you have automation that depended on the
  old behaviour, these are the ones to check."
- **Tables.** No recent release uses one.
- **Restating the commit.** The reader does not care which function moved.
