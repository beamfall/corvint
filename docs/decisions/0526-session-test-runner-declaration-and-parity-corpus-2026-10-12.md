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
   `declaredDigest`. The Makefile delegates to the declaration: `make go-test` is the one fixed
   recipe line `go run ./tools/test-commands-run`, which runs `go test <go.flags> ./...` with
   `go.env` from the repository root (item 3). A maintained repository gate,
   `make test-commands-check` (`tools/test-commands-check`, in `GATE_STEPS` and the CI doc-gates
   job), reads the Makefile text, never make's output, and asserts that the `go-test` rule is
   exactly that line, that no Makefile or included line sets `SHELL`, `.SHELLFLAGS`, `MAKEFLAGS`
   or `.RECIPEPREFIX`, exports `MAKEFLAGS` or every variable, or defines a target- or
   pattern-specific variable that could reach `go-test`, that the text needs no make to read
   (no `$(eval)`, `define`, computed or missing `include`), that every declared `make` language
   command names a target that dry-runs, and that no `GATE_STEPS` recipe word names a runner of a
   `none` language (STR-V0-006 (a) to (e)). The Makefile reader and its make-semantics tests are
   deleted, not kept behind a flag.
2. STR-V0-013's parity input becomes the retained substitute corpus,
   `tools/str-parity/testdata/pairs-645304a2.txt`: the 50 first-parent pairs ending at
   `645304a2fb71065f9061d8d8d866600d703e84c6` that slice 2 measured, committed as a file the parity
   tool reads (`-corpus`) and checks against `git rev-list`. The original AFP-V0-011 pair list and
   the AFP-V0-029 shadow rows stay `NOT_OBSERVED`; nothing reconstructs them from memory. The
   AFP-V0-029 shadow bar remains a precondition for any CI narrowing.
3. Owner decision (2026-10-12, in chat): the Makefile runs the declaration. This replaces the
   make-observing drift gate (a recording `go` shim on a minimal `PATH`, a control recipe for the
   make-introduced names, a recipe shape rule and a `make -p` database parse) that Codex review
   rounds 4 to 8 kept finding incomplete, each round surfacing another make semantic the gate had
   to reverse-engineer: exports and `.EXPORT_ALL_VARIABLES` (4), the working directory, a second
   `go` and `MAKEFLAGS` in the recipe (5), `SHELL` wrappers and make functions in expanded
   variables (6), the 4.x database header and target- or pattern-specific `SHELL` (7),
   pattern-specific values of every expanded variable (8). With one fixed recipe line and
   `tools/test-commands-run`, `make go-test` has no second statement of the command; the one
   other statement, `make gate`'s `ledger/go-test`, is pinned to the declaration by STR-V0-006
   rule (f), and the gate becomes the bounded check of STR-V0-006 (text rules, plus the `make -n`
   dry-runs of rules (d) and (e)). The shim, the control recipe, the database parser
   and their tests are deleted; the round-8 fix was discarded uncommitted.
4. Owner decision (2026-10-12): Codex review is capped at one round on the redesigned gate, then
   ship. Acceptance no longer waits for a `VERDICT: PASS`; it follows that one independent review
   round, and each of its findings is either fixed or recorded as a limit in STR-V0-006 and the
   build log. That round (Codex, on `21af64b58`) returned six findings: the `ledger/go-test` gap
   (fixed, rule (f)), a recipe line hidden in a conditional and `.IGNORE` (fixed), a declared
   `PATH` (the reader refuses it), and the text-only and `none` wording (narrowed). None was
   left open.
5. Owner decision (2026-10-12): `make gate` is waived for V1-0040. The ticket's `requiredGates`
   full gate is `NOT_RUN` under that waiver; the evidence that replaces it is the focused tests
   (`tools/test-commands-check`, `tools/test-commands-run`, `internal/sessiontest`), the CI
   doc-gates list run locally, the use-case receipts checks and the CI run on PR #815.

Amended requirement IDs (text changed, IDs stable): `STR-V0-003` (declaration file, no Makefile
interpretation, unreadable is `INCOMPLETE` in every scope), `STR-V0-004` (`none` records a
`noSuite` entry and runs nothing; undeclared stays `INCOMPLETE`), `STR-V0-006` (the
`test-commands-check` gate and the `declaredDigest` binding), `STR-V0-013` (the retained corpus as
the parity input) and the `STR-V0-015` wording that named the Makefile `GO_TEST_TIMEOUT`. No
requirement is added or removed; the delivery status stays experimental.

## Enforcement

- `internal/sessiontest/declared.go` is the only reader; `TestSTRV0003ReadDeclared` pins the
  unreadable cases and the digest's independence from member order.
- `tools/test-commands-check/main_test.go` runs the gate against the repository, three
  declaration mutations (a declared `make` target that does not exist, a `none` language the gate
  runs, a `none` language without a runner list) and an absent file; thirty-two Makefile text
  mutations (thirty-five with a recipe line inside a conditional, `.IGNORE` and a global
  `.IGNORE`), each of which must fail (a changed, continued or second `go-test` recipe line, no
  recipe, a prerequisite, `::`, a second rule, `SHELL`/`.SHELLFLAGS`/`MAKEFLAGS`/`.RECIPEPREFIX`
  plain, exported, target- and pattern-specific, `export MAKEFLAGS`, bare `export`,
  `.EXPORT_ALL_VARIABLES`, target-, pattern-, match-anything- and expanded-target variables,
  pattern and expanded-target recipes tab-indented and after `;`, `$(eval)`, `define`, a computed
  and a missing `include`, a second and an expanded `GATE_STEPS`) and five that must pass; a
  nested-include fixture with the GOC-V0-010 prerequisite-only line; twelve `ledger/go-test`
  mutations (TestSTRV0006LedgerMatchesDeclaration); and the line classifier.
  `tools/test-commands-run/main_test.go` runs the runner under a fake `go` on `PATH` (argv,
  directory, environment precedence, exit status) and pins its four refusals. Its recorded
  limits are in STR-V0-006 (the environment make itself starts with, and the `none` check seeing
  only direct recipe words).
- `tools/str-parity -corpus` refuses a corpus whose lines differ from
  `git rev-list --first-parent --parents` over the same end and count.

## Consequences

- A language the declaration marks `none` is not `INCOMPLETE`; the assertion is the project's,
  scoped to the gate, and visible in every manifest and dry-run document (`noSuite`). Adding a
  suite to the gate for such a language means replacing `none` with its command (the gate refuses
  the stale `none` when a `GATE_STEPS` recipe word names that language's runner; a suite started
  inside a script, a driver or CI is the owner's to review), which the next `corvint test` then runs in `FULL`; `typescript` already runs
  `make host-adapter-test`, which the drivers do not run, so parity records it as
  `str-v0-004-declared-language`.
- The CI profile keeps its own constants (`-p 1 -count=1 -race -timeout 50m`, the closed CI
  environment, AFP-V0-028): the declaration describes the local full suite, and the CI call site
  is slice 3's.
- The driver parity report is reproducible from committed inputs only.

## Rollback

Revert the lane's commits on `claude/v1-0040-impl`: the slice 1 Makefile reader returns with its
tests, STR-V0-003/004/006/013/015 return to their slice 2 text, `make go-test` returns to
`$(GO_TEST_COMMAND) ./...`, `tools/test-commands-run` goes with its tests, and the gate step
leaves `GATE_STEPS` and the CI doc-gates list. The corpus file can stay; it is inert without `-corpus`.

## Slice 3

Slice 3 (`claude/v1-0040-slice3`, `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-12-session-test-runner-slice-3.md`)
switches `make gate-affected` to `corvint test` and adds a shadow-only CI step beside the driver.
It amends STR-V0-012 (the gate-affected call site, a bad BASE as a usage error, inherited
`MAKEFLAGS`, ci constants held to `ci.yml`, declared commands reported as checks in the ci
profile, partition inputs), STR-V0-013 (the shadow step and `CORVINT_PR_FORWARD_EPOCH` at 1, a
vacuous advance) and STR-V0-015 (a per-shard budget: 6h local as an outer hang bound, 60m ci). AFP-V0-011 gains only a
note that it now governs the retained rollback `script/gate-affected.sh`. This decision's body
above is unchanged.
