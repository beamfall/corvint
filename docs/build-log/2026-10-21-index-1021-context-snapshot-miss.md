# index-1021: snapshot-miss notice, generic-import prefilter, dirty-worktree disclosure

Date: 2026-10-21. Lane: claude/index-1021 (base f284484eb). Tickets: V1-1640, V1-0328, V1-1285,
V1-2207, V1-1758. Requirements `TCP-V0-140` and `TCP-V0-141` are accepted under the owner's
standing auto-accept (2026-10-11). Snapshot and index on-disk formats are unchanged, so the
benchmark/format gate (AGENTS.md invariant 7) is not engaged.

## V1-1640: a no-snapshot `corvint context` silently rebuilt the index

Problem: with no snapshot of the tree for the running engine (a fresh clone, or any rebuilt binary,
whose engine digest misses the old snapshot), `corvint context` builds the whole index in memory,
never persists it (the command is read-only), and said nothing.

Decisions:

- (a) `TCP-V0-140`: on a miss over at least 1,000 tracked paths, stderr names the in-memory build,
  the tracked-path count and its duration, and suggests `corvint index`. Stdout bytes, exit status
  and read-only behaviour are unchanged. Small repositories, snapshot hits and the
  `CORVINT_BENCH_SNAPSHOT_TRACE=1` benchmark run print nothing.
- (b) `docs/DOGFOOD.md` step 1 now runs `corvint index --if-stale` once per fresh checkout before
  the pre-change queries. The lane charter already says so.
- (c) `sourceImports`' generic arm used an alternation regexp scanned from every byte. It now uses
  a keyword-anchored matcher, `genericImports`, with the same captures. The old regexp is kept as
  an oracle. `TestGenericImportsMatchesRegexpOracle` covers an edge-case table, and
  `FuzzGenericImportsMatchesRegexpOracle` ran 60 s (about 500k executions) with no difference. A
  one-off differential over all 11,874 tracked files (4,767 with matches) was identical.

Measurements: no snapshot, same task, packets byte-identical (`cmp`), load average 126 to 161.

| binary | user CPU s | wall s |
| --- | --- | --- |
| base | 6.95, 6.99, 6.79 | 9.09, 8.69, 4.58 |
| this change | 5.60, 5.45, 5.77 | 8.83, 3.41, 3.53 |

User CPU fell about 18%. Under that load, wall time is noise. In the baseline CPU profile,
`sourceImports` took about 9.9%; `worktreePin` (18.8%) and `tokeniseSource` (13%) remain.

## V1-0328: "re-index is a full rebuild writing ~63 MB per tree"

The ticket was marked unverified, so the claim was measured first, with the base binary in a plain
clone:

| run | wall s | user s | max RSS | snapshot bytes |
| --- | --- | --- | --- | --- |
| cold `corvint index` | 4.03 | 6.84 | 1.43 GB | 177,622,402 (9,316 sources) |
| re-index after a one-file commit | 1.92 | 2.38 | 1.29 GB | 177,622,425 |

Verdict: the full-rebuild claim is false. Incremental reuse of verified facts already shipped as
`IDX-SNAP-V0-037` (commit 4a73e8e27). The byte claim is also wrong in the other direction: every
re-index writes a complete snapshot of about 177.6 MB per tree, not 63 MB. Reducing those bytes
needs a delta or shared-segment encoding, which is an on-disk format change under invariant 7.
That is recorded as a residual, and no change lands here.

## V1-1285: "`context --task` answered from a stale index revision"

Reproduction: packet revision `a29258ae` is the tree of commit
1a248337c7c58d4bd817902884934cf127d4d268 (the slice-2 HEAD). It is not a stale commit, because the
packet's `revision` is a tree id. Pinned there, the current binary routes
`docs/specs/session-test-runner-v0.md` as a critical `spec-mentioned` row that "defines
STR-V0-008". The current HEAD routes the same task the same way, so a requirement ID does yield the
spec. The answerability verdict is still `no-specific-terms`, which is a residual.

The real gap was a dirty worktree, whose uncommitted edits were silently not evidence.
`TCP-V0-141` adds one `coverage.uncertainty` line naming the dirty-path count, the commit and its
tree. Observed in this lane: "the worktree has 5 uncommitted paths; ...".

## V1-2207: dogfood prepare/bind `local-outcome NOT_PRODUCED record-index-failed`

Classification: expected and documented, not a bug. It is not a duplicate of V1-1658, although
V1-1658's hidden untracked files are one trigger. `RecordDogfood`
(`internal/tracerecordrepo/adapter.go`) refuses to record an outcome while the worktree is dirty.
Before the sidecar `.corvint/change.cem.json` is committed, that is the expected row
(`docs/DOGFOOD.md` steps 3 and 4, `DCW-V0-015`). The `fix:` hint names the untracked paths
(`DCW-V0-091`) or the uncommitted-sidecar recovery (`internal/dogfoodflow/change.go`). Eight
earlier build-log entries record the same pre-commit row clearing after the commit, for example
`2026-10-12-v1-1276-dogfood-producer-cem-1-0.md`. This lane's own dogfood run is the
same-change evidence: see the outcome below.

## V1-1758: spec index conflicts on adjacent appends

Not started. `INDEX.json` and `README.md` are owned inputs to the generator (`SRG-V0-012`), not
pure projections. Two new specs therefore conflict in content that regeneration cannot rebuild.
That needs a design decision (a union-safe layout, or a resolve command that unions the entries),
which does not fit this lane's budget.

## Evidence, NOT_RUN and rollback

- Tests that fail without the fix: `TestGenericImportsMatchesRegexpOracle`,
  `TestTaskContextDisclosesDirtyWorktreeAgainstCommittedTree` and
  `TestTaskContextFullBuildNoticeNamesBuildAndIndex`.
- `TestTaskContextBuildReportsMissTimeAndTrackedCount` pins the new return value. It also passes
  without the notice, because it tests the timing, not the message.
- `internal/contextindex` was run as a whole package. `cmd/corvint` was run on the context, packet,
  golden, MCP and dirty subsets.
- `TestCEMEndToEndAndGoldenEnvelope` fails locally only because its golden pins `go1.27.2` and this
  host has `go1.27.3`. That is environmental and unrelated.
- NOT_RUN: `make gate` (scoped lane policy) and the full `cmd/corvint` package.
- Rollback: revert the commit. There is no index or snapshot migration, and the analyzer schema
  bump re-pins on revert.
