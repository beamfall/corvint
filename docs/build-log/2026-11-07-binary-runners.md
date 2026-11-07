<!-- SPDX-License-Identifier: Apache-2.0 -->
# Rule (d) follows execution edges (V1-4983)

Lane `unbrd`, fix round 4 over `2026-11-07-unbounded-readers-review-fix-3`, base
`4052b8540b5fff818a819db37b55371ea5428c2c`, starting at `c80f5059c`. The fix-only review confirmed
round 3's seven fixes and raised one blocking P1, which is also ticket V1-4983. Requirements
AFP-V0-2990 to AFP-V0-2995 (`docs/specs/affected-plan-selection-v0.md`).

## Finding (held)

Rule (d) skipped Go binary consumers. `Graph.unboundedReaders` followed import edges, the drivers'
path imports and one level of test users, but no `execs` edge. In `reach`, the binary-consumer
step (`builtCommands`, `execUsersOf`) ran before `unboundedReadersOf`, so a command selected only
by rule (d) never reached its runners. The CI selector's closure walked imports only
(`reach(false, …)`, AFP-V0-464). That requirement's failure mode assumed that a package running
`go` or a built binary is its own unbounded reader, but no scan marks that.

The regressions were written first and run at `c80f5059c`. Each failed:

- `TestRuleDReachesGoRunRunner_AFPV02990` (planner), the review's exact case. `tool` prints
  `"../config%s\n"`, and `runner`'s test runs `exec.Command("go", "run", "example.test/m/tool")`,
  trims the output and opens it. An edit of `config.go` selected `tool`, but `runner` had no
  witness.
- `TestRuleDReachesGetwdRunner_AFPV02990` (planner), round 3's friction case. `tool` calls
  `os.Getwd` and hands the root on. An edit of `elsewhere/data.txt` selected `tool` only.
- `TestRuntimeUncertaintyFollowsExecutionEdges_AFPV02991` (planner). None of `wrapper`, `outer`,
  `harness`, `consumer` and `testonly` was selected on an unrelated edit, and the runner of the
  test-less command `reporter` was not selected by an edit of `data/report.txt`.
- `TestGoRunRunnerParity_AFPV02990`, `TestGetwdRunnerParity_AFPV02990` and
  `TestExecutionEdgeParity_AFPV02992` (CI selector). Rule (d) was `[tool]`, cold, warm and in the
  index, and the verdict did not run the runners.

## Approach

The fix treats an execution edge like an import edge wherever a traversal follows code, and keeps
a test-only run one level past what it runs, as a test-only import already is.

1. **Split code runs from test runs (AFP-V0-2991).** The Go plugin records `testOnlyExecs`: the
   commands whose every use is in a `_test.go` file. It searches the sources first, and an unparsed
   source uses every candidate. A declared edge is never test-only. A runtime edge (AFP-V0-819)
   that the unit's code does not already run is test-only, because a test file holds it. In this
   repository, 99 of the 163 exec edges are test-only, and 16 units run a command from code.
2. **One traversal rule.** `traverseWhere` takes the code-run edges as a fourth class, after
   imports, advisory and route-dispatch edges, with a `BINARY_EXEC` witness. `testRunnersOf` adds a
   test-only runner one level past the walk. Rule (d) (AFP-V0-2990, AFP-V0-2991), the test-less
   reader walk (AFP-V0-2992) and the build-reach traversal of rules (a) and (b) (AFP-V0-2993) all go
   through it. Build reach was included because the CI selector's rules (a) and (b) already follow
   a non-test literal that names a package directory transitively (`namers`). Without it, `corvint
   test` could select fewer packages than CI.
3. **Keep the declarations' authority.** Two cases needed care:
   - A runner whose declared read scope excludes the path stays out of the test-less reader walk
     (AFP-V0-161), as rule (d) already leaves read-scoped units out. Without this,
     `conformance/cli-parity-v0` and `tools/ci-forward-row` were selected by a spec edit.
   - A unit that rule (d) reaches only through a test-only run is a terminal selection for plugin
     participation (AFP-V0-560). Its own code reads nothing unbounded. The first version let two
     Playwright specs that run root-locating Go commands (`conformance/interactive-alpha/fixture/e2e.spec.js`,
     `tools/web-flows/test/fixture/flows.spec.ts`) make the TypeScript plugin take part in every plan.
     A one-line documentation edit then went from 1 unknown to 67.
4. **The CI selector widens (AFP-V0-2994).** It now knows `package main` directories, carried in
   the scan memo as `command`. Any token that can name a command directory is a run: from code when
   a non-test file holds it, from tests otherwise. Tokens resolve as the plugin's `commandDirectory`
   resolves them, or through any of their component runs. The selector cannot see AFP-V0-402
   positions, so it may select more than the planner, never fewer. It reads
   `.corvint/test-binary-execs.json`, and on a declaration it cannot apply it makes every package a
   rule (d) unit.

Fault injection, one mechanism at a time, each reverted afterwards:

| Mechanism removed | Regressions that fail |
|---|---|
| code-run class of `traverseWhere` | `…FollowsExecutionEdges_AFPV02991`, `…BuildReachWalksCodeRunners_AFPV02993` |
| `testRunnersOf` in `unboundedReaders` | both `_AFPV02990` planner tests, `…FollowsExecutionEdges_AFPV02991` |
| read-scope bound of the test-less walk | `…TestlessReaderRunnersKeepReadScopes_AFPV02992` |
| source-first split (every exec counted as code) | `…FollowsExecutionEdges_AFPV02991` (`beyond` selected), `…ExecsSplitCodeFromTestRuns_AFPV02991`, `…BuildReachWalksCodeRunners_AFPV02993` |
| selector code runners in `runReach` | `TestExecutionEdgeParity_AFPV02992` |
| selector test runners in `runReach` | both `_AFPV02990` parity tests, `…EdgeParity_AFPV02992`, `…NeverNarrowerThanThePlanner_AFPV02994` |
| selector declaration (`applyBinaryExecs`) | `…EdgeParity_AFPV02992`, `…DeclarationWidens_AFPV02994` |

## Repository impact

Measured on 2026-11-07 on this branch's tree. Both binaries were built from it, the "before" one at
`c80f5059c` and the "after" one with this round's code. Each run was `corvint affected --worktree
--paths-from FILE --format json` over the path set of each of the last 12 first-parent merges of
`origin/main` (`git diff --name-only M^1 M`), plus the three canonical single-path changes. Units
are `plan.selected`; Go packages are `provider.go.packages`. The "manifest edges off" column comes
from a throwaway build, not committed, that skips the literal exec edges of `internal/langplugin`
and `internal/langplugin/decl` and changes nothing else.

| Change | Paths | Units | Go packages | Go packages, manifest edges off | Unknowns |
|---|---|---|---|---|---|
| `538e823f4` #124 claude/dogfood-friction | 28 | 402 → 447 | 129 → 172 (+43) | 134 | 74 → 74 |
| `4052b8540` #121 claude/invariant-7-bundled-apps | 19 | 308 → 316 | 52 → 64 (+12) | 48 | 76 → 76 |
| `b75053a5b` #115 claude/batch-1106c | 152 | 426 → 469 | 150 → 191 (+41) | 152 | 83 → 83 |
| `1a401eb5e` #120 claude/greptile-gate-2 | 11 | 311 → 327 | 47 → 61 (+14) | 44 | 75 → 75 |
| `d74f54b27` #112 claude/macos-cmd-corvint-shard | 10 | 288 → 299 | 39 → 54 (+15) | 36 | 71 → 71 |
| `539b55331` #118 claude/batch-1106ir | 15 | 319 → 331 | 67 → 80 (+13) | 63 | 77 → 77 |
| `000d4c56c` #117 claude/derive-cishards | 5 | 311 → 329 | 64 → 83 (+19) | 64 | 68 → 68 |
| `13342d85f` #116 claude/batch-1106d | 63 | 429 → 469 | 150 → 188 (+38) | 152 | 79 → 79 |
| `670559b9e` #114 claude/flows-command-pkg | 107 | 365 → 380 | 88 → 104 (+16) | 89 | 85 → 85 |
| `093750bb2` #113 claude/analyzer-requirements-dedupe | 21 | 348 → 425 | 92 → 167 (+75) | 97 | 69 → 71 |
| `044058f1b` #111 claude/ci-fast-cadence | 15 | 315 → 326 | 52 → 66 (+14) | 49 | 71 → 71 |
| `654df5053` #109 claude/breakage-command-pkg | 11 | 338 → 355 | 81 → 99 (+18) | 83 | 69 → 69 |
| `.github/workflows/ci.yml` only | 1 | 266 → 287 | 29 → 50 (+21) | 32 | 67 → 67 |
| `script/heavy.sh` only | 1 | 295 → 319 | 48 → 72 (+24) | 53 | 66 → 66 |
| `docs/TICKET-PRIORITY.md` only | 1 | 252 → 276 | 25 → 47 (+22) | 29 | 1 → 1 |

Over the 12 merges, units go from 4160 to 4473 (+7.5%) and Go packages from 1011 to 1329 (+31%).
With the manifest edges off, the same plans select 4155 units and 1011 Go packages. One edge class
therefore accounts for nearly all of the widening:

- `cmd/corvint-go-test-provider` imports `internal/liveverify/parentverify`, which locates the root
  through `--show-toplevel`.
- The language manifests in `internal/langplugin` and `internal/langplugin/decl` hold that
  command's directory as a `Paths` literal. Neither package runs a process.
- AFP-V0-402 counts a literal outside a comparison or a test message as a possible run.
- The rule (d) closure therefore walks from the manifests into `internal/contextindex` and some 30
  importers.

Neither analyzer can prove that no importer hands those paths to `go build`, so the edge stays
sound. Removing it needs a precision change (literal-to-process flow) or an owner declaration that
names the manifest literals as data. That is recorded as a follow-up, not done here.

Other effects:

- The two extra unknowns of `093750bb2` are `UNSELECTED_BINARY_CONSUMER`
  `tools/web-flows/test/e2e.mjs` and `navigation-e2e.mjs`. Build reach now walks a code run to
  `cmd/corvint-web-flows`, whose declared consumers (AFP-V0-762) the plan names. The CI selector
  admits that unknown (`BinaryConsumer`).
- The rule (d) floor in `.corvint/unbounded-readers.json` grows from 34 to 58 recorded units: 23 to
  45 Go packages, plus the two Playwright specs. 19 of the 24 new units come through the manifest
  edge. The other five each run a root-locating binary: `internal/tasks` runs `cmd/corvint`,
  `internal/criterionexperiment` runs `cmd/corvint-tasks`, `tools/ci-reuse-plan` runs
  `cmd/corvint-go-test-provider`, and the two specs run `cmd/corvint-console` and
  `cmd/corvint-web-flows` through declared runtime edges. The record states each chain and cause.
- The CI selector's own rule (d) set (`gate-affected-select -unresolved`) grows from 41 to 67
  packages. It holds every Go unit of the planner's floor but `conformance/ocm-v0`, whose cause is a
  shell chain the Go-only selector cannot see. That was already true at `c80f5059c`, and CI unions
  the plan's packages. 23 packages are selector-only, where the widening rule is broader than the
  plugin's.
- `TestPrecisionReplay_AFPV09003` counted the heavy.sh-only plan's language units against the
  reported 86. That count is now 104, of which 31 are rule (d) floor units, against 80 with 10
  before. The bound now applies to the units beyond the floor: 70 before, 73 after. The floor is
  held unit by unit by the AFP-V0-025 ratchet.

## Limits

- Readers bounded by a literal, a glob, a declared scope, an existence declaration or an import
  graph read still do not select their runners when they have tests of their own. AFP-V0-021
  already leaves their importers unselected.
- The runners of a command that only an enclosed path reaches stay one edge past it.
- The inbound runtime-caller closure (AZU-V0-004) gains code-run edges but no test-only level. Its
  seeds are TypeScript callers, which reach no Go command.
- `V1-4983`'s ticket body is not readable on this host (journal-absent, and the staged snapshot
  predates it). The round instructions and the dispatch text for the folded `binexec` lane served
  as its acceptance criteria.
