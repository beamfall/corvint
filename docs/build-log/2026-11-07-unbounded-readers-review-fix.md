<!-- SPDX-License-Identifier: Apache-2.0 -->
# Unbounded readers: the review's four corrections (V1-4283)

Lane `unbrd`, fix round 1 over `2026-11-06-unbounded-readers`, base
`4052b8540b5fff818a819db37b55371ea5428c2c`. The independent review returned `CHANGES_REQUIRED`
with four P1 findings. The governing rule for the round: a missed test is the failure that
matters, over-selection is acceptable, and any uncertainty keeps the broader selection. Each
finding was checked against the code before it was fixed; all four held.

## Findings and corrections

- AFP-V0-2950, joined root (held). After `cwd, _ := os.Getwd()`, the proof checked only the uses of
  `cwd`, so `p := filepath.Join(cwd, "docs")` followed by two `filepath.Dir(p)` calls read a file
  outside the proven `/docs` scope. Now, for each bounded join, the proof follows the joined path
  to its consumers (`boundedJoinConsumed`, identical in the planner and the CI selector). It admits
  only `os.ReadFile`, `os.Stat`, `os.Lstat` and `os.ReadDir`, directly or through one local
  variable that is never rebound, addressed or passed elsewhere. `os.Open` is refused because the
  file's `Name` returns the path, and so is a walk, because its callback receives the path.
- AFP-V0-2951, command boundary (held). The first version stopped every rule (d) chain at a
  `package main` unit. A make target that runs a repository-walking command was then selected only
  when its recipe or the command changed. Commands are root locators again. The driver now records
  the package patterns of `go vet` and `go list` as `buildPathImports`, because those commands run
  no code and leave no binary. The graph keeps an edge that only those patterns name, with no path
  import, declared import or declared caller behind it, out of the rule (d) walk. `go build` stays
  an execution edge: a target that depends on it may run the binary (`check: build-tool` then
  `bin/tool check`). So do `go install`, `go run`, `go test` and `go generate`.
- AFP-V0-2952, line-broken literal (held). A line break did not prove a literal was text:
  `strings.TrimSpace(fmt.Sprintf("../config%s\n", suffix))` opens `../config.go`. A literal now
  counts as text only when it is passed directly to a standard-stream print: `fmt.Print`,
  `fmt.Printf`, `fmt.Println`, `fmt.Fprint*` or `io.WriteString` writing to `os.Stdout` or
  `os.Stderr`, or `os.Stdout`/`os.Stderr.WriteString` (`outputOnlyLiterals`, identical in both
  analyzers). The two repository files the first version exempted give no such evidence:
  `internal/perfbudget` formats with `fmt.Appendf`, and `tools/cem-stable-regen` builds its table
  in a `strings.Builder` it returns. Both keep rule (d).
- AFP-V0-2953, environment-named source (held). A file that never assigns `ENVFILE` proves nothing
  about where its caller's `. "$ENVFILE"` points. Every source that neither a literal nor a fresh
  directory resolves is dynamic again. `script/heavy.sh`, whose remote half runs
  `. "${ENVFILE/#\~/$HOME}"`, is back on the floor, and so is every unit that runs it.
  `script/dogfood-cache_test.sh` reaches heavy.sh through the dogfood wrappers, so TSP-V0-950
  refuses its read scope. The declaration is removed and the test is recorded again (AFP-V0-2955).

AFP-V0-2954 (same-valued reassignment, `dogfood-finish.sh`) and the
`qualify-tasks-basic_test.sh` read scope were not challenged and stay.

## Measured plans

`corvint affected --base <head> --format json` was run on a scratch commit that edits only the
named file, using the lane binary built from each head. The scratch commits lived in a temporary
detached worktree, since removed. The columns count the selected units, the ones with
`witness.anyDirtyPath`, and the distinct `witness.cause.unit` values.

| Change | Base | First version (`777379fa5`) | Review fix |
|---|---|---|---|
| (a) `.github/workflows/ci.yml` | 265 / 81 / 26 | 252 / 20 / 18 | 264 / 80 / 24 |
| (b) `script/heavy.sh` | 295 / 12 / 12 | 295 / 10 / 10 | 295 / 11 / 11 |
| (c) `docs/TICKET-PRIORITY.md` | 253 / 117 / 30 | 240 / 55 / 21 | 252 / 113 / 28 |
| (c') `docs/DOGFOOD.md` | 262 / 38 / 22 | 259 / 18 / 17 | 261 / 37 / 21 |

Whole Go packages selected: 29, 48, 25 and 31, the same as at base. The doc-only change (c) now
selects 113 unbounded units, by cause:

- 40 `script/heavy.sh` (`shell:dynamic-source`), up from 32 at base: `dogfood-finish.sh` no longer
  has a dynamic source of its own, so its runners now report heavy.sh as their cause.
- 21 `benchmarks/runner` (25 at base) and 19 `cmd/corvint`. The four fewer runner units are
  `go vet`/`go list` targets.
- 5 `tools/cem-stable-regen`, 2 `internal/perfbudget`, 2 `cmd/corvint-tasks`, and 1 each for the
  recorded single packages and the two shell tests.

The Go plugin's per-unit reasons and root-locator flags over the repository are now identical to
the base's. The first version's join proof (AFP-V0-2950) never took a repository package off the
floor, so its correction costs no selection here. The returned selection comes from AFP-V0-2951,
AFP-V0-2952 and AFP-V0-2953, and the ticket's targets for (a) and (c) are not met. The remaining
floor is genuine under the review's rule: each unit has a recorded witness, and none of the
removed proofs could be made sound without new evidence (see Limits).

`make unbounded-readers-check` passes with the ten heavy.sh-reached units recorded again. Compared
with base, the record only moves `qualify-tasks-basic_test.sh` to its declaration and rewords
three reasons. `make selection-baseline-check` passes: narrow 33/43 (floor 30), 0 misses over 26
failing packages of 15 failed CI runs, the same seven changes moved from `FULL` to `NARROW`.

## Fault injection

Each correction has a regression test that fails with the first version restored:

- AFP-V0-2950: making `boundedJoinConsumed` accept every consumer drops `widened` (the review's
  case), `joinreturn`, `opener`, `rejoined` and `handed` from rule (d) in
  `TestBoundedJoinLocatorIsNoRead_AFPV02950` and `TestBoundedJoinLocatorParity_AFPV02950`.
- AFP-V0-2951: restoring the `!command` guard leaves `run-tool`, `build-tool`, `check` and
  `vet-and-run` unselected in `TestCommandLocatorReachesItsRunners_AFPV02951`. Before the scanner
  stopped recording build-only patterns as generic path words, the same test selected the
  `go vet`/`go list` targets, so both directions are covered (`TestDriverBuildOnlyPatterns_AFPV02951`).
- AFP-V0-2952: the line-break-only rule exempts `prose`, `template`, `formatted` and `buffered` in
  `TestLineBrokenClimbIsText_AFPV02952`, and the planner then fails to select the formatted reader
  for an edit of `config.go`. The CI selector's `TestLineBrokenClimbParity_AFPV02952` fails the
  same way.
- AFP-V0-2953: the first version's shell plugin leaves five environment-named forms bounded in
  `TestEnvironmentNamedSourceStaysDynamic_AFPV02953`, and a test that holds only `. "$ENVFILE"`
  goes unselected by an edit of `lib.sh`.

## Limits

- heavy.sh's floor is real by the analyzer's standard: the file it sources is operator
  configuration on the remote host, and no repository text bounds it. Taking heavy.sh off the
  floor needs evidence the review would accept: an explicit project declaration that the
  environment-named source reads no repository file, or a heavy.sh change that sources the file
  only from a provably external location. Either is a new contract for the owner to decide, and
  neither is attempted here.
- `go build` and `go install` stay execution edges. Tracking where the built binary goes and
  whether a step runs it would narrow make targets that only build, but that tracking is not
  modelled.
- Text proven only by stdout or stderr sinks leaves buffered and returned text, including
  `cem-stable-regen`'s table, on rule (d). An interprocedural writer proof would be needed to
  admit it.
