# 2026-10-15 — V1-1399 selector narrows read-scope declaration edits; V1-1400 (b) wording

## Contract

Owner approval in chat, 2026-10-15: "approve V1-1399 narrowing and the V1-1400 wording"
(decision 0528). `AFP-V0-023` now narrows a declaration edit for the planner and for the
selector. `AFP-V0-050` gains clause (4) for the `AFP-V0-012` selector (`internal/gateselect`, run
by `tools/gate-affected-select` and `corvint test`). The non-goal that kept them on the
`AFP-V0-023` rule is removed, and `--snapshot` stays a non-goal. The `AFP-V0-050` (3)
parenthetical is replaced by the full base condition that V1-1400 implemented: ownership by the
nearest `go.work`-listed module, a regular `go.mod`, a walk the observer makes, outside the
owner's testdata, and directly holding a regular `.go` file.

## Decisions

- **The base comes from the receipt, not a new argument.** The compact receipt that
  `script/gate-affected.sh` and `corvint test` already pass carries `range.base` (the full
  `--base` id) and `revision` (HEAD). `Select` reads `range.base`, else `revision`, so its
  signature and every caller stay unchanged. A receipt with a `snapshot` member, or a commit
  that is not a full object id, is an unknown base.
- **Standard library and git plumbing.** `tools/gate-ledger` copies `select.go`, `readers.go` and
  `readscopes.go` into an isolated fixture module and builds them, so `internal/gateselect`
  cannot import `internal/liveverify/affected/golang` or `gitauth`. The base reader therefore
  lives in `readscopes.go` and runs `git ls-tree -r -z --full-tree <commit>^{commit}` and
  `git cat-file blob`. It uses the hermetic environment `cmd/corvint` uses: no system or global
  config, no replace objects, no lazy fetch, no optional locks, a 30 s deadline, and bounded
  output (64 MiB tree, 1 MiB declaration, 4 MiB `go.work`). Limit: unlike `CommittedReadScopes`,
  it does not re-hash objects.
- **Parity, not a second rule.** `baseOwner`, `baseFixtureModule`, `walkedDirectory` and the
  `go.work` use grammar mirror `committedOwner`, `fixtureOfObservedModule`, the observer walk and
  `workspaceDirectories`. A test asserts agreement with `golang.CommittedReadScopes` on every
  case. A drift test keeps `skippedDirectories` equal to `affected.SkippedDirectories`, less the
  hidden names and the `build`, `dist` and `target` directories the Go plugin admits.
- **Removed keys.** A removed key is selected only while its directory is still a package in the
  observed index. A deleted package is out of the selector's reach, as for any other dirty path.
- **Dogfood base.** PR #922 merged (`ead9d5dcf`) before the first commit, so the branch was
  moved onto `origin/main` (`321a66f71`) and BASE is `origin/main`, as the charter says. The
  local merge of #922's head was never pushed.

## Evidence

The four new tests ran against BASE source (`origin/main` before #922's merge, plus #922's
head, `4b62edc64`) and against the fix, each three times, at load averages 47 to 68 on the
12-core host:

| Test | BASE | Fixed |
| --- | --- | --- |
| `TestSelectorNarrowsDeclarationEditAgainstThePlanBase_AFPV0050` (tool) | 3/3 FAIL: `declared [alpha beta gamma], want [gamma]` | 3/3 PASS |
| `TestCorvintTestNarrowsReadScopeDeclarationEdit_AFPV0050` (`corvint test --dry-run`) | 3/3 FAIL: argv `example.test/m/a …/b …/c` | 3/3 PASS |
| `internal/gateselect` (narrowing, parity, drift) | does not compile on BASE: no base reader | 3/3 PASS |

Mutations, one per check, each run on its focused test at load averages 37 to 55:

| # | Mutation | Result |
| --- | --- | --- |
| M1 | changed entries treated as unchanged | killed |
| M2 | removed key not selected | killed |
| M3 | package whose scope holds `.corvint/` dropped | killed |
| M4 | `--snapshot` guard removed | killed |
| M5 | `revision` preferred over `range.base` | killed |
| M6 | abbreviated commit admitted | killed |
| M7 | unknown base treated as an empty declaration | equivalent: every current key is then "added", which is the AFP-V0-023 rule |
| M8 | nested `go.mod` ignored | killed |
| M9 | testdata check dropped | killed |
| M10 | walk check (hidden or skipped) dropped | killed |
| M11 | regular `.go` requirement dropped | killed |
| M12 | `go.mod` regular-file check dropped | equivalent: a non-regular `go.mod` either owns nothing or lies in testdata, which is refused anyway; kept to mirror `committedOwner` |
| M13 | `go.work` ignored | killed |
| M14 | symlinked declaration read | killed; the first fixture linked to a path, whose blob never parses, so it survived until the link's target text became a valid declaration |
| M15 | symlinked `.go` counted | killed |
| M16 | `skippedDirectories` drifts | killed |
| M17 / M18 | selector never narrows | killed by the tool test and by the `corvint test` test |

Focused tests passed on the fix:
- `internal/gateselect`
- `tools/gate-affected-select`
- `tools/gate-ledger`, whose isolated fixture build covers the new standard-library imports
- `tools/corvint-pr-tests` (1603 s at load 42 to 61)
- the `cmd/corvint` subset `AFPV0050|STRV00|ReadScope|CorvintTest|AFPV0023|TestTest`

The new tests read only temporary directories, which the CI confinement (`.github/testconfine`)
grants, so `.corvint/test-read-scopes.json` is unchanged. `make gate` and the full
`go test ./...` are `NOT_RUN`, by the owner's standing preference for scoped issue work.
