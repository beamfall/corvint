# Affected Plan Python and Playwright Frontiers V0

Owner: Russell Lewis
Date: 2026-10-31
Requirement prefix: `AFP-V0`
Intent status: per requirement and unchanged by the split; AFP-V0-181 to AFP-V0-189, AFP-V0-382 to AFP-V0-389, AFP-V0-404 to AFP-V0-414, AFP-V0-417 to AFP-V0-419 and AFP-V0-423 to AFP-V0-426 accepted under the owner's standing auto-accept (2026-10-11); AFP-V0-442, AFP-V0-443 and AFP-V0-472 to AFP-V0-477 accepted under the standing lane auto-accept (2026-10-11); AFP-V0-680 to AFP-V0-682 experimental under it (2026-10-28); AFP-V0-960 and AFP-V0-961 experimental under it (2026-10-31)
Delivery status: experimental
Authoritative inputs: `docs/specs/affected-plan-v0.md` (parent contract: wire, authority,
failure codes and acceptance gates), `internal/liveverify/affected/python`,
`internal/liveverify/affected` Playwright discovery, `AGENTS.md` invariants 2, 4 and 8. Split
unchanged from the parent (V1-3429) when its Requirements section passed the 256-obligation OCM
limit; the requirements keep their `AFP-V0` IDs, see build-log entry `2026-10-31-afp-split`.

## Agent digest
- Claim: `corvint affected` selects Python tests, advises a pytest command and bounds the Playwright environment, dotenv, alias and tag-closure frontiers.
- Status: per requirement and unchanged by the split; AFP-V0-181 to AFP-V0-189, AFP-V0-382 to AFP-V0-389, AFP-V0-404 to AFP-V0-414, AFP-V0-417 to AFP-V0-419 and AFP-V0-423 to AFP-V0-426 accepted under the owner's standing auto-accept (2026-10-11); AFP-V0-442, AFP-V0-443 and AFP-V0-472 to AFP-V0-477 accepted under the standing lane auto-accept (2026-10-11); AFP-V0-680 to AFP-V0-682 experimental under it (2026-10-28); AFP-V0-960 and AFP-V0-961 experimental under it (2026-10-31); delivery experimental.
- Exists: `internal/liveverify/affected/python`, the Playwright discovery and tag-source files in `internal/liveverify/affected`, `advisoryPytestCheck` in `cmd/corvint`.
- Blocked on: the parent spec's gates; live external-corpus qualification of the Python adapter.
- Read next: Requirements; the parent spec's Non-goals and authority and Failure modes.

## User and current state

The affected user and the plan wire are the parent's: an engineer or agent that wants the tests a
change plausibly needs before the mandatory gate. This companion holds the non-Go frontiers of
that plan: the experimental Python adapter and its advisory pytest command, and the Playwright
environment, dotenv, alias, declaration and tag-closure frontiers. The text below moved verbatim
from the parent; references to Failure modes inside a moved section mean the paragraphs that
moved with it.

## Requirements

### Python adapter (experimental)

`internal/liveverify/affected/python`, registered in `languages.All()`, observes Python modules from
source text. Qualification: experimental. No live external-corpus qualification has run, the adapter
is not advertised as delivered, and the known under-selections under Failure modes must close or be
accepted before promotion.

- **AFP-V0-181:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2050)) The Python
  plugin MUST own exactly the paths ending in `.py` and MUST give each such file one unit with id
  `python:` plus its repository-relative dotted name (a trailing `/__init__` dropped), so no two files
  claim one identity. A file is a test unit when its name starts with `test_` or ends in `_test.py`,
  or a directory component is `tests` or `test` (the discovery conventions pytest and unittest
  share); otherwise it is a source unit. Evidence: `TestUnitsGivesEachFileOneIdentity`,
  `TestIsTestFileMatchesDiscoveryConventions`.
- **AFP-V0-182:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2050)) Import
  edges MUST come from source text alone, with no interpreter or virtual environment: each
  `import a, b as c` or `from x import y` statement, joined across continuation lines (AFP-V0-187). Each name resolves to the longest
  observed module name that prefixes it, where a file under `src/`, `lib/` or `python/` is also
  importable relative to that root. A name inside a string or comment may add an edge; that only
  widens selection. Evidence: `TestParseImportLineReadsEveryStatementForm`,
  `TestResolveRefPrefersTheLongestObservedModule`, `TestModuleAliasesExposeSourceRootLayouts`.
- **AFP-V0-183:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2050)) Each name a
  `from X import name` line imports (not `*`, after any `as` alias, parentheses and trailing comment
  removed) MUST also be tried as the submodule `X.name`, so a change to `pkg/sub.py` selects the test
  that writes `from pkg import sub`, in a regular or namespace package and through a relative
  import. Before this, such a change selected nothing and reported `NO_SELECTABLE_TEST`. Evidence:
  `TestAFPV0183FromImportNamesReachSubmodules` (fails without the change). Rollback: drop the name
  loop in `parseImportLine`.
- **AFP-V0-184:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2050)) A relative
  import MUST be anchored at the importing file's directory, one level up per extra leading dot. One
  that resolves to no observed module, or climbs to the repository root or above, MUST raise the
  frontier `python:relative-import-unresolved` and add no edge. Evidence:
  `TestResolveRefAnchorsRelativeImportsAtTheImportingPackage` (resolution) and
  `TestAFPV0184UnresolvedRelativeImportRaisesItsFrontier` (missing module and above-root raises; V1-2298).
- **AFP-V0-185:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2050)) What source
  text cannot settle MUST be a frontier reason, never a silent omission: `importlib` or `__import__`
  on a line raises `python:dynamic-import`; an indented import raises `python:conditional-import`
  and keeps its edge; an unreadable file raises `python:unreadable-source`. Evidence:
  `TestScanImportsFlagsWhatSourceTextCannotSettle` (dynamic and conditional) and
  `TestAFPV0185UnreadableSourceRaisesItsFrontier` (a file over the source read bound; V1-2298).
- **AFP-V0-186:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2050)) On the
  tracked layered Python corpus `internal/liveverify/affected/testdata/python-corpus` (10 modules, 21
  test files), a one-file edit MUST narrow the full test set to exactly the tests whose imports reach
  the edited module, including the paired test of a module that depends on it. Amended (V1-2298):
  the requirement first named the repository's own 18-module corpus (`src/context_corvint_index.py`),
  which is not tracked on `origin/main`, so its test skipped. Evidence:
  `TestPythonSelectionNarrowsARealCorpus` (editing `src/corpus/index.py` selects 8 of 21 tests).

- **AFP-V0-187:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2297)) An
  `import` or `from` statement continued across lines, inside open parentheses or brackets or after
  a trailing backslash, MUST be read as one statement: its lines are joined, after `#` comments are
  removed, until the brackets balance, and every name it lists is read as AFP-V0-182 and AFP-V0-183
  read a one-line statement. The continuation lines are consumed, so an indented one never raises
  `python:conditional-import`, and a statement still open at end of file keeps the names it shows.
  Before this, `from pkg import (` followed by one name per line read none of the names, so a change
  to `pkg/late.py` selected nothing. Evidence: `TestAFPV0187MultiLineFromImportReadsEveryName`,
  `TestAFPV0187ContinuationLinesAreNotStatements` (both fail without the change). Rollback: read each
  physical line on its own again in `scanImports` and drop `joinContinuation`.
- **AFP-V0-188:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2296)) A
  column-zero `pytest_plugins =` or `pytest_plugins +=` assignment (not a comparison), joined across
  lines as AFP-V0-187, MUST add an absolute import of each module its string literals name. A value
  that is not only string literals MUST raise the frontier `python:pytest-plugins-unresolved`.
  Evidence: `TestAFPV0188PytestPluginsAreImports` (list, multi-line, string and augmented forms; the
  non-literal raise; a comparison not read). Rollback: drop the `pytest_plugins` branch of
  `scanImports` and `pluginModules`.
- **AFP-V0-189:** (accepted under the owner's standing auto-accept (2026-10-11; V1-2296)) Each test
  file MUST have an import edge to every `conftest.py` in its own directory and each ancestor up to
  the repository root, because pytest loads those without an import. A `conftest.py` change therefore
  selects every test at or below its directory and none beside or above it; with AFP-V0-188 a plugin
  module a conftest names reaches the same tests. Evidence: `TestAFPV0189ConftestSelectsEveryTestBelowIt`
  (fails without the change). Rollback: remove `linkConftests`.
- **AFP-V0-680:** (experimental under the standing lane auto-accept, 2026-10-28; V1-3001) Every
  import statement of a physical line that holds several statements separated by `;` MUST add its
  edges, wherever it stands on the line (`import a; import b`, `x = 1; import b`, `import a; from
  pkg import (` continued on the following lines as AFP-V0-187). The line is split once after its
  comment is removed, so the last statement can continue, and once as written, so a `#` inside a
  string does not hide a later import; the lines a continued statement spans are scanned again on
  their own without the conditional-import flag, so a string that only looks like an open import
  never hides the imports after it. A reference is recorded once per file. Evidence:
  `TestAFPV0680EverySemicolonStatementIsAnImport` (fails without the change: `import a; import b`
  did not select the test when `b.py` changed). Rollback: restore the single
  `strings.HasPrefix(trimmed, ...)` check, the `index = next` advance and the one `parseImportLine`
  call in `scanImports`. Amended for V1-3044 (2026-10-30, same auto-accept): an import that is the
  body of a one-line compound statement (`if x: import a`, `try: import a`, `except ImportError:
  import b`, `with c: import a`, `def f(): import a`, any of `if elif else try except finally with
  while for def class async case`) MUST add its edges too; the header ends at its first colon outside
  brackets and string literals that is not the walrus `:=`, and such an import raises
  `python:conditional-import` like an indented one, so a test whose only import of a changed module
  is conditional is still selected. A header colon the scan cannot find leaves the line unread, as
  before. Evidence: `TestAFPV0680CompoundStatementBodyIsAnImport` (fails without the amendment:
  `if x: import b` did not select the test when `b.py` changed). Rollback: restore the boolean
  `hasImportStatement` and drop `importStatement` and `compoundBody`. Amended again (2026-10-30, V1-3044
  review): a `;` separates statements only outside string literals (single, double and triple
  quoted, with backslash escapes) and before a `#` comment, so `if ";" in value: import b` keeps
  its header and adds its edge; a line that ends inside a string literal and mentions `import`
  MUST raise `python:conditional-import`, never nothing, since its statements are not known.
  Evidence: the `semicolon in a string`, `semicolon in a triple`, `escaped quote` and `open triple
  string` cases of `TestAFPV0680CompoundStatementBodyIsAnImport` (each fails with a plain split on
  `;`). Rollback: replace `splitStatements` with `strings.Split(line, ";")` and drop `stringEnd`.
- **AFP-V0-681:** (experimental under the standing lane auto-accept, 2026-10-28; V1-3002) An
  import of a dotted name MUST add an edge to the unit of every observed module name that prefixes
  it, not only the longest: Python runs each ancestor package's `__init__.py` before the submodule,
  so `import pkg.sub`, `from pkg.sub import f` and `from pkg import sub` each reach
  `pkg/__init__.py` and a change to it selects a test that imports only the submodule. Relative
  imports anchor as AFP-V0-184 and then reach the same ancestors. Evidence:
  `TestAFPV0681SubmoduleImportReachesEveryAncestorPackage` (fails without the change for
  `pkg/__init__.py`), `TestResolveRefPrefersTheLongestObservedModule` (the longest name first,
  then each ancestor). Rollback: return the first match from `resolveRef`.
- **AFP-V0-682:** (experimental under the standing lane auto-accept, 2026-10-28; V1-3003) A
  module name that several files own through source-root aliases (`lib/core.py` and `src/core.py`
  are both `core`) MUST keep every owner, in file order, and an import of the name MUST add an edge
  to each of them, so the importing unit's `imports` show the ambiguity and a change to either file
  selects its consumers. Which file an interpreter loads depends on its path, which is not
  modelled; no owner is hidden, so no frontier is raised. Evidence:
  `TestAFPV0682SourceRootAliasReachesEveryOwner` (fails without the change: only `python:lib.core`
  was an import, so a `src/core.py` change selected nothing). Rollback: register only the first
  owner of each name in `UnitsSource`.

Python non-goals: no interpreter, import hook, `sys.path` or `.pth` evaluation, packaging metadata
(`pyproject.toml`, `setup.cfg`) or virtual-environment discovery; no pytest collection.

Python failure modes and gaps (each widens unknowns or under-selects; none is promoted):
A plugin registered outside a `.py` file (`-p` on the command line, `pytest.ini`, `pyproject.toml`
or a `pytest11` entry point) is not read: a change to that file is already an unowned unknown, but
a change to the plugin module it names selects only the tests that import it. A test that reads a non-Python data file records no path token, so that file stays an
unowned unknown. `sys.path` changes and source roots other than `src/`, `lib/` and `python/` are not
modelled.

### Playwright environment inputs and TypeScript alias and config frontiers (GH #1640, #1604)

- **AFP-V0-382:** (accepted under the owner's standing auto-accept (2026-10-11; GH #1640))
  `corvint affected --playwright-config PATH` MUST accept `--playwright-env NAME=VALUE` and
  `--playwright-env NAME`, repeatable, each `NAME` (letters, digits and underscores, not starting
  with a digit) at most once. `NAME=VALUE` is a caller-supplied value (`source` `ARGUMENT`);
  `NAME` reads that one variable from the invoking environment (`source` `ENVIRONMENT`). No
  variable the caller did not name is read. The flag without `--playwright-config` or with
  `--snapshot` (a snapshot plan reads no invoking environment), a repeated name, or an invalid
  name is `invalid-arguments`. Falsifier: a plan that resolves a variable
  the caller did not name, or a parser that accepts a repeated or invalid name. Rollback: remove
  the `--playwright-env` branch and `affectedPlaywrightEnvironment` from `cmd/corvint/affected.go`.
- **AFP-V0-383:** (accepted under the owner's standing auto-accept (2026-10-11; GH #1640))
  A Playwright plan whose invocation named a variable MUST carry `plan.environment`, sorted by
  name, with each variable's `name`, `source`, `state` (`SET` or `UNSET`), `value` and the
  value's lowercase hex SHA-256 `sha256`; an `UNSET` variable has an empty `value` and `sha256`.
  A plan that named none omits `plan.environment`; nothing else in it changes beyond the details
  AFP-V0-387 and AFP-V0-388 require.
  The recorded value is the caller's observation, not repository evidence: a different value is
  visibly a different plan input. Falsifier: a plan under a named variable without its record, a
  digest that is not the value's, or a plan with the member when none was named.
- **AFP-V0-384:** (accepted under the owner's standing auto-accept (2026-10-11; GH #1640))
  The modelled `use.launchOptions.executablePath` expressions are `process.env.NAME` and
  `process.env['NAME']` (either quote), alone or followed by `?? 'literal'` or `|| 'literal'`.
  A key or fallback literal with any escape sequence is not modelled, since its JavaScript value
  is not decoded here. Every other computed expression keeps the TJAA-V0-035 EXECUTION unknown
  unchanged.
- **AFP-V0-385:** (accepted under the owner's standing auto-accept (2026-10-11; GH #1640))
  For a modelled expression over a recorded variable, each project's executable MUST resolve as
  JavaScript evaluates it: an `UNSET` bare read is `undefined`, recorded as
  `executable.bundled: true` (Playwright launches its bundled browser) only when the config names
  no `channel`, which otherwise decides the installed binary and keeps the unknown; a `SET` value is
  `executable.path`; `??` takes the literal only for `UNSET`, and `||` takes it for `UNSET` or an
  empty value. A resolved project carries `executable` (`bundled`, `path`, `variable`) and no
  computed-executable EXECUTION unknown. The resolution never changes selection: the EXECUTION
  axis already left scope unchanged (TJAA-V0-035). Falsifier: a resolved project that keeps the
  unknown, or a resolution that differs from JavaScript evaluation.
- **AFP-V0-386:** (accepted under the owner's standing auto-accept (2026-10-11; GH #1640))
  The resolution MUST NOT apply when something other than the caller's environment can set the
  variable before Playwright evaluates the config: the config writes `process.env` (TJAA-V0-042
  already refuses the identity), loads any module other than `@playwright/test` or the
  `path`/`os`/`url`/`fs` built-ins (with or without `node:`; a dotenv loader is the common case),
  has module loads that are not all static, names a module loader or evaluator under any alias
  (`require`, `createRequire`, `module`, `loadEnvFile`, `eval`, `Function`, `globalThis`,
  `global`), or names `globalSetup`. The EXECUTION unknown stays and its detail names
  the variable and the writer.
- **AFP-V0-387:** (accepted under the owner's standing auto-accept (2026-10-11; GH #1640))
  A modelled read of a variable the plan did not record, or one that evaluates to an empty string
  (a value whose Playwright meaning the profile does not model), MUST keep the EXECUTION unknown,
  and its detail MUST name the variable and, when unrecorded, the `--playwright-env NAME` input
  that would record it. Missing evidence stays an unknown; no value is guessed.
- **AFP-V0-388:** (accepted under the owner's standing auto-accept (2026-10-11; GH #1640))
  When no `--playwright-discovery` receipt is given, the `playwright:discovery-unproven` unknown's
  detail MUST start `MISSING:` and name the absent receipt and the
  `corvint affected discovery --playwright-config PATH --playwright-list FILE` command that writes
  it, since that input alone empties the selection. Scope, fallback and selection are unchanged.
- **AFP-V0-389:** (accepted under the owner's standing auto-accept (2026-10-11; GH #1604))
  In the general TypeScript adapter (no `--playwright-config`), a test of a runner other than
  Playwright MUST NOT keep tsconfig `paths` aliases unresolved (TJAA-V0-037/050) when no module
  in its relative-import closure names a bare reference the nearest `tsconfig.json` can claim.
  The test stays a blocker when any closure module has an unparsed, dynamic or unresolved load
  (including a relative import that resolves to no observed module), names a workspace package, or sits under an invalid nearest `tsconfig.json`, and a runner
  config (such as a Jest config) still blocks as before. Falsifier: an exempt test whose closure
  names a claimed alias, or an exemption under an opaque closure module.
- **AFP-V0-404:** (accepted under the owner's standing auto-accept (2026-10-11; GH #1604))
  In the general TypeScript adapter, a Playwright config's membership markers MUST NOT raise
  `typescript:executable-config-unresolved` when the closed static profile resolves its
  membership and the general adapter agrees with it: the only markers are `testDir` and
  `projects:`, the parse leaves no membership unknown (identity, use-value and dependency
  unknowns decide which browser runs, not which files belong; an absent `projects` is one
  implicit project), no project has `testMatch` or `testIgnore`, and every observed file the
  profile assigns to a project is a Playwright test of that config in the general adapter. Any
  other config keeps the frontier. Falsifier: a config without the frontier whose profile owns a
  file the adapter classifies otherwise, or one with `testMatch`, `globalSetup` or a computed
  `testDir`. Rollback for AFP-V0-389/404: restore the immediate `FrontierConfig` raise in
  `observe` and drop the `aliasFree` argument of `playwrightOnlyRunners`.

### Playwright dotenv inputs and grep-omission verification (GH #1646, #1647)

- **AFP-V0-405:** (accepted under the owner's standing auto-accept (2026-10-11; GH #1646/#1647);
  amends AFP-V0-386) A config's dotenv use MUST NOT be a writer under AFP-V0-386 when it is a
  modelled load: the config's leading top-level statements (before any other config code) are only
  `'use strict'`, side-effect imports of other modules, import declarations, require bindings and
  dotenv `config()` expression statements; dotenv is bound as a default or namespace import of
  `dotenv` or a plain `NAME = require('dotenv')`, or called as `require('dotenv').config(...)`; and
  each `config()` argument is absent or an object literal with only a literal `path` (no escape,
  no `~`, a repository-relative path that path cleaning leaves unchanged apart from one leading
  `./`, so not `.env/` or `missing/../.env`, which dotenv passes to the filesystem as written, and
  that is not `.`, outside the repository or a `.vault` file), `override: false`, and literal `true`/`false` `quiet` or `debug`. An absent `path` is
  `.env`. The path is relative to the repository root, the plan argv's working directory. Such a
  load reads one statically known file and never overrides a variable already present. Falsifier: a
  modelled load whose file or override behaviour depends on a value not in source.
- **AFP-V0-406:** (accepted under the owner's standing auto-accept (2026-10-11; GH #1646/#1647))
  When a plan named a variable and the config's only possible writers are modelled loads, each
  load's file MUST be observed and recorded, in load order, in `plan.environmentFiles` with its
  `path`, `site` (`CONFIG:LINE` of the call) and `state`: `ABSENT` when no file exists, or
  `PRESENT` with the file's lowercase hex SHA-256 `sha256`. A present file is parsed after dotenv's
  CRLF/CR normalization in the one line grammar every dotenv release reads alike: blank and whole-line
  `#` comment lines are skipped, and every other line is `NAME=VALUE` (optional spaces or tabs
  around `=`; a `NAME` of word characters, `.` or `-`) whose value is unquoted, or wholly single- or
  double-quoted with nothing after the closing quote, with no `#`, backslash, backtick, control
  character or JavaScript whitespace other than space and tab; the last duplicate wins. A plan
  without such a load omits `plan.environmentFiles`. Falsifier: a recorded digest that is not the
  file's, or a parsed value that differs from dotenv's.
- **AFP-V0-407:** (accepted under the owner's standing auto-accept (2026-10-11; GH #1646/#1647))
  Each recorded variable of such a plan MUST carry `resolved`, the value the config reads after the
  loads: a variable `SET` by the caller stays as recorded; an `UNSET` variable takes the first
  observed file's value that names it (`source` `DOTENV`, `file`, `state` `SET`, `value` and its
  `sha256`), or stays `UNSET` when no file names it. The variable's own `source`, `state`, `value`
  and `sha256` remain the caller's observation. AFP-V0-385 and AFP-V0-387 then evaluate the
  resolved value, so an absent `.env` leaves the `--playwright-env` value standing. Falsifier: a
  file value that overrides a caller-set variable, or a resolution that differs from dotenv's.
- **AFP-V0-408:** (accepted under the owner's standing auto-accept (2026-10-11; GH #1646/#1647))
  Every other dotenv use MUST keep the AFP-V0-386 EXECUTION unknown, and its detail MUST name the
  call site as `CONFIG:LINE` or the file: a named import, a non-literal or other `config()`
  argument (including `override: true`, which older dotenv releases ignore), a call that is not an
  expression statement, any dotenv use after other config code, an existing or uncheckable
  `<file>.vault` (dotenv decrypts it when `DOTENV_KEY` is set), an unreadable, non-regular or
  non-UTF-8 file, or a line outside the AFP-V0-406 grammar (named by number). `dotenv/config`,
  `NODE_OPTIONS`, any other loader and `globalSetup` stay writers as in AFP-V0-386. Falsifier: a
  resolved executable under one of these uses.
- **AFP-V0-409:** (accepted under the owner's standing auto-accept (2026-10-11; GH #1646/#1647);
  amends TJAA-V0-021 (b)) For a listing omission, the producer MUST decide whether the source tags
  are literal: the config, the test file and their static import closure all read as UTF-8, are
  all in the source set the receipt's source digest binds (so editing any of them makes the
  receipt BINDING_MISMATCH), have only static loads that resolve to exactly one existing
  repository candidate (two candidates such as `tags.cjs` and `tags.js` are ambiguous) or to
  `@playwright/test`, `dotenv`, `dotenv/config` and the `path`/`os`/`url`/`fs` or `node:`
  built-ins, name in code none of the reflective globals and their aliases
  (`Object`/`Reflect`/`JSON`/`Proxy`/`eval`/`Function`/`globalThis`/`global`/`window`/`self`/`structuredClone`),
  the routes to them (`constructor`/`prototype`/`__proto__`/`__defineGetter__`/`__defineSetter__`),
  the enumerating or defining helpers
  (`values`/`entries`/`assign`/`defineProperty`/`defineProperties`/`fromEntries`/`getOwnPropertyNames`/`getOwnPropertyDescriptor`/`getOwnPropertyDescriptors`/`setPrototypeOf`)
  (the in-place array mutators are no longer among them; AFP-V0-425), hold no `'tag'`
  string, identifier escape, escaped string property key, computed object key or method, computed
  member write, or member access by other than an integer or string literal, and every code `tag` is
  an object-literal key whose value is a literal `@\S+` string (no escape, at least one character
  after `@`) or an array of them. Those strings are the closure's literal tags. Any other closure
  is not literal. Falsifier: a literal closure in which a describe or config tag could hold
  whitespace or be `@` alone.
- **AFP-V0-410:** (accepted under the owner's standing auto-accept (2026-10-11; GH #1646/#1647);
  amends TJAA-V0-021 (b)) When the source tags are literal and each case's explicit tags (the
  reported tags less the title `@\S+` runs) are all among them, the reported tokens equal the grep
  title's parts, so a case that (b) excludes is excluded for certain: the omission MUST be accepted
  without `unverifiedUnits` and without the `playwright:discovery-omission-unverified` unknown.
  Patterns, titles or tags outside that rule keep (b). Falsifier: a verified omission whose case
  Playwright's grep keeps, or one whose sources hold a non-literal tag.
- **AFP-V0-411:** (accepted under the owner's standing auto-accept (2026-10-11; GH #1646/#1647);
  amends TJAA-V0-021) The `playwright:discovery-omission-unverified` detail MUST name the unit
  count and list the units as `project "P" test PATH`, at most ten, then `and N more`.
- **AFP-V0-412:** (accepted under the owner's standing auto-accept (2026-10-11; GH #1646/#1647))
  A `BINDING_MISMATCH` `playwright:discovery-unproven` detail MUST name each binding that differs
  (revision, config path, config digest, source digest) with the receipt's and the plan's values
  (object ids and digests as their first 12 characters) and say to regenerate the receipt at this
  revision with `corvint affected discovery --playwright-config PATH --playwright-list FILE` from
  a fresh `playwright test --list --reporter=json`. State, scope and fallback are unchanged.
- **AFP-V0-413:** (accepted under the owner's standing auto-accept (2026-10-11; GH #1646/#1647))
  A `grep` or `grepInvert` using a JavaScript-only regular-expression feature (lookaround, a
  backreference, a flag other than `i`, `m` or `s`, or any other construct outside TJAA-V0-021
  (a)) MUST NOT be verified by AFP-V0-410: the omission stays in `unverifiedUnits`, and a pattern
  static selection cannot read keeps the plan `STATIC_UNRESOLVED`.
- **AFP-V0-414:** (accepted under the owner's standing auto-accept (2026-10-11; GH #1646/#1647))
  The new `plan.environmentFiles` and `environment[].resolved` members are omitted when empty, so a
  plan without a modelled dotenv load is byte-identical to before; the `playwright-discovery/0`
  profile is unchanged, and a receipt written before AFP-V0-410 that lists `unverifiedUnits` stays
  valid and unverified. Rollback for AFP-V0-405 to AFP-V0-414: restore the `dotenv` writer branch
  in `playwrightEnvironmentWriter`, drop `playwright_dotenv.go`, `playwright_tag_sources.go`, the
  `sourceTags` argument of `classifyPlaywrightGrepOmission`, and `playwrightBindingMismatch` and
  `playwrightUnitList` in `playwright_discovery.go`.
- **AFP-V0-417:** (accepted under the owner's standing auto-accept (2026-10-11; GH #1660);
  amends AFP-V0-386) A loader name of AFP-V0-386 MUST count as a writer when it occurs as a whole
  identifier in the config's code, in a template substitution, or as a whole string literal that
  is a property key through which the config can read that property: a computed member key
  (`box['global']`), a computed property key (`{ ['loadEnvFile']: load }`) or a quoted property
  key (`{ 'loadEnvFile': load }`), since a destructuring pattern reads through either. Inside other
  string, template or regular-expression text, an array literal element (`['loadEnvFile']`)
  included, it MUST be data, such as a project or dependency named `'global-setup'`, but only while the
  config tokenizes and holds no reflective channel through which text could become code or reach
  a property the scan cannot see; otherwise every occurrence counts, as before. The writer detail
  MUST name the matched name and its `CONFIG:LINE` site. `globalSetup` stays a writer wherever the
  config names it. Falsifier: a config whose only `global` words are string data keeping the
  EXECUTION unknown, or a code reference to `global`, `globalThis` or another loader name, or a
  quoted or computed destructuring key naming one, resolving the executable.
- **AFP-V0-418:** (accepted under the owner's standing auto-accept (2026-10-11; GH #1660);
  amends AFP-V0-409) A static load in the tag closure whose bare specifier the nearest bound alias
  config resolves through `paths` or `baseUrl` to exactly one bound repository file MUST be
  followed into that file, with the resolution the dependency graph uses (TJAA-V0-012,
  TJAA-V0-037). The nearest alias config is the one Playwright reads: the `tsconfig.json` or
  `jsconfig.json` of the nearest folder above the importing file that holds either, the
  `tsconfig.json` when a folder holds both. Every `jsconfig.json` MUST be in the source set the
  discovery receipt binds, so editing one invalidates the receipt. It MUST stay unresolved, and the
  closure not literal, when no bound alias config claims the specifier, the nearest one is not
  valid (unreadable, malformed or using `extends`), the importing file is not `.ts` or `.tsx` and
  the nearest config does not set `allowJs` to `true` (a `jsconfig.json` defaults it to `true`),
  since Playwright then maps nothing, the target is missing or ambiguous, or the Playwright config
  names `tsconfig` (Playwright may then read another file). Falsifier: a verified omission whose
  alias target holds a tag that is not literal, or one followed through an alias config
  Playwright does not read or applies to that importer.
- **AFP-V0-419:** (accepted under the owner's standing auto-accept (2026-10-11; GH #1660);
  amends AFP-V0-411) When the tag closure of the first unverified unit is not literal, the
  `playwright:discovery-omission-unverified` detail MUST append where it stopped: the file and
  the first specifier that resolves to no single repository file, a load that is not static, a
  file that is unreadable or does not tokenize, a file outside the bound sources, or a file whose
  tag or construct is not literal. The receipt and the `playwright-discovery/0` profile are
  unchanged; the reason is recomputed at plan time from the bound sources. Rollback for
  AFP-V0-417 to AFP-V0-419: restore the loader-name loop in `playwrightEnvironmentWriter`, drop
  `playwrightEnvLoaderReference`, `playwrightEnvKeyString`, `playwrightObservedName`, the
  `allowJS` field, the alias argument of `playwrightTagFileExact`, `resolveAlias`,
  `exactOrStop` and the `stop` field, and the appended detail in `SelectPlaywrightWithEnvironment`.
### Playwright frontier sites and tag-closure precision (GH #1688)

GitHub #1688 re-tested a Playwright repository on main `b64eb76` and found the Playwright plan
UNKNOWN with causes it could not see or that were false positives. AFP-V0-423 to AFP-V0-426 name
the real sites and narrow one redundant tag rule; build-log entry
2026-10-24-playwright-frontier-sites-gh1688 records the analysis. No receipt, plan field or
profile changes encoding: only detail and frontier-site text changes, and one tag rule accepts more.

- **AFP-V0-423:** (accepted under the owner's standing auto-accept (2026-10-11; GH #1688);
  amends AFP-V0-404) When a Playwright config keeps `typescript:executable-config-unresolved`, the
  site MUST name what the static profile refuses, never a modelled `testDir` or `projects:`
  marker: the first other membership marker (`testMatch`, `globalSetup` and the rest) at its line
  with that marker as the token, else the config file with the first membership unknown's reason,
  `projects` when the parse yields no project, `testMatch` for a project matcher, or
  `owns <file>` for a profile-owned file the adapter does not classify as a Playwright test of the
  config. Which configs keep the frontier is unchanged. Failure mode: the site points at a literal
  `testDir` and the owner looks for a cause that is not there. Falsifier: a frontier entry whose
  token is `testDir` or `projects`, or a config whose only computed value is a project's
  `dependencies` gaining the frontier. Acceptance: `TestPlaywrightConfigFrontierNamesBlocker_GH1688`,
  `TestFrontierSitesNameFileLineAndToken_GH1604`.
- **AFP-V0-424:** (accepted under the owner's standing auto-accept (2026-10-11; GH #1688))
  The `playwright:dynamic-source-reachability` selection unknown MUST name what left the shared
  TypeScript graph unresolved: each widening unknown of the shared plan other than
  `NO_SELECTABLE_TEST`, as its reason and path (`UNOWNED_DIRTY_PATH data/seed.json`), or for a
  `LANGUAGE_FRONTIER` reason the sites that raised it with their tokens, at most 8 entries and 8
  sites per reason with the rest counted. The unknown is raised exactly when it was before. Failure
  mode: an UNKNOWN selection whose cause needs a separate `--path` run to find. Falsifier: a
  dynamic-source unknown whose detail omits a widening unknown of the shared plan when fewer than
  9 exist. Acceptance: `TestPlaywrightDynamicSourceNamesSites_GH1688`.
- **AFP-V0-425:** (accepted under the owner's standing auto-accept (2026-10-11; GH #1688);
  amends AFP-V0-409 and AFP-V0-419) A test details object with no `tag` key, such as
  `{ annotation: annotation('id') }`, MUST add no tag and MUST NOT make its file inexact. The
  in-place array mutators (`push`, `unshift`, `splice`, `fill`, `copyWithin`) MUST NOT make a file
  inexact: every literal tag array is the value of a literal `tag` key, so the only routes to it
  name `tag` outside a key, index a member by other than a literal, or name a reflective global,
  route or helper, and each of those already keeps the closure inexact. When a file stays inexact
  its stop MUST name the first rule it breaks and that construct's line, as
  `<file> holds a tag or construct that is not one literal @\S+ string (line N: <rule>)`.
  Failure mode: an ordinary `page.fill` or `ids.push` hides the whole grep omission behind a
  generic stop. Falsifier: a literal closure in which a tag array can be mutated to hold
  whitespace or `@` alone without one of the remaining rules firing, or a stop whose line is not
  the construct's. Acceptance: `TestPlaywrightTagClosureDetailsWithoutTag_GH1688`,
  `TestPlaywrightGrepOmissionSourceTags_GH1647`.
- **AFP-V0-426:** (accepted under the owner's standing auto-accept (2026-10-11; GH #1688))
  In Playwright mode the `typescript:e2e-runtime-dependency` execution unknown MUST fold every
  site whose token is the Playwright runner itself (an `@playwright/test` import or the
  `playwright` runner identity) into one count, `N test file(s) depend on the Playwright runner
  @playwright/test`, and MUST still name every other runtime-dependency site. The unknown and its
  axis are unchanged; the general adapter's path-mode frontier still names each site. Failure
  mode: the runner import listed once per spec buries the evidence that matters. Falsifier: a
  non-runner runtime dependency missing from the detail, or a count that differs from the
  runner sites. Acceptance: `TestPlaywrightRuntimeDependencySummary_GH1688`.
  Rollback for AFP-V0-423 to AFP-V0-426: return `playwrightStaticMembership` to a bool and raise
  `FrontierConfig` at `dynamicConfigMarker`, restore the fixed dynamic-source detail and
  `frontierDetail` for the runtime unknown, drop `graphFrontierDetail`, `namedFrontierSites` and
  `playwrightRuntimeDetail`, and restore the mutators and the boolean result of
  `playwrightTagTextLiteral`.
- **AFP-V0-960:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-2562,
  amends AFP-V0-425) Three ordinary spec constructs MUST NOT make a closure file inexact. (1) A
  `const NAME = JSON.parse(text)` or `let NAME = JSON.parse(text)` with one argument, no reviver,
  no type annotation, whose binding the fresh-object escape analysis (TJAA-V0-095 to 099) admits:
  each use is a member read or a call argument of a neutral sink, so the keys the parse builds
  never reach a test details argument. Any other `JSON.parse` (a reviver, a direct member read, a
  copy, an enumeration, an argument) still stops the closure. (2) `Object.values(NAME)` on the
  same terms TJAA-V0-117 sets for `Object.entries(NAME)`. (3) A computed member read `X[NAME]`
  whose NAME is the counter of the innermost enclosing `for (let NAME = DIGITS; ...) { ... }`
  block loop, in which every other use of NAME is an update (`++`, `--`), a comparison, a single
  arithmetic operand, a property name or the whole index of another member access: NAME is then a
  number, so the read takes a numeric key and never the key `tag`. A counter that is assigned,
  rebound, shadowed, read outside its loop, or used as a computed write target still stops the
  closure. Failure mode: an ordinary e2e spec reading a JSON fixture, summing `Object.values` or
  walking `rows[i]` hides the whole grep omission behind a generic stop. Falsifier: a GH #1647
  hidden-tag fixture, or a spec that reaches a tag array through one of the three admitted forms,
  whose closure stays exact. Acceptance: `TestPlaywrightTagClosureParseValuesCounter_V12562`,
  `TestPlaywrightGrepOmissionSourceTags_GH1647`, `TestPlaywrightTagClosureDetailsWithoutTag_GH1688`,
  `TestPlaywrightTagClosureFreshObjectTypeKeyAndEntries_GH1881`.
- **AFP-V0-961:** (experimental under the owner's standing lane auto-accept (2026-10-11); V1-2582)
  A neutral module that can build an object whose keys come from text MUST NOT leave the closure
  exact. When any closure file imports `url`, `node:url` or `dotenv`, the first closure file that
  names `parse` (other than `JSON.parse`), `parsed`, `populate` or `processEnv` in code MUST stop
  the closure, naming that line as ``(line N: `NAME` builds an object from text, so a `tag` key
  can exist without its name)``: `url.parse(text, true).query`, `dotenv.parse(text)`, a
  `config()` result's `parsed` and `dotenv.populate` or `config({ processEnv })` each yield a
  `tag` property without the text `tag`. A `node:` built-in import is followed as non-neutral
  unless it is one of `node:path`, `node:os`, `node:url`, `node:fs`, `node:fs/promises`,
  `node:crypto`, `node:child_process`, `node:process`, `node:buffer`, `node:assert`,
  `node:events`, `node:timers`, `node:timers/promises` or `node:test`; others, such as `node:vm`,
  `node:module`, `node:querystring` or `node:util`, can evaluate code or build keyed objects.
  Failure mode: `test('x', url.parse('/?tag=@x', true).query, ...)` selects a tagged test that
  the closure reports as untagged, so a grep omission goes unreported. Falsifier: a spec that
  passes such a value as details, with a closure that stays exact. Known gap: `process.env`
  passed whole or spread as details is not modelled. Acceptance:
  `TestPlaywrightTagClosureParseValuesCounter_V12562`.
  Rollback for AFP-V0-960 and AFP-V0-961: drop `playwrightCounterIndex`, `playwrightCounterLoop`,
  `playwrightCounterUse`, `playwrightParseCallEnd`, `playwrightParseDeclaration` and the `values`
  method, restore the `{`-only fresh-object initializer, and drop `playwrightTagKeyModules`,
  `playwrightTagKeyBuilders`, `playwrightTagNeutralBuiltins` and the `builderStop` aggregation,
  restoring the blanket `node:` neutrality.

### Playwright declaration closure and the declaration path (AFP-V0-442, AFP-V0-443, GitHub #1751)

Intent (GitHub #1751 retest, V1-2693): a repository whose standalone Node CLI (`scripts/*.mjs`)
reads a declared documentation file got `playwright:declared-path-contradicted`, although no
Playwright test loads that script, and a branch that adds `.corvint/documentation-paths.json`
raised `UNOWNED_DIRTY_PATH` for the declaration itself. Both kept a bounded plan from narrowing.

- **AFP-V0-442:** (accepted under the owner's standing lane auto-accept (2026-10-11); V1-2693, GitHub #1751)
  Amending `TJAA-V0-101` (2), only a source in the Playwright closure contradicts the
  documentation declaration. There are four parts.
  - (1) The closure is the config's unit, every unit holding a bound Playwright test, and every
    unit those reach by an import, test import, advisory import, route-dispatch import or exec
    edge, including the global hooks the config binds. To a fixpoint, it also holds every unit a
    change of which reaches a closure unit (as importer or path-literal reader, such as a spec
    that names a script as an argument), with that unit's own edges. It is incomplete when more
    than 2,048 units lie outside the edge closure, when a closure unit carries a unit frontier
    or dropped its path tokens at the bound, or when a graph frontier reason has no located site
    or a site whose file is in the closure or is no graph unit.
  - (2) A file that names a matched path is in the closure, and contradicts the declaration as
    `TJAA-V0-101` (3) says, when it is the config, is no TypeScript graph unit (for example
    `jsconfig.json`), the closure is incomplete, or its unit is in the closure. The unknown's
    detail adds that reason. Any file whose membership is unknown therefore still refuses. The
    closure is built only once some file names a matched path.
  - (3) Every other file that names a matched path is an out-of-closure reader. The plan's
    `declaration` member lists each one, in path order, in `outOfClosure` as `path` (the first
    matched path it names) and `site` (file, line and spelling); the member is omitted when there
    are none. Such a reader alone leaves `state` `APPLIED`.
  - (4) A project-owned list of tool-only readers is a non-goal: the closure classifies a reader
    from observed edges, while an asserted exemption would let declared authority override
    observed evidence that disagrees with it (`TJAA-V0-101` (3)).
  Non-goals: modelling a reader that a test reaches only through a spawned process whose command
  no closure unit names (a `package.json` script, a `webServer` command), which is the bounded
  exclusion `TJAA-V0-013` already keeps for helpers no spec reaches; modelling reporters, which
  the closure does not include. Failure modes: an incomplete closure or an unowned reader refuses,
  so the plan widens to the full suite rather than narrows. Rollback: drop `playwrightClosure` and
  the `outOfClosure` member and count every observed file as `TJAA-V0-101` (2) did; no persisted
  store changes.
- **AFP-V0-443:** (accepted under the owner's standing lane auto-accept (2026-10-11); V1-2693, GitHub #1751)
  Extending `AFP-V0-160`, a dirty `.corvint/documentation-paths.json` that no unit names and no
  dirty-path walk reaches (no path literal, declared read scope, directory reader or unbounded
  reader) MUST raise no `UNOWNED_DIRTY_PATH` and MUST be listed in `nonSource` as one
  `CORVINT_DECLARATION` group, after the `DECLARED_DOCUMENTATION` group: the affected planner
  reads it, classifies the plan's paths by its current bytes and records its digest, so it is no
  build or test input. A unit that names or reaches it keeps the unknown entry, so in a
  repository whose own code reads the declaration, such as Corvint, the path stays unowned.
  Failure modes: a reader that resolves the path at run time from parts no token holds is not
  seen, as for `AFP-V0-088`. Rollback: drop `declarationPath` and `ClassCorvintDeclaration`.

### Advisory pytest command (AFP-V0-472 to AFP-V0-477, V1-2760)

The Python adapter selects test units, but until this slice `advice.checks` named no command for
them, so an agent on a Python repository got a plan it could not run. These requirements add one
advisory pytest entry beside the Go (AFP-V0-009) and RSpec (AFP-V0-066) entries, from the plan and
a bounded read of the configuration files pytest itself would read. They amend AFP-V0-009's
advice list only; `provider`, the plan, the compact form and `corvint test` are unchanged.

- **AFP-V0-472:** (accepted under the owner's standing lane auto-accept (2026-10-11); V1-2760)
  Command and working directory. When `plan.selected` holds a unit whose id begins `python:` with
  a `tests` file ending `.py`, and AFP-V0-473 to AFP-V0-476 admit it, `advice.checks` MUST gain one
  `advisory` entry with source `affected-plan`, placed after the Go and RSpec entries, whose
  command is `python -m pytest` followed by the sorted, deduplicated test modules AFP-V0-474 keeps,
  each prefixed `./` and POSIX single-quoted as AFP-V0-066 quotes spec files. The command runs from
  the repository root, the directory every advice command assumes; `python -m` puts that directory
  on `sys.path`, which is what a flat-layout project needs. The reason counts the Python test units
  and names the repository root, the configuration file pytest reads (or that pytest's defaults
  apply) and any AFP-V0-475 source root.
- **AFP-V0-473:** (accepted under the owner's standing lane auto-accept (2026-10-11); V1-2760)
  Declared use of pytest. The entry exists only when the repository declares pytest: the
  AFP-V0-474 configuration file holds a pytest section (a `pyproject.toml` pytest only falls back
  to, without a pytest table, declares nothing); a selected file, or a unit of the graph, is a
  `conftest.py`; or a root `pyproject.toml`, `setup.cfg`, `setup.py`, `tox.ini`, `Pipfile` or
  `requirements*.txt`, each read only as a regular file of at most 256 KiB, names `pytest` as a
  whole word. Otherwise `unknown` gains
  `NO_ADVISORY_PYTEST_COMMAND: the plan selected Python test units but the repository declares no pytest configuration, conftest.py or pytest dependency`.
  The dependency search is bounded: at most 4096 repository-root entries listed, 16
  `requirements*.txt` files and 1 MiB of manifest bytes read in all. Reaching a bound before
  `pytest` is found yields
  `NO_ADVISORY_PYTEST_COMMAND: no pytest configuration or conftest.py is declared and the root dependency manifests exceed the discovery bound (<bound>)`
  instead, naming the bound that was hit.
- **AFP-V0-474:** (accepted under the owner's standing lane auto-accept (2026-10-11); V1-2760)
  Configuration discovery and `python_files`. The configuration is the one pytest 8's
  `determine_setup` selects for the test modules as its arguments, run from the repository root:
  searching from their deepest common directory up to the repository root, the first of
  `pytest.ini` (always, even when empty), `.pytest.ini` with a `[pytest]` section,
  `pyproject.toml` with a `[tool.pytest.ini_options]` or `[tool.pytest]` table, `tox.ini` with a
  `[pytest]` section and `setup.cfg` with a `[tool:pytest]` section, in that order per directory;
  when none holds one, the first `pyproject.toml` seen is the configuration file with pytest's
  defaults. When that search finds nothing, a `setup.py` regular file between the common directory
  and the repository root stops discovery with pytest's defaults; otherwise the same search runs
  from each module's own directory in argument order. Values are read as pytest reads them: an ini
  value is the raw text after `=` or `:`, continuation lines joined, whole-line `#` and `;`
  comments skipped, and split with POSIX shell rules (Python `shlex.split`), a value that cannot
  be split yielding
  `NO_ADVISORY_PYTEST_COMMAND: <path> sets <key> to a value pytest cannot split`; a TOML array is
  taken as its strings and a TOML string is shell-split, with `#` comments outside strings
  dropped. Its `python_files`
  patterns (default `test_*.py *_test.py`, and a present but empty value matching nothing) keep
  the candidates pytest treats as test modules, by
  Python `fnmatch` on the base name, or on the path end for a pattern with `/`; the plugin's other
  test-directory files, such as `conftest.py`, helpers and `__init__.py`, drop out. Because
  filtering can deepen the common directory, the search repeats over the kept modules until the
  configuration file is the same, at most three times; otherwise `unknown` gains
  `NO_ADVISORY_PYTEST_COMMAND: the pytest configuration pytest would read for the selected test files does not settle (<a>, then <b>)`.
  A candidate configuration file that exists but is not a regular file of at most 256 KiB yields
  `NO_ADVISORY_PYTEST_COMMAND: <path> may hold the pytest configuration but is not a readable regular file of at most 256 KiB`.
  No `rootdir`, `-c`, `--rootdir`, `PYTEST_ADDOPTS` or `addopts` value is evaluated; pytest
  applies the file's own `addopts`, `testpaths` and plugins when the command runs.
- **AFP-V0-475:** (accepted under the owner's standing lane auto-accept (2026-10-11); V1-2760)
  Source-root module path. The command is prefixed `PYTHONPATH='<root>[<sep><root>]' ` naming each
  Python source root (`src`, `lib`, `python`, the adapter's `SourceRoots`) that is not itself a
  regular package (no `<root>/__init__.py`), that the configuration's `pythonpath` (resolved
  against the configuration file's directory) does not already name, and that an emitted module,
  inside the root or outside it, reaches through the graph's import edges to a `python:<root>.`
  unit. Such a test imports the root's modules by their root-relative names, so it fails to import
  without the root on the module path, as a src-layout project's tests do; pytest puts a test's
  own directory there, not the root. `<sep>` is the path-list separator of the platform Corvint
  runs on (`:`, or `;` on Windows), where the agent runs the advice; the env-prefix form itself
  still assumes a POSIX shell, as every advice command does.
- **AFP-V0-476:** (accepted under the owner's standing lane auto-accept (2026-10-11); V1-2760)
  Named reasons. A selection whose modules all fail `python_files` yields no entry and
  `NO_ADVISORY_PYTEST_COMMAND: the plan selected <n> Python test unit(s) but no selected file matches python_files <patterns>`
  (`<patterns>` is `(empty)` for an empty value);
  more than 1024 modules yield
  `NO_ADVISORY_PYTEST_COMMAND: <n> selected test files exceed the 1024-file bound`. Every reason
  this slice cannot name a runnable command is exactly one such line, so a Python selection never
  ends with neither a command nor a reason.
- **AFP-V0-477:** (accepted under the owner's standing lane auto-accept (2026-10-11); V1-2760)
  Scope. A plan that selects no `python:` unit with a `.py` test file gains nothing, so every
  receipt without one keeps its exact bytes; the Go and RSpec entries, their unknown lines, the
  AFP-V0-038 compact form and `provider` (which keeps only `go`) are unchanged, and the pytest
  command is never compacted.

Non-goals: no execution, interpreter, virtual environment or dependency resolution; no `tox`,
`nox`, `ansible-test`, `unittest` or Django runner; no node ids or `-k` expressions (whole modules
only); no pytest plugin, `conftest.py` hook or `pytest_collect_file` model; no `PYTHONPATH` beyond
the adapter's source roots; and no change to Python selection itself.
Failure modes: the command can still fail where the environment lacks the project's dependencies
or Python version, or where a project's own runner owns its tests (ansible `test/integration`
targets reached by the Python adapter are listed as modules and may not collect under plain
pytest); the selection precision is the adapter's (AFP-V0-181 to AFP-V0-186), and the unknown
frontier and mandatory checks still apply (AFP-V0-004). A `[tool.pytest.ini_options]` table
declared with dotted keys or an inline table is not read, so its `python_files` fall back to the
default. Falsifier: a Python plan with no pytest entry and no `NO_ADVISORY_PYTEST_COMMAND` line; an
entry whose modules pytest's own configuration would not treat as test modules; a src-layout
command without the `PYTHONPATH` prefix that its tests need; or a non-Python receipt whose bytes
change. Rollback: drop the `advisoryPytestCheck` call in `compileAffectedAdvice`; no state, cache
or wire member besides the advice entries depends on it.

## Non-goals

The parent contract's non-goals and authority rules apply to every requirement here.

AFP-V0-382 to AFP-V0-388 do not read any variable the caller did not name, do not model
`NODE_OPTIONS` preloads, shell wrappers, `globalSetup` or other process.env writers (each keeps
the unknown), and do not resolve the TJAA-V0-047 e2e-runtime-dependency frontier. AFP-V0-405 to
AFP-V0-408 model only the dotenv prelude loads they name: no `dotenv/config`, `dotenv-expand`,
`@dotenvx`, `process.loadEnvFile`, variable expansion, multiline or escaped values, encrypted
`.vault` files or a runtime `DOTENV_KEY`/`DOTENV_CONFIG_*` setting. AFP-V0-409 to AFP-V0-413 do not
evaluate computed patterns, titles or tags, run Playwright, or read tags through a package outside
the repository.
AFP-V0-389/404 do not change the `--playwright-config` mode or alias resolution for another
runner's loader.

## Failure modes

The parent contract's failure codes apply. The Python failure modes are inline under the Python
adapter section.

`--playwright-env` failure modes (AFP-V0-382 to AFP-V0-387): a repeated or invalid name, or the
flag without `--playwright-config` or with `--snapshot`, is `invalid-arguments`; an unrecorded
variable, an empty value, an undefined value under a `channel`, an escaped literal, an
unmodelled expression or a possible writer keeps the EXECUTION unknown with a detail
that names the variable. A caller-supplied value that differs from the runtime's is the caller's
observation and is visible in `plan.environment`; it moves no test, because the executable is on
the EXECUTION axis. AFP-V0-389/404 fail closed: any unobserved load, workspace package, invalid
tsconfig, unmodelled marker or disagreement between the profile and the adapter keeps the
existing frontier. AFP-V0-405 to AFP-V0-413 fail closed: an unmodelled dotenv form, `.vault`
file, unreadable file or unmodelled line keeps the EXECUTION unknown and names the site or line,
and a non-literal tag closure (including an escaped or reflective route to a tag, an ambiguous
module extension or a dependency outside the bound source set), a reported tag the sources do not
hold or an unevaluable pattern keeps the omission unverified with its units named.

## Acceptance evidence and traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| AFP-V0-181 | `Language.Owns`, `primaryModuleName`, `isTestFile` in `internal/liveverify/affected/python/python.go` | `TestUnitsGivesEachFileOneIdentity`; `TestIsTestFileMatchesDiscoveryConventions` |
| AFP-V0-182 | `scanImports`, `parseImportLine`, `resolveRef`, `moduleAliases`, `SourceRoots` | `TestParseImportLineReadsEveryStatementForm`; `TestResolveRefPrefersTheLongestObservedModule`; `TestModuleAliasesExposeSourceRootLayouts` |
| AFP-V0-183 | the imported-name loop in `parseImportLine` | `TestAFPV0183FromImportNamesReachSubmodules` |
| AFP-V0-184 | `resolveRef`, `resolve`, `FrontierRelativeImport` | `TestResolveRefAnchorsRelativeImportsAtTheImportingPackage`, `TestAFPV0184UnresolvedRelativeImportRaisesItsFrontier` |
| AFP-V0-185 | `scanImports`, `FrontierDynamicImport`, `FrontierConditionalImport`, `FrontierUnreadableSource` | `TestScanImportsFlagsWhatSourceTextCannotSettle`, `TestAFPV0185UnreadableSourceRaisesItsFrontier` |
| AFP-V0-186 | `languages.All()` registration; `python.New` | `TestPythonSelectionNarrowsARealCorpus` |
| AFP-V0-187 | `scanImports`, `joinContinuation` in `internal/liveverify/affected/python/python.go` | `TestAFPV0187MultiLineFromImportReadsEveryName`; `TestAFPV0187ContinuationLinesAreNotStatements` |
| AFP-V0-188 | `scanImports`, `pluginsAssignment`, `pluginModules`, `FrontierPytestPlugins` | `TestAFPV0188PytestPluginsAreImports` |
| AFP-V0-189 | `linkConftests` | `TestAFPV0189ConftestSelectsEveryTestBelowIt` |
| AFP-V0-382 | `--playwright-env` in `parseAffectedOptions`, `addAffectedPlaywrightEnv`, `affectedPlaywrightEnvironment` in `cmd/corvint/affected.go` | `TestAffectedPlaywrightEnvFlag_GH1640`, `TestAffectedPlaywrightEnvResolvesExecutableEndToEnd_GH1640` |
| AFP-V0-383 | `PlaywrightEnvironmentInput`, `NewPlaywrightEnvironmentInput`, `canonicalPlaywrightEnvironment` in `internal/liveverify/affected/typescript/playwright_env.go`; `PlaywrightPlan.Environment` | `TestPlaywrightEnvironmentInputRecord_GH1640`, `TestPlaywrightStaticPlanRecordsEnvironment_GH1640`, `TestAffectedPlaywrightEnvResolvesExecutableEndToEnd_GH1640` |
| AFP-V0-384 | `playwrightEnvExecutable` | `TestPlaywrightExecutableResolvesFromRecordedEnvironment_GH1640`, `TestPlaywrightExecutableUnknownKeepsReason_GH1640` (unmodelled expression), `TestPlaywrightExecutableEscapedLiteralKeepsUnknown_GH1640` |
| AFP-V0-385 | `resolvePlaywrightExecutables`, `PlaywrightExecutable`, `PlaywrightProject.Executable` | `TestPlaywrightExecutableResolvesFromRecordedEnvironment_GH1640`, `TestPlaywrightStaticPlanRecordsEnvironment_GH1640`, `TestPlaywrightExecutableUndefinedWithChannelKeepsUnknown_GH1640`, `TestAffectedPlaywrightEnvResolvesExecutableEndToEnd_GH1640` |
| AFP-V0-386 | `playwrightEnvironmentWriter`, `playwrightEnvLoaderNames` | `TestPlaywrightExecutableUnknownKeepsReason_GH1640` (dotenv module, global setup), `TestPlaywrightExecutableLoaderWriterKeepsUnknown_GH1640` |
| AFP-V0-387 | `resolvePlaywrightExecutables` details | `TestPlaywrightExecutableUnknownKeepsReason_GH1640`, `TestPlaywrightStaticPlanRecordsEnvironment_GH1640` |
| AFP-V0-388 | the `MISSING` detail in `SelectPlaywrightWithEnvironment` (`playwright_discovery.go`) | `TestPlaywrightMissingDiscoveryNamesInput_GH1640` |
| AFP-V0-389 | `aliasFreeTests`, `aliasFreeClosure` in `internal/liveverify/affected/typescript/alias_gate.go`; `playwrightOnlyRunners` in `typescript.go` | `TestAliasFreeRunnerAndStaticPlaywrightMembership_GH1604`, `TestAliasReachingRunnerStillBlocks_GH1604`, `TestUnresolvedRelativeImportStillBlocks_GH1604`, `TestBlockedAliasGateSelectsAndNamesBlocker_GH1604`, `TestGeneralAdapterPlaywrightMembershipAndAliases_GH1578` |
| AFP-V0-404 | `playwrightStaticMembership` in `alias_gate.go`; the deferred `FrontierConfig` raise in `observe` | `TestAliasFreeRunnerAndStaticPlaywrightMembership_GH1604`, `TestPlaywrightConfigFrontierKeptWhenMembershipDiffers_GH1604`, `TestFrontierSitesNameFileLineAndToken_GH1604` |
| AFP-V0-405 | `playwrightDotenvLoads`, `playwrightDotenvArguments` in `internal/liveverify/affected/typescript/playwright_dotenv.go`; `playwrightEnvironmentWriter` in `playwright_env.go` | `TestPlaywrightDotenvAbsentFileKeepsRecordedValue_GH1646` |
| AFP-V0-406 | `observePlaywrightDotenv`, `parsePlaywrightDotenv`, `PlaywrightEnvironmentFile`; `PlaywrightPlan.EnvironmentFiles` | `TestPlaywrightDotenvPresentFileResolves_GH1646`, `TestPlaywrightDotenvLineGrammar_GH1646`, `TestPlaywrightStaticPlanRecordsEnvironmentFile_GH1646` |
| AFP-V0-407 | `resolvePlaywrightDotenv`, `PlaywrightEnvironmentResolution`; `resolvePlaywrightExecutables` | `TestPlaywrightDotenvAbsentFileKeepsRecordedValue_GH1646`, `TestPlaywrightDotenvPresentFileResolves_GH1646` |
| AFP-V0-408 | the frontier details of `playwrightDotenvLoads` and `observePlaywrightDotenv` | `TestPlaywrightDotenvFrontierNamesSite_GH1646`, `TestPlaywrightExecutableUnknownKeepsReason_GH1640` (dotenv module), `TestPlaywrightExecutableLoaderWriterKeepsUnknown_GH1640` |
| AFP-V0-409 | `playwrightTagSources`, `playwrightTagFileExact`, `playwrightTagTextLiteral`, `playwrightLiteralTagProperty`, `playwrightResolveRelative` in `playwright_tag_sources.go` | `TestPlaywrightGrepOmissionSourceTags_GH1647`, `TestPlaywrightGrepOmissionDependencyInvalidates_GH1647` |
| AFP-V0-410 | the `sourceTags` rule of `classifyPlaywrightGrepOmission` in `playwright_grep_omission.go`; `checkPlaywrightListMembership` in `playwright_discovery_list.go` | `TestPlaywrightGrepOmissionSourceTags_GH1647`, `TestPlaywrightDiscoveryGrepOmission_TJAA_V0_021` |
| AFP-V0-411 | `playwrightUnitList` in `playwright_discovery.go` | `TestPlaywrightUnitListBound_GH1647`, `TestPlaywrightGrepOmissionSourceTags_GH1647`, `TestPlaywrightDiscoveryGrepOmission_TJAA_V0_021` |
| AFP-V0-412 | `playwrightBindingMismatch` in `playwright_discovery.go` | `TestPlaywrightBindingMismatchNamesBinding_GH1647` |
| AFP-V0-413 | `playwrightTitleMatchers` refusal in `classifyPlaywrightGrepOmission` | `TestPlaywrightGrepOmissionJavaScriptOnlyRegex_GH1647`, `TestPlaywrightDiscoveryGrepOmission_TJAA_V0_021` (static grepInvert outside the partwise model) |
| AFP-V0-414 | `omitzero` on `PlaywrightPlan.EnvironmentFiles` and `PlaywrightEnvironmentInput.Resolved` | `TestPlaywrightStaticPlanRecordsEnvironment_GH1640`, `TestPlaywrightDiscoveryGrepOmission_TJAA_V0_021` (receipt unverifiedUnits) |
| AFP-V0-417 | `playwrightEnvLoaderReference`, `playwrightEnvKeyString`, `playwrightEnvironmentWriter` in `playwright_env.go` | `TestPlaywrightEnvLoaderNameInStringIsData_GH1660` (string, template, regex and array-literal data resolve; code, template substitution, computed member key, quoted and computed destructuring keys and escaped text keep the writer with its line) |
| AFP-V0-418 | `resolveAlias`, `boundFiles` in `playwright_tag_sources.go`; `allowJS` in `parseTypeScriptAliases`; `playwrightObservedName` in `playwright.go`; the alias argument of `playwrightTagFileExact` | `TestPlaywrightGrepOmissionFollowsTypeScriptAliases_GH1660` (paths, baseUrl, a nearer jsconfig, a tsconfig beside a jsconfig and a `.js` importer under `allowJs` or a jsconfig verified; hidden tag, no tsconfig, `extends`, missing target, a nearer jsconfig redirecting to a hidden tag, a `.js` importer without `allowJs` and a config naming `tsconfig` stay unverified), `TestPlaywrightSourcesBindJSConfig_GH1660` (a jsconfig.json edit changes the source identity) |
| AFP-V0-419 | `exactOrStop` and `playwrightTagFile.stop` in `playwright_tag_sources.go`; the stop detail in `SelectPlaywrightWithEnvironment` | `TestPlaywrightGrepOmissionFollowsTypeScriptAliases_GH1660` (each stop names the file and specifier) |
| AFP-V0-423 | `playwrightStaticMembership` in `alias_gate.go`; the `FrontierConfig` raise in `observe` | `TestPlaywrightConfigFrontierNamesBlocker_GH1688`, `TestFrontierSitesNameFileLineAndToken_GH1604` |
| AFP-V0-424 | `graphFrontierDetail`, `namedFrontierSites` in `playwright.go` | `TestPlaywrightDynamicSourceNamesSites_GH1688` |
| AFP-V0-425 | `playwrightTagIndirection`, `playwrightTagTextLiteral`, `playwrightTagFileExact` in `playwright_tag_sources.go` | `TestPlaywrightTagClosureDetailsWithoutTag_GH1688`, `TestPlaywrightGrepOmissionSourceTags_GH1647` |
| AFP-V0-426 | `playwrightRuntimeDetail`, `classifyPlaywrightFrontier` in `playwright.go` | `TestPlaywrightRuntimeDependencySummary_GH1688` |
| AFP-V0-960 | `playwrightCounterIndex`, `playwrightParseDeclaration`, `playwrightEntriesCall` in `playwright_tag_sources.go` | `TestPlaywrightTagClosureParseValuesCounter_V12562`, `TestPlaywrightGrepOmissionSourceTags_GH1647` |
| AFP-V0-961 | `playwrightTagKeyModules`, `playwrightTagKeyBuilders`, `playwrightTagNeutralBuiltins` in `playwright_tag_sources.go` | `TestPlaywrightTagClosureParseValuesCounter_V12562` |
| AFP-V0-442 | `playwrightDeclaredPaths`, `playwrightClosure.build` and `playwrightClosure.member` in `internal/liveverify/affected/typescript/playwright_declaration.go` | `TestAffectedPlaywrightDeclarationClosure_GH1751` in `cmd/corvint/affected_playwright_closure_gh1751_test.go` (an out-of-closure `scripts/check.mjs` reader recorded in `outOfClosure` with `APPLIED`; a script a spec names, a module that script imports, and a closure with a dynamic import still refuse); `TestAffectedPlaywrightDeclarationReporterShape_GH1751` in `cmd/corvint/affected_playwright_reporter_shape_gh1751_test.go` (the reporter's committed `--base` range: a `testDir` config, a `package.json`-run CLI naming `docs/migrations/test-behavior-contracts.json` at line 11 plus its `node --test` file, recorded out of closure; plan narrowed to the changed spec); `TestAffectedPlaywrightPathDeclaration_GH1751` (an imported helper, specs and the config still refuse); reporter repository `NOT_OBSERVED` |
| AFP-V0-443 | `declarationPath` and `ClassCorvintDeclaration` in `internal/liveverify/affected/documentation.go`, called from `Select` | `TestDocumentationDeclarationIsCorvintDeclaration_AFPV0443` (classified when unnamed; a file literal, a climbing directory literal and an unbounded reader keep `UNOWNED_DIRTY_PATH`); the declaration-added-in-the-range case of `TestAffectedPlaywrightDeclarationClosure_GH1751` and of `TestAffectedPlaywrightDeclarationReporterShape_GH1751`; `TestAffectedDocumentationDeclarationAddedInRange_GH1751` in `cmd/corvint/affected_documentation_range_gh1751_test.go` (generic plan: a declaration and a declared document both added in the range give `BOUNDED` with both classified; without the declaration the document is `UNOWNED_DIRTY_PATH`) |
| AFP-V0-472 | `advisoryPytestCheck` in `cmd/corvint/affected_pytest.go`, called from `compileAffectedAdvice` in `cmd/corvint/affected.go` | `TestAFPV0472FlatLayoutPytestCommand` (flat layout, pyproject.toml, conftest.py and a helper filtered, no prefix) and `TestAFPV0475SrcLayoutPytestCommand` in `cmd/corvint/affected_pytest_advice_test.go`; both fixtures' commands run green under pytest 8.4.2, and the src one errors without its prefix; replay over the four V1-2717 pairs (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-26-affected-pytest-advice.md`) |
| AFP-V0-473 | `hasConftest`, `adviceDeclaresPytestDependency` in `cmd/corvint/affected_pytest.go` | `TestAFPV0473PytestNeedsADeclaration` (no declaration is the unknown; an empty `pytest.ini`, a `tox.ini` `[pytest]` section, `requirements-dev.txt` and `setup.py` each declare; `notpytest_x` does not; a selected `conftest.py` does); `TestAFPV0473PytestDependencyDiscoveryBound` (17 `requirements*.txt` files and 1.2 MiB of manifests are the bound line); `TestAFPV0474PytestPerArgumentDiscovery` (a table-less `pyproject.toml` declares nothing) |
| AFP-V0-474 | `settlePytestConfig`, `determinePytestConfig`, `locatePytestConfig`, `pytestConfigFrom`, `pytestConfigValues`, `iniPytestValues`, `tomlPytestValues`, `tomlValue`, `shlexSplit`, `matchPythonFiles`, `fnmatchExpression` in `cmd/corvint/affected_pytest.go` | `TestAFPV0474PytestConfigurationDiscovery` (a nearer `tests/unit/pytest.ini` wins after filtering, `pytest.ini` outranks `pyproject.toml`, no matching module, an oscillating pair does not settle, an oversized `pyproject.toml` is unknown); `TestAFPV0474PytestPerArgumentDiscovery` (the per-argument search finds `tests/a/pytest.ini`, a root `setup.py` stops discovery, a table-less `pyproject.toml` is the file); `TestAFPV0474PytestValueParsing` (TOML comments outside strings are not values, quoted ini values are shell-split, an unclosed quote is the split line, an empty `python_files` matches nothing); `TestAFPV0475SrcLayoutPytestCommand` (multi-line `setup.cfg` `python_files`); pytest 8.4.2 collects a quoted `python_files` value |
| AFP-V0-475 | `pytestSourceRoots`, `advicePathListSeparator` in `cmd/corvint/affected_pytest.go` | `TestAFPV0475SrcLayoutPytestCommand` (`PYTHONPATH='src'` from the graph's import edges; a declared `pythonpath` drops it); `TestAFPV0475TestsUnderASourceRootGetThePrefix` (`src/tests/test_core.py` importing `calc.core` gets the prefix, which pytest 8.4.2 needs; two roots joined by the platform separator; a commented TOML `pythonpath` declares nothing) |
| AFP-V0-476 | the `NO_ADVISORY_PYTEST_COMMAND` lines and `adviceMaxPytestFiles` in `cmd/corvint/affected_pytest.go` | `TestAFPV0476PytestBoundAndQuoting` (1025 modules are the bound line; sorted, deduplicated, `./`-prefixed, quoted operands); the unknown cases of `TestAFPV0473PytestNeedsADeclaration`, `TestAFPV0473PytestDependencyDiscoveryBound`, `TestAFPV0474PytestConfigurationDiscovery` and `TestAFPV0474PytestValueParsing` |
| AFP-V0-477 | the Python-unit guard at the top of `advisoryPytestCheck` | `TestAFPV0477NonPythonPlanAdviceUnchanged` (no `.py` test file adds nothing; the pytest entry follows unchanged Go and RSpec entries); the existing AFP-V0-009 and AFP-V0-066 advice tests unchanged |
| AFP-V0-680 | `scanImports`, `hasImportStatement`, `splitStatements`, `stringEnd`, `isImportStatement`, `importStatement`, `compoundBody` in `internal/liveverify/affected/python/python.go` | `TestAFPV0680EverySemicolonStatementIsAnImport`, `TestAFPV0680CompoundStatementBodyIsAnImport` |
| AFP-V0-681 | `resolveRef`, `resolve` | `TestAFPV0681SubmoduleImportReachesEveryAncestorPackage`; `TestResolveRefPrefersTheLongestObservedModule` |
| AFP-V0-682 | the `moduleNames` owner lists in `UnitsSource`, `resolveRef`, `resolve` | `TestAFPV0682SourceRootAliasReachesEveryOwner` |

## Rollback

Move these sections, paragraphs and rows back to the stubs in `docs/specs/affected-plan-v0.md`,
delete this file and run `make spec-requirements`. The IDs did not change, so no test, ticket,
code comment or receipt changes.
