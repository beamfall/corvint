# Corvint Tasks obligation ledger V0

Owner: Russell Lewis
Date: 2026-10-08
Intent status: accepted (decision 0456; V1-1022)
Delivery status: experimental

Authoritative inputs: the owner request in GitHub beamfall/corvint#680, tracked as native ticket
V1-1022 ("native per-ticket obligation ledger with step-level Playwright witnessing"). This document
was agent-drafted. The owner accepted TOL-V0-001..021 as written in chat on 2026-10-08 (decision
0456; V1-1022), which also resolved the former Unresolved decisions by accepting the positions this
text takes. Acceptance settles intent only. The V1-1022 implementation is experimental (see the Agent digest),
and nothing may be promoted or advertised as delivered until its acceptance evidence exists.
The agent-drafted additions TOL-V0-022..027 (GitHub beamfall/corvint#712 and #713) are proposed,
pending owner acceptance; decision 0456 does not cover them. The profile reuses unchanged the ticket, mutation,
receipt and fold boundaries of the [agent lease contract](corvint-tasks-agent-leases-v0.md), the
reference-plus-evidence-event shape of [operator notes](corvint-tasks-operator-notes-v0.md)
(ON-V0-002), the revision-only write of [evidence attachments](corvint-tasks-evidence-attachments-v0.md)
(TEA-V0-001) and the WORKER opt-in of [know-how notes](corvint-tasks-know-how-notes-v0.md)
(KHN-V0-021..023). Playwright facts come from the
[Playwright provider contract](playwright-external-provider-v0.md) and a scratch observation recorded
in the build log.

## Agent digest
- Claim: A native ticket can carry a bounded ledger of named obligations that a Playwright json report witnesses step by step, so progress counts proof, not sessions.
- Status: TOL-V0-001..021 accepted (decision 0456; V1-1022; owner issue #680); TOL-V0-022..027 proposed, pending owner acceptance (GitHub #712, #713); experimental; TOL-V0-001..018 and 020..027 implemented with focused tests; the TOL-V0-019 plain-language line and the live Playwright 1.63 fixture are NOT_RUN.
- Exists: the experimental `ticket obligations seed|show|witness|set|plan` verbs, the read-only `preflight` verb (TOL-V0-025..027), the record member, the `internal/tasks/obligation` report reader, the receipt audit and the fingerprint, stall and loop integration; the reused mutation, evidence-store, fingerprint (CAL-V0-057), stall (CAL-V0-185) and loop (CAL-V0-102) paths.
- Blocked on: a live Playwright 1.63 fixture before any crediting code can qualify.
- Read next: User and current state; Requirements; Failure modes and trust; Resolved owner decisions.

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
- `TOL-V0-022`: A `test.fail` test MUST never credit, and an obligation it marks expected-fail MUST
  be reported `defectConfirmed`, not `failed`. Status: (proposed, pending owner acceptance; GitHub #712).
  - A test is a `test.fail` test when its `expectedStatus` is `failed` or the test or its retry-0
    result carries a `fail` annotation; the annotation's description is the fail description.
  - A step id is expected-fail when the step title says expected-fail (`expected-fail`,
    `expected fail`, `expected to fail`), or when the step failed with an error naming a defect id.
    A defect id is a whole token `[A-Z][A-Z0-9]{0,15}-N` whose prefix is not the ledger prefix;
    when the fail description names defect ids, only those count.
  - A title-path id is expected-fail when the title naming it says expected-fail, or the fail
    description or that title names a defect id.
  - An id whose every match is expected-fail and failed is reported in `defectConfirmed`
    `[{id, defect, error}]`: the defect id (the error's, else the title's, else the description's,
    else null) and the error's first non-blank line, with escapes and control characters removed,
    bounded to 240 bytes and withheld when it matches the secret screen (else null).
  - An expected-fail match that passed confirms nothing and the id is reported `failed`. An id
    with expected-fail and ordinary matches is `conflicting`.
  - Reporting is not recording: witness still never sets `DEFECT` (TOL-V0-011) and the event is
    unchanged, so the writer's TOL-V0-013 recomputation is unchanged.
- `TOL-V0-023`: Any other obligation named in a `test.fail` test (in its title path or a step,
  passing or failing) MUST be reported in `mixedExpectedFail` `[{id, tests, remedy}]` with the
  `<specId>@<project>` test keys and the remedy to move the expected-fail obligation into its own
  test. It is not credited, not `failed`, and other tests in the same report still credit.
  Status: (proposed, pending owner acceptance; GitHub #712).
- `TOL-V0-024`: `witness --from-playwright-report` MUST accept `--post-check LOG
  --post-check-status N` naming the run's post-check log and exit status (0..255). Both flags are
  given together and only with a report, else usage `ERROR`. A missing or unreadable log refuses
  `MISSING_EVIDENCE`; at most 1 MiB of it is read. A non-zero status MUST refuse `GATE_FAILED` with
  the detail prefix `OBLIGATION_POST_CHECK_FAILED:` and the log's first actionable line (the first
  non-blank line naming an error, failure, mismatch or violation or carrying a failure mark, else
  the first non-blank line, cleaned and screened as in TOL-V0-022), before any write, so nothing
  from that run is credited. A zero status credits as without the flags. The post-check is not
  recorded in the event. Status: (proposed, pending owner acceptance; GitHub #712).
- `TOL-V0-025`: `corvint-tasks preflight <ticket> [--commit OID] [--path PREFIX ...]` MUST check a
  ledger ticket's `OPEN`, `DEFECT` and `BLOCKED` obligations against the spec files (`*.spec.*`,
  `*.test.*` with a JavaScript or TypeScript extension, at most 4,096, optionally under `--path`)
  at `--commit` (default `HEAD`), read from Git objects with no checkout. Status: (proposed,
  pending owner acceptance; GitHub #713).
  - `UNNAMED`: no test title, describe title, `test.step` title or `<contract>:<id>` contract
    annotation in a string literal names the id. Comments do not name.
  - `MIXED_EXPECTED_FAIL` with `path` and `line` of the test declaration: a `test.fail` test
    (`test.fail(title, body)`, or a `test.fail(...)` call in the test, its describe or the file)
    names an obligation that is not expected-fail there. An id is expected-fail when its ledger
    state is `DEFECT`, its naming title says expected-fail, the fail description names it, or, for
    a test-title id, the fail description or test title names a defect id (TOL-V0-022).
  - `SPLIT_TESTS` with `path` and `line`: two or more tests in one file and describe scope each
    name exactly one obligation, and they name at least two distinct ones. `test.fail` tests and
    tests annotated `isolated` (an annotation type or tag `isolated`/`@isolated`, on the test or a
    describe) are exempt.
  - The scan is a heuristic tokenizer (comments, string, template and regular-expression literals),
    not a JavaScript parser; a spec file above 1 MiB is skipped with a warning.
  - Each finding is `{kind, id, path, line, detail, remedy}`. Any finding refuses with the item
    status `PREFLIGHT_FAILED`, `ok` false and a non-zero exit; otherwise the status is `PASSED`.
    A ticket without a ledger refuses. Preflight MUST write no queue state.
- `TOL-V0-026`: `preflight --plan PATH` MUST add the TOL-V0-020 findings of the plan document at
  `PATH` in the commit, with that path. A path that is not a file there is a `PLAN_MISSING`
  finding; an oversized, malformed or other-ticket plan refuses as `ticket obligations plan` does.
  Status: (proposed, pending owner acceptance; GitHub #713).
- `TOL-V0-027`: `preflight --deep --gate GATE ...` MUST run each named policy gate, under the lease
  runner's `COMMAND` gate rules (worktree cwd, expected exit, declared environment, timeout and
  output cap), in one temporary detached worktree of the commit that it removes before returning,
  so uncommitted state in the caller's checkout never runs. An undeclared gate refuses
  `GATE_UNKNOWN` and an unsupported one `UNSUPPORTED`, before any worktree is created; `--gate`
  without `--deep` or `--deep` without a gate is a usage `ERROR`. A gate that does not pass is a
  `DEEP_CHECK_FAILED` finding whose `id` is the gate and whose detail quotes its first actionable
  line. Gate runs record no gate result. Status: (proposed, pending owner acceptance; GitHub #713).

## Failure modes and trust

| Failure | Effect without this profile | Required behaviour |
|---|---|---|
| Forged title: a report names an id in a test or step the repository never wrote | — | No credit unless the spec file at `--commit` literally contains the id (TOL-V0-012); the command reports the id as `unbound` and no event stores it. Titles alone are never trusted. |
| Hand-edited report JSON, or a report whose failing matches are later disputed | — | The report is discarded after validation; the event keeps its digest and the passing, source-bound matches. Audit re-checks those against the commit but cannot prove the discarded report held no failing match or came from a real run. Trust is bounded by the declared actor and role (TOL-V0-015); a qualified receipt path is a non-goal for V0. |
| Duplicate ids: seeded twice, or matched by several tests, steps or projects | — | Seed refuses `DUPLICATE_ID` (TOL-V0-003/005); a cross-ticket prefix collision refuses; an id matched by both passing and failing results is `conflicting` and uncredited (TOL-V0-011). |
| Report from a different commit | — | The source presence check at the declared commit (TOL-V0-012) refuses matches whose file or id is absent there; a commit the repository lacks refuses. A report run on an unrecorded working tree that happens to match is a retained limit (owner decision 6, decision 0456). |
| Flaky retries | A pass at retry 1 looks like success | Only retry 0 credits (TOL-V0-010); a `flaky` test's retry-1 pass never credits. |
| Missing, truncated, oversize or foreign-format report | — | Refuses with `OBLIGATION_REPORT:` and writes nothing (TOL-V0-009). |
| Unqualified Playwright version | — | Refuses `UNSUPPORTED_VERSION` with `OBLIGATION_REPORT_VERSION_UNQUALIFIED:`. |
| Secrets in report output | — | The report is never stored; the WITNESS event carries ids, paths and titles only, and they are secret-screened before storage; a detected secret refuses `SECRET_DETECTED`. |
| Ledger churn used to fake progress | Any revision write is progress today | The fingerprint uses the acceptance revision and the monotone high water (TOL-V0-016/017). |
| Evidence store missing a ledger event, or a broken `previous` chain | — | `show` reports `UNKNOWN`; receipt audit reports `INCONSISTENT`; nothing is treated as an empty or partial ledger. Counts on the record stay readable. |
| Witness or seed larger than the 64 KiB derived-event slot | — | Refuses `LIMIT_EXCEEDED` (`OBLIGATION_EVENT_TOO_LARGE:`) and writes nothing; the caller splits with `--ids` or several seeds (TOL-V0-002/009). |
| A stale WORKER process witnesses after reclamation | — | The payload names attempt and generation; a non-current generation refuses `FENCED` (TOL-V0-015). |
| Concurrent writers | — | The required CAS refuses the stale writer `REVISION_CONFLICT`; the request-id replay makes retries idempotent. |
| `test.fail` test that names an obligation | A failing expected-fail test reads as a failed obligation, and an ordinary obligation inside it can never pass | Expected-fail obligations are reported `defectConfirmed` with the defect id and error line and never credit (TOL-V0-022); any other obligation in the test is reported `mixedExpectedFail` with the remedy (TOL-V0-023); `preflight` flags it at file and line before the run (TOL-V0-025). |
| The run's post-check (lint, contract or consolidation script) failed | A passing report credits anyway | `--post-check` with a non-zero status refuses `GATE_FAILED` (`OBLIGATION_POST_CHECK_FAILED:`) quoting the log's first actionable line, and writes nothing (TOL-V0-024). |
| Obligations unnamed or split across tests, discovered only after a long run | A whole run is spent before the gap shows | `preflight` refuses `PREFLIGHT_FAILED` with each `UNNAMED`, `MIXED_EXPECTED_FAIL`, `SPLIT_TESTS`, plan and `--deep` gate finding and its remedy, reading the commit's Git objects and writing no queue state (TOL-V0-025..027). The scan is heuristic: a name built only by `${}` interpolation or outside a string literal is not seen. |
| `preflight --deep` interrupted | — | The temporary detached worktree and its administrative entry may remain; `git worktree prune` repairs it. A failed removal is reported, not ignored. |
| Older binary meets a record with the member | — | The closed reader refuses the record (fail-closed); see Rollout and rollback. |

## Non-goals and simpler baseline

- Gating completion, gates or acceptance on obligations. V0 only records and reports; whether
  `complete` should require every core obligation `WITNESSED` was resolved no in V0 (owner decision 1,
  decision 0456).
- Running tests, launching Playwright or editing reports. Corvint consumes a finished report only.
- Consuming PWP-V0 qualified receipts. The `/2` redaction profile retains steps, so a later
  `VERIFIED_RECEIPT` source could credit from a receipt Corvint itself produced; it is excluded here.
- Playwright tags or annotations as id carriers in the report; other test frameworks; the blob or
  html reporters. (Proposed TOL-V0-025 reads `<contract>:<id>` string literals and an `isolated`
  annotation from spec source for preflight only; they never credit.)
- The `fp-ledger.py` `splits` heuristic (same spec, describe and project without an `isolated`
  annotation) as a witness or plan rule. The plan check reports SPLIT per obligation instead;
  proposed TOL-V0-025 reports a source-level `SPLIT_TESTS` finding in `preflight` only.
- Running preflight automatically at pool acquire or claim. That hook is a follow-up (GitHub #713);
  V0 offers the explicit read only.
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
| TOL-V0-022..024 (proposed) | GitHub #712 | `internal/tasks/obligation` (report reader, `expected_fail.go`), `internal/tasks/cli` (witness flags and lists) | `TestTOLV0022_ExpectedFailDefectConfirmed`, `TestTOLV0023_MixedExpectedFail`, `TestTOLV0024_PostCheckRefusesCredit` (`internal/tasks/cli`), on synthetic json reports | live Playwright 1.63 `test.fail` fixture (`NOT_RUN`) |
| TOL-V0-025..027 (proposed) | GitHub #713 | `internal/tasks/obligation/source.go` (scanner), `internal/tasks/store/preflight.go` (Git reads, deep worktree), `internal/tasks/cli/preflight.go` | `TestTOLV0025_ScannerSkipsCommentsRegexAndInterpolation` (`internal/tasks/obligation`); `TestTOLV0025_PreflightRefusesUnnamedMixedAndSplit`, `TestTOLV0025_PreflightCleanTicketPasses`, `TestTOLV0026_PreflightPlanCheck`, `TestTOLV0027_PreflightDeepRunsGatesInCleanWorktree` (`internal/tasks/cli`) | preflight against a real Playwright suite (`NOT_RUN`) |
| TOL-V0-019, 020 | V1-1022 | `internal/tasks/cli` (queue status, show, list, plan) | `TestTOLV0019_QueueStatusLegacyIdentity`, `TestTOLV0019_ObligationSummary`, `TestTOLV0020_PlanCheck` (UNASSIGNED, SPLIT, UNKNOWN_OBLIGATION, ALREADY_CLOSED) (`internal/tasks/cli`) | TOL-V0-019 plain-language status line (`NOT_RUN`: `queue status` has no plain output mode to carry it; JSON members only) |

Pre-design evidence (OBSERVED, non-qualifying): on 2026-10-08 a scratch run of Playwright 1.61.1
(not the qualified 1.63) with Node v22 and the built-in json reporter showed that `test.step`
entries carry their own `error`, that a passing sibling of a soft-failed step has none, that a
parent of a soft-failed nested step carries an error, that a hard failure omits later steps, that
a soft failure outside any step appears only in `result.errors`, that a retry-1 pass gives test
status `flaky`, and that `config.metadata` carries no git commit by default. The build-log entry
records it; it qualifies nothing.

## Resolved decisions

These drafting choices were accepted by the owner with the requirements (decision 0456).

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

## Resolved owner decisions

Resolved by the owner on 2026-10-08 (decision 0456; V1-1022) by accepting the positions this spec takes:

1. `complete` and gates do NOT require every core obligation `WITNESSED` in V0.
2. Retry 0 only (TOL-V0-010); no per-ticket retry policy.
3. OWNER by default; OPERATOR only through an explicit `policy.roles.OPERATOR` row; a WORKER report
   witness only by opt-in policy.
4. Bounds as stated: 256 obligations, 1,024 ledger events (a worst-case fold reads 64 MiB), 64 KiB
   events, 64 MiB reports.
5. HELD tickets admit ledger writes (unlike TEA-V0).
6. Report-embedded commit metadata (Playwright `captureGitInfo`) is not required in V0; the source
   presence check alone binds the commit.
7. The TOL-V0-021 stall restart and its dispatcher ledger version are kept, with the rollback limit
   recorded under Rollout and rollback.
8. The workState `State` stays in the TOL-V0-017 fingerprint; it is not dropped for ledger tickets.

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

The proposed TOL-V0-022..027 add no stored state: the `defectConfirmed` and `mixedExpectedFail` lists and the
`preflight` verb are read-side output, and the witness event is unchanged, so rolling them back is
reverting the change.
