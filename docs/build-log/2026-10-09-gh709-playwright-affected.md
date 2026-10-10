# GitHub #709 Playwright affected: device spreads, discovery producer, MALFORMED reason

Decision: three proposed requirements in
`docs/specs/typescript-javascript-affected-adapter-v0.md`, all pending owner acceptance.

- `TJAA-V0-018` (V1-1065): the static `playwright-affected/0` profile already resolved
  `use: { ...devices['<known name>'] }`, but any runtime-computed `use` value (a global
  `baseURL: process.env.BASE_URL ?? ...`, a project `storageState: authFile`) marked the browser
  identity of every project unresolved. Only the identity options must now be static. That set is
  `PlaywrightUseIdentityKeys`, the list the provider's qualified reporter (#49) already resolves at
  runtime. The #49 code runs in Node at runtime, so it shares no parser with the static profile.
  What is shared is the key set: `TestQualifiedReporterIdentityKeysMatchStaticProfile` checks it
  against the embedded `qualified-reporter.cjs` instead of keeping a second copy.
- `TJAA-V0-019` (V1-1066): `corvint affected discovery --playwright-config PATH --playwright-list
  FILE` converts a caller-run `playwright test --list --reporter=json` report into the canonical
  receipt (`typescript.PlaywrightDiscoveryFromList`). It refuses filtered, sharded, errored,
  foreign-config or out-of-root listings with `unsupported-playwright-discovery`.
- `TJAA-V0-020` (V1-1067): a MALFORMED summary now names `reason`
  (`DECODE_FAILED|NON_CANONICAL_BYTES|INVALID_FIELD`) and a one-line `detail`. An oversize regular
  discovery file now reports MALFORMED instead of MISSING.

Investigation of the issue's STATIC_UNRESOLVED result (142 units only in the receipt, 0 static): it
was a Corvint defect, the one fixed by `TJAA-V0-018`. When any project's identity is unresolved,
`selectPlaywrightStatic` empties the static units, so the whole receipt appears as `onlyInReceipt`.
The reporter's config is not available, so this cause is inferred. It was reproduced on base
`1120e565` with a generic config that has a computed global `baseURL` and per-project device spreads
beside `storageState`. Result: `state=STATIC_UNRESOLVED onlyInReceipt=8 onlyInStatic=0`, with
`browser-identity-unresolved` for all four projects.

A second, independent trap: Playwright reports `file` relative to `config.rootDir` (the top-level
`testDir`), not to the repository. A hand-built receipt that copies those names gets
`UNIVERSE_MISMATCH` with every unit on both sides. The producer rewrites them to repository-relative
paths, and the spec now states that `test` is repository-relative.

Evidence:
- Failing first, before each fix:
  - The V1-1065 test reported unresolved identity for all three device-spread cases.
  - The V1-1066 and V1-1067 tests failed to compile because the producer and the reason fields were
    absent.
  - The CLI test got `unrecognized arguments: discovery`.
- Pass after the fixes:
  - `go test ./internal/liveverify/affected/typescript ./internal/jstestprovider ./internal/appflows`
  - `go test -run 'Affected|Help|E2ESafe|Playwright' ./cmd/corvint`
- The multi-project fixture is a real Playwright 1.61.1 listing (`testdata/playwright-list/multi-project.json`,
  with the root rebased). It has a setup dependency, two device projects, and a project with its own
  `testDir`. Its produced receipt reconciles to MATCHED, and the bytes are identical across runs.

Limits:
- The device table still recognizes only Desktop Chrome, Firefox and Safari. Other descriptors stay
  unresolved.
- A produced receipt is still caller-declared. The producer cannot prove the listing came from the
  bytes it binds, or that environment-driven config branches matched.
- Only one real listing shape (Playwright 1.61.1) was exercised.
- Other unknowns in the issue, such as `dynamic-source-reachability` from unresolved bare imports,
  are unchanged.
- An absent or unreadable discovery file is still MISSING.
- `make gate` was not run.

## Review round 1

An independent review of the first three commits returned FAIL with four findings. The orchestrator
decided each fix, and each fix landed with a test that failed first:

- The producer stamped a stale listing with fresh HEAD and source bindings. It now enumerates the
  (project, file) pairs the config selects in the current sources through the static
  `testDir`/`testMatch`/`testIgnore` subset. It refuses with `unsupported-playwright-discovery` when
  the listing omits or adds a pair, or when that membership is not static. In the e2e-safe
  regression, a stale listing plus a newly committed spec no longer reaches MATCHED.
- A non-identity `use` value with side effects was ignored. The review's repro assigns
  `devices['Desktop Chrome'].browserName` inside `baseURL`. `TJAA-V0-018` now admits only
  syntactically side-effect-free values. There is no call allowlist, so `path.join(...)` and
  `headers()` widen again.
- `affected discovery` checked HEAD before its last source observation. It now re-reads HEAD after
  that observation, and an injected-drift test pins the ordering.
- The spec wrongly said quoted identity keys widen. A quoted literal key is the same key and resolves;
  only computed keys widen. A test pins the behaviour, which was already correct.

Limits added by this round:
- A file the config selects but whose tests all fail Playwright's runtime filters (for example, a file
  with zero tests) makes the producer refuse.
- A member read can invoke an existing getter; this is admitted.
- The `affected` plan path still reads HEAD before its source verification; it is unchanged.


## Review round 2

The second independent review returned FAIL with two findings. The orchestrator decided both fixes,
and each landed with a test that failed first:

- Membership enumeration used only the files the static profile parsed, so a new `.mts` or `.cts`
  spec dropped out and a stale listing was stamped. The producer now finds candidate test files by
  path alone, using every extension Playwright's default `testMatch` accepts. It refuses when the
  config selects a file the static profile does not parse, or when the listing omits a file. That
  includes a `.ts` spec that is not valid UTF-8. The e2e-safe stale-listing regression now covers
  `.ts`, `.mts` and `.cts`.
- The side-effect-free check ignored implicit coercion. In the review's repro, `${url}` calls a
  user `toString` that rewrites `devices['Desktop Chrome']`, and Chromium becomes Firefox. A
  recursive-descent checker replaces the tokenizer. Identifiers and member reads may appear only
  whole or under non-coercing operators. Template substitutions, computed keys and coercing
  operators admit only operands proven primitive: literals, templates, `process.env.NAME`, and
  operator results over those. A dynamic template such as `${port}` now widens.

Limits added by this round:
- A config that shadows `process` can make a `process.env.NAME` read non-primitive. This is not
  detected.
- A value admitted whole, such as `baseURL: url`, is not modelled for Playwright's own later use.
- Files with other extensions that a custom `testMatch` selects are not enumerated.

## Review round 3

The third independent review returned FAIL with six findings. The orchestrator decided each fix.
Each landed with a test that failed first:

- The number lexer swallowed `1..payload` as one token, so `1..payload + ''` passed as a primitive.
  That expression converts a `Number.prototype` getter's object through its `toString`. Numbers now
  lex to the ECMAScript grammar, and a dot after a complete literal starts a member read. A member
  read is never proven primitive. A legacy octal, a malformed literal (`0x`, `1_`, `1.5n`) or one
  followed by an identifier character (`1abc`, `1.toString`) is refused.
- `process.env.NAME` is no longer proven primitive. A getter, a Proxy or a replaced `process.env`
  can return an object. It is still admitted whole and under the non-coercing operators. The
  round-2 limit about a shadowed `process` no longer applies. TJAA-V0-018 now names the operands
  coercion admits: string and numeric literals, templates over those, `true`, `false`, `null`,
  the results of operators over those, and the always-primitive `typeof`, `void`, `!`, `===` and
  `!==`.
- Membership enumeration reused the shared source walker, which skips hidden directories and
  `build`, `dist`, `vendor`, `target` and the other skipped names. A spec such as
  `e2e/build/b.spec.ts` was therefore invisible to membership and to the source digest. The
  producer now searches each selected `testDir` itself, and refuses when the config selects a file
  inside such a directory, naming the directory. The shared walker is unchanged.
  - Deliberate narrowing of the decision ("refuse when any such directory exists"): the refusal
    needs a selected file inside the directory, not just the directory. Under the default
    `testDir` (the config directory), the repository's own `.git` would otherwise refuse every
    listing.
  - `node_modules` below a `testDir` is not searched. Playwright 1.60's `collectFiles` skips
    every directory with that name.
  - A `testDir` reached through a symbolic link is refused. The source walker does not follow it;
    Playwright does.
- `++` and `--` were read as two unary signs, so `++process.env.COUNTER` was admitted. They are now
  refused, both as prefixes and between operands.
- A listed file that is not valid UTF-8 passed membership but was absent from the source digest.
  Every selected file, listed or not, must now be readable as bounded UTF-8 source.
- `**` recursion was unbounded. Binary recursion now shares the depth bound, so
  `strings.Repeat("1**", 100000)+"1"` is refused.

Limits added by this round:
- A test that Playwright would skip through `.gitignore` but that sits in a skipped directory
  still refuses.
- A getter or Proxy trap reached by an admitted whole-value read is still not modelled.

## Review round 4

The fourth independent review returned FAIL with four findings; a fifth came from reviewing the
stacked #717 producer branch. The orchestrator decided to fix all five fail-closed. Each landed with
a test that failed on `0efb3dc1` and passes after:

- U+2028 and U+2029 did not end a `//` comment, so `baseURL: 'safe' // c<U+2028> + mutate()` hid a
  call that Node runs before a device spread. `stripComments` and `jsCommentEnd` now end line
  comments at both. Outside literals, `stripComments` rewrites each one to LF plus two spaces
  (offsets kept), so the line-anchored import patterns and the Playwright parsers see the same
  lines. The triple-slash reference pattern reads the body with both mapped to LF, and a JSX
  quote spanning one is ambiguous like one spanning LF. Strings keep them, as ES2019 allows.
  Tests: `TestPlaywrightUseValueUnicodeLineTerminator_V1_1065`,
  `TestUnicodeLineTerminatorsEndCommentsAndLines`.
- The default `.spec.`/`.test.` markers were case-sensitive. Playwright's `createFileMatcher`
  (v1.61.1 `util.ts`) uses minimatch with `nocase: true`, so `e2e/B.SPEC.ts` is a test. The markers
  and string globs now match case-insensitively. This also feeds the skipped-directory refusal
  (`e2e/build/B.SPEC.ts`). Regular expressions keep their own flags.
- A string glob without a leading `**/` was anchored to the absolute path unchanged.
  `createFileMatcher` prefixes `**/`, so `testMatch: ['**/a.spec.ts', 'b.spec.ts']` selects
  `b.spec.ts`; the matcher now does the same. Tests: membership rows in
  `TestPlaywrightDiscoveryFromListMembership_V1_1066` and
  `TestPlaywrightStringGlobsArePrefixedAndCaseInsensitive`.
  - Extension beyond the finding, same matcher and same failure class: minimatch syntax the glob
    compiler did not model was compiled as literal text and could under-select. Extglobs
    (`@(a|b)`), backslash escapes, single-item braces (`{a}`, literal in minimatch) and range
    braces (`{1..3}`) now make the matcher non-static, so selection widens and the producer
    refuses. Character classes already did.
- `playwrightStaticValue` recursed without a bound before the pure-expression depth guard, so
  20,000 nested arrays took about 1.3 s, and the review's 100,000-deep value about 10^10
  character visits. It now shares
  `playwrightPureMaxDepth` (128) and refuses deeper values. Test:
  `TestPlaywrightStaticValueNestingIsBounded_V1_1065`.
- The listing producer permitted but did not require `--list`, so an execution report (which
  applies `test.only`; `--list` disables it) passed as a complete listing. `--list` after `test` is
  now required. Test: the `execution-report` row of `TestPlaywrightDiscoveryFromListRefusals_V1_1066`.

TJAA-V0-018 and TJAA-V0-019 (still proposed) now state the line terminators, the static-value
nesting bound, the required `--list` and the `createFileMatcher` semantics.

Limits added by this round:
- Go's `(?i)` folds a few non-ASCII letters (for example Kelvin `K`) that JavaScript's non-Unicode
  case-insensitive matching does not. Membership can therefore over-include, which refuses or
  over-selects rather than narrowing.
- Brace groups with two or more plain items remain modelled. Other minimatch options (`matchBase`,
  `nobrace`) are not used by Playwright and are not modelled.

## Review round 5

The fifth independent review of `74c1c0fd` returned FAIL with three P1 findings. The orchestrator's
decisions were final: fail closed and prefer general rules. Each fix has a test that failed on
`74c1c0fd` and passes after:

- Templates were skipped as flat quotes, so the backtick that opens a nested template in a `${}`
  substitution closed the outer one. In ``baseURL: `${`//${mutate()}<U+2028>`}` ``, `stripComments`
  then blanked `//${mutate()}` as a comment and rewrote the terminator inside template content, and
  the purity check passed a value that calls user code.
  - `stripComments` now keeps a stack of substitution brace depths. A backtick or a substitution's
    closing `}` resumes template content, which is never rewritten. Nesting past 64 open
    substitutions (`jsTemplateMaxDepth`) and an unterminated template are `ErrSyntax`.
  - The Playwright scanners (`playwrightSplitTopLevel`, `playwrightTopLevelColon`,
    `playwrightBalancedValue`) skip templates through the same bounded, iterative `templateEnd`, and
    refuse when it fails.
  - `scanRequireCode`/`scanRequireTemplate` were already nesting-aware. Their recursion is now bounded
    by the same limit.
  - The other `quotedEnd` callers handle only `'`/`"`, or JSON `"` in `playwright_aliases.go`.
  - `jsCommentEnd` never skips templates; its callers do.
  - Tests: `TestPlaywrightUseValueNestedTemplate_V1_1065`, `TestNestedTemplateLiteralsKeepContent`,
    `TestPlaywrightScannersSkipNestedTemplates`.
- `playwrightGlobPattern` compiled every `**` as a separator-crossing `.*`. In minimatch, `**` is
  globstar only as a whole path component and acts as `*` elsewhere, so `testIgnore:
  '**/e2e/**.spec.ts'` over-ignored `e2e/sub/b.spec.ts`. A stale listing then passed membership.
  `**` that is not a whole component now makes the glob non-static (refusing was simpler to prove
  exact than modelling it). Whole-component `**`, including a trailing `tests/**`, is unchanged.
- The regex matcher dropped `g`, `u` and `y`. Playwright tests from `lastIndex` 0, so the sticky
  `/b\.spec\.ts$/y` never ignores an absolute path, while the unanchored Go regex did. Only `i`, `m`
  and `s` are now mapped; any other flag (`y`, `g`, `u`, `d`, `v`, ...) makes the matcher
  non-static.
- Tests for the glob and flag fixes:
  - Two stale-listing rows in `TestPlaywrightDiscoveryFromListMembership_V1_1066`, each using the
    reviewer's repro: project `p`, `testDir: 'e2e'`, a listing of `keep.test.ts`, then
    `e2e/sub/b.spec.ts` added.
  - New rows in `TestPlaywrightComputedStringsAndUnsupportedGlobsWiden`.
  - `TestPlaywrightStringGlobsArePrefixedAndCaseInsensitive` gains `ims` and trailing-globstar
    projects. They guard the still-modelled cases and passed both before and after.

TJAA-V0-018 now states nested-template lexing and its bound. TJAA-V0-019 now states whole-component
globstar and the `i`/`m`/`s` flag allowlist.

Follow-up (orchestrator decision, final), closing the regex limit this round first recorded:
- Go's `.` (without `s`) and `(?m)` anchors treat only LF as a line terminator, while JavaScript
  also treats CR, U+2028 and U+2029 as line terminators. The producer now refuses when any of these
  is present in:
  - the repository root (as given or resolved);
  - any candidate path enumerated for membership, including those in skipped directories;
  - any listed spec path or `rootDir`.
- Static selection over such a test path widens with `playwright:project-membership-unresolved`.
  The `.`/`(?m)` differences are therefore unreachable.
- Tests, each failing on `936a10d2`:
  - the `line terminator in path` rows of `TestPlaywrightDiscoveryFromListMembership_V1_1066`: a
    U+2028 path that is listed and selected, and an unselected CR candidate;
  - `TestPlaywrightLineTerminatorPathWidensMembership`.
- The non-ASCII `(?i)` folding limit from round 4 stays recorded. A path-only check would not
  close it: a Kelvin sign in the pattern folds onto an ASCII `k` in a path. Closing it would take
  both a pattern check and a path check, and refusing every non-ASCII path or pattern would turn
  away legitimate tests. It is not a one-line check.

## Review round 6

The sixth independent review returned FAIL with one P1 finding at `playwright.go:837`: regex bodies
were compiled by Go as written. In `testIgnore: /\A.*b\.spec\.ts$/`, JavaScript reads `\A` as a
literal A, so the regex ignores nothing. Go reads it as a begin-text anchor and ignored
`e2e/sub/b.spec.ts`, so a stale listing was stamped.

The orchestrator's final decision was an allowlist. `playwrightRegexBodyStatic` admits a body only
when every construct reads identically in JavaScript without `u` and in Go RE2:
- literal characters, and `.`, `^`, `$`, `|`;
- `(...)` and `(?:...)` groups only;
- quantifiers `*`, `+`, `?` after an atom, and `{n}`, `{n,}`, `{n,m}` (n <= m <= 1000), each
  optionally lazy;
- non-empty classes of literals, ranges and allowed escapes;
- the escapes `\d \D \w \W \b \B \t \n \r \f \v`, plus a backslash before ASCII syntax punctuation.

Anything else is non-static. The matcher then follows the existing non-static path: the producer
refuses and selection widens. Constructs refused this way include:
- the escapes `\A`, `\z`, `\Q`/`\E`, `\x`, `\u`, `\p`, `\0` and `\c`;
- backreferences;
- `(?i)`, named groups, lookaround and other `(?` forms;
- POSIX classes and any nested `[`;
- `[]`, `[^]` and `[\b]`;
- stray `{`, `}` and `]`;
- quantifiers on an assertion or another quantifier.

`\s` and `\S` are non-static because JavaScript `\s` includes Unicode spaces (U+00A0, U+FEFF,
U+2000..U+200A, ...) while Go's is ASCII only.

Same failure class, beyond the finding: a JavaScript regex without `u`, and minimatch's compiled
glob, match UTF-16 code units. `.`, `[^/]` or a negated class therefore consumes half of a surrogate
pair, where Go consumes the whole rune. The round-5 path rule now also refuses (producer) or widens
(static selection) when a path contains a character outside the Basic Multilingual Plane.

Tests, each failing on `7b58bad5`:
- the `/\A.*b\.spec\.ts$/` stale-listing row and an emoji-path row in
  `TestPlaywrightDiscoveryFromListMembership_V1_1066`;
- `\A`, `\s`, `(?i)`, backreference, `[[:alpha:]]` and `\x61` rows in
  `TestPlaywrightComputedStringsAndUnsupportedGlobsWiden`;
- `TestPlaywrightRegexBodyAllowlist`. Its positive cases include `\d`, `\.`, `[a-z]{2,3}` and
  `(?:a|b)`, and it checks that each allowlisted body also compiles in Go.

The non-ASCII `(?i)` folding limit from round 4 remains recorded. `\w` and `\b` under `i` are part
of it, because Go folds the Kelvin sign and long s into `\w`.

## Review round 7

Two P1 findings, both fixed by refusing the construct. The matcher becomes non-static, so the
producer refuses and static selection widens.

- Leading-zero repeat bound. JavaScript (Annex B) reads `b{01}` as one `b`. Go's `regexp/syntax`
  rejects a leading zero in a repeat count and reads the braces as literal text. So
  `testMatch: /keep\.test\.ts$|b{01}\.spec\.ts$/` selected `b.spec.ts` only in JavaScript, and a
  listing captured before that file existed was stamped. A bound must now be `0` or ASCII digits
  without a leading zero, with n <= m <= 1000. Cross-check against Go's `parseRepeat`: Go parses
  `{n}`, `{n,}` and `{n,m}` as repeats only with such bounds (and errors on counts above 1000 or
  m < n); every other brace form is literal in Go. JavaScript treats those same allowed forms as the
  same repeats, so they agree, and `{01}`, `{1,02}`, `{1, 2}`, `{ 1}` and `{,2}` are refused.
- Character above U+FFFF in a regex body. JavaScript without `u` reads `😀?` as a high surrogate
  and an optional low surrogate; Go reads an optional emoji. `testIgnore: /😀?b\.spec\.ts$/`
  therefore ignored `b.spec.ts` only in Go. Any such character in a body is now non-static,
  whether literal, in a class or escaped. The same rule applies to string globs compiled as
  minimatch patterns, which also compile to a regex without `u`.

Tests, each failing on `f6b8eb5a`:
- both exact repros as stale-listing rows in `TestPlaywrightDiscoveryFromListMembership_V1_1066`;
- `{01}`, `{00}`, `{1,02}` and non-BMP literal, class, range and escape rows in
  `TestPlaywrightRegexBodyAllowlist` (`{1, 2}`, `{ 1}`, `{,2}` and the escaped form were already
  refused and are now pinned), with `{0}`, `{0,0}`, `{10,100}` and BMP non-ASCII positives;
- `/a{01}\.spec\.ts$/`, an emoji regex and an emoji glob row in
  `TestPlaywrightComputedStringsAndUnsupportedGlobsWiden`.

## Review round 8

Two findings, both closed by refusing what the profile does not model. In each case the matcher or
the use-layer identity becomes non-static, so the producer refuses and static selection widens.

- Repeated `/` in a string glob (`TJAA-V0-019`). Playwright 1.61.1's `createFileMatcher` prefixes
  `**/` and calls its bundled minimatch 3.1.5 with `{nocase: true, dot: true}`. minimatch splits
  both the pattern and the path on `/\/+/`. As a result `**/e2e//*.spec.ts` matches `e2e/b.spec.ts`,
  and so does a leading `/` (which becomes `**//`), while our literal reading did not. With the
  exact repro, a listing captured before `e2e/b.spec.ts` existed was stamped `raw`. Any string glob
  containing `//` is now non-static, and the glob is not normalized.

  We checked minimatch's other preprocessing against the bundled copy (`mm.js` in the round
  scratchpad). A leading `./`, a `..` component and a trailing `/` never match a repository-relative
  test path in either minimatch or our model, so they agree and are not refused.
  `TestPlaywrightGlobAgreesWithBundledMinimatch` pins those agreement rows beside the four
  repeated-slash rows.
- Getter behind a member read (`TJAA-V0-018`). Previously the pure-expression check admitted any
  member read. `const options = { get baseURL() { return go(); } }` then read as
  `baseURL: options.baseURL` could rewrite a devices descriptor during config evaluation, yet the
  browser identity stayed resolved.

  A member read is now admitted only through one of two roots:
  - the `devices` binding of a top-level `@playwright/test` import, read through a string-literal
    device key to a scalar descriptor field, or to `viewport`/`screen` followed by
    `width`/`height`;
  - a top-level `const` bound once to a recursively plain object literal, read through existing
    keys down to a primitive leaf.

  Every occurrence of the root in the comment-stripped file, strings included, must be such a read
  in a read-only position. That rules out assignment, compound assignment, update, `delete`, any
  destructuring target, an alias, and a call argument such as `Object.defineProperty`. An `eval`
  token anywhere admits neither root.

  Every other root is refused: `process.env`, `this`, other imports, function results and numeric
  literals. A `...devices[...]` spread in `use` now also needs the sound `devices` root. This closes
  the related gap where a top-level write to a devices descriptor kept the spread resolved.

Consequences:
- `process.env.BASE_URL` and other `process.env` reads in a `use` layer now widen the browser
  identity. This reverses the V1-1065 positive rows for env-derived `baseURL`/`trace`, and the
  scaffold-style `process.env.BASE_URL ?? '...'` config is no longer narrowed.
- The multi-project fixtures in `playwright_discovery_list_test.go` and
  `cmd/corvint/affected_playwright_test.go` now read `server.baseURL` from a const literal, so the
  positive path stays covered.
- The unused `playwrightUseIdentity` wrapper is removed.

Residual gaps, retained:
- A bare identifier can still read a global accessor.
- A side-effect import or another module can mutate `devices` or `process.env` before the config
  evaluates.
- A computed alias of `devices` obtained via `require` or destructuring with a computed key is not
  traced.
- Go's `(?i)` Kelvin/long-s folding from round 4 remains open.

Tests, each failing on `b79e9d9a` unless noted:
- the repeated-slash repro as a stale-listing row in `TestPlaywrightDiscoveryFromListMembership_V1_1066`;
- `**/tests//*.spec.ts`, `tests//a.spec.ts` and `/tests/*.spec.ts` widen rows in `playwright_test.go`;
- the repeated-slash rows of `TestPlaywrightGlobAgreesWithBundledMinimatch`. Its `./`, `..` and
  trailing-`/` agreement rows already passed.
- `TestPlaywrightMemberReadRoots_GH709Round8`, which covers:
  - the exact getter repro;
  - the devices-descriptor write beside a spread;
  - 31 refused rows: setter, method, spread, computed key, `__proto__`, shorthand, non-primitive
    value, member written, updated or deleted, root reassigned, `Object.defineProperty`, nested
    alias, destructuring, second binding, `export const`, `let`, block scope, `eval`, missing key,
    read through a leaf, undeclared, `process.env`, `this`, and devices object, prototype-key and
    write reads. All failed before except "called member", which was already refused.
  - 6 admitted rows: plain const, nested plain const with a template, optional chain, literal read
    elsewhere, devices scalar, devices `viewport.width`.
- `TestPlaywrightPureExpressionScopedMemberReads`, which is new. In `TestPlaywrightPureExpression`
  the unscoped member-read rows moved from admitted to refused.

## Review round 9

Codex round 9 on `ce716cab` reported three bypasses. Each one is closed by refusing a whole token
class rather than one more construct, so the check fails closed. When a fix applies, the use-layer
identity or the matcher becomes non-static: static selection widens and the producer refuses.

- Escaped identifiers (P1, `TJAA-V0-018`). The occurrence scan matched the root name only as
  written. So `devices['Desktop Chrome'].defaultBrowserType = 'firefox';` rewrote the
  descriptor, and the `...devices['Desktop Chrome']` spread still resolved to chromium. Now any `\`
  in code means the file admits neither root, whatever identifier it is in. In code means outside
  string, template and regular-expression content; there, only an escaped identifier can hold a
  backslash.
- Parenthesized `delete` (P1, `TJAA-V0-018`). `delete (devices['Desktop Firefox'].defaultBrowserType);`
  passed as a read, because the token before the occurrence was `(`. Now any `delete`, `++` or `--`
  token in code means the file admits neither root, whatever the operand. We also checked whether a
  parenthesized reference gets the same assignment, compound-assignment and destructuring-target
  checks as a direct one. It does: every enclosing bracket group is rejected when its close is
  followed by an assignment operator, `of` or `in`. Rows now pin that for `(x) =`, `((x)) +=`,
  `[(x)] =`, `({ a: (x) } =`, and `for ((x) of …)` / `for ((x) in …)`.
- Brace alternative holding `/` (P2, `TJAA-V0-019`). Brace expansion runs before minimatch splits
  on runs of `/`. So `e2e/{/,x}*.spec.ts` expands to `**/e2e//*.spec.ts`, which matches
  `e2e/a.spec.ts`, and a stale listing was stamped. Now any brace alternative containing `/` is
  non-static. The bundled minimatch 3.1.5 matches `/repo/e2e/b.spec.ts` with each of
  `**/e2e/{/,x}*.spec.ts`, `**/e2e/{x,/}*.spec.ts`, `**/{e2e/,x}b.spec.ts` and
  `**/{e2e,x}/b.spec.ts`; it does not match `**/{a/,x}e2e/b.spec.ts`.

New cost:
- A config with a `delete`, `++` or `--` in code anywhere admits neither member-read root. That
  includes code unrelated to the config, such as a loop counter.
- So does a config with an escaped identifier anywhere.
- So does a glob like `{e2e/,x}b.spec.ts`, even though the static model would read it the same way.
- In every one of these cases the browser identity, or the membership, widens.
- Strings, templates and regular expressions are unaffected. `'--headed'` or `/a\.spec\.ts$/`
  beside a root stays admitted, and a positive row pins this.

Tests, each failing on `ce716cab` unless noted:
- In `TestPlaywrightMemberReadRoots_GH709Round8`:
  - both exact repros;
  - the escaped devices root `\u{65}`, the escaped literal root and an escape in an unrelated
    identifier;
  - a parenthesized `delete` of a literal member, plus `delete`, `++` and `--` elsewhere in the
    file;
  - the parenthesized assignment, compound-assignment, destructuring and for-in/of rows (these
    already passed and are pinned);
  - two positive rows.
- The brace repro as a stale-listing row in `TestPlaywrightDiscoveryFromListMembership_V1_1066`.
- `tests/{/,x}*.spec.ts` and `{tests/,x}a.spec.ts` widen rows in
  `TestPlaywrightComputedStringsAndUnsupportedGlobsWiden` (each narrowed before).
- Four brace rows in `TestPlaywrightGlobAgreesWithBundledMinimatch`.

## Review round 10

Codex round 10 on `e514e8bb` reported one P2, which costs precision but does not affect soundness.
`playwrightSlashStartsRegex` read a `/` after `=>` or after `return` as division. So in
`const makePattern = () => /a\.spec\.ts$/;` the regex backslashes counted as code, the round-9
escape guard dropped the `devices` root, and a valid config fell back to the full suite.

Two callers share the rule: the module lexer behind the member-read scope, and
`playwrightSplitTopLevel`, which splits array and object items. For both, a `/` now starts a regular
expression in these positions:
- after an operator or opening punctuation, as before;
- after `=>`;
- after one of the reserved words `return`, `typeof`, `instanceof`, `in`, `new`, `delete`, `void`,
  `throw`, `case`, `do` or `else`.

Everywhere else it is division: after an identifier, a number, `)`, `]`, `}` or a property name such
as `x.return`.

This deviates from the decision in one respect. `of`, `yield` and `await` can also be identifiers,
for example `const of = 4; of / 2`. If we read a division after one of them as a regex, code would
be hidden from the escape, mutation-token and occurrence scans, which would be unsound. A `/` after
one of these words is therefore treated as ambiguous: the lexer fails, so neither root is admitted,
and the splitter refuses. A word glued to a non-ASCII byte or a `\` is also refused, since it is
part of a longer identifier.

The cost is that a regex literal directly after `of`, `yield` or `await` (for example
`for (const r of /x/ …)`) now refuses the source. Before this change it was misread as division.

Tests:
- These rows in `TestPlaywrightMemberReadRoots_GH709Round8` failed on `e514e8bb`:
  - the arrow repro;
  - `return /x\.y/`;
  - a row with `typeof`, `throw` and `else return void` regexes.
- These negative rows already passed and are now pinned: division after an identifier, after `)`
  and after a keyword-named property each keep an escaped `devices` write between two divisions
  visible as code; a slash after `of` refuses.
- `TestPlaywrightSlashStartsRegex` is new and covers the rule table and the shared splitter. It
  could not run before because the function returned a single bool.
