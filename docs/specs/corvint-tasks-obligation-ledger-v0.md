# Corvint Tasks obligation ledger V0

Owner: Russell Lewis
Date: 2026-10-08
Intent status: proposed
Delivery status: not-started

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
- Status: proposed (owner issue #680, V1-1022; owner acceptance required); not-started; no code, store member or command exists.
- Exists: this proposal only; the reused mutation, evidence-store, fingerprint (CAL-V0-057), stall (CAL-V0-185) and loop (CAL-V0-102) paths.
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
  reference `{prefix, revision, head, counts, highWater}`. `prefix` is 1..16 bytes of `[A-Z0-9]`
  starting with a letter; `revision` is a Count of ledger writes (1..1024); `head` is the lowercase
  sha256 of the current ledger snapshot (TOL-V0-002); `counts` is `{witnessed, total, deferred,
  coreWitnessed, coreTotal}` and `highWater` is `{acceptanceRevision, witnessed}`, all canonical
  Count strings. `total` excludes `deferred` obligations. `obligations` MUST join
  `wire.TicketRecordOptionalKeys`, and the Core reader (`internal/taskman`) MUST admit and validate
  it. A record without the member keeps its exact legacy bytes.
- `TOL-V0-002`: The ledger content MUST be an immutable canonical snapshot document with profile
  `taskman-obligation-ledger/0` stored by digest in the existing evidence store before the receipt
  that references it, linked by `previous` (sha256 or null) to the prior head, exactly as ON-V0-002
  links note events. Its closed shape is `{profile, ticketId, prefix, revision, previous,
  obligations}`, with `obligations` sorted by `id` and at most 256 entries; the document is at most
  512 KiB. A snapshot no receipt references is orphaned and never CURRENT. The canonical JSON, Count
  and limit rules of `internal/tasks/wire` apply unchanged.
- `TOL-V0-003`: An obligation entry MUST be the closed object `{id, title, core, state, reason,
  evidence, updatedAt, updatedBy}`. `id` is `<prefix>-<1..6 decimal digits without leading zero>`;
  `title` is 1..256 bytes of nonblank prose; `core` is a boolean; `state` is one of `OPEN`,
  `WITNESSED`, `DEFECT`, `BLOCKED` or `DEFERRED`; `reason` is null or 1..512 bytes; `evidence` is
  null unless `state` is `WITNESSED`; `updatedAt` is the envelope `issuedAt`; `updatedBy` is the
  writing actor. A duplicate `id` within a snapshot refuses `DUPLICATE_ID`.
- `TOL-V0-004`: Witness evidence MUST be the closed object `{source, manifestSha256, testId,
  titlePath, stepTitle, stepPath, retry, commit, playwrightVersion, acceptanceRevision}`. `source`
  is `PLAYWRIGHT_REPORT` or `DECLARED`. For `PLAYWRIGHT_REPORT`, `manifestSha256` is the digest of
  the stored witness extract (TOL-V0-009) and `testId` is `<spec.id>@<projectName>`; `titlePath`
  and `stepPath` are ordered arrays (semantic order, not sorted); `stepTitle` is null for a
  test-level match; `retry` is the Count `0`. For `DECLARED`, `manifestSha256` is a caller-supplied
  sha256 of the manifest the declarer vouches for, the Playwright fields are null, and `testId` is
  1..256 bytes. `commit` is a 40- or 64-hex object name that MUST resolve to a commit in the
  repository at write time.
- `TOL-V0-005`: `corvint-tasks ticket obligations seed --target T --expected-revision N --payload P`
  MUST apply operation `OBLIGATIONS_SEED` with the closed payload `{prefix, obligations}` (1..256
  sorted unique `{id, title, core}`). The first seed declares `prefix`; it refuses `DUPLICATE_ID`
  when a non-archived native ticket already declares it. A later seed MUST repeat the same prefix
  (`MALFORMED` otherwise) and refuses `DUPLICATE_ID` for an id already present. Seeded entries are
  `OPEN`.
- `TOL-V0-006`: `corvint-tasks ticket obligations show --target T` is a read. It MUST print the
  current snapshot, the reference counts, the head and its receipt sequence, and MUST NOT write any
  state. A ticket without a ledger prints an empty result with exit 0. A head the evidence store
  cannot produce or that fails its digest is `UNKNOWN` with a typed diagnostic, never an empty
  ledger.
- `TOL-V0-007`: `corvint-tasks ticket obligations set` MUST apply operation `OBLIGATIONS_SET` with
  the closed payload `{changes}`: 1..64 entries `{id, state, core, reason}` sorted by unique `id`,
  where `state` is null or one of `OPEN`, `DEFECT`, `BLOCKED`, `DEFERRED`, `core` is null or a
  boolean, at least one is non-null and `reason` is required. `WITNESSED` is reachable only through the
  witness operation (`TOL-V0-008`). Demoting a `WITNESSED` entry clears its evidence and requires the reason. Changing
  `core` is OWNER-only. An unknown id refuses `MALFORMED` with the detail prefix
  `OBLIGATION_UNKNOWN:`.
- `TOL-V0-008`: `corvint-tasks ticket obligations witness` MUST apply operation
  `OBLIGATIONS_WITNESS` with the closed payload `{source, manifestSha256, commit, credits, reason}`.
  `credits` is 1..256 entries sorted by unique `id`, each `{id, testId, titlePath, stepTitle,
  stepPath}`; `reason` is required for `DECLARED` and null otherwise. A credited `OPEN`, `DEFECT` or
  `BLOCKED` entry becomes `WITNESSED` with TOL-V0-004 evidence. An entry already `WITNESSED` keeps
  its original evidence and is listed as `alreadyWitnessed`. A `DEFERRED` entry is not credited.
  `DECLARED` witness is OWNER-only by default.
- `TOL-V0-009`: `witness --from-playwright-report FILE --commit SHA` MUST read at most 64 MiB of
  one JSON document produced by Playwright's built-in `json` reporter and admit only the observed
  closed top-level shape (`config`, `errors`, `stats`, `suites`). `config.version` MUST be in the
  qualified version list (initially empty until the TOL-V0-020 live fixture passes on 1.63.x);
  otherwise the command refuses `UNSUPPORTED_VERSION` with the detail prefix
  `OBLIGATION_REPORT_VERSION_UNQUALIFIED:`. The report itself MUST NOT be retained, because results
  carry `stdout`, `stderr` and attachments. The writer derives a closed, secret-screened extract
  `taskman-obligation-witness/0` `{profile, ticketId, reportSha256, playwrightVersion, commit,
  matches}` (each match `{id, testId, titlePath, stepTitle, stepPath, project, retry, passed}`,
  sorted by canonical bytes, at most 4,096 matches), stores it by digest, and sets
  `manifestSha256` to the extract digest. A missing, unreadable, oversize, non-JSON or wrong-shape
  report refuses `MISSING_EVIDENCE` or `MALFORMED` with the detail prefix `OBLIGATION_REPORT:` and
  writes nothing.
- `TOL-V0-010`: Crediting MUST use only results with `retry` `0` of tests whose `expectedStatus` is
  `passed`. A step credits the ids in its own title exactly when that step object has no `error`
  member; a later sibling step's failure, a soft failure outside any step (`result.errors`) or the
  test's final status do not affect it, so independent `expect.soft` steps credit independently. A
  parent step whose nested step soft-failed carries its own `error` (observed on 1.61.1) and does
  not credit. An id in the title path (describe titles and the test title, not the file) credits
  only when the retry-0 result `status` is `passed`. A result with status `timedOut`,
  `interrupted` or `skipped` credits nothing. Ids match as whole tokens bounded by a non
  `[A-Za-z0-9-]` byte or the string edge.
- `TOL-V0-011`: Every match of an id in the report MUST pass for the id to be credited. If one
  match passes and another fails (another project, test or step), the id is listed as
  `conflicting` and not credited. An id matched only by failing results is listed as `failed`; an
  id of the ledger prefix that the ledger does not hold is listed as `unknown`. Witness never sets
  `DEFECT` automatically.
- `TOL-V0-012`: Each credited match MUST pass a source presence check: its `spec.file` (relative to
  `config.rootDir`) MUST map to a repository-relative path inside the repository, and the blob of
  that path at `--commit` MUST contain the id as a literal whole token. Otherwise the match does
  not credit and is listed as `unbound` with the reason `OUTSIDE_REPOSITORY`, `ABSENT_AT_COMMIT` or
  `ID_NOT_IN_SOURCE`. This is the only check that binds a report to the declared commit; the report
  itself carries no commit by default.
- `TOL-V0-013`: The writer MUST re-derive `credits` from the stored extract and refuse with outcome
  `VALIDATION_FAILED`, code `MALFORMED` and the detail prefix `OBLIGATION_CREDIT_MISMATCH:` when the
  payload differs. A witness that credits nothing writes no receipt and returns `written: false`
  with the `credited`, `alreadyWitnessed`, `conflicting`, `failed`, `unknown`, `unbound` and
  `unmatched` id lists (each bounded and sorted). `receipt audit` MUST re-derive each witness
  receipt's credits from its stored extract and report a mismatch or a missing extract as
  `INCONSISTENT`.
- `TOL-V0-014`: Every `OBLIGATIONS_*` write MUST be an ordinary receipt-backed mutation: the
  TM-V0-006 request-id replay, the `expectedRevision` CAS (required), Apply, finalize, journal
  append and fold. The record revision moves by 1 and `acceptanceRevision` does not move, because
  no acceptance-relevant member changes; gate results stay bound (TEA-V0-001 precedent). Each write
  moves `obligations.revision` by 1 and updates `head`, `counts` and `highWater`. Writes are admitted
  on OPEN and HELD native tickets; DRAFT, COMPLETED, ARCHIVED and IMPORT tickets refuse `BLOCKED`
  with `TICKET_STATE`. `ADOPT_FILE` and `IMPORT_APPLY` MUST refuse any difference in the member.
  A ledger write never satisfies a gate, an acceptance criterion or a completion in V0.
- `TOL-V0-015`: Roles: OWNER by default for all three operations. OPERATOR only through an
  explicit `policy.roles.OPERATOR` row (the `ExplicitGrantOperations` class). A WORKER MAY run
  `witness --from-playwright-report` only when the policy carries the optional closed key
  `obligations {workerWitness: true}` and, as in KHN-V0-022, the actor holds the ticket's live
  claim at the current generation (`FENCED` when stale, `PROVENANCE_UNVERIFIED` when the attempt
  inventory cannot be audited). A WORKER can never seed, set or declare. REVIEWER, IMPORTER and
  SYSTEM cannot be granted any `OBLIGATIONS_*` operation. Omitting the policy key keeps canonical
  policy bytes.
- `TOL-V0-016`: `highWater` MUST be monotone within one acceptance revision: after each write it
  is `max(previous witnessed, current witnessed)` when `highWater.acceptanceRevision` equals the
  record's, and resets to `{current acceptanceRevision, current witnessed}` otherwise. Demotions
  lower `counts.witnessed` but never `highWater` within the revision, so set-then-rewitness churn
  is not progress.
- `TOL-V0-017`: For a ticket carrying `obligations`, the CAL-V0-057 per-ticket fingerprint line
  MUST become `Status|AcceptanceRevision|State` followed by the line
  `obligations|<highWater.acceptanceRevision>|<highWater.witnessed>`; gate and attempt rows are
  unchanged. Ledger, note and attachment writes therefore stop counting as progress while a raised
  high-water mark counts. A ticket without the member MUST fingerprint byte-identically to today.
  The native observation MUST carry `dispatch.Ticket.Obligations` (the reference counts and high
  water, nil when absent), which a workState program cannot supply, and `dispatch status` MUST show
  `witnessed/total` for observed tickets that carry it. A CAL-V0-185 `stall` count MUST also restart
  when the observed `highWater.witnessed` rises.
- `TOL-V0-018`: While a policy carries CAL-V0-102 `loopDetection`, the claim that records an ended
  generation's `loopEvidence` MUST add the optional `witnessedHighWater` (Count, or absent when the
  ticket has no ledger). A HANDOFF generation whose value exceeds the previous generation's at the
  same acceptance revision is not a no-progress generation. An absent value keeps the existing
  CAL-V0-102 rule unchanged.
- `TOL-V0-019`: `queue status` and `queue status --summary` MUST add the key `obligations`
  `{tickets, witnessed, total, deferred, core: {witnessed, total}}` summed over OPEN and HELD native
  tickets that carry a ledger, present only when at least one does, so legacy output stays
  byte-identical; this amends the CAL-V0-167/CAL-V0-184 summary key set. `ticket show` and
  `ticket list` items MUST carry the optional `obligations` count object, and plain-language
  output MUST print `obligations: witnessed/total (core cw/ct)`. These are reads derived from the
  loaded records with no evidence-store or receipt scan.
- `TOL-V0-020`: `corvint-tasks ticket obligations plan --target T --plan FILE` is an optional
  read-only check of a closed plan document `taskman-obligation-plan/0` `{profile, ticketId,
  tests}`, each test `{test, project, obligations}` with 1..256 sorted unique ids, at most 4,096
  tests. It MUST report each `OPEN`, `DEFECT` or `BLOCKED` obligation with zero planned tests as
  `UNASSIGNED`, with two or more as `SPLIT` (with the test count and test list), and each planned
  id the ledger lacks or holds as `WITNESSED`/`DEFERRED` as `UNKNOWN_OBLIGATION` or `ALREADY_CLOSED`.
  It exits 0 only when every listed open obligation is assigned to exactly one planned test, and
  writes nothing.

## Failure modes and trust

| Failure | Effect without this profile | Required behaviour |
|---|---|---|
| Forged title: a report names an id in a test or step the repository never wrote | — | No credit unless the spec file at `--commit` literally contains the id (TOL-V0-012); the extract records the match as `unbound`. Titles alone are never trusted. |
| Hand-edited report JSON | — | The writer stores only a re-derivable extract and the report digest; it cannot prove the report came from a real run. Trust is bounded by the declared actor and role (TOL-V0-015); a qualified receipt path is a non-goal for V0. |
| Duplicate ids: seeded twice, or matched by several tests, steps or projects | — | Seed refuses `DUPLICATE_ID` (TOL-V0-003/005); a cross-ticket prefix collision refuses; an id matched by both passing and failing results is `conflicting` and uncredited (TOL-V0-011). |
| Report from a different commit | — | The source presence check at the declared commit (TOL-V0-012) refuses matches whose file or id is absent there; a commit the repository lacks refuses. A report run on an unrecorded working tree that happens to match is a retained limit (Unresolved decision 6). |
| Flaky retries | A pass at retry 1 looks like success | Only retry 0 credits (TOL-V0-010); a `flaky` test's retry-1 pass never credits. |
| Missing, truncated, oversize or foreign-format report | — | Refuses with `OBLIGATION_REPORT:` and writes nothing (TOL-V0-009). |
| Unqualified Playwright version | — | Refuses `UNSUPPORTED_VERSION` with `OBLIGATION_REPORT_VERSION_UNQUALIFIED:`. |
| Secrets in report output | — | The report is never stored; the extract carries titles, ids and booleans only and is secret-screened before storage; a detected secret refuses `SECRET_DETECTED`. |
| Ledger churn used to fake progress | Any revision write is progress today | The fingerprint uses the acceptance revision and the monotone high water (TOL-V0-016/017). |
| Evidence store missing a referenced snapshot | — | `show` and reads report `UNKNOWN`; receipt audit reports `INCONSISTENT`; nothing is treated as an empty ledger. |
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

Nothing is implemented, so every row is `NOT_RUN`. Test names below are the required witnesses,
marked `(PLANNED)` because none exists yet.

| Requirement | Ticket acceptance | Implementation boundary | Delivered evidence | Required integrated evidence (NOT_RUN) |
|---|---|---|---|---|
| TOL-V0-001..004 | V1-1022 | `internal/tasks/wire` (optional key), `internal/tasks/ticket` (record codec), `internal/taskman` (Core reader), evidence store | none | `TestTOLV0001_RecordMemberRoundTrip` (PLANNED), `TestTOLV0001_ReaderAdmitsObligations` (PLANNED), `TestTOLV0002_SnapshotCanonicalAndChained` (PLANNED), `TestTOLV0003_EntryCodecRefusals` (PLANNED), `TestTOLV0004_EvidenceCodec` (PLANNED); legacy byte-identity fixture |
| TOL-V0-005..008, 014, 015 | V1-1022 | `internal/tasks/mutation` (payloads, Apply), `internal/tasks/intent` (grants, policy key), `internal/tasks/transaction` (adopt/import guards, claim checks), `internal/tasks/cli` | none | `TestTOLV0005_SeedPrefixAndDuplicates` (PLANNED), `TestTOLV0006_ShowIsReadOnly` (PLANNED), `TestTOLV0007_SetTransitions` (PLANNED), `TestTOLV0008_DeclaredWitness` (PLANNED), `TestTOLV0014_RevisionOnlyWrite` (PLANNED), `TestTOLV0015_RoleMatrix` (PLANNED); native archive round trip; two-process CAS; interrupted-commit redo |
| TOL-V0-009..013 | V1-1022 | new report reader under `internal/tasks` (no Node dependency), secret screen, receipt audit | none | `TestTOLV0009_ReportAdmissionAndRetention` (PLANNED) (version, shape, size, no report or stdout retained, secret screen), `TestTOLV0010_StepOwnErrorCredits` (PLANNED) (soft sibling, nested parent, hard failure, soft outside steps), `TestTOLV0010_Retry0Only` (PLANNED), `TestTOLV0011_ConflictingMatches` (PLANNED), `TestTOLV0012_SourcePresence` (PLANNED) (forged title, wrong commit, outside repository), `TestTOLV0013_CreditMismatchAndAudit` (PLANNED); live Playwright 1.63 fixture on a PWP-V0-008 tuple producing each case |
| TOL-V0-016..018 | V1-1022 | `internal/tasks/dispatch` (roster, ledger, status), `internal/tasks/transaction/loop_detect.go` | none | `TestTOLV0016_HighWaterMonotone` (PLANNED), `TestTOLV0017_FingerprintLegacyIdentity` (PLANNED), `TestTOLV0017_LedgerChurnIsNotProgress` (PLANNED), `TestTOLV0017_HighWaterRaiseIsProgress` (PLANNED), `TestTOLV0017_StallRestartsOnWitness` (PLANNED), `TestTOLV0018_LoopEvidenceHighWater` (PLANNED) |
| TOL-V0-019, 020 | V1-1022 | `internal/tasks/cli` (queue status, show, list, plan) | none | `TestTOLV0019_QueueStatusLegacyIdentity` (PLANNED), `TestTOLV0019_ObligationSummary` (PLANNED), `TestTOLV0020_PlanCheck` (PLANNED) (UNASSIGNED, SPLIT, UNKNOWN_OBLIGATION, ALREADY_CLOSED) |

Pre-design evidence (OBSERVED, non-qualifying): on 2026-10-08 a scratch run of Playwright 1.61.1
(not the qualified 1.63) with Node v22 and the built-in json reporter showed that `test.step`
entries carry their own `error`, that a passing sibling of a soft-failed step has none, that a
parent of a soft-failed nested step carries an error, that a hard failure omits later steps, that
a soft failure outside any step appears only in `result.errors`, that a retry-1 pass gives test
status `flaky`, and that `config.metadata` carries no git commit by default. The build-log entry
records it; it qualifies nothing.

## Resolved decisions

These are drafting choices, proposed for owner acceptance, not accepted decisions.

1. Storage by reference: the record carries counts and a head digest; the ledger lives in the
   evidence store, because 256 entries with evidence exceed the 128 KiB record bound.
2. Revision-only writes (TEA-V0-001/ON-V0-005 precedent): ledger writes never move
   `acceptanceRevision`, so gate results stay bound.
3. Retry 0 only, matching `fp-ledger.py add --step`; flaky passes never credit.
4. A step credits from its own `error` member alone; Playwright already propagates a nested
   soft failure to the parent's `error`.
5. All matches must pass; witness never infers `DEFECT`.
6. Commit binding by source presence, because the json reporter carries no commit by default.
7. Reuse closed result codes with stable detail prefixes (KHN-V0-022 pattern); no new code.
8. The fingerprint swaps revision for acceptance revision only for ledger tickets, so existing
   dispatch behaviour stays byte-identical.

## Unresolved decisions

Owner decisions required before acceptance:

1. Should `complete` (or a gate) require every core obligation `WITNESSED` at the current
   acceptance revision? V0 says no.
2. Retry policy: retry 0 only, or an explicitly declared per-ticket policy that admits retry N?
3. Grants: OPERATOR by explicit row and WORKER report witness by opt-in policy, or OWNER only?
4. Bounds: 256 obligations, 1,024 ledger revisions, 512 KiB snapshots, 64 MiB reports.
5. Whether HELD tickets admit ledger writes (proposed yes, unlike TEA-V0).
6. Whether to require report-embedded commit metadata (Playwright `captureGitInfo`) once qualified,
   in addition to the source presence check.
7. Whether the TOL-V0-017 fingerprint change should also drop the workState `State` for ledger
   tickets, so self-reported state stops counting as progress.

## Rollout and rollback

Rollout is additive and opt-in per ticket: a record without `obligations` keeps its exact legacy
bytes, a policy without the `obligations` key keeps canonical bytes, `queue status` without any
ledger is byte-identical, and the fingerprint of a ticket without a ledger is unchanged. Rollback
is reverting the change before any store holds a record with `obligations`. After such a write an
older binary refuses that record at the closed reader (fail-closed, no silent loss); recovery is to
run the newer binary or restore the store from a backup taken before the first seed. Stored
snapshots and extracts are inert evidence-store blobs and need no migration.
