# Read-start freshness and decode allocation for GitHub #1598

Date: 2026-10-23. Lane: claude/corpus-residual-gh1598, based on the batch head
`origin/claude/batch-1023a` (3a6da1a65, which holds #1621). Requirements `DCP-V1-465..468` in
`docs/specs/documentation-corpus-v1.md` are proposed and experimental. Ticket V1-2477 holds the
residual of #1598.

## Targets on the batch base

V1-2477's four acceptance targets, checked against 3a6da1a65 before this change:

1. Warm CLI `flow` and `gaps` under 200 ms median on the 66.75 MB `DCP-V1-418` fixture, measured
   with `--timings`: not met. Base medians were 205 to 248 ms at load 11 to 24 (table below).
2. Peak RSS bounded and recorded against the artifact size with a stated target: met by #1621.
   `DCP-V1-430` states at most 3.5 times the artifact (234 MB here); base reads measured 186 to
   239 MB, and one base `flow` run at load 14 to 16 reached 239 MB, above the bound.
3. Each listed path reuses the retained artifact or records why it rederives: met by #1621
   (`DCP-V1-432`; the delta current side is `DCP-V1-460..464`).
4. MCP first and warm calls re-measured at scale: #1621 recorded 359 ms and a 39 ms warm p50. They
   are re-measured below against this change.

## Where the base read spent its time

A trace build of the base (load about 14) showed the critical path. The primary checkout's live
observation started from the header callback, about 16 ms in, and ran 150 to 210 ms. It finished
after the decode (scan end 150 to 185 ms) on most runs, so freshness set the read's wall time. A
CPU profile over five `flow` runs gave:

- `runtime.madvise`: 27.5 percent of CPU. This is fresh-span setup on darwin, so it scales with
  allocation volume.
- `memclr`: 12.5 percent.
- `deriveClaimTerms`: 13 percent. Most of it was growing postings slices one append at a time.

The canonical check boxed a copy of every decoded element (`element.Interface()`), about 32 MB of
`reflect.unsafe_New` per read. Allocation per indexed read was 304 MB.

## Decisions

- `DCP-V1-465..467`: a CLI query read starts the primary checkout's `ObserveTree` when the read
  starts, before the artifact is opened. It needs only `--root`. The first freshness of that root
  takes the observation, once. Any later freshness, or one of another root, observes again. Every
  exit cancels and joins the observation, after the header-started freshness is joined.
  Additional checkouts still wait for the header, because their directories come from the
  artifact. `freshness_ms` still counts from the header. Freshness now ends before the decode on
  most runs: `freshness_ms` is 136 to 165 ms while `parse_ms` is 150 to 177 ms.
- `DCP-V1-468`: the canonical check encodes through `element.Addr().Interface()`. Doccorpus
  records have no custom marshalers, so the compared bytes are identical. Each decode batch's term
  map is sized from its claim count. The index join counts each term's postings first and
  allocates each list once; a term that only one batch holds keeps that batch's list. The handoff
  gives up its blocks at most once, so nothing else sees the aliased list. Allocation per indexed
  read fell to about 233 to 255 MB.
- Tried and dropped, all measured on clean builds:
  - A wider Git isolation slot (8): no gain.
  - Fewer decode workers: slower.
  - A heap warm-up: no gain.
  - `GOGC`/`GOMEMLIMIT` tuning: RSS went over the bound or CPU rose.
  - A 128-claim split chunk: within noise.

## Measurements

Host: 10-core darwin/arm64, with other lanes running. Each binary was built clean
(`vcs.modified=false`) and ran on an artifact it built itself.

- Base: 3a6da1a65.
- Head: 497040cbc, the code and spec commit of this change.

Runs alternate between base and head, three runs per read in each round. Times are the
`--timings` `total_ms`.

Final rounds, load 12.6 to 14.8 (nine runs per binary across three alternating rounds):

| Read | Base median (range) | Head median (range) |
| --- | --- | --- |
| `flow --id adapter:lib0-1` | 208.7 ms (198.6-261.6) | 190.7 ms (183.3-244.0) |
| `gaps` | 229.3 ms (206.7-247.2) | 198.3 ms (187.3-341.0) |

Per-round head medians:

| Read | Round 1 (disturbed) | Round 2 | Round 3 |
| --- | --- | --- | --- |
| `flow` | 231.9 ms | 186.2 ms | 190.7 ms |
| `gaps` | 272.4 ms | 191.5 ms | 193.7 ms |

The base medians in the same three rounds were 201.7, 203.4 and 231.2 ms for `flow`, and 237.1,
210.5 and 237.8 ms for `gaps`.

Four earlier rounds at load 9.6 to 11.7, on clean builds of the head code (two of them without
the batch term-map size hint), gave medians of 185.8 to 202.8 ms for `flow` and 199.2 to 214.6 ms for `gaps`. Two base rounds at the
same load gave 212.2 and 237.2 ms for `flow`, and 217.8 and 228.7 ms for `gaps`.

CPU per read fell from 1.51 to 1.95 s to 1.41 to 1.66 s.

Peak RSS:

| Build | `flow` | `gaps` |
| --- | --- | --- |
| Head | 172 to 201 MB, 2.6 to 3.0 times the artifact | 180 to 222 MB, up to 3.3 times |
| Base | 188 to 226 MB | 189 to 222 MB |

Every head run is within the 3.5-times bound.

Corpus MCP: `TestDCPV1416CorpusMCPFirstAndWarmCalls` with `CORVINT_READ_FIXTURE_LARGE=1` at load 7
to 11 on head. The first call took 290 ms (load 85, freshness 150, query 17). Warm calls had a p50
of 37 ms, a p95 of 189 ms and a max of 191 ms. Rederivation took 2.23 s. The MCP bridge does not
use the read-start observation, so its first-call gain comes from the decode allocation changes and
a lower host load.

## Against the targets

- Warm `flow` and `gaps` under 200 ms median: borderline, so not counted as met. In the final
  rounds the pooled nine-run medians were 190.7 ms and 198.3 ms, and both undisturbed rounds came
  in under 200 ms. However, the `gaps` margin there was under 2 ms pooled and 6 to 9 ms per round,
  one disturbed round gave 232 ms and 272 ms, and three of the four earlier rounds had a `gaps`
  median of 201.5 to 214.6 ms. The change takes 20 to 30 ms off both medians against the base in
  the same rounds. The new floor is the per-process decode and canonical check, a `parse_ms` of
  about 150 to 180 ms. Getting robustly under 200 ms would need a decoder that does not use
  reflect-driven JSON decoding, a smaller decoded representation, or a quiet-host measurement that
  shows the current margin holds.
- Memory: recorded against the stated 3.5-times bound and met on every head run. The owner's
  1.5-times example stays unmet, as #1621 recorded.
- Retained-artifact reuse on every listed path: unchanged from #1621, met.
- MCP first and warm calls: re-measured above.

## Verification

Focused tests:

- `internal/doccorpus`, the whole package in `-short` mode: pass.
- `-race` on `TestDCPV146[5-7]`, `TestDCPV1434`, `TestDCPV1431`, `TestDCPV1412` and
  `TestDCPV1410`: pass.
- `cmd/corvint` with `-race -run 'TestDCPV1431CorpusRefusalJoinsFreshness|TestDCPV1413|TestDCPV1432'`:
  pass.
- `cmd/corvint` with `-short -run 'DocsCorpus|TestDCPV14|CorpusMCP|CorpusTimings'`: pass.
- `TestDCPV1416CorpusMCPFirstAndWarmCalls` on the large fixture: pass.
