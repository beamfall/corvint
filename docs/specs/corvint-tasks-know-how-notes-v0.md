# Corvint Tasks know-how notes V0

Owner: Russell Lewis
Date: 2026-10-07
Intent status: accepted by decision 0443 (V1-0955); KHN-V0-021..023 proposed (V1-0987; GitHub #671)
Delivery status: experimental

Authoritative inputs: GitHub issue beamfall/corvint#655 and the agent-filed native ticket V1-0955
("cross-ticket know-how notes"), implemented on orchestrator instruction on 2026-10-07. The
requirements below were accepted by the owner in decision 0443. The issue's adopter context
stays in the issue. The write path reuses the
[evidence attachments profile](corvint-tasks-evidence-attachments-v0.md) pattern (TEA-V0-001: an
optional record member changed only by an ordinary receipt-backed mutation). The claim surface is
the one in the [agent lease contract](corvint-tasks-agent-leases-v0.md), and it is unchanged.

## Agent digest
- Claim: Agents record cross-ticket know-how notes pinned to file blobs; reads compute STALE/UNKNOWN freshness and claims deliver intersecting notes as untrusted data.
- Status: accepted by decision 0443 (V1-0955); KHN-V0-021..023 proposed (V1-0987; GitHub #671); experimental; `ticket know-how add|retract|list`, the optional `knowHow` record member and claim delivery exist with focused tests.
- Exists: record/Core codecs, KNOWHOW_ADD/KNOWHOW_RETRACT through Apply, write-time secret screen and blob pins, read-time freshness from one batched Git call, a 2 KiB claim projection and an authority boundary test. Proposed: an opt-in policy `knowHow.workerAdd` lets the claim holder add a scoped note as WORKER (KHN-V0-021..023).
- Blocked on: owner questions 5, 6 and 8 (V1-0964, V1-0962); archive round trip, concurrency and redo witnesses; Core packet delivery is a non-goal here.
- Read next: Requirements; Owner questions; Acceptance evidence and traceability; Rollout and rollback.

## User and current state

An agent that finishes a ticket often learns something the next agent on the same files needs.
Examples are a build step the docs omit, a flaky fixture, or a file that must change in lockstep
with another. Without this profile the knowledge has three homes:
- the completed ticket's body, which no later claim reads;
- an operator note, which is advisory text for one ticket and has no file anchors;
- a Markdown memory file, which is outside the audited store and never says when the code it
  describes has changed.

None of these is delivered to the next claim on the same files, and none says whether the note
still describes the current code.

This profile adds a small cross-ticket record. It holds a bounded note anchored to repository files
that are pinned to Git blobs. The note is written through the ordinary mutation path. A claim of a
ticket whose `effects.touchPaths` intersect the anchors receives it, together with freshness
computed at read time.

The simpler baseline is the body of a Markdown memory file. It has no anchors, no freshness, no
screen and no audited history, and it does not reach a claim.

## Requirements

- `KHN-V0-001`: `ticket know-how add` MUST pin every `--anchor PATH` to the Git blob that path
  names in the writer's commit, and MUST record that commit in the entry. The commit is `HEAD` of
  the working directory, or `--commit OID` so that a retry can reproduce the payload. Each anchor
  path is a repository-relative file. The shared Path grammar refuses absolute paths and paths
  that contain `..`, and a trailing-`/` directory prefix is also refused. The command screens its
  arguments (KHN-V0-004) before any parse or pin error can echo them. One `git cat-file
  --batch-check` process resolves the commit first and then every blob as `<commit oid>:<path>`,
  so a concurrent commit cannot mix two commits into one pin set. A missing path, a
  non-blob object or an unresolvable revision is refused (MALFORMED), and nothing is written. A
  note is never pinned to an object the writer's commit does not hold.
- `KHN-V0-002`: A ticket record MAY carry the optional member `knowHow`. When present it is a
  non-empty append-order array of at most 32 closed entries, and it is omitted when empty, so a
  record without notes keeps its legacy bytes.
  - An ADD entry is `{seq, operation, text, anchors, routes, commit, supersedes, reason, attempt,
    generation, evidencePath, actor, recordedAt}`. It has nonblank text of 1..1024 bytes, 1..4
    `{blob, path}` anchors sorted by unique path, and 0..4 sorted unique route or flow tokens of
    at most 64 bytes each.
  - A RETRACT entry is `{seq, operation, note, reason, actor, recordedAt}`.
  - `seq` is the 1-based position.
  - A superseding ADD and every RETRACT name an earlier ADD that is still active, and carry a
    nonblank reason of 1..512 bytes. A non-superseding ADD carries no reason.
  - `actor` is `{id, role}`, with role OWNER or OPERATOR, or WORKER under KHN-V0-023.
  - `attempt`, `generation` and `evidencePath` are optional writer-asserted provenance, except
    that a WORKER entry's attempt and generation are verified (KHN-V0-022).

  The Tasks codec and the Core ticket reader MUST both enforce these rules and refuse anything
  else.
- `KHN-V0-003`: `KNOWHOW_ADD` (`{text, anchors, routes, commit, supersedes, reason, attempt,
  generation, evidencePath}`) and `KNOWHOW_RETRACT` (`{note, reason}`) MUST be ordinary
  receipt-backed mutations with a required expectedRevision CAS and request-id replay.
  - The home ticket is the ticket written to. It must be live (DRAFT, OPEN or HELD) and NATIVE;
    otherwise the write is BLOCKED/TICKET_STATE.
  - Each write appends exactly one entry, with actor, role and time taken from the trusted binding
    and clock. It never rewrites or drops an earlier entry.
  - It bumps `revision` and leaves `acceptanceRevision`, status and gates unchanged, so a live
    attempt is neither blocked nor invalidated.
  - Supersede and retract only target an active ADD on the same home ticket.
  - A 33rd entry is VALIDATION_FAILED/LIMIT_EXCEEDED.
  - OWNER holds both operations by default. OPERATOR holds them only through an explicit
    `policy.roles.OPERATOR` row, and no policy roles row can grant them to any other role.
    WORKER may hold `KNOWHOW_ADD` only through KHN-V0-021.
  - ADOPT_FILE refuses `knowHow` as a protected field, and IMPORT_APPLY refuses an imported
    record that adds, rewrites or drops entries.
- `KHN-V0-004`: Before an entry is appended, the writer MUST screen the text, the reason, the
  route tokens, the anchor paths and the evidence path with the shared Core secret screen. A hit
  is refused as VALIDATION_FAILED/MALFORMED, and the detail starts with the stable prefix
  `KNOWHOW_SECRET_DETECTED:`. The detail names the field and never the matched text.
- `KHN-V0-005`: Freshness MUST be computed at read time against the reader's committed `HEAD`,
  and it is never stored. One Git process resolves the reader's `HEAD` commit first and then
  every distinct anchor path as `<commit oid>:<path>`, so every state is relative to the reported
  `head`; uncommitted edits are ignored.
  - For each anchor: a blob equal to the pin is CURRENT and a different blob is STALE. A missing
    path, a non-blob object, an unborn `HEAD`, no checkout or any Git failure is UNKNOWN, never
    CURRENT.
  - A note is STALE when any anchor is STALE, else UNKNOWN when any anchor is UNKNOWN, else
    CURRENT.
- `KHN-V0-006`: `claim` and `claim --next` MUST add a `knowHow` member to their result.
  - The member holds the active notes of every ticket whose anchors intersect the claimed
    ticket's `effects.touchPaths`. An anchor matches a touchPath that equals it, or a touchPath
    that ends in `/` and is a prefix of it.
  - Notes are ordered CURRENT, then UNKNOWN, then STALE; newest first within a state; then by
    ticket id and note seq.
  - The member is a compact projection: ticket, note, freshness, text, anchor paths with their
    states, routes and time.
  - It keeps the longest ordered prefix of notes such that the canonical encoding of the whole
    member, including `trust`, `head`, `matched`, `omitted`, `state` and the read hint given when
    notes were left out, fits 2048 bytes.
  - The member is computed when the response is built, so a replay shows current freshness. An
    unreadable inventory makes it `UNAVAILABLE` with a code and a warning, and never fails the
    committed claim.
  - `ticket know-how list [--path PATH ...] [--ticket T] [--limit N]` is a pure read of the same
    selection in full form, with pins, provenance and supersession, capped at N notes (default 50,
    at most 200). It takes no lock and writes nothing.
- `KHN-V0-007`: Every delivered projection MUST carry `trust: UNTRUSTED_AGENT_AUTHORED_DATA`, and
  `list` sets the result's `untrusted` flag. Notes are agent-authored data, never instructions,
  acceptance, evidence or authority. No ranking, planning, dispatch, gate, completion, review,
  release or Core context path may read the member. The set of production files that name it is
  fixed by a test.

- `KHN-V0-021`: Policy MAY carry the optional closed member `knowHow: {workerAdd: boolean}`.
  Absent or `false`, a WORKER `KNOWHOW_ADD` MUST stay refused exactly as before the key existed:
  UNAUTHORIZED with no code and the detail `outside hypothetical role subset`, with nothing
  written. `true` adds `KNOWHOW_ADD`, and nothing else, to WORKER's default row; a
  `policy.roles.WORKER` row still limits WORKER and then outranks the key. `KNOWHOW_RETRACT`,
  every other WORKER write and every OWNER, OPERATOR, REVIEWER, IMPORTER and SYSTEM behaviour are
  unchanged. A policy roles row naming `KNOWHOW_ADD` for WORKER stays refused at policy load. An
  unknown member, a missing or non-boolean `workerAdd`, or a non-object value is MALFORMED.
  Policy-declared anchor prefixes are not part of this key. The store screens a WORKER
  `KNOWHOW_ADD` before the lock against the policy in the committed intent tree, so with the key
  absent or false the refusal still precedes the store checks and §5.2 recovery; an opted-in tree
  is checked again under the lock against the canonical policy record. After the key is removed,
  an identical retry of a committed WORKER add is therefore refused rather than replayed; the note
  stays. Status: proposed (V1-0987; GitHub #671).
- `KHN-V0-022`: A WORKER `KNOWHOW_ADD` admitted by KHN-V0-021 MUST be checked, after request
  replay and before the KHN-V0-003 status, entry-cap and KHN-V0-004 secret checks, against the
  attempt record the transaction layer reads from the audited journal under the store lock. In
  order, each refusal names its case with a stable detail prefix and reuses a closed §11 code:
  - a non-null `supersedes`: UNAUTHORIZED, `KNOWHOW_WORKER_SUPERSEDE:`;
  - a null `attempt` or `generation`: VALIDATION_FAILED/MALFORMED,
    `KNOWHOW_WORKER_ATTEMPT_REQUIRED:`;
  - an absent attempt, an attempt with no lease, a terminal phase, an expired lease or a
    generation other than the attempt's: REVISION_CONFLICT/FENCED,
    `KNOWHOW_WORKER_ATTEMPT_STALE:`;
  - a lease holder other than the binding id (the claim `--holder` must equal
    `CORVINT_TASKS_ACTOR`): UNAUTHORIZED, `KNOWHOW_WORKER_ATTEMPT_FOREIGN:`;
  - an attempt for another ticket than the target: BLOCKED/OUT_OF_SCOPE,
    `KNOWHOW_WORKER_OTHER_TICKET:`;
  - an anchor that no entry of the target's `effects.touchPaths` covers, by the KHN-V0-006 match:
    BLOCKED/OUT_OF_SCOPE, `KNOWHOW_WORKER_ANCHOR_OUT_OF_SCOPE:`.

  Because `effects` is acceptance-relevant, the touchPaths cannot change while the attempt is
  live. As for every lease command, a supervised attempt's lease does not expire by time. The
  writer-checkpoint route, which models without attempt records, declines the request, so the
  complete route always decides it. The details name payload fields, never their values. The 1024-byte text, 4-anchor,
  32-entry and secret-screen bounds apply unchanged. Status: proposed (V1-0987; GitHub #671).
- `KHN-V0-023`: An entry written under KHN-V0-022 MUST record actor role WORKER together with the
  verified `attempt` and `generation`. The Tasks codec and the Core ticket reader MUST admit role
  WORKER only on a non-superseding ADD whose `attempt` and `generation` are non-null, and refuse
  it on a superseding ADD, a RETRACT or an ADD without them. Status: proposed (V1-0987; GitHub
  #671).

## Failure modes and trust

- **Note content is unverified.** Note text is a claim by the writing principal. Attempt,
  generation and evidence path are writer-asserted provenance and are not checked against the
  attempt ledger, except on a WORKER entry (KHN-V0-022).
- **WORKER notes.** With `knowHow.workerAdd`, the claim holder can write notes that the next claim
  on the same files receives. The scope limits where a note is anchored and which ticket holds
  it, not what it says. The text stays untrusted data (KHN-V0-007), and OWNER or an operator with
  the grant can RETRACT it. The holder check trusts the local actor binding (decision 0003), which
  is a recorded claim, not an authentication.
- **Freshness is a file-level signal, not a semantic check.** A CURRENT note can still be wrong,
  and a STALE note can still be right. STALE only means an anchored file changed since the pin.
- **Git unavailable.** A missing Git binary, no checkout, an unborn `HEAD` or a failed batch makes
  every note UNKNOWN. The claim still succeeds, and the delivered notes say UNKNOWN rather than
  CURRENT.
- **Secrets.** The write-time screen is the shared Core screen. It finds known secret shapes and
  is not a guarantee. A note that slips through stays in the audited history, because entries
  are never deleted. Recovery is a RETRACT plus the store's ordinary history handling.
- **Crash and concurrency.** These are the existing ordinary-mutation cases. A commit either
  writes one receipt or nothing. Concurrent writers serialize through the store lock and the
  expectedRevision CAS.
- **Replay.** An identical retry replays without screening again. A retry after `HEAD` moved must
  pass the same `--commit` and `--issued-at`, or it builds a different payload and conflicts on
  the request id.
- **Hand edits.** ADOPT_FILE refuses a hand edit of the ledger as a protected field.
- **Prompt injection.** Delivered text is labelled untrusted, and the help names it as data.
  Consumers must treat it as data.

## Non-goals and simpler baseline

- No Core `corvint_query` or context-packet delivery. Only Tasks claim and list deliver notes.
- No symbol-level or line-range anchors. Anchors are whole files.
- No LTA-V0-006 skill export and no learning input of any kind.
- No automatic re-confirmation or repinning. A STALE note is refreshed by an explicit superseding
  ADD.
- No cross-ticket supersede or retract. No writes on COMPLETED or ARCHIVED home tickets.
- No WORKER grant by default. KHN-V0-021 is opt-in and grants only a scoped ADD; WORKER never
  supersedes or retracts, and no policy-declared anchor prefixes widen the touchPaths scope.
- No foreign-import carrier. The `ticket import` closed key set still refuses the member.

## Acceptance evidence and traceability

| Requirement | Ticket acceptance | Implementation boundary | Delivered evidence | Required integrated evidence (NOT_RUN) |
|---|---|---|---|---|
| KHN-V0-001 | V1-0955 criteria 1-2 | `internal/tasks/store` (pins); `internal/tasks/cli` (verb) | `TestKHNV0001_PinsResolveTheWritersCommit` (HEAD and explicit commit; missing file, directory, unknown revision and no checkout refused); `TestKHNV0006_KnowHowThroughTheCLI` (pinned blobs listed; missing, absolute and `..` anchors refused end to end) | durable qualification |
| KHN-V0-002 | V1-0955 criteria 1-2 | `internal/tasks/ticket` (codec, view); `internal/tasks/wire` (bounds); `internal/taskman` (Core reader) | `TestKHNV0002_PayloadRefusals`; `TestKHNV0002_RecordCodecRefusals`; `TestKHNV0002_ReaderKnowHow`; `TestIssue502_ReaderAdmitsSharedOptionalKeys` (shared fixture with ADD, supersede, OPERATOR provenance and RETRACT) | native archive round trip of a note-bearing store |
| KHN-V0-003 | V1-0955 criterion 2 | `internal/tasks/mutation` (payload, Apply, adopt); `internal/tasks/intent` (policy grant); `internal/tasks/transaction` (import guard) | `TestKHNV0003_AddSupersedeRetractKeepHistory`; `TestKHNV0003_WriteRefusals`; `TestKHNV0003_AdoptFileRefusesKnowHow`; `TestKHNV0003_ImportApplyNeverCarriesKnowHow`; `TestKHNV0006_KnowHowThroughTheCLI` (replay, supersede, retract, `ticket show`, receipt audit CONSISTENT) | concurrent two-process CAS; interrupted-commit redo of a know-how receipt |
| KHN-V0-004 | V1-0955 criterion 5 | `internal/tasks/mutation` (screen); decision 0397 V1-0955 addendum | `TestKHNV0004_SecretScreenRefusesTheWrite`; `TestImportDirection` and the boundary controls (the exact two-file `internal/secretscreen` edge); `TestKHNV0006_KnowHowThroughTheCLI` (secret text, path and route refused before pinning, never echoed) | none |
| KHN-V0-005 | V1-0955 criterion 3 | `internal/tasks/store` (freshness) | `TestKHNV0005_FreshnessIsComputedAtReadTime` (CURRENT, STALE, UNKNOWN for a deleted file; note precedence; dirty tree ignored; no checkout, non-repository and unborn HEAD all UNKNOWN; ordering); `TestKHNV0006_KnowHowThroughTheCLI` (STALE and UNKNOWN after a commit) | none |
| KHN-V0-006 | V1-0955 criterion 4 | `internal/tasks/store` (selection, projection, claim delivery); `internal/tasks/cli` (list, claim result, help) | `TestKHNV0006_SelectionAndProjection`; `TestKHNV0006_KnowHowThroughTheCLI` (list writes no state or intent bytes; prefix, exact and ticket filters; claim delivers the intersecting compact note); `TestKHNV0006_DeliveredMemberFitsTheCap` (the whole member within 2 KiB across uniform and mixed note sizes, and no longer prefix fits); `TestKHNV0006_ClaimDeliveryIsCapped` (claim --next; the whole member within 2 KiB, matched, omitted and hint; newest first) | an UNAVAILABLE delivery witness from an unreadable inventory |
| KHN-V0-007 | V1-0955 criterion 5 | `internal/tasks/cli` (labels); every reader file | `TestKHNV0007_KnowHowNeverReachesRankingEvidenceOrAuthority` (fixed reader set); the trust label asserted in `TestKHNV0006_KnowHowThroughTheCLI` and `TestKHNV0006_ClaimDeliveryIsCapped` | none |
| KHN-V0-021 | V1-0987 (GitHub #671) | `internal/tasks/intent` (policy key); `internal/tasks/mutation` (role row); `internal/tasks/transaction` (admission) | `TestKHNV0021_PolicyKnowHowWorkerAddOptIn`; `TestKHNV0021_WorkerAddIsPolicyOptIn` (absent and false refuse; RETRACT, non-body REFINE and a WORKER roles row still refuse; OWNER and OPERATOR unchanged); `TestKHNV0021_WorkerKnowHowRefusedWithoutPolicy` (CLI: same detail, state unchanged; refused before the store checks when no store exists) | none |
| KHN-V0-022 | V1-0987 (GitHub #671) | `internal/tasks/mutation` (scope); `internal/tasks/transaction` (attempt observation); `internal/tasks/store` (pre-lock screen, attempt audit, writer-route decline) | `TestKHNV0022_WorkerAddScope` (success and each refusal with its prefix; cap and secret screen still apply); `TestKHNV0022_WorkerKnowHowThroughTheCLI` (claim holder adds; stale generation, out-of-scope anchor, other ticket, foreign actor refused; WORKER retract refused; after release a new add is fenced and the committed one replays); `TestKHNV0022_WorkerAttemptObservation` (real attempt records: expiry boundary, terminal phases, supervision, absent and unleased) | concurrent lease expiry during a WORKER add |
| KHN-V0-023 | V1-0987 (GitHub #671) | `internal/tasks/ticket` (codec); `internal/taskman` (Core reader) | `TestKHNV0023_CodecWorkerEntry`; `TestKHNV0023_ReaderWorkerEntry`; `TestKHNV0022_WorkerAddScope` (the entry round-trips) | none |

## Resolved decisions

These are agent decisions, made fail-closed and accepted by the owner in decision 0443.

1. **TEA pattern rather than derived events.** The write path is the TEA-V0-001 pattern: an
   optional record member changed only by ordinary mutations. It does not use the ON-V0 derived
   event and history reader. History comes from the append-only ledger, the
   previousRecordSha256 chain, the receipts and the journal. No receipt kind, event kind, lock or
   recovery path is added, which is the cheaper design.
2. **Home ticket must be live and NATIVE.** COMPLETED is refused because a release candidate
   binds the completed record digest, so a later write would invalidate that binding. ARCHIVED is
   refused as with TEA.
3. **No cross-ticket targets.** Supersede and retract stay on the home ticket, so one ticket's
   writer cannot withdraw another ticket's note.
4. **No new §11 error code.** The closed code set is not widened in this slice. A secret hit is
   MALFORMED with a stable detail prefix.
5. **Freshness against the committed `HEAD`.** It uses the reader's committed `HEAD` and not the
   dirty tree. One `cat-file` batch is millisecond-scale, it is reproducible, and it runs no
   background work.
6. **A missing anchor is UNKNOWN, not STALE.** A deletion or rename cannot be told apart from an
   unreadable object without more Git work, and UNKNOWN never claims currency.
7. **Greedy ordered prefix under the byte cap.** The reader sees the true order, and the omitted
   count says how much is left.
8. **Computed at response time.** Claim delivery is not pinned at admission, so a replay shows
   current freshness instead of stale state.

## Owner questions

Decision 0443 answers questions 1 to 4 and 7 by keeping the current behaviour: TEA-style ledger
history, no notes on COMPLETED home tickets, supersede and retract within the home ticket only,
no WORKER grant, and the stated bounds. Decision 0444 answers questions 5, 6 and 8 with yes; the
work and its requirements are V1-0964 (5, 6) and V1-0962 (8).

1. Accept TEA-style ledger history, or require ON-V0-style derived events for know-how?
2. Should COMPLETED home tickets accept notes, given the release digest binding (decision 2)?
3. Should supersede or retract be allowed across home tickets, and by which role?
4. Should WORKER be grantable, for example to the claim holder, perhaps writing to the claimed
   ticket only? GitHub #671 reopens this; KHN-V0-021..023 propose an opt-in, attempt-scoped ADD
   and need an amendment of decision 0443's answer before acceptance.
5. Should a dedicated §11 code (for example `SECRET_DETECTED`) replace the MALFORMED detail
   prefix?
6. Should `attempt` and `generation` be verified against the attempt ledger instead of being
   writer-asserted?
7. Are the bounds (32 entries, 1024 text bytes, 4 anchors, 4 routes, 2 KiB claim cap) right?
8. Should know-how reach Core `corvint_query` packets, and under which authority label?

## Rollout and rollback

Rollout is additive. A record without notes keeps its exact legacy bytes, existing policies do not
grant OPERATOR the operations or WORKER `KNOWHOW_ADD`, and a claim of a ticket with no intersecting notes adds an empty
`knowHow` member.

Rollback is reverting the change before any store holds a record with `knowHow`. After such a
write, an older binary refuses that record at its closed reader: the failure is closed and nothing
is lost silently. Recovery is to run the newer binary, or to restore the store from a backup
taken before the first note.

The WORKER grant (KHN-V0-021..023) rolls back by removing `knowHow` from policy, which restores the refusal at once.
An older binary refuses a policy carrying the `knowHow` key and a record holding a WORKER entry,
both closed; a stored WORKER entry stays in history and can be retracted by OWNER. A
`knowHow` policy change is not handoff-neutral: like any other policy change it fences an
open handoff.
