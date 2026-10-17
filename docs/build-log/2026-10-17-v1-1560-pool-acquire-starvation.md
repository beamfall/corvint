# V1-1560: pool acquire under a moving journal (issue 1010)

Date: 2026-10-17
Ticket: V1-1560 (GitHub issue beamfall/corvint#1010, P1)
Requirements: PSR-V0-050..055 (`docs/specs/corvint-tasks-pool-safe-reuse-v0.md`, section "V1-1560 pool
acquire under a moving journal"). They are accepted under the owner's standing auto-accept of lane
requirements (2026-10-11). CAL-V0-264 stays declined, and this change does not revive it.

Re-homed (owner-approved, 2026-10-17): these requirements were first numbered CAL-V0-330..337 in the
agent leases spec; they moved to PSR-V0-050..057 in `docs/specs/corvint-tasks-pool-safe-reuse-v0.md`
with the same text so the leases spec stays within the 256-obligation bound.

## Problem

`pool acquire` waited 10 to 40 minutes. The setup was a journal of about 45,000 receipts, a
dispatcher committing every few seconds, and 8 acquirers.

Pool verbs are not on the CAL-V0-116 writer route, so each of the 16 `leaseWrite` rounds
(CAL-V0-263) ran the complete audit from receipt 1 outside the gate. A write anywhere during that
audit made the round `SNAPSHOT_MOVED`. Each retry paid for a full audit again, so the CPU per
waiting acquirer grew with retries. Whether a round succeeded depended on the gap between writes
being longer than the audit itself.

## Design

Each lease command reuses its own settled audit from receipt 1. Nothing is shared or persisted.

1. **Prefix fallback (PSR-V0-050).** The preparation audit makes one complete attempt. That attempt
   stops early when `head.json` moves, checked every 512 items. The audit then walks the prefix to
   the head it captured, using the CAL-V0-265 walk. A prefix walk never decides a refusal: when it
   refuses, the result is `ErrPrefixDeclined`, and for the rest of the command the complete audit
   decides.
2. **Resume base (PSR-V0-051).** A settled `ModeFull` or `ModePrefix` result, with its own
   observation, yields an in-memory base. The base holds the writer checkpoint plus, for every
   receipt, request, evidence and pinned file the audit read, its stamps, change time, digest and a
   "settled" flag.
3. **Resumed write (PSR-V0-052).** A round that ended `SNAPSHOT_MOVED` with a base retries through
   `leaseWriter` under the gate and the writer lock, for any lease verb. The audit is
   `AuditForWriter` from the base's checkpoint (tail of 256 or fewer) plus `held()`, which checks
   that:
   - every base receipt is unchanged in stat and change time, and its bytes match when it was not
     yet settled;
   - every request shard file is a base file or a file the tail posted, and the base request count
     still matches;
   - base evidence and pinned files are unchanged, and tail-posted afterimages match their digests;
   - the invalidation token is unchanged.

   Any difference is a decline: nothing is written, and that round's ordinary preparation runs.
4. **Tamper detection (PSR-V0-053).** The CAL-V0-116 rewritten-prefix detection is kept because
   the base came from this command's complete or prefix audit. Any later rewrite changes the change
   time, so the resume declines, and the complete route then refuses the journal as before.
5. **Overtaken guard (PSR-V0-054).** Debugging the first load run showed that `WatchChanges`, which
   registers the change guard and visits every store file, was itself overtaken on most rounds
   (up to 13 rounds). A resumable round whose guard registration moved now still runs the bounded
   audit, only to derive a base, and ends `SNAPSHOT_MOVED`. Without a guard it never refuses,
   never takes the complete route and never commits.

Bound (PSR-V0-055): before its first resume, a command spends at most one early-stopped complete
attempt and one prefix walk. After that, each round costs one bounded writer section, plus one
more preparation audit per declined resume. Neither cost depends on the writer rate or on the
number of retries.

## Evidence

Focused tests (Darwin, Go 1.27.2):

| Test | Base cc487c92e | After |
|---|---|---|
| `TestPSRV0052_PoolAcquireResumesUnderWriter` (a writer appends during every preparation) | 16 complete audits, 16 rounds, `SNAPSHOT_MOVED` | completed, 1 audit, `Resumed` 1 |
| `TestPSRV0053_ResumeKeepsRewrittenPrefixDetection` | n/a | rewritten receipt (mtime restored) never commits; head unchanged |
| `TestPSRV0054_OvertakenGuardStillResumes` / `...KeepsRewrittenPrefixDetection` | n/a | completes with the guard always overtaken; tamper still refused |
| `TestPSRV0050/0051/0052_*` (`internal/tasks/journal`) | n/a | prefix fallback, settled-only base, 6 decline cases × settled/unsettled stamps |

Opt-in load harness `TestPSRV0055_PoolAcquireLoad` run settings:
- 10,000 receipts, 8 acquirer processes and one writer every 3,000 ms;
- 240 s of load, a 180 s grace and a 420 s hard stop;
- 12 CPUs on local Darwin, with host load 53 to 68 throughout (other lanes running).

Each acquirer cycles through claim, acquire, pool release, confirm-safe and release.

| | Base cc487c92e | After |
|---|---|---|
| acquire calls | 8 | 18 |
| completed | 2 (252 s, 288 s) | 18 (all resumed) |
| `SNAPSHOT_MOVED` after 16 rounds | 1 (374 s) | 0 |
| unfinished at hard stop | 5 | 0 |
| rounds max | 16 | 3 |
| wall p50 / max | 417.6 s / 443.2 s | 42.9 s / 65.2 s |

The base binary was built from a worktree at cc487c92e, with the same harness except that the
`Resumed` field was replaced by 0. Before the PSR-V0-054 change, the branch completed 20 of 20
acquires with up to 13 rounds (max 64 s).

## Limits

- **45,000 receipts: NOT_RUN.** At this host load, setup alone would take about 15 minutes. By
  linear extrapolation (an inference), the prefix walk would take about 3 minutes at host load 60.
  The documented bound of under 2 minutes is measured only at 10,000 receipts.
- **Linux and quiet-host runs: NOT_RUN.**
- **Retry count and audit duration in the retryable result: NOT_PRODUCED.** This was the issue's
  optional item. The CLI does not expose `Timing.Resumed`.
- **Requests that keep the complete route every round.** These are handoff-policy candidates,
  models that read an elided path, and commands with a declined prefix walk. They can still
  exhaust 16 rounds.
- **Listing cost under the writer lock.** The receipt and request-shard listings grow with the file
  count.
- **Platforms without change times.** These never resume.
- **Large tails.** A tail beyond 256 receipts declines.

## Independent review findings (PR #1018 at c843d6aab)

An independent Codex review found two defects. Both were confirmed against the code with
regression tests that fail on c843d6aab:

1. **High: the base check did not hold through commit.** `held` ran once, inside the resumed audit.
   No change guard watches the resumed route, and the commit rebind checks only the head and the
   intent tree. A receipt rewritten after the check could therefore be committed over.
   `TestPSRV0056_ResumeRechecksBaseBeforeCommit` rewrites receipt 2 at `fast.lease.observe` and at
   `fast.lease.model`. On c843d6aab the acquire completed and committed receipt 77 in both cases.
   Fix (PSR-V0-056): `leaseWriter` calls `journal.Reader.RecheckResume` under the lock, after
   modelling and the rebind and before it reports, retains or publishes anything. Every request
   shard is now listed afresh rather than taken from the audit's capture, so a shard made after
   the capture is also seen. The recheck costs one more receipt and request listing per resumed
   write.
2. **Medium: native-close failures in the base check became declines.** The check read through
   `Native.List` and `Native.Read`, each with its own lifetime, so a failed close turned into an
   ordinary decline. The command then fell back to preparation, against CAL-V0-116.
   `TestPSRV0057_ResumeCheckCloseFailureIsCleanup` fails the close of the check's receipts listing.
   On c843d6aab the result was "checkpoint unusable" with no cleanup error.
   Fix (PSR-V0-057): the check reads through one `nativeRead` of its own. The resumed audit folds
   that lifetime's close into its own cleanup channel (`UNSUPPORTED_FILESYSTEM`, read lifetime,
   `PhysicalObservation.Cleanup`), which `observeWriter` treats as terminal. `RecheckResume`
   returns a close failure as cleanup, and `leaseWriter` returns it as terminal.

## Rollback

Revert the commits. A resume base exists only in one command's memory, and no store format, receipt,
checkpoint format or wire result changes. Resumed writes advance the existing writer checkpoint
under CAL-V0-117, which is the same format either build reads.
