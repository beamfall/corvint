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
