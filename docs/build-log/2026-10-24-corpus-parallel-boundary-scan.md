# 2026-10-24 V1-2571: parallel boundary scan of split corpus members

V1-2571 (target 1 of V1-2477, part of GitHub #1598) asked for a split-safe parallel or vectorised
scan of the element bounds a warm CLI read finds before it hands each split element to a decode
worker (`DCP-V1-414`). `DCP-V1-551..553` record the contract in
`docs/specs/documentation-corpus-v1.md`.

## Design

The decode scans a split member's elements in parallel segments of the next window region (at most
4 MiB, segments of at least 32 KiB, up to `GOMAXPROCS`). Each segment after the first guesses its
first element just after a separator: a comma followed by the first element's bytes through its first
member name and colon (`,{"id":` in a canonical artifact). Inside a canonical JSON string every quote
is escaped, so the separator cannot occur there. Each segment scans whole elements with the
sequential scan's own `scanValue`. Runs are joined only while each starts where its predecessor's
next element does. The first segment starts at the scan position, so by induction every joined
element is exactly the sequential scan's next one. A wrong guess, an anomaly or a truncation only
shortens the join, and the sequential scan resumes after the last element handed over.

Rejected: a two-pass summary scan that worked out each segment's quote parity and bracket depth
first, then fixed up the segment starts. It read every byte twice, and on the measuring hosts it saved
nothing, because their CPUs were already saturated.

## Equivalence

`TestDCPV1552ParallelScanMatchesSequential` compares every decode with eight segmentations against
a one-segment scan. The comparison covers records, digest tail and refusal message, in memory and
streamed at windows of 7, 61 and 512 bytes and 1 MiB. Its inputs are:

- the committed fixture;
- claims holding adversarial texts;
- every streamed refusal shape;
- the claims member truncated or carrying `"`, `\`, `]`, `}` or `,` at up to 1500 offsets.

`TestDCPV1551*` check the scan alone against `scanElements`. Canonical and hostile arrays use
one-byte segments, so a bound falls at every byte; on hostile arrays the result is a prefix. 4000
corrupted arrays must yield a prefix only.

Results, linux/WSL2:

- `./internal/doccorpus` ok, 52.2 s;
- `internal/corpusindex`, `internal/mcp/corpusbridge`, `internal/perfbudget` and `internal/corpusrepublish` ok.

## Measurements

| measurement | host | load average | base `a6818460` | change `96168926` |
| --- | --- | --- | --- | --- |
| claims scan alone, in process | darwin, 12 CPU | about 5 | 36.1 ms | 8.5 ms |
| streamed decode, in process | darwin | about 5 | 75.7 ms | 65.5 ms |
| in-memory decode, in process | darwin | about 5 | 62.0 ms | 56.7 ms |
| warm CLI `flow` total, p50 / p99 | linux/amd64 WSL2, 32 threads | 22.3 to 26.0 | 239.4 / 561.9 ms | 258.1 / 561.3 ms |
| warm CLI `gaps` total, p50 / p99 | linux/amd64 WSL2 | 22.3 to 26.0 | 289.1 / 639.8 ms | 286.9 / 625.4 ms |
| warm CLI peak RSS, max | linux/amd64 WSL2 | 22.3 to 26.0 | 223 MB | 220 MB |
| warm CLI `flow` total, p50 / p99 | darwin | 24.2 to 118.7 | 792.3 / 1056.1 ms | 718.0 / 1175.7 ms |
| warm CLI `gaps` total, p50 / p99 | darwin | 24.2 to 118.7 | 794.2 / 1153.7 ms | 789.7 / 1087.8 ms |
| warm CLI peak RSS, max | darwin | 24.2 to 118.7 | 228 MB | 219 MB |

How the CLI rows were measured:

- Each CLI row is 20 interleaved runs per binary, one new process per run, on the `DCP-V1-418` fixture.
- The artifact was built by the base binary.
- Both binaries were built from clean trees, so both read it `artifact-verified` (`DCP-V1-471`).
- A dirty-built binary is unattested and rederives the artifact from source, which takes about 3.5 s and 700 MB.

## Outcome

Peak RSS stays within the 234 MB bound of `DCP-V1-430`.

The target of a warm `flow` and `gaps` p50 under 170 ms is **not met**:

- Neither host was quiet. Other lanes kept the box at load 22 to 26 and darwin at 24 to 119.
- Under that load the CLI difference is within the noise.

Instrumented phase timings on darwin show the decode has a new critical path:

- The scan is no longer the bottleneck.
- The main goroutine now waits on decode workers. Those workers spend about 170 ms of CPU per decode on the JSON unmarshal and the canonical re-encoding check.
- `reflect.MakeSlice` on the dispatch path is the next serial cost.

Reaching the target needs that worker CPU cut and a quiet-host qualification run. Both are filed as
follow-up work.
