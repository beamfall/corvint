# V1-4726: test dry-run CPU and allocation

Date: 2026-11-05. Performance repair; existing intent and outputs are unchanged.
Governing contracts: AFP-V0-012 (legacy attribution), AFP-V0-021 (literal readers),
AFP-V0-551 (fixture markers), STR-V0-001/007 (same observed graph), STR-V0-072 and
TSP-V0-006 (repeated source-slicing plans). No new requirement IDs or performance
figures in a requirement are introduced or amended.

## Observation and decision

The baseline is origin/main at `28a3ff86a2c3d17662765da12cc9251c06466f5f`.
Both binaries were built with local Go 1.27.3. The outside shared clone was checked
out at `21fea70cb1fec853eecd55ebaadc3a61a1e11534`; the measured base was
`6394d6fdc94d0a7a15c5a46b7fdc0eaddc75b9c0` (selection-baseline pr-3).

The directory worker already observes each Go directory once per graph build.
The repeated work was downstream: every source-slicing eligibility plan rebuilt
all units' split path tokens, including the cross-unit deduplication map. The
first allocation profile attributed 344.78 MiB to that work. The Go lexer also
constructed `kind.String() + text` for every token, retained per-literal fixture
locations even without a marker, and allocated regexp match indexes and token
slices. Its first allocation profile attributed 439.37 MiB to `golang.pathTokens`.

The first repair alone fell short: median user CPU was 10.92 -> 7.89 seconds
(27.7%). The observed remaining dominant cost was the independent legacy
attribution scan called by `cmd/corvint/test.go`. Decision: apply the same
allocation repairs in `internal/gateselect/readers.go`, preserving its broader
lexicon and its different comment/root-locator handling. Replacing that index
with the planner's narrower unit tokens would change authority and selection.
The command already passes its compiled graph to slicing and its one attribution
index to the verdict, so no command control-flow change was needed.

## Change and compatibility

- Build the split token index once while finishing a graph; selection-input
  copies share it read-only. The index is derived in memory and excluded from
  canonical graph bytes, digest fields and the existing disk-cache encoding.
  Internally assembled unfinished graphs retain the uncached observation path.
- Keep token kind and text separately in both lexers. Use the identical ASCII
  character class directly, skip printf rewriting when there is no percent,
  and append literal tokens into file-local buffers. Only files containing a
  possible fixture marker retain per-literal line spans; the lexer still
  distinguishes marker comments from strings.
- Inspect short outer path components without allocating a split slice. Iterate
  directory-move words and parent-path components without materializing slices.
  Hash the graph domain and canonical body incrementally instead of copying the
  body to prefix it. No new disk cache or process-global cache was added.

## Same-host measurements

Platform: WSL2/Linux, 32 logical CPUs, Go 1.27.3. These are local lane-host
measurements, not a quiet-host qualification: load1 was 5.51 to 5.95, and native
Windows activity was NOT_OBSERVED. Warm OS page cache; no cache deletion or
privileged host changes. Three alternating before/after runs of exactly
`test --base 6394d6fdc94d0a7a15c5a46b7fdc0eaddc75b9c0 --dry-run --manifest M`
on the same clone. Program identity keeps existing cache entries separate in
validity, so alternating versions exercises identity misses.

| Run | Before wall / user / system seconds | After wall / user / system seconds |
| --- | --- | --- |
| 1 | 3.34 / 10.58 / 1.18 | 2.32 / 6.25 / 1.08 |
| 2 | 3.37 / 11.25 / 1.46 | 2.38 / 6.45 / 1.16 |
| 3 | 3.37 / 10.58 / 1.35 | 2.27 / 5.96 / 0.84 |
| Median | 3.37 / 10.58 / 1.35 | 2.32 / 6.25 / 1.08 |

Median user CPU fell **40.9%**, meeting the requested 40% threshold on this host;
wall time fell 31.2%. The margin is small and shared-host timing remains noisy.
Median user-plus-system CPU was 11.93 -> 7.33 seconds (38.6% less); the 40.9%
figure uses the requested user-CPU measurement.

CPU profiles, both also collecting an allocation profile at shutdown, had 13.94
and 7.16 seconds of samples. `go tool pprof -top -cum` reported:

| Function | Before cumulative CPU | After cumulative CPU |
| --- | ---: | ---: |
| `main.testRunInput` | 3.72 s | 1.87 s |
| `golang.Language.observeDirectory` | 3.17 s | 1.65 s |
| `golang.pathTokens` | 2.54 s | 1.07 s |
| `Graph.pathTokens` / new `Graph.splitPathTokens` | 1.06 s | 0.05 s |
| `gateselect.scanSource` | 1.81 s | 0.90 s |
| `gateselect.pathTokens` / new `appendPathTokens` | 0.82 s | 0.15 s |
| `main.testSourceEligibles` | 0.98 s | 0.21 s |

Sampled allocated bytes fell from 2184.45 MiB to 1502.93 MiB (31.2%). The
independent attribution scan, Go parser/scanner and GC remain visible costs.

## Verification and limits

At change start, `make dogfood-change BASE=28a3ff86a2c3d17662765da12cc9251c06466f5f`
retained pre-change context, query and impact receipts. Their SHA-256 values were
`0bda1babe689f06250c99082f1b32d3acc72c02950462d2d60b91d3e35d786ad`,
`76a503785b7275e10102096b6139c492600141618dc2ccff755f5756dd65da4c`, and
`ab4e813bada1c3d1f7979649f71d822f14dd4b1507b9ca72476a75836d622a87`.
Context reported `no-specific-terms`, 20/22 included results and four omitted
paired tests; query did not abstain but omitted four ranked results; impact
included its four requested paths and omitted 785 other results. The initial
impact scope did not include the subsequently profiled attribution package.
The initial coordination query packet coverage was NOT_PRODUCED
`packet-coverage-unreadable`; the context/query/impact receipts themselves were
PRODUCED. Initial CEM/OCM/outcome steps were NOT_PRODUCED `no-diff-yet`, as expected
before an implementation commit. These are retained limits, not coverage claims.

`corvint affected --base 28a3ff86a2c3d17662765da12cc9251c06466f5f` selected all
three touched packages. Their existing tests ran as whole packages, without a
`-run` filter, each in its own `script/heavy.sh` command with GOTOOLCHAIN=local,
`-count=1 -timeout 30m`:

- `./internal/liveverify/affected/golang/`: PASS, 9.212 s.
- `./internal/liveverify/affected/`: PASS, 56.946 s.
- `./internal/gateselect/`: PASS, 2.427 s.

New tests exercise immutable token reuse across repeated/concurrent plans and
selection-input copies, isolation between graphs, fixture-token spans across
buffer growth, legacy comment/root-locator boundaries, and both optimized
lexicons against the original regexps over deterministic arbitrary-byte inputs.
An initial affected-package run exposed unfinished synthetic graphs and an
unsorted new fixture; those failures were repaired without changing existing
assertions, and the full package passed. The moved-citation repin found no
changes; `make -s line-citations-check` passed.

Repository-wide tests, doc-gates, selection-baseline-check, Windows vet and
model-backed review are NOT_RUN by lane instruction; the host verifier and
independent reviewer own those steps. Native ticket completion is NOT_PRODUCED:
the lane forbids task writes, and V1-4726 was NOT_OBSERVED in the local task
projection, primary checkout or supplied older read-only snapshot.

All **58/58** entries in `benchmarks/selection-baseline/changes.json` matched.
For each detached historical head, both binaries ran `affected --base B` and
`test --base B --dry-run --manifest M` in the outside clone. Direct byte
comparisons of stdout, stderr and manifests passed; every exit status was zero.
The binaries carry the same baseline VCS revision, keeping stale-binary
diagnostics comparable. `comparison.jsonl` binds every case's base, head,
status and output digests; its SHA-256 is
`040464536204862fc7c21a49b316c8a2b96fa47697cc8719831b5b436d535b75`.

## Evidence and reproduction

Raw files and scripts are outside the worktree at
`/home/russell/lanes/dryruncpu-evidence/`. The sandbox forbids `/var/tmp`, so
`bin/`, `clone/`, all manifests and the comparison script live there. Synthetic
`.git` directories injected at writable roots caused normal profile destinations
to refuse `profile-invalid`; `profile.py` passes anonymous memory files through
`CORVINT_CPUPROFILE` and `CORVINT_MEMPROFILE`, then copies them to the evidence
directory after process exit. It changes no executable or repository content.

SHA-256 bindings:

| Artifact | SHA-256 |
| --- | --- |
| `bin/before` | `cce00e936ceea4ee8518b941013d6dcce3f4f72751d67a188dfd431fa2926e93` |
| `bin/after` | `82f4d7c2934ab23c85c3795413689ddfc12f0a74230e225d6d5bd7e89cd2bb8b` |
| `before-profile.cpu` | `66b04b9a3296448ca811d27894f8de1bfbd9c24ca12aa6d6d7d1b97c5daba090` |
| `after-final.cpu` | `aee8b868671573626d01e59dd9d114cb1972c639a4d62ca35d3e2dbd8bdf8b01` |
| `before-profile.mem` | `015c3bfa53b574d2685479d99a1cd5f0a865105907627262261db30099e1c945` |
| `after-final.mem` | `818c88e02bea910fddda4c658b1450e04a0215584348066c391897cd0d9f906e` |
| `measurements.json` | `d0117dcc08efc1d087ece77912ddfd57a256482a5e3205660e977341fd202497` |
| `profile.py` | `612c7f703547c927e495d344d11cc55644325dfc27d5b07d97b4d6725864f159` |
| `measure.py` | `76803336c7a8623cb1a92c5a8c0a4b8ae9c8b2534220247331d3aff4c0b02682` |
| `compare.py` | `1b954fd7a938c97fd066764144980f52dd2fb6333036c52657f862f40884da56` |

Rollback: revert the performance repair and its tests together. No persisted
format migration, golden update or repository read-side state needs reversal.
