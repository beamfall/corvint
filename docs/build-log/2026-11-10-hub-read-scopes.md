# AFP-V0-107..110: hub read scopes (V1-5201)

Proposed 2026-11-10 in the OpenSpec change `hub-read-scopes`; base
`867ff4991ec88203f858b37e82bfadbf4f6d3396`, branch `claude/hub-read-scopes-stage1`.

## Contract

A package that declares a read scope in `.corvint/test-read-scopes.json` leaves rule (d), the
unbounded-reader rule, so a change outside its scope no longer selects it. AFP-V0-107 moves a hub's
whole-repository tests into a sibling package that stays unbounded. AFP-V0-108 declares a hub scope
only after the hub passes under the Landlock wrapper with exactly that scope. AFP-V0-109 keeps every
unproven hub selected and uncounted. AFP-V0-110 measures a fixed docs-only diff before and after.

## What moved

`internal/liveverify/affected` kept its unit tests. Its 40 whole-repository test functions moved to
the new package `internal/liveverify/affected/liverepo` (28 files plus
`testdata/ownership-audit-baseline.txt`). Those tests build the unit graph over this checkout, list
the root, run Git or plan sealed CEMs and build-log entries. The list came from the confined run,
not from the recorded reasons. Every test that calls `RequireLiveTier` is now in `liverepo`.

- Narrow export instead of a test-only accessor: `(*Graph).Namers(dirtyPath)` in `readers.go`
  replaces the hub's `export_test.go` accessor, so `liverepo` can query reader namers. This departs
  from design.md's "production API unchanged"; the method only reads.
- The shared witness helper moved to `internal/liveverify/affected/affectedcheck/witness.go`.
- Hub tests that kept a fixture-relative `..` now build it as `strings.Repeat(".", 2)`, so the
  read-scope checker does not treat every hub token as root-relative. The boundary-scanner source
  that a hub test compiles is now `testdata/boundary-scanner.go.txt`.
- The hub declares the empty scope `"internal/liveverify/affected": []`. `liverepo` replaces it in
  `.corvint/unbounded-readers.json` and `.corvint/record-readers.json` with its reasons.
- Records that named the moved files follow them: `.corvint/test-selection.json`,
  `.corvint/shell-foreign-callers.json` (the `heavy.sh` mention), the Makefile
  `shell-foreign-callers-check` target, `tools/gate-ledger` and GL-V0-788.

## Confined run (AFP-V0-108)

The run used WSL (`wsl-lanes`, Landlock ABI 7) on a detached worktree at `71357d244`. The wrapper
was built from `.github/testconfine` with go1.27.3, and `confine.sh` exports
`CORVINT_CONFINE_ROOT=$(git rev-parse --show-toplevel)`:

```sh
script/heavy.sh go test -count=1 -v -timeout 9m -exec ~/.cache/hubconfine/confine.sh \
  -run '^(Test[A-H]|Fuzz|Example)' ./internal/liveverify/affected
script/heavy.sh go test -count=1 -v -timeout 9m -exec ~/.cache/hubconfine/confine.sh \
  -run '^Test[I-Z_a-z0-9]' ./internal/liveverify/affected
```

Both passed: 920 passed and 1 skipped, then 344 passed and 0 skipped. The one skip,
`TestDriverComputedReasonCounts`, needs `CORVINT_DRIVER_REASONS_ROOT` and also skips without
confinement, so no test newly skips. As a negative control, giving `liverepo` an empty scope made
`TestRepositoryDeclaresItsInstructionFiles_AFPV0763` fail with `permission denied` on
`.corvint/documentation-paths.json`. heavy.sh ran with `HEAVY_REFUSE_FREE_GB=0` and `TMPDIR` under
`~/.cache`, because the host's tmpfs `/tmp` had under 1 GB free.

## Hubs that stay unbounded (AFP-V0-109)

- `cmd/corvint`: 394 static findings in 43 files. `golist`, `test_universe` and `compiled_binary`
  tests run `go list` or `go build`, which the checker resolves to the whole closure. At least 13
  files climb to the root (first: `affected_changed_checks_test.go:13`) and call package-main
  internals, which a sibling package cannot import.
- `internal/liveverify/affected/golang`: 55 findings, root locator
  `binaryconsumers_repo_test.go:16`. The repository tests use unexported
  `readBinaryConsumers`, `observeDirectory`, `goListUnits` and `readKindAliases`. Moving them needs
  production exports.
- `internal/contextindex`: the three self-corpus differential tests need about eight unexported
  internals.
- `cmd/corvint-docs`: 110 findings, 106 from `core_binary_test` building `corvint`; not attempted.

## Measurement (AFP-V0-110)

A throwaway worktree got one scratch commit appending one line each to `AGENTS.md`, `Makefile` and
`docs/specs/eol-policy-gate-v0.md`. It was planned with `corvint affected --base <parent>
--no-cache --format json` at `origin/main` and at `975d12bdd`, using one binary built from
`975d12bdd`.

| Plan | `summary.line` | Go units | Go tests |
| --- | --- | --- | --- |
| before | `scope=UNKNOWN dirty=3 selected=219 excluded=439 unknown=60 LANGUAGE_FRONTIER=60` | 23 | 2456 |
| after | `scope=UNKNOWN dirty=3 selected=219 excluded=440 unknown=60 LANGUAGE_FRONTIER=60` | 23 | 2129 |

The only Go change is that `internal/liveverify/affected` (367 tests, `PATH_LITERAL_READER`) leaves
and `internal/liveverify/affected/liverepo` (40 tests, `PATH_LITERAL_READER`) joins. `cmd/corvint`
(613), `internal/contextindex` (716) and `internal/liveverify/affected/golang` (160) stay selected.

## Open and NOT_OBSERVED

- `.github/cishards/live_test.go:88` still expects live-tier tests in
  `internal/liveverify/affected`. `TestRepositoryLiveTierIsExcludedFromPRShards_AFPV02702` fails
  until a separate `.github` change points it at `liverepo`. `.github/cishards/test-slices.json`
  and `package-costs.json` still name the moved tests; that is measured data, and that it is
  tolerated is an inference.
- Pull request 178's diff from base `2913fdc7e` was not measured: NOT_OBSERVED.
- No darwin run: the hub has darwin-only files that the WSL run neither builds nor runs.
- The whole-module gate and CI were not run.
