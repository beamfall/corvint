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
and are experimental.
The profile reuses unchanged the ticket, mutation,
receipt and fold boundaries of the [agent lease contract](corvint-tasks-agent-leases-v0.md), the
reference-plus-evidence-event shape of [operator notes](corvint-tasks-operator-notes-v0.md)
(ON-V0-002), the revision-only write of [evidence attachments](corvint-tasks-evidence-attachments-v0.md)
(TEA-V0-001) and the WORKER opt-in of [know-how notes](corvint-tasks-know-how-notes-v0.md)
(KHN-V0-021..023). Playwright facts come from the
[Playwright provider contract](playwright-external-provider-v0.md) and a scratch observation recorded
in the build log.

## Agent digest
- Claim: A native ticket can carry a bounded ledger of named obligations that a Playwright json report witnesses step by step, so progress counts proof, not sessions.
- Status: accepted (decisions 0456, 0484, 0485 and 0519; V1-1022): TOL-V0-001..021 by decision 0456 (owner issue #680), TOL-V0-022..027 by decision 0484 (GitHub #712, #713) and TOL-V0-028..033 by decision 0485 (GitHub #714); TOL-V0-034..036 (obligation aliases, GitHub #792, V1-1177) by decision 0519; TOL-V0-037..046 (review sources, GitHub #813) accepted under the owner's standing auto-accept (2026-10-11), implemented with focused tests; TOL-V0-047..050 (preflight result cache, GitHub #1037, V1-1594) accepted under the same standing auto-accept, implemented with focused tests; TOL-V0-051..054 (failure-signature ledger, GitHub #1048, V1-1605) accepted under the same standing auto-accept, implemented with focused tests; experimental; TOL-V0-001..018 and 020..036 implemented with focused tests (TOL-V0-028..033 on a fake test command); the TOL-V0-019 plain-language line and the live Playwright 1.63 fixture are NOT_RUN.
- Exists: the experimental `ticket obligations seed|show|witness|set|plan` verbs, the read-only `preflight` verb (TOL-V0-025..027), the experimental `run-batch` and `qualify` verbs (TOL-V0-028..033), the record member, the `internal/tasks/obligation` report reader, the receipt audit and the fingerprint, stall and loop integration; the reused mutation, evidence-store, fingerprint (CAL-V0-057), stall (CAL-V0-185) and loop (CAL-V0-102) paths.
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
  - An alias is 1..64 bytes of `[A-Za-z0-9._:-]` whose first and last bytes are letters or digits.
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
    already proves), the TOL-V0-031 fixture digest, the config sha256, the sorted `--spec` paths,
    the canonical ledger entries, the `--ids` scope and the role.
  - A failure is a refused preflight, or a batch whose outcome is not OK or that records a
    `FAILED` obligation. Its signature is the sha256 of its sorted failure lines (each preflight
    finding; each `FAILED` obligation's id and error, the `NOT_RUN` reason and the outcome codes
    of a batch) with every digit run folded to `#` and whitespace collapsed.
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
  - The admitted run resets the count, so the next identical failure is refused again. A
    reference already spent on the entry refuses `LOOP_DETECTED` with
    `FAILURE_REMEDIATION_REUSED:`; more than 16 references on one entry refuse `LIMIT_EXCEEDED`.
  - A preflight the remediation admits runs its checks even under `--cache`.
- `TOL-V0-054`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #1048)) The ledger MUST live in
  `corvint-tasks/failure-signatures` under the user cache directory, created mode 0700, beside
  the TOL-V0-050 cache and outside every repository, Tasks store and trace store, so both verbs
  keep product invariant 4. Each entry is one closed `corvint-tasks-failure-signature/0`
  document of at most 64 KiB written atomically; entries older than 30 days are empty and are
  pruned, with at most 1,024 kept. Only the verb that wrote an entry reads it, to decide whether
  to run; it is never evidence, a witness, a gate result or authority. An input digest or ledger
  that cannot be read runs the job unguarded with status `SKIPPED` and the reason, and a failed
  ledger write is reported in `reason`.

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
| TOL-V0-022..024 (decision 0484) | GitHub #712 | `internal/tasks/obligation` (report reader, `expected_fail.go`), `internal/tasks/cli` (witness flags and lists) | `TestTOLV0022_ExpectedFailDefectConfirmed`, `TestTOLV0023_MixedExpectedFail`, `TestTOLV0024_PostCheckRefusesCredit` (`internal/tasks/cli`), on synthetic json reports | live Playwright 1.63 `test.fail` fixture (`NOT_RUN`) |
| TOL-V0-025..027 (decision 0484) | GitHub #713 | `internal/tasks/obligation/source.go` (scanner), `internal/tasks/store/preflight.go` (Git reads, deep worktree), `internal/tasks/cli/preflight.go` | `TestTOLV0025_ScannerSkipsCommentsRegexAndInterpolation` (`internal/tasks/obligation`); `TestTOLV0025_DescribeTitleIDsReachNestedTests` (`internal/tasks/obligation`); `TestTOLV0025_PreflightRefusesUnnamedMixedAndSplit`, `TestTOLV0025_PreflightCleanTicketPasses`, `TestTOLV0025_PreflightPathNarrowsTheTreeListing`, `TestTOLV0025_PreflightSkipsOversizedSpecUnread`, `TestTOLV0025_PreflightRefusesALineBreakSpecPath`, `TestTOLV0026_PreflightPlanCheck`, `TestTOLV0027_PreflightDeepRunsGatesInCleanWorktree`, `TestTOLV0027_PreflightDeepRunsNoRepositoryHooks`, `TestTOLV0027_PreflightDeepGateThatChangesSourceFails`, `TestTOLV0027_PreflightDeepInterruptRetiresGateAndWorktree`, `TestTOLV0027_PreflightDeepInterruptRetiresEarlierGateDescendants`, `TestTOLV0027_PreflightDeepRetiresAFinishedGatesDescendants`, `TestTOLV0027_PreflightDeepInterruptStopsAHungStatus`, `TestTOLV0027_PreflightDeepStatusPastTheGateTimeoutFails`, `TestTOLV0027_PreflightDeepCleanStatusRetiresItsGroup` (`internal/tasks/cli`); `TestTOLV0027_PreflightStatusStopsAtFirstEntry`, `TestTOLV0027_PreflightBoundedCancelDuringRetireSignalsBeforeReap` (`internal/tasks/store`) | preflight against a real Playwright suite (`NOT_RUN`) |
| TOL-V0-028..033 (decision 0485) | GitHub #714 | `internal/tasks/obligation/batch.go` (outcomes, causes, named tests), `internal/tasks/store/batch_run.go` (command runner, checkout state, fixture digest, detached worktree), `internal/tasks/cli/run_batch.go`, `internal/tasks/cli/qualify.go` | `TestTOLV0028_RunConfigIsClosed`, `TestTOLV0029_RunBatchCreditsAndWritesCauses` (WITNESSED, FAILED with the step error, NOT_REACHED, NOT_NAMED, request id reuse), `TestTOLV0029_RunBatchPrepFailureIsNotReached`, `TestTOLV0030_RunBatchRefusesAStaleOrDirtyCheckout`, `TestTOLV0031_RepeatFailureRefusedBeforeLaneAcquire` (refusal before acquire, `--ids` subset on a pool lane with capture and release, new fixture evidence admits), `TestTOLV0032_QualifyVerdicts` (qualified, retry, candidate failure, post-check, new and pre-existing neighbour failure, static check, manifest, no leftover worktree, retried pre-existing neighbour, neighbour top-level errors with and without a failing test), `TestTOLV0030_RefusedReleaseFailsTheJob`, `TestTOLV0030_AcquireRefusalCodeIsTheJobs`, `TestTOLV0031_RootFixturePathIsDigested`, `TestTOLV0033_InterruptBeforeWitnessSubmitCreditsNothing`, `TestTOLV0033_CaptureRunsAfterAnInterruptDuringTests`, `TestTOLV0033_ManifestStreamsAnOversizedArtifact`, `TestTOLV0033_ReportPathMustBeARegularFile` (a FIFO report, a FIFO or changed report swapped in by the capture, qualify), `TestTOLV0033_ManifestRecordsNonRegularEntriesByType` (symlink to `/dev/zero`, FIFO, 1025 MiB sparse file), `TestTOLV0033_PlantedSummarySymlinkIsRefused`, `TestTOLV0033_ZeroCreditInterruptIsLaneFailed`, `TestTOLV0032_AbnormalNeighbourRunDisqualifies` (neighbour and base timeouts), `TestTOLV0033_InterruptDuringLastManifestHashRefuses`, `TestTOLV0031_PriorSummaryReadIsBoundToItsDescriptor` (a prior summary that grows past 4 MiB or is swapped for a FIFO after its check) (`internal/tasks/cli`), all with a fake shell test command that writes Playwright-shaped json; the interrupt tests replace the job context, the release refusal is injected, and the manifest-hash and prior-summary swap points are test hooks; the FIFO and symlink tests skip on Windows | a real Playwright suite under `run-batch` and `qualify`; a real `SIGINT`/`SIGTERM` delivered to a running job; untracked-file change detection (`NOT_RUN`) |
| TOL-V0-034..036 (accepted, decision 0519) | V1-1177 (GitHub #792) | `internal/tasks/wire/obligations.go` (alias grammar), `internal/tasks/ticket/obligations.go` (seed and entry codec, `admitAlias`), `internal/tasks/obligation/names.go` (alias whole-token matching), report, batch and source readers, `internal/tasks/transaction/obligation_audit.go`, `internal/tasks/mutation/obligations.go` (screen) | `TestTOLV0034_SeedWithAliases` (aliased and alias-free entries, later aliased seed, grammar refusals, audit), `TestTOLV0035_AliasDuplicateAndCollision` (in one seed, against the ledger, a ledger id, its own id, an id inside, an other-prefix alias admitted), `TestTOLV0036_PreflightMatchesAliases` (test title, `test.step` title, contract annotation, alias inside longer tokens stays `UNNAMED`), `TestTOLV0036_WitnessCreditsByAliasKeyedToNativeID` (credit, evidence and lists keyed to native ids, defect-shaped alias in a `test.fail` step, audit `CONSISTENT`) (`internal/tasks/cli`); `TestTOLV0036_AliasWholeToken`, `TestTOLV0036_AliasMasksDefectIDs`, `TestTOLV0036_ContractAliasAndNamedTests` (`internal/tasks/obligation`), on synthetic json reports | the adopter's suite and a live Playwright 1.63 report naming aliases (`NOT_RUN`) |
| TOL-V0-037..046 (accepted, owner's standing auto-accept 2026-10-11) | GitHub #813 | `internal/tasks/ticket/obligations.go` (review-source codec, set and entry), `internal/tasks/intent/policy.go` (`requireSourceReview`), `internal/tasks/obligation/review.go` (preflight findings, legacy-cases reader, digest comparison), `internal/tasks/cli/preflight.go`, `internal/tasks/cli/obligation_sources.go` (show and plan `sourceReview`), `internal/tasks/mutation/obligations.go` (screen) | `TestTOLV0037_ReviewSourceCodec`, `TestTOLV0039_LegacyLedgerBytesUnchanged` (`internal/tasks/ticket`); `TestTOLV0039_PolicyLeavesLedgerBytes`, `TestTOLV0040_PreflightRequiresSourceReview` (missing, blank query, empty searched, DEFERRED skipped, secret screen, policy off), `TestTOLV0042_StaleSourcesListed` (legacy-case, manual-case and legacy-spec staleness, WITNESSED kept, unresolved reasons, plan `ok` unchanged, no state written, schema, trailing-data and repeated-member refusal), `TestTOLV0043_LegacyCasesFIFORefused` (`internal/tasks/cli`) | a live test-management export imported and reviewed against a migrated suite (`NOT_RUN`) |
| TOL-V0-047..050 (accepted, owner's standing auto-accept 2026-10-11) | V1-1594 (GitHub #1037) | `internal/tasks/store/worktree_state.go` (working-tree digest), `internal/tasks/cli/preflight_cache.go` (key, entry codec, replay, prune), `internal/tasks/cli/preflight.go` | `TestTOLV0047_PreflightCacheReplaysIdenticalInputs` (HIT replay, untracked, tracked, commit and implementation misses), `TestTOLV0048_PreflightFreshBypassesTheCache` (FRESH replace, refused replay, usage, SKIPPED), `TestTOLV0049_PreflightCacheIsBounded` (age miss, count and age pruning, foreign file kept), `TestTOLV0050_PreflightCacheStaysOutsideTheRepository` (`internal/tasks/cli`) | measured hit rate and saved time over a day of agent sessions (`NOT_RUN`) |
| TOL-V0-051..054 (accepted, owner's standing auto-accept 2026-10-11) | V1-1605 (GitHub #1048) | `internal/tasks/cli/failure_signatures.go` (flags, signature, ledger, audit), `internal/tasks/cli/preflight.go`, `internal/tasks/cli/run_batch.go` | `TestTOLV0051_FailureLedgerIsOffByDefault` (off by default, flag validation, RECORDED count, CLEARED pass), `TestTOLV0052_RepeatedSignatureRefusesNamingSignatureAndCount` (typed refusal naming signature, count and digest; changed input admits), `TestTOLV0052_RunBatchRefusesRepeatedSignature` (refused before the test command and results directory, across `--out` dirs), `TestTOLV0053_RemediationClearsOnceWithAudit` (audit line, reuse refused, cache bypass), `TestTOLV0054_LedgerStaysPrivateAndFailsOpenToRunning` (no repository or store change, mode 0700, SKIPPED) (`internal/tasks/cli`) | measured loop reduction across agent sessions (`NOT_RUN`) |
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
