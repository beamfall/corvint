# Decision 0111 — work-queue producer tests that substitute the script share one compiler cache

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls
and record them (orchestration thread, 2026-09-12). The number may be renumbered on merge if
another lane claims 0111.

## Context

`workProductionFixture` (`cmd/corvint/work_materialization_test.go`) clones one seed of the actual
work-queue producer per test and stated that no runtime or build caches are shared. Each
`observeWork` capture therefore runs `script/corvint-work-queue`'s observer-owned branch, which builds
`cmd/corvint-work-queue` with `GOCACHE="$owner/build/cache"` inside a fresh `/tmp/corvint-work-run-*`
root: a cold compile of about 140 packages, stdlib included, on every capture. 29 captures in the
package pay it, 22 of them in the five `TestWorkFinalCheck*` tests. Measured on this host at load
84 on 12 CPUs, that cold build took 17.4 s; the same build from a different directory with the
first build's cache took 2.1 s. `docs/agent-memory/optimizations.md` recorded the shared build as
the remaining lever and as needing an owner call.

## What each test's evidence depends on

The spec text these tests trace to is `docs/specs/work-queue-observation-v0.md` `WQO-V0-004`,
`-005`, `-006`, `-013`, `-014`, `-017`, `-021`, `-025`, `-031`, `-032`, `-033`, `-034` and `-042`.
Only `WQO-V0-005`'s repair paragraph speaks about compilation: the observer owns one private
target, HOME and TMPDIR; same-run temporary compilation may be reused within that tuple; the
standalone script's temporary build owns interruption cleanup. That is production behavior of the
script, and nothing here changes the script or production code.

- The claims that depend on the script's own build are `TestWorkSelfAdapterSourceParity` (the
  run-owned executable exists and is not rebuilt within the run), `TestWorkSelfAdapterStandaloneParity`
  (standalone build scratch is removed), `TestWorkActualProducerInterruptedScratchCleanup` (the
  actual script builds, then the killed producer's scratch belongs to the run root) and
  `TestWorkActualRunnerIntegration` (the unmodified actual script passes the checked runner). They
  run the unmodified actual script.
- The final-check tests (`WQO-V0-021/025/031/032/034`) assert observation state, proposal binding,
  error codes, mutation retention and process-group cleanup around the final `verify`. Their
  fixture already replaces the adapter script with a wrapper. The three `TestObserveWork*` tests
  (`WQO-V0-004/005/013/017/033/042`) assert isolation from caller bytes and ambient environment,
  receipt count and mutation diagnostics; their fixture already prepends lines to the script. None
  of these claims depends on how long the producer compile takes or whether its cache was empty.

## Call

1. A test whose fixture already substitutes the adapter script (`workFinalFixture`,
   `workFixtureScriptPrefix`) may share one Go build cache per package run. The cache is a fresh
   `os.MkdirTemp` directory created on first use and removed in `TestMain`; it is never inherited
   host state. A script prelude links `$owner/build/cache` to it only when an observer-owned tuple
   exists and the run has no build directory yet.
2. The script still runs `go build` on the materialized target for every run. Go keys every cache
   entry by the inputs that produced it, so a fixture with changed Go source recompiles the changed
   packages; the shared cache can change build time, never the built program.
3. What stays per test: the private checkout and Git object store, the observer's run root, HOME,
   TMPDIR and owner tuple, the built executable (every run links its own), and every fixture
   mutation. A test that runs the unmodified actual script keeps a private cold build, as do the
   four tests named above, because their claims are about that script and its build scratch.
4. A shared prebuilt binary was set aside: it would skip the script's build branch entirely and
   need its own staleness key, where a shared compiler cache keeps the build and inherits Go's.

Decision 0060 kept a throwaway `GOCACHE` for the parity runner because its claim is the candidate
binary's identity and a shared cache there would be unrecorded host state. Neither condition holds
here: these tests make no binary-identity claim, and the cache is created empty by the test binary.

## Evidence

Interleaved runs of prebuilt test binaries (`go test -c` at the base and with this change), same
worktree, `-test.count=1 -test.timeout 30m -test.run '^(TestWorkFinalCheck.*|TestObserveWork.*)$'`,
12-CPU host shared with other lanes; load averages are the 1-minute values at start and end.

| run | load start→end | wall | CommandFailures | CaptureBinding | SourceBoundary | InterruptedAdapter | ClosingContext | three ObserveWork |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| before 1 | 148.8→73.5 | 650 s, exit 1 | 123.6 s | 206.7 s | 82.7 s | 45.1 s FAIL | 93.0 s | 98.3 s |
| after 1 | 73.5→25.5 | 504 s, exit 0 | 82.7 s | 212.2 s | 106.0 s | 31.2 s | 30.8 s | 39.5 s |

The before-1 failure is `TestWorkFinalCheckInterruptedAdapter/WQO-V0-034/deadline`, the 5 s
wall clock tracked in `docs/agent-memory/tests.md`; it is not caused by this change. The load fell
sharply across the pair, so the wall-time delta overstates the effect. The per-test split is the
useful reading: the tests with one or two captures (`ObserveWork`, `ClosingContext`) fell by
about two thirds, while `CaptureBinding` and `SourceBoundary` did not fall, so their remaining
cost is per-subcase work other than compilation (fixture clone and commit, materialization,
source qualification, the final verify). A second interleaved pair was still running when this
record was committed.

The product path is unchanged: a real capture still cold-compiles inside the 30 s wall bound
(`docs/agent-memory/bugs.md`, 2026-09-12 work observe entry).

## Amendment 2026-09-12: the test cache is fallback-only

Decision 0130 records the intervening production change from 8a6062e3: the script now selects the
safe per-user `/tmp/corvint-work-queue-gocache-<uid>` before building. The prelude retained by this
decision links `$owner/build/cache` to the package-run test cache, but that path is selected only
when the per-user cache fails the script's ownership, mode, symlink, or repository-boundary checks.
On a normal host the prelude is inert and substituted fixtures use the same per-user cache as the
unmodified script. Call item 1 and the performance interpretation in Evidence are superseded only
to that extent; the fallback remains shared per package run so this decision still covers hosts
where the per-user cache is unusable. The preceding cold-compile/30-second sentence is likewise
superseded by decision 0130's per-user cache and decision 0082's 10-minute hang detector. Product
behavior and the other calls are unchanged.

The Rollback below describes the pre-8a6062e3 base. On the current base, removing the prelude and
its cleanup would remove shared caching only from the run-private fallback; the safe per-user cache
would remain unchanged.

## Rollback

Revert this decision's commit: the prelude helper, its two call sites, the `TestMain` cleanup and
the fixture comment return to the per-capture cold build.
