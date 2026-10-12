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
  in every plan. A selected unit in a language without an adapter does require the command.
  Owner question below.
- No plan: the declared full scope runs on shard 0, read independently of the planner: the
  Makefile `go-test` target and each `<lang>-test` target. A declaration that cannot be read or
  names nothing is `INCOMPLETE`.
- Go recipe typing: the `go-test` recipe must be one line of the form
  `[NAME=VALUE ...] go test FLAGS ./...`. It may use `$(VAR)`, `:=`, `=` and `?=`, where `?=`
  defers to the invoking environment. It may not set `-json`, `-run` or `-skip`, and may
  not use shell metacharacters. Anything else leaves the recipe untyped, including export,
  override, include, define, `.EXPORT_ALL_VARIABLES`, a variable assigned under a conditional, and
  a rule under a conditional. An untyped recipe never narrows Go: a narrowed run is `INCOMPLETE`
  and a `FULL` run calls `make go-test`. The local argv adds `-json` so outcomes can be attributed
  to packages.
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
  `<git-dir>/corvint/test-run[.I].json`, which is neither the index nor a `.corvint/` ledger.
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
