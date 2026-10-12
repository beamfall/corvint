# V1-0040 slice 3: `make gate-affected` on `corvint test`, a CI shadow beside the driver

Date: 2026-10-12. Branch `claude/v1-0040-slice3` from `claude/v1-0040-impl` at `6ecae5897`.
Scope: `docs/specs/session-test-runner-v0.md` rollout steps 2 and 3, STR-V0-012, STR-V0-013 and
STR-V0-015 amended (IDs stable); AFP-V0-011 gains a pointer to the retained rollback. Delivery
stays experimental: `corvint test` decides nothing in CI.

## What changed

- Step 2. `make gate-affected` runs `script/gate-affected-corvint.sh "$(BASE)"`, which builds
  `corvint` from the checkout into a private temporary directory (never `PATH`), runs
  `corvint test --base BASE`, forwards HUP/INT/TERM to it and passes its status through. The
  target no longer reads `GO_TEST_COMMAND`. `script/gate-affected.sh` and its test stay;
  `make gate-affected-test` runs both scripts' tests.
- Step 3. `.github/workflows/ci.yml` `go-product-shard` gains a shadow step before the full Go
  tests: the pinned `corvint test --profile ci --placement universe --shard I/8 --dry-run`
  against the PR base, `continue-on-error`, five-minute timeout, artifact
  `corvint-test-shadow-<shard>` (14 days). It runs only where the driver's pinned build runs
  (`pull_request` with a non-empty `CORVINT_PR_TOOL_SOURCE`) and never decides, executes or
  narrows; `tools/corvint-pr-tests` keeps deciding and the AFP-V0-029 bar stays binding. The
  pinned tools are still built outside the PR checkout by the existing step.
- `CORVINT_PR_FORWARD_EPOCH: "1"` is introduced in the job environment (AFP-V0-030). The advance
  is vacuous: no forward shadow row exists in any epoch, and the trusted pins are empty, so the
  shadow step itself produces zero rows.
- The runner budget is per shard (review note N2): 6 hours locally (70 minutes in the lane's
  commit; raised at integration, below), 60 minutes in the ci profile, one deadline over every invocation; nothing launches after it is spent.
- The ci profile reports every declared language command, make targets included, in `checks`
  instead of planning a declared unit.

## Decisions on the deferred questions

- Partition inputs. The tested checkout's `.github/cishards` JSON, the same bytes the full
  branch's protected helper copies, placed by the algorithm compiled into the pinned binary. A
  change under `.github/` is already an AFP-V0-028 FULL condition, and partition-drift admission
  stays with the driver (`ci-shards --profile`). No embedded copy and no code change.
- CI argv/env constants. Kept in `internal/sessiontest/run.go`, not read from
  `.github/cishards/test-commands.json`: that declaration is the local full suite (no `-race`,
  30m, the local cache), while AFP-V0-028 makes the `ci.yml` full branch the authority.
  `TestSTRV0012CIConstantsMatchFullBranch` holds them equal to `ci.yml`. Per N1 it compares
  against the environment the full branch's `go test` sees (the job `env` over the runner
  environment, which `corvint test` inherits through `os.Environ`), not the driver's closed
  environment. Mutation check: changing one full-branch `-timeout` to 40m, or `CGO_ENABLED` to
  `"0"`, failed the test; `ci.yml` was restored byte-for-byte.
- MAKEFLAGS. Inherited unchanged, as `make go-test`'s own `go test` sees it; CI runs no make. The
  make jobserver file descriptors are not passed through (a recorded limit; its effect is
  `NOT_OBSERVED`).
- The ci profile and `make go-test`. Refused as a launch: the CI full branch runs only Go, and
  `host-adapter-test` is not part of CI PR runs, so a declared command or make target is listed
  as still required.
- Bad BASE. A BASE that does not resolve, or one shaped like an option, is a usage error
  (exit 2) instead of the old full fallback; the run never guesses a base.
- Ordering. The full branch orders affected packages first; `corvint test` does not. Outcomes do
  not depend on order.

## Evidence

- Corvint first: `corvint affected --base 6ecae5897…` rc=0, RUNNABLE, 187 packages; `corvint
  context` used for the call-site route before editing.
- `go test ./internal/sessiontest ./tools/corvint-pr-tests ./tools/ci-reuse-plan
  ./tools/docs-ci-plan ./internal/genesis ./internal/gateselect
  ./internal/liveverify/affected/golang ./tools/test-commands-check ./tools/str-parity` rc=0;
  `.github/testconfine` rc=0; `./internal/contextindex` rc=0; `cmd/corvint -run
  'TestSTR|TestTest|TestSession'` rc=0.
- `script/gate-affected-corvint_test.sh` passes (real `go test` in a fixture repository: clean,
  narrowed, `go.mod` FULL, committed range, failing test, bad and option-shaped BASE, SIGTERM
  forwarding; the forwarding case was mutation-checked).
- `make gate-affected-test` rc=2: the retained `script/gate-affected_test.sh` fails its
  deleted-file case (expects `gate-affected: frontier example.com/fixture/core <- core/core.go`,
  gets a plain selection). The same failure reproduces unchanged at `6ecae5897` and at
  `origin/main` `eb53998e0`, so it predates this slice; the new script's test passes.
- `make -s ci-least-privilege-check` and its test rc=0 with the shadow steps.
- A real `make gate-affected BASE=6ecae5897…` on the uncommitted slice (not a dry run): NARROWED
  236 execution units, then INCOMPLETE (exit 3, make rc 2) when the 70-minute local shard budget
  ran out with 131 of 236 units passed and 103 without an outcome; source drift was also
  reported, because this log and the Makefile comment were edited during the run. Two units
  failed for reasons outside the slice: `conformance/release-artifact-v0`
  TestCorvintHostArchivePartialProof needs a clean worktree, and `internal/liveverify/affected`
  TestSelectionOnTheLiveDirtyWorktree missed its 100ms timing budget (160ms) while other test
  runs loaded the host. Finding: at the declared `-p 1` (the `make go-test` flags), a change that
  reaches about 236 packages does not finish inside 70 minutes locally, so such a change gets
  INCOMPLETE rather than a verdict; the old script had no total budget. The budget value is the
  pre-existing per-invocation one, unchanged for a single Go invocation. Integration review
  (2026-10-12) raised the local budget to 6 hours: a local gate that cannot reach a verdict on a
  broad change is a regression against the script it replaces, the per-package `-timeout 30m`
  still detects hangs, and `-p 1` stays because it is the declared suite's flag. The ci budget
  stays 60 minutes under the job's 75. A full local `-p 1` run's duration is `NOT_OBSERVED`, so
  a change that outruns 6 hours still yields INCOMPLETE.

## Not run or not observed

- The CI shadow step has never run (`NOT_RUN`): the trusted pins are empty, so it is inactive,
  and it needs owner AFP-V0-016 consent for `.github/workflows/ci.yml`.
- AFP-V0-029 shadow rows: zero; the bar is unmet and still binds before any CI narrowing.
- Balanced placement is not retained in CI (`NOT_RUN`); CI retains universe placement only.
- `make gate` was waived by the owner (`NOT_RUN`).

## Integration review (one Codex round)

One read-only Codex round on the slice-3 integration diff (`0927631ce..c204e9262`) raised two P2
findings, both fixed. First, `script/gate-affected-corvint.sh` built `corvint` in the foreground,
so a trapped SIGTERM waited for a hung build; the build now runs as a waited background child and
the signal is forwarded to it (new case in `script/gate-affected-corvint_test.sh`, which fails on
the old script). Second, runner discovery in `Runner.Execute` used the caller's context and its own
two-minute probe bound, so it could outrun the shard budget; probes now run under the shard
deadline and none starts after it (STR-V0-015 text names discovery; `TestSTRV0015BudgetBoundsTheShard`
adds a hung version probe). Under the owner's one-round cap no second round ran.

## Rollback

Point `gate-affected` back at `GO_TEST_COMMAND='$(GO_TEST_COMMAND)' script/gate-affected.sh
"$(BASE)"` and delete the two shadow steps and `CORVINT_PR_FORWARD_EPOCH` from `ci.yml`; no CI
decision changes. Reverting the per-shard budget restores the per-invocation deadline.
