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
- Status: proposed/experimental; TJAA-V0-019 accepted (decision 0483) with its V1-1175 amendment and TJAA-V0-021 accepted (decision 0519); TJAA-V0-030..033 (V1-2042) and TJAA-V0-034..037 (GitHub #1575-#1578) and TJAA-V0-038..045 (GitHub #1593, #1594) and TJAA-V0-046..053 (GitHub #1604, #1605) and TJAA-V0-054..060 (GitHub #1604 re-test, #1625, PR #1628 review) under the standing lane auto-accept
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
  `tsconfig` option (each stays a named frontier, as `TJAA-V0-012` requires); no change to the
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
- An observer frontier other than a dynamic import keeps the whole narrowing `NOT_NARROWED`.
- A multi-binding declaration is keyed by its first name.
- The narrowed set has no live recall measurement. Its recall stays `NOT_RUN`, and callers that run
  only the candidates plus the `UNRESOLVED` projects accept that risk themselves.

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
| independent real-repository recall | 2026-08-29 build-log evidence | observed |
| runtime/framework/OS qualification | `LPCV-V0-043..046` promotion matrix | `NOT_RUN` |
