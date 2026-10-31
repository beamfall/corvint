# Corvint Tasks journal segments V0

Owner: Russell Lewis
Date: 2026-10-20
Intent status: JSG-V0-001..014 accepted by the orchestrator under decision 0548's delegation and the owner's standing auto-accept (2026-10-11) (decision 0549; V1-0887, V1-1512); JSG-V0-015 accepted under the same standing auto-accept (V1-0850); JSG-V0-020..021 accepted under the same standing auto-accept (V1-2204, V1-2278)
Delivery status: experimental
Authoritative inputs: decision 0549 (`docs/decisions/0549-tasks-lease-write-performance-design-2026-10-20.md`),
which records the measurements, targets T1 to T3 and forks F1 to F3; the owner's 2026-10-20 ruling
that performance issues are 1.0 work; the writer route, checkpoint and resume contracts of
[the agent leases contract](corvint-tasks-agent-leases-v0.md) (CAL-V0-115..117, CAL-V0-189,
CAL-V0-224, CAL-V0-265, CAL-V0-300..304) and of the lease preparation contract (PSR-V0-050..054);
[journal retention](corvint-tasks-journal-retention-v0.md) (JRT-V0); the archive export and verify
procedure (TM-V0-022). This contract cites those requirements and does not restate them. The leases
contract is at its obligation bound, so this domain spec holds the new requirements.

## Agent digest
- Claim: Lease writes stop reading intent and whole history: the journal seals 512-receipt segments, so each write is bounded by the active segment.
- Status: JSG-V0-001..014 accepted by the orchestrator under decision 0548's delegation and the owner's standing auto-accept (2026-10-11) (decision 0549; V1-0887, V1-1512); JSG-V0-015 accepted under the same standing auto-accept (V1-0850); JSG-V0-020..021 accepted under the same standing auto-accept (V1-2204, V1-2278) / experimental; slices S1 to S7 land in order on the tasks-perf lane.
- Exists: the decision 0549 profile and spike; the CAL-V0-224 interim probe that JSG-V0-007 retires.
- Blocked on: nothing; the format gate (Acceptance evidence) must pass before INIT creates a segmented store by default.
- Read next: Requirements; Failure modes; Compatibility and rollback; Traceability.

## Intent and boundary

Decision 0549 measured where lease write time goes: a HEARTBEAT reads and digests the whole intent
tree twice and measures every manifest entry, so its cost grows with tickets; the writer checkpoint
grows by 64 bytes per receipt and is rewritten on every write; the complete audit opens one file per
receipt, so a widen or reap at 40,000 receipts takes more than 20 s; and a resumed lease round stats
every base receipt inside the writer lock. This contract removes each term:

- liveness writes observe only `head.json`, the primary branch and the private files their model
  reads (JSG-V0-001, JSG-V0-002);
- capacity is measured incrementally and the writer checkpoint is split into a sealed base and an
  active delta (JSG-V0-003, JSG-V0-004);
- receipts are sealed into 512-receipt segment files that hold their exact bytes, so listings are
  exact again and bounded (JSG-V0-005..008);
- complete coverage advances at each seal and a resumed write rechecks the sealed prefix by bytes
  (JSG-V0-009..012);
- an operator-invoked migration converts a store both ways, byte for byte (JSG-V0-013).

Targets T1 to T3 are 1.0 targets and are measured only by the benchmarks named in Acceptance
evidence; maintained tests assert counts (JSG-V0-014). No background process, daemon or scheduler
is added (invariant 7): every seal runs in the writer whose commit makes it due.

## Requirements

- `JSG-V0-001`: (accepted; decision 0549 R1) A HEARTBEAT or RENEW write MUST NOT list, read, stat-digest or hash any intent ticket or release on the writer route. It reads only `intent/queue.json` and `intent/policy.json`, by name. Its pre-effect recheck binds `head.json`, the primary branch, those two files, `reservations.json`, `pools.json`, `programs.json` and each attempt its plan rewrites to the bytes the model read; a change to any of them declines the route before effects. The authority session still pins the intent directories it may publish into, an O(1) open that lists and reads nothing. CAL-V0-189 keeps the intent-tree binding for every other verb. The liveness model reads no ticket (CAL-V0-233), so no liveness outcome depends on intent; a model access to an elided intent path declines the route.
- `JSG-V0-002`: (accepted; decision 0549 R2, fork F1) A HEARTBEAT or RENEW result reports `snapshot.intentTreeSha256` as `null`, which the wire schema allows; a lease mutation result's `snapshot` is `null` whole. No liveness result reports an intent digest that the write did not observe. A consumer that needs the intent digest runs an audit.
- `JSG-V0-003`: (accepted; decision 0549 R3) The writer checkpoint carries the measured manifest encoding size and the per-entry sizes the capacity model needs. A write that changes k manifest entries measures only those k. The complete mutation route, the complete lease route and the scheduled refresh compare every carried entry, size, request afterimage and aggregate with their complete audit and invalidate a checkpoint that differs, as any other writer-checkpoint refusal does (CAL-V0-117); only that audit's own checkpoint may then replace it. `receipt audit` stays read-only and never reads the derived checkpoint.
- `JSG-V0-004`: (accepted; decision 0549 R4) The writer checkpoint is two derived files: a sealed base, rewritten only when the active set seals (or, before a store is segmented, when the active tail reaches 512 receipts), holding the ticket inventory, the request index of every receipt it covers and the sealed segment digests; and an active delta, bounded by the active set, holding the requests and counts after the base (at most 512 receipts once segmented). The delta binds the base by SHA-256; a delta whose base is absent or differs is unusable and the write declines to the complete route. A write decodes the inventory only for the entries its model reads, and looks a request up in the base without decoding it.
- `JSG-V0-005`: (accepted; decision 0549 R5) A segmented store keeps receipts F..S in sealed segment files `segments/<first>-<last>.seg`, each holding exactly 512 consecutive receipts, where F is the store's first live receipt (1, or the retired-prefix anchor of JRT-V0) and S−F+1 is a multiple of 512; receipts S+1..head stay in `receipts/`. A segment (profile `taskman-segment/0`) holds, in canonical framing, the exact on-disk bytes of its receipts and of the request afterimages they posted, each with its path. Receipt digests, the hash chain and decision 0538 elision stubs are unchanged by sealing.
- `JSG-V0-006`: (accepted; decision 0549 R6) The segment manifest `segments/manifest-<S>.json` (profile `taskman-segment-manifest/0`) lists each segment's range, SHA-256, byte size, last receipt SHA-256 and request count. `head.json` of a segmented store has profile `taskman-journal-head/1` and binds the manifest's S and SHA-256; an unsegmented store keeps `taskman-journal-head/0`. A segment, manifest or range that `head.json` does not bind is a stray, and the complete audit refuses it. The manifest is versioned by name so that publishing a new one never replaces the bound one before `head.json` moves.
- `JSG-V0-007`: (accepted; decision 0549 R7) Every writer audit lists `receipts/` exactly and requires exactly the names S+1..head, and lists `segments/` exactly and requires exactly the names the bound manifest lists. The CAL-V0-224 liveness probe and its accepted residual are retired for every store; an unsegmented store's writers list `receipts/` exactly as before CAL-V0-224.
- `JSG-V0-008`: (accepted; decision 0549 R8, amended 2026-10-21 in slice S3) A writer whose commit on a segmented store leaves 1,024 receipts after S seals S+1..S+512 into the next segment under its lock, once its commit is durable. The head receipt is never sealed, so at least 512 receipts stay active. The seal binds every sealed byte by digest before it publishes: each receipt's `prev` chains from the last sealed receipt, the last receipt's digest equals the one its successor's `prev` binds, and each request afterimage equals the digest its posting receipt records. The seal is a staged transaction with a marker `segments/SEAL_INCOMPLETE` naming the range, the segment digest and the prior and next `head.json` digests: it writes and syncs the segment and the next manifest, publishes `head.json` (the commit point), removes the sealed receipt and request files and the prior manifest, then removes the marker. Only a writer holding the lock settles a marker: a guard read made before the lock is taken (a lease's unlocked pre-round check) that finds the marker takes the writer lock and settles it there, and never redoes, rolls back or removes anything without the lock, so a live seal another writer holds is left intact (amended 2026-10-28, V1-3007). Any writer that finds the marker under the lock first redoes the removals (when `head.json` is the next head and every remaining file matches the segment) or rolls back (when `head.json` is the prior head), and otherwise refuses JOURNAL_FORKED. A seal that cannot complete leaves the store in exactly one bound form and declines no committed write. The marker is written and synced before any segment file; a reader that finds it reports REDO_PENDING. A seal that fails is settled by the same writer at once, and the write it followed still succeeds. A write never converts an unsegmented store: that is `journal seal` (JSG-V0-013) or, after the format gate, INIT. Import does not seal; a store with a retired prefix is not sealed (sealing from the anchor is deferred, JSG-V0-013).
- `JSG-V0-009`: (accepted; decision 0549 R9, amended 2026-10-21 in slice S5) A seal validates its receipts with the complete per-receipt validators, starting from the validator end state the sealed base carries, and carries the new end state into the next sealed base. The sealed base (`taskman-writer-base/1`) records its checkpoint header, aggregates and the SHA-256 of every sealed segment at its sequence, so a writer seal resumes the validators from it across processes. A writer seal validates when its checkpoint's base sits at `FullSeq`, `FullSeq` lies in first−1..last−1 of the range, the checkpoint reaches the range's last receipt and the base's segment digests lead the manifest being extended; it validates the receipts `FullSeq`+1..last from the bytes it seals, requires the validated chain to reach the range's last receipt digest, and rebases the checkpoint on a new base at the range's last receipt whose entries, requests and notes at or before it equal the checkpoint's. When that succeeds over a prefix still bound by digest, the checkpoint's complete coverage `FullSeq` advances to the range's last receipt, retained once the seal is complete under a fresh invalidation token. Any refusal declines the seal, removes the writer checkpoint and leaves the write it followed committed.
- `JSG-V0-010`: (accepted; decision 0549 R10, amended 2026-10-21 in slice S5) On a segmented store `WriterFullBound` is replaced by a bound of 1,024 receipts past `FullSeq` (two segments), and the scheduled complete refresh is not run: while validated seals succeed, head − `FullSeq` is at most 1,023, so no writer falls to the complete route because no refresh succeeded. A seal that fails on a fault removes the writer checkpoint, so the next writer takes the complete route, which refuses the fault, instead of letting the active set grow without bound. A seal whose checkpoint does not reach back to its range (no base at `FullSeq`, or `FullSeq` past the range) seals without validating and leaves `FullSeq` where it is; the 1,024 bound then sends a later writer through the complete route, whose checkpoint the next seal validates from. The CAL-V0-117/CAL-V0-265 per-caller refresh becomes the seal. An unsegmented store keeps `WriterFullBound` and the scheduled refresh unchanged.
- `JSG-V0-011`: (accepted; decision 0549 R11, fork F3) When a sealed base exists, widen, reap, the pool verbs and `Mutate` resume across processes from it (PSR-V0-052 no longer needs a same-process base). The resumed write rechecks the sealed prefix by re-hashing every segment against the digest the sealed base and the bound manifest both record, so a rewritten sealed byte declines the route to the complete audit, which refuses it (PSR-V0-225 runs that re-hash before the preparation gate and re-validates each segment's stamps under it); rewritten-prefix detection (CAL-V0-116, PSR-V0-053) is kept by bytes, not by stamps. The gated recheck stats only the segment files, the manifest and the at most 512 active receipt and request files.
- `JSG-V0-012`: (accepted; decision 0549 R12) The complete route and `receipt audit` read each bound segment once, check its digest against the manifest, and validate every framed receipt and request with the existing validators in sequence order, exactly as if the files were in `receipts/` and `requests/`. Every tamper the unsegmented audit detects is detected; a sealed byte change also fails the segment digest. `receipt audit` is never shortened. Archive export, archive verify, liveness compaction and the doctor journal counts read through the same segment view.
- `JSG-V0-013`: (accepted; decision 0549 R13, amended 2026-10-21 in slice S4) `corvint-tasks journal seal` and `corvint-tasks journal unseal` are operator-invoked, run under the writer lock after a complete settled audit, and refuse with nothing written on any fault. `seal` converts an unsegmented store to the segmented format and seals 512-receipt ranges while 1,024 receipts remain after S, the writer's JSG-V0-008 trigger, so at least 512 stay active; a store below the trigger is left unsegmented. `unseal` restores the exact receipt and request files and a `taskman-journal-head/0` head without segment fields; the tree it restores is byte-identical to the pre-seal tree. Unseal reverses one segment at a time as a staged transaction with the JSG-V0-008 marker, the prior and next heads swapped, so a stopped unseal settles like a stopped seal. The archive of a segmented store is byte-identical to the archive of the same store unsegmented. A binary that predates this format refuses a segmented store through its closed `head.json` decode (a newer profile) and never reads a partial history. Once the format gate passes, INIT creates a segmented store. `receipt retire` and `receipt unretire` refuse on a store with sealed segments (unseal first). A store with a retired prefix stays unsegmented: `journal seal` and `journal unseal` refuse it as UNSUPPORTED, and sealing from the anchor is deferred.
- `JSG-V0-014`: (accepted; decision 0549 R14) Maintained tests assert counts, not durations, at two history sizes and two ticket counts: no intent or ticket read on a liveness write; at most k capacity measurements for k changed entries; the gated recheck's stat count bounded by segments plus active files; the complete audit's open count bounded by segments plus active receipts plus active requests. Durations are measured only by benchmarks B1 to B3.
- `JSG-V0-015`: (accepted under the owner's standing auto-accept, 2026-10-11; V1-0850) A complete-route lease commit derives, encodes and writes the read checkpoint and a fresh writer-checkpoint base to private temporaries beside their destinations before it takes the writer lock. Under the lock, once the head digest is the audited one, it only renames them into place and writes the bounded writer delta, so no checkpoint byte that grows with the store is encoded or written while the lock is held. A staged base is published only when the commit retains under the invalidation token it was encoded with and the lock holder still reads that token; when the commit's complete audit replaced the token it encodes under the lock as before, and when another writer replaced it nothing is retained. A temporary that cannot be written or renamed skips retention, which is derived state, and never fails the write. A later stager removes temporaries older than ten minutes; a younger one is left to its owner. Both checkpoints keep their formats and their read-time verification (CAL-V0-115, JSG-V0-003, JSG-V0-004).
- `JSG-V0-020`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2204, V1-2278) The B3 single-writer harness, `TestJSGV0020_SingleWriterHarness` (opt-in through `CORVINT_TASKS_B3`), MUST, for each store size it is given (default 10,000 and 40,000 receipts, sealed by the writer's own seal before the first call), run one writer and no peer process through the same number of calls (default 20) of each of `heartbeat`, `renew`, `widen` and `reap`, with the in-process audit cache cleared before every call as a fresh command-line process would. For each size, series and verb it MUST report the call count, the count of calls that did not complete with their codes, the minimum, the nearest-rank p50, p95 and p99 and the maximum of the call's wall time in milliseconds, the complete preparation audits and resumed writes the calls ran, and the median of each `LeaseTiming` phase. It MUST run two series per size: `quiet`, and `moved`, which also commits one dispatcher write after every complete preparation audit, the window of PSR-V0-052 and JSG-V0-011. Every series MUST record the host (operating system, architecture, kernel, CPU count, GOMAXPROCS, Go version) and the 1-minute load before and after the series, and MUST label itself `UNQUALIFIED` when the higher of the two loads is above the lower of 12 and the CPU count, or when a load was not observed, and `QUALIFIED` otherwise. The harness asserts no duration (JSG-V0-014): its report is a measurement against T3, not an acceptance.
- `JSG-V0-021`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2204) The B2 report of `TestCALV0263_LeaseGateLoad` and the pool acquire report of `TestPSRV0055_PoolAcquireLoad` MUST carry the host and the qualification label that JSG-V0-020 defines, derived from the loads recorded before and after the run, so a result from a host above the bound is never read as a qualifying one.

## Non-goals

- No background sealer, compactor, daemon or scheduler (invariant 7).
- No change to receipt, request or elision-stub bytes, digests or the hash chain.
- Liveness compaction (CAL-V0-300..304) elides only active receipts; sealed segments keep the bytes
  they sealed. Compacting sealed history means `journal unseal`, `receipt compact`, `journal seal`.
- No fsync coalescing (decision 0549 fork F2): a liveness commit keeps its 4 to 5 full syncs.
- No intent digest in liveness results by stamp cache (decision 0549 fork F1 (b) declined).
- A macOS qualification of the targets stays `NOT_RUN` until a macOS host below its core count runs the
  B1 to B3 harnesses; the Linux measurements are in
  `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-tasks-linux-qualification-b3-harness.md`.

## Failure modes

- A seal that crashes at any step leaves `SEAL_INCOMPLETE`; the next writer redoes or rolls it back
  under the lock (JSG-V0-008). A marker whose head is neither the prior nor the next head refuses
  JOURNAL_FORKED.
- A tampered sealed segment fails its manifest digest: the complete route and `receipt audit`
  refuse it, and a resumed write declines to the complete route, which refuses it.
- A forged segment with a matching forged manifest and `head.json` is the internally consistent
  forgery that CAL-V0-116 already leaves to the per-receipt validators; JSG-V0-012 runs them on
  every complete audit.
- A checkpoint whose carried capacity differs from the measured one is invalidated and removed;
  the complete audit that found it may retain its own (JSG-V0-003).
- A filesystem without change times keeps the listing route for every verb, as today; those hosts
  are outside the targets.
- A migration on a store with a fault refuses and leaves the store unchanged (JSG-V0-013).
- A lease commit stopped between staging and publishing leaves a private temporary beside the
  checkpoint; the next stager removes it after ten minutes, and readers never open it (JSG-V0-015).

## Acceptance evidence

Format gate, before INIT creates a segmented store by default:
1. Seal then unseal is byte-identical on fixtures at 3,000, 13,500 and 40,000 receipts, with and
   without decision 0538 elisions.
2. `receipt audit` gives the same result and refusal codes on both forms for every tamper class:
   each CAL-V0-116 accepted-guarantee case, a byte flip in a sealed segment, frame reorder,
   truncation, an extra or missing segment, a manifest edit and restored stamps.
3. A kill-point crash matrix over each seal step: every step recovers or declines.
4. A prior binary refuses a segmented store with a typed format error.
5. Benchmarks B1 to B3 meet T1 to T3 (decision 0549 Targets).

Benchmarks: B1, the hold-profile harness over the T1 grid (medians of 5, host load recorded); B2,
`TestCALV0263_LeaseGateLoad` at 40,000 receipts and 15 writers, for T2; B3,
`TestJSGV0020_SingleWriterHarness`, the single-writer heartbeat, renew, widen and reap harness at
10,000 and 40,000 receipts, for T3. A run under a 1-minute load above the host's core
count is recorded and does not qualify.

## Compatibility and rollback

The writer checkpoint is `taskman-writer-checkpoint/3` from slice S2 (S1 introduced `/2` with the
per-entry canonical sizes and the intent aggregate): an active delta that binds the derived base
`taskman-writer-base/0` by SHA-256; slice S5 moves the base to `taskman-writer-base/1`, which adds its
checkpoint header, aggregates and sealed segment digests, and a `/0` base is refused like any other
version, so the next write takes the complete route once. Both files are derived: a reader refuses any other profile, an
absent base or one that differs, and the write declines to the complete route, which retains a new
pair. A retired-prefix anchor keeps the frozen `/1` layout, because it is journal state an
operator retains rather than a derived file.
An unsegmented store keeps `taskman-journal-head/0` and is audited as before, except that its
writers list `receipts/` exactly (JSG-V0-007). A segmented store is refused by older binaries
through the newer head profile. Rollback of a store: `corvint-tasks journal unseal`, which restores
the pre-seal tree byte for byte so the prior release reads it again. Rollback of the feature:
unseal every segmented store, then revert slices S7 to S1; S1, S2, S5 and S6 are internal and
revert by commit.

## Traceability

| Requirement | Evidence |
| --- | --- |
| JSG-V0-001 | S1: `TestJSGV0001_LivenessReadsNoIntentRecords`, `TestJSGV0001_LivenessBindDeclinesMovedInputs` (store); `TestJSGV0001_ElidedIntentMeasureParity` (transaction); `TestCALV0115_WriterCheckpointCodecIsClosed` (journal, profile /2 and intent aggregate); `TestCALV0116_WriterRouteParity` (liveness parity) |
| JSG-V0-002 | S1: `TestJSGV0002_LivenessResultReportsNoIntentDigest` (cli) |
| JSG-V0-003 | S2: `TestJSGV0003_SummarizedCapacityMeasuresOnlyChangedEntries` (transaction, after 4 and 16 pause cycles, summarized at the first and the last receipt); `TestJSGV0003_CompleteAuditVerifiesCarriedValues` (journal, 11 forgeries); `TestJSGV0003_CompleteRouteVerifiesRetainedCheckpoint` (store, mutation route and refresh); `TestCALV0115_ReceiptAuditIgnoresDerivedCheckpoints` |
| JSG-V0-004 | S2: `TestCALV0115_WriterCheckpointCodecIsClosed` (delta/base codec, absent, foreign and malformed base); `TestCALV0115_WriterCheckpointFallsBackToCompleteAudit` (base-removed, base-corrupt, base-foreign); `TestJSGV0004_RetiredPrefixAnchorIsFrozen`; S5: segment digests, header and aggregates in the base (`taskman-writer-base/1`) with the base-meta rows of `TestCALV0115_WriterCheckpointCodecIsClosed` (header length, truncation, file count, intent aggregate, segment count, base exceeding the delta, header at another sequence, worktree and queue differing, version 0) |
| JSG-V0-005 | S3: `TestJSGV0005_SealUnsealRollback` (seal through 512, logical reads of every sealed file, inventory equal but for `head.json`, unseal byte-identical, reseal after unseal); `TestJSGV0006_SegmentCodecRoundTripsAndRefuses` |
| JSG-V0-006 | S3: `TestJSGV0006_SegmentCodecRoundTripsAndRefuses`, `TestJSGV0006_SegmentedHeadProfile` (a `/0`-only reader refuses `/1` with UNSUPPORTED_VERSION), `TestJSGV0006_SegmentTamperAndMovedReads`; format gate rows in `TestCALV0131_EveryLiveFormatRefusesANewerVersion` |
| JSG-V0-007 | S3: `TestJSGV0007_SegmentLayoutRefusals` (sealed receipt still linked, unexpected name, unbound segment, segments beside a `/0` head); the CAL-V0-224 probe tests are removed |
| JSG-V0-008 | S3: `TestJSGV0008_WriterSealsOnlySegmentedStores`, `TestJSGV0008_SealStopsSettle` (stop after each of four stages), `TestJSGV0008_SealSettleRefusesForeignState`, `TestJSGV0008_SealDeclinesUnprovenInput`; V1-3007: `TestV13007_UnlockedLeaseGuardLeavesActiveSealIntact`, `TestV13007_LeaseSettlesAbandonedSealUnderLock` |
| JSG-V0-009 | S5: `TestJSGV0009_SealValidatesAndAdvancesFullSeq` (two validated seals, `FullSeq` advanced to each range's last receipt, rebased checkpoint verified by a complete audit), `TestJSGV0009_SealDeclinesACarriedValueThatDiffers` (forged carried sequence: seal declines, checkpoint removed, write completes, next write takes the complete route), `TestJSGV0009_SealWithoutCheckpointSealsPlainly`; `TestJSGV0003_CompleteRouteVerifiesRetainedCheckpoint` (forgery past the base) |
| JSG-V0-010 | S5: `TestJSGV0010_SegmentedWriterBound` (1,023 past `FullSeq` takes the writer route, 1,024 declines), `TestJSGV0009_SealValidatesAndAdvancesFullSeq` (no refresh scheduled on a segmented store, head − `FullSeq` at most 1,023 after every write); `TestCALV0116_WriterFullBoundDeclines` (unsegmented bound unchanged) |
| JSG-V0-011 | S6a (a command's own base on a segmented store: prefix walk and resume through the bound segments, the resumed audit re-hashes every segment and the manifest, the gated recheck stats them): `TestJSGV0011_SealedStoreWidenAndReapResume` (widen and reap under a dispatcher write after every audit: one audit, one resumed write), `TestJSGV0011_ResumeDeclinesRewrittenSegment` (segment rewritten in place with its modification time restored, after the base audit and at `fast.lease.observe`: the resume declines, nothing commits, the complete route's decision stands); B2 shape run in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-21-journal-1021h-sealed-resume.md` (not qualifying: host load above the core count). S6b (resume across processes from the persisted sealed base without a complete preparation audit) is delivered by LGL-V0-009, LGL-V0-010 and LGL-V0-012 (`TestLGLV0009_PoolVerbsResumeFromSealedBase`, `TestLGLV0012_WidenAndReapResumeFromSealedBase`) |
| JSG-V0-012 | S3: the audit rows of JSG-V0-005 to JSG-V0-007; S4: `TestJSGV0013_JournalSealAndUnseal` (sealed archive equals the unsegmented archive), `TestJSGV0012_CompactionOnASealedStore`, `TestJSGV0012_ArchiveExportRefusesADamagedSegment` |
| JSG-V0-013 | S4: `TestJSGV0013_JournalSealAndUnseal`, `TestJSGV0013_MigrationRefusesAFault`, `TestJSGV0013_UnsealStopsSettle` (stop after each of four stages), `TestJSGV0013_SegmentedStoreRefusesRetireAndUnretire`, `TestJSGV0013_RetiredStoreRefusesJournalMigration`, `TestJSGV0013_JournalSealVerbs` (cli) |
| JSG-V0-014 | S1 (no intent read at 70 and 210 tickets, 0 and 30 prior heartbeats): `TestJSGV0001_LivenessReadsNoIntentRecords`; S2 (one capacity measurement count at two history and two inventory sizes): `TestJSGV0003_SummarizedCapacityMeasuresOnlyChangedEntries`; S7: B3 `TestJSGV0020_SingleWriterHarness`, B2 `TestCALV0263_LeaseGateLoad`, both opt-in and asserting no duration (JSG-V0-020, JSG-V0-021); slices S4 to S6 as above |
| JSG-V0-015 | `TestJSGV0015_LeaseCommitStagesCheckpointsOutsideWriterLock` (store, a single-round claim, V1-2638; 40 and 400 tickets: zero read-checkpoint and base bytes written under the writer lock, locked delta within 32 bytes across sizes, staged bytes grow 8x, published checkpoint resumes to the complete audit's head, intent tree, sequence and agreement, no temporaries left); `TestJSGV0015_StagedWriterCheckpointNeedsItsToken` (replaced token retains nothing, commit-replaced token falls back, matching token publishes, stale temporary swept and fresh one kept); measurements in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-21-storeperf-1021u-lease-commit-staged-checkpoint.md`; `TestJSGV0015_MovedClaimResumesOnTheWriterRoute` (store: a moved first commit resumes on the writer route, which retains the read checkpoint under the lock by CAL-V0-119) |
| JSG-V0-020 | `TestJSGV0020_PercentilesAndQualification` (nearest-rank percentiles and the load bound), `TestJSGV0020_HarnessRunsBothModes` (a sealed 1,100-receipt store, both series: every verb timed the requested number of times with no failure and ordered percentiles, a complete preparation audit and a resumed write per widen and reap in `moved`, no resume in `quiet`, and each series' qualification label is the one its recorded `LoadBefore` and `LoadAfter` give); measurements in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-tasks-linux-qualification-b3-harness.md` |
| JSG-V0-021 | `TestJSGV0021_GateLoadReportCarriesHostAndQualification` (host tuple, a quiet and a loaded label, an unobserved load) |
