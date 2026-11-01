# Corvint Tasks store initialization V0

Owner: Russell Lewis
Date: 2026-09-25 (CTS-V0-001, CTS-V0-003 and CTS-V0-004 accepted 2026-09-27; CTS-V0-002 accepted 2026-09-28; CTS-V0-006 requested 2026-10-01 in issue 433; CTS-V0-008 proposed 2026-10-10 in V1-0825 and accepted 2026-10-10, decision 0490; CTS-V0-010 accepted 2026-10-15 under the owner's standing auto-accept, V1-1379; CTS-V0-011 accepted 2026-10-21 under the same auto-accept, V1-0758; CTS-V0-012 accepted 2026-10-21 under the same auto-accept, V1-2143; CTS-V0-013 accepted 2026-10-21; CTS-V0-014 to CTS-V0-017 accepted 2026-10-22 under the same auto-accept, V1-1660; CTS-V0-018 accepted 2026-10-22 under the same auto-accept, V1-2401; CTS-V0-020 and CTS-V0-021 accepted 2026-10-23 under the same auto-accept, V1-2400; CTS-V0-022 accepted 2026-10-23 under the same auto-accept, V1-0903; CTS-V0-025 proposed 2026-10-24 in V1-2523, not accepted; CTS-V0-026 accepted 2026-10-24 under the standing lane auto-accept, V1-2506; CTS-V0-800 accepted 2026-10-31 under the same auto-accept, V1-2479; CTS-V0-870 to CTS-V0-873 accepted 2026-10-31 under the same auto-accept, V1-3278, V1-3263, V1-3287 and V1-3304)
Intent status: accepted (owner decisions 2026-09-27 and explicit two-issue fix request 2026-09-28)
Delivery status: experimental
Authoritative inputs: owner request [issue 336](https://github.com/beamfall/corvint/issues/336), decision 0397 (corvint-tasks built in tree), `AGENTS.md`,
`docs/SPEC-DRIVEN-DEVELOPMENT.md`, tickets V1-0323, V1-0310 and V1-0331, and the in-tree sources under
`internal/tasks/store`, `internal/tasks/journal`, `internal/tasks/intent`, `internal/tasks/importer`,
`internal/tasks/transaction` and `internal/tasks/cli`.

## Agent digest
- Claim: `corvint-tasks init` refuses an intent store that already holds records, and works in a repository reached through a symlinked ancestor such as macOS `/tmp`.
- Status: accepted (owner decisions 2026-09-27 and explicit two-issue fix request 2026-09-28); experimental. CTS-V0-001 through CTS-V0-004 are implemented; CTS-V0-002 permits only four unaudited inventory reads when the journal directory is absent; CTS-V0-005 adds tested non-fixture external-agent setup templates; CTS-V0-006 makes journal-backed reads wait out an in-flight writer (issue 433) instead of failing `REDO_PENDING` or `SNAPSHOT_MOVED` at once; CTS-V0-008 (accepted, decision 0490; V1-0825) makes an unlocked journal audit or archive export wait out a writer's descriptor-less staging slots before refusing `MALFORMED`, while an audit made under the writer lock still refuses them at once. CTS-V0-010 (V1-1379) makes a journal-absent `ticket show` miss name the stale `.taskman` snapshot and the journal it lacks instead of claiming the ticket does not exist. CTS-V0-011 (V1-0758) makes `ADOPT_FILE` compose added, changed or removed `requiresPool` and `requiredRoles` through `REFINE`. CTS-V0-012 (V1-2143) makes `ticket search --text` token-based and makes a list or search `--fields` refusal for `labels` or `effects` name the read that returns them. CTS-V0-014 to CTS-V0-017 (V1-0398) are the import-map writer: a non-fixture queue whose `importMapSha256` names `.taskman/import-map.json` initializes with the map journaled at genesis, takes every non-fixture write, keeps the map immutable, and imports only under the map's identity. CTS-V0-018 (V1-2401) checks the whole import plan against the map before the first batch and names every conflict. CTS-V0-020 and CTS-V0-021 (V1-2400) add the one amendment path for that map after genesis: an OWNER-only `import amend-map` that journals the amended map and the queue naming it under one `IMPORT_PLAN` receipt, bound to the journaled map the owner reviewed, refusing any identity conflict `DUPLICATE_ID`. CTS-V0-022 (V1-0903) charges the CTS-V0-006 budget with pauses only, so a read or export whose body takes longer than two seconds (a 10k-ticket store) still waits out a writer it meets. CTS-V0-023 (V1-0903) bounds the directory roots one export holds to a true peak of 16, so a 10k-ticket store no longer exhausts a low `ulimit -n`; CTS-V0-024 corrects the CTS-V0-020 rollback guidance: an `IMPORT_PLAN` receipt needs no restore. CTS-V0-025 (V1-2523, proposed) refuses every Tasks command but help and version `UNSUPPORTED` up front off darwin and linux. CTS-V0-026 (V1-2506) amends CTS-V0-008: once the unlocked journal audit's wait is spent under an unchanged head, a free writer lock proves the slots a killed writer's orphans, and the audit reads past exactly those slots instead of refusing `MALFORMED` until the next writer.
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
  CTS-V0-022 amends this: the budget is two seconds of pauses, never the read's own run time.
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
  (CAL-V0-019). CAL-V0-340 amends the journal audit's spent-budget refusal: when the head that
  audit read moved during the wait, which warrants another bounded observation but does not prove
  the slots' holders are live, the audit refuses `SNAPSHOT_MOVED` (retryable) instead.

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
  CTS-V0-020 amends this: the `IMPORT_PLAN` amendment is the one other transaction that may post both.
  The journal audit's projection check covers the map like any other canonical intent record.
- `CTS-V0-017`: (accepted under the owner's standing auto-accept (2026-10-11), V1-0398) `import`
  into a mapped queue MUST select the journaled map in its audit and pass it to the `IMPORT_APPLY`
  model, which refuses the batch `MALFORMED` without it. An imported record whose
  (`sourceQueueId`, `sourceItemId`) the map assigns to another ticket, or whose ticket the map
  assigns to another source item, MUST be refused `DUPLICATE_ID` with nothing written; an item the
  map does not name imports as before.
- `CTS-V0-018`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2401) `import`
  into a mapped queue MUST check every planned record against the journaled map before it writes the
  first `IMPORT_APPLY` batch. A plan with any CTS-V0-017 conflict MUST be refused `DUPLICATE_ID`,
  naming each conflicting source item and its ticket, with no batch written and a non-zero exit. The
  per-batch CTS-V0-017 check stays the authority. Non-goals: amending the map after genesis (V1-2400,
  now CTS-V0-020) and checking conflicts with tickets outside the plan.
- `CTS-V0-020`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2400)
  `corvint-tasks import amend-map --request-id ID --expected-map-sha256 SHA256 --file PATH [--role
  OWNER]` MUST commit, through the §5.2 writer, one receipt of the reserved kind `IMPORT_PLAN` (stage
  operation `IMPORT_PLAN`: at most 7 staged artifacts and a 1838-byte descriptor, the queue and the
  map each inline or as one blob) that posts the canonical `taskman-import-map/0` file as
  `intent/import-map.json` and `intent/queue.json` with `importMapSha256` naming it, and nothing
  else. The request digest binds `expectedImportMapSha256` and the new map's digest. The amendment is
  reviewed in that it is OWNER-only (an OPERATOR or other binding is `UNAUTHORIZED`) and names the
  journaled map the owner reviewed: another current map is `REVISION_CONFLICT`, and the same map is
  a completed no-change. A queue without a map, a fixture queue, a map that does not decode or names
  another queue is refused `MALFORMED`. The journal audit checks the amended map and queue like any
  canonical intent record, the request ID replays the committed receipt, and later imports select
  the amended map (CTS-V0-017, CTS-V0-018).
- `CTS-V0-021`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2400) An
  amendment MUST keep every existing ticket identity: every ticket the journaled map names stays
  mapped (its source item may change), a source item the journaled map names keeps its ticket, a
  source item an existing ticket was imported from stays on that ticket, no ticket is mapped
  twice, and an entry for a ticket the journaled map did not name may name an
  existing ticket only when that ticket is the import of exactly that source item. Any other
  amendment is refused `VALIDATION_FAILED` `DUPLICATE_ID`, naming every conflict, with nothing
  written. Non-goals: adding a map to a queue that has none, the importer following a renamed source
  item to its mapped ticket (it still derives the ticket from the source item ID, so CTS-V0-017
  refuses a renamed item's import), and any change to how an import chooses tickets.
- `CTS-V0-022`: (accepted under the owner's standing auto-accept (2026-10-11), V1-0903) The
  CTS-V0-006 patience budget of a journal-backed read and of `archive export` MUST be charged with
  the pauses the read takes and nothing else: the body's own run time, however long, MUST NOT spend
  it, and the export's attempts share one pause budget. A read whose body outlasts two seconds and
  then meets a writer between receipt link-in and head rename at its re-probe (or at the next
  attempt's probe) MUST pause, see the writer finish and re-read instead of reporting
  `REDO_PENDING` with no pause. Within one read the backoff restarts only before the first body:
  across re-reads it keeps doubling to 400 ms, so a store that moves under every long body is re-read
  a handful of times inside the budget, not once per 25 ms pause. A failure after a pause names the
  pause count and the time since the read began. The budget, attempt bound, codes and envelope are
  otherwise unchanged.
- `CTS-V0-023`: (accepted under the owner's standing auto-accept (2026-10-11), V1-0903) One
  `archive export` attempt MUST hold at most 16 no-follow directory roots at once beyond its two
  path-bound anchors (the state dir and the intent directory), counting the temporary root each
  listing binds, as a true peak: a slot is reserved, by releasing the least recently used root that
  is neither the one needed nor one of its ancestors, before each open. A released root MUST be
  reopened only under its retained parent and only as the directory the listing observed; any other
  directory there fails closed `SNAPSHOT_MOVED`. More nested roots than the bound refuse
  `RESOURCE_EXHAUSTED`. The export stream MUST stay byte-identical to an unbounded read. Non-goals:
  bounding file descriptors outside the export's directory roots, and the journal reader's
  descriptors.
- `CTS-V0-024`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2400) The CTS-V0-020
  rollback MUST NOT require restoring a store that holds an `IMPORT_PLAN` receipt. A fully applied
  amendment leaves only that receipt, whose kind the closed receipt set held before CTS-V0-020 and
  which the unchanged journal reader validates as canonical intent posts; the non-fixture writer
  stages no descriptor, so no stage decoder meets `IMPORT_PLAN`. An amendment interrupted after its
  receipt is linked is completed by the next writer's redo, which a pre-amendment writer runs
  unchanged. A rolled-back writer only refuses new amendment requests.
- `CTS-V0-025`: (proposed 2026-10-24, V1-2523, not accepted) On every GOOS but darwin and linux
  (so windows/amd64, which the 1.0 release spec lists `UNSUPPORTED` and decision 0061 defers),
  `corvint-tasks` MUST refuse every command except help, per-command help (including the help a
  command's own parser answers: `--help` anywhere in a ticket mutation argv, and `pool --help`) and
  `version` with
  outcome `ERROR`, code `UNSUPPORTED`, not retryable, before it reads or writes any store, intent
  or worktree state. The warning MUST carry `UNSUPPORTED: platform: Corvint Tasks is unsupported on
  this platform`, returned by `wire.PlatformRefusal`, which is nil on darwin and linux. PathText
  stays POSIX-absolute: a drive-letter path is not widened into it. Non-goals: making Tasks work on
  Windows (V1-2485 owns that scope decision), a new §11 code, and the `lane-leader` re-exec, which
  only a refused parent could spawn. Rollback: revert the commit; the refusal goes away and a
  Windows command fails later as before (`MALFORMED` PathText or the `safeopen` refusal).
- `CTS-V0-026`: (accepted 2026-10-24 under the owner's standing lane auto-accept (2026-10-11),
  V1-2506) amends CTS-V0-008 for the journal audit only. When an audit that does not hold the writer
  lock has spent the whole CTS-V0-008 wait beside descriptor-less slots under a head that did not
  move (so CAL-V0-340 does not apply), and its source is a native state directory named `taskman`,
  it MUST probe the writer lock `<git-common-dir>/taskman.lock` once before refusing. The probe
  MUST NOT create, write, truncate or follow a symlink to the file: it opens the regular file
  read-only through a root confined to the common directory, checks its identity before and after,
  and tests `flock(LOCK_SH|LOCK_NB)`, releasing it at once. Every writer holds that lock
  exclusively for the whole of its publish (CTS-V0-008), so a lock that is free proves every slot
  of the refused observation a killed writer's orphan. The audit then MUST observe again, setting
  aside exactly those slots, each bound by file identity, mode, size and modification time, and
  report `StagingPresent` with the slots named (`OrphanStageSlots`; `receipt audit` adds a warning
  naming them). A slot that is new or not identical to its granted observation proves a live writer
  and MUST refuse `SNAPSHOT_MOVED` (retryable) at `staging`. A lock that is held, absent, a symlink,
  not a regular file, replaced during the probe, or untestable (another GOOS) proves nothing and
  keeps the CTS-V0-008 `MALFORMED`. The probe never waits, so a writer polling for the lock loses
  at most one poll interval and never fails on its account. A writer-locked audit, a non-native
  source and an unassigned slot beside a descriptor or descriptor temp keep refusing at once. The
  read still never removes a slot; the next writer removes it (CAL-V0-019). Non-goals: the archive
  `export` reader, which keeps the CTS-V0-008 refusal; a writer that `REDO_PENDING` shows died after
  its receipt commit, which reads still never redo (TM-V0-008, V1-2506 follow-up); any change to a
  writer or to the state format. Rollback: revert the commit; reads beside a killed writer's orphan
  slots refuse `MALFORMED` again until the next writer, and no state format, code or envelope
  changes.
- `CTS-V0-800`: (accepted 2026-10-31 under the owner's standing lane auto-accept (2026-10-11),
  V1-2479) amends CTS-V0-021. An amendment MUST NOT rename the source item of a ticket the store
  holds as an import: when an existing ticket was imported from source item S of queue Q and the
  amended map names that ticket, the entry naming it MUST be (Q, S). The importer derives the
  ticket from the source item ID, so a renamed entry would make the ticket's re-import refuse
  under CTS-V0-017; the amendment is refused `VALIDATION_FAILED` `DUPLICATE_ID` naming the ticket,
  its import source and the refused rename, with nothing written, beside every other CTS-V0-021
  conflict. A mapped ticket the store does not hold yet keeps the CTS-V0-021 rename. Non-goals:
  the importer following a renamed source item (still a CTS-V0-021 non-goal) and any change to
  how an import chooses tickets. Rollback: revert the commit; such a rename commits again and its
  re-import refuses as before, and no state format, code or envelope changes.
- `CTS-V0-870`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3278)
  A `ticket search` with no filter is a defined usage error, not a read: it MUST refuse
  `MALFORMED` at field `argv` with a null snapshot before any store read, and its warning MUST name
  `ticket list` as the unfiltered read. A test that expects a store refusal from `ticket search`
  MUST pass a filter and assert the refusal's own reason, so the usage error cannot stand in for it.
  Non-goals: making an unfiltered search succeed. Rollback: revert; the warning loses the route.
- `CTS-V0-871`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3263)
  Every read that takes `--summary` or `--fields` (`ticket show`, `list`, `search`, `roadmap`, `queue status` and the
  other compact reads) MUST accept `--json` as a no-op, because its output is always one JSON
  envelope. A leading global `--root DIR` (or `--root=DIR`) before the verb MUST run the command as
  if started in `DIR` (a relative `DIR` is taken from the working directory), so a relative file
  argument the verb reads or writes (`archive verify FILE`, `archive restore FILE`,
  `ticket obligations plan --plan`, `ticket similar --digest`, `run-batch`/`qualify` `--config` and `--out`)
  is taken from `DIR` too; `--root` without a
  directory is a `MALFORMED` usage error naming the form. `ticket show` help MUST name
  `items[0].revision` and `items[0].record.acceptanceCriteria`. Non-goals: a non-JSON output mode.
  Rollback: revert; `--json` and a leading `--root` are refused again.
- `CTS-V0-872`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3287)
  A write to a stdout whose reader has closed MUST NOT kill `corvint-tasks` with `SIGPIPE` or add an error report: the write
  fails `EPIPE` and is dropped, and the exit status stays the command's outcome status (0 for an OK
  read), so a `set -o pipefail` pipeline into `head` sees the command's result. A child process
  (a gate command) keeps the default `SIGPIPE` disposition. Non-goals: retrying the write.
  Rollback: revert; a large read into a closed pipe dies with status 141 again.
- `CTS-V0-873`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3304)
  A ticket argument whose local token ends in an unpadded number (`QB-1`) that names no ticket MUST resolve to the one
  ticket whose token differs only by leading zeros in that number (`QB-0001`); an exact token
  always wins. When two or more such tickets exist the read MUST refuse as not found and name each
  candidate. Non-goals: prefix or fuzzy matching. Rollback: revert; an unpadded token is not found.
- `CTS-V0-990`: (accepted under the owner's standing lane auto-accept (2026-10-11), V1-3565)
  `corvint-tasks init` MUST refuse, before it qualifies the filesystem or creates the state
  directory, when the intent store lacks `queue.json` or `policy.json`. The refusal MUST carry
  outcome `BLOCKED` in the store report, `REFUSED` at the CLI and code `UNINITIALIZED`, and a
  reason naming the missing path and the step that creates it: copying the canonical template
  from `internal/tasks/cli/testdata/external-agents/` per `docs/TASKS-EXTERNAL-AGENTS.md`
  (CTS-V0-005). Init MUST NOT create either file: the operator authors and customizes them.
  Rollback: revert the commit; init on a fresh repository fails `MALFORMED` with the bare `lstat`
  warning again, and no state format changes.
- `CTS-V0-991`: (accepted under the owner's standing lane auto-accept (2026-10-11), V1-3566) When
  `policy.json` holds every key this binary requires but also a top-level key it does not know,
  the decoder MUST refuse `UNSUPPORTED_VERSION` (not `MALFORMED`), naming the unknown keys, the
  file's `profile` and `policyVersion`, and that a newer `corvint-tasks` likely wrote it. A
  missing required key, with or without unknown ones, stays `MALFORMED`. Every Tasks envelope
  whose codes include `UNSUPPORTED_VERSION` MUST add one warning naming this binary's
  `version` (`<Version>+build.<Build>`, as `corvint-tasks version` prints it), so the operator
  can compare builds across hosts. The refusing read MUST still write nothing. Non-goals:
  nested unknown keys, which stay `MALFORMED`, and older binaries already released, which
  keep their message. Rollback: revert the commit; an older binary's report of a newer policy
  reads `MALFORMED` again, and no state format changes.
- `CTS-V0-820`: (accepted 2026-11-01 under the owner's standing lane auto-accept (2026-10-11);
  V1-3904) (superseded by `CTS-V0-990`, which delivered the same refusal first with code `UNINITIALIZED`; batch-1101c) `corvint-tasks init` in a repository whose intent store lacks `queue.json` or
  `policy.json`, such as a fresh `git init`, MUST refuse before it creates the state directory or
  any journal byte. The refusal MUST carry outcome `REFUSED`, the existing code
  `PREREQUISITE_UNSATISFIED`, and a warning naming each missing file by path and the fix: author the
  queue and policy from the `docs/TASKS-EXTERNAL-AGENTS.md` templates, then rerun `init`. Before,
  init exited `ERROR` `MALFORMED` with a raw `lstat` message. Non-goals: writing a default queue or
  policy, and any change to genesis bytes. Rollback: revert; the raw `MALFORMED` returns.
- `CTS-V0-821`: (accepted 2026-11-01 under the owner's standing lane auto-accept (2026-10-11);
  V1-3465; refines CTS-V0-010) A journal-absent `ticket show` miss MUST also consult this
  repository's own remote-tracking ref, `refs/remotes/origin/HEAD` or else `refs/remotes/origin/main`,
  read-only and without fetching. When that commit's tree holds `.taskman/tickets/LOCAL.json`, the
  CTS-V0-010 warning MUST name the ref, the commit SHA and the exact sync steps: read the record
  read-only with `git show SHA:.taskman/tickets/LOCAL.json`, or update this checkout's projection
  with `git merge --ff-only REF` on a checkout of that branch, then retry. When the tree lacks it,
  the warning MUST say so and name `git fetch origin` as the step that refreshes the ref; only a
  probe that read the commit's tree and found no such entry proves that absence. The read
  stays a refusal: the remote record is never decoded, served or trusted, so the outcome, code,
  item and snapshot of CTS-V0-010 are unchanged, and no other store or worktree is searched. A
  repository without either ref, a ticket id whose local part is not a plain file name, or a git
  failure or timeout, a tree probe that cannot read the tree included, leaves the CTS-V0-010
  warning as it is. Non-goals: serving a ticket from a
  ref, fetching, and writing the projection. Rollback: revert; the CTS-V0-010 wording returns.

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
| Init with no `.taskman/queue.json` or `policy.json` | Refused with `UNINITIALIZED` naming the missing file and the template step; nothing is created (CTS-V0-990, which supersedes CTS-V0-820). |
| A lane clone's `.taskman` predates a ticket its origin holds | The journal-absent miss names `origin`'s commit, the read-only `git show` and the `merge --ff-only` sync step; nothing is fetched or served (CTS-V0-821). |
| Intent directory unreadable | Refused with `UNSUPPORTED_FILESYSTEM`; nothing is created. |
| Repository reached through a symlinked ancestor | Resolved; the canonical primary worktree is recorded (CTS-V0-004). |
| Primary worktree or `.git` is a symbolic link | Refused with `UNSUPPORTED_FILESYSTEM`, unchanged (CTS-V0-004). |
| Journal directory absent | Only the four inventory verbs return a validated unaudited projection with no snapshot or journal facts (CTS-V0-002). |
| Journal exists but is partial, corrupt, symlinked or unreadable | Strict failure; no fallback and no repair (CTS-V0-002). |
| Intent changes repeatedly or journal appears during inventory read | Bounded refusal with `SNAPSHOT_MOVED`; no mixed result (CTS-V0-002). |
| Read probes a writer between receipt link-in and head rename | Paused with backoff and probed again; succeeds on the new head once the writer finishes (CTS-V0-006). |
| Read or export whose body outlasts the budget (a 10k-ticket export takes about 18 s) meets a writer in flight at its re-probe | Paused and re-read as above, the pauses still bounded by two seconds; before CTS-V0-022 it exited 1 with `REDO_PENDING` and no pause (V1-0903). |
| Writer stays inside the window beyond the two-second budget (crashed, or a loaded host) | `NOT_RUN`/`REDO_PENDING` naming the wait and that the read is retryable; the next mutating command redoes the receipt (CTS-V0-006). |
| Store commits throughout the budget so no two probes agree | `NOT_RUN`/`SNAPSHOT_MOVED` after the budget plus the unpaused TM-V0-008 attempts, naming the wait (CTS-V0-006). |
| Unlocked journal audit or archive export sees a writer's descriptor-less staging slots | Paused with backoff and read again; succeeds once the writer removes its slots (CTS-V0-008). |
| Audit under the writer lock (reconcile, barrier, policy, release, import, mutation, redo) sees descriptor-less staging slots | `MALFORMED` `unassigned stage slot` at once, with no pause: under the lock they can only be a killed writer's orphans (CTS-V0-008). |
| Descriptor-less staging slots outlast the two-second budget (killed writer, or a loaded host) | `MALFORMED` `unassigned stage slot` naming the byte-smallest slot, as before; the next writer removes the orphans (CTS-V0-008, CAL-V0-019). A journal audit whose head moved during the wait refuses `SNAPSHOT_MOVED` instead (CAL-V0-340). |
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
| Import plan with one or more import-map conflicts | Refused `DUPLICATE_ID` before the first batch, naming every conflicting source item; no batch is written, including any earlier conflict-free one (CTS-V0-018). |
| Import-map amendment by a non-OWNER binding, or naming a map other than the journaled one | `UNAUTHORIZED`, or `REVISION_CONFLICT`; nothing is written (CTS-V0-020). |
| Import-map amendment on a queue without a map, a fixture queue, or with a malformed or other-queue map | `MALFORMED`; nothing is written (CTS-V0-020). |
| Import-map amendment that renames the source item of a ticket the store holds as an import | `VALIDATION_FAILED` `DUPLICATE_ID` naming the ticket, its import source and the rename; nothing is written, so the ticket's re-import stays usable (CTS-V0-800). |
| Import-map amendment that moves a mapped item, drops or doubles a mapped ticket, or claims a ticket that is not that item's import | `VALIDATION_FAILED` `DUPLICATE_ID` naming every conflict; nothing is written (CTS-V0-021). |
| An older binary reads a store with an `IMPORT_PLAN` receipt | Its journal audit admits the receipt kind and validates the posts as canonical intent records, and its redo completes an amendment interrupted after link-in; only a new amendment request is refused (CTS-V0-024). |
| Export of a store with more listed directories than the root bound (`requests/` shards) | Least recently used roots are released and reopened under their parent; the stream is unchanged (CTS-V0-023). |
| A released directory root is replaced before it is reopened | `SNAPSHOT_MOVED`; the export retries or fails closed as any moved read (CTS-V0-023). |
| Any command but help or version on Windows or another GOOS without the safe opening boundary | `ERROR` `UNSUPPORTED` naming the platform; nothing is read or written (CTS-V0-025). Before it, a Windows test fixture failed `MALFORMED` on its drive-letter `primaryWorktree` PathText. |
| Unlocked journal audit spends the CTS-V0-008 wait beside a killed writer's orphan slots with the writer lock free | The audit sets aside exactly those slots and reads the store, naming them; `receipt audit` warns; the next writer removes them (CTS-V0-026). |
| A writer stages or replaces a slot after the free-lock proof | `SNAPSHOT_MOVED` (retryable) at `staging`, never a read that skips its slot (CTS-V0-026). |
| The writer lock is held, absent, a symlink or replaced when the wait is spent | The unchanged CTS-V0-008 `MALFORMED` `unassigned stage slot` (CTS-V0-026). |
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

CTS-V0-018 (V1-2401) is accepted when a store test imports a three-item export into a mapped queue
where two items conflict with the map, and the import refuses `DUPLICATE_ID` naming both source items
with no batch and the store digest unchanged. On the tree without the pre-check the same test fails:
the import returns no error and one refused batch. Rollback removes `ImportMapConflicts` and its
call in `importWith`, leaving the per-batch CTS-V0-017 refusal; it touches no stored state.

CTS-V0-020 and CTS-V0-021 (V1-2400) are accepted when a store test amends a mapped queue's map
(adding an item and a new source queue) under OWNER with the journaled digest, and the receipt is
`IMPORT_PLAN`, the queue and the `.taskman/import-map.json` projection name the new map, the journal
audits `CONSISTENT`, the same request replays with no second receipt and the store digest unchanged,
and an import of the newly mapped item commits; when amendments that move a mapped item, drop a
mapped ticket, map one ticket twice, re-point an item an existing ticket imports, or claim another
item's import or a native ticket each refuse
`DUPLICATE_ID` naming the conflict, a stale expected map refuses `REVISION_CONFLICT`, an OPERATOR
binding `UNAUTHORIZED` and an other-queue map `MALFORMED`, with the store digest unchanged
throughout and an admissible amendment committing afterwards; when an unmapped queue refuses
`MALFORMED`; and when the stage codec test holds the `IMPORT_PLAN` descriptor maxima exactly. On the
tree without CTS-V0-020 the store has no amendment entry point. Rollback removes `ImportPlan`,
`StageImportPlan`, `ImportMapAmend`, `importMapAmendConflicts` and `import amend-map`. A store that
already holds an `IMPORT_PLAN` receipt needs no restore (CTS-V0-024): when the amendment is fully
applied the rolled-back writer's journal reader admits its receipt; when it was interrupted after
link-in, the rolled-back writer's redo completes it. Earlier guidance to restore a pre-amendment
archive confused those historical receipts with active stage descriptors, and following it would
discard later work.

CTS-V0-023 (V1-0903) is accepted by `TestCTSV0023_ExportHoldsBoundedRoots`: an export of a store
with 200 `requests/` shards observes at most 16 held roots after every open, holds at most 24
descriptors over its baseline, verifies, and streams the same bytes as an export with no effective
bound; and by `TestCTSV0023_ReopenedShardMustBeTheListedDirectory`, where a released shard replaced
before its reopening fails `SNAPSHOT_MOVED`. On the tree without the bound the export holds a root
per shard; with the reservation moved after the open the peak test observes 17. Rollback restores
the unbounded root map; no state format, code or envelope changes.

CTS-V0-024 (V1-2400) is accepted by `TestCTSV0024_RollbackDistinguishesInterruptedAmendment`: an
applied amendment leaves no staging descriptor and audits `CONSISTENT`, and an amendment interrupted
before its head write is completed by an ordinary ticket `CREATE`'s redo, the map projected and the
store `CONSISTENT`. `internal/tasks/journal` and `internal/tasks/store/redo.go` are unchanged from
`origin/main`, so this is the pre-amendment code path; no older binary was run. It corrects guidance
only and passes on the tree before the correction.

CTS-V0-025 (V1-2523, proposed) is evidenced by `TestCTSV0025_SupportedPlatformHasNoRefusal`
(darwin, linux: no refusal, a command reaches its own handling) and
`TestCTSV0025_UnsupportedPlatformRefusesUpFront` (other GOOS: `UNSUPPORTED` with the platform text,
the worktree untouched, `version` still exit 0), which `make windows-vet` type-checks and only a
Windows host runs (`NEEDS-HOST`). `TestCTSV0025_ParserHelpAnswersOnEveryPlatform` proves
parser-level help still answers, and the unsupported-platform test proves a `--help` that is a
flag's value stays refused. `fixture.TempRepo`, the `atm` CLI test helper and the internal tests
that call `Run` directly skip a Windows test on exactly this refusal through the Tasks-side
`fixture.SkipUnsupportedPlatform` (Tasks imports no Core package, decision 0397, so it does not call
`internal/testplatform`), which `TestCTSV0025_UnsupportedPlatformSkipsOnlyOnWindows` proves does not
skip on darwin or linux. On a Windows host (2026-10-24, batch 1024c) the CTS-V0-025 tests,
`TestCriterionBindingOfflineVerify` (skips) and the help tests passed; 13 other
`internal/tasks/cli` tests that do not reach this refusal still fail there, out of this scope. Rollback is a revert; no state format or code changes.

CTS-V0-800 (V1-2479) is accepted by `TestCTSV0800_ImportMapAmendRefusesImportedSourceRename`
(`internal/tasks/store`): on a store that imported BF-1 under the map (BF-1 -> BF-1), the amendment
(NEW-1 -> BF-1) is refused `DUPLICATE_ID` naming the ticket and its import source with the store
byte-identical, the unchanged re-import of BF-1 still completes as a no-change, and a rename of a
mapped ticket the store does not hold yet still commits (CTS-V0-021). Without the check the
amendment commits and the re-import refuses `DUPLICATE_ID`, which is the reported defect.

CTS-V0-026 (V1-2506) is accepted by journal tests that drive the wait with the stage-pause hook:
`TestCTSV0026_FreeWriterLockSetsAsideKilledWritersOrphans` (two orphans beside a free lock: the
audit, a ticket read and a mutation audit succeed after exactly the budget, both slots are named and
kept, and the lock file is unchanged), `TestCTSV0026_UnprovenOrphansStayMalformed` (held, absent and
symlinked lock keep `MALFORMED`), `TestCTSV0026_SlotsStagedAfterTheProofAreRetryable` (a new or
replaced slot after the probe is a retryable `SNAPSHOT_MOVED` after one probe),
`TestCTSV0026_WriterLockedAndNonNativeAuditsNeverProbe` and `TestCTSV0026_ProbeNamesTheWriterLock`
(a real `authority.AcquireLock` hold keeps `MALFORMED`; once released the audit reads). The store
regression `TestCTSV0026_KilledGateRunLeavesAReadableStore` SIGKILLs a real `gate run` subprocess at
an artifact publication while one of its slots is occupied, then runs `ticket show`, `queue status`
and `receipt audit` through the CLI: all exit 0 and the audit warning names the orphan, and the next
gate run removes it; with the probe disabled the same test fails `MALFORMED` at `ticket show`. The
Pi `tasks-host` interruption test no longer meets `MALFORMED`; its remaining failures are the
designed `REDO_PENDING` of a writer killed after its receipt commit, out of this requirement.

CTS-V0-022 (V1-0903) is accepted by `TestCTSV0022_LongBodyKeepsPausePatience` (snapshot) and
`TestCTSV0022_SlowExportStillWaitsOutWriter` (archive), with the escalating moved re-read backoff
held by `TestCTSV0006_MovedReadsPauseBetweenAttempts`: a body that outlasts the budget and leaves a
writer in its pending window at the re-probe is paused, re-read and succeeds, with the pauses
inside the budget; both fail on the tree without the change with `REDO_PENDING` and no pause. On a
synthetic 10,000-ticket, 22,000-receipt store (`TestRC3_SyntheticStore`), 22 runs of the built
`archive export` (5 quiet and 6 under a looping `pause`/`unpause` writer, each for the old and new
binary) all exited 0 and verified, about 24 s each: a real writer's window is milliseconds, so a
quiet or slowly written store does not reproduce the exit. An uncommitted in-process run on the same
store that plants a writer's pending receipt for 1.5 s at the end of the first 6.5 s export body
failed `REDO_PENDING` on the old code and exported after one re-read with the change, the store
restored byte-identical afterwards. Rollback restores the wall-clock deadline in `Reader.Read` and
`readArchive`; no state format, code or envelope changes.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| CTS-V0-001 | `internal/tasks/store/store.go` (`Init`, `existingRecord`, `firstEntry`), `internal/tasks/cli/init.go` | TestCTSV0001_InitRefusesOverExistingRecords, TestCTSV0001_InitRefusesOverCommittedTickets |
| CTS-V0-990 | `internal/tasks/store/store.go` (`Init`, `missingIntentFile`), `internal/tasks/cli/init.go` | TestCTSV0990_InitNamesMissingQueueAndPolicy |
| CTS-V0-991 | `internal/tasks/intent/policy.go` (`DecodePolicy`, `newerPolicyKeys`), `internal/tasks/cli/cli.go` (`emit`, `binaryVersionWarning`) | TestCTSV0991_NewerPolicyKeyIsVersionSkew, TestCTSV0991_SkewEnvelopeNamesTheBinary |
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
| CTS-V0-018 | `internal/tasks/transaction/import_map.go` (`ImportMapConflicts`, `importMapConflict`), `internal/tasks/store/import.go` (`importWith`) | TestCTSV0018_ImportPlanNamesEveryMapConflict |
| CTS-V0-020 | `internal/tasks/transaction/model.go` (`ImportPlan`, `Digest`, model posts), `internal/tasks/transaction/import_map.go` (`importMapUnchanged`), `internal/tasks/snapshot/stage.go` (`StageImportPlan`), `internal/tasks/snapshot/stage_observation.go`, `internal/tasks/store/policy.go` (`ImportMapAmend`, `administrativeWriteRound`), `internal/tasks/cli/import_map.go` | TestCTSV0020_ImportMapAmendCommitsAndReplays, TestCTSV0020_ImportMapAmendRefusesUnmappedQueue, TestTMV0002_AS10_StageCodecActualMaxima |
| CTS-V0-021 | `internal/tasks/transaction/import_map.go` (`importMapAmendConflicts`) | TestCTSV0021_ImportMapAmendRefusesConflicts |
| CTS-V0-800 | `internal/tasks/transaction/import_map.go` (`importMapAmendConflicts`) | TestCTSV0800_ImportMapAmendRefusesImportedSourceRename |
| CTS-V0-022 | `internal/tasks/snapshot/probe.go` (`Reader.Read`, `AfterWait`), `internal/tasks/archive/stage_read.go` (`readArchive`) | TestCTSV0022_LongBodyKeepsPausePatience, TestCTSV0022_SlowExportStillWaitsOutWriter, TestCTSV0006_MovedReadsPauseBetweenAttempts |
| CTS-V0-023 | `internal/tasks/archive/stage_read.go` (`maxOpenRoots`, `archiveRead.parent`, `reserve`, `binding`) | TestCTSV0023_ExportHoldsBoundedRoots, TestCTSV0023_ReopenedShardMustBeTheListedDirectory |
| CTS-V0-024 | `docs/specs/corvint-tasks-store-init-v0.md` (rollback guidance); `internal/tasks/store/redo.go`, `internal/tasks/journal` (unchanged) | TestCTSV0024_RollbackDistinguishesInterruptedAmendment |
| CTS-V0-026 | `internal/tasks/journal/lock_probe.go`, `lock_probe_unix.go`, `lock_probe_other.go` (`writerLockFree`), `internal/tasks/journal/audit.go` (`audit`, `Reader.writerLockFree`, `Result.OrphanStageSlots`), `internal/tasks/journal/stage_read.go` (`readStage`, `sameSlot`), `internal/tasks/cli/receipt.go` (orphan warning) | TestCTSV0026_FreeWriterLockSetsAsideKilledWritersOrphans, TestCTSV0026_UnprovenOrphansStayMalformed, TestCTSV0026_SlotsStagedAfterTheProofAreRetryable, TestCTSV0026_WriterLockedAndNonNativeAuditsNeverProbe, TestCTSV0026_ProbeNamesTheWriterLock, TestCTSV0026_KilledGateRunLeavesAReadableStore |
| CTS-V0-025 | `internal/tasks/wire/platform_other.go`, `platform_unix.go` (`PlatformRefusal`), `internal/tasks/cli/platform.go` (`platformRefusal`), `internal/tasks/fixture/fixture.go` (`TempRepo`) | TestCTSV0025_SupportedPlatformHasNoRefusal, TestCTSV0025_UnsupportedPlatformRefusesUpFront, TestCEMLockAndTasksRefusalsSkipOnlyOnWindows |
| CTS-V0-010 | `internal/tasks/cli/cli.go` (`ticketShow`, `journalAbsentNotFound`) | TestCTSV0010_JournalAbsentShowNamesTheStaleSnapshot; `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-15-v1-1384-1379-tasks-cli-refusals.md` |
| CTS-V0-009 | `internal/tasks/cli/attempt_detach.go` (`attemptAttach`), `internal/tasks/snapshot/probe.go` (`Reader.Read`) | TestCTSV0009_AttachWaitsForReceiptLinkBeforeHeadRename, TestCTSV0009_AttachReportsPendingAfterPatience; loaded qualification: TestATRV0008_DetachedRunSurvivesItsLauncher |
| CTS-V0-008 | `internal/tasks/snapshot/stage_slot_wait.go` (`StageSlotWait`), `internal/tasks/journal/stage_read.go` (`readStage`, `slotsInFlight`), `internal/tasks/journal/audit.go` (`audit`, `Reader.WriterLocked`, `stageSleep`), `internal/tasks/store/guards.go` (`lockedJournalReader`) and its writer-locked call sites, `internal/tasks/archive/stage_read.go` (`readStage`, `slotsInFlight`, `readArchive`) | TestCTSV0008_StageSlotWaitBackoffAndBudget, TestCTSV0008_AuditWaitsForWriterHeldStageSlot, TestCTSV0008_OrphanStageSlotStillMalformedAfterBoundedWait, TestCTSV0008_UnassignedSlotBesideDescriptorRefusedAtOnce, TestCTSV0008_WriterLockedAuditRefusesOrphanSlotWithoutPause, TestCTSV0008_MovedObservationsAndSpentWaitCompose, TestCTSV0008_LockedReconcileRefusesOrphanSlotWithoutWaiting, TestCTSV0008_ArchiveWaitsForWriterHeldStageSlot, TestCTSV0008_ArchiveOrphanSlotMalformedAfterBoundedWait, TestCTSV0008_ArchiveSlotBesideDescriptorRefusedAtOnce, TestCTSV0008_ArchiveMovedAttemptsAndSpentWaitCompose, TestCTSV0008_ArchiveStagePauseLeavesPatienceForPending; unchanged refusal order: TestCALV0114_DivergentProjectionRefusalIsDeterministic |
| CTS-V0-870 | `internal/tasks/cli/cli.go` (unfiltered `ticket search` refusal) | TestCTSV0870_UnfilteredSearchNamesTicketList, TestCTS002NoFallbackForExistingJournal |
| CTS-V0-871 | `internal/tasks/cli/cli.go` (`Run`, `globalRoot`), `internal/tasks/cli/compact.go` (`compactRead`), `internal/tasks/cli/command_help.go` | TestCTSV0871_JSONIsANoOpAndRootPrecedesTheVerb, TestCTSV0871_RootGovernsRelativeFileArguments |
| CTS-V0-872 | `cmd/corvint-tasks/main.go` (`SIGPIPE` notify) | TestCTSV0872_ClosedStdoutPipeKeepsTheOutcomeStatus |
| CTS-V0-873 | `internal/tasks/cli/cli.go` (`resolveTicketArg`, `paddedCandidates`, `ticketShow`) | TestCTSV0873_UnpaddedTokenResolvesTheOnePaddedTicket |
| CTS-V0-820 | Superseded (CTS-V0-990) | Its behavior is tested by TestCTSV0990_InitNamesMissingQueueAndPolicy |
| CTS-V0-821 | `internal/tasks/cli/journal_absent_remote.go` (`remoteProjectionHint`) | TestCTSV0821_JournalAbsentMissNamesOriginSyncStep, TestCTSV0821_UnreadableTreeLeavesWarningUncertain |
