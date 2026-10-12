# Decision 0526 — `corvint test` reads a project-owned test-commands declaration; parity is measured over the retained substitute corpus

Date: 2026-10-12. Status: accepted by the owner. Authority: two owner decisions in chat,
2026-10-12, for V1-0040 (`docs/specs/session-test-runner-v0.md`, intent accepted by decision
0524); the slice-requirement amendments below are accepted under the owner's standing instruction
of 2026-10-11 ("auto accept slice requirements too"), conditional on the lane's Codex review
reaching `VERDICT: PASS`.

## Context

Slice 1 of the session test runner typed the local Go full-suite command by reading the Makefile:
`internal/sessiontest/declared.go` grew a 1451-line GNU-make reader (conditionals, prerequisites,
target-specific variables, byte-level recipe parsing) and three test functions pinning its
grammar against the repository Makefile. Fourteen Codex review rounds on slice 1 were spent
closing make-semantics paths that the reader did or did not model, and the independent review of
the lane (`scratchpad/review-v1-0040`) recommended replacing the interpreter with a declaration the
project owns and a gate that keeps it honest. Separately, STR-V0-013 named the 50 first-parent
pairs of the AFP-V0-011 measurement and the AFP-V0-029 shadow rows as the parity input; neither
list was retained, so slice 2 measured a substitute corpus (the 50 first-parent pairs ending at
`645304a2fb71065f9061d8d8d866600d703e84c6`) and recorded the original inputs as `NOT_OBSERVED`.

## Decision

1. The repository declares its full-suite commands in `.github/cishards/test-commands.json`,
   profile `corvint-test-commands/0`: the Go full-suite environment and flags, and for every other
   language either its full command (an argv run from the repository root; a path-containing
   `argv[0]` resolves against the root) or `{"none": reason}`, the project's gate-scoped assertion
   that no `make gate` step, driver (`script/gate-affected.sh`, `tools/corvint-pr-tests`) or CI
   job runs a suite for the language. `none` is forbidden for a language whose suite the gate does
   run: `typescript` declares `["make", "host-adapter-test"]`, the gate step that runs the
   `integrations/pi` suites; suites outside the gate (`extensions/vscode`,
   `integrations/pi-protected`, `tools/web-flows`) are recorded limits. `corvint test` reads the file as a
   plain, bounded, strictly-schemed read (unknown or duplicate members, a missing `go` member, an
   oversized file and a malformed entry are all unreadable, which is `INCOMPLETE` in every scope),
   and never interprets the Makefile. The manifest binds the declaration's digest as
   `declaredDigest`. A maintained repository gate, `make test-commands-check`
   (`tools/test-commands-check`, in `GATE_STEPS` and the CI doc-gates job), observes what
   `make go-test` actually hands `go` through a recording `go` shim on a minimal `PATH` and asserts
   that the shim ran exactly once with argv `test <go.flags> ./...` and an environment equal to
   `go.env` after excluding only the closed make-introduced name set recorded in STR-V0-006, that
   every declared `make` language command names a target that dry-runs, and that no `GATE_STEPS`
   recipe word names a runner of a `none` language. The Makefile reader and its make-semantics
   tests are deleted, not kept behind a flag.
2. STR-V0-013's parity input becomes the retained substitute corpus,
   `tools/str-parity/testdata/pairs-645304a2.txt`: the 50 first-parent pairs ending at
   `645304a2fb71065f9061d8d8d866600d703e84c6` that slice 2 measured, committed as a file the parity
   tool reads (`-corpus`) and checks against `git rev-list`. The original AFP-V0-011 pair list and
   the AFP-V0-029 shadow rows stay `NOT_OBSERVED`; nothing reconstructs them from memory. The
   AFP-V0-029 shadow bar remains a precondition for any CI narrowing.

Amended requirement IDs (text changed, IDs stable): `STR-V0-003` (declaration file, no Makefile
interpretation, unreadable is `INCOMPLETE` in every scope), `STR-V0-004` (`none` records a
`noSuite` entry and runs nothing; undeclared stays `INCOMPLETE`), `STR-V0-006` (the
`test-commands-check` gate and the `declaredDigest` binding), `STR-V0-013` (the retained corpus as
the parity input) and the `STR-V0-015` wording that named the Makefile `GO_TEST_TIMEOUT`. No
requirement is added or removed; the delivery status stays experimental.

## Enforcement

- `internal/sessiontest/declared.go` is the only reader; `TestSTRV0003ReadDeclared` pins the
  unreadable cases and the digest's independence from member order.
- `tools/test-commands-check/main_test.go` runs the gate against the repository and nine
  declaration mutations (flag value, flag order, dropped flag, extra and changed environment
  variable, absent file, declared `make` target that does not exist, a `none` language the gate
  runs, a `none` language without a runner list) and twenty Makefile mutations (target-specific
  `export`, global `export`, `.EXPORT_ALL_VARIABLES`, absolute-path `go`, two `go` calls, a recipe
  failing after `go`, a changed flag, a changed directory, a recipe setting `MAKEFLAGS`, a
  Makefile exporting `MAKEFLAGS` or setting `SHELL`, a second `go` through an absolute path
  chained, on its own line or as an argument, a shell wrapper script or `nice` before `go`,
  `$(shell ...)` in the recipe, in a variable it expands or in a target-specific variable, a
  quoted `"python3"` in a gate step), each of which must fail. The gate runs make under a minimal
  environment, so the Makefile defaults are what it observes; it checks the make-introduced names
  against a control recipe in its own makefile under the same make rather than trusting them,
  requires the go-test recipe to be one `NAME=VALUE... go` command line so nothing the shim cannot
  see runs beside it, and reads make's database to refuse any make function the recipe would
  expand and any Makefile `SHELL`. Make functions expanded while the Makefile is read are its
  recorded limit.
- `tools/str-parity -corpus` refuses a corpus whose lines differ from
  `git rev-list --first-parent --parents` over the same end and count.

## Consequences

- A language the declaration marks `none` is not `INCOMPLETE`; the assertion is the project's,
  scoped to the gate, and visible in every manifest and dry-run document (`noSuite`). Adding a
  suite to the gate for such a language means replacing `none` with its command (the gate refuses
  the stale `none`), which the next `corvint test` then runs in `FULL`; `typescript` already runs
  `make host-adapter-test`, which the drivers do not run, so parity records it as
  `str-v0-004-declared-language`.
- The CI profile keeps its own constants (`-p 1 -count=1 -race -timeout 50m`, the closed CI
  environment, AFP-V0-028): the declaration describes the local full suite, and the CI call site
  is slice 3's.
- The driver parity report is reproducible from committed inputs only.

## Rollback

Revert the lane's commits on `claude/v1-0040-impl`: the slice 1 Makefile reader returns with its
tests, STR-V0-003/004/006/013/015 return to their slice 2 text, and the gate step leaves
`GATE_STEPS` and the CI doc-gates list. The corpus file can stay; it is inert without `-corpus`.
