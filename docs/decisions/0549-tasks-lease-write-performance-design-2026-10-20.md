# Decision 0549 — Lease write performance for 1.0: bounded liveness writes and a segmented journal

Date: 2026-10-20. Status: accepted. The orchestrator decided forks F1 to F3 and accepted this
design on 2026-10-20 under decision 0548's delegation for V1-0887 and the owner's ruling that
performance issues are 1.0 work; the owner can revisit. The targets come from that ruling: the
V1-0887 miss and the V1-1512 deferrals are not acceptable for 1.0. This decision replaces the
"sharded layout escalation" that decision 0548 row V1-0887 named. It moves issue 967 proposals 2
(remainder) and 3 into 1.0. Proposal 5 stays deferred as operator ergonomics.

Related tickets, now in `v1-0`:
- V1-0850: `retainCheckpoint` under the writer lock.
- V1-0910: stat-keyed intent and receipt caches.
- V1-0916: refresh in the triggering caller.
- V1-0646: the intent tree is read per mutation.
- V1-1193: per-read root re-resolution.
- V1-0926 and V1-1656: this lane has not confirmed which slice each one maps to.

## Context: where the time goes (measured 2026-10-20)

All measurements: darwin, 12 cores, under the host load shown. Each figure is the median of 5 runs
on the store hold-profile harness. Times are in milliseconds.

| Tickets | Receipts | Load (1 min) | Heartbeat | Renew | Observe | Model | Bind | Commit (fsync) | Read checkpoint |
|---|---|---|---|---|---|---|---|---|---|
| 880 | 13,500 | 30.8→25.2 | 145 | 148 | 50.6 | 30.2 | 24.5 | 24.8 (13.4) | 1.2 |
| 3,000 | 13,500 | 24.1→22.0 | 470 | 496 | 190 | 106 | 122 | 38 (20.5) | 3.0 |
| 9,500 | 13,500 | 32.0→33.8 | 1,042 | 1,062 | 409 | 305 | 273 | 33.6 (20.5) | 8.2 |

**What the heartbeat cost is made of**
- The cost grows with tickets, about 105 µs per ticket. It is flat in receipts, from 3,000 to
  13,500, since the CAL-V0-224 probe.
- pprof at 9,500 tickets attributes it as follows:
  - capture-time intent reads: 32%;
  - `intent.TreeDigest`, run again before effects per CAL-V0-189: 23%;
  - `checkCapacity`/`MeasureManifestEncoding` in the model: 21%.
- At 3,000 tickets, the scratch timers measured:
  - intent readdir: 1.8 ms;
  - readdir plus lstat: 8.5 ms (2.8 µs per ticket);
  - read plus SHA-256: 98 ms;
  - `TreeDigest`: 119 ms.
- A heartbeat issues 4 to 5 `F_FULLFSYNC` calls (about 4.5 ms each) and 10 to 13 directory syncs.
- The writer checkpoint is rewritten on every write. Its `requests` blob grows by 64 bytes per
  receipt.

**Widen and reap, single writer, 40,000 receipts, 31 tickets, load 13.5→28**
- Widen took 25.3 to 31.6 s of wall time:
  - the pre-gate complete audit (`prepareLease`, then `auditPrefixFallback`): 21.1 to 26.7 s;
  - validation: 3.2 to 3.9 s;
  - gate hold: 47 to 94 ms.
- Reap took 21.4 to 24.1 s, almost all of it in the same audit.
- pprof: 55 to 63% of the time is `safeopen.openat`, one open per receipt. Capacity measurement is
  8.6%.
- These verbs always prepare the complete route (CAL-V0-265 failure modes). The PSR-V0-052 resumed
  write applies only after a round moves.

**Widen and reap under 15 writers (the V1-1512 run)**
- Gate hold: 785 to 990 ms, in rounds 2 to 8.
- Validation inside the hold: 8 to 18 ms. Journal write and fsync: about 45 ms.
- From the code: in a resumed round, `RecheckResume` (`internal/tasks/journal/resume.go`) runs
  under the writer lock immediately before the commit. Its `held` step lists `receipts/` and
  stats every base receipt 1..seq and every base request file. That is receipt-proportional work
  inside the gate, and every queued heartbeat waits behind it. The split inside the hold was not
  timed separately, so this attribution is inferred from the code and the round numbers.

## Targets

- **T1.** A HEARTBEAT or RENEW median of at most 50 ms.
  - Grid: every cell of tickets {880, 3,000, 9,500} × receipts {3,000, 13,500, 40,000}.
  - Flatness: the largest cell median is at most 1.5 times the smallest.
  - Conditions: the reference darwin host with a 1-minute load at or below its core count.
  - A run under heavier load is recorded and does not qualify.
- **T2.** At 40,000 receipts with 15 writers (`TestCALV0263_LeaseGateLoad` profile):
  - heartbeat and renew p99 at most 250 ms (now 1,138 and 1,131 ms);
  - widen and reap gate-hold p99 at most 50 ms;
  - no `SNAPSHOT_MOVED` and no `LOCK_TIMEOUT`.
- **T3.** A single-writer widen or reap at 40,000 receipts takes at most 2 s of wall time (now 21
  to 32 s).

## Requirements

R1 to R14 are numbered here. Slice S0 re-homes them, with the same numbering, in a new domain spec,
`docs/specs/corvint-tasks-journal-segments-v0.md` (prefix `JSG-V0`). The leases spec is at its
obligation bound and gets no new `CAL` IDs.

1. **R1 (liveness observation).** A HEARTBEAT or RENEW MUST NOT read or digest the intent tree
   or any ticket file.
   - It binds its pre-effect recheck to `head.json`, the primary branch and the digests of the
     private files its model read: the attempt, the lease and its reservation.
   - CAL-V0-189 keeps the intent-tree binding for every other verb.
   - The model already reads no ticket (CAL-V0-233), so the outcome cannot depend on intent.
2. **R2 (liveness result).** The result's `snapshot.intentTreeSha256` for a liveness write is
   decided by fork F1 (`null`). The result MUST NOT report an intent digest that this write did not observe.
3. **R3 (incremental capacity).** The writer checkpoint MUST carry the measured manifest encoding
   size and the per-entry sizes the model needs.
   - A write that changes k entries measures only those k (O(k)).
   - The complete route and `receipt audit` measure everything, and they refuse a checkpoint whose
     carried size differs from the measured one.
4. **R4 (bounded checkpoint).** The writer checkpoint MUST split into two files:
   - a sealed base, written only when a segment seals, holding the ticket inventory and the sealed
     segment digests;
   - an active delta, bounded by the active segment (at most 512 receipts), holding the requests
     blob and the counts.
   A write decodes the inventory only for the entries its model reads.
5. **R5 (segment).** A store in the segmented format keeps receipts 1..S in sealed segment files
   `segments/<first>-<last>.seg`, with S a multiple of 512, and receipts S+1..head in `receipts/`.
   - A segment holds the exact on-disk bytes of its 512 receipts and of the request afterimages
     they posted, in canonical framing.
   - Receipt digests, the hash chain and decision 0538 elision stubs are unchanged.
6. **R6 (segment manifest).** The segment manifest is `segments/manifest.json`. It lists each
   segment's range, its SHA-256 and a request-name index. `head.json` binds the manifest digest and
   S. A segment, manifest or range that `head.json` does not bind is a stray.
7. **R7 (exact names restored).** Every writer audit MUST:
   - list `receipts/` exactly and require the names S+1..head;
   - list `segments/` exactly and require the manifest's names.
   This retires the CAL-V0-224 liveness probe and its accepted failure mode. The listing is bounded
   by 512 + 80 names at 40,000 receipts.
8. **R8 (seal).** The writer whose commit brings `receipts/` to 512 or more receipts past S seals
   the next segment.
   - Before the gate: it reads and validates the 512 receipts with the complete per-receipt
     validators and stages the segment.
   - Under the gate: it rechecks the stamps of those 512 files, renames the segment in, rewrites the
     manifest and `head.json`, removes the 512 receipt files, and syncs.
   - A seal is a §5.2 transaction with a staged plan. A crash at any step is either redone to the
     sealed state or rolled back to the unsealed state, and never leaves both forms bound.
9. **R9 (incremental complete coverage, proposal 3).** A sealed segment's validation carries the
   validator end state (the generation history summary) into the next segment's validation.
   `FullSeq` advances to S when the seal succeeds over a sealed prefix that is still bound by
   digest.
10. **R10 (no cliff, proposal 2).** `WriterFullBound` is removed. head − `FullSeq` is at most
    1,023 while seals succeed, so no writer falls to the complete route because a refresh never
    succeeded. A seal that cannot run because of a fault makes the writer take the complete route,
    which refuses the fault, instead of letting the active set grow. This uses no background
    process (invariant 7). The CAL-V0-117/265 per-caller refresh becomes the seal.
11. **R11 (resume across processes).** When a sealed base exists, a resumed write rechecks the
    sealed prefix by re-hashing every segment against its manifest digest. This covers widen,
    reap, the pool verbs and `Mutate` (fork F3, accepted).
    - The gated recheck (`held`) stats the segment files and the at most 512 active files.
    - The CAL-V0-116 and PSR-V0-053 rewritten-prefix detection is kept by bytes, not by stamps.
12. **R12 (complete audit over segments).** The complete route and `receipt audit` read each
    segment once and validate every framed receipt and request with the existing validators.
    - Tamper detection is a superset of today's: a sealed byte change also fails the segment
      digest.
    - `receipt audit` is never shortened.
13. **R13 (migration).** The new commands are `corvint-tasks journal seal` and
    `corvint-tasks journal unseal`. Both are operator-invoked and run under the writer lock after
    a complete audit.
    - `seal` converts a store. A fault refuses it, and nothing is written.
    - `unseal` restores the exact receipt and request files and a `head.json` without segment
      fields. The tree it restores is byte-identical to the pre-seal tree.
    - A new store starts segmented.
    - A binary that predates this format refuses a segmented store through its closed `head.json`
      decode. It never reads a partial history.
14. **R14 (counts, not durations).** Maintained tests assert counts at two history sizes and two
    ticket counts: no intent or ticket read on a liveness write, at most k capacity measurements,
    the `held` stat count, and the complete-audit open count. Durations are measured only by the
    benchmarks below.

## Expected savings (estimates from the profile)

At 9,500 tickets and 13,500 receipts:

| Lever | Steps removed | Estimated saving |
|---|---|---|
| R1 | Observe's intent read and the bind `TreeDigest` | about 650 ms (about 300 ms at 3,000 tickets) |
| R3 | Capacity measurement in the model | about 220 ms |
| R4 | Inventory decode and checkpoint rewrite size | about 15 ms |
| R5 to R7 | Receipt listing and request walk | Removes all receipt-proportional terms in the gate |

Expected liveness budget after these levers:

| Step | Estimate |
|---|---|
| Probe/listing (at most 592 names) | 2 ms |
| Checkpoint and private reads | 3 ms |
| Model and bind | 3 ms |
| Commit | 25 ms (20 ms of it `F_FULLFSYNC`) |
| **Total** | **about 33 ms** |

Widen at 40,000 receipts:
- R11 replaces about 40,000 opens with 79 segment reads plus about 80 MB of SHA-256 (about 0.3 s).
- The gate hold falls from about 1 s to the cost of statting about 600 files.
- Without F3, R12 alone gives an estimated 5 to 7 s: openat is removed, but validation and
  capacity remain.

## Forks (decided 2026-10-20 by the orchestrator under decision 0548)

- **F1: the intent digest in liveness results. Decided: (a) `null`.**
  - A HEARTBEAT or RENEW result reports `snapshot.intentTreeSha256` as `null`, which the wire
    schema allows. A consumer that needs the intent digest runs an audit.
  - Declined: (b) a ctime-keyed stat cache (V1-0910). It stays ticket-proportional (about 2.8 µs
    per ticket, about 28 ms at 10,000 tickets under load) and adopts file stamps as evidence,
    which CAL-V0-116 declines.
- **F2: fsync coalescing. Decided: keep the current 4 to 5 `F_FULLFSYNC` calls; no coalescing.**
  - T1 fits by the estimate. Ordering the writes so that 2 full syncs suffice would save about
    10 ms but needs its own crash-matrix proof.
- **F3: widen, reap and pool verbs resume across processes (R11). Decided: accepted.**
  - The owner earlier declined to extend the writer route to every lease verb because that route
    dropped rewritten-prefix detection (PSR-V0-053): a prefix rewritten with restored stamps was
    no longer observed.
  - R11 does not rely on stamps for the sealed prefix. A resumed write re-hashes every sealed
    segment against the manifest digest that `head.json` binds, so any rewritten byte in the
    prefix fails the digest and the write declines to the complete route, which refuses it. The
    PSR-V0-053 detection is therefore preserved by bytes, which removes the reason for the earlier
    decline. The active receipts (at most 512) keep the existing per-file checks.
  - Without F3, T3 is not reachable. T2 is still reachable through the bounded `held` step.
- **T1, T2 and T3 are all 1.0 targets.**

## Failure modes

- A seal that crashes mid-way leaves a staged plan. The next writer redoes or rolls it back. A
  seal in progress declines the fast route, as staging does today.
- A tampered sealed segment fails its manifest digest:
  - the complete route and `receipt audit` refuse it;
  - a resumed write declines to the complete route, which refuses it.
- A forged segment with a matching forged manifest and `head.json` is the internally consistent
  forgery that CAL-V0-116 already leaves to the per-receipt validators. R12 still runs them on
  every complete audit.
- A host whose filesystem has no change time keeps the listing route for every verb. This is as
  today, and those hosts are outside the targets.
- Migration on a store with a fault refuses and leaves the store unchanged.

## Rollback

- Each slice lands behind a code path that the prior binary's behavior can replace:
  - S1, S2, S5 and S6 are internal and revert by commit;
  - S3 and S4 change the format.
- For the format slices, `corvint-tasks journal unseal` restores the pre-seal tree byte for byte,
  so the prior release reads the store again.
- The format gate below must pass before any default store is created segmented.

## Format gate (before S3 and S4 become the default)

1. Seal then unseal is byte-identical on fixtures at 3,000, 13,500 and 40,000 receipts, with and
   without decision 0538 elisions.
2. `receipt audit` gives the same result and refusal codes on both forms for every tamper class:
   - each CAL-V0-116 accepted-guarantee case;
   - a byte flip in a sealed segment;
   - frame reorder, truncation, extra or missing segment;
   - a manifest edit;
   - restored stamps.
3. A kill-point crash matrix over each seal step: every step recovers or declines.
4. A prior binary refuses a segmented store with a typed format error.
5. Benchmarks B1 to B3 meet T1 to T3.

## Tests and benchmarks

- **B1:** the hold-profile harness over the T1 grid. Medians of 5, host load recorded.
- **B2:** `TestCALV0263_LeaseGateLoad` at 40,000 receipts and 15 writers, for T2.
- **B3:** the single-writer widen/reap harness at 40,000 receipts, for T3.
- Maintained count tests, per R14:
  - the liveness route reads 0 intent files;
  - capacity measures at most k entries;
  - `held` stats at most (segments + 512) files;
  - the complete audit opens at most (segments + active receipts + active requests) files.
- Every existing CAL-V0-116, PSR-V0-053/054 and CAL-V0-265 tamper and refusal test runs against
  both formats.
- Linux qualification stays `NOT_RUN` unless a Linux host is available.

## CAL-V0-224 liveness probe until S3 lands

The probe remains the interim liveness route, tightened on 2026-10-20 after the Codex review.

- **Quiescence.** It applies only when `receipts/` was quiescent at the first stat: both its
  modification time and its change time were at least one second old. Otherwise both captures list
  exactly. Any rename after that stat therefore lands in a strictly later change time, even on
  whole-second stamps, and `utimes` cannot restore a change time. This closes the case of a rename
  from H to H+2 in the same timestamp tick.
- **Refresh backstop.** The CAL-V0-265 refresh prefix walk now fails closed when there is a stray,
  a staged name or a stage digest. A refresh under appends therefore cannot advance `FullSeq` over
  unsettled history. The probe's backstop holds: the next listing writer, or the scheduled
  complete audit at most 512 receipts later.
- **Accepted residual.** On a quiescent directory, a missing history name or a stray that the walk
  does not read is not observed by a liveness write until that backstop runs. R7 removes this
  failure mode.

## Slices and estimated size

| Slice | Content | Estimated size |
|---|---|---|
| S0 | `JSG-V0` spec; re-home R1 to R14 | docs only |
| S1 | R1, R2 (F1 = `null`) | M: about 300 Go + 300 test lines |
| S2 | R3, R4 base/delta split without segments | M: about 450 + 350 |
| S3 | R5 to R8: segment format, seal, redo, exact listings, probe retired | L/XL: about 2,000 + 1,500 |
| S4 | R12, R13: complete audit over segments, seal/unseal migration, prior-binary refusal | L: about 800 + 700 |
| S5 | R9, R10: incremental coverage, cliff removed | M: about 500 + 400 |
| S6 | R11 (F3 accepted): cross-process resume for widen, reap, pool and `Mutate` | M: about 500 + 400 |
| S7 | B1 to B3 harnesses, format-gate fixtures and qualification runs | M: about 400 + runs |

- Total: about 4,950 Go lines and about 3,650 test lines.
- T1 needs S1, S2 and S3. T2 needs S3 (and S6 for widen rounds). T3 needs S6.

## Amendment (2026-10-24): F2 for heartbeat and renew

The owner decision of 2026-10-24 for V1-2203 ("heartbeat fsync floor") amends F2 for heartbeat and
renew only: their receipts may commit without their own fsync. Claim, submit, gate, complete,
release and every authority-bearing write keep every sync.

The delivered change uses that allowance only for the staging directory. A heartbeat or renew
session skips its staging-directory sync calls and keeps every file sync and every destination
directory sync (LGL-V0-005 to 007). Skipping a receipt's own sync would leave lost or torn tails
that the store refuses as a fork rather than adopts as an older heartbeat, so it is not implemented
(LGL-V0-008). See `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-24-tasks-heartbeat-staging-sync.md`.
