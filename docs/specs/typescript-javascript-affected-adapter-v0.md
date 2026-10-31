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
- Status: proposed/experimental; TJAA-V0-019 accepted (decision 0483) with its V1-1175 amendment and TJAA-V0-021 accepted (decision 0519); TJAA-V0-030..033 (V1-2042) and TJAA-V0-034..037 (GitHub #1575-#1578) and TJAA-V0-038..045 (GitHub #1593, #1594) and TJAA-V0-046..053 (GitHub #1604, #1605) and TJAA-V0-054..060 (GitHub #1604 re-test, #1625, PR #1628 review) and TJAA-V0-090/091 (GitHub #1701) and TJAA-V0-092/093 (GitHub #1717) and TJAA-V0-094/095 (GitHub #1737) and TJAA-V0-096..099 (GitHub #1737 item 2) and TJAA-V0-100 (GitHub #1750) and TJAA-V0-101 (GitHub #1751) and TJAA-V0-102 (GitHub #1751) and TJAA-V0-103/104 (GitHub #1737) and TJAA-V0-105 (GitHub #1737 residual) and TJAA-V0-106..115 (GitHub #1877-#1880) and TJAA-V0-116/117 (GitHub #1881) and TJAA-V0-118..120 (GitHub #1881 item 3) under the standing lane auto-accept; TJAA-V0-121..125 (GitHub #1883-#1887), TJAA-V0-126 (GitHub #1751, V1-2667), TJAA-V0-127 (V1-2584) and TJAA-V0-140..142 (V1-2812 per-test closures) experimental under the standing lane auto-accept
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
  whitespace or comments separate `import` from its parenthesis, except as `TJAA-V0-034` scopes
  type-position import types and literal text. When scanning for comments and
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
  except as `TJAA-V0-018` (accepted, decision 0483) admits for options outside the browser/device
  identity set.
  It MAY resolve nearest-ancestor `tsconfig.json` JSON/JSONC `baseUrl` and `paths` declarations:
  exact keys precede wildcard keys and longest wildcard prefixes precede shorter prefixes. Overlapping
  equal-prefix patterns or target lists with multiple existing candidates MUST widen rather than
  approximate runtime precedence. Device defaults MUST NOT override an explicit inherited browser.
  Explicit `use.defaultBrowserType` MUST widen; only recognized device descriptors supply defaults.
  Resolution MUST stay inside the observed repository, and MUST
  widen on missing declared targets, ambiguous source candidates, config inheritance, unsupported
  root-directory/module-suffix resolution, or explicit Playwright `tsconfig` overrides. The general
  `affected-plan/0` adapter MUST retain its existing unresolved-alias frontier, except as
  `TJAA-V0-037` admits when Playwright is the only runner identity it observes.
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
  Amendment (accepted, decision 0514): a changed helper that no Playwright
  spec reaches is an explicit bounded exclusion from this profile's selection, not a selection
  unknown. With matched discovery and otherwise resolved selection, the plan MUST retain
  `scope=BOUNDED`, `fallback=NONE`, and no selected unit for that helper; reached helpers still
  select their specs and project relations as above. The shared plan's `NO_SELECTABLE_TEST`
  (AFP-V0-020) MUST NOT by itself widen this profile: running the full E2E suite cannot exercise
  an unreached helper. This exclusion adds no helper row to `excluded` (which describes test
  project/file pairs), does not claim the helper is tested, and does not suppress other selection
  unknowns, execution unknowns, or TJAA-V0-015 discovery fallback.
  Superseded in part by `TJAA-V0-106` (GitHub #1877): an end-to-end run may load an unreached
  source through the application under test, so an unreached dirty path is now the selection
  unknown `playwright:out-of-suite-source` unless the repository declares it unreached and the
  Playwright closure provably leaves it out; only then is the bounded exclusion above kept.
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
- `TJAA-V0-018`: (accepted, decision 0483; V1-1065; GitHub #709) In a global or project
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
  admits neither root, whatever that token's operand. A `/` starts a regular expression after
  an operator, opening punctuation, `=>` or one of `return`, `typeof`, `instanceof`, `in`, `new`,
  `delete`, `void`, `throw`, `case`, `do`, `else` (not a property name such as `x.return`), and is
  division after any other operand; after `of`, `yield` or `await`, which may be identifiers, and
  directly after a non-ASCII character or an identifier escape (`\u0061`, `\u{61}`), it is
  ambiguous and the source is refused as unparsed. Every other
  member read (`this.x`, a member of an import from another module, a function result or a
  numeric literal such as `1..x`, and `process.env.NAME` outside `TJAA-V0-035`) MUST keep the
  identity unresolved. No other member read is proven primitive: an admitted one is accepted only
  as a whole value and under the non-coercing operators. Numeric literals MUST be lexed
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
- `TJAA-V0-019`: (accepted, decision 0483; V1-1066; GitHub #709; V1-1175 amendment accepted, decision 0519) The command
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
  when any enumerated pair is missing from the listing (a stale or filtered listing) and
  `TJAA-V0-021` does not account for the omission, when the
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
- `TJAA-V0-020`: (accepted, decision 0483; V1-1067; GitHub #709) When `discovery.state`
  is `MALFORMED`, the summary MUST carry `reason`, one of `DECODE_FAILED` (over the 4 MiB bound,
  invalid JSON, or a duplicate, unknown or mistyped member), `NON_CANONICAL_BYTES` (with the first
  differing byte offset) or `INVALID_FIELD` (naming the first failing field, such as `profile`,
  `revision`, `config.sha256` or `units[i].test`), and `detail`, one whitespace-collapsed line of at
  most 240 bytes plus an ellipsis. A raw Playwright JSON report given as a receipt MUST say so and
  name `corvint affected discovery`. Both members MUST be absent in every other state. An oversize
  regular discovery file MUST report `MALFORMED`, not `MISSING`; an absent, unreadable or non-regular
  file stays `MISSING`.
- `TJAA-V0-021`: (accepted, decision 0519 (V1-1175); GitHub #790) A project `grep` or
  `grepInvert` is per-case runtime filtering, not file membership, so a caller-run listing omits a
  selected (project, file) pair when that project's filter drops every case of the file. When a
  pair `TJAA-V0-019` enumerates is missing from the listing and its project declares `grep` or
  `grepInvert`, the producer MUST read each case of that file the listing names under any project.
  Playwright's grep title joins, with single spaces, the project name, the file suite title, the
  config tags, each describe title and its explicit tags, the test title and the test's explicit
  tags. The listing shows the titles exactly but reports tags only as the `@\S+` runs of the config
  tags, describe titles and tags and test title, followed by the test's own tags, so a config or
  describe tag containing whitespace is reported only in part and one that is `@` alone or has
  whitespace right after `@` not at all.
  (a) When each declared value is a static regular-expression literal, or an array of them, that is
  evaluable part by part (static under `TJAA-V0-019`, flags only `i`, `m` and `s`, an ASCII body
  under `i`, and no `.`, `^`, `$`, literal space, `\D`, `\W`, negated class, class containing a
  character at or below U+0020 or a range starting at `\t`, `\n`, `\r`, `\f` or `\v`), so it matches
  the joined title exactly when it matches one of its parts, a case is excluded for certain when
  `grepInvert` matches the project name or a title (file suite, describe or test), which no hidden
  tag text can undo. The producer MUST accept the omission and emit the pair as a unit when every
  case is excluded for certain.
  (b) Otherwise the reported tags, less the `@\S+` runs of the titles, stand in for the explicit
  tags. When a declared value is not evaluable as in (a) (dynamic, a non-literal, or a construct
  outside (a)), or when every case not excluded for certain is excluded by those parts (`grepInvert`
  matches one, or `grep`, any array element, matches none), or is outside the matcher model (a part
  outside the Basic Multilingual Plane or holding U+017F or U+212A under `i`, or reported tags
  lacking a title's `@` run), the producer MUST emit the pair as a unit and also list it in the
  optional sorted `unverifiedUnits` member of the receipt: hidden tag text could still match
  `grep` or fail to match `grepInvert`, so a negative `grep` decision is never verified.
  (c) Otherwise, including a file the listing names under no project, it MUST refuse as
  `TJAA-V0-019` does, the message stating that the project's grep/grepInvert does not exclude every
  listed case of that file. A project without `grep` or `grepInvert` keeps the `TJAA-V0-019`
  refusal unchanged. A dynamic `grep` or `grepInvert` no longer refuses the listing, because only
  case filtering depends on it; static selection still reports it as a config-syntax unknown, so the
  plan stays `STATIC_UNRESOLVED`. A decoded receipt whose `unverifiedUnits` is present but empty,
  unsorted, duplicated or not a subset of `units` is `MALFORMED` with `INVALID_FIELD`; when a
  `MATCHED` receipt carries `unverifiedUnits`, the plan MUST add the `EXECUTION` unknown
  `playwright:discovery-omission-unverified` naming the count and first unit. A case filtered out of
  every project is absent from the listing, so it cannot be checked; under the caller-declared
  listing it is excluded for that project too. A case the reported parts keep is refused under (c)
  even though hidden tag text could exclude it, because refusal is the conservative outcome. The per-test listing producer and a top-level `grep` are unchanged.
  Amended by `AFP-V0-409` to `AFP-V0-411` (GitHub #1647): when the config, test file and import
  closure hold only literal `@\S+` tags and the case's explicit tags are among them, a case (b)
  excludes is excluded for certain, and the unverified unknown's detail lists up to ten units.
- `TJAA-V0-030`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2042; GitHub #1294) When the `playwright-affected/0` plan's final fallback is
  `FULL_RELEVANT_SUITE`, the plan MUST carry a `narrowed` member. It is advisory evidence beside the
  fallback and has these members: `basis` `STATIC_IMPORT_GRAPH`, `state` `NARROWED` or
  `NOT_NARROWED`, `reason` and `detail` only when the state is `NOT_NARROWED`, and sorted
  `candidates`, `projects` and `symbols`. It MUST NOT change `scope`, `fallback`, `selected`,
  `excluded` or `unknown`, and it is never a reason to run less than the fallback. A plan whose
  fallback is not `FULL_RELEVANT_SUITE` MUST omit the member, so its bytes are unchanged.
  `candidates` are formed in three steps. Start from the Playwright units that the static base
  selection reaches from the dirty paths, plus any units added under `TJAA-V0-032`. Expand them over
  project dependency and teardown edges. Then remove the units of every `UNRESOLVED` project.
  `projects` lists each statically declared project with its `argv`, its `membership` and, when the
  membership is `UNRESOLVED`, a `detail`.
- `TJAA-V0-031`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2042) The state MUST be `NARROWED` only when every dirty path is a
  test-side module. A module is test-side when it is owned and when every root of its reverse static
  import closure, the module included, is a test unit of any runner. The closure is taken before
  Playwright membership binding and after global-hook binding, so the config, its global
  setup/teardown hooks and every module they import are not test-side. Otherwise the state MUST be
  `NOT_NARROWED`, with the first reason in this order:
  - `NO_DIRTY_PATH`: there are no dirty paths.
  - `CONFIG_UNRESOLVED`: any config unknown is not attributed to exactly one project under the
    per-project rule of `TJAA-V0-032`. Examples are global `testDir` or membership, the project
    set, a dependency cycle, hooks, and the matcher model.
  - `GRAPH_FRONTIER`: any observer frontier other than config, E2E runtime dependency or dynamic
    import, or any other language frontier.
  - `NOT_TEST_SIDE`: an unowned or unindexed dirty path, the dirty config, or a dirty module that a
    non-test root reaches.
  - `GRAPH_FRONTIER`: a dynamic import in a module that is not test-side, or a module that cannot be
    read or parsed for the dynamic-import check.
- `TJAA-V0-032`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2042) Narrowing MUST be kept per project.
  (a) An unknown raised by one project's own entry makes only that project `UNRESOLVED`. Such
  unknowns include its `testDir`, `testMatch`, `testIgnore`, `dependencies`, `teardown`, `grep`,
  `grepInvert`, `metadata` or `use` browser identity, and a reference to a project the config does
  not declare. The detail
  names those unknowns. The project contributes no candidates and keeps its `argv`, so it can still
  be run in full. Every statically resolved project keeps its candidates.
  (b) A test-side module that contains a dynamic import or a computed require is a runtime load
  frontier. Every Playwright unit that statically reaches such a module MUST be added as a candidate
  with witness kind `DYNAMIC_IMPORT_FRONTIER`.
  (c) A discovery receipt counts as bound when its state is `STATIC_UNRESOLVED`, `UNIVERSE_MISMATCH`
  or `MATCHED`. With a bound receipt, a listed project that the static config does not declare
  makes the narrowing `NOT_NARROWED` with reason `DISCOVERY_PROJECT_SET`. Otherwise each
  `STATIC_CONFIG` project becomes `DISCOVERY_MATCHED` when its sorted static test files equal the
  files listed for it, and `UNRESOLVED` (with candidates removed) when they differ. A `MISSING`,
  `MALFORMED` or `BINDING_MISMATCH` receipt leaves memberships `STATIC_CONFIG`.
- `TJAA-V0-033`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2042) `symbols` MUST hold one entry for each owned dirty path, for at most 64
  paths. Each entry has `path`, `state` (`RESOLVED` or `UNRESOLVED`), the sorted named exports
  `added`, `removed` and `modified`, `otherChanged` and `additiveOnly`.
  - How modules are compared: comments are stripped, and each module is split into top-level
    statements at line starts that begin with a declaration, import or export keyword. Code
    whitespace collapses; string and template content is kept exactly.
  - How exports are keyed: a named export statement is keyed by the names it binds (`default`, `*`,
    `* as ns`, export lists, and declarations). Any other statement, including an unmodelled export
    form, counts toward `otherChanged`.
  - `additiveOnly`: true only when the base statement sequence equals the current sequence after
    removing the statements whose names are all newly added.
  - When the state MUST be `UNRESOLVED`, with a `detail`: a JSX file, content over 1 MiB, content
    that does not tokenize, an unreadable base or current content, no comparison base, or a path
    past the bound.
  - Comparison base and reads: the CLI comparison base is the `--base` commit when one is given,
    otherwise `HEAD`. Base content is read in one bounded `git cat-file --batch`. A path absent at
    the base is a new file, and a non-blob or oversize object stays unread.
  - Limits: symbols are syntactic evidence only. They MUST NOT narrow, widen or remove a candidate,
    because an added export can still run code at load time.

- `TJAA-V0-034`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2455; GitHub
  #1575) The `import(` scan MUST skip string, template-literal and regular-expression text and the
  member name `x.import(`. In a `.ts`, `.tsx`, `.mts` or `.cts` source, `import('<literal>')` with
  exactly one quoted-literal argument is a type-position import type, not computed loading, when it
  directly follows `typeof`, or when it is followed by a `.Name` member chain that is not followed by
  `(`, a backtick, `!` or `?.`, contains no `then`, `catch` or `finally` member and does not follow
  `await`. Such an import type records the literal as a static reference. Every other `import(`
  call, including any call in a `.js`/`.jsx`/`.mjs`/`.cjs` source and any computed or template
  argument, MUST stay computed loading and raise `typescript:dynamic-import`. A `/` opens a
  regular expression after an expression-opening punctuator, `=>`, or one of the keywords `return`,
  `typeof`, `instanceof`, `in`, `new`, `delete`, `void`, `throw`, `case`, `do` or `else` that is not
  a member name; `TJAA-V0-038` settles `!`. After `)`, `}`, `+`, `-`, `<`, another `>`, `of`, `yield`
  or `await` the slash is ambiguous: when reading it as a same-line regular expression would change a literal,
  substitution or comment boundary and the source contains the word `import` or `require`, the file
  MUST keep a frontier rather than drop an edge (`typescript:scan-unresolved` under `TJAA-V0-039`). A comment opener, and in a JSX source the
  slash of a `</` or `/>` tag, is not ambiguous; an unparsed file keeps `typescript:unparsed-source`.
- `TJAA-V0-035`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2456; GitHub
  #1576) For `TJAA-V0-018`, `process.env.NAME` (a dotted name that is not an `Object.prototype`
  name and does not start with `__`) MUST be admitted and proven primitive, and `Number(...)`,
  `Number.parseInt(...)`, `Number.parseFloat(...)`, `parseInt(...)` and `parseFloat(...)` calls
  whose arguments are all proven primitive MUST be admitted with a primitive result that takes no
  further member, call or template, when: the file contains none of the tokens `globalThis`,
  `global`, `window` or `self`, nothing else admits a root under the existing `eval`, `delete`,
  `++`, `--` and escape guards, and every occurrence of `process` (respectively of each numeric
  callee), strings included, is that exact read (respectively call) in a read-only position. A
  shadowing declaration, write, alias or call argument of either root therefore refuses it. A
  trailing comma MUST be accepted in array literals as in object literals. A `launchOptions` value
  whose only computed property is a pure `executablePath` MUST NOT leave the browser identity
  unresolved; the project instead carries an execution-axis unknown
  `playwright:browser-executable-unresolved`. Every other identity key, including any other
  computed `launchOptions` property, MUST stay a static literal. Residual: an imported module or
  runtime preload that replaces `process.env` or `Number` is outside the static model, as one that
  mutates a `devices` descriptor already is.
- `TJAA-V0-036`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2457; GitHub
  #1577) A project `dependencies` value that is the identifier of a top-level `const` declared
  once, whose initializer is a literal string array, a parenthesized one, or a conditional
  `test ? A : B` whose branches are such values recursively, MUST resolve to the sorted union of
  the branch arrays for selection and for `--playwright-discovery-from-list`. Every other
  occurrence of the name MUST be the whole `dependencies: NAME` value of a project object literal
  in the `projects` array literal of the `export default` (or `export default defineConfig(...)`)
  config object; any other use (`let`, a write, `push`, an alias, a call argument, another object's
  property value, a second declaration) or a non-literal element MUST keep
  `playwright:project-dependency-unresolved`.
- `TJAA-V0-037`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2458; GitHub
  #1578) In the general `affected-plan/0` adapter, an owned file whose runner resolves to
  Playwright Test MUST be a source, not a test unit, unless its name matches Playwright's default
  `testMatch` (`.spec.` or `.test.` with a loadable extension) or a recognized Playwright config
  governing it names membership markers the adapter does not model (which already raises
  `typescript:executable-config-unresolved`) and the path is a conventional test candidate. When a
  `tsconfig.json` declares `baseUrl` or `paths` and every runner identity observed (each recognized
  runner config, each manifest-declared or configured runner, each test observation) is Playwright
  Test, with at least one present and no Playwright config containing a `tsconfig` token, the
  adapter MUST resolve aliases exactly as `TJAA-V0-012` does for the opt-in profile, widening on the
  same failures; otherwise it MUST keep `typescript:path-alias-unresolved`.

- `TJAA-V0-038`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2467; GitHub
  #1593) The ambiguous-slash check of `TJAA-V0-034` MUST read tokens with the same lexer as the
  import scan over the comment-stripped text the scanners read, so a `/` inside a string, template
  text, a comment or a regular-expression literal that the lexer admits (its closing slash
  included) is never a candidate, and a comment before a slash is whitespace to its verdict; the
  regular-expression reading of a candidate is still taken over the raw text. A `/` after `(`, `,`,
  `=`, `:`, `[`, `&`, `|`, `?`, `{`, `;`, `=>`, a listed keyword or at the start of the text opens a
  regular expression. A `!` takes the verdict of the position it stands in: where an operand may
  begin, or when a line terminator precedes it, it is the prefix negation and the slash after it
  opens a regular expression; after an operand on the same line it is a non-null assertion and
  the slash is division; `TJAA-V0-045` keeps every other unsupported predecessor ambiguous. The
  GitHub #1593 rows
  (`s.replace(/[^/]+/, '').replace('/x', '')` and its `[^x]` and `[^\/]` variants, and
  `url.replace(/^https?:\/\/[^/]+/, '')`) MUST raise no frontier.
- `TJAA-V0-039`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2467; GitHub
  #1593) A module whose import scan finds no computed loading but an ambiguous slash that
  `TJAA-V0-034` keeps MUST raise `typescript:scan-unresolved`, not `typescript:dynamic-import`.
  `typescript:dynamic-import` MUST mean that a `require` or `import(` call with a computed argument
  was read in code. In the Playwright profile `typescript:scan-unresolved` is a selection unknown
  and, like any frontier other than a dynamic import, keeps advisory narrowing `NOT_NARROWED`.
- `TJAA-V0-040`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2467; GitHub
  #1593) Every `typescript:*` frontier reason MUST record the sites that raised it:
  `path:line` (1-based; LF, CR, CR LF, U+2028 and U+2029 end a line) when one line holds the cause
  (a computed loading call, an ambiguous slash, the opening token of an unterminated literal or
  comment, the specifier of an unresolved or undeclared import at the offset the import scan
  parsed it (never a text search that could match a comment or string), the first unmodelled config
  marker, the first tsconfig `"paths"` or `"baseUrl"` key), and the repository-relative `path`
  when only the file is known (an unreadable or unread module, a runner or E2E classification of
  the whole file, an invalid tsconfig or `package.json`, a `package.json` `mocha` member, or a
  specifier the scan did not parse, such as a path an alias rewrote). Naming sites MUST cost one
  line index per file plus a binary search per site, so the work stays linear in the observed
  source. In the `playwright-affected/0` plan the `detail` of each unknown raised from the shared
  TypeScript graph MUST be `shared TypeScript graph frontier at ` followed by at most 8
  sorted sites joined by `, `, plus ` and N more` when more exist.
- `TJAA-V0-041`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2468; GitHub
  #1594) Amending `TJAA-V0-035`, a `globalThis`, `global`, `window` or `self` token MUST refuse
  the `process` root and the numeric conversions only when it occurs in code (a template
  substitution included) or as the whole content of a string literal that directly follows a code
  `[` (a computed member key). Its occurrence inside string, template or regular-expression text,
  or in a comment, MUST NOT refuse them (`'global-setup'`, `testMatch: 'setup/global-setup.ts'`).
  That exemption holds only while no text can become code or reach a property the scan cannot
  see, so the config MUST refuse both roots when it has any of: a `Function`, `constructor` or `vm`
  token anywhere; a `\` escape in string or template text; a dynamic `import(`; a `require(` whose
  argument is not one escape-free string literal; or a computed member key (a `[` after an
  identifier that is not a keyword, `)`, `]`, `.` or a literal) other than empty (a type such as
  `string[]`), a decimal integer or one escape-free string literal. `eval` already refuses every
  root.
- `TJAA-V0-042`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2468; GitHub
  #1594) Amending `TJAA-V0-035`, an occurrence of `process` that is the whole target of an `=`,
  `??=`, `||=` or `&&=` expression statement at the top level of the config,
  `process.env.NAME` or `process.env['NAME']` with a name `TJAA-V0-035` admits, MUST NOT refuse
  the `process` root. The statement MUST start the text, follow `;` or `}`, or follow a line break
  after an operand. Node stores the value as a string, so a later read of NAME stays proven
  primitive in a non-identity key. A computed `launchOptions.executablePath` that reads a written
  NAME MUST leave the browser identity unresolved, because `launchOptions` is an identity key. A
  write in a nested block or function, as part of another expression, with any other operator, a
  replacement of `process.env`, or `process.env` passed to a call MUST still refuse the root.
- `TJAA-V0-043`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2468; GitHub
  #1594) In a non-identity `use` key, `process.env.X || 'literal'`, `process.env.X ?? 'literal'`,
  `process.env['X']`, `process.env.X === 'lit' ? 'a' : 'b'` and
  `Number.parseInt(process.env.X || 'lit')` MUST keep the browser identity resolved under
  `TJAA-V0-035`, including in the GitHub #1594 config shape with a top-level
  `require('dotenv').config()` call.
- `TJAA-V0-044`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2468; GitHub
  #1594) The `playwright:browser-identity-unresolved` detail MUST keep its
  `<project> use.browserName/device is dynamic or unsupported` prefix and append, for the first `use`
  property that blocked it, `: `, the reason, ` at `, the blocking token in backticks and
  `(<config path>:<line>)`. The token is the identifier at the refusing position, or its first
  non-space run of at most 24 bytes, and the line is where that text occurs in the config. When the same `use` text occurs more than
  once, every occurrence is named, at most 4. When the value needs the `process` root, a numeric
  conversion or `devices` that the config refused, the detail MUST append `; <root> is not
  admitted: `, the reason, and the same token-and-line form for the first token that refused it (a global object name, a non-read `process`, `eval`, a `delete`/`++`/`--`/escape
  token, or a non-call conversion).
- `TJAA-V0-045`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2467; PR #1596
  review) Where regexStart cannot classify a `/` lexically it MUST keep the slash ambiguous, so
  `TJAA-V0-034` and `TJAA-V0-039` widen instead of a misread literal hiding an import. That covers
  a slash after a reserved or contextual keyword that is not an operand (`debugger`, `break`,
  `continue`, `yield`, `await`, `of`, `let` and the rest; `this`, `super`, `null`, `true` and
  `false` stay division), after a non-ASCII byte, after `\`, and after a word that an identifier
  escape or a non-ASCII identifier part precedes. A regular-expression reading also moves a
  boundary when the division reading would form `//` or `/*` from its inner or closing slash
  (`/a\//`).

Intent and boundary (GitHub #1593, #1594): the reporter could find neither the four files that
raised a false `typescript:dynamic-import` nor the config tokens that kept browser identity
unresolved without bisecting files, and every Playwright selection widened to the full suite.

- Non-goals: the general `affected-plan/0` frontier and its `LANGUAGE_FRONTIER` details stay the
  bare reason codes; only the Playwright plan names sites. `}` and `)` stay ambiguous predecessors,
  because a block and an object literal (or a condition and a call) need grammar state; their
  ambiguity is now named `typescript:scan-unresolved` at its line. No runtime evaluation of the
  config, and no model of what `dotenv` or another imported module writes.
- Failure modes, each fail-closed: a slash the lexer cannot classify keeps
  `typescript:scan-unresolved`; an unterminated literal keeps `typescript:unparsed-source`; a
  global object name in code, a computed `['globalThis']` key, a nested or compound
  `process.env` write, and a written key read by `executablePath` keep
  `playwright:browser-identity-unresolved`; a `use` text the config does not contain verbatim says
  `line not observed` rather than guessing a line.
- Residual: a module the config imports (such as `dotenv`) or a runtime preload that replaces
  `process.env` or a global stays outside the static model, as under `TJAA-V0-035`. A top-level
  write executes arbitrary right-hand-side code, as any top-level statement already could.

- `TJAA-V0-046`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1605)
  Amending `TJAA-V0-034` and `TJAA-V0-045`, a `/` after a `)` that closes the argument list of a
  call, on the same line, MUST read as division, so a `//` after it opens a line comment
  (`const now = Date.now() / 1000; // Convert to seconds for comparison` raises no frontier). A
  line break between the `)` and the `/` keeps it ambiguous, because it may end a bodiless
  signature (`declare function f(a)`). The `)` closes a call when its matching `(` lies at most 256 bytes before it, no quote, backtick,
  slash, brace or non-ASCII byte lies between them, and the `(` directly follows a member name
  (`x.now(`), an identifier that is not a reserved, contextual or operand keyword (`now(`), `)` or
  `]`. Any other `)`, including one that closes an `if`, `while`, `for` or `with` head or follows a
  keyword such as `typeof`, stays ambiguous under `TJAA-V0-034`.
- `TJAA-V0-047`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1605) In a
  Playwright config, `process.env['NAME']` and `process.env["NAME"]` with a plain string literal
  MUST be the same read, and the same top-level write target, as `process.env.NAME`: they keep the
  browser identity resolved exactly when the dotted form does, including in the GitHub #1605 config
  (`baseURL: process.env['HOSTNAME'] || 'http://127.0.0.1:3000'` beside a top-level
  `process.env.E2E_RUN_ID ??= ...` write).
- `TJAA-V0-048`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1605) A
  project's browser identity MUST depend only on `browserName`, `channel`, a `...devices[...]`
  spread, `launchOptions` (whose computed `executablePath` stays the execution-axis unknown of
  `TJAA-V0-035`), `defaultBrowserType` and the other `TJAA-V0-018` identity keys, and on the
  `process.env` names they read. A non-identity `use` field (`baseURL`, `trace`, ...) whose value is
  side-effect free under `TJAA-V0-018`, and a top-level `process.env.NAME` write under
  `TJAA-V0-042` to a name no identity field reads, MUST NOT leave it unresolved when every
  `process` occurrence on the write's right side is admitted (`TJAA-V0-049`); that includes the
  GitHub #1605 `??=` write whose right side reads `process.pid`.
- `TJAA-V0-049`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1605)
  Amending `TJAA-V0-035` and `TJAA-V0-042`, an occurrence of `process` that is exactly
  `process.pid`, `process.ppid`, `process.platform` or `process.arch` in a read-only position MUST
  NOT refuse the `process` root, and such a read MUST be admitted as a value proven primitive. A
  write, update, call or alias of `process` or of those members (`process.pid = 1`,
  `process.cwd()`, `process.exit(...)`, `const p = process`, `const env = process.env`) MUST still
  refuse it, including through enclosing bracket groups: a group that is called (`(process.pid)()`),
  optionally called (`?.(`), tagged, constructed (`new (...)`), deleted or updated
  (`(process.pid)++`, `++(...)`), or followed by a member chain that is an assignment or update
  target (`(process).x = 1`, `(process.pid).x++`), or by a member chain that does not parse, MUST
  refuse the root.
- `TJAA-V0-050`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1604)
  Amending `TJAA-V0-037`, a runner that only a package dependency names MUST NOT block tsconfig alias
  resolution in the general adapter: a dependency alone assigns no runner (`TJAA-V0-003`) and loads
  no module. A recognized runner config, a configured runner (`package.json` `mocha`, `jest` or
  `detox` member, a Deno or Bun manifest) and a test observation of another runner still block. A
  Playwright config blocks only when `tsconfig` occurs outside its comments, or anywhere when the
  config does not tokenize or has an ambiguous slash that could move a comment boundary. When resolution is
  blocked, the first blocking identity in config, manifest and test-path order MUST be a
  `typescript:path-alias-unresolved` site: the config or manifest path, or the test path, with the
  runner as its token, or the config line of the `tsconfig` token.
- `TJAA-V0-051`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1604) When
  aliases resolve, a `paths` target MUST be read relative to `baseUrl` when declared and otherwise
  relative to the directory of the tsconfig that declares it, and a spec that imports a changed
  module through such an alias, directly or through a relative import from another module, MUST be
  selected in `--path` mode (the GitHub #1604 rows `lib/fixtures/control.ts`,
  `lib/helpers/TestTimingLogger.ts` and `lib/api/client/ApiLogger.ts`). An alias whose target is not
  an observed file at the observed revision MUST add no edge and MUST raise
  `typescript:path-alias-unresolved` at the importing specifier; no target is guessed.
- `TJAA-V0-052`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1604)
  Amending `TJAA-V0-040` and its non-goal, the general `affected-plan/0` adapter MUST name each
  `typescript:*` frontier site as one frontier entry `reason:site`, where site is the
  `TJAA-V0-040` `path:line` or `path`, followed by a space and the backquoted token when one is
  known: the specifier the module spelled (the original of an alias rewrite) for an import site;
  the identifier at the offset, or its first run of at most 24 bytes without space, for an offset
  site (a computed loading call, an ambiguous slash, an unterminated literal, a config marker, a
  tsconfig key); for a whole-file classification (`e2e-runtime-dependency`, `runtime-flags`,
  `puppeteer-host`, `detox-configuration`) the line and specifier of the runner package import when
  the module imports it, otherwise the runner; the competing runners or configs for an ambiguous
  runner. Each reason names at most 16 sorted sites followed by one `reason:+N` entry, so AFP-V0-381
  `frontierSites` still counts every site. The `playwright-affected/0` shared-graph detail names the
  same token after each site. A reason without a site stays the bare reason.
- `TJAA-V0-053`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1604) A
  `tsconfig.json` the alias reader refuses MUST name the refusing key as its site token, at the line
  of its first quoted occurrence followed by `:` (the file alone when there is none): `extends`, `moduleSuffixes` or `rootDirs` (the first in that order),
  `baseUrl` outside the repository, or the first `paths` pattern in sorted order that is malformed;
  a config that does not parse names only the file.

Intent and boundary (GitHub #1604, #1605): the reporter's `--path` plans selected 0 of the specs
that import a helper through a tsconfig alias, and only reason counts said why; three Playwright
config and lexer residuals kept every selection at the full suite.

- Non-goals: no support for tsconfig `extends`, `rootDirs`, `moduleSuffixes` or a Playwright
  `tsconfig` option (each stays a named frontier, as `TJAA-V0-012` requires; `TJAA-V0-114` later
  follows a relative `extends` chain inside the root); no change to the
  `TJAA-V0-018` identity key set or to `defaultBrowserType` widening; a non-identity `use` value
  that can call user code still blocks, because it could rewrite a `devices` descriptor
  (superseded by `TJAA-V0-054`: it is now a selection-axis `playwright:use-value-unverified` unknown that keeps the identity); no Node
  built-in module model for bare imports; no grammar-state lexer, so a `)` that is not a call stays
  ambiguous.
- Failure modes, each fail-closed: a missing alias target, an alias claimed by two patterns or
  targets, and a blocked alias gate keep `typescript:path-alias-unresolved` at named sites; a
  `)` whose opener is out of the window or behind a literal keeps the ambiguous slash; a write or
  call on `process` keeps the identity unresolved.
- Residual: the reporter's repository was not available (`NOT_OBSERVED`); its blocking identity is
  now named by the frontier site rather than inferred. A comment-only `tsconfig` mention no longer
  blocks, and a dependency-only runner no longer blocks, which were the two plausible causes
  reproduced on synthetic fixtures.

- `TJAA-V0-054`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1625)
  Amending `TJAA-V0-018` and `TJAA-V0-048`, a project's browser identity MUST be derived only from
  the `TJAA-V0-018` identity keys (`browserName`, `channel`, `defaultBrowserType`, `launchOptions`
  with its `executablePath`, the other listed keys) and a literal `...devices[...]` spread, and from
  the `process.env` names they read. A `use` key outside that set (`navigationTimeout`,
  `actionTimeout`, `baseURL`, `trace`, `storageState`, ...) MUST NOT leave the identity
  unresolved, whatever its value form. When such a value is not side-effect free under
  `TJAA-V0-018` (a call, assignment, update, `require` or `await` may install a module-resolution
  hook such as `tsconfig-paths` `register`, or rewrite a `devices` descriptor, outside the config's
  view), the project keeps its recovered identity but MUST carry one selection-axis unknown
  `playwright:use-value-unverified` per value, naming the key, the first refused token and its
  config line, and the refused scope root as `TJAA-V0-044` does, so the plan scope is `UNKNOWN` and
  falls back to the full suite and a discovery receipt cannot bound it. Only a provably
  side-effect-free non-identity value is ignored entirely. A discovery listing check
  (`TJAA-V0-019`) tolerates the unknown as it tolerates an unresolved identity, because the value
  does not decide file membership. A computed identity key, a refused device spread, and a config
  whose own text writes, deletes or aliases `devices` (which refuses the devices root) MUST still
  raise `playwright:browser-identity-unresolved`.
- `TJAA-V0-055`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1625)
  Amending `TJAA-V0-035`, `Number.isInteger(...)` MUST be admitted beside `Number.parseInt(...)`
  and `Number.parseFloat(...)`, with a primitive result, under the same operand and occurrence
  rules. The member name `parseInt` or `parseFloat` of a code `Number.` read MUST be judged as an
  occurrence of `Number` only, so it does not refuse the bare `parseInt` or `parseFloat` root. A
  write, alias, optional call or shadowing of `Number` or of a member still refuses the root.
- `TJAA-V0-056`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1604
  re-test) The alias reader MUST resolve the re-test comment's tree as `tsc` does: a
  `compilerOptions` key order with `baseUrl` last after `paths`, an `include` list
  (`**/*.ts`, `**/*.js`) that does not change module resolution, a sibling
  `tsconfig.typecheck.json` that is not a nearest `tsconfig.json` and is not read, and the
  `tsc --init` comment layout; `lib/configs` resolves to `lib/configs/index.ts`,
  `lib/api/endpoints/TeetimesApiClient` to that `.ts` file and the dotted basename
  `lib/helpers/common.helper` to `lib/helpers/common.helper.ts`, so `--path lib/fixtures/control.ts`
  selects the specs importing it through the alias.
- `TJAA-V0-057`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1604
  re-test) Amending `TJAA-V0-037` and `TJAA-V0-050`, when the alias gate is blocked the general
  adapter MUST still resolve imports through the nearest `tsconfig.json` under the same rules and
  failures as an open gate, adding each resolved edge beside the edges the gate already had
  (`TJAA-V0-059`), and MUST keep `typescript:path-alias-unresolved`
  at each alias-declaring tsconfig's `"paths"` or `"baseUrl"` line and at the blocking identity, so
  the plan stays not bounded. An import tsconfig resolves MUST NOT be a site of its own; one it
  cannot resolve stays a site under `TJAA-V0-051`.
- `TJAA-V0-058`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1604
  re-test) Amending `TJAA-V0-052`, the sites that explain a whole reason (the alias gate's tsconfig
  and blocker sites, and a tsconfig the reader refuses) MUST all be named, first and in sorted
  order, in the general adapter's frontier entries (grouped by reason in sorted reason order and
  not re-sorted afterwards) and in the `playwright-affected/0` shared-graph detail; the 16-site
  (respectively 8-site) bound applies only to the other sites, in sorted order, and `reason:+N`
  (respectively `and N more`) counts the rest. The language-neutral `affected-plan/0` unknown list
  stays its canonical sorted set of those entries: every pinned site is present there, but its
  position is not re-ordered. Repository-owned text that is a site token MUST NOT begin with a
  NUL byte for this marking to stay exact (an import specifier with a literal NUL byte is a
  residual).
- `TJAA-V0-059`: (accepted under the owner's standing auto-accept (2026-10-11); PR #1628 review)
  Under a blocked alias gate the tsconfig edges are speculative and MUST be additive: a bare
  reference that names a workspace package (`TJAA-V0-005`) MUST keep its workspace-link edge
  beside the tsconfig-resolved edge, so the selection under the blocked gate is a superset of the
  selection without tsconfig resolution. An open gate (`TJAA-V0-037`) still rewrites the
  reference, as Playwright's loader does.
- `TJAA-V0-060`: (accepted under the owner's standing auto-accept (2026-10-11); PR #1628 review)
  Amending `TJAA-V0-018`, a `delete`, `++` or `--` token that lies inside the value of a
  non-identity key of a global or project `use` layer, whose text names no `devices` and has no
  `\`, MUST NOT refuse the devices root, so a literal device spread keeps its identity; the value
  holding the token is the `TJAA-V0-054` selection unknown instead. The token still refuses the
  process, numeric-conversion and plain-literal roots. A layer is located only when its text occurs
  once in the config. A token anywhere else (a statement, an identity value, a computed key, a
  value beside a `devices` occurrence) and any `\` escape in code still refuse every root.

Intent and boundary (GitHub #1604 re-test, #1625): the re-test reported 368 alias sites at the
importing modules, resolved at none, and `affected --path lib/fixtures/control.ts` still selected
0 of 93 specs; a Playwright config whose `use` held `Number.parseInt(...)` timeouts kept every
project's identity unresolved.

- Non-goals: no tsconfig `extends`, `rootDirs`, `moduleSuffixes`, project references or duplicate
  key support (a duplicated key still fails the strict parse and names the file); no model of
  another runner's loader (a Jest `moduleNameMapper`, a Vite alias), whose divergence stays the
  gate frontier; no change to the `TJAA-V0-018` identity key set or the reporter's identity keys.
- Failure modes, each fail-closed: a blocked gate keeps the plan not bounded while adding the
  tsconfig edges beside the workspace links; an alias target that does not exist stays a site at
  the importing specifier; a non-identity value that can run code is a selection-axis unknown, so
  the plan falls back; a mutation token outside a non-identity value, a computed identity key or a
  devices rewrite in the config keeps `playwright:browser-identity-unresolved`.
- Root cause and residual: the reporter's repository was not available (`NOT_OBSERVED`). The
  comment's tsconfig and imports already resolve on the base revision (`TJAA-V0-056` is a
  regression guard, not a fix). 368 sites with none resolved is the shape of a blocked gate, where
  each module reported its first alias import, and the bound sorted those `lib/...` sites first and
  hid the blocker and tsconfig sites in `+352`. Which identity blocked the gate in that repository
  is still not observed; after this change it is named, and the specs are selected regardless.

- `TJAA-V0-090`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1701)
  Amending `AFP-V0-409` and `AFP-V0-425`, a `JSON` token in the tag closure MUST NOT make its file
  inexact when it begins a direct call `JSON.stringify(value)`, `JSON.stringify(value, null)` or
  `JSON.stringify(value, null, space)`: the token is not a member (`x.JSON`, `JSON?.`) and no
  argument is empty or a spread. Its value is a string, which reaches a test only as a title (the
  listing shows titles exactly) or as a `tag` value (the tag rule already refuses a non-literal
  one), and without a replacer function the call runs only its argument's own `toJSON` methods
  and getters, user code under the same rules. Every other `JSON` use stays inexact and names its
  line: a replacer that is not `null`, `JSON.stringify` read without a direct call, `JSON.parse`,
  whose value can become a details object or a `tag` key, and any declaration or import of `JSON`
  (which is itself a `JSON` token outside such a call). The other reflective names are unchanged.
- `TJAA-V0-091`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1701)
  Amending `AFP-V0-404`, a project `testMatch` MUST NOT raise
  `typescript:executable-config-unresolved` in the general adapter when every `testMatch` in the
  config text is the `testMatch` of one parsed project and each of its items is an escape-free
  quoted string glob with no globstar beyond Playwright's own leading `**/`, no brace, class,
  extglob or negation, and no empty, `.` or `..` segment; no observed path inside that project's
  test directory is shorter than such a glob while its own segments match the glob's tail, so the
  match Playwright applies to the absolute path never depends on directories above the
  repository; and, as before, every observed file the profile assigns to a project is a
  Playwright test of that config. A computed, template, regular-expression, global, nested,
  spread or commented `testMatch`, or a glob whose tail a shorter test path matches, keeps the
  frontier at the first `testMatch` line (`AFP-V0-423`); any `testIgnore` still keeps it; and a
  profile-owned file the adapter classifies otherwise is still named as `owns <file>`.

Intent and boundary (GitHub #1701): on `a681846` a reporter's Playwright mode was bounded, but 33
specs whose module-level helpers built a request body with
`Buffer.from(JSON.stringify(payload)).toString('base64')` kept 303 units "omission unverified",
and `affected --path` stayed unknown for every change because a setup project's
`testMatch: 'setup/global-setup.ts'` raised the executable-config frontier.

- Non-goals: no general value-flow analysis of reflective calls; no deferral of test or hook
  bodies (Playwright's listing never runs them, but recognizing a callback as a Playwright body
  needs the test binding's provenance and a guard against a rewritten `test` member across the
  closure); no regular-expression `testMatch` in path mode; no `testIgnore`.
- Failure modes, each fail-closed: a `JSON` use that is not a plain stringify call still stops
  the tag closure and names its line, so the omission stays unverified; a `testMatch` the rule
  does not accept, or whose glob a test path is too short to decide, names the first `testMatch`
  line; a test path outside the matcher model (a non-BMP character or a line break) inside a
  literal-testMatch project's testDir also names that line; a testMatch reached only through an
  escaped key names the config as `testMatch`; a literal glob that assigns a file the adapter
  classifies as another runner's test or as support code names `owns <file>`.
- Residual: a patch of the global `JSON.stringify` from a file outside the closure, as for every
  other global, is outside the static model (`AFP-V0-409`). Membership markers other than
  `testMatch` that the reporter's config may hold (such as `globalSetup`) still raise the frontier
  at their own line; the reporter's config was not available (`NOT_OBSERVED`).

- `TJAA-V0-092`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1717)
  Amending `TJAA-V0-091` and `TJAA-V0-037`, a literal project `testMatch` glob MAY hold whole
  `**` segments after its first segment, provided its last segment is not `**` and no segment
  holds `**` beside other characters. Such a glob still raises no executable-config frontier only
  when no observed path inside the project's test directory has a match that depends on
  directories above the repository. Playwright matches `**/` plus the glob against the absolute
  path. A path whose own segments match `**/` plus the glob is decided. A path that matches only
  after one or more of the glob's leading segments are dropped depends on the checkout's location,
  and it names the frontier. The frontier MUST name the line of the first `testMatch` whose value
  the rule refuses (a computed or regular-expression value, a glob outside the rule, or the glob
  that path reaches), not the first `testMatch` line of the config. A file that imports
  `@playwright/test` and that a project with such a literal `testMatch` owns MUST be classified
  as a Playwright test of its config wherever it lives. Before, only a conventional test path
  kept that classification, so `setup/global-setup.ts` under `testDir: './'` was support code
  and raised `owns <file>`. A project's `dependencies`, computed or not, never raise the
  executable-config frontier in path mode, because they order projects and do not decide
  membership (`playwright:project-dependency` stays a membership-neutral unknown).
- `TJAA-V0-093`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1717)
  Amending `AFP-V0-409` and `AFP-V0-425`, a computed member read MUST NOT make its tag-closure file
  inexact when its receiver is a fresh array. A fresh array is an array literal (`['a', 'b'][index]`)
  whose `[` follows the start of the text, an operator, an opening punctuator or an expression
  keyword that is not a property name (so `box.default[0][index]` is not one), or the result of a direct `.split(...)` or `.map(...)` method
  call (`each.split(';')[index]`, `each?.split(';')[index]`, `all.map(f)[index]`). This holds only
  while no file in the closure defines or reads `split` or `map` other than as a direct method call
  `.split(` or `.map(`, and holds no `'split'` or `'map'` string. A user-defined `split` or `map`
  method could return a details object, which the computed read could then reach. Every other
  computed read still stops and names its line: a name, a parenthesized receiver, an indexed
  literal, a bracket after a property name, or another call such as `.slice(...)`. A computed write on a fresh array also still
  stops (`a computed member write can set a tag`). When a fresh-array read is the only thing that
  would make a closure exact, and a `split` or `map` definition sits in a closure file, the stop
  names that definition's file and line.

Intent and boundary (GitHub #1717): on `ec41e4e` the reporter's `affected --path
lib/fixtures/control.ts` still reported `playwright.config.ts:120` `testMatch`. Line 120 was the
literal `setup/global-setup.ts`, but the refused value was the next project's
`setup/**/*.setup.ts` globstar. The tag closure also stopped at
`splitIds(each.split(';')[index] ?? '')`, which kept 303 units "omission unverified".

- Non-goals: no other array-producing methods (`filter`, `slice`, `concat`, `Array.from`); no
  value-flow tracking of a fresh array through a name; no regular-expression `testMatch`; no
  `testIgnore`; no change to how project `dependencies` are parsed.
- Failure modes, each fail-closed: a globstar glob whose leading-segment-dropped form a test path
  matches, a trailing or in-segment globstar, or a computed or regular-expression `testMatch`
  names that `testMatch`'s own line. A `split` or `map` that the closure defines, reads or names
  as a string makes every fresh-method index stop at the definition. A computed read on any other
  receiver stops at its line as before. A Playwright file a literal `testMatch` owns but whose
  config is not literal is still classified only by `TJAA-V0-037`.
- Residual: a patch of `String.prototype.split` or `Array.prototype.map` from a file outside the
  closure, as for every other global, is outside the static model (`AFP-V0-409`). The TypeScript
  non-null form `value![key]` is not yet treated as a member access by the closure's receiver
  test. The reporter's repository was not available (`NOT_OBSERVED`).

- `TJAA-V0-094`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1737)
  Amending `TJAA-V0-091` and `TJAA-V0-092`, a project `testIgnore` MUST NOT by itself make path
  mode raise the executable-config frontier, nor name it as `testMatch`. When every project
  `testIgnore` is a string literal or an array of string literals, and each is a root-free glob
  under the `TJAA-V0-092` glob rule, the ignore verdict is decided statically. That holds only
  while no observed path that the project's `testMatch` holds sits outside the matcher model or
  depends on directories above the repository for any `testIgnore` glob of that project.
  Otherwise, and for a computed or regular-expression value, a glob outside the rule, or a global
  (top-level) `testIgnore`, the frontier MUST name that `testIgnore`'s own line as
  `playwright.config.ts:N` `testIgnore`. A `testMatch` that the rule refuses MUST still name its
  own line, so a config whose every `testMatch` is a literal never names `testMatch`.
- `TJAA-V0-095`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1737)
  Amending `AFP-V0-409`, `AFP-V0-425` and `TJAA-V0-093`, a plain computed member write
  `NAME[key] = value` MUST NOT make its tag-closure file inexact when NAME is a fresh object the
  file provably keeps from every test and describe title, tag and details argument. Three
  conditions define such an object.
  - Its module declares NAME exactly once, as `const NAME = { ... }`. The declaration is not
    exported, carries at most a simple type annotation, and its object literal (including a
    spread such as `{ ...body.settings }`) ends the expression, so the value is a new object that
    no other binding holds.
  - Every other code occurrence of NAME is one of these:
    - a property name after `.`;
    - an object-literal key;
    - a plain write target `NAME[...] =`;
    - the right operand of `in` (a key test or a for-in subject, whose value is a key or a
      boolean);
    - a shorthand property, property value or spread of object literals that nest only through
      property values inside the first argument of a direct `.fulfill(...)` call.
  - No closure file holds `this` or `super`. A setter, a `toJSON` method or another method that
    the write or a serializer invokes would bind the object there.

  While such an object reaches a `.fulfill(...)` argument, no closure file may define, read or
  name `fulfill` other than as a direct method call. The rule trusts that a `.fulfill(...)` callee
  outside the closure is Playwright's `Route.fulfill`, which serializes its argument into a
  response and hands it to no test.

Intent and boundary (GitHub #1737): on `b64eb76` the reporter's path-mode run still named an
unlined `playwright.config.ts` `testMatch` although every `testMatch` was a string literal. The
wholesale refusal of any project `testIgnore` named `testMatch` without a line (inferred; the
reporter's config is `NOT_OBSERVED`). The tag closure also stopped at
`settings[key] = value` in a route-mocking helper. There `const settings = { ...body.settings }`
reaches only an `in` test and `route.fulfill({ response, json: { ...body, settings } })`.

- Non-goals:
  - No general value-flow analysis that would prove other constructs keep their values from a
    test: no `let` or `var` objects, no aliasing, no passing to a call other than
    `.fulfill(...)`, and no return. `TJAA-V0-096` later admits a `let` binding that is never
    reassigned and scopes the proof to one binding.
  - No computed read of a fresh object, and no compound write (`||=`, `+=`). Such an access reads
    the member, and a spread initializer can copy a tag array into it. `TJAA-V0-097` and
    `TJAA-V0-098` later admit the positions where that member value cannot flow on.
  - No regular-expression `testIgnore`, and no global `testIgnore`.
  - No change to `projects`, `owns <file>` or membership-unknown sites, which still carry no line.
- Failure modes, each fail-closed:
  - Any other use of the object stops at the write's line with `a computed member access can
    reach a tag array without naming it`. That covers a read, an alias, a call argument, a
    return, a template substitution, a property read, a parameter or second declaration, a
    `let` declaration, an exported or aliasing initializer, a wrapped or conditional `fulfill`
    argument, another route method, a compound write and a computed read. `TJAA-V0-096` to
    `TJAA-V0-098` later admit a `let` that is never reassigned, and a compound write or computed
    read in the positions they name.
  - A `this` or `super` in any closure file stops at its line. A `fulfill` definition or read
    stops at its line while an object reaches a `fulfill` argument.
  - A method named by an escaped string (`'\x66ulfill'(o) {}`) stops at its line in every
    closure, because the `split`, `map` and `fulfill` guards cannot read its name (PR review).
  - A `testIgnore` the rule refuses names its own line.
- Residual:
  - A `.fulfill(...)` callee outside the closure that is not Playwright's `Route.fulfill` is
    outside the static model, as are patched globals (`AFP-V0-409`).
  - A computed read on an object-literal receiver (`{ ... }[key]`) is not yet treated as a member
    access by the closure's receiver test, and neither is `value![key]`. `TJAA-V0-103` later stops
    at `{ ... }[key]`.
  - A membership-unknown site, `projects` and `owns <file>` still name the config without a line.
    `TJAA-V0-104` later names each line, or why it has none.
  - The reporter's repository was not available (`NOT_OBSERVED`).

- `TJAA-V0-096`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1737
  item 2) Amending `TJAA-V0-095`, a computed member access `NAME[...]` MUST NOT make its
  tag-closure file inexact only when a scoped escape analysis of NAME's binding proves that the
  bound object's value cannot reach a test or describe title, tag or details argument. The proof
  has four parts.
  - The binding is the innermost `const NAME = { ... }` or `let NAME = { ... }` whose lexical scope
    holds the access. That scope is the enclosing `{ ... }` block or function body, or the module.
    The declaration sits directly in that scope, names NAME first in its declaration, and keeps the
    `TJAA-V0-095` initializer and export conditions. A `for (...)` header declaration, a
    destructuring pattern, a later name of a declaration list and every `var` bind no fresh object.
  - Every code occurrence of NAME in that scope is a use that `TJAA-V0-095` or one of
    `TJAA-V0-097` to `TJAA-V0-099` admits. Occurrences inside a nested block that declares its own
    `const` or `let` NAME belong to that inner binding and are proven separately. Any other
    occurrence refuses the binding. That covers a parameter, a `var`, function, class or catch
    binding, a reassignment or update of a `let`, a read, an alias, a call argument and a return.
    So no binding that shadows NAME between the declaration and the access can pass for it.
  - No file of the closure holds a `with (` statement, which resolves names against an object
    before the scope chain.
  - No file of the closure defines an accessor (`get` or `set` followed by a property name, in an
    object literal or a class). A getter or setter on the literal, or on a prototype that a
    computed `__proto__` write installs, runs on a computed member access. It can then hand the
    value it reads or is written to a test without `this` or the object's name. This guard stops
    at the accessor's line (PR review).
  - NAME holds only ASCII bytes and never occurs next to a non-ASCII byte. The token matcher ends an
    identifier at a combining mark, a connector or a zero-width joiner, so `box\u0301` could
    otherwise pass for a declaration or a use of `box` (PR review).
  - A shorthand, value or spread of NAME in a `.fulfill(...)` argument is admitted only while no
    object literal of that chain is followed by an assignment operator, since `({ NAME } = value)`
    writes into NAME instead of reading it.

  The `TJAA-V0-095` `this`, `super` and `fulfill` stops and the `AFP-V0-409` reflective-name stops
  still apply to the whole closure.
- `TJAA-V0-097`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1737
  item 2) A compound computed write `NAME[key] op= value` MUST be admitted when it begins a
  statement, so the statement discards its value. Compound operators include `||=`, `&&=`, `??=`,
  `+=` and every other compound assignment. A statement begins after the start of the module,
  `;`, a code `{` or `}`, `else`, `do`, or the `)` of an `if`, `for` or `while` header. In any other
  position the write stops. Such positions include an arrow body, an initializer, a call argument,
  a label and a template substitution, where the old member value can flow on.
- `TJAA-V0-098`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1737
  item 2) A computed read `NAME[key]` MUST be admitted only where an operator consumes it into a
  boolean or a type string:
  - the left operand of `===`, `!==`, `==` or `!=`;
  - followed by an explicit end, the right operand of one of those operators, the operand of `!` or
    `typeof`, or the whole test of an `if (...)` or `while (...)`.

  The explicit ends are `)`, `]`, `}`, `;`, `,`, `:`, a conditional `?`, `&&`, `||`, `??` and an
  equality operator. `?.`, a logical assignment, a line break and the end of the module are not
  ends, since they can continue the operand. A read after `new` or `await` stops, since the
  operator runs the read value's constructor or `then` first.
- `TJAA-V0-099`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1737
  item 2) NAME as the whole first argument of a plain `JSON.stringify(...)` call MUST be
  admitted only when the call is itself a whole property value of object literals nested directly
  in a `.fulfill(...)` argument, as in `body: JSON.stringify(NAME)`. The returned string carries
  the object's members, so a call whose string can reach anything else stops. Examples are an
  initializer, a test title argument and a conditional operand (PR review). Such a call counts as
  an object sink for the `TJAA-V0-095` `fulfill` stop. The call is plain when
  `playwrightPlainStringify` (`TJAA-V0-090`) accepts it: no replacer function and no spread.
  Inside a wrapper call or an object literal the use stops. With a replacer, or through `JSON?.`,
  the file stops at its `JSON`.

Proof rule and boundary (GitHub #1737 item 2): the issue asks that a construct stop the closure
only if its value can reach a title, describe title or `tag` argument. `TJAA-V0-096` to
`TJAA-V0-099` deliver that rule for computed member accesses on fresh object bindings, and only
where a lexical proof exists. Each admitted use of the binding yields one of these:
- a key, a boolean or a type string;
- a value its statement discards;
- the object, or its plain JSON string, handed to Playwright's `Route.fulfill`, which serializes
  it into a response.

So neither the object nor a member read from it reaches any other expression. A computed write
can then set a `tag` only on an object that no test call receives, and a computed read can return
a tag array only into a primitive. The methods that these operators run implicitly bind the object
only through `this` or `super`, which the closure refuses. Such methods include `toJSON`,
`valueOf`, `toString`, `Symbol.hasInstance` and `then`. A getter or setter also receives the value
read or written, so the closure refuses every accessor definition while it admits a fresh object. Every reflective route to a
binding is already inexact under `AFP-V0-409`, including `eval`, `Function`, `constructor` and the
global object aliases. Soundness takes precedence over breadth: every construct the analysis
cannot classify fails closed.

- Non-goals:
  - No alias, points-to or interprocedural analysis. An alias, a call argument other than a
    `.fulfill(...)` argument, a return, an export or a closure capture that hands the object on
    stays refused.
  - No `var` binding, no declaration in a `for (...)` header or a destructuring pattern, and no
    later name of a declaration list.
  - No computed read in any position outside `TJAA-V0-098`, such as an `||` or `??` operand, a
    member, a call, a computed key or an initializer.
  - No sink other than a direct `.fulfill(...)` argument. `route.continue`, `request.post` and
    `page.evaluate` are refused, since the value they return can come back to the caller.
- Failure modes, each fail-closed at the line of the binding's first computed member access, with
  `a computed member access can reach a tag array without naming it`:
  - an unadmitted occurrence anywhere in the binding's scope;
  - an access outside every fresh declaration's scope;
  - a `with` statement in the closure;
  - a NAME that holds or adjoins a non-ASCII byte;
  - a destructuring assignment into the binding;
  - a compound write outside statement position;
  - a read outside the `TJAA-V0-098` positions;
  - a stringified argument that is not the whole first argument of a plain `JSON.stringify` call.
  A stringify call with a replacer, or through `JSON?.`, stops at its own `JSON` line. An accessor
  definition anywhere in the closure stops at its own line once a fresh object is admitted.
- Residual:
  - The issue's own example, the reporter's helper, is admitted since `TJAA-V0-095`. The general
    form the issue asks for cannot be proven lexically for an aliased object, an object passed to
    a call, a returned object, or a value read into anything but a primitive. Those cases need an
    interprocedural alias analysis that this adapter does not have, so they still stop.
  - The callee of `.fulfill(...)` is still trusted to be Playwright's `Route.fulfill`. Patched
    globals stay outside the static model (`AFP-V0-409`). So does a browser that the closure
    launches at collection time and that could hand a fulfilled body back through
    `page.evaluate` or `response.json()`.
  - A script that is not a module shares its top-level `let` and `const` bindings with other
    scripts by name. The adapter treats every closure file as a module.
- `TJAA-V0-116`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1881
  item 1) Amending `TJAA-V0-096`, an occurrence of NAME in the binding's scope MUST be admitted as
  a type member key, which names no binding, when its innermost bracket is a `{` and it is either
  `NAME?:` after `{`, `,` or `;`, or `NAME:` after `;`. No expression can follow a name with `?`
  and then `:`, so `NAME?:` is only an optional member of a type literal, an interface or a class
  field. After `;`, `NAME:` is a type member key or a statement label, and a label is no read.
  A `break NAME` or `continue NAME` still refuses the binding.
  - Non-goals: no general type-position parser; `readonly NAME:`, `NAME!:` and method signatures
    still refuse the binding.
  - Failure modes, each fail-closed at the binding's first computed member access: an optional
    parameter `(NAME?: T)`, whose innermost bracket is `(`; a conditional `NAME ? a : b`; an
    optional chain `NAME?.member`.
- `TJAA-V0-117`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1881
  item 2) A direct call `Object.entries(NAME)`, whose only argument is the plain identifier NAME,
  MUST NOT stop the tag closure at its `Object` or `entries` token when NAME resolves to a fresh
  object binding that `TJAA-V0-096` admits and whose literal initializer holds no spread. Such a
  call is then an admitted use of the binding, and the file counts as accessing a fresh object, so
  the `TJAA-V0-095` `this`, `super` and `fulfill` stops and the `TJAA-V0-096` accessor stop apply.
  The call returns new arrays of the object's own keys and of the values the file wrote into it.
  Every literal tag array is the value of a literal `tag` key, and the closure refuses every other
  route to one (naming `tag`, a computed member on any other receiver, the reflective names), so
  none of those values is a tag array. A spread could copy another object's `tag` member into the
  literal, so a spread anywhere in the initializer keeps the call stopped.
  - Non-goals: `Object.keys`, `Object.values`, `Object.fromEntries` and every other `Object`
    member still stop; so does `Object.entries` over any other value, such as a parameter or the
    generic id-collecting walk of the issue's second file.
  - Failure modes, each fail-closed at the `Object` line: a spread in the initializer, at any depth;
    a second argument; a parenthesized, member or call argument; `Object.entries?.(...)`; a
    binding that another use refuses, such as an escape or a shadowing parameter.
- `TJAA-V0-118`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1881
  item 3) `corvint affected discovery` MUST accept an optional
  `--playwright-ungrepped-list FILE` (library `PlaywrightDiscoveryFromLists`): the JSON report of
  the same unfiltered `playwright test --list --reporter=json` run taken with every project `grep`
  and `grepInvert` removed from the config. The producer MUST refuse it under every refusal of the
  discovery listing (errors, filtering, sharding, another config, a rootDir or test outside the
  root, a spec without a project) and when it names a project/file pair the config does not
  select in the current sources. Without it the producer is unchanged, and the
  `playwright-discovery/0` wire does not change: a verified omission is a unit that is not in
  `unverifiedUnits`.
- `TJAA-V0-119`: (same acceptance) The ungrepped listing MUST verify a grep omission only exactly.
  An execution is keyed by project, repository-relative file, line, column and title path. The
  ungrepped listing verifies no project unless no key repeats in either listing, every execution
  of the discovery listing is in the ungrepped listing, and every project without `grep` or
  `grepInvert` lists the same executions in both. It then verifies a project with `grep` or
  `grepInvert` only when both patterns are statically evaluable part by part
  (`playwrightTitleMatchers`) and, for every execution the ungrepped listing places in that
  project, the case is inside the matcher model and the configured patterns, applied to the
  project name, the title path and the reported explicit tags, keep it exactly when the discovery
  listing lists it. An omitted pair is explained when its project is verified and the ungrepped
  listing places a case of its file in that project.
- `TJAA-V0-120`: (same acceptance) Every omission the ungrepped listing does not verify MUST keep
  the `TJAA-V0-021` rules: it stays an unverified unit and keeps
  `playwright:discovery-omission-unverified`, or it is refused. A verified omission also stands
  when the discovery listing alone could not explain it, such as a file with no case kept in any
  project.
  - Non-goals: the producer runs no Playwright command, and it does not bind the ungrepped listing
    to the config bytes it was taken with; the second listing does not change the tag closure.
  - Failure modes, each leaving the omission to `TJAA-V0-021`: a dynamic or non-partwise pattern;
    a case outside the matcher model; a prediction that disagrees with the discovery listing
    (hidden tag text Playwright reports only in part); a missing pair; a repeated key; a project
    without grep listed differently. A malformed, filtered or foreign ungrepped listing is
    refused.
  - Residual: both listings stay caller-declared. Two listings from different sources or
    environments that agree in every checked respect, such as a test defined at run time in only
    one of them, are not detected.
- `TJAA-V0-100`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1750)
  A `--playwright-config` plan or discovery check MUST NOT read, as a graph input, a worktree path
  that Git ignores and HEAD does not track. Such paths come from
  `git ls-files --others --ignored --exclude-standard --directory`, the same list that
  `AFP-V0-094` uses to refuse `--path`. Every walk of the root skips them for the whole command:
  the shared TypeScript graph, the Playwright source digest, tag sources and discovery
  membership. They raise no frontier, enter no receipt and are not dirty paths. Two kinds of path
  stay graph inputs and keep their frontiers: a tracked file that an ignore rule matches (Git
  never lists it as ignored), and an untracked file that no ignore rule matches. A change in that
  ignored list between the start and the end of the compile MUST refuse with
  `unsupported-affected-drift`. An import resolver that tries several candidate files MUST treat
  an excluded candidate as unresolved and MUST NOT fall through to a later candidate. An entry
  holding U+FFFD (Git's lossy decoding of an invalid name) or no valid relative path MUST exclude
  nothing. A walk that records a graph cache stamp MUST NOT apply the exclusion.

- `TJAA-V0-105`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1737
  residual, V1-2675)
  Amending `TJAA-V0-094`, a project `testIgnore` glob that is a literal root-free string glob MUST
  NOT raise the executable-config frontier only because it could match an observed path that its
  project's `testMatch` holds through directories above the repository. Such a match can only
  drop that path from the project, never add one, so the static membership, which keeps the
  path, is a superset of Playwright's for every checkout location. This holds however many
  `projects[]` entries repeat the glob, as in the reporter's `testIgnore:
  'tests/examples/**/*.spec.ts'` in three projects. A path outside the matcher model, a computed
  or regular-expression value, a glob outside the `TJAA-V0-092` rule and a global `testIgnore`
  MUST still name that `testIgnore`'s own line. A `testMatch` glob that reaches above the
  repository still refuses, since it can add a path. Residual: when a checkout's own parent
  directories do make the glob match, a selected test that the project ignores runs no test in
  that project; Playwright then reports that it found no tests rather than passing silently.
  Rollback: restore `playwrightGlobReachesRoot` in the `testIgnore` loop of
  `playwrightStaticMembership`; only executable-config frontier entries change.

Intent and boundary (GitHub #1750): after a local Playwright run, the gitignored
`playwright-report/trace/assets/*.js` bundles hold `import(name)`. The graph walked them as source,
so their dynamic-import frontiers forced `FULL_RELEVANT_SUITE` for a one-spec change. Git already
says that such a file is no project input.

- Non-goals:
  - The generic `affected` graph (no `--playwright-config`) and its graph cache are unchanged.
    `--path` keeps refusing an ignored file that HEAD does not track (`AFP-V0-094`), except one
    that only a TypeScript unit lists, which `AFP-V0-440` (GitHub #1774) excludes by
    this rule.
  - No reading of `.gitignore` by Corvint itself: only Git's own listing decides.
  - No exclusion of tracked files, and no other new exclusion.
- Failure modes, each fail-closed:
  - A local import that resolves only to an excluded file is unresolved and raises its frontier.
    So does an import whose excluded candidate comes before a tracked alternative, such as
    `./helper.cjs` beside a tracked `helper.js` (PR #1753 review).
  - An ignored name Git cannot decode exactly stays a graph input (PR #1753 review).
  - A spec file under an ignored directory drops out of static membership, so a discovery
    listing that names it mismatches and refuses.
  - A failed `git ls-files` refuses with `unsupported-affected-status`.
  - A concurrent compile of the same root that holds a different ignored list refuses.
- Residual:
  - The generic `affected` TypeScript graph still reads ignored, untracked files.
  - The exclusion is held per root for the process. Another walk of the same root in the same
    process during the hold would apply it too; no such in-process caller exists today.
  - Git's `--directory` listing does not name an ignored file inside an untracked directory that
    also holds an unignored file. That file stays a graph input, which is fail-closed.
- `TJAA-V0-101`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1751)
  Extending `AFP-V0-160` to the `playwright-affected/0` plan, a dirty path that the
  repository's `.corvint/documentation-paths.json` declares, and that therefore raised no
  `UNOWNED_DIRTY_PATH` in the shared graph, MUST be recorded and checked against observed
  evidence. There are four parts.
  - (1) The plan carries a `declaration` member: `path` (the declaration's path), `sha256` (of its
    bytes), `matched` (the declared dirty paths, in dirty-path order) and `state`. The member is
    present only when the declaration matched a dirty path, so other plans keep their bytes. The
    declaration is read again and MUST still hold the entries the graph was built with; otherwise
    the plan fails rather than records another declaration.
  - (2) The declaration is contradicted when any file the Playwright observer reads (the config
    and every `ObservePlaywrightSources` input except the declared path itself) names a matched
    path as a string. A name is the whole path or a trailing part of it (the file name, or a path
    relative to an ancestor) closed by a quote. It may also be the path or one of its ancestor
    directories, opened by a quote, `./`, `../` or a template substitution's `}/`. It is closed by
    a quote, or by `/` before a quote, substitution or glob character. A quote that opened a
    spelling closes it only at the same quote character; a spelling opened by `/` closes only at
    the nearest quote before it on its line, or at any quote when there is none or a template
    substitution's `}` lies between. Matching runs on the text with
    JavaScript string escapes decoded (`\u002e`, `\u{2e}`, `\x2e`, `\/`, line continuations).
    An escaped quote, backslash or control escape neither opens nor closes a spelling. A static
    import, a `readFileSync` or `readdirSync` literal, a fixture's `join("docs", ...)` and a config
    import each name it.
  - (3) A contradicted declaration has `state` `REFUSED`. Each contradicted path raises one
    `SELECTION` unknown `playwright:declared-path-contradicted`. The unknown names the
    declaration, its digest, the path, and the first evidence site as file, line and spelling.
    The plan therefore falls back to `FULL_RELEVANT_SUITE`. Project authority narrows only where
    no observed evidence disagrees with it.
  - (4) An applied declaration has `state` `APPLIED`, and every `excluded` entry carries
    `declaration`, the declaration's path. Without the declaration, the matched paths would leave
    the plan unbounded.
  Part (2) is amended by `AFP-V0-442` (V1-2693): only a file in the Playwright closure, or one
  whose closure membership is unknown, contradicts the declaration; any other reader is listed
  in the member's `outOfClosure`.

Intent and boundary (GitHub #1751): a branch that changes JSON metadata under `docs/` that no
test reads fell back to the full suite through `UNOWNED_DIRTY_PATH`. The existing project-owned
declaration (`AFP-V0-160`) already narrowed that plan, but the receipt did not say so. Its reader
rule also selects nothing for a TypeScript test, because the TypeScript plugin records no path
literals.

- Non-goals:
  - No new declaration file or glob grammar. The `AFP-V0-160` file and directory entries are the
    declaration.
  - No glob-to-owning-tests mapping.
  - No change to the generic `affected-plan` selection. `TJAA-V0-102` later records TypeScript
    path literals, so the generic reader rule selects a TypeScript test that reads the path.
- Failure modes, each fail-closed:
  - A string that the rule does not count as a name is no contradiction: a route such as
    `"/docs/contracts"`, another directory such as `"src/docs"`, or a longer sibling path. A test
    that reads the path through such a spelling, for example an absolute path built from a
    route, is not seen.
  - Every counted match refuses the declaration, including a match in a comment.
- Residual:
  - A path computed without any literal part (`readFileSync(process.env.SETUP_PATH)`), a Windows `\` spelling
    and a reader outside the observer's file set (a Node script that Playwright never loads) are
    not seen. This is the `AFP-V0-160` computed-path blind spot.
  - The reporter's repository was not available (`NOT_OBSERVED`).

- `TJAA-V0-102`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1751,
  V1-2661) Extending `AFP-V0-021` to the TypeScript plugin, every TypeScript unit MUST record as
  `PathTokens` the path-shaped tokens of its own string-literal, template-text and
  regular-expression contents (without the expression's `/` delimiters). A token is a run of the `AFP-V0-012` lexicon
  (`[A-Za-z0-9._~@+/-]`) in the content after JavaScript string escapes are decoded (the
  `TJAA-V0-101` decoder). It is kept when it carries a `/` or `.`, is not a bare number, and has a
  letter or digit, as the shell plugin keeps one. Comments and code contribute no token. A module
  whose comments or literals do not tokenize is read whole instead, so a literal the lexer cannot
  place still names its paths. A Storybook story set records the union of its files' tokens. A
  unit with more than `MaxPathsPerUnit` distinct tokens keeps none and is marked
  `PathTokensBounded`. The generic `affected-plan` reader rule then selects a TypeScript test that
  names a dirty path, including a declared documentation path (`AFP-V0-160` (3)), with a
  `PATH_LITERAL_READER` witness. A bounded unit that nothing else reaches raises the named
  `LANGUAGE_FRONTIER` unknown `typescript:path-token-bound:<unit>`.

Intent and boundary (GitHub #1751): `TJAA-V0-101` recorded and checked the declaration only in the
Playwright plan. The generic plan classified a declared `docs/` path as documentation while a
TypeScript test that read it through `readFileSync("docs/...")` was excluded with
`NO_DEPENDENCY_PATH_TO_DIRTY_UNIT`, because the TypeScript plugin recorded no path literals. The
`AFP-V0-160` reason states that every unit whose path literal reaches the path is selected; this
makes that true for TypeScript.

- Non-goals:
  - No resolution of a token against the unit's directory or a runtime root. A non-Go unit keeps
    the component-run relation (`AFP-V0-072`), so a route such as `"/docs/contracts"` selects its
    test, which the `TJAA-V0-101` contradiction rule does not count as a name.
  - No exclusion of import specifiers. An import of `./helper.js` also names any dirty
    `helper.js`, an over-selection that never drops a reader. `TJAA-V0-126` later amends this for
    a specifier that resolves to a unit.
  - No change to the reader rule, which still does not traverse a reader to its dependents.
- Failure modes, each fail-closed:
  - A module that does not tokenize contributes every lexicon run of its text (or of its
    comment-free text), never none.
  - A unit over the token bound keeps no token and names the bound as an unknown.
- Residual:
  - A test-less helper module that reads a declared path is selected as a reader but contributes
    no check, and the tests that import it are not selected, because the reader rule does not
    traverse (`AFP-V0-021`). The path is still classified as documentation. `AFP-V0-495` later
    traverses such a reader when no unit owns the path.
  - A path computed without any literal part, and a JSX text run the lexer misreads as code, are
    not seen.
  - The reporter's repository was not available (`NOT_OBSERVED`).

- `TJAA-V0-103`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1737,
  V1-2629) Amending `TJAA-V0-095`, a `[` that follows a `}` in a tag-closure file MUST stop the
  closure at its line with the reason ``a `[` after `}` can index an object literal and reach a tag
  array without naming it``. A `}` ends an object literal as well as a block, so `{ tags: [...] }[k]`
  reads or writes a member of a literal that can hold a tag array. A block followed by an array
  literal stops too, conservatively. A literal index (`{ a: 1 }[0]`), and the empty `[]` of an
  array type such as `{ a: number }[]`, stays exact.

Intent and boundary (GitHub #1737): the `TJAA-V0-095` residual named a computed read on an
object-literal receiver as not treated as a member access by the closure's receiver test.

- Non-goals: no proof that the indexed literal holds no tag array; no change to `value![key]`.
- Failure modes, each fail-closed: a read, a write and a block followed by an array literal each
  stop at the `[` line.
- Residual: `value![key]` is still not treated as a member access by the receiver test.

- `TJAA-V0-104`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1737,
  V1-2631) Amending `TJAA-V0-095`, every path-mode executable-config frontier site
  (`typescript:executable-config:<site> <token>`) MUST name its blocking line, or append to its
  token `(no line: <why>)`. There are four sites.
  - (1) `projects`, when no project set is static, names the line of the config's `projects`
    key.
  - (2) `owns <file>` names the line of the owning project's `testMatch` key, or of the project
    entry when its `testMatch` is the default.
  - (3) A membership-unknown site names the construct its detail reports. An export or dynamic
    config names its `export default`; an array or identity detail names `projects`; an entry,
    name or duplicate detail names that entry; a global key names that key outside `projects`;
    a per-project key names that key inside its project's entry. Only the two global phrasings
    (`global testDir is not one static relative path`, `global <key> is outside the static
    project subset`) are global, so a project named `global` still names its own key.
  - (4) An escaped matcher key such as `"test\u004datch"` names the line where the decoded key is
    spelled.

  A site that has no line names why: the config does not tokenize, `projects` is absent, the
  config has no `export default`, the config does not spell the key, or the owning project entry
  is not located.

Intent and boundary (GitHub #1737): the `TJAA-V0-095` residual left the membership-unknown,
`projects` and `owns <file>` sites naming the config without a line, so an operator could not
find the construct that blocked a path-mode run.

- Non-goals: no change to which configs are refused; only the site and token text change.
- Failure modes, each fail-closed: a construct the locator cannot find keeps the config-level
  site and records why; the refusal itself never changes.
- Residual: the line is the first spelling of the key, so a config that spells a key in a nested
  object before the project entry can name that earlier line.

### Playwright selection scope (GitHub #1877-#1880)

- `TJAA-V0-106`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1877,
  V1-2777) Amending `TJAA-V0-013`, a dirty TypeScript or JavaScript path whose unit reaches no
  Playwright test by a static edge (the shared plan's `NO_SELECTABLE_TEST`) MUST raise the
  selection unknown `playwright:out-of-suite-source`, which forces `FULL_RELEVANT_SUITE`; the plan
  MUST NOT be `BOUNDED` with zero selected for it. The detail names at most 8 paths, counts the
  rest, and names the remedy. A repository MAY declare paths outside the suite's reach in
  `.corvint/playwright-unreached-paths.json`
  (`{"profile":"corvint-playwright-unreached-paths/0","paths":[...]}`, the AFP-V0-160 (1) entry
  rules: repository-relative files or `/`-ended directories, strictly ascending, at most 256, none
  under `.corvint/`, at most 64 KiB, closed JSON). A dirty path the declaration matches is
  excluded only when the Playwright closure provably leaves it out; a matched path the closure may
  load (its unit is in the closure, it is no graph unit, it is the config, or closure membership is
  incomplete) MUST refuse the record and raise a second `playwright:out-of-suite-source` unknown
  naming the path and why. An invalid declaration is an error, never an empty one. The unknown is
  exempt from the `TJAA-V0-015` `STATIC_UNRESOLVED` discovery reconciliation, like the dynamic
  source unknown, so a matched discovery receipt still runs the full suite.
- `TJAA-V0-107`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1877,
  V1-2777) A plan that matched the unreached declaration MUST record it as `plan.unreached`
  `{path, sha256, matched, state}` with state `APPLIED` or `REFUSED`, and every `excluded` entry of
  an `APPLIED` plan MUST cite it as `unreached`. The Playwright source digest MUST bind the
  declaration's identity, so a changed declaration invalidates a discovery receipt.
- `TJAA-V0-108`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1878,
  V1-2778) In Playwright mode a shared TypeScript graph frontier reason MUST widen the plan only
  when one of its sites can change the answer: the reason has no located site; closure membership
  is over its bound; a site file is dirty; a site's unit is in the Playwright closure or on a
  reverse-reachability path from a dirty file; or a site outside every unit is a source file a
  Playwright project would load as a test, a config-like file in the root or in an ancestor
  directory of the config or of a closure file (except for the cross-language asset reason, see
  `TJAA-V0-112`), or a path a closure unit or dirty-reached unit may name (a closure unit keeps no
  path tokens over the bound, or the reason's unowned files reach a relevant unit). Every other
  reason is informational. Narrowing a frontier MUST NOT remove the `TJAA-V0-106` unknown.
- `TJAA-V0-109`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1878,
  V1-2778) Each informational reason MUST be listed in `plan.informational` with axis
  `INFORMATIONAL`, its sites, and why it cannot change the selection; an over-bound source also
  names the `AFP-V0-444` remedy. It MUST NOT appear in `unknown`, change `scope` or `fallback`, or
  enter the closure-completeness check of `TJAA-V0-101`.
- `TJAA-V0-110`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1878,
  V1-2778) Advisory narrowing (`TJAA-V0-030..033`) MUST skip an informational reason, both in the
  observed frontier and in the shared plan's `LANGUAGE_FRONTIER` unknowns.
- `TJAA-V0-111`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1879,
  V1-2779) A package-manager lockfile (`pnpm-lock.yaml`, `shrinkwrap.yaml`) MUST NOT raise
  `typescript:cross-language-test-asset-unowned` in any directory. A repository MAY declare
  non-test-asset paths in `.corvint/non-test-asset-paths.json`
  (`{"profile":"corvint-non-test-asset-paths/0","paths":[...]}`, the `TJAA-V0-106` entry rules); a
  declared path raises no asset frontier, an invalid declaration is an error, the Playwright plan
  records it as `plan.nonTestAssets` `{path, paths, sha256}`, and the source digest binds it.
- `TJAA-V0-112`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1879,
  V1-2779) In Playwright mode the cross-language asset reason MUST widen only under `TJAA-V0-108`'s
  dirty, closure, reverse-reachability or naming tests; an unnamed asset in a directory the suite
  does not load is informational.
- `TJAA-V0-113`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1880,
  V1-2780) A tsconfig `baseUrl` or `paths` target with a `../` MUST be accepted when the joined,
  cleaned path stays inside the repository root; one that escapes it, or an absolute value, stays
  refused with the `TJAA-V0-053` site.
- `TJAA-V0-114`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1880,
  V1-2780) A tsconfig `extends` that is one string starting with `./` or `../`, resolving inside
  the root to an observed config (with `.json` appended when the value lacks it) MUST be followed
  with TypeScript's inheritance: the nearest config that sets `baseUrl`, `paths` or `allowJs` wins,
  a `baseUrl` resolves against its declaring config, and `paths` resolve against the effective
  `baseUrl` or, with none, against their declaring config. A chain is followed for at most 8
  hops; a longer one is refused as `extends depth bound 8` at the `extends` key. A package name,
  an array, an escape, an unobserved target, a cycle, or a refusal inside an inherited config is
  refused as `extends`. Every `tsconfig*.json` (and, for Playwright, every `jsconfig*.json`) MUST
  be observed, bound into the Playwright source digest, and read by the tag closure, so a sibling
  target such as `tsconfig.base.json` resolves from real source bytes. Where a second runner keeps
  aliases unresolved, a `tsconfig.json` whose observed chain declares `paths` or `baseUrl` MUST
  raise the path-alias frontier pinned at the declaring config's key.
- `TJAA-V0-115`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1880,
  V1-2780) A Playwright global or project `testDir` with a `../` MUST be accepted when it stays
  inside the root; one that escapes it stays the project-membership unknown.

Intent and boundary (GitHub #1877-#1880): an unreached application source let the plan say
`BOUNDED` with zero selected although an end-to-end run loads the application; a frontier in an
unrelated package forced the full suite; lockfiles and unrelated YAML were treated as test assets;
and an inside-root `../` or an `extends` chain was refused outright. Safety outranks narrowing:
the out-of-suite unknown is never removed by frontier scoping.

- Non-goals: no runtime model of what the application under test serves; no observation of
  configs other than `tsconfig*.json` and `jsconfig*.json` as `extends` targets; no package
  `extends` resolution; no YAML runner attribution.
- Failure modes, each fail-closed: an undeclared unreached path widens; a declared path the
  closure may load refuses the declaration; an invalid declaration is an error; an incomplete or
  over-bound closure makes every reason relevant; an unresolvable or over-deep `extends` keeps the
  named refusal.
- Residual: an `extends` to a file outside the `tsconfig*.json`/`jsconfig*.json` spellings (for
  example `./configs/base.json`) is unobserved and still refused; a monorepo whose dirty paths include non-Playwright unit-test sources widens until it declares
  them; the extra closure and reach computations are not yet measured for latency.

### Playwright rc.3 fixes (GitHub #1883-#1887)

- `TJAA-V0-121`: (experimental under the standing lane auto-accept (2026-10-11); GitHub #1884,
  V1-2785) Amending `TJAA-V0-106`, every dirty path outside the TypeScript graph (the shared
  plan's `UNOWNED_DIRTY_PATH`), whatever its language, MUST raise its own
  `playwright:out-of-suite-source` selection unknown naming the path and the
  `.corvint/playwright-unreached-paths.json` remedy, so fail closed never depends on an unrelated
  graph frontier also being unresolved. A path the declaration matches is applied, and leaves
  `playwright:dynamic-source-reachability` (it no longer counts as an unresolved graph entry),
  only when none of these holds: it is the Playwright config or the declaration itself; closure
  membership is bounded or incomplete; it lies under a project's (or the global) test directory;
  it may configure the run (`TJAA-V0-108`'s config-like test over the root and the ancestors of
  the config and of every closure file); or an observed source inside the closure, or of unknown
  membership, names it as a string. Otherwise the declaration is `REFUSED` with an unknown naming
  the path and why.
  - Non-goals: no model of what the application under test loads at run time; no change to
    `TJAA-V0-030..033` advisory narrowing, which still refuses on any unowned path.
  - Failure modes, each fail-closed: an undeclared unowned path widens; a declared path matching
    any condition above refuses the declaration; a root-level config whose test directory is `.`
    refuses every unowned declaration (conservative residual).
- `TJAA-V0-122`: (same acceptance; GitHub #1885, V1-2786) A bare import specifier MUST NOT raise
  the path-alias frontier when `node_modules/<package>/package.json` is a regular file, following
  symlinks, in the importing file's directory or an ancestor inside the root, such as a
  `node_modules` symlinked to a store outside the root, and the nearest such package directory,
  with every symlink followed, resolves outside the root's real path or inside a `node_modules`
  directory below it (for example a pnpm `node_modules/.pnpm` store). Only the manifest's
  existence and where the package directory resolves are observed; the `node_modules` path
  segments and the resolution are recorded for the graph cache stamp (`AFP-V0-391`). The package
  is a dependency location, never graph source.
  - Non-goals: no package `exports`, `main` or version resolution; no read of the manifest.
  - Failure modes, each keeping the frontier: a package installed nowhere; a package directory
    that resolves into repository source outside every `node_modules` directory (a workspace or
    alias link such as `node_modules/alias -> ../src/lib` whose manifest names another package,
    which would otherwise under-select tests of the linked source); an immutable tree source,
    which has no on-disk resolution; an unresolvable root or package directory. A `..`, `.` or
    empty package segment is not looked up; an immutable-tree stat error other than not-exist is a
    source error.
- `TJAA-V0-123`: (same acceptance; GitHub #1887, V1-2787) A top-level `metadata` member of the
  Playwright config MUST NOT raise a config-syntax unknown or refuse discovery, as project-level
  `metadata` already does not: Playwright uses it only for reporting. `config.sha256` still binds
  the config bytes that carry it, so a changed `metadata` still invalidates a discovery receipt.
  - Failure modes: an import or call it makes stays subject to every other config and graph rule.
- `TJAA-V0-124`: (same acceptance; GitHub #1883, V1-2784) `PlaywrightDiscoveryFromWrappedLists`
  MUST accept an ungrepped listing whose `configFile` names a declared wrapper config instead of
  the bound config only when the wrapper is a canonical TypeScript/JavaScript source other than
  the bound config, in the bound config's directory (so `testDir` and every other relative path
  resolve as for the bound config), at most 4096 bytes of UTF-8, and token for token
  `PlaywrightUngreppedWrapperTemplate`: it imports `./<config file name>` (with or without its
  extension) and removes only top-level and per-project `grep` and `grepInvert`. Comments,
  whitespace, quote style, semicolons and trailing commas may differ. Every `TJAA-V0-118..120`
  rule then applies to that listing unchanged.
  - Non-goals: the wrapper is not evaluated; no other wrapper shape is recognized.
  - Failure modes, each a refusal: an extra, missing or changed token (for example stripping
    `testIgnore` or overriding `testDir`); another directory; the bound config itself; an
    unreadable, oversize or non-UTF-8 wrapper; a wrapper without an ungrepped listing; a listing
    naming the wrapper without the declaration (the `TJAA-V0-118` foreign-config refusal).
- `TJAA-V0-125`: (same acceptance; GitHub #1883, V1-2784) `corvint affected discovery` MUST accept
  `--playwright-ungrepped-config PATH` at most once, as a repository-relative canonical path, and
  only with `--playwright-ungrepped-list`. It MUST refuse a wrapper that HEAD tracks, or when Git
  cannot say, and MUST hold the wrapper out of every source walk of the discovery (the
  `TJAA-V0-100` hold), so the receipt's source digest equals the one a later plan computes after
  the wrapper is deleted. The held Git and declared list that the drift check compares does not
  include the wrapper. `corvint affected --help` documents the template and that the wrapper is
  deleted after discovery. The `playwright-discovery/0` wire does not change.
  - Failure modes: a tracked wrapper, a repeated, non-canonical or unaccompanied flag, and every
    `TJAA-V0-124` refusal exit 2 with no receipt.
- `TJAA-V0-126`: (experimental under the owner's standing lane auto-accept (2026-10-11); GitHub
  #1751, V1-2667) Amending `TJAA-V0-102`, a single-module unit's `PathTokens` MUST leave out a
  string literal that is exactly one quoted import, export-from, `require` or literal dynamic-import
  specifier at the offset the import scan parsed, when that specifier resolved to a unit. The
  module is reached through its import edge, so the specifier text naming a same-named file
  elsewhere would only over-select.
  - The same text in any other literal, such as `readFileSync('./helper.js')`, is still a token.
  - Non-goals: excluding an unresolved specifier, an alias-rewritten one or a Storybook story
    set's specifiers, which keep their tokens.
  - Failure modes: a module that calls `createRequire` keeps every token, because its `require`
    resolution base is unknown; a module that does not tokenize is read whole and keeps every token; a literal
    whose spelling differs from the parsed specifier is kept. Each keeps an over-selection, never
    an under-selection.
  - Rollback: call `pathTokens` instead of `pathTokensSkipping` and drop
    `unresolvedSpecifierTokens`.

### Per-test Playwright closures (V1-2812)

Intent (owner, V1-2812): `corvint runs stability --pool-by closure` should pool a test's runs
across edits the test cannot load without a caller hand-computing its closure. The adapter
computes, read-only, which files one bound Playwright test file may load and which it provably
cannot, from the same graph phase and closure walk the plan uses.

- `TJAA-V0-140`: (experimental under the standing lane auto-accept (2026-10-11); V1-2812)
  `PlaywrightTestClosures(root, config)` MUST return one `PlaywrightTestClosure` per distinct
  test file a project of the config admits, sorted by path, under rule `corvint-test-closure/0`.
  A test's closure is the `AFP-V0-442` walk seeded from the config's unit, the test's unit and
  the units of every test a project reached through its projects' `dependencies` or `teardown`
  admits (its own projects add only the test). `Files` are the files of the closure's units plus
  the config. `Excludable` are the files of units in the whole suite's closure that are not in
  the test's closure and that no unit outside the whole suite's closure reaches; a suite file
  that application code also imports is never excludable. Non-TypeScript files are never listed
  as excludable.
  - Non-goals: runtime loading (a file read with `fs`, a fixture loaded by URL), cross-test module
    state shared by one worker, and the application under test, which the run identity binds by
    revision.
- `TJAA-V0-141`: (same acceptance) A closure that cannot be computed MUST be unknown, never
  narrowed: every test is unknown when the config is not statically resolved, the whole suite's
  closure is bounded, or it exceeds 2048 units; a test is unknown when its closure has unit
  frontier, bounded path tokens or a relevant graph frontier site inside it (for example a
  dynamic import), when a seed file is not a graph unit, or when the walk disagrees with the
  whole suite's. A frontier outside the whole suite's closure (a unit outside it with frontier
  or bounded tokens, a frontier reason with no located site or a site outside it) MUST widen:
  the test's `Files` stay and `Excludable` is empty, with `Widened` naming the cause, since such
  code may load a suite file. A source change during the computation refuses.
- `TJAA-V0-142`: (same acceptance) The whole-suite plan MUST be unchanged: the graph phase is
  shared through `preparePlaywrightGraph` with identical errors, and the closure walk's seeds
  default to every bound test unit.
  - Acceptance: `TestPlaywrightTestClosures_TJAA140` and `TestPlaywrightTestClosuresFailClosed_TJAA141`
    in `playwright_test_closure_tjaa140_test.go`; the existing `internal/liveverify/affected/typescript`
    package tests pass unchanged (TJAA-V0-142).
- `TJAA-V0-147`: (experimental under the standing lane auto-accept (2026-10-28); V1-2914; GitHub
  #1920; `PWR-V0-183`) Amends `TJAA-V0-041`. A `\` escape inside a quoted string literal that is one
  whole element (preceded by the `[` or a `,`, followed by a `,` or the `]`) of an array literal
  that is not a computed member key and whose first two elements are the escape-free literals
  `'runs'` and `'identity'` MUST NOT refuse the `process` root or the numeric conversions: it is the
  argv of the config's own `corvint runs identity` call, where a `--project-test-dir` regexp
  carries escapes. The exemption is sound under `TJAA-V0-041` because such a string reaches a
  property name or a module only through a computed member key, `require`, dynamic `import(`,
  `Function`, `constructor`, `vm` or `eval`, each refused on its own site whenever its argument is
  not one escape-free literal. Every other escape in string or template text keeps refusing.
  - Acceptance: `TestPlaywrightTestClosuresIdentityArgvEscape_PWRV0183` in
    `playwright_test_closure_gh1920_test.go` (the issue's identity argv admitted with and without a
    dynamic `launchOptions`; the same escape in another argv, a computed key or a plain const still
    refuses).
- `TJAA-V0-127`: (standing auto-accept, 2026-10-27; V1-2584, AFP-V0-404, TJAA-V0-091) A static
  regular-expression `testMatch` of a Playwright project (a body that `playwrightRegexBodyStatic`
  accepts, flags only `i`, `m`, `s`) MUST resolve in path mode when its verdict on each test path
  cannot depend on the directories above the repository. Playwright searches the regex in
  `<checkout>/<path>`, so a regex with no start anchor and no word boundary (`^`, `\b`, `\B`) that
  matches `/<path>` matches on every checkout; one that does not is decided only when every match
  is end-anchored and no match can contain `/`. Any other regex, and a path outside the matcher
  model, MUST keep the `testMatch` executable-config site. A `testIgnore` regex resolves when it is
  context-free (no start anchor, no word boundary), since ownership remains a superset under the
  `TJAA-V0-105` argument. The resolver (`playwright_regex_root.go`) is shared with
  `corvint tests unselected` (TSA-V0-654).
  - Non-goals: lookaround, back-references and the `u`, `v`, `g`, `y` flags stay unresolved.
  - Failure modes: the rule is conservative, so an unanchored regex that misses a path keeps the
    site even when no real checkout could match it (safe widening).
- `TJAA-V0-960`: (experimental under the standing lane auto-accept (2026-10-11); V1-2533; amending
  `TJAA-V0-037` and `TJAA-V0-012`) The general adapter's alias path MUST read the alias config
  Playwright reads (AFP-V0-418): in each folder its `tsconfig.json`, or its `jsconfig.json` when no
  `tsconfig.json` is beside it, the nearest such folder governing an owner. A jsconfig.json that
  declares `baseUrl` or `paths` is an alias config for the gate exactly as a tsconfig.json is. With
  the gate open, a bare specifier of a module that is not `.ts` or `.tsx` MUST be mapped only when the
  nearest config sets a literal `allowJs: true`, or is a jsconfig.json without `allowJs`; otherwise
  it stays a bare reference and widens as an unresolved bare import. Under a blocked gate
  (`TJAA-V0-059`) the speculative edge is still added for every owner. `jsconfig*.json` is an
  observed name, so an edit to one is a resolution input (AFP-V0-511) and changes the source identity.
  - Non-goals: `checkJs`, `maxNodeModuleJsDepth` and the `.mts`/`.cts` owner rules of tsc.
  - Failure modes: a runner other than Playwright that reads a jsconfig.json differently is held by
    the existing gate; an unreadable `tsconfig.json` beside a jsconfig.json is not in the bodies, so
    the jsconfig.json is read, and the unreadable file raises its own frontier.
  - Falsifier: `TestGeneralAdapterJSConfigAliasesAndAllowJs_V12533` (a nearer jsconfig.json alias
    wins over a root tsconfig.json; a tsconfig.json beside it is read first; a `.js` spec's alias
    is unmapped and widens without allowJs and maps with it; `jsconfig*.json` is a resolution input),
    failing on base `780f9e970`.
  - Rollback: restore the `tsconfig.json`-only test in `readTypeScriptAliases` and the declaring
    loop, drop the allowJs condition in `resolveTypeScriptAliases` and `javaScriptConfigName` from
    `observedName`; no persisted format or receipt changes.

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
`test`, sorted by project then path), and optionally `unverifiedUnits`, a non-empty sorted subset
of `units` whose listing omission a project grep explains only unverifiably (`TJAA-V0-021`,
accepted in decision 0519). It describes the complete unfiltered configured listing,
including dependencies and teardowns, not case execution counts. It is caller-declared discovery,
not authenticated execution attestation. Corvint never executes Playwright or config to produce it;
`corvint affected discovery` (`TJAA-V0-019`, accepted, decision 0483) only converts a caller-run
JSON listing.
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
custom loaders and a package or out-of-root `extends` remain conservative frontiers (`TJAA-V0-114`
follows a relative chain inside the root). Source digests include tsconfig inputs.

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
`require`, an ambiguous slash the scan cannot classify (`TJAA-V0-039`), unresolved relative imports, undeclared/path-aliased bare imports, ambiguous or unknown
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

## Advisory fixture narrowing boundary (V1-2042)

Intent: GitHub #1294 reports that a change to a shared test-support module produced zero selected
units and `FULL_RELEVANT_SUITE`, because unrelated per-project config made discovery
`STATIC_UNRESOLVED`. `TJAA-V0-030..033` keep that fallback. They add, beside it, the statically
importing tests as an advisory candidate set, the projects that must still run in full, and the
changed symbols. The owner's standing auto-accept of lane requirements (2026-10-11) covers these
requirements; see the 2026-10-20 V1-2042 build-log entry.

Failure modes, each one fail-closed:

- Any unattributed config unknown, unexpected graph frontier, non-test-side or unowned dirty path,
  dirty config, or undeclared discovery project gives `NOT_NARROWED` with a reason. The fallback is
  unchanged.
- One project's unresolved membership gives `UNRESOLVED` for that project only. It is listed with
  its `argv` to run in full, and it is never silently dropped.
- An unreadable base, an unparsable module, an oversize file, JSX or too many paths gives
  `UNRESOLVED` symbols. It never gives an additive claim.

Non-goals and named gaps:

- Only the opt-in `playwright-affected/0` profile is covered. `affected-plan/0`, other runners and
  other languages get no `narrowed` member.
- Runtime loading outside JavaScript module syntax is not modelled. This includes bundler globs and
  `import.meta.glob`, HTML script tags, file reads, worker-shared state, and a test-support module
  that changes application state through a server.
- An observer frontier other than a dynamic import keeps the whole narrowing `NOT_NARROWED` only
  when it is unsited or sited outside the graph (`TJAA-V0-762`).
- A multi-binding variable declaration is keyed per bound name (`TJAA-V0-761`); a destructuring
  declarator binds no name there.
- The narrowed set has no live recall measurement. Its recall stays `NOT_RUN`, and callers that run
  only the candidates plus the `UNRESOLVED` projects accept that risk themselves.

V1-2060 amendments (experimental under the owner's standing lane auto-accept (2026-10-11)):

- `TJAA-V0-761`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-2060)
  Amending `TJAA-V0-033`: an exported `const`, `let` or `var` statement that binds more than one
  plain name MUST key one symbol per bound name, each by its own declarator text split at commas
  outside brackets, strings and templates, so a change to one initializer modifies only that name
  and an added declarator adds only its name. A statement binding one name, or a destructuring
  declarator, is keyed as before.
- `TJAA-V0-762`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-2060)
  Amending `TJAA-V0-031`: a frontier reason other than a dynamic import (in the observed frontier
  or the shared plan's `LANGUAGE_FRONTIER` unknowns) whose every recorded site is a file of the
  TypeScript graph MUST NOT refuse the narrowing; instead the tests whose static import closure
  reaches one of those files are added as candidates with witness `FRONTIER_SITE_REACH`, as a
  dynamic import's are. A reason with no site, or with a site outside the graph, still gives
  `NOT_NARROWED GRAPH_FRONTIER`. The narrowing stays advisory: a hidden edge the frontier stands for
  can only reach a test through its site's file, but its recall stays `NOT_RUN`.
- `TJAA-V0-763`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-2060)
  Decision on further profiles and runners, each deferred with its reason: `affected-plan/0` gets
  no `narrowed` member, because its fallback is the plan's own `UNKNOWN` scope over every plugin,
  and a TypeScript-only candidate set beside a multi-language plan would read as a selection of the
  whole plan; Vitest, Jest and the Node test runner get none, because their plans have no
  Playwright-style project fallback whose full-suite runs narrowing could shorten; other languages
  get none, because each lacks a test-side/source-side split like `TJAA-V0-031`'s. Runtime loading
  outside JavaScript module syntax (bundler globs and `import.meta.glob`, HTML script tags, file
  reads, worker-shared state, server-side state) stays unmodelled and keeps refusing through the
  existing frontiers and the not-test-side rule; no class of it is modelled by this amendment.

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
The GitHub #790 extension (`TJAA-V0-021`, accepted in decision 0519) reverts independently: refuse every
listing omission again, treat a dynamic project grep as unresolved membership, and drop the optional
`unverifiedUnits` member and the `playwright:discovery-omission-unverified` unknown; receipts that
never carried the member are unchanged.
The GitHub #1294 extension (`TJAA-V0-030..033`) reverts independently. Revert it by removing the
optional `narrowed` plan member, the per-project unknown attribution and the comparison-base read.
The fallback, selection and every bounded plan are unchanged, and no store or receipt is written.
The GitHub #1593/#1594 extension (`TJAA-V0-038..045`) reverts independently: restore the raw-text
ambiguous-slash scan that raised `typescript:dynamic-import`, the fixed
`shared TypeScript graph frontier` detail, the any-occurrence global-name and process-write
refusals and the fixed identity detail. Only unknown details and the new reason code change; no
persisted format, store or receipt is written.
The GitHub #1604/#1605 extension (`TJAA-V0-046..053`) reverts independently: restore the bare
general-adapter frontier reasons and token-free Playwright details, the dependency and any-text
`tsconfig` alias gate, the ambiguous `)` slash and the process-root refusal of `process.pid`. The
general graph digest changes with the located frontier entries; no persisted store is written.
The GitHub #1604 re-test and #1625 extension (`TJAA-V0-054..060`) reverts independently: refuse
the identity again for a non-identity value that is not side-effect free and drop
`playwright:use-value-unverified` and the confined-mutation devices root, refuse `Number.isInteger`
and the `Number.parseInt` member name, skip tsconfig resolution under a blocked gate, and render
sites in plain sorted order. Only graph
edges, frontier entries and unknown details change; no persisted store or receipt is written.
The GitHub #1701 extension (`TJAA-V0-090`, `TJAA-V0-091`) reverts independently: drop
`playwrightPlainStringify` so every `JSON` token stops the tag closure again, and restore the
`testMatch` marker refusal and the any-matcher project refusal in `playwrightStaticMembership`.
Only the omission verdict and executable-config frontier entries change; no persisted store or
receipt is written.
The GitHub #1717 extension (`TJAA-V0-092`, `TJAA-V0-093`) reverts independently. To revert it,
refuse every globstar in `playwrightRootFreeGlob`, name the first `testMatch` line again, drop
`playwrightLiteralOwned` from `classify`, and drop the fresh-array exemption
(`playwrightFreshArrayAt`, and `playwrightMethodDefinition` over `playwrightFreshMethods`) from `playwrightTagTextLiteral`.
Only test classification, executable-config frontier entries and the omission verdict change;
no persisted store or receipt is written.
The GitHub #1737 extension (`TJAA-V0-094`, `TJAA-V0-095`) reverts independently. To revert it,
restore the wholesale project-`testIgnore` refusal in `playwrightStaticMembership` and drop the
`testIgnore` checks there. Then drop the fresh-object write exemption from
`playwrightTagTextLiteral`: `playwrightFreshObjectWriteAt`, `playwrightFreshObject`, the
`written` set, and the `this`, `super` and `fulfill` stops. Only executable-config frontier
entries and the omission verdict change; no persisted store or receipt is written.
The GitHub #1737 item 2 proof (`TJAA-V0-096` to `TJAA-V0-099`) reverts on its own to the
`TJAA-V0-095` rule. To revert it, make `playwrightFreshObject` refuse every occurrence other than
the `TJAA-V0-095` uses: drop the compound-write (`playwrightStatementStart`), consumed-read
(`playwrightConsumedRead`, `playwrightReadEnds`) and stringify (`playwrightStringifyArgument`)
cases, and accept only `const`. The accessor stop (`playwrightAccessorDefinition`) and the
non-ASCII guard (`playwrightNonASCIINeighbour`) only narrow the rule and can stay. The scoped binding lookup (`playwrightFreshDeclarationFor`,
`playwrightBlockDeclaration`), the `with` stop and the destructuring guard only narrow what the
rule admits, so they can stay. Only the omission verdict changes; no persisted store or receipt
is written.
The GitHub #1881 extension (`TJAA-V0-116`, `TJAA-V0-117`) reverts independently. To revert it,
drop the type-member-key case (`playwrightTypeMemberKey`) and the `Object.entries` case
(`playwrightEntriesArgument`, `playwrightFreshObjectWithoutSpread`) from `playwrightFreshObject`,
and the `enumerated` exemption (`playwrightEntriesCall`) from `playwrightTagTextLiteral`. Only the
omission verdict changes; no persisted store or receipt is written.
The ungrepped listing (`TJAA-V0-118..120`) reverts independently: drop the
`--playwright-ungrepped-list` option and `PlaywrightDiscoveryFromLists`, the ungrepped argument of
`observePlaywrightListMembership` and `checkPlaywrightListMembership`, and
`playwright_ungrepped_list.go`. Receipts it produced stay valid; without it their verified
omissions would be unverified units again.

The GitHub #1750 extension (`TJAA-V0-100`) reverts independently. To revert it, remove
`internal/liveverify/affected/ignored.go`, its walk checks in `walkSourceFilesContext` and
`Source.HeldIgnored`, and the `excluded` argument of `resolveImport`. Then remove
`holdPlaywrightIgnored` and `playwrightIgnoredDrift` from `cmd/corvint/affected.go`. Only
which files the Playwright plan and discovery walks read changes; no persisted store or receipt is
written.

The GitHub #1751 extension (`TJAA-V0-101`) reverts independently. To revert it, delete
`playwright_declaration.go`, the `Declaration` members of `PlaywrightPlan` and
`PlaywrightExclusion`, and their use in `selectPlaywrightStaticDraft`. Then drop
`ReadDocumentationDeclaration` and `DeclaredDocumentation` from `documentation.go`. The plan
loses the `declaration` record and the contradiction refusal. No persisted store is written.

The GitHub #1751 generic extension (`TJAA-V0-102`) reverts independently. To revert it, delete
`path_tokens.go` and drop the `PathTokens` and `PathTokensBounded` assignments and the
`observation.body` assignment from `typescript.go`. TypeScript units then record no path tokens
again. Only the plan's selection and graph digest change; no persisted store is written, and a
graph cache entry is keyed by the program identity, so a reverted binary never reads one.

The GitHub #1737 extensions (`TJAA-V0-103`, `TJAA-V0-104`) revert independently. To revert
`TJAA-V0-103`, drop the `}`-receiver stop from `playwrightTagTextLiteral`'s `[` case. To revert
`TJAA-V0-104`, drop `playwrightUnknownAt`, `playwrightKeyAt`, `playwrightKeyOffsets`,
`playwrightProjectEntries` and `playwrightProjectKeyAt` and restore the config-level sites in
`playwrightStaticMembership`. Only executable-config frontier text and the omission
verdict change; no persisted store or receipt is written.

The GitHub #1877-#1880 extensions (`TJAA-V0-106..115`) revert together. Delete
`playwright_frontier_scope.go` and `path_declarations.go`, drop the `informational` parameter and
the out-of-suite call from `selectPlaywrightStaticDraft`, `draftPlaywrightNarrowing` and
`graphFrontierUnresolved`, restore the `TJAA-V0-013` exclusion, the discovery exemption, the
lockfile and declaration checks in `observe`/`ambiguousTestAsset`, `parseTypeScriptAliases` without
`extends`, and `playwrightRelativeDirectory` for `testDir`. No persisted store is written; the new
plan fields are omitted when empty, and the declarations are inert files to an older binary.

The GitHub #1883-#1887 extensions (`TJAA-V0-121..125`) revert independently. `TJAA-V0-121`:
drop the unowned branch and `playwrightUnownedMayLoad`/`playwrightUnderTestDir` from
`playwrightOutOfSuiteSources` and the `appliedUnowned` argument of `graphFrontierUnresolved` and
`graphFrontierDetail`; an unowned path then widens only through the dynamic-source unknown again.
`TJAA-V0-122`: drop `installedPackage` from `unresolvedBareImport` and delete
`Source.ResolvesToDependency`. `TJAA-V0-123`: restore `metadata` to the top-level refusal list in
`parsePlaywrightConfig`. `TJAA-V0-124`/`125`: delete
`playwright_ungrepped_wrapper.go`, `PlaywrightDiscoveryFromWrappedLists`, the
`--playwright-ungrepped-config` option, `refuseTrackedPlaywrightWrapper` and the extra held path.
No persisted store or receipt format changes; receipts produced through a wrapper stay valid.

The V1-2812 per-test closures (`TJAA-V0-140..142`) revert together: delete
`playwright_test_closure.go`, the `seeds` field and its condition in `playwrightClosure.members`,
and inline `preparePlaywrightGraph` back into `selectPlaywrightStaticDraft`. Nothing is
persisted; run-identity documents carrying `corvint-test-closure/0` digests are then dropped by
`PWR-V0-074`'s rollback.

## Traceability

| Requirement | Implementation / evidence | Status |
|---|---|---|
| `TJAA-V0-001..004`, `TJAA-V0-006..008` | `internal/liveverify/affected/typescript/` focused tests | experimental |
| `TJAA-V0-005` | `TestTemplateSubstitutionRequireBuildsDependencyEdge`, `TestMultilineJSXQuoteAmbiguityRaisesFrontier`, `TestSameLineJSXApostropheAmbiguityRaisesFrontier`, `TestStandaloneJSXApostrophesRaiseFrontier`, `TestJSXTextCannotImitateALiteralOpeningContext`, `TestKeywordEndingJSXTextCannotHideRequireWithoutBraces`, `TestJSXAttributeAndExpressionStringsRemainParsed`, `TestOrdinaryTSXStringsAndJSXExpressionLiteralsRemainParsed`, `TestWorkspacePackageImportReachesTheImportingTest_V1_0283`, and import-resolution focused tests in `internal/liveverify/affected/typescript/` | experimental |
| `TJAA-V0-009` | `internal/liveverify/affected/conformance_test.go` TypeScript seam case | experimental |
| `TJAA-V0-010..017` | `internal/liveverify/affected/typescript/playwright.go`, `playwright_test.go`, and `cmd/corvint/affected_playwright_test.go` | experimental |
| `TJAA-V0-013` bounded helper exclusion (V1-0211) | `TestPlaywrightUnreachedHelperBoundedExclusion_V1_0211` in `internal/liveverify/affected/typescript/playwright_test.go` | accepted, decision 0514; synthetic matched discovery |
| `TJAA-V0-014..017` fixture qualification | `internal/liveverify/affected/typescript/playwright_qualification_test.go`, `testdata/playwright-qualification.tsv` | synthetic fixture evidence; runtime promotion excluded |
| `TJAA-V0-012..017` example-app shape | `TestPlaywrightExampleAppQualification`, `TestPlaywrightGlobalUseInheritance`, `TestPlaywrightAliasResolutionBoundaries` in `internal/liveverify/affected/typescript/playwright_example_app_test.go` | synthetic global-use, alias and hook closure; exact consumer `NOT_OBSERVED` |
| `TJAA-V0-018` (accepted, decision 0483) | `TestPlaywrightDeviceSpreadBesideRuntimeUseValues_V1_1065`, `TestPlaywrightUseValueSideEffects_V1_1065`, `TestPlaywrightUseValueUnicodeLineTerminator_V1_1065`, `TestPlaywrightStaticValueNestingIsBounded_V1_1065`, `TestPlaywrightUseValueNestedTemplate_V1_1065`, `TestPlaywrightMemberReadRoots_GH709Round8` in `playwright_example_app_test.go`; `TestPlaywrightSlashStartsRegex` in `playwright_test.go`; `TestUnicodeLineTerminatorsEndCommentsAndLines`, `TestNestedTemplateLiteralsKeepContent` in `typescript_test.go`; `TestPlaywrightScannersSkipNestedTemplates` in `playwright_test.go`; `TestPlaywrightPureExpression`, `TestPlaywrightPureExpressionScopedMemberReads` in `playwright_pure_test.go`; `TestQualifiedReporterIdentityKeysMatchStaticProfile` in `internal/jstestprovider/identity_keys_test.go` | experimental |
| `TJAA-V0-019` (accepted, decision 0483) | `TestPlaywrightDiscoveryFromListMultiProject_V1_1066`, `TestPlaywrightDiscoveryFromListRefusals_V1_1066`, `TestPlaywrightDiscoveryFromListMembership_V1_1066` in `playwright_discovery_list_test.go`; `TestPlaywrightStringGlobsArePrefixedAndCaseInsensitive`, `TestPlaywrightComputedStringsAndUnsupportedGlobsWiden`, `TestPlaywrightLineTerminatorPathWidensMembership`, `TestPlaywrightRegexBodyAllowlist`, `TestPlaywrightGlobAgreesWithBundledMinimatch` in `playwright_test.go` over a real Playwright 1.61.1 `--list --reporter=json` report (`testdata/playwright-list/multi-project.json`); `TestAffectedPlaywrightDiscoveryProducer_GH709` in `cmd/corvint/affected_playwright_test.go`; `TestAffectedPlaywrightDiscoveryStaleListing_GH709`, `TestAffectedPlaywrightDiscoveryHeadDriftAfterSources_GH709` in `cmd/corvint/affected_playwright_discovery_test.go` | experimental; one real listing shape |
| `TJAA-V0-020` (accepted, decision 0483) | `TestPlaywrightDiscoveryMalformedReason_V1_1067` in `playwright_discovery_test.go`; `TestAffectedPlaywrightDiscoveryProducer_GH709` | experimental |
| `TJAA-V0-021` (accepted, decision 0519; V1-1175) | `TestPlaywrightDiscoveryGrepOmission_TJAA_V0_021` (GitHub #790 three-project repro, unexplained and no-grep refusals, dynamic and non-partwise grep, title-derived and explicit tags, `unverifiedUnits` validation), `TestPlaywrightRegexBodyPartwise_TJAA_V0_021`, `TestPlaywrightTitleTags_TJAA_V0_021` in `playwright_grep_omission_test.go`; grep-title construction checked against Playwright 1.63 source, not a live run | experimental; hand-built listings, `NOT_RUN` live |
| `TJAA-V0-030..032` (V1-2042) | `TestPlaywrightNarrowingSharedFixture_TJAA_V0_030` (GitHub #1294 repro, determinism), `TestPlaywrightNarrowingAbsentFromBoundedPlan_TJAA_V0_030`, `TestPlaywrightNarrowingRefusals_TJAA_V0_031` (application, unowned, dirty-config and global-config refusals), `TestPlaywrightNarrowingPerProject_TJAA_V0_032` (unknown dependency, dynamic-import taint, application dynamic import, discovery matched, differing and undeclared project) in `playwright_narrowing_test.go`; `TestAffectedPlaywrightNarrowedSymbols_TJAA_V0_033` in `cmd/corvint/affected_playwright_test.go` | experimental; synthetic fixtures, live recall `NOT_RUN` |
| `TJAA-V0-761..763` (V1-2060) | `TestMultiBindingExportKeysEachName_TJAA_V0_761`, `TestPlaywrightNarrowingFrontierTaintsOnlyReachingTests_TJAA_V0_762`; `TJAA-V0-763` is a decision with no test |
| `TJAA-V0-033` (V1-2042) | `TestChangedSymbols_TJAA_V0_033`, `TestChangedSymbolsUnresolved_TJAA_V0_033`, `TestPlaywrightNarrowingReportsSymbols_TJAA_V0_033` in `playwright_symbols_test.go`; `TestAffectedPlaywrightNarrowedSymbols_TJAA_V0_033` (HEAD and `--base` comparison) | experimental; syntactic only |
| `TJAA-V0-034` (V1-2455; GitHub #1575) | `TestScanImportsSeparatesImportTypesAndLiteralTextFromDynamicImports` in `typescript_test.go` | experimental |
| `TJAA-V0-035` (V1-2456; GitHub #1576) | `TestPlaywrightEnvUseValuesKeepIdentity_GH1576`, `TestPlaywrightComputedExecutablePathIsExecutionUnknown_GH1576` in `playwright_gh1576_test.go`; `TestPlaywrightUseValueSideEffects_V1_1065`, `TestPlaywrightMemberReadRoots_GH709Round8` in `playwright_example_app_test.go` | experimental |
| `TJAA-V0-036` (V1-2457; GitHub #1577) | `TestPlaywrightConstDependenciesResolveToBranchUnion_GH1577` in `playwright_gh1576_test.go` | experimental |
| `TJAA-V0-037` (V1-2458; GitHub #1578) | `TestGeneralAdapterPlaywrightMembershipAndAliases_GH1578` in `playwright_gh1578_test.go`; `TestPlaywrightExampleAppQualification`, `TestUnitsDetectsEverySupportedRunnerFromImportsOrConfiguration` | experimental; synthetic fixtures |
| `TJAA-V0-038` (V1-2467; GitHub #1593) | `TestRegexLiteralAfterOpenerIsNotDivision_GH1593`, `TestPlaywrightRegexReplaceKeepsSelectionAndNamesFrontierSites_GH1593` in `typescript_gh1593_test.go`; `TestScanImportsSeparatesImportTypesAndLiteralTextFromDynamicImports` in `typescript_test.go` | experimental |
| `TJAA-V0-039` (V1-2467; GitHub #1593) | `TestAmbiguousSlashIsScanUnresolved_GH1593` in `typescript_gh1593_test.go`; `TestScanImportsSeparatesImportTypesAndLiteralTextFromDynamicImports` in `typescript_test.go` | experimental |
| `TJAA-V0-040` (V1-2467; GitHub #1593) | `TestFrontierSitesAreLinear_PR1596`, `TestFrontierSiteIsTheParsedSpecifier_PR1596` in `typescript_pr1596_test.go`; `TestFrontierSitesNameEveryReason_GH1593`, `TestPlaywrightRegexReplaceKeepsSelectionAndNamesFrontierSites_GH1593`, `TestAmbiguousSlashIsScanUnresolved_GH1593` in `typescript_gh1593_test.go` | experimental |
| `TJAA-V0-041` (V1-2468; GitHub #1594) | `TestPlaywrightReflectiveAccessRefusesEnvAndNumericRoots_PR1596`, `TestPlaywrightGlobalNamesInLiteralsKeepEnvReads_GH1594`, `TestPlaywrightEnvIdentityIssueConfig_GH1594` in `playwright_gh1594_test.go` | experimental |
| `TJAA-V0-042` (V1-2468; GitHub #1594) | `TestPlaywrightEnvWritesOutsideIdentity_GH1594`, `TestPlaywrightEnvIdentityIssueConfig_GH1594` in `playwright_gh1594_test.go`; `TestPlaywrightEnvUseValuesKeepIdentity_GH1576` in `playwright_gh1576_test.go` | experimental |
| `TJAA-V0-043` (V1-2468; GitHub #1594) | `TestPlaywrightEnvDefaultForms_GH1594`, `TestPlaywrightEnvIdentityIssueConfig_GH1594` in `playwright_gh1594_test.go` | experimental |
| `TJAA-V0-044` (V1-2468; GitHub #1594) | `TestPlaywrightBrowserIdentityNamesBlockingToken_GH1594` in `playwright_gh1594_test.go` | experimental |
| `TJAA-V0-045` (V1-2467; PR #1596 review) | `TestUnsupportedSlashPredecessorPreservesUncertainty_PR1596` in `typescript_pr1596_test.go` | experimental |
| `TJAA-V0-046` (GitHub #1605) | `TestDivisionAfterCallBeforeLineComment_GH1605` in `playwright_gh1605_test.go` | experimental |
| `TJAA-V0-047..049` (GitHub #1605) | `TestPlaywrightIdentityIgnoresUnrelatedFieldsAndWrites_GH1605` in `playwright_gh1605_test.go`; `TestPlaywrightEnvWritesOutsideIdentity_GH1594`, `TestPlaywrightEnvDefaultForms_GH1594` in `playwright_gh1594_test.go` | experimental |
| `TJAA-V0-050` (GitHub #1604) | `TestTSConfigPathAliasesSelectSpecs_GH1604`, `TestFrontierSitesNameFileLineAndToken_GH1604`, `TestPlaywrightConfigTSConfigToken_GH1604` in `typescript_gh1604_test.go`; `TestGeneralAdapterPlaywrightMembershipAndAliases_GH1578` in `playwright_gh1578_test.go` | experimental; synthetic fixtures |
| `TJAA-V0-051` (GitHub #1604) | `TestTSConfigPathAliasesSelectSpecs_GH1604`, `TestTSConfigPathAliasMissingTargetStaysFrontier_GH1604` in `typescript_gh1604_test.go` | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-052` (GitHub #1604) | `TestFrontierSitesNameFileLineAndToken_GH1604`, `TestFrontierSitesAreBoundedPerReason_GH1604` in `typescript_gh1604_test.go`; `TestPlaywrightRegexReplaceKeepsSelectionAndNamesFrontierSites_GH1593` in `typescript_gh1593_test.go`; `TestLocatedTypeScriptFrontierKeepsE2EAnswer_TJAAV0052` in `internal/appflows/selection_frontier_test.go` | experimental |
| `TJAA-V0-053` (GitHub #1604) | `TestFrontierSitesNameFileLineAndToken_GH1604` in `typescript_gh1604_test.go` | experimental |
| `TJAA-V0-054` (GitHub #1625; PR #1628 review) | `TestPlaywrightNonIdentityFieldsNeverBlockIdentity_GH1625` in `playwright_gh1625_test.go`; `TestPlaywrightSideEffectUseValueKeepsSelectionUnknown_PR1628` in `typescript_pr1628_test.go`; `TestPlaywrightBrowserIdentityNamesBlockingToken_GH1594` in `playwright_gh1594_test.go`; `TestPlaywrightUseValueSideEffects_V1_1065`, `TestPlaywrightMemberReadRoots_GH709Round8` in `playwright_example_app_test.go` | experimental |
| `TJAA-V0-055` (GitHub #1625) | `TestPlaywrightNumberMemberCallsAreAdmitted_GH1625` in `playwright_gh1625_test.go` (the issue's `use` lines and refusal message) | experimental |
| `TJAA-V0-056` (GitHub #1604 re-test) | `TestTSConfigRetestTreeResolvesAsTsc_GH1604` in `typescript_gh1604_retest_test.go` (the comment's tsconfig, both spellings, and the four example imports) | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-057` (GitHub #1604 re-test) | `TestBlockedAliasGateSelectsAndNamesBlocker_GH1604` in `typescript_gh1604_retest_test.go`; `TestFrontierSitesNameFileLineAndToken_GH1604` in `typescript_gh1604_test.go` | experimental; synthetic blockers |
| `TJAA-V0-058` (GitHub #1604 re-test) | `TestPinnedFrontierSitesPrecedeTheBound_GH1604`, `TestBlockedAliasGateSelectsAndNamesBlocker_GH1604` in `typescript_gh1604_retest_test.go` (entry order, not only presence) | experimental |
| `TJAA-V0-059` (PR #1628 review) | `TestBlockedGateKeepsWorkspaceLinks_PR1628` in `typescript_pr1628_test.go` | experimental; synthetic workspace |
| `TJAA-V0-060` (PR #1628 review) | `TestPlaywrightConfinedMutationKeepsDeviceIdentity_PR1628` in `typescript_pr1628_test.go` | experimental |
| `TJAA-V0-090` (GitHub #1701) | `TestPlaywrightTagClosurePlainStringify_GH1701` in `playwright_gh1701_test.go` (the issue's helper shape); `TestPlaywrightTagClosureDetailsWithoutTag_GH1688` in `playwright_gh1688_test.go` | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-091` (GitHub #1701) | `TestPlaywrightPathModeLiteralTestMatch_GH1701`, `TestPlaywrightPathModeLiteralTestMatchOutsideMatcherModel_GH1701`, `TestPlaywrightPathModeLiteralTestMatchOwnsSupport_GH1701` in `playwright_gh1701_test.go`; `TestPlaywrightConfigFrontierNamesBlocker_GH1688` in `playwright_gh1688_test.go` | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-092` (GitHub #1717) | `TestPlaywrightPathModeGlobstarTestMatch_GH1717` and `TestPlaywrightGlobstarReachesRoot_GH1717` in `playwright_gh1717_test.go` (the issue's project shape); `TestPlaywrightPathModeLiteralTestMatch_GH1701` in `playwright_gh1701_test.go` | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-093` (GitHub #1717) | `TestPlaywrightTagClosureFreshArrayIndex_GH1717` in `playwright_gh1717_test.go` (the issue's helper line) | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-094` (GitHub #1737) | `TestPlaywrightPathModeLiteralTestIgnore_GH1737` in `playwright_gh1737_test.go` | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-095` (GitHub #1737) | `TestPlaywrightTagClosureFreshObjectWrite_GH1737` in `playwright_gh1737_test.go` (the issue's helper shape) | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-096` (GitHub #1737 item 2) | `TestPlaywrightTagClosureFreshObjectProof_GH1737` rows `TJAA-V0-096 *` in `playwright_gh1737_test.go` (with the `let` reassignment row of `TestPlaywrightTagClosureFreshObjectWrite_GH1737`) | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-097` (GitHub #1737 item 2) | `TestPlaywrightTagClosureFreshObjectProof_GH1737` rows `TJAA-V0-097 *` in `playwright_gh1737_test.go` (with its compound-write row) | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-098` (GitHub #1737 item 2) | `TestPlaywrightTagClosureFreshObjectProof_GH1737` rows `TJAA-V0-098 *` in `playwright_gh1737_test.go` | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-099` (GitHub #1737 item 2) | `TestPlaywrightTagClosureFreshObjectProof_GH1737` rows `TJAA-V0-099 *` in `playwright_gh1737_test.go` | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-100` (GitHub #1750) | `TestAffectedPlaywrightIgnoredReportNarrows_GH1750` in `cmd/corvint/affected_playwright_ignored_gh1750_test.go` (the issue's report shape, plus the tracked-but-ignored and unignored controls); `TestHoldIgnoredExclusionSkipsWalk_TJAA_V0_100` in `internal/liveverify/affected/ignored_test.go`; `TestIgnoredCandidateStaysUnresolved_TJAA_V0_100` in `ignored_resolution_tjaa100_test.go` | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-101` (GitHub #1751) | `TestAffectedPlaywrightPathDeclaration_GH1751` in `cmd/corvint/affected_playwright_path_owners_gh1751_test.go` (no-declaration control, recorded digest and matched path, named exclusion, seven refused contradictions including an escaped literal); `TestDeclaredPathNamed_TJAA_V0_101` and `TestDeclaredPathNamedDecodesEscapes_TJAA_V0_101` in `playwright_declaration_tjaa101_test.go` | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-102` (GitHub #1751) | `TestGenericPlanSelectsTypeScriptDocumentationReader_GH1751` (quoted, template, escaped and file-name readers selected as `PATH_LITERAL_READER`; comment-only and other-path controls excluded), `TestTypeScriptPathTokensBoundRefusesWithNamedUnknown_GH1751` and `TestTypeScriptPathTokensReadLiteralContentsOnly_TJAA102` in `path_tokens_tjaa102_test.go`; the `route spelling selects the reader` case of `TestAffectedPlaywrightPathDeclaration_GH1751` | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-103` (GitHub #1737) | `TestPlaywrightTagClosureObjectLiteralReceiver_GH1737` in `playwright_gh1737_test.go` (read, write and block-then-array stops; literal-index and array-type controls) | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-104` (GitHub #1737) | `TestPlaywrightPathModeConfigSitesNameLines_GH1737` and `TestPlaywrightUnknownAtRecordsWhyNoLine_TJAA104` in `playwright_frontier_lines_tjaa104_test.go`; the escaped-key and owns rows of `playwright_gh1701_test.go` | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-105` (GitHub #1737 residual) | `TestPlaywrightPathModeProjectsTestIgnore_GH1737` in `playwright_gh1737_projects_test.go` (the reporter literal and a literal array repeated in three `projects[]` entries raise no frontier; computed, regular-expression and trailing-globstar values name line 5); the amended parent-directory row of `TestPlaywrightPathModeLiteralTestIgnore_GH1737` | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-106`, `TJAA-V0-107` (GitHub #1877) | `TestPlaywrightOutOfSuiteSourceWidens_GH1877`, `TestPlaywrightUnreachedDeclarationExcludesAndCites_GH1877`, `TestPlaywrightUnreachedDeclarationRefusals_GH1877` in `playwright_gh1877_test.go`; amended `TestPlaywrightUnreachedHelperBoundedExclusion_V1_0211` and the `TJAA-V0-012` discovery row | experimental |
| `TJAA-V0-108..110` (GitHub #1878) | `TestPlaywrightFrontierOutsideClosureIsInformational_GH1878`, `TestPlaywrightOverBoundSourceScopedToClosure_GH1878`, `TestPlaywrightConfigFrontierScopedToAncestors_GH1878` in `playwright_gh1878_test.go`; the closure and non-closure dynamic-import rows of `playwright_narrowing_test.go`; amended `TestAffectedPlaywrightIgnoredReportNarrows_GH1750` and `TestAffectedGeneratedPathsDeclaration_GH1811` in `cmd/corvint` | experimental |
| `TJAA-V0-111`, `TJAA-V0-112` (GitHub #1879) | `TestLockfilesAndDeclaredPathsAreNotTestAssets_GH1879`, `TestPlaywrightTestAssetFrontierScoped_GH1879` in `typescript_gh1879_test.go` | experimental |
| `TJAA-V0-113..115` (GitHub #1880) | `TestTypeScriptAliasesAcceptInsideRootParents_GH1880`, `TestTypeScriptAliasesFollowExtendsChain_GH1880`, `TestPlaywrightTestDirInsideRootParent_GH1880`, `TestPlaywrightSelectsThroughInheritedAlias_GH1880`, `TestPlaywrightSelectsThroughObservedBaseConfig_GH1880`, `TestInheritedAliasUnderSecondRunnerStaysFrontier_GH1880` in `playwright_aliases_gh1880_test.go`; amended `TestFrontierSitesNameFileLineAndToken_GH1604` | experimental |
| `TJAA-V0-116` (GitHub #1881 item 1) | `TestPlaywrightTagClosureFreshObjectTypeKeyAndEntries_GH1881` rows `TJAA-V0-116 *` in `playwright_gh1881_test.go` (the issue's handler, `;` and `,` type keys; optional parameter, conditional and optional-chain stops) | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-117` (GitHub #1881 item 2) | `TestPlaywrightTagClosureFreshObjectTypeKeyAndEntries_GH1881` rows `TJAA-V0-117 *` in `playwright_gh1881_test.go` (admitted walk and entries-only object; spread, nested spread, parameter, extra argument, `values`, escape, parenthesized, optional-call, shadowing and `this` stops) | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-118..120` (GitHub #1881 item 3) | `TestPlaywrightDiscoveryUngreppedListing_GH1881` in `playwright_gh1881_list_test.go` (agreeing listing verifies; prediction mismatch, missing pair, missing discovery case, repeated key, dynamic grep and no-grep-project mismatch stay unverified; an otherwise refused omission is verified; foreign, filtered and malformed ungrepped listings are refused); `TestAffectedPlaywrightDiscoveryProducer_GH709` ungrepped rows in `cmd/corvint/affected_playwright_test.go` | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-121` (GitHub #1884) | `TestPlaywrightOutOfSuiteSourceAnyLanguage_GH1884` in `playwright_gh1884_test.go` (undeclared TypeScript and Ruby paths widen; a declared Ruby directory applies; package.json, test-directory and spec-named declarations refuse) | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-122` (GitHub #1885) | `TestPlaywrightSymlinkedNodeModules_GH1885` and `TestInstalledPackageLinkIntoRepositorySource_GH1885` in `playwright_gh1885_test.go` (symlinked store installed: no alias frontier, `BOUNDED`; not installed: frontier stays; link into repository source or a tree source: not installed, frontier stays; in-repository pnpm store link: installed) | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-123` (GitHub #1887) | `TestPlaywrightDiscoveryTopLevelMetadata_GH1887` in `playwright_gh1887_test.go` (empty and run-identity `metadata` accepted; `config.sha256` differs for each) | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-124`, `TJAA-V0-125` (GitHub #1883) | `TestPlaywrightUngreppedWrapperConfig_GH1883` in `playwright_gh1883_test.go` (canonical and formatting-variant wrappers accepted; altered, foreign, other-directory, self, absent and unaccompanied wrappers refused); the wrapper rows of `TestAffectedPlaywrightDiscoveryProducer_GH709` in `cmd/corvint/affected_playwright_test.go` (held out of the digest, tracked wrapper and bad flags refused) | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-126` (GitHub #1751, V1-2667) | `TestResolvedImportSpecifierIsNotPathToken_TJAA126` and `TestPathTokensSkipOnlyTheResolvedSpecifier_TJAA126` in `gh1751_residuals_test.go` and `TestCreateRequireKeepsResolvedSpecifierToken_TJAA126` (a dirty `src/a/helper.js` selects its importer and a test reading the same text, not a test importing another `./helper.js`; only the literal at the specifier's offset is skipped; a `createRequire` module keeps its tokens) | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-140`, `TJAA-V0-141` (V1-2812) | `TestPlaywrightTestClosures_TJAA140` and `TestPlaywrightTestClosuresFailClosed_TJAA141` in `playwright_test_closure_tjaa140_test.go` (disjoint helpers excluded, a helper application code reaches kept; dynamic import in a test unknown for that test only; frontier outside the suite widens; unresolved config unknown; dependency project tests joined) | experimental; real Playwright run `NOT_RUN` |
| `TJAA-V0-142` (V1-2812) | `internal/liveverify/affected/typescript` package tests unchanged and passing after the extraction | experimental |
| `TJAA-V0-147` (V1-2914; GitHub #1920; `PWR-V0-183`) | `TestPlaywrightTestClosuresIdentityArgvEscape_PWRV0183` in `playwright_test_closure_gh1920_test.go` (identity argv escape admitted; other argv, computed key and plain const escapes still refuse) | experimental |
| `TJAA-V0-127` (V1-2584) | `TestPlaywrightPathModeRegexTestMatch_V12584` and `TestPlaywrightRegexReachesRoot_V12584` in `playwright_regex_root_test.go` (end-anchored, case-insensitive, alternation and mixed-array regexes resolve; unanchored, start-anchored, slash-matching, word-boundary, `u`-flag and lookahead regexes keep the site); the regex rows of `playwright_gh1688_test.go`, `playwright_gh1737_test.go` and `playwright_gh1737_projects_test.go` | experimental; reporter repository `NOT_OBSERVED` |
| `TJAA-V0-960` (V1-2533) | `TestGeneralAdapterJSConfigAliasesAndAllowJs_V12533` in `alias_jsconfig_v12533_test.go`; `aliasConfigFile`, `readTypeScriptAliases` and `resolveTypeScriptAliases` in `playwright_aliases.go`; `observedName` and the alias-config loop in `typescript.go` | experimental |
| independent real-repository recall | 2026-08-29 build-log evidence | observed |
| runtime/framework/OS qualification | `LPCV-V0-043..046` promotion matrix | `NOT_RUN` |
