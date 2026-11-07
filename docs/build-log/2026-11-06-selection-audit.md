# Selection audit: the last 15 merged PRs against a strict lower bound

Date: 2026-11-06
Owner direction: priority 1 is that local and CI runs execute exactly the tests that need to run;
measure how far `main` is from that today (lane selaudit, base `b75053a5b`).
Requirement: AFP-V0-2610 (`docs/specs/affected-plan-selection-v0.md`), the one fix this audit made.

## Method

For each of beamfall/corvint #105 to #120 (the 15 most recently merged; #119 is not among them)
the range is `merge-base(mergeCommit^1, head)..head`. Four selections were compared.

- **CI ran**: every package outcome (`"Action":"pass|fail","Package",…,"Elapsed"`) in the 16
  `go-product-shard` job logs of the PR's `pull_request` CI run. The decisions come from the
  `ci-narrowing-0` artifact: mode, reasons, `driverSelected` (the pinned `corvint-pr-tests`
  driver, which runs `gate-affected-select`), `shadowSelected` (the pinned
  `corvint test --dry-run`), and the base. Run-level `gh run view --log` returned nothing for all
  15 runs, so the logs were read per job. Two shard logs were missed at first (#116 shard 1, #107
  shard 13) and fetched again.
- **corvint test today**: `corvint test --base MB --dry-run --profile ci --confine /bin/true`,
  using the lane's build of `b75053a5b`, run at each PR head.
- **corvint affected today**: `corvint affected --base MB --format json`, the same binary
  (`provider.go.packages`). All 15 plans are `UNKNOWN`, from 68 to 82 `LANGUAGE_FRONTIER`
  entries each.
- **Lower bound (LB)**: packages that have test files and whose test binary
  (`go list -test -deps`) imports a non-test package this change edited, plus packages with an
  edited file of their own. To these are added tests that read a changed non-Go file by a
  package-relative literal (`../..` escapes, `testdata/`) or through `//go:embed`. Repo-relative
  literals in tests were checked one by one: every hit (`docs/specs/INDEX.json`, `Makefile`,
  `.github/workflows/ci.yml` in `internal/tasks/*`, `gateselect`, `cem/workflow`, …) was a fixture
  path in a temporary repository, so none counts. This bound leaves out binary consumers (tests that
  exec a built `corvint`), declared read scopes and directory walkers. The truly necessary set lies
  between the LB and the selections, so "over the LB" is an upper bound on waste, not all of it
  waste.

"pkg-s" is the sum of package `Elapsed` seconds (`-p 4`, retries included). "Shard job-s" is the
sum of the 16 shard jobs' durations.

## Results

| PR | paths | CI mode (reason) | CI ran | driver | shadow (CI) | `corvint test` today | LB | CI/LB | test/LB | CI pkg-s | LB pkg-s | shard job-s |
|---|---:|---|---:|---:|---:|---|---:|---:|---:|---:|---:|---:|
| 120 | 11 | FULL (`.github/` touched) | 399 | 97 | 43 | NARROWED 43 | 3 | 133 | 14 | 18192 | 551 | 8865 |
| 118 | 15 | NARROWED | 102 | 118 | 62 | NARROWED 62 | 4 | 26 | 16 | 6577 | 717 | 6228 |
| 117 | 5 | FULL (`.github/`; driver FALLBACK `UNINDEXED_SOURCE_PATH .github/cishards/slices_test.go`) | 398 | 1 | 440 FULL | FALLBACK 446 | 0 | — | — | 20625 | 0 | 9663 |
| 116 | 63 | NARROWED | 216 | 235 | 145 | NARROWED 145 | 34 | 6.4 | 4.3 | 13326 | 7381 | 8386 |
| 115 | 152 | NARROWED | 252 | 276 | 140 | NARROWED 145 | 97 | 2.6 | 1.5 | 17527 | 12168 | 10061 |
| 114 | 176 | FULL (both FALLBACK `UNINDEXED_SOURCE_PATH cmd/corvint/flow_coverage.go`, 29 such) | 403 | 1 | 445 FULL | FALLBACK 445 | 4 | 101 | 111 | 19231 | 1792 | 9182 |
| 113 | 21 | NARROWED | 200 | 221 | 90 | NARROWED 90 | 19 | 11 | 4.7 | 15017 | 2536 | 9098 |
| 112 | 10 | FULL (`.github/`; FALLBACK `UNINDEXED_SOURCE_PATH .github/macos/shard.py`) | 398 | 1 | 440 FULL | FALLBACK 443 | 2 | 199 | 222 | 18050 | 341 | 8805 |
| 111 | 15 | FULL (`.github/` touched) | 401 | 89 | 48 | NARROWED 48 | 7 | 57 | 6.9 | 18339 | 694 | 9307 |
| 110 | 10 | NARROWED | 193 | 214 | 88 | NARROWED 88 | 23 | 8.4 | 3.8 | 13475 | 2448 | 8513 |
| 109 | 11 | NARROWED | 146 | 166 | 76 | NARROWED 76 | 3 | 49 | 25 | 7060 | 1169 | 6429 |
| 108 | 10 | NARROWED | 181 | 198 | 146 | NARROWED 146 | 94 | 1.9 | 1.6 | 9224 | 4330 | 6963 |
| 107 | 17 | NARROWED | 85 | 101 | 46 | NARROWED 46 | 5 | 17 | 9.2 | 6093 | 166 | 6285 |
| 106 | 3 | NARROWED | 85 | 101 | 54 | NARROWED 54 | 1 | 85 | 54 | 6666 | 15 | 6189 |
| 105 | 232 | FULL (shadow FALLBACK `LANGUAGE_FRONTIER go:unobserved-module:interop/cem01-go:…`; driver omits 173 of its packages) | 401 | 270 | 443 FULL | FALLBACK 443 | 88 | 4.6 | 5.0 | 18577 | 11278 | 9742 |

Over all 15 PRs CI spent 207,978 pkg-s, and 45,586 of them (22%) were on LB packages. 6 of 15 PRs
ran the whole suite. A narrowed run still costs about 6,200 to 10,000 shard job-s: 10 to 13 of its
16 shards run a single package, and each shard has about 130 to 180 s of fixed overhead.

**Under-selection: none found.** No LB package was missing from what CI ran. Five LB packages
were left out of the `corvint test` selection in #115 and #116 (`internal/browsercov`,
`jslinecov`, `linecov/v8`, `nativelinecov`, `rubylinecov` through `internal/linecov`;
`internal/tasks/authority`, `intent` through `internal/tasks/supervisor`). The plan records these
as `goAdditiveExcluded`. In both cases the edited declaration is not reachable from the excluded
package: no excluded package references `linecov.Collect`, and `tasks/transaction` uses only
`supervisor.HostVocabulary`, which does not reach the edited `ownedGroup.drain`. These are sound
exclusions below a package-level bound. Two limits remain: the check is package-level and
literal-based, and it does not cover shell, host-adapter or nested-module suites.

## Causes ranked by wasted CI pkg-s (outside the LB)

1. **The AFP-V0-098 `.github/` guard and FALLBACKs force FULL, about 98,000 pkg-s over 6 PRs.**
   - `.github/` guard alone: #120 and #111. `corvint test` narrowed to 43 and 48 packages and the
     driver to 97 and 89, yet each run spent about 18,000 pkg-s; E∩shadow was 6,187 and 5,837.
     The guard also covers #117 and #112, so fixing their FALLBACK alone does not narrow them.
     **NEW**.
   - `.github/` Go and Python sources are `UNINDEXED_SOURCE_PATH` (#117, #112). Fixed by the
     in-flight branches **claude/fullplan** (AZU-V0-9920 to 9922, bounded `.github` Go/Python
     discovery) and **claude/azu-small** (V1-4688, AZU-V0-005, hidden-directory indexing). Related:
     V1-2056.
   - A deleted or moved non-test Go file of a surviving package (#114, 22 of its 29 entries):
     `gate-affected-select` admits it (CIN-V0-860 (4)) but `corvint test` fell back to FULL.
     This is a local/CI divergence of the V1-4363 kind. **Fixed here** (AFP-V0-2610, below).
   - Deleted TypeScript/JavaScript fixtures under a Go package's `testdata/` (#114, the other 7
     entries) stay `UNINDEXED_SOURCE_PATH` in both the planner and the selector. **NEW**.
   - #105: the shadow refuses a `go:unobserved-module` frontier that the driver admits
     (CIN-V0-860), so the superset guard is FULL; the nested module runs in its own `go-interop`
     job. **NEW** (a local/CI divergence; related to V1-0872).
2. **Shadow-selected packages beyond the LB in narrowed runs: 39,358 pkg-s** (maximum per
   package; the 9 narrowed PRs). By witness: `BINARY_EXEC` 11,511 (by design, since the LB omits
   binary consumers; the largest are `internal/companionrelease`, `internal/cemchain`,
   `tools/corvint-pr-tests`), `PATH_LITERAL_READER` 11,002, `DECLARED_READ_SCOPE` 5,956,
   `UNBOUNDED_READER` 5,402, `DEPENDENCY_PATH` 3,903, `IMPORT_GRAPH_READER` 1,488.
   - Every PR carries a sealed CEM and a build-log entry, and their readers are selected each time.
     `.corvint/changes/*.cem.json` costs 5,000 pkg-s as a `PATH_LITERAL_READER` (10 units:
     `cmd/corvint`, `internal/cem/{corpusbinding,wire}`, `cem2proto`, `cemchain`, `console`,
     `contextindex`, `liveverify/affected{,/golang}`) and 1,419 as a `DEPENDENCY_PATH`.
     `docs/build-log/*.md` costs 1,995 (literal) and 1,678 (unbounded). V1-1820 (completed) bounded
     this once to 8 units and it has grown back. **NEW**.
   - Unbounded readers (`.corvint/test-read-scopes.json` 2,016, `Makefile` 1,130): V1-0719
     (completed ratchet). **claude/selprec2** makes Go reader matching wider on purpose
     (`anyDepth`, `keepsReader` deleted) for soundness, so expect this row to grow, not shrink.
3. **Driver over shadow in narrowed runs: 17,256 pkg-s.** CI runs the selection of
   `gate-affected-select`, not `corvint test`'s. It is a strict superset by 47 to 133 packages
   (#106: 101 against 54; reproduced locally on `b75053a5b` with `tools/corvint-pr-tests --mode
   plan`). The extras come from the selector's own rules: `reader … <- .corvint/changes/*.cem.json`
   and `<- docs/build-log/*.md`, and `unresolved … "--show-toplevel"` (for example
   `cmd/corvint-mcp` through `internal/jstestprovider`). Existing: V1-1315 (cut CI over to
   `corvint test`) and V1-2162 (the `--show-toplevel` literal keeps 62 packages unresolved).
4. **Fixed shard overhead (runner time, not selection): about 2,000 to 2,800 shard job-s per
   narrowed run.** 16 shards are started whatever the selected cost, and 10 to 13 of them run one
   package. **NEW** (optimization).
5. Retries of failed packages (not selection): 2,608 pkg-s across 15 runs, from
   `internal/tasks/cli` (422 and 578 s), `internal/analyzernativebridge`,
   `cmd/corvint-go-test-provider`, `internal/tasks/store`, `internal/modelgateway`,
   `internal/lspstdio` and `internal/update`. Reported only.

claude/lineidx disables line narrowing for 1.0. That changes no package count in this audit.

## Fix: AFP-V0-2610

`seed` (`internal/liveverify/affected/select.go`) now drops `UNINDEXED_SOURCE_PATH` for a
deleted non-test `.go` path under AFP-V0-620's conditions. Before, only `_test.go` paths were
covered. The conditions: only the Go plugin claims the path, it is absent from disk, it is not
under an unobserved directory, and its directory still holds an observed package. `goSourceRule`
already gave that package a `DIRECT_SOURCE_CHANGE` witness, a traversal root, so its importers
were already selected. A deleted file is a `DELETED_FILE` additive fallback, so ADN-V0 narrowing
never applies to it.

Replays use a binary built from this change:

- **One deleted file**: `git rm internal/sharedpath/open_other.go` on `b75053a5b`. `corvint test
  --dry-run --profile ci` goes from
  `FALLBACK … UNINDEXED_SOURCE_PATH internal/sharedpath/open_other.go` (442 packages) to
  `NARROWED 62 execution units`, which include `internal/sharedpath` and its
  importers `cmd/corvint` and `internal/cem/witnesspaths`.
- **PR #114**: `UNINDEXED_SOURCE_PATH` entries drop from 29 to 7, and the FALLBACK now names
  `cmd/corvint/testdata/flows/acceptance/after-review/app/web/profile.js`. That is the remaining
  NEW cause above, so #114 alone would still run FULL.

`make doc-gates` with this change passed `selection-baseline-check`: narrow=33/43, misses=0, floor
narrow>=30. Seven retained ranges were reported
`MOVED … ci FULL -> NARROW (verdict FALLBACK -> run)`:
pr-41, pr-43, pr-46, ci-4a7bf04c8, ci-8db625151, ci-cbc790640 and ci-0df97c8c7. Whether those moves
come from this change is NOT_PRODUCED. The same gate at `b75053a5b` stopped at pr-41 with `corvint
test … exit status 2` and truncated stderr, yet a direct replay of pr-41's `corvint test --dry-run`
gives the same `NARROWED` manifest from both binaries.

Rollback: restore the `_test.go` suffix check in `deletedGoSource`.
