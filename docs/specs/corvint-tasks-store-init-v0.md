# Corvint Tasks store initialization V0

Owner: Russell Lewis
Date: 2026-09-25 (CTS-V0-001, CTS-V0-003 and CTS-V0-004 accepted 2026-09-27; CTS-V0-002 accepted 2026-09-28)
Intent status: accepted (owner decisions 2026-09-27 and explicit two-issue fix request 2026-09-28)
Delivery status: experimental
Authoritative inputs: decision 0397 (corvint-tasks built in tree), `AGENTS.md`,
`docs/SPEC-DRIVEN-DEVELOPMENT.md`, tickets V1-0323, V1-0310 and V1-0331, and the in-tree sources under
`internal/tasks/store`, `internal/tasks/journal`, `internal/tasks/intent`, `internal/tasks/importer`,
`internal/tasks/transaction` and `internal/tasks/cli`.

## Agent digest
- Claim: `corvint-tasks init` refuses an intent store that already holds records, and works in a repository reached through a symlinked ancestor such as macOS `/tmp`.
- Status: accepted (owner decisions 2026-09-27 and explicit two-issue fix request 2026-09-28); experimental. CTS-V0-001 through CTS-V0-004 are implemented; CTS-V0-002 permits only four unaudited inventory reads when the journal directory is absent.
- Exists: journal-absent inventory reads with no audit identity, the coded init refusal, the ancestor resolution, the `corvint-tasks import` verb with its `IMPORT_APPLY` stage operation, and their store, transaction and CLI tests.
- Blocked on: broader task-store authority recovery (V1-0310) remains open; CTS-V0-002 has narrow independent owner acceptance. For a non-fixture import writer, V1-0398.
- Read next: Requirements; Import export and batching; Failure modes; Traceability.

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
the explicit owner request to fix the journal-absent reads on 2026-09-28 accepts CTS-V0-002 below. This narrow amendment does not accept or recover the broader missing authority in V1-0310.

Non-goals: adopting committed native records into a new genesis, journal-optional reads beyond the four CTS-V0-002 verbs, a new
wire code, any change to genesis bytes, projection checks or the closed code set, any other change to
the journal format than the one `IMPORT_APPLY` stage operation below (its receipt kind is already in
the closed receipt-kind set), any automatic
repair of an existing frozen store, and, for CTS-V0-003, the cutover verb, admission, attempts and
drains, the import-map writer, and writing or changing the foreign export.

## Requirements

- `CTS-V0-001`: `corvint-tasks init` MUST refuse, before it creates the state directory or any
  journal byte, when the intent store's `tickets/` or `releases/` directory holds any entry. The
  refusal MUST carry outcome `BLOCKED` in the store report, `REFUSED` at the CLI, the existing code
  `INTENT_DIVERGED`, and a reason naming the first record found. An absent or empty directory MUST
  NOT refuse. An unreadable directory MUST refuse as `UNSUPPORTED_FILESYSTEM`.
- `CTS-V0-002`: Only `queue status`, `roadmap`, `ticket show` and `ticket search` MUST
  answer from the validated current primary-worktree intent projection when the journal directory
  itself is absent. Stable uncommitted intent edits are visible; this is not a committed-Git claim.
  Success MUST retain the closed result envelope with `snapshot: null` and a warning identifying
  journal absence, the unaudited worktree projection, and unobserved journal history and liveness.
  Queue `headSeq`, `generation` and `attempts` MUST be `NOT_OBSERVED`; `barrier` and `liveAttempts`
  MUST be null, meaning unobserved rather than absent. No receipt, audit identity or eligibility
  authority may be invented. Reads MUST NOT create or alter intent or journal state. Existing,
  partial, corrupt, symlinked or unreadable journals MUST retain strict read failures; a missing
  journal file inside an existing directory never qualifies. Each read MUST validate the captured
  intent, recheck its digest and journal absence after assembly, retry intent drift at most four
  attempts, and refuse `SNAPSHOT_MOVED` on persistent drift or journal appearance. All other reads,
  receipt audits and mutations MUST retain their initialized-journal requirements; CTS-V0-001
  still refuses init over populated native records.
- `CTS-V0-003`: (accepted 2026-09-27, owner decision relayed by the orchestrator session "Work
  progress orchestration") An explicit `corvint-tasks import` verb MUST read a
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

## Import export and batching

The export (profile `corvint-tasks-import/0`, at most `MaxIntentTreeBytes`) is JSON lines, each
ending in LF. Line 1 is the closed header `{"profile","sourceQueueId"}`; every later line is the
closed item `{"sourceItemId","block","ticket"}`, at most `MaxTicketsPerQueue` items. `block` is the
verbatim foreign block that `sourceRevisionSha256` hashes. `ticket` holds exactly the 23 exporter-
mapped `taskman-ticket/0` fields: `acceptanceCriteria`, `archivedFrom`, `body`, `capabilities`,
`completion`, `dependencies`, `dueDate`, `effects`, `estimateMinutes`, `executionClass`, `holds`,
`kind`, `labels`, `milestone`, `order`, `owner`, `priority`, `requiredGates`, `requirementRefs`,
`status`, `supersededBy`, `supersedes` and `title`. Each hold is `{holdId, actor, reason}` and a
completion is `{actor, reason}` or null. The importer owns every other field: it sets `ticketId`
from the local queue and `sourceItemId`, the `source` object, `shadowOverlay`, empty `approvals`,
the revision chain and the timestamps, stamps each hold's `placedAt` (kept per `holdId` across
revisions), and turns a completion into a `MANUAL` completion with no evidence or manifest. Every
dependency and supersession target must be in the export or the store, and every gate must be
declared by the store's policy; a dependency cycle is recorded, not refused, and reads report it as
for any store.

The whole export is planned against one audit, under one lock and one authority session, before the
first write. The planned records then commit in ascending `ticketId` order as `IMPORT_APPLY`
batches: each batch is one receipt of kind `IMPORT_APPLY` with a null `ticketId`, posting up to the
existing stage limits (11 staged artifacts, 8 inline receipt posts, a 2422-byte stage descriptor), so
about seven records per batch. Each batch passes the full writer: it is re-audited against the head
it extends and replays by a request ID derived from its records. The `IMPORT_APPLY` transaction model
independently refuses a `NATIVE` writer, a non-shadow or non-`IMPORT` record, a new ticket past
revision 1, a broken revision chain, and a target held by a native record or another source item.

## Failure modes

| Failure | Behavior |
|---|---|
| Init over committed tickets or releases | Refused with `INTENT_DIVERGED`; no state directory is created (CTS-V0-001). |
| Intent directory unreadable | Refused with `UNSUPPORTED_FILESYSTEM`; nothing is created. |
| Repository reached through a symlinked ancestor | Resolved; the canonical primary worktree is recorded (CTS-V0-004). |
| Primary worktree or `.git` is a symbolic link | Refused with `UNSUPPORTED_FILESYSTEM`, unchanged (CTS-V0-004). |
| Journal directory absent | Only the four inventory verbs return a validated unaudited projection with no snapshot or journal facts (CTS-V0-002). |
| Journal exists but is partial, corrupt, symlinked or unreadable | Strict failure; no fallback and no repair (CTS-V0-002). |
| Intent changes repeatedly or journal appears during inventory read | Bounded refusal with `SNAPSHOT_MOVED`; no mixed result (CTS-V0-002). |
| Journal already initialized | Unchanged: the existing already-initialized refusal applies first. |
| Store already frozen by an earlier init | Not repaired here; the operator removes `.git/taskman`. CTS-V0-003 imports foreign exports only and does not adopt committed native records. |
| Import item invalid or duplicated in the export | Refused before the first write; nothing is written (CTS-V0-003). |
| Import target held by a native record or another source item | Refused before the first write; nothing is written (CTS-V0-003). |
| Import into a queue whose `canonicalWriter` is `NATIVE` | Refused before the first write, so no imported record can become eligible without a cutover (CTS-V0-003). |
| Import interrupted after some writes | Each written record is complete and journaled; rerunning the same export skips the unchanged items and finishes the rest (CTS-V0-003). |
| Import batch refused after earlier batches committed (for example store capacity) | Earlier batches stay committed and the CLI reports the refused batch with a warning; a rerun of the same export skips the committed items (CTS-V0-003). |
| Dependency or supersession target absent from both the export and the store, or a gate not declared by policy | Refused before the first write with `DEPENDENCY_MISSING` or `GATE_UNKNOWN`; nothing is written (CTS-V0-003). |

## Acceptance and rollback

CTS-V0-001 is accepted by the focused store and CLI tests below: a refused init leaves no state
directory, and the same init succeeds once the record is removed. CTS-V0-003 is accepted by focused
tests for idempotent re-import, a changed item writing the next revision, refusal over a native
record with the same `ticketId`, and the `IMPORT` source rules of the ticket record. CTS-V0-002 is accepted by the four read verbs, strict negative controls, bounded race tests and
a standalone clone proof. Receipt audit remains evidence of the local journal in its primary
checkout: a clone may validate published intent and compare committed bytes and ticket status,
but cannot reproduce primary `headSeq`, `projectionAgreement`, receipts or history without that
journal. Rollback of CTS-V0-002 routes the four verbs back through `withStore`; no migration or
journal deletion is required.
Rollback of CTS-V0-001 removes the `existingRecord` guard in `internal/tasks/store/store.go`; no
journal, intent or wire migration is needed, because the guard only refuses before anything is
written. Rollback of CTS-V0-003 removes the `import` verb and the `IMPORT_APPLY` stage operation;
records it wrote stay valid `IMPORT` shadow records that are never eligible, their receipts stay
decodable, and nothing else in the store refers to them. A store with a pending `IMPORT_APPLY` stage
from an interrupted run is finished by a rerun before rollback.

CTS-V0-004 is accepted by a CLI test that initializes a repository through a symlinked ancestor,
checks that `head.json` records the canonical path, and reads the queue through both spellings.
Rollback removes `canonicalAncestors` in `internal/tasks/intent/worktree.go`; a repository under a
symlinked ancestor is then refused again, and a journal it already wrote keeps the canonical path.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| CTS-V0-001 | `internal/tasks/store/store.go` (`Init`, `existingRecord`, `firstEntry`), `internal/tasks/cli/init.go` | TestCTSV0001_InitRefusesOverExistingRecords, TestCTSV0001_InitRefusesOverCommittedTickets |
| CTS-V0-002 | `internal/tasks/cli/inventory.go` (`withInventoryStore`), `internal/tasks/cli/cli.go` | TestCTS002JournalAbsentReads, TestCTS002NoFallbackForExistingJournal, TestCTS002ProjectionRaces |
| CTS-V0-003 | `internal/tasks/importer/importer.go` (`Decode`, `Plan`), `internal/tasks/store/import.go` (`Import`, `importBatch`, `packImport`), `internal/tasks/transaction/model.go` (`ImportApply`, `importPosts`, `importChain`), `internal/tasks/snapshot/stage.go` (`StageImportApply`), `internal/tasks/cli/import.go` | TestCTSV0003_ImportWritesShadowRecordsAndReimportIsIdempotent, TestCTSV0003_ChangedBlockWritesNextRevision, TestCTSV0003_ImportRefusesOverNativeRecord, TestCTSV0003_ImportRefusesWithNothingWritten, TestCTSV0003_ImportBatchesWithinStageLimits, TestCTSV0003_ImportApplyPostsAndChainsRevisions, TestCTSV0003_ImportApplyRefusals, TestCTSV0003_CLIImportWritesShadowRecordsBlockedOnCutover; IMPORT source rules: TestTMV0003_AS02_FieldRelationships, TestTMV0004_AS05_EligibilityDerived |
| CTS-V0-004 | `internal/tasks/intent/worktree.go` (`finish`, `canonicalAncestors`) | TestCTSV0004_InitThroughSymlinkedAncestor |
