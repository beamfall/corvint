# TypeScript / JavaScript affected-test adapter V0

Owner: Russell Lewis
Date: 2026-08-29
Intent status: proposed
Delivery status: experimental
Authoritative inputs: operator task of 2026-08-29,
[`live-proof-carrying-verification-v0.md`](live-proof-carrying-verification-v0.md),
[`../DOGFOOD.md`](../DOGFOOD.md)

## Agent digest
- Claim: Static JavaScript and TypeScript analysis conservatively maps changes to runner-addressable tests and widens on ambiguity.
- Status: proposed/experimental
- Exists: `internal/liveverify/affected/typescript` static runner/dependency adapter and seam tests.
- Blocked on: runtime tuple, command/config execution, cancellation, and full-CI recall qualification.
- Read next: Requirements; Explicit unknown frontier; Traceability.

## User and measurable job

For changes to `.ts`, `.tsx`, `.js`, `.jsx`, `.mjs`, and `.cjs` files, Corvint should discover every
statically identifiable JavaScript/TypeScript test file, preserve its runner identity, and connect
it to local modules and framework configuration without executing repository code. Ambiguous
discovery or dependency reachability must widen the affected plan instead of looking like a safe
exclusion.

This experimental source observer is not a qualified runtime provider. Framework/runtime/OS tuples,
exact command execution, config interpretation, cancellation, and full-CI recall remain `NOT_RUN`.

## Requirements

- `TJAA-V0-001`: `Owns` MUST claim exactly `.ts`, `.tsx`, `.js`, `.jsx`, `.mjs`, and `.cjs`; it MUST
  NOT claim Gherkin, YAML, JSON, MDX, CoffeeScript, or host-independent E2E assets. An observed
  `.mts` or `.cts` file (any case) stays unowned and unread and MUST raise `typescript:unparsed-source`,
  because it may import an owned module that no unit edge then records.
- `TJAA-V0-002`: A normally file-addressable test MUST be one file unit. Its stable identity MUST
  include the `typescript:` namespace, detected runner, and repository-relative path. Legacy
  Storybook test-runner stories MUST instead form one configured story-set unit because that runner
  has no story-file positional address.
- `TJAA-V0-003`: Runner detection MUST use direct runner imports/registrations, recognized config,
  or package/runtime manifest evidence combined with test discovery conventions. An owned source
  below a top-level `test/` or `tests/` directory is a test candidate; absent or unrecognized runner
  evidence MUST assign the unknown runner and raise `typescript:test-runner-unknown`. A test-like
  name alone MUST NOT select a named runner. A runner dependency in the nearest manifest MUST keep
  that runner's conventional test directories (`__tests__` for Jest; `cypress`, `playwright`, `e2e`,
  `spec`, `specs` for browser runners) test candidates even without configuration; the dependency
  alone assigns the unknown runner rather than silently classifying the file as source.
- `TJAA-V0-004`: Vitest, Jest, AVA, `node:test`, Playwright Test, Bun test, Deno test, Cypress,
  WebdriverIO, TestCafe, Nightwatch, Detox, legacy Storybook test-runner, and Storybook's Vitest
  addon MUST remain distinct runner identities. A Puppeteer import MUST identify browser automation,
  not invent a runner; its host runner remains authoritative.
- `TJAA-V0-005`: Static relative `import`, re-export, and `require` edges MUST resolve extensionless,
  explicit-extension, and directory-index forms to observed file units. Missing relative edges,
  computed loading, path aliases, executable discovery configuration, and unparsed source MUST add a
  deterministic language frontier. A dynamic `import` call MUST count as computed loading whatever
  whitespace or comments separate `import` from its parenthesis. When scanning for comments and
  `require`, each `${...}` substitution in a template literal MUST be scanned as code while the
  template's literal text stays opaque. A plain `'`- or `"`-quoted token that crosses a line in a
  `.jsx` or `.tsx` source, or whose delimiter does not follow a statically recognized JavaScript or
  TypeScript literal-opening context there, MUST raise `typescript:unparsed-source`, because V0
  cannot distinguish it safely from apostrophes or quotes in JSX text. Recognized contexts are the
  start of a line; the punctuators `( [ = { , : ; ! ? & | + - * % ^ ~`; `=>`; and the keywords
  `as`, `await`, `case`, `default`, `delete`, `do`, `else`, `export`, `from`, `import`, `in`,
  `instanceof`, `new`, `of`, `return`, `throw`, `typeof`, `void`, and `yield`. Even after one of
  those contexts, a quoted token spanning an identifier-token `require` MUST raise the same
  frontier rather than hide a possible JSX-text delimiter. A `/` that is not a comment start MUST
  read as one regular-expression literal, whose
  quotes and backticks open nothing, only when it closes on its line and follows nothing or one of
  `( , = : [ & | ? { ; ~ ^ % *` (whitespace ignored); every other `/` reads as division.
  A line-leading `/// <reference path="...">` directive MUST resolve as a relative edge, its path
  read relative to the file whether or not it starts with `./`.
- `TJAA-V0-006`: Owned framework config MUST be an affecting source unit for every governed test.
  Non-owned manifests/config and project, environment, loader, permission, browser, device, tag, or
  setup values that this observer cannot reconstruct MUST remain unknown.
- `TJAA-V0-007`: Multiple runners/configs that can own one physical test path MUST produce one
  `unknown` test unit plus an ambiguity frontier. The adapter MUST NOT choose a winner or emit two
  units that violate the seam's unique path ownership rule.
- `TJAA-V0-008`: Playwright/Cypress/WebdriverIO/TestCafe/Nightwatch/Detox/Puppeteer tests without a
  qualified static application binding MUST retain an E2E runtime-dependency frontier. V0 does not
  qualify local imports as complete black-box reachability. Detox configuration matrices and legacy
  Storybook story-set addressing MUST remain explicit frontiers.
- `TJAA-V0-009`: Discovery and canonical graph bytes MUST be deterministic for fixed repository
  authority, and every discovered eligible test MUST participate in selection or exclusion under
  `LPCV-V0-013`, `LPCV-V0-014`, `LPCV-V0-016`, and `LPCV-V0-019`.
- `TJAA-V0-010`: The opt-in `playwright-affected/0` profile MUST bind the config path and SHA-256,
  every statically declared project name, project `grep`/`grepInvert`, `testDir`, `testMatch`,
  `testIgnore`, dependency and teardown edges, browser/device identity, and the exact project argv.
  The existing `affected-plan/0` bytes and one-path/one-unit ownership rule MUST remain unchanged.
- `TJAA-V0-011`: One selected Playwright unit MUST identify one physical test file under one
  project. A physical file may therefore produce several profile units without becoming several
  owners in the shared V0 graph. Unit IDs and all arrays MUST be sorted and deterministic.
- `TJAA-V0-012`: Static config support is deliberately closed. Project arrays and membership fields
  MUST be literals; `testMatch`/`testIgnore` MAY be literal strings or regular expressions; device
  spreads MAY name a literal `devices[...]` descriptor. Imported/computed projects, generated test
  lists, functions, environment branches, feature flags, unknown device descriptors, unresolved
  browser identity, malformed regular expressions, and dependency cycles MUST widen to
  `FULL_RELEVANT_SUITE` with a typed selection unknown.
- `TJAA-V0-013`: A changed source, page object, fixture, scenario builder, or helper MUST select the
  reverse-import closure's Playwright tests under every statically applicable project. A changed
  config selects the full relevant suite. Selecting a setup project MUST also select every unit in
  its transitive dependent projects; a selected dependent project MUST select its dependency units
  and teardown unit. No unknown may remove a unit.
- `TJAA-V0-014`: Project grep/tag and metadata inputs are identity, not file-exclusion authority.
  This static profile MUST NOT exclude a file merely because its cases cannot be proven to match a
  project grep. Runtime feature flags and externally managed application state are execution-axis
  unknowns; they remain visible but do not by themselves claim the selection axis is incomplete.
- `TJAA-V0-015`: When a selection-axis unknown exists, `scope` MUST be `UNKNOWN`, fallback MUST be
  `FULL_RELEVANT_SUITE`, and every statically discovered Playwright test/project pair MUST be
  selected. If the project set itself is unknown, the profile MUST emit no runnable unit and state
  that the caller must run the complete Playwright configuration.
- `TJAA-V0-016`: The profile MUST be read-only and bounded by the shared source-walk and source-read
  limits. It MUST bind and revalidate a digest of every source input the TypeScript observer may
  consume. For fixed repository bytes, config path, HEAD, and dirty set, canonical output MUST be
  byte-identical. Invalid paths, unreadable config, source drift, or Git drift fail closed without a
  partial receipt.
- `TJAA-V0-017`: Qualification requires a fixture with Chromium plus Angular/React project variants,
  setup dependencies, grep/metadata identity, and shared page-object/fixture/scenario edges. It MUST
  prove config and setup widening, dynamic-import/config widening, repeated-byte identity, and zero
  unsafe narrowing before the profile can be promoted from experimental.

## Opt-in Playwright project profile

`corvint affected --playwright-config PATH` emits `playwright-affected/0`; it does not alter the
closed `affected-plan/0` receipt. The profile reuses the shared TypeScript import graph only for
physical path ownership and reachability, then expands reached Playwright files into project units.
This keeps project multiplicity out of the language-agnostic graph while still binding each runnable
unit to its config and project inputs.

The supported static subset follows Playwright's project contract: dependencies run setup projects
first, teardown projects run after their setup/dependent cohort, project grep is retained as runtime
case filtering, and project `testMatch`/`testIgnore` controls file membership. A form outside the
closed subset is not approximated. It widens the selection or, when projects cannot be identified,
abstains from runnable units.

## Detection and runner addressing

### Playwright fixture qualification boundary

`TestPlaywrightQualification` in
`internal/liveverify/affected/typescript/playwright_qualification_test.go` checks the repository-owned
`testdata/playwright-qualification.tsv` inventory: 117 test files, four cases per file, nine feature
cohorts, and Chromium/Angular/React file variants plus setup/cleanup. This is synthetic source-graph
qualification under `TJAA-V0-017`, not execution of Playwright or recall measured in a consumer repo.
The manifest is checked against generated source independently of the selector; expected selections
are enumerated from cohort ranges, never from its graph or exclusions. The seven static change
cases require 1,187 units in total and reject both missing and extra units. Dynamic-source and
unknown-membership cases require the full independently enumerated 353-unit baseline as a subset;
additional conservative units remain permitted. Unknown project sets require full-config fallback
with zero runnable approximations. Every case repeats canonical serialization for identical inputs.

Retained-provider composition uses the existing JavaScript reporter parser and `ProjectPinned`:
matching source/config bindings still leave E2E freshness unknown, mismatches and stale app builds
are stale, missing source stays unknown, and ambiguous envelopes are refused. Passed execution never
supplies mutation strength or closes the selection receipt's external-application frontier. The
current provider has no project-aware result join; issue #19 owns that dependency. This fixture
qualification does not promote the runtime provider, qualify full-CI recall, or assert case-level
recall from file-level selection. The general adapter remains proposed/experimental.

The command column is the runner's exact file or story-set form once the named placeholders have
been resolved from provider configuration. V0's `affected.Language` result has no structured argv,
working-directory, runner, config, project, tag, loader, browser, permission, or device field, so
these commands are documented conventions and are not machine-carried by this seam.

| Runner | Decisive source evidence | Repository/config evidence | Address once provider inputs are known |
|---|---|---|---|
| Vitest | import from `vitest` | `vitest.config.*`; Vitest-specific `vite.config.*`; package dependency corroborates presence only | `npx vitest run --config C [--project P] FILE` |
| Jest | import from `@jest/globals` | `jest.config.*`; package `jest` field/dependency | `npx jest --config C [--selectProjects P] --runTestsByPath FILE` |
| AVA | import from `ava` | package dependency corroborates presence only | `npx ava FILE` |
| `node:test` | import/require `node:test` | no manifest-only ownership | `node [LOADER_FLAGS] --test FILE` |
| Playwright Test | import `@playwright/test` | `playwright.config.*` | `npx playwright test FILE --config=C [--project=P]` |
| Bun test | import `bun:test` | `bunfig.toml` | `bun test ./FILE` |
| Deno test | `Deno.test(...)` | `deno.json[c]` | `deno test --config C [PERMISSION_FLAGS] FILE` |
| Cypress | import `cypress`; config-scoped `.cy.*`/`cypress/` file | `cypress.config.*`; package dependency corroborates presence only | `npx cypress run --e2e --spec FILE --config-file C` |
| WebdriverIO | import `@wdio/globals` | `wdio*.conf.*`; package dependency corroborates presence only | `npx wdio run C --spec FILE` |
| Puppeteer | import `puppeteer[-core]` corroborates E2E only | host Jest/Vitest/node runner evidence | host runner's file command; no Puppeteer test command exists |
| TestCafe | import `testcafe` | `.testcaferc.*`; package dependency corroborates presence only | `npx testcafe BROWSERS FILE --config-file C` |
| Nightwatch | import `nightwatch` | `nightwatch.conf.*`, `nightwatch.config.*`, `nightwatch.json` | `npx nightwatch FILE --config C [--env E]` |
| Detox | import `detox`; Detox Jest environment | `.detoxrc*`, `detox.config.*`, package `detox` | `npx detox test -c DEVICE_CONFIGURATION FILE` |
| Storybook test-runner | CSF `*.stories.*` under package evidence | `@storybook/test-runner`; `.storybook/test-runner.*` | `npx test-storybook [--url URL]` for the complete story-set unit |
| Storybook Vitest addon | import `@storybook/addon-vitest/vitest-plugin` | addon plus named Vitest project | `npx vitest run --project=P FILE` |

Package dependencies establish repository-level presence but, except for explicit Jest or Detox
configuration and Storybook's project-wide runner, do not by themselves assign every test-like file
in a monorepo to that runner. Package scripts are not ownership evidence: wrapper commands can
delegate to another package, configuration, or dynamically computed file set. Puppeteer is
supported as a dependency/E2E marker while the detected host runner owns the test.
Config membership in V0 is conservative: recognized executable config establishes runner presence,
while non-default/computed discovery keys add `typescript:executable-config-unresolved`; V0 does not
execute or fully interpret arbitrary config programs.

The shared repository walker skips dot-directories. Consequently `.storybook/test-runner.*` cannot
currently establish legacy Storybook presence by itself, and a dirty hidden config cannot become an
owned affecting source. Package dependency evidence still enables the addressable story-set unit;
config-only legacy Storybook repositories remain explicitly unsupported rather than silently
claimed. Resolving this requires a reviewed change to the shared walker or auxiliary-input model,
which is outside this plugin-only slice.

## Explicit unknown frontier

The adapter reports fixed frontier reasons for unreadable/unparsed files, dynamic `import`/computed
`require`, unresolved relative imports, undeclared/path-aliased bare imports, ambiguous or unknown
runners, executable config, TypeScript `node:test` loader flags, black-box E2E reachability,
Puppeteer without a host, Detox device/build configuration, legacy Storybook addressing, and
observed cross-language test assets. A test file that generates cases with `test.each`, loops, or
reflection is still file-addressable and does not need case-level discovery. Generated test files,
wrapper-computed file lists, imported config fragments, and custom loaders remain unknown because
the source observer cannot establish their eligible file universe.

## Cross-language ownership decision required

The present seam assigns each path to at most one language unit. Gherkin `.feature` files are bound
to steps in any host language; Maestro flows are YAML; Appium bodies may be JavaScript/TypeScript,
Python, Java, or another client language; Storybook may discover `.mdx`; TestCafe may discover
`.testcafe`/CoffeeScript; and Cucumber-backed WebdriverIO/Nightwatch projects may mix `.feature`
files with JS/TS steps. These are verification inputs, but none has a unique language owner.

If two plugins claim one such path, `affected.Build` fails with `ErrDuplicateOwner`, so the graph,
witness chain, and exclusion universe cannot be produced. If no plugin claims it, a dirty edit
correctly becomes `UNOWNED_DIRTY_PATH`, but the asset cannot itself be an eligible unit. A reviewed
resolution would need to define shared auxiliary-input ownership, whether one path may seed several
provider units, cross-provider identities, deterministic collision/digest rules, selection and
exclusion semantics, and how config-only assets invalidate discovery. V0 leaves these paths unowned
and does not widen the interface or choose a language precedence.

## Acceptance evidence and rollback

- Focused unit tests cover all named runners, coexistence, import resolution, config edges, dynamic
  and cross-language frontiers, legacy Storybook aggregation, and the six owned extensions.
- The common seam conformance fixture covers dependency closure, witnesses, exclusions, unknown
  widening, deterministic bytes, namespace composition, and cross-language independence.
- At least one independently maintained real repository must report exact unique-file discovery
  recall, false positives, framework-label mismatches, and explicit Unknown files.
- Full Go build/test/vet and frozen CLI parity must remain byte-compatible.

Rollback removes `internal/liveverify/affected/typescript`, its conformance fixture/case, and this
experimental spec. No persisted format, CLI registry, or existing receipt is changed.

## Traceability

| Requirement | Implementation / evidence | Status |
|---|---|---|
| `TJAA-V0-001..004`, `TJAA-V0-006..008` | `internal/liveverify/affected/typescript/` focused tests | experimental |
| `TJAA-V0-005` | `TestTemplateSubstitutionRequireBuildsDependencyEdge`, `TestMultilineJSXQuoteAmbiguityRaisesFrontier`, `TestSameLineJSXApostropheAmbiguityRaisesFrontier`, `TestStandaloneJSXApostrophesRaiseFrontier`, `TestJSXTextCannotImitateALiteralOpeningContext`, `TestKeywordEndingJSXTextCannotHideRequireWithoutBraces`, `TestJSXAttributeAndExpressionStringsRemainParsed`, `TestOrdinaryTSXStringsAndJSXExpressionLiteralsRemainParsed`, and import-resolution focused tests in `internal/liveverify/affected/typescript/` | experimental |
| `TJAA-V0-009` | `internal/liveverify/affected/conformance_test.go` TypeScript seam case | experimental |
| `TJAA-V0-010..017` | `internal/liveverify/affected/typescript/playwright.go`, `playwright_test.go`, and `cmd/corvint/affected_playwright_test.go` | experimental |
| `TJAA-V0-014..017` fixture qualification | `internal/liveverify/affected/typescript/playwright_qualification_test.go`, `testdata/playwright-qualification.tsv` | synthetic fixture evidence; runtime promotion excluded |
| independent real-repository recall | 2026-08-29 build-log evidence | observed |
| runtime/framework/OS qualification | `LPCV-V0-043..046` promotion matrix | `NOT_RUN` |
