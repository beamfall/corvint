# 2026-10-14 V1-0587: why the corpus capacity allocation ceiling is race- and load-sensitive

Ticket V1-0587 (GH399 follow-up). `TestIndexedCorpusCapacityQualification` (DCP-V1-040, synthetic
25,000 claims and 5,000 symbols) brackets indexed `Open`, `Query`, native `ParseArtifact`, native
`Query` and receipt `Encode` with `runtime.MemStats.TotalAlloc` and fails an ordinary run above
1 GiB. Race runs keep every functional check and report the budget `NOT_RUN` (commit b4a0f8eae).
This entry retains the measured diagnosis and the chosen repair. Base: `origin/main`
`d63c1c3a1b091c6bc13aecc81bd03e0d3fa8c0a3`.

## Decision

Repair option (b): the product genuinely allocates near the ceiling, so the ceiling stays 1 GiB
and the optimisation is reported as a finding. Neither the ceiling, the race guard nor the DCP-V1-040
wording changes. The only code change is test reporting: the failure message now names the measured
`open_query_total_alloc`, `open_query_mallocs` and `gc_cycles` (the hosted failure printed no byte
total), and the pass log adds the same fields plus `gomaxprocs`.

## Root cause (measured, except where marked inferred)

- The bracket contains no test-started goroutines: `runtime.NumGoroutine()` is 2 (runner and test)
  before and through every phase. Neither `internal/corpusindex` nor `internal/doccorpus` starts a
  goroutine, sets a finalizer or calls `t.Parallel`; logging happens after the bracket.
  `TotalAlloc` is process-wide, so this does not prove that every byte is the measured calls'
  (runtime-internal allocation is not separated), but the variable part is: it matches the
  encoder-growth step below exactly, with an unchanged malloc count.
- Malloc counts are stable (about 1,439,300 to 1,440,200 in every mode, ordinary or race) while
  the byte total moves in discrete steps of about 160.7 MB. Per phase, typical ordinary run: `Open`
  658.1 MB (1.18 M mallocs, 4 GC cycles), indexed `Query` 45 KB, `ParseArtifact` 109.6 MB, or
  270.3 MB in the step-up runs, native `Query` 15 KB, `Encode` 16 KB.
- The step is `encoding/json/v2` encoder-buffer growth. `json.Marshal` takes its encoder from a
  `sync.Pool` (`jsontext.bufferedEncoderPool`) and keeps large buffers when they are well used. A
  warm encoder marshals the 29,811,203-byte corpus with 29,818,880 bytes and 1 malloc; a cold one
  (pool cleared by two GC cycles) needs 190,565,456 bytes and 61 mallocs, growing its buffer
  geometrically. The difference, 160,746,576 bytes, is exactly the observed step.
- Whether a pooled encoder is warm depends on the scheduler and the GC. Source facts
  (`sync/pool.go`): pool items survive one GC cycle in the victim cache and are dropped on the
  second; the per-P `private` slot is reachable only from the same P; race builds drop one `Put`
  in four at random. Measured: GOMAXPROCS=1 lowers the total to about 405.8 MB, and lower GOGC
  (more cycles) raises it. Inferred: P migration after a GC or a preemption is what makes the
  default 12-P runs miss, and host load acts through GC frequency and migration; the per-call
  migration was not traced.

## Measurements (Darwin/arm64, Go 1.27.2, GOMAXPROCS 12, shared host at load 10 to 30)

`go test -count=5 -run '^TestIndexedCorpusCapacityQualification$' -v ./internal/corpusindex` and a
scratch per-phase probe (not committed). Contention: one busy loop per CPU (12) for the run,
removed by an EXIT/INT/TERM trap.

| Mode | open_query_total_alloc (bytes) | Result |
| --- | --- | --- |
| ordinary, quiet (test x5) | 928,464,592; 767,693,192; 955,667,760; 928,435,256; 774,354,072 | 5 PASS |
| ordinary, quiet (probe x5) | 767,717,608; 928,453,640; 767,709,016; 767,721,688; 767,702,632 | 5 PASS |
| ordinary, contention (test x5) | 928,486,192; 767,723,720; 957,342,504; 767,711,320; 767,705,312 | 5 PASS |
| ordinary, GOMAXPROCS=1 (probe x3) | 405,852,056; 417,483,176; 405,824,120 | best case: Open 296.2 MB |
| ordinary, GOGC=10 (test x4) | 1,115,746,416; 954,965,192; 954,990,752; 954,995,880 | 1 FAIL, 3 PASS |
| ordinary, GOGC=10 / GOGC=1 (probe x3 each) | 955,014,424; 1,115,753,504; 955,021,536 / 955.0 M x3 | same steps |
| race, quiet (test x5) | 1,251,016,312; 1,304,009,816; 795,252,056; 929,512,512; 956,056,872 | 5 PASS (budget NOT_RUN) |
| race, contention (test x2 + x3) | 1,116,805,136; 1,076,457,512; 1,251,006,240; 1,116,794,328; 1,056,553,960 | 5 PASS (budget NOT_RUN) |
| ordinary + injected 512 MiB in bracket | 1,103,490,752 (566,619,840 + 536,870,912) | FAIL with bytes named |

Hosted CI, Linux/amd64 Go 1.27.2 (`go-product-shard (6)`, `-race -json -p 1`): run 38610840891
logged 1,090,213,832 and run 38602297631 logged 1,413,394,768, both race-instrumented with the
budget `NOT_RUN`. Ordinary Linux/amd64 totals are `NOT_OBSERVED`: CI runs the product packages only
under `-race`, and the ordinary `go test` steps do not print passing test logs.

Ordinary headroom under the 1,073,741,824-byte ceiling: 28.5% at the common 767.7 MB mode, 10.8% at
the largest default-GOGC value (957,342,504), and none in the high-GC mode (1,115,746,416, 3.9% over).
The 2026-10-01 parallel Linux/arm64 failure (three packages, Landlock-confined) fits this mechanism:
concurrent packages raise GC frequency and P migration. That reading is an inference; that host
was not re-measured.

## Finding for the orchestrator (not repaired here)

Product optimisation: `corpusindex.Open` and `doccorpus.ParseArtifact` marshal the whole 30 to
35 MB corpus or index several times (canonical re-encode, digest re-encode, index re-encode) through
`json.Marshal`, whose cold-buffer growth costs about 6.4 times the output. Encoding into a buffer
pre-sized from the known input length (for example `json.MarshalWrite` into a grown `bytes.Buffer`)
or removing the duplicate re-encodes should move the allocation towards the measured 405.8 MB
GOMAXPROCS=1 case. That projection is inferred, not measured: removing duplicate marshals alone
still leaves the remaining marshals on pooled encoders, so only pre-sized output buffers would
remove the pool-state dependence. The change is in `internal/corpusindex` and `internal/doccorpus`, outside
this lane's test-only scope.

## Failure modes and limits

- Ordinary runs can still exceed the ceiling when GC runs often (reproduced with GOGC=10) or under
  heavy load; the failure now names the bytes, mallocs and GC cycles so a hosted failure can be
  matched to the encoder-growth steps. The 1 GiB budget is unchanged and is not silently raised.
- No race-only sanity ceiling was added: DCP-V1-040 says race runs report observed allocations with
  the ordinary budget `NOT_RUN`, and the measured race spread (795 MB to 1.41 GB) has no stable bound
  without the optimisation above.
- GH399's broader qualification limits stay as recorded: external utility, producer authentication
  and current source validation remain `NOT_OBSERVED`.

## Acceptance evidence

- AC1: the comparison above (ordinary, race, contention, GOMAXPROCS=1, low GOGC, hosted race) and
  the encoder cold/warm measurement; the extra allocation is product-owned JSON buffer growth whose
  size depends on pool state, not test accounting of foreign allocations.
- AC2: the 1 GiB ordinary ceiling and the race guard are unchanged; race runs still execute every
  functional check (10 race runs above passed); an injected 512 MiB allocation still fails the
  ordinary ceiling.
- AC3: focused checks, review and hosted CI are listed in the lane report and PR.

## Review

One read-only Codex round: CONCERNS, three labelling findings in this entry (inferred mechanism
under a measured heading, process-wide attribution, unmeasured optimisation projection). All three
were confirmed and fixed by labelling; the test diff passed (ceiling and race guard preserved, no
product or DCP-V1-040 change).

## Rollback

Revert the test commit; it only changes the failure message and log fields.
