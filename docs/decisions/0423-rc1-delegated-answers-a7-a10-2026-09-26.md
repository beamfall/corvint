# Decision 0423: delegated answers A7 to A10 for 1.0.0-rc.1

Date: 2026-09-26. Status: accepted. Authority: the repository owner delegated the choice on
2026-09-26 ("you can answer those questions for me").
Tickets: V1-0018, V1-0318, V1-0358.

## Context

Decision 0422 recorded the owner's answers to A1 to A6 and left A7 to A10 open. The owner then
delegated those four answers to the agent. The agent chose the recommendation for each, with the
changes recorded below.

Every open pull request that appended to `docs/BUILD-LOG.md` conflicted after each merge (V1-0358).
The 2026-09-26 audit output sat uncommitted in the primary checkout and blocked the v0-6 promotion
script, which requires a tree that is clean outside `.taskman/`.

## Decision

1. **A7, build-log layout (V1-0358).**
   - `docs/BUILD-LOG.md` stays at its path and is closed to new entries. None of its lines move, so
     every existing citation stays valid. A citation into it still needs an `@<hash>` anchor.
   - Each new entry is its own file, `docs/build-log/YYYY-MM-DD-<slug>.md`. Its first line is the
     heading `## YYYY-MM-DD <IDs>: <title>`. The slug is required, so no new file reuses a per-day
     name `docs/build-log/YYYY-MM-DD.md` from the unpublished historical records (`docs/README.md`).
   - An entry file is not edited after it merges. A correction is a new entry that names the entry
     it corrects, so a bare line citation into an entry file stays valid.
   - There is no committed index, because an index file would conflict the same way the log did.
     The index is `rg -n '^## ' docs/BUILD-LOG.md docs/build-log/`.
2. **A8, the 2026-09-26 audit output.**
   - The audit report and its raw review artifacts are not published. The report carries absolute
     local paths and links into the raw directory. Both move to the owner's private release
     evidence, outside the repository.
   - The audit's build-log entry is published as `docs/build-log/2026-09-26-comprehensive-audit.md`.
     It names the tickets the audit created instead of linking the report.
   - The uncommitted `docs/BUILD-LOG.md` edit in the primary checkout is dropped after it is kept as
     a patch beside the report. The owner runs the move, which clears the tree for the v0-6
     promotion.
3. **A9, GitHub issues.**
   - beamfall/corvint#175 received a status comment on 2026-09-26 and stays open.
   - beamfall/corvint#170 is closed with a comment when `1.0.0-rc.1` is published. It was fixed by
     beamfall/corvint#172 (the parent-directory probe) and beamfall/corvint#178 (the MCP reason
     class, decision 0383).
4. **A10, the `corvint-tasks` repository (V1-0318).** After `1.0.0-rc.1` is published,
   `beamfall/corvint-tasks` is frozen. Its README points to the in-tree source, and it takes no
   further changes. Until then it stays as it is.

## Non-goals

- This decision completes no ticket, changes no dependency and promotes nothing. Those store
  mutations stay owner-run.
- It rewrites no existing `docs/BUILD-LOG.md` entry and moves none into `docs/build-log/`.
- It adds no check that refuses a new `docs/BUILD-LOG.md` entry.
- It does not change the frozen `benchmarks/daily-loop-v0` harness. That harness excludes only
  `docs/BUILD-LOG.md` from a task's changed paths, and its digest is preregistered. A later corpus
  revision must also exclude `docs/build-log/`.

## Rollback

Revert this change. `docs/BUILD-LOG.md` becomes the append-only log again, and each entry under
`docs/build-log/` can be appended to it. A8 is undone by moving the report and raw directory back
and applying the kept patch. A9 is undone by reopening beamfall/corvint#170, and A10 by removing
the README pointer in `beamfall/corvint-tasks`.
