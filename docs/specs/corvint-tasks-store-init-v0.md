# Corvint Tasks store initialization V0

Owner: Russell Lewis
Date: 2026-09-25 (CTS-V0-001, CTS-V0-003 and CTS-V0-004 accepted 2026-09-27; CTS-V0-002 accepted 2026-09-28; CTS-V0-006 requested 2026-10-01 in issue 433; CTS-V0-008 proposed 2026-10-10 in V1-0825 and accepted 2026-10-10, decision 0490; CTS-V0-010 accepted 2026-10-15 under the owner's standing auto-accept, V1-1379; CTS-V0-011 accepted 2026-10-21 under the same auto-accept, V1-0758; CTS-V0-012 accepted 2026-10-21 under the same auto-accept, V1-2143; CTS-V0-013 accepted 2026-10-21; CTS-V0-014 to CTS-V0-017 accepted 2026-10-22 under the same auto-accept, V1-1660)
Intent status: accepted (owner decisions 2026-09-27 and explicit two-issue fix request 2026-09-28)
Delivery status: experimental
Authoritative inputs: owner request [issue 336](https://github.com/beamfall/corvint/issues/336), decision 0397 (corvint-tasks built in tree), `AGENTS.md`,
`docs/SPEC-DRIVEN-DEVELOPMENT.md`, tickets V1-0323, V1-0310 and V1-0331, and the in-tree sources under
`internal/tasks/store`, `internal/tasks/journal`, `internal/tasks/intent`, `internal/tasks/importer`,
`internal/tasks/transaction` and `internal/tasks/cli`.

## Agent digest
- Claim: `corvint-tasks init` refuses an intent store that already holds records, and works in a repository reached through a symlinked ancestor such as macOS `/tmp`.
- Status: accepted (owner decisions 2026-09-27 and explicit two-issue fix request 2026-09-28); experimental. CTS-V0-001 through CTS-V0-004 are implemented; CTS-V0-002 permits only four unaudited inventory reads when the journal directory is absent; CTS-V0-005 adds tested non-fixture external-agent setup templates; CTS-V0-006 makes journal-backed reads wait out an in-flight writer (issue 433) instead of failing `REDO_PENDING` or `SNAPSHOT_MOVED` at once; CTS-V0-008 (accepted, decision 0490; V1-0825) makes an unlocked journal audit or archive export wait out a writer's descriptor-less staging slots before refusing `MALFORMED`, while an audit made under the writer lock still refuses them at once. CTS-V0-010 (V1-1379) makes a journal-absent `ticket show` miss name the stale `.taskman` snapshot and the journal it lacks instead of claiming the ticket does not exist. CTS-V0-011 (V1-0758) makes `ADOPT_FILE` compose added, changed or removed `requiresPool` and `requiredRoles` through `REFINE`. CTS-V0-012 (V1-2143) makes `ticket search --text` token-based and makes a list or search `--fields` refusal for `labels` or `effects` name the read that returns them. CTS-V0-014 to CTS-V0-017 (V1-0398) are the import-map writer: a non-fixture queue whose `importMapSha256` names `.taskman/import-map.json` initializes with the map journaled at genesis, takes every non-fixture write, keeps the map immutable, and imports only under the map's identity.
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
drains, and writing or changing the foreign export. The import-map writer was a non-goal until
2026-10-22; CTS-V0-014 to CTS-V0-017 (V1-0398) now specify it. Its genesis posts one more record only
for a non-fixture queue that names a map, so every fixture or null-map genesis stays byte-identical.
Updating or rewriting an import map after genesis, deriving one from an export, and admitting a map
on a fixture queue remain non-goals.

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

- `CTS-V0-005`: The external-agent setup guide MUST provide canonical queue, policy and ticket-create
  templates that initialize a new non-fixture native queue. They MUST keep enforced budget fields
  empty (unobserved, never enforced), use whole-repository fallback for incomplete effects, and
  retain the CAL-V0-020 qualification cutover before admission. The guide MUST distinguish empty
  initialization from journal-absent reads and document canonical UTF-8/set-array input.
- `CTS-V0-006`: (requested 2026-10-01, issue 433) A journal-backed read (`withStore` verbs and
  `archive export`) that probes a committed receipt `head.json` does not name yet (`REDO_PENDING`,
  the §5.2 window between a writer's receipt link-in and its head rename) MUST treat it as a writer
  in flight, not as a fact about the store: it MUST pause with bounded backoff (25 ms doubling to
  400 ms) and probe again until the writer has applied the receipt or a patience budget of two
  seconds is spent, and only then report `REDO_PENDING`. A snapshot that moved between probe and
  re-probe MUST be paused on and re-read, uncounted, while the same budget lasts; the TM-V0-008
  attempt bound applies to the unpaused re-reads once it is spent. A failure reported after at
  least one pause MUST name the wait and say the read is retryable, with the code unchanged. The
  read MUST NOT write, lock, redo the receipt or lengthen the budget; a writer that crashed inside
  the window is still reported as `REDO_PENDING` after the budget, and `JOURNAL_FORKED` and every
  other probe failure stay immediate. The `taskman-command-result/0` envelope and the
  `outcomeFor` mapping (`NOT_RUN`) are unchanged; a test binary disables the budget so planted
  pending receipts are reported at once.
- `CTS-V0-007`: (proposed (V1-0759)) A re-import that writes the next revision of an `IMPORT` record
  MUST keep that record's `requiresPool` and `requiredRoles` byte-identical when present. These
  acceptance-relevant keys are set by `REFINE` and are not export fields, so the next revision MUST
  NOT drop them and MUST NOT bump `acceptanceRevision` on their account. A record without them
  stays without them.
- `CTS-V0-008`: (accepted, decision 0490; V1-0825) A native writer holds staging slots `staging/aNN` with no
  `staging/active.json` descriptor under its writer lock for the whole of its publish (prepared,
  linked or replaced, then removed). A journal audit or an archive `export` that does not hold the
  writer lock, and sees such slots unchanged across both of its captures with neither
  `staging/active.json` nor `staging/active.json.tmp` present, MUST treat them as a writer in
  flight: it MUST pause (25 ms doubling to 400 ms) and read again until the slots are gone or
  exactly two seconds of pauses are spent, and only then refuse with the unchanged `MALFORMED`
  `unassigned stage slot` naming the byte-smallest slot (CAL-V0-114; the export names it without
  the `staging/` prefix, as before). The budget MUST NOT depend on `DefaultPatience`, which test
  binaries zero, and the export's pauses MUST NOT draw on its CTS-V0-006 deadline. Slots that
  appear or vanish between the two captures stay `SNAPSHOT_MOVED` under the four-attempt bound
  (TM-V0-008, and the export's own four attempts): a pause does not spend an attempt, a moved
  observation does not reset or extend the budget, and four moves still end in `SNAPSHOT_MOVED`.
  An audit made by a caller that holds the writer lock (reconcile, barrier, policy, release,
  import, ticket mutation and redo) MUST refuse such slots at once with no pause, since no other
  writer can be publishing. An unassigned slot beside a `staging/active.json` descriptor or an
  `active.json.tmp` temp MUST still be refused at once, and neither reader may write, lock or
  remove a slot; a killed writer's orphan slots stay refused until the next writer removes them
  (CAL-V0-019).

- `CTS-V0-009`: (accepted, decision 0513) The initial journal-backed
  head read of `run --attach` (ATR-V0-008, ATR-V0-011) MUST wait out a live writer's
  receipt-link-before-head-rename window using the same bounded pending-writer read protocol
  as CTS-V0-006: 25 ms backoff doubling to 400 ms, with the shared two-second patience budget.
  When the writer finishes within that budget, attach MUST continue against the settled head.
  A redo still pending after the budget MUST refuse with attach's unchanged `ERROR`/`REDO_PENDING`,
  naming the wait and stating that the read is retryable. Attach MUST NOT write, lock, redo
  the receipt, or extend the budget; non-transient failures remain immediate. This journal
  patience applies even with `--wait 0`, which bounds waiting for the run, not its head read.
- `CTS-V0-010`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1379) When a
  journal-absent `ticket show` (CTS-V0-002) does not find the requested ticket in the captured intent
  projection, its refusal MUST NOT say the ticket does not exist in the queue. The projection is the
  checkout's `.taskman` snapshot, which a plain clone holds without the queue's journal and which can
  predate tickets the journal holds. The refusal MUST keep outcome `REFUSED` with no code, no item,
  `snapshot: null` and the CTS-V0-002 journal-absent warning, and add one warning that names the
  ticket, the projection's intent tree SHA-256 and the absent journal path, says a ticket created
  after the last committed `.taskman` snapshot is `NOT_OBSERVED` there rather than proven absent, and
  names reading it from the checkout whose `.git` holds the journal (the primary worktree or one of
  its linked worktrees). The reader MUST NOT search for, open or trust any other store: a plain clone
  has no recorded link to the primary worktree. A journal-backed miss keeps its existing wording.
- `CTS-V0-011`: (accepted under the owner's standing auto-accept (2026-10-11), V1-0758) `ADOPT_FILE`
  MUST compose a difference in `requiresPool` or `requiredRoles` into its one `REFINE`, as it does
  for the other `REFINE` fields, whether the intent file adds the key, changes its value or drops
  it (a dropped key composes the `REFINE` clear). The post record MUST carry exactly the file's
  value, and the composed `REFINE` passes the same step rules as a direct one, so a pool the policy
  does not declare is refused `MALFORMED` and the ticket stays diverged. No adopt path may report a
  file adopted while dropping a file-only key: the difference is taken over the union of both
  records' keys, and any key no composed operation reproduces is refused `ADOPT_UNSUPPORTED_FIELD`.
- `CTS-V0-012`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2143) `ticket search
  --text T` MUST match a ticket when every whitespace-separated word of `T` occurs, case-insensitively,
  in its title or its body, in any order and not necessarily adjacent; a `T` with no word is a usage
  error `MALFORMED` before any read. `ticket list` and `ticket search` items do not carry `labels` or
  `effects`; a `--fields` refusal naming either key (or a sub-key of it) MUST stay `MALFORMED` with no
  item and MUST name the supported route, `ticket show <id> --fields record.<key>` or `ticket export`.
  Keys without a declared route keep the existing refusal wording.
- `CTS-V0-013`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1660) A checkout
  made by `git clone --shared`, whose objects are reached through `objects/info/alternates`, MUST be
  a supported Tasks repository: `init`, `ticket create` and the inventory reads (`queue status`,
  `ticket list`, `receipt audit`, `doctor`) succeed there with no `UNSUPPORTED` or
  `UNSUPPORTED_FILESYSTEM` refusal, since Tasks resolves only the worktree, its `.git` directory and
  `.taskman`, never the object store. A tool that does refuse alternates, such as dogfood's
  `unsupported-object-alternates`, names that cause itself.
- `CTS-V0-014`: (accepted under the owner's standing auto-accept (2026-10-11), V1-0398) `init` of a
  queue whose `fixture` is false and whose `importMapSha256` is non-null MUST read
  `.taskman/import-map.json`, bind it into the `INIT` request digest as `importMapSha256`, and post it
  as `intent/import-map.json` in the genesis receipt beside the queue, policy, `VERSION` and empty
  reservations. The journal audit MUST accept exactly that one extra genesis post, and the store's
  inventory MUST list the map as the intent tree does. A queue with a null `importMapSha256`, and
  every fixture queue, MUST initialize with no map read and byte-identical genesis.
- `CTS-V0-015`: (accepted under the owner's standing auto-accept (2026-10-11), V1-0398) `init` MUST
  refuse `VALIDATION_FAILED` `MALFORMED`, creating no state directory, when the named map is absent,
  differs from `importMapSha256`, does not decode as `taskman-import-map/0`, or names another queue;
  and a fixture queue that names a map stays refused as before (CAL-V0-003). Once initialized, the
  mapped non-fixture queue MUST take every write CAL-V0-001, CAL-V0-002, CAL-V0-020 and CAL-V0-027
  admit for a null-map non-fixture queue, under the same checks.
- `CTS-V0-016`: (accepted under the owner's standing auto-accept (2026-10-11), V1-0398) After genesis
  the import map is immutable: no transaction other than `INIT` may post `intent/import-map.json`, and
  a transaction that posts the queue MUST keep its `importMapSha256`; either is refused `MALFORMED`.
  The journal audit's projection check covers the map like any other canonical intent record.
- `CTS-V0-017`: (accepted under the owner's standing auto-accept (2026-10-11), V1-0398) `import`
  into a mapped queue MUST select the journaled map in its audit and pass it to the `IMPORT_APPLY`
  model, which refuses the batch `MALFORMED` without it. An imported record whose
  (`sourceQueueId`, `sourceItemId`) the map assigns to another ticket, or whose ticket the map
  assigns to another source item, MUST be refused `DUPLICATE_ID` with nothing written; an item the
  map does not name imports as before.

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
| Read probes a writer between receipt link-in and head rename | Paused with backoff and probed again; succeeds on the new head once the writer finishes (CTS-V0-006). |
| Writer stays inside the window beyond the two-second budget (crashed, or a loaded host) | `NOT_RUN`/`REDO_PENDING` naming the wait and that the read is retryable; the next mutating command redoes the receipt (CTS-V0-006). |
| Store commits throughout the budget so no two probes agree | `NOT_RUN`/`SNAPSHOT_MOVED` after the budget plus the unpaused TM-V0-008 attempts, naming the wait (CTS-V0-006). |
| Unlocked journal audit or archive export sees a writer's descriptor-less staging slots | Paused with backoff and read again; succeeds once the writer removes its slots (CTS-V0-008). |
| Audit under the writer lock (reconcile, barrier, policy, release, import, mutation, redo) sees descriptor-less staging slots | `MALFORMED` `unassigned stage slot` at once, with no pause: under the lock they can only be a killed writer's orphans (CTS-V0-008). |
| Descriptor-less staging slots outlast the two-second budget (killed writer, or a loaded host) | `MALFORMED` `unassigned stage slot` naming the byte-smallest slot, as before; the next writer removes the orphans (CTS-V0-008, CAL-V0-019). |
| Unassigned staging slot beside a stage descriptor or descriptor temp | `MALFORMED` at once, with no pause (CTS-V0-008). |
| Journal already initialized | Unchanged: the existing already-initialized refusal applies first. |
| Store already frozen by an earlier init | Not repaired here; the operator removes `.git/taskman`. CTS-V0-003 imports foreign exports only and does not adopt committed native records. |
| Import item invalid or duplicated in the export | Refused before the first write; nothing is written (CTS-V0-003). |
| Import target held by a native record or another source item | Refused before the first write; nothing is written (CTS-V0-003). |
| Import into a queue whose `canonicalWriter` is `NATIVE` | Refused before the first write, so no imported record can become eligible without a cutover (CTS-V0-003). |
| Import interrupted after some writes | Each written record is complete and journaled; rerunning the same export skips the unchanged items and finishes the rest (CTS-V0-003). |
| Import batch refused after earlier batches committed (for example store capacity) | Earlier batches stay committed and the CLI reports the refused batch with a warning; a rerun of the same export skips the committed items (CTS-V0-003). |
| Dependency or supersession target absent from both the export and the store, or a gate not declared by policy | Refused before the first write with `DEPENDENCY_MISSING` or `GATE_UNKNOWN`; nothing is written (CTS-V0-003). |
| Init of a non-fixture queue whose named import map is absent, differs from `importMapSha256`, is malformed or names another queue | `VALIDATION_FAILED` `MALFORMED`; no state directory is created (CTS-V0-015). |
| Init of a fixture queue that names an import map | Refused `MALFORMED` as before (CTS-V0-015, CAL-V0-003). |
| A later transaction posts the import map or changes the queue's `importMapSha256` | Refused `MALFORMED`; nothing is written (CTS-V0-016). |
| Import item whose source or ticket the import map assigns elsewhere | Refused `DUPLICATE_ID`; nothing is written (CTS-V0-017). |
| Re-import of an `IMPORT` record that a `REFINE` gave `requiresPool` or `requiredRoles` | Both keys are kept byte-identical; only the changed `source` bumps `acceptanceRevision` (CTS-V0-007). |

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
The safe opening boundary retains a pinned, no-follow descriptor for every directory component.
On Linux, intermediate descriptors use `O_PATH|O_DIRECTORY`, so traversal does not require
directory read access; the final directory still opens with `O_RDONLY|O_DIRECTORY`. Both retain
`O_NOFOLLOW`, and conversion to `os.Root` retains its descriptor identity check. Darwin keeps its
existing readable intermediate descriptors. `TestCTSV0004_TraversalOnlyAncestors` exercises
absolute and relative walks through an execute-only directory and refuses an unreadable final
directory; it explicitly skips when the runner can bypass those permissions. Existing replacement
tests continue to reject symlinks and FIFOs. This changes neither canonicalization nor store bytes.
Rollback of this Linux traversal optimization restores readable intermediate descriptors; it
requires no store migration and makes Landlock confinement of these packages fail again.
Rollback removes `canonicalAncestors` in `internal/tasks/intent/worktree.go`; a repository under a
symlinked ancestor is then refused again, and a journal it already wrote keeps the canonical path.

CTS-V0-006 is accepted by snapshot tests that finish a simulated writer during the pause and read
the new head, report `REDO_PENDING` only after the budget with the store byte-identical, and pause
moved re-reads; and by CLI tests that run `queue status` and `plan preview` against a planted
pending receipt a goroutine applies, and repeatedly under a writer committing twelve receipts with
a short pending window, where the same test with the budget disabled reproduces issue 433; and by
archive tests that run `archive export` repeatedly under a writer committing eight receipts (with
the budget disabled it reproduces the issue) and that bound the pauses of all four archive attempts
to one budget, with the final `SNAPSHOT_MOVED` naming the wait and called retryable. Rollback
sets `DefaultPatience` to `NoPatience` in `internal/tasks/snapshot/probe.go`, which restores the
immediate `REDO_PENDING` and the unpaused attempts; no state format or envelope changes.

CTS-V0-007 (proposed, V1-0759) is evidenced by a store test that imports a record, sets
`requiresPool` and `requiredRoles` on it with `REFINE`, re-imports a changed block, and checks both
keys byte-identical, one `acceptanceRevision` bump for the changed `source`, and a clean receipt
audit. Rollback removes the carry-over in `importer.record`; no store migration is needed, and a
later re-import then drops the keys again.

CTS-V0-008 (accepted, decision 0490; V1-0825) is evidenced by journal and archive tests that drive the race with
the capture and sleep hooks rather than timing: a descriptor-less slot planted between the captures
of the first attempt (moved), held across the second and removed during its single pause audits
clean, and the same test against the unfixed reader reproduces the V1-0825 CI refusal; an export
started beside a held slot exports the settled store after one pause; orphan slots still refuse
`MALFORMED` at the byte-smallest slot, unchanged, after pauses that sum to exactly the budget, in
both readers; a slot beside a descriptor or descriptor temp is refused with no pause in both; moved
observations before and after the budget is spent compose with it as required; a writer-locked
audit (plain, request lookup and mutation audit) refuses an orphan slot with no pause, and a
locked reconcile beside an orphan slot returns inside the budget, where the unlocked reader
held the lock for the whole two seconds. Rollback sets `StageSlotPatience` to zero in
`internal/tasks/snapshot/stage_slot_wait.go`, which restores the immediate refusal in both readers
and makes the writer-locked mode a no-op; no state format, code or envelope changes.

CTS-V0-009 (accepted, decision 0513) requires a deterministic attach test
that holds a simulated writer after receipt link-in, proves `REDO_PENDING`, and completes the
head rename from the reader's pause hook. A second test leaves the redo pending through the
patience budget and checks the unchanged refusal and retryable warning. Both tests check that
attach writes no state, accounting separately for the simulated writer's head rename. The
loaded qualification is 100 race-enabled repetitions of
`TestATRV0008_DetachedRunSurvivesItsLauncher` with `GOMAXPROCS=3`; exact counts belong in the
V1-1126 build-log entry. Rollback restores the single immediate head probe in attach; no state
format or wire code changes.

CTS-V0-013 (V1-1660) records supported behaviour that current source already has: the
`git clone --shared` fixture `TestCTSV0013_SharedCloneIsSupported` runs init, a create and the
inventory reads in such a clone. Rollback removes the test; no source, state format or wire code
changes.

CTS-V0-014 to CTS-V0-017 (V1-0398) are accepted when a store test initializes a non-fixture queue
whose `importMapSha256` names a canonical map and drives create, prioritize, import, pause, policy
update and execution cutover through the roadmap writer, and release, plan, claim, heartbeat and
lease release through the native writer, each journal audit staying `OK` with the queue digest and
map projection unchanged; when init refuses an absent, mismatched, other-queue or fixture map with
no state directory; and when an import whose source or ticket the map assigns elsewhere refuses
`DUPLICATE_ID` with the store digest unchanged. The first and last tests fail on the unfixed tree
with the old `unsupported queue identity/state` refusal. Rollback restores the
`ImportMapSha256 != nil` refusal in `validateInput`, `stage_observation.go`, `release.go` and
`reconcile.go`, drops the map from the INIT digest and posts, the genesis count, the store inventory
and the import audit selection, and removes `import_map.go`; a store initialized with a map must
then be re-initialized, since its genesis carries the extra post. Null-map and fixture stores need
no migration.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| CTS-V0-001 | `internal/tasks/store/store.go` (`Init`, `existingRecord`, `firstEntry`), `internal/tasks/cli/init.go` | TestCTSV0001_InitRefusesOverExistingRecords, TestCTSV0001_InitRefusesOverCommittedTickets |
| CTS-V0-002 | `internal/tasks/cli/inventory.go` (`withInventoryStore`), `internal/tasks/cli/cli.go` | TestCTS002JournalAbsentReads, TestCTS002NoFallbackForExistingJournal, TestCTS002ProjectionRaces |
| CTS-V0-003 | `internal/tasks/importer/importer.go` (`Decode`, `Plan`), `internal/tasks/store/import.go` (`Import`, `importBatch`, `packImport`), `internal/tasks/transaction/model.go` (`ImportApply`, `importPosts`, `importChain`), `internal/tasks/snapshot/stage.go` (`StageImportApply`), `internal/tasks/cli/import.go` | TestCTSV0003_ImportWritesShadowRecordsAndReimportIsIdempotent, TestCTSV0003_ChangedBlockWritesNextRevision, TestCTSV0003_ImportRefusesOverNativeRecord, TestCTSV0003_ImportRefusesWithNothingWritten, TestCTSV0003_ImportBatchesWithinStageLimits, TestCTSV0003_ImportApplyPostsAndChainsRevisions, TestCTSV0003_ImportApplyRefusals, TestCTSV0003_CLIImportWritesShadowRecordsBlockedOnCutover; IMPORT source rules: TestTMV0003_AS02_FieldRelationships, TestTMV0004_AS05_EligibilityDerived |
| CTS-V0-004 | `internal/tasks/intent/worktree.go` (`finish`, `canonicalAncestors`), `internal/tasks/safeopen` | TestCTSV0004_InitThroughSymlinkedAncestor, TestCTSV0004_TraversalOnlyAncestors, TestTMV0001_AS10_DirectorySwapNoRedirectOrBlock |
| CTS-V0-005 | `docs/TASKS-EXTERNAL-AGENTS.md`, `internal/tasks/cli/testdata/external-agents/`, CLI help | TestExternalAgentTemplatesRequireQualification |
| CTS-V0-006 | `internal/tasks/snapshot/probe.go` (`Reader.Read`, `Reader.Patience`, `DefaultPatience`, `readBackoff`, `afterWait`), `internal/tasks/fixture/fixture.go` (`init`, `ApplyReceipt`) | TestCTSV0006_ReadWaitsForInFlightWriter, TestCTSV0006_ReadReportsPendingAfterPatience, TestCTSV0006_MovedReadsPauseBetweenAttempts, TestCTSV0006_ReadVerbsWaitForWriterToApplyReceipt, TestCTSV0006_ReadVerbsUnderConcurrentWriter; unchanged attempt counts: TestTMV0008_AS36_ReadRetriesThenSnapshotMoved, TestTMV0008_AS07_ReadsLeaveStoreByteIdentical |
| CTS-V0-007 | `internal/tasks/importer/importer.go` (`record`) | TestCTSV0007_ReimportKeepsRefinedPoolAndRoles |
| CTS-V0-011 | `internal/tasks/mutation/adopt.go` (`composeAdopt`, `differingFields`, `adoptCovered`) | TestCTSV0011_AdoptComposesPoolAndRoles, TestCTSV0011_AdoptUnknownPoolRefused |
| CTS-V0-013 | `internal/tasks/intent/worktree.go` (`Resolve`; no object-store read) | TestCTSV0013_SharedCloneIsSupported |
| CTS-V0-012 | `internal/tasks/cli/cli.go` (`searchFilter.matches`, `parseSearchFilter`, `recordFieldRoutes`), `internal/tasks/cli/compact.go` (`fieldRoute`) | TestCTSV0012_SearchTextIsTokenBasedAndFieldsNameTheRoute |
| CTS-V0-014 | `internal/tasks/store/store.go` (`readImportMap`, `Init`), `internal/tasks/transaction/model.go` (`Digest`, INIT posts), `internal/tasks/journal/records.go` (genesis post count), `internal/tasks/snapshot/stage.go` (INIT stage shape), `internal/tasks/store/scan.go` (`scanIntentWithReader`) | TestCTSV0014_ImportMappedQueueTakesEveryWrite |
| CTS-V0-015 | `internal/tasks/transaction/import_map.go` (`bindImportMap`), `internal/tasks/transaction/model.go` (`validateInput`), `internal/tasks/store/release.go`, `internal/tasks/store/reconcile.go`, `internal/tasks/snapshot/stage_observation.go` | TestCTSV0015_ImportMapBindingRefusesInit, TestCTSV0014_ImportMappedQueueLeasesAndReleases, TestCALV0001_ImportMappedQueueStaysRefused |
| CTS-V0-016 | `internal/tasks/transaction/import_map.go` (`importMapUnchanged`) | TestCTSV0014_ImportMappedQueueTakesEveryWrite |
| CTS-V0-017 | `internal/tasks/transaction/import_map.go` (`importMapAdmits`), `internal/tasks/store/import.go` (`auditImport`, `importBatch`) | TestCTSV0017_ImportRefusesMapIdentityConflicts, TestCTSV0014_ImportMappedQueueTakesEveryWrite |
| CTS-V0-010 | `internal/tasks/cli/cli.go` (`ticketShow`, `journalAbsentNotFound`) | TestCTSV0010_JournalAbsentShowNamesTheStaleSnapshot; `docs/build-log/2026-10-15-v1-1384-1379-tasks-cli-refusals.md` |
| CTS-V0-009 | `internal/tasks/cli/attempt_detach.go` (`attemptAttach`), `internal/tasks/snapshot/probe.go` (`Reader.Read`) | TestCTSV0009_AttachWaitsForReceiptLinkBeforeHeadRename, TestCTSV0009_AttachReportsPendingAfterPatience; loaded qualification: TestATRV0008_DetachedRunSurvivesItsLauncher |
| CTS-V0-008 | `internal/tasks/snapshot/stage_slot_wait.go` (`StageSlotWait`), `internal/tasks/journal/stage_read.go` (`readStage`, `slotsInFlight`), `internal/tasks/journal/audit.go` (`audit`, `Reader.WriterLocked`, `stageSleep`), `internal/tasks/store/guards.go` (`lockedJournalReader`) and its writer-locked call sites, `internal/tasks/archive/stage_read.go` (`readStage`, `slotsInFlight`, `readArchive`) | TestCTSV0008_StageSlotWaitBackoffAndBudget, TestCTSV0008_AuditWaitsForWriterHeldStageSlot, TestCTSV0008_OrphanStageSlotStillMalformedAfterBoundedWait, TestCTSV0008_UnassignedSlotBesideDescriptorRefusedAtOnce, TestCTSV0008_WriterLockedAuditRefusesOrphanSlotWithoutPause, TestCTSV0008_MovedObservationsAndSpentWaitCompose, TestCTSV0008_LockedReconcileRefusesOrphanSlotWithoutWaiting, TestCTSV0008_ArchiveWaitsForWriterHeldStageSlot, TestCTSV0008_ArchiveOrphanSlotMalformedAfterBoundedWait, TestCTSV0008_ArchiveSlotBesideDescriptorRefusedAtOnce, TestCTSV0008_ArchiveMovedAttemptsAndSpentWaitCompose, TestCTSV0008_ArchiveStagePauseLeavesPatienceForPending; unchanged refusal order: TestCALV0114_DivergentProjectionRefusalIsDeterministic |
