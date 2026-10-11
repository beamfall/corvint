# V1-0726 — Dispatch progress accounting

Base: `32bfc85a44fd82b5793c7e30b2231200e16c025a` (`origin/main` at task start).
Intent: CAL-V0-057 cooldown amendment **proposed, pending owner acceptance (V1-0726)**.
No new requirement ID or decision file is allocated. The existing busy guard and sticky-tier
implementation were present; LOW-A remained reproducible. The activity-path case already rejected
the configuration, but had no named subtest or specific refusal assertion.

## Change and rollback

Known progress outside a session resets a cooling key's `NoProgress` count while retaining its
cooldown deadline. Parking therefore requires `parkAfter` new sessions without progress after the
reset. Parked keys still release their backoff, sticky roles keep their reached tier, and a worker
in the ledger defers progress accounting to `finish`. Operator unpark behavior is unchanged.
Rollback removes the cooldown count reset; it restores LOW-A without changing the ledger format.

## Evidence

Focused race tests cover the live-worker ledger guard, sticky tiers after between-session progress,
cooldown preservation and fresh parking evidence, and model-only activity-path refusal. Fixtures
call the real `unpark` and `finish` paths without process discovery. The cooldown regression failed
before the fix with `NoProgress:1` after progress. Mutation runs use Go overlays, leaving the
working source intact: remove the busy guard, erase sticky tiers on progress, omit the cooldown
count reset, omit between-session ladder accounting, and accept activity-path-only model rendering.
Results and final documentation checks are retained in `/private/tmp/v1-0726-evidence/`.

Final outcomes:

- Focused `go test -race -count=1 -timeout 30m ./internal/tasks/dispatch` passed for
  `TestCALV0052_EscalationConfigRefusesUnsupportedModels` and the three new CAL-V0-057 tests,
  using `GOTOOLCHAIN=local TMPDIR=/private/tmp/claude-501/gotmp GOMAXPROCS=3`.
- All five overlay mutations failed at their intended behavioral assertions. An initial sticky-tier
  mutant did not compile because it left an unused variable; it was corrected and rerun, and the
  compilation failure is not counted as mutation evidence.
- The independent read-only reviewer reported no findings on the source, regression tests or
  proposed contract amendment. Review excluded process qualification and did not run tests.
- `make -s` checks passed: `spec-requirements-check`, `requirement-definitions-check`,
  `traceability-tests-check`, `decision-numbers-check`, `line-citations-check`,
  `error-code-ownership-check`, `unbounded-readers-check`, `use-case-receipts-check`, and
  `use-case-receipts-test`. Explicit `sh script/repin-use-case-receipts.sh --check` passed:
  all 18 receipts current; receipt-script tests passed five cases.
- The spec was staged in a private task index/object directory before `make -s spec-requirements`;
  the shared Git index could not be written under the sandbox. Documentation checks used that
  index, plus a task-local `mktemp` shim routing its no-template default to the authorized TMPDIR.
  The first generator attempt hit macOS's denied default temp directory; the adapted run passed.
  No permanent tooling change was made. No full gate or repository-wide Go tests were run.

Corvint query and tracked-path impact were used before edits; affected plans were used before
tests. Query omitted four ranked results and withheld two test candidates; impact omitted 96
results, including callers. Affected advice retains language frontiers and the unowned spec path;
its full-gate suggestions do not override the owner's scoped test instructions. These receipts
are context advice, not proof of behavior. Frozen retrieval/learning evaluations are not applicable
to this dispatch accounting change.

## Limits

The original process-based CAL-V0-052/055/057 baseline failed because the sandbox denies
`/bin/ps`: supervision cannot finish-account workers. It is not reported as passing integration.
The requested deterministic regression tests still exercise accounting and are run with `-race`.
`make dogfood-change` failed writing the shared Git worktree directory outside the sandbox;
CEM/OCM binding and sealing are NOT_PRODUCED. Committing, publishing and native ticket completion
are excluded by the owner's explicit request; `.taskman` is unchanged.

Installed tools: Core `1.0.0-rc.3` build 407; Tasks
`0.0.0-tcp01-unverified+build.407`. Official release reads found Core `v1.0.0-rc.2` and Tasks
`tasks-dev-20260929.2` as the newest applicable published channels. Both updater checks failed
their version smoke with `descendant snapshot unavailable`; freshness and local qualification
remain UNKNOWN, and no downgrade or installation was attempted. Account usage telemetry was
unavailable through the exposed tools; no spend enforcement or savings claim is made.
