# Corvint Tasks know-how notes V0

Owner: Russell Lewis
Date: 2026-10-07
Intent status: proposed, pending owner acceptance (V1-0955)
Delivery status: experimental

Authoritative inputs: GitHub issue beamfall/corvint#655 and the agent-filed native ticket V1-0955
("cross-ticket know-how notes"), implemented on orchestrator instruction on 2026-10-07. The
requirements below are proposed and the owner has not accepted them. The issue's adopter context
stays in the issue. The write path reuses the
[evidence attachments profile](corvint-tasks-evidence-attachments-v0.md) pattern (TEA-V0-001: an
optional record member changed only by an ordinary receipt-backed mutation). The claim surface is
the one in the [agent lease contract](corvint-tasks-agent-leases-v0.md), and it is unchanged.

## Agent digest
- Claim: Agents record cross-ticket know-how notes pinned to file blobs; reads compute STALE/UNKNOWN freshness and claims deliver intersecting notes as untrusted data.
- Status: proposed, pending owner acceptance (V1-0955); experimental; `ticket know-how add|retract|list`, the optional `knowHow` record member and claim delivery exist with focused tests.
- Exists: record/Core codecs, KNOWHOW_ADD/KNOWHOW_RETRACT through Apply, write-time secret screen and blob pins, read-time freshness from one batched Git call, a 2 KiB claim projection and an authority boundary test.
- Blocked on: owner acceptance of the requirements and the owner questions; archive round trip, concurrency and redo witnesses; Core packet delivery is a non-goal here.
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
  that contain `..`, and a trailing-`/` directory prefix is also refused. The commit and every
  blob are resolved with one batched `git cat-file --batch-check` call. A missing path, a
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
  - `actor` is `{id, role}`, with role OWNER or OPERATOR.
  - `attempt`, `generation` and `evidencePath` are optional writer-asserted provenance.

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
    `policy.roles.OPERATOR` row, and no other role can be granted them.
  - ADOPT_FILE refuses `knowHow` as a protected field, and IMPORT_APPLY refuses an imported
    record that adds, rewrites or drops entries.
- `KHN-V0-004`: Before an entry is appended, the writer MUST screen the text, the reason, the
  route tokens, the anchor paths and the evidence path with the shared Core secret screen. A hit
  is refused as VALIDATION_FAILED/MALFORMED, and the detail starts with the stable prefix
  `KNOWHOW_SECRET_DETECTED:`. The detail names the field and never the matched text.
- `KHN-V0-005`: Freshness MUST be computed at read time against the reader's committed `HEAD`,
  and it is never stored. The reader's commit and every distinct anchor path are resolved in one
  batched Git call, and uncommitted edits are ignored.
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
  - It keeps the longest ordered prefix whose canonical encoding fits 2048 bytes and carries
    `matched`, `omitted` and a read hint when notes were left out.
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

## Failure modes and trust

- **Note content is unverified.** Note text is a claim by the writing principal. Attempt,
  generation and evidence path are writer-asserted provenance and are not checked against the
  attempt ledger.
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
- No WORKER grant. Workers report know-how through their owner or operator in this slice.
- No foreign-import carrier. The `ticket import` closed key set still refuses the member.

## Acceptance evidence and traceability

| Requirement | Ticket acceptance | Implementation boundary | Delivered evidence | Required integrated evidence (NOT_RUN) |
|---|---|---|---|---|
| KHN-V0-001 | V1-0955 criteria 1-2 | `internal/tasks/store` (pins); `internal/tasks/cli` (verb) | `TestKHNV0001_PinsResolveTheWritersCommit` (HEAD and explicit commit; missing file, directory, unknown revision and no checkout refused); `TestKHNV0006_KnowHowThroughTheCLI` (pinned blobs listed; missing, absolute and `..` anchors refused end to end) | durable qualification |
| KHN-V0-002 | V1-0955 criteria 1-2 | `internal/tasks/ticket` (codec, view); `internal/tasks/wire` (bounds); `internal/taskman` (Core reader) | `TestKHNV0002_PayloadRefusals`; `TestKHNV0002_RecordCodecRefusals`; `TestKHNV0002_ReaderKnowHow`; `TestIssue502_ReaderAdmitsSharedOptionalKeys` (shared fixture with ADD, supersede, OPERATOR provenance and RETRACT) | native archive round trip of a note-bearing store |
| KHN-V0-003 | V1-0955 criterion 2 | `internal/tasks/mutation` (payload, Apply, adopt); `internal/tasks/intent` (policy grant); `internal/tasks/transaction` (import guard) | `TestKHNV0003_AddSupersedeRetractKeepHistory`; `TestKHNV0003_WriteRefusals`; `TestKHNV0003_AdoptFileRefusesKnowHow`; `TestKHNV0003_ImportApplyNeverCarriesKnowHow`; `TestKHNV0006_KnowHowThroughTheCLI` (replay, supersede, retract, `ticket show`, receipt audit CONSISTENT) | concurrent two-process CAS; interrupted-commit redo of a know-how receipt |
| KHN-V0-004 | V1-0955 criterion 5 | `internal/tasks/mutation` (screen); decision 0397 V1-0955 addendum | `TestKHNV0004_SecretScreenRefusesTheWrite`; `TestImportDirection` and the boundary controls (the exact two-file `internal/secretscreen` edge); `TestKHNV0006_KnowHowThroughTheCLI` (secret refused, never echoed) | none |
| KHN-V0-005 | V1-0955 criterion 3 | `internal/tasks/store` (freshness) | `TestKHNV0005_FreshnessIsComputedAtReadTime` (CURRENT, STALE, UNKNOWN for a deleted file; note precedence; dirty tree ignored; no checkout, non-repository and unborn HEAD all UNKNOWN; ordering); `TestKHNV0006_KnowHowThroughTheCLI` (STALE and UNKNOWN after a commit) | none |
| KHN-V0-006 | V1-0955 criterion 4 | `internal/tasks/store` (selection, projection, claim delivery); `internal/tasks/cli` (list, claim result, help) | `TestKHNV0006_SelectionAndProjection`; `TestKHNV0006_KnowHowThroughTheCLI` (list writes no state or intent bytes; prefix, exact and ticket filters; claim delivers the intersecting compact note); `TestKHNV0006_ClaimDeliveryIsCapped` (claim --next; 2 KiB cap, matched, omitted and hint; newest first) | an UNAVAILABLE delivery witness from an unreadable inventory |
| KHN-V0-007 | V1-0955 criterion 5 | `internal/tasks/cli` (labels); every reader file | `TestKHNV0007_KnowHowNeverReachesRankingEvidenceOrAuthority` (fixed reader set); the trust label asserted in `TestKHNV0006_KnowHowThroughTheCLI` and `TestKHNV0006_ClaimDeliveryIsCapped` | none |

## Resolved decisions

These are agent decisions, made fail-closed and pending owner acceptance.

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

1. Accept TEA-style ledger history, or require ON-V0-style derived events for know-how?
2. Should COMPLETED home tickets accept notes, given the release digest binding (decision 2)?
3. Should supersede or retract be allowed across home tickets, and by which role?
4. Should WORKER be grantable, for example to the claim holder, perhaps writing to the claimed
   ticket only?
5. Should a dedicated §11 code (for example `SECRET_DETECTED`) replace the MALFORMED detail
   prefix?
6. Should `attempt` and `generation` be verified against the attempt ledger instead of being
   writer-asserted?
7. Are the bounds (32 entries, 1024 text bytes, 4 anchors, 4 routes, 2 KiB claim cap) right?
8. Should know-how reach Core `corvint_query` packets, and under which authority label?

## Rollout and rollback

Rollout is additive. A record without notes keeps its exact legacy bytes, existing policies do not
grant OPERATOR the operations, and a claim of a ticket with no intersecting notes adds an empty
`knowHow` member.

Rollback is reverting the change before any store holds a record with `knowHow`. After such a
write, an older binary refuses that record at its closed reader: the failure is closed and nothing
is lost silently. Recovery is to run the newer binary, or to restore the store from a backup
taken before the first note.
