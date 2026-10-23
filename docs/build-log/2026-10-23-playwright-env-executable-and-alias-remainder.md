# Playwright env-read executablePath and the #1604 alias remainder

Date: 2026-10-23. Lane: claude/affected-gh1640-1604 (base 0641ad2e0). Issues: GitHub #1640 and
#1604. Requirements `AFP-V0-382..390` (`docs/specs/affected-plan-v0.md`) are accepted under the
owner's standing auto-accept (2026-10-11).

## #1640: a Playwright executablePath read from process.env

Reproduction on base: a config whose shared `use.launchOptions.executablePath` is
`process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH` gave every project the TJAA-V0-035
`browser-executable` EXECUTION unknown. That unknown moves no test. The reporter's 0-selected
result came from the absent `--playwright-discovery` receipt (state `MISSING`), whose detail was
only the word `MISSING`.

Decisions:

- The caller supplies the observation, so a read command still reads nothing implicitly.
  `--playwright-env NAME=VALUE` records a supplied value, and `--playwright-env NAME` reads only
  that variable from the invoking environment. The plan records each named variable in
  `plan.environment` with its source, state, value and SHA-256, so two plans under different
  values differ visibly. Reading the whole environment, or a fixed list of variables, was
  rejected. It would make the plan depend on inputs nobody named, and a value is not repository
  evidence pinned to Git content.
- An unset variable is `undefined` in JavaScript. Playwright then launches its bundled browser,
  so this is recorded as `executable.bundled: true` rather than guessed. `??` and `||` with a
  string literal follow JavaScript semantics. An empty evaluated value stays unknown, because the
  profile does not model what Playwright does with an empty path.
- Missing evidence stays an unknown with a reason, never a resolution:
  - A variable the plan did not record keeps the unknown, and its detail names
    `--playwright-env NAME`.
  - Anything that can write the variable first also keeps the unknown: a module load other than
    `@playwright/test`, a `node:` built-in or `path`/`os`/`url`/`fs` (dotenv being the common
    case), a non-static load, or `globalSetup`.
  - The config's own writes already refuse the identity (TJAA-V0-042).
- The `MISSING` discovery detail now names the receipt and the `corvint affected discovery`
  command that writes it (AFP-V0-388).

Live check with the lane binary on a #1640-shaped fixture, with a discovery receipt:

| Input | Scope | Selected | Executable | Executable unknowns |
|---|---|---|---|---|
| `--playwright-env PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH`, variable unset | `BOUNDED` | 2 | `{bundled:true}` on both projects | none |
| `=/opt/chrome/chrome` | — | — | `path` recorded with source `ARGUMENT` and its digest | — |
| `=` (empty) | — | — | — | kept |
| No flag | — | — | — | kept; the detail names the variable |

## #1604: tsconfig paths aliases on base

Reproduction on base 0641ad2e0 with the issue's retest shape, a Playwright repository with
`lib/*` paths aliases and a `node:test` script:

```
corvint affected --path lib/helpers/TestTimingLogger.ts
```

The aliased importer `tests/a.spec.ts` was already selected (TJAA-V0-056), so that part of the
issue was already fixed. Three false-positive frontier sites remained:

1. `path-alias-unresolved:scripts/...test.mjs` with token `node-test`. A `node:test` script
   counted as a second runner identity, although nothing it loads names an alias.
2. `path-alias-unresolved:tsconfig.json` with token `"paths":`. This site is pinned because of
   (1).
3. `executable-config-unresolved:playwright.config.ts` with token `testDir`. The general adapter
   flagged a literal `testDir`/`projects` config, although the closed Playwright profile parses
   it exactly.

Decisions:

- AFP-V0-389 narrows TJAA-V0-050. A test of another runner blocks alias resolution only when its
  loader can meet an alias. That means a module in its relative-import closure names a bare
  reference that the nearest `tsconfig.json` claims. The check fails closed: any opaque closure
  module, workspace package or invalid tsconfig keeps the blocker. Runner configs block as
  before.
- AFP-V0-390 defers the Playwright config frontier until the tests are classified. It drops the
  frontier only when both of these hold:
  - The profile's parse leaves no membership unknown, with no `testMatch` or `testIgnore`.
  - Every file the profile assigns to a project is that config's Playwright test in the general
    adapter.

  So the general adapter's membership provably equals the config's.
- Three existing tests were adjusted, because they pinned blockers whose second-runner test
  imported no alias:
  - `TestGeneralAdapterPlaywrightMembershipAndAliases_GH1578`
  - `TestBlockedAliasGateSelectsAndNamesBlocker_GH1604`
  - `TestFrontierSitesNameFileLineAndToken_GH1604`

  Each fixture's test now reaches an alias. The GH1604 frontier fixture adds `testMatch`, so it
  still exercises the `testDir` config site.

After the change, the same command reports only the TJAA-V0-047 `e2e-runtime-dependency` sites,
and `tests/a.spec.ts` is still selected. Scope stays `UNKNOWN` through that frontier, which is a
separate design (an e2e test's runtime target is not statically observed) and is not part of
this change.

## Limits

- `NODE_OPTIONS` preloads, shell wrappers and `.env` files are not observed.
- A dotenv or `globalSetup` config keeps the executable unknown.
- The alias blocker token stays the runner name.
- The `e2e-runtime-dependency` frontier still holds the #1604 plan at `UNKNOWN`.
