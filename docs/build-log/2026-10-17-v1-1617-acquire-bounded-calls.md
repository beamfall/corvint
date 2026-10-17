# V1-1617: bounded pool acquire calls (issue 1010 follow-up)

Date: 2026-10-17
Ticket: V1-1617 (follow-up to GitHub issue beamfall/corvint#1010, which is closed)
Requirements: PSR-V0-110..116 (`docs/specs/corvint-tasks-pool-safe-reuse-v0.md`, section "V1-1617
bounded acquire calls"). They are accepted under the owner's standing auto-accept of lane
requirements (2026-10-11). The agent leases spec is at its 256-obligation bound, and nothing was
added to it.

## Problem

The PSR-V0-051 resume base lives only in the memory of one command. Building it costs one
early-stopped complete attempt plus one prefix walk. A client that bounds each `pool acquire` at
150 s and starts a new process every time therefore never reaches a resumed round. The reporter
saw 11 `ACQUIRED` against 146 `PENDING`, with one member `FREE`, at 47,000 receipts and 3 to 5
fast-route writes a minute.

## Decision: route (b), the detached acquire; route (a) declined

**Route (a)** would accept a tail of only heartbeat or renew receipts against the audited head. It
is declined (PSR-V0-110) for three reasons:

1. **It adds nothing.** PSR-V0-052 already accepts every valid tail of at most 256 receipts,
   whatever its kinds. It validates each receipt through the shared step and models the request
   again at the new head. A kind-classified route would add no resumed round that the existing
   route does not already give.
2. **It trusts what only the step can check.** Its proof would have to show that a receipt kind
   cannot change pool membership, allocation or the FREE/ALLOCATED view. But the kind is a field
   of the receipt itself, so trusting it before the step has checked the receipt is exactly what
   PSR-V0-053 forbids. The new negatives show that the existing route trusts no kind: a forged
   kind, a forged request record and a "heartbeat" whose files change policy membership or free an
   allocation all fall back to the complete route's own decision.
3. **It breaks for the acquirer's own attempt.** A heartbeat or renew of the acquiring attempt
   changes the generation that the acquire was modelled on, so "heartbeat-only" is not
   decision-neutral even in principle.

**Route (b)** carries progress across bounded calls. It needs no new persisted state: the detached
acquire (ATR-V0-020..029, issue 1005) already runs the command in one child process for up to
600 s, behind a request id. A client that bounds each call issues `--detach --request-id R --wait
N`, and then `--attach` or the same `--detach`, and gets `PENDING` (exit 75) until the one child
finishes. The child keeps its in-memory base across the client's calls. PSR-V0-051 is unchanged
and no base is written to disk (PSR-V0-111).

## Criterion 3: retry count and audit duration

This is delivered for the detached route (PSR-V0-112). After each preparation round, the child
writes `progress.json` with the rounds begun, the resumed writes and the summed audit milliseconds.
The `PENDING` item carries these as `progress`, or `null` before the first round. It is advisory:
the kept result, replay and `--attach` purity are unchanged. A non-detached `SNAPSHOT_MOVED` still
reports no retry count, because no caller can wait on it, and it remains NOT_PRODUCED as in
V1-1560.

## Evidence (reduced scale)

Opt-in harness `TestPSRV0055_PoolAcquireLoad` setup:

- 10,000 receipts, 2 acquirers and a two-member pool;
- the new `WRITER_KIND=lease` dispatcher: claims, then a heartbeat or renew every 15 s, which is
  about 4 writes a minute;
- 180 s of load plus 60 s of grace;
- 12-CPU Darwin host at load 69 to 87 (shared with other lanes).

| Mode | Acquires | Result |
| --- | --- | --- |
| Cut at 150 s (above the audit) | 4 of 4 `ACQUIRED` | None cut; all resumed; at most 2 rounds; p50 56.9 s, max 65.8 s; audit max 60.0 s; watch max 2.0 s |
| Cut at 30 s, new call each time | 0 of 6 `ACQUIRED` | All 6 cut; none resumed; audit max 48.3 s |
| Detach, 30 s windows | 2 of 2 `ACQUIRED` | Both resumed; within 2 windows (58.9 s); audit max 51.1 s |

The cut and detach runs scale the bound down so that it sits below the first audit, as 150 s does
at 47,000 receipts.

The harness detach mode models the detached child as one uncut call counted in client windows. That
the CLI `--detach` keeps one child and one mutation per request id is covered by the ATR-V0
tests. `TestPSRV0112_PendingReportsProgress` covers the `progress` member.

**Extrapolation (inference, not measured).** The measured first audit at 10,000 receipts is 33 to
60 s. Linear extrapolation gives 155 to 282 s at 47,000 receipts. That exceeds 150 s, so no in-call
change short of a faster walk would let a cut call finish. A detached acquire would finish in 2 to
3 windows of 150 s, inside its 600-second ceiling, provided the first audit stays under about 500 s.

**NOT_RUN:** the 47,000-receipt, quiet-host and Linux runs are NOT_RUN.

## Issue 1063 folded in

GitHub issue 1063 reports a pool acquire that stayed CPU-bound for 38 minutes or more on a moving
journal. The coordinator folded it into this lane with three asks.

1. **Start from a valid writer checkpoint, or end the prefix walk at the checkpoint sequence and
   audit only the tail. Declined.** This is the CAL-V0-264 writer route for pool verbs. The owner
   declined that route because it drops the CAL-V0-116 rewritten-prefix detection, and a lane
   cannot reverse it. It remains an owner decision.
2. **Report walk time and restart counts. Met for detached acquires.** PSR-V0-112 reports them as
   `auditMillis` and `preparationRounds`. There is no split of complete and prefix walks, and no
   `doctor` output.
3. **Note the walk cost of elided receipts in the `receipt compact` help. Met** by PSR-V0-114.

Experiment, kept as `TestPSRV0115_ResumeAfterCompaction` (PSR-V0-115): a journal with elided
heartbeat receipts still yields a resume base, with one complete audit and one resumed write.
Compaction alone therefore does not keep an acquire on complete audits.

The root cause of the reported run is NOT_OBSERVED. One candidate mechanism, an unconfirmed
inference from reading the code:

- `resumeFrom` sets a command-wide `noResume` after any failed `ResumeBase`, for example a
  `requests/` file in the capture listing that the prefix walk did not read.
- From then on, all remaining rounds of the 16 run complete audits.
- At multi-minute walks, that alone gives 38 minutes or more.

Confirming it needs a 47,000-receipt run with per-round stage timing, which is NOT_RUN here. The PR
therefore carries `Refs #1063`, not `Closes`.

## Review round 1

One independent review round (Codex) on PR #1069 raised two findings. Both are fixed.

- P1: the PSR-V0-056 recheck did not reread the tail.
  - Under the writer lock and after modelling, `RecheckResume` checked:
    - the base receipts;
    - base request files;
    - evidence and pinned files.
  - It never reread receipts after the base. A tail-posted request file was accepted by its
    canonical entry alone.
  - A tail receipt or tail request record rewritten after the resumed audit, with its modification
    time restored, was therefore committed over.
  - `TestPSRV0116_TailRewrittenAfterAuditNeverCommits` reproduced this: the acquire committed
    receipt 80.
  - Fix (PSR-V0-116): the resumed walk keeps the length and SHA-256 of every file it consumed
    (`Result.resumeRead`). The recheck rereads each tail receipt and each request file the tail
    posted, and compares them with those values.
  - The new store test and two new journal cases of
    `TestPSRV0056_RecheckResumeSeesChangeAfterAudit` fail without the fix and pass with it.
  - The fast path still resumes: `TestPSRV0112_LeaseProgressAfterEachRound` and the PSR-V0-052..056
    store tests pass.
- P2: the spec's failure mode said a repeat `--detach` after the ceiling starts a fresh child. The
  code (`internal/tasks/cli/lease_detach.go`) returns the kept result whenever `result.json`
  exists, which ATR-V0-021 and ATR-V0-022 require.
  - The code is correct, so the spec is fixed. The failure mode and PSR-V0-111 now say that the
    kept result answers the request id and that only a new request id starts a fresh child.
  - `TestPSRV0111_CeilingNeedsAFreshRequestID` checks both:
    - a repeat after the ceiling returns the kept bytes;
    - a new request id completes.

## Notes

- The coordinator reported that another lane's `pkill` ended some `store.test` runs around
  15:2x UTC. None of them were this lane's runs. This lane's runs used other `-run` patterns and
  completed on their own.
- Residual: a faster prefix walk, so that a cut call itself fits 150 s at 47,000 receipts, is out
  of scope here.
