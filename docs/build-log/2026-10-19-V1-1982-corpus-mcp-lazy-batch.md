# V1-1982: the corpus MCP server loads lazily and answers path batches

Date: 2026-10-19
Ticket: V1-1982 (GitHub issues beamfall/corvint#1236 and #1237).
Requirements: DCP-V1-143..145 (`docs/specs/documentation-corpus-v1.md`), accepted under the
owner's standing auto-accept of lane requirements (2026-10-11).
The Failure modes sentence on MCP startup now says startup reads the header only; no accepted requirement is amended.

## Decisions

- **Startup reads the header, not the artifact.** `corpusbridge.New` checks the root, reads the
  artifact once under the existing bound, keeps its digest and decodes only top-level `schema`
  and `capabilities` for tool advertisement. Rederivation (DCP-V1-011) moves to the first call,
  which reports its cost; an unreadable header still fails startup with `corpus-unavailable`.
- **The memo is re-verified, never trusted.** A later call reuses the opened artifact only while
  the artifact bytes match the startup digest and every pinned revision still resolves to its
  tree; otherwise it rederives. Opening stays serialized behind a one-slot semaphore that honours
  the call's context. The memo is never handed to the reader as a runtime index, so receipts stay
  byte-identical to the CLI.
- **Live freshness is keyed by the repository observation.** Building the live context index
  costs about 2 s per call on an 11k-file repository and dominated every warm call. The overlay,
  including its memoized symbol re-resolutions, is reused while `contextindex.Observe` (object
  format, commit, tree, status digest, dirty paths) is unchanged: the same key the snapshot store
  reuses an index under. The observation is read before the build it keys, so a concurrent change
  can only force a recompute.
- **Only cited symbol anchors re-resolve.** DCP-V1-141 reports anchors cited by the returned
  records, so re-resolution is now lazy and per cited anchor instead of eager over the whole
  artifact; a pinned input that is unchanged in the live tree short-circuits to its pinned bytes.
- **A batch is the union of single calls.** `paths` (1..100, unique) returns
  `corvint-corpus-batch/1` with each single-path receipt verbatim minus its trailing newline, or
  an `unresolved` row. `oneOf` is not used in the input schema because some clients reject it at
  the top level; exclusivity is checked server-side as `invalid-arguments`.
- **Deadline, not hang.** Each call runs under a 50 s deadline. A batch names the paths it did
  not reach; a single call returns `deadline-exceeded`. Calls are served concurrently, so later
  calls are unaffected. `_meta["corvint/timing"]` carries load, verify, freshness, query, total
  and process CPU milliseconds.

## Measurement

Synthetic repository: 2,000 Markdown pages of 48 paragraphs (100,000 claims, 2,000 subjects,
97 MB artifact) and 10,000 Go files, built with the `corvint-corpus-input/2` manifest. Same host,
the new binary against its own build and the base (`547d264`) binary against its own build.

| Measure | Base `547d264` | This change |
|---|---|---|
| `tools/list` from process start | 2.53 s | 0.20 to 0.58 s |
| Physical footprint after `tools/list` | not sampled; 1.12 GB peak footprint over the run | 19 to 21 MB (258 MB peak during the one bounded read) |
| First `docs_locate` (rederive + freshness) | 20.3 s, then refused (`corpus-refused`, native validation failed) | 2.9 to 3.3 s, reported as `load: first-call` |
| Warm single `docs_locate` | 22.5 s, refused; 20 calls took 429 s and 567 CPU-s | 0.13 s |
| 100-path batch, 20 runs | not supported | p50 0.14 s, p95 0.15 s |

The base CLI on the same corpus fails the same way after 21.8 s with
`git-budget-exceeded`: eager re-resolution reads every pinned input through Git, so no base call on
this corpus ever answered. The new CLI answers the same locate in 2.75 s.

`ps` RSS stays near 265 MB after startup on macOS because freed pages are kept until memory
pressure; `footprint` reports what the process is charged.

Not measured: the reporter's own 101k-record corpus and host. A docs-scope corpus of this
repository could not be built (`native paragraph bound exceeded` under `/1`, and
`vacuous or oversized anchor` under `/2`), so the synthetic corpus stands in.
