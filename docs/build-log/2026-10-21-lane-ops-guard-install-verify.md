# 2026-10-21 lane-ops: verify refusal, heredoc-safe lane guard, bounded local installs

Tickets: V1-1657 (`DCW-V0-092`), V1-1953 (`ASD-V0-024`), V1-1979 (`ASD-V0-025`). V1-1627..V1-1632
and V1-1546 were checked against main as already delivered. The requirements are accepted under
the owner's standing auto-accept (2026-10-11).

## Problem

- V1-1657: `corvint dogfood finish-lane` accepted a verify file whose commands carry quotes (for
  example a quoted `-run` regex). It ran prepare, bind and commit first, and only the local outcome
  step refused with `unsupported-verify-syntax`, so one finish attempt was lost per mistake.
- V1-1953: the lane hook that refuses polling loops and unwrapped `go test` flattens the whole
  command, so a heredoc that writes a file containing `while ... sleep` is refused. This lane hit
  it again while editing a spec with an inline script.
- V1-1979: local tool installs left an unbounded set of ad-hoc `.bak` copies (21 in
  `~/.local/bin`); `corvint-update` itself is already bounded by `UPD-V0-007`.

## Decisions

- `DCW-V0-092`: finish-lane validates the verify input (file readability, size bound and
  `trace.ValidateVerificationCommands`) before any step and refuses with exit 2, the reason, the
  validator detail and a `fix:` line. `localOutcome` shares the same helpers so both paths name the
  same reasons.
- `ASD-V0-024`: `script/lane-guard.sh` is an in-repo replacement for the machine hook, in perl with
  core `JSON::PP` (no jq). It drops heredoc bodies before matching and accepts either `heavy.sh`
  path, so it works before and after the semaphore cutover. Installing it in Claude Code settings
  is the operator's step; this lane does not edit `~/.claude`.
- `ASD-V0-025`: `script/install-local-bin.sh` renames a sibling copy over the target (avoiding the
  in-place overwrite that SIGKILLs a running binary), keeps one rollback by default and prunes only
  its own `corvint-prev-<n>` names. Existing `.bak*` files are left for the operator.

## Already delivered (evidence)

- V1-1627..V1-1632: `docs/LANE-ORCHESTRATION.md` sections 1-7, `script/heavy.sh`,
  `script/lane-idle-check.sh`, `script/lane-clean.sh`, the `dogfood-finish` input refusal,
  `ASD-V0-013..018`, `make lane-scripts-test` and
  `docs/build-log/2026-10-18-v1-1627-lane-orchestration.md`. Residuals: the batch measurements in
  section 7 stay `NOT_PRODUCED`, and lanes still run the `/private/tmp` heavy.sh prototype; the
  cutover to `script/heavy.sh` is the orchestrator's.
- V1-1546: `DCW-V0-073` (V1-1786), `internal/dogfoodflow/batch_dropped.go`, its `batch_test.go`
  case, and the `docs/DOGFOOD.md` rule against `-X ours`/`-X theirs`.

## Evidence

- `TestFinishLaneDCWV0092RefusesVerifySyntaxBeforePrepare` passes; with the new call removed it
  fails. `-run 'TestFinishLane|Verify|Outcome' ./internal/dogfoodflow/` and
  `-run TestDogfoodFinishLane ./cmd/corvint/` pass.
- `make lane-scripts-test`: 9 groups pass. The guard group fails against the old flattening
  behaviour on the heredoc case, and the install group fails with pruning disabled.

## NOT_RUN

`make gate`, whole-package Go tests and the live hook install (operator-owned). V1-1980, V1-1724,
V1-1651, V1-1645 and the Tasks tickets V1-1895, V1-2119, V1-0826, V1-0549 and V1-0580 were not started
within the lane budget.

## Rollback

Revert this change: the finish-lane pre-check, the two scripts, their test groups, the guide
paragraphs and `DCW-V0-092`/`ASD-V0-024..025`. Lanes return to the outcome-step refusal, the
machine hook and manual installs.
