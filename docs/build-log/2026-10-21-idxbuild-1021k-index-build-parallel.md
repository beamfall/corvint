# Index build: parallel phases and fewer allocations (V1-2285), appmap selection pin (V1-0999), V1-0205 re-bench

Date: 2026-10-21. Lane `idxbuild-1021k`, branch `claude/idxbuild-1021k`, base
`8def072e08e9730a6102657c887655f62993b31d`. Requirement `GPK-V0-088` is accepted under the owner's
standing auto-accept of lane requirements (2026-10-11).

## V1-2285: index build wall time

Problem: a cold `corvint index` spent its CPU in the walk's `openat` (22%), GC (about 30%),
tokenising (13%) and `goSymbols` (8.5%), with the per-source tables, the vocabulary and the symbol
windows run one after another.

Decisions (`GPK-V0-088`):

- `compile` builds the vocabulary beside the per-source tables and the symbol windows. The
  vocabulary reads only `Sources` and the reuse inputs, which neither of the others writes; the
  identifier graph needs both and still runs last.
- `invertSourceTerms` inverts the term, word and path-term postings in parallel. All three callers
  use it (fresh build, blob-fact build, reused vocabulary).
- `buildSymbolWindows` reuses one cleared term set per worker instead of a fresh `terms()` map per
  symbol, and presizes the worker's interner.
- `parserSource` hands go/parser the source string's bytes without a copy (2.4% of build CPU);
  the parser only reads them.
- Not changed: the snapshot encoding and `snapshot.go` (open PR #1458 owns them), and the analyzer
  schema (`script/analyzer-schema.sh check` passes with no bump). Holding directory fds was already
  done by `cleanFileOpener`; the leaf `openat` remains.

Evidence (12-CPU host, all runs alternating base and new, load recorded per run):

- Content: the sha256 of the JSON-encoded built index is
  `a5a31c34934efec2ad272e5d464eb9254c1f117a5154e657de3e85c2230d334f` (228,135,754 bytes) for base
  and new. The gob bytes differ run to run on both because map order is not fixed.
- In-process `BuildForSnapshot` (`CORVINT_INDEX_REUSE=0`, excludes the snapshot write), five pairs at
  load 40-62: base 4.50, 5.42, 3.95, 2.76, 2.29 s; new 3.20, 4.35, 2.47, 1.85, 1.96 s. Median
  3.95 to 2.47 s (-37%), mean 3.79 to 2.77 s (-27%). An earlier six pairs at load 12-16 gave mean
  2.32 to 1.88 s (-19%) before the symbol-window change.
- CLI `corvint index` cold, six pairs at load 50-60: wall median 3.72 to 3.42 s (-8%), CPU
  (user+sys) about 8.2 s on both.
- Profiles (pprof, same host): openat is 13% of base and 20% of new samples; madvise 13.8% to
  12.3%; `goSymbols` 10.0% to 8.3%; `documentRecord` 9.4% to 8.3%.

Floor: the CLI wall did not reach -20% at this load. With every core busy, parallel phases cannot
lower wall time, and total CPU is unchanged within noise. What remains is the leaf `openat` per
pinned file (about 80 microseconds of kernel time each under load), the snapshot gob encode (about
0.3 s, in `snapshot.go`, held by PR #1458), Go body parsing (needed for parse-error semantics) and
the per-source term maps, which blob facts keep as data. The in-process build met -20%; the CLI
target is not shown and needs a quiet-host measurement.

Tests: `TestGPKV0088SymbolWindowsMatchEachSymbolsTerms` failed with `clear(window)` removed (symbol
2 carried the previous window's terms). `TestGPKV0088ConcurrentCompileMatchesSequentialPhases`
failed with two inversion results swapped.

## V1-0999: regression test for the appmap-only selection

Criterion 1 was met by attribution in `2026-10-20-affected-appmap-ruby-lane.md`.
`TestAppmapOnlyChangeSelectsOnlyJustifiedUnboundedReaders_V1_0999` builds this repository's graph
and selects for `internal/appmap/verify.go`. It pins five things:

- the owning unit is selected `DIRECT_SOURCE_CHANGE`;
- every witness resolves;
- each `UNBOUNDED_READER` selection is a graph-wide unbounded reader;
- that reader is not read-scoped and has a recorded cause (its own unbounded reads, or a dependency
  that locates the root);
- some units stay excluded.

At this base the selection is 56 selected, 15 unbounded readers and 271 excluded, with scope
`UNKNOWN`. The test fails when every unit is treated as an unbounded reader
(`internal/analyzercap` then has an unresolvable witness).

## V1-0205: re-bench, acceptance not met

Frozen `tools/retrieval-bench` v2, `--arms context`, all samples, `CORVINT_CONTEXT_*` unset, on a
lane binary. Numbers are recall@20 / @10 / @5 against the V1-0186 base (062b0151):

| Subset | Lane binary | V1-0186 base |
| --- | --- | --- |
| code2test | 0.4642 / 0.3962 / 0.3160 | 0.5116 / 0.3994 / 0.2877 |
| comment2context | 0.5062 / 0.3771 / 0.2687 | 0.5042 / 0.3438 / 0.2562 |
| edit2ripple | 0.6336 / 0.5517 / 0.3937 | 0.6293 / 0.4928 / 0.3448 |

code2test recall@20 fell 0.0474 below the baseline, so "recall@20 keeps its result" does not hold.
The drop is from changes made after V1-0186, not from the floor, but no arm separates the two.
The orientation task ("move the agent-memory backlog into the Corvint task store") still routes
`docs/AGENT-ROUTES.md` at result 2 (`AGENTS.md:124`). trace2code, abstention and the seven-task
unrelated probe were NOT_RUN once the criterion had failed; the seven-task set was never frozen.
V1-0205 stays open.

## NOT_RUN and rollback

NOT_RUN:

- a CLI timing on a quiet host;
- tier-2 affected units, left to batch CI;
- `make gate`.

Rollback: revert this branch's changes to these files, the two GPK tests,
`internal/liveverify/affected/appmap_selection_test.go` and the GPK-V0-088 section:

- `internal/contextindex/index.go`
- `internal/contextindex/termtable.go`
- `internal/contextindex/blob_shards.go`
- `internal/contextindex/snapshot_reuse.go`
- `internal/contextindex/symbolwindows.go`
- `internal/contextindex/parse.go`
