# Corvint Tasks lane flow V0

Owner: Russell Lewis
Date: 2026-10-31
Intent status: experimental under the owner's standing lane auto-accept (2026-10-11; V1-3569, V1-3463, V1-3577, V1-2909, V1-3331, V1-3576)
Delivery status: experimental

Authoritative inputs: the agent-filed native tickets V1-3569 and V1-3463 (in-flight lane work is
invisible while the queue barrier is OPERATOR/ADMISSION), V1-3577 (no verb carries an undelivered
remainder into a linked ticket), V1-2909 (requirement-ID collisions across unpushed local branches),
V1-3331 (the delivery screen misses tickets delivered on unmerged lane heads) and V1-3576 (927 of
1,049 OPEN tickets have COVERAGE_UNKNOWN effects). The ordinary mutation path of the
[agent lease contract](corvint-tasks-agent-leases-v0.md), the TEA-V0-001 attachment pattern of
[evidence attachments](corvint-tasks-evidence-attachments-v0.md) and the reads of
[closeout debt](corvint-tasks-closeout-debt-v0.md) are reused unchanged.

## Agent digest
- Claim: Lanes record a typed progress phase on live tickets without a claim; read-only checks screen lane heads, requirement IDs and effects proposals.
- Status: experimental under the owner's standing lane auto-accept (2026-10-11; V1-3569, V1-3463, V1-3577, V1-2909, V1-3331, V1-3576); delivery experimental: the progress phase, collision check, lane-ref screen, effects proposals and `ticket split` exist with focused tests.
- Exists: operation `PROGRESS_SET` and the optional record member `progress`, the `ticket progress` verb, the `--phase` search filter, phase in show and roadmap rows, closeout-debt `progress-head` rows, `tools/requirement-id-collisions`, delivery-screen `--lane-refs`, `ticket effects-proposals`, and `ticket split`.
- Blocked on: the operator-applied batch set-effects of V1-3576 (lanes may not write the store); durable qualification.
- Read next: Requirements; Failure modes and trust; Acceptance evidence and traceability; Rollout and rollback.

## User and current state

With the barrier OPERATOR/ADMISSION no lane can claim, so every in-flight ticket reads OPEN with no
live attempt. The only record of where work stands was a hand-stamped operator note: unstructured,
replaced on each write and not queryable. A local lane branch can also hold delivered tickets and new
requirement IDs that no read on `origin/main` sees, so lanes are dispatched for work that is already
done and number requirements that a peer branch already used.

## Requirements

- `TLF-V0-880`: `corvint-tasks ticket progress --target T --expected-revision N --payload P` MUST
  apply operation `PROGRESS_SET` with the closed payload `{branch, head, host, lane, phase, pr,
  reason}` to a DRAFT, OPEN or HELD NATIVE ticket as one receipt-backed revision. `phase` is one of
  `IN_PROGRESS`, `DELIVERED_ON_BRANCH`, `IN_REVIEW`, `IN_MERGE_QUEUE`, `MERGED_AWAITING_CLOSEOUT`,
  `BLOCKED` or `CLEARED`; `lane` and `host` are labels or null, `branch` an identifier or null,
  `head` a Git OID or null, `pr` a count or null and `reason` prose of at most 512 bytes or null.
  `DELIVERED_ON_BRANCH` requires branch and head, `IN_REVIEW` and `IN_MERGE_QUEUE` require pr,
  `MERGED_AWAITING_CLOSEOUT` requires head (the merge commit), `BLOCKED` requires reason, and
  `CLEARED` admits only reason. The writer MUST append exactly one entry, the payload plus `actor`
  and `recordedAt` from the trusted binding and clock, to the optional record member `progress`
  (at most 64 entries, append-only). The post record MUST equal the pre record in every member except
  `progress` and the finalizer fields; `acceptanceRevision`, status, gates, criteria and completion
  MUST stay unchanged. The writer MUST refuse a COMPLETED, ARCHIVED or IMPORT ticket
  BLOCKED/TICKET_STATE, a payload equal to the latest entry VALIDATION_FAILED/DUPLICATE_ID, a 65th
  entry VALIDATION_FAILED/LIMIT_EXCEEDED, a malformed payload or a phase missing its required members
  MALFORMED, and a stale expectedRevision REVISION_CONFLICT. OWNER and OPERATOR hold the operation by
  default; no other role may be granted it. ADOPT_FILE and IMPORT_APPLY MUST refuse any `progress`
  difference, and the Tasks and Core readers MUST refuse a malformed member.
- `TLF-V0-881`: A ticket's effective progress MUST be its latest entry while the ticket is DRAFT,
  OPEN or HELD and that entry's phase is not `CLEARED`, and none otherwise; completion and archive
  therefore clear it without a further write. Progress MUST NOT be read by admission, claim
  eligibility, planning order, ranking, gates, criterion binding, completion or authority. A
  `PROGRESS_SET` write MUST succeed under an OPERATOR/ADMISSION barrier, and the same barrier MUST
  still refuse a claim after the write.
- `TLF-V0-882`: `ticket show` MUST list the entries and the effective progress, each `ticket
  search` and `roadmap` row of a live ticket with effective progress MUST carry `progress` (the
  effective entry), and `ticket search --phase PHASE` MUST match exactly the tickets whose effective
  phase is PHASE; `--phase NONE` matches tickets without one. A closed ticket never matches a phase
  other than `NONE`.
- `TLF-V0-883`: `ticket closeout-debt` MUST report each OPEN ticket whose effective progress names a
  head reachable in the scanned ref as a candidate row with citation `progress-head` and the head as
  its commit, alongside the message-cited rows, so a marker whose branch head merged is reported even
  when no commit message names the ticket.
- `TLF-V0-884`: `corvint-tasks ticket split ID --criteria N,M --expected-revision R --request-id Q`
  MUST create one NATIVE child ticket carrying the named acceptance criteria (1-based, in parent
  order), the parent's `requiredRoles`, `requiresPool` and `executionPrerequisites`, a copy of every
  digest of the parent's attached-evidence entries with each entry's acceptance revision, actor and
  reason kept in a copy's reason, and a `supersedes` link to the parent, and then remove those
  criteria from the parent. It runs as an ordered sequence of ordinary single-ticket mutations under
  derived request IDs (`Q/create`, `Q/child` when the parent has executionPrerequisites, which CREATE
  cannot carry, so the child opens only with them, then `Q/evidence/I`, `Q/refine`) sharing one
  `issuedAt`, so each step writes its own receipt. The parent is changed last, and its REFINE MUST
  refuse `STALE_TICKET`, under the same writer lock, unless the child still carries the moved
  criteria at the acceptance revision the split gave it: an interrupted split leaves the criteria on
  both tickets, never on neither, and rerunning the same command with the same `--request-id` and
  `--issued-at` replays the finished steps and completes the rest, first settling a receipt the
  interruption left `REDO_PENDING`. A split whose `Q/refine` is already recorded writes nothing only
  when that record is this split (the parent at R+1, a recorded `Q/create` whose child supersedes
  the parent with this split's note); any other record refuses `REQUEST_ID_CONFLICT`. It MUST
  refuse, writing nothing, a parent that is not OPEN or DRAFT or not at revision R, an empty,
  repeated or out-of-range criterion list, a list naming every criterion, a parent body that cannot
  take the bounded split note (`LIMIT_EXCEEDED`, also in `--dry-run`), and a MANUAL or EXTERNAL
  parent with executionPrerequisites (`TICKET_STATE`).
- `TLF-V0-885`: `ticket split --dry-run` MUST print the child CREATE payload and the parent's
  resulting criteria and write nothing; after a split the parent MUST complete on its remaining
  criteria and receipt audit MUST stay CONSISTENT.
- `TLF-V0-886`: `tools/requirement-id-collisions` MUST read every local branch and detached worktree
  head that shares history with the base ref (default `origin/main`; it contains one of the base's
  root commits) and is not contained in it, and report, as `REQUIREMENT_ID_COLLISION`, each
  requirement ID whose defining bullet (`` - `ID`: `` in `docs/specs/*.md`) is new on two of those
  refs with a different file or first line, is defined in two files of one ref, or is defined on a
  ref in a different file than on the base. An amendment of an existing ID's text in its own file is
  not a collision. It MUST exit 1 on a collision, 0 when none, and 2 on a read failure or more than
  256 lane heads, MUST print the scanned heads, and MUST write nothing. Its Git reads MUST NOT
  lazily fetch a missing promisor object or use any transport (a missing object is a read failure,
  exit 2), and repository configuration such as `grep.lineNumber` or `grep.column` MUST NOT change
  how a defining bullet is parsed.
- `TLF-V0-960`: (experimental under the standing lane auto-accept, 2026-10-31; V1-3650; refines
  `TLF-V0-886`) Before comparing definitions, `tools/requirement-id-collisions` MUST skip every
  scanned head that another scanned head contains (`git merge-base --is-ancestor`), such as a lane
  head already merged into a batch branch, and list the skipped heads on one
  `skipped N lane head(s) another scanned head contains:` line. The containing head carries the
  lane's bullets, so a batch that later rewords one is no collision; a lane the batch does not
  contain is still compared with it. Evidence: `TestTLFV0960_LaneHeadMergedIntoBatchIsSkipped`
  (fails on BASE with a false collision), the updated `TestTLFV0886_ReproducesASDV0Collisions`;
  on the WSL host (2026-10-31) the check scanned 16 of 28 heads, skipped 12 contained ones, and
  kept the one real collision (`CAP-V0-970`). Rollback: drop `outermost`.
- `TLF-V0-887`: `ticket delivery-screen --lane-refs` MUST additionally scan every local branch and
  detached worktree head not contained in the screened ref that shares a root commit with it (an
  unrelated lineage is no lane), at most `--limit` commits each outside
  it, and report a candidate ticket that such a commit delivers (a delivery line or ticket-scoped
  subject of `TCD-V0-167`) as `DELIVERED_ON_LANE` with `laneCommits` (ref, commit, subject), after
  `DELIVERED` and ahead of `TRUNCATED` and `CLEAR`, and a candidate whose effective progress phase is
  `DELIVERED_ON_BRANCH`, `IN_REVIEW`, `IN_MERGE_QUEUE` or `MERGED_AWAITING_CLOSEOUT` as `IN_FLIGHT`;
  the item lists the scanned `laneHeads`, and more than 256 such heads is refused
  `LIMIT_EXCEEDED`. A lane scan cut at `--limit` MUST be reported as `scanTruncated` on its
  `laneHeads` entry and `laneScanTruncated` on the item, and a candidate not otherwise resolved that
  was created before the oldest scanned commit of such a lane is `TRUNCATED`, never `CLEAR`.
  Without the flag the screen MUST be unchanged.
- `TLF-V0-888`: `ticket effects-proposals` MUST, for each OPEN ticket whose effects coverage is
  `UNKNOWN`, propose touch paths derived from repository paths named in the ticket's title, body
  and acceptance criteria that exist in the worktree, each with the text span that named it, and
  report tickets with no candidate as `NO_PROPOSAL`. A path token holds a slash or an extension,
  or is a bare name (a dotfile such as `.gitignore`, or an extensionless name such as `Makefile`)
  that exactly matches a non-directory entry of the worktree root. It is a pure read: it MUST NOT write the store,
  the journal or the repository.
- `TLF-V0-889`: A proposal MUST NOT change claimability, eligibility, blockers, planning or any
  record until an operator applies it with `ticket set-effects`; the proposal output is labelled
  `proposal: not authority` and carries no request id or payload that the writer accepts as-is.
- `TLF-V0-820`: (experimental under the owner's standing lane auto-accept (2026-11-01; V1-3603);
  refines TLF-V0-880) The CLI MUST read an absent `branch`, `head`, `host`, `lane`, `pr` or
  `reason` key of a `ticket progress` payload as null, filling it before the canonical payload and
  its request digest, so a payload that omits a nullable key and the same payload spelling it
  `null` are one request. An absent `phase` MUST still refuse MALFORMED, and the refusal MUST name
  the missing key. The mutation envelope's payload stays the closed seven-key object. Non-goals:
  default values for any other operation's payload. Rollback: revert; an omitted key refuses
  MALFORMED naming it.

## Failure modes and trust

- Progress entries are claims by the writing principal. Nothing checks that the branch or head
  exists; `closeout-debt` and `delivery-screen` only report what Git shows them.
- A crash between stage and commit is the existing ordinary-mutation case; no derived event is
  declared. Two writers serialize through the store lock and the expectedRevision CAS.
- Revision bump: a progress write consumes one ticket revision; a live attempt binds the acceptance
  revision, not the record digest, so a progress write never invalidates it.
- The collision tool and lane-ref screen read local refs only; a peer lane on another host whose
  branch was never fetched stays invisible and is reported as such in the result's `scanned` list.
- Effects proposals are heuristics over prose and are labelled as such.
- A split is not atomic across its steps. Another writer can change the parent between the child
  CREATE and the parent REFINE; the REFINE then refuses on the expectedRevision CAS, the child stays,
  and the operator refines the parent by hand. A rerun after any interruption sees each recorded step
  as a replay.

## Non-goals and simpler baseline

- No automatic advance from GitHub: no read queries GitHub, and no merge writes progress.
- No removal or edit verb: a mistaken entry stays in the history and a later entry supersedes it.
- No progress on IMPORT tickets and no foreign-import carrier.
- No store write by `effects-proposals`; the bulk apply is an operator step.

## Acceptance evidence and traceability

| Requirement | Ticket acceptance | Implementation boundary | Delivered evidence | Required integrated evidence (NOT_RUN) |
|---|---|---|---|---|
| TLF-V0-880 | V1-3569 criterion 1; V1-3463 criterion 1 | `internal/tasks/ticket` (progress codec, view); `internal/tasks/mutation` (payload, Apply, adopt); `internal/tasks/intent` (role matrix); `internal/tasks/transaction` (import guard); `internal/taskman` (Core reader); `internal/tasks/cli` (verb, help) | `TestTLFV0880_ProgressSetLeavesTheRecordUnchanged`; `TestTLFV0880_ProgressSetRefusals`; `TestTLFV0880_RecordCodec`; `TestTLFV0880_ReaderProgress`; `TestTLFV0880_AdoptFileRefusesProgress` | concurrent two-process CAS; durable qualification |
| TLF-V0-820 | V1-3603 | `internal/tasks/mutation` (`CanonicalPayload`) | `TestTLFV0820_ProgressPayloadOmittedNullableKeys` | none |
| TLF-V0-881 | V1-3569 criteria 3-4 | `internal/tasks/ticket` (EffectiveProgress) | `TestTLFV0881_BarrierAdmitsProgressAndStillRefusesClaims`; `TestTLFV0881_EffectiveProgress` | none |
| TLF-V0-882 | V1-3569 criterion 2; V1-3463 criterion 2 | `internal/tasks/cli` (search, roadmap, show) | `TestTLFV0882_SearchAndRoadmapByPhase` | none |
| TLF-V0-883 | V1-3569 criterion 3 | `internal/tasks/cli/closeout_debt.go` | `TestTLFV0883_CloseoutDebtReportsMergedProgressHead` | integration with V1-3496 closeout-debt changes |
| TLF-V0-884, TLF-V0-885 | V1-3577 | `internal/tasks/cli/split.go` | `TestTLFV0884_SplitMovesCriteriaToALinkedChild`; `TestTLFV0884_InterruptedSplitResumes`; `TestTLFV0884_SplitRefusals`; `TestTLFV0885_DryRunWritesNothing`; `TestTLFV0885_ParentCompletesOnRemainingCriteria`; `TestTLFV0884_SplitKeepsExecutionConstraints`; `TestTLFV0884_ResumeRefusesARefinedChild`; `TestTLFV0884_ResumeSettlesAPendingReceipt`; `TestTLFV0884_RecordedRefineMustBeThisSplit`; `TestTLFV0884_ParentBodyAtTheLimitRefusesFirst`; `TestTLFV0884_EvidenceProvenanceSurvivesRepeatedDigests` | durable qualification |
| TLF-V0-886 | V1-2909 | `tools/requirement-id-collisions` | `TestTLFV0886_ReproducesASDV0Collisions`; `TestTLFV0886_LineNumberConfigStillParses`; `TestTLFV0886_PartialCloneNeverFetches` | none |
| TLF-V0-960 | V1-3650 | `tools/requirement-id-collisions` (`outermost`) | `TestTLFV0960_LaneHeadMergedIntoBatchIsSkipped`; `TestTLFV0886_ReproducesASDV0Collisions` | none |
| TLF-V0-887 | V1-3331 | `internal/tasks/cli/delivery_screen.go` | `TestTLFV0887_DeliveryScreenSeesUnmergedLaneHead`; `TestTLFV0887_TruncatedLaneScanIsNotClear` | none |
| TLF-V0-888, TLF-V0-889 | V1-3576 criteria 1 and 3 | `internal/tasks/cli/effects_proposals.go` | `TestTLFV0888_EffectsProposals`; `TestTLFV0888_RootFilesWithoutExtension`; `TestTLFV0889_ProposalsAreNotAuthority` | V1-3576 criterion 2 (operator-applied batch set-effects) BLOCKED: lanes may not write the store |

## Resolved decisions

1. Representation: an optional append-only record member written by an ordinary mutation, as for
   TEA-V0-001, instead of a parallel event chain (the operator-note pattern) or a side ledger, which
   would not be receipt-backed.
2. Role: OPERATOR holds `PROGRESS_SET` by default, because the barrier case is exactly the one where
   only the operator can write and the live policy grants no explicit rows.
3. Live statuses: DRAFT, OPEN and HELD admit progress; a blocked or held ticket is often the one
   whose state matters most.
4. Clearing by derivation: completion and archive end the effective progress, so no closeout verb
   needs a second write.
5. Phase vocabulary: V1-3463's "committed", "PR open" and "merged" map to `DELIVERED_ON_BRANCH`,
   `IN_REVIEW` and `MERGED_AWAITING_CLOSEOUT`; V1-3569's "awaiting merge" is `IN_MERGE_QUEUE`.
6. Split transaction model (TLF-V0-884): an ordered, resumable sequence of existing single-post
   mutations, not a new multi-post receipt. Every receipt validator, recovery path and audit assumes
   one ticket post per mutation; a multi-post receipt would change persisted journal state for one
   verb. Changing the parent last makes the only intermediate state a duplicated criterion, which a
   rerun resolves.

## Unresolved decisions

- Whether `batch-delivered` should emit ready `PROGRESS_SET` payloads for each delivered ticket.

## Rollout and rollback

Rollout is additive: a record without progress keeps its exact bytes. Rollback is reverting the
change before any store holds a record with `progress`; after such a write an older binary refuses
that record at the closed reader (fail-closed), and recovery is the newer binary or a store backup
taken before the first progress write. The tools and read flags have no state to roll back.
