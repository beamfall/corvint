# 2026-10-22 lane tasks-relocate-1022w: explicit task-store relocation (V1-2437 / #1557, V1-1461)

## Problem

V1-2437 (GitHub #1557): moving a workspace made every `corvint-tasks` command refuse
`UNSUPPORTED_FILESYSTEM` because `head.primaryWorktree` named the old path, a symlink at the old
path was refused, and the only recovery was moving everything back. V1-1461: a copied store could
not be read off its original path, and a copy taken during writes was `JOURNAL_FORKED`, so V1-1459
had to qualify on a fresh store instead of a copy.

## Decisions

- New spec `docs/specs/corvint-tasks-store-relocation-v0.md`, TRL-V0-001 to TRL-V0-006, accepted
  under the owner's standing auto-accept (2026-10-11). No CAL-V0 IDs; CTW-V0-001's relocation
  refusal is unchanged for every command except the explicit `relocate`.
- Relocation is one administrative write, `corvint-tasks relocate --from OLD --request-id ID`, run
  from the new primary worktree. It is never inferred: the journal is read under `OLD` only when the
  head records exactly `OLD`, and every other command still refuses a moved store, now naming the
  command. The symlink refusal and fail-closed path resolution are untouched.
- The receipt kind RELOCATE posts one pinned `taskman-relocate/0` descriptor naming both paths. The
  journal walk now tracks the chain's primary worktree from INIT through each RELOCATE and binds it
  to the head at the head sequence, so a head re-pinned without a receipt, or moved back afterwards,
  is `JOURNAL_FORKED`. Old binaries refuse the new kind fail-closed.
- Quiescence: reservations must be empty (`QUIESCENCE_UNPROVED`); with `--dispatch-config` every
  program's dispatcher lock under `stateDir` is taken non-blocking and held across the write.
  Without the flag the dispatcher check is reported `NOT_OBSERVED`, because the store does not
  record which dispatch configs exist.
- Redo of an interrupted RELOCATE re-pins the head from the receipt's descriptor (TM-V0-009 path).
- V1-1461: the supported snapshot path is a copy taken under quiescence plus the same explicit
  relocate; a torn copy refuses `JOURNAL_FORKED` with text naming the quiesce requirement
  (TRL-V0-006). A snapshot verb that quiesces writers itself is a non-goal.
- V1-1459 reconciliation: V1-1459 is COMPLETED on a fresh store because a relocated copy was refused.
  Its criteria stay satisfied as recorded; with TRL-V0-006 a future copied-store qualification can
  bind a quiesced copy with `relocate` instead of a fresh store. No reopening is proposed.
- Operator docs: `docs/TASKS-SUPERVISION.md` "Moving or copying a task store" states that the store,
  the dispatch config and its `stateDir`/`evidence` directories must be on real paths with no
  symlink component (TRL-V0-005).

## Evidence

- Tests pass for every directly changed package: `internal/tasks/cli` and `internal/tasks/store`
  in full, run as test-name chunks to stay inside the 600 s per-command budget;
  `internal/tasks/transaction`, `snapshot`, `journal`, `archive` and `dispatch` in full; and
  `internal/tasks/service`, `importer`, `criterionbinding` and `TestImportDirection` downstream.
  The CAL-V0-131 newer-version table gained the `taskman-relocate/0` row.
- Fail-without-fix: disabling the head/chain primary binding fails both
  `TestTRLV0002_HeadRepinWithoutReceiptForks` subtests (forged and reverted heads audit CONSISTENT);
  disabling the dispatcher lock or the reservation check fails both `TestTRLV0003_*` tests. At the
  base, the `relocate` verb and descriptor codec do not exist, so the other TRLV tests cannot pass.

## NOT_RUN

- Archive export/restore into a fresh store at a new path (#1557's alternative criterion): the
  archive restore command is not implemented, so #1557 is referenced, not closed.
- Live relocation of an operator store, process kill during the write, and the other selected
  units of `corvint affected` outside `internal/tasks` (dependency paths only; `make gate` not run).
- `--dispatch-config` on Windows: the lock hold is unsupported there and refuses.

## Rollback

Revert the lane commit. A store that already committed a RELOCATE receipt is then refused
fail-closed by the reverted binary (unknown receipt kind) and must be read with a relocating
binary; stores that never relocated are unaffected.
