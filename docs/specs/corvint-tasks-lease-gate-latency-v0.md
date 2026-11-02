# Corvint Tasks lease-gate latency V0

Owner: Russell Lewis
Date: 2026-10-24
Intent status: proposed
Delivery status: experimental

Authoritative inputs: native tickets V1-2531, V1-2537, V1-2539, V1-2203, V1-2205, V1-1702, V1-2592, V1-2866, V1-3186 and the owner decision of 2026-10-27 on concurrent publish directory syncs. These requirements
amend the write rounds of the [agent lease contract](corvint-tasks-agent-leases-v0.md), whose
Requirements section is at the 256-obligation OCM limit (`maxObligations`), so they live here.
They change no lease semantics, stored state, wire format or receipt.

## Agent digest
- Claim: Lease rounds skip the ready-route cleanup lock, keep fdinfo files open, parse tickets in parallel, skip liveness staging syncs, overlap stage and publish syncs.
- Status: proposed; experimental; LGL-V0-001, 002, 004, 005, 006, 007, 009 and 010 are implemented with focused tests; LGL-V0-011 measured a 47k-receipt pool release at p99 1.03 s (target 2 s); LGL-V0-003 is a measured target that is NOT MET (claim gate-wait p99 1.7 s against 500 ms); LGL-V0-008 keeps every published artifact's sync and records why; LGL-V0-013 stages a lease write's artifacts together (heartbeat median 82.0 to 63.0 ms, indicative); LGL-V0-014 decodes a large ticket inventory concurrently (claim hold p50 228 to 172 ms at 880 tickets, WSL2); LGL-V0-016 overlaps one phase's publish directory syncs (claim hold about 16 ms lower, WSL2); the V1-0887 heartbeat median of 50 ms is still NOT MET (65.5 ms at 13.5k and 80.8 ms at 40k receipts, WSL2, 2026-10-27), and LGL-V0-003 is still NOT MET (gate-wait p99 1.78 s at 31 tickets, 2.32 s at 880 with LGL-V0-016).
- Exists: the writer-route staging decline, retained Linux fdinfo files, the single-parse ticket loader and its concurrent decode, the wire ASCII scan, the heartbeat/renew staging-sync skip and the together staging of lease writes and the overlapped publish syncs, each with a focused test.
- Blocked on: V1-2539 (claim wait: serial FIFO admission of a claim burst, about 13 claim holds at p99; 500 ms needs a design decision on non-serial admission or a claim staging-sync skip (concurrent publish directory syncs are LGL-V0-016); the 2026-10-27 claim hold floor is commit near 80 ms plus observe near 47 ms) and V1-2203/V1-2205 (the rest of the heartbeat floor, commit syncs near 38-54 ms, observe near 20 ms and Qualify near 15 ms before the gate, needs an owner store-format or sync decision: LGL-V0-008).
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
- `LGL-V0-780`: (proposed; V1-2539; orchestrator decision 2026-10-31 choosing the claim staging-sync skip that LGL-V0-003 names, extending to claims the owner's 2026-10-24 V1-2203 allowance for heartbeat and renew; owner confirmation of the decision 0549 F2 extension pending; experimental under the standing lane auto-accept) A claim session (`CLAIM` and `CLAIM_NEXT`) MUST skip every staging-directory sync exactly as LGL-V0-005 skips them for heartbeat and renew. It crosses the same `sync:staging` boundaries and runs the same checks, keeps every file sync, destination-directory sync and mkdir parent sync, and keeps every sync when it first redoes another write's pending post effects. LGL-V0-007's residue argument holds for a claim unchanged. A crash can leave only a staging slot name holding a second link to an artifact the claim linked (the receipt, the request entry, the new attempt) or replaced (the head, the ticket). Staging is never read as state: the writer route declines while any staging entry exists, and the complete route clears orphans under the lock before it observes, so the residue reads as a killed writer's orphan (CAL-V0-019, CTS-V0-026). The acknowledged claim then reads exactly as committed, and no second holder can claim the same ticket. The claim's commit point (the receipt link and its receipts-directory sync) is unchanged, so a claim the caller saw acknowledged is as durable as before. Every other verb keeps every sync. LGL-V0-006's count becomes two sync calls per artifact for a claim, as for heartbeat and renew. Failure mode (V1-2539): fourteen concurrent claims are admitted first-in-first-out through the preparation gate, and each claim's hold paid its staging syncs. Non-goals: non-serial admission, and skipping any file or destination sync.
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
- `LGL-V0-010`: (proposed; V1-1702; decision 0549 R11/F3) Only `POOL_ACQUIRE` and `POOL_RELEASE`,
  and `WIDEN` and `REAP` under LGL-V0-012, MUST try the sealed base, once, in the command's first
  round, and only when the checkpoint's `FullSeq` is S; every other verb keeps its route. Acquire is
  included because a complete-route acquire retains a checkpoint whose `FullSeq` is its head, so the
  release after it would find no sealed base. A resumed pool write MUST keep `FullSeq` at S.
- `LGL-V0-011`: (proposed; V1-1702) The pool release target is a wall time of at most 2 s per
  command at 47,000 receipts, measured by the opt-in `TestLGLV0011_PoolReleaseLatency` on a real
  disk (V1-2563) with the in-process audit cache cleared before each call, reporting p50, p95, p99,
  n, audits, resumes and host load.
- `LGL-V0-012`: (proposed; V1-2592; decision 0549 R11/F3, target T3) `WIDEN` and `REAP`, named or
  bare, MUST try the sealed base under LGL-V0-009 and LGL-V0-010 with the same precheck, held
  re-validation and decline rules, and a resumed widen or reap MUST keep `FullSeq` at S, so it no
  longer rebases the checkpoint past the sealed base the next widen, reap or pool verb resumes
  from. A bare reap commits each expired lease's fence as its own write; each of them resumes. A
  resumed route MUST re-run the base check under the lock before it reports any result, a replay of
  an already committed request included. The
  target is a widen and a reap wall p99 below 2 s under `TestCALV0263_LeaseGateLoad` at 40k
  receipts and 15 writers on the WSL2 quiet host, with no widen, reap or claim gate-hold p99
  regression against the same run before the change; every run reports n, p50 and p99 with its
  platform and host load, and a miss is reported NOT MET.
- `LGL-V0-013`: (proposed; V1-2203, V1-2539; experimental under the standing lane auto-accept) A
  lease write (every verb that commits through `commitLease` or the writer route) whose plan has at
  most 11 artifacts, each naming its own destination, MUST decide every artifact's publication once
  before the first is published and stage every artifact it will publish with one
  `Session.PrepareAll`: each file is created, written and confirmed in plan order with `Prepare`'s
  boundaries and checks, then all of the files are fully synced, concurrently, and only after every
  sync has returned is any stage closed, re-checked and registered. Publication then runs exactly as
  before: evidence, the receipt link (the commit point), posts and the head, in the same order, each
  with its own destination-directory sync, and `LGL-V0-005`'s staging-sync skip still applies to
  heartbeat and renew. No artifact is linked or renamed before its own file sync has returned, so
  what a write commits is as durable at its commit point as before. Any failure in `PrepareAll`
  cleans up every file it created and registers none; a stop before any artifact is published
  leaves no staged slot. A plan that repeats a destination or has more than 11 artifacts, and every
  write that is not a lease write, keeps staging one artifact at a time. `SyncCalls` counts the
  same calls as before (`LGL-V0-006`).
- `LGL-V0-014`: (proposed; V1-2539, V1-2203; experimental under the standing lane auto-accept)
  `loadTickets` MUST first find, in record order, the longest prefix of canonical tickets whose
  cumulative bytes stay within `wire.MaxIntentTreeBytes`, then decode and check that prefix (the
  `ticket.Decode`, projection lookup and unselected-divergence check of `LGL-V0-004`) on at most
  `min(GOMAXPROCS, 8)` goroutines, each record into its own slot, and only after every decode has
  returned report the refusal of the lowest-indexed failing record, or the byte-bound limit when
  every record before the bound decoded; it MUST therefore return the same inventory, or the same
  first refusal, as the serial walk it replaces. An inventory of fewer than 64 records, or a host
  with one usable processor, decodes serially. `TestCALV0263_LeaseGateLoad` takes
  `CORVINT_TASKS_LEASE_GATE_LOAD_TICKETS` so the decode cost of a live-sized queue is measurable.
- `LGL-V0-015`: (proposed; V1-2866; experimental under the standing lane auto-accept)
  `TestCALV0263_LeaseGateLoad` MUST default to a queue of 880 tickets, near the live store's
  ticket count, and never build fewer than writers+16; `CORVINT_TASKS_LEASE_GATE_LOAD_TICKETS`
  overrides it (31 reproduces runs before 2026-10-27). The report and the summary log line name
  the ticket count, and every LGL-V0-003 run states it beside platform and host load.
- `LGL-V0-016`: (proposed; V1-2203, V1-2539; owner decision 2026-10-27 "Try concurrent publish
  directory syncs for the lease commit floor (V1-2203/LGL-V0-003); keep store format unchanged";
  experimental under the standing lane auto-accept) A lease write (`applyLease`) MUST issue the
  destination- and staging-directory syncs its posts make after publishing together, once the
  last post is published and before the barrier deletion and the head rename, and the head's
  together after the head is published and before the write returns: inside each phase every
  link or replace still crosses its sync boundary and re-checks its destination in plan order, and
  only the `fsync` call is deferred (`Session.DeferPublishSyncs`/`FlushPublishSyncs`). Every
  ordering edge recovery relies on is kept: evidence, then the receipt link and its receipts
  directory sync, each return before the next artifact is published; no sync is in flight when
  any artifact is published; a failed sync fails the write before any later step. Directory
  creation, staging, barrier deletion, slot removal and every non-lease write sync at once, as
  before; a phase left on an error still issues its deferred syncs. `SyncCalls` counts the same
  calls (`LGL-V0-006`), and no stored state, wire format or receipt changes.

- `LGL-V0-017`: (proposed; V1-2877; experimental under the standing lane auto-accept)
  `TestCALV0263_LeaseGateLoad` and the worker processes it starts MUST read
  `CORVINT_TASKS_LEASE_GATE_LOAD_SYNC`: `serial` runs a lease write's publish directory syncs one
  by one (before `LGL-V0-016`), `concurrent` or unset keeps the overlap, anything else fails the
  test. Only this harness reads it, so one test binary pairs serial and concurrent runs for
  `LGL-V0-003` and V1-0887 measurements; no production path reads it, the store format is
  unchanged, and the report names the mode in `publishSyncs`.
- `LGL-V0-890`: (proposed; V1-2203; experimental under the standing lane auto-accept and the owner's
  2026-11-01 allowance to change the heartbeat durability path) A writer MUST qualify the Git common
  dir (the §5.1 probe: allowlisted type, a fully synced probe file, a working flock and a directory
  sync) before its first write in a process, and MAY reuse that successful qualification for every
  later write of the same process while the directory is the same file (`os.SameFile` on a fresh
  `Lstat` before each write). A failed qualification MUST NOT be reused, and a directory replaced
  or remounted at the same path MUST be qualified again. Reuse removes only the probe: every
  artifact a write publishes keeps its file sync and its destination directory sync (LGL-V0-008),
  so an acknowledged heartbeat is as durable as before. A one-shot command line still qualifies
  once per process; a supervisor or other long-lived writer pays the probe once.
- `LGL-V0-891`: (proposed; V1-2621; experimental under the standing lane auto-accept and the owner's
  2026-11-01 direction to add a base-at-S capture) On a segmented store whose manifest seals through S,
  a lease or Mutate write that takes the complete route while the bound writer checkpoint has
  `FullSeq` S and its base at S MUST retain its complete audit's checkpoint re-based on that sealed
  base (`WriterCheckpoint.RebaseOnSealedBase`): `FullSeq` S, every entry, request and note posted after
  S in the delta. The re-base MUST refuse, and the write retain the complete audit's own checkpoint
  (`FullSeq` at its head, as before), unless every entry, request and note that audit derived from
  receipt 1 at or before S is exactly the sealed base's and the base's segment digests are the
  manifest's; and it MUST NOT be used when that write's verify (JSG-V0-003) refuted the bound
  checkpoint. The next pool acquire or release then still resumes from the sealed base (LGL-V0-009).
  No store format changes and no process stays resident.
- `LGL-V0-885`: (proposed; V1-3950; experimental under the standing lane auto-accept) `gate run` on
  a policy gate that declares `sharedResource` MUST hold an exclusive, non-blocking lock named by
  that resource's class and key for the whole run, so at most one gate run per resource executes on
  one host at a time; a gate without `sharedResource` takes no lock. The lock file lives under the
  repository's Git common directory (`taskman-gate-resources/<sha256(class, key)>.lock`), never in
  the state directory, so the store inventory is unchanged. A run that finds the resource held MUST
  refuse `LOCK_TIMEOUT` (retryable) before it executes or records anything, leaving the store
  byte-identical, so the same `gate run` can succeed once the holder ends. Gates on other resources
  run concurrently. On a platform without `flock` the run refuses `UNSUPPORTED`. `preflight` still
  refuses a gate with `sharedResource` (`UNSUPPORTED`), because it does not serialize. This amends
  the CAL A13 restriction that `gate run` refuses `sharedResource`. Failure mode (V1-3950): the
  policy accepted `sharedResource` but `gate run` refused every such gate, so a heavy gate could
  neither run nor be serialized. Non-goals: cross-host serialization, waiting or queueing for the
  resource, and declared `inputs`.
- `LGL-V0-900`: (proposed; V1-2623) When a lease command's preparation failed `SNAPSHOT_MOVED`,
  its commit step MUST, under the writer lock, re-read the head and the segment manifest it binds;
  a manifest that is absent or whose SHA-256 differs from the head's binding MUST refuse at once
  as `JOURNAL_FORKED` (non-retryable), not spend every preparation round and refuse
  `SNAPSHOT_MOVED`, because under the lock an interrupted seal is settled and none is in flight,
  so the store is corrupt rather than moving. A head that moved still retries and reports
  `SNAPSHOT_MOVED` as before (CAL-V0-372). When that failure names a bound segment, the same
  locked step MUST re-read that one segment and refuse `JOURNAL_FORKED` when it is absent or its
  size or SHA-256 differs from the manifest entry; no other segment is re-read, so the check is
  bounded by one segment.
- `LGL-V0-800`: (proposed; V1-3186) The `receipt audit` receipt binding fold (CAL-V0-135) MUST
  read and decode each `attempts/` post of a receipt at most once, sharing the bytes, the decoded
  attempt and any read or decode error between the external review, escalation, obligation and
  gate reuse steps, as it already shares `intent/tickets/` posts between the review and escalation
  steps. The shared result MUST be exactly what each step computed for itself, errors included,
  and MUST be discarded when the fold moves to another receipt; no result is reused across
  receipts, processes or invocations, so the explicit audit still reads and checks every receipt,
  post and blob, and reports the same outcome, refusals and unknowns. Target: on the same host and
  load, the complete audit's median wall time at 3k, 13k and 40k synthetic receipts and on a copy
  of the live queue is at least 20 % below the base binary's in interleaved runs; the steps only
  read the shared records.
- `LGL-V0-801`: (proposed; V1-0757; experimental under the standing lane auto-accept) The
  CAL-V0-263 load harness MUST accept `CORVINT_TASKS_LEASE_GATE_LOAD_MUTATORS=N` (default 0, which
  keeps every earlier run unchanged) and then start N more worker processes that take no lease and
  cycle through creating a ticket, refining it, completing it by hand (`COMPLETE_MANUAL`) and
  setting a note on a ticket of their own that no lease writer claims, each through `Mutate`, so
  lease and non-lease writers contend for the writer lock on one store. Each such write MUST be
  recorded as a sample with verb `create`, `refine`, `complete-manual` or `note`, its whole-call
  wall time, writer-lock hold and code, and summarized per verb beside the lease verbs. After the
  run the harness MUST run the complete audit and record it in the report's `audit` field, failing
  unless it is `CONSISTENT AGREES`; the report names the mutator count. The harness measures and
  checks; it sets no latency target and no production path reads the knob.
- `LGL-V0-810`: (proposed; V1-3941; experimental under the standing lane auto-accept) While
  `gate run` executes a gate program (not a reused result), it MUST heartbeat the attempt it runs
  for: an ordinary `HEARTBEAT` lease write (CAL-V0-120) by the gate run's actor at the attempt's
  generation, under its own derived request id, every third of the policy `heartbeatTTLSeconds`
  and at most every 240 s, so a gate that runs longer than the TTL leaves its holder
  `FRESH_HOLDER` rather than `STALE_HOLDER`. The first beat that does not complete (fenced,
  released, supervised, or failed) ends the beating without failing the gate run. The beating MUST
  stop, and any beat in flight finish, before the gate result is recorded. Each beat holds the
  writer lock only for its own write; the gate program still runs outside the lock (CAL-V0-016).

## Non-goals

no writer-route admission for the pool verbs (CAL-V0-264 stays declined); no sealed-base resume
for mutations; no store-format change and no resident process (LGL-V0-009 reads only
the existing segments, manifest and writer checkpoint);

no sync of a published artifact or its destination directory is coalesced or removed; only
heartbeat and renew skip staging-directory syncs (LGL-V0-005, under the owner decision of
2026-10-24 that amends decision 0549 F2 for those two verbs), and no verb commits a receipt without
its own fsync (LGL-V0-008); decision 0411's per-check mount-ID
observation is kept; no round count, lock wait or gate order changes; no stored state, wire format
or receipt changes. LGL-V0-013 overlaps the file syncs of artifacts already staged; it merges or
drops no sync, publishes in the same order, and does not change admission: claim-vs-claim gate
wait (V1-2539) remains serial FIFO through one gate, and group commit of several requests' receipts
is not attempted. LGL-V0-014 parallelizes the decode in place under the gate; it moves no claim
work out of the gate, so nothing needs re-validation under the lock, and it does not change the
store-format, `fsync` or admission decisions the remaining targets need. LGL-V0-016 overlaps
only directory syncs of one publication phase that no step of that phase relies on; it merges,
drops or reorders across phases no sync, and does not change claim admission.

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
itself checks that it is absent. With LGL-V0-013 a crash or stop after `PrepareAll` and before the
receipt link leaves only unpublished slots, the orphan a killed writer already leaves (CAL-V0-019,
CTS-V0-026); a stop after the receipt link leaves the committed receipt for redo, as before. A
writer killed during the concurrent syncs leaves the same kind of slots. The gain depends on the
filesystem grouping concurrent fsyncs (ext4's journal does); a filesystem that serializes them
pays the sequential cost, never more syncs. A decision taken before publication cannot be changed
by an earlier artifact of the same plan, because the destinations are distinct and the writer lock
is held; a plan with a repeated destination falls back to sequential staging, and every link or
replace still re-checks its destination when it publishes. With LGL-V0-014 a malformed or divergent
record past the first failing one is decoded and discarded rather than skipped, which costs time
only on a refused write; a record beyond the byte bound is never decoded. The decode uses up to
eight more goroutines inside the gate, so a host whose processors are saturated sees no gain and
pays only the goroutine start-up; the serial threshold keeps small queues on the old path.
With LGL-V0-016 a crash inside a post or head phase can leave that phase's published posts or head
whose directory entries are not yet durable; this is the state a crash between one publish and its
own directory sync already leaves, after the receipt is durable, so redo republishes them from the
receipt as before. A failed deferred sync fails the flush, and the write returns the error before
the barrier deletion or the head is published, as a failed immediate sync did. The gain depends on
the filesystem grouping concurrent directory fsyncs; one that serializes them pays the sequential
cost, never more syncs.

With LGL-V0-800 a fold step that mutated a shared attempt or ticket record would change what a
later step of the same receipt sees; every step only reads them, which
`TestLGLV0800_AttemptMemoMatchesFreshDecode` and the unchanged audit outcomes guard. Reuse within
one receipt is the whole boundary: a cached audit result across invocations would need its own
input binding and is not part of this contract.

## Acceptance evidence

`TestLGLV0800_AttemptMemoMatchesFreshDecode` (`internal/tasks/transaction/ticket_posts_test.go`,
V1-3186) shows the memo returns what a fresh read and decode return for a valid attempt, a
non-record, a malformed attempt and an absent blob, reads each blob once however often it is asked,
and starts afresh for another receipt. The transaction package and the store fold, audit and review
tests pass unchanged. Interleaved runs (five per binary per size, Darwin arm64, 12 CPUs, load 9 to
14) gave median wall times base to new of 0.93 to 0.66 s at 3,000 synthetic receipts, 3.20 to
2.43 s at 13,000, 6.43 to 4.69 s on a 15,147-receipt copy of the live queue and 10.06 to 7.43 s at
40,000, with byte-identical audit items; total allocation on the live copy fell from 7.95 to
5.42 GB. The runs and profiles are in the build-log entry
`docs/build-log/2026-10-31-tasks-core-w2.md`.

`TestLGLV0885_GateRunSerializesASharedResource` (`internal/tasks/store/gate_resource_unix_test.go`,
V1-3950) holds one `sharedResource` gate running: a second gate on the same resource refuses
`LOCK_TIMEOUT` (retryable) without running its command and leaves the store digest unchanged, a
gate on another resource passes meanwhile, and after the holder ends the refused gate passes; all
three record `PASSED`, and `preflight` refuses the gate `UNSUPPORTED`.

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
before/after load and hold-profile reports in the build-log entry `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-tasks-lease-latency.md`.
`TestLGLV0005_SkippedStagingSyncKeepsEveryOtherSync`
(`internal/tasks/authority/staging_sync_skip_test.go`) runs one stage, link, slot removal, stage and
replace cycle with and without the skip: the full session syncs staging four times and the skipping
one never, with the same file and destination directory syncs, the same boundaries in the same order
and the same published bytes, and `SyncCalls` equals the calls observed (LGL-V0-006).
`TestLGLV0006_OnlyLivenessWritesSkipStagingSync`
(`internal/tasks/store/lease_staging_sync_test.go`) counts two sync calls per artifact for a
heartbeat, a renew and, since LGL-V0-780, a claim, and at least 16, four per artifact, for a release
on both lease routes, with the audit passing; without the store wiring it fails with an 18-call
heartbeat. `TestLGLV0780_ClaimStagingResidueIsAnOrphan` (same file) is LGL-V0-780's crash
injection: after a claim on both lease routes it restores every slot name a crash can undo, as
second links to the claim's receipt, its posts and `head.json`, and shows that the audit verifies,
the claim reads as acknowledged, a second holder's claim of the same ticket is not granted, and the
next writer clears the residue and commits.
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
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-tasks-heartbeat-staging-sync.md`; the 50 ms heartbeat median is NOT MET.
`TestLGLV0009_PoolVerbsResumeFromSealedBase` (`internal/tasks/store/lease_sealed_base_test.go`)
runs an acquire and a release on a sealed base, each resumed with no complete audit and `FullSeq`
kept at S, and compares the release with the complete route on a copy of the same pre-state
without the checkpoint: the reports are equal and every store file is byte-equal except the writer
checkpoint; with the gate forced off it fails (resumed 0, audits 1).
`TestLGLV0009_SealedBaseDeclinesRewrittenSegment` and `TestLGLV0009_SealedBaseDeclinesChangedState`
(same file) rewrite a segment or the manifest before the command, after the precheck or under the
writer lock, add a stray request file, rewrite a tail receipt or invalidate the checkpoint: each
declines, never commits over the change, and reports the complete route's decision.
`TestLGLV0010_SealedBaseGate` (same file) admits exactly the acquire, release, widen and reap verbs
to the sealed base and keeps an acquire whose checkpoint's `FullSeq` is past S on the complete
route. `TestLGLV0012_WidenAndReapResumeFromSealedBase`
(`internal/tasks/store/lease_sealed_widen_reap_test.go`) runs a widen, a named reap and a bare reap
on a sealed base: each resumes with no complete audit, keeps `FullSeq` at S, and writes the same
store files and report as the complete route on a copy of the same pre-state without the
checkpoint. `TestLGLV0012_ResumedWidenAndReapRevalidateBaseUnderGate` (same file) rewrites a sealed
segment or the manifest after the precheck or while the resumed write observes under the writer
lock, or a tail receipt after the precheck: the widen or reap declines, commits nothing, and reports
the complete route's decision, so the reused base is the one the gate re-validates.
`TestLGLV0012_ResumedReplayRevalidatesBaseUnderGate` (same file) replays a resumed widen and reap
over a segment rewritten while the replay observes: the resumed route re-checks the base before it
reports a replay too, declines, and the command reports the complete route's decision; before the
review fix the replay was reported over the rewritten segment. The bare reap case fences two
expired leases and checks that its survey and both fences resume. `TestLGLV0009_SealedBaseCloseFailureIsCleanup`
(`internal/tasks/journal/sealed_base_close_test.go`) fails the base read lifetime's close over a
declining manifest and gets `UNSUPPORTED_FILESYSTEM` cleanup, not a decline; against the pre-review
code it fails with a decline. LGL-V0-011 is evidenced by the before/after reports in the
build-log entry `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-pool-release-sealed-resume.md` (linux/WSL2 ext4,
47,000 receipts: release p50 20.3 s, n=3, before; p50 0.68 s, p99 1.03 s, n=20, after).
`TestLGLV0013_PrepareAllStagesSyncsAndPublishesLikePrepare`
(`internal/tasks/authority/prepare_all_test.go`) stages two receipts and a head in one call: every
item's create, write and file-sync boundaries come first and in order, all three file syncs return
before the first stage-close, `SyncCalls` grows by the three file and three staging syncs, each
stage carries its own digest and links or replaces with its bytes, staging is empty afterwards, a
single item crosses exactly `Prepare`'s boundaries, and an empty call is refused.
`TestLGLV0013_PrepareAllSyncFailureCleansEveryStage` (same file) fails the first or the last file
sync with EIO: the call reports a created-stage effect, staging is empty and no stage is
registered. `TestLGLV0013_LeaseStagesTogetherPublishesTheSameStore`
(`internal/tasks/store/lease_staged_together_test.go`) runs a claim, heartbeat, renew and release
on both lease routes with the stages together and one at a time: each destination holds its
artifact's planned bytes, staging is empty after every write, the audit passes, and both runs make
the same sync calls and leave the same files per state directory. `TestLGLV0013_LeaseFaultAfterStagingLeavesNoSlot`
(same file) stops a heartbeat before each of its four artifacts: staging is empty, the next
heartbeat completes and the audit passes; without the slot bookkeeping both store tests fail with
staging residue. The indicative before/after hold-profile and gate-load reports (linux/WSL2 ext4,
under load) are in the build-log entry `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-26-tasks-lease-staged-sync.md`.
`TestLGLV0014_ConcurrentTicketLoadKeepsSerialRefusalOrder`
(`internal/tasks/transaction/ticket_load_order_test.go`) loads 192 tickets with the concurrent
decode and with the serial walk it replaced: a valid inventory is record-for-record equal, and a
malformed record before a divergent one, a divergent before a malformed, the byte bound after valid
records, a malformed record before the bound and an early divergent before a late malformed record
each return the serial walk's first refusal; it also passes under `-race`. The before/after
gate-load and hold-profile reports with host labels, and the V1-2205 qualification of the 50 ms
heartbeat median and 250 ms heartbeat/renew p99 targets as a retained miss with its cause, are in
the build-log entry `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-27-tasks-lease-ticket-decode.md`.
`TestLGLV0015_GateLoadDefaultsToLiveSizedQueue`
(`internal/tasks/store/lease_gate_load_default_test.go`) pins the 880-ticket default (at least
800), an explicit size, and the writers+16 minimum. The measurement recipe naming it is in the
same build-log entry.
`TestLGLV0016_OverlappedPublishSyncsKeepEveryOrderingEdge`
(`internal/tasks/store/lease_publish_sync_overlap_test.go`) records, through
`authority.SetDirectorySyncTestHook` and the publish fault hook, every publication and directory
sync start and return of a claim, heartbeat, renew and release on both lease routes: no sync is in
flight at any publication, the receipts sync returns before the first post, each post's directory
sync returns after the last post and before the head, the head's after the head, the claim's syncs
overlap (one at a time without the change), and both runs make the same sync calls with a clean
audit; flushing the post syncs only at the end makes it fail. `TestLGLV0016_FaultAtEachPublishSyncStopsTheNextStep`
(same file) fails each directory sync of a claim (20) and of a heartbeat (7) in turn: the write
fails, nothing is published after the failed sync, staging is empty, and the retried write
completes with a clean audit. The before/after hold-profile (two runs each) and 880-ticket
gate-load (one run each) reports with host loads are in the build-log entry
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-27-tasks-lease-publish-sync-overlap.md`.

`TestLGLV0009_SealedBaseDeclinesRewrittenSegment` (`internal/tasks/store/lease_sealed_base_test.go`)
rewrites the bound manifest in place with its stamps kept, before the command, after the resume
precheck and under the writer lock, and does the same to a bound segment: each acquire refuses
`JOURNAL_FORKED` (LGL-V0-900), the one before the command in its first round; `TestCALV0372_TornGuardReadSpendsOneRound` keeps a moved
head retrying. `TestLGLV0009_TailRemovedBaseBlobKeepsTheResume`
(`internal/tasks/journal/resume_tail_blob_test.go`, V1-2622) shows a base evidence blob the walked
tail removed keeps a sealed-base resume, while one rewritten in place declines it; blobs are
content-addressed, so no write can give a base blob's path other bytes.

For `LGL-V0-801`, `TestCALV0263_LeaseGateLoad` with `CORVINT_TASKS_LEASE_GATE_LOAD_MUTATORS=3`, 6 lease
writers, 7,000 receipts, 100 tickets, 30 s and 1 s think time (Darwin arm64, 12 CPUs, 1-minute
load 7.09, labelled QUALIFIED) made 248 calls, 102 of them non-lease, with no error code and no
`LOCK_TIMEOUT`, and the audit after the run was `CONSISTENT AGREES`; wall p50/p99 was create
133/705 ms, refine 108/220 ms, complete-manual 103/241 ms and note 106/240 ms, beside heartbeat
120/1089 ms and renew 117/780 ms. The run and a mixed-binary probe (a build of
`32aa5a3f` of 2026-10-03 from the pre-1.0 lineage, against a store the current build initialized and wrote, which
refuses `MALFORMED: policy.json: closed object violated` without writing) are in the build-log
entry `docs/build-log/2026-10-31-tasks-core-w2.md`.

For `LGL-V0-810`, `TestLGLV0810_GateRunHeartbeatsItsHolder`
(`internal/tasks/store/gate_heartbeat_test.go`) shortens the beat gap, claims at minute 0 and runs
a gate at minute 11 under the default 600 s TTL: a gate program that sleeps 1 s leaves the holder
`FRESH_HOLDER`, while a gate that ends before the first beat leaves it `STALE_HOLDER`, and both
gate results record `COMPLETED`.

## Rollback

revert the source; the guards again take the cleanup lock whenever staging is present
and each observation opens its fdinfo file afresh; `loadTickets` again re-parses each decoded
ticket and every hostile check decodes runes; heartbeat and renew again sync the staging directory
twice per published artifact. No stored state, wire format or receipt changes, so a store written with
LGL-V0-005 reads unchanged under the reverted binary, and any staging residue is the orphan both
binaries already clear. Reverting LGL-V0-009 and LGL-V0-010 returns the pool verbs to the complete
audit, and reverting LGL-V0-012 (the `sealedResumeVerb` cases for `WIDEN` and `REAP`) returns
widen and reap to it; the writer checkpoint, segments and receipts they read and write are unchanged in format.
Reverting LGL-V0-885 returns `gate run` to refusing a gate that declares `sharedResource`; the
lock files under the Git common directory are inert and can be removed; nothing in the store changes.
Reverting LGL-V0-013 (`applyLease` back to `applyBeforeCommit`, or `leaseStagesTogether` false)
returns lease writes to staging one artifact at a time; nothing it writes differs in format, and
any slot it leaves is the orphan both binaries clear. Reverting LGL-V0-014 (`loadTickets` back to
the serial loop) returns the decode to one goroutine with the same results; nothing is stored.
Reverting LGL-V0-015 returns the harness default to writers+16 tickets; set
`CORVINT_TASKS_LEASE_GATE_LOAD_TICKETS=880` to keep live-sized runs. Reverting LGL-V0-016
(`leasePublishSyncsOverlap` false, or the source) returns lease writes to syncing each publication's
directories before the next is published; nothing it writes differs in format, and the same
binary or an older one reads and redoes the store unchanged. Reverting LGL-V0-800 (the
`TicketPosts.attempt` memo, or passing a nil memo) returns each binding step to reading and
decoding its own attempt posts; nothing is stored. Reverting LGL-V0-801 removes the mutator
workers and the post-run audit; with the knob unset the harness runs as before. Reverting LGL-V0-810
(`gateHeartbeat` in `internal/tasks/store/gate_heartbeat.go`) stops gate run heartbeating; the
beats it wrote are ordinary heartbeat receipts that every binary reads.
