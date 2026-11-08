<!-- SPDX-License-Identifier: Apache-2.0 -->
# Selection precision 2: CI selector parity, temp roots, history inputs

Date: 2026-11-03. Lane selprec2 (base `cb491adb15dd`, lane selprec's head). Tickets: V1-4437,
V1-4173, V1-3282, V1-4284, V1-4280, V1-4283. Requirements: AFP-V0-2240 to AFP-V0-2244 in
`docs/specs/affected-plan-selection-v0.md`; AFP-V0-2240 and AFP-V0-2241 amend AFP-V0-012 (c) for
the CI selector, AFP-V0-2243 amends AFP-V0-2204.

## Decisions

- **The CI selector's rule (c) follows the planner's Go naming relation (AFP-V0-2240 to
  AFP-V0-2242).** V1-4437 asked for AFP-V0-2203 (anchored tokens) to be ported to
  `internal/gateselect`. Ported alone, it moved PR 86's `reader` lines only from 79 to 78. The
  rest of rule (c)'s over-selection came from lone tokens (`"out"`, `"repo"`, `"test"`, `"src"`):
  rule (c) let them match any path component, but the planner's V1-0290 rule lets them name only a
  file name, placed by AFP-V0-072. So `namesWithin` ports `namesFileOnlyWithin` whole, anchored
  branch included, and `keepsReader` applies it with the planner's `namesPath` precondition. The
  two lexicons differ (AFP-V0-765). Everywhere but one case, rule (c)'s lexicon keeps more or
  broader tokens, so it stays a superset of the planner. The exception is `..\x`, which this
  lexicon splits into `..` and `x`. A package holding a parent-only token therefore treats every
  token as climbing. `TestRuleCMatchesThePlanner_AFPV02242` builds both relations over one fixture
  and fails on any difference. AFP-V0-2204 has no counterpart: the index holds Go packages only,
  and shell tests never reach the CI selector. `-bounds` (gate-ledger) uses the same filter, so a
  ledger key follows the selection relation.
- **Temp roots behind `|| exit 1` (AFP-V0-2243).** The selprec handback said the remaining shell
  readers "copy trees, assign the name more than once or assign it in a function". A probe of all
  23 found a different picture. Seven were kept only because `tmp=$(mktemp -d ...) || exit 1` put
  `||` after the assignment, which AFP-V0-2204 refused. None was kept only for a function
  assignment (`lane-scripts_test.sh` assigns one in a function, but also runs `git clone`). The
  assignment is the first command of its list, so it always runs in the file's own shell.
  `&&`/`||` after it are now accepted; a pipeline, the background, or a list the assignment leads
  that ends in `&` still is not. The tree-copy readers stay.
- **No TypeScript anchored narrowing (V1-4173 remainder, not done).** A first version certified
  TypeScript modules: it added the literal words the TypeScript lexicon drops as placement
  vocabulary and refused modules that name `cwd`. The independent review showed the gap is
  structural, not lexical. A TypeScript runner's working directory is unknown (AFP-V0-072 (3)),
  and a relative join (`'fixtures' + '/repo'`) or a call that returns the working directory
  (`process.cwd()` under a renamed destructuring, `path.resolve()`, `process.env.PWD`) roots the
  token there. The version was withdrawn before commit, so the five TypeScript `"/repo"` readers
  stay. Narrowing them needs the plugin to know each runner's working directory.
- **History inputs are mentions (AFP-V0-2244). This is an orchestrator-made, owner-delegated
  decision.** `internal/dogfoodflow` replays a parent commit's `script/batch-regen.sh` in a private
  clone (DCW-V0-401), and its test writes a fixture copy. Neither reads the worktree file, yet the
  declaration listed the three files as callers. A `heavy.sh` edit reached batch-regen.sh and,
  through the dogfoodflow Go unit, 25 `BINARY_EXEC` units. A new `history` member would have been
  the clearer shape, but `tools/declaration-compat-check` (AFP-V0-710) fails any member the CI
  planner pin lacks: the pinned planner would drop the whole declaration and widen every plan. The
  files are listed as `mentions` instead, with the reason naming them a history input.
- **V1-3282 is not implemented, by measurement.** A body edit of `flowWordBefore` falls back on
  `CHANGE_REACHES_PACKAGE_INITIALIZATION: init in dialect_hooks.go`. Behind the init, the
  `webImpact` var's initializer (`regexp.MustCompile` calls) also fails ADN-V0-004's safe rule.
  Two throwaway builds, never committed, measured the bounds:
  - With both fallbacks ignored and every contextindex declaration that calls `LookupImpact`
    marked reached (the proposal's lookup-site rule), the plan selects 147 of 151 units: 38
    referencing importers and 76 exposed names.
  - The ~93 target needs the lookup sites ignored as well (91 units, one exposed name). That is
    unsound: `nonGoImpactUnknowns` (reached from `Impact` and `Build`) calls
    `dialect.LookupImpact`, whose registered `webImpact.CodeLines` calls `flowWebMask`, then
    `flowWordBefore`.
  A sound version would need a store-only proof of the registry's register functions, an importer
  check of its lookups, and an initializer rule for function values. That is soundness-critical
  code for 4 units, so it is left for the owner. The larger lever is the recorded follow-up of
  moving `webImpact` to the TypeScript plugin (LGP-V0-027).
- **V1-4280 is blocked.** `unresolved-package=.` is emitted by the make-recipe plugin
  (`internal/liveverify/affected/makefile`, V1-3788, AFP-V0-994 to AFP-V0-999). That plugin
  exists only on the unmerged branch `claude/lane-mkr-w37-1101`, not in this base or on
  `origin/main` at `14e055d69`. Nothing in this tree models `ledger/go-test`.
- **V1-4283 is NOT_PRODUCED.** Landlock is available here (ABI 7, probed with the
  `landlock_create_ruleset` version call). Every one of the 28 named units has a recorded
  structural read the confinement contract cannot grant: the filesystem root `/`, `.git`, a nested
  toolchain build, a bubblewrap sandbox, or a walk of the whole repository. A confined run would
  reproduce those refusals, not bound a unit.

## Measurements

The same binary pair was used throughout: `before` built at the lane base (`corvint`,
`gate-affected-select`) and `after` built from this tree. "CI selector" is `gate-affected-select
PLAN github.com/Beamfall/corvint ROOT` on the plan the same-side planner wrote. "Same plan" is the
after selector on the before plan.

| Run | Planner before / after (selected, unknown) | Planner witnesses changed | CI selector `reader` lines, packages: before / after (same plan) |
|---|---|---|---|
| PR 86 (`--base f89db15bd` at `64321325b`) | 170, 83 / 166, 83 | PATH_LITERAL 69 to 62; DEPENDENCY 44 to 47 (three shell readers re-witnessed) | 79, 257 / 28, 209 (28, 209) |
| `--by-path --path internal/jslinecov/testdata/repo/lib.js` (PR 86 tree) | 64 additions + 25 shared / 57 + 26 | PATH_LITERAL readers: Go 6 to 6, shell 23 to 16, TypeScript 5 to 5 | n/a |
| contextindex body edit (`flowWordBefore`) | 151, 4 / 151, 4 | none | 9, 177 / 2, 170 |
| `script/heavy.sh` edit, lane base | 71, 1 / 71, 1 | none (the declaration is data) | 11, 72 / 4, 65 |
| `script/heavy.sh` edit, declaration committed | 71, 1 / 52, 1 | BINARY_EXEC 25 to 0, DEPENDENCY 10 to 6 | 11, 72 / 4, 46 |
| gate-prerequisite edit (`script/check-stray-backups.sh`) | 50, 1 / 50, 1 | none | 11, 53 / 4, 46 |

With an empty package list, the CI selector still selects every planner PATH_LITERAL Go reader on
PR 86 except four. Those four are the same before and after: readers of
`.corvint/test-read-scopes.json` whose declared scopes the `declared` rule decides.

## Soundness evidence

- Negative controls: `TestRuleCAnchoredTokens_AFPV02240`, `TestRuleCLoneTokens_AFPV02241`, and
  the eight new cases of `TestTempRootPathsAreNoTokens_AFPV02204`.
- Fault injection (each mutation applied, the test run, the mutation reverted):
  - Placement removed from `namesWithin`: `TestRuleCLoneTokens_AFPV02241` and
    `TestRuleCMatchesThePlanner_AFPV02242` fail. The parity test names `placed` missing for both
    fixture paths.
  - `keepsReader` returning true (the old relation): the parity test fails on five paths.
  - The pipeline/background refusal removed: the `piped` and `background` cases fail.
  - The `/..` suffix dropped from `parentClimbs`: the `mixed` control and the parity test fail.
  - The list walk's `&` refusal removed: `list in background` and `or list in background` fail.
- Five gateselect fixtures and one golang fixture relied on a lone token naming a directory, and
  now carry a token that names the path under the planner's relation, keeping each test's intent:
  `TestSelectBoundsDeclaredPathReaders`, `..._ADNV0010`, the two evidence tests,
  `...StopsClosureAtTestOnlyTokenHolder`, and `TestADNV0010And0011ProducerToConsumer`.

## Independent review

A subagent reviewed the uncommitted code diff read-only against the spec section. It reproduced
three MEDIUM findings with throwaway tests; all are resolved:
1. `` `a/..\notes` ``: rule (c) yields `a/..` and `notes`, the planner one climbing token, so rule
   (c) dropped a planner reader. Fix: `parentClimbs` also counts a token ending in `/..`, and the
   `mixed` fixture joins the parity test.
2. `tmp=$(mktemp -d) && true &`: bash backgrounds the whole list, so the assignment does not
   persist. Fix: the trail check walks the list and refuses one that ends in `&`.
3. TypeScript `AnchorsPlaced` could be rooted at the working directory (renamed `cwd`
   destructuring, `path.resolve()`, `process.env.PWD`). Resolved by withdrawing the TypeScript
   narrowing (see Decisions).
One LOW finding (JavaScript identifiers turned into strings, missing from the TypeScript word
list) fell away with the withdrawal. The review found the panics, unit validation, cache decoding,
root package and memo, and the dogfoodflow move sound.

## Not produced

- A CI observation of the narrowing: NOT_OBSERVED. CI runs the pinned `CORVINT_PR_TOOL_SOURCE`
  build, so AFP-V0-2240 and AFP-V0-2241 reach it only with the next re-pin.
- `make gate`: NOT_RUN, per the lane rules.
