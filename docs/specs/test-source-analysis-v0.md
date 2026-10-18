# Test Source Analysis V0

Owner: Russell Lewis
Date: 2026-10-17
Intent status: accepted under the owner's standing auto-accept (2026-10-11; V1-1587, V1-1589, V1-1592)
Delivery status: experimental
Authoritative inputs: V1-1587 / GitHub #1030, V1-1589 / GitHub #1032 and V1-1592 / GitHub #1035
owner requests, agent-drafted and accepted under the owner's standing auto-accept (2026-10-11);
AGENTS.md invariants 1, 2, 4, 7 and 8.

## Agent digest
- Claim: Read-only analysis of committed JS/TS tests selects needed setup projects, lints changed test lines and lists candidate duplicate tests.
- Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1587, V1-1589, V1-1592)/experimental
- Exists: `internal/testsource`, `internal/contextindex/setup_select.go` and the experimental `corvint tests setup-select`, `corvint tests lint` and `corvint tests duplicates` commands.
- Blocked on: an MCP tool for setup-select and a preflight lint hookup are not built; real-suite qualification is NOT_RUN.
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
  does not declare), `registry-unresolved`, `missing-path` (a spec that is not a clean, tracked JS/TS
  path at the revision), `empty-spec-list`, `import-unresolved`, `source-unreadable` or
  `bound-exceeded` (at most 10 000 files in the closure, 100 reasons).
- `TSA-V0-004`: (standing auto-accept, 2026-10-11; V1-1587) A comment-only mention of an import or a
  registry key MUST NOT select anything (TSA-V0-011).
- `TSA-V0-005`: (standing auto-accept, 2026-10-11; V1-1587) The JSON output (schema
  `corvint-tests-setup-select/0`) MUST record the full revision, the specs, the universe size and an
  `inputDigest` (`sha256:` over the sorted path and blob of every file read, the registries and the
  application-map manifests), deterministic across calls. `--format lines` MUST print only the setup
  paths, one per line, usable as a test provider's setup project list.

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
  a skip/fixme without a reason string).
- `TSA-V0-014`: (standing auto-accept, 2026-10-11; V1-1589) Every rule MUST have a positive fixture
  that fires it and a negative fixture that does not.
- `TSA-V0-015`: (standing auto-accept, 2026-10-11; V1-1589) A comment `corvint-lint-allow RULE:
  REASON` on the finding's line, or alone on the line above it, MUST suppress exactly RULE there and
  nothing else. One without a reason (`missing-reason`) or naming an unknown rule (`unknown-rule`)
  suppresses nothing. Every in-scope or applied suppression MUST be echoed with whether it was used.
- `TSA-V0-016`: (standing auto-accept, 2026-10-11; V1-1589) `--json` output (schema
  `corvint-tests-lint/0`) MUST be byte-identical across runs over the same inputs. Exit status MUST be
  0 with no findings, 1 with findings and 2 on any refusal.

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

### Actor authorization (V1-1770)

Ticket V1-1770, GitHub #1115. A test that logs in as a fixture actor whose role does not hold the
ability its request needs fails with a 403 that reads like an application bug. This slice finds
that mismatch statically, before a run, for a literal subset of one stack: JS/TS fixtures and tests
on the client side, and Rails routes, controllers and a CanCanCan-style `Ability` class on the
server side. It is library-only (`testsource.ActorAuthorization`); it reads no declared role table,
because the literal Ruby forms below are parsed directly. It lives in this spec rather than the
test-validity or application-map specs because it is a static read of committed test source, like
lint and duplicates, and it projects no runtime receipt.

- `TSA-V0-030`: (standing auto-accept, 2026-10-11; V1-1770 / GitHub #1115) Actor-role resolution
  MUST bind a request to a fixture actor only through literal forms. A fixture is an exported
  `function NAME(` or `const NAME = (...) =>` builder in a non-test JS/TS file whose body calls a
  user factory: a call whose callee chain names `user`, or whose arguments carry a `role:` key. The
  role is that key's string literal, or else the one literal `role = '...'` default among the
  factory's definitions. The fixture's single top-level `return { ... }` maps each key to the seeded
  user its value names (the user's variable, else an identifier passed to exactly one factory call).
  In a test, `const V = [await] FIXTURE(...)` binds V, and the latest call whose name contains
  `login` or `signin` before the request, in the same test or before it outside every test, sets
  the actor from its `V.KEY` argument.
- `TSA-V0-031`: (standing auto-accept, 2026-10-11; V1-1770 / GitHub #1115) Request-to-ability
  resolution MUST read a request as `X.get|post|put|patch|delete('/path')` inside a test, where a
  template substitution may fill a whole path segment and then matches only a dynamic route
  segment. The first matching route in `config/routes.rb` is used, from these literal forms:
  `Rails.application.routes.draw do`, `namespace :x do`, `resources :x` with `only:`/`except:`
  symbol lists and nesting, `<verb> 'path', to: 'c#a'` (or `=>`) and `root`. The ability is the one
  literal `authorize! :action, Constant` in that action of `app/controllers/<controller>_controller.rb`.
- `TSA-V0-032`: (standing auto-accept, 2026-10-11; V1-1770 / GitHub #1115) Rule-set evaluation MUST
  read `app/models/ability.rb` as a `class Ability` whose `initialize` calls `send(user.role)` (or
  `public_send`, optionally `.to_sym`), where each argument-free method is the rule set of the role
  it names, and a bare call to another rule set inside it is inheritance. A role holds an ability
  when an unconditional `can` in its inheritance closure or in `initialize` matches it: the action
  is equal, `:manage`, or a CanCanCan default alias (`read` for index/show, `create` for new,
  `update` for edit), and the subject is equal or `:all`. A role is denied only when its whole
  closure parsed and nothing matches.
- `TSA-V0-033`: (standing auto-accept, 2026-10-11; V1-1770 / GitHub #1115) A denied request MUST be
  reported with status `UNAUTHORIZED` and code `ACTOR-UNAUTHORIZED`. The report names the test file,
  line and title, the method and path, the actor (fixture, key, role, where the role was seeded),
  the route and the check with their file:line, and `grantedBy`, the granting rule file:line of each
  lowest sufficient role. A lowest sufficient role holds the ability and inherits no other role
  that holds it. The report also lists `alternatives`, every other fixture actor whose resolved
  role holds the ability.
- `TSA-V0-034`: (standing auto-accept, 2026-10-11; V1-1770 / GitHub #1115) The report (schema
  `corvint-tests-actor-authorization/0`) MUST read committed objects only at one revision, within
  the package's `MaxFiles` and `MaxFileBytes` bounds. It MUST list the closed set of supported input
  forms and every resolved fixture actor. It MUST be deterministic over the same inputs. Its verdict
  is `ACTOR_UNAUTHORIZED` when any request is unauthorized, else `UNKNOWN` when any is unknown, else
  `CLEAN`. An `AUTHORIZED` or `EXEMPT` request does not change the verdict.
- `TSA-V0-035`: (standing auto-accept, 2026-10-11; V1-1770 / GitHub #1115) Every link that does not
  resolve MUST make the request `UNKNOWN`, never authorized or unauthorized. The reason names the
  first failing link, with a file:line detail, from this closed set:
  - actor links: `actor-unbound`, `actor-not-literal`, `actor-key-not-user`, `actor-key-ambiguous`,
    `fixture-return-not-literal`, `fixture-ambiguous`, `role-not-literal`, `role-ambiguous`,
    `role-default-unresolved`;
  - the request path: `path-not-literal`;
  - route links: `routes-unavailable`, `route-not-found`, `route-unparsed`;
  - controller links: `controller-unavailable`, `action-not-found`, `check-not-found`,
    `check-not-literal`, `check-implicit`, `check-multiple`;
  - ability links: `ability-unavailable`, `role-dispatch-not-literal`, `role-undefined`,
    `inheritance-cycle`, `rule-conditional`, `rule-cannot`, `statement-unparsed`.
- `TSA-V0-036`: (standing auto-accept, 2026-10-11; V1-1770 / GitHub #1115) A test that asserts a
  401, 403 or 422 status with `toBe`, `toEqual`, `toStrictEqual` or `toHaveStatus` on a numeric
  literal MUST have every request reported `EXEMPT` with reason `asserts-rejection` and the
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

## Refusals

Every command exits 2 with one JSON object `{"code", "error", "ok": false}` on stderr and nothing on
stdout when it refuses: `tests-invalid-arguments` (usage), `tests-invalid-revision` (a revision that
is not a commit), `tests-invalid-option` (a threshold or cutoff out of range, or `--gate` without
`--base`), `tests-source-unavailable` (a Git or index read failed), `tests-bound-exceeded`,
`tests-decisions-invalid` and `tests-output-unavailable` (stdout could not be written).

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
(issue #1032 AC5). The two missing pieces are residuals of V1-1587 and V1-1589.

Failure modes: the lexer is heuristic around regular expressions and JSX, so an uncertain file is
listed and its findings may be incomplete. Setup selection trusts declared registries and import
syntax; a setup that is reached only through runtime configuration (for example a Playwright
`dependencies` entry) is not seen, so `selected` is only as complete as the static import graph. A
false-negative lint rule is possible for an assertion helper not named `expect*`, `assert*`,
`verify*` or `should*`. Duplicate scores compare assertion text only: two tests with the same
assertions over different setup are reported, and two tests that check the same behaviour through
different assertions are not.

## Implementation, rollout and rollback

Parsing lives in `internal/testsource` (lint, duplicates, Git reads) and
`internal/contextindex/setup_select.go` (setup closure on the revision index); CLI parsing is in
`cmd/corvint/tests.go`. The lexer gains only a comment-collecting variant and an exported view. This
additive experimental profile changes no Core profile or persisted format. Rollback removes the
dispatch line, the help entries and the two packages' new files; there is no data migration.

## Acceptance evidence

Synthetic Git repositories: a registry-keyed spec selects its context's setup only; a hub and the
registry are not followed; comment-only mentions select nothing; computed, unknown, missing and empty
inputs return ALL with typed reasons; the digest is stable and changes with the spec list. Each lint
rule fires on its positive fixture and not on its negative one; changed-lines scope excludes old
findings; suppressions apply to exactly the named rule and are echoed. Duplicates pair normalised
lines across files, honour the cutoff, gate on changed tests and resolve with reasoned decisions.

## Delivery

| Requirement | Implementation | Focused evidence | Status |
|---|---|---|---|
| TSA-V0-001 | `internal/contextindex/setup_select.go` | `TestSelectSetupClosure`, `TestSelectSetupTraversesRegistrySeeders` | experimental |
| TSA-V0-002 | `internal/contextindex/setup_select.go` | `TestSelectSetupClosure` | experimental |
| TSA-V0-003 | `internal/contextindex/setup_select.go` | `TestSelectSetupAll`, `TestSelectSetupUniverseIncludesUnindexedFiles` | experimental |
| TSA-V0-004 | `internal/contextindex/setup_select.go` | `TestSelectSetupClosure` | experimental |
| TSA-V0-005 | `internal/contextindex/setup_select.go`, `cmd/corvint/tests.go` | `TestSelectSetupDigest`, `TestTestsCLI` | experimental |
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
| TSA-V0-030 | `internal/testsource/actor_auth.go` | `TestActorAuthorizationIssueRepro`, `TestActorAuthorizationUnknownLinks` | experimental |
| TSA-V0-031 | `internal/testsource/actor_auth_rails.go` | `TestActorAuthorizationIssueRepro`, `TestActorAuthorizationUnknownLinks` | experimental |
| TSA-V0-032 | `internal/testsource/actor_auth_rails.go` | `TestActorAuthorizationInheritance`, `TestActorAuthorizationIssueRepro` | experimental |
| TSA-V0-033 | `internal/testsource/actor_auth.go` | `TestActorAuthorizationIssueRepro`, `TestActorAuthorizationInheritance` | experimental |
| TSA-V0-034 | `internal/testsource/actor_auth.go` | `TestActorAuthorizationIssueRepro`, `TestActorAuthorizationDeterministic` | experimental |
| TSA-V0-035 | `internal/testsource/actor_auth.go`, `internal/testsource/actor_auth_rails.go` | `TestActorAuthorizationUnknownLinks` | experimental |
| TSA-V0-036 | `internal/testsource/actor_auth.go` | `TestActorAuthorizationIssueRepro` | experimental |
