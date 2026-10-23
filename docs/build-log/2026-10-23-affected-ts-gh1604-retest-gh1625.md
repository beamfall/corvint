# Blocked tsconfig alias gate and non-identity Playwright use values for GitHub #1604 re-test, #1625

Date: 2026-10-23. Lane: opencode/affected-ts-gh1604-1625 (base 7b3a8e55f). Requirements
`TJAA-V0-054..060` in `docs/specs/typescript-javascript-affected-adapter-v0.md` are accepted under
the owner's standing auto-accept (2026-10-11). They amend `TJAA-V0-018`/`035`/`048` (Playwright
identity and numeric conversions) and `TJAA-V0-037`/`050`/`052` (the alias gate and site bound).

## Decisions

- #1604 cause. The reporter's repository was not available (`NOT_OBSERVED`). A fixture of the
  re-test comment's tree selects correctly on the base revision: the exact compilerOptions with
  `baseUrl` last, the `include` globs, the sibling `tsconfig.typecheck.json`, the `tsc --init`
  comment layout, and the four example imports (index file, plain `.ts`, dotted basename). It was
  also checked with `tsc --traceResolution` (TypeScript 5.9.3). Key order, include patterns,
  index and extension probing, dotted basenames and the tsconfig choice are therefore not the
  cause, and `TJAA-V0-056` keeps them as a regression guard. The reported shape is 368 sites at
  importing modules, resolved at none, with only `lib/...` sites shown. That matches a blocked
  `TJAA-V0-037` gate, not a failed resolution. With the gate blocked, each module raised its first
  alias import through the undeclared-bare-import rule. The 16-site bound sorts `lib/...` first,
  so the tsconfig and blocker sites were hidden in `+352`. A synthetic probe reproduces this shape
  for a Playwright config that loads `tsconfig-paths`, a second runner config in a subdirectory,
  and another runner's test.
- `TJAA-V0-057`: a blocked gate now still adds the edges that tsconfig resolution names. The
  tsconfig and blocker frontier stays, so the plan is not bounded, but it selects the importing
  specs. This is a superset of the old edges and never removes an unknown. `TJAA-V0-058` lists the
  sites that explain a whole reason before the bound, in both the general frontier and the
  Playwright shared-graph detail. The pin is a reserved prefix on the stored token, so the frontier
  set's type and its other readers are unchanged.
- #1625 part 1 (`TJAA-V0-054`): a `use` key outside the `TJAA-V0-018` identity set no longer
  blocks the identity, whatever its value. This supersedes the #1605 lane's non-goal at the owner's
  direction in the issue. Invariant 2 still requires the uncertainty to be reported: a value that
  is not side-effect free could run code that rewrites a `devices` descriptor outside the config's
  view. It is named as a `playwright:use-value-unverified` unknown (execution-axis here, made
  selection-axis by the review repair below), with the same
  token, line and refused-root detail as `TJAA-V0-044`. A rewrite inside the config still refuses
  the devices root, so the GitHub #709 repros still block. The #709 purity tests now assert
  "unresolved or unverified" rather than "unresolved".
- #1625 part 2 (`TJAA-V0-055`): this reproduces the issue's exact message. A later
  `Number.isInteger(...)` refused the `Number` root, and the earliest refused occurrence was the
  `parseInt` member name of `Number.parseInt`. `Number.isInteger` is now an admitted conversion,
  and a member name after a code `Number.` is judged only as part of `Number`.

## Independent review

One read-only reviewer found no HIGH issue and one MED issue: pinned sites were still cut by the
bound, so 16 or more alias tsconfigs could hide the blocker again. Every pinned site is now named,
and the bound applies only to the rest. LOW fixes: the blocked-gate test asserts the plan is
`UNKNOWN` at the blocker, the non-primitive operand row declares its const, two stale comments
now cite `TJAA-V0-054`, the #1605 process rows assert the exact refused root, and the `Number.`
member check is a constant-time boundary test. Not changed: the pin is a NUL prefix on the
stored token. Real tokens stop at control bytes, and an import specifier containing a literal NUL
byte is recorded as a residual in `TJAA-V0-058`.

## PR #1628 review repair (`TJAA-V0-059`, `TJAA-V0-060`)

A second independent review found one P1, two P2 and one P3 issue. Each fix has a test that fails
on the reviewed head (ff3cddad7 + merge e8d743509).

- P1: an execution-axis `playwright:use-value-unverified` could not trigger a fallback. A
  `trace: (register({...}), 'on')` call from `tsconfig-paths` changes runtime module resolution, so
  a change the static graph does not connect could exclude its importing spec under a `BOUNDED`
  scope. `TJAA-V0-054` now makes the unknown selection-axis: the recovered identity is kept, the
  plan is `UNKNOWN` with the full-suite fallback, and a discovery receipt cannot bound it. The
  listing membership check tolerates it as it tolerates an unresolved identity. This supersedes
  the "unresolved or unverified" test relaxation above: the helper now requires a selection-axis
  unknown. Test: `TestPlaywrightSideEffectUseValueKeepsSelectionUnknown_PR1628`.
- P2 (workspace): under a blocked gate the tsconfig resolver replaced a bare reference, so a
  workspace package that tsconfig also maps lost its workspace-link edge. `TJAA-V0-059` keeps the
  original reference beside the tsconfig edge when it names a workspace package, so the selection
  is a superset of the gate's own. Test: `TestBlockedGateKeepsWorkspaceLinks_PR1628`.
- P2 (purity): a `++`, `--` or `delete` anywhere refused every root, the devices root included, so
  `navigationTimeout: counter++` blocked a device spread. `TJAA-V0-060` exempts the devices root
  when every such token lies in a non-identity `use` value that names no `devices` and has no `\`;
  `playwrightDevicesBlock` still refuses any update, delete or alias of a devices occurrence. The
  other roots stay refused, so the mutating value is the `TJAA-V0-054` selection unknown. Test:
  `TestPlaywrightConfinedMutationKeepsDeviceIdentity_PR1628`.
- P3: `locatedFrontier` re-sorted the rendered entries, putting the pinned sites back behind
  `lib/...` sites. Entries are now grouped by reason in sorted reason order and not re-sorted.
  The tests now assert positions. The language-neutral `affected-plan/0` unknown list remains its
  canonical sorted set, where every pinned site is present but not reordered; this is recorded in
  `TJAA-V0-058`.

## Evidence

- Base-revision reproduction: the new `playwright_gh1625_test.go` and
  `typescript_gh1604_retest_test.go`, run against 7b3a8e55f with constant shims, fail with the
  issue's refusal text and with the blocker hidden behind the bound. They pass after the change.
  The comment-tree test passes on both revisions.
- Focused tests: `go test ./internal/liveverify/affected/... ./internal/appflows/...
  ./internal/changee2e/... ./internal/jstestprovider/...` passed.
  `go test ./cmd/corvint -run 'Affected|Impact|Playwright|E2ESafe|AFP|Flows|AFU|TestPlan|Change'`
  passed.
- Not run: `make gate` and the full `cmd/corvint` package (lane charter). The dogfood CEM was not
  run. Live recall on the reporter's repository is `NOT_OBSERVED`.
