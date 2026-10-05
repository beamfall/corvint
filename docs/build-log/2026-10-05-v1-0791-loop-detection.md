## 2026-10-05 V1-0791: opt-in no-progress loop detection as a derived hold

Human-owned intent: owner request [issue 587](https://github.com/beamfall/corvint/issues/587) part 2,
ticket V1-0791, owner decision D8. Routing loops that hand a ticket back and forth without changing
anything consumed about 26 sessions on one ticket, and no read could see the loop. The owner asked
for an opt-in hold derived from audited attempt history, shaped like ESC-V0-006, that changes no
byte while the policy does not opt in.

Requirements: `CAL-V0-102..103` in `docs/specs/corvint-tasks-agent-leases-v0.md` (V1-0791
amendment, inside `## Requirements`), assigned by the coordinator, and TCP-00 amendment A23 (A22 is
taken by V1-0780 in the same batch).

### Change

- Policy: optional top-level `loopDetection {maxNoProgressGenerations, maxAlternatingReturns}`,
  both required counts 1..256, closed keys; omission keeps the canonical policy bytes.
- Wire: optional absent-only `priorGenerations[]` key `loopEvidence {disposition,
  candidateTreeOid, gateResults, reviews}` of `taskman-attempt/0`, written by a re-claim only while
  the policy carries `loopDetection`, and only beside the V1-0788 recorded history. The closed
  decoder refuses it without history, `NONE` beside a recorded target, and `REVIEW_RETURNED` on a
  non-review stage. New closed detail code `LOOP_DETECTED`: 74 codes after A21's 73.
- Derivation (`transaction.LoopHoldOf`): from the ticket's newest attempt when it is ended, not
  `COMPLETED` and bound to the current acceptance revision. No progress counts the newest run of
  clean `HANDOFF` generations with no gate result, no review and no new tree; alternating returns
  counts implement-`HANDOFF`/review-`REVIEW_RETURNED` pairs. Generations without evidence are
  UNKNOWN: they never count and break the run.
- Surfaces: the hold rides on `ticket.Context.Loop`, so eligibility, claim, claim-next, recorded
  claimability, `plan preview`, `ticket show` and `ticket blockers` report `LOOP_DETECTED` with the
  counted generations and next action `reopen`. The dispatcher observation carries it, the roster
  skips the ticket, `dispatch status` lists `loopDetected`, and the dispatcher emits one
  `needs-owner` event `{kind: blocked, code: LOOP_DETECTED, signal, generations}` per episode
  (review round 2 added `acceptanceRevision`),
  remembered in the optional closed ledger member `seen.loops`.
- Clearing: an acceptance-revision change; the owner's existing `ticket reopen` (CAL-V0-043) now
  also admits a loop-held ticket. A ticket that is neither exhausted nor held still refuses
  `RETRY_BUDGET_NOT_EXHAUSTED`.

### Review round 1

Codex r1 returned three P2 findings, all fixed with regression tests:

- The ledger reader ran its strict member check only beside `progress` or `poolSweeps`, so a ledger
  carrying only `seen.loops` admitted aliases and duplicate members. Any member folding to `seen`
  that holds a member folding to `loops` now requires the strict reader, which also refuses a
  `null` map or hold (`TestCALV0103_LedgerLoopsStrictWithoutProgress`).
- Plan and claim-next kept blocker detail only under author exclusion, so they reported
  `LOOP_DETECTED` without its evidence. A held plan entry now carries the optional member
  `loop {signal, acceptanceRevision, generations, limit}`, and a claim-next refusal appends the
  hold's detail; both are absent without the policy (`TestCALV0102_CLILoopHoldSurfaces`,
  `TestCALV0102_ClaimHonoursLoopHold`).
- The dispatcher recorded an episode before its event was appended and ignored the append error,
  so an unwritable event log lost the event for good. An episode whose append fails is now kept
  with the optional ledger member `pending: true` and retried on the next tick or after a restart
  (`TestCALV0103_LoopEscalationSurvivesEventAppendFailure`).

### Review round 2

Codex r2 confirmed the r1 fixes and returned one P2: a failed append could leave bytes behind.
A part-way write left an unterminated fragment, so the retried line merged with it and was dropped
as unparseable; a complete write followed by a close or sync error left the episode pending beside
a readable event, so the retry duplicated it. Fixed with fault-injection tests through a test seam
on the append helper:

- Every event append first ends a trailing unterminated fragment with a newline, so the appended
  line parses. This is in the shared helper rather than only the loop retry, because any event
  appended after a fragment, such as `stopped`, otherwise merges with it; a well-formed log
  receives exactly the same bytes (`TestCALV0103_LoopEscalationSurvivesPartialEventWrite`).
- Before raising an episode the dispatcher looks for its event in the readable 1 MiB tails of the
  current and rotated logs and, when found, records the episode without appending
  (`TestCALV0103_LoopEscalationNotDuplicatedAfterCloseFailure`). The episode key, and therefore the
  event detail, the `seen.loops` hold and the `dispatch status` rows, gained `acceptanceRevision`,
  because after a reopen the newest generation alone need not distinguish two episodes.
- The check runs before every raise, not only a pending retry, so a crash between the append and
  the ledger save also raises no duplicate while the log is readable
  (`TestCALV0103_LoopEscalationNotDuplicatedAfterUnsavedLedger`). The recorded limit narrows to an
  unreadable log or an event that has left the scanned tail.

### Review round 3

Codex r3 approved `ced25d81` with no P1, P2 or P3 findings. Two areas stay unverified: rotation
recovery, which was inspected in source only and has no dedicated regression test, and power-loss
durability, which the fault injection (a close failure and lost ledger state) does not establish.

### Decisions and limits

- Acknowledgement reuses owner `ticket reopen` instead of a new verb. It bumps the acceptance
  revision and also restarts the retry budget through the fresh attempt. Whether a lighter
  operator acknowledgement is wanted was raised with the owner, who kept reopen (2026-10-05).
- The dispatcher escalation is an event, not a native escalation record: the ESC-V0-010 OPEN
  writer requires an active lease and a matched reservation, which a held ticket has not got.
  Whether the escalation writer should gain a dispatcher- or operator-origin OPEN was raised with
  the owner, who kept the event (2026-10-05).
- Evidence is recorded only under the opt-in, so history written before it is UNKNOWN and a loop
  that began earlier is detected only after enough newly recorded generations.
- A hand-off with `--evidence` but no tree counts as no progress; work outside the candidate tree,
  gates and reviews is invisible to the signal.
- Not evaluated in `ticket list`, `roadmap` or `queue status`. The dispatcher also emits at its
  baseline observation.
- Core is unchanged: its closed policy reader (`internal/taskman/capture.go`) already refuses a
  policy carrying `loopDetection`, as it refuses `pools`, `supervision` and `externalReviews`.
- Downgrade: older closed readers refuse a policy history that ever carried `loopDetection`, and
  (inferred from the closed key set) an attempt carrying `loopEvidence`; the spec rollback names
  the byte-string checks and the backup route.

### Evidence

- D8 byte equality: `TestCALV0102_PolicyAbsentMatchesNMinusOne` pins the SHA-256 of a
  policy-absent transcript (three claim/hand-off rounds, a fourth claim, claim-next, both plans,
  recorded claimability and ticket views). The same file, which uses only pre-change API, produced
  the same digest on base `58d089327178d834762f7fb83df3c013749ec386`;
  `TestCALV0102_NMinusOneScenarioOptsIn` shows the opted-in scenario records evidence and holds.
- Focused tests: `TestCALV0102_*` and `TestCALV0103_*` in
  `internal/tasks/{intent,snapshot,transaction,store,cli,dispatch}`,
  `TestAgentLeasesSpecEnumeratesCALV0102` (`internal/lrfrepo`), the wire code count, the focused
  packages and the store subset named in the handoff, gofmt, `go vet ./internal/tasks/...`,
  `go build ./...`, the `GOOS=windows` cross-build, the CI doc gates and use-case receipt checks.
- NOT_RUN: `make gate`, the repository-wide suite, interop, live dispatcher and store qualification,
  an older binary reading a store with the new keys, and the dogfood bind/seal (owned by the
  coordinator after review).
