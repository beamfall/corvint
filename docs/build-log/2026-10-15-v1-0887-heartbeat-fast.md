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
  canonical ticket set to its `intent/tickets/` projections by digest and length
  (`bindTicketProjections`) instead of decoding every record. Refusals (`MALFORMED` for a divergent,
  missing, extra or repeated projection) and every ticket-reading verb are unchanged. Test:
  `TestCALV0233_LivenessLeaseBindsTicketsWithoutDecoding` (`internal/tasks/transaction`).
  Mutation: `LivenessLease` returning false (liveness verbs back on the decoding route) fails it.
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
directory was quiescent (modification time at least `receiptsQuiescence` = 1 s before the listing
began). The second capture, which only confirms that nothing moved during the walk, stats the
directory from its own no-follow handle (`nativeRead.DirInfo`) and reuses the first listing when the
directory is the same file with the same mode, size, modification time and change time
(`sameDirStamp`; `changeTime` is `Ctimespec` on Darwin and `Ctim` on Linux, and reports no change
time elsewhere, so other platforms always list). The reused listing is copied into the second
observation with its staging flag, so the inventory digest and the settled check are those of the
first listing. The listing lives in the attempt's `writerBefore` value and never outlives it.

Why the stamps suffice: the writer lock excludes every cooperating writer for the whole audit, so
only a foreign process can change `receipts/` during the walk. Its change lands in a modification
time at or after the listing start, which is strictly later than a quiescent stamp on any filesystem
whose granularity is at most 1 s (APFS nanoseconds, Linux jiffies, HFS+ 1 s; FAT cannot hold the
journal, which needs hard links), and in a new change time, which `utimes` cannot restore. APFS also
changes the directory size with its entry count. The one case the stamps do not separate is a clock
stepped back by more than one second during one sub-second attempt together with a foreign change
in that window; the complete route still refuses such a journal. The inventory digest already bound
the directory's `mode\0size\0mtime` before this change, so a reuse that compared nothing but
identity would still have been caught by `sameObservation`; the comparison is kept explicit so the
reuse never depends on that digest.

Tests (`internal/tasks/journal`): `TestCALV0224_WriterAuditWorkIsHistoryIndependent` (two listings
for a fresh directory, one for a quiescent one, k + 1 reads, at history 4 and 48; a temporary name
under a quiescent stamp declines), `TestCALV0224_ForeignReceiptDuringWalkMovesQuiescentListing`
(a settled-looking receipt written after a quiescent first listing: the attempt moves, the retry
lists the fresh directory twice and declines `receipt names differ from head`; 4 listings in 2
attempts), `TestCALV0224_ListingNeverOutlivesItsAttempt` (two settled audits list exactly once
each; an orphaned temporary name under restored stamps declines `not plainly settled` with one
listing). The listing harness also pins that every attempt's first capture lists exactly once.

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

## Verification

Focused tests from the affected plan: `internal/tasks/journal` (whole package), and the lease,
writer-checkpoint, liveness and fixture-session selections of `internal/tasks/transaction`,
`internal/tasks/authority`, `internal/tasks/store` and `internal/tasks/cli`, BASE and lane with
counts; `go vet` and cross-OS vet (linux, windows) for the touched packages; `gofmt -l`; the local
doc gates (spec-requirements, requirement definitions, traceability, line citations, unbounded
readers, use-case receipts, test commands). `make gate` and the full `go test ./...` were not run
(owner's standing preference for scoped issue work). Dogfood: CEM bound, `dogfood-check` and
`dogfood-seal` against the base; one Codex review round. The synthetic stores were deleted after
the final series.
