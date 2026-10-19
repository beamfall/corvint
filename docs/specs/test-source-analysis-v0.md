# Test Source Analysis V0

Owner: Russell Lewis
Date: 2026-10-17
Intent status: accepted under the owner's standing auto-accept (2026-10-11; V1-1587, V1-1589, V1-1592, V1-1698, V1-1769, V1-1783, V1-1784, V1-1824, V1-1798, V1-1787, V1-1858, V1-1866, V1-1867, V1-1884, V1-1887, V1-1889)
Delivery status: experimental
Authoritative inputs: V1-1587 / GitHub #1030, V1-1589 / GitHub #1032 and V1-1592 / GitHub #1035
owner requests, V1-1698 / GitHub #1082 for shared-constant keys, and V1-1769 / GitHub #1114 owner requests, V1-1783 / GitHub #1123, V1-1784 / GitHub #1124, V1-1824 / GitHub #1140 and V1-1798 / GitHub #1128 and #1130 and V1-1858 / GitHub #1159, V1-1866 / GitHub #1163 and V1-1867 / GitHub #1164 and V1-1884 / GitHub #1168 and V1-1887 / GitHub #1171 and V1-1889 / GitHub #1173 owner requests, agent-drafted and accepted under the owner's standing auto-accept (2026-10-11);
AGENTS.md invariants 1, 2, 4, 7 and 8.

## Agent digest
- Claim: Read-only analysis of committed tests selects setup, lints changes, lists duplicates, flags validity defects and reports declared teardown dependents.
- Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1587, V1-1589, V1-1592, V1-1698, V1-1769, V1-1783, V1-1784, V1-1824, V1-1798, V1-1787, V1-1858, V1-1866, V1-1867, V1-1884, V1-1887, V1-1889)/experimental
- Exists: `internal/testsource`, `internal/testvalidity/source_rules.go`, `internal/contextindex/setup_select.go` and the experimental `corvint tests setup-select`, `corvint tests lint`, `corvint tests duplicates`, `corvint tests validity` and `corvint tests teardown` commands.
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

## Teardown dependents (V1-1884)

V1-1884 (GitHub #1168) flags an E2E test whose teardown deletes a parent resource while a dependent
record that one of the test's actions produced is still present, so the delete can be refused
(a 409 or 422 from a foreign-key or "in use" guard). It is agent-drafted and accepted under the
owner's standing auto-accept (2026-10-11). Which actions produce which dependents comes from a
project-owned side-effect declaration, never from a guess. Deriving that graph from the source of the
system under test is a named residual. The reading reuses the TSA-V0-010 lexer and declaration model
for JS/TS and the logical-line Ruby reader of TSA-V0-034 for RSpec.

- `TSA-V0-110`: (standing auto-accept, 2026-10-11; V1-1884 / GitHub #1168) `corvint tests teardown
  --files PATH[,PATH...] --declaration PATH [--receipt FILE ...] [--rev REV]` MUST read at most 256
  named committed test files (each at most 1 MiB) and the named committed declaration file at one
  revision (default HEAD). It MUST print one `corvint-test-teardown-dependents/0` JSON document: the
  revision, the declaration's path, blob sha256, version and pinned revision, one entry per file
  (`framework` `rspec` or `jsts`, `uncertain` with a reason where the reader abstained, the
  examples read and every example that has a resource), the findings, one entry per receipt and
  file pair, and `graphSource: "declaration"`. A `_spec.rb` file is read as RSpec and an
  `IsTestFile` JS/TS file as Jest, Vitest or Playwright; any other file is listed uncertain with
  `not-a-test-file`. It MUST exit 1 when a `declared` finding exists and 0 otherwise. At most 16
  `--receipt` files are read.
- `TSA-V0-111`: (standing auto-accept, 2026-10-11; V1-1884 / GitHub #1168) The declaration MUST be
  the closed `corvint-side-effect-declaration/0` document below, decoded with unknown fields and
  trailing data refused as `tests-declaration-invalid`. `version` is a positive integer and
  `revision` the 40- or 64-hex commit of the system under test the declaration describes; both are
  echoed and never resolved. Every `kind` matches `[a-z][a-z0-9_-]{0,63}` and is unique, every
  `create`, `delete` and `call` name is an identifier (a Ruby name may end in `?` or `!`) that
  appears once in the whole document, each resource has at least one create and one delete name,
  an action's `parent` and `dependent` name declared kinds, `dependent` requires `parent`, and
  `cascade` requires `dependent`. An action with no `parent` is declared inert: it produces no
  dependent. At most 256 resources, 1024 actions and 64 names per list are read.
- `TSA-V0-112`: (standing auto-accept, 2026-10-11; V1-1884 / GitHub #1168) The reading MUST place
  each call an example makes in one phase:
  - JS/TS: `beforeEach` (also `test.beforeEach`) bodies are setup, the example body is body, a
    `finally { }` block inside the body and `afterEach` (also `test.afterEach`) bodies are
    teardown.
  - RSpec: `before`, `before(:each)`, `before(:example)`, `prepend_before`, `append_before`, `let`,
    `let!` and `subject` bodies are setup, the example body is body, and an `ensure` clause inside
    the body and `after`, `after(:each)`, `after(:example)`, `prepend_after` and `append_after`
    bodies are teardown.
  - A hook applies to an example when it sits at file level or in a group that encloses the example,
    wherever it is declared in that group.
  - Body code, including its finally/ensure code, runs in source order before every hook; order
    inside one hook is line order, and a hook in an inner group runs before one in an outer group.
    Between two hooks of the same group the order is unknown.
  - A call is conditional when it may not run: inside a nested arrow or function, under an
    `if`/`for`/`while`/`else`/`do`/`catch` or `rescue` branch, after `&&`, `||` or `?`, or with a
    Ruby `if`/`unless`/`while`/`until`/`and`/`or` modifier, lambda, proc or inline block. `try`,
    `begin`, `finally` and `ensure` are not conditions.
  - An RSpec hook with metadata arguments, or `around`, is not read, and a hook without a block is
    opaque. A group delimited by `{` and `}` on separate lines is read like a `do … end` group; any
    other group form makes the file uncertain.
  - A call is a callee name followed by `(`; in Ruby also a declared name without parentheses and,
    in teardown, a bare name that starts a statement.
- `TSA-V0-113`: (standing auto-accept, 2026-10-11; V1-1884 / GitHub #1168) A resource of kind K MUST
  be reported for an example when a create name of K is called in its setup or body and a delete name
  of K is called in its teardown. The resource names its variable when the create's value is
  assigned to one (`x = await create…(`, `const x =`, `@x = create_…`, `let(:x) { create_… }`), the
  create line and the first teardown delete line, and lists with lines the actions after the create:
  each declared action call whose `parent` is K (in teardown, when it may run before a parent
  delete), and each other call whose receiver or argument
  names the variable (an assertion call such as `expect` or a matcher is not an action).
- `TSA-V0-114`: (standing auto-accept, 2026-10-11; V1-1884 / GitHub #1168) A resource's side-effect
  graph MUST be incomplete, with every applying reason from this closed set, when: an action on it is
  not declared (`action-not-declared`); its create is not assigned to a variable
  (`resource-not-bound`); a `beforeAll`/`afterAll`, `before(:all)`/`after(:all)`/`:context`/`:suite`
  or `around` hook or an RSpec hook with metadata arguments that applies to the example calls a
  declared name, or an RSpec hook without a block applies (`hook-not-read`); a dependent delete
  that would remove the dependent is conditional (`cleanup-conditional`); a teardown
  call before the parent delete, or in unknown order with it, is not declared
  (`teardown-call-not-declared`); the order of a relevant teardown pair is
  unknown (`teardown-order-unknown`); or the file is uncertain (`file-uncertain`). Every finding
  for an incomplete resource MUST be `advisory` and carry those reasons.
- `TSA-V0-115`: (standing auto-accept, 2026-10-11; V1-1884 / GitHub #1168) For each declared action
  on a resource whose `dependent` D does not cascade, each teardown delete of K that the action may
  run before MUST be checked. A `TEARDOWN-DEPENDENT-LEFTOVER` finding for the first failing one
  MUST name the example, the kind, the dependent kind, the action line and that parent delete line,
  with cause `dependent-not-removed` when no unconditional delete name of D is known to run after the
  action and before that parent delete, or cause `parent-deleted-before-dependent`, also giving the
  first dependent delete line after the action, when a delete of D after the action runs only after
  the parent delete or in unknown order with it. The finding is `declared` when the graph is
  complete.
- `TSA-V0-116`: (standing auto-accept, 2026-10-11; V1-1884 / GitHub #1168) There MUST be no finding
  for an action whose dependent is deleted unconditionally after the action and before every parent
  delete the action may run before, for an action declared `cascade: true`, or for an inert action.
- `TSA-V0-117`: (standing auto-accept, 2026-10-11; V1-1884 / GitHub #1168) A `--receipt` MUST be a
  test-validity provider input (`readProviderReceipt`). Only a JS provider receipt can bind; any
  other input abstains with `receipt-not-js`. A receipt binds a file when exactly one
  `identity.testFileDigests` key equals the path or ends with `/` plus it (else
  `file-not-in-receipt` or `file-ambiguous`) and its digest equals the sha256 of the committed blob
  (else `digest-mismatch`). From a bound receipt, a failed attempt (or a failed outcome without
  attempts) whose anchor names that file at a resource's parent delete line, and whose outcome
  `name` equals the example's literal title, unique in the file, MUST be attached to that
  resource's findings with the receipt, retry, line, the `DELETE` status 409 or 422 when its
  request-outcome record holds one, and the dependent kind when the resource has exactly one
  declared non-cascading dependent kind. A resource with such a row that has a status and no other
  finding MUST get an `advisory` finding with cause `recorded-teardown-conflict`. An attempt whose
  anchor file names more than one `testFileDigests` key MUST attach to no file and is counted in
  the binding's `ambiguousAnchors`.
- `TSA-V0-118`: (standing auto-accept, 2026-10-11; V1-1884 / GitHub #1168) Each resource MUST carry
  a verdict from `leftover` (a finding), `unknown` (graph incomplete) or `no-leftover-declared`
  (complete graph, no finding). The report MUST never call a teardown safe: `no-leftover-declared`
  holds only relative to the declaration, and an example without a resource has no verdict.
- `TSA-V0-119`: (standing auto-accept, 2026-10-11; V1-1884 / GitHub #1168) The command MUST read
  committed Git content and the named receipts only, write nothing, use no network, and print the
  same bytes for the same inputs. A declaration that is not a committed regular file refuses
  `tests-source-unavailable`; an unreadable receipt refuses `tests-source-unavailable`; more files
  or receipts than the bounds refuse `tests-bound-exceeded`.

```json
{"schema": "corvint-side-effect-declaration/0", "version": 1,
 "revision": "0123456789abcdef0123456789abcdef01234567",
 "resources": [
  {"kind": "order", "create": ["createOrder", "create_order"], "delete": ["deleteOrder", "delete_order"]},
  {"kind": "confirmation", "create": ["createConfirmation"], "delete": ["deleteConfirmation", "delete_confirmation"]},
  {"kind": "audit_entry", "create": ["createAuditEntry"], "delete": ["deleteAuditEntry"]}],
 "actions": [
  {"call": "confirmOrder", "parent": "order", "dependent": "confirmation"},
  {"call": "archiveOrder", "parent": "order", "dependent": "audit_entry", "cascade": true},
  {"call": "renameOrder"}]}
```

Non-goals:
- deriving the side-effect graph from the source, schema or migrations of the system under test;
- telling two resources of one kind apart, so one dependent delete clears every action of that
  kind;
- reading helper files, shared contexts, fixture finalizers or `beforeAll`/`afterAll` resources;
- a kind taken from a receiver (`record.destroy`) or a factory argument (`create(:order)`);
- joining Ruby provider documents or mapping a request route to a kind.

Failure modes:
- A call reached only through a helper, a callback stored in a variable or a dynamic name is not
  seen. A missed action leaves a leftover unreported but never makes a teardown safe.
- A Ruby method called without parentheses on the resource (`order.status`) reads as an attribute,
  not an action.
- An undeclared teardown call such as `page.close()` before the parent delete keeps findings
  advisory.
- A receipt whose anchor points into a helper, or whose test title is computed or repeated, joins
  nothing.

Acceptance evidence: synthetic fixtures only. An RSpec and a Playwright file each with a leftover
dependent, a dependent deleted first, a cascade, an inert action, a parent deleted before its
dependent, an undeclared action, an unbound create, an `around`/`beforeAll` hook and two same-group
`after` hooks; declarations with each refused shape; and a JS receipt that binds, one with another
digest and one without the file. No live qualification on a real suite was run (NOT_RUN).

Rollback removes `internal/testsource/teardown_*.go`, `internal/testvaliditydoc/js_receipt.go`,
`cmd/corvint/tests_teardown.go`, the tests, the dispatch line and help entries, the `tests-declaration-invalid` code and this section.
Nothing is persisted, and no existing report or wire contract changes.

### Traceability (V1-1884)

| Requirement | Code | Evidence | Status |
|---|---|---|---|
| TSA-V0-110 | `internal/testsource/teardown_dependents.go`, `cmd/corvint/tests_teardown.go` | `TestTeardownFrameworksAndFiles`, `TestTestsTeardownCLI` | experimental |
| TSA-V0-111 | `internal/testsource/teardown_declaration.go` | `TestTeardownDeclarationRefusals` | experimental |
| TSA-V0-112 | `internal/testsource/teardown_reading.go` | `TestTeardownPhasesJS`, `TestTeardownPhasesRSpec`, `TestTeardownSourceOrder`, `TestTeardownConditionalCleanup`, `TestTeardownSameGroupParentDeletes`, `TestTeardownRSpecHookAndGroupForms` | experimental |
| TSA-V0-113 | `internal/testsource/teardown_dependents.go` | `TestTeardownLeftoverJS`, `TestTeardownLeftoverRSpec`, `TestTeardownTeardownActions` | experimental |
| TSA-V0-114 | `internal/testsource/teardown_dependents.go` | `TestTeardownIncompleteGraph`, `TestTeardownConditionalCleanup`, `TestTeardownSameGroupParentDeletes`, `TestTeardownRSpecHookAndGroupForms` | experimental |
| TSA-V0-115 | `internal/testsource/teardown_dependents.go` | `TestTeardownLeftoverJS`, `TestTeardownLeftoverRSpec`, `TestTeardownTeardownActions`, `TestTeardownSourceOrder`, `TestTeardownConditionalCleanup` | experimental |
| TSA-V0-116 | `internal/testsource/teardown_dependents.go` | `TestTeardownLeftoverJS`, `TestTeardownLeftoverRSpec`, `TestTeardownConditionalCleanup` | experimental |
| TSA-V0-117 | `internal/testsource/teardown_receipts.go`, `cmd/corvint/tests_teardown.go` | `TestTeardownReceiptJoin`, `TestTeardownReceiptAmbiguousAnchor`, `TestTestsTeardownCLI` | experimental |
| TSA-V0-118 | `internal/testsource/teardown_dependents.go` | `TestTeardownIncompleteGraph`, `TestTeardownLeftoverJS` | experimental |
| TSA-V0-119 | `internal/testsource/teardown_dependents.go`, `cmd/corvint/tests_teardown.go` | `TestTeardownFrameworksAndFiles`, `TestTestsTeardownCLI` | experimental |

## Refusals

Every command exits 2 with one JSON object `{"code", "error", "ok": false}` on stderr and nothing on
stdout when it refuses: `tests-invalid-arguments` (usage), `tests-invalid-revision` (a revision that
is not a commit), `tests-invalid-option` (a threshold or cutoff out of range, or `--gate` without
`--base`), `tests-source-unavailable` (a Git or index read failed), `tests-bound-exceeded`,
`tests-decisions-invalid`, `tests-declaration-invalid` (TSA-V0-111) and `tests-output-unavailable`
(stdout could not be written).

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

## Intra-example stub path and value origin (V1-1887, V1-1889)

V1-1887 (GitHub #1171) and V1-1889 (GitHub #1173) add three advisory rules to the RSpec and
Jest/Vitest example readers. A first-party stub can decide the value that an assertion checks. A
spy can sit on an object that no code ever receives. An assertion can check a value that only the
test built. Each rule follows values inside one example: its body, the hooks and `let`s that apply
to it, and the declarations of its enclosing groups or describes. The rules never read the code
under test, so they cannot prove a defect. They give an advisory finding or no finding.

- `TSA-V0-120`: (standing auto-accept, 2026-10-11; V1-1887 / GitHub #1171, V1-1889 / GitHub #1173)
  Each located example of a `_spec.rb` file (TSA-V0-062) and of a JS/TS test file (TSA-V0-073)
  MUST be read for `STUB-ON-PATH`, `STUB-NEVER-INVOKED` and `ASSERTED-VALUE-NOT-FROM-SUBJECT`.
  Every finding of these rules MUST be advisory and name the test, a location line and a cause
  line. It MUST NOT make a test INELIGIBLE, and it MUST NOT change the test's hygiene or strength.
  An example's findings come after its existing findings, in this order: `STUB-ON-PATH` in stub
  order, then `STUB-NEVER-INVOKED`, then `ASSERTED-VALUE-NOT-FROM-SUBJECT` in assertion line order.
- `TSA-V0-121`: (standing auto-accept, 2026-10-11; V1-1887 / GitHub #1171) The reader MUST
  recognize these stubs:
  - RSpec: `allow(R)` or `expect(R)` with `.to receive(:m)`, optionally `allow_any_instance_of`,
    and an optional `.and_return(V)`, in the example body or in a `before` hook that applies to it.
  - Jest/Vitest: `jest.spyOn(A, 'm')` and `vi.spyOn(A, 'm')`; `mocked(X)`, `(X as T)` or `X.m`
    followed by `mockReturnValue`, `mockResolvedValue` or `mockImplementation` (each also in its
    `Once` form).

  Each receiver MUST be classified as one of four kinds:
  - subject: the subject, `described_class`, a named subject, or a `let` or local computed from them;
  - boundary: TSA-V0-123;
  - first-party: a constant, or a module the test imports by a relative path or a `@/`, `~/` or
    `src/` path;
  - unresolved: anything else.

  A receiver is fresh when the example or a `let` builds it, with `new`, a double or an object
  literal.
- `TSA-V0-122`: (standing auto-accept, 2026-10-11; V1-1887 / GitHub #1171) A stub on a first-party
  receiver MUST be reported as `STUB-ON-PATH` in either of two cases:
  - A positive assertion calls the stubbed method itself.
  - The example has an action line that names the subject, and an assertion on a value that names
    the subject expects the stub's informative return value, or the identifier it returns.

  A negated (`not_to`, `to_not`, `.not`) assertion is never evidence.

  The finding MUST name the stubbed symbol, the assertion line and the action line. An empty,
  `nil`/`null`/`undefined`, boolean, `[]` or `{}` return value is not evidence. A stub that
  TSA-V0-124 reports MUST NOT also be reported here.
- `TSA-V0-123`: (standing auto-accept, 2026-10-11; V1-1887 / GitHub #1171) A stub on an external
  boundary MUST NEVER be reported as on-path. In RSpec, a boundary is any of these:
  - the closed root list in `rspecBoundaryConsts`: HTTP clients, the clock, randomness, the
    process and file system, logging, queues, mailers and third-party SDKs;
  - an enqueue or delivery method such as `perform_later` or `deliver_later`.

  In Jest/Vitest, a boundary is any of these:
  - a module that is not first-party;
  - a TSA-V0-079 boundary module or global;
  - `Date`.

  A stub on the subject (left to TSA-V0-066/079) or on an unresolved receiver MUST give no finding.
- `TSA-V0-124`: (standing auto-accept, 2026-10-11; V1-1887 / GitHub #1171) A silent stub MUST be
  reported as the advisory static `STUB-NEVER-INVOKED` when all of these hold:
  - the stub is an RSpec `allow` or a JS spy, not a message expectation and not `any_instance`;
  - its receiver is fresh and is not the subject or a boundary;
  - in the example's scope, nothing names the receiver except its own binding, stubs on it, and
    `have_received` or `expect(...)` reads of it.

  The finding's location is the stub and its cause is the binding. The message MUST say the
  reading is static.
- `TSA-V0-125`: (standing auto-accept, 2026-10-11; V1-1887 / GitHub #1171) The run-based form of
  `STUB-NEVER-INVOKED` is not built. That form would report a stubbed method that a recorded
  passing run never called. No receipt input records RSpec or Jest/Vitest method calls; Playwright
  route stubs cover only an HTTP boundary. The run-based form MUST stay a named residual, and
  nothing may report it until such an input exists.
- `TSA-V0-126`: (standing auto-accept, 2026-10-11; V1-1889 / GitHub #1173) An assertion's actual
  value MUST be traced back through locals and `let`s, at most 4 bindings deep. It MUST be reported
  as `ASSERTED-VALUE-NOT-FROM-SUBJECT`, with the assertion line and the origin line, when both of
  these hold:
  - Every source is a value the test builds alone: a literal, an object or array literal, a
    factory `build`/`create`, a double, `Capybara.string`, a Nokogiri document, `OpenStruct`, a
    `Struct`, or `document.createElement`.
  - No such value is handed to the subject or to any other code in the example's scope.
- `TSA-V0-127`: (standing auto-accept, 2026-10-11; V1-1889 / GitHub #1173) A Jest/Vitest
  assertion on `screen` or `document` MUST be reported when all of these hold:
  - the example's scope writes the DOM itself, through `innerHTML =`, `createElement` or
    `insertAdjacentHTML`;
  - the scope has no `render`, `mount` or similar call, no subject name, no `new`, no same-file
    function call, no use of a non-test-library import binding (a namespace import's
    member call included) and no bare import.
- `TSA-V0-128`: (standing auto-accept, 2026-10-11; V1-1889 / GitHub #1173) An expectation that the
  test's own call meets MUST be reported in these forms:
  - an RSpec `have_been_enqueued`, `have_enqueued_job` or `have_enqueued_sidekiq_job` expectation
    whose job the example enqueues itself before the expectation, or inside the expectation's
    block, with the expected `with` arguments (the delay of `perform_in`/`perform_at` dropped);
  - a Jest/Vitest `toHaveBeenCalled*` expectation on a callee the example calls directly, outside
    any nested function, with the arguments the expectation repeats. An ordinal expectation
    (`toHaveBeenNthCalledWith`, `toHaveBeenLastCalledWith`), and a direct call followed by
    `mockClear`, `mockReset`, `mockRestore` or an `*AllMocks` call before the expectation, give no
    finding.
- `TSA-V0-129`: (standing auto-accept, 2026-10-11; V1-1889 / GitHub #1173) These MUST NOT be
  reported:
  - an assertion on the subject's return value or rendered output;
  - an assertion on persisted state read back through `reload`, a finder or a query;
  - a value handed to the subject or to other code;
  - a negated assertion;
  - a value from an unresolved call;
  - a constant expectation, which JS-CONSTANT-EXPECTATION already reads.

  Any trace that does not complete, because a binding cannot be resolved, a hop is too deep, a
  nesting is unknown or a delimiter is unbalanced, MUST give no finding.
- `TSA-V0-130`: (standing auto-accept, 2026-10-11; V1-1887 / GitHub #1171, V1-1889 / GitHub #1173)
  These readings MUST be deterministic over the same input, read only the given text, write nothing
  and use no network. They reach `corvint tests validity` through the existing example readers, and
  its help MUST name them. The JSON shape, exit statuses and projection rules MUST stay unchanged.

Non-goals:
- reading the code under test;
- data flow across modules, helpers or examples;
- a dependency-graph reading of what the action can reach;
- the run-based `STUB-NEVER-INVOKED` (TSA-V0-125);
- `receive_messages`, `receive_message_chain`, a `jest.fn()` assigned onto an object, and module
  factory mocks as stubs;
- Playwright-specific shapes;
- any INVALID severity.

Failure modes:
- A first-party constant or module the boundary list does not name is treated as first-party. Its
  stub can then be reported on-path, but only as advisory.
- An action that reaches a stub only through a helper is unseen. The on-path finding then rests on
  the value match alone, and a coincidental equal literal is a false advisory finding.
- A spy whose object reaches code only through dynamic access, `eval` or a global is reported as
  never invoked, but only as advisory.
- A test-built value that the subject reads through a global or a database row the factory
  persisted, without the test handing it over, is reported. `reload` and finder reads are excluded
  for this reason, but other channels are not.
- Fixtures cover synthetic files only. Real-suite precision is NOT_MEASURED.

Acceptance evidence: synthetic fixtures only.
- RSpec fixtures:
  - on-path through the result value, a direct stub call and a `let`-built collaborator;
  - boundary, clock, queue, unresolved and value-mismatch stubs that give no finding;
  - never-invoked locals and `let`s beside passed objects, message expectations and constants;
  - test-built factory, page and enqueue values beside reloaded, handed-over, subject, literal and
    unresolved values.
- The same Jest/Vitest shapes, plus a cast and object-literal spies.
- DOM written by the test beside a rendered DOM.
- Self-call and callback forms of call expectations.
- `readSourceValidity` keeps every flagged test ELIGIBLE and is deterministic.

No live qualification on a real suite was run (NOT_RUN). Rollback removes the following and the two
reader caches they use; callers then read the earlier rules unchanged, and nothing is persisted:
- `internal/testsource/validity_rspec_flow.go`, `internal/testsource/validity_js_flow.go` and
  `internal/testvalidity/source_rules_flow.go`;
- their tests;
- the two `flowFindings` calls and the `let` bodies map;
- the help sentence;
- this section.

### Traceability (V1-1887, V1-1889)

| Requirement | Code | Evidence | Status |
|---|---|---|---|
| TSA-V0-120 | `internal/testsource/validity_rspec_flow.go`, `internal/testsource/validity_js_flow.go`, `internal/testvalidity/source_rules_flow.go` | `TestFlowFindingsAdvisoryAndDeterministic`, `TestStubOnPathRSpec`, `TestStubOnPathJS` | experimental |
| TSA-V0-121 | `internal/testsource/validity_rspec_flow.go`, `internal/testsource/validity_js_flow.go` | `TestStubOnPathRSpec`, `TestStubOnPathJS`, `TestStubNeverInvokedRSpec` | experimental |
| TSA-V0-122 | `internal/testsource/validity_rspec_flow.go`, `internal/testsource/validity_js_flow.go` | `TestStubOnPathRSpec`, `TestStubOnPathJS`, `TestStubOnPathMessages`, `TestFlowReviewRSpec`, `TestFlowReviewJS` | experimental |
| TSA-V0-123 | `internal/testsource/validity_rspec_flow.go`, `internal/testsource/validity_js_flow.go` | `TestStubOnPathRSpec`, `TestStubOnPathJS` | experimental |
| TSA-V0-124 | `internal/testsource/validity_rspec_flow.go`, `internal/testsource/validity_js_flow.go` | `TestStubNeverInvokedRSpec`, `TestStubOnPathJS`, `TestFlowReviewRSpec`, `TestFlowReviewJS` | experimental |
| TSA-V0-125 | none (residual) | `TestStubNeverInvokedRSpec` (static message) | residual |
| TSA-V0-126 | `internal/testsource/validity_rspec_flow.go`, `internal/testsource/validity_js_flow.go` | `TestAssertedValueNotFromSubjectRSpec`, `TestAssertedValueNotFromSubjectJS`, `TestFlowReviewRSpec`, `TestFlowReviewJS` | experimental |
| TSA-V0-127 | `internal/testsource/validity_js_flow.go` | `TestAssertedValueNotFromSubjectJS`, `TestFlowReviewJS` | experimental |
| TSA-V0-128 | `internal/testsource/validity_rspec_flow.go`, `internal/testsource/validity_js_flow.go` | `TestAssertedValueNotFromSubjectRSpec`, `TestAssertedValueNotFromSubjectJS`, `TestFlowReviewJS` | experimental |
| TSA-V0-129 | `internal/testsource/validity_rspec_flow.go`, `internal/testsource/validity_js_flow.go` | `TestAssertedValueNotFromSubjectRSpec`, `TestAssertedValueNotFromSubjectJS`, `TestJSExampleAssertions`, `TestFlowReviewRSpec` | experimental |
| TSA-V0-130 | `internal/testsource/validity_command.go`, `cmd/corvint/tests.go` | `TestFlowFindingsAdvisoryAndDeterministic`, `TestTestsValidityCLI` | experimental |

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

## Seed parity between linked tests (V1-1883)

V1-1883 (GitHub #1167) compares the setup a new test performs with the setup of the legacy test it
claims to preserve. Two tests can share a title and assertions while seeding different data: three
children against one, an attribute set on one side and left at its default on the other, or a total
asserted over a single record. It is agent-drafted and accepted under the owner's standing
auto-accept (2026-10-11). The reading reuses the TSA-V0-062 RSpec reader and the TSA-V0-010 lexer
and declaration model. The link is an explicit argument; no link store is added.

- `TSA-V0-100`: (standing auto-accept, 2026-10-11; V1-1883 / GitHub #1167) `corvint tests
  validity` MUST accept a repeatable `--legacy NEW=LEGACY` link (`SourceValidityOptions.Links`).
  - Each side is `PATH:LINE`: a repository-relative committed file and the declaring line of one
    example. `--files` may be omitted when a link is given.
  - A side that is not `PATH:LINE` with a positive line refuses `tests-invalid-arguments`; a
    malformed `NEW=LEGACY` value exits 2. A file that is not committed at the revision refuses
    `tests-source-unavailable`. More than 256 links refuses `tests-bound-exceeded`.
- `TSA-V0-101`: (standing auto-accept, 2026-10-11; V1-1883 / GitHub #1167) Each side's setup MUST
  be read from committed Git content in exactly these forms:
  - RSpec (`_spec.rb`): FactoryBot `create`, `create_list`, `build` and `build_list`, bare or as
    `FactoryBot.`, with a literal kind, a literal list count, `:trait` and `key: value` or
    `:key => value` arguments. They are read in the example body, the `before`/`before(:each)`
    hooks, `let!` and `subject!` of its groups, and the lazy `let` and `subject` bodies that scope
    invokes, transitively and innermost first. `subject` resolves to a named subject
    (`subject(:user) { ... }`), and a `let!` runs the innermost definition of its name, so an inner
    `let` overriding an outer `let!` counts once, with the inner body.
  - RSpec YAML fixtures: a `fixtures :table, ...` line of the example's groups reads
    `spec/fixtures/TABLE.yml`, else `test/fixtures/TABLE.yml`. Each top-level record is one seed
    of the singular kind with its one-level `key: value` attributes.
  - JS/TS (`IsTestFile`): `create`, `build`, `createList` and `buildList`, bare or on one receiver
    (`factory.create('user', {...})`), with a literal string kind, a literal list count and an
    optional object literal of `key: value` attributes. They are read in the example body and the
    `beforeEach`/`beforeAll` hooks (bare or `test.`) that apply to it.
  - Only a call that runs whenever its body runs is read. A call nested in a block, lambda or
    callback, or in a condition or short-circuit operand of its statement (`if`, `unless`, a
    modifier, `&&`, `||`, `?:`, an arrow that is not invoked), abstains per TSA-V0-107.
  - Setup in another example, a non-applying hook, or a `def` helper scope never invokes, is not
    read.
- `TSA-V0-102`: (standing auto-accept, 2026-10-11; V1-1883 / GitHub #1167) Each analysed side MUST
  be normalised to a sorted list of kinds. Each kind has its summed count and each explicitly set
  attribute with its sorted distinct values.
  - Names are lower snake_case. A trait is attribute `trait:NAME` = `true`.
  - A symbol or string is a quoted string; a number or boolean is itself; `nil`, `null` and
    `undefined` are `null`; any other expression, and a nested YAML value, is `<expr>`.
- `TSA-V0-103`: (standing auto-accept, 2026-10-11; V1-1883 / GitHub #1167) The report MUST carry a
  `seedParity` member, one entry per link in argument order, only when a link is given. Each entry
  has `new`, `legacy`, `status`, `sides` and `findings`.
  - `sides` holds the legacy then the new side: `side`, `test`, `analysed`, `reason` and
    `location` when not analysed, and `kinds`.
  - `status` is `parity` when both normalised shapes are identical, with no divergence or
    uncertain finding; `diverges` when they differ; `uncertain` when the only differences are
    attribute values that hold `<expr>` (TSA-V0-105); `not-analysed` per TSA-V0-107. Parity means equal setup shapes
    only, never equal behaviour.
- `TSA-V0-104`: (standing auto-accept, 2026-10-11; V1-1883 / GitHub #1167) A kind seeded on one
  side only, or with different counts, MUST give an advisory `SEED-PARITY-DIVERGES` finding.
  - Its message names both counts and, per side, the asserting lines that read the kind: an
    identifier equal to the kind or its plural, or a name the seed is bound to (a let, a local or
    a destructured name).
  - Findings that an asserting line reads order first, then by kind and attribute. `location` is
    the new side's first seed of the kind (else the new test) and `cause` the legacy side's.
- `TSA-V0-105`: (standing auto-accept, 2026-10-11; V1-1883 / GitHub #1167) For a kind both sides
  seed, an attribute set explicitly on one side and left at its default on the other MUST give a
  `SEED-PARITY-DIVERGES` finding naming the side and its values. An attribute both sides set to
  literal values only, with different value sets, also gives one. An attribute both sides set with
  different value sets where either holds `<expr>` MUST instead give an advisory
  `SEED-PARITY-UNCERTAIN` finding naming both value sets; it is never parity. Equal value sets,
  `<expr>` included, give no finding.
- `TSA-V0-106`: (standing auto-accept, 2026-10-11; V1-1883 / GitHub #1167) An asserting line on
  either side whose identifiers hold an aggregate word (`total`, `sum`, `all`, `every`, `reduce`,
  `inject`, `average`, `balance`, `aggregate`) and read a group of a kind MUST give an advisory
  `SEED-AGGREGATE-SINGLE-MEMBER` finding when either side seeds exactly one of that kind.
  - A group is the kind's plural identifier or a plural bound name.
  - It is reported on each side whose assertion qualifies, at the asserting line, with `cause` the
    first single side's seed.
- `TSA-V0-107`: (standing auto-accept, 2026-10-11; V1-1883 / GitHub #1167) A side MUST abstain,
  naming its first reason and `path:line`, when:
  - the file is neither `_spec.rb` nor a JS/TS test file: `seed-file-unsupported`;
  - the reader is uncertain or unbalanced: `seed-source-uncertain`;
  - no single example is declared on the line: `seed-example-not-found`;
  - a setup-like identifier (a create, build, insert, upsert, seed, fixture, factory or populate
    word) in scope is not a supported form, including a block-taking call: `seed-form-unsupported`;
  - setup-like code runs where scope is not resolved (`before(:all)`, `around`, `after`, an invoked
    `def`, or a JS file-level call): `seed-scope-unresolved`;
  - a factory call in scope may not run (TSA-V0-101): `seed-execution-unresolved`;
  - the kind, list count or attributes are not literal: `seed-kind-not-literal`,
    `seed-count-not-literal`, `seed-attributes-not-literal`;
  - a fixture file is missing (`seed-fixture-unavailable`), not of the plain shape
    (`seed-fixture-unsupported`: ERB, tabs, anchors, aliases, merge keys, lists, `fixtures :all`),
    or over the bound (`seed-fixture-bound-exceeded`). A `spec/fixtures` file that is over the
    bound or not a regular file abstains; `test/fixtures` is read only when it is absent.
  When either side abstains, `status` MUST be `not-analysed`. There is one advisory
  `SEED-PARITY-NOT-ANALYSED` finding per unanalysed side and no divergence or aggregate finding.
- `TSA-V0-108`: (standing auto-accept, 2026-10-11; V1-1883 / GitHub #1167) Seed-parity findings
  MUST be advisory and carry the new test as `test`. They MUST NOT add a `tests` entry, change a
  hygiene or strength axis, or change the exit code.
- `TSA-V0-109`: (standing auto-accept, 2026-10-11; V1-1883 / GitHub #1167) The reading MUST be
  deterministic. It MUST read only committed Git content: each side at most 1 MiB, and at most 16
  fixture tables per side, each at most 1 MiB. It MUST write nothing and use no network.

Non-goals:
- reusing the documentation-corpus legacy crosswalk (it needs a behaviour registry and inventory);
  a link store;
- telling `build` from `create` persistence, factory defaults, factory traits' contents, sequences
  or associations a factory adds;
- helpers, page objects or shared contexts in other files; `let_it_be`, `before(:all)` and
  database seeds files;
- deciding that the asserted value depends on the divergence: ordering by a reading assertion is a
  hint, not proof.

Failure modes:
- A setup-like word the forms do not cover abstains the side, so most real suites will report
  `not-analysed` until more forms are added; this is the intended failure direction.
- A helper whose name holds no setup word (`make_order`, `given_cart`) is not seen. Its records are
  missing from the shape, so parity can be reported when the helper seeds differently.
- The plural and aggregate readings are English-word heuristics; a miss leaves a finding
  unreported, never a side mis-analysed.

Acceptance evidence: synthetic fixtures only (`internal/testsource/seed_parity_test.go`,
`cmd/corvint/tests_test.go`).
- A 3 vs 1 child count with the asserting lines of both sides.
- An explicit vs default attribute and differing literal values.
- A sum/reduce aggregate over a single-member seed, reported on both sides.
- Each abstention reason on each side, with no parity reported, including conditional and
  deferred calls and an oversized fixture with a fallback file present.
- An unknown value against a literal reported uncertain; a named subject, an overridden `let!`,
  a commented quoted YAML scalar and a `.should` aggregate chain.
- Identical shapes written differently, with no finding.
- YAML fixtures, including ERB and missing files; the JS/TS form; and the CLI flag and exit code.

No live qualification on a real migration was run (NOT_RUN). Rollback removes
`internal/testsource/seed_parity*.go`, `internal/testvalidity/source_rules_seed.go`, the `Links`
option and `seedParity` member, the `lets`/`fixtures` fields of `rspecGroup`, the `--legacy` flag,
their tests and this section. Nothing is persisted.

### Traceability (V1-1883)

| Requirement | Code | Evidence | Status |
|---|---|---|---|
| TSA-V0-100 | `internal/testsource/validity_command.go`, `internal/testsource/seed_parity.go`, `cmd/corvint/tests.go` | `TestTSAV0100SeedLinkArguments`, `TestTSAV0108SeedParityCLI` | experimental |
| TSA-V0-101 | `internal/testsource/seed_parity_rspec.go`, `internal/testsource/seed_parity_jsts.go`, `internal/testsource/validity_rspec.go` | `TestTSAV0101SeedShapeRSpec`, `TestTSAV0101SeedYAMLFixtures`, `TestTSAV0105SeedExplicitVersusDefault`, `TestTSAV0107SeedConditionalAbstains`, `TestTSAV0101SeedNamedSubject`, `TestTSAV0101SeedOverriddenEagerLet`, `TestTSAV0101SeedYAMLQuotedComment` | experimental |
| TSA-V0-102 | `internal/testsource/seed_parity.go` | `TestTSAV0102SeedNormalisation`, `TestTSAV0101SeedShapeRSpec` | experimental |
| TSA-V0-103 | `internal/testsource/seed_parity.go`, `internal/testvalidity/source_rules_seed.go` | `TestTSAV0103SeedIdenticalShapesParity`, `TestTSAV0104SeedCountDivergence`, `TestTSAV0103SeedExprVersusLiteralUncertain` | experimental |
| TSA-V0-104 | `internal/testsource/seed_parity.go` | `TestTSAV0104SeedCountDivergence`, `TestTSAV0105SeedExplicitVersusDefault` | experimental |
| TSA-V0-105 | `internal/testsource/seed_parity.go` | `TestTSAV0105SeedExplicitVersusDefault`, `TestTSAV0101SeedYAMLFixtures`, `TestTSAV0103SeedExprVersusLiteralUncertain` | experimental |
| TSA-V0-106 | `internal/testsource/seed_parity.go` | `TestTSAV0106SeedAggregateSingleMember`, `TestTSAV0106SeedShouldAggregate` | experimental |
| TSA-V0-107 | `internal/testsource/seed_parity.go`, `internal/testsource/seed_parity_rspec.go`, `internal/testsource/seed_parity_jsts.go` | `TestTSAV0107SeedNotAnalysed`, `TestTSAV0101SeedYAMLFixtures`, `TestTSAV0107SeedConditionalAbstains`, `TestTSAV0107SeedFixtureOversizeNoFallback` | experimental |
| TSA-V0-108 | `internal/testsource/validity_command.go`, `cmd/corvint/tests.go` | `TestTSAV0108SeedParityAdvisoryOnly`, `TestTSAV0108SeedParityCLI` | experimental |
| TSA-V0-109 | `internal/testsource/seed_parity.go`, `internal/testsource/seed_parity_rspec.go` | `TestTSAV0101SeedYAMLFixtures`, `TestTSAV0100SeedLinkArguments` | experimental |

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

## Unselected test files (V1-1888)

V1-1888 (GitHub #1172) reports committed test-looking JS/TS files that no configured runner selects,
and keeps them out of witness credit. A Playwright spec in a Jest-only repository, or a file a
runner's exclude pattern drops, looks like coverage but never runs. It is agent-drafted and accepted
under the owner's standing auto-accept (2026-10-11). The reading reuses the TSA-V0-010 lexer and
declaration model; it runs nothing and reads committed Git content only.

- `TSA-V0-140`: (standing auto-accept, 2026-10-11; V1-1888 / GitHub #1172)
  `testsource.UnselectedFiles` MUST read, at one resolved revision, every committed runner
  configuration file and every test-looking file under the TSA-V0-012 bounds, refusing
  `tests-bound-exceeded` rather than reading part of the repository.
  - A test-looking file is a JS/TS file (TSA-V0-010 extensions, not `.d.ts`) outside
    `node_modules` whose stem ends with `.spec`, `.test`, `_spec`, `_test`, `-spec`, `-test`,
    `.specs`, `.tests`, `.e2e` or `.cy`, or that sits under a directory named `__tests__`,
    `__test__`, `test`, `tests`, `spec`, `specs`, `e2e`, `integration` or `cypress`, and that
    declares at least one test or describe call.
  - Other files are not listed.
- `TSA-V0-141`: (standing auto-accept, 2026-10-11; V1-1888 / GitHub #1172) The modelled runners are
  Jest, Vitest, Playwright and Mocha. A runner instance exists in a directory for each
  `jest.config.*`, `vitest.config.*`, `vite.config.*` with a `test` property (when no
  `vitest.config.*` sits beside it), `playwright.config.*`, `.mocharc.*`, package.json `jest` or
  `mocha` member, or package.json dependency on `jest`, `vitest`, `@playwright/test` or `mocha`.
  - With no configuration file the instance uses the runner's defaults:
    - Jest: `testMatch` `**/__tests__/**/*.?([mc])[jt]s?(x)` and
      `**/?(*.)+(spec|test).?([mc])[jt]s?(x)`; `testPathIgnorePatterns` `/node_modules/`.
    - Vitest: `include` `**/*.{test,spec}.?(c|m)[jt]s?(x)`; `exclude` `**/node_modules/**` and
      `**/.git/**`.
    - Playwright: `testDir` the configuration directory; `testMatch`
      `**/*.@(spec|test).?(c|m)[jt]s?(x)`.
    - Mocha: `spec` `./test`, non-recursive, extensions `js`, `cjs` and `mjs`.
  - Jest matches globs against `<rootDir>/path` and its ignore patterns as regular expressions.
  - Playwright matches `testMatch` and `testIgnore` case-insensitively against the absolute path,
    directory components included. A pattern without a leading `**/` gets one, an absolute
    pattern is outside the reading, and `testDir` moves the instance's directory.
  - A Mocha spec without glob characters selects that file, that path with a configured
    extension, or the files with a configured extension in that directory (recursively when set).
    Mocha leaves out a path with a hidden segment unless the pattern names a segment that starts
    with a literal `.`; the other runners match hidden paths.
  - A dependency on `cypress`, `karma`, `jasmine`, `ava`, `@web/test-runner`, `@wdio/cli`, `tap`,
    `uvu`, `@japa/runner`, `@storybook/test-runner` or `testcafe`, or one of their configuration
    files, a `vitest.workspace` or `vitest.projects` file, or an unparsable package.json, is
    unmodelled runner evidence. A `vitest.workspace` or `vitest.projects` file also makes every
    Vitest instance uncertain (`runner-config-project-selection`), and one exists in its
    directory.
- `TSA-V0-142`: (standing auto-accept, 2026-10-11; V1-1888 / GitHub #1172) A configuration MUST be
  read only as literal values: strings, arrays of strings and `true`/`false` for the selection keys
  (Jest `testMatch`, `testPathIgnorePatterns`; Vitest `test.include`, `test.exclude`; Playwright
  `testDir`, `testMatch`, `testIgnore`; Mocha `spec`, `ignore`, `exclude`, `extension`,
  `recursive`). Other keys are ignored. The instance is uncertain, with that reason, on:
  - a spread, `mergeConfig` or `defineWorkspace` (`runner-config-spread`, `runner-config-merged`);
  - a Jest `testRegex`, `roots`, `rootDir`, `projects` or `testPathPattern` key, a Vitest
    `projects`, `workspace`, `root` or `dir` key, or a Mocha `config` or `package` key
    (`runner-config-key-unsupported`);
  - a repeated or non-literal selection key, or a non-object Vitest `test`
    (`runner-config-key-repeated`, `runner-config-value-dynamic`);
  - a computed property, or a selection key name (or Vitest `test`, Playwright `projects`) that
    is not a `key:` property, such as a shorthand `{ testMatch }`, `module.exports.testMatch = …`
    or a variable (`runner-config-key-dynamic`);
  - a Playwright selection key inside `projects` (`runner-config-project-selection`);
  - unparsable JSON, YAML, or a Vitest JSON file (`runner-config-unparsed`);
  - two configurations for one runner in one directory (`runner-config-multiple`);
  - a pattern outside the glob dialect (`runner-pattern-unsupported`). Braces `{a,b}` and numeric
    ranges `{1..3}` expand first, to at most 256 patterns. Then the dialect is `**` as a whole
    path segment (elsewhere it is `*`), `*`, `?`, `[...]` with `!` negation and the extglobs
    `?(…)`, `*(…)`, `+(…)` and `@(…)`. A negated pattern, `!(…)`, a `**` inside an extglob, a
    letter, stepped or zero-padded range, a brace without a comma, an escape or an unbalanced
    bracket is outside it.
- `TSA-V0-143`: (standing auto-accept, 2026-10-11; V1-1888 / GitHub #1172) A file's framework MUST
  be attributed from its literal imports and requires:
  - `@playwright/test` or `playwright/test` is Playwright, `vitest` is Vitest, `@jest/globals` is
    Jest and `mocha` is Mocha;
  - `node:test`, `bun:test`, `ava`, `tap`, `uvu`, `@japa/runner` or `cypress` is unmodelled;
  - more than one of these is ambiguous;
  - otherwise, a test or describe root the file calls (or `expect`) bound from another module is
    indirect, and anything else uses globals.
  - An explicit framework runs only under its runner. Globals run under Jest, Vitest or Mocha, and
    indirect under any modelled runner.
- `TSA-V0-144`: (standing auto-accept, 2026-10-11; V1-1888 / GitHub #1172) Each test-looking file
  MUST get one selection:
  - `selected` when any compatible runner instance that is not uncertain selects it: the file is
    under the instance's directory, matches an include pattern and matches no exclude pattern.
    Selected files MUST NOT be reported.
  - `uncertain`, never reported, when:
    - the file is lexically uncertain (`source-unparsed`);
    - its framework is ambiguous or unmodelled (`framework-ambiguous`, `framework-unmodelled`);
    - there is no runner evidence at all (`no-runner-configuration`);
    - it uses globals or an indirect import and unmodelled runner evidence exists
      (`runner-unmodelled`);
    - a compatible instance is uncertain (that instance's reason).
  - `unselected` otherwise, with reason:
    - `no-runner-for-framework` when no compatible instance exists;
    - otherwise the first, in this order, that any compatible instance gives:
      - `excluded` with the exclude pattern that matched;
      - `suffix-mismatch` when a file in the same directory named `corvintprobe.test`,
        `corvintprobe.spec` or `corvintprobe`, with the file's extension, `.js` or `.ts`, would be
        selected;
      - `outside-include`.
    - The finding names the runner and configuration that gave the reason.
- `TSA-V0-145`: (standing auto-accept, 2026-10-11; V1-1888 / GitHub #1172)
  `corvint tests unselected [--rev REV]` MUST print one `corvint-test-file-selection/0` document:
  - `revision`;
  - `runners` (runner, config, directory, status `read`, `defaults` or `uncertain`, reason,
    include and exclude);
  - `unmodelledRunners`;
  - `files`, the inventory of every test-looking file with its framework and selection;
  - `findings`, one `TEST-FILE-UNSELECTED` per unselected file, carrying path, framework, reason,
    runner, config and pattern.
  - It MUST exit 1 when there is a finding and 0 otherwise. A refusal exits 2 with the TSA-V0-012
    codes.
  - The output is deterministic over the same revision.
- `TSA-V0-146`: (standing auto-accept, 2026-10-11; V1-1888 / GitHub #1172) `corvint tests
  validity` (`testsource.SelectedSourceValidity`) MUST give every test of a named file that
  TSA-V0-144 classifies `unselected` an `invalid` `TEST-FILE-UNSELECTED` finding, so its hygiene
  is `INELIGIBLE` with that reason and it cannot be witness or coverage credit. The finding's
  cause is the reason and, when present, `: pattern`.
  - The tests are the ones the report already lists for the file, plus one per test declaration
    it does not list.
  - An `uncertain` or `selected` file, a `_spec.rb` file and a file that is not test-looking are
    unchanged.

Non-goals:
- running a runner, reading CLI flags, environment-dependent configuration or a workspace's
  per-project selection;
- RSpec, Go and other languages, and runners other than the four modelled;
- Jest `testRegex`, `roots` and `rootDir`, Vitest `root`/`dir` and projects, and Playwright
  per-project selection, which abstain;
- carrying the flag into the doc-corpus legacy crosswalk or the Playwright witness import, which
  read receipts rather than this report (a V1-1888 residual).

Failure modes:
- A runner configured only through a CLI flag, a script argument or an environment variable is
  not seen. Its files are reported against the defaults or another runner.
- An import alias of a test root, or a fixture re-exported through a barrel that the file does not
  bind by name, is read as globals, so a Playwright-only repository can report it as
  `no-runner-for-framework`.
- A package with a Vitest dependency and a `vite.config.*` without a `test` property uses Vitest
  defaults.
- Every configuration and test-looking file counts against the TSA-V0-012 bounds, so a very large
  repository refuses `tests-bound-exceeded` instead of reporting.

Acceptance evidence: synthetic Git repositories only.
- A Jest-plus-Vitest repository reports a Playwright spec as `no-runner-for-framework`, a file
  under an ignored directory as `excluded` naming `/legacy/`, a `.tests.js` file as
  `suffix-mismatch` and a Vitest file outside `include` as `outside-include`. Selected files and
  non-test-looking files are not reported, and `coverage.include` does not count.
- Playwright `testDir`/`testIgnore` and a Mocha `.mocharc.json` spec are read.
- Playwright matches the absolute path case-insensitively, Mocha skips hidden files and selects
  an explicitly named file outside `extension`, brace ranges expand, an inline `**` stays in one
  segment, and a Vitest workspace makes an explicit Vitest file uncertain.
- Each abstain reason in TSA-V0-142 leaves files `uncertain` with no finding, as do no runner
  evidence, an unmodelled runner and a `node:test` file.
- The glob dialect is pinned.
- `tests validity` on an unselected file is refused with `TEST-FILE-UNSELECTED`, and a selected
  file is unchanged.
- The CLI exits 1, 0 and 2.

No live qualification on a real suite was run (NOT_RUN). Rollback removes
`internal/testsource/unselected*.go`, `cmd/corvint/tests_unselected*.go`, the `unselected`
dispatch and help, and this section, and restores `runTestsValidity` to call `SourceValidity`.
Nothing is persisted.

### Traceability (V1-1888)

| Requirement | Code | Evidence | Status |
|---|---|---|---|
| TSA-V0-140 | `internal/testsource/unselected.go` | `TestUnselectedReasonsTSAV0144`, `TestUnselectedRevisionRefusalTSAV0145` | experimental |
| TSA-V0-141 | `internal/testsource/unselected_runners.go` | `TestUnselectedReasonsTSAV0144`, `TestUnselectedPlaywrightAndMochaTSAV0141`, `TestUnselectedRunnerSemanticsTSAV0141` | experimental |
| TSA-V0-142 | `internal/testsource/unselected_runners.go`, `internal/testsource/unselected_glob.go` | `TestUnselectedAbstainsTSAV0142`, `TestUnselectedGlobDialectTSAV0142`, `TestUnselectedRunnerSemanticsTSAV0141` | experimental |
| TSA-V0-143 | `internal/testsource/unselected.go` | `TestUnselectedIndirectFrameworkTSAV0143`, `TestUnselectedNoEvidenceTSAV0144` | experimental |
| TSA-V0-144 | `internal/testsource/unselected.go`, `internal/testsource/unselected_runners.go` | `TestUnselectedReasonsTSAV0144`, `TestUnselectedNoEvidenceTSAV0144`, `TestUnselectedPlaywrightAndMochaTSAV0141` | experimental |
| TSA-V0-145 | `internal/testsource/unselected.go`, `cmd/corvint/tests_unselected.go` | `TestTestsUnselectedCLI`, `TestUnselectedRevisionRefusalTSAV0145` | experimental |
| TSA-V0-146 | `internal/testsource/unselected.go`, `cmd/corvint/tests.go` | `TestSelectedSourceValidityRefusesUnselectedTSAV0146`, `TestTestsUnselectedCLI` | experimental |

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
