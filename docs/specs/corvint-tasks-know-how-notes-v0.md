# Corvint Tasks know-how notes V0

Owner: Russell Lewis
Date: 2026-10-07
Intent status: accepted by decision 0443 (V1-0955); KHN-V0-008..015 proposed (V1-0964); KHN-V0-016..020 proposed (V1-0963)
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
- Status: KHN-V0-001..007 accepted by decision 0443 (V1-0955); KHN-V0-008..015 proposed (V1-0964); KHN-V0-016..020 proposed (V1-0963); experimental; `ticket know-how add|retract|reconfirm|list`, the optional `knowHow` record member and claim delivery exist with focused tests.
- Exists: record/Core codecs, KNOWHOW_ADD/KNOWHOW_RETRACT/KNOWHOW_RECONFIRM through Apply, write-time secret screen (`SECRET_DETECTED`), blob and `--symbol PATH#NAME` declaration-digest pins, attempt/generation provenance verified against the audited attempt inventory (`PROVENANCE_UNVERIFIED`), read-time freshness from batched Git calls, a reconfirm that refuses `KNOWHOW_NOT_STALE`, a 2 KiB claim projection, an authority boundary test, and deterministic archive, concurrency, redo, UNAVAILABLE and commit-race witnesses.
- Blocked on: owner acceptance of KHN-V0-008..015 (V1-0964) and KHN-V0-016..020 (V1-0963); owner question 8 (V1-0962); durable qualification; Core packet delivery and a WORKER grant (V1-0987) are non-goals here.
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
  is refused as VALIDATION_FAILED with the owned code `SECRET_DETECTED` (KHN-V0-010; it was
  MALFORMED before V1-0964), and the detail starts with the stable prefix
  `KNOWHOW_SECRET_DETECTED:` (retained by KHN-V0-011). The detail names the field and never the
  matched text.
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

The requirements below are proposed (V1-0964). Decision 0444 answered owner questions 5 and 6 with
yes; these requirements return to the owner for acceptance.

- `KHN-V0-008`: A `KNOWHOW_ADD` that names `attempt` or `generation` MUST be verified against the
  journal-audited attempt inventory before the entry is appended. The attempt must exist and its
  home ticket must be the home ticket written to. A generation must be the attempt's current
  generation or one of its recorded prior generations. A generation without an attempt, an
  unknown attempt, another ticket's attempt, an unrecorded generation, and an inventory that was
  not observed are each refused VALIDATION_FAILED with the owned code `PROVENANCE_UNVERIFIED` at
  `/payload/attempt` or `/payload/generation`. The detail never repeats the asserted values, and
  nothing is written. The check is one reusable function, `mutation.CheckKnowHowProvenance`, so a
  later write path (for example a WORKER grant) calls the same check and adds its own liveness or
  holder test on top. Liveness is not required here: a note may record what a finished attempt
  learned. A write without either member is unchanged. Status: proposed (V1-0964).
- `KHN-V0-009`: The writer MUST load and audit the complete attempt inventory for any
  `KNOWHOW_ADD` that names an attempt or a generation. Such a write never takes the writer fast
  route, every `attempts/` file joins the audited input set, and the ledger handed to the check is
  built from that audited state. A ledger that was not built is nil, and a nil ledger refuses
  (KHN-V0-008), so a missing input fails closed. The check runs only for a fresh write; a replay of
  a committed request returns its receipt without checking again. Status: proposed (V1-0964).
- `KHN-V0-010`: A secret hit in a know-how argument or payload MUST be refused with the owned §11
  detail code `SECRET_DETECTED` rather than MALFORMED with a prefix. `SECRET_DETECTED` and
  `PROVENANCE_UNVERIFIED` are registered in the closed code set (76 codes) and classified as
  not retryable until the request or local input changes. Both appear only in refusal results;
  neither is ever written to a record, receipt, journal entry or archive. Status: proposed
  (V1-0964).
- `KHN-V0-011`: The transition MUST keep the detail prefix `KNOWHOW_SECRET_DETECTED:` on every
  `SECRET_DETECTED` detail as a deprecated alias for one transition window, so a script that
  matches the prefix keeps working. A script that matched `code == "MALFORMED"` plus the prefix
  must change to the new code or to the prefix alone. An older strict reader that validates the
  closed code set refuses a result carrying either new code, which fails closed; nothing it reads
  from the store changes. The prefix is removed only by a later amendment of this requirement.
  Status: proposed (V1-0964).
- `KHN-V0-012`: A native archive export of a store whose tickets carry know-how ledgers MUST
  verify, and each archived ticket record MUST be byte-identical to its projection and decode to
  the same ledger, including RETRACT entries. Status: proposed (V1-0964).
- `KHN-V0-013`: Competing know-how writers MUST resolve through the writer lock and the
  expectedRevision CAS. Two writers that composed against the same revision commit exactly one
  entry; the other is REVISION_CONFLICT with nothing written, an identical retry repeats that
  refusal, and a writer rebuilt at the new revision appends the next entry without touching the
  first. A know-how receipt linked before its head and projection were published is redone by
  the next writer exactly once, and its request then replays. Status: proposed (V1-0964).
- `KHN-V0-014`: When the inventory cannot be loaded while a claim response is built, the claim
  MUST still return its committed result, with the `knowHow` member in state `UNAVAILABLE`, the
  load's code, null notes, the trust label and a warning. Once the inventory reads again, the same
  replay delivers the notes. Status: proposed (V1-0964).
- `KHN-V0-015`: A commit that moves `HEAD` after a pin's commit was resolved and before its paths
  are asked MUST NOT mix two commits into one pin set: the pin returns the earlier commit and that
  commit's blobs. Status: proposed (V1-0964).

KHN-V0-016 to KHN-V0-020 add symbol anchors and an explicit re-confirm write (V1-0963, a
follow-up of V1-0955 / GitHub #655). They are proposed and need the owner's acceptance; the
implementation is experimental until then.

- `KHN-V0-016`: `ticket know-how add` MUST accept `--symbol PATH#NAME`, repeatable and combinable
  with `--anchor`, up to the 4-anchor cap. NAME is the part after the last `#`: 1..128 bytes of
  printable ASCII without space or `#`. A symbol anchor is `{blob, path, symbol, symbolSha256}`.
  `blob` is the file's pin as in KHN-V0-001. `symbolSha256` is the SHA-256 of the one declaration
  of NAME that the context index's own extractor reports for that blob
  (`contextindex.SymbolExtents`; no parser or language is added). A Go method is named
  `Receiver.Method` and its extent runs from its doc comment to its closing brace; other languages
  use the index extractor's reported extent. A file anchor keeps its exact `{blob, path}` form.
  Anchors are sorted by (path, symbol) without duplicates, a file anchor before that path's
  symbols, and all anchors of one path pin the same blob. A symbol that is missing, declared more
  than once, in a file no extractor admits or that the extractor refuses, or in a blob over 1 MiB
  is refused MALFORMED at `/payload/anchors` before any mutation, with the detail prefix
  `KNOWHOW_UNRESOLVED:`, and nothing is written. Symbol names are screened like paths
  (KHN-V0-004). The Tasks codec and the Core reader enforce the same anchor rules.
- `KHN-V0-017`: Freshness of a symbol anchor MUST follow the declaration's digest. A blob equal to
  the pin is CURRENT without reading content. Otherwise the reader reads each changed blob once
  (one `git cat-file --batch` for all of them, each at most 1 MiB) and re-extracts the symbol: an
  equal digest is CURRENT and a different one is STALE. A symbol that is missing, renamed,
  duplicated, unsupported, oversize, or unreadable, and every anchor of a deleted file, is UNKNOWN,
  never CURRENT. A file anchor keeps KHN-V0-005 unchanged, so a file anchor on the same path goes
  STALE when any byte changes. Note precedence and ordering are KHN-V0-005 and KHN-V0-006.
- `KHN-V0-018`: `ticket know-how reconfirm T --note N` MUST append one RECONFIRM entry
  `{seq, operation, note, anchors, commit, attempt, generation, actor, recordedAt}` through the
  ordinary receipt-backed mutation `KNOWHOW_RECONFIRM` (`{note, anchors, commit, attempt,
  generation}`) with the KHN-V0-003 CAS, replay, live NATIVE home ticket, 32-entry cap and role
  grant (OWNER by default, OPERATOR only through an explicit policy row). The CLI re-pins the
  note's current anchors at `HEAD` or `--commit` with the KHN-V0-016 rules. The entry names an
  earlier active note and carries exactly that note's anchor paths and symbols. It rewrites no
  earlier entry, so the ADD's original pins and every earlier RECONFIRM stay in `ticket show`.
  Reads use the latest RECONFIRM's anchors and commit as the note's effective pins; `list`'s full
  form adds `reconfirmed {seq, actor, recordedAt, attempt, generation}`. A superseding ADD or a
  RETRACT ends the note as before, and RECONFIRM is not a target of either. A RECONFIRM that names
  `attempt` or `generation` passes the same `CheckKnowHowProvenance` check (KHN-V0-008) against
  the audited attempt inventory (KHN-V0-009), and never takes the writer fast route.
- `KHN-V0-019`: A re-confirm of a note that is not STALE MUST be refused, not treated as a no-op.
  A re-pin in which no anchor's pin (the blob of a file anchor, the digest of a symbol anchor)
  changed is VALIDATION_FAILED/MALFORMED with the detail prefix `KNOWHOW_NOT_STALE:`, enforced in
  Apply, the Tasks codec and the Core reader. A re-confirm whose current pin cannot be resolved
  (an UNKNOWN anchor) is refused with `KNOWHOW_UNRESOLVED:` before anything is submitted. A
  note that is not active, or a re-pin that drops, adds or renames an anchor, is refused MALFORMED.
- `KHN-V0-020`: The change MUST be wire-compatible. A record whose ledger holds only file anchors
  and ADD and RETRACT entries decodes, re-encodes and projects byte-identically, and gains no
  symbol or reconfirm key. The new anchor keys, the RECONFIRM operation and the
  `KNOWHOW_RECONFIRM` mutation are additive within `taskman-ticket/0`; an older binary refuses a
  record or receipt carrying them at its closed key sets, so it fails closed and never misreads
  them.

## Failure modes and trust

- **Note content is unverified.** Note text is a claim by the writing principal. Since V1-0964
  the attempt and generation are checked against the audited attempt inventory (KHN-V0-008); the
  evidence path is still writer-asserted.
- **Freshness is a file-level signal, not a semantic check.** A CURRENT note can still be wrong,
  and a STALE note can still be right. STALE only means an anchored file changed since the pin.
  A symbol anchor narrows this to one declaration's text: a change elsewhere that alters what the
  declaration means (a callee, a constant) leaves it CURRENT.
- **Symbol extraction limits.** The extent comes from the context index's extractor, whose
  coverage differs by language. A symbol it cannot place exactly once reads UNKNOWN and cannot be
  pinned or re-confirmed. Changing that extractor's extent rules can turn pinned symbols STALE;
  that is visible and fail-closed, never a silent CURRENT.
- **Re-confirm is a claim, not a review.** A RECONFIRM records that its writer checked the note
  against new code. Like the note text it is unverified; only its `attempt` and `generation` are
  checked against the audited attempt inventory, as for ADD (KHN-V0-008).
- **Git unavailable.** A missing Git binary, no checkout, an unborn `HEAD` or a failed batch makes
  every note UNKNOWN. The claim still succeeds, and the delivered notes say UNKNOWN rather than
  CURRENT.
- **Secrets.** The write-time screen is the shared Core screen. It finds known secret shapes and
  is not a guarantee. A note that slips through stays in the audited history, because entries
  are never deleted. Recovery is a RETRACT plus the store's ordinary history handling.
- **Crash and concurrency.** These are the existing ordinary-mutation cases. A commit either
  writes one receipt or nothing. Concurrent writers serialize through the store lock and the
  expectedRevision CAS (KHN-V0-013). Because the lock serializes the mutation itself, the only
  window in which two writers overlap is composition before the lock. The witness forces that
  overlap in a fixed order through the store API, with no sleeps or goroutine timing.
- **Replay.** An identical retry replays without screening again. A retry after `HEAD` moved must
  pass the same `--commit` and `--issued-at`, or it builds a different payload and conflicts on
  the request id.
- **Hand edits.** ADOPT_FILE refuses a hand edit of the ledger as a protected field.
- **Prompt injection.** Delivered text is labelled untrusted, and the help names it as data.
  Consumers must treat it as data.

## Non-goals and simpler baseline

- No Core `corvint_query` or context-packet delivery. Only Tasks claim and list deliver notes.
- No line-range anchors, and no new parser or language support for symbol anchors
  (KHN-V0-016 reuses the index extractor). No symbol rename tracking: a renamed symbol is UNKNOWN.
- No LTA-V0-006 skill export and no learning input of any kind.
- No automatic re-confirmation or repinning. A STALE note is refreshed by an explicit RECONFIRM
  (KHN-V0-018) or a superseding ADD, and no read ever writes a pin.
- No cross-ticket supersede or retract. No writes on COMPLETED or ARCHIVED home tickets.
- No WORKER grant. Workers report know-how through their owner or operator in this slice. The
  grant is V1-0987; it reuses the KHN-V0-008 check and is not specified here.
- No foreign-import carrier. The `ticket import` closed key set still refuses the member.

## Acceptance evidence and traceability

| Requirement | Ticket acceptance | Implementation boundary | Delivered evidence | Required integrated evidence (NOT_RUN) |
|---|---|---|---|---|
| KHN-V0-001 | V1-0955 criteria 1-2 | `internal/tasks/store` (pins); `internal/tasks/cli` (verb) | `TestKHNV0001_PinsResolveTheWritersCommit` (HEAD and explicit commit; missing file, directory, unknown revision and no checkout refused); `TestKHNV0006_KnowHowThroughTheCLI` (pinned blobs listed; missing, absolute and `..` anchors refused end to end) | durable qualification |
| KHN-V0-002 | V1-0955 criteria 1-2 | `internal/tasks/ticket` (codec, view); `internal/tasks/wire` (bounds); `internal/taskman` (Core reader) | `TestKHNV0002_PayloadRefusals`; `TestKHNV0002_RecordCodecRefusals`; `TestKHNV0002_ReaderKnowHow`; `TestIssue502_ReaderAdmitsSharedOptionalKeys` (shared fixture with ADD, supersede, OPERATOR provenance and RETRACT); archive round trip under KHN-V0-012 | none |
| KHN-V0-003 | V1-0955 criterion 2 | `internal/tasks/mutation` (payload, Apply, adopt); `internal/tasks/intent` (policy grant); `internal/tasks/transaction` (import guard) | `TestKHNV0003_AddSupersedeRetractKeepHistory`; `TestKHNV0003_WriteRefusals`; `TestKHNV0003_AdoptFileRefusesKnowHow`; `TestKHNV0003_ImportApplyNeverCarriesKnowHow`; `TestKHNV0006_KnowHowThroughTheCLI` (replay, supersede, retract, `ticket show`, receipt audit CONSISTENT); competing writers and redo under KHN-V0-013 | durable two-process qualification |
| KHN-V0-004 | V1-0955 criterion 5 | `internal/tasks/mutation` (screen); decision 0397 V1-0955 addendum | `TestKHNV0004_SecretScreenRefusesTheWrite` (SECRET_DETECTED with the prefix); `TestImportDirection` and the boundary controls (the exact two-file `internal/secretscreen` edge); `TestKHNV0006_KnowHowThroughTheCLI` (secret text, path and route refused before pinning, never echoed) | none |
| KHN-V0-005 | V1-0955 criterion 3 | `internal/tasks/store` (freshness) | `TestKHNV0005_FreshnessIsComputedAtReadTime` (CURRENT, STALE, UNKNOWN for a deleted file; note precedence; dirty tree ignored; no checkout, non-repository and unborn HEAD all UNKNOWN; ordering); `TestKHNV0006_KnowHowThroughTheCLI` (STALE and UNKNOWN after a commit) | none |
| KHN-V0-006 | V1-0955 criterion 4 | `internal/tasks/store` (selection, projection, claim delivery); `internal/tasks/cli` (list, claim result, help) | `TestKHNV0006_SelectionAndProjection`; `TestKHNV0006_KnowHowThroughTheCLI` (list writes no state or intent bytes; prefix, exact and ticket filters; claim delivers the intersecting compact note); `TestKHNV0006_DeliveredMemberFitsTheCap` (the whole member within 2 KiB across uniform and mixed note sizes, and no longer prefix fits); `TestKHNV0006_ClaimDeliveryIsCapped` (claim --next; the whole member within 2 KiB, matched, omitted and hint; newest first); UNAVAILABLE delivery under KHN-V0-014 | none |
| KHN-V0-008 | V1-0964 (owner question 6) | `internal/tasks/mutation` (`CheckKnowHowProvenance`, Apply) | `TestKHNV0008_ProvenanceIsVerified` (absent, current and prior generation pass; generation alone, nil ledger, unknown attempt, other ticket, unrecorded generation refused without echo); `TestKHNV0008_ReusableCheck` (non-live attempt passes; liveness left to the caller); `TestKHNV0008_ProvenanceThroughTheCLI` (four refusals with byte-identical state and intent trees; the verified add is shown by `ticket show` and replays) | durable qualification |
| KHN-V0-009 | V1-0964 (owner question 6) | `internal/tasks/store` (writer route, reviewAudit); `internal/tasks/transaction` (attempt load, ledger) | `TestKHNV0008_KnowHowNamesAttempt` (the predicate that routes the write); `TestKHNV0008_ProvenanceThroughTheCLI` (a claimed attempt verifies only through the audited inventory) | none |
| KHN-V0-010 | V1-0964 (owner question 5) | `internal/tasks/wire` (codes, retry); `internal/tasks/mutation` (screen) | `TestKHNV0004_SecretScreenRefusesTheWrite`; `TestKHNV0010_ArgumentScreenUsesTheOwnedCode`; `TestKHNV0006_KnowHowThroughTheCLI` (SECRET_DETECTED and never MALFORMED end to end); `TestTMV0002_AS01_CommandResultEnvelope` (76 codes); `TestCALV0078_ClassificationCoversEveryCode` | none |
| KHN-V0-011 | V1-0964 (owner question 5) | `internal/tasks/mutation` (detail prefix) | `TestKHNV0004_SecretScreenRefusesTheWrite` and `TestKHNV0010_ArgumentScreenUsesTheOwnedCode` (the prefix is kept on every detail) | an older binary reading a 76-code result (fails closed by construction) |
| KHN-V0-012 | V1-0964 | `internal/tasks/archive` (unchanged) | `TestKHNV0012_ArchiveRoundTripKeepsKnowHow` (export, verify, byte-identical record, ADD and RETRACT decoded) | archive import into a fresh store |
| KHN-V0-013 | V1-0964 | `internal/tasks/store` (lock, CAS, redo; unchanged) | `TestKHNV0013_CompetingWritersOneWinner`; `TestKHNV0013_RedoBindsAPendingKnowHowReceipt` | two OS processes racing the lock |
| KHN-V0-014 | V1-0964 | `internal/tasks/cli` (claim delivery; unchanged) | `TestKHNV0014_UnreadableInventoryDeliversUnavailable` (replayed claim, UNAVAILABLE, MALFORMED code, null notes, trust label, warning; recovery delivers the note) | none |
| KHN-V0-015 | V1-0964 | `internal/tasks/store` (pin batch; unchanged) | `TestKHNV0015_CommitRaceResolvesOneCommit` (a commit forced between the commit answer and the path questions through the `askAtCommit` writer seam) | none |
| KHN-V0-007 | V1-0955 criterion 5 | `internal/tasks/cli` (labels); every reader file | `TestKHNV0007_KnowHowNeverReachesRankingEvidenceOrAuthority` (fixed reader set); the trust label asserted in `TestKHNV0006_KnowHowThroughTheCLI` and `TestKHNV0006_ClaimDeliveryIsCapped` | none |
| KHN-V0-016 | V1-0963 criterion 1 | `internal/contextindex` (extents); `internal/tasks/store` (symbol pins); `internal/tasks/ticket`, `internal/tasks/wire`, `internal/taskman` (anchor codecs); `internal/tasks/cli` (`--symbol`); decision 0397 V1-0963 addendum | `TestKHNV0016_SymbolExtentsReuseTheIndexExtractors`; `TestKHNV0016_SymbolPinsReuseTheIndexExtractor` (Go function, method and Python def pinned; missing, bare method, duplicate, unsupported and missing file refused `KNOWHOW_UNRESOLVED`); `TestKHNV0016_SymbolAnchorsInPayloadAndRecord` (encoding, payload and record refusals, secret symbol name); `TestKHNV0020_ReaderKnowHowSymbolsAndReconfirm` (Core reader); `TestImportDirection` and the boundary controls (the one-file `internal/contextindex` edge) | durable qualification on a non-Go repository |
| KHN-V0-017 | V1-0963 criterion 1 | `internal/tasks/store` (freshness) | `TestKHNV0017_SymbolFreshnessFollowsTheDeclaration` (other symbol edited or moved CURRENT; pinned symbol edited STALE; renamed, duplicated, deleted file, oversize file and no checkout UNKNOWN; file anchor on the same path STALE); `TestKHNV0018_SymbolAnchorsAndReconfirmThroughTheCLI`; `BenchmarkKnowHowFreshness32SymbolAnchors` | none |
| KHN-V0-018 | V1-0963 criterion 2 | `internal/tasks/mutation` (payload, Apply); `internal/tasks/intent` (grant); `internal/tasks/ticket` (codec, effective pins); `internal/tasks/store` (projection); `internal/tasks/cli` (verb, help) | `TestKHNV0018_ReconfirmRepinsWithProvenance` (attempt and generation verified by `CheckKnowHowProvenance`, unobserved or unknown attempt PROVENANCE_UNVERIFIED, prior bytes kept, effective pins, cap, role grants); `TestKHNV0008_KnowHowNamesAttempt` (an attempt-naming reconfirm takes the audited route); `TestKHNV0018_ProjectionShowsEffectivePinsAndProvenance`; `TestKHNV0018_SymbolAnchorsAndReconfirmThroughTheCLI` (claimed attempt verified, unknown attempt PROVENANCE_UNVERIFIED, STALE re-pinned to CURRENT, `ticket show` keeps both pins, receipt audit CONSISTENT) | concurrent two-process CAS; interrupted-commit redo of a reconfirm receipt |
| KHN-V0-019 | V1-0963 criterion 2 | `internal/tasks/wire` (shared rule); `internal/tasks/mutation`; `internal/tasks/ticket`; `internal/taskman`; `internal/tasks/cli` | `TestKHNV0019_ReconfirmOfANoteThatIsNotStaleIsRefused`; `TestKHNV0020_ReaderKnowHowSymbolsAndReconfirm`; `TestKHNV0018_SymbolAnchorsAndReconfirmThroughTheCLI` (CURRENT and repeated reconfirm `KNOWHOW_NOT_STALE`, UNKNOWN `KNOWHOW_UNRESOLVED`) | none |
| KHN-V0-020 | V1-0963 criterion 3 | `internal/tasks/ticket`; `internal/taskman`; `internal/tasks/store` | `TestKHNV0020_LegacyKnowHowReadsExactlyAsBefore`; `TestIssue502_RecordEscalationsKey` and `TestKHNV0002_ReaderKnowHow` (unchanged legacy fixture); `TestKHNV0018_ProjectionShowsEffectivePinsAndProvenance` (file-anchor note projects without new keys) | an older released binary refusing a RECONFIRM-bearing store |

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
4. **No new §11 error code.** The closed code set was not widened in V1-0955, and a secret hit
   was MALFORMED with a stable detail prefix. Superseded by decision 0444's answer to owner
   question 5: KHN-V0-010 and KHN-V0-011 (proposed, V1-0964) add `SECRET_DETECTED` and keep the
   prefix for one transition window.
5. **Freshness against the committed `HEAD`.** It uses the reader's committed `HEAD` and not the
   dirty tree. One `cat-file` batch is millisecond-scale, it is reproducible, and it runs no
   background work.
6. **A missing anchor is UNKNOWN, not STALE.** A deletion or rename cannot be told apart from an
   unreadable object without more Git work, and UNKNOWN never claims currency.
7. **Greedy ordered prefix under the byte cap.** The reader sees the true order, and the omitted
   count says how much is left.
8. **Computed at response time.** Claim delivery is not pinned at admission, so a replay shows
   current freshness instead of stale state.
9. **Symbol digest is the declaration text (V1-0963, proposed).** The pin is the SHA-256 of the
   extent text, so moving a declaration or editing a neighbour leaves it CURRENT, while any byte
   inside it, including its doc comment, makes it STALE. A declaration named more than once is
   UNKNOWN rather than an arbitrary choice.
10. **Re-confirm of a note that is not STALE is refused, not a no-op (V1-0963, proposed).** A
    no-op would still need a receipt or a special replay path, and an accepted entry would spend
    one of the 32 ledger slots on a pin that says nothing new. Refusing tells the caller its
    premise (the note is STALE) is wrong. The refusal reuses MALFORMED with a stable detail
    prefix, so no §11 code is added for it (the codes KHN-V0-010 adds cover secret hits and
    provenance only). UNKNOWN is refused because a pin needs
    a resolvable target.
11. **Re-confirm keeps the grant of ADD and RETRACT (V1-0963, proposed).** OWNER by default and
    OPERATOR only through a policy row. No WORKER grant is added here (V1-0987 owns that).

## Owner questions

Decision 0443 answers questions 1 to 4 and 7 by keeping the current behaviour: TEA-style ledger
history, no notes on COMPLETED home tickets, supersede and retract within the home ticket only,
no WORKER grant, and the stated bounds. Decision 0444 answers questions 5, 6 and 8 with yes; the
work and its requirements are V1-0964 (5, 6) and V1-0962 (8).

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

## Amendments to TCP-00

KHN-V0-010 (proposed, V1-0964) adds two §11 detail codes, bringing the closed set to 76:
`SECRET_DETECTED` (a know-how secret screen hit) and `PROVENANCE_UNVERIFIED` (KHN-V0-008). Both
are classified with the codes whose request or local input must change first, in the retry table
of the [agent lease contract](corvint-tasks-agent-leases-v0.md). Neither is persisted.

## Compatibility of the code change

| Reader | Before V1-0964 | After V1-0964 |
|---|---|---|
| Script matching the prefix `KNOWHOW_SECRET_DETECTED:` | matches | matches (the prefix is a deprecated alias, KHN-V0-011) |
| Script matching `code == "MALFORMED"` plus the prefix | matches | no longer matches; switch to `SECRET_DETECTED` or the prefix alone |
| Older strict reader validating the closed code set | 74 codes | refuses a result carrying a new code (fails closed) |
| Stored records, receipts, journal and archives | no code stored | unchanged; neither code is ever written |
| A write naming attempt/generation | stored as asserted | verified; unverifiable provenance is refused `PROVENANCE_UNVERIFIED` |

Notes already stored with unverified provenance stay as written; the check applies to new writes
only, and a replay of a committed request is never checked again.

## Rollout and rollback

Rollout is additive. A record without notes keeps its exact legacy bytes, existing policies do not
grant OPERATOR the operations, and a claim of a ticket with no intersecting notes adds an empty
`knowHow` member.

Rolling back V1-0964 alone restores the MALFORMED code and writer-asserted provenance; it
changes no stored bytes, because neither new code is persisted and verified provenance uses the
existing members.

Rollback is reverting the change before any store holds a record with `knowHow`. After such a
write, an older binary refuses that record at its closed reader: the failure is closed and nothing
is lost silently. Recovery is to run the newer binary, or to restore the store from a backup
taken before the first note.

The V1-0963 additions (KHN-V0-016 to KHN-V0-020) follow the same rule. A store with only file
anchors and ADD and RETRACT entries is unchanged by them, so reverting the change is safe until a
store holds a symbol anchor or a RECONFIRM entry. After that, a binary without V1-0963 refuses the
record closed; recovery is the newer binary or a backup from before the first such write. The
`internal/contextindex` import is one file (`internal/tasks/store/know_how_symbols.go`) and is
reverted with it.
