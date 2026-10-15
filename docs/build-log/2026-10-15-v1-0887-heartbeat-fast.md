# V1-0887 fast heartbeat and renew: ticket binding, shard pins, name scan, listing reuse (issue 642)

Date: 2026-10-15. Tickets: V1-0887 (P1, GitHub issue 642), V1-1408 (writer-checkpoint decode),
V1-1410 (`scanReceiptNames` per-name cost). Base: `d47b0f01e5250aeab69bbf63bf44f9a285aeb3be`
(`origin/main` `6457b86bcd4b9048cfc93645b0250c4372115d72` merged at `81426c6f6a7108a8a87dea0f9bf38193df66ac7a`, and merged again before publication).
Spec: `docs/specs/corvint-tasks-agent-leases-v0.md`. New lane requirements CAL-V0-233 and
CAL-V0-234 are accepted under the owner's standing auto-accept (2026-10-11). CAL-V0-224 is
amended under an owner approval given on 2026-10-15 and relayed by the orchestrator, verbatim:
"yes, approve amending CAL-V0-224 for this". No other accepted requirement text changed
(CAL-V0-116, CAL-V0-117, CAL-V0-048, CAL-V0-230, the receipt-audit and journal-format rows are
untouched); CAL-V0-232's description of "two exact listings" predates the amendment and is reported
for the owner rather than edited.

## Intent

The owner (2026-10-15): "half a second on each time seems nuts. you need to figure out a way to
get this more optimized." V1-0887 asks for a heartbeat or renew that changes nothing else to
complete in milliseconds on a store of at least 13,000 receipts, flat from 3,000 to 13,000, with
lease, generation fencing and refusal semantics unchanged. The lane target was 50 ms wall at
880 tickets and 13,000 receipts. It is not met; the numbers reached are below.

## Method

- Stores: three synthetic stores (880 tickets; 3,000, 6,000 and 13,000 receipts) from the opt-in
  `TestRC3_SyntheticStore` generator, each in a scratch directory, never a live `.git/taskman`;
  one claimed attempt per store (re-claimed at generation 2 after the lease expired mid-lane).
- CLI timing: BASE and lane `corvint-tasks` binaries, one process per call with `--timing`,
  interleaved A/B (base, lane, base, lane...), one warm-up per binary and verb, `/usr/bin/time -p`
  for wall and CPU (user + sys), median of 7, host load (one-minute, 12 CPUs) recorded before and
  after each series. The final series sleeps 1.2 s between calls so `receipts/` is quiescent after
  the previous call's receipt publish (CAL-V0-224 as amended).
- Go benchmarks: `-count=10`, medians computed by a scratch script (benchstat is not installed);
  allocation counts are the robust signal under host load 44 to 57.
- No CPU burners were used; every series ran under the machine's ambient load from other lanes.

## Base profile (from `docs/build-log/2026-10-15-v1-0887-1403-liveness-guard.md`)

About 525 ms CPU per heartbeat at 13,000 receipts (load about 60): transaction model about
165 ms (canonical encoding and ticket decoding), writer-audit observe about 97 ms, authority session
about 70 ms, observation bind about 37 ms; about 31 ms of the 48 ms history growth in the two
`receipts/` listings.

## Phase A and B: CAL-V0-233 and CAL-V0-234 (commit `4c0283ba4`)

- CAL-V0-233: a HEARTBEAT or RENEW reads no ticket, so `transaction.validateInput` binds the
  canonical ticket set to the `intent/tickets/` projections the lease routes name for it
  (`Input.CanonicalTicketPaths`), by digest and length per path (`bindTicketProjections`), instead
  of decoding every record. Refusals follow the decoding route's codes, messages and order
  (`MALFORMED` for a missing, divergent or swapped projection and for an extra projection,
  `DUPLICATE_ID` for a repeated or case-folded identity); every ticket-reading verb, and a liveness
  caller that names no paths, decode as before. The first cut bound by a digest multiset, which
  accepted swapped projections and refused a repeated record `MALFORMED`; the review below found it
  and the per-path binding replaced it. Test: `TestCALV0233_LivenessLeaseBindsTicketsWithoutDecoding`
  (`internal/tasks/transaction`), a differential against the decoding route. Mutations:
  `LivenessLease` returning false (liveness verbs back on the decoding route) fails it; M5, binding
  by digest anywhere in the tree instead of at the named path, fails its swapped case ("unselected
  divergent ticket: liveness <nil>, decoding route MALFORMED").
- CAL-V0-234: the authority fixture session pins a request shard the first time a target under it
  is used (`pinShard`) instead of opening and re-verifying all 256 shards per operation. Test:
  `TestCALV0234_RequestShardsPinnedOnFirstUse` (`internal/tasks/authority`). Mutation: restoring
  the eager 256-shard pin loop in the constructor fails it.

CLI A/B after Phase A and B (median of 7, load 47.5 before and 45.7 after; every outcome OK):

| receipts | verb | base wall ms (min) | lane wall ms (min) | base CPU ms | lane CPU ms |
|---:|---|---:|---:|---:|---:|
| 3,000 | heartbeat | 1840 (1160) | 670 (290) | 570 | 220 |
| 3,000 | renew | 2570 (920) | 540 (380) | 660 | 250 |
| 6,000 | heartbeat | 1110 (900) | 810 (420) | 570 | 250 |
| 6,000 | renew | 3090 (1590) | 1610 (750) | 610 | 340 |
| 13,000 | heartbeat | 2190 (1640) | 1110 (470) | 680 | 290 |
| 13,000 | renew | 1400 (940) | 700 (410) | 630 | 300 |

Wall time under that load is dominated by scheduling; CPU time is the stable signal and falls by
about 55 to 60%.

## V1-1410: receipt names without parse-and-format (commit `c0309156a`)

`receiptNames` now checks each name's fixed shape (12 digits, `.json`, non-zero) and accumulates
sequences and the inventory digest without `strconv`, `ReceiptName` formatting or `fmt.Fprintf`.
Codes, messages and the first-error order in listing order are unchanged.

Benchmarks (`-count=10`, medians; base load 44.8 to 54.7, lane load 53.5 to 49.0):

| benchmark | base ms (allocs, B) | lane ms (allocs, B) |
|---|---:|---:|
| `ScanReceiptNames/3000` | 21.5 (31,237; 856 KB) | 16.4 (3,207; 408 KB) |
| `ScanReceiptNames/13000` | 98.2 (128,701; 4.0 MB) | 64.5 (13,644; 1.97 MB) |
| `ListReceiptNames/3000` (listing only) | 17.0 (3,203) | 10.3 (3,203) |
| `ListReceiptNames/13000` (listing only) | 49.7 (13,638) | 55.5 (13,638) |
| `ReceiptNames/3000` (per-name work only) | - | 0.67 (3; 52 KB) |
| `ReceiptNames/13000` (per-name work only) | - | 2.97 (3; 227 KB) |

Per-name work at 13,000 names goes from about 115,000 allocations and 2.3 MB to 6 allocations and
0.23 MB; the listing itself (`ReadDirNames`, one allocation per name) is now the whole cost of a
scan. Wall medians under load are noisy; allocation counts are exact.

Tests: `TestV11410_ReceiptNameSeqAgreesWithOracle` (the pre-change parse-and-format check kept as
the oracle, over every single-byte edit of canonical names, every length 0..24 and the digit
boundaries) and `TestV11410_ReceiptNamesFirstErrorOrder` (aliased, repeated, temporary and invalid
names judged at their own listing position). Mutations: accepting an all-zero sequence
(`return n, true`) fails the oracle test; forgetting settled names (dropping the `dense[seq]`
record) fails the first-error-order cases `repeated settled` and `repeat before invalid`.

## V1-1408: writer-checkpoint decode (measured, refuted; commit `c0309156a`)

`BenchmarkDecodeWriterCheckpoint` (`internal/tasks/journal`, `-count=10`, load 44.1 to 44.4):

| tickets | request records | size bytes | wall median ms | allocs |
|---:|---:|---:|---:|---:|
| 880 | 3,000 | 301,621 | 5.0 | 30,039 |
| 880 | 6,000 | 493,621 | 13.3 | 30,039 |
| 880 | 13,000 | 941,622 | 21.2 | 30,040 |
| 0 | 13,000 | 832,503 | 1.07 | 84 |
| 880 | 0 | 109,618 | 5.2 | 30,037 |

A scratch rusage measurement (three runs of 200 decodes each, CPU per decode): 880/3,000 about
4.0 to 4.4 ms; 880/6,000 about 4.4 to 4.8 ms; 880/13,000 about 4.7 to 5.3 ms; 0/13,000 about
0.74 to 0.87 ms; 880/0 about 3.7 to 4.3 ms. History adds about 1 ms at 13,000 records (the
sha256 trailer, the order check and the copy over 832 KB); the ticket-proportional embedded JSON
decode (about 4 ms, 30,000 allocations) dominates. Decode size grows with history (about 64 bytes
per retained request) but decode cost is not history-proportional in any way that matters, so no
bound on retained request records was added. The suspicion is refuted; the ticket-proportional
decode is a residual for V1-0915.

## CAL-V0-224 amendment: quiescent listing reuse (commits `765ac2107`, `1d514ac6f`)

Design: the writer audit's first capture lists `receipts/` exactly and records whether the
directory was quiescent (modification time and change time both at least `receiptsQuiescence` =
1 s before the listing began, `quiescentDir`). The second capture, which only confirms that nothing moved during the walk, stats the
directory from its own no-follow handle (`nativeRead.DirInfo`) and reuses the first listing when the
directory is the same file with the same mode, size, modification time and change time
(`sameDirStamp`; `changeTime` is `Ctimespec` on Darwin and `Ctim` on Linux, and reports no change
time elsewhere, so other platforms always list). The reused listing is copied into the second
observation with its staging flag, so the inventory digest and the settled check are those of the
first listing. The listing lives in the attempt's `writerBefore` value and never outlives it.

Why the stamps suffice: the writer lock excludes every cooperating writer for the whole audit, so
only a foreign process can change `receipts/` during the walk. A listing is reused only when the
directory's modification time and change time were both at least 1 s old when the listing began, so
a change after that start lands in a strictly later change time on any filesystem whose stamp
granularity is at most 1 s (APFS nanoseconds, Linux jiffies, HFS+ whole seconds; FAT cannot hold the
journal, which needs hard links), and `utimes` can restore the modification time but not the change
time, which records the restoration itself. Requiring the change time to be quiescent closes the
whole-second case the first cut left open, where a change in the listing's own second under a
restored modification time would have kept both stamps (review finding below). APFS also changes
the directory size with its entry count. The one case the stamps do not separate is a clock stepped
back by more than one second during one sub-second attempt together with a foreign change in that
window; the complete route still refuses such a journal. The inventory digest already bound
the directory's `mode\0size\0mtime` before this change, so a reuse that compared nothing but
identity would still have been caught by `sameObservation`; the comparison is kept explicit so the
reuse never depends on that digest.

Tests (`internal/tasks/journal`): `TestCALV0224_WriterAuditWorkIsHistoryIndependent` (two listings
for a fresh directory and for a modification time restored under a fresh change time, one for a
quiescent one, k + 1 reads, at history 4 and 48; a temporary name that appears during the walk
declines after 4 listings), `TestCALV0224_ForeignReceiptDuringWalkMovesQuiescentListing`
(a settled-looking receipt written after a quiescent first listing: the attempt moves, the retry
lists the fresh directory twice and declines `receipt names differ from head`; 4 listings in 2
attempts), `TestCALV0224_ListingNeverOutlivesItsAttempt` (two settled audits list exactly once
each; an orphaned temporary name under restored stamps declines `not plainly settled` with one
listing). The listing harness also pins that every attempt's first capture lists exactly once. A
quiescent directory is reached by waiting for its change time to age (`awaitQuiescent`), never by
restoring stamps, and a foreign name injected during a walk leaves a future modification time, so
the retried attempt's listing count does not depend on scheduling.

Mutations, each reverted after its run:

- M1, reuse regardless of quiescence (`o.receiptsQuiescent = !listedAt.IsZero()`):
  `TestCALV0224_WriterAuditWorkIsHistoryIndependent` fails ("fresh ... 1 receipts/ listings and 4
  receipt reads, want 2 and 4").
- M2, stamps ignored beyond identity with the first capture's metadata carried:
  `TestCALV0224_ForeignReceiptDuringWalkMovesQuiescentListing` fails (the attempt succeeds with the
  foreign receipt unseen: "receipt added during the walk: <nil>, want the retried attempt's
  decline"). Ignoring only the comparison is caught by the inventory digest with one listing fewer
  ("3 listings in 2 attempts ..., want 4 in 2").
- M3, listing retained across attempts: `TestCALV0224_ListingNeverOutlivesItsAttempt` fails
  ("attempt 1's first capture ended at 0 cumulative listings, want 1").
- M4, quiescence judged from the modification time alone (`quiescentDir` ignoring the change
  time): `TestCALV0224_WriterAuditWorkIsHistoryIndependent` fails ("restored forMutation=false: 1
  receipts/ listings and 4 receipt reads, want 2 and 4").

Journal format, receipt names, refusal codes and replay bytes are unchanged, so the journal stays
readable by the released `corvint-tasks` without migration.

## Final CLI A/B (all commits; median of 7, load 50.9 before and 49.1 after; every outcome OK)

| receipts | verb | base wall ms (min) | lane wall ms (min) | base CPU ms | lane CPU ms | lane snapshot/validation/write/fsync ms |
|---:|---|---:|---:|---:|---:|---|
| 3,000 | heartbeat | 960 (640) | 290 (230) | 530 | 190 | 92/46/36/20 |
| 3,000 | renew | 870 (660) | 410 (260) | 550 | 220 | 117/84/40/21 |
| 6,000 | heartbeat | 1400 (930) | 640 (390) | 580 | 230 | 193/139/119/29 |
| 6,000 | renew | 2530 (950) | 560 (450) | 630 | 240 | 188/148/59/30 |
| 13,000 | heartbeat | 810 (520) | 320 (300) | 560 | 220 | 118/54/43/22 |
| 13,000 | renew | 3050 (1840) | 960 (650) | 700 | 270 | 278/199/69/22 |

CPU per heartbeat falls from about 560 ms to about 220 ms at 13,000 receipts (about 60%), and the
3,000 to 13,000 CPU difference is within the noise band (190 to 230 ms heartbeat, 220 to 270 ms
renew). Wall time under host load about 50 on 12 CPUs is scheduling-bound; the fastest lane
heartbeat observed was 230 ms at 3,000 and 300 ms at 13,000 receipts. The 50 ms target and
"flat within 5 ms" are not met under this load and could not be measured at low load during the
lane.

In-process lane profile at 13,000 receipts (12 heartbeats with the quiescence sleep, load 49 to 60):
about 265 ms CPU per heartbeat, about 75% of it in system calls. Per heartbeat: writer-audit
capture about 82 ms (intent-tree reads about 48 ms in the first capture, the `receipts/` listing
about 22 ms, the other listings about 14 ms); observation bind about 44 ms (`intent.TreeDigest`
reads the intent tree again before effects, per CAL-V0-189); transaction model about 36 ms
(`CheckCapacity`/`measure` about 33 ms); authority session operations about 22 ms; apply before
commit about 22 ms.

## Residuals and owner decisions

- Target not met: the lane leaves a 13,000-receipt heartbeat at about 220 ms CPU (about 300 ms
  wall at the best observed, under load about 50). Reaching milliseconds needs the ticket-proportional
  work reduced (V1-0915): the intent tree is read twice (first capture, then `TreeDigest` before
  effects, both required by CAL-V0-116/189 as accepted), `CheckCapacity` measures the manifest per
  plan, and the authority session checks every retained parent per operation.
- The remaining history-proportional work is one exact listing (about 2 µs per name under load,
  about 27 ms at 13,000) plus about 1 ms of checkpoint decode. Avoiding the listing needs a
  CAL-V0-116 change (next-name probe, stamp evidence that survives an attempt, or a sharded
  `receipts/`), which is an owner decision; none was made here.
- CAL-V0-232's text still says "the two exact listings of `receipts/`"; it is accepted and was not
  edited (one sentence for the owner to update).
- Low-load measurement: none was possible during the lane (ambient load 44 to 60 from other lanes).

## Independent review (Codex 0.162.0, one read-only round)

Run against the sealed lane (`91d334547`). A first invocation hung reading stdin (`codex exec`
needs stdin closed) and produced no review; the rerun completed and is the lane's one round.
Findings and dispositions, fixed in the commit that follows the seal:

- P1, listing reuse under whole-second stamps (CAL-V0-224): quiescence was judged from the
  modification time alone, so on a filesystem with whole-second change times a foreign process
  that changed `receipts/` in the listing's own second and restored the modification time kept
  both stamps, and the second capture could reuse a stale listing. Fixed: a listing is quiescent
  only when the modification time and the change time are both at least `receiptsQuiescence` old
  (`quiescentDir`), so a change after the listing start lands in a strictly later change time at
  any granularity up to one second; the spec text and the safety argument above say so. Test: the
  "restored" case of `TestCALV0224_WriterAuditWorkIsHistoryIndependent`; mutation M4.
- P2, liveness ticket binding by digest multiset (CAL-V0-233): swapped projections (each file
  holding the other's bytes) bound where the decoding route refuses `unselected divergent ticket`,
  and a repeated record was refused `MALFORMED` where the decoding route gives `DUPLICATE_ID`,
  while the spec and this log claimed identical refusals. Fixed: the lease routes name each
  canonical record's path (`Input.CanonicalTicketPaths`), the model binds per path with the
  decoding route's refusals and order, and the test is a differential against that route;
  mutation M5.
- P2, listing-count assertions depended on scheduling: a retry more than one second after an
  injected foreign name found a quiescent directory and reused its listing (3 listings, not 4).
  Fixed: the tests stamp a future modification time after the injected name, and the quiescent
  harness waits for the change time to age instead of restoring the modification time alone.
- CAL-V0-232's "two exact listings" sentence contradicts the amended CAL-V0-224; it is an
  accepted row and stays an owner decision (Residuals above).

## Post-review CLI A/B (fixed binary; median of 7, load 30.5 before and 42.8 after; every outcome OK)

Same method as the final A/B above, run after the review fixes against the BASE binary on the
three synthetic stores (generation 3 after a re-claim; the 600-minute scratch leases had expired).
Wall and CPU in ms; stages are the CLI's own `--timing` medians (snapshot read / validation /
journal write / fsync).

| store | verb | base wall / CPU | base stages | lane wall / CPU | lane stages |
| --- | --- | --- | --- | --- | --- |
| 3k | heartbeat | 450 / 440 | 76 / 199 / 103 / 18 | 190 / 160 | 66 / 33 / 19 / 19 |
| 3k | renew | 500 / 440 | 97 / 232 / 101 / 21 | 210 / 180 | 74 / 33 / 18 / 20 |
| 6k | heartbeat | 450 / 440 | 74 / 186 / 106 / 20 | 190 / 160 | 64 / 33 / 19 / 19 |
| 6k | renew | 510 / 460 | 101 / 203 / 117 / 19 | 230 / 190 | 88 / 38 / 22 / 20 |
| 13k | heartbeat | 470 / 460 | 95 / 201 / 107 / 20 | 210 / 170 | 72 / 32 / 20 / 20 |
| 13k | renew | 510 / 480 | 112 / 203 / 112 / 22 | 260 / 190 | 83 / 34 / 25 / 27 |

Validation (ticket decode) fell from ~200 ms to ~33 ms and is flat; the journal write (second
capture) from ~105 ms to ~20 ms. Lane CPU is 160 to 190 ms and moves at most 10 ms from 3k to 13k
on either verb; wall moves 20 ms (heartbeat) and 50 ms (renew) under a load that rose from 30 to 43
during the series. The snapshot read (first exact listing, intent reads, tree digest) is now the
largest stage at 64 to 88 ms and is the CAL-V0-116 floor named under Residuals. The 50 ms target
stays unmet.

## Verification

Focused tests from the affected plan: `internal/tasks/journal` (whole package), and the lease,
writer-checkpoint, liveness and fixture-session selections of `internal/tasks/transaction`,
`internal/tasks/authority`, `internal/tasks/store` and `internal/tasks/cli`, BASE and lane with
counts; after the review fixes, the whole `internal/tasks/journal` and `internal/tasks/transaction`
packages and the lease, writer, checkpoint, liveness and attempt selection of `internal/tasks/store`,
with mutations M4 and M5; `go vet` and cross-OS vet (linux, windows) for the touched packages; `gofmt -l`; the local
doc gates (spec-requirements, requirement definitions, traceability, line citations, unbounded
readers, use-case receipts, test commands). `make gate` and the full `go test ./...` were not run
(owner's standing preference for scoped issue work). Dogfood: CEM bound, `dogfood-check` and
`dogfood-seal` against the base; one Codex review round. The synthetic stores were deleted after
the final series.
