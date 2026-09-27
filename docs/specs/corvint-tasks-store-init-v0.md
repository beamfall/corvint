# Corvint Tasks store initialization V0

Owner: Russell Lewis
Date: 2026-09-25 (CTS-V0-001, CTS-V0-003 and CTS-V0-004 accepted 2026-09-27)
Intent status: accepted for CTS-V0-001, CTS-V0-003 and CTS-V0-004 (owner decisions 2026-09-27); CTS-V0-002 proposed
Delivery status: experimental
Authoritative inputs: decision 0397 (corvint-tasks built in tree), `AGENTS.md`,
`docs/SPEC-DRIVEN-DEVELOPMENT.md`, tickets V1-0323, V1-0310 and V1-0331, and the in-tree sources under
`internal/tasks/store`, `internal/tasks/journal`, `internal/tasks/intent` and `internal/tasks/cli`.

## Agent digest
- Claim: `corvint-tasks init` refuses an intent store that already holds records, and works in a repository reached through a symlinked ancestor such as macOS `/tmp`.
- Status: accepted for CTS-V0-001, CTS-V0-003 and CTS-V0-004 (owner decisions 2026-09-27); CTS-V0-002 proposed; experimental. CTS-V0-001 and CTS-V0-004 are implemented, CTS-V0-003 (shadow import) is accepted and not yet implemented, CTS-V0-002 is a proposal only.
- Exists: the coded init refusal, the ancestor resolution, and their store and CLI tests.
- Blocked on: CTS-V0-003 implementation and tests; owner acceptance and the recovered task-store contract (V1-0310) for CTS-V0-002.
- Read next: Requirements; Failure modes; Traceability.

## User and boundary

The corvint-tasks journal lives in `.git/taskman` and is never committed; the intent store
`.taskman/` is committed. A fresh clone therefore has committed tickets and no journal. Genesis binds
only the queue and policy, so every ticket or release record already present has no journal
afterimage, and every later read refuses the whole store as `INTENT_DIVERGED` (V1-0323). The
operator's only recovery was to delete the new journal by hand.

This spec owns store initialization and the first import path until the task-store contract is
recovered (V1-0310); it does not restate or replace that contract, and it adds no wire code.

On 2026-09-27 the owner accepted CTS-V0-003 as a shadow import of a foreign roadmap export, detached
from V1-0310, because it adds no wire code. The decision was relayed to this build by the orchestrator
session "Work progress orchestration". The owner accepted CTS-V0-001 and CTS-V0-004 the same day;
CTS-V0-002 remains a proposal and still waits on owner acceptance and V1-0310.

Non-goals: adopting committed native records into a new genesis, a journal-optional read mode, a new
wire code, any change to genesis bytes, journal format, projection checks or the closed code set, any
automatic repair of an existing frozen store, and, for CTS-V0-003, the cutover verb, admission,
attempts and drains, the import-map writer, and writing or changing the foreign export.

## Requirements

- `CTS-V0-001`: `corvint-tasks init` MUST refuse, before it creates the state directory or any
  journal byte, when the intent store's `tickets/` or `releases/` directory holds any entry. The
  refusal MUST carry outcome `BLOCKED` in the store report, `REFUSED` at the CLI, the existing code
  `INTENT_DIVERGED`, and a reason naming the first record found. An absent or empty directory MUST
  NOT refuse. An unreadable directory MUST refuse as `UNSUPPORTED_FILESYSTEM`.
- `CTS-V0-002`: (proposed, not implemented) A read verb (`queue status`, `roadmap`, `ticket show`,
  `ticket search`) run where the journal is absent SHOULD answer from the committed intent store
  alone, labelled journal-absent and unaudited, instead of failing. It MUST NOT create the journal,
  MUST NOT report receipts or audit identities it cannot bind, and every mutation MUST still require
  an initialized journal.
- `CTS-V0-003`: (accepted 2026-09-27, owner decision relayed by the orchestrator session "Work
  progress orchestration"; not yet implemented) An explicit `corvint-tasks import` verb MUST read a
  JSON export of foreign tickets into an initialized store and write one `taskman-ticket/0` record
  per item. Each record MUST carry `source.kind` `IMPORT`, `sourceItemId` equal to the foreign
  primary ID, the export's `sourceQueueId`, `sourceRevisionSha256` equal to the SHA-256 of the
  item's verbatim foreign block, and `shadowOverlay` true. The verb MUST validate the whole export
  and every record it would write before its first write, and MUST refuse with nothing written on
  an invalid or duplicated item. Re-import MUST be idempotent on `sourceItemId` and
  `sourceRevisionSha256`: an unchanged item writes nothing, a changed item writes the next revision
  of the same ticket, and no record is ever deleted, including one absent from a later export. The
  verb MUST refuse, with nothing written, when a target `ticketId` is held by a `NATIVE` record or by
  an `IMPORT` record with a different `sourceItemId`, and when the queue's `canonicalWriter` is
  `NATIVE`; it MUST NOT refuse because `IMPORT` records already exist. Every imported record
  therefore reads blocker `CUTOVER_MISSING` and is never eligible until a separate cutover.
- `CTS-V0-004`: Store resolution MUST resolve symbolic links in the directories above the primary
  worktree, such as the macOS `/tmp` and `/var` links into `/private`, and MUST record that resolved
  primary-worktree path, so `init` and every later verb succeed from either spelling and name one
  primary worktree. A symbolic link at the primary worktree or in its `.git` path MUST still refuse
  as `UNSUPPORTED_FILESYSTEM`, and so MUST an ancestor that cannot be resolved.

## Failure modes

| Failure | Behavior |
|---|---|
| Init over committed tickets or releases | Refused with `INTENT_DIVERGED`; no state directory is created (CTS-V0-001). |
| Intent directory unreadable | Refused with `UNSUPPORTED_FILESYSTEM`; nothing is created. |
| Repository reached through a symlinked ancestor | Resolved; the canonical primary worktree is recorded (CTS-V0-004). |
| Primary worktree or `.git` is a symbolic link | Refused with `UNSUPPORTED_FILESYSTEM`, unchanged (CTS-V0-004). |
| Journal already initialized | Unchanged: the existing already-initialized refusal applies first. |
| Store already frozen by an earlier init | Not repaired here; the operator removes `.git/taskman`. CTS-V0-003 imports foreign exports only and does not adopt committed native records. |
| Import item invalid or duplicated in the export | Refused before the first write; nothing is written (CTS-V0-003). |
| Import target held by a native record or another source item | Refused before the first write; nothing is written (CTS-V0-003). |
| Import into a queue whose `canonicalWriter` is `NATIVE` | Refused before the first write, so no imported record can become eligible without a cutover (CTS-V0-003). |
| Import interrupted after some writes | Each written record is complete and journaled; rerunning the same export skips the unchanged items and finishes the rest (CTS-V0-003). |

## Acceptance and rollback

CTS-V0-001 is accepted by the focused store and CLI tests below: a refused init leaves no state
directory, and the same init succeeds once the record is removed. CTS-V0-003 is accepted by focused
tests for idempotent re-import, a changed item writing the next revision, refusal over a native
record with the same `ticketId`, and the `IMPORT` source rules of the ticket record. CTS-V0-002 needs
owner acceptance, the recovered task-store contract, and its own tests before any implementation.
Rollback of CTS-V0-001 removes the `existingRecord` guard in `internal/tasks/store/store.go`; no
journal, intent or wire migration is needed, because the guard only refuses before anything is
written. Rollback of CTS-V0-003 removes the `import` verb; records it wrote stay valid `IMPORT`
shadow records that are never eligible, and nothing else in the store refers to them.

CTS-V0-004 is accepted by a CLI test that initializes a repository through a symlinked ancestor,
checks that `head.json` records the canonical path, and reads the queue through both spellings.
Rollback removes `canonicalAncestors` in `internal/tasks/intent/worktree.go`; a repository under a
symlinked ancestor is then refused again, and a journal it already wrote keeps the canonical path.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| CTS-V0-001 | `internal/tasks/store/store.go` (`Init`, `existingRecord`, `firstEntry`), `internal/tasks/cli/init.go` | TestCTSV0001_InitRefusesOverExistingRecords, TestCTSV0001_InitRefusesOverCommittedTickets |
| CTS-V0-002 | proposed; no implementation | none until accepted |
| CTS-V0-003 | accepted 2026-09-27; not yet implemented | none yet |
| CTS-V0-004 | `internal/tasks/intent/worktree.go` (`finish`, `canonicalAncestors`) | TestCTSV0004_InitThroughSymlinkedAncestor |
