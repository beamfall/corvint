# Test Source Analysis V0

Owner: Russell Lewis
Date: 2026-10-17
Intent status: accepted under the owner's standing auto-accept (2026-10-11; V1-1587, V1-1589, V1-1592, V1-1698, V1-1769, V1-1783, V1-1784, V1-1824, V1-1798, V1-1787, V1-1858, V1-1866, V1-1867, V1-1919, V1-1930, V1-1931, V1-1932, V1-1944)
Delivery status: experimental
Authoritative inputs: V1-1587 / GitHub #1030, V1-1589 / GitHub #1032 and V1-1592 / GitHub #1035
owner requests, V1-1698 / GitHub #1082 for shared-constant keys, and V1-1769 / GitHub #1114 owner requests, V1-1783 / GitHub #1123, V1-1784 / GitHub #1124, V1-1824 / GitHub #1140 and V1-1798 / GitHub #1128 and #1130 and V1-1858 / GitHub #1159, V1-1866 / GitHub #1163, V1-1867 / GitHub #1164, V1-1919 / GitHub #1185, V1-1930 / GitHub #1191, V1-1931 / GitHub #1192 and V1-1932 / GitHub #1193 owner requests and V1-1944, agent-drafted and accepted under the owner's standing auto-accept (2026-10-11);
AGENTS.md invariants 1, 2, 4, 7 and 8.

## Agent digest
- Claim: Read-only analysis of committed tests selects setup projects, lints changed lines, lists candidate duplicates and flags static test-validity defects.
- Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1587, V1-1589, V1-1592, V1-1698, V1-1769, V1-1783, V1-1784, V1-1824, V1-1798, V1-1787, V1-1858, V1-1866, V1-1867, V1-1919, V1-1930, V1-1931, V1-1932, V1-1944)/experimental
- Exists: `internal/testsource`, `internal/testvalidity/source_rules.go`, `internal/contextindex/setup_select.go` and the experimental `corvint tests setup-select`, `corvint tests lint`, `corvint tests duplicates` and `corvint tests validity` commands.
- Blocked on: an MCP tool for setup-select, a preflight lint hookup, showing advisory findings in a run receipt and a preflight surface for the static test-validity rules are not built; real-suite qualification is NOT_RUN.
- Read next: Requirements; Boundaries and failure modes; Delivery.

## User and verified baseline

An adopter with a Playwright-style suite runs many specs against shared setup projects that seed
data contexts, and reviews test changes for quality and duplication by hand. At
`55abb815c4c20a771363d38e315ddfddf14c6972` Corvint parses the declared string-key registry
dialect (`internal/contextindex/registry.go`) and resolves web imports, but nothing maps a spec set
to the setup projects it needs, lints test sources, or compares test assertions. The simpler
baseline is running every setup project for every spec, reviewing by eye, and grepping for repeated
`expect` lines. Success is a deterministic, bounded answer that falls back to the safe superset
(ALL setup files) or an explicit uncertainty whenever it cannot prove a narrower one.

## Requirements

### Shared reading

- `TSA-V0-010`: (standing auto-accept, 2026-10-11; V1-1589, V1-1592) Lint and duplicates MUST read
  only committed Git content at an explicit revision (`git cat-file`, `ls-tree`, `diff`), never the
  worktree, and MUST tokenize with the application map's JavaScript/TypeScript lexer
  (`appmap.LexJS`) rather than a second lexer. Test files are JS/TS files whose stem ends in `.spec`
  or `.test`; page-object files are JS/TS files under a `pages`, `page-objects`, `pageobjects` or
  `pom` directory or whose stem ends in `.page` or `Page`. Declarations are `test`/`it`/`specify`
  (and `x`/`f` variants) calls with a title and callback, nested in `describe`/`context`/`suite`
  calls, with `only`, `skip`, `fixme`, `fail`, `slow`, `serial` and `parallel` modifiers; a
  `test.skip(...)` call without a callback is a runtime annotation. A file the lexer cannot place,
  or with unbalanced brackets, is listed as uncertain rather than silently dropped. At most 5000
  files of at most 1 MiB each (64 MiB in total) are read; more refuses with `tests-bound-exceeded`.
  Changed paths come from a NUL-delimited `git diff --name-only -z` inventory, and every patch
  header MUST decode (Git's trailing tab after a name with a space, C-quoting) to an inventory path
  or the read refuses with `tests-source-unavailable`. Diff reads MUST be independent of Git
  attributes, repository diff configuration and external drivers (`--text`, `--no-textconv`,
  `--no-ext-diff`, fixed `a/` and `b/` prefixes), so no local attribute can hide a hunk (amended
  2026-10-17 under the standing auto-accept after the PR #1054 review).
- `TSA-V0-011`: (standing auto-accept, 2026-10-11; V1-1587, V1-1589) Comments MUST be returned apart
  from tokens, so a mention inside a comment never counts as code (an import, a registry lookup, an
  assertion), and annotations are read from comments only.

### Setup selection (V1-1587)

- `TSA-V0-001`: (standing auto-accept, 2026-10-11; V1-1587) `corvint tests setup-select --specs
  PATH[,PATH...]` MUST list the setup files (JS/TS files whose stem ends in `.setup` or `.seed`)
  that the named spec files reach at the revision. A spec reaches a file through a static import,
  `export ... from`, a literal `import()` or a literal `require()`, transitively, skipping type-only
  imports. It also reaches a setup file through a literal key of a declared string-key registry
  (the AMAP registry dialect): the key's bound module is a data context, and every setup file that
  imports that module directly is selected and is itself traversed like any reached file, so its
  own setup dependencies are selected and its unresolved imports return ALL (amended 2026-10-17 after
  the PR #1054 review). Mode is `selected`.
- `TSA-V0-002`: (standing auto-accept, 2026-10-11; V1-1587) A registry declaration file and a hub
  (a non-setup file that imports every setup file in a universe of at least two) MUST NOT be
  followed and MUST be listed under `notFollowed` with reason `registry` or `hub`, so a barrel never
  widens a selection to everything.
- `TSA-V0-003`: (standing auto-accept, 2026-10-11; V1-1587) When the closure cannot be proved, the
  result MUST be mode `all` with every setup file and at least one typed reason. The setup universe
  is every tracked setup path at the revision, including generated or oversized files the index does
  not read, so ALL is the whole safe superset (amended 2026-10-17 after the PR #1054 review). Reasons: `computed-lookup` (a
  non-literal registry key, `import()` or `require()`), `unknown-key` (a literal key the registry
  does not declare), `shared-constant-key-unresolved` (TSA-V0-007), `registry-unresolved`, `missing-path` (a spec that is not a clean, tracked JS/TS
  path at the revision), `empty-spec-list`, `import-unresolved`, `source-unreadable` or
  `bound-exceeded` (at most 10 000 files in the closure, 100 reasons).
- `TSA-V0-004`: (standing auto-accept, 2026-10-11; V1-1587) A comment-only mention of an import or a
  registry key MUST NOT select anything (TSA-V0-011).
- `TSA-V0-005`: (standing auto-accept, 2026-10-11; V1-1587) The JSON output (schema
  `corvint-tests-setup-select/0`) MUST record the full revision, the specs, the universe size and an
  `inputDigest` (`sha256:` over the sorted path and blob of every file read, the registries and the
  application-map manifests), deterministic across calls. `--format lines` MUST print only the setup
  paths, one per line, usable as a test provider's setup project list.
- `TSA-V0-006`: (standing auto-accept, 2026-10-11; V1-1698 / GitHub #1082) A shared constant is an
  exported object literal (`export const|let|var NAME [: T] = {`, `export default {`, or a local
  object constant exported through `export { NAME [as ALIAS] }`). An import of it is followed through
  `export * from`, `export * as NS from` and `export { A as B } from` re-exports, renamed imports and
  `NS.NAME` namespace members (also nested, `NS.INNER.NAME`, and `require("m").NAME` or
  `import("m").NAME` with a literal module), at most 8 hops. An `export *` cycle MUST NOT hide a
  constant another path reaches: a resolution shaped by a module already being resolved or by a
  bound is never cached (amended 2026-10-18 after the PR #1096 review). A read is also seen in the
  module that declares the constant (`export const C = {...}; ... C.key`). When a reached file
  reads a literal property of a
  shared constant (`C.key`, `C?.key`, `C!.key`, `C["key"]`, `C?.["key"]`, a `${}` template
  substitution, or a `{ key, key: alias } = C` destructuring), every setup file that reads the same
  literal property of the same constant MUST be selected and traversed like a registry seeder
  (TSA-V0-001); a setup file that reads that constant only non-literally is also selected while any
  key of it is reached. A spec that reaches no shared constant is unaffected: it stays `selected`,
  with an empty list when nothing else is reached. `typeof C`, import and export clauses and
  comments (TSA-V0-011) are not reads, nor is a declaration that shadows the name (`const|let|var|
  function|class C`, `C =>`, or a simple `function`, arrow or `catch` parameter `C`); reads of a
  shadowing binding are attributed to the imported constant, which can only over-select.
- `TSA-V0-007`: (standing auto-accept, 2026-10-11; V1-1698 / GitHub #1082) A reached read of a
  shared constant that is not a literal property (a computed `C[k]`, a whole-object use such as
  `use(C)` or `Object.keys(C)`, or a namespace used other than as `NS.NAME`) MUST return mode `all`
  with reason `shared-constant-key-unresolved`, naming the file and line, whenever a setup file not
  already selected reads the same constant (for a namespace, any shared constant); it MUST NOT fail
  open to `selected`. A reached read the selector cannot attribute to one constant MUST return ALL
  the same way whenever an unselected setup file reads any shared constant: a namespace used as a
  value, a literal `require("m")` or `import("m")` used other than as `.NAME` (CommonJS and dynamic
  import bindings are not resolved), a re-export chain past 8 hops or 4096 export-table lookups, a
  re-export through an unreadable or unresolved module, or a template literal whose `${}`
  substitutions nest deeper than 8 (amended 2026-10-18 after the PR #1096 review). While any
  shared-constant read is reached, an unselected setup file holding such a read itself MUST return
  ALL with `shared-constant-key-unresolved` naming that setup file and line, and an unreadable setup
  file adds `source-unreadable`. Reaching an analysis bound is never the same as "not a constant".

### Lint (V1-1589)

- `TSA-V0-012`: (standing auto-accept, 2026-10-11; V1-1589) `corvint tests lint --base REV [--rev
  REV]` MUST lint the test and page-object files at the revision and, by default, report only
  findings whose line range touches a line added or changed since `--base`, or strictly contains a
  deletion (`scope: changed-lines`); `--all-lines` reads every test and page-object file at the
  revision, changed or not, and reports every finding (`scope: all-lines`) (amended 2026-10-17 after
  the PR #1054 review). An unknown revision refuses with `tests-invalid-revision`.
- `TSA-V0-013`: (standing auto-accept, 2026-10-11; V1-1589) The rule set MUST be closed, with stable
  IDs: `no-assertion` (a test with no assertion call), `weak-assertion-only` (every assertion is a
  visibility or URL matcher), `conditional-assertion` (an assertion inside `if`, `switch`, a ternary
  or `catch`), `swallowed-wait-error` (a wait whose error is caught and dropped), `fixed-wait`
  (`waitForTimeout`, or `setTimeout`, `sleep`, `delay`, `pause` or `wait` with a number),
  `tautological-assertion` (`expect` of a literal, or `toBe`/`toEqual`/`toStrictEqual` of the subject
  itself), `brittle-selector` (an XPath, positional or CSS-class selector, or `.nth(N)`), and `skip-only-without-reason` (`.only`, or
  a skip/fixme without a reason string). The advisory rules `one-shot-ui-state-assertion`
  (`TSA-V0-029`) and `exact-text-cross-variant` (`TSA-V0-031`) close the set (amended 2026-10-18;
  V1-1769).
- `TSA-V0-014`: (standing auto-accept, 2026-10-11; V1-1589) Every rule MUST have a positive fixture
  that fires it and a negative fixture that does not.
- `TSA-V0-015`: (standing auto-accept, 2026-10-11; V1-1589) A comment `corvint-lint-allow RULE:
  REASON` on the finding's line, or alone on the line above it, MUST suppress exactly RULE there and
  nothing else. One without a reason (`missing-reason`) or naming an unknown rule (`unknown-rule`)
  suppresses nothing. Every in-scope or applied suppression MUST be echoed with whether it was used.
- `TSA-V0-016`: (standing auto-accept, 2026-10-11; V1-1589) `--json` output (schema
  `corvint-tests-lint/0`) MUST be byte-identical across runs over the same inputs. Exit status MUST be
  0 with no findings, 1 with findings and 2 on any refusal; advisory findings (`TSA-V0-032`) do not
  count as findings for the exit status (amended 2026-10-18; V1-1769).

### Duplicates (V1-1592)

- `TSA-V0-020`: (standing auto-accept, 2026-10-11; V1-1592) `corvint tests duplicates [--ref REV]`
  MUST normalise each non-skipped test's `expect` chains (canonical token rendering, so quoting and
  line breaks do not matter, keeping a regular expression's source and a template's substitution
  expressions so different patterns or expressions never render alike), drop a line held by more than `--max-line-tests` tests (default 5,
  range 2..100), and report pairs of tests in different files that share at least two remaining lines
  with a Jaccard score at least `--threshold` (default 0.5, range (0,1]). A test is identified as
  `path :: describe > title`, with ` [n]` for a repeat. Pairs are sorted by score, then identifiers.
  Decisions are applied to every pair first; then at most 1000 unresolved and 1000 resolved pairs are
  listed (`truncated`), while `unresolvedCount` and `resolvedCount` count them all (amended
  2026-10-17 after the PR #1054 review).
- `TSA-V0-021`: (standing auto-accept, 2026-10-11; V1-1592) With `--gate TICKET --base REV` only pairs
  where at least one test was added or changed since the base (an added or changed line in its span,
  or a deletion strictly inside it) MUST be considered, the changed tests are listed, and the command
  exits 1 while `unresolvedCount` is above zero, including pairs beyond the listed bound; report mode
  exits 0 (amended 2026-10-17 after the PR #1054 review).
- `TSA-V0-022`: (standing auto-accept, 2026-10-11; V1-1592) The project-owned, committed file
  `.corvint/test-duplicates.json` (schema `corvint-test-duplicate-decisions/0`, at most 1 MiB and
  10 000 decisions, read at the analysed revision) MUST resolve a pair by a `distinct-from` decision
  with a non-empty `reason` or a `covered-by` decision with a non-empty `obligation`. A decision
  without its reason or obligation, of an unknown kind, or naming an invalid pair resolves nothing
  and is listed under `ignoredDecisions` with its problem. A malformed file (unknown fields, wrong
  schema, trailing data) refuses with `tests-decisions-invalid`.
- `TSA-V0-023`: (standing auto-accept, 2026-10-11; V1-1592) Every report MUST state that pairs are
  candidates for review, not verdicts.

### Advisory lint rules (V1-1769)

- `TSA-V0-029`: (standing auto-accept, 2026-10-11; V1-1769) The advisory rule
  `one-shot-ui-state-assertion` MUST flag `expect(await X)` (including `expect.soft`) where X
  contains a one-shot Playwright UI-state read (`isVisible`, `isHidden`, `isEnabled`, `isDisabled`,
  `isChecked`, `isEditable`, `count`, `textContent`, `innerText`, `allTextContents`,
  `allInnerTexts`, `inputValue` or `getAttribute` called as a member) either directly or returned
  through a helper resolved under `TSA-V0-030`. A returned local `const`/`let`/`var` is followed,
  with or without a type annotation, and a returned expression is read through enclosing
  parentheses and `await`. A `return` inside a function nested in a helper is not that helper's
  return. A standalone call whose name matches a read (`isVisible(page)`) is a helper call, not a
  read. The message MUST name the read, its location, the helper chain and the web-first matcher
  that retries it. The finding anchors on the `expect` line and spans the whole assertion, so a
  change on any of its lines is in the changed-line scope. It MUST NOT fire on web-first matchers
  over a locator (`await expect(locator).toBeVisible()`, `toHaveCount`), on `expect.poll`, inside an
  `expect(...).toPass()` callback, or on a subject without `await`. Helper following is bounded at
  `MaxHelperDepth` = 3 hops: the helper the assertion calls is hop 1 (amended 2026-10-18 after the
  PR #1119 review).
- `TSA-V0-030`: (standing auto-accept, 2026-10-11; V1-1769) A helper MUST resolve by name to
  function declarations, class or object methods and arrow-function or function-expression
  properties and variables in the asserting file and in the modules it imports through relative
  specifiers (`./`, `../`; a `.js` specifier also names the `.ts`/`.tsx` file, and a directory its
  `index` module), read at the revision with committed objects only, transitively up to 3 import
  levels. The linted files and every level of imported modules draw on one budget of
  `TSA-V0-010`'s 5000 files and 64 MiB, and an imported module the lexer cannot fully place is
  listed in `uncertain`. In both advisory rules, a helper with no visible definition, or one that
  would need a fourth hop, MUST yield an advisory finding with `resolution: UNKNOWN` and the reason,
  never silence. Playwright, Promise and built-in call names with no visible definition of the same
  name are not helpers and are neither followed nor reported (amended 2026-10-18 after the PR #1119
  review).
- `TSA-V0-031`: (standing auto-accept, 2026-10-11; V1-1769) `corvint tests lint --runs
  REPORT.json` (repeatable, at most 64) MUST read Playwright JSON reports and keep those whose
  declared `corvintRunIdentity.testRevision` names the linted revision (equal, or a prefix of at
  least 7 characters). A linted test whose literal title path, joined to its file through the
  declared `testRootPath` (or a unique path suffix when undeclared), is `expected` in one project and
  `unexpected` in another is a split; the advisory rule `exact-text-cross-variant` MUST then name
  each `exact: true` locator (`getByText`, `getByRole`, `getByLabel`, `getByPlaceholder`,
  `getByTitle`, `getByAltText`) in the test body, and every distinct one reachable through the
  helpers it calls resolved under `TSA-V0-030`, as a likely cause together with one passing and one
  failing project, in one finding per line that spans the call. The `exact` option is read from an
  object literal with a bare or quoted key, the last `exact` or spread winning. Only the literal
  `true` is exact. A non-literal value, a spread, a shorthand or a non-literal options argument
  yields `resolution: UNKNOWN` with the source (amended 2026-10-18 after the PR #1119 review). The report's
  `crossVariant` member MUST list the projects, the run count and every split with whether an
  exact-text locator was named. An unreadable report refuses with `tests-source-unavailable`.
- `TSA-V0-032`: (standing auto-accept, 2026-10-11; V1-1769) Without split-capable evidence
  `crossVariant.status` MUST be `NOT_PRODUCED` with one reason: `no-run-evidence` (no `--runs`),
  `revision-not-declared` (no report declares a test revision), `no-run-at-revision` (none declares
  this one) or `single-project` (the kept runs hold fewer than two projects); the exact-text rule
  then reports nothing. Findings of both advisory rules carry `advisory: true`, are printed as
  `RULE (advisory)` or `RULE (advisory, UNKNOWN)`, are suppressible under `TSA-V0-015`, and never
  change the exit status.

### Actor authorization (V1-1770)

Ticket V1-1770, GitHub #1115. A test that logs in as a fixture actor whose role does not hold the
ability its request needs fails with a 403 that reads like an application bug. This slice finds
that mismatch statically, before a run, for a literal subset of one stack: JS/TS fixtures and tests
on the client side, and Rails routes, controllers and a CanCanCan-style `Ability` class on the
server side. It is library-only (`testsource.ActorAuthorization`); it reads no declared role table,
because the literal Ruby forms below are parsed directly. It lives in this spec rather than the
test-validity or application-map specs because it is a static read of committed test source, like
lint and duplicates, and it projects no runtime receipt.

- `TSA-V0-033`: (standing auto-accept, 2026-10-11; V1-1770 / GitHub #1115) Actor-role resolution
  MUST bind a request to a fixture actor only through literal forms. A fixture is an exported
  `function NAME(` or `const NAME = (...) =>` builder in a non-test JS/TS file whose body calls a
  user factory: a call whose callee chain names `user`, or whose arguments carry a `role:` key. The
  role is that key's string literal, or else the one `role = '...'` default among the factory's
  definitions whose whole value is a single string literal. A spread or computed property after
  the `role:` key, or anywhere in a call without one, can set the role at run time and makes it
  `role-not-literal`. The fixture's single top-level `return { ... }` maps each key to the seeded
  user its value names (the user's variable, else an identifier passed to exactly one factory call).
  In a test, `const V = [await] FIXTURE(...)` binds V, and the latest call whose name contains
  `login` or `signin` before the request, in the same test or before it outside every test, sets
  the actor from its `V.KEY` argument.
- `TSA-V0-034`: (standing auto-accept, 2026-10-11; V1-1770 / GitHub #1115) Request-to-ability
  resolution MUST read a request as `X.get|post|put|patch|delete('/path')` inside a test, where a
  template substitution may fill a whole path segment and then matches only a dynamic route
  segment. A call on a receiver named like `request` whose path is anything else (a variable, a
  concatenation, a path not starting with `/`) is still a request, reported `path-not-literal`. The first matching route in `config/routes.rb` is used, from these literal forms:
  `Rails.application.routes.draw do`, `namespace :x do`, `resources :x` with `only:`/`except:`
  symbol lists and nesting, `<verb> 'path', to: 'c#a'` (or `=>`) and `root`. The ability is the one
  literal `authorize! :action, Constant` in that action of `app/controllers/<controller>_controller.rb`.
  A check inside a nested block or a `rescue`/`else`/`when` clause, or after a statement that can
  leave the action early (`return`, `raise`, `fail`, `throw`), is conditional (`check-conditional`).
- `TSA-V0-035`: (standing auto-accept, 2026-10-11; V1-1770 / GitHub #1115) Rule-set evaluation MUST
  read `app/models/ability.rb` as a `class Ability` whose `initialize` calls `send(user.role)` (or
  `public_send`, optionally `.to_sym`), where each argument-free method is the rule set of the role
  it names, and a bare call to another rule set inside it is inheritance. A role holds an ability
  when an unconditional `can` in its inheritance closure or in `initialize` matches it: the action
  is equal, `:manage`, or a CanCanCan default alias (`read` for index/show, `create` for new,
  `update` for edit), and the subject is equal or `:all`, and the whole closure parsed. A role is
  denied only when its whole closure parsed and nothing matches. A statement that names `can` or
  `cannot` without starting with it (`return can ...`, `x ||= can ...`) and a control keyword such as
  `return` are unparsed, and any unparsed statement in the closure makes a grant or a denial UNKNOWN
  (`statement-unparsed`), because it could grant or revoke.
- `TSA-V0-036`: (standing auto-accept, 2026-10-11; V1-1770 / GitHub #1115) A denied request MUST be
  reported with status `UNAUTHORIZED` and code `ACTOR-UNAUTHORIZED`. The report names the test file,
  line and title, the method and path, the actor (fixture, key, role, where the role was seeded),
  the route and the check with their file:line, and `grantedBy`, the granting rule file:line of each
  lowest sufficient role. A lowest sufficient role holds the ability and inherits no other role
  that holds it. The report also lists `alternatives`, every other fixture actor whose resolved
  role holds the ability.
- `TSA-V0-037`: (standing auto-accept, 2026-10-11; V1-1770 / GitHub #1115) The report (schema
  `corvint-tests-actor-authorization/0`) MUST read committed objects only at one revision, within
  the package's `MaxFiles` and `MaxFileBytes` bounds. It MUST list the closed set of supported input
  forms and every resolved fixture actor. It MUST be deterministic over the same inputs. Its verdict
  is `ACTOR_UNAUTHORIZED` when any request is unauthorized, else `UNKNOWN` when any is unknown, else
  `CLEAN`. An `AUTHORIZED` or `EXEMPT` request does not change the verdict.
- `TSA-V0-038`: (standing auto-accept, 2026-10-11; V1-1770 / GitHub #1115) Every link that does not
  resolve MUST make the request `UNKNOWN`, never authorized or unauthorized. The reason names the
  first failing link, with a file:line detail, from this closed set:
  - actor links: `actor-unbound`, `actor-not-literal`, `actor-key-not-user`, `actor-key-ambiguous`,
    `fixture-return-not-literal`, `fixture-ambiguous`, `role-not-literal`, `role-ambiguous`,
    `role-default-unresolved`;
  - the request path: `path-not-literal`;
  - route links: `routes-unavailable`, `route-not-found`, `route-unparsed`;
  - controller links: `controller-unavailable`, `action-not-found`, `check-not-found`,
    `check-not-literal`, `check-implicit`, `check-multiple`, `check-conditional`;
  - ability links: `ability-unavailable`, `role-dispatch-not-literal`, `role-undefined` (also an
    empty role), `inheritance-cycle`, `rule-conditional`, `rule-cannot`, `statement-unparsed`.

  Every request MUST carry one of the four statuses.
- `TSA-V0-039`: (standing auto-accept, 2026-10-11; V1-1770 / GitHub #1115) A test that asserts a
  401, 403 or 422 status with `toBe`, `toEqual`, `toStrictEqual` or `toHaveStatus` on a numeric
  literal, not negated with `.not`, on a subject that is a response status (`x.status()`, `x.status`,
  `x.statusCode`, or the response itself for `toHaveStatus`), MUST have every request reported `EXEMPT` with reason `asserts-rejection` and the
  assertion's file:line, never `ACTOR-UNAUTHORIZED`.

Non-goals (V1-1770): authorization libraries other than the CanCanCan-style `Ability` (Pundit,
Action Policy, hand-written guards), runtime policy (database-held roles, conditions evaluated
against records), requests sent through `fetch()` or a page navigation, `load_and_authorize_resource`,
a role dispatch through `if`/`case`, and a CLI command. Each of these is UNKNOWN or out of scope,
never authorized. Failure modes: the reading is heuristic. A login helper not named `*login*` or
`*signin*` leaves the actor unbound, and a fixture binding is matched by name across tests in one
file. A route matched before an unparsed `routes.rb` statement that would shadow it is still used.
A `cannot` matching the ability makes the result UNKNOWN rather than modelling rule order. Acceptance
evidence: the issue's assistant/manager repository as a synthetic Git fixture. The assistant's
`PUT /api/widgets/:id` is `ACTOR-UNAUTHORIZED` with `grantedBy` manager at line 14 of the fixture Ability class,
lowest role `manager` (admin excluded because it inherits manager), and alternative `beta.manager`.
The manager pair is `AUTHORIZED`, the 403-asserting test is `EXEMPT`, and each closed UNKNOWN reason
has its own case. No live qualification on a real suite was run (NOT_RUN). Rollback removes
`internal/testsource/actor_auth*.go` and this section; nothing is persisted.

### Static test-validity rules (V1-1783, V1-1784)

Tickets V1-1783 (GitHub #1123) and V1-1784 (GitHub #1124). Two rules read committed test source
and report a test whose passing or skipped result proves nothing. The finding vocabulary
(`testvalidity.SourceFinding`, the rule IDs and severities) and the one place a finding reaches a
projection (`testvalidity.ApplySourceFindings`) live in `internal/testvalidity`. The readers live in
`internal/testsource`, because that package already holds the JS lexer, the declaration model and
the Ruby line helpers, and `internal/testvalidity` cannot import them without a cycle. The entry
point is the library function `testsource.ValidityFindings(name, text)`. No CLI, MCP or preflight
command calls it yet.

- `TSA-V0-045`: (standing auto-accept, 2026-10-11; V1-1783 / GitHub #1123) In a JS/TS test file
  (`IsTestFile`), rule `ISOLATED-IN-SERIAL` MUST report every test that is isolated and sits under
  a serial ancestor.
  - A test is isolated when its literal title holds the word `@isolated`, or when its details
    argument or an enclosing describe's details argument has a `tag` value (a string or an array
    of strings) holding the literal `'@isolated'`, or an annotation whose `type` is the literal
    `'isolated'`. Other details text, such as an annotation's `description`, is not a tag. An
    object key may be bare or a quoted string literal (`mode:` or `'mode':`).
  - A serial ancestor is any of these:
    - a `test.describe.serial(...)` declaration enclosing the test;
    - an enclosing describe whose own body (not a nested describe's body) calls
      `test.describe.configure({ mode: 'serial' })`;
    - such a configure call outside every describe, which makes the whole file serial.
  - The finding names the test's `path:line` and title. Its cause is the outermost serial
    ancestor's `path:line`: the describe's line, or the line of the file-level configure call.
  - Its message says to move the test out of the serial suite, or to drop serial mode when the
    suite's tests are independent.
- `TSA-V0-046`: (standing auto-accept, 2026-10-11; V1-1783 / GitHub #1123) `ISOLATED-IN-SERIAL`
  MUST NOT report:
  - a test that has no isolation annotation, even under a serial suite;
  - an isolated test whose ancestors are all non-serial (no configure call, or a `mode` of
    `'parallel'` or `'default'`);
  - an isolated test in a sibling of a serial describe.
  When a configure call's `mode` is not a string literal, the report is uncertain with reason
  `serial-mode-not-literal`, and the call does not make its scope serial. A file the lexer cannot
  place fully is uncertain with reason `source-unparsed`, and its findings may be incomplete.
- `TSA-V0-047`: (standing auto-accept, 2026-10-11; V1-1783 / GitHub #1123) An `ISOLATED-IN-SERIAL`
  finding MUST have severity `advisory`, and MUST NOT change any projection axis. A skipped run is
  already `SKIPPED` on the execution axis, and a passing run of the test still witnesses it.
- `TSA-V0-048`: (standing auto-accept, 2026-10-11; V1-1784 / GitHub #1124) The RSpec reader MUST
  read a `_spec.rb` file as logical lines (`rubyLines`) and nothing more than:
  - example-group nesting: `describe`, `context`, `feature`, `shared_examples`, `shared_context`
    and `it_behaves_like` do-blocks, with an optional `RSpec.` prefix and an `x`/`f` prefix;
  - each group's declarations: `subject`, `subject(:name)`, `let(:name)`, `let!(:name)` and
    `subject!`;
  - each group's memoizing hooks:
    - `before`, `prepend_before` and `append_before` with no scope or with `:each` / `:example`;
    - a `let!`, which invokes its own name;
    - a `subject!`, which invokes `subject`;
    A `let!` or `subject!` hook also invokes its block's names, but only while the helper it
    calls is not redeclared in a group nested between the hook and the example, because the
    redeclared body runs instead.
  - each example's lines (`it`, `specify`, `example`, `scenario`, `its`, `focus`, with an `x`/`f`
    prefix).
  These blocks are opaque: `before(:all|:context|:suite)`, `around`, the `after` hooks and `def`.
  Every other `do`/`if`/`unless`/`class`/`module` block is transparent. A hook invokes the bare
  identifiers in its block. It does not invoke these:
  - a word inside a string literal, quoted or a bracket-delimited percent literal (`%q(...)`,
    `%Q[...]`, `%w{...}`, `%i<...>`, `%(...)`);
  - a method on a receiver (`.x`);
  - a symbol (`:x`);
  - an instance or global variable;
  - a hash key (`x:`);
  - a Ruby keyword;
  - a block parameter;
  - a local assigned in the block.
- `TSA-V0-049`: (standing auto-accept, 2026-10-11; V1-1784 / GitHub #1124) Rule
  `MEMOIZED-BLOCK-EXPECTATION` MUST report, with severity `invalid`, every block expectation
  `expect { NAME }`, or a multi-line `expect do` / `NAME` / `end`, whose whole block is one bare
  NAME, when both of these hold:
  - NAME resolves from the example's group chain (innermost declaration first) to a let, or to
    the subject. A named subject `subject(:x)` is the let `x`, and `subject` resolves to it while
    it is the innermost subject. An anonymous or undeclared `subject` is its own helper. Distinct
    named subjects are distinct helpers;
  - a memoizing hook of the example's own group or an enclosing group invokes a name that
    resolves to the same helper.
  The finding names the example's `path:line` and title and the expectation's `path:line`. Its
  cause is the hook's `path:line`: the innermost group's hook first, then the lowest line.
  These are not reported:
  - a hook in a sibling group;
  - a block that is not one bare name (`expect { described_class.new.perform }`);
  - a NAME that no group in the chain declares, other than `subject`;
  - a NAME no hook on the chain invokes.
- `TSA-V0-050`: (standing auto-accept, 2026-10-11; V1-1784 / GitHub #1124) The RSpec reader MUST
  abstain, reporting uncertain with no findings, in these cases:
  - a block does not close, or an `end` has no opener: reason `rspec-block-unbalanced`;
  - nesting exceeds 64 blocks anywhere, including inside an example or hook body: reason
    `rspec-depth-exceeded`.
  Either rule's input MUST be at most `MaxFileBytes`; a larger file is uncertain with reason
  `source-too-large` and is not read.
- `TSA-V0-051`: (standing auto-accept, 2026-10-11; V1-1784 / GitHub #1124)
  `testvalidity.ApplySourceFindings` MUST be the verdict wiring for the caller-selected findings of
  one test.
  - Any `invalid` finding makes the hygiene axis `INELIGIBLE`, with reason
    `memoized-block-expectation` for this rule, or the rule ID for any other invalid rule.
  - Each invalid finding's location and cause are appended to the hygiene anchors.
  - When hygiene was already `INELIGIBLE`, its reason is kept.
  - The other four axes and advisory findings are left unchanged.
  A proof binding that requires `ELIGIBLE` hygiene, such as the test-consolidation witness check
  (`WitnessIneligible`), therefore refuses the example.
- `TSA-V0-052`: (standing auto-accept, 2026-10-11; V1-1783, V1-1784) `ValidityFindings` MUST read
  only the given text. It reads a JS/TS test file for TSA-V0-045 and a `_spec.rb` file for
  TSA-V0-049, and returns no finding for any other file. It MUST be deterministic over the same
  input. It never reports a finding it could not place.

Ticket V1-1824 (GitHub #1140) adds a third rule to the same RSpec reader: a spec helper that
takes a block and hands an assertion a block that only references it, or never calls it, makes
every example that passes it a block vacuous. TSA-V0-048 still treats a `def` as opaque for
`MEMOIZED-BLOCK-EXPECTATION`; this rule reads the `def` itself.

- `TSA-V0-053`: (standing auto-accept, 2026-10-11; V1-1824 / GitHub #1140) The RSpec reader MUST
  read as a helper each `def` whose innermost enclosing block is an example group, or that is
  outside every block. It is a block helper when its parameter list ends with a named block
  parameter (`&block`), or when it has no block parameter and its body calls `block_given?`.
  Its body is read outside string literals and comments. In the body:
  - a call of the block is `yield`, `super` (which forwards the block), `binding`, or any use of
    the block parameter other than a reference-only use;
  - a reference-only use is the parameter, optionally with `.itself`, as the whole body of a
    one-line literal block (`{ block }`, `{ |x| block }`), or as a whole line inside a multi-line
    `do` or brace block opened in the body. A bare reference at the def's own level, or inside an
    `if`, `unless`, `case`, `while` or `begin` there, may be the return value, so it is a call;
  - an assertion is a call named `assert`, `assert_*`, `refute`, `refute_*`, `expect` or
    `perform_enqueued_jobs`, with parenthesised arguments, or unparenthesised arguments before a
    `do` block.
- `TSA-V0-054`: (standing auto-accept, 2026-10-11; V1-1824 / GitHub #1140) Rule `BLOCK-NOT-CALLED`
  MUST report, with severity `invalid`, a block helper when either holds:
  - an assertion in its body takes a literal block (`{ ... }` on one line, `do` ... `end`, or a
    brace block over three lines) whose whole body is reference-only uses of the named block
    parameter. The finding's location is the first such assertion line;
  - the body has no call of the block. The finding's location is the def line.
  The helper's finding names the def's `path:line` as both test and cause, and the helper's name
  as title. Each example in the same file that calls the helper with a block (`name { }`,
  `name(args) { }`, `name args do`, or a `&` argument) inherits a finding of the same rule and
  severity. That finding names the example's `path:line` and title, the call's `path:line` as
  location and the def's `path:line` as cause. The call resolves along the example's group chain
  to the innermost group that defines that name, and to the last such def in that group. A file
  level def is the outermost. The helper's finding comes first, then its callers in source order.
- `TSA-V0-055`: (standing auto-accept, 2026-10-11; V1-1824 / GitHub #1140) `BLOCK-NOT-CALLED` MUST
  NOT report:
  - a block forwarded with `&block` (`assert_no_enqueued_jobs(&block)`, `instance_exec(&block)`,
    `instance_eval(&block)`);
  - `block.call`, `yield`, `super`, or any other use it does not classify as reference-only;
  - a helper that calls the block on every path and hands no assertion a reference-only block;
  - a helper with an anonymous `&` or a `...` parameter, a nested `def`, a string interpolation
    or a heredoc, which is not read;
  - a `block_given?` helper whose body calls `raise`, `fail`, `enum_for` or `to_enum`;
  - a `def self.` method, or a `def` inside a class, module or any other non-group block;
  - a def with no block parameter and no `block_given?`;
  - an example that calls the helper without a block, or whose chain resolves the name to
    another def.
  When it cannot place the file, it abstains with the TSA-V0-050 reasons and bounds, so a block
  helper never yields a finding from an uncertain report.
- `TSA-V0-056`: (standing auto-accept, 2026-10-11; V1-1824 / GitHub #1140)
  `testvalidity.ApplySourceFindings` MUST make the hygiene axis `INELIGIBLE` with reason
  `block-not-called` for an invalid `BLOCK-NOT-CALLED` finding. The TSA-V0-051 anchor, existing
  reason and other-axis rules apply unchanged.

Non-goals (V1-1783, V1-1784):
- the "creates its own fixture data and session" heuristic for isolation, and runtime
  annotations pushed from a test body;
- the optional runner summary that tells "skipped because an earlier serial test failed" apart
  from a plain skip (#1123, a residual);
- expectations whose target lines had zero coverage in the example's run (#1124, a residual that
  needs per-example coverage input);
- transitive let chains (a hook invoking a let whose body invokes the subject);
- an invocation earlier in the example body itself;
- `before(:all)` state;
- shared examples included from another file;
- a general Ruby parser;
- a CLI, MCP or preflight surface for these findings.

Non-goals (V1-1824): path-sensitive reading of a helper that calls its block on some paths but not
others; a helper defined in another file (`spec/support`), a module included from it, or a
`define_method` helper; a hook, let or helper that calls a block helper (only examples inherit the
finding); and any assertion name outside the TSA-V0-053 list.

Failure modes:
- The Ruby reading is line-based. A trailing `if ... end` on one line, an endless `def`, or a
  string literal holding a brace or an interpolated quote can misplace a block. An imbalance makes
  the file abstain. A balanced misplacement can attribute a declaration to the wrong group.
- A ternary's middle branch reads like a hash key, so it is not counted as an invocation.
- A Playwright isolation marker held in a variable, or a configure call on a renamed `test`
  import, is not seen.
- `BLOCK-NOT-CALLED` matches the block helper by name. A local variable with the helper's name
  followed by a brace, or a def in a module included elsewhere in the file, can be misread. Any
  unclassified use of the parameter counts as a call, so the rule errs towards silence.

Acceptance evidence: synthetic fixtures only.
- The issue's alpha/beta serial repro.
- A `test.describe.serial` suite with tag, title and annotation forms.
- A file-level configure with a describe-level annotation.
- Negatives: not isolated under serial, isolated under a parallel or unconfigured describe, and a
  serial sibling.
- A computed mode, which is uncertain.
- An RSpec file covering subject, named subject, let, let! and multi-line `expect do`.
- RSpec negatives: a sibling-group before, `described_class.new.perform`, no memoizing ancestor,
  `before(:all)` and a shadowing local.
- RSpec abstention for an unbalanced block, a stray end, depth and size.
- The #1140 helper and its two calling examples; one-line brace, `.itself`, unparenthesised
  arguments, a three-line brace block, a vacuous branch beside a calling one, a helper that never
  calls its block, a `block_given?` helper that never yields, and a file-level helper.
- `BLOCK-NOT-CALLED` negatives: `&block` forwarded, `block.call`, `yield`, `instance_exec`,
  `instance_eval`, every path calling, `super`, a returned bare block, a def in a module or class,
  `&` and `...` parameters, a call without a block and a nested override.
- A test-consolidation witness refused as `WitnessIneligible` once its RSpec finding is applied,
  while an advisory finding leaves a passing witness reusable.

No live qualification on a real suite was run (NOT_RUN). Rollback removes
`internal/testvalidity/source_rules.go`, `internal/testsource/validity*.go`, their tests and this
section. `BLOCK-NOT-CALLED` alone rolls back by removing `internal/testsource/validity_rspec_block.go`,
its test, the helper capture in `validity_rspec.go`, the rule ID and reason, and TSA-V0-053..056. Nothing is persisted, and no existing projection, report or wire contract changes.

## Refusals

Every command exits 2 with one JSON object `{"code", "error", "ok": false}` on stderr and nothing on
stdout when it refuses: `tests-invalid-arguments` (usage), `tests-invalid-revision` (a revision that
is not a commit), `tests-invalid-option` (a threshold or cutoff out of range, or `--gate` without
`--base`), `tests-source-unavailable` (a Git or index read failed), `tests-bound-exceeded`,
`tests-decisions-invalid` and `tests-output-unavailable` (stdout could not be written).

## RSpec hygiene and strength in test-validity (V1-1798, V1-1787)

V1-1798 (GitHub #1128 and #1130, slice S5) puts RSpec example hygiene and strength facts into the
test-validity projection. V1-1787 asks for a production command path for the TSA-V0-045 and
TSA-V0-049 findings. Both are agent-drafted and accepted under the owner's standing auto-accept
(2026-10-11). This section supersedes two earlier non-goals of the static test-validity rules:
"a CLI, MCP or preflight surface for these findings" now has the surfaces below, and an example
declared in a shared example group is now flagged. A preflight surface is still not built.

- `TSA-V0-062`: (standing auto-accept, 2026-10-11; V1-1798 / GitHub #1128, #1130)
  `testsource.RSpecExampleSources(name, text)` MUST read one `_spec.rb` file's examples with the
  TSA-V0-048 reader. It returns one `testvalidity.ExampleSource` per example, keyed by the
  example's declaring line, with `read` = `located`, `expectations` and `findings`.
  - `findings` are the `ValidityFindings` that name the example, then the TSA-V0-063..066
    findings in that rule order, at most one per rule.
  - The reader MUST also treat the `skip` and `pending` declarators as examples.
  - It MUST abstain with no examples for the TSA-V0-050 reasons and `source-too-large`, and with
    `not-an-rspec-file` for any other file.
- `TSA-V0-063`: (standing auto-accept, 2026-10-11; V1-1798 / GitHub #1128) An example MUST get an
  `invalid` `RSPEC-SKIPPED` finding when it is skipped or pending. Its location is the example's
  line; its cause is the first triggering line, the example's own lines before its groups,
  innermost group first. The triggers are:
  - an `xit`, `xspecify`, `xexample`, `xscenario`, `skip` or `pending` declarator;
  - `:skip` or `:pending` symbol metadata, or a `skip:` or `pending:` key whose value is not
    `false` or `nil`, on the declaring line outside string literals and before the block opens;
  - an unconditional `skip` or `pending` statement directly in the example body: not in a nested
    block and with no trailing `if` or `unless`;
  - an ancestor group declared with `xdescribe` or `xcontext`, or carrying that metadata.
- `TSA-V0-064`: (standing auto-accept, 2026-10-11; V1-1798 / GitHub #1128) An example MUST get an
  `invalid` `RSPEC-FOCUSED` finding when it is focused, with its location and cause chosen
  as for `RSPEC-SKIPPED`. The triggers are:
  - an `fit`, `fspecify`, `fexample`, `fscenario` or `focus` declarator;
  - `:focus` metadata or a `focus:` key that is not `false` or `nil`;
  - an ancestor group declared with `fdescribe` or `fcontext`, or carrying that metadata.
- `TSA-V0-065`: (standing auto-accept, 2026-10-11; V1-1798 / GitHub #1128) An example declared
  inside a `shared_examples`, `shared_examples_for` or `shared_context` group MUST get an `invalid`
  `SHARED-EXAMPLE-ONLY` finding. Its cause is the innermost such group. Its assertion runs only
  through an including group that the cited `file:line` does not name, so it is not proof for the
  citation.
- `TSA-V0-066`: (standing auto-accept, 2026-10-11; V1-1798 / GitHub #1130) An example MUST get an
  `advisory` `DESCRIBED-CLASS-STUBBED` finding when `described_class` is stubbed. This means
  `allow(described_class)`, `expect(described_class)` or their `_any_instance_of` forms followed
  by `.to`, `.not_to` or `.to_not` and `receive*` or `have_received`, outside string literals.
  - A stub in the example body has the stub line as its location and the example as its cause.
  - Otherwise the first stub in a `before(:each)`, `let!` or `subject!` hook on the group chain
    (innermost group first) is reported, caused by the hook.
  - The finding does not change hygiene.
- `TSA-V0-067`: (standing auto-accept, 2026-10-11; V1-1798 / GitHub #1130) `expectations` MUST
  count the expectations in the example's own lines, outside string literals. An expectation is
  `expect(`, `expect {`, `expect do`, `is_expected`, `should`, `should_not` or
  `expect_any_instance_of`. Hooks, lets and helper methods are not counted. Their distinct
  `path:line` locations are the strength anchors.
- `TSA-V0-068`: (standing auto-accept, 2026-10-11; V1-1798, V1-1787)
  `testvaliditydoc.DiscoverWithSources(root, reader)` MUST apply example sources to discovered
  Ruby evidence. `corvint test-validity --discover` and the MCP `corvint.test_validity` tool's `discover`
  argument MUST both pass `testsource.RSpecExampleSources`, so they keep emitting the same bytes.
  - For each retained example with a bound `specBlob`, the spec file is read once per file
    through the confined, no-follow, `MaxInputBytes` receipt reader. It is used only when its Git
    blob ID equals the retained one.
  - A file that is not bound makes hygiene `ABSTAINED` `rspec-source-not-bound`.
  - A reader that abstains makes hygiene `ABSTAINED` with its reason.
  - An example the reader did not place at its line is `ABSTAINED` `rspec-example-not-located`.
  - A placed example is `ELIGIBLE` `rspec-example-source-read`, then takes its findings through
    `ApplySourceFindings` (TSA-V0-051). An `INELIGIBLE` axis is never relaxed.
  - Each such test carries a `sourceRules` member `{read, expectations, findings}`.
  - An example with no bound blob keeps its projection and has no member.
  - Go documents, `--receipt` documents, and `Discover(root)` without a reader are unchanged.
    JS/TS documents take TSA-V0-082 (amended by V1-1858).
- `TSA-V0-069`: (standing auto-accept, 2026-10-11; V1-1798 / GitHub #1130) Strength MUST stay
  `NOT_MEASURED`, and a strength a mutation run measured is kept. Only a mutation run decides
  `KILLED` or `SURVIVED`. Its reason is the first that applies:
  - `described-class-stubbed`, anchored at the stub lines;
  - `no-expectation-in-example-body`, anchored at the example;
  - `example-body-expectations`, anchored at the expectation lines.
- `TSA-V0-070`: (standing auto-accept, 2026-10-11; V1-1798 / GitHub #1128)
  `rubytestprovider.ProjectExample` MUST project an RSpec `pending` example as hygiene
  `INELIGIBLE` `rspec-example-pending` on every path, receipt and discovery alike. This amends the
  LPCV-V0-093 reading that Ruby hygiene is never claimed, for that one status; every other status
  keeps `UNSUPPORTED` `ruby-hygiene-not-produced` until TSA-V0-068 applies.
- `TSA-V0-071`: (standing auto-accept, 2026-10-11; V1-1787, V1-1798)
  `corvint tests validity --files PATH[,PATH...] [--files ...] [--rev REV]` MUST read the named
  committed files at REV (default `HEAD`).
  - Duplicate paths are read once. At most 256 files may be named, each at most `MaxFileBytes`.
  - It applies `ValidityFindings` to each file, and the TSA-V0-062 example rules to each
    `_spec.rb` file.
  - It writes `corvint-test-source-validity/0` JSON: `{schema, revision, files: [{path,
    uncertain, reason?, findings}], tests: [{test, hygiene}]}`.
  - Each test's hygiene is `ApplySourceFindings` over an `ELIGIBLE` `source-rules-read`
    baseline.
  - It exits 1 when any test is `INELIGIBLE`, which refuses that test as proof, and 0 otherwise.
  - It refuses per Refusals:
    - a missing or invalid path: `tests-invalid-arguments`;
    - a path that is not a committed regular file: `tests-source-unavailable`;
    - too many or oversized files: `tests-bound-exceeded`.
- `TSA-V0-072`: (standing auto-accept, 2026-10-11; V1-1798, V1-1787) These readings MUST be
  deterministic over the same input. They MUST read only the given text, committed Git content or
  the bound worktree spec file, and MUST write nothing and use no network.

Non-goals:
- resolving which group includes a shared example group, or following `it_behaves_like` into
  another file;
- conditional skips, `skip` inside a nested block, and metadata set through
  `config.filter_run`, `define_derived_metadata` or a variable;
- counting expectations in helper methods, hooks or custom matchers;
- stubs on a constant that aliases `described_class`;
- applying source findings to a named `--receipt` (JS/TS discovery is TSA-V0-082, V1-1858);
- an MCP tool for `tests validity`, and a preflight surface.

Failure modes:
- The reading is line-based and inherits the TSA-V0-048 failure modes. A misplaced block can
  attach an example to the wrong group and so to the wrong skip, focus or shared ancestor.
- A described_class stub written across lines, or through `allow(subject.class)`, is not seen.
- A spec edited after the run is `rspec-source-not-bound`; the reading never guesses from the
  edited text.

Acceptance evidence: synthetic fixtures only.
- An RSpec file covering every skip and focus form, `skip: false`, a conditional skip and the
  words inside a title.
- Shared examples and a nested shared context.
- described_class stubs in a body, a before hook and a let!, plus a stub on another class and one
  inside a string.
- Discovery with an injected reader over a bound and an edited spec.
- An end-to-end `test-validity --discover` run whose memoized example is refused, with identical
  MCP bytes.
- A `tests validity` run that reports ISOLATED-IN-SERIAL and MEMOIZED-BLOCK-EXPECTATION findings
  and exits 1.

No live qualification on a real suite was run (NOT_RUN). Rollback removes
`internal/testvalidity/source_rules_rspec.go`, `internal/testsource/validity_rspec_examples.go`,
`internal/testsource/validity_command.go`, `internal/testvaliditydoc/ruby_source.go`, their tests,
the `tests validity` subcommand, the `DiscoverWithSources` callers, the pending line in
`rubytestprovider.ProjectExample` and this section. Nothing is persisted.

### Traceability (V1-1798, V1-1787)

| Requirement | Code | Evidence | Status |
|---|---|---|---|
| TSA-V0-062 | `internal/testsource/validity_rspec_examples.go` | `TestRSpecExampleSourcesCarryValidityFindings`, `TestRSpecExampleSourcesAbstain`, `TestRSpecExampleSkippedAndFocused` | experimental |
| TSA-V0-063 | `internal/testsource/validity_rspec_examples.go` | `TestRSpecExampleSkippedAndFocused`, `TestApplyExampleSourceHygiene` | experimental |
| TSA-V0-064 | `internal/testsource/validity_rspec_examples.go` | `TestRSpecExampleSkippedAndFocused`, `TestApplyExampleSourceHygiene` | experimental |
| TSA-V0-065 | `internal/testsource/validity_rspec_examples.go` | `TestRSpecExampleSharedOnly`, `TestApplyExampleSourceHygiene` | experimental |
| TSA-V0-066 | `internal/testsource/validity_rspec_examples.go` | `TestRSpecExampleDescribedClassStub`, `TestApplyExampleSourceStrength` | experimental |
| TSA-V0-067 | `internal/testsource/validity_rspec_examples.go` | `TestRSpecExampleDescribedClassStub`, `TestRSpecExampleSkippedAndFocused` | experimental |
| TSA-V0-068 | `internal/testvaliditydoc/ruby_source.go`, `cmd/corvint/test_validity.go`, `internal/mcp/testvaliditybridge` | `TestDiscoverRubySourceRules`, `TestTestValidityDiscoverRSpecSourceRules`, `TestApplyExampleSourceHygiene` | experimental |
| TSA-V0-069 | `internal/testvalidity/source_rules_rspec.go` | `TestApplyExampleSourceStrength`, `TestTestValidityDiscoverRSpecSourceRules` | experimental |
| TSA-V0-070 | `internal/rubytestprovider/document.go` | `TestLPCVV0093RubyDocumentDecodesClosedAndProjects`, `TestTestValidityDiscoverRSpecSourceRules` | experimental |
| TSA-V0-071 | `internal/testsource/validity_command.go`, `cmd/corvint/tests.go` | `TestTestsValidityCLI`, `TestSourceValidityReport`, `TestSourceValidityRefusals` | experimental |
| TSA-V0-072 | `internal/testsource/validity_command.go`, `internal/testsource/validity_rspec_examples.go` | `TestSourceValidityReport`, `TestDiscoverRubySourceRules` | experimental |

## JS/TS example hygiene and strength in test-validity (V1-1858)

V1-1858 (GitHub #1159) gives Jest, Vitest and Playwright test files the per-example facts that the
RSpec readings TSA-V0-062..072 give. Before it, `corvint tests validity` on a `.spec.ts` or `.test.ts` file
listed only tests named by an ISOLATED-IN-SERIAL finding. It is agent-drafted and accepted under the
owner's standing auto-accept (2026-10-11). The reading reuses the TSA-V0-010 lexer and declaration
model (`parseSource`); it adds no second parser.

- `TSA-V0-073`: (standing auto-accept, 2026-10-11; V1-1858 / GitHub #1159)
  `testsource.JSExampleSources(name, text)` MUST read one JS/TS test file (`IsTestFile`) with the
  TSA-V0-010 lexer. It returns one `testvalidity.ExampleSource` per example, keyed by the line of
  the example's callee, with `read`, `expectations` and `findings`.
  - An example is a call rooted at `test`, `it`, `specify`, `xit`, `xtest` or `fit`. Its member
    chain may hold `only`, `skip`, `todo`, `each`, `concurrent`, `sequential` and `fixme`. A
    describe is rooted at `describe`, `context`, `suite`, `xdescribe`, `xcontext` or `fdescribe`,
    or is `test.describe`; its chain may also hold `serial` and `parallel`. A `.each` chain is
    followed by its table, either `(…)` or a tagged template, then the declaring call.
  - `findings` are the `ValidityFindings` that name the example, then TSA-V0-074..079 findings in
    that rule order.
  - The whole file MUST abstain with no examples for the TSA-V0-050 reasons and
    `source-too-large`. Any other file abstains with `not-a-js-test-file`.
  - An example MUST be returned with `read` set to an abstention reason, and no findings, when:
    - its chain holds `failing`, `fails`, `fail`, `skipIf` or `runIf`, or it sits in a describe
      whose chain does: `js-example-shape-unsupported`;
    - it is not a todo and has no function or arrow-function callback after its title:
      `js-example-body-not-literal`. An arrow function may carry a TypeScript return type,
      `(): void => {…}`, for an example and for a describe;
    - another example's callee starts on the same line: `js-example-line-ambiguous`.
  - Members outside both lists (`test.step`, `test.use`, `test.beforeEach`, …) are not
    declarations.
- `TSA-V0-074`: (standing auto-accept, 2026-10-11; V1-1858 / GitHub #1159) An example MUST get an
  `invalid` `JS-SKIPPED` finding when it is skipped, and an `invalid` `JS-TODO` finding when it is a
  `todo`. Its location is the example's line; its cause is the first trigger, the example's own
  declaration before its describes, innermost describe first. The skip triggers are:
  - an `xit` or `xtest` root, or a `skip` or `fixme` member;
  - an ancestor describe with an `xdescribe` or `xcontext` root, or a `skip` or `fixme` member;
  - an unconditional runtime `test.skip()` or `test.fixme()` call: no arguments, or a first
    argument that is the literal `true`. It counts when it is in the example body, or outside every
    example in an enclosing describe body or at file level.
- `TSA-V0-075`: (standing auto-accept, 2026-10-11; V1-1858 / GitHub #1159) An example MUST get an
  `invalid` `JS-FOCUSED` finding when an `fit` root or `only` member focuses it, or an `fdescribe`
  root or `only` member focuses an ancestor describe. The location and cause are chosen as for
  `JS-SKIPPED`.
- `TSA-V0-076`: (standing auto-accept, 2026-10-11; V1-1858 / GitHub #1159) An example declared
  with `.each`, or inside a describe declared with `.each`, MUST get an `advisory`
  `JS-EACH-ROW-BINDING` finding. Its cause is the `.each` declaration. Its message gives the row
  count for an array literal table (top-level elements) or a tagged-template table (non-blank lines
  after the header), and otherwise says the rows were not counted. One `file:line` names every row,
  so a row's outcome is bound to the line, not to its row.
- `TSA-V0-077`: (standing auto-accept, 2026-10-11; V1-1858 / GitHub #1159) `expectations` MUST
  count the assertion calls in the example body (TSA-V0-013 assertions, plus `assert.` and
  `expect.` member calls and a `.should` member). `expect.assertions(…)` and
  `expect.hasAssertions(…)` declare a count and are not assertions here or in TSA-V0-078. It MUST
  also count each call of a same-file function definition (`definitions`) whose body asserts,
  followed to `MaxHelperDepth`; a name already followed is followed again when a shorter path
  reaches it. Two calls on one line count twice. Only the strength anchors are distinct: one
  `path:line` per line that holds a counted call.
  - An example that is not a todo, with zero expectations, MUST get an `advisory`
    `JS-NO-ASSERTION` finding located at the example.
  - The finding is withheld when the body, or a followed helper, calls a name that the file
    imports from a relative module other than a TSA-V0-079 subject module and does not define,
    since the reader cannot resolve it.
- `TSA-V0-078`: (standing auto-accept, 2026-10-11; V1-1858 / GitHub #1159) An example whose body
  holds at least one assertion, all of them `expect(…)` chains, and every chain constant MUST get
  an `advisory` `JS-CONSTANT-EXPECTATION` finding. It is located at the first chain and caused by
  the example. A chain is constant when:
  - its subject is one literal token (number, literal string, `true`, `false`, `null` or
    `undefined`);
  - or it is a `toBe`, `toEqual` or `toStrictEqual` whose subject renders like its argument;
  - or its subject is one identifier bound in the example body by `const NAME = LITERAL`, where
    the literal ends the statement: a `;`, the end of the body, or a new line whose first token is
    an identifier other than `in`, `instanceof`, `as` or `satisfies`. Any other next line may
    continue the expression, so the binding is not a literal.
- `TSA-V0-079`: (standing auto-accept, 2026-10-11; V1-1858 / GitHub #1159) Module mocks
  (`jest.mock`, `jest.doMock`, `jest.unstable_mockModule`, `vi.mock`, `vi.doMock` with a literal
  specifier), spies (`jest.spyOn`, `vi.spyOn`), fake timers (`useFakeTimers`, `setSystemTime`)
  and an assignment to `global.fetch`, `globalThis.fetch` or `window.fetch` MUST be classified.
  - The subject under test is (a) a module specifier whose base name, without extension (or
    the directory name for `index`), equals the test file's stem without `.spec` or `.test`; or
    (b) the module that the outermost describe's title or identifier argument is imported from.
    Identifiers imported from a subject module, and that title, name the subject.
  - A mock of a subject module, or a spy whose receiver's root identifier names the subject,
    gives an `advisory` `JS-SUBJECT-MOCKED` finding.
  - A boundary is a mock of `axios`, `node-fetch`, `cross-fetch`, `got`, `superagent`, `undici`,
    `ky`, `http`, `https` or their `node:` forms; a spy on `fetch`, `setTimeout`, `setInterval`
    or `Date`, on `global`, `globalThis`, `window` or an identifier imported from a boundary
    module; a fake-timer call; or a `fetch` assignment. It gives an `advisory`
    `JS-BOUNDARY-MOCKED` finding.
  - A mock applies to an example when it is in the example body, or outside every example and
    every named function definition, at file level or in an enclosing describe body (its hooks
    included). A named function that is an example's or describe's own callback,
    `it('works', function example() {…})`, is that declaration's body, not a named helper. Each distinct location gives one finding, located at the mock and caused by the
    example. Other mocks give no finding.
- `TSA-V0-080`: (standing auto-accept, 2026-10-11; V1-1858 / GitHub #1159)
  `testvalidity.ApplyJSExampleSource(p, source, anchor)` MUST project one JS example as
  `ApplyExampleSource` does, with `js-example-source-read` as the located hygiene reason.
  Invalid findings make hygiene `INELIGIBLE` with the rule ID as reason (TSA-V0-051). Strength
  MUST stay `NOT_MEASURED`, and a measured strength is kept. Its reason is the first that applies:
  - `subject-mocked`, anchored at the JS-SUBJECT-MOCKED locations;
  - `no-expectation-in-example-body`, anchored at the example;
  - `constant-expectations-only`, anchored at the JS-CONSTANT-EXPECTATION location;
  - `boundary-mocked`, anchored at the JS-BOUNDARY-MOCKED locations;
  - `example-body-expectations`, anchored at the expectation lines.
- `TSA-V0-081`: (standing auto-accept, 2026-10-11; V1-1858 / GitHub #1159) `corvint tests
  validity` MUST apply TSA-V0-073 to each named JS/TS test file. That file's `findings` are its
  `ValidityFindings`, then the TSA-V0-074..079 findings of every example in line order, and `tests`
  gains one entry per example, in line order, followed by any test only a finding names.
  - A located example's hygiene is `ApplySourceFindings` over the `ELIGIBLE` `source-rules-read`
    baseline. It also carries a `strength` member, the TSA-V0-080 strength axis.
  - An unlocated example is `ABSTAINED` with its read reason and has no `strength` member.
  - A file the reader abstains on is `uncertain` with that reason and keeps the TSA-V0-071
    finding-named tests.
  - `_spec.rb` files and the exit codes keep TSA-V0-071. `testsource.ExampleSources` routes a
    `_spec.rb` file to TSA-V0-062 and a JS/TS test file to TSA-V0-073.
- `TSA-V0-082`: (standing auto-accept, 2026-10-11; V1-1858 / GitHub #1159)
  `DiscoverWithSources` MUST apply TSA-V0-073 to discovered JS/TS receipt outcomes. `corvint
  test-validity --discover` and the MCP `discover` argument both pass `testsource.ExampleSources`
  and keep emitting the same bytes.
  - An outcome is considered only when its `anchor.file` equals, or is a `/`-suffix of, exactly
    one `identity.testFileDigests` key that is an absolute path inside the worktree.
  - That file is read once through the confined receipt reader. When its SHA-256 differs from the
    retained digest, or it cannot be read, hygiene is `ABSTAINED` `js-source-not-bound`.
  - A reader abstention gives hygiene `ABSTAINED` with its reason, except `not-a-js-test-file`,
    which keeps the projection.
  - The example is the one declared at the anchor line, else the innermost example whose lines
    hold it (a failure frame); with none, hygiene is `ABSTAINED` `js-example-not-located`.
  - A placed example takes TSA-V0-080 with anchor `path:line`, and carries a `sourceRules`
    member. An `INELIGIBLE` axis is never relaxed. Outcomes with no anchor, or an anchor that binds
    no single digest key, are unchanged.
- `TSA-V0-083`: (standing auto-accept, 2026-10-11; V1-1858 / GitHub #1159) These readings MUST be
  deterministic over the same input. They MUST read only the given text, committed Git content or
  the bound worktree test file, and MUST write nothing and use no network.

Non-goals:
- following helpers or mocks into other modules, or reading the module under test;
- conditional skips (`test.skip(condition)`, `skipIf`, `runIf`), `failing` and `fail` examples,
  and skips set through runner configuration or the command line;
- counting assertions in hooks, in custom matchers or through `expect.assertions`;
- mocks set inside a named helper, through `__mocks__` directories or runner config;
- giving RSpec files one `tests` entry per example;
- a Jest provider, or declaration anchors for passing Vitest tests.

Failure modes:
- The reading inherits the TSA-V0-010 lexer failure modes; an unbalanced file abstains.
- A subject that the file names through an alias, a re-export or a path alias is not seen as the
  subject, so its mock is not flagged.
- A boundary list miss leaves a boundary mock unreported, never reported as the subject.
- Discovery attaches only where the receipt has an anchor. Vitest anchors exist only for failures,
  and Playwright failure anchors may point into a helper file, which keeps its projection.

Acceptance evidence: synthetic fixtures only.
- A JS file covering every skip, todo, focus and `.each` form, a conditional skip, `test.step`, an
  unsupported `failing` example, a non-literal body and two examples on one line.
- Assertion counting through a same-file helper, a relative-import helper and no assertions.
- Constant, self-comparing and const-local expectations, beside a live one.
- Subject and boundary mocks by file stem, by describe identifier, by spy and by fake timers.
- `tests validity` on a Jest/Vitest file with one entry per example, and discovery over a bound and
  an edited Playwright spec, with identical MCP bytes.

No live qualification on a real suite was run (NOT_RUN). Rollback removes
`internal/testvalidity/source_rules_jsts.go`, `internal/testsource/validity_js_examples.go`,
`internal/testvaliditydoc/js_source.go`, their tests, the JS branch of `readSourceValidity` and
`ExampleSources`, and this section; callers go back to `RSpecExampleSources`. Nothing is
persisted.

### Traceability (V1-1858)

| Requirement | Code | Evidence | Status |
|---|---|---|---|
| TSA-V0-073 | `internal/testsource/validity_js_examples.go` | `TestJSExampleSourcesShapes`, `TestJSExampleSourcesAbstain`, `TestJSExampleTypedCallback` | experimental |
| TSA-V0-074 | `internal/testsource/validity_js_examples.go` | `TestJSExampleRunState`, `TestJSExampleTypedCallback`, `TestApplyJSExampleSource` | experimental |
| TSA-V0-075 | `internal/testsource/validity_js_examples.go` | `TestJSExampleRunState`, `TestApplyJSExampleSource` | experimental |
| TSA-V0-076 | `internal/testsource/validity_js_examples.go` | `TestJSExampleRunState` | experimental |
| TSA-V0-077 | `internal/testsource/validity_js_examples.go` | `TestJSExampleAssertions` | experimental |
| TSA-V0-078 | `internal/testsource/validity_js_examples.go` | `TestJSExampleAssertions` | experimental |
| TSA-V0-079 | `internal/testsource/validity_js_examples.go` | `TestJSExampleMocks`, `TestJSExampleNamedCallbackMock` | experimental |
| TSA-V0-080 | `internal/testvalidity/source_rules_jsts.go` | `TestApplyJSExampleSource` | experimental |
| TSA-V0-081 | `internal/testsource/validity_command.go`, `cmd/corvint/tests.go` | `TestSourceValidityJSExamples`, `TestTestsValidityCLI` | experimental |
| TSA-V0-082 | `internal/testvaliditydoc/js_source.go`, `cmd/corvint/test_validity.go`, `internal/mcp/testvaliditybridge` | `TestDiscoverJSSourceRules`, `TestTestValidityDiscoverJSSourceRules` | experimental |
| TSA-V0-083 | `internal/testsource/validity_js_examples.go`, `internal/testvaliditydoc/js_source.go` | `TestJSExampleSourcesShapes`, `TestDiscoverJSSourceRules` | experimental |

## Ruby vacuous examples and cross-file helpers in test-validity (V1-1866)

V1-1866 (GitHub #1163) adds RSpec example rules for assertions that cannot fail, found in an
adopter's suite: a block passed to a helper that never yields it, a negated `change` over a record
identity, and a `satisfy` predicate that assigns. A helper is often defined in an included shared
context in `spec/support`, so the reading resolves helper calls across the support files a run
loads, bound to their content. It is agent-drafted and accepted under the owner's standing
auto-accept (2026-10-11). The reader is the TSA-V0-062 line reader; it adds no Ruby parser.

- `TSA-V0-084`: (standing auto-accept, 2026-10-11; V1-1866 / GitHub #1163) An example MUST get an
  `invalid` `RB-BLOCK-NEVER-YIELDED` finding when its body passes a block (`name do` or `name {`)
  to a helper call that resolves (TSA-V0-088) to a `def` with no `&` or `...` parameter whose body
  holds no nested `def`, no interpolation or heredoc, and none of `yield`, `block_given?`,
  `iterator?`, `super`, `binding`, `proc`, `Proc`, `lambda`, `define_method`, the `eval` and
  `exec` family, `send`, `__send__`, `public_send` or `method_missing`, in its body or its
  parameters' default values (`def name(v = yield)` yields). A one-line `def name; …; end` and an
  endless `def name = …`, which opens no block, are helpers, and a one-line example's calls after
  its opening `{` are read. Its location is the call line and its cause the `def` line,
  in whichever file defines it. RSpec DSL calls (`expect`, `let`, `change`, `satisfy`,
  `allow`, Rails and Ruby block words such as `travel_to`, `each` and `tap`, hooks and the example
  and group words) are never helper calls.
- `TSA-V0-085`: (standing auto-accept, 2026-10-11; V1-1866 / GitHub #1163) An example MUST get an
  `advisory` `RB-CHANGE-ON-IDENTITY` finding for `not_to change { X.reload }` or
  `to_not change { X.reload }` whose block ends at the `reload`: RSpec compares the value's before
  and after hash, which for an ActiveRecord model is its primary key, so the expectation may pass
  whatever its attributes do. The reader cannot prove the receiver is such a model (a mutable
  object whose `reload` returns itself changes its hash), so the finding is advisory and leaves
  hygiene `ELIGIBLE`. `change { X.reload.attr }` and a positive `to change` give none. Location is
  the expectation line, cause the example.
- `TSA-V0-086`: (standing auto-accept, 2026-10-11; V1-1866 / GitHub #1163) An example MUST get an
  `invalid` `RB-ASSIGNMENT-IN-PREDICATE` finding when the last statement of a one-line
  `satisfy { |x| … }` block, or the last line of a `satisfy do |x| … end` block, is a `=` or `||=`
  assignment to a name, index or attribute, not a comparison (`==`, `=~`, `=>`), of a truthy
  literal (a number, string, symbol, `true`, or array or hash literal): the predicate returns that
  literal whatever the subject holds. An assignment of any other expression (`valid = x.even?`)
  may be a real predicate and gives none. Location is the `satisfy` line, cause the example.
- `TSA-V0-087`: (standing auto-accept, 2026-10-11; V1-1866 / GitHub #1163) An example whose every
  expectation is a negated `raise_error`, and whose title holds none of the case-insensitive
  stems `rais`, `error`, `exception`, `fail`, `crash`, `blow`, `explod`, `succeed`, `success`,
  `work`, `run`, `handle` or `accept`, MUST get an `advisory` `RB-RAISE-ERROR-ONLY` finding
  located and caused at the example. Advisory findings leave hygiene `ELIGIBLE`.
- `TSA-V0-088`: (standing auto-accept, 2026-10-11; V1-1866 / GitHub #1163) A helper call MUST
  resolve to the nearest definition: a `def` in the example's group or an ancestor, then a `def`
  in a shared context that group includes with `include_context` or `include_examples`, the later
  line winning within one group, then a file-level `def`. Shared contexts are looked up in the spec
  file and in `testvalidity.RubySupport`, the bound support set (`spec/spec_helper.rb`,
  `spec/rails_helper.rb`, `spec/support/**/*.rb`).
  - `corvint tests validity` reads the support set at the same commit; discovery uses the
    receipt's retained helper set, each file only when its blob still matches.
  - A shared context named by a non-literal, defined in no read file, or not readable, and a
    module or class `def` the reader does not follow, MUST give an `abstained`
    `RB-HELPER-UNRESOLVED` finding; two definitions MUST give `RB-HELPER-AMBIGUOUS`. So MUST a
    shared context found while the support set is unbound or a support file did not read, a shared
    context that itself includes another inline, and one group level whose candidate definitions
    sit in different files (a spliced shared body and its customization block, TSA-V0-089), which
    have no line order to compare. Within one shared context the later `def` wins. A
    block-passing call no read file defines, while the support set is unbound or incomplete, MUST
    give `RB-HELPER-UNRESOLVED` caused at the call. The TSA-V0-054 `BLOCK-NOT-CALLED` example
    finding MUST use this same resolution. Location is
    the call, cause the include or `def`. They are emitted only for a block-passing call that
    could resolve to a never-yielding `def`, or when the support set is unbound.
  - `ApplySourceFindings` MUST make an `ELIGIBLE` axis `ABSTAINED` with the first abstained
    rule as reason when no finding is invalid; an invalid finding still wins.
  - `RSpecExampleSources` without a support set reads it as unbound,
    `rspec-support-not-given`.

Non-goals:
- following `include` of Ruby modules, `config.include`, method_missing, `define_method` or
  `alias`;
- judging whether a yielded block asserts, or whether a negated change is meaningful elsewhere;
- reading the code under test.

Failure modes:
- A helper that forwards its block implicitly through `super` or `send` is not flagged; one
  that hands it on through a method the reader cannot see (for example a module method given the
  block with `&`) is excluded by the `&` parameter rule, so the rule under-reports rather than
  guesses.
- A support set over the provider bounds is unbound, so block-passing calls abstain instead of
  flagging.
- The line reader inherits TSA-V0-062's uncertainty; an unbalanced file abstains whole.

Acceptance evidence: synthetic fixtures only.
- A spec and a support shared context with a never-yielding helper, a yielding one, a
  `block.call` one and a local override (`TestRSpecBlockNeverYieldedAcrossFiles`), and a same-file
  one-line def (`TestRSpecBlockNeverYieldedSameFile`).
- Each abstention: unbound support, context not found, two definitions, non-literal include and a
  module def (`TestRSpecHelperAbstentions`).
- Clean and flagged forms of TSA-V0-085..087.

No live qualification on a real suite was run (NOT_RUN). Rollback removes
`internal/testsource/validity_rspec_vacuous.go`, its test, the `RB-*` rules and
`SeverityAbstained`, the support plumbing in `validity_command.go` and this section; callers go
back to `RSpecExampleSources`. Nothing is persisted.

### Traceability (V1-1866)

| Requirement | Code | Evidence | Status |
|---|---|---|---|
| TSA-V0-084 | `internal/testsource/validity_rspec_vacuous.go`, `internal/testsource/validity_rspec_block.go` | `TestRSpecBlockNeverYieldedAcrossFiles`, `TestRSpecBlockNeverYieldedSameFile`, `TestRSpecHelperForms` | experimental |
| TSA-V0-085 | `internal/testsource/validity_rspec_vacuous.go` | `TestRSpecChangeOnIdentity` | experimental |
| TSA-V0-086 | `internal/testsource/validity_rspec_vacuous.go` | `TestRSpecAssignmentInPredicate` | experimental |
| TSA-V0-087 | `internal/testsource/validity_rspec_vacuous.go` | `TestRSpecRaiseErrorOnlyIsAdvisory` | experimental |
| TSA-V0-088 | `internal/testsource/validity_rspec_vacuous.go`, `internal/testvalidity/source_rules.go`, `internal/testsource/validity_command.go` | `TestRSpecHelperAbstentions`, `TestRSpecBlockNeverYieldedAcrossFiles`, `TestRSpecResolutionAbstains`, `TestRSpecInclusionCustomizationOverride` | experimental |

## RSpec shared examples through one inclusion (V1-1867)

V1-1867 (GitHub #1164) reads a shared example through the one inclusion site that ran it, so its
`let` overrides, hooks and helpers are the ones RSpec evaluates. A bare reference to a shared
example keeps `SHARED-EXAMPLE-ONLY` (TSA-V0-065). It is agent-drafted and accepted under the
owner's standing auto-accept (2026-10-11). The reader stays the TSA-V0-062 line reader.

- `TSA-V0-089`: (standing auto-accept, 2026-10-11; V1-1867 / GitHub #1164) `corvint tests
  validity` MUST accept the qualified reference `SPEC:INCLUDE_LINE>SHARED:EXAMPLE_LINE`, for
  example `spec/x_spec.rb:12>spec/support/y.rb:7`, with `same` as `SHARED` when the shared group
  is defined in the including spec. Both files are read at the same commit with the TSA-V0-088
  support set.
  - The inclusion at `INCLUDE_LINE` (`it_behaves_like`, `it_should_behave_like`,
    `include_examples` or `include_context`) MUST be read by splicing the shared group's body where
    RSpec evaluates it: `it_behaves_like` and `it_should_behave_like` nest it in a new group placed
    before any customization block's own body; `include_examples` and `include_context` expand it
    inline in the including group.
  - The example at `EXAMPLE_LINE` then gets the normal TSA-V0-062..088 verdict. Every finding's
    test is the qualified reference; locations and causes keep their own file and line.
  - A bare `SHARED:LINE` reference MUST keep `SHARED-EXAMPLE-ONLY`, whose message names the
    qualified form.
- `TSA-V0-090`: (standing auto-accept, 2026-10-11; V1-1867 / GitHub #1164) Two inclusions of one
  shared group MUST be read independently: each qualified reference gets the verdict of its own
  splice, so one site can be `INELIGIBLE` while the other is `ELIGIBLE`.
- `TSA-V0-091`: (standing auto-accept, 2026-10-11; V1-1867 / GitHub #1164) A qualified read that
  cannot bind one inclusion MUST give the test `ABSTAINED` and the file uncertain, with a distinct
  reason:
  - `rspec-inclusion-not-found`: no inclusion at `INCLUDE_LINE`;
  - `rspec-inclusion-in-example-or-hook`: the inclusion is inside an example or hook body;
  - `rspec-inclusion-loop-generated`: the inclusion, or a sibling the scoped id walk must count,
    is inside an `each`-style loop;
  - `rspec-inclusion-name-not-literal`: the shared group is named by a non-literal;
  - `rspec-shared-group-not-found`, `rspec-shared-group-ambiguous` (two or more visible
    definitions, or a support set that could hold another), `rspec-shared-group-nested` (a shared
    group defined in, or including, another shared group) and `rspec-shared-group-unbound` (the
    support set is unbound);
  - `rspec-inclusion-example-not-in-group`: `EXAMPLE_LINE` is not an example of that group;
  - `rspec-example-loop-generated`: the example is generated in a loop.
- `TSA-V0-092`: (standing auto-accept, 2026-10-11; V1-1867 / GitHub #1164) Receipt discovery MUST
  read an example whose receipt id (`./spec/x_spec.rb[1:3:2:1]`) names another spec than the file
  declaring it through the inclusion that id places, and anchor its projection at the qualified
  reference.
  - The scoped id walk counts top-level groups and `it_behaves_like` inclusions; an inline
    inclusion takes one slot per shared child, and shared definitions take none.
  - The rest of the path MUST land on the named shared example, and an `it_behaves_like` or
    `it_should_behave_like` inclusion's generated description MUST appear in the receipt's
    `full_description`, and a receipt-based read without a `full_description` cannot be
    cross-checked. Otherwise the read abstains with `rspec-inclusion-id-unplaced` or
    `rspec-inclusion-description-mismatch`.
  - The including spec's content is the blob of another example the receipt retains for that
    spec; without one the read abstains `rspec-inclusion-source-unbound`.
  - `testvaliditydoc.DiscoverWithReaders` takes the readers; the CLI and the MCP test-validity
    bridge pass `testsource.RSpecExampleSourcesWithSupport` and `testsource.RSpecInclusionSource`.

Non-goals:
- binding shared group block parameters or `it_behaves_like 'x', args` arguments: the body is
  spliced with its parameters unbound;
- following module `include` or `config.include_context`;
- reading more than one inclusion per reference, or reading inclusions the receipt did not run.

Failure modes:
- A shared group that reads its arguments may be judged on its literal body; findings that depend
  on those arguments can be missed or reported for a value the run never used.
- A scoped id from an RSpec version that numbers differently places no inclusion and abstains.
- A support set that could hold another definition abstains as ambiguous rather than picking one.

Acceptance evidence: synthetic fixtures only.
- A support shared group included twice, once with a stub reader and once with a yielding one
  (`TestRSpecInclusionEvaluatesEachSiteIndependently`); a same-file group with a `let` override
  (`TestRSpecInclusionSameFile`).
- Every refusal code (`TestRSpecInclusionRefusals`), the scoped id and description join
  (`TestRSpecInclusionReceiptJoin`), the CLI read (`TestSourceValidityQualifiedReference`) and
  discovery from a receipt (`TestDiscoverRubyInclusionJoin`).

No live qualification on a real suite was run (NOT_RUN). Rollback removes
`internal/testsource/validity_rspec_inclusion.go`, its test, `qualifiedValidity`, the
`RSpecInclusion` type, the inclusion reasons, `ParseRSpecScopedID`, `DiscoverWithReaders` and this
section; the callers go back to `DiscoverWithSources`. Nothing is persisted.

### Traceability (V1-1867)

| Requirement | Code | Evidence | Status |
|---|---|---|---|
| TSA-V0-089 | `internal/testsource/validity_rspec_inclusion.go`, `internal/testsource/validity_command.go` | `TestRSpecInclusionEvaluatesEachSiteIndependently`, `TestRSpecInclusionSameFile`, `TestRSpecInclusionCustomizationOverride`, `TestSourceValidityQualifiedReference` | experimental |
| TSA-V0-090 | `internal/testsource/validity_rspec_inclusion.go` | `TestRSpecInclusionEvaluatesEachSiteIndependently` | experimental |
| TSA-V0-091 | `internal/testsource/validity_rspec_inclusion.go`, `internal/testvalidity/source_rules_rspec.go` | `TestRSpecInclusionRefusals`, `TestSourceValidityQualifiedReference` | experimental |
| TSA-V0-092 | `internal/testsource/validity_rspec_inclusion.go`, `internal/testvaliditydoc/ruby_source.go`, `internal/testvalidity/source_rules_rspec.go` | `TestRSpecInclusionReceiptJoin`, `TestDiscoverRubyInclusionJoin` | experimental |

## Outcome, cleanup, input and vacuity rules (V1-1919, V1-1930, V1-1931, V1-1932)

V1-1930 (GitHub #1191), V1-1919 (GitHub #1185), V1-1931 (GitHub #1192) and V1-1932 (GitHub #1193)
add four static test-validity rules to `corvint tests validity` for JS/TS sources and behaviour
contracts, plus one fail-closed Playwright receipt reason. They are agent-drafted and accepted under
the owner's standing auto-accept (2026-10-11). The readers stay the TSA-V0-062 token reader over
committed source; nothing reads a database, a network or a run. Each finding cites a committed
`path:line`; when the file does not state what a rule needs, the rule does not report.

- `TSA-V0-160`: (standing auto-accept, 2026-10-11; V1-1930 / GitHub #1191) `EITHER-OR-OUTCOME`
  MUST be an invalid finding on an assertion inside a JS/TS test body that accepts two or more
  distinct literal outcomes, since it passes whichever the product returns and cannot detect a
  regression from one to another. The finding makes the test `INELIGIBLE` (TSA-V0-051); its
  message names the accepted values and the annotation of TSA-V0-163. An assertion outside every
  test body is not reported.
- `TSA-V0-161`: (standing auto-accept, 2026-10-11; V1-1930 / GitHub #1191) The JS/TS forms MUST be:
  `expect([a, b]).toContain(x)` and `expect([a, b].includes(x))` with two or more distinct
  literals; `expect(x === a || x === b)` over one subject, with or without parentheses around each
  equality; `toBeOneOf([a, b])` and
  `assert.oneOf(x, [a, b])`; and a regular-expression literal accepting a closed set of outcomes:
  an anchored alternation of words (`/^(DENIED|FORBIDDEN)$/`), an alternation of digit strings
  (`/401|403/`) or a digit class inside a status code (`/^40[13]$/`). Numbers are compared by
  value, so `200` and `200.0` are one value. A negated chain, a non-literal set, one distinct value,
  two different subjects, and an alternation of words that is unanchored or anchored only at its
  ends (`/^A|B$/`, a text search) are not reported.
- `TSA-V0-162`: (standing auto-accept, 2026-10-11; V1-1930 / GitHub #1191) A Markdown, YAML or
  JSON file named to `corvint tests validity --files` MUST be read as a behaviour contract: a line
  whose key or label starts with `expected` and whose value is two or more distinct single-token
  outcomes joined by `or`, or status codes or upper-case enum names joined by `/`, is an invalid
  `EITHER-OR-OUTCOME` finding whose test, location and cause are that line and whose title is the
  key. Prose, dates and a single value are not reported; other file types are not read as
  contracts.
- `TSA-V0-163`: (standing auto-accept, 2026-10-11; V1-1930 / GitHub #1191) A comment
  `corvint-validity-allow EITHER-OR-OUTCOME: <reason>` on the flagged line, or alone on the line
  above, MUST exempt that line. In a behaviour contract the line above MUST be a comment (`#`,
  `//` or `<!--`); data on it naming the annotation does not exempt the value. An annotation
  without a reason, or naming another rule, MUST NOT exempt the line.
- `TSA-V0-164`: (standing auto-accept, 2026-10-11; V1-1930 / GitHub #1191) Proposing the one value
  a passing receipt observed is NOT_PRODUCED: no receipt records assertion operands, so the
  message names the accepted values only and never guesses which one is correct.
- `TSA-V0-165`: (standing auto-accept, 2026-10-11; V1-1919 / GitHub #1185) `CLEANUP-HANDLE-OUTLIVED`
  MUST be an advisory finding on a cleanup closure registered on a registry whose fixture the file
  declares with a longer literal scope (`{ scope: 'worker' }`) when the closure captures a
  shorter-lived handle: a test-scoped fixture the file declares, or a Playwright test-scoped
  built-in (`request`, `page`, `context`). Its location is the registration line and its cause the
  registry's declaration; its message names the handle and where it is declared and suggests a
  context the closure owns (`playwright.request.newContext()`). A closure taking its own context
  parameter, or declaring its own local under the handle's name, is not reported.
- `TSA-V0-166`: (standing auto-accept, 2026-10-11; V1-1919 / GitHub #1185) The rule MUST read a
  registration inside a test body, inside a test-scoped fixture function (including through a
  local alias `const api = request`), and an `afterAll` hook reading a variable a `beforeEach`
  assigned from a test-scoped handle. A JS/TS file that is not a test file (a fixture file) MUST be
  read for this rule alone, with the registration line as the finding's test.
- `TSA-V0-167`: (standing auto-accept, 2026-10-11; V1-1919 / GitHub #1185) A registry or handle
  whose scope the file does not state (imported from another file, or a non-literal `scope`), a
  test-scoped registry, and a closure capturing no handle MUST NOT be reported.
- `TSA-V0-168`: (standing auto-accept, 2026-10-11; V1-1919 / GitHub #1185) A Playwright JSON report
  with at least one suite, a run-level `errors` entry and no outcome other than `passed`, `flaky`
  or `skipped` MUST give the infrastructure failure `passed-cleanup-failed`, whose detail is the
  first error's scrubbed, secret-screened message, so a run whose worker fixture teardown or
  cleanup failed fails closed instead of reading as passed. A report with a failing outcome is
  left unchanged.
- `TSA-V0-169`: (standing auto-accept, 2026-10-11; V1-1931 / GitHub #1192)
  `UNSTUBBED-OUTBOUND-INPUT` MUST be an advisory finding on a test input that names a stubbed
  integration parameter with a value no stub in the file matches. The stub table is the file's
  literal request matchers, Playwright `X.route('<glob>', ...)` and MSW
  `http|rest.<method>('<url>', ...)`. A Playwright matcher path with query parameters names an
  integration and its stubbed values. An input is an object property `param: 'value'` naming a
  stubbed parameter, or a literal URL whose path a matcher names (equal, or after a leading `**`)
  with a query value for a stubbed parameter. An input is covered by a stubbed value whose
  Playwright glob (`**`, `*`, `{a,b}`) matches it, by an MSW handler at its path (MSW ignores the
  query when matching), or by a Playwright matcher without a query whose glob matches the request,
  such as a `**/*` catch-all.
- `TSA-V0-170`: (standing auto-accept, 2026-10-11; V1-1931 / GitHub #1192) The finding's cause MUST
  be the stub line of the nearest stubbed value of the same parameter by edit distance, the first
  on a tie, and its message MUST name the input value and that value.
- `TSA-V0-171`: (standing auto-accept, 2026-10-11; V1-1931 / GitHub #1192) A file with any matcher
  that is not a literal string, or with no stub, MUST NOT be read for the rule, since that matcher
  may cover the value.
- `TSA-V0-172`: (standing auto-accept, 2026-10-11; V1-1932 / GitHub #1193; amended 2026-10-19,
  owner decision, V1-1944) `VACUOUS-EMPTY-EQUALITY` MUST be an advisory finding on a count or
  equality assertion inside a test body whose two operands are both collection-derived:
  `toHaveCount` or `toHaveLength` with a collection-derived argument, or `toBe`, `toEqual` or
  `toStrictEqual` between two operands ending in `.length`, `.size` or `.count()`. A test whose
  in-scope fixture or hook seeds the collection, as TSA-V0-306 defines, is excluded. Its location
  and cause are the assertion line and its message suggests a non-empty precondition or the exact
  seeded count. A negated chain or a literal operand is not reported.
- `TSA-V0-173`: (standing auto-accept, 2026-10-11; V1-1932 / GitHub #1193) An assertion in the same
  test that one of the collections is non-empty MUST suppress the finding: `toBeGreaterThan(n)`
  with `n >= 0`, `toBeGreaterThanOrEqual(n)` with `n >= 1`, a positive literal count or length,
  `toBeTruthy` on a count, a negated zero or `[]`, a negated `toBeEmpty`, or a visibility or text
  assertion on the collection or a member of it. A text assertion of `[]` or an empty string is not
  a precondition, since it passes on an empty collection.
- `TSA-V0-174`: (standing auto-accept, 2026-10-11; V1-1931 / GitHub #1192) The run-time
  `ENV-UNSTUBBED-CALL` reading (a 5xx whose server log shows an outbound call to an unstubbed
  host) is NOT_PRODUCED with reason `server-outbound-log-not-produced`: no receipt retains a server
  log or its outbound calls. It MUST NOT be inferred from a status code alone.
- `TSA-V0-175`: (standing auto-accept, 2026-10-11; V1-1932 / GitHub #1193) The run-time
  `VACUOUS-EMPTY-EQUALITY` reading (both operands observed empty) is NOT_PRODUCED with reason
  `assertion-operands-not-produced`: no receipt records assertion operands.
- `TSA-V0-176`: (standing auto-accept, 2026-10-11; V1-1932 / GitHub #1193) Reporting drift of a
  matrix row's `file:line` citations at the row's pinned commit is NOT_PRODUCED: no row source with
  a pinned commit exists in this profile to read.

Non-goals:
- choosing the correct value of an either-or assertion, or reading natural-language contract
  prose beyond `expected` keys;
- following a fixture, registry or stub declared in another file;
- evaluating non-literal matchers, request builders or seed functions;
- adding a field to the appflows wire (AFU-V1-042): the qualified external path already fails
  closed on `reporter-global-error`.

Failure modes:
- A registry imported from a fixture file is not reported in the test file that uses it; the
  fixture file itself is read only when named.
- A test whose in-scope fixture or hook seeds rows gets no `VACUOUS-EMPTY-EQUALITY` advisory
  (TSA-V0-306); one whose fixture is declared in another file gets a `fixture-cross-file`
  abstention instead (TSA-V0-307). Seeding written inside the test body is not read, so such a test
  still gets the advisory when it has no precondition; the advisory does not change hygiene.
- A run whose only error is a teardown failure now reports infrastructure, and the runner drops
  its test outcomes, so its passing tests are not counted as proof.

Acceptance evidence: synthetic fixtures only.
- `TestEitherOrOutcomeForms`, `TestEitherOrOutcomeNegatives`, `TestEitherOrOutcomeAnnotation`,
  `TestEitherOrOutcomeContract`, `TestEitherOrOutcomeRefusesProof` and
  `TestEitherOrOutcomeReviewCases`.
- `TestCleanupHandleOutlivedInTest`, `TestCleanupHandleOutlivedFixtureFile`,
  `TestCleanupHandleOutlivedAbstains`, `TestCleanupHandleOutlivedShadowed`,
  `TestParsePlaywrightJSONPassedCleanupFailed` and
  `TestParsePlaywrightJSONCleanupGateNegatives`.
- `TestUnstubbedOutboundInputReported`, `TestUnstubbedOutboundInputCoverage` and
  `TestUnstubbedOutboundInputAbstains`.
- `TestVacuousEmptyEqualityReported`, `TestVacuousEmptyEqualityNegatives`,
  `TestVacuousEmptyEqualityEmptyText` and `TestVacuousEmptyEqualityOutsideTest`.

No live qualification on a real suite was run (NOT_RUN). Rollback removes
`internal/testsource/validity_either_or.go`, `validity_cleanup_handle.go`,
`validity_unstubbed_input.go`, `validity_vacuous_empty.go`, their tests,
`internal/testvalidity/source_rules_outcome.go`, `internal/jstestprovider/playwright_cleanup.go`,
the `outcomeFindings` call and the two `ValidityFindings` cases, and this section. Nothing is
persisted.

### Traceability (V1-1919, V1-1930, V1-1931, V1-1932)

| Requirement | Code | Evidence | Status |
|---|---|---|---|
| TSA-V0-160 | `internal/testsource/validity_either_or.go`, `internal/testvalidity/source_rules_outcome.go` | `TestEitherOrOutcomeForms`, `TestEitherOrOutcomeRefusesProof` | experimental |
| TSA-V0-161 | `internal/testsource/validity_either_or.go` | `TestEitherOrOutcomeForms`, `TestEitherOrOutcomeNegatives`, `TestEitherOrOutcomeReviewCases` | experimental |
| TSA-V0-162 | `internal/testsource/validity_either_or.go`, `internal/testsource/validity.go` | `TestEitherOrOutcomeContract` | experimental |
| TSA-V0-163 | `internal/testsource/validity_either_or.go` | `TestEitherOrOutcomeAnnotation`, `TestEitherOrOutcomeContract`, `TestEitherOrOutcomeReviewCases` | experimental |
| TSA-V0-164 | none | NOT_PRODUCED | gap |
| TSA-V0-165 | `internal/testsource/validity_cleanup_handle.go` | `TestCleanupHandleOutlivedInTest`, `TestCleanupHandleOutlivedShadowed` | experimental |
| TSA-V0-166 | `internal/testsource/validity_cleanup_handle.go`, `internal/testsource/validity.go` | `TestCleanupHandleOutlivedInTest`, `TestCleanupHandleOutlivedFixtureFile` | experimental |
| TSA-V0-167 | `internal/testsource/validity_cleanup_handle.go` | `TestCleanupHandleOutlivedAbstains` | experimental |
| TSA-V0-168 | `internal/jstestprovider/playwright_cleanup.go`, `internal/jstestprovider/playwright.go` | `TestParsePlaywrightJSONPassedCleanupFailed`, `TestParsePlaywrightJSONCleanupGateNegatives` | experimental |
| TSA-V0-169 | `internal/testsource/validity_unstubbed_input.go` | `TestUnstubbedOutboundInputReported`, `TestUnstubbedOutboundInputCoverage` | experimental |
| TSA-V0-170 | `internal/testsource/validity_unstubbed_input.go` | `TestUnstubbedOutboundInputReported` | experimental |
| TSA-V0-171 | `internal/testsource/validity_unstubbed_input.go` | `TestUnstubbedOutboundInputAbstains` | experimental |
| TSA-V0-172 | `internal/testsource/validity_vacuous_empty.go` | `TestVacuousEmptyEqualityReported`, `TestVacuousEmptyEqualityOutsideTest`, `TestVacuousEmptyEqualitySeeded` | experimental |
| TSA-V0-173 | `internal/testsource/validity_vacuous_empty.go` | `TestVacuousEmptyEqualityNegatives`, `TestVacuousEmptyEqualityEmptyText` | experimental |
| TSA-V0-174 | none | NOT_PRODUCED (`server-outbound-log-not-produced`) | gap |
| TSA-V0-175 | none | NOT_PRODUCED (`assertion-operands-not-produced`) | gap |
| TSA-V0-176 | none | NOT_PRODUCED | gap |

## Cross-file abstentions, fixture seeding and JSON annotation (V1-1944)

V1-1944 is an agent-drafted follow-up to the V1-1919, V1-1930, V1-1931 and V1-1932 rules, accepted
under the owner's standing auto-accept (2026-10-11). It covers three cases. A stub table or cleanup
registry declared in a sibling module now produces an explicit cross-file abstention rather than
silence. A test whose setup seeds data no longer gets `VACUOUS-EMPTY-EQUALITY`. A JSON behaviour
contract gains a documented annotation form. The reader is still the TSA-V0-062 token reader over
one committed file at a time. It does not follow imports: resolving the sibling module is
NOT_PRODUCED (TSA-V0-312). TSA-V0-305 to TSA-V0-307 add seeding to the reading that TSA-V0-172
describes, and TSA-V0-172 itself is amended (2026-10-19, owner decision) to exclude the tests
TSA-V0-306 suppresses; no other accepted requirement is amended. TSA-V0-301 to TSA-V0-304 add abstention records
while still never reporting a finding that TSA-V0-167 or TSA-V0-171 forbids.

- `TSA-V0-300`: (standing auto-accept, 2026-10-11; V1-1944) An advisory reading withheld
  because what decides it is declared in another file MUST be recorded as an abstention, not as a
  finding. The abstention goes in the `abstentions` list of the file's `ValidityReport` and of its
  `corvint-test-source-validity/0` file entry, and has these members:
  - `rule`;
  - `reason`: `stub-table-cross-file`, `cleanup-registry-cross-file` or `fixture-cross-file`;
  - `test` and `location`: the `path:line` of the test and of the statement;
  - `cause`: the `path:line` of the import naming the other module, or the location when the file
    names none;
  - `message`: the withheld reading and the module.

  An abstention changes no axis, verdict, `uncertain` flag or exit code. The member is omitted when
  the list is empty, so a report without abstentions is byte-identical to before.
- `TSA-V0-301`: (standing auto-accept, 2026-10-11; V1-1944) A sibling stub import is an import,
  re-export, `require` or dynamic `import()` with both of these properties:
  - its specifier is relative (`.`, `..`, `./…`, `../…`);
  - its specifier, or a name its `import … from` or `export … from` clause names (imported or
    local), contains `stub`, `mock`, `msw`, `handler`, `route`, `fake` or `intercept`, ignoring
    case.

  In a file that has its own literal stub table and a sibling stub import,
  `UNSTUBBED-OUTBOUND-INPUT` MUST NOT report an input that no local stub covers. That input MUST be
  a `stub-table-cross-file` abstention whose cause is the import and whose message is the finding's
  message plus the module that may cover the value. A package import, or a sibling import naming
  none of those words, leaves TSA-V0-169 unchanged.
- `TSA-V0-302`: (standing auto-accept, 2026-10-11; V1-1944) A file with tests, no stub of its own
  and a sibling stub import MUST record one file-level `stub-table-cross-file` abstention.
  - Its test, location and cause are the import line.
  - Its message names the module and says the rule is not read for this file.
  - The file's inputs are still not read (TSA-V0-171).
  - A file with no test records nothing.
- `TSA-V0-303`: (standing auto-accept, 2026-10-11; V1-1944) A cleanup closure registered on a
  registry that is a fixture the file does not declare, and that captures a test-scoped handle
  (TSA-V0-165), MUST be a `cleanup-registry-cross-file` abstention.
  - Its location is the registration line.
  - Its test is the enclosing test, or the registration line in a fixture file (TSA-V0-166).
  - Its cause is the file's first sibling import, else its first import other than
    `@playwright/test`.
  - Its message names the registry, the captured handle and the module that declares the
    registry's scope.
- `TSA-V0-304`: (standing auto-accept, 2026-10-11; V1-1944) A cross-file registration whose closure
  captures no test-scoped handle MUST NOT record an abstention. Under TSA-V0-167, no cross-file
  registration is a finding.
- `TSA-V0-305`: (standing auto-accept, 2026-10-11; V1-1944) A seeding call MUST be either of:
  - a call `name(` or `x.name(` whose name is `seed`, `create`, `insert` or `populate`, or whose
    name starts with `seed`, `create`, `insert`, `populate` or `add` followed by an upper-case
    letter, digit or `_` (`seedTodos`, `createUser`, `addTodo`, `db.insert`);
  - a member call `x.post(` or `x.put(`.

  A bare `add(` (a set or a cleanup registry) is not a seeding call. Neither are `addInitScript`,
  `addCookies`, `addScriptTag`, `addStyleTag` or `addLocatorHandler`. A function or method
  declaration `name(...) {` is not a call.
- `TSA-V0-306`: (standing auto-accept, 2026-10-11; V1-1944) `VACUOUS-EMPTY-EQUALITY` MUST NOT be
  reported in a test whose setup seeds data. Setup seeds data in either of these cases:
  - The test requests a fixture the file declares (`base.extend({...})`) whose function makes a
    seeding call, directly or through a declared fixture that function requests in turn.
  - A `beforeEach` or `beforeAll` hook in the test's scope has a callback that makes a seeding
    call. The hook is in scope when it is at file level, or when every `describe` that contains the
    hook also contains the test.

  A hook in a sibling `describe`, or a fixture or hook without a seeding call, does not suppress
  the finding.
- `TSA-V0-307`: (standing auto-accept, 2026-10-11; V1-1944) A `VACUOUS-EMPTY-EQUALITY` reading
  that TSA-V0-306 does not suppress MUST be a `fixture-cross-file` abstention instead of a finding
  when the test requests a fixture that is none of these:
  - a fixture the file declares;
  - a Playwright built-in (`page`, `context`, `request`, `browser`, `playwright`, `browserName`);
  - a Playwright option fixture (`baseURL`, `viewport`, `storageState`, `locale` and the other
    `use` options).

  The fixture is declared elsewhere and may seed the collection. The abstention's cause is chosen
  as in TSA-V0-303, and its message names the fixture.
- `TSA-V0-308`: (standing auto-accept, 2026-10-11; V1-1944) The annotation of an
  `EITHER-OR-OUTCOME` value in a JSON behaviour contract MUST be a sibling member in the same
  object as the flagged member:
  `"corvint-validity-allow": {"<key>": "EITHER-OR-OUTCOME: <reason>"}`. It exempts every line that
  holds a `<key>` member of that object. Data is not a comment, so this form keeps TSA-V0-163
  intact: it is a keyed member of the object, not data on the line above.
- `TSA-V0-309`: (standing auto-accept, 2026-10-11; V1-1944) None of these MUST exempt the value:
  - a missing or empty reason;
  - another rule;
  - a key that does not name the member;
  - an annotation in another object, or nested inside a member;
  - an annotation whose value is a string instead of an object;
  - a member with another name;
  - data on the line above;
  - a file that is not exactly one valid JSON value.
- `TSA-V0-310`: (standing auto-accept, 2026-10-11; V1-1944) The message of an `EITHER-OR-OUTCOME`
  finding in a `.json` contract MUST end by naming the sibling-member form with the flagged key,
  since JSON has no comment line. Markdown and YAML messages are unchanged.
- `TSA-V0-311`: (standing auto-accept, 2026-10-11; V1-1944) The abstention reasons and
  `RuleAbstention` MUST be defined once in `internal/testvalidity`, which is the only owner of the
  reason strings.
- `TSA-V0-312`: (standing auto-accept, 2026-10-11; V1-1944) Resolving a sibling module's stub table,
  registry scope or fixture body is NOT_PRODUCED, with reason `cross-file-resolution-not-produced`.
  `ValidityFindings` reads one file without repository access, and an import's target, aliases and
  re-exports are not resolved. The rules abstain (TSA-V0-301 to TSA-V0-303, TSA-V0-307) instead of
  guessing.

Non-goals:
- following imports, path aliases or re-exports to another file;
- reading `auto` fixtures, global setup or seeding written inside the test body;
- tying a seeding call to the particular collection an assertion compares;
- changing the comment annotations of TSA-V0-163.

Failure modes:
- A stub module whose name contains none of the stub words is not recognised. Its importer is read
  as before: findings are reported when the file has local stubs, and the file is not read when it
  has none.
- A sibling stub import withholds every uncovered input in its file, including inputs that module
  does not cover. The reading is advisory and is surfaced as an abstention, not dropped.
- A seeding call that seeds an unrelated collection still suppresses the finding: the reading
  under-reports and never over-reports.
- A helper named like a seeding call that writes nothing (`createLocator`) suppresses the finding.
  So does a seeding call inside a nested function the hook or fixture never invokes.
- A JSON contract that is not valid JSON gets no exemptions, so its findings stay invalid.

Acceptance evidence: synthetic fixtures only.
- `TestUnstubbedOutboundInputCrossFile`
- `TestCleanupHandleOutlivedCrossFile`
- `TestVacuousEmptyEqualitySeeded`, `TestVacuousEmptyEqualityNotSeeded` and
  `TestVacuousEmptyEqualityCrossFileFixture`
- `TestEitherOrOutcomeJSONAnnotation`
- `TestSourceValidityAbstentionsWire`

Each new decision point was mutated, and the tests killed every mutation of tested behaviour.
The mutated points were:
- sibling and stub-word detection, long import clauses and `require(…,)`;
- seeding names, the not-seeding list, the bare `add` exclusion and method declarations;
- transitive fixtures, hook scope and option fixtures;
- the JSON member name, key, rule, reason and single-value check, and the JSON message;
- cross-file registration, capture and nested registrations;
- VE seeding and cross-file fixtures;
- the UO withheld and unread paths;
- the wire copy. No live qualification on a real suite was run (NOT_RUN).

Rollback: remove `internal/testsource/validity_crossfile.go`, its test,
`internal/testvalidity/source_rules_crossfile.go`, the `abstentions` fields, and the cross-file,
seeding and JSON branches in `validity_unstubbed_input.go`, `validity_cleanup_handle.go`,
`validity_vacuous_empty.go` and `validity_either_or.go`. Then remove this section. Nothing is
persisted. Without the `abstentions` member, the report is the prior wire.

### Traceability (V1-1944)

| Requirement | Code | Evidence | Status |
|---|---|---|---|
| TSA-V0-300 | `internal/testvalidity/source_rules_crossfile.go`, `internal/testsource/validity.go`, `internal/testsource/validity_command.go` | `TestSourceValidityAbstentionsWire` | experimental |
| TSA-V0-301 | `internal/testsource/validity_crossfile.go`, `internal/testsource/validity_unstubbed_input.go` | `TestUnstubbedOutboundInputCrossFile` | experimental |
| TSA-V0-302 | `internal/testsource/validity_unstubbed_input.go` | `TestUnstubbedOutboundInputCrossFile` | experimental |
| TSA-V0-303 | `internal/testsource/validity_cleanup_handle.go` | `TestCleanupHandleOutlivedCrossFile`, `TestSourceValidityAbstentionsWire` | experimental |
| TSA-V0-304 | `internal/testsource/validity_cleanup_handle.go` | `TestCleanupHandleOutlivedCrossFile` | experimental |
| TSA-V0-305 | `internal/testsource/validity_crossfile.go` | `TestVacuousEmptyEqualitySeeded`, `TestVacuousEmptyEqualityNotSeeded` | experimental |
| TSA-V0-306 | `internal/testsource/validity_crossfile.go`, `internal/testsource/validity_vacuous_empty.go` | `TestVacuousEmptyEqualitySeeded`, `TestVacuousEmptyEqualityNotSeeded` | experimental |
| TSA-V0-307 | `internal/testsource/validity_vacuous_empty.go` | `TestVacuousEmptyEqualityCrossFileFixture`, `TestVacuousEmptyEqualityNotSeeded` | experimental |
| TSA-V0-308 | `internal/testsource/validity_crossfile.go`, `internal/testsource/validity_either_or.go` | `TestEitherOrOutcomeJSONAnnotation` | experimental |
| TSA-V0-309 | `internal/testsource/validity_crossfile.go` | `TestEitherOrOutcomeJSONAnnotation` | experimental |
| TSA-V0-310 | `internal/testsource/validity_either_or.go` | `TestEitherOrOutcomeJSONAnnotation` | experimental |
| TSA-V0-311 | `internal/testvalidity/source_rules_crossfile.go` | `TestSourceValidityAbstentionsWire` | experimental |
| TSA-V0-312 | none | NOT_PRODUCED (`cross-file-resolution-not-produced`) | gap |

## Decision file format

```json
{"schema": "corvint-test-duplicate-decisions/0",
 "decisions": [
  {"decision": "distinct-from", "test": "e2e/a.spec.ts :: cart > adds", "other": "e2e/b.spec.ts :: adds",
   "reason": "a checks the signed-in cart, b the guest basket"},
  {"decision": "covered-by", "test": "e2e/c.spec.ts :: adds", "other": "e2e/a.spec.ts :: cart > adds",
   "obligation": "CART-001"}]}
```

The pair is unordered. The file belongs to the project and is edited by people; Corvint only reads
it.

## Boundaries and failure modes

Non-goals: running tests, reading uncommitted changes, rewriting tests, judging whether a pair is a
real duplicate, languages other than JS/TS, test frameworks without `test`/`it` and
`describe` calls, an MCP tool for setup-select, and a preflight requirement for clean lint
(issue #1032 AC5). The two missing pieces are residuals of V1-1587 and V1-1589. The advisory rules
do not run tests, do not infer project variants from configuration, do not follow locators stored in
constructor fields or properties, and do not show findings in a run receipt next to the failure
(a V1-1769 residual).

Failure modes: the lexer is heuristic around regular expressions and JSX, so an uncertain file is
listed and its findings may be incomplete. Setup selection trusts declared registries and import
syntax; a setup that is reached only through runtime configuration (for example a Playwright
`dependencies` entry) is not seen, so `selected` is only as complete as the static import graph. A
false-negative lint rule is possible for an assertion helper not named `expect*`, `assert*`,
`verify*` or `should*`. Duplicate scores compare assertion text only: two tests with the same
assertions over different setup are reported, and two tests that check the same behaviour through
different assertions are not. The one-shot rule resolves helpers by name, not by type: a same-named
method on another class can be followed, an import alias (`import { a as b }`), a package import or
a dynamic dispatch is UNKNOWN, and an awaited non-UI call to an unresolved helper (for example an
API client) is reported UNKNOWN rather than silently skipped. The cross-variant rule names only
`exact: true` locators; a split caused by anything else is listed in `crossVariant.splits` with
`exactText: false` and no finding, and a test with a computed title cannot be joined. On a split
test, a call to a page-object helper the rule cannot resolve is an UNKNOWN finding, so a suite with
many unresolved helpers reports one per such line. A change only inside a helper's definition, not
on the asserting or calling lines, does not bring the finding into the changed-line scope.

Shared-constant keys (TSA-V0-006/007) over-select by design: every setup that reads a reached key is
selected, even one that only reads it for a different purpose, and a read through a parameter or
local that shadows the constant's name counts as a read of the constant. Non-goals that therefore
fail closed through TSA-V0-007 rather than being resolved: local aliasing of the object
(`const d = C`), spread, CommonJS `require` and dynamic `import()` bindings of a constant, keys built
at runtime, chains longer than 8 hops, templates nested deeper than 8, and a destructured or method
parameter that shadows the name (read as a whole-object use). The selector does no scope analysis.
A shared constant written by a setup at runtime is not seen, nor is a JSON module's object.

## Implementation, rollout and rollback

Parsing lives in `internal/testsource` (lint, duplicates, Git reads) and
`internal/contextindex/setup_select.go` (setup closure on the revision index); CLI parsing is in
`cmd/corvint/tests.go`. The lexer gains only a comment-collecting variant and an exported view. This
additive experimental profile changes no Core profile or persisted format. Rollback removes the
dispatch line, the help entries and the two packages' new files; there is no data migration. The
V1-1769 advisory rules roll back by removing `internal/testsource/uistate.go` and `variants.go`, the
two rule IDs, the `advisory`, `resolution` and `crossVariant` report members and the `--runs` flag;
the other rules and exit statuses are unchanged.

## Acceptance evidence

Synthetic Git repositories: a registry-keyed spec selects its context's setup only; a hub and the
registry are not followed; comment-only mentions select nothing; computed, unknown, missing and empty
inputs return ALL with typed reasons; a spec reading `SHARED.scenario_a` (the #1082 fixture, also
through a fixture file, a barrel, a renamed re-export, a namespace member, a template substitution
or destructuring) selects only the setup reading the same key, while a computed `SHARED[k]`, a
whole-object use or a namespace used as a value returns ALL with `shared-constant-key-unresolved`;
the PR #1096 review forms (an `export * as` re-export, an `export *` cycle, exactly 8 hops, a read
in the declaring module, `require("m").NAME` and shadowing parameters) select, while a 9-hop setup,
a namespace-valued setup read, literal `require()`/`import()` bindings and a template nested 9 deep
return ALL,
and a spec reaching no shared constant stays `selected` with an empty list; the digest is stable and changes with the spec list. Each lint
rule fires on its positive fixture and not on its negative one; changed-lines scope excludes old
findings; suppressions apply to exactly the named rule and are echoed. Duplicates pair normalised
lines across files, honour the cutoff, gate on changed tests and resolve with reasoned decisions.
The one-shot rule fires on direct reads and on reads returned through same-file methods, arrow
properties, functions, locals and relatively imported modules, stays silent on web-first matchers,
`expect.poll` and `toPass`, and reports UNKNOWN for an undefined helper and a fourth hop. Typed and
awaited returned locals and a standalone helper named like a read are followed, and a nested
function's return is not. The exact-text rule names the locator for a split at the revision
(directly and through a helper), lists every reachable exact locator once, and reports UNKNOWN for
a missing helper, a fourth hop and non-literal options. Each NOT_PRODUCED reason is produced with
no finding. A change on a later line of a multi-line assertion or locator is in scope. A malformed
imported module is uncertain, and an aggregate over the shared read budget refuses. A CLI run joins a synthetic Playwright report.
No live qualification on a real suite or real Playwright run was performed.

## Delivery

| Requirement | Implementation | Focused evidence | Status |
|---|---|---|---|
| TSA-V0-001 | `internal/contextindex/setup_select.go` | `TestSelectSetupClosure`, `TestSelectSetupTraversesRegistrySeeders` | experimental |
| TSA-V0-002 | `internal/contextindex/setup_select.go` | `TestSelectSetupClosure` | experimental |
| TSA-V0-003 | `internal/contextindex/setup_select.go` | `TestSelectSetupAll`, `TestSelectSetupUniverseIncludesUnindexedFiles` | experimental |
| TSA-V0-004 | `internal/contextindex/setup_select.go` | `TestSelectSetupClosure` | experimental |
| TSA-V0-005 | `internal/contextindex/setup_select.go`, `cmd/corvint/tests.go` | `TestSelectSetupDigest`, `TestTestsCLI` | experimental |
| TSA-V0-006 | `internal/contextindex/setup_shared.go`, `internal/contextindex/setup_select.go` | `TestSelectSetupSharedConstantKeys`, `TestSelectSetupSharedConstantReviewForms` | experimental |
| TSA-V0-007 | `internal/contextindex/setup_shared.go` | `TestSelectSetupSharedConstantFailsClosed`, `TestSelectSetupSharedConstantReviewForms`, `TestSelectSetupSharedConstantOpenSetups` | experimental |
| TSA-V0-010 | `internal/testsource`, `internal/appmap/jslex_api.go` | `TestSourceModel`, `TestChangedPathsIgnoreAttributesAndDecodeNames` | experimental |
| TSA-V0-011 | `internal/appmap/jslex.go` | `TestLexJSComments` | experimental |
| TSA-V0-012 | `internal/testsource/lint.go` | `TestLintChangedLinesOnly`, `TestLintAllLinesReadsUnchangedFiles` | experimental |
| TSA-V0-013 | `internal/testsource/lint.go` | `TestLintRuleFixtures`, `TestLintRuleDetails`, `TestRenderKeepsRegexAndSubstitutions` | experimental |
| TSA-V0-014 | `internal/testsource/lint_test.go` | `TestLintRuleFixtures`, `TestLintChangedLinesOnly` | experimental |
| TSA-V0-015 | `internal/testsource/lint.go` | `TestLintSuppression` | experimental |
| TSA-V0-016 | `internal/testsource/lint.go`, `cmd/corvint/tests.go` | `TestLintTextDeterministic`, `TestTestsCLI` | experimental |
| TSA-V0-020 | `internal/testsource/duplicates.go` | `TestDuplicatesReport`, `TestRenderKeepsRegexAndSubstitutions` | experimental |
| TSA-V0-021 | `internal/testsource/duplicates.go`, `cmd/corvint/tests.go` | `TestDuplicatesGateAndDecisions`, `TestDuplicatesGateCountsBeyondOutputBound`, `TestDuplicatesGateSeesDeletionOnlyChange`, `TestTestsCLI` | experimental |
| TSA-V0-022 | `internal/testsource/duplicates.go` | `TestDuplicatesGateAndDecisions` | experimental |
| TSA-V0-023 | `internal/testsource/duplicates.go` | `TestDuplicatesReport`, `TestTestsCLI` | experimental |
| TSA-V0-029 | `internal/testsource/uistate.go` | `TestLintRuleFixtures`, `TestOneShotHelperForms`, `TestOneShotLocalsAndNesting`, `TestLintMultiLineChangeScope` | experimental |
| TSA-V0-030 | `internal/testsource/uistate.go`, `internal/testsource/git.go`, `internal/testsource/lint.go` | `TestOneShotUnknown`, `TestLintFollowsImportedHelpers`, `TestCrossVariantFollowing`, `TestLintImportedHelperUncertainty`, `TestLintImportBudgetIsCumulative` | experimental |
| TSA-V0-031 | `internal/testsource/variants.go`, `internal/testsource/uistate.go`, `cmd/corvint/tests.go` | `TestLintRuleFixtures`, `TestCrossVariantEvidence`, `TestCrossVariantFollowing`, `TestLintMultiLineChangeScope`, `TestTestsLintAdvisoryCLI` | experimental |
| TSA-V0-032 | `internal/testsource/variants.go`, `internal/testsource/lint.go`, `cmd/corvint/tests.go` | `TestCrossVariantEvidence`, `TestNewRuleSuppression`, `TestLintFollowsImportedHelpers`, `TestTestsLintAdvisoryCLI` | experimental |
| TSA-V0-033 | `internal/testsource/actor_auth.go` | `TestActorAuthorizationIssueRepro`, `TestActorAuthorizationUnknownLinks`, `TestActorAuthorizationReviewRegressions` | experimental |
| TSA-V0-034 | `internal/testsource/actor_auth_rails.go` | `TestActorAuthorizationIssueRepro`, `TestActorAuthorizationUnknownLinks`, `TestActorAuthorizationReviewRegressions` | experimental |
| TSA-V0-035 | `internal/testsource/actor_auth_rails.go` | `TestActorAuthorizationInheritance`, `TestActorAuthorizationIssueRepro`, `TestActorAuthorizationReviewRegressions` | experimental |
| TSA-V0-036 | `internal/testsource/actor_auth.go` | `TestActorAuthorizationIssueRepro`, `TestActorAuthorizationInheritance` | experimental |
| TSA-V0-037 | `internal/testsource/actor_auth.go` | `TestActorAuthorizationIssueRepro`, `TestActorAuthorizationDeterministic` | experimental |
| TSA-V0-038 | `internal/testsource/actor_auth.go`, `internal/testsource/actor_auth_rails.go` | `TestActorAuthorizationUnknownLinks`, `TestActorAuthorizationReviewRegressions` | experimental |
| TSA-V0-039 | `internal/testsource/actor_auth.go` | `TestActorAuthorizationIssueRepro`, `TestActorAuthorizationReviewRegressions` | experimental |
| TSA-V0-045 | `internal/testsource/validity.go` | `TestIsolatedInSerialForms`, `TestIsolatedInSerialTagAndQuotedKey` | experimental |
| TSA-V0-046 | `internal/testsource/validity.go` | `TestIsolatedInSerialNegatives`, `TestIsolatedInSerialTagAndQuotedKey` | experimental |
| TSA-V0-047 | `internal/testvalidity/source_rules.go`, `internal/testsource/validity.go` | `TestIsolatedInSerialForms`, `TestApplySourceFindingsAdvisoryAndExisting`, `TestValidityFindingsDispatchAndVerdict`, `TestWitnessRefusesMemoizedBlockExpectation` | experimental |
| TSA-V0-048 | `internal/testsource/validity_rspec.go` | `TestMemoizedBlockExpectationFindings`, `TestMemoizedBlockExpectationNegatives`, `TestMemoizedBlockExpectationOverrides`, `TestMemoizedBlockExpectationPercentStrings` | experimental |
| TSA-V0-049 | `internal/testsource/validity_rspec.go` | `TestMemoizedBlockExpectationFindings`, `TestMemoizedBlockExpectationNegatives`, `TestMemoizedBlockExpectationOverrides` | experimental |
| TSA-V0-050 | `internal/testsource/validity_rspec.go`, `internal/testsource/validity.go` | `TestMemoizedBlockExpectationAbstains`, `TestMemoizedBlockExpectationDepthInsideExample` | experimental |
| TSA-V0-051 | `internal/testvalidity/source_rules.go` | `TestApplySourceFindingsInvalidMakesIneligible`, `TestApplySourceFindingsAdvisoryAndExisting`, `TestValidityFindingsDispatchAndVerdict`, `TestWitnessRefusesMemoizedBlockExpectation` | experimental |
| TSA-V0-052 | `internal/testsource/validity.go` | `TestValidityFindingsDispatchAndVerdict` | experimental |
| TSA-V0-053 | `internal/testsource/validity_rspec_block.go`, `internal/testsource/validity_rspec.go` | `TestBlockNotCalledIssueExample`, `TestBlockNotCalledForms` | experimental |
| TSA-V0-054 | `internal/testsource/validity_rspec_block.go` | `TestBlockNotCalledIssueExample`, `TestBlockNotCalledForms` | experimental |
| TSA-V0-055 | `internal/testsource/validity_rspec_block.go` | `TestBlockNotCalledNegatives`, `TestBlockNotCalledAbstainsAndVerdict` | experimental |
| TSA-V0-056 | `internal/testvalidity/source_rules.go` | `TestApplySourceFindingsBlockNotCalled`, `TestBlockNotCalledAbstainsAndVerdict` | experimental |
