# Corvint Tasks obligation ledger V0

Owner: Russell Lewis
Date: 2026-10-08
Intent status: accepted (decisions 0456, 0484, 0485 and 0519; V1-1022)
Delivery status: experimental

Authoritative inputs: the owner request in GitHub beamfall/corvint#680, tracked as native ticket
V1-1022 ("native per-ticket obligation ledger with step-level Playwright witnessing"). This document
was agent-drafted. The owner accepted TOL-V0-001..021 as written in chat on 2026-10-08 (decision
0456; V1-1022), which also resolved the former Unresolved decisions by accepting the positions this
text takes. Acceptance settles intent only. The V1-1022 implementation is experimental (see the Agent digest),
and nothing may be promoted or advertised as delivered until its acceptance evidence exists.
The agent-drafted additions TOL-V0-022..027 (GitHub beamfall/corvint#712 and #713) were accepted
as written in chat on 2026-10-10 (decision 0484). The agent-drafted TOL-V0-028..033 (GitHub
beamfall/corvint#714) were accepted on 2026-10-10, as amended by the first independent review round
(decision 0485, recorded from the coordinating agent's relay of the owner's chat approval), and are
experimental. The agent-drafted TOL-V0-034..036 (adopter obligation aliases, GitHub
beamfall/corvint#792, V1-1177) were accepted on 2026-10-11 under the owner's standing auto-accept
(decision 0519), and are experimental. The agent-drafted TOL-V0-037..046 (per-obligation review
sources and an auditable "none found", GitHub beamfall/corvint#813) were accepted on 2026-10-11
under the owner's standing auto-accept, and are experimental. The agent-drafted TOL-V0-047..050
(a private preflight result cache, GitHub beamfall/corvint#1037, V1-1594) were accepted on
2026-10-17 under the owner's standing auto-accept, and are experimental. The agent-drafted TOL-V0-051..054
(an opt-in failure-signature ledger that refuses identical re-runs, GitHub beamfall/corvint#1048, V1-1605) were accepted on 2026-10-17 under the same standing auto-accept,
and are experimental. The agent-drafted TOL-V0-055..059 (ledger migration blockers: a 128-byte
alias, plan reruns and non-core rows, a `PARTIAL` state and witness provenance, GitHub
beamfall/corvint#1071, V1-1649) were accepted on 2026-10-18 under the same standing auto-accept,
and are experimental. The agent-drafted TOL-V0-060..067 (local Playwright version qualification
and declared test repositories, GitHub beamfall/corvint#1070, V1-1648) were accepted on 2026-10-18
under the same standing auto-accept, and are experimental. The profile reuses unchanged the ticket,
mutation, receipt and fold boundaries of the [agent lease contract](corvint-tasks-agent-leases-v0.md), the
reference-plus-evidence-event shape of [operator notes](corvint-tasks-operator-notes-v0.md)
(ON-V0-002), the revision-only write of [evidence attachments](corvint-tasks-evidence-attachments-v0.md)
(TEA-V0-001) and the WORKER opt-in of [know-how notes](corvint-tasks-know-how-notes-v0.md)
(KHN-V0-021..023). Playwright facts come from the
[Playwright provider contract](playwright-external-provider-v0.md) and a scratch observation recorded
in the build log.

## Agent digest
- Claim: A native ticket can carry a bounded ledger of named obligations that a Playwright json report witnesses step by step, so progress counts proof, not sessions.
- Status: accepted (decisions 0456, 0484, 0485 and 0519; V1-1022): TOL-V0-001..021 by decision 0456 (owner issue #680), TOL-V0-022..027 by decision 0484 (GitHub #712, #713) and TOL-V0-028..033 by decision 0485 (GitHub #714); TOL-V0-034..036 (obligation aliases, GitHub #792, V1-1177) by decision 0519; TOL-V0-037..046 (review sources, GitHub #813) accepted under the owner's standing auto-accept (2026-10-11), implemented with focused tests; TOL-V0-047..050 (preflight result cache, GitHub #1037, V1-1594) accepted under the same standing auto-accept, implemented with focused tests; TOL-V0-051..054 (failure-signature ledger, GitHub #1048, V1-1605) accepted under the same standing auto-accept, implemented with focused tests; TOL-V0-055..059 (migration blockers, GitHub #1071, V1-1649) accepted under the same standing auto-accept, implemented with focused tests; TOL-V0-060..067 (version qualification and test repositories, GitHub #1070, V1-1648) accepted under the same standing auto-accept, implemented with focused tests; TOL-V0-079..083 (test boundaries and scope decisions, GitHub #1180, V1-1913) and TOL-V0-084..088 (reviewed-shortfall worklist, GitHub #1182, V1-1915) accepted under the same standing auto-accept, implemented with focused tests; TOL-V0-100..109 (ledger gate hardening, GitHub #1202, V1-1945, V1-1946, V1-1949) accepted under the same standing auto-accept, implemented with focused tests; TOL-V0-120..125 (obligation mirrors and the #1177 unblock follow-up, GitHub #1214, V1-1966) accepted under the same standing auto-accept, implemented with focused tests; TOL-V0-126..128 (live fixture qualification, V1-1022) accepted under the owner's standing auto-accept (2026-10-11), implemented with focused tests and a retained live run; TOL-V0-131 (review staleness at the candidate, V1-1922) and TOL-V0-132 (plan witnesses at the candidate, V1-2248) accepted under the same standing auto-accept, implemented with focused tests; TOL-V0-133..140 (witness receipt store, GitHub #1837, V1-2731) accepted under the same standing auto-accept, implemented with focused tests; experimental; TOL-V0-001..018 and 020..036 implemented with focused tests (TOL-V0-028..033 on a fake test command); the TOL-V0-019 plain-language line is NOT_RUN; the live Playwright 1.63.0 fixture passed on a PWP-V0-008 tuple (TOL-V0-128).
- Exists: the experimental `ticket obligations seed|show|witness|set|plan` verbs, the read-only `preflight` verb (TOL-V0-025..027), the experimental `run-batch` and `qualify` verbs (TOL-V0-028..033), the read-only `boundary`, `worklist` and `capability` verbs (TOL-V0-083, 086..088, 108), `satisfiedBy` mirrors with their show, status, worklist and dispatch reads (TOL-V0-120..123), the record member, the `internal/tasks/obligation` report reader, the receipt audit and the fingerprint, stall and loop integration; the witness receipt store with `witness --retain`, `receipt put|get|path|prune` and `receipt audit --witness-receipts` (TOL-V0-133..140); the reused mutation, evidence-store, fingerprint (CAL-V0-057), stall (CAL-V0-185) and loop (CAL-V0-102) paths.
- Blocked on: nothing for Playwright 1.63.0, the shipped qualified version since its live fixture run was retained (TOL-V0-126..128); any other version still needs a local `qualify-version` run (TOL-V0-060..063).
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
  validate it. A record without the member keeps its exact legacy bytes. Both readers MUST refuse
  the member on an IMPORT record and on a shadow overlay, the Tasks codec as `MALFORMED` at
  `/obligations` and the Core reader with the reason `obligations: only a NATIVE record carries an
  obligation ledger` (V1-1035; amendment accepted under the owner's standing auto-accept, 2026-10-11).
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
  - `state` is null or one of `OPEN`, `DEFECT`, `BLOCKED`, `DEFERRED` (and `PARTIAL`,
    TOL-V0-058). `core` is null or a boolean. At least one of them is non-null, and `reason` is
    required.
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
    fixture in Acceptance evidence passes on a 1.63.x PWP-V0-008 tuple; it now holds `1.63.0`
    (TOL-V0-126). Otherwise the command
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
  unchanged. TOL-V0-124 extends this with an unblock line.
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
  it writes nothing. TOL-V0-056 and TOL-V0-057 amend which obligations need a planned test and add
  the optional `rerun` member.
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
  be reported `defectConfirmed`, not `failed`. Status: (accepted, decision 0484; GitHub #712).
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
  Status: (accepted, decision 0484; GitHub #712).
- `TOL-V0-024`: `witness --from-playwright-report` MUST accept `--post-check LOG
  --post-check-status N` naming the run's post-check log and exit status (0..255). Both flags are
  given together and only with a report, else usage `ERROR`. A missing or unreadable log refuses
  `MISSING_EVIDENCE`; at most 1 MiB of it is read. A non-zero status MUST refuse `GATE_FAILED` with
  the detail prefix `OBLIGATION_POST_CHECK_FAILED:` and the log's first actionable line (the first
  non-blank line naming an error, failure, mismatch or violation or carrying a failure mark, else
  the first non-blank line, cleaned and screened as in TOL-V0-022), before any write, so nothing
  from that run is credited. A zero status credits as without the flags. The post-check is not
  recorded in the event. Status: (accepted, decision 0484; GitHub #712).
- `TOL-V0-025`: `corvint-tasks preflight <ticket> [--commit OID] [--path PREFIX ...]` MUST check a
  ledger ticket's `OPEN`, `DEFECT` and `BLOCKED` obligations against the spec files (`*.spec.*`,
  `*.test.*` with a JavaScript or TypeScript extension, at most 4,096, optionally under `--path`)
  at `--commit` (default `HEAD`), read from Git objects with no checkout. Status: (accepted,
  decision 0484; GitHub #713).
  - The tree listing is streamed and restricted by `--path` (literal pathspecs), so Git lists
    nothing outside the prefixes; more than 4,096 spec files, or one path above 64 KiB, refuses
    `LIMIT_EXCEEDED` while it is read. Object sizes are read before content, so the content of a
    blob above 1 MiB is never requested.
  - Git's object queries are line-framed, so a spec file whose path holds a line feed or carriage
    return is never asked: it is an `UNREADABLE_SPEC_PATH` finding with that `path`, and the other
    files' answers are unaffected. `--path` and `--plan` values with a line break are a usage
    `ERROR`.
  - A test names the ids in its title path (its describe titles and its own title) and its
    `test.step` titles, as runtime witnessing reads them.
  - `UNNAMED`: no test title, describe title, `test.step` title or `<contract>:<id>` contract
    annotation in a string literal names the id. Comments do not name.
  - `MIXED_EXPECTED_FAIL` with `path` and `line` of the test declaration: a `test.fail` test
    (`test.fail(title, body)`, or a `test.fail(...)` call in the test, its describe or the file)
    names an obligation that is not expected-fail there. An id is expected-fail when its ledger
    state is `DEFECT`, its naming title says expected-fail, the fail description names it, or, for
    a title-path id, the fail description or that title names a defect id (TOL-V0-022).
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
  Status: (accepted, decision 0484; GitHub #713).
- `TOL-V0-027`: `preflight --deep --gate GATE ...` MUST run each named policy gate, under the lease
  runner's `COMMAND` gate rules (worktree cwd, expected exit, declared environment, timeout and
  output cap), in one temporary detached worktree of the commit that it removes before returning, so
  uncommitted state in the caller's checkout never runs. Every Git command preflight runs disables
  repository hooks and the file-system monitor. After each gate the worktree MUST still be a clean
  checkout of the commit (HEAD, HEAD tree, no change or untracked file), as the lease runner
  requires; otherwise that gate is a `DEEP_CHECK_FAILED` finding naming the change and no later gate
  runs. The status is streamed and read only to its first entry. Reading the worktree can run
  repository programs (a clean filter on a file the gate touched), so each post-gate Git command
  runs in its own process group, which is killed when the command ends, on `SIGINT`/`SIGTERM`, or
  when the gate's declared timeout passes, always before the command is reaped, so its group id
  cannot have been reused; a check that does not finish within that timeout is a `DEEP_CHECK_FAILED`
  finding, never a pass, and no later gate runs. Each gate runs in its own process group, which is
  killed when the gate exits, so nothing it left running outlives it. `SIGINT` or `SIGTERM` kills
  the running gate's process group, waits for it, removes the worktree and refuses `UNSUPPORTED`;
  `SIGKILL` can leave the worktree, which `git worktree prune` repairs. A process that leaves its
  gate's process group (for example with `setsid`) is not tracked, and creating the worktree (its
  checkout, including smudge filters) is not bounded. An undeclared gate refuses `GATE_UNKNOWN` and
  an unsupported one `UNSUPPORTED`, before any worktree is created; `--gate` without `--deep` or
  `--deep` without a gate is a usage `ERROR`. A gate that does not pass is a `DEEP_CHECK_FAILED`
  finding whose `id` is the gate and whose detail quotes its first actionable line. Gate runs record
  no gate result.
  Status: (accepted, decision 0484; GitHub #713).
- `TOL-V0-028`: `run-batch` and `qualify` MUST read their commands from one closed local
  configuration file named by `--config`: a single JSON object of schema
  `corvint-tasks-run-config/0` of at most 64 KiB with exactly the members `testCommand` (required
  argv), `prepCommand`, `postCheck`, `captureCommand`, `staticChecks` (a list of argv),
  `runs` (1..20), `neighbours` (`NONE` or `CHANGED_DIRECTORIES`), `fixturePaths`
  (repository-relative), `env` (variable names) and `timeoutSeconds` (1..3600). An unknown member, a
  second JSON value, an empty or NUL argument, an absolute or `..` fixture path or a value out of
  range refuses VALIDATION_FAILED `MALFORMED` (an oversized file `LIMIT_EXCEEDED`) before anything
  runs. Every command runs as argv, never through a shell, under the lease runner's `COMMAND` gate
  rules (declared environment only, timeout, output cap, its own process group killed when it
  exits), with `CORVINT_RUN_DIR`, `CORVINT_POOL_ID` and `CORVINT_POOL_MEMBER` added; the test
  command also receives `PLAYWRIGHT_JSON_OUTPUT_NAME` and the resolved spec files as trailing
  arguments. Status: (accepted, decision 0485; GitHub #714).
- `TOL-V0-029`: `run-batch <ticket> --config FILE --spec PATH ... --out DIR --request-id ID` MUST run
  one batch with no agent session: optional prep, the test command once on the spec files under the
  `--spec` paths at the commit, the optional post-check, then witness the report through the
  existing `ticket obligations witness` path (TOL-V0-008..015, 022..024 unchanged, including
  `--ids`, `--role`, the post-check refusal and the qualified-version list). It MUST write
  `<DIR>/<ID>/summary.json` (schema `corvint-tasks-batch-summary/0`) with the commit, the fixture
  digest, the config and report digests, each step's class, exit code, first actionable error line
  and log, the witness outcome, and one cause per ledger entry: `WITNESSED`, `ALREADY_WITNESSED`,
  `FAILED` with the failing step's or test's error line, `NOT_REACHED` with the error of the failed
  test that names the obligation in source (or, when no admissible report exists, of the failed
  earlier step), `NOT_NAMED` (no source test of the batch names it), `NOT_OBSERVED`,
  `DEFECT_CONFIRMED`, `UNCREDITED` with the reason, `DEFERRED` or `OUT_OF_SCOPE`. A job without an
  admissible report refuses `GATE_FAILED` and writes nothing to the store; a refused witness keeps
  its own outcome and codes. The output is untrusted and not retryable; reusing a request id
  refuses `REQUEST_ID_CONFLICT`, so earlier evidence is never overwritten.
  Status: (accepted, decision 0485; GitHub #714).
- `TOL-V0-030`: A job MUST run only in a clean checkout of the commit: a checkout at another commit
  refuses `STALE_TREE` and tracked changes refuse `DIRTY_WORKTREE`, before the results directory is
  created. With `--pool P --attempt A --generation G` the job acquires a member for the caller's
  claimed attempt through the existing `pool acquire` verb (its refusal code is the job's), runs
  the optional capture command so server-side errors land in `capture.log` before the lane is reset,
  and returns the allocation with `pool release`, on every path after a successful acquire. The
  capture and the release run on a fresh context, not the job's, so an interrupted job still
  captures and releases; the capture stays bounded by `timeoutSeconds`. A refused release fails the
  job: `run-batch` witnesses nothing, records `UNCREDITED` and refuses with the release's code and
  `LANE_FAILED:`, and `qualify` is `NOT_QUALIFIED` with `LANE_FAILED`. A `qualify` verdict that is
  `NOT_QUALIFIED` after a refused acquire or release refuses with that refusal's code (for example
  `RESOURCE_COLLISION`), not `GATE_FAILED`. The claim, renewal and HANDOFF stay with the caller. Status: (accepted, decision 0485; GitHub #714).
- `TOL-V0-031`: Before any lane is acquired or command run, `run-batch` MUST refuse `LOOP_DETECTED`
  with `OBLIGATION_REPEAT_FAILURE:` naming each in-scope `OPEN`, `DEFECT` or `BLOCKED` obligation
  that at least two earlier summaries of the same ticket under `--out` record as `FAILED` with the
  same error line at the same fixture digest. The fixture digest is the sha256 of the Git object
  id at the commit of each configured fixture path and each `--spec` path (the root tree for `.`,
  `ABSENT` when missing),
  so changing a fixture or spec is new evidence that admits the job. Leaving the obligation out
  with `--ids` also admits it. More than 1,024 entries under `--out`, a summary over 4 MiB or a
  malformed summary refuses. Each earlier summary is read only as a regular file, through the one
  descriptor its type and size were checked on and with at most 4 MiB plus one byte read, so a
  FIFO or symlink swapped in, or a file that grows after the check, can neither block the job nor
  bypass the bound. Status: (accepted, decision 0485; GitHub #714).
- `TOL-V0-032`: `qualify --config FILE --spec PATH ... --base REV --out DIR --request-id ID` MUST run
  the static checks first and without a lane, then on the lane prep, the candidate specs `runs`
  times, the post-check and, with `CHANGED_DIRECTORIES`, every other spec under a directory of a
  path changed between base and commit (a change at the repository root selects none), stopping at
  the first reason. Its verdict is `NOT_QUALIFIED` with every reason named: `STATIC_CHECK_FAILED`,
  `RETRY` (any candidate or neighbour result with retry above 0, even one whose failure the base
  shares), `CANDIDATE_FAILURE` (a retry-0 result that is not the
  expected status, a top-level report error, no tests, a non-zero exit or no admissible report),
  `POST_CHECK_FAILED`, `NEW_NEIGHBOUR_FAILURE` (a new neighbour test failure, or any top-level
  error in the neighbour report whether or not a test failed, any abnormal end of the neighbour
  run, meaning `TIMEOUT`, `SIGNAL`, `OUTPUT_LIMIT`, `SPAWN_FAILED` or `UNKNOWN`, whatever its tests
  show, or a non-zero exit with no failing test or error to explain it) or `LANE_FAILED`; otherwise
  `QUALIFIED`. Each failing
  neighbour test MUST be re-run at the base in one temporary detached worktree, removed on every
  path; a test that fails there too is `PRE_EXISTING` and does not disqualify, and one the base did
  not observe is new. A base run that ended abnormally observes nothing. `NOT_QUALIFIED` refuses `GATE_FAILED` with `QUALIFY_NOT_QUALIFIED:`.
  `qualify` never credits obligations or writes the store. Status: (accepted, decision 0485;
  GitHub #714).
- `TOL-V0-033`: `qualify` MUST write `<DIR>/<ID>/verdict.json` (schema
  `corvint-tasks-qualify-verdict/0`) with the commit, base, config digest, steps, runs, neighbour
  results and reasons, and then `manifest.json` listing every other entry of the pack with its type.
  A regular file is hashed as a stream, so no file is read whole into memory, and listed with its
  sha256 and size; one larger than 1 GiB (`maxManifestFileBytes`) is `FILE_OVER_LIMIT` with its size
  and no digest. A `SYMLINK`, `FIFO`, `DEVICE`, `SOCKET` or other entry is listed by type with no
  digest and is never followed or opened. An interrupt is checked after each file's hash and
  again just before the manifest is published, and leaves the verdict without a manifest, refusing `GATE_FAILED` with `LANE_FAILED:`. A job reads the test
  command's report, and the witness reads it and the post-check log again, only as regular files:
  the path is refused from `lstat` when it is not one, and opened without following a final
  symlink and without blocking on a FIFO where the platform allows, so a FIFO or symlink at the
  report path is no admissible report (`CANDIDATE_FAILURE`, or `UNCREDITED` for `run-batch`) and
  the capture and release still run. The witness also refuses `GATE_FAILED` with `LANE_FAILED:`
  when the report's digest is no longer the one the test run produced. Every file a job writes
  (`summary.json`, `verdict.json`, `manifest.json` and the logs) is created exclusively inside the
  results directory, never following or replacing an existing entry; a reserved name a command
  planted, or a failed write, fails the job with `LANE_FAILED:` before anything is credited or
  concluded, and the planted entry is left unchanged. `SIGINT` or `SIGTERM`
  kills the running command's process group, still runs the capture and releases the lane
  (TOL-V0-030), and records `LANE_FAILED: interrupted`; `run-batch` then credits nothing
  (`UNCREDITED`). The interrupt boundary for `run-batch` is the witness submit: the job context is
  checked after the report and its source evidence are read and before the witness mutation is
  submitted, and an interrupt seen by then credits nothing and refuses `GATE_FAILED` with
  `LANE_FAILED: interrupted before the witness was submitted`, even when the witness would have
  credited nothing; once submitted, the witness
  mutation is atomic and is not split by a later interrupt. A checkout that changed during the job
  is `LANE_FAILED` for `qualify` and credits nothing for `run-batch`. Status: (accepted, decision
  0485; GitHub #714).
- `TOL-V0-034`: A seed item MAY carry an optional adopter `alias`, so a suite that already names
  its obligations in another scheme (`M001`, `ABC-12.V4`) does not have to rename every test title.
  Status: accepted (decision 0519; V1-1177; GitHub #792).
  - The seed item becomes `{id, alias?, title, core}` and the folded entry `{id, alias?, title,
    core, state, reason, evidence, updatedAt, updatedBy}`. A seed item or entry without `alias`
    keeps its exact TOL-V0-003/005 bytes, and the member is absent rather than null.
  - An alias is 1..64 bytes of `[A-Za-z0-9._:-]` whose first and last bytes are letters or digits
    (raised to 1..128 bytes by TOL-V0-055).
    Leading zeros are allowed (`M001`). Anything else refuses `MALFORMED`. Requiring a letter or
    digit at the ends keeps the trailing-punctuation rule of TOL-V0-036 unambiguous.
  - The alias is fixed by the seed that creates the entry. `set`, `--ids`, the plan document
    (TOL-V0-020/026) and every id list in output stay native-id only. The native id grammar and the
    whole-token rule of TOL-V0-010 are unchanged.
  - The alias is secret-screened with the seed's titles before storage (TOL-V0-009 screen).
- `TOL-V0-035`: A seed MUST refuse, and write nothing, when an alias repeats an alias already in
  the ledger or earlier in the same payload (`DUPLICATE_ID`, detail prefix
  `OBLIGATION_ALIAS_DUPLICATE:`), or when it holds any valid native id of the ledger prefix as a
  TOL-V0-010 whole token, including the alias being a native id (`DUPLICATE_ID`, detail prefix
  `OBLIGATION_ALIAS_COLLISION:`). The collision check covers ids no seed has added yet, so a later
  seed can never make an alias ambiguous, and native-id matching never fires inside an alias.
  Aliases are scoped to their ledger: two tickets may use the same alias. Status: accepted (decision
  0519; V1-1177; GitHub #792).
- `TOL-V0-036`: Wherever TOL-V0-010..013 and 022..029 find an obligation named in a report title,
  a `test.step` title, a spec source or a fail description, an alias MUST name its entry exactly as
  the native id does, and every result MUST be keyed to the native id. Status: accepted (decision
  0519; V1-1177; GitHub #792).
  - An alias matches as a whole token: a maximal run of `[A-Za-z0-9_.:-]` equal to the alias once
    its trailing `.` and `:` bytes are dropped, so an alias never matches inside an identifier.
    `M001:`, `M001.` and `[M001]` name `M001`; `M001-login`, `M001_extra`, `xM001`, `M001.1`
    and `a.M001` do not.
  - Crediting (TOL-V0-010/011), the source presence check and its audit (TOL-V0-012/013) accept
    the native id or the alias in the spec file at `--commit`. The witness event, the credit lists,
    the evidence and the batch summary carry native ids only, so an alias never reaches a recorded
    id field.
  - The expected-fail rules (TOL-V0-022/023) mask every alias span before looking for defect ids,
    so an alias shaped like a defect id (`ABC-12.V4`) never names defect `ABC-12`.
  - Preflight (TOL-V0-025) and the batch causes (TOL-V0-029) count a test, describe or `test.step`
    title naming the alias as naming the obligation. Preflight alone also counts a
    `<contract>:<alias>` annotation in a string literal, exactly as it counts `<contract>:<id>`;
    the batch causes read titles only, for native ids and aliases alike.
  - The receipt audit folds the ledger chain before the audited witness to learn its aliases, so
    an alias-bound match audits `CONSISTENT` from stored content alone.

### Review sources (GitHub #813)

A credited migrated test should say which legacy case it replaces, or show that someone searched
and found none. A free-text "none found" cannot be checked. TOL-V0-037..046 record what a reviewer
read as structured, digest-pinned review sources. They are review records, never witness
evidence: they never credit, demote or block an obligation, satisfy a gate, or change a count.

- `TOL-V0-037`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #813)) A folded entry MAY carry an optional `sources` list of 1..16 review sources, kept
  in the order written.
  - `{kind, ref, digest}` with `kind` `legacy-case`, `legacy-spec` or `manual-case` names a
    reviewed source. `ref` is 1..256 non-blank bytes: a case ID for `legacy-case` and
    `manual-case`, a repository path for `legacy-spec`. `digest` is the 64-lowercase-hex sha256
    the reviewer read.
  - `{kind: none, query, searched, at}` records an empty search. `query` is 0..1024 bytes,
    `searched` a sorted unique list of 0..32 non-blank source ids of 1..256 bytes, and `at` a
    timestamp.
  - Each element is closed to its kind's keys. An unknown kind, a key of the other shape, a bad
    digest, an empty list or an unsorted `searched` refuses `MALFORMED`; a list of more than 16
    sources refuses `LIMIT_EXCEEDED`, the code of the sibling array bounds. The codec
    admits an empty `query` or `searched`; TOL-V0-041 refuses them under the policy, so a stored
    ledger stays readable when the policy changes.
- `TOL-V0-038`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #813)) An `OBLIGATIONS_SET` change MAY carry `sources`. A change becomes `{id, state,
  core, reason, sources?}` and must name a state, a core flag, sources or a combination; `sources`
  replaces the entry's whole list and stamps `updatedAt`/`updatedBy` like any set. A sources-only
  change leaves state and core as they are.
- `TOL-V0-039`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #813)) An entry or set change without sources MUST keep its exact pre-#813 bytes (the
  member is absent, never null), and the `requireSourceReview` policy MUST NOT change any ledger
  byte: the same writes fold to the same entries with the policy on and off.
- `TOL-V0-040`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #813)) The policy key `obligations` becomes `{workerWitness, requireSourceReview?}`.
  With `requireSourceReview: true`, `preflight` (TOL-V0-025) MUST add a `SOURCE_REVIEW_MISSING`
  finding naming each OPEN, DEFECT or BLOCKED obligation that records no sources. WITNESSED and
  DEFERRED obligations are not checked. A policy without the key keeps the check off.
- `TOL-V0-041`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #813)) Under the same policy, `preflight` MUST add a `SOURCE_REVIEW_NONE_UNSEARCHED`
  finding naming each checked obligation that has a `none` source with a blank query (Unicode
  whitespace only) or an empty `searched` set. Each finding carries a remedy, and any finding refuses `PREFLIGHT_FAILED`
  (TOL-V0-025).
- `TOL-V0-042`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #813)) When some entry records sources, `ticket obligations show` and `ticket obligations
  plan` MUST add `sourceReview`.
  - `sourceReview` is `{specCommit, staleCount, stale, unresolvedCount, unresolved}`. `stale`
    lists each legacy-case, legacy-spec or manual-case source whose current digest differs from
    the reviewed one, as `{id, state, kind, ref, reviewed, current}`. `unresolved` lists each
    whose current digest cannot be read, with a `reason`.
  - Neither list changes a state: a WITNESSED obligation with a stale source stays WITNESSED.
    `plan`'s `ok` and exit code are unchanged.
  - A ledger without sources adds no member.
- `TOL-V0-043`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #813)) `show` and `plan` take an optional `--legacy-cases FILE`, a
  `corvint-legacy-cases/1` record from `corvint docs corpus import-cases` (DCP-V1-070), read up to
  32 MiB from a regular file; a symlink, FIFO, device or directory refuses `MALFORMED` without
  blocking. `legacy-case` and `manual-case` refs resolve by case ID to that record's case digest.
  A document that is not exactly one JSON value, a repeated object member, another schema, a case
  without a 64-hex digest or a repeated case ID refuses `MALFORMED`. Without
  the flag, or for a case not in the record, the source is unresolved with the reason.
- `TOL-V0-044`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #813)) A `legacy-spec` ref MUST resolve as a clean repository-relative path at `HEAD`,
  read from Git objects with the bounded TOL-V0-012 reader. Its current digest is the sha256 of
  the blob, and `specCommit` names the commit read. A ref that is not such a path, is absent, or is
  over the read bound is unresolved with the reason.
- `TOL-V0-045`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #813)) Source refs, queries and searched ids MUST pass the TOL-V0-009 secret screen
  before storage, refusing `SECRET_DETECTED` without echoing the value.
- `TOL-V0-046`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #813)) `show`, `plan` and `preflight` stay reads: the review-source checks write no queue,
  intent or evidence state. Review sources never reach witness evidence, counts, high water, gates
  or completion.
### Preflight result cache (GitHub #1037)

Agents repeat the same static preflight over unchanged trees many times a day. TOL-V0-047..050
let `preflight` replay a stored report when every input it reads is unchanged. The cache is a
private, local convenience: a replayed report is the same untrusted read result a fresh run
returns, and nothing reads the cache except `preflight --cache`.

- `TOL-V0-047`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1037)) `preflight --cache` MUST key its report on the sha256 of: the check
  implementation (the sha256 of the running executable, so any rebuild or version change misses);
  the ticket id, the canonical ticket record and the canonical folded ledger entries; the policy
  sha256; the resolved `--commit` and `HEAD`; the binary diff of tracked files against `HEAD`
  (no external diff, no textconv); the path, type and content digest (or link target) of every
  untracked file Git does not ignore; and the `--path` prefixes (sorted) and `--plan` path. The
  working tree is read with repository hooks and the file-system monitor disabled and
  `GIT_OPTIONAL_LOCKS=0`. More than 4,096 untracked files or 256 MiB of untracked bytes, an unborn
  `HEAD`, an unreadable executable or any failed read makes the key unavailable.
- `TOL-V0-048`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1037)) The report item gains `cache: {status, key, storedAt, reason}`.
  - `HIT`: an entry for an identical key replays the stored item, warnings and outcome (a stored
    `PREFLIGHT_FAILED` still refuses) and adds a `PREFLIGHT_CACHE_HIT:` warning naming the
    stored time and key.
  - `MISS`: no valid entry; the checks run and their report is stored.
  - `FRESH`: `--fresh` runs the checks without reading the cache and replaces the entry.
  - `SKIPPED`: the key is unavailable (TOL-V0-047); the checks run and the reason is reported,
    so a cache fault never replays and never refuses. An unreadable entry is a `MISS` with the
    reason and is replaced; a failed store keeps `MISS` or `FRESH` with `storedAt` null and the
    reason.
  - Without `--cache` the item and output are unchanged. `--fresh` without `--cache` and
    `--cache` with `--deep` are usage errors: `--deep` runs gates, which are not static checks.
- `TOL-V0-049`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1037)) Entries MUST be bounded.
  - Each entry is one closed `corvint-tasks-preflight-cache/0` JSON document of at most 4 MiB,
    named by its key and written atomically.
  - An entry older than 7 days, dated more than a minute in the future, not a regular file,
    oversize, malformed or naming another key is a miss.
  - Every store prunes entries older than 7 days, then the oldest beyond 256, and abandoned
    temporary files older than an hour. Pruning never removes a file the cache did not name.
- `TOL-V0-050`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1037)) The cache MUST live in `corvint-tasks/preflight` under the user cache
  directory (`os.UserCacheDir`), created mode 0700, outside every repository, Tasks store and
  trace store. This keeps product invariant 4: `preflight` stays a read that writes no repository,
  queue, intent, evidence or trace state. A cached report is never evidence, a witness, a gate
  result, a completion input or authority: no other verb or reader opens the directory, and its
  item stays `untrusted`. Deleting the directory loses only speed.

### Refusing identical failing re-runs (GitHub #1048)

An agent that re-runs a failing preflight or batch at unchanged inputs gets the same failure
again. TOL-V0-031 already refuses a batch whose obligation failed twice with one error inside one
results directory. TOL-V0-051..054 add an opt-in, cross-directory guard for `preflight` and
`run-batch`, keyed on everything the run reads, that stops the third identical attempt until an
input changes or the agent declares what it fixed.

- `TOL-V0-051`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1048)) The guard MUST be off unless a call gives
  `--refuse-repeat N` (1..20, canonical decimal); without it, `preflight` and `run-batch` read and
  write no ledger and their output is unchanged. With it, the run's item gains
  `failureSignature: {threshold, status, inputDigest, signature, count, remediation, reason}`.
  - The input digest of `preflight` is its TOL-V0-047 cache key. That of `run-batch` is the
    sha256 of the running executable, the ticket id, the commit (whose clean checkout the job
    already proves), the TOL-V0-031 fixture digest, the config sha256, the sha256 of each
    environment variable the config passes (or its absence), the `--pool`, the sorted `--spec`
    paths, the canonical ledger entries, the `--ids` scope and the role.
  - A failure is a refused preflight, or a batch whose outcome is not OK or that records a
    `FAILED` obligation. Its signature is the sha256 of its sorted failure lines (each preflight
    finding; each `FAILED` obligation's id and error, the `NOT_RUN` reason and the outcome codes
    of a batch) with every bare digit run folded to `#` and whitespace collapsed. Digits inside an
    identifier token (a letter, then letters and digits, with `-` or `_` joined parts, such as
    `AC-1` or `V1-1605`) are kept, so failures of different obligations never share a signature.
  - The ledger keeps one entry per verb, ticket and input digest. A failure with the stored
    signature counts one more (`RECORDED`); another signature restarts at 1; a pass removes the
    entry (`CLEARED`, count 0).
- `TOL-V0-052`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1048)) When the entry for the run's input digest
  holds a count of at least N, the run MUST refuse `LOOP_DETECTED` with a
  `FAILURE_SIGNATURE_REPEATED:` detail naming the verb, the count, the signature and the input
  digest, before any check, lane acquire, test command or results directory. Any change to an
  input moves the run to another digest, so it is admitted. A preflight cache hit is a replay, not
  a run, and records nothing; the guard is checked before the replay.
- `TOL-V0-053`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1048)) `--remediation REF` (requires `--refuse-repeat`;
  1..256 bytes of single-line UTF-8 text, secret-screened, `SECRET_DETECTED` when it matches)
  MUST admit one refused run per distinct reference.
  - Before the run, one `corvint-tasks-remediation-audit/0` line naming the time, verb, ticket,
    input digest, signature, count and reference is appended to `remediations.jsonl` in the
    ledger directory. A failed append or a log at 4 MiB refuses rather than running unaudited.
  - The admitted run keeps the count, so its identical failure refuses the next run at any N. A
    reference already spent on the entry refuses `LOOP_DETECTED` with
    `FAILURE_REMEDIATION_REUSED:`; more than 16 references on one entry refuse `LIMIT_EXCEEDED`.
  - A preflight the remediation admits runs its checks even under `--cache`.
- `TOL-V0-054`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1048)) The ledger MUST live in
  `corvint-tasks/failure-signatures` under the user cache directory, created mode 0700, beside
  the TOL-V0-050 cache and outside every repository, Tasks store and trace store, so both verbs
  keep product invariant 4. Each read-check-write of an entry, the remediation spend and the audit
  append hold an exclusive lock on `.lock` in that directory, so concurrent runs neither lose a
  count nor spend one reference twice; a lock not taken within 10 s, or a platform without one,
  makes the ledger unavailable. Each entry is one closed `corvint-tasks-failure-signature/0`
  document of at most 64 KiB written atomically; entries older than 30 days are empty and are
  pruned, with at most 1,024 kept. Only the verb that wrote an entry reads it, to decide whether
  to run; it is never evidence, a witness, a gate result or authority. An input digest or ledger
  that cannot be read runs the job unguarded with status `SKIPPED` and the reason, and a failed
  ledger write is reported in `reason`.

### Ledger migration blockers (GitHub #1071)

Migrating an adopter's 4,562-row per-ticket ledger (GitHub beamfall/corvint#1071, V1-1649) hit
five blockers. These requirements remove them without changing the event profile, and each new
member is optional with the TOL-V0-034 precedent: absent, it keeps the exact earlier bytes.

- `TOL-V0-055`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1071)) An
  obligation alias (TOL-V0-034) MUST be 1..128 bytes, keeping the TOL-V0-034 grammar. An alias
  outside that length MUST refuse `MALFORMED` with a detail that states its length and the
  `1..128 bytes` limit, and writes nothing.
- `TOL-V0-056`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1071)) A
  plan test MAY carry an optional `rerun` member: 1..256 sorted unique obligation ids that the test
  re-runs as regression checks.
  - With `rerun` present, `obligations` MAY be empty (0..256 ids). Without it, `obligations` keeps
    the TOL-V0-020 bound of 1..256.
  - One test naming an id under both `obligations` and `rerun` refuses `MALFORMED`.
  - A rerun id the ledger holds as `WITNESSED` is reported in the output member `reruns`
    (`{id, tests}`, sorted by id). That member is present only when some test carries `rerun`, so a
    plan without `rerun` keeps its exact output.
  - A rerun id the ledger holds in any other state is the failing finding `RERUN_NOT_WITNESSED`;
    a rerun id the ledger lacks is `UNKNOWN_OBLIGATION`.
  - A `WITNESSED` or `DEFERRED` id under `obligations` stays `ALREADY_CLOSED`. `preflight --plan`
    reports the same findings with remedies (TOL-V0-026).
- `TOL-V0-057`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1071)) This
  requirement amends which obligations TOL-V0-020 requires in a plan.
  - `UNASSIGNED` MUST apply only to core obligations in state `OPEN` or `PARTIAL` with no planned
    test.
  - A non-core obligation, or a `DEFECT` or `BLOCKED` one, needs no plan entry.
  - Any pending obligation (`OPEN`, `PARTIAL`, `DEFECT` or `BLOCKED`) that two or more tests list
    under `obligations` is `SPLIT`.
  - The check exits 0 exactly when it reports no finding.
- `TOL-V0-058`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1071)) The
  ledger MUST offer an optional `PARTIAL` state for an obligation with only some steps witnessed.
  - `set` (TOL-V0-007) moves an obligation to or from `PARTIAL`, and the reason is required.
  - `PARTIAL` is pending wherever `OPEN`, `DEFECT` and `BLOCKED` are: in preflight naming
    (TOL-V0-025), witness classification, run-batch causes and source review. A witness credits a
    `PARTIAL` obligation to `WITNESSED`.
  - `PARTIAL` counts in `total` and never in `witnessed` or `coreWitnessed`, so it never raises
    `highWater.witnessed` (TOL-V0-016, 021).
- `TOL-V0-059`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1071)) A
  witness (TOL-V0-008) MAY carry an optional closed `provenance` object. It is set with
  `--test-commit OID`, `--environment-tree OID`, `--evidence-ref TEXT` and `--test-title TEXT`.
  - Members: `testCommit` and `environmentTree`, each a 40- or 64-hex object name; `evidenceRef`,
    1..256 bytes of non-blank prose; and `testTitle`, 1..256 bytes of non-blank prose. Every
    member is optional, and at least one is named.
  - The object is copied to the evidence of each entry that witness credits, and `show` reports it
    per witness.
  - Every given flag is validated, an explicitly empty value included: a flag given with an empty
    value is a malformed OID or prose under its 1-byte minimum, never an absent member.
  - Its prose members are secret-screened with the TOL-V0-009 screen. A match refuses
    `SECRET_DETECTED`, and a malformed OID or out-of-bounds prose refuses `MALFORMED`; neither
    writes.
  - Provenance is caller-supplied and reported only. It is never resolved against Git, never
    compared with the report, and never part of the TOL-V0-013 credit recomputation.
  - A witness without it keeps its exact payload and evidence bytes.

### Adopter qualification and declared test repositories (GitHub #1070)

An adopter with a live Playwright suite could credit nothing (GitHub beamfall/corvint#1070,
V1-1648). The shipped qualified version list is empty, so every real report refuses
`UNSUPPORTED_VERSION`. The witness commit and the TOL-V0-012 source check also had to resolve in the
repository that holds the task store, while many adopters keep their browser tests in a separate
repository. These requirements add a local qualification path and a declared test repository. They
change no event profile, and each new payload or evidence member is optional with the TOL-V0-034
precedent: absent, it keeps the exact earlier bytes.

- `TOL-V0-060`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1070)) The
  qualification fixture MUST be one Playwright spec file, `obligation-qualification.spec.ts`,
  embedded in the binary with a fixed sha256. `ticket obligations qualify-version --print-fixture`
  MUST print its text, its sha256 and the expected class of each of its nine ids, and write nothing.
  - The ids are `TOLQ-1`..`TOLQ-9`, in one describe block that allows one retry.
  - `TOLQ-1`: a passing step, `credited`.
  - `TOLQ-2`: a step whose `expect.soft` fails, `failed`.
  - `TOLQ-3`: a passing step after that soft failure, `credited` (TOL-V0-010).
  - `TOLQ-4`: a parent step whose nested step soft-fails, `failed`.
  - `TOLQ-5`: a test title whose test soft-fails outside any step, `failed`.
  - `TOLQ-6`: a passing step in that test, `credited`.
  - `TOLQ-7`: a test that fails at retry 0 and passes at retry 1, `failed` (retry 0 only).
  - `TOLQ-8`: a passing test title, `credited`.
  - `TOLQ-9`: a `test.fail` test titled expected-fail that fails, `defectConfirmed` (TOL-V0-022).
- `TOL-V0-061`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1070))
  `ticket obligations qualify-version --from-playwright-report FILE` MUST check one json-reporter
  document of that fixture with the TOL-V0-009 reader, the TOL-V0-010..012 and TOL-V0-022
  classifier and a synthetic `OPEN` ledger of the nine ids, without reading any Tasks record.
  - Any `config.version` label is admitted for this check only. The shape bound, the size bound
    and the refusals of TOL-V0-009 are unchanged and keep `OBLIGATION_REPORT:`.
  - Every match of every id, whatever its outcome, MUST be in one file named
    `obligation-qualification.spec.ts` beneath `config.rootDir`, read as the embedded fixture
    bytes. A match in any other file, a second such file or a path outside `config.rootDir`
    refuses as below, naming the id and the file.
  - The check passes only when the report has no top-level `errors` and each id's class equals its
    expected class: no id is `unknown`, `unbound`, `conflicting`, `mixedExpectedFail`, missing or
    in another class.
  - A failing check MUST refuse `GATE_FAILED` with the detail prefix
    `OBLIGATION_QUALIFICATION_FAILED:`, naming the first differing id with its expected and observed
    class, and write nothing.
  - The report is never retained, as in TOL-V0-009. Only its sha256 is kept.
- `TOL-V0-062`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1070)) A
  passing check MUST record the version in the store-local allow-list
  `<git-common-dir>/taskman-obligation-qualifications.json`, profile
  `taskman-obligation-qualifications/0`, with its evidence.
  - Evidence per version: `reportSha256`, `fixtureSha256`, the sorted `credited`, `failed` and
    `defectConfirmed` id lists, and `qualifiedAt`.
  - The read, update and atomic write run under the store mutation lock
    (`<git-common-dir>/taskman.lock`, with the default lock wait), so concurrent runs each keep
    their version instead of the last writer dropping another's.
  - The file is written atomically, holds at most 64 versions and is read through a 1 MiB bound.
    A 65th version refuses `LIMIT_EXCEEDED` and writes nothing. Qualifying a recorded version
    again replaces its evidence.
  - A file written by a later build refuses `UNSUPPORTED_VERSION` (CAL-V0-131). A malformed,
    oversize or non-regular file refuses `MALFORMED` for every verb that reads it, and is never
    read as an empty list.
  - The read opens the final entry no-follow and nonblocking, fstats that descriptor, requires a
    regular file and reads at most the bound, so a symlink or FIFO swapped in refuses `MALFORMED`
    without blocking. A platform without that boundary (Windows) refuses the read.
  - The file is local operator state. It is outside the journal, the archive and the intent tree,
    and it is never evidence, a grant or a gate result. Deleting it, or one entry, withdraws the
    version.
- `TOL-V0-063`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1070))
  `witness` and `run-batch` MUST admit a report whose `config.version` is in the shipped list or
  in the store-local allow-list of the repository they run in. An allow-list entry admits its
  version only when its `fixtureSha256` is the shipped fixture's and its `credited`, `failed` and
  `defectConfirmed` lists are exactly the fixture's expected classes; any other entry admits
  nothing until `qualify-version` replaces it. Every other version keeps the
  TOL-V0-009 refusal unchanged: `UNSUPPORTED_VERSION` with `OBLIGATION_REPORT_VERSION_UNQUALIFIED:`.
  An admitted report is credited by TOL-V0-010..013 unchanged.
- `TOL-V0-064`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1070)) The
  policy key `obligations` MAY carry an optional `testRepositories` object. It maps 1..16 names
  (labels) to local paths.
  - A path is 1..4096 bytes without control characters. It is absolute, or relative to the
    primary worktree.
  - A malformed or oversize object refuses the policy `MALFORMED` or `LIMIT_EXCEEDED`.
  - A policy without the member keeps its exact bytes and meaning.
- `TOL-V0-065`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1070))
  `witness --test-repository NAME` MUST resolve NAME through the current policy to that local
  checkout. The `--commit` resolution of a report or declared witness, and the TOL-V0-012 source
  check, then run in that repository instead of the store's.
  - The payload and the evidence of every credited entry carry the optional member
    `testRepository: NAME`. A witness without the flag keeps its exact payload and evidence bytes.
  - A name the policy does not declare refuses `MALFORMED` with the detail prefix
    `OBLIGATION_TEST_REPOSITORY_UNDECLARED:`, at the command and in the writer, and writes nothing.
  - A declared path that is not itself the root of a Git worktree, including a directory inside
    another repository, refuses `MISSING_EVIDENCE` and writes nothing.
  - The writer binds the declared path the command resolved. If the policy at write time declares
    the name with another path, or no resolved path is bound, it refuses `REVISION_CONFLICT`
    `STALE_POLICY` and writes nothing.
  - The writer's TOL-V0-013 recomputation binds the name: a payload whose `testRepository`
    differs from the recomputation refuses `OBLIGATION_CREDIT_MISMATCH:`.
- `TOL-V0-066`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1070))
  `receipt audit` MUST re-check a witness that names a test repository there. The repository is
  resolved from the policy the history had posted at that receipt (TOL-V0-013).
  - A report witness re-checks each match's path at the commit.
  - A declared witness checks that the commit exists.
  - A name that policy does not declare, a declared directory that is not itself a worktree root,
    a repository that is not observable or a missing commit or path is `INCONSISTENT`.
- `TOL-V0-067`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1070))
  Without `--test-repository`, every refusal MUST stay as before.
  - A spec outside the store's repository is `unbound` `OUTSIDE_REPOSITORY`.
  - A commit only the test repository holds refuses `MISSING_EVIDENCE` ("the repository holds no
    commit").
  - A version neither list holds refuses as in TOL-V0-009.

### Live fixture qualification (V1-1022; 2026-10-20)

TOL-V0-009 keeps the shipped qualified version list empty until the live fixture passes on a 1.63.x
`PWP-V0-008` tuple. The V1-1648 slice delivered `qualify-version` on synthetic reports only and left
the live run `NOT_RUN`. These requirements retain that run, add a repeatable live test and ship the
one version it qualifies.

- `TOL-V0-126`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1022) The shipped
  TOL-V0-009 qualified version list MUST be exactly `1.63.0`, the version whose live fixture run
  TOL-V0-128 retains. A `1.63.0` report is admitted without a store-local allow-list. Any other
  version, including another 1.63 patch, still refuses `UNSUPPORTED_VERSION`
  (`OBLIGATION_REPORT_VERSION_UNQUALIFIED:`) until `qualify-version` records it (TOL-V0-063).
  Admitting a shipped version writes no allow-list.
- `TOL-V0-127`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1022)
  `TestTOLV0127_LiveFixtureQualifies` MUST run only when `CORVINT_PLAYWRIGHT_MODULES` (a
  `node_modules` holding `@playwright/test`) and `CORVINT_TOL_QUALIFY_NODE` (an absolute `node`
  with a sibling `npx`, put first on `PATH`) are set; otherwise it skips as `NOT_RUN`, never as
  qualification success. It writes the embedded fixture unchanged and runs it with the built-in
  `json` reporter, which must exit 1. It checks the report with `qualify-version`, which must
  report the fixture's classes and digest. A witness of that version must then credit and audit
  `CONSISTENT`; when the version is not shipped, it must refuse before qualification.
- `TOL-V0-128`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1022) The live
  run is retained for one `PWP-V0-008` tuple: macOS arm64, Node v22.23.2 and `@playwright/test`
  1.63.0. It ran on 2026-10-20 and passed, with fixture sha256
  `382442f6bd67256eb5c8c1864411b332e8849d1acf469b2cdb78651e1d285674` and report sha256
  `046ffc0ecc7891de5b90594b15914a37da3319df1771b4219e05c4606ae4d0b1`. The same test with 1.64.0
  on Node v22.23.2 also passed (report sha256
  `3b4a90c0485000f584f25670e2959bb630e05487d4818835dd72ab0f3bcfb50c`). That qualifies 1.64.0
  only store-locally; it is not shipped, because TOL-V0-009 names a 1.63.x tuple. A report digest
  identifies one run, since the report carries timings and temporary paths. The report
  bytes are not retained (TOL-V0-009), and the build log
  `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-20-V1-1022-1024-playwright-live-qualification.md` records the run.

Rollback: set the shipped list back to empty. A `1.63.0` report then refuses again unless the
store-local allow-list holds it. Witness events already written keep their bytes and audit
`CONSISTENT`, because the audit recomputes credit from the receipt and never re-reads the version
list.

### Reviewer-verdict witnesses (GitHub #1170)

Some obligations are witnessed by reading the cited example, not by a test run. These
requirements let `witness` credit an obligation on K independent reviewer verdicts under an
agreement policy, keep held verdicts, and stop crediting once the cited example changes. They
reuse the witness write, the obligation chain and the CAL-V0-107 author derivation
(`exclude-authors`); no new journal or record profile is added.

- `TOL-V0-073`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1170))
  `OBLIGATIONS_WITNESS` MUST admit the source `REVIEWED`. Such a payload credits exactly one
  obligation `{id}`, has a null `reportSha256`, `playwrightVersion` and `declaration`, and carries
  the optional member `review`. `review` is present exactly when the source is `REVIEWED`; a
  payload or evidence without it keeps its exact earlier bytes, and an older closed reader refuses
  a payload that carries it `MALFORMED`. A `REVIEWED` witness is OWNER-only (as `DECLARED`,
  TOL-V0-015) and names no attempt.
- `TOL-V0-074`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1170))
  `review` is the closed object `{policy, subject, authors, verdicts}`:
  - `policy` is `{reviewers, maxStubRisk}`: K in 1..16 agreeing reviewers and the highest admitted
    stub risk, `none` or `boundary`.
  - `subject` is `{path, startLine, endLine, blobSha256}`: the cited example's repository path, a
    1-based line range with `startLine <= endLine`, and the sha256 of the path's blob at the
    witness commit, which the command computes from Git.
  - `authors` is the sorted unique list (0..64) of member labels that may not review: every pool
    member the CAL-V0-107 derivation records for the ticket's implement generations (mode `ALL`),
    plus the caller's `--exclude-author` labels.
  - `verdicts` is 1..16 objects sorted unique by `reviewer`, each
    `{reviewer, verdict, stubRisk, reason}`: a label, `correct|partial|wrong`,
    `none|boundary|on-path` and a 1..512-byte non-blank reason.
  - A reviewer listed in `authors` refuses `MALFORMED` with the detail prefix
    `OBLIGATION_REVIEWER_NOT_INDEPENDENT:`. When the derivation is unverified (CAL-V0-107) and the
    caller named no `--exclude-author`, the command refuses `INDEPENDENCE_UNVERIFIED` and writes
    nothing.
- `TOL-V0-075`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1170)) The
  outcome MUST be a pure function of `review`: `CREDITED` when at least K verdicts are given, every
  verdict is `correct` and no stub risk exceeds `maxStubRisk` (`on-path` is never admitted);
  otherwise `HELD`, with the sorted failing axes from `reviewers`, `stubRisk` and `verdict`.
  - `CREDITED` sets the entry `WITNESSED` with evidence `{source: REVIEWED, eventSha256, commit}`
    (report members and `declaration` null).
  - `HELD` writes the event and keeps every verdict and reason. An entry witnessed `REVIEWED` goes
    back to `OPEN` with no evidence; any other state is kept.
  - Either way the entry carries the optional member `review`
    `{outcome, axes, eventSha256, commit, review}`, plus `testRepository` when the witness named
    one; an entry without it keeps its exact bytes.
- `TOL-V0-076`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1170)) The
  fold MUST refuse `OBLIGATION_CREDIT_MISMATCH:` for a `REVIEWED` witness of a `DEFERRED` entry, of
  an entry `WITNESSED` by another source, and of an entry `WITNESSED` by `REVIEWED` whose recorded
  subject blob equals the new one. A changed subject blob admits a fresh review.
- `TOL-V0-077`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1170); review-round amendment accepted by the owner 2026-10-19, PR #1189)
  `obligations show` MUST report `witnessReview` for every entry with a `review` member: id,
  outcome, axes, verdicts with reasons, and `stale` (true when the blob of `subject.path` in
  the repository the review read it from, at the TOL-V0-131 basis, differs from `blobSha256` or
  the path is absent). That repository is the store worktree, or, for a witness given `--test-repository`, the
  policy-declared test repository the entry's `review.testRepository` names; such an entry also
  reports `testRepository`. A test repository that is no longer declared or no longer readable
  reports `stale: null` with `staleUnknown: TEST_REPOSITORY_UNRESOLVED` and `credited: false`. A
  stale `CREDITED` review reports `credited: false`, and `witnessReview.effectiveWitnessed` counts
  witnessed entries minus stale or unresolved reviewed ones. The read writes nothing; the stored state and counts change
  only through a new review (TOL-V0-076). A ledger without a `review` member keeps its exact
  `show` output.
- `TOL-V0-078`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1170)) Each
  verdict's reviewer and reason and the subject path MUST pass the TOL-V0-009 secret screen; a
  match refuses `SECRET_DETECTED` naming `/payload`, never the value.
- `TOL-V0-131`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1922)
  `obligations show` MUST judge a store-worktree review's staleness at the ticket's newest
  submitted candidate branch when that branch resolves, at `HEAD` only otherwise, and MUST name the
  basis in each `witnessReview` entry. `staleBasis` is `{source, commit}` with `source:
  CANDIDATE_BRANCH | HEAD | TEST_REPOSITORY_HEAD` and the commit judged, or `commit: null` when the
  repository did not resolve; a test-repository review stays judged at that repository's `HEAD`.
  The candidate branch is the one the ticket's newest attempt with a submitted candidate tree
  names, the branch `gate packet` reviews (ERG-V0-017). The `obligations plan` witness statuses
  (TOL-V0-104) use the same basis (TOL-V0-132). The judgement is still read-time only: no stored count, gate
  or fingerprint sees it.
- `TOL-V0-132`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2248)
  `obligations plan --witnesses` MUST judge each `REVIEWED` ledger witness at the TOL-V0-131 basis:
  the ticket's newest submitted candidate branch when it resolves, `HEAD` otherwise, and a test
  repository's `HEAD` for a test-repository review. Each `REVIEWED` witness MUST add `staleBasis`
  `{source, commit}` as in TOL-V0-131; other witnesses carry no `staleBasis`. The status names
  `REVIEW_STALE_AT_HEAD` and `REVIEW_UNRESOLVED_AT_HEAD` (TOL-V0-105) keep their earlier spelling
  for wire stability and mean stale or unresolved at the named basis. Without `--witnesses` the
  candidate is not read and the plan output keeps its exact earlier bytes.

### Test boundaries and scope decisions (GitHub #1180)

Some obligations cite code that no runnable test in its repository can reach. These requirements
let an entry record the source locations it cites, report which pending entries sit outside every
test boundary, and move an entry out of the in-scope (core) count only through a recorded decision
that another actor accepts as OWNER. They reuse `OBLIGATIONS_SET`, the obligation chain and the
declared test repositories (TOL-V0-064); no new journal or record profile is added.

- `TOL-V0-079`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1180)) An `OBLIGATIONS_SET`
  change and an entry MUST admit the optional member `cites`: 1..16 objects
  `{repository, path}`, where `repository` is null for the store's own repository or a label naming
  a declared test repository, and `path` is a repository path to a file or directory (a trailing
  `/` names the same directory). A change's
  `cites` replaces the entry's. An entry without the member keeps its exact earlier bytes, and an
  older closed reader refuses a change that carries it `MALFORMED`. A label is checked at write
  by default under TOL-V0-102 (owner decision 2026-10-19, V1-1963); under an explicit
  `obligations.requireDeclaredCites` false it is not, and an undeclared label reads `UNKNOWN`
  (TOL-V0-083).
- `TOL-V0-080`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1180)) A change MUST admit the optional member
  `scope`: `{action: PROPOSE, core, evidence}` with a 1..512-byte non-blank evidence reference, or
  `{action: ACCEPT, core}`. A change with `scope` has a null `core`, else it refuses `MALFORMED`; the
  change's `reason` is the decision's reason.
- `TOL-V0-081`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1180)) `PROPOSE` MUST record the entry member
  `scope` `{status: PROPOSED, core, reason, evidence, proposedBy, proposedAt}` and leave `core`
  unchanged. `ACCEPT` is OWNER-only (`UNAUTHORIZED` otherwise). It refuses `MALFORMED` with the
  detail prefix `OBLIGATION_SCOPE_NO_PROPOSAL:` unless the entry holds a `PROPOSED` decision for the
  same `core`. It refuses with the prefix `OBLIGATION_SCOPE_SELF_ACCEPT:` when the acting actor id
  is the proposer's. Otherwise it sets `status: ACCEPTED` with `acceptedBy` and `acceptedAt`, and sets
  the entry's `core`. The reference counts then exclude an entry accepted out of scope, as for
  any non-core entry. Each decision stays in the chain's retained requests; a later `PROPOSE`
  replaces the entry member. `obligations show` MUST list `outOfScope` (entries with `core` false
  and an accepted decision) and `scopeProposals` (pending decisions), each with its decision. A
  ledger with no scope decision keeps its exact `show` output.
- `TOL-V0-082`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1180)) The optional policy key
  `obligations.requireScopeDecision` (default false) MUST, when true, refuse every plain `core`
  change `VALIDATION_FAILED` with the code `APPROVAL_MISSING` and the detail prefix
  `OBLIGATION_SCOPE_DECISION_REQUIRED:`, so `core` moves only through TOL-V0-081. Absent or false
  keeps the TOL-V0-007 core change.
- `TOL-V0-083`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1180)) The read
  `ticket obligations boundary --target T` MUST write nothing. It lists the store's repository and
  each repository a pending (neither `WITNESSED` nor `DEFERRED`) entry cites at its `HEAD`. It
  reports each repository's test inventory: status `KNOWN` with the commit, the count of
  runnable test files by kind and up to 5 example paths, or `UNKNOWN` with a reason. A runnable test
  file is a tracked path matching a closed naming set (Go, Python, JavaScript/TypeScript, Ruby,
  JVM, Rust, .NET, Swift, PHP, Elixir). Each pending entry gets one verdict:
  - `TESTABLE`: one cite is tracked in a repository with at least one runnable test file;
  - `UNKNOWN` with `NO_CITATIONS` when it records no `cites`, or `REPOSITORY_UNAVAILABLE` when a
    cited repository could not be listed;
  - otherwise `NO_TEST_BOUNDARY` with `NO_RUNNABLE_TESTS` (cited code is tracked, but only where
    no runnable test exists) or `NO_TRACKED_CODE` (no cite is tracked).

  The read lists `noTestBoundary` ids and `rescopeCandidates`: entries out of scope by an accepted
  decision whose verdict is now `TESTABLE`.

### Reviewed-shortfall worklist (GitHub #1182)

Reviewed shortfalls (TOL-V0-075 `HELD`) are scattered across tickets. These requirements rank them
into one queue a worker can take a test file at a time, without re-queuing an entry whose new
witness candidate awaits review.

- `TOL-V0-084`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1182)) A change MUST admit the optional member
  `candidate` `{path, startLine, endLine}`: a new witness candidate awaiting review, with a 1-based
  line range. The entry records it with `at` (the request's issue time). A later `REVIEWED`
  witness or any credit of the entry clears it. An entry without the member keeps its exact bytes.
- `TOL-V0-085`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1182)) A verdict MUST admit the optional member
  `finding: NO_FALSIFIABLE_TEXT`, which says the obligation states nothing a test can falsify. A
  `finding` beside a `correct` verdict refuses `MALFORMED`. A verdict without it keeps its exact
  bytes.
- `TOL-V0-086`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1182)) The read
  `ticket obligations worklist [--target T]` MUST write nothing. It covers `--target`, or every
  `OPEN` or `HELD` ticket with a ledger. It considers each pending entry whose latest review is
  `HELD`, in this order:
  - `needsObligationText` when any verdict carries `NO_FALSIFIABLE_TEXT`;
  - `excluded` with `CANDIDATE_AWAITING_REVIEW` (a `candidate` is recorded), `BLOCKED` (the entry is
    `BLOCKED`), `SUBJECT_UNVERIFIED` (the reviewed test repository cannot be read or has no `HEAD` commit) or
    `SUBJECT_CHANGED` (the reviewed example's blob at that repository's `HEAD` differs or is absent);
  - otherwise an item.

  Each item carries the ticket and priority, its remaining pending core count, the obligation id
  and title, the verdict (`partial` when any reviewer said partial, else `held`), the missing
  clauses (each non-`correct` verdict's reviewer, verdict and reason), the stub-risk notes (each
  reviewer whose risk is not `none`), the test repository, path and line range, the review time and
  the review age in whole days. A ledger that cannot be folded is listed under `unknown`.
- `TOL-V0-087`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1182)) `--rank` MUST take a comma-separated
  ordering of `priority` (P0 first), `nearest` (fewest remaining pending core entries first) and
  `age` (oldest review first), each at most once, default `priority,nearest`. Ties break by ticket id,
  then obligation id. An unknown or repeated key refuses `MALFORMED`.
- `TOL-V0-088`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1182)) `--next K` (0..100) MUST return the ranked
  head plus up to K later items that cite the same test file (test repository and path), in rank
  order.

### Ledger gate hardening (V1-1945, V1-1946, V1-1949; GitHub #1202)

The reviewed-witness, boundary and plan reads judged evidence at `HEAD` alone. A review whose test
file later changed could not be resolved where it was reviewed, an unborn test-repository `HEAD`
read as a stale review, a cite label typo was admitted silently, the test classifier was a closed
naming set with no declared extension, a plan check counted only browser-report coverage, and no
read answered whether a capability exists in a deployed environment rather than at `HEAD`. These
requirements add read-side resolution and three optional policy members. They add no journal,
record or event profile and change no accepted requirement's default output.

- `TOL-V0-100`: (accepted under the owner's standing auto-accept (2026-10-11; V1-1946)) When a
  reviewed witness names a declared test repository whose `HEAD` names no commit (an unborn
  branch), `show`'s `witnessReview` entry MUST report `stale` null with `staleUnknown`
  `TEST_REPOSITORY_UNRESOLVED` and withhold the credit from `effectiveWitnessed`, exactly as for
  a test repository that cannot be resolved (TOL-V0-077). A test repository with a `HEAD` commit
  whose reviewed blob differs still reports `stale` true.
- `TOL-V0-101`: (accepted under the owner's standing auto-accept (2026-10-11; V1-1945)) Each
  `boundary` entry with a recorded review, and each `worklist` item and `excluded` entry, MUST
  carry `reviewedSubject` `{repository, path, commit, status, reason}`: the reviewed file resolved
  at the review's own commit in the repository the review read, never at `HEAD`. `status` is
  `RESOLVED` with a null `reason` when that commit holds the file with the reviewed digest;
  otherwise `UNAVAILABLE` with the reason `TEST_REPOSITORY_UNRESOLVED` (the repository cannot be
  resolved), `REVIEWED_COMMIT_UNAVAILABLE` (it does not hold the reviewed commit),
  `SUBJECT_ABSENT_AT_REVIEW`, `SUBJECT_UNREADABLE_AT_REVIEW` (over the read bound) or
  `SUBJECT_MISMATCH_AT_REVIEW`. The member never changes a verdict, an item, an exclusion or its
  reason; an entry without a review has no member.
- `TOL-V0-102`: (accepted under the owner's standing auto-accept (2026-10-11; V1-1945); default
  amended to true by owner decision 2026-10-19, V1-1963) The optional policy key
  `obligations.requireDeclaredCites` (default true) MUST, when absent or true, refuse an
  `OBLIGATIONS_SET` change any of whose `cites` names a repository label that
  `obligations.testRepositories` does not declare: `VALIDATION_FAILED` with the code `MALFORMED`
  and the detail prefix `OBLIGATION_CITE_REPOSITORY_UNDECLARED:`, writing nothing. A null
  repository (the store's own) is always admitted. An explicit false opts out to the pre-amendment
  TOL-V0-079 write (a label is not checked at write and reads `UNKNOWN`); a non-boolean value is
  refused `MALFORMED`.
- `TOL-V0-103`: (accepted under the owner's standing auto-accept (2026-10-11; V1-1945)) The
  optional policy key `obligations.testFilePatterns` MUST admit 1..32 objects `{kind, pattern}`:
  `kind` a label and `pattern` a 1..256-byte relative `path.Match` pattern with no TAB, LF or CR;
  an empty or over-count array refuses the policy `LIMIT_EXCEEDED` and a malformed pattern
  `MALFORMED`. The boundary inventory (TOL-V0-083) and the
  plan witness type (TOL-V0-104) classify a tracked path by the closed naming set first, then by
  the first declared pattern that matches: against the base name when the pattern has no `/`,
  else against the whole path, with `*` never crossing `/`. An inventory classified with declared
  patterns adds `declaredPatterns` (their count); without the key every inventory keeps its exact
  bytes.
- `TOL-V0-104`: (accepted under the owner's standing auto-accept (2026-10-11; V1-1949, GitHub
  #1202)) `ticket obligations plan --witnesses` MUST add `ledgerWitnesses`: one object per
  `WITNESSED` entry `{id, source, testType, commit, testRepository, eventSha256, status}`, where
  `testType` is `browser` for a Playwright report, `declared` for an OWNER declaration, the naming
  or declared kind of a reviewed file (TOL-V0-103), else `unclassified`. A `WITNESSED` row is
  never `UNASSIGNED` (TOL-V0-020) whatever its test type, so a plan whose open rows all hold
  receipt-backed reviewed witnesses at a pinned commit is not refused for missing witnesses.
  Without `--witnesses` the plan output keeps its exact earlier bytes.
- `TOL-V0-105`: (accepted under the owner's standing auto-accept (2026-10-11; V1-1949)) Under
  `--witnesses`, a `REVIEWED` witness MUST have `status` `REVIEW_STALE_AT_HEAD` when its reviewed
  blob differs at the `HEAD` of the repository the review read, `REVIEW_UNRESOLVED_AT_HEAD` when
  that cannot be judged (TOL-V0-077, 100), else `COVERED`; every other witness is `COVERED`. Each
  non-`COVERED` witness adds a `STALE_REVIEW` finding, sets `ok` false and refuses the plan with a
  warning that names `STALE_REVIEW`.
- `TOL-V0-106`: (accepted under the owner's standing auto-accept (2026-10-11; V1-1949, GitHub
  #1202)) Under `--witnesses`, each `UNASSIGNED` finding MUST carry `considered` `{witness,
  insufficient}`: a recorded candidate (`{kind: CANDIDATE, path}`, `CANDIDATE_AWAITING_REVIEW`), a
  `HELD` review (`{kind: REVIEWED, path, commit, testRepository, eventSha256, axes}`,
  `REVIEW_HELD`), else a null `witness` with `NO_LEDGER_WITNESS`.
- `TOL-V0-107`: (accepted under the owner's standing auto-accept (2026-10-11; V1-1949, GitHub
  #1202)) The optional policy key `obligations.environmentBaselines` MUST admit 1..17 members
  keyed `.` (the store's repository) or a test-repository label, each a full `refs/...` name
  (no whitespace, `~^:?*[\`, `..`, `@{`, trailing `/` or `.lock`) or a 40 or 64 lowercase-hex
  object id; an empty or over-count object refuses the policy `LIMIT_EXCEEDED` and anything else
  `MALFORMED`. It names the revision each
  environment runs.
- `TOL-V0-108`: (accepted under the owner's standing auto-accept (2026-10-11; V1-1949)) The read
  `ticket obligations capability --path P [--contains TEXT] [--repository NAME]` MUST write
  nothing and answer only from the declared baseline's commit, never from `HEAD`. Its untrusted
  item is `{baseline {repository, ref, commit}, path, contains, status, reason, matches,
  truncated}`: `PRESENT` with up to 16 tracked paths at or under `P` (containing `TEXT` when
  given) and `truncated` when more matched or the file or blob bound left a file unexamined,
  or `ABSENT` with `NOT_FOUND_AT_BASELINE`. `P` is a
  clean repository-relative path (`.` for all) and `TEXT` 1..256 bytes on one line, else the
  call refuses as usage.
- `TOL-V0-109`: (accepted under the owner's standing auto-accept (2026-10-11; V1-1949)) The
  capability read MUST abstain with `status` `UNKNOWN`, empty `matches` and the reason
  `ENVIRONMENT_BASELINE_UNDECLARED` (no baseline for that repository; `ref` and `commit` null),
  `TEST_REPOSITORY_UNRESOLVED`, `ENVIRONMENT_BASELINE_UNRESOLVED` (the ref does not resolve or its
  tree cannot be listed; `commit` null) or `SEARCH_BOUND_EXCEEDED` (nothing matched but more than
  1,024 files were listed or a blob was over the read bound).

### Obligation mirrors (GitHub #1214)

Two tickets can carry the same obligation: a parent decides an item that a child or sibling also
lists. The second copy stayed `OPEN` until someone repeated the decision, so the dispatcher kept
offering a ticket whose remaining work was already done elsewhere. A ticket parked because every
item was `BLOCKED` also stayed parked after an item was unblocked, since a state change below the
high-water mark did not change its fingerprint (GitHub #1177 follow-up). These requirements add one
optional entry member and read-side derivation. They add no journal, record or event profile.

- `TOL-V0-120`: (accepted under the owner's standing auto-accept (2026-10-11; V1-1966, GitHub
  #1214)) A `set` change MUST admit the optional member `satisfiedBy`: either a closed object
  `{ticketId, id}` naming an owner ticket and one of its obligation ids, or `null`, which clears
  the mirror. The entry records it as the optional member `satisfiedBy`. An entry without it keeps
  its exact bytes. A `ticketId` equal to the ledger's own ticket refuses `MALFORMED`
  `OBLIGATION_MIRROR_SELF:`. The owner is not resolved at write time; a missing owner is a read
  finding (TOL-V0-121).
- `TOL-V0-121`: (accepted under the owner's standing auto-accept (2026-10-11; V1-1966, GitHub
  #1214)) `ticket obligations show` MUST add `mirrors` `{completable, items}` when the ledger has
  a mirror, and omit it otherwise. Each item gives the entry `id`, `satisfiedBy`, its recorded
  `localState`, its effective `state` and `derived`. Resolution goes one hop to the owner's recorded
  entry state. A pending local entry whose owner entry is `WITNESSED` or `DEFERRED` is derived: its
  `state` is the owner's and its `source` is `{ticketId, id, state, eventSha256}` (the owner's
  evidence event, null for a deferral). A derived item carries no review of its own. Any other
  pending mirror stays at its local state with `reason`: `OWNER_TICKET_MISSING` (no such ticket in
  this queue, including a ticket of another queue), `OWNER_LEDGER_MISSING`, `OWNER_LEDGER_UNKNOWN`
  (the owner ledger cannot be folded), `OWNER_OBLIGATION_MISSING` (no such id, including a renamed
  one) or `OWNER_UNDECIDED` (the owner entry is `OPEN`, `PARTIAL`, `DEFECT` or `BLOCKED`). An owner
  that is itself a derived mirror does not decide this one. The stored ledger, its counts and its
  high-water mark are unchanged.
- `TOL-V0-122`: (accepted under the owner's standing auto-accept (2026-10-11; V1-1966, GitHub
  #1214)) For each `OPEN` or `HELD` ticket with a ledger, the reads MUST report
  `MIRROR-DECIDED-ELSEWHERE` `{code, ticketId, completable, items}` listing its derived mirrors, and
  `MIRROR-OWNER-UNRESOLVED` `{code, ticketId, items}` listing pending mirrors whose owner ticket,
  ledger or obligation is missing or unreadable. `OWNER_UNDECIDED` is no finding. `queue status`
  carries them as `obligationMirrors`, and `ticket obligations worklist` carries them as
  `findings`; each member is present only when non-empty. The worklist excludes a derived mirror
  with `MIRROR_DECIDED_ELSEWHERE` before its other classes.
- `TOL-V0-123`: (accepted under the owner's standing auto-accept (2026-10-11; V1-1966, GitHub
  #1214)) A ledger is `completable` when it has a derived mirror and every pending entry is one.
  The dispatcher's native plan MUST defer such a ticket with `MIRRORS_DECIDED_ELSEWHERE`. The
  deferral is dispatcher-only, like `WORK_STATE_HELD`; `plan preview` and `claim` are unchanged.
  The roster MUST skip the ticket for every role, and the dispatcher MUST emit one `needs-owner`
  event when the reason first appears, so the owner completes the ticket.
- `TOL-V0-124`: (accepted under the owner's standing auto-accept (2026-10-11; V1-1966; GitHub
  #1177 follow-up)) The fold MUST count entries that leave `BLOCKED` for any state. This count is
  derived and never encoded. When it is positive, the TOL-V0-017 fingerprint adds the line
  `obligations-unblocked|<n>` after the `obligations` line; a ticket that never unblocked
  fingerprints byte-identically to before. An item leaving `BLOCKED` therefore releases a parked
  key at the next observation through the CAL-V0-057 unpark path. An observation whose ledger fold
  is unknown MUST NOT unpark or count as progress.
- `TOL-V0-125`: (accepted under the owner's standing auto-accept (2026-10-11; V1-1966; GitHub
  #1177 follow-up)) `doctor` MUST report `PARKED_WITH_OPEN_ITEMS` for each configured dispatcher's
  parked ticket key whose `OPEN` or `HELD` ticket has a ledger with fewer witnessed than total
  (non-deferred) items. `who` is the ticket and the detail names the dispatcher and the count.

### Witness receipt store (GitHub #1837)

A witness keeps only the Playwright report digest (TOL-V0-013). These requirements add an optional,
local, content-addressed store for the report bytes, so an auditor can resolve the digest a
witnessed obligation cites, and an audit that names each digest it cannot resolve.

- `TOL-V0-133`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2731; GitHub #1837) The witness receipt store MUST be the flat
  directory `<StateDir>/witness-receipts/` whose entries are regular files named by the lowercase
  SHA-256 hex of their bytes. It MUST be bounded: at most `MaxEvidenceBlobBytes` per file,
  100,000 files and `MaxEvidenceStoreBytes` in total; a put past either bound refuses
  `LIMIT_EXCEEDED`. A put checks the bounds after its own temp file is written and counts every
  temp file, so concurrent puts cannot together exceed them. A put in progress is a `.tmp-<hex>` file that a listing skips; any other
  non-digest entry, a symlink or a non-regular file refuses `MALFORMED`, never ignored. The store
  runs no daemon and uses no network.
- `TOL-V0-134`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2731; GitHub #1837) `receipt put FILE` MUST store FILE's bytes under
  their SHA-256 write-once: a new file is written to an exclusive temp file, synced and hard-linked
  to its digest name (`stored: NEW`); an identical stored file is verified, its modification time
  refreshed and reported `stored: EXISTING`; a stored file whose bytes no longer match its name
  refuses `MALFORMED` and is never overwritten. A put MUST NOT create the state directory or
  write under the journal; the result item is `retained` `{sha256, bytes, stored, path}`.
- `TOL-V0-135`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2731; GitHub #1837) `ticket obligations witness --from-playwright-report
  FILE --retain` MUST store the exact report bytes it
  validated, after checking they hash to the witnessed `reportSha256` and before the witness
  event is submitted, and only when the witness credits at least one obligation. The result item
  MUST carry `retained` as in TOL-V0-134. `--retain` without `--from-playwright-report` refuses as
  usage. A `run-batch` job witness does not retain in this slice. The ledger event wire format is unchanged: retention is recorded in the result item,
  and the store is the durable record that it happened.
- `TOL-V0-136`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2731; GitHub #1837) `receipt get SHA256` MUST write the stored bytes
  of the digest to stdout and nothing else, and `receipt path SHA256` MUST return `{sha256, bytes,
  path, verified: true}`. Both MUST open the stored file without blocking, require the opened
  descriptor to be the regular file the directory names, and re-hash the stored bytes on every read: an absent digest
  refuses `MISSING_EVIDENCE` and a mismatched or non-regular file `MALFORMED`. Both are read-only
  verbs: they create nothing, including the store directory.
- `TOL-V0-137`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2731; GitHub #1837) The journal audit inventory MUST admit the
  `witness-receipts` directory at the state root and MUST NOT read its files as journal objects,
  so a put or a witness with `--retain` leaves `receipt audit` consistent.
- `TOL-V0-138`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2731; GitHub #1837) `receipt audit --witness-receipts` MUST add a
  `witnessReceipts` section `{witnessedReports, resolved, findingsTotal, findingsTruncated,
  findings, storedFiles, citedDigests, uncitedStoredFiles}` and check, once per digest, every
  `WITNESSED` ledger entry whose evidence is a Playwright report. Each digest that is not a stored,
  digest-verified file is a finding `{ticketId, obligationId, code, reportSha256, reviewable:
  false, detail}` with `code` `RECEIPT_MISSING`, `RECEIPT_MISMATCH` or `RECEIPT_UNREADABLE`; a
  ledger that cannot be folded is a `LEDGER_UNREADABLE` finding with its `chainCode`, and a store
  that cannot be listed is a `STORE_UNREADABLE` finding added to, never in place of, the others. Any finding
  MUST make the audit `REFUSED` `MISSING_EVIDENCE`; findings list at most 256 rows with the total
  always complete. The flag is a read: it creates nothing. Without the flag the audit output is
  unchanged.
- `TOL-V0-139`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2731; GitHub #1837) `archive export` MUST carry the store as the flat
  `witness-receipts/<sha256>` entries, skip in-progress temp files, and fail closed on any other
  entry; `archive verify` and import treat it as a known layout directory.
- `TOL-V0-140`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2731; GitHub #1837) Retention MUST follow the obligation: `receipt
  compact` and `receipt retire` never touch the store, and `receipt prune [--dry-run]` MUST hold
  the writer lock from its ledger scan through its last removal (a dry run takes no lock), so no
  witness can cite a digest while prune removes it, and MUST keep
  every digest any current ledger entry cites, in any state, and every file whose modification
  time is within 15 minutes, so a witness between its put and its ledger write is not raced; the
  removal re-checks the time immediately before it deletes. Prune MUST remove nothing and refuse
  `MISSING_EVIDENCE` while any ledger cannot be folded. Prune also removes temp files older
  than 15 minutes, which only a put that died leaves. `--dry-run` lists and removes nothing. The
  result is `{dryRun, storedFiles, cited, keptRecent, removedTotal, removedTruncated, removed,
  removedStaleTemps}`.

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
| A qualification report that does not behave like the fixture (a soft failure credited, a retry credited, a step dropped) | The version would be trusted on assertion | `qualify-version` refuses `GATE_FAILED` with `OBLIGATION_QUALIFICATION_FAILED:` naming the first differing id, and records nothing (TOL-V0-061). |
| The allow-list file is edited by hand, truncated, oversize or written by a later build | A forged or unreadable list could admit or hide versions | Closed bounded reader: `MALFORMED`, or `UNSUPPORTED_VERSION` for a later profile, for every verb that reads it; never read as empty (TOL-V0-062). An entry admits its version only with the shipped fixture's digest and exact classes (TOL-V0-063). The file is local and admits only a version label; crediting still runs TOL-V0-010..013 against Git. |
| The suite lives in another repository | Every id is `unbound` `OUTSIDE_REPOSITORY`, or the commit is missing | A policy-declared `testRepositories` name moves commit resolution and source presence there (TOL-V0-064, 065); without it the refusals are unchanged (TOL-V0-067). |
| A witness names an undeclared test repository, or the declaration is later removed | Credit from an unknown checkout | The command and the writer refuse `OBLIGATION_TEST_REPOSITORY_UNDECLARED:`; the audit resolves the name from the policy posted at that receipt, and an undeclared name or unobservable repository is `INCONSISTENT` (TOL-V0-065, 066). |
| Secrets in report output | — | The report is never stored; the WITNESS event carries ids, paths and titles only, and they are secret-screened before storage; a detected secret refuses `SECRET_DETECTED`. |
| Ledger churn used to fake progress | Any revision write is progress today | The fingerprint uses the acceptance revision and the monotone high water (TOL-V0-016/017). |
| Evidence store missing a ledger event, or a broken `previous` chain | — | `show` reports `UNKNOWN`; receipt audit reports `INCONSISTENT`; nothing is treated as an empty or partial ledger. Counts on the record stay readable. |
| Witness or seed larger than the 64 KiB derived-event slot | — | Refuses `LIMIT_EXCEEDED` (`OBLIGATION_EVENT_TOO_LARGE:`) and writes nothing; the caller splits with `--ids` or several seeds (TOL-V0-002/009). |
| A stale WORKER process witnesses after reclamation | — | The payload names attempt and generation; a non-current generation refuses `FENCED` (TOL-V0-015). |
| Concurrent writers | — | The required CAS refuses the stale writer `REVISION_CONFLICT`; the request-id replay makes retries idempotent. |
| `test.fail` test that names an obligation | A failing expected-fail test reads as a failed obligation, and an ordinary obligation inside it can never pass | Expected-fail obligations are reported `defectConfirmed` with the defect id and error line and never credit (TOL-V0-022); any other obligation in the test is reported `mixedExpectedFail` with the remedy (TOL-V0-023); `preflight` flags it at file and line before the run (TOL-V0-025). |
| The run's post-check (lint, contract or consolidation script) failed | A passing report credits anyway | `--post-check` with a non-zero status refuses `GATE_FAILED` (`OBLIGATION_POST_CHECK_FAILED:`) quoting the log's first actionable line, and writes nothing (TOL-V0-024). |
| Obligations unnamed or split across tests, discovered only after a long run | A whole run is spent before the gap shows | `preflight` refuses `PREFLIGHT_FAILED` with each `UNNAMED`, `MIXED_EXPECTED_FAIL`, `SPLIT_TESTS`, `UNREADABLE_SPEC_PATH`, plan and `--deep` gate finding and its remedy, reading the commit's Git objects and writing no queue state (TOL-V0-025..027). The scan is heuristic: a name built only by `${}` interpolation or outside a string literal is not seen. |
| `preflight --deep` interrupted | A running gate and its checkout outlive preflight | Each gate's process group is killed when the gate exits, so no earlier gate's background process survives; `SIGINT`/`SIGTERM` kills the running gate's process group, or the post-gate Git check's, waits for it and removes the worktree before refusing (TOL-V0-027). Only `SIGKILL` can leave the worktree and its administrative entry; `git worktree prune` repairs it. A failed removal is reported, not ignored. |
| A batch obligation fails the same way run after run | Each run spends a lane and a session before anyone sees the repeat | `run-batch` refuses `LOOP_DETECTED` (`OBLIGATION_REPEAT_FAILURE:`) before any lane is acquired once two summaries at the same fixture digest record the same error (TOL-V0-031). |
| A batch summary that hides why an obligation did not move | — | Every ledger entry gets one cause, with the error line or the earlier failed step (TOL-V0-029). The cause is a heuristic for `NOT_REACHED`, `NOT_NAMED` and `NOT_OBSERVED`: it reads source names with the TOL-V0-025 scanner. |
| A flaky, partially failing or neighbour-breaking candidate looks final | A green final run hides a retry, a failed post-check or a broken neighbour | `qualify` is `NOT_QUALIFIED` with the named reason (TOL-V0-032); a neighbour failure the base shares is `PRE_EXISTING`, unless the neighbour run ended abnormally, which disqualifies on its own, or the base run did, which observes nothing. |
| A command leaves a FIFO, symlink or device in the results directory, or plants a results file | The job blocks on a FIFO, hashes `/dev/zero` forever, or overwrites the file a symlink points at | Reports and logs are read only as regular files without following or blocking; the manifest lists non-regular entries by type unopened and caps each file at 1 GiB; results files are created exclusively, and a planted one fails the job with nothing credited (TOL-V0-033); prior summaries are read through one bounded regular-file descriptor (TOL-V0-031). |
| The checkout moves or is interrupted under a job | A report from other code credits the commit | The job runs only a clean checkout of the commit, re-checks it after the run, and an interrupt or change credits nothing and fails qualification (TOL-V0-030, 033). An interrupt is honoured up to the witness submit; a submitted witness is atomic. A refused lane release fails the job. A command that changes untracked files only is not detected. |
| A migrated test is credited with a free-text "none found", or the legacy case it replaces changes after review | A reviewer cannot check the claim, and a false negative hides | Review sources pin the reviewed case or spec by digest, or record the query and the searched source ids. `requireSourceReview` makes preflight refuse missing sources or an unaudited `none`. `show` and `plan` list a changed digest as stale without demoting the state (TOL-V0-037..046). |
| An adopter suite names obligations in its own scheme (`M001`), so titles never hold the native id | Every title must be renamed or no obligation is credited | A seeded alias names its entry as a whole token in reports, steps, source, preflight and the audit, with every result keyed to the native id (TOL-V0-034..036). A duplicate alias, or one holding a native id, refuses at seed (TOL-V0-035); an alias inside a longer token never matches, and a defect-shaped alias is never read as a defect. |
| Older binary meets an alias-bearing seed event | — | Its closed seed reader refuses the retained request, so `show` is `UNKNOWN` and the audit `INCONSISTENT` for that ticket (fail-closed); see Rollout and rollback. |
| A preflight replays a report for a tree, untracked file or check build it was not computed on | A stale "ready" hides a new finding | The key covers the commit, `HEAD`, the tracked diff, every non-ignored untracked file and the executable digest; any read that cannot be keyed runs the checks (`SKIPPED`), and `--fresh` forces a run (TOL-V0-047, 048). |
| The cache directory grows without bound, or holds a planted or corrupted entry | Disk fills, or a forged report replays | Entries are bounded at 256 and 7 days; a non-regular, oversize, malformed, future-dated or wrong-key entry is a miss; a replayed item is untrusted and never evidence (TOL-V0-049, 050). |
| An agent loops on an identical failing preflight or batch | Lanes and tokens are spent on a result already known | Opt-in `--refuse-repeat N` refuses the next run at an unchanged input digest, naming the signature and count; a changed input or an audited, single-use `--remediation` admits it (TOL-V0-051..053). |
| The failure ledger is unreadable or wrong | A loop guard that refuses on bad state would block honest work | An unreadable ledger or input digest runs the job unguarded (`SKIPPED`); the ledger is private, opt-in and never evidence (TOL-V0-054). |
| Older binary meets a record with the member | — | The closed reader refuses the record (fail-closed); see Rollout and rollback. |
| An adopter's ids are longer than 64 bytes, or a plan must re-run witnessed rows and skip optional ones | A real ledger cannot migrate without renaming ids or faking plan entries | Aliases reach 128 bytes and the refusal names the limit (TOL-V0-055); `rerun` lists witnessed regression rows, and only core `OPEN` or `PARTIAL` rows must be planned (TOL-V0-056, 057). |
| A partly witnessed row is recorded as `OPEN` or `WITNESSED` | Progress is hidden, or overstated | `PARTIAL` is pending and never counts as witnessed (TOL-V0-058). |
| Witness provenance is wrong or holds a secret | A misleading or leaked reference is kept with the evidence | Provenance is reported only, never evidence the writer checks; its prose is secret-screened (TOL-V0-059). |
| A provenance flag is given an empty value (an unset shell variable) | The witness writes without the provenance the caller meant to record | Every given flag is validated; an empty value refuses `MALFORMED` before any write (TOL-V0-059). |
| A long dotted adopter id has three dot-separated runs of 10 or more bytes | The shared secret screen reads it as token-shaped and refuses the seed `SECRET_DETECTED` | Retained limit: the screen is shared and unchanged here; such an id needs a different separator. |
| A reviewer verdict comes from the ticket's author, or the cited example changes after review | Self-review or a stale review credits an obligation | Reviewers listed in the derived authors refuse; `show` reports a changed subject blob stale and un-credited until a new review, judged in the repository the review read, at the ticket's candidate branch when one resolves (TOL-V0-074, 077, 131); an unresolvable test repository abstains (`TEST_REPOSITORY_UNRESOLVED`). Retained limit: stored counts, gates and the dispatcher fingerprint do not see read-time staleness. |
| An agent moves an obligation out of the core count to raise a completion rate | The rate counts less work while looking complete | `core` moves through a recorded proposal that a different actor accepts as OWNER; `requireScopeDecision` refuses the plain core change `APPROVAL_MISSING` (TOL-V0-081, 082). Retained limit: actor ids and roles are recorded claims (decision 0003), so one operator acting under two ids is not detected. |
| An obligation cites no code, or a repository that cannot be listed | A missing boundary is read as "no tests" | The verdict is `UNKNOWN` (`NO_CITATIONS`, `REPOSITORY_UNAVAILABLE`), never `NO_TEST_BOUNDARY` (TOL-V0-083). The test classifier is a naming heuristic: a matching file no runner executes still counts. |
| A worker re-queues an entry whose new candidate is not yet reviewed, or works from a stale review | Duplicate work, or work against changed bytes | The worklist excludes `CANDIDATE_AWAITING_REVIEW`, `SUBJECT_CHANGED` and `SUBJECT_UNVERIFIED` entries (TOL-V0-086). |
| A test repository's `HEAD` is unborn, or the reviewed commit is gone | An unborn `HEAD` reads as a stale review; a reviewer cannot see what was reviewed | An unborn `HEAD` is `TEST_REPOSITORY_UNRESOLVED` with the credit withheld (TOL-V0-100); `reviewedSubject` names the reviewed commit and `REVIEWED_COMMIT_UNAVAILABLE` when it is gone (TOL-V0-101). |
| A cite names a mistyped repository label | The obligation reads `UNKNOWN` and no one notices | `requireDeclaredCites`, default true (owner decision 2026-10-19, V1-1963), refuses it `OBLIGATION_CITE_REPOSITORY_UNDECLARED:` (TOL-V0-102). Retained limit: an explicit false opts out, and the label then reads `UNKNOWN`. |
| A project's tests use a naming convention outside the closed set | The boundary reports `NO_RUNNABLE_TESTS` for a tested repository | Declared `testFilePatterns` extend the set (TOL-V0-103). Retained limit: still a naming heuristic, never a runner probe. |
| A plan check counts only browser coverage, or a stale review as coverage | API or unit witnesses are re-demanded; a changed test still counts | `--witnesses` names every ledger witness and its type, and a stale review refuses `STALE_REVIEW` (TOL-V0-104, 105). |
| A capability question is answered from `HEAD` | A feature merged but not deployed reads as present | The capability read answers only at a declared baseline and abstains `UNKNOWN` without one (TOL-V0-107..109). |
| An obligation is decided on another ticket | The copy stays `OPEN`; the dispatcher keeps offering a ticket whose work is done | `satisfiedBy` derives the owner's terminal state with the owner's event as its source; `MIRROR-DECIDED-ELSEWHERE` lists it; a ticket with only decided mirrors is deferred and offered to the owner (TOL-V0-120..123). |
| A mirror's owner is missing, renamed, unreadable or in another queue | The mirror silently reads as open work | It stays open with `OWNER_TICKET_MISSING`, `OWNER_LEDGER_MISSING`, `OWNER_LEDGER_UNKNOWN` or `OWNER_OBLIGATION_MISSING`, listed as `MIRROR-OWNER-UNRESOLVED` (TOL-V0-121, 122). Retained limit: resolution is one hop and same-queue only. |
| A ticket parked with every item `BLOCKED` has an item unblocked | It stays parked: the fingerprint did not change | The unblock line changes the fingerprint and the existing unpark releases the key; an unknown fold never unparks; doctor names a parked ticket with open items (TOL-V0-124, 125). |

| A witnessed report's digest cannot be resolved | An auditor cannot review the report a witness credited | `receipt audit --witness-receipts` names each unresolvable digest as an unreviewable typed finding and refuses `MISSING_EVIDENCE`; `--retain` stores the bytes at witness time (TOL-V0-135, 138). |
| A stored receipt is tampered, or a cited receipt is pruned | The digest resolves to other bytes, or the evidence a live witness cites disappears | Every read re-hashes (`RECEIPT_MISMATCH`, `MALFORMED`); a put never overwrites; prune keeps every cited and recently put digest and refuses while a ledger is unfoldable (TOL-V0-134, 136, 140). |

## Non-goals and simpler baseline

- Gating completion, gates or acceptance on obligations. V0 only records and reports; whether
  `complete` should require every core obligation `WITNESSED` was resolved no in V0 (owner decision 1,
  decision 0456).
- Running tests, launching Playwright or editing reports, except through the experimental
  `run-batch` and `qualify` verbs (TOL-V0-028..033), which run only argv the operator configured in
  a local file and credit only through the unchanged witness path. They are not a test framework,
  install no browser or Playwright, schedule nothing, and own no claim, renewal or HANDOFF.
- `qualify` crediting obligations, recording gate results or gating completion; its verdict pack is
  local, untrusted evidence only.
- Consuming PWP-V0 qualified receipts. The `/2` redaction profile retains steps, so a later
  `VERIFIED_RECEIPT` source could credit from a receipt Corvint itself produced; it is excluded here.
- Playwright tags or annotations as id carriers in the report; other test frameworks; the blob or
  html reporters. (TOL-V0-025 reads `<contract>:<id>` string literals and an `isolated`
  annotation from spec source for preflight only; they never credit.)
- The `fp-ledger.py` `splits` heuristic (same spec, describe and project without an `isolated`
  annotation) as a witness or plan rule. The plan check reports SPLIT per obligation instead;
  TOL-V0-025 reports a source-level `SPLIT_TESTS` finding in `preflight` only.
- Running preflight automatically at pool acquire or claim. That hook is a follow-up (GitHub #713);
  V0 offers the explicit read only.
- Dispatcher role predicates that match on witnessed counts.
- A foreign-queue importer carrying the member.
- Storing reports, screenshots or traces.
- Caching `preflight --deep` gate runs, `run-batch`, `qualify` or witness results, sharing the
  preflight cache across machines, or reading it as evidence (TOL-V0-047..050).
- A default-on failure guard, a guard for `qualify` or `ticket obligations witness`, judging
  whether a declared remediation is real, or sharing the failure ledger across machines
  (TOL-V0-051..054).
- Per-step progress counts, a step list for `PARTIAL`, checking provenance against Git or the
  report, or treating a rerun of a witnessed row as a new witness (TOL-V0-055..059).
- Reviewer verdicts gating completion, an agreement rule other than unanimous `correct`, verifying
  that a reviewer label names a real person (TOL-V0-073..078), or storing a staleness judgement
  (TOL-V0-131, 132).
- Refusing plain core changes by default, running a test provider to prove it runnable, or ranking by anything but priority, nearest
  completion and review age (TOL-V0-079..088).
- Running Playwright to qualify a version, shipping a qualified version, sharing the allow-list
  across machines, or qualifying from anything but the embedded fixture (TOL-V0-060..063).
- Running `run-batch`, `qualify` or `preflight` in a declared test repository, several test
  repositories per witness, or fetching or cloning one (TOL-V0-064..067).

- Checking cite labels under an explicit `requireDeclaredCites` false, probing that a declared
  pattern's files are run by any
  runner, judging environment capability from anything but a declared ref or object id, fetching
  a baseline, or reading deployment state; `preflight --plan` does not take `--witnesses`
  (TOL-V0-100..109).
- Writing a mirror's state into its ledger, chaining mirrors beyond one hop, resolving owners in
  another queue, completing a ticket on the owner's behalf, or checking the owner at write time
  (TOL-V0-120..123).

- Storing `DECLARED` manifest bytes, recording retention in the ledger event, fetching a missing
  report, or proving a retained report came from a real run; the store resolves a digest, it does
  not qualify the run (TOL-V0-133..140).

The simpler baseline stays available: a body checklist plus the external script. This profile is
justified only if the owner wants the queue and dispatcher to see proven progress natively.

## Acceptance evidence and traceability

V1-1022 implements the profile as experimental under the owner acceptance recorded in decision
0456 (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-08-v1-1022-obligation-ledger.md`). Delivered evidence is focused Go
tests on synthetic Playwright json reports. The integrated items in the last column and the
TOL-V0-019 plain-language status line remain `NOT_RUN`. The live Playwright 1.63.0 fixture passed
on 2026-10-20 (TOL-V0-128), so the shipped qualified version list holds `1.63.0` (TOL-V0-126).

| Requirement | Ticket acceptance | Implementation boundary | Delivered evidence | Required integrated evidence (NOT_RUN) |
|---|---|---|---|---|
| TOL-V0-001..004 | V1-1022 | `internal/tasks/wire` (optional key), `internal/tasks/ticket` (record codec), `internal/taskman` (Core reader), `internal/tasks/snapshot` (derived-event slot) | `TestTOLV0001_RecordMemberRoundTrip`, `TestTOLV0002_EventCanonicalAndChained`, `TestTOLV0002_EventSlotBound` (65,536-byte boundary), `TestTOLV0003_EntryCodecRefusals`, `TestTOLV0004_EvidenceCodec` (`internal/tasks/ticket`); `TestTOLV0001_ImportRecordRefusesObligations` (`internal/tasks/ticket`); `TestTOLV0001_ReaderAdmitsObligations`, `TestTOLV0001_ReaderRefusesNonNativeObligations` (`internal/taskman`); `TestTOLV0002_OversizedWriteRefused`, `TestTOLV0002_FoldFromRetainedRequestsAfterRestart` (one event per MUTATE), `TestTOLV0003_UpdatedAtIsIssuedAtNotRecordedAt` (`internal/tasks/cli`); legacy byte identity in `TestTOLV0001_RecordMemberRoundTrip` and `TestTOLV0019_QueueStatusLegacyIdentity` | none beyond the live row below |
| TOL-V0-005..008, 014, 015 | V1-1022 | `internal/tasks/mutation` (payloads, Apply), `internal/tasks/intent` (grants, policy key), `internal/tasks/transaction` (adopt/import guards, claim checks), `internal/tasks/cli` | `TestTOLV0005_SeedPrefixAndDuplicates`, `TestTOLV0006_ShowIsReadOnly`, `TestTOLV0007_SetTransitions`, `TestTOLV0008_DeclaredWitness`, `TestTOLV0014_RevisionOnlyWrite`, `TestTOLV0014_WitnessReplay` (CAS, replay, HELD admitted, COMPLETED refused), `TestTOLV0015_RoleMatrix`, `TestTOLV0015_WorkerStaleGenerationFenced` (same actor after reclamation) (`internal/tasks/cli`) | native archive round trip; two-process CAS; interrupted-commit redo (`NOT_RUN`) |
| TOL-V0-009..013 | V1-1022 | report reader `internal/tasks/obligation` (no Node dependency), secret screen, receipt audit (`internal/tasks/transaction/obligation_audit.go`) | `TestTOLV0009_ReportAdmissionAndRetention`, `TestTOLV0009_SubsetIgnoresExcludedMatchBound`, `TestTOLV0010_StepOwnErrorCredits`, `TestTOLV0010_Retry0Only`, `TestTOLV0011_ConflictingMatches`, `TestTOLV0012_SourcePresence`, `TestTOLV0013_CreditMismatchAndAudit`, `TestTOLV0013_AuditAfterReportDeleted`, `TestTOLV0013_DeclaredWitnessAudit`, `TestTOLV0013_DeclaredCommitAudited`, `TestTOLV0013_TamperedWorkerGenerationInconsistent` (`internal/tasks/cli`), all on synthetic json reports | live Playwright 1.63 fixture on a PWP-V0-008 tuple producing each case (retained 2026-10-20 through `qualify-version`, TOL-V0-127, 128) |
| TOL-V0-016..018, 021 | V1-1022 | `internal/tasks/dispatch` (roster, stall, ledger version, status), `internal/tasks/transaction/loop_detect.go` | `TestTOLV0016_HighWaterMonotone` (`internal/tasks/cli`); `TestTOLV0017_FingerprintLegacyIdentity`, `TestTOLV0017_LedgerChurnIsNotProgress`, `TestTOLV0017_HighWaterRaiseIsProgress`, `TestTOLV0021_StallRestartsOnRaise`, `TestTOLV0021_PreviousLedgerVersionAdopted` (`internal/tasks/dispatch`); `TestTOLV0018_LastRaiseEndsNoProgressRun` (`internal/tasks/transaction`) | live dispatcher run (`NOT_RUN`) |
| TOL-V0-022..024 (decision 0484) | GitHub #712 | `internal/tasks/obligation` (report reader, `expected_fail.go`), `internal/tasks/cli` (witness flags and lists) | `TestTOLV0022_ExpectedFailDefectConfirmed`, `TestTOLV0023_MixedExpectedFail`, `TestTOLV0024_PostCheckRefusesCredit` (`internal/tasks/cli`), on synthetic json reports | live Playwright 1.63 `test.fail` fixture (`NOT_RUN`) |
| TOL-V0-025..027 (decision 0484) | GitHub #713 | `internal/tasks/obligation/source.go` (scanner), `internal/tasks/store/preflight.go` (Git reads, deep worktree), `internal/tasks/cli/preflight.go` | `TestTOLV0025_ScannerSkipsCommentsRegexAndInterpolation` (`internal/tasks/obligation`); `TestTOLV0025_DescribeTitleIDsReachNestedTests` (`internal/tasks/obligation`); `TestTOLV0025_PreflightRefusesUnnamedMixedAndSplit`, `TestTOLV0025_PreflightCleanTicketPasses`, `TestTOLV0025_PreflightPathNarrowsTheTreeListing`, `TestTOLV0025_PreflightSkipsOversizedSpecUnread`, `TestTOLV0025_PreflightRefusesALineBreakSpecPath`, `TestTOLV0026_PreflightPlanCheck`, `TestTOLV0027_PreflightDeepRunsGatesInCleanWorktree`, `TestTOLV0027_PreflightDeepRunsNoRepositoryHooks`, `TestTOLV0027_PreflightDeepGateThatChangesSourceFails`, `TestTOLV0027_PreflightDeepInterruptRetiresGateAndWorktree`, `TestTOLV0027_PreflightDeepInterruptRetiresEarlierGateDescendants`, `TestTOLV0027_PreflightDeepRetiresAFinishedGatesDescendants`, `TestTOLV0027_PreflightDeepInterruptStopsAHungStatus`, `TestTOLV0027_PreflightDeepStatusPastTheGateTimeoutFails`, `TestTOLV0027_PreflightDeepCleanStatusRetiresItsGroup` (`internal/tasks/cli`); `TestTOLV0027_PreflightStatusStopsAtFirstEntry`, `TestTOLV0027_PreflightBoundedCancelDuringRetireSignalsBeforeReap` (`internal/tasks/store`) | preflight against a real Playwright suite (`NOT_RUN`) |
| TOL-V0-028..033 (decision 0485) | GitHub #714 | `internal/tasks/obligation/batch.go` (outcomes, causes, named tests), `internal/tasks/store/batch_run.go` (command runner, checkout state, fixture digest, detached worktree), `internal/tasks/cli/run_batch.go`, `internal/tasks/cli/qualify.go` | `TestTOLV0028_RunConfigIsClosed`, `TestTOLV0029_RunBatchCreditsAndWritesCauses` (WITNESSED, FAILED with the step error, NOT_REACHED, NOT_NAMED, request id reuse), `TestTOLV0029_RunBatchPrepFailureIsNotReached`, `TestTOLV0030_RunBatchRefusesAStaleOrDirtyCheckout`, `TestTOLV0031_RepeatFailureRefusedBeforeLaneAcquire` (refusal before acquire, `--ids` subset on a pool lane with capture and release, new fixture evidence admits), `TestTOLV0032_QualifyVerdicts` (qualified, retry, candidate failure, post-check, new and pre-existing neighbour failure, static check, manifest, no leftover worktree, retried pre-existing neighbour, neighbour top-level errors with and without a failing test), `TestTOLV0030_RefusedReleaseFailsTheJob`, `TestTOLV0030_AcquireRefusalCodeIsTheJobs`, `TestTOLV0031_RootFixturePathIsDigested`, `TestTOLV0033_InterruptBeforeWitnessSubmitCreditsNothing`, `TestTOLV0033_CaptureRunsAfterAnInterruptDuringTests`, `TestTOLV0033_ManifestStreamsAnOversizedArtifact`, `TestTOLV0033_ReportPathMustBeARegularFile` (a FIFO report, a FIFO or changed report swapped in by the capture, qualify), `TestTOLV0033_ManifestRecordsNonRegularEntriesByType` (symlink to `/dev/zero`, FIFO, 1025 MiB sparse file), `TestTOLV0033_PlantedSummarySymlinkIsRefused`, `TestTOLV0033_ZeroCreditInterruptIsLaneFailed`, `TestTOLV0032_AbnormalNeighbourRunDisqualifies` (neighbour and base timeouts), `TestTOLV0033_InterruptDuringLastManifestHashRefuses`, `TestTOLV0031_PriorSummaryReadIsBoundToItsDescriptor` (a prior summary that grows past 4 MiB or is swapped for a FIFO after its check) (`internal/tasks/cli`), all with a fake shell test command that writes Playwright-shaped json; the interrupt tests replace the job context, the release refusal is injected, and the manifest-hash and prior-summary swap points are test hooks; the FIFO and symlink tests skip on Windows | a real Playwright suite under `run-batch` and `qualify`; a real `SIGINT`/`SIGTERM` delivered to a running job; untracked-file change detection (`NOT_RUN`) |
| TOL-V0-034..036 (accepted, decision 0519) | V1-1177 (GitHub #792) | `internal/tasks/wire/obligations.go` (alias grammar), `internal/tasks/ticket/obligations.go` (seed and entry codec, `admitAlias`), `internal/tasks/obligation/names.go` (alias whole-token matching), report, batch and source readers, `internal/tasks/transaction/obligation_audit.go`, `internal/tasks/mutation/obligations.go` (screen) | `TestTOLV0034_SeedWithAliases` (aliased and alias-free entries, later aliased seed, grammar refusals, audit), `TestTOLV0035_AliasDuplicateAndCollision` (in one seed, against the ledger, a ledger id, its own id, an id inside, an other-prefix alias admitted), `TestTOLV0036_PreflightMatchesAliases` (test title, `test.step` title, contract annotation, alias inside longer tokens stays `UNNAMED`), `TestTOLV0036_WitnessCreditsByAliasKeyedToNativeID` (credit, evidence and lists keyed to native ids, defect-shaped alias in a `test.fail` step, audit `CONSISTENT`) (`internal/tasks/cli`); `TestTOLV0036_AliasWholeToken`, `TestTOLV0036_AliasMasksDefectIDs`, `TestTOLV0036_ContractAliasAndNamedTests` (`internal/tasks/obligation`), on synthetic json reports | the adopter's suite and a live Playwright 1.63 report naming aliases (`NOT_RUN`) |
| TOL-V0-037..046 (accepted, owner's standing auto-accept 2026-10-11) | GitHub #813 | `internal/tasks/ticket/obligations.go` (review-source codec, set and entry), `internal/tasks/intent/policy.go` (`requireSourceReview`), `internal/tasks/obligation/review.go` (preflight findings, legacy-cases reader, digest comparison), `internal/tasks/cli/preflight.go`, `internal/tasks/cli/obligation_sources.go` (show and plan `sourceReview`), `internal/tasks/mutation/obligations.go` (screen) | `TestTOLV0037_ReviewSourceCodec`, `TestTOLV0039_LegacyLedgerBytesUnchanged` (`internal/tasks/ticket`); `TestTOLV0039_PolicyLeavesLedgerBytes`, `TestTOLV0040_PreflightRequiresSourceReview` (missing, blank query, empty searched, DEFERRED skipped, secret screen, policy off), `TestTOLV0042_StaleSourcesListed` (legacy-case, manual-case and legacy-spec staleness, WITNESSED kept, unresolved reasons, plan `ok` unchanged, no state written, schema, trailing-data and repeated-member refusal), `TestTOLV0043_LegacyCasesFIFORefused` (`internal/tasks/cli`) | a live test-management export imported and reviewed against a migrated suite (`NOT_RUN`) |
| TOL-V0-047..050 (accepted, owner's standing auto-accept 2026-10-11) | V1-1594 (GitHub #1037) | `internal/tasks/store/worktree_state.go` (working-tree digest), `internal/tasks/cli/preflight_cache.go` (key, entry codec, replay, prune), `internal/tasks/cli/preflight.go` | `TestTOLV0047_PreflightCacheReplaysIdenticalInputs` (HIT replay, untracked, tracked, commit and implementation misses), `TestTOLV0048_PreflightFreshBypassesTheCache` (FRESH replace, refused replay, usage, SKIPPED), `TestTOLV0049_PreflightCacheIsBounded` (age miss, count and age pruning, foreign file kept), `TestTOLV0050_PreflightCacheStaysOutsideTheRepository` (`internal/tasks/cli`) | measured hit rate and saved time over a day of agent sessions (`NOT_RUN`) |
| TOL-V0-051..054 (accepted, owner's standing auto-accept 2026-10-11) | V1-1605 (GitHub #1048) | `internal/tasks/cli/failure_signatures.go` (flags, signature, ledger, audit), `internal/tasks/cli/failure_lock_unix.go` (ledger lock), `internal/tasks/cli/preflight.go`, `internal/tasks/cli/run_batch.go` | `TestTOLV0051_FailureLedgerIsOffByDefault` (off by default, flag validation, RECORDED count, CLEARED pass), `TestTOLV0052_RepeatedSignatureRefusesNamingSignatureAndCount` (typed refusal naming signature, count and digest; changed input admits), `TestTOLV0052_RunBatchRefusesRepeatedSignature` (refused before the test command and results directory, across `--out` dirs), `TestTOLV0053_RemediationClearsOnceWithAudit` (audit line, reuse refused, cache bypass), `TestTOLV0054_LedgerStaysPrivateAndFailsOpenToRunning` (no repository or store change, mode 0700, SKIPPED), `TestTOLV0053_RemediationAllowsOneRunAtAnyThreshold` (N=3), `TestTOLV0051_SignatureKeepsStructuralIDs` (AC-1 and AC-2 apart, run-specific numbers folded), `TestTOLV0054_ConcurrentRunsSerializeTheLedger` (eight concurrent failures, one reference given twice), `TestTOLV0051_RunBatchDigestCoversEnvironmentAndPool` (`internal/tasks/cli`) | measured loop reduction across agent sessions (`NOT_RUN`) |
| TOL-V0-055..059 (accepted, owner's standing auto-accept 2026-10-11) | V1-1649 (GitHub #1071) | `internal/tasks/wire/obligations.go` (alias limit), `internal/tasks/ticket/obligations.go` (`PARTIAL`, `ObligationPending`, provenance codec, witness fold), `internal/tasks/ticket/obligation_plan.go` (`rerun`, `RERUN_NOT_WITNESSED`, core-only `UNASSIGNED`), `internal/tasks/cli/obligations.go` (provenance flags, `reruns`), `internal/tasks/cli/preflight.go` (remedy), `internal/tasks/mutation/obligations.go` (screen), the pending checks in `internal/tasks/obligation` and `internal/tasks/cli/run_batch.go` | `TestTOLV0055_AliasLimit128` (110-row seed with 78-byte aliases, 128 admitted, 129 refused naming the limit, audit), `TestTOLV0056_0057_PlanRerunAndNonCore` (110-row plan with witnessed reruns and omitted non-core, `BLOCKED` and `DEFECT` rows passes; `RERUN_NOT_WITNESSED`, listed `SPLIT`, core `UNASSIGNED`, both-lists `MALFORMED`, old output unchanged), `TestTOLV0058_PartialNeverWitnessed` (counts, reason required, plan, witness credits), `TestTOLV0059_WitnessProvenance` (per-entry provenance, absent member kept, bad OID, secret) (`internal/tasks/cli`); `TestTOLV0020_PlanCheck` and `TestTOLV0026_PreflightPlanCheck` updated for TOL-V0-057; `TestTOLV0034_SeedWithAliases` refuses 129 bytes | the adopter's 4,562-row ledger migrated end to end (`NOT_RUN`) |
| TOL-V0-060..067 (accepted, owner's standing auto-accept 2026-10-11) | V1-1648 (GitHub #1070) | `internal/tasks/obligation/obligation-qualification.spec.ts` (fixture), `internal/tasks/obligation/qualification.go` (`Qualify`, allow-list codec), `internal/tasks/obligation/report.go` (any-version qualification read), `internal/tasks/cli/obligation_qualify.go` (`qualify-version`), `internal/tasks/cli/obligations.go` (`--test-repository`, admitted versions), `internal/tasks/cli/run_batch.go` (admitted versions), `internal/tasks/intent/policy.go` (`obligations.testRepositories`), `internal/tasks/ticket/obligations.go` (`testRepository` member), `internal/tasks/mutation/obligations.go` (writer check), `internal/tasks/transaction/obligation_audit.go`, `internal/tasks/store/external_review.go` and `internal/tasks/store/obligations.go` (`ExactWorktreeRoot`; audit in the test repository) | `TestTOLV0060_FixtureExpectedClasses`, `TestTOLV0061_QualifyMatchesFixture` (soft parent, retry-0, sibling, expected-fail differences; passing, failed and defect matches outside the one fixture file), `TestTOLV0062_AllowListCodec` (round trip, newer profile, unknown member, 65th version), `TestTOLV0062_AllowListReadRefusesSymlinkAndFIFO`, `TestTOLV0063_AllowListAdmitsOnlyFixtureEvidence` (`internal/tasks/obligation`); `TestTOLV0060_PrintFixture`, `TestTOLV0061_QualificationRefusals`, `TestTOLV0062_AllowListRecorded`, `TestV11745_QualifyVersionHoldsTheStoreLock`, `TestTOLV0063_QualifiedVersionAdmitted`, `TestTOLV0063_ForgedAllowListEntryNotAdmitted`, `TestTOLV0063_RunBatchAdmitsStoreLocalQualifiedVersion` (run-batch with a version only the store-local allow-list admits credits; before qualify-version the job refuses `GATE_FAILED` per TOL-V0-029 carrying `OBLIGATION_REPORT_VERSION_UNQUALIFIED:` and writes nothing; V1-1747), `TestTOLV0064_PolicyTestRepositories`, `TestTOLV0064_DeclaredDirectoryMustBeWorktreeRoot`, `TestTOLV0065_SeparateTestRepository1_64` (AC3), `TestTOLV0065_WriterBindsDeclaredPath`, `TestTOLV0066_AuditInTestRepository`, `TestTOLV0066_AuditRequiresExactTestRepositoryRoot`, `TestTOLV0067_UndeclaredRefusalsUnchanged` (`internal/tasks/cli`); `TestCALV0131_EveryLiveFormatRefusesANewerVersion` classifies `taskman-obligation-qualifications/0` | a live Playwright run of the fixture recorded by `qualify-version` (retained 2026-10-20, TOL-V0-128); run-batch with a store-local version (code path only, `NOT_RUN`) |
| TOL-V0-073..078 (accepted, owner's standing auto-accept 2026-10-11) | V1-1886 (GitHub #1170) | `internal/tasks/ticket/obligation_review.go` (review codec, outcome), `internal/tasks/ticket/obligations.go` (source, evidence, entry, fold), `internal/tasks/mutation/obligations.go` (authority, screen), `internal/tasks/cli/obligations.go` (`--reviewed`, `--review`, `--exclude-author`, `witnessReview`) | `TestTOLV0074_ReviewCodec`, `TestTOLV0075_ReviewOutcome`, `TestTOLV0076_ReviewFold` (`internal/tasks/ticket`); `TestTOLV0073_ReviewedWitnessCreditsOnAgreement`, `TestTOLV0075_HeldVerdictsKeptWithReasons`, `TestTOLV0077_EditedExampleMakesReviewStale`, `TestTOLV0077_TestRepositoryReviewStaleness` (a test-repository review is judged in that repository: fresh with the store copy removed, stale after the test repository's copy changes, `TEST_REPOSITORY_UNRESOLVED` and uncredited once it no longer resolves), `TestTOLV0074_AuthorReviewerRefused`, `TestTOLV0075_ReviewAuthorSet` (the reviewed-witness author set `reviewAuthorSet`: implement members across pools unioned with `--exclude-author` by member label, review/integrate members and other tickets excluded, `INDEPENDENCE_UNVERIFIED` until an explicit exclusion covers an unrecorded generation; V1-1924) (`internal/tasks/cli`) | reviews by real independent reviewers on an adopter ledger (`NOT_RUN`) |
| TOL-V0-079..083 (accepted, owner's standing auto-accept 2026-10-11) | V1-1913 (GitHub #1180) | `internal/tasks/ticket/obligation_scope.go` (cites, scope, fold), `internal/tasks/mutation/obligations.go` (ACCEPT authority, `requireScopeDecision`, screen), `internal/tasks/intent/policy.go`, `internal/tasks/obligation/boundary.go`, `internal/tasks/cli/obligation_boundary.go` (`boundary`, `outOfScope`) | `TestTOLV0079_CitesBoundsReplaceAndDefault`, `TestTOLV0080_ScopeShape`, `TestTOLV0083_NoTestBoundaryScopeDecisionAndRescope`, `TestTOLV0082_RequireScopeDecisionRefusesPlainCore` (`internal/tasks/cli`); `TestTOLV0083_BoundaryTestKind`, `TestTOLV0083_BoundaryVerdictDirectoryCite` (`internal/tasks/obligation`) | a boundary read on an adopter repository without tests (`NOT_RUN`) |
| TOL-V0-084..088 (accepted, owner's standing auto-accept 2026-10-11) | V1-1915 (GitHub #1182) | `internal/tasks/ticket/obligation_scope.go` (candidate), `internal/tasks/ticket/obligation_review.go` (finding), `internal/tasks/obligation/worklist.go`, `internal/tasks/cli/obligation_boundary.go` (`worklist`) | `TestTOLV0084_CandidateShapeAndClear`, `TestTOLV0086_WorklistBatchCandidateAndNeedsText`, `TestTOLV0085_FindingNeedsShortfallVerdict` (`internal/tasks/cli`); `TestTOLV0087_RankPriorityThenNearest` (`internal/tasks/obligation`) | a worker draining a real worklist (`NOT_RUN`) |
| TOL-V0-100..109 (accepted, owner's standing auto-accept 2026-10-11) | V1-1945, V1-1946, V1-1949 (GitHub #1202) | `internal/tasks/cli/obligation_review.go` (`reviewHeadStates`), `internal/tasks/cli/obligation_subject.go` (`reviewedSubject`, plan witnesses, `capability`), `internal/tasks/cli/obligation_boundary.go`, `internal/tasks/cli/obligations.go` (`plan --witnesses`), `internal/tasks/mutation/obligations.go` (`requireDeclaredCites`), `internal/tasks/intent/policy.go`, `internal/tasks/obligation/boundary.go` (`BoundaryTestKindWith`) | `TestTOLV0100_UnbornTestRepositoryHeadUnresolved`, `TestTOLV0101_ReviewedSubjectAtReviewedCommit`, `TestTOLV0102_DeclaredCitesRequired`, `TestTOLV0103_DeclaredTestFilePatterns`, `TestTOLV0104_PlanCountsLedgerWitnesses`, `TestTOLV0105_PlanStaleReviewedWitness`, `TestTOLV0106_UnassignedNamesConsideredWitness`, `TestTOLV0107_CapabilityAtEnvironmentBaseline`, `TestTOLV0107_CapabilityUndeclaredPrimary`, `TestTOLV0108_CapabilityBoundsAndReadOnly` (`internal/tasks/cli`); `TestTOLV0103_BoundaryTestKindWith` (`internal/tasks/obligation`); `TestTOLV0102_103_107_ObligationsPolicyMembers` (`internal/tasks/intent`); 12 mutations killed (build log) | a capability read against a live deployment ref and a non-browser adopter plan (`NOT_RUN`); a baseline commit whose tree cannot be listed, an over-bound blob, and `SUBJECT_ABSENT/UNREADABLE/MISMATCH_AT_REVIEW` (reachable only through a damaged object store) have no fixture (`NOT_RUN`) |
| TOL-V0-120..125 (accepted, owner's standing auto-accept 2026-10-11) | V1-1966 (GitHub #1214; #1177 follow-up) | `internal/tasks/ticket/obligation_mirror.go`, `internal/tasks/ticket/obligations.go` (`satisfiedBy`, `Unblocks`), `internal/tasks/obligation/mirror.go`, `internal/tasks/obligation/worklist.go`, `internal/tasks/cli/obligation_mirror.go`, `internal/tasks/cli/obligations.go`, `internal/tasks/cli/obligation_boundary.go`, `internal/tasks/cli/cli.go` (`obligationMirrors`), `internal/tasks/cli/dispatch.go`, `internal/tasks/transaction/plan.go`, `internal/tasks/dispatch/roster.go`, `internal/tasks/dispatch/loop.go`, `internal/tasks/cli/doctor_sessions.go` | `TestTOLV0120_SatisfiedByRecordedAndCleared`, `TestTOLV0124_UnblocksCounted` (`internal/tasks/ticket`); `TestTOLV0121_ResolveMirrors`, `TestTOLV0122_MirrorFindings`, `TestTOLV0123_MirrorsCompletable` (`internal/tasks/obligation`); `TestTOLV0123_MirrorsDecidedLeaveWindow` (`internal/tasks/transaction`); `TestTOLV0123_MirrorsDecidedIsNotDispatched`, `TestTOLV0124_UnblockReleasesParkedTicket` (`internal/tasks/dispatch`); `TestTOLV0120_MirrorReadsAndAcceptance`, `TestTOLV0124_UnblockObservedAndDoctorReportsParked` (`internal/tasks/cli`) | a live dispatcher run over a store with mirrors (`NOT_RUN`); cross-queue owners are reported missing, never resolved |
| TOL-V0-133..140 (accepted, owner's standing auto-accept 2026-10-11) | V1-2731 (GitHub #1837) | `internal/tasks/evidencestore/store.go`, `internal/tasks/cli/witness_receipts.go`, `internal/tasks/cli/receipt.go` (`--witness-receipts`), `internal/tasks/cli/obligations.go` (`--retain`), `internal/tasks/cli/run_batch.go` (`readJobReportRaw`), `internal/tasks/obligation/report.go` (`ReadReportRaw`), `internal/tasks/journal/audit.go`, `internal/tasks/archive/layout.go` | `TestTOLV0133_ListSkipsTempRefusesUnknown`, `TestTOLV0134_PutWriteOnce`, `TestTOLV0136_GetVerifiesAndCreatesNothing`, `TestTOLV0140_RemoveKeepsRefreshed` (`internal/tasks/evidencestore`); `TestTOLV0135_WitnessRetainResolves`, `TestTOLV0135_RetainNeedsReport`, `TestTOLV0138_AuditReportsUnresolvable` (also TOL-V0-137), `TestTOLV0139_ArchiveCarriesReceipts`, `TestTOLV0140_PruneKeepsCited` (`internal/tasks/cli`) | a retaining witness of a live Playwright run and a store at its file or byte bound (`NOT_RUN`); `run-batch` jobs do not retain |
| TOL-V0-132 (accepted, owner's standing auto-accept 2026-10-11) | V1-2248 | `internal/tasks/cli/obligation_subject.go` (`planWitnesses`), `internal/tasks/cli/obligations.go` (`obligationsPlan`), `internal/tasks/cli/obligation_review.go` (`staleBasisValue`) | `TestTOLV0132_PlanWitnessJudgedAtCandidate` (HEAD basis without a candidate; an edit on HEAD only stays `COVERED` at the candidate; an edit on the candidate is `REVIEW_STALE_AT_HEAD` at `CANDIDATE_BRANCH`) (`internal/tasks/cli`) | an operator plan checked across a diverging candidate (`NOT_RUN`) |
| TOL-V0-131 (accepted, owner's standing auto-accept 2026-10-11) | V1-1922 | `internal/tasks/cli/obligation_review.go` (`reviewHeadStates`, `staleBasis`), `internal/tasks/cli/obligations.go` (`reviewCandidate`) | `TestTOLV0131_StalenessJudgedAtCandidate` (HEAD basis without a candidate; an edit on HEAD only leaves the review fresh and credited at the candidate; an edit on the candidate only makes it stale) (`internal/tasks/cli`) | an operator ledger reviewed across a candidate that diverges from `HEAD` (`NOT_RUN`) |
| TOL-V0-126 (accepted, owner's standing auto-accept 2026-10-11) | V1-1022 | `internal/tasks/obligation/report.go` (`qualifiedVersions`), `internal/tasks/cli/command_help.go` | `TestTOLV0126_ShippedVersionAdmitted` (`internal/tasks/cli`) | none beyond TOL-V0-128 |
| TOL-V0-127 (accepted, owner's standing auto-accept 2026-10-11) | V1-1022 | `internal/tasks/cli/obligation_qualify_live_test.go` | `TestTOLV0127_LiveFixtureQualifies` (`internal/tasks/cli`), live; a mutation that credits retry 1 instead of retry 0 made it fail `OBLIGATION_QUALIFICATION_FAILED` (TOLQ-7) | a Linux tuple run (`NOT_RUN`) |
| TOL-V0-128 (accepted, owner's standing auto-accept 2026-10-11) | V1-1022 | live evidence | `TestTOLV0127_LiveFixtureQualifies` passed on macOS arm64, Node v22.23.2, `@playwright/test` 1.63.0 and on 1.64.0 (build log 2026-10-20) | none |
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

The additions TOL-V0-073..078 add the optional `review` payload and entry members and the
`REVIEWED` source. A ledger that never received a reviewed witness keeps its exact bytes; after one
is written, an older binary refuses that ledger at the closed reader, and recovery is to run the
newer binary or restore the store from a backup taken before the first reviewed witness.
TOL-V0-131 stores nothing: rollback is reverting its code and requirement row, after which
staleness is judged at `HEAD` again and the additive `staleBasis` member disappears. TOL-V0-132
likewise stores nothing; reverting it returns plan witness statuses to `HEAD` and drops their
`staleBasis`.

The additions TOL-V0-133..140 add the `witness-receipts/` state directory and the read-side
`retained` and `witnessReceipts` members; the ledger, the journal and their wire formats are
unchanged. Deleting the directory is always safe: the audit then reports each witnessed digest
`RECEIPT_MISSING`. An older binary refuses the unknown `witness-receipts` entry in the journal
audit and in `archive export`, so rollback is reverting the change and moving the directory
out of the state directory (keep it to restore later); no ledger or journal migration is needed.

The additions TOL-V0-079..088 add the optional `cites`, `scope` and `candidate` change and entry
members, the optional verdict `finding` and the optional policy key `requireScopeDecision`; the
`boundary` and `worklist` verbs are reads. A ledger that never received one of these members keeps
its exact bytes. After one is written, an older binary refuses that request `MALFORMED` at its
closed reader, so the ledger is `UNKNOWN` and its audit `INCONSISTENT`, never silently stripped.
Rollback is reverting the change and removing the policy key before any such write; after one,
run the newer binary or restore the store from a backup taken before it. The same CAL-V0-131
profile question as TOL-V0-034 applies.

The additions TOL-V0-022..027 add no stored state: the `defectConfirmed` and `mixedExpectedFail` lists and the
`preflight` verb are read-side output, and the witness event is unchanged, so rolling them back is
reverting the change.

The additions TOL-V0-028..033 add no stored state beyond the witness event the unchanged
witness path writes. Their summaries, logs, reports and verdict packs live only under the
operator's `--out` directory, and pool acquire and release use the existing verbs. Rolling them back
is reverting the change; existing results directories become inert files.

The accepted TOL-V0-034..036 keep the `taskman-obligation-event/0` profile, the record member and
the wire shape of every alias-free ledger; only a seed that carries an alias writes new bytes, in
its retained request and in the folded entry. This follows the TOL-V0-001 precedent (decision
0456) of an optional member read by closed readers: an older binary refuses the alias-bearing seed
request `MALFORMED` at its closed reader, so that ticket's ledger is `UNKNOWN` and its audit
`INCONSISTENT`, never silently stripped. Rollback is reverting the change before any seed carries
an alias; after that, run the newer binary or restore the store from a backup taken before the
first aliased seed. Whether CAL-V0-131 instead requires a `/1` event or seed profile for this
optional member is an open owner question: with `/0` kept, the adjacent-compatibility format sets
do not change, so that check cannot detect an old binary meeting an aliased seed.

The accepted TOL-V0-037..046 follow the same optional-member precedent. Only a set change that
carries `sources` writes new bytes; an older binary refuses that request `MALFORMED` at its closed
reader, so the ledger is `UNKNOWN`, never silently stripped. The `requireSourceReview` policy key
and the `sourceReview` output add no stored state. Rollback is reverting the change before any set
carries sources; after that, run the newer binary or restore the store from a backup taken before
the first sources write. The same CAL-V0-131 profile question applies.

The accepted TOL-V0-047..050 add no repository, queue or trace state. The cache directory under
the user cache directory is the only new file state; it is opt-in per call and holds derived
reports only. Rollback is reverting the change and, optionally, deleting
`corvint-tasks/preflight` from the user cache directory; an older binary never reads it.

The accepted TOL-V0-051..054 add no repository, queue or trace state and are off unless a call
gives `--refuse-repeat`. The ledger directory and its `remediations.jsonl` under the user cache
directory are the only new file state. Rollback is reverting the change and, optionally, deleting
`corvint-tasks/failure-signatures`; an older binary never reads it, and deleting it only forgets
counts (keep `remediations.jsonl` if the audit trail is wanted).

The accepted TOL-V0-055..059 keep the `taskman-obligation-event/0` and `taskman-obligation-plan/0`
profiles. Only a seed with an alias of 65..128 bytes, a set to `PARTIAL`, or a witness with
`provenance` writes new bytes, in the retained request and the folded entry; a plan with `rerun`
is a read-only input. This follows the TOL-V0-034 optional-member precedent: an older binary
refuses such a request `MALFORMED` at its closed reader, so that ticket's ledger is `UNKNOWN` and
its audit `INCONSISTENT`, never silently stripped. Rollback is reverting the change before any
such write; after one, run the newer binary or restore the store from a backup taken before it.
The same CAL-V0-131 profile question as TOL-V0-034 applies.

The accepted TOL-V0-060..067 add no journal or ticket profile. `qualify-version` writes only
`taskman-obligation-qualifications.json` beside the store (classified under CAL-V0-131); deleting it
returns every report to the shipped list, so later reports of that version refuse
`UNSUPPORTED_VERSION` again while ledgers already credited keep their evidence. Only a witness with
`--test-repository` writes new bytes (the optional `testRepository` member of the retained request
and the folded evidence). As with TOL-V0-034 and TOL-V0-059, an older binary refuses such a request
`MALFORMED` at its closed reader, so that ledger is `UNKNOWN` and its audit `INCONSISTENT`, never
silently stripped; a policy with `obligations.testRepositories` is likewise refused by an older
policy reader. Rollback is reverting the change and removing the policy member before any such
witness; after one, run the newer binary or restore the store from a backup taken before it.

The accepted TOL-V0-100..109 add no journal, record, event or plan profile and no stored ledger
bytes: `reviewedSubject`, `ledgerWitnesses`, `considered` and the `capability` item are read-side
output, and `ledgerWitnesses` and `considered` appear only under `--witnesses`. The policy members
`requireDeclaredCites`, `testFilePatterns` and `environmentBaselines` are optional; an older policy
reader refuses a policy that carries one (fail-closed). `requireDeclaredCites` defaults to true
(owner decision 2026-10-19, V1-1963): a store whose ledger relied on writing undeclared cite labels
sets it false to keep the pre-amendment write; existing ledger bytes are never re-checked. Rollback is reverting the change and
removing those policy members; no store state needs repair. The TOL-V0-080 scope evidence bound
was raised from an implemented 256 bytes to the accepted 512 (build log 2026-10-19); a ledger
holding a 257..512-byte evidence reference is refused `LIMIT_EXCEEDED` by an older binary's closed
reader, so its ledger is `UNKNOWN`, never silently stripped, and recovery is to run the newer
binary.

The accepted TOL-V0-120..125 add no journal, record, event or plan profile. Only a `set` change
with `satisfiedBy` writes new bytes (the optional member of the retained request and the folded
entry). An older binary refuses such a request `MALFORMED` at its closed reader, so that ledger is
`UNKNOWN` and its audit `INCONSISTENT`, never silently stripped; clearing the mirror with `null`
leaves the earlier event in the chain, so recovery is to run the newer binary or restore the store
from a backup taken before the first mirror. The unblock count is fold-derived: the fingerprint of
a ticket that had an item leave `BLOCKED` changes once when the newer binary first observes it,
which at most releases a parked key one time. The `mirrors`, `findings` and `obligationMirrors`
members, the `MIRRORS_DECIDED_ELSEWHERE` deferral and `PARKED_WITH_OPEN_ITEMS` are read-side and
leave with a revert.
