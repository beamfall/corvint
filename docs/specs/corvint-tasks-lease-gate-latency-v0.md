# Corvint Tasks lease-gate latency V0

Owner: Russell Lewis
Date: 2026-10-24
Intent status: proposed
Delivery status: experimental

Authoritative inputs: native tickets V1-2531, V1-2537, V1-2539, V1-2203 and V1-1702. These requirements
amend the write rounds of the [agent lease contract](corvint-tasks-agent-leases-v0.md), whose
Requirements section is at the 256-obligation OCM limit (`maxObligations`), so they live here.
They change no lease semantics, stored state, wire format or receipt.

## Agent digest
- Claim: Lease rounds skip the ready-route cleanup lock, keep fdinfo files open, parse tickets once; heartbeat and renew skip only staging-directory syncs.
- Status: proposed; experimental; LGL-V0-001, 002, 004, 005, 006, 007, 009 and 010 are implemented with focused tests; LGL-V0-011 measured a 47k-receipt pool release at p99 1.03 s (target 2 s); LGL-V0-003 is a measured target that is NOT MET (claim gate-wait p99 1.7 s against 500 ms); LGL-V0-008 keeps every published artifact's sync and records why; the V1-0887 heartbeat median of 50 ms is still NOT MET.
- Exists: the writer-route staging decline, retained Linux fdinfo files, the single-parse ticket loader, the wire ASCII scan and the heartbeat/renew staging-sync skip, each with a focused test.
- Blocked on: V1-2539 (claim wait) and V1-2203 (the rest of the heartbeat floor needs an owner store-format decision: LGL-V0-008).
- Read next: Requirements; Failure modes; Acceptance evidence.

## User and current state

Human-owned input: native tickets V1-2531, V1-2537, V1-2539 and V1-2203 (owner rule: performance
issues are 1.0 work, V1-0887 targets a heartbeat median of at most 50 ms and a p99 of at most
250 ms). On the WSL2 host at 84b08d351, `TestCALV0263_LeaseGateLoad` (40k receipts, 15 writers,
5 x 120 s) showed heartbeat and renew time outside the gate, in the round's guards, at p99
169-187 ms. The guards' orphan cleanup took the writer lock whenever `staging/` was non-empty, and
a live peer's commit keeps its slots there for its whole hold, so a ready writer-route write
queued on the lock for that hold before it reached the gate, then queued again inside the route.
A per-syscall count of one heartbeat at 40k receipts put about 85 % of the journal-write CPU in
the session's pinned-parent checks, each re-observing about eleven mounts by opening, reading and
closing `/proc/self/fdinfo/<fd>` (decision 0411).

## Requirements

- `LGL-V0-001`: (proposed; V1-2531) A lease write round MUST clear orphan stages under a cleanup
  lock before the preparation gate only when its writer-checkpoint route is not ready. When the
  route is ready the round MUST enter it with no cleanup lock; the route, holding the gate and the
  writer lock, MUST decline `staging` when the staging directory holds any entry (under the lock
  every entry is an orphan, because only the lock holder stages), writing nothing, and the round
  then clears orphans under a lock of its own exactly as before and continues to the resume or
  complete route. What a round commits is unchanged.
- `LGL-V0-002`: (proposed; V1-2203) On Linux a session MUST open each pinned descriptor's procfs
  `fdinfo` file once, on the descriptor's first mount observation, retain it for the session's
  life, and re-read it from offset 0 on every later observation (procfs regenerates the content
  on each read from offset 0), closing every retained file when the session closes its handles.
  Every observation still re-reads the mount ID decision 0411 requires; only the open and close
  per observation are removed.
- `LGL-V0-003`: (proposed; V1-2539) The claim latency target under `TestCALV0263_LeaseGateLoad`
  at 40k receipts and 15 writers on the WSL2 quiet host is a claim gate-wait p99 of at most
  500 ms (at most four claim holds of the measured 125 ms p99 queued ahead) and a claim wall p99 of
  at most 1500 ms. Every claim run reports both against this target, labelled with its platform
  and host load; a run that misses it is reported NOT MET, never rounded. The harness starts all
  15 writers' claims together, so claim gate wait is first-in-first-out serialization: the k-th
  claim waits for about k earlier claim holds plus the liveness holds interleaved with them, and
  a 500 ms p99 over 14 queued claims needs a claim hold near 36 ms, against a measured fsync and
  commit floor near 70 ms. The target therefore cannot be met by trimming in-gate CPU alone.
- `LGL-V0-004`: (proposed; V1-2531) Lease-gate validation MUST NOT parse a canonical ticket twice:
  `loadTickets` relies on `ticket.Decode`, whose `wire.Parse` already refuses any bytes that differ
  from the encoding of the parsed value plus one trailing LF, and drops the second parse and
  re-encode it performed. The wire hostile-code-point check, and the string parser before it
  formats a refusal position, MUST first scan the string's bytes and accept without decoding runes
  when every byte is below 0x80 and none is a control hostileRune refuses (TAB, LF and CR only
  where prose permits them); any other string takes the existing rune-by-rune check. Which bytes
  are accepted and which refusal, code and position are reported are unchanged.
- `LGL-V0-005`: (proposed; V1-2203) A heartbeat or renew session (`transaction.LivenessLease`) MUST
  make no sync call for any staging-directory sync: after a prepare, after a slot removal or orphan
  unlink, and after a replace's rename out of `staging/`. It MUST still cross the same
  `sync:staging` boundary and run the same checks there. Every file sync, every destination
  directory sync and every mkdir parent sync MUST still run, so each artifact the session publishes
  is as durable at its commit point as before. Every other verb MUST make every sync it made before,
  and a heartbeat or renew that first redoes another write's pending post effects MUST make every
  sync for that redo. The owner decision of 2026-10-24 for V1-2203 authorizes this for heartbeat and renew only, and
  amends decision 0549 F2 for those two verbs.
- `LGL-V0-006`: (proposed; V1-2203) A session MUST count the file and directory sync calls it makes
  (`Session.SyncCalls`), and a lease write's `LeaseTiming.Syncs` MUST sum them over the write's
  sessions. Like `SyncDuration` the count is a diagnostic, never proof or authority; the CLI does not
  report it. On both lease routes a heartbeat or renew makes two sync calls per artifact it publishes
  (8, or 10 when it creates a request shard), and a claim or release four per artifact.
- `LGL-V0-007`: (proposed; V1-2203) The only state a crash after a heartbeat or renew can leave that
  a full-sync session could not is staging residue: a slot name whose unsynced removal or rename
  the crash undid, holding a second link to an artifact the session linked (the receipt, the request
  entry) or replaced (the attempt, the head).
  That residue MUST read as the orphan a killed writer leaves (CAL-V0-019). With the writer lock free
  the audit sets it aside and verifies (CTS-V0-026); the lease reads as the acknowledged write left
  it; the next writer clears the residue and commits; and no other holder is granted the lease.
- `LGL-V0-008`: (proposed; V1-2203) A heartbeat or renew MUST keep the file sync and the destination
  directory sync of every artifact it publishes. The store reads a lost or torn tail receipt as
  `JOURNAL_FORKED` or `MALFORMED`, and a head that lost its last advance as `REDO_PENDING`; it never
  adopts a shorter chain as an older heartbeat. The owner decision's wider allowance (a liveness
  receipt committed without its own fsync, made durable by the next synced write or by a group sync
  of at most 1 s) is therefore not implemented: under the lane's stop clause it would weaken
  integrity. Adopting a lost tail would need a durable liveness watermark that the closed
  `head.json` member set cannot carry (a store-format cutover that old binaries refuse), recovery
  that deletes committed receipts and reissues their sequence numbers, and a periodic group sync
  that only a daemon could run (product invariant 7). The owner's premise also fails for renew. `planRenew` sets the expiry to the renew's own time plus
  its lease minutes, so a lost renew that had shortened a lease leaves the longer, earlier expiry in
  force. That design needs its own owner decision on the store format.
- `LGL-V0-009`: (proposed; V1-1702) On a segmented store whose bound writer checkpoint's `FullSeq`
  is the sealed sequence S (the manifest's `Through`), a new command MAY derive a resume base from
  the sealed segments instead of the complete audit (`SealedResumeBase`): the manifest, every
  segment and every evidence or pinned blob the base relies on are stamped, re-hashed by the resume
  precheck before the preparation gate, and re-hashed again under the writer lock with the
  manifest digest compared. The resumed walk then reads every active receipt S+1..head (fewer than
  `WriterSegmentedBound`, beyond `MaxWriterTail`) and checks receipt S, served from the last bound
  segment, by its chain digest, sequence and head generation against the checkpoint. Any stamp,
  digest, manifest, request-file, tail-receipt or checkpoint change MUST decline the resume, and the
  command MUST keep the complete route and its decision; the resumed write MUST produce the same
  report and the same store bytes as the complete route on the same pre-state. A failed close of
  the base's read lifetime MUST terminate the command as cleanup (`UNSUPPORTED_FILESYSTEM`,
  PSR-V0-057), never decline.
- `LGL-V0-010`: (proposed; V1-1702; decision 0549 R11/F3) Only `POOL_ACQUIRE` and `POOL_RELEASE`
  MUST try the sealed base, once, in the command's first round, and only when the checkpoint's
  `FullSeq` is S; widen, reap and every other verb keep their route. Acquire is included because a
  complete-route acquire retains a checkpoint whose `FullSeq` is its head, so the release after it
  would find no sealed base. A resumed pool write MUST keep `FullSeq` at S.
- `LGL-V0-011`: (proposed; V1-1702) The pool release target is a wall time of at most 2 s per
  command at 47,000 receipts, measured by the opt-in `TestLGLV0011_PoolReleaseLatency` on a real
  disk (V1-2563) with the in-process audit cache cleared before each call, reporting p50, p95, p99,
  n, audits, resumes and host load.

## Non-goals

no writer-route admission for the pool verbs (CAL-V0-264 stays declined); no sealed-base resume
for widen, reap or mutations; no store-format change and no resident process (LGL-V0-009 reads only
the existing segments, manifest and writer checkpoint);

no sync of a published artifact or its destination directory is coalesced or removed; only
heartbeat and renew skip staging-directory syncs (LGL-V0-005, under the owner decision of
2026-10-24 that amends decision 0549 F2 for those two verbs), and no verb commits a receipt without
its own fsync (LGL-V0-008); decision 0411's per-check mount-ID
observation is kept; no round count, lock wait or gate order changes; no stored state, wire format
or receipt changes.

## Failure modes

with LGL-V0-001 a round that finds staging under the route's lock pays one more
lock acquisition (the cleanup lock) than before, only when a stage is present; a write killed
between the route's decline and the cleanup leaves the orphan for the next writer's cleanup, as
before. With LGL-V0-002 a retained fdinfo file whose read fails, or that reports a different
mount, fails the observation exactly as a fresh open would; a descriptor number reused after a
close cannot alias a retained file, because the retained file is keyed by the session's own
pinned `*os.File` and closed with it. LGL-V0-004 is safe only while `wire.Parse` keeps its
re-encode comparison and trailing-LF check; a change that relaxes either must restore the
`canonical` call in `loadTickets`, and `TestLGLV0004_NoncanonicalTicketBytesStillRefused` then
fails first. The ASCII scan accepts nothing the rune check refuses, because every byte below 0x80
is a whole rune and the scan refuses every C0 control the rune check refuses. With LGL-V0-005 a
crash after a heartbeat or renew can leave staging residue on a filesystem whose metadata journal
does not already order the staging entry with the destination directory sync. A reader that meets
that residue waits `StageSlotPatience` (2 s, CTS-V0-008) before the audit sets it aside, or refuses
`MALFORMED` while the writer lock is held, until the next writer clears it; this is the cost a killed
writer's orphan already has. The skip is safe only while each artifact's own file sync and its
destination directory sync run before the session reports its commit; a change that drops either
for a liveness session reaches LGL-V0-008, and `TestLGLV0005_SkippedStagingSyncKeepsEveryOtherSync`
fails first. With LGL-V0-009 a sealed segment or manifest rewritten in place with its stamps
restored is caught by the precheck or held re-hash and declines; a manifest rewritten before the
command fails the head guards first, which retry as `SNAPSHOT_MOVED` rather than refusing as
corruption (pre-existing behaviour). A store whose last write took the complete route has
`FullSeq` past S, so LGL-V0-010 finds no sealed base and the pool verb pays the complete audit, as
before; the first resumed acquire restores the fast path only after a write rebased the checkpoint
to S. A base blob that a walked tail receipt removed is not re-hashed under the lock; the walk
itself checks that it is absent.

## Acceptance evidence

`TestLGLV0001_ReadyRouteTakesNoCleanupLockOutsideGate`
(`internal/tasks/store/lease_orphan_route_test.go`) holds the writer lock 600 ms over an orphan
slot: the heartbeat spends under half that hold in its guards and at least half waiting inside the
route, declines `staging`, clears the orphan, commits once, replays the same request and serves
the next heartbeat from the route; without the change it fails on the guards time.
`TestLGLV0002_RetainedFDInfoObservesCurrentState`
(`internal/tasks/authority/fixture_session_linux_test.go`) seeks a pinned file to three offsets and
sees each retained re-read report the current `pos` and equal a fresh observation, with one
retained file per descriptor and none after the handles close. `TestLGLV0004_ASCIIFastPathAgreesWithHostileRune` (`internal/tasks/wire/plain_ascii_test.go`)
checks every ASCII byte in both modes, alone and inside a string, against `hostileRune`, keeps
non-ASCII strings on the rune check, and keeps a parsed hostile string's position and code point
in its refusal; `TestLGLV0004_NoncanonicalTicketBytesStillRefused`
(`internal/tasks/transaction/canonical_ticket_test.go`) refuses a canonical ticket with a missing
or extra trailing LF, insignificant whitespace or an escape form as malformed and accepts the
canonical bytes. LGL-V0-003 and the LGL-V0-004 hold reduction are evidenced by the
before/after load and hold-profile reports in the build-log entry `docs/build-log/2026-10-24-tasks-lease-latency.md`.
`TestLGLV0005_SkippedStagingSyncKeepsEveryOtherSync`
(`internal/tasks/authority/staging_sync_skip_test.go`) runs one stage, link, slot removal, stage and
replace cycle with and without the skip: the full session syncs staging four times and the skipping
one never, with the same file and destination directory syncs, the same boundaries in the same order
and the same published bytes, and `SyncCalls` equals the calls observed (LGL-V0-006).
`TestLGLV0006_OnlyLivenessWritesSkipStagingSync`
(`internal/tasks/store/lease_staging_sync_test.go`) counts 8 or 10 sync calls for a heartbeat and a
renew and at least 16, four per artifact, for a claim and a release on both lease routes, with the
audit passing; without the store wiring it fails with an 18-call heartbeat.
`TestLGLV0005_LivenessRequestRedoesPendingWriteWithFullSyncs` (same file) leaves a claim's head
advance pending and redoes it once through a replayed heartbeat and once through a replayed claim:
both make the same sync calls and the audit passes; with the skip set before the redo it fails, 2
calls against 4. `TestLGLV0007_LivenessStagingResidueIsAnOrphan` (same file) is the crash injection:
after a heartbeat and after a renew, on both lease routes, it restores every slot name a crash can
undo, as second links to the last receipt, its request entry, the attempt and `head.json`, and shows that the audit verifies, the lease reads as acknowledged, the next heartbeat
clears the residue and commits on the same attempt and generation, and a second holder's claim is
not granted. `TestLGLV0008_LostLivenessTailIsRefusedNotAdopted` (same file) deletes or tears the
heartbeat's receipt, or restores the pre-heartbeat head, and pins the `JOURNAL_FORKED`, `MALFORMED`
and `REDO_PENDING` refusals that keep every artifact's own sync in force. The before/after
`TestCALV0118_WriterHoldProfile` reports (linux/WSL2 ext4, under load) are in the build-log entry
`docs/build-log/2026-10-24-tasks-heartbeat-staging-sync.md`; the 50 ms heartbeat median is NOT MET.
`TestLGLV0009_PoolVerbsResumeFromSealedBase` (`internal/tasks/store/lease_sealed_base_test.go`)
runs an acquire and a release on a sealed base, each resumed with no complete audit and `FullSeq`
kept at S, and compares the release with the complete route on a copy of the same pre-state
without the checkpoint: the reports are equal and every store file is byte-equal except the writer
checkpoint; with the gate forced off it fails (resumed 0, audits 1).
`TestLGLV0009_SealedBaseDeclinesRewrittenSegment` and `TestLGLV0009_SealedBaseDeclinesChangedState`
(same file) rewrite a segment or the manifest before the command, after the precheck or under the
writer lock, add a stray request file, rewrite a tail receipt or invalidate the checkpoint: each
declines, never commits over the change, and reports the complete route's decision.
`TestLGLV0010_SealedBaseGate` (same file) keeps a widen and an acquire whose checkpoint's `FullSeq`
is past S on the complete route. `TestLGLV0009_SealedBaseCloseFailureIsCleanup`
(`internal/tasks/journal/sealed_base_close_test.go`) fails the base read lifetime's close over a
declining manifest and gets `UNSUPPORTED_FILESYSTEM` cleanup, not a decline; against the pre-review
code it fails with a decline. LGL-V0-011 is evidenced by the before/after reports in the
build-log entry `docs/build-log/2026-10-24-pool-release-sealed-resume.md` (linux/WSL2 ext4,
47,000 receipts: release p50 20.3 s, n=3, before; p50 0.68 s, p99 1.03 s, n=20, after).

## Rollback

revert the source; the guards again take the cleanup lock whenever staging is present
and each observation opens its fdinfo file afresh; `loadTickets` again re-parses each decoded
ticket and every hostile check decodes runes; heartbeat and renew again sync the staging directory
twice per published artifact. No stored state, wire format or receipt changes, so a store written with
LGL-V0-005 reads unchanged under the reverted binary, and any staging residue is the orphan both
binaries already clear. Reverting LGL-V0-009 and LGL-V0-010 returns the pool verbs to the complete
audit; the writer checkpoint, segments and receipts they read and write are unchanged in format.
