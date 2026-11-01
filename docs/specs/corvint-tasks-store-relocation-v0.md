# Corvint Tasks store relocation V0

Owner: Russell Lewis
Date: 2026-10-22
Intent status: accepted (owner standing auto-accept 2026-10-11)
Delivery status: experimental
Authoritative inputs: tickets V1-2437 (GitHub #1557) and V1-1461, the V1-1459 completion, the
Corvint Tasks contract TCP-00 (`TM-V0-007`, `TM-V0-009`), `docs/specs/corvint-tasks-intent-worktree-v0.md`
(`CTW-V0-001` keeps the primary identity and the relocation refusal), and the in-tree sources under
`internal/tasks/snapshot`, `internal/tasks/transaction`, `internal/tasks/journal`,
`internal/tasks/store`, `internal/tasks/dispatch` and `internal/tasks/cli`.

## Agent digest
- Claim: A moved or copied task store is bound to a new primary worktree only by an explicit `relocate` that commits one receipt naming both paths.
- Status: accepted (owner standing auto-accept 2026-10-11); experimental. TRL-V0-001 through TRL-V0-006 are implemented and tested in tree; live qualification on an operator store is NOT_RUN.
- Exists: the `relocate` verb, the `taskman-relocate/0` pinned descriptor, chain primary tracking in the journal walk, relocate redo, the optional dispatcher-lock hold, the consistent-snapshot refusal text, and `archive restore` into an absent state dir.
- Blocked on: live qualification on an operator store.
- Read next: Requirements; Failure modes; Traceability.

## User and boundary

`head.json` and the INIT descriptor record the primary worktree. Moving the checkout, or copying
`.git/taskman` into another clone, made every command refuse `UNSUPPORTED_FILESYSTEM` (reads) or
`UNSUPPORTED` (writes) with "relocation unsupported", and the only recovery was to move everything
back or hand-edit `head.json`. This spec adds one administrative write that re-pins the head
explicitly. Relocation is never inferred from the current path: every other command keeps
refusing a store whose head names another primary worktree, and the refusal now names the command
to run.

## Requirements

- `TRL-V0-001`: `corvint-tasks relocate --from OLD --request-id ID [--role OWNER|OPERATOR]
  [--dispatch-config FILE]` runs from the new primary worktree. `--from` and `--request-id` are
  required, each flag appears at most once, and only OWNER or OPERATOR may relocate. The command
  reads and guards the journal under `OLD` only when `head.primaryWorktree` equals `OLD` exactly;
  any other recorded value, or `OLD` equal to the resolved primary worktree, refuses and writes
  nothing. The read and write refusals for a moved store name this command and the recorded path.
- `TRL-V0-002`: A completed relocation commits one RELOCATE receipt that posts exactly one pinned
  `taskman-relocate/0` descriptor `{queueId, requestId, fromPrimaryWorktree, toPrimaryWorktree}`
  and sets `head.primaryWorktree` to the destination. The journal walk tracks the chain's primary
  worktree from the INIT descriptor through each RELOCATE descriptor; a RELOCATE receipt whose
  descriptor is missing, duplicated, names another queue or request, or does not continue the
  recorded primary, and a head whose primary differs from the chain's at the head sequence, refuse
  `JOURNAL_FORKED`. A head re-pinned without a receipt is therefore detected by `receipt audit`.
  RELOCATE is a new receipt kind, so a binary without it refuses the store fail-closed.
- `TRL-V0-003`: Relocation refuses `QUIESCENCE_UNPROVED`, writing nothing, while any attempt holds
  a reservation. With `--dispatch-config`, it takes, without waiting, the config's `stateDir` start
  gate exclusive (an advisory lock on the directory, creating it when absent) and then every
  program's single-dispatcher lock, and holds them across the write. A dispatcher start holds the
  same gate shared while it creates its program directory and takes its lock, so a start during a
  relocation is refused and a start the scan cannot yet see keeps the relocation out. A held gate or
  lock, or a symbolic link where a program directory or its lock belongs, refuses
  `QUIESCENCE_UNPROVED`. Without the flag the result warns that the dispatcher check was
  `NOT_OBSERVED`. The ordinary writer guards still apply, including the barrier and the intent
  branch check.
- `TRL-V0-004`: A relocation interrupted after its receipt is linked is settled by `TM-V0-009` redo,
  which re-pins the head from the receipt's descriptor; the identical request then replays. A new
  request after a completed relocation refuses, since the head no longer records `OLD`.
- `TRL-V0-005`: The store, the dispatch config and its `stateDir` (including each worker's
  `evidence` directory) must be on real paths with no symlink component; relocation refuses a
  `stateDir` that does not resolve to itself. The symlink refusal and
  fail-closed path resolution are unchanged; a symlink at the old path is not a relocation.
- `TRL-V0-006`: A copied store is bound to a new clone by the same explicit relocation. A copy is
  usable only as a consistent snapshot, taken while no `corvint-tasks` writer, dispatcher or live
  attempt holds the store; a torn copy refuses `JOURNAL_FORKED` and the refusal names that
  quiesce requirement. Relocating a copy forks local history under the same queue id; keeping only
  one copy writable is the operator's responsibility.
- `TRL-V0-870`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3554) `corvint-tasks archive restore FILE` restores a `taskman-archive/0` stream (`TM-V0-022`) only after
  `archive verify` accepts the whole file, and refuses with that verify code (`MALFORMED`,
  `JOURNAL_FORKED`, `LIMIT_EXCEEDED` or `UNSUPPORTED_VERSION`) a tampered or truncated stream. It
  refuses `UNSUPPORTED` when the state dir exists or the archived head binds journal segments, and
  `INTENT_DIVERGED` when the intent root's `queue.json` names another queue, or an intent file
  present at the intent root differs from the archive's bytes or is not in the archive. Every
  refusal is decided before the writer lock is taken and writes nothing: no state dir, no intent
  file and no lock file.
- `TRL-V0-871`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3554) A completed restore writes every archived state file byte-identically at its path, the genesis
  directories, and every archived intent file absent from the intent root, under the writer lock.
  The intent files are written first and the state dir is published by one rename of a synced
  staging directory beside it, so an interruption leaves no state dir (an identical rerun
  completes) or the whole store; a failed write removes the intent files it created. A re-export
  of the restored store is byte-identical to the archive. An archive whose head names another
  primary worktree is restored with a warning naming `relocate --from <recorded>`, which binds it
  (`TRL-V0-001`, `TRL-V0-006`).

## Non-goals and simpler baseline

The simpler baseline is the prior contract: relocation unsupported, move the checkout back. It was
rejected because #1557 and V1-1461 show operators with no supported path. Not part of V0: inferring
a relocation from the current path, accepting symlinked store or config paths, a store-copy or
snapshot verb that quiesces writers itself, restore over an existing state dir, merging an archive
into a live store, and restoring a segmented journal.

## Failure modes

| Situation | Behavior |
| --- | --- |
| Checkout moved, any ordinary command | `UNSUPPORTED_FILESYSTEM` or `UNSUPPORTED` naming `corvint-tasks relocate --from <recorded>` (`TRL-V0-001`). |
| `--from` names neither the recorded nor the resolved primary | Refused; store unchanged (`TRL-V0-001`). |
| A live attempt holds a reservation | `QUIESCENCE_UNPROVED`; store unchanged (`TRL-V0-003`). |
| A dispatcher holds its program lock (with `--dispatch-config`) | `QUIESCENCE_UNPROVED`; store unchanged (`TRL-V0-003`). |
| A dispatcher starts during a relocation, or is starting when it begins (with `--dispatch-config`) | The start is refused, or the relocation refuses `QUIESCENCE_UNPROVED`; never both run (`TRL-V0-003`). |
| A program directory or `stateDir` component is a symbolic link | `QUIESCENCE_UNPROVED`; store unchanged (`TRL-V0-003`, `TRL-V0-005`). |
| No `--dispatch-config` | Relocation proceeds with a `NOT_OBSERVED` warning (`TRL-V0-003`). |
| Interrupted after the receipt link | Retry redoes the head re-pin, then replays (`TRL-V0-004`). |
| `head.json` edited to a new path without a receipt, or moved back after a relocation | `JOURNAL_FORKED` on audit and reads (`TRL-V0-002`). |
| Copy taken during writes | `JOURNAL_FORKED` naming the consistent-snapshot requirement (`TRL-V0-006`). |
| Symlink at the old path | Unchanged symlink refusal (`TRL-V0-005`). |
| `archive restore` of a tampered or truncated stream | The verify code; nothing written (`TRL-V0-870`). |
| `archive restore` with a state dir present, or of a segmented head | `UNSUPPORTED`; nothing written (`TRL-V0-870`). |
| `archive restore` whose intent differs from the intent root | `INTENT_DIVERGED`; nothing written (`TRL-V0-870`). |
| `archive restore` interrupted | No state dir and a rerun completes, or the whole store (`TRL-V0-871`). |
| Windows | The dispatcher lock hold is unsupported and `--dispatch-config` refuses; the reservation check still applies (`TRL-V0-003`). |

## Acceptance evidence

Focused tests in `internal/tasks/cli`, `internal/tasks/snapshot`, `internal/tasks/transaction`,
`internal/tasks/journal` and `internal/tasks/store`. The archive round trip runs on fixture stores
(`TRL-V0-870`, `TRL-V0-871`). Live relocation of an operator store and process kill during the
write are NOT_RUN.

## Rollback

Revert the commit. A store that committed a RELOCATE receipt is then refused fail-closed by the
reverted binary (unknown receipt kind) and needs the relocating binary, or the checkout moved back
before relocation, to be read; stores that never relocated are unaffected.

## Traceability

| Requirement | Evidence |
| --- | --- |
| `TRL-V0-001` | `TestTRLV0001_RelocateMovedRepository` |
| `TRL-V0-002` | `TestTRLV0001_RelocateMovedRepository`, `TestTRLV0002_HeadRepinWithoutReceiptForks`, `TestTRLV0002_RelocateDescriptorCodec` |
| `TRL-V0-003` | `TestTRLV0003_RelocateRefusesLiveAttempt`, `TestTRLV0003_RelocateRefusesHeldDispatcher`, `TestTRLV0003_HoldIdleRefusesSymlinkedProgramDir`, `TestTRLV0003_DispatcherStartRefusedDuringRelocation`, `TestTRLV0003_HoldIdleRefusesStartingDispatcher` |
| `TRL-V0-004` | `TestTRLV0004_RelocateRedoRepinsHead`, `TestTRLV0001_RelocateMovedRepository` |
| `TRL-V0-005` | `TestTRLV0005_HoldIdleRefusesSymlinkedStateDir` for the dispatch `stateDir`; otherwise a documentation requirement (`docs/TASKS-SUPERVISION.md`). |
| `TRL-V0-006` | `TestTRLV0006_RelocateCopiedStore` |
| `TRL-V0-870` | `TestTRLV0870_RestoreRefusalsWriteNothing` |
| `TRL-V0-871` | `TestTRLV0871_RestoreRoundTripsByteIdentically`, `TestTRLV0871_RestoreAtANewPathThenRelocate` |

## Unresolved decisions

- Restore over an existing or partially written state dir is refused rather than merged; the
  operator removes it first. A crashed restore leaves at most a `.taskman-restore-*` staging
  directory beside the state dir, which the next restore does not reuse.
