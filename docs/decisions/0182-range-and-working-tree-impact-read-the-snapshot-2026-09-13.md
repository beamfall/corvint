# Decision 0182 — the range and working-tree `impact` profiles read the snapshot

Date: 2026-09-13. Status: accepted, delegated coordinator call. Authority: repository owner
delegation to make owner calls and record them (orchestration thread, 2026-09-12).

`docs/specs/index-snapshot-v0.md` proposes `IDX-SNAP-V0-021` in the same change: `corvint impact
--base` (both range profiles) and `corvint impact --working-tree-untracked` read the committed
tree's snapshot on the terms `IDX-SNAP-V0-008` gives the query verbs, and build on any miss exactly
as before. It closes the part of the AT-04 work decisions 0177 and 0180 left: those two profiles
were the last `impact` paths that built unconditionally.

The call: accept `IDX-SNAP-V0-021`. Both profiles read `Source.Data`, which a deferred pack body
leaves nil, so they take only the whole-file read, as `prove --base` does under `IDX-SNAP-V0-020`.
A hit rehydrates `DirtyPaths`, `StatusSHA256` and the live `HEAD` from Git, so the range profiles'
clean-worktree check and the working-tree profile's closing comparison against a fresh build judge
the values a build would; the working-tree profile keeps that closing build, and on a dirty
worktree its answer is unchanged. A stale, absent, or corrupt snapshot is a miss that calls
`contextindex.Build` unchanged. `affected` and `prove --checkpoint` (`FPK-V0-024`) are untouched.
No candidate was rejected.

Evidence: `TestImpactRangeAndWorkingTreeProfilesReadTheSnapshotWithoutChangingAByte`
(`cmd/corvint/index_snapshot_test.go`) compares stdout, stderr and exit across an absent, a stale
and a current snapshot for the default and expanded range profiles on a clean worktree and for the
working-tree profile with an untracked Go file; it passes. On this repository the pre-change and
changed binaries gave byte-identical output over absent, stale, current gob and current pack
snapshots for all three invocations, with the medians recorded in the clause (`impact --base` 1.84
to 0.98 s, `--working-tree-untracked` 2.00 to 1.13 s at load 20-26).

Consequences: `docs/specs/index-snapshot-v0.md` adds `IDX-SNAP-V0-021`, rewords the
`IDX-SNAP-V0-019` exclusion, and cross-references the clause in the non-goals, evidence and
traceability; `docs/specs/REQUIREMENTS.tsv` is regenerated.

Rollback: restore the unconditional build for `impactWorktree` and `impactBaseSet` in the `impact`
dispatch in `cmd/corvint/main.go`; no stored state changes, so no migration.
