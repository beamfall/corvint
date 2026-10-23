# Documentation-corpus read residual for GitHub #1598

Date: 2026-10-23. Lane: opencode/corpus-read-residual-gh1598, based on PR #1609's head
(9708bc5dd). Requirements `DCP-V1-430..434` in `docs/specs/documentation-corpus-v1.md` are
proposed and experimental. They follow `DCP-V1-410..419`.

## Starting point

After #1609, on the `DCP-V1-418` fixture (66.75 MB artifact, 10,001 inputs, four additional
repositories): warm CLI `flow` 217 ms and `gaps` 226 ms median; the artifact decode and canonical
check cost 120 to 140 ms in every new process; freshness 122 to 183 ms in parallel; peak RSS 205 to
264 MB (3 to 4 times the artifact). `delta`, the joined `--corpus` read, flow docs, flow
coverage, the indexed companion build and republish still rederived.

Profiles of a read in a test process (same fixture) showed:
- the 66.75 MB read buffer stayed live beside the decoded records (about 47 MB) during decode;
- fresh-page costs (`madvise`, zeroing, span setup) were 25 to 40 percent of CPU, driven by
  allocation volume (about 285 MB allocated per indexed read);
- claim-term tokenization was about 110 ms of CPU per index derivation, 45 to 65 ms of wall
  time after the decode;
- json v2 already shares repeated short strings through the pooled decoder's string cache, so
  extra string interning saved about 1 MB and was dropped.

## Decisions

- `DCP-V1-431`: the CLI read and the joined `--corpus` read stream the artifact. A reader
  goroutine reads 1 MiB blocks ahead, a hasher goroutine hashes them in order (holding back the
  last 4 KiB for the tail), and the scanner works window by window. Each batch of record-array
  elements decodes from the window it was found in. A block buffer goes back to a small pool once
  its window, its hash and every batch decoding from it have released it. A short carry is placed
  in headroom before the next block, so the block is not copied. The in-memory reader (corpus
  MCP) uses the same scanner as one window. One decoder means one set of refusals. The tests
  split the fixture at block sizes from 1 byte to 1 MiB, with carries that fit the headroom and
  carries that overflow it.
- `DCP-V1-432`: the delta current side and the joined `--corpus` read take the verified read.
  Delta gains `--verify-rederive` and names its verified current side in a limitation. Flow docs
  generate, flow coverage, the indexed companion build, republish and carry-forward keep
  rederiving. Each writes records onward or issues a verdict that is consumed as source-validated,
  and the content digest proves integrity, not origin.
- `DCP-V1-433`: no persisted read index or sidecar. A posting set read back from a file can be
  trusted only by deriving it again, because completeness cannot be checked without tokenizing
  every record. Trusting it would let derived state decide which records answer. `build` writes
  only stdout, and a read writes nothing.
- `DCP-V1-434`: claim terms are derived on the decode workers, batch by batch, while the scan
  goes on. The index derivation takes them only when the batches cover every claim in order and
  the artifact was admitted. The index and its refusals are unchanged. This moved most of the
  index's wall time into the decode phase.
- Tried and dropped: a soft Go memory limit sized from the artifact (`GOMEMLIMIT` equivalent).
  At 1.25 times the artifact plus 16 MiB the live set did not fit; flow ran 43 collections and
  doubled in wall time. Looser limits gave little memory back. Lowering `GOGC` to 50 cut flow and
  gaps peak RSS by about 40 MB at some CPU cost, but it is a process-wide knob and was not adopted.

## Measurements

Host: 10-core darwin/arm64. Load averages were 37 to 51 throughout from other lanes, so every wall
time below is inflated and noisy. Each binary was built clean (`vcs.modified=false`) and ran
on an artifact it built itself. Before is 9708bc5dd (the #1609 head); after is 8755c162a (the
code commit of this change). Runs alternate before and after per read. Wall time is the process
wall time, including process start.

Round 1 (5 runs each, load 37 to 51):

| CLI read | Before median (range) | After median (range) | Peak RSS before / after |
| --- | --- | --- | --- |
| `info` | 357 ms (323-607) | 310 ms (266-467) | 207-216 / 152-158 MB |
| `search --query ledger --limit 20` | 382 ms (353-420) | 367 ms (306-414) | 236-238 / 166-184 MB |
| `get --id adapter:lib0-1` | 311 ms (265-340) | 292 ms (268-322) | 211-213 / 147-161 MB |
| `flow --id adapter:lib0-1` | 381 ms (352-482) | 347 ms (322-377) | 250-279 / 195-213 MB |
| `coverage` | 349 ms (340-498) | 342 ms (286-475) | 211-213 / 152-167 MB |
| `locate --path docs/area01/feature00001.md` | 301 ms (283-544) | 329 ms (263-385) | 207-215 / 152-164 MB |
| `gaps` | 434 ms (378-461) | 414 ms (376-532) | 250-279 / 206-218 MB |

Round 2 (3 runs each, load 44 to 45):

| CLI read | Before median (range) | After median (range) | Peak RSS before / after |
| --- | --- | --- | --- |
| `info` | 392 ms (367-531) | 324 ms (302-389) | 211-214 / 152-165 MB |
| `search` | 301 ms (261-362) | 281 ms (279-378) | 236-237 / 156-176 MB |
| `get` | 256 ms (243-304) | 252 ms (252-393) | 209-212 / 147-162 MB |
| `flow` | 358 ms (335-379) | 296 ms (290-447) | 251-273 / 214-224 MB |
| `coverage` | 259 ms (243-268) | 255 ms (241-260) | 208-213 / 148-161 MB |
| `locate` | 246 ms (242-288) | 262 ms (247-316) | 208-210 / 152-162 MB |
| `gaps` | 315 ms (300-325) | 322 ms (308-325) | 249-281 / 217-230 MB |

`--timings`, 3 runs each at load 26 to 36: `index_ms` fell from 70 to 97 ms to 33 to 60 ms for
`flow` and from 64 to 78 ms to 43 to 72 ms for `gaps`, because the claim terms now arrive with
the decode. `parse_ms` rose by about the same amount (147 to 201 ms before, 181 to 208 ms after,
outliers aside), because the tokenization CPU moved into the decode phase rather than
disappearing, and the host had no spare cores to absorb it. `freshness_ms` (Git in five
checkouts) was 186 to 230 ms before and 208 to 228 ms after, and it ends last on most runs.

Corpus MCP, `TestDCPV1416CorpusMCPFirstAndWarmCalls` with `CORVINT_READ_FIXTURE_LARGE=1`, base and
head test binaries run back to back at load 24 to 35:
- base: first call 531 ms (load 164, freshness 292, query 26); warm p50 29 ms, p95 262 ms, max
  342 ms; rederivation 3.30 s.
- head: first call 359 ms (load 109, freshness 193, query 17); warm p50 39 ms, p95 220 ms, max
  230 ms; rederivation 3.63 s.

Warm p95 comes from calls inside the `DCP-V1-314` racy window, which run Git.

## Against the targets

- Memory: peak RSS fell by 50 to 60 MB on every read: 147 to 184 MB (2.2 to 2.8 times the
  artifact) for the unindexed reads and 195 to 230 MB (2.9 to 3.4 times) for `flow` and `gaps`.
  The recorded bound of `DCP-V1-430`, at most 3.5 times, is met. The owner's target of memory
  close to the artifact size (the 1.5 times example) is not met. The decoded records alone are
  about 47 MB and the query index about 18 MB. Reaching 1.5 times needs a smaller decoded
  representation or lazy record decoding, not a different reader.
- Warm CLI `flow` and `gaps` under 200 ms: not met. On this host, at load 37 to 51, no read
  measured under 240 ms, before or after. `flow` improved by 9 to 17 percent in median; `gaps`
  was within noise.
- Per-process decode and verify: still paid in every new process, now streamed and overlapped
  with the read, the hash and the claim-term derivation. It was not removed.
- Freshness: unchanged, and now the last phase to finish on most runs.
- Remaining paths: delivered. Each either reuses the verified artifact or has a recorded reason
  to rederive (`DCP-V1-432`).
- MCP first and warm calls: re-measured at scale (above). The first call stays under 1 s and the
  warm p50 under 100 ms.

## Verification

Focused tests: `internal/doccorpus` (whole package), `internal/testvaliditydoc`,
`internal/mcp/corpusbridge`, `cmd/corvint-corpus-mcp`, `cmd/corvint-corpus-http`,
`internal/corpusindex`, `internal/corpusrepublish`, `internal/flowdocs`, `internal/flowcoverage`,
`internal/delta`, and `cmd/corvint` with `-run 'Corpus|DCPV1|DocsCorpus|DocsFlows'`. The new
streaming and decode-time term tests also ran under `-race`. Results are in the pull request.
