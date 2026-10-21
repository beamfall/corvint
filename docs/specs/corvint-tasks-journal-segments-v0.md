# Corvint Tasks journal segments V0

Owner: Russell Lewis
Date: 2026-10-20
Intent status: JSG-V0-001..014 accepted by the orchestrator under decision 0548's delegation and the owner's standing auto-accept (2026-10-11) (decision 0549; V1-0887, V1-1512)
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
- Status: JSG-V0-001..014 accepted by the orchestrator under decision 0548's delegation and the owner's standing auto-accept (2026-10-11) (decision 0549; V1-0887, V1-1512) / experimental; slices S1 to S7 land in order on the tasks-perf lane.
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
- `JSG-V0-008`: (accepted; decision 0549 R8, amended 2026-10-21 in slice S3) A writer whose commit on a segmented store leaves 1,024 receipts after S seals S+1..S+512 into the next segment under its lock, once its commit is durable. The head receipt is never sealed, so at least 512 receipts stay active. The seal binds every sealed byte by digest before it publishes: each receipt's `prev` chains from the last sealed receipt, the last receipt's digest equals the one its successor's `prev` binds, and each request afterimage equals the digest its posting receipt records. The seal is a staged transaction with a marker `segments/SEAL_INCOMPLETE` naming the range, the segment digest and the prior and next `head.json` digests: it writes and syncs the segment and the next manifest, publishes `head.json` (the commit point), removes the sealed receipt and request files and the prior manifest, then removes the marker. Any writer that finds the marker under the lock first redoes the removals (when `head.json` is the next head and every remaining file matches the segment) or rolls back (when `head.json` is the prior head), and otherwise refuses JOURNAL_FORKED. A seal that cannot complete leaves the store in exactly one bound form and declines no committed write. The marker is written and synced before any segment file; a reader that finds it reports REDO_PENDING. A seal that fails is settled by the same writer at once, and the write it followed still succeeds. A write never converts an unsegmented store: that is `journal seal` (JSG-V0-013) or, after the format gate, INIT. Import does not seal; a store with a retired prefix is not sealed (slice S4 adds sealing from the anchor).
- `JSG-V0-009`: (accepted; decision 0549 R9) A seal validates its 512 receipts with the complete per-receipt validators, starting from the validator end state the sealed base carries for the previous segment, and carries the new end state into the next sealed base. When that validation succeeds over a prefix still bound by digest, the checkpoint's complete coverage `FullSeq` advances to S.
- `JSG-V0-010`: (accepted; decision 0549 R10) `WriterFullBound` is removed. While seals succeed, head − `FullSeq` is at most 1,023, so no writer falls to the complete route because no refresh succeeded. A seal that fails on a fault makes the next writer take the complete route, which refuses the fault, instead of letting the active set grow without bound. The CAL-V0-117/CAL-V0-265 per-caller refresh becomes the seal.
- `JSG-V0-011`: (accepted; decision 0549 R11, fork F3) When a sealed base exists, widen, reap, the pool verbs and `Mutate` resume across processes from it (PSR-V0-052 no longer needs a same-process base). The resumed write rechecks the sealed prefix by re-hashing every segment against the digest the sealed base and the bound manifest both record, so a rewritten sealed byte declines the route to the complete audit, which refuses it; rewritten-prefix detection (CAL-V0-116, PSR-V0-053) is kept by bytes, not by stamps. The gated recheck stats only the segment files, the manifest and the at most 512 active receipt and request files.
- `JSG-V0-012`: (accepted; decision 0549 R12) The complete route and `receipt audit` read each bound segment once, check its digest against the manifest, and validate every framed receipt and request with the existing validators in sequence order, exactly as if the files were in `receipts/` and `requests/`. Every tamper the unsegmented audit detects is detected; a sealed byte change also fails the segment digest. `receipt audit` is never shortened. Archive export, archive verify, liveness compaction and the doctor journal counts read through the same segment view.
- `JSG-V0-013`: (accepted; decision 0549 R13) `corvint-tasks journal seal` and `corvint-tasks journal unseal` are operator-invoked, run under the writer lock after a complete settled audit, and refuse with nothing written on any fault. `seal` converts an unsegmented store to the segmented format and seals every whole 512-receipt range after F. `unseal` restores the exact receipt and request files and a `taskman-journal-head/0` head without segment fields; the tree it restores is byte-identical to the pre-seal tree. A binary that predates this format refuses a segmented store through its closed `head.json` decode (a newer profile) and never reads a partial history. Once the format gate passes, INIT creates a segmented store. `receipt retire` refuses on a store with sealed segments (unseal first); after a retire, sealing starts at the anchor.
- `JSG-V0-014`: (accepted; decision 0549 R14) Maintained tests assert counts, not durations, at two history sizes and two ticket counts: no intent or ticket read on a liveness write; at most k capacity measurements for k changed entries; the gated recheck's stat count bounded by segments plus active files; the complete audit's open count bounded by segments plus active receipts plus active requests. Durations are measured only by benchmarks B1 to B3.

## Non-goals

- No background sealer, compactor, daemon or scheduler (invariant 7).
- No change to receipt, request or elision-stub bytes, digests or the hash chain.
- Liveness compaction (CAL-V0-300..304) elides only active receipts; sealed segments keep the bytes
  they sealed. Compacting sealed history means `journal unseal`, `receipt compact`, `journal seal`.
- No fsync coalescing (decision 0549 fork F2): a liveness commit keeps its 4 to 5 full syncs.
- No intent digest in liveness results by stamp cache (decision 0549 fork F1 (b) declined).
- Linux qualification of the targets stays `NOT_RUN` unless a Linux host is available.

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
`TestCALV0263_LeaseGateLoad` at 40,000 receipts and 15 writers, for T2; B3, the single-writer
widen/reap harness at 40,000 receipts, for T3. A run under a 1-minute load above the host's core
count is recorded and does not qualify.

## Compatibility and rollback

The writer checkpoint is `taskman-writer-checkpoint/3` from slice S2 (S1 introduced `/2` with the
per-entry canonical sizes and the intent aggregate): an active delta that binds the derived base
`taskman-writer-base/0` by SHA-256. Both files are derived: a reader refuses any other profile, an
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
| JSG-V0-004 | S2: `TestCALV0115_WriterCheckpointCodecIsClosed` (delta/base codec, absent, foreign and malformed base); `TestCALV0115_WriterCheckpointFallsBackToCompleteAudit` (base-removed, base-corrupt, base-foreign); `TestJSGV0004_RetiredPrefixAnchorIsFrozen`; segment digests in the base: pending (slice S5; decided in S3, build log 2026-10-21-jsg-s3-segments-and-seal) |
| JSG-V0-005 | S3: `TestJSGV0005_SealUnsealRollback` (seal through 512, logical reads of every sealed file, inventory equal but for `head.json`, unseal byte-identical, reseal after unseal); `TestJSGV0006_SegmentCodecRoundTripsAndRefuses` |
| JSG-V0-006 | S3: `TestJSGV0006_SegmentCodecRoundTripsAndRefuses`, `TestJSGV0006_SegmentedHeadProfile` (a `/0`-only reader refuses `/1` with UNSUPPORTED_VERSION), `TestJSGV0006_SegmentTamperAndMovedReads`; format gate rows in `TestCALV0131_EveryLiveFormatRefusesANewerVersion` |
| JSG-V0-007 | S3: `TestJSGV0007_SegmentLayoutRefusals` (sealed receipt still linked, unexpected name, unbound segment, segments beside a `/0` head); the CAL-V0-224 probe tests are removed |
| JSG-V0-008 | S3: `TestJSGV0008_WriterSealsOnlySegmentedStores`, `TestJSGV0008_SealStopsSettle` (stop after each of four stages), `TestJSGV0008_SealSettleRefusesForeignState`, `TestJSGV0008_SealDeclinesUnprovenInput`, `TestJSGV0008_SegmentedStoreRefusesCompactAndRetire` |
| JSG-V0-009 | pending (slice S5) |
| JSG-V0-010 | pending (slice S5) |
| JSG-V0-011 | pending (slice S6) |
| JSG-V0-012 | pending (slice S4) |
| JSG-V0-013 | pending (slice S4) |
| JSG-V0-014 | S1 (no intent read at 70 and 210 tickets, 0 and 30 prior heartbeats): `TestJSGV0001_LivenessReadsNoIntentRecords`; S2 (one capacity measurement count at two history and two inventory sizes): `TestJSGV0003_SummarizedCapacityMeasuresOnlyChangedEntries`; slices S4 to S7 pending |
