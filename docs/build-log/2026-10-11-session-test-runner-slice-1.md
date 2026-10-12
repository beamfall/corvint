# V1-0040 slice 1: experimental `corvint test` session test runner

Date: 2026-10-11. Base: `136dfa483866558e67821c70c0680fc8965ed7eb` (spec commit on
`claude/v1-0040-impl`). Scope: `docs/specs/session-test-runner-v0.md` STR-V0-001..007, 009..011,
014, 015, 017, 018, with 004, 012 and 016 partial; STR-V0-008 and STR-V0-013 are `NOT_RUN`. The
intent stays proposed; no call site (`script/gate-affected.sh`, `tools/corvint-pr-tests`, Makefile,
workflows) changes, so the existing drivers remain the rollback.

Contracts and decisions:

- Planning: `corvint test` calls the same `compileAffectedGraph` as `corvint affected --base`
  and the AFP-V0-012 attribution, now in `internal/gateselect` (moved from
  `tools/gate-affected-select` without a behaviour change). `internal/sessiontest` owns the
  manifest, receipt, Go adapter and fan-in. It is not called `testrunner` because that package
  already exists.
- Verdict: `FULL` for every attribution `FALLBACK`, for every `plan.unknown` entry outside the
  AFP-V0-028 allowlist, for a selected package outside the Go universe, and when there is no
  verdict. An allowlisted non-Go `LANGUAGE_FRONTIER` entry names a frontier category, not a unit,
  so it does not require that language's declared command. This repository has six such entries
  in every plan. A selected unit, or a `NO_SELECTABLE_TEST` unknown unit, in a language without
  an adapter does require the command. Accepted by the owner (below).
- No plan: the declared full scope runs on shard 0, read independently of the planner: the
  Makefile `go-test` target and each `<lang>-test` target for every registered language name. A
  declaration that cannot be read or names nothing is `INCOMPLETE`.
- FULL with a plan also runs every declared `<lang>-test` command, because a fallback such as a
  deleted source file names only a path, not its language, and it covers every non-Go language
  whose registered plugin owns a `plan.dirty` path (undeclared means `INCOMPLETE`). An unreadable
  declaration under FULL is `INCOMPLETE`; so is a Makefile using `include` or `eval`, which can
  declare targets this reader never sees.
- The `ci` profile never runs `make go-test`: with no plan, or FULL with no Go universe, Go is
  `INCOMPLETE`, because that target would drop the CI confinement, `-race` and environment.
- Go recipe typing is conservative: anything the reader does not reproduce exactly leaves
  go-test untyped, never partially typed. The `go-test` recipe must be one line of the form
  `[NAME=VALUE ...] go test FLAGS ./...`, without shell metacharacters, `-json`, `-run`, `-skip`
  or an inline `PATH=`. Blank and comment lines do not end the recipe. It may use `$(VAR)`, `=`,
  `:=`, `::=` and `?=`: `?=` leaves a variable the Makefile already assigned alone and otherwise
  prefers the invoking environment, and `:=` expands immediately (an immediate value that cannot
  be expanded is refused). A rule naming several targets declares each one. Untyped:
  - a conditional (`ifeq`, `ifneq`, `ifdef`, `ifndef`, `else`, `endif`) inside the go-test recipe
    span, because make keeps reading the recipe through it, or a go-test rule inside a conditional
    block; a `define` block anywhere; an unmatched or unterminated conditional;
  - a top-level `=`, `:=` or `::=` assignment to a variable present in the invoking environment
    (make passes its Makefile value to the recipe), or to `PATH`, `GO[A-Z0-9]+` or `CGO_*`
    (`GO_TEST_FLAGS` is not a Go variable); `?=` on an inherited variable keeps the environment
    value and is allowed;
  - `export`, `unexport`, `override` or `.EXPORT_ALL_VARIABLES` in any position, `+=`, `!=`,
    `include` and `eval`; `MAKEFLAGS` carrying a variable assignment; a non-empty `MAKEFILES`
    (also `INCOMPLETE`); an assignment to `SHELL`, a `MAKE*` control variable or a dotted special
    variable other than `.DEFAULT_GOAL`; a special target other than `.PHONY`;
  - a rule target containing `$`, a tab line outside a rule, and any other top-level line the
    reader does not recognise;
  - a variable assigned under a conditional, a go-test rule with prerequisites, target-specific
    variables or `::`, and a `%` pattern rule matching go-test.
  Conditionals elsewhere are allowed. An `ifeq`/`ifneq (A,B)` line is evaluated only inside a
  subset that is exact by construction for `make go-test`, and reads no Makefile or environment
  variable: literal words of ASCII letters, digits and `-_./` separated by single spaces,
  `$(MAKECMDGOALS)` as exactly `go-test`, `$(filter LITERALS,X)` with literal patterns and no
  `%`, and `$(or X,Y)`, with words split only on ASCII space and tab. A false branch is skipped as
  make skips it. Anything else (another variable reference, `%`, a tab, doubled or surrounding
  blanks, a comment, a non-ASCII byte or another function) reads every branch. Codex round 6
  showed that evaluating user or inherited variables is not exact (trailing blanks before a
  comment, recursive inherited values, `?=` against make's built-in defaults such as `CC`, and
  Unicode word separation), so the evaluator stopped doing it rather than chase GNU make. An
  invoking `MAKEFLAGS` with the `e` flag (environment overrides) also leaves go-test untyped. This
  repository's Makefile still types go-test: its line-49 `$(GATE_STEPS) ...: | gate-receipt-clear`
  sits under `ifneq ($(filter gate,$(or $(MAKECMDGOALS),gate)),)`, which is false for
  `make go-test`, and the 63-71 block holds no go-test rule
  (`TestSTRV0006RepositoryMakefileTypesGoTest`; a mutation that never skips a branch fails it on
  line 49). An untyped recipe never narrows Go: a narrowed run is `INCOMPLETE` and a `FULL` run
  calls `make go-test`. The local argv adds `-json` so outcomes can be attributed to packages.
- CI profile argv and environment are the full CI branch's: `-exec <confine> -p 1 -count=1 -race
  -timeout 50m`, with `GOPROXY=off`, `GOSUMDB=off`, `GOWORK=off`, an empty `GOFLAGS`,
  `CGO_ENABLED=1` and `CORVINT_CONFINE_ROOT`. The CI admission parts of STR-V0-012 belong to the
  call site and are not here.
- Placement: `cishards.PlanFrom` over the repository's `.github/cishards` files, or the embedded
  defaults when they are absent, intersected with the run set. Split packages expand into their
  slice units, including the catch-all slice. `cmd/corvint` now links `.github/cishards`.
- Exit codes: 0 PASS, 1 FAIL, 2 usage, 3 INCOMPLETE, 128+n for a signal (HUP 129, INT 130, TERM
  143). Both verify-shards and a single shard use 3 for missing evidence, so callers can tell it
  apart from a failing test.
- Writes: `--dry-run` writes only `--manifest PATH`. A run writes only its receipt, by default
  `<git-dir>/corvint/test-run[.I].json`, which is neither the index nor a `.corvint/` ledger. A
  supplied or default output path is refused before planning when it is tracked, under `.corvint/`, or in Git
  metadata other than `<git-dir>/corvint/test-run[.I].json`. It is also refused when its leaf is a
  symlink (the write replaces the entry, not the target) or when it is an unignored worktree file,
  which would be a dirty path and change the bound source identity.
- Source identity refuses a dirty directory (a submodule or nested repository), whose contents
  it cannot bind; the run exits 2 before planning. Every shard rechecks the identity before it
  finishes, including one that launched nothing.
- Fan-in recomputes the source digest from the manifest's and each receipt's source identity and
  checks the receipt revision, so a receipt cannot keep the digest string over other identities.
- Process ownership: each adapter runs as its own process group. Cancel, a signal or the 70
  minute budget kills the whole group. A platform without process groups runs nothing and is
  `INCOMPLETE`.
- Discovery: before any Go unit runs, the runner version is read and `go list ./...` must equal
  the planned universe.

Evidence:

- Corvint `affected --base 645304a2` selected 112 packages; 5 had direct source changes.
- Focused tests:
  - `internal/sessiontest`
  - `internal/gateselect`
  - `tools/gate-affected-select`
  - `internal/liveverify/affected/golang`
  - `.github/cishards`
  - `cmd/corvint` with `-run` over `TestSTRV00*` plus the help, require-clean, Core-freeze and
    affected tests
- On this repository, a dry run at the slice base was `FULL`, because of the
  `UNINDEXED_SOURCE_PATH` entries the refactor leaves behind. Its universe equalled
  `go list ./...` (326 packages), and the four `--shard I/4` manifests were byte-identical.

Not run:

- STR-V0-013 parity
- balanced placement (STR-V0-008)
- a hosted CI run
- non-Go adapters

To fit the new verb within the 4096-byte root-help bound (CCF-V1-009), three phrases in root
help were shortened.

Owner decisions (2026-10-11, relayed by the orchestrator):

- Accepted as implemented: allowlisted non-Go frontiers do not require a declared command; the
  `<lang>-test` declaration format; `--manifest` on a run; `-json` in the local argv; no-plan FULL
  on shard 0; exit 3 for `INCOMPLETE`; the default receipt under `<git-dir>/corvint/`; FULL running
  every declared language plus every dirty-owning language; refusing a dirty submodule; refusing
  an unignored worktree output (identity safety wins over convenience).
- Resolved by the conservative typing above: a `$(VAR)` rule target leaves go-test untyped.
- Open, deferred to slice 3 where the CI call site binds them: repository partition files versus
  the embedded copy, the CI argv and environment constants, `MAKEFLAGS` handling at the call site,
  and the `ci` profile refusing `make go-test`.
