# Corvint Tasks obligation ledger V0

Owner: Russell Lewis
Date: 2026-10-08
Intent status: proposed
Delivery status: experimental

Authoritative inputs: the owner request in GitHub beamfall/corvint#680, tracked as native ticket
V1-1022 ("native per-ticket obligation ledger with step-level Playwright witnessing"). This document
is an agent-drafted proposal. Acceptance is human-owned: no requirement below is accepted, and
nothing may be implemented, promoted or advertised as delivered until the owner accepts it (in whole
or in part) and resolves the Unresolved decisions. It reuses unchanged the ticket, mutation,
receipt and fold boundaries of the [agent lease contract](corvint-tasks-agent-leases-v0.md), the
reference-plus-evidence-event shape of [operator notes](corvint-tasks-operator-notes-v0.md)
(ON-V0-002), the revision-only write of [evidence attachments](corvint-tasks-evidence-attachments-v0.md)
(TEA-V0-001) and the WORKER opt-in of [know-how notes](corvint-tasks-know-how-notes-v0.md)
(KHN-V0-021..023). Playwright facts come from the
[Playwright provider contract](playwright-external-provider-v0.md) and a scratch observation recorded
in the build log.

## Agent digest
- Claim: A native ticket can carry a bounded ledger of named obligations that a Playwright json report witnesses step by step, so progress counts proof, not sessions.
- Status: proposed (owner issue #680, V1-1022; owner acceptance required); experimental; TOL-V0-001..018, 020 and 021 implemented with focused tests; the TOL-V0-019 plain-language line and the live Playwright 1.63 fixture are NOT_RUN.
- Exists: the experimental `ticket obligations seed|show|witness|set|plan` verbs, the record member, the `internal/tasks/obligation` report reader, the receipt audit and the fingerprint, stall and loop integration; the reused mutation, evidence-store, fingerprint (CAL-V0-057), stall (CAL-V0-185) and loop (CAL-V0-102) paths.
- Blocked on: owner acceptance and the Unresolved decisions; a live Playwright 1.63 fixture before any crediting code can qualify.
- Read next: User and current state; Requirements; Failure modes and trust; Unresolved decisions.

## User and current state

Corvint Tasks records progress per ticket and per session. The dispatcher's CAL-V0-057 fingerprint
(`baseFingerprint` in `internal/tasks/dispatch/roster.go`) hashes `Status|Revision|State` per ticket
plus attempts with durable work, so any revision-bumping write counts as progress, including a
note, an attached evidence digest or a work-state string. CAL-V0-185 `stall` counts finished
sessions since the native status last changed. CAL-V0-102 loop detection counts a clean HANDOFF
without gate results, reviews or a new candidate tree as no progress. None of these knows how many
of a ticket's acceptance obligations are actually proven. For a large browser-tested ticket an agent
can spend many sessions while the number of passing, named checks stays flat, and the queue cannot
see it.

The owner built an external script (`fp-ledger.py`) to close that gap: `add --step` credits a row
when its own Playwright `test.step` passed at retry 0 even if a later soft assertion failed, `plan`
checks that each open obligation maps to exactly one test, and `splits` lists tests that should
have been combined. The simpler baseline is that script plus a body checklist: it works for one
operator, but it is not receipt-backed, not visible to `queue status` or the dispatcher, and not
bound to a commit or a report digest.

The word "obligation" already names the dependency obligation `COMPLETED|GATE_PASSED`
(`ticket.Obligations` in `internal/tasks/ticket/record.go`). This profile's ledger entries are a
different concept; its wire names are always prefixed (`obligations` member, `OBLIGATIONS_*`
operations, `taskman-obligation-*` profiles) and never reuse the dependency field.

## Requirements

- `TOL-V0-001`: A NATIVE ticket record MAY carry the optional member `obligations`, a closed
  reference `{prefix, revision, head, counts, highWater, lastRaise}`. `prefix` is 1..16 bytes of
  `[A-Z0-9]` starting with a letter. `revision` is a Count of ledger events (1..1024). `head` is the
  lowercase sha256 of the newest ledger event (TOL-V0-002). `counts` is `{witnessed, total,
  deferred, coreWitnessed, coreTotal}`, and `total` excludes `deferred` obligations. `highWater` is
  `{acceptanceRevision, witnessed}`. `lastRaise` is null or `{attempt, generation,
  acceptanceRevision}` (TOL-V0-018). All counts are canonical Count strings. `obligations` MUST
  join `wire.TicketRecordOptionalKeys`, and the Core reader (`internal/taskman`) MUST admit and
  validate it. A record without the member keeps its exact legacy bytes.
- `TOL-V0-002`: Each `OBLIGATIONS_*` write MUST derive exactly one immutable canonical ledger
  event `taskman-obligation-event/0` and post it in the existing single MUTATE derived-event slot.
  The slot holds at most one event of at most `MaxDerivedEventBytes` (65,536 bytes;
  `internal/tasks/snapshot/stage.go`). The event is stored by digest before the receipt that
  references it, exactly as ON-V0-002 stores note events. Its closed shape is `{profile, ticketId,
  revision, previous, requestSha256, request}`.
  - `previous` is the prior head or null.
  - `request` is the canonical mutation envelope that wrote it, retained whole as the operator note
    event retains its request (`internal/tasks/journal/operator_note.go`). `requestSha256` is its
    digest, which MUST equal the receipt's `mutationSha256`.
  - The operation, payload, actor, `issuedAt` and, for a WORKER witness, `attempt` and `generation`
    are read from the retained request, so the fold and audit never depend on a discarded envelope.
  - The current ledger is the fold of the chain from the first event to `head`. A write whose event would exceed the
  slot refuses `LIMIT_EXCEEDED` with the detail prefix `OBLIGATION_EVENT_TOO_LARGE:` and writes
  nothing; the caller splits the work into several writes. An event no receipt references is
  orphaned and never CURRENT. The canonical JSON, Count and limit rules of `internal/tasks/wire`
  apply unchanged. No new staging slot, receipt kind or recovery path is added.
- `TOL-V0-003`: A folded obligation entry MUST be the closed object `{id, title, core, state,
  reason, evidence, updatedAt, updatedBy}`.
  - `id` is `<prefix>-<1..6 decimal digits without leading zero>`.
  - `title` is 1..256 bytes of nonblank prose, and `core` is a boolean.
  - `state` is one of `OPEN`, `WITNESSED`, `DEFECT`, `BLOCKED` or `DEFERRED`.
  - `reason` is null or 1..512 bytes. `evidence` is null unless `state` is `WITNESSED`.
  - `updatedAt` is the retained request's `issuedAt`, and `updatedBy` is its actor.

  A ledger holds at most 256 entries. A duplicate `id` refuses `DUPLICATE_ID`.
- `TOL-V0-004`: Witness evidence MUST be the closed object `{source, eventSha256, commit,
  reportSha256, playwrightVersion, matches, declaration}`.
  - `source` is `PLAYWRIGHT_REPORT` or `DECLARED`, and `eventSha256` is the digest of the WITNESS
    event that credited the entry.
  - `commit` is a 40- or 64-hex object name. It MUST resolve to a commit in the repository at
    write time.
  - For `PLAYWRIGHT_REPORT`, `matches` lists every match of the id (all passing, TOL-V0-011). Each
    match is `{path, testId, titlePath, stepTitle, stepPath}`, where `path` is the
    repository-relative spec file and `testId` is `<spec.id>@<projectName>`. `titlePath` and
    `stepPath` are ordered arrays (semantic order, not sorted), and `stepTitle` is null for a
    test-level match. `declaration` is null.
  - For `DECLARED`, `reportSha256`, `playwrightVersion` and `matches` are null.
    `declaration` is `{manifestSha256, testId, reason}`: a caller-vouched sha256, 1..256 bytes and
    1..512 bytes.
- `TOL-V0-005`: `corvint-tasks ticket obligations seed --target T --expected-revision N --payload P`
  MUST apply operation `OBLIGATIONS_SEED` with the closed payload `{prefix, obligations}` (1..256
  sorted unique `{id, title, core}`).
  - The first seed declares `prefix`. It refuses `DUPLICATE_ID` when a non-archived native ticket
    already declares that prefix.
  - A later seed MUST repeat the same prefix (`MALFORMED` otherwise). It refuses `DUPLICATE_ID`
    for an id already present.
  - Seeded entries are `OPEN`.
- `TOL-V0-006`: `corvint-tasks ticket obligations show --target T` is a read. It MUST print the
  folded ledger, the reference counts, the head and its receipt sequence, and MUST NOT write any
  state. A ticket without a ledger prints an empty result with exit 0. When an event in the chain
  is missing from the evidence store, fails its digest or breaks the `previous` link, the ledger is
  `UNKNOWN` with a typed diagnostic, never an empty or partial ledger.
- `TOL-V0-007`: `corvint-tasks ticket obligations set` MUST apply operation `OBLIGATIONS_SET` with
  the closed payload `{changes}`.
  - `changes` is 1..64 entries `{id, state, core, reason}`, sorted by unique `id`.
  - `state` is null or one of `OPEN`, `DEFECT`, `BLOCKED`, `DEFERRED`. `core` is null or a
    boolean. At least one of them is non-null, and `reason` is required.
  - `WITNESSED` is reachable only through `OBLIGATIONS_WITNESS` (`TOL-V0-008`).
  - Demoting a `WITNESSED` entry clears its evidence and requires the reason.
  - Changing `core` is OWNER-only.
  - An unknown id refuses `MALFORMED` with the detail prefix `OBLIGATION_UNKNOWN:`.
- `TOL-V0-008`: `corvint-tasks ticket obligations witness` MUST apply operation
  `OBLIGATIONS_WITNESS` with the closed payload `{source, commit, reportSha256, playwrightVersion,
  credits, declaration, attempt, generation}`.
  - `credits` is 1..256 entries sorted by unique `id`. Each is `{id, matches}` for
    `PLAYWRIGHT_REPORT`, or `{id}` for `DECLARED`.
  - `attempt` and `generation` are required for a WORKER (TOL-V0-015) and null otherwise.
  - A credited `OPEN`, `DEFECT` or `BLOCKED` entry becomes `WITNESSED` with TOL-V0-004 evidence.
  - An entry already `WITNESSED` keeps its original evidence and is reported as
    `alreadyWitnessed`. It is not repeated in the event.
  - A `DEFERRED` entry is not credited.
  - `DECLARED` is OWNER-only by default.
- `TOL-V0-009`: `witness --from-playwright-report FILE --commit SHA [--ids ID,...]` MUST read at
  most 64 MiB of one JSON document produced by Playwright's built-in `json` reporter. It admits only
  the observed closed top-level shape (`config`, `errors`, `stats`, `suites`).
  - `config.version` MUST be in the qualified version list. The list is empty until the live
    fixture in Acceptance evidence passes on a 1.63.x PWP-V0-008 tuple. Otherwise the command
    refuses `UNSUPPORTED_VERSION` with the detail prefix `OBLIGATION_REPORT_VERSION_UNQUALIFIED:`.
  - The report MUST NOT be retained, because results carry `stdout`, `stderr` and attachments.
    Only its sha256 (`reportSha256`) and the WITNESS event are kept.
  - The event's titles are secret-screened before storage. A detected secret refuses
    `SECRET_DETECTED`.
  - The optional `--ids` restricts crediting to a sorted subset, so that a large witness can be
    split to fit TOL-V0-002.
  - A missing, unreadable, oversize, non-JSON or wrong-shape report refuses `MISSING_EVIDENCE` or
    `MALFORMED` with the detail prefix `OBLIGATION_REPORT:` and writes nothing.
- `TOL-V0-010`: Crediting MUST use only results with `retry` `0` of tests whose `expectedStatus` is
  `passed`.
  - A step credits the ids in its own title exactly when that step object has no `error` member.
    A later sibling step's failure, a soft failure outside any step (`result.errors`) or the
    test's final status does not affect it. Independent `expect.soft` steps therefore credit
    independently.
  - A parent step whose nested step soft-failed carries its own `error` (observed on 1.61.1) and
    does not credit.
  - An id in the title path (describe titles and the test title, not the file) credits only when
    the retry-0 result `status` is `passed`.
  - A result with status `timedOut`, `interrupted` or `skipped` credits nothing.
  - Ids match as whole tokens bounded by a non `[A-Za-z0-9-]` byte or the string edge.
- `TOL-V0-011`: Every match of an id in the report MUST pass, and be bound by TOL-V0-012, for the
  id to be credited.
  - If one match passes and another fails (another project, test or step), the id is reported as
    `conflicting` and is not credited.
  - An id matched only by failing results is reported as `failed`.
  - An id with the ledger prefix that the ledger does not hold is reported as `unknown`.
  - Witness never sets `DEFECT` automatically.
- `TOL-V0-012`: Each match MUST pass a source presence check. Its `spec.file` (relative to
  `config.rootDir`) MUST map to a repository-relative `path` inside the repository. The blob at that
  path in `--commit` MUST contain the id as a literal whole token. Otherwise the id is reported as
  `unbound` with the reason `OUTSIDE_REPOSITORY`, `ABSENT_AT_COMMIT` or `ID_NOT_IN_SOURCE`, and is
  not credited. This is the only check that binds a report to the declared commit, because the
  report carries no commit by default.
- `TOL-V0-013`: Validation and audit are source-specific.
  - For `PLAYWRIGHT_REPORT`, the writer MUST recompute TOL-V0-010..012 from the report and refuse
    any payload that differs. The refusal has outcome `VALIDATION_FAILED`, code `MALFORMED` and
    the detail prefix `OBLIGATION_CREDIT_MISMATCH:`.
  - `receipt audit` MUST re-verify each ledger event from its stored content alone:
    - the event digest, the `previous` link and the fold;
    - that the retained request decodes, that its digest equals the receipt's `mutationSha256`,
      and that its request ID and queue ID match the receipt (the operator-note audit binding);
    - for a WORKER witness, that the retained `attempt` and `generation` named a live attempt
      held by that actor at the receipt's prior state;
    - that every credited id has at least one match;
    - that each match's `path` holds the id at `commit` (TOL-V0-012).
  - A missing or failing event is `INCONSISTENT`. The audit cannot prove that the discarded report
    held no failing match. That limit is retained (Failure modes and trust).
  - For `DECLARED`, the audit checks the event shape, the commit and the actor's grant only.
  - A witness that credits nothing writes no receipt and returns `written: false`. Like a writing
    witness, it returns the `credited`, `alreadyWitnessed`, `conflicting`, `failed`, `unknown`,
    `unbound` and `unmatched` id lists, each bounded and sorted.
- `TOL-V0-014`: Every `OBLIGATIONS_*` write MUST be an ordinary receipt-backed mutation. It uses
  the TM-V0-006 request-id replay, the `expectedRevision` CAS (required), Apply, finalize, journal
  append and fold.
  - The record revision moves by 1 and `acceptanceRevision` does not move, because no
    acceptance-relevant member changes. Gate results stay bound (TEA-V0-001 precedent).
  - Each write moves `obligations.revision` by 1 and updates `head`, `counts` and `highWater`, and
    `lastRaise` where TOL-V0-018 applies.
  - Writes are admitted on OPEN and HELD native tickets. DRAFT, COMPLETED, ARCHIVED and IMPORT
    tickets refuse `BLOCKED` with `TICKET_STATE`.
  - `ADOPT_FILE` and `IMPORT_APPLY` MUST refuse any difference in the member.
  - In V0 a ledger write never satisfies a gate, an acceptance criterion or a completion.
- `TOL-V0-015`: Roles are as follows.
  - OWNER is granted all three operations by default. OPERATOR gets them only through an explicit
    `policy.roles.OPERATOR` row (the `ExplicitGrantOperations` class).
  - A WORKER MAY run `witness --from-playwright-report` only when the policy carries the optional
    closed key `obligations {workerWitness: true}`. The payload MUST also name the WORKER's
    `attempt` and `generation`. These are checked exactly as KHN-V0-022 checks them
    (`internal/tasks/mutation/know_how.go`):
    - missing values refuse `MALFORMED`;
    - an unaudited attempt inventory refuses `PROVENANCE_UNVERIFIED`;
    - an attempt that is not live, or whose generation is not current, refuses `FENCED`, including
      an older process of the same actor after reclamation;
    - an attempt held by another binding refuses `UNAUTHORIZED`.
  - A WORKER can never seed, set or declare.
  - REVIEWER, IMPORTER and SYSTEM cannot be granted any `OBLIGATIONS_*` operation.
  - Omitting the policy key keeps canonical policy bytes.
- `TOL-V0-016`: `highWater` MUST be monotone within one acceptance revision.
  - After each write, when `highWater.acceptanceRevision` equals the record's, it is
    `max(previous witnessed, current witnessed)`. Otherwise it resets to
    `{current acceptanceRevision, current witnessed}`.
  - Demotions lower `counts.witnessed` but never `highWater` within the revision, so
    set-then-rewitness churn is not progress.
- `TOL-V0-017`: For a ticket that carries `obligations`, the CAL-V0-057 per-ticket fingerprint line
  MUST become `Status|AcceptanceRevision|State`, followed by the line
  `obligations|<highWater.acceptanceRevision>|<highWater.witnessed>`. Gate and attempt rows are
  unchanged.
  - Ledger, note and attachment writes therefore stop counting as progress, while a raised
    high-water mark counts.
  - A ticket without the member MUST fingerprint byte-identically to today.
  - The native observation MUST carry `dispatch.Ticket.Obligations` (the reference counts and high
    water, nil when absent). A workState program cannot supply it.
  - `dispatch status` MUST show `witnessed/total` for observed tickets that carry it.
- `TOL-V0-018`: A WORKER witness that raises `highWater.witnessed` MUST set `lastRaise` to its
  `{attempt, generation, acceptanceRevision}`. A write by any other actor leaves `lastRaise`
  unchanged.
  - While a policy carries CAL-V0-102 `loopDetection`, `LoopHoldOf` already receives the record. In
    `noProgressRun` it MUST treat the generation that `lastRaise` names as progress, which ends the
    trailing run. This applies only when `lastRaise` matches the attempt ID, the generation and the
    record's current acceptance revision.
  - Because the value is on the record, the check needs no new claim-time capture and no
    `loopEvidence` change. A claim admitting the next generation sees it before `LoopHoldOf` runs.
  - A null or non-matching `lastRaise` keeps the CAL-V0-102 rule unchanged.
  - Witnesses by OWNER or OPERATOR are not attributed to a generation.
- `TOL-V0-019`: `queue status` and `queue status --summary` MUST add the key `obligations`
  `{tickets, witnessed, total, deferred, core: {witnessed, total}}`.
  - The sums cover OPEN and HELD native tickets that carry a ledger.
  - The key is present only when at least one such ticket exists, so legacy output stays
    byte-identical. This amends the CAL-V0-167/CAL-V0-184 summary key set.
  - `ticket show` and `ticket list` items MUST carry the optional `obligations` count object.
  - Plain-language output MUST print `obligations: witnessed/total (core cw/ct)`.
  - These reads are derived from the loaded records, with no evidence-store or receipt scan.
- `TOL-V0-020`: `corvint-tasks ticket obligations plan --target T --plan FILE` is an optional
  read-only check of a closed plan document `taskman-obligation-plan/0` `{profile, ticketId,
  tests}`. Each test is `{test, project, obligations}` with 1..256 sorted unique ids, and a plan
  holds at most 4,096 tests. The check reports:
  - `UNASSIGNED` for each `OPEN`, `DEFECT` or `BLOCKED` obligation with zero planned tests;
  - `SPLIT`, with the test count and test list, for each such obligation with two or more;
  - `UNKNOWN_OBLIGATION` for a planned id the ledger lacks;
  - `ALREADY_CLOSED` for a planned id the ledger holds as `WITNESSED` or `DEFERRED`.

  It exits 0 only when every listed open obligation is assigned to exactly one planned test, and
  it writes nothing.
- `TOL-V0-021`: A CAL-V0-185 `stall` count MUST restart at zero when a ticket's observed
  `highWater.witnessed` rises within the same acceptance revision.
  - The comparison baseline is the optional `StallState` member `witnessed {acceptanceRevision,
    highWater}`, written only for ledger tickets.
  - The member changes the closed dispatcher ledger, so it MUST ship as the next
    `taskman-dispatch-state` version (after `/3`). The previous version is adopted with an absent
    baseline, which is seeded from the first observation without a restart. The new version is
    listed in `version` formats (CAL-V0-131), and older builds refuse it `UNSUPPORTED_VERSION`
    (CAL-V0-132).
  - A raise observed while a session is running restarts the count at that session's finish, as a
    status change does.

## Failure modes and trust

| Failure | Effect without this profile | Required behaviour |
|---|---|---|
| Forged title: a report names an id in a test or step the repository never wrote | — | No credit unless the spec file at `--commit` literally contains the id (TOL-V0-012); the command reports the id as `unbound` and no event stores it. Titles alone are never trusted. |
| Hand-edited report JSON, or a report whose failing matches are later disputed | — | The report is discarded after validation; the event keeps its digest and the passing, source-bound matches. Audit re-checks those against the commit but cannot prove the discarded report held no failing match or came from a real run. Trust is bounded by the declared actor and role (TOL-V0-015); a qualified receipt path is a non-goal for V0. |
| Duplicate ids: seeded twice, or matched by several tests, steps or projects | — | Seed refuses `DUPLICATE_ID` (TOL-V0-003/005); a cross-ticket prefix collision refuses; an id matched by both passing and failing results is `conflicting` and uncredited (TOL-V0-011). |
| Report from a different commit | — | The source presence check at the declared commit (TOL-V0-012) refuses matches whose file or id is absent there; a commit the repository lacks refuses. A report run on an unrecorded working tree that happens to match is a retained limit (Unresolved decision 6). |
| Flaky retries | A pass at retry 1 looks like success | Only retry 0 credits (TOL-V0-010); a `flaky` test's retry-1 pass never credits. |
| Missing, truncated, oversize or foreign-format report | — | Refuses with `OBLIGATION_REPORT:` and writes nothing (TOL-V0-009). |
| Unqualified Playwright version | — | Refuses `UNSUPPORTED_VERSION` with `OBLIGATION_REPORT_VERSION_UNQUALIFIED:`. |
| Secrets in report output | — | The report is never stored; the WITNESS event carries ids, paths and titles only, and they are secret-screened before storage; a detected secret refuses `SECRET_DETECTED`. |
| Ledger churn used to fake progress | Any revision write is progress today | The fingerprint uses the acceptance revision and the monotone high water (TOL-V0-016/017). |
| Evidence store missing a ledger event, or a broken `previous` chain | — | `show` reports `UNKNOWN`; receipt audit reports `INCONSISTENT`; nothing is treated as an empty or partial ledger. Counts on the record stay readable. |
| Witness or seed larger than the 64 KiB derived-event slot | — | Refuses `LIMIT_EXCEEDED` (`OBLIGATION_EVENT_TOO_LARGE:`) and writes nothing; the caller splits with `--ids` or several seeds (TOL-V0-002/009). |
| A stale WORKER process witnesses after reclamation | — | The payload names attempt and generation; a non-current generation refuses `FENCED` (TOL-V0-015). |
| Concurrent writers | — | The required CAS refuses the stale writer `REVISION_CONFLICT`; the request-id replay makes retries idempotent. |
| Older binary meets a record with the member | — | The closed reader refuses the record (fail-closed); see Rollout and rollback. |

## Non-goals and simpler baseline

- Gating completion, gates or acceptance on obligations. V0 only records and reports; whether
  `complete` should require every core obligation `WITNESSED` is Unresolved decision 1.
- Running tests, launching Playwright or editing reports. Corvint consumes a finished report only.
- Consuming PWP-V0 qualified receipts. The `/2` redaction profile retains steps, so a later
  `VERIFIED_RECEIPT` source could credit from a receipt Corvint itself produced; it is excluded here.
- Playwright tags or annotations as id carriers; other test frameworks; the blob or html reporters.
- The `fp-ledger.py` `splits` heuristic (same spec, describe and project without an `isolated`
  annotation). The plan check reports SPLIT per obligation instead.
- Dispatcher role predicates that match on witnessed counts.
- A foreign-queue importer carrying the member.
- Storing reports, screenshots or traces.

The simpler baseline stays available: a body checklist plus the external script. This profile is
justified only if the owner wants the queue and dispatcher to see proven progress natively.

## Acceptance evidence and traceability

V1-1022 implements the profile as experimental under the owner acceptance recorded in decision
0456 (`docs/build-log/2026-10-08-v1-1022-obligation-ledger.md`). Delivered evidence is focused Go
tests on synthetic Playwright json reports; the live Playwright 1.63 fixture, the integrated items in
the last column and the TOL-V0-019 plain-language status line remain `NOT_RUN`. The qualified
Playwright version list is empty, so a real report refuses `UNSUPPORTED_VERSION` until a live
fixture passes.

| Requirement | Ticket acceptance | Implementation boundary | Delivered evidence | Required integrated evidence (NOT_RUN) |
|---|---|---|---|---|
| TOL-V0-001..004 | V1-1022 | `internal/tasks/wire` (optional key), `internal/tasks/ticket` (record codec), `internal/taskman` (Core reader), `internal/tasks/snapshot` (derived-event slot) | `TestTOLV0001_RecordMemberRoundTrip`, `TestTOLV0002_EventCanonicalAndChained`, `TestTOLV0002_EventSlotBound` (65,536-byte boundary), `TestTOLV0003_EntryCodecRefusals`, `TestTOLV0004_EvidenceCodec` (`internal/tasks/ticket`); `TestTOLV0001_ReaderAdmitsObligations` (`internal/taskman`); `TestTOLV0002_OversizedWriteRefused`, `TestTOLV0002_FoldFromRetainedRequestsAfterRestart` (one event per MUTATE), `TestTOLV0003_UpdatedAtIsIssuedAtNotRecordedAt` (`internal/tasks/cli`); legacy byte identity in `TestTOLV0001_RecordMemberRoundTrip` and `TestTOLV0019_QueueStatusLegacyIdentity` | none beyond the live row below |
| TOL-V0-005..008, 014, 015 | V1-1022 | `internal/tasks/mutation` (payloads, Apply), `internal/tasks/intent` (grants, policy key), `internal/tasks/transaction` (adopt/import guards, claim checks), `internal/tasks/cli` | `TestTOLV0005_SeedPrefixAndDuplicates`, `TestTOLV0006_ShowIsReadOnly`, `TestTOLV0007_SetTransitions`, `TestTOLV0008_DeclaredWitness`, `TestTOLV0014_RevisionOnlyWrite`, `TestTOLV0014_WitnessReplay` (CAS, replay, HELD admitted, COMPLETED refused), `TestTOLV0015_RoleMatrix`, `TestTOLV0015_WorkerStaleGenerationFenced` (same actor after reclamation) (`internal/tasks/cli`) | native archive round trip; two-process CAS; interrupted-commit redo (`NOT_RUN`) |
| TOL-V0-009..013 | V1-1022 | report reader `internal/tasks/obligation` (no Node dependency), secret screen, receipt audit (`internal/tasks/transaction/obligation_audit.go`) | `TestTOLV0009_ReportAdmissionAndRetention`, `TestTOLV0009_SubsetIgnoresExcludedMatchBound`, `TestTOLV0010_StepOwnErrorCredits`, `TestTOLV0010_Retry0Only`, `TestTOLV0011_ConflictingMatches`, `TestTOLV0012_SourcePresence`, `TestTOLV0013_CreditMismatchAndAudit`, `TestTOLV0013_AuditAfterReportDeleted`, `TestTOLV0013_DeclaredWitnessAudit`, `TestTOLV0013_DeclaredCommitAudited`, `TestTOLV0013_TamperedWorkerGenerationInconsistent` (`internal/tasks/cli`), all on synthetic json reports | live Playwright 1.63 fixture on a PWP-V0-008 tuple producing each case (`NOT_RUN`: Playwright is not installed on this host; the qualified version list stays empty) |
| TOL-V0-016..018, 021 | V1-1022 | `internal/tasks/dispatch` (roster, stall, ledger version, status), `internal/tasks/transaction/loop_detect.go` | `TestTOLV0016_HighWaterMonotone` (`internal/tasks/cli`); `TestTOLV0017_FingerprintLegacyIdentity`, `TestTOLV0017_LedgerChurnIsNotProgress`, `TestTOLV0017_HighWaterRaiseIsProgress`, `TestTOLV0021_StallRestartsOnRaise`, `TestTOLV0021_PreviousLedgerVersionAdopted` (`internal/tasks/dispatch`); `TestTOLV0018_LastRaiseEndsNoProgressRun` (`internal/tasks/transaction`) | live dispatcher run (`NOT_RUN`) |
| TOL-V0-019, 020 | V1-1022 | `internal/tasks/cli` (queue status, show, list, plan) | `TestTOLV0019_QueueStatusLegacyIdentity`, `TestTOLV0019_ObligationSummary`, `TestTOLV0020_PlanCheck` (UNASSIGNED, SPLIT, UNKNOWN_OBLIGATION, ALREADY_CLOSED) (`internal/tasks/cli`) | TOL-V0-019 plain-language status line (`NOT_RUN`: `queue status` has no plain output mode to carry it; JSON members only) |

Pre-design evidence (OBSERVED, non-qualifying): on 2026-10-08 a scratch run of Playwright 1.61.1
(not the qualified 1.63) with Node v22 and the built-in json reporter showed that `test.step`
entries carry their own `error`, that a passing sibling of a soft-failed step has none, that a
parent of a soft-failed nested step carries an error, that a hard failure omits later steps, that
a soft failure outside any step appears only in `result.errors`, that a retry-1 pass gives test
status `flaky`, and that `config.metadata` carries no git commit by default. The build-log entry
records it; it qualifies nothing.

## Resolved decisions

These are drafting choices, proposed for owner acceptance, not accepted decisions.

1. Storage by reference: the record carries counts and a head digest, and the ledger is a chain
   of at most 64 KiB events in the existing single MUTATE derived-event slot. 256 entries with
   evidence exceed the 128 KiB record bound, and a whole-ledger snapshot would exceed the slot, so
   each write posts only its delta and reads fold the chain.
2. Revision-only writes (TEA-V0-001/ON-V0-005 precedent): ledger writes never move
   `acceptanceRevision`, so gate results stay bound.
3. Retry 0 only, matching `fp-ledger.py add --step`; flaky passes never credit.
4. A step credits from its own `error` member alone; Playwright already propagates a nested
   soft failure to the parent's `error`.
5. All matches must pass; witness never infers `DEFECT`.
6. Commit binding by source presence, because the json reporter carries no commit by default.
   The event keeps each match's repository path so audit can repeat the check without the report.
7. Reuse closed result codes with stable detail prefixes (KHN-V0-022 pattern); no new code.
8. The fingerprint swaps revision for acceptance revision only for ledger tickets, so existing
   dispatch behaviour stays byte-identical.
9. Loop integration reads `lastRaise` from the record that `LoopHoldOf` already receives, instead of
   recording a value at claim time, which `lease_claim.go` would only write after the hold check.

## Unresolved decisions

Owner decisions required before acceptance:

1. Should `complete` (or a gate) require every core obligation `WITNESSED` at the current
   acceptance revision? V0 says no.
2. Retry policy: retry 0 only, or an explicitly declared per-ticket policy that admits retry N?
3. Grants: OPERATOR by explicit row and WORKER report witness by opt-in policy, or OWNER only?
4. Bounds: 256 obligations, 1,024 ledger events (a worst-case fold reads 64 MiB), 64 KiB events,
   64 MiB reports.
5. Whether HELD tickets admit ledger writes (proposed yes, unlike TEA-V0).
6. Whether to require report-embedded commit metadata (Playwright `captureGitInfo`) once qualified,
   in addition to the source presence check.
7. Whether the stall restart (TOL-V0-021) is worth a dispatcher ledger version, or should be
   dropped in favour of the fingerprint alone.
8. Whether the TOL-V0-017 fingerprint change should also drop the workState `State` for ledger
   tickets, so self-reported state stops counting as progress.

## Rollout and rollback

Rollout is additive and opt-in per ticket: a record without `obligations` keeps its exact legacy
bytes, a policy without the `obligations` key keeps canonical bytes, `queue status` without any
ledger is byte-identical, and the fingerprint of a ticket without a ledger is unchanged. Rollback
is reverting the change before any store holds a record with `obligations`. After such a write an
older binary refuses that record at the closed reader (fail-closed, no silent loss); recovery is to
run the newer binary or restore the store from a backup taken before the first seed. Stored
ledger events are inert evidence-store blobs and need no migration. A dispatcher that wrote the
TOL-V0-021 ledger version cannot be rolled back in place: an older build refuses it
`UNSUPPORTED_VERSION` (CAL-V0-132), so the operator stops the dispatcher and restores or removes
its state file first.
