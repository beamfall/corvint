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
- Go recipe typing is exact by construction (Codex round 8 redesign). Rounds 1 to 7 patched a
  reader that reimplemented make by rejecting known-bad constructs; round 8 found seven more
  places where its accept set still differed from make or `/bin/sh` (an inherited
  `MAKECMDGOALS` hiding a conditional go-test as absent, trailing blanks trimmed from values, `?=`
  ignoring make's built-in defaults, recipe backslash continuations, Unicode word splitting,
  mismatched `${F)` delimiters, and `GNUmakefile`/`makefile` precedence). The root cause was the
  negative design, so the reader now types only a small positive grammar whose meaning is the
  same to make, to the shell and to the typed argv, and anything outside it is untyped, never a
  guess. Typed: the file make reads (`GNUmakefile`, then `makefile`, then `Makefile`, by
  `os.Stat` in make's order; none means absent); one go-test recipe line that is not a
  backslash continuation, after an optional `@`; `$(NAME)`/`${NAME}` references with matching
  delimiters to variables assigned at top level by `=`, `:=`, `::=` or `?=` (values kept
  verbatim after the operator, so trailing blanks stay significant and a `#` is checked, not
  stripped; `:=` expands immediately; `?=` takes an earlier Makefile value, then the environment,
  and is refused for a make built-in such as `CC`, `LINK.o` or any `MAKE*` name); an expanded
  recipe whose every byte is an ASCII letter, digit, `-_./=,`, space or tab, split on ASCII blank
  runs into `[NAME=VALUE ...] go test FLAGS ./...` without `PATH=`, `-json`, `-run` or `-skip`.
  Untyped (unchanged from earlier rounds where they already were): a conditional inside the
  go-test recipe span or around the go-test rule, `define`, unmatched or unterminated
  conditionals, text after `else`, an assignment to an inherited variable, `PATH`, `GO*`, `CGO_*`,
  `SHELL` or a `MAKE*`/dotted control variable, `export`/`unexport`/`override`/
  `.EXPORT_ALL_VARIABLES`, `+=`, `!=`, `include`, `eval`, special targets other than `.PHONY`,
  tab lines outside a rule, unrecognised lines, variables assigned under a conditional, go-test
  prerequisites or target-specific variables, `%` pattern rules matching go-test, and a
  `MAKEFLAGS`/`GNUMAKEFLAGS` word outside the harmless set (`s`, `w`, `k`, `-jN`, jobserver,
  print-directory and output-sync words); an option that can change what make reads or runs
  (`-e`, `-n`, `-R`, `-f`, `--eval`, `--warn-*`, `-- VAR=VALUE`) also marks the scope
  `INCOMPLETE`, as do a non-empty `MAKEFILES`, `.DEFAULT`, a computed rule target and a top-level
  `$(...)` line, because each can define a rule the reader never sees. Absence is claimed only
  when certain: `./go-test` counts as go-test, a `%-test` pattern rule declares go-test (untyped)
  and every wanted `<lang>-test`, and an inherited `MAKECMDGOALS` makes every `$(MAKECMDGOALS)`
  conditional unknown so both branches are read (round 8 finding 1 previously reported absent).
  Backslash continuation follows make: only an odd run of trailing backslashes continues, a
  non-recipe join condenses the surrounding blanks to one space, and a continued recipe line is
  untyped because the shell joins its pieces without a blank. `\r` is a byte outside the
  alphabet, not a line end. Conditionals elsewhere are allowed. An `ifeq`/`ifneq (A,B)` line is
  evaluated only inside a subset that is exact by construction for `make go-test`, and reads no
  Makefile or environment variable: literal words of ASCII letters, digits and `-_./` separated
  by single spaces, `$(MAKECMDGOALS)` as exactly `go-test` (unless inherited), `$(filter
  LITERALS,X)` with literal patterns and no `%`, and `$(or X,Y)` checked without short-circuit,
  with the whole text validated first (no braces, no `$$`, every `$` opens `$(`, balanced
  parentheses). A false branch is skipped as make skips it; anything else reads every branch.
  Verified against GNU Make 3.81 probes: `CC ?= clang` keeps `cc`, `F = -race ` keeps the blank,
  `F = x\\` is not a continuation, `${F)` is an error, `./go-test:` and `%-test:` both answer
  `make go-test`, and `MAKECMDGOALS=other make go-test` takes the `other` branch. This
  repository's Makefile still types go-test: its line-49 `$(GATE_STEPS) ...: | gate-receipt-clear`
  sits under `ifneq ($(filter gate,$(or $(MAKECMDGOALS),gate)),)`, which is false for
  `make go-test`, and the 63-71 block holds no go-test rule
  (`TestSTRV0006RepositoryMakefileTypesGoTest`; `TestSTRV0006RepositoryMakefileDependsOnConditional`
  mutates that condition to an unknown one and shows the reader then refuses on line 50;
  `TestSTRV0006ReaderGrammar` holds one case per round-8 finding and probe). An untyped recipe
  never narrows Go: a narrowed run is `INCOMPLETE` and a `FULL` run calls `make go-test`. The
  local argv adds `-json` so outcomes can be attributed to packages.
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
