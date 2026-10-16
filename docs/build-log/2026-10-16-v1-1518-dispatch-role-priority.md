# V1-1518: dispatch role priority direction and shadowed-role warning

Date: 2026-10-16
Ticket: V1-1518 (bug), [issue 994](https://github.com/beamfall/corvint/issues/994)
Requirement: CAL-V0-290 (`docs/specs/corvint-tasks-agent-leases-v0.md`), new and accepted under the
owner's standing auto-accept of lane requirements (2026-10-11). No accepted requirement was amended.

## Problem

Issue 994: a configuration with a broad role `worker` (priority 50, match states `work`) and a narrow
role `special` (priority 92, `idGlob` `T-001[0-4]`, states `work`) never launched `special`. The
roster (`internal/tasks/dispatch/roster.go`) sorts candidates by pin, then role priority ascending,
then P-rank, plan order, key and role index. For one ticket only the role differs, so the lower value
always wins. The order was correct, but neither `Role.Priority`, the dispatch config reference in
`docs/TASKS-SUPERVISION.md` nor the spec said which direction wins, and nothing reported a role that
another role hides.

## Decision

- Document the direction in the three places named by the issue: the `Role.Priority` comment, the
  "Continuous dispatcher" config reference, and CAL-V0-290 in the leases spec. CAL-V0-054's order is
  unchanged and its accepted text is not amended.
- Add `Config.RoleShadows` (`internal/tasks/dispatch/shadow.go`). It is a conservative containment
  check over the closed `Match` predicate, written against `matches`/`stateMatches`/`gatesMatch`. It
  reports A only when an enabled match role B with a strictly lower value admits every ticket A
  admits. Uncertain containment answers false, so a report is never a false positive. Glob
  containment is proved only for an absent glob, equal globs, or a literal (wildcard-free) A glob that
  B matches. The work-state rule follows CAL-V0-053: a predicate that names states or exclusions
  never admits `UNKNOWN`, so B with states never covers A with none.
- Report as warnings, never refusals, so existing configurations keep loading. The dispatcher emits a
  `config` event (an existing kind) with outcome `ROLE_SHADOWED` at `Open` and after an applied
  CAL-V0-127 reload. `dispatch status` adds `roleShadows` and one result warning per shadow. Both read
  the configuration file, so status reports a shadow even when no dispatcher runs.
- The roster is untouched, and the tests pin that the repro still assigns `T-0011` to `worker`. A
  shadowed role still launches a ticket its shadow cannot take (shown with the shadow at its cap).

## Evidence

- Focused tests at the fix: `GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/tasks/dispatch ./internal/tasks/cli`
  passed. In `dispatch`, 242 passed and 2 were skipped, against 240 top-level tests at base
  `7fa7f6892` and 244 at the fix. In `cli`, 396 passed and 5 were skipped, against 400 tests at base
  and 401 at the fix. The five new tests are `TestCALV0290_*` in
  `internal/tasks/dispatch/shadow_test.go` and `internal/tasks/cli/dispatch_role_shadow_internal_test.go`.
  At base they do not compile, because `RoleShadow` does not exist there.
- `corvint affected --base 7fa7f6892` selected both packages through direct source changes, plus
  many units that read docs. Of those, `internal/specindex`, `internal/tasks/service`,
  `internal/breakagemap`, `internal/flowdocs`, `internal/doccompiler`, `script/spec-coverage-audit`
  and the traceability, requirement-definition and line-citation script tests were run and passed.
  `breakagemap` first hit its 38 s deadline at load average about 60. It passed alone and then as a
  whole package. Its witness is a fixture file literally named `shadow.go`. The remaining
  doc-reader units and `cmd/corvint` were NOT_RUN, under the owner's focused-test preference. The
  plan's language-frontier unknowns are retained.
- Mutations, each reverted:
  - comparing `b.Priority > a.Priority` reports equal-priority roles, and `ShadowCoverageIsConservative` fails;
  - dropping the `matchCovers` check makes three tests fail;
  - treating an `UNKNOWN` exclusion in B as narrowing makes the "shadow excludes UNKNOWN" case fail;
  - removing the start warning makes `ShadowWarningEvents` fail;
  - suppressing the status field makes `DispatchStatusReportsShadowedRoles` fail.
- `gofmt -l`, `go vet`, and Windows/amd64 and Linux/amd64 cross-vet of `internal/tasks/dispatch`
  and `internal/tasks/cli` all passed. The local doc-gates `line-citations-check`,
  `unbounded-readers-check`, `use-case-receipts-check`, `use-case-receipts-test`,
  `test-commands-check` and `spec-requirements-check` all passed.
- Independent review: one Codex round (`codex exec -s read-only`) over the base..fix diff reported
  no concrete findings. It found the coverage helpers conservative against `matches`,
  `stateMatches` and `gatesMatch`, and the roster order unchanged. The review was static, and Codex
  ran no tests.

## Residuals

- Equal-priority overlaps are not warned. The earlier role wins by index, which the configuration
  order shows. This stays within the lane's "strictly lower" definition.
- Shadows the check cannot prove are not reported. These include several roles that cover one role
  together, and general glob-into-glob containment.
- `dispatch explain` output is unchanged (out of scope).
