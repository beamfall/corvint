# Test Source Analysis V0

Owner: Russell Lewis
Date: 2026-10-17
Intent status: accepted under the owner's standing auto-accept (2026-10-11; V1-1587, V1-1589, V1-1592, V1-1698, V1-1769, V1-1783, V1-1784, V1-1824, V1-1798, V1-1787, V1-1858)
Delivery status: experimental
Authoritative inputs: V1-1587 / GitHub #1030, V1-1589 / GitHub #1032 and V1-1592 / GitHub #1035
owner requests, V1-1698 / GitHub #1082 for shared-constant keys, and V1-1769 / GitHub #1114 owner requests, V1-1783 / GitHub #1123, V1-1784 / GitHub #1124, V1-1824 / GitHub #1140 and V1-1798 / GitHub #1128 and #1130 and V1-1858 / GitHub #1159 owner requests, agent-drafted and accepted under the owner's standing auto-accept (2026-10-11);
AGENTS.md invariants 1, 2, 4, 7 and 8.

## Agent digest
- Claim: Read-only analysis of committed tests selects setup projects, lints changed lines, lists candidate duplicates and flags static test-validity defects.
- Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1587, V1-1589, V1-1592, V1-1698, V1-1769, V1-1783, V1-1784, V1-1824, V1-1798, V1-1787, V1-1858)/experimental
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
