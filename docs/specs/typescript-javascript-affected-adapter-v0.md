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
- `TJAA-V0-004`: Vitest, Jest, AVA, Mocha, `node:test`, Playwright Test, Bun test, Deno test, Cypress,
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
  (proposed, decision 0398; V1-0283) A bare import whose package (`@scope/name` or `name`, the
  first one or two path segments) is the `name` of a `package.json` in the repository MUST NOT be
  a frontier by that fact alone: it resolves to one unit `typescript:workspace:<directory>` per
  such `package.json` directory, built only when something imports it, which imports every source
  unit inside that directory. The importer gets an edge to that unit, so a change to any source of
  the package reaches a test in another workspace package that imports it by name. The edge set is
  a superset of what the package entry (`main`, `exports`, subpath imports) can reach, because V0
  does not resolve entries. A named package with no source unit inside its directory, or with more
  than 20,000, MUST keep `typescript:path-alias-unresolved`, so a test that imports it is never
  claimed covered. Rollback restores the frontier for every workspace-package import.
- `TJAA-V0-006`: Owned framework config MUST be an affecting source unit for every governed test.
  Non-owned manifests/config and project, environment, loader, permission, browser, device, tag, or
  setup values that this observer cannot reconstruct MUST remain unknown. Recognized Mocha
  `.mocharc.js`, `.cjs`, `.mjs`, `.json`, `.jsonc`, `.yaml` and `.yml` configurations and
  manifest `mocha` configuration may identify the runner but retain a configuration frontier:
  require/loader/spec inputs can broaden discovery. Mocha TypeScript and TSX execution
  flags remain unknown; Chai alone, dependency-only and scripts-only evidence cannot select Mocha.
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
  spreads MAY name a literal `devices[...]` descriptor. The opt-in profile MUST apply literal
  global `use` defaults before project `use` overrides, preserve browser/device identity, and bind
  the inherited input in the project fragment identity. Other `use` values MUST be static literals,
  except as `TJAA-V0-018` (proposed) admits for options outside the browser/device identity set.
  It MAY resolve nearest-ancestor `tsconfig.json` JSON/JSONC `baseUrl` and `paths` declarations:
  exact keys precede wildcard keys and longest wildcard prefixes precede shorter prefixes. Overlapping
  equal-prefix patterns or target lists with multiple existing candidates MUST widen rather than
  approximate runtime precedence. Device defaults MUST NOT override an explicit inherited browser.
  Explicit `use.defaultBrowserType` MUST widen; only recognized device descriptors supply defaults.
  Resolution MUST stay inside the observed repository, and MUST
  widen on missing declared targets, ambiguous source candidates, config inheritance, unsupported
  root-directory/module-suffix resolution, or explicit Playwright `tsconfig` overrides. The general
  `affected-plan/0` adapter MUST retain its existing unresolved-alias frontier.
  Imported/computed projects, generated test
  lists, functions, environment branches, feature flags, unknown device descriptors, unresolved
  browser identity, malformed regular expressions, and dependency cycles MUST widen to
  `FULL_RELEVANT_SUITE` with a typed selection unknown.
  Candidate project/file pairs MUST come only from project `testDir`, `testMatch`, and `testIgnore`
  over observed owned sources. Other files, including fixtures importing Playwright, remain dependency
  graph sources; they MUST NOT become commands merely because they occur under a test directory.
- `TJAA-V0-013`: A changed source, page object, fixture, scenario builder, or helper MUST select the
  reverse-import closure's Playwright tests under every statically applicable project. A changed
  config selects the full relevant suite. Selecting a setup project MUST also select every unit in
  its transitive dependent projects; a selected dependent project MUST select its dependency units
  and teardown unit. Literal `globalSetup`/`globalTeardown` paths MUST attach their transitive source
  dependencies to the config so hook/helper changes select every configured test. Missing or computed
  hook paths MUST widen. No unknown may remove a unit.
- `TJAA-V0-014`: Project grep/tag and metadata inputs are identity, not file-exclusion authority.
  This static profile MUST NOT exclude a file merely because its cases cannot be proven to match a
  project grep. Runtime feature flags and externally managed application state are execution-axis
  unknowns; they remain visible but do not by themselves claim the selection axis is incomplete.
- `TJAA-V0-015`: Per-file commands require a matching complete caller-owned discovery receipt.
  Missing, malformed, stale, mismatched, or statically unresolved discovery MUST produce `UNKNOWN`,
  `FULL_RELEVANT_SUITE`, empty selected/excluded file rows, and exactly one `fallbackArgv`:
  `npx playwright test --config=PATH`. With a matched universe, unresolved source reachability MUST
  select exactly the complete discovered universe, preserving setup/teardown closure and all
  uncertainty; it MUST NOT multiply helpers by projects. `discovery` MUST report its state,
  input SHA-256, and sorted `onlyInStatic`/`onlyInReceipt` differences when bindings match.
- `TJAA-V0-016`: The profile MUST be read-only and bounded by the shared source-walk and source-read
  limits. It MUST bind and revalidate a digest of every source input the TypeScript observer may
  consume. For fixed repository bytes, config path, HEAD, and dirty set, canonical output MUST be
  byte-identical. Invalid paths, unreadable config, source drift, or Git drift fail closed without a
  partial receipt. Discovery input MUST be at most 4 MiB, canonical JSON (optionally one trailing LF),
  rejecting duplicate/unknown/missing members, noncanonical paths, unsupported profile, and unsorted
  or duplicate project/file pairs. Bindings MUST include immutable HEAD, config path/SHA-256 and
  current source digest. The CLI MUST re-read discovery bytes before emission and refuse drift.
- `TJAA-V0-017`: Qualification requires a fixture with Chromium plus Angular/React project variants,
  setup dependencies, grep/metadata identity, and shared page-object/fixture/scenario edges. It MUST
  prove config and setup widening, dynamic-import/config widening, repeated-byte identity, and zero
  unsafe narrowing before the profile can be promoted from experimental. Qualification MUST use
  independently enumerated discovery inputs, prove helper-only exclusions and exact universe
  reconciliation, and exercise absent/stale/malformed/missing/extra receipt pairs and one-command
  fallback. Real `--list` evidence and synthetic membership oracles MUST keep their distinct labels.
- `TJAA-V0-018`: (proposed, pending owner acceptance; V1-1065; GitHub #709) In a global or project
  `use` layer, the browser/device identity options MUST have static literal values: `browserName`,
  `defaultBrowserType`, `channel`, `headless`, `connectOptions`, `viewport`, `screen`, `userAgent`,
  `isMobile`, `hasTouch`, `deviceScaleFactor`, `locale`, `timezoneId`, `colorScheme`, `permissions`,
  `contextOptions`, `launchOptions`. This is the same set the external provider's qualified reporter
  resolves at runtime (`internal/jstestprovider`), and a test MUST keep the two lists equal. An
  identifier or quoted string key is the same key, so `'browserName': 'firefox'` resolves like
  `browserName: 'firefox'`. A computed value of any other option (`baseURL`, `storageState`, `trace`,
  ...) MUST NOT leave the project's browser identity unresolved when its evaluation syntactically
  cannot call user code: literals, identifiers, member reads (including optional chaining) of an
  admitted root, array literals, object literals of `key: value` or shorthand properties, and the non-coercing operators
  `===`, `!==`, `&&`, `||`, `??`, `?:` (test and branches), `!`, `typeof` and `void` over those.
  Template substitutions, computed member and property keys (`obj[expr]`, `{ [expr]: v }`), unary
  `+`, `-` and `~`, and every other binary operator (arithmetic, bitwise, shifts, `<`, `>`, `<=`,
  `>=`, `==`, `!=`, `in`, `instanceof`) convert their operands and could call a user `toString`,
  `valueOf`, `Symbol.toPrimitive` or `Symbol.hasInstance`, so they MUST admit only operands proven
  primitive: string and numeric literals, templates whose substitutions are proven primitive (a
  template without substitutions included), `true`, `false`, `null`, the results of these operators
  and of `&&`, `||`, `??` and `?:` over such operands, and the results of `typeof`, `void`, `!`, `===` and `!==`, which are always
  primitive. A member read can invoke a getter or Proxy trap, so it is admitted only when its root
  identifier is (a) the `devices` binding of a top-level `import { devices } from '@playwright/test'`,
  read as `devices['<name>']` (a string-literal key that is not an `Object.prototype` name and does
  not start with `__`) followed by `.userAgent`, `.deviceScaleFactor`, `.isMobile`, `.hasTouch`,
  `.defaultBrowserType` or `.viewport`/`.screen` then `.width`/`.height`, or (b) a top-level
  `const NAME = { ... }` declared once whose properties, recursively, are all plain `key: value`
  with an identifier or string key (no `get`/`set` accessor, method, spread, computed key,
  shorthand, duplicate key or `__proto__`) and proven-primitive or plain-object values, read through
  dotted or string-literal keys that exist down to a primitive leaf. Every occurrence of either root
  in the file, strings included, MUST be such a read in a read-only position (never an assignment,
  update, `delete` or destructuring target, alias, call or argument of a call such as
  `Object.defineProperty`); a parenthesized occurrence gets the same checks, so a
  parenthesized assignment, compound assignment, destructuring or for-in/of target is refused. A
  file containing the token `eval`, or in code (outside string, template and regular-expression
  content) a `delete`, `++` or `--` token or any `\` (only an escaped identifier holds one there),
  admits neither root, whatever that token's operand. Every other
  member read (`process.env.NAME`, `this.x`, a member of an import from another module, a function
  result or a numeric literal such as `1..x`) MUST keep the identity unresolved. No member read is
  proven primitive: an admitted one is accepted only as a whole value and under the non-coercing
  operators. Numeric literals MUST be lexed
  to the ECMAScript grammar (decimal with fraction and exponent, `0x`/`0o`/`0b`, `_` between digits,
  the BigInt `n` suffix on integers); a legacy octal, a malformed literal or one followed directly by
  an identifier character is refused, and a dot after a complete literal starts a member read. `++`
  and `--` are update operators, never two signs, and nesting is bounded with right-associative `**`
  included; the static-literal check of an identity value has the same nesting bound. Comments and
  lines end at every ECMAScript line terminator (LF, CR, U+2028, U+2029) outside string, template
  and regular-expression content. Template literals MUST be lexed with their nesting: a backtick in a
  `${}` substitution opens a nested template, the substitution's matching `}` resumes the enclosing
  one, and template content is never treated as a comment or rewritten; nesting deeper than 64
  open substitutions is refused as unparsed source. A literal
  `...devices['<known name>']` spread beside such options MUST resolve to that device's browser
  when the file's `devices` root is admitted under (a); otherwise the spread widens. A
  value containing any call (including tagged templates and optional calls; there is no call
  allowlist), assignment, update, `delete`, `new`, `await`, `yield`, `import`, function, arrow
  or class expression, spread, method or accessor definition, comma operator, regular expression
  literal or coercion of an operand not proven primitive MUST keep the browser identity unresolved,
  because evaluating it could rewrite a devices descriptor. So MUST a non-literal identity value, a
  computed key (`[expr]`) of the `use` layer itself, any other spread, a computed device
  name and a device name outside the recognized table; each widens with
  `playwright:browser-identity-unresolved`.
- `TJAA-V0-019`: (proposed, pending owner acceptance; V1-1066; GitHub #709) The command
  `corvint [--root PATH] affected discovery --playwright-config PATH --playwright-list FILE` MUST
  convert the JSON report of a caller-run `playwright test --list --reporter=json` (at most 64 MiB,
  read as a regular file) into the canonical `playwright-discovery/0` receipt on stdout with one
  trailing LF, bound to the current HEAD, config path/SHA-256 and source digest. Playwright reports
  each spec `file` relative to `config.rootDir`; the producer MUST join it to `rootDir` and emit the
  repository-relative path the static profile uses, resolving symlinks, and MUST emit one unit per
  distinct (`projectName`, file) pair from every nested suite, deduplicated and sorted. It MUST refuse
  with `unsupported-playwright-discovery` and no stdout when the report is not JSON or lacks
  `config`/`suites`/`errors`, reports any error, records no `test` argv, records no `--list` after
  `test` (an execution report applies `test.only`, which `--list` disables) or any argument after
  `test` other than `--list`, `--reporter=json` (or `--reporter json`) and `--config`/`-c`, is sharded,
  names a config file other than PATH under the root, has a `rootDir` outside the root, names a test
  outside the root or with an unsupported extension, or has a test without `projectName`. Because it
  stamps the current bindings, it MUST independently enumerate the (project, file) pairs the config
  selects among the current sources through the static profile's `testDir`/`testMatch`/`testIgnore`
  subset (a config without `projects` is Playwright's one unnamed project), matching as Playwright's
  `createFileMatcher` does: a string glob without a leading `**/` gets one, string globs and the
  default `.spec.`/`.test.` markers match case-insensitively, `**` is a globstar only as a whole
  path component, and a regular expression keeps its own flags. A glob using minimatch syntax the
  profile does not model (classes, extglobs, escapes, single-item or range braces, `**` inside a
  component) is not static, and so is a string glob containing `//` (minimatch 3.1.5 splits the
  glob and the path on runs of `/`, so `e2e//*.spec.ts` and a leading `/`, prefixed to `**//`,
  match paths a literal reading would not) or a brace alternative containing `/` (brace
  expansion runs first, so `e2e/{/,x}*.spec.ts` can expand to `//`), and so is a regular expression with any flag other than `i`, `m` and
  `s` (Playwright tests from `lastIndex` 0, so a sticky `y` cannot be dropped). A regular
  expression body is static only when every construct reads identically in JavaScript without the
  `u` flag and in Go RE2: literal characters; `.`, `^`, `$` and `|`; `(...)` and `(?:...)` groups
  (any other `(?` form is not static); `*`, `+` and `?` after an atom, and `{n}`, `{n,}` and
  `{n,m}` with n <= m <= 1000, each bound `0` or ASCII digits without a leading zero, each
  optionally lazy; non-empty `[...]` and `[^...]` classes of
  literals, ranges and allowed escapes, without a nested `[`; and only the escapes `\d \D \w \W
  \b \B \t \n \r \f \v` (not `\b`/`\B` in a class) and a backslash before ASCII syntax
  punctuation. Every other construct (such as `\A`, a literal A in JavaScript and an anchor in Go;
  `\s`/`\S`, which include Unicode spaces only in JavaScript; backreferences, `\x`, `\u`, `\p`,
  POSIX classes, any other `{`, including `{01}`, which repeats in JavaScript and is literal text in
  Go, `{1, 2}` and `{,2}`) is not static. A regular-expression body or string glob containing any
  character above U+FFFF, literal, in a class or escaped, is not static, because JavaScript
  without `u` and minimatch read it as two UTF-16 code units and Go as one rune. Go's `.` and `(?m)` anchors treat only LF as a line
  terminator where JavaScript also treats CR, U+2028 and U+2029, and a JavaScript expression
  without `u` matches UTF-16 code units, so the producer MUST refuse when the repository root, any
  enumerated candidate path or any listed path contains CR, LF, U+2028, U+2029 or a character
  outside the Basic Multilingual Plane, and static selection over such a test path widens with
  `playwright:project-membership-unresolved`. It MUST find candidate files by path alone among
  every file with an extension Playwright's default `testMatch` accepts
  (`.js`, `.ts`, `.jsx`, `.tsx`, `.mjs`, `.cjs`, `.mts`, `.cts` and their `x` forms), whether or not
  the static profile can parse or read them. It MUST refuse with `unsupported-playwright-discovery`
  when any enumerated pair is missing from the listing (a stale or filtered listing), when the
  listing names a pair the config does not select (including a file absent from the current
  sources), when a selected file is one the static profile does not parse (such as `.mts` or
  `.cts`) or one it cannot read as bounded UTF-8 source, listed or not, when the config selects a
  file inside a directory the shared source walker skips (a hidden directory or one of `build`,
  `dist`, `vendor`, `target` and the other skipped names, at any depth of the file's path; Playwright
  still runs it, but it is outside the bound source digest), naming that directory, when a `testDir`
  is reached through a symbolic link, or when that membership is not statically resolved; only an
  unresolved browser identity is tolerated. `node_modules` below a `testDir` is not searched,
  because Playwright never descends it. It MUST read HEAD again after its last source observation and fail
  with `unsupported-affected-drift` on any HEAD or source change while it runs, even when the tree is
  unchanged. It is read-only and runs no Playwright or config. The listing remains caller-declared:
  the producer cannot prove the report was taken from the bytes it binds, nor that environment-driven
  config branches matched.
- `TJAA-V0-020`: (proposed, pending owner acceptance; V1-1067; GitHub #709) When `discovery.state`
  is `MALFORMED`, the summary MUST carry `reason`, one of `DECODE_FAILED` (over the 4 MiB bound,
  invalid JSON, or a duplicate, unknown or mistyped member), `NON_CANONICAL_BYTES` (with the first
  differing byte offset) or `INVALID_FIELD` (naming the first failing field, such as `profile`,
  `revision`, `config.sha256` or `units[i].test`), and `detail`, one whitespace-collapsed line of at
  most 240 bytes plus an ellipsis. A raw Playwright JSON report given as a receipt MUST say so and
  name `corvint affected discovery`. Both members MUST be absent in every other state. An oversize
  regular discovery file MUST report `MALFORMED`, not `MISSING`; an absent, unreadable or non-regular
  file stays `MISSING`.

## Opt-in Playwright project profile

`corvint affected --playwright-config PATH --playwright-discovery FILE` emits `playwright-affected/0`; it does not alter the
closed `affected-plan/0` receipt. The profile reuses the shared TypeScript import graph only for
physical path ownership and reachability, then expands reached Playwright files into project units.
This keeps project multiplicity out of the language-agnostic graph while still binding each runnable
unit to its config and project inputs.

The supported static subset follows Playwright's project contract: dependencies run setup projects
first, teardown projects run after their setup/dependent cohort, project grep is retained as runtime
case filtering, and project `testMatch`/`testIgnore` controls file membership. A form outside the
closed subset is not approximated. It widens the selection or, when projects cannot be identified,
abstains from runnable units.

The canonical `playwright-discovery/0` object has exactly `config` (`path`, `sha256`), `profile`,
`revision` (full lower-case Git object ID), `sourceDigest`, and `units` (objects with `project` and
`test`, sorted by project then path). It describes the complete unfiltered configured listing,
including dependencies and teardowns, not case execution counts. It is caller-declared discovery,
not authenticated execution attestation. Corvint never executes Playwright or config to produce it;
`corvint affected discovery` (`TJAA-V0-019`, proposed) only converts a caller-run JSON listing.
Each unit's `test` is repository-relative, not relative to Playwright's `rootDir`.
`sourceDigest` uses the public observer's `playwright-sources:sha256:` identity for current source
bytes. A receipt generated before dirty source changes is stale. Omitting the optional input safely
produces the complete-config fallback; default `affected-plan/0` compatibility remains unchanged.
Rollback removes the opt-in profile changes; no persistent source or external state is written.

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

`TestPlaywrightExampleAppQualification` extends that synthetic repository shape with global `use`, a
global-setup helper, and alias imports through specs, fixtures, page objects, workflows and scenario
builders. A cohort helper change selects exactly 41 of 353 units (13 files across three projects plus
setup/cleanup); a global-setup helper or config change selects all 353. Computed imports, undeclared
or missing aliases, unsupported config inheritance, ambiguous module candidates and dynamic config
inputs require full fallback without exclusions. Identical inputs reproduce canonical receipt bytes.
The exact example-app-e2e checkout/config was unavailable (`NOT_OBSERVED`); this is not consumer recall or
execution qualification. JSONC comments/trailing commas are supported; package-directory resolution,
custom loaders and `extends` remain conservative frontiers. Source digests include tsconfig inputs.

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
The issue 41 extension can instead be reverted independently: restore the opt-in profile's global
`use`/alias frontiers and remove global-hook edge binding and its qualification cases.
The GitHub #709 extension (`TJAA-V0-018..020`) reverts independently: restore the static-literal
check for every `use` value, remove `affected discovery` and its producer, and drop the MALFORMED
`reason`/`detail` members; no persisted format or store is written.

## Traceability

| Requirement | Implementation / evidence | Status |
|---|---|---|
| `TJAA-V0-001..004`, `TJAA-V0-006..008` | `internal/liveverify/affected/typescript/` focused tests | experimental |
| `TJAA-V0-005` | `TestTemplateSubstitutionRequireBuildsDependencyEdge`, `TestMultilineJSXQuoteAmbiguityRaisesFrontier`, `TestSameLineJSXApostropheAmbiguityRaisesFrontier`, `TestStandaloneJSXApostrophesRaiseFrontier`, `TestJSXTextCannotImitateALiteralOpeningContext`, `TestKeywordEndingJSXTextCannotHideRequireWithoutBraces`, `TestJSXAttributeAndExpressionStringsRemainParsed`, `TestOrdinaryTSXStringsAndJSXExpressionLiteralsRemainParsed`, `TestWorkspacePackageImportReachesTheImportingTest_V1_0283`, and import-resolution focused tests in `internal/liveverify/affected/typescript/` | experimental |
| `TJAA-V0-009` | `internal/liveverify/affected/conformance_test.go` TypeScript seam case | experimental |
| `TJAA-V0-010..017` | `internal/liveverify/affected/typescript/playwright.go`, `playwright_test.go`, and `cmd/corvint/affected_playwright_test.go` | experimental |
| `TJAA-V0-014..017` fixture qualification | `internal/liveverify/affected/typescript/playwright_qualification_test.go`, `testdata/playwright-qualification.tsv` | synthetic fixture evidence; runtime promotion excluded |
| `TJAA-V0-012..017` example-app shape | `TestPlaywrightExampleAppQualification`, `TestPlaywrightGlobalUseInheritance`, `TestPlaywrightAliasResolutionBoundaries` in `internal/liveverify/affected/typescript/playwright_example_app_test.go` | synthetic global-use, alias and hook closure; exact consumer `NOT_OBSERVED` |
| `TJAA-V0-018` (proposed) | `TestPlaywrightDeviceSpreadBesideRuntimeUseValues_V1_1065`, `TestPlaywrightUseValueSideEffects_V1_1065`, `TestPlaywrightUseValueUnicodeLineTerminator_V1_1065`, `TestPlaywrightStaticValueNestingIsBounded_V1_1065`, `TestPlaywrightUseValueNestedTemplate_V1_1065`, `TestPlaywrightMemberReadRoots_GH709Round8` in `playwright_example_app_test.go`; `TestUnicodeLineTerminatorsEndCommentsAndLines`, `TestNestedTemplateLiteralsKeepContent` in `typescript_test.go`; `TestPlaywrightScannersSkipNestedTemplates` in `playwright_test.go`; `TestPlaywrightPureExpression`, `TestPlaywrightPureExpressionScopedMemberReads` in `playwright_pure_test.go`; `TestQualifiedReporterIdentityKeysMatchStaticProfile` in `internal/jstestprovider/identity_keys_test.go` | experimental |
| `TJAA-V0-019` (proposed) | `TestPlaywrightDiscoveryFromListMultiProject_V1_1066`, `TestPlaywrightDiscoveryFromListRefusals_V1_1066`, `TestPlaywrightDiscoveryFromListMembership_V1_1066` in `playwright_discovery_list_test.go`; `TestPlaywrightStringGlobsArePrefixedAndCaseInsensitive`, `TestPlaywrightComputedStringsAndUnsupportedGlobsWiden`, `TestPlaywrightLineTerminatorPathWidensMembership`, `TestPlaywrightRegexBodyAllowlist`, `TestPlaywrightGlobAgreesWithBundledMinimatch` in `playwright_test.go` over a real Playwright 1.61.1 `--list --reporter=json` report (`testdata/playwright-list/multi-project.json`); `TestAffectedPlaywrightDiscoveryProducer_GH709` in `cmd/corvint/affected_playwright_test.go`; `TestAffectedPlaywrightDiscoveryStaleListing_GH709`, `TestAffectedPlaywrightDiscoveryHeadDriftAfterSources_GH709` in `cmd/corvint/affected_playwright_discovery_test.go` | experimental; one real listing shape |
| `TJAA-V0-020` (proposed) | `TestPlaywrightDiscoveryMalformedReason_V1_1067` in `playwright_discovery_test.go`; `TestAffectedPlaywrightDiscoveryProducer_GH709` | experimental |
| independent real-repository recall | 2026-08-29 build-log evidence | observed |
| runtime/framework/OS qualification | `LPCV-V0-043..046` promotion matrix | `NOT_RUN` |
