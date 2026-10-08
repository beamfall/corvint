# 2026-10-08: Batch F integration (V1-0597/0613, V1-0608, V1-0860, V1-1021, V1-0861, V1-0466)

## Intent

Integrate six finished, independently reviewed lanes as one batch branch,
`claude/batch-f-2026-10-08`, on top of batch E (`claude/batch-e-2026-10-08` head `0f650720`, PR
#683), because batch E allocates `TRE-V0-024..028`. Each lane is merged with `git merge --no-ff`, so
its reviewed commits stay intact. The integration adds requirement renumbering, conflict resolution
and one executor composition between two lanes that both retire detached descendants.

## Merges

| Order | Lane branch | Lane head | Renumber commit | Merge commit | Conflicts |
| --- | --- | --- | --- | --- | --- |
| 1 | `claude/v1-0597-0613-swiftpm` | `d62c96df` | `aa593dbd` | `32b7f0c8` | spec, `REQUIREMENTS.tsv`, `cmd/corvint-test-runner/main.go`, `internal/testrunner/execution.go` |
| 2 | `claude/v1-0608-playwright-reap` | `b3b05237` | `48389f61` | `b572b912` | spec, `REQUIREMENTS.tsv`, `internal/testrunner/execute_unix.go`; build-only collisions in `internal/groupreap` |
| 3 | `claude/v1-0860-jasmine-profile` | `a0d18975` | none | `6f150c13` | spec, `REQUIREMENTS.tsv`, `internal/testrunner/dynamic/README.md` |
| 4 | `claude/v1-1021-gh679-empty-effects-warning` | `a4224768` | none | `14af74ad` | none |
| 5 | `claude/v1-0861-boost-test-profile` | `567a8afe` | none | `6e7ac81c` | spec, `REQUIREMENTS.tsv` |
| 6 | `claude/v1-0466-stage-receipt-classes` | `948b9c1e` | none | `c29ecaec` | `REQUIREMENTS.tsv` |

## Requirement renumbering

| Lane | Ticket | Lane ID | Integrated ID |
| --- | --- | --- | --- |
| SwiftPM XCTest transport | V1-0597 | `TRE-V0-024` | `TRE-V0-029` |
| SwiftPM detached-descendant retirement | V1-0613 | `TRE-V0-025` | `TRE-V0-030` |
| Escaped-descendant containment | V1-0608 | `TRE-V0-024` | `TRE-V0-034` |
| Escaped-descendant containment | V1-0608 | `PGO-V0-007` | unchanged |
| Jasmine profile | V1-0860 | `TRE-V0-031..033` | unchanged |
| Boost.Test JUnit profile | V1-0861 | `TRE-V0-036..039` | unchanged |
| Empty-effects warning | V1-1021 | `CAL-V0-192/193` | unchanged |
| Stage receipt classes | V1-0466 | `CAL-V0-027` amendment | unchanged |

`TRE-V0-035` stays reserved. Each renumbering is one commit on top of the lane head, made before the
merge, rewriting every lane-introduced occurrence (spec bullets and traceability rows, lane
build-log entry, code comments, test comments and failure messages). No reference to
`TRE-V0-024..039` existed at either lane base, so every rewritten occurrence was lane-owned. No test
function name encoded a TRE ID.

## Conflict resolutions

- `docs/specs/test-runner-execution-v0.md`: lanes appending sections were kept in merge order. The
  runner inventory now names Boost.Test, the concrete count is 22 dynamic (Jasmine) and 18 native
  (Boost.Test), and the digest counts 59 concrete profiles. Diagnostic-table rows from batch E were
  kept; the `process-containment` row was added; `parse.go` and `execute_unix.go` line citations
  were re-anchored to the line whose content still matches the recorded anchor.
- `internal/testrunner/dynamic/README.md`: both the Jasmine shim provenance and batch E's Mocha
  selection sentence are kept.
- `cmd/corvint-test-runner/main.go`, `internal/testrunner/execution.go`: both sides kept.
- `docs/specs/REQUIREMENTS.tsv`: generated; regenerated with `make -s spec-requirements`.
- `internal/groupreap`: V1-0613 and V1-0608 each declared the Darwin `kinfo_proc` offsets
  `kinfoStat`, `kinfoPID`, `kinfoPPID` and `kernProc` with equal values; `retire_darwin.go` now uses
  the `proctable_darwin.go` declarations. The V1-0613 test helper `alive(pid)` is renamed `pidAlive`.
- `internal/groupreap/escape.go`, `internal/testrunner/execute_unix.go`: V1-0613 ran a phase through
  `RunRetiring` (token/ancestry retirement, opt-in per plan, Darwin only) and V1-0608 through
  `RunContained` (structural retirement, every phase). New `RunContainedRetiring(cmd, r)` keeps
  `RunContained` exactly for `r == nil`; with a retirer it records the leader after start and calls
  `Retire` after the leader exits and before it is reaped, ahead of the structural pass, so the
  `TRE-V0-030` record is taken first. Non-graceful cancellation runs `Retire` while the leader is
  live, then signals only the leader as `TRE-V0-034` requires. `RunRetiring` stays for its tests.
- Independent review (Codex, read-only) found one P1 in that composition: after a normal leader
  exit the Retirer could kill a token-bearing parent whose child had no token and had started after
  the last 200 ms sample, orphaning the child out of the structural tree while containment still
  reported complete. `RunContainedRetiring` now freezes the owned tree (`freezeOwned`, extracted
  from `retireEscaped`) and records its escaped identities before `Retire`, so such an orphan is
  retired by its frozen identity. `TestRunContainedRetiringKeepsUnprovenOrphan` (Darwin) failed
  before the fix (`helper=true`, containment `Err:<nil>` with no survivors) and passes after,
  five repeated runs included; a run-time sampler seam (`sampleProcesses`) makes the missed sample
  deterministic. A read-only re-review of that fix found one P1 on its failure path: a freeze that
  failed after stopping a tokenless descendant recorded nothing, so the Retirer could still orphan it
  and leave it suspended. `freezeOwned` now reports each individually stopped identity as it is
  stopped, which is recorded for identity retirement, and an incomplete freeze or record (error or
  identity bound) runs the structural pass before the Retirer, as `RunContained` does.
  `TestFreezeOwnedReportsStoppedBeforeFailure` injects a table failure after one stop and requires
  that identity to be reported and tracked; it is new API, so its before-state is a build failure,
  not a behavioural failure.
- Semantic test conflict: V1-0613's `TestExecuteRetiresDetachedDescendants` (`timeout-retire=false`)
  and the opt-in live `TestSwiftPMXCTestLiveDetachedTeardown/contained-only` asserted that detached
  descendants survive without the retirement flag. `TRE-V0-034` now retires them structurally on
  every phase, so the integrated build failed the first (`detached pid 42322 alive=false`). Both now
  assert no survivors and no `retirement` report for the unflagged plan; the receipt shape check is
  unchanged.

## Verification

- Focused packages (Darwin arm64, `-p 1 -count=1`): `cmd/corvint-test-runner`, `internal/groupreap`,
  `internal/tcq`, `internal/testrunner/...`, `internal/tasks/{cli,mutation,snapshot,transaction}` pass
  after the semantic test fix; `internal/testrunner` failed before it as recorded above.
- After the review fix, `internal/groupreap`, `internal/testrunner` and `cmd/corvint-test-runner`
  pass again, and the live SwiftPM witnesses below pass again.
- Live SwiftPM 6.4 (`/usr/bin/swift`, lane fixture copy): `TestSwiftPMXCTestLiveThreeOutcomes` and
  `TestSwiftPMXCTestLiveDetachedTeardown` (contained-only, timeout, interrupt) pass on the integrated
  tree; contained-only left no survivors, and timeout and interrupt each retired xctest and helper by
  ancestry.
- Doc gates, `go test ./internal/specindex`, `go vet` (darwin and linux) and `gofmt` are clean.
- `TestRetirerRetiresDetachedDescendants` failed once in a full `internal/groupreap` run ("owner
  token unreadable for pid N: invalid argument" for an unrelated same-uid process) and did not
  reproduce in later full or targeted runs; it is retained as a suspected host-race flake.
- NOT_RUN: `make gate`, `go test ./...`, Linux execution of the groupreap and executor tests, and
  live Playwright, Jasmine and Boost.Test qualification of the integrated tree (each lane recorded
  its own).

## Rollback

Revert the merge commits individually; each lane's own rollback section applies. Reverting the
V1-0608 merge also requires restoring `RunRetiring` in `execute_unix.go`.
