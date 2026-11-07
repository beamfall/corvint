<!-- SPDX-License-Identifier: Apache-2.0 -->
# Root locators, text literals and runtime sources are not reads (V1-4283)

Lane `unbrd`, base `4052b8540b5fff818a819db37b55371ea5428c2c`. Owner goal: every run executes only
the tests the change can affect, and never fewer. Ticket V1-4283 measured that units marked
`UNBOUNDED_READER` were selected for every change through 26 causes, most of them a located root
that no code read from and two shell scripts whose sources the plugin could not place.

## What changed

Requirements AFP-V0-2950 to AFP-V0-2955 in `docs/specs/affected-plan-selection-v0.md` (the
closing sentence of TSP-V0-004 in `docs/specs/test-selection-precision-v0.md` now defers to
AFP-V0-2953):

- AFP-V0-2950: an `os.Getwd` result whose every use is a literal comparison or the first argument of
  `filepath.Join`/`path.Join` with literal plain components is bounded. The joined path becomes a
  root-anchored token (`/docs/guide.md`), so the planner and the CI selector select the package
  when that path or anything below it changes and otherwise not. The proof extends AZU-V0-9930
  (`locatorproof.go`, identical in `internal/liveverify/affected/golang` and `internal/gateselect`).
- AFP-V0-2951: a `package main` unit keeps its own `unboundedReads` and its own selection, but is
  not `locatesRoot`, and a rule (d) chain stops at it (`Unit.Command`, `traverseWhere`). A library
  locator imported by a command (`internal/breakagemap` into `cmd/corvint`) reached every make
  target and CI job that builds the binary before this change.
- AFP-V0-2952: a `..` token a non-test file holds only inside line-broken literals without a
  directory-changing word is text the program writes (`internal/perfbudget/fixture.go`'s Ruby
  source, `tools/cem-stable-regen/main.go`'s Markdown table), not a path. The CI selector's scan
  memo carries the exemption per file.
- AFP-V0-2953: a `source` of `$NAME`/`${NAME}`/`${NAME/…}` where the file never gives NAME a value
  and sources no repository file reads a file the environment names (`heavy.sh`'s
  `HEAVY_REMOTE_ENV`). The former TSP-V0-004 near miss `envfile.sh` is now the positive case.
- AFP-V0-2954: a directory variable assigned twice with identical text resolves
  (`script/dogfood-finish.sh`'s `repo=$(cd "$(dirname "$0")/.." && pwd)` in two branches).
- AFP-V0-2955: `script/dogfood-cache_test.sh` and `script/qualify-tasks-basic_test.sh` declare their
  read scopes (TSP-V0-950); `.corvint/unbounded-readers.json` drops the eleven units that left the
  floor; `script/heavy.sh` keeps its remote half provable under TSP-V0-004.

Two further shell-plugin imprecisions blocked `heavy.sh` and are fixed under the existing
contracts: the lexer split a `[[ a || b ]]` test that follows `if` into two commands, so `$path`
read as a run target (`openTest` in `lex.go`); and a run target with a literal non-shell file name
(`"$sdk/go"`) was expanded through every value of its directory variable until the value bound cut
the expansion and marked the unit dynamic, although no expansion could name a script
(`constructed` in `computed.go`, TSP-V0-960). `heavy.sh` itself changed only variable names: the
payload directory `w` was reused by `unwrap_shell` and the embedded Perl word list, and the Go
SDK probe reused the generic loop variable `c`.

## Measured plans

`corvint affected --base <head> --format json` on a scratch commit that only edits the named file,
built from the lane head in each row. `selected` counts every unit in `plan.selected`;
`unbounded` the ones with `witness.anyDirtyPath`; `causes` the distinct `witness.cause.unit`
values. Scope stays `UNKNOWN` in every row for the pre-existing TypeScript frontier reasons.

| Change | Before: selected / unbounded / causes | After: selected / unbounded / causes |
|---|---|---|
| (a) `.github/workflows/ci.yml` | 265 / 81 / 26 | 252 / 20 / 18 |
| (b) `script/heavy.sh` | 295 / 12 / 12 | 295 / 10 / 10 |
| (c) `docs/TICKET-PRIORITY.md` | 253 / 117 / 30 | 240 / 55 / 21 |
| (c') `docs/DOGFOOD.md` | 262 / 38 / 22 | 259 / 18 / 17 |

Whole Go packages selected on the doc-only change: 25 before, 24 after (the `go:` units are
dominated by each package's own recorded floor). The 55 units still selected on the doc-only
change, by cause:

- 33 are make targets and CI jobs whose recipes name `./...` or `go vet ./...`/`go test ./...`
  (`make gate`, `make go-vet`, `make doc-gates`, `ci.yml#go-product`, …), reached from
  `conformance/work-queue-v0`, a library whose `ownershipSnapshot` records `os.Getwd` in a map and
  so escapes the AFP-V0-2950 proof. A `./...` runner compiles every unit, so any library locator
  reaches it; before this change the same targets were reached through `cmd/corvint` or
  `heavy.sh`. A driver rule that tells `go vet` from `go test` would narrow them; it is out of
  this lane's scope (friction 5).
- 18 are single Go packages selected for their own tests, every one listed with its reason in
  `.corvint/unbounded-readers.json` (git over the checkout, nested builds, root listings).
- 2 are `internal/lspstdio` and `tools/linecov`, recorded dependents of `internal/jstestprovider`,
  which runs `git rev-parse --show-toplevel`; `make capability-catalog-check` and `make tasks-test`
  compile `internal/tasks/store`, which carries the same literal.
- 1 is `script/lane-scripts_test.sh`, recorded: it sources a `sed`-filtered copy of `heavy.sh`
  from its temporary directory, which no proof links.

`make unbounded-readers-check` passes with 24 units (36 before). `make selection-baseline-check`
passes: narrow 33/43 merged changes (floor 30), 0 misses over the 26 failing packages of 15 failed
CI runs, and seven recorded changes moved from `FULL` to `NARROW` (`pr-41`, `pr-43`, `pr-46`,
`ci-4a7bf04c8`, `ci-8db625151`, `ci-cbc790640`, `ci-0df97c8c7`).

## Fault injection

Each mechanism has a test that fails with the fix removed and a control that keeps a real reader:

- AFP-V0-2950: `TestBoundedJoinLocatorIsNoRead_AFPV02950` (a literal join is selected only by the
  joined file; a directory join by anything below it; returned, copied, climbed, computed,
  second-position, `.`, package-level and passed roots stay rule (d), and so does a dependent of
  one) and `TestBoundedJoinLocatorParity_AFPV02950` (both analyzers, cold and warm memo).
- AFP-V0-2951: `TestCommandLocatorReachesNoRunner_AFPV02951` (the command is selected, its runners
  are not; a library locator's importer and the command that compiles it still are).
- AFP-V0-2952: `TestLineBrokenClimbIsText_AFPV02952`, `TestLineBrokenClimbParity_AFPV02952`
  (prose, a format template and a raw string exempt; a single-line path, a `cd` script, a token
  both forms hold and a test file keep rule (d); the exempt token still names its file).
- AFP-V0-2953: `TestEnvironmentInputSourceIsBounded_AFPV02953` (five admitted forms; assigned,
  `read`, `for`, default, defaulting, trailing-path, positional, sibling-literal, sibling-dynamic,
  two-operand and `local` controls stay dynamic).
- AFP-V0-2954: `TestSameValuedReassignmentResolves_AFPV02954` (two and three identical assignments
  link; differing, `local`, `read`, `for`, `+=`, default and `export` forms do not).
- Repository: `TestRepositoryScriptsAreBoundedSources_V14283` pins `heavy.sh` and
  `dogfood-finish.sh` bounded and linked, the two declared tests read-scoped, the five commands
  without `locatesRoot`, `perfbudget` and `cem-stable-regen` without the text-literal reason, and
  `jstestprovider` still a locator. `TestHeavyShEditKeepsShellReaders_V14170` now expects the two
  heavy.sh callers for a heavy.sh edit and not for an unrelated one.

## Checks

Focused tests through `script/heavy.sh`, one package per command: `internal/liveverify/affected`,
`internal/liveverify/affected/golang`, `internal/liveverify/affected/shell`, `internal/gateselect`
(each whole, with the reason), `tools/unbounded-readers`, `tools/test-read-scopes-check`,
`tools/plan-delta`, `tools/selection-baseline`, and the eight `cmd/corvint` tests
`script/test-run-pattern.sh` named plus `-run 'Affected|UNBOUNDED|Unbounded|ReadScope|TestUseCase'`;
all pass. `make windows-vet`, `make unbounded-readers-check`, `make test-read-scopes-check`,
`make shell-foreign-callers-check` and `make selection-baseline-check` pass.
`script/lane-scripts_test.sh` fails groups 27 and 55 on this WSL host with the base `heavy.sh`
exactly as with the lane's (`go env GOFLAGS` counted in the stub's argument log; the unreadable
index fixture refuses `index-copy-failed`), so the failure is the host's, retained as friction.

## Limits

Rule (d) still holds every package whose root escapes into a function argument, a struct or map
value or a return (`conformance/work-queue-v0`, `internal/opencodequalification`, the conformance
mains for their own tests); `--show-toplevel` stays a locator because the subprocess that answers
it reads Git metadata. A joined path later climbed with `filepath.Dir` is a documented residual of
AFP-V0-2950. `script/lane-scripts_test.sh` stays on the floor. Windows evidence is the vet twin
only.
