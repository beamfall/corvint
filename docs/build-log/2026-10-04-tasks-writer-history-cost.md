# Tasks writer cost against receipt history

Native ticket V1-0645, the writer follow-up to S12 (issue 446), was owner-prioritised on
2026-10-04 as the root cause of issues 494 and 545. This entry records:
- the profile of writer cost against receipt history;
- the decision to stop at a proposed design, CAL-V0-070/S20 in
  `docs/specs/corvint-tasks-agent-leases-v0.md`;
- the maintained baseline.

Base is public cd70ba0ab538a612a13d146934cf95ef356c333d. CAL-V0-070/S20 is a provisional ID: the
coordinator confirms it.

## Measurement

The fixture is synthetic and built in temporary directories only. No live store was read or
written. Its parts:
- 50 tickets created through `store.Mutate`;
- every later receipt appended through the fixture codec, posting one request afterimage and one
  inline ticket afterimage padded to about 6 KB per receipt;
- no evidence, attempts or reservations.

A complete `FULL` audit must return CONSISTENT/AGREES before any timing. The profile is the opt-in
`TestCALV0070_WriterHistoryProfile`:

```sh
CORVINT_TASKS_HISTORY_PROFILE=<report.json> \
  GOTOOLCHAIN=local go test -count=1 -timeout 30m \
  -run TestCALV0070_WriterHistoryProfile ./internal/tasks/store/
```

The run took medians of 3 on an Apple M2 Max (12 CPUs) with Go 1.27.1 on darwin/arm64. Host load
average was 25–33, because other agents were running concurrently. Times are in milliseconds.

| Receipts | Lookup audit | Inventory | Complete audit | Tree digest | `Mutate` | Lease watch | Lease observed audit | Guarded inventory | S12 checkpoint + tail |
|---|---|---|---|---|---|---|---|---|---|
| 500 | 412 | 320 | 513 | 8 | 1,471 | 70 | 525 | 8.5 | 14 |
| 2,000 | 1,534 | 1,415 | 1,434 | 5 | 5,033 | 205 | 1,364 | 22 | 12 |
| 7,000 | 4,558 | 4,129 | 3,939 | 6 | 16,561 | 570 | 4,716 | 56 | 13 |

Between 2,000 and 7,000 receipts, the marginal cost per receipt is:
- 0.60 ms for the lookup audit;
- 0.54 ms for the inventory;
- 0.50 ms for the complete audit;
- 0.66 ms for the remainder of `Mutate`.

The run that produced the table used the earlier provisional label CAL-V0-069 in its report. Only
the label differs; the code is the same.

CPU profile of three mutations at 2,000 receipts (10.6 s of CPU):

| Where | Share |
|---|---|
| Journal audits | 64% |
| Inventory | 29% |
| `transaction.Model` (capacity) | 3% |
| `apply` | 2% |

Underneath, `safeopen.InRoot` accounts for 41% cumulative, and flat time is mostly syscalls. Every
path is opened by descending from `/` and bridging through `/dev/fd`. `wire.Parse` is about 5% and
GC about 7%. SHA-256 is not visible. Allocation is 1.655 GB and 9.006 million allocations per
mutation. The cost is dominated by receipt and record decode/encode; the inventory reads about
200 MB per mutation.

A CPU profile of three mutations at 7,000 receipts (39.8 s of CPU in `Mutate`, load average
39–50) puts:
- 59% in journal audits;
- 37% in the inventory;
- 2.7% in `transaction.Model` capacity measurement;
- 0.7% in `apply`.

The wall-time remainder is therefore not a separate CPU step. The likely cause is contention and
GC under host load, which is an inference.

Baseline, the maintained `BenchmarkCALV0070_MutateAt2000Receipts` with `-benchtime 5x -benchmem`.
Both runs gave the same allocation, 1.655 GB and 9.006 million allocations per operation:

| Run | Time per operation | Host load average (before → after) |
|---|---|---|
| 1 | 11.50 s | 13 → 44 |
| 2 | 7.11 s | 28 → 41 |

Wall time is dominated by host load on this shared host. The allocation figures are the stable
before number, and wall time is comparable only under recorded load.

## Decisions

1. Both writer paths scale with history.
   - `store.Mutate` (ticket mutations) makes three history passes.
   - Lease commands (claim, renew, heartbeat and release) make one observed pass and reuse its
     digests. At 7,000 receipts that pass takes 4,716 ms, plus 570 ms for a change guard that
     watches every retained file.

   The live 2.9 s per lease operation in issues 494 and 545 is therefore the single-pass cost.
   The design covers both paths.
2. Stop at design. The brief allowed implementation only for a single dominant O(history) step
   with an obviously correct bounded fix. The profile shows three comparable history passes under
   the lock, plus a growing remainder, all driven by per-file open cost. Removing any one of them
   leaves the cost proportional to history.
3. A writer checkpoint, separate from the S12 read checkpoint. Older runtimes neither read nor
   replace `<state directory>.writer-checkpoint.json`, so a mixed-version store degrades to
   complete audits rather than churning a shared file. It adds three things to the S12 contents:
   - a request index, which makes duplicate-request refusal independent of physical projections;
   - the name sets of the write-once directories, so names-only listings still refuse strays and
     gaps on every mutation;
   - exact capacity aggregates, so `CheckCapacity` never opens history files.
4. Derived only from complete audits, with a cadence of `K` receipts. A resumed observation never
   seeds a checkpoint, which avoids compounding trust. That bounds both the tail and
   prefix-tamper detection latency by `K`.
5. Equivalence or fallback. The resumed path must produce byte-identical receipts and identical
   capacity verdicts. Any doubt falls back to the complete audit, whose verdict governs
   (product invariant 2).

Rejected:
- Retaining the checkpoint after each resumed write. That would compound trust and leave tamper
  detection unbounded.
- Trusting projection presence for duplicate detection. A deleted request file would let a
  duplicate ID fork the request history, which the complete audit then refuses permanently.
- Extending the S12 file to a new profile. Older readers would fall back to complete audits, and
  older writers would replace it.
- A mutable index or database. That violates invariant 7.

## Plan and owner decisions

The implementation plan, in order:
- (A) Merge the lookup and complete audits, and reuse the observed digests in the inventory, as
  lease preparation already does. There is no format change. The projection from the primitives is
  about 2.0–2.1 s at 2,000 receipts, still proportional to history; this is an inference, not a
  measurement.
- (B) Open the state root once per scan rather than per file. This is a constant factor.
- (C) The writer checkpoint, as its own reviewed slice under the invariant-7 benchmark and format
  gate.

Owner decisions:
- Whether to accept writers resuming from derived state. This reverses the S12 non-goal and the
  last sentence of CAL-V0-061 for mutations.
- Whether to accept prefix-tamper detection latency of up to `K` receipts, and the value of `K`
  (proposed 256).
- Whether the full-audit cadence runs inline in writers (one writer pays about one complete audit
  every `K` receipts) or as an operator-run `receipt audit` with a hard tail bound.
- Whether the request index and capacity aggregates are an acceptable derived encoding under
  invariant 7.
- The acceptance factor between the 7,000- and 500-receipt writer cost.

## Limits

The fixture has 50 tickets, so costs proportional to live state (the intent tree, attempts and
reservations) are understated relative to the live store. The profile isolates history cost only.

Not run:
- Linux;
- default `GOMAXPROCS` on a quiet host;
- the live-store measurement;
- the issue 545 concurrent waves.

No receipt, journal, intent or archive format changed. No store was written outside temporary
fixtures.
