# Binary-boundary audit of cmd/ (2026-10-17)

Owner input, 2026-10-17, verbatim: "and while you are there we should make sure we are breaking up
the tooling correclty into appropriate binaries", then "maybe it could be called corvint-test as it
could cover all types of tests". One read-only audit (a Claude subagent at high effort) inventoried
`cmd/` at `19c54a3ab`. The orchestrator re-checked the claims marked *verified* against source; the
rest are the audit's static-grep findings and inferences.

## Findings

- `cmd/` holds `corvint` plus 52 companions. Every companion is a thin wrapper (9 to 1,640 non-test
  lines) over `internal/*`, with no copied code between mains. The split is shallow, so merges are
  mostly packaging work.
- *Verified:* nine binaries ship in the companion bundle (`internal/companionrelease/companionrelease.go:121-129`):
  corvint, console, dashboard-snapshot, mcp, docs-mcp, test-validity-mcp, js-test-provider,
  go-test-provider and tasks. `corvint-update` knows only `core` and `tasks`. Everything else is
  unshipped and used only from CI, scripts or development.
- *Verified:* the frozen core set is `init adopt index query context impact affected prove cem ocm
  frontier dogfood` (CCF-V1-001, `docs/specs/core-compatibility-freeze-v1.md:39`). About 37 other
  core verbs are unfrozen.
- Four unfrozen core paths execute user code: `test` (`internal/sessiontest/run.go:128`),
  `prove --mutate` (`cmd/corvint/prove.go:1289`), `test-validity negate`
  (`cmd/corvint/test_validity_negate.go:190`) and `work` (runner). These are the natural
  `corvint-test` candidates.
- *Verified:* `net/http` is in core's import graph through `internal/appflows` ->
  `internal/jstestprovider`. Whether any core command reaches it is unconfirmed.
- Four independent `go test` command builders exist (sessiontest, liveverify/gorunner,
  liveverify/mutate, testrunner/native).
- *Verified:* `internal/companionrelease/retained_test.go:142` classifies any `corvint-test*` name as
  test-validity-mcp, and core `corvint test` prints `corvint-test:` diagnostics.

## Decisions

- The CCE-V0 companion is `corvint-test`, with E2E as its `e2e` group; V0 adds nothing to core
  (CCE-V0-015). The two name collisions above are part of the first slice (V1-1665).
- Keep the shipped, accepted or server binaries as they are: tasks, update, mcp, docs-mcp,
  test-validity-mcp, console, dashboard-snapshot, lsp, pulse, remote-provider and work-queue. The
  shipped go/js test providers stay separate for now, because their names are wire `Source` values
  and an argv contract core calls.
- No binary moves in this change. Each restructure needs its own spec or decision:
  - V1-1667 (ideas): fold test-runner, tests-accept, behavior-falsify, playwright-minimize and
    optionally web-flows into `corvint-test` groups, and move the execution halves of core `test`,
    `test-validity negate` and `prove --mutate`.
  - V1-1668 (ideas): merge the unshipped families into `corvint-release`, `corvint-postmerge`,
    `corvint-corpus` and `corvint-analyzer <lang>`. cem-candidate plus cem-experiments was judged low
    value.
  - V1-1669 (optimizations): one internal `go test` runner package.
  - V1-1670 (questions): `net/http` reachability from core under invariant 7.
