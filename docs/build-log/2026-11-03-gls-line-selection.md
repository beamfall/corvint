# Go line selection: implementation and retained limits

Tickets: V1-4364, V1-4365, V1-4366. Base:
`6f12f4704235dc8f9bea19fc12995bfb68c746c3`. Measurements observed 2026-11-04.
Implementation: `86f307dddde2ad20403b70c08fafc3a09955955b`; the corrected real-index
budget workload is committed at `7f05005d7`. Intent remains accepted; delivery remains
experimental. This entry records no promotion or qualification.

## Implementation and proving checks

- V1-4364: `GLS-V0-001..003`, `008..016` refine `LPCV-V0-950..954`, `956`, `958`,
  `961..962`. The immutable `/4` index reads only touched sections, with digest verification.
  The collector retains subprocess coverage and discards incomplete process trees, failed and
  timed-out runs. Declaration/helper/new-file edits, missing records and in-order uncertainty
  retain the static scope. `TestCompactRoundTrip`, `TestCompactDamage`, `TestCollectParts`,
  `TestCompactProfile`, `TestCollectorProcessGroup`, `TestFaultInjection`, `TestDecide`,
  `TestHeadTestNames` and the affected CLI tests pass. `TestFaultInjection` changes a covered
  function, observes its selected test fail, and exercises an unexecuted-body negative control;
  the order-sensitive fixture passes in both orders and retains both paths.
- V1-4365: `GLS-V0-030..034`, refining `LPCV-V0-963`. The seeded harness implements
  deletion, negation and constant changes, cycles case/else-if/loop/closure/statement sites,
  runs every suite package individually, refuses a failing baseline and retains every miss.
  `TestQualifyFixtureHasNoMiss`, `TestQualifyReportsASeededMiss`,
  `TestQualifyRefusesFailingBaseline` and `TestSummarizeAndQualified` pass. The intentionally
  missing coverage edge is detected. Real-tree qualification is externally blocked below.
- V1-4366: `GLS-V0-020`, `040..047`, `050`, refining `LPCV-V0-957`, `959`, `961` and
  the `LPCV-V0-020` miss kill switch. The same decision function feeds affected, local test
  manifests and PR/merge-queue lists. CI requires matching qualification and a retained proof
  before narrowing; missing evidence retains the static lists. Applied exclusions alone enter
  miss detection; confirmation requires the exact landed range and a passing base plus two
  failing head runs. `TestCorvintTestLineIndexNarrowsTheRun`,
  `TestCISelectionRewritesWholePackages`, `TestMissFindsExcludedFailures`, the session-test
  suite and the CI reuse/forward-row suites pass. Three workflow YAML documents and 55 shell
  steps parse. Hosted execution is NOT_RUN: this lane cannot publish.

The focused runs are one package per command through `script/heavy.sh`, with local Go 1.27.3,
`-count=1 -timeout 30m`. The complete `internal/linecov` suite passes in 12.321 s;
`tools/linecov` in 19.703 s; the focused affected/test/use-case/help CLI set in 7.662 s;
`internal/sessiontest` in 3.726 s; `tools/ci-reuse-plan` in 8.074 s; and
`tools/ci-forward-row` in 42.583 s. Full command logs are retained privately under
`../lineidx-evidence/final/`. Frozen line-selection fixtures and the budget are the applicable
frozen checks; unrelated language, ranking and learning evaluations are NOT_RUN, outside this
slice. Independent review is orchestrator-owned and NOT_PRODUCED by this lane.

## Latency and index size (GLS-V0-003; LPCV-V0-958)

The retained index is pinned to `23368e8dda547c6d58b0681aa2dd116eda592da7`, with
17,885,718 bytes (17.06 MiB), below the 64 MiB minimum bound. It has records in 386 packages;
it is not a completely collected module. Missing/unordered packages remain whole. Previous
collector chunks that timed out or crashed remain visible in the private collection logs.

`TestLazyPlanBudget` now fills gaps left by ineligible candidate files and refuses a real index
with fewer than 50 eligible files. Before that repair the real-index test silently used only
38 files (p95 25.087 ms); that run does not prove the 50-file target. The corrected run selects
for exactly 50 changed files and 386 units, declaring 12,922 tests: p50 27.812 ms,
p95 43.444 ms, 501 kept by coverage and 8,137 excluded. This is a synthetic body-edit workload
on real indexed content, excluding Git/blob/head-file reads as required. It is a Linux host
observation under concurrent lane load, not CI/Darwin performance evidence or a claim that the
real PR sample excludes that many tests.

## Twenty merged PRs (GLS-V0-050)

One build and one index were used on twenty consecutive first-parent merged PRs ending at the
lane base. Each range uses its real first parent; no source file was filtered from a range.
The static `plan`, `provider` and `advice` match with and without `--line-index` for every range.
The retained JSON binds the binary/index SHA-256, index commit, every base/head, counts,
fallback categories and load/selection/Git times:
[measurement](../../.agent-evidence/lineidx-pr-measurement.json).

Counts below are planned distinct top-level Test/Example/Fuzz names under all build constraints,
not measured executions. Non-Go tests, vet and time saved are outside these counts. The result
is **153,050 static versus 153,050 line-selected names: zero reduction**. Nine ranges fall back
on unresolved static scope (including unselected binary consumers and unowned workflow paths);
eleven are analysed but all reached packages require static fallback. This sample does not
establish the owner's desired reduction.

| PR | Static Go packages | Static names | Line-selected names | Fallback reason/categories |
|---|---:|---:|---:|---|
| #88 | 123 | 8254 | 8254 | static-scope-unknown |
| #87 | 150 | 9361 | 9361 | static-scope-unknown |
| #85 | 151 | 9786 | 9786 | static-scope-unknown |
| #86 | 271 | 12565 | 12565 | static-scope-unknown |
| #84 | 148 | 9274 | 9274 | static-scope-unknown |
| #83 | 267 | 12537 | 12537 | static-scope-unknown |
| #82 | 250 | 12248 | 12248 | static-scope-unknown |
| #81 | 56 | 5491 | 5491 | non-go, uninstrumented |
| #80 | 245 | 12214 | 12214 | added, non-go |
| #79 | 34 | 4910 | 4910 | non-go |
| #78 | 30 | 4470 | 4470 | non-go |
| #77 | 57 | 5492 | 5492 | non-go, uninstrumented |
| #76 | 57 | 5535 | 5535 | added, non-go |
| #75 | 32 | 4858 | 4858 | non-go |
| #74 | 70 | 5841 | 5841 | declaration, non-go |
| #73 | 42 | 5122 | 5122 | non-go, test-helper |
| #72 | 72 | 5903 | 5903 | declaration, non-go, test-helper |
| #71 | 130 | 8615 | 8615 | static-scope-unknown |
| #69 | 66 | 5608 | 5608 | static-scope-unknown |
| #70 | 50 | 4966 | 4966 | declaration |

## Qualification failure and enablement boundary

The bounded real-tree attempt used seed `20261103`, five requested valid mutants, one requested
control, 1,129 candidate sites in `internal/gateselect`, and all 426 root-module packages as the
suite, at the index's exact commit. The clean baseline passed the five benchmark packages and
failed 45 tests in `cmd/corvint` after 272.039 s. It executed **zero mutants and zero controls**;
zero observed misses here is not a passing qualification. The failed summary and original report
digest are retained in [blocked qualification](../../.agent-evidence/lineidx-qualification-blocked.json).

Focused reproductions on the unchanged commit show the sandbox's synthetic read-only `.git`
under `/home/russell/.cache` turns a non-repository fixture into a child of that repository, and
its `/` ownership (`nobody`) is rejected by the work-runner ancestor check. These are host
blockers outside the lane's writable authority; no gate, ownership check or fixture was relaxed.
The experimental executable also had `vcs.modified=true` because the CEM was prepared: it cannot
produce a source-bound qualification record. Rerun with a clean VCS-stamped executable, a newly
collected index and a passing baseline on the qualification host.

The Go family remains **unqualified**, with no `.github/linecov/qualification.json`; CI retains
its static plan even if the repository variable is set. The required Beamfall core/plugin Go
corpus qualification is NOT_RUN on this restricted host. Other language corpus families are
outside this Go lane and remain unqualified here; no cross-language delivery is claimed.
`LPCV-V0-960` watch streaming and per-selected-run record refresh are not delivered by this slice.
The local integration is `corvint affected` / `corvint test --line-index`; no
`corvint-test-selected` executable exists in this tree or the installed PATH.

## Decisions, rollback and retained friction

Keep the companion spec because the affected-plan contract is at its obligation cap. Do not
weaken static unknowns to improve the measurements. Keep CI narrowing off and the implementation
experimental until the real-tree and pinned Go-corpus qualification and independent review pass.
Rollback is removal of `--line-index` locally and leaving the repository switch off in CI;
index removal loses only selection performance. Workflow changes remain in separate commits.

Git operations succeed but their hooks also try the forbidden shared metadata path
`/home/russell/projects/corvint/.git/packed-refs.lock`, even with automatic maintenance disabled.
No common metadata configuration was changed. Disposable qualification/measurement metadata lives
under the permitted `.git/worktrees/lineidx/corvint/` subtree. The earlier interrupted measurement
had left the lane detached; its clean delivery branch was restored before continuing.

The allocated AFP range cannot be checked by `requirement-ids-free` because that helper limits
IDs to 0..999; the new GLS prefix is confirmed free at origin/main. The catalog check's make
recipe invokes multiple Go packages directly; its successful result is retained, with that
invocation-policy discrepancy recorded. Subsequent focused test commands use the required wrapper.
The full repository gate is NOT_RUN by the charter; focused passes do not imply equivalent coverage.
Ticket-store writes, publication, seal and independent review are reserved to the orchestrator.

Terminal checks at the source of `7f05005d7`: `make doc-gates` PASS (354.1 s),
`make line-citations-check` PASS, changed-file `gofmt -l` empty, `git diff --check` PASS,
and `make windows-vet` PASS for both root and interop modules (5.5 s). The newly staged
record files also pass the focused path/blob/license checks. The existing static-selection
frozen baseline passes its own floor (33/43 merged ranges, zero misses over 26 failing
packages); it is separate from this entry's twenty-range line-selection comparison.
