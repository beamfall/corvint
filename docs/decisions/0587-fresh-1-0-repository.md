# Decision 0587 — Start Corvint 1.0 in a fresh repository

Date: 2026-10-29. Status: accepted by the owner. Authority: owner decision in chat, 2026-10-29
(fresh 1.0 repository plan).

## Context

The Corvint repository carried every build-log entry, sealed CEM, closed ticket and dated
benchmark result written since the project started. A new reader of the 1.0 tree had to step
around about 4,800 history-only files to find the product, and the closed `docs/BUILD-LOG.md`
alone ran to 7,702 lines. Decision 0586 relicensed the whole tree under Apache-2.0 the same day.

## Decision

1. The existing repository is archived, read-only, as `beamfall/corvint-history`. Its final
   commit is `1c4d8793cf71cbb01b8509b68a6cb1502a40a13e` ("Merge pull request #2031"). The archive
   keeps the complete history and is not rewritten.
2. Corvint 1.0 starts as a single root commit in a fresh `beamfall/corvint` repository. The tree
   is the archive's final tree minus history-only material:
   - `docs/build-log/` entries and the closed `docs/BUILD-LOG.md`; `docs/build-log/README.md`
     remains and names the archive;
   - sealed CEMs under `.corvint/changes/`, except the CEM the use-case receipts pin and the CEMs
     the affected-plan replay tests name;
   - task-store tickets whose status is not OPEN or HELD, except the completed tickets that a kept
     ticket's dependency closure names, so no kept ticket gains a missing dependency;
   - dated one-off result files that no test, Makefile target, gate script, embedded admission,
     spec acceptance evidence or pinned receipt reads.
3. A citation of an old commit SHA, a closed ticket, a pruned CEM, a build-log entry or a pruned
   dated result resolves in the archive at that commit. Live documents cite pruned paths as
   `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/<path>`.
4. New build-log entries keep the per-file convention of decision 0423,
   `docs/build-log/YYYY-MM-DD-<slug>.md`, in the 1.0 repository.
5. The Go module path stays `github.com/Beamfall/corvint`, so imports, `go install` lines and
   pinned build metadata do not change.
6. The license is Apache-2.0 across the whole repository (decision 0586). The contributor
   agreement remains an optional additional grant; no contribution requires it.
7. `VERSION` stays `1.0.0-rc.3`, and the fresh repository first ships `v1.0.0-rc.3` as a prerelease (owner, 2026-10-29). `v1.0.0` stable still needs the stable requirements PRS-V1-013..018, the CEM stable gate and V1-0019..V1-0021.

## Non-goals

This decision publishes no release, tag or package, and changes no product behavior, wire format,
requirement or gate. It does not move the separate `corvint-taskman` repository.

## Failure modes

- A test or tool that reads a commit older than the 1.0 root through Git history fails in a fresh
  clone of the new repository. Such a reader must use the archive or a committed fixture.
- A citation into a pruned path that this change did not rewrite stays dangling in the new tree;
  it still resolves in the archive at the commit above.

## Rollback

The archive remains the complete history. Rolling back means continuing development from
`beamfall/corvint-history` at `1c4d8793cf71cbb01b8509b68a6cb1502a40a13e`; nothing in the archive
is deleted or rewritten by this decision.
