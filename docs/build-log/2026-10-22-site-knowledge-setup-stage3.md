# sk-1022p: site knowledge MCP projection, typed keyboard steps, named joins and effects, and stage 3 setup

Date: 2026-10-22. Lane: claude/sk-1022p, carrying PR #1542 (stacked on merged #1535 f033f5fb2).
Tickets: V1-1898, V1-2405, V1-2406, V1-2407, V1-2408, V1-1900 (partial). Spec:
`docs/specs/site-knowledge-howto-v0.md`.

## Owner authority

- New requirements `SKH-V0-015`, `SKH-V0-016` (V1-2407) and `SKH-V0-024` (V1-2406) are accepted
  under the owner's standing auto-accept (2026-10-11). Collision check: `docs/specs/REQUIREMENTS.tsv`
  and `git grep` at origin/main hold no `SKH-V0-015`..`019`, `024`..`029` or `035`..`039`.
- The first adopter stays core-history (owner decision 2026-10-22), read-only: sampled at commit
  `6b429373a0337f305c253823db93ddba9cf33ddf` in a disposable clone, nothing run, no source copied.

## Problem

Stage 1 had no corpus MCP projection (`SKH-V0-013`). On the adopter, 230 steps were opaque.
Every one of the 10 page objects was unjoined, and 24 of 32 methods had an `unknown` effect, with
no reason given for either. Stage 3 (data setup) was not started.

## Decisions

- **MCP (SKH-V0-013, V1-1898, V1-2405).** There is one new corpus tool, `corvint.map_knowledge`,
  with `view` `page_objects|steps|setup`, `sources`, an optional `app` and `budget`. It is listed
  only when the planner's `--map` is configured. It returns the same bytes as the CLI as
  `structuredContent`, with the text framed as untrusted repository data. A schema violation is
  `InvalidParams`, and a coded failure is a tool error.
- **Typed keyboard and pointer steps (SKH-V0-024, V1-2406).** The IR adds the kinds `press`, `focus`
  and `hover`. `page.keyboard.press`, `type` and `insertText` target `keyboard`, and Capybara
  `hover` and `send_keys` are mapped too. Every other keyboard call stays opaque.
- **Named joins and effects (SKH-V0-015, 016, V1-2407).** An unbound page object joins through its
  methods' visit targets on the AMAP `page-object-url` basis. Otherwise it keeps a closed
  `join_reason` and, for an unresolved target, a `join_detail`. Method effects follow the
  ANL-V0-035 precedence, and every `unknown` names its `effect_reason`.
- **Setup (SKH-V0-030, 031, 033, 034, V1-1900 partial, V1-2408).** `flows appmap setup` and the
  `setup` view report:
  - `test.extend` fixtures across extend chains, with scope, option, auto and cleanup, plus
    `requires` and `unresolved` dependencies;
  - hooks;
  - storageState reads and writes, pinned to path and blob, with contents never read;
  - route mocks (always `mocked`), API seeding and `cy.task`.

  Each test links to the records that arrange it. Sources not read are named in `not_read`.
- **Profiles stay `/0`.** All three profiles are unreleased and experimental, so the new fields and
  kinds extend `/0` in place.
- **Duplicate top-level keys fixed.** The head counts `page_objects`, `tests` and `records` had the
  same names as their sections, so a JSON decoder silently dropped the counts. They are renamed to
  `page_objects_total`, `tests_total` and `records_total`. The CLI test now refuses a repeated
  top-level key.

## Evidence

- Focused tests:
  - `TestSKHV0_024_PressFocusAndHoverAreTyped`
  - `TestSKHV0_015_016_URLJoinAndNamedEffects`
  - `TestSKHV0_030_031_SetupRecordsAndTestLinks`
  - `TestSKHV0013CorpusMCPMapKnowledge`
  - `TestSKHV0_013_PageObjectsAndStepsCLI`
- Wider runs: `TestSKHV0|TestAMAP|TestANL` (internal/appmap) ok in 264 s, and
  cmd/corvint-corpus-mcp ok.
- The new tests do not build against the PR #1542 sources (`undefined: SKSetup`, `EffectReason`,
  `extractSiteKnowledge` arity). Two checks failed before their fixes during this lane:
  - the object-literal `storageState` link;
  - the duplicate-key check.
- Adopter sample, byte-identical on rerun:
  - Opaque steps fell from 230 to 154 (`parse-unsupported` 74, `dynamic-value` 40,
    `unresolved-call` 40). New kinds: `press` 54, `focus` 12, `hover` 1. Tests complete: 30 of 62.
  - Page objects: 10, with 32 methods. Effects: 2 `navigation-only`, 6 `state-effect` and 24
    `unknown`. The unknowns are `parse-unsupported` 14, `arranges-run-only` 8,
    `unresolved-call` 1 and `no-ui-actions` 1.
  - Joined: 0. Seven are `page-object-unbound`. Three are `page-object-unresolved-target`: 2
    `ambiguous-template` and 1 `dynamic-target`.
  - Setup: 12 records (11 `route-mock`, all `mocked`; 1 dynamic `storage-state`). Tests linked: 12
    of 62. Unresolved fixtures: 0.
- Cause of the zero joins: the app mounts under a router basename, and the map's catch-all screens
  have no specificity ranking.

## NOT_RUN and residuals

- `make gate` was NOT_RUN (owner preference for scoped work).
- Stage 3 remainder is NOT_STARTED:
  - FactoryBot, fixture files, seeds, setup endpoints and Playwright setup projects / config
    `storageState` / `globalSetup` (the `SKH-V0-032` reuse);
  - the `SKH-V0-031` condition query.
- V1-1901..V1-1903 (stages 4..6) are NOT_STARTED. Stage 4 amends the AMAP map edge contract and
  needs its own lane.
- V1-1236 is not delivered in ANL-V0. `SKH-V0-012` follows inherited methods and helpers for site
  knowledge only. The ANL-V0-035 edge effects still name these as `unknown`.
- AMAP residuals: a router-basename declaration, and splat-screen specificity ranking.
- Dogfood friction: the first bind refused `local-outcome` with `secret-shaped-path`. This entry's
  original lane-prefixed file name (`sk-` followed by more than 20 name characters) matched the
  secret screen's `sk-` API-key pattern. The entry was renamed; the false positive is a residual.

## Rollback

Revert this lane's commits. The profiles are experimental and derived, so no stored state
migrates. Removing `corvint.map_knowledge` only unlists the tool.
