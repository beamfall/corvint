<!-- SPDX-License-Identifier: Apache-2.0 -->
# Unbounded readers: the second review's two corrections (V1-4283)

Lane `unbrd`, fix round 2 over `2026-11-07-unbounded-readers-review-fix`, base
`4052b8540b5fff818a819db37b55371ea5428c2c`. The fix-only review confirmed the four round-1
corrections and raised two blocking P1 findings. The rule for the round is the same: a missed test
is the failure that matters, over-selection is acceptable, and where a proof does not hold the
uncertainty is kept. Both findings were checked against the code first, and both held.

## Findings and corrections

- Listing output drives execution (held). `go list ./lib | xargs -I PKG go test PKG` gave lib only
  a build-only edge (AFP-V0-2951), and `go test PKG` recorded nothing, since a bare word reads as a
  standard-library pattern. A non-test root locator in lib therefore no longer reached the target.
  The scanner test showed `paths map[]`, `build paths map[lib:true]`, and the real Makefile target
  went unselected on an unrelated change. The fix has three parts:
  - AFP-V0-2956: a `go vet` or `go list` whose standard output feeds another command records
    execution edges. That covers a pipe (`|`, `|&`), a command substitution (`$(...)`, backticks)
    and a process substitution (`<(...)`) at any nesting, and make's `$(shell ...)`. The scanner
    also stops reading the word after `<(` as a redirect target.
  - AFP-V0-2957: a command that `xargs`, `parallel` or `find -exec` runs is fed arguments the
    scanner cannot see. A go invocation then imports every owner (`...`), and a make call is
    computed. An interpreter, or a command word that holds the replacement string, marks the site
    computed. `driverXargs` now parses xargs options on its own; the shared wrapper loop took
    xargs `-p` as an option with a value and swallowed the command.
  - AFP-V0-2958: an interpreter that reads its program from standard input is a computed
    invocation. That covers `| sh`, `bash -s` and a heredoc-fed `bash <<EOF`, whose body the
    scanner skips.
- Stdout is mutable (held). Code can point `os.Stdout` at an `os.Pipe` writer, print
  `"../config%s\n"`, read the pipe back, trim it and open `../config.go`. The output proof exempted
  that literal, and an edit of `config.go` selected nothing, in both analyzers. Two conditions now
  apply:
  - AFP-V0-2959: the exemption holds only for a `package main` file. No other package can import a
    command, so its directory, external test package included, is all the code in its process. A
    library's printed text keeps rule (d): an importer may capture the stream, and no per-package
    scan sees it. The review asked for evidence from the package and its tests; this condition
    goes further because only a command makes that evidence complete.
  - AFP-V0-2960: no file of the directory, tests included, may show evidence of capturing a
    stream (`capturesStream`, identical in both analyzers). The evidence is any of:
    - a pipe, a duplicated descriptor, a replaced handle or `NewFile`;
    - a child's captured `Output`, `CombinedOutput`, `StdoutPipe` or `StderrPipe`;
    - an assignment to, the address of, or a composite-literal key named `Stdout` or `Stderr`;
    - a `//go:linkname`, or a file that does not parse.

    The test is lexical and broad on purpose. Exempt tokens are held back until the whole
    directory is seen. The CI selector's scan memo carries each file's `main` and `captures` flags.

## Fault injection

Each regression fails before its fix. The new tests were written first and run against `b49a8348b`:

- AFP-V0-2956/2957: `TestDriverListingOutputKeepsExecution_AFPV02956` failed on the review's case,
  ten other scripts and the fed make call.
- AFP-V0-2956/2957, real Makefile: `TestListingOutputReachesItsRunner_AFPV02956` left `list-run`
  unselected.
- AFP-V0-2958: `TestDriverStdinProgramIsComputed_AFPV02958` failed on all eight stdin forms.
- AFP-V0-2959/2960, planner: `TestLineBrokenClimbIsText_AFPV02952` failed on `printedlib`, and
  `TestCapturedStreamKeepsClimb_AFPV02960` failed on all six capture cases. The pipe readers and
  the test-captured command went unselected by `config.go`.
- AFP-V0-2959/2960, CI selector: `TestCapturedStreamParity_AFPV02960` returned rule (d) `[]` cold
  and warm, and `TestLineBrokenClimbParity_AFPV02952` missed `printedlib`.

Each guard was then removed on its own from the fixed code, and its test failed again:

| Guard removed | Failing test |
|---|---|
| the `s.feeds == 0` condition | `TestDriverListingOutputKeepsExecution_AFPV02956` |
| the fed `...` import | `TestDriverListingOutputKeepsExecution_AFPV02956` |
| both of the above | `TestListingOutputReachesItsRunner_AFPV02956` |
| the stdin rule | `TestDriverStdinProgramIsComputed_AFPV02958` |
| the `main` condition, planner | `TestLineBrokenClimbIsText_AFPV02952` |
| the `main` condition, selector | `TestLineBrokenClimbParity_AFPV02952` |
| the capture pass, planner | `TestCapturedStreamKeepsClimb_AFPV02960` |
| the capture pass, selector | `TestCapturedStreamParity_AFPV02960` |

## Measured plans

`corvint affected --worktree --path PATH --no-cache --format json` was run with a binary built
from `b49a8348b` and one built from the fixed code, on the same tree. The columns count the
selected units, the ones with `witness.anyDirtyPath`, and the distinct `witness.cause.unit` values.

| Change | `b49a8348b` | Fix round 2 |
|---|---|---|
| (a) `.github/workflows/ci.yml` | 266 / 77 / 24 | 266 / 77 / 24 |
| (b) `script/heavy.sh` | 295 / 11 / 11 | 295 / 11 / 11 |
| (c) `docs/TICKET-PRIORITY.md` | 252 / 113 / 28 | 252 / 113 / 28 |
| (c') `docs/DOGFOOD.md` | 261 / 37 / 21 | 261 / 37 / 21 |

The selected units, their witness kinds and causes, and the unknowns are identical, unit by unit,
for all four changes. Whole Go packages are 29, 48, 25 and 31. Neither correction moves a
repository unit:
- No repository command had printed-text tokens that the exemption actually used.
- Two repository drivers feed `go list` output to another command. `ci.yml` pipes `go list ./...`
  into `ci-shards`, and `linecov-index.yml` substitutes `$(go list -m)`. Their jobs
  (`ci.yml#go-product-shard` and the others, `linecov-index.yml#collect` and `#merge`) already had
  execution edges to every owner and were unbounded readers through `benchmarks/runner` before and
  after.
- No repository driver gives an interpreter its program on standard input. The `find -exec`
  actions in `ci.yml` run `rm`, `chmod` and `cat`, and `go test -exec` is a flag that follows its
  `go` word, so neither feeds a later command word.

These counts use `--worktree --path`, so they are not comparable with the round-1 table, which
used scratch commits and `--base`. `make unbounded-readers-check` and
`make test-read-scopes-check` pass with the records unchanged.

## Limits

- AFP-V0-2957 sees a feeder only as a command word (`xargs`, `parallel`) or a `find` action word.
  A wrapper script that runs xargs is a path import of its own unit, as before.
- A file that a `go list` writes and a later step reads is covered only through the reader. A fed
  run imports every owner, and an expansion pattern already does.
- `capturesStream` is lexical. It cannot see a capture made through reflection or `unsafe`, or a
  capture in a C file. The analyzer already treats a cgo file as a frontier.
