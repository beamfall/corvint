# Change-Compiled E2E V0 — hosted blocks, generated scenarios, shadow-gated retirement

Owner: Russell Lewis
Date: 2026-10-17
Intent status: accepted (owner, 2026-10-22)
Delivery status: experimental
Authoritative inputs: owner acceptance of 2026-10-22 ("I want the CCE stuff in 1.0"; recorded in
`docs/build-log/2026-10-22-feat1-cce-site-knowledge.md`); owner direction of 2026-10-17 (this session; recorded in
`docs/build-log/2026-10-17-change-compiled-e2e-direction.md`): "Draft the spec as augment-first:
hosted blocks, generated scenarios, a shadow evaluation against the existing suite before any test
is retired"; `AGENTS.md`; `docs/SPEC-DRIVEN-DEVELOPMENT.md`; `docs/specs/application-map-v0.md`
(AMAP-V0); `docs/specs/application-map-scenario-planner-v0.md` (AMSP-V0);
`docs/specs/application-flow-understanding-v1.md` (AFU-V1 declared outcomes);
`docs/specs/application-navigation-ledger-v0.md` (ANL-V0 proven edges);
`docs/specs/run-verified-navigation-v0.md` (RVN-V0); `docs/specs/playwright-run-evidence-v0.md`
(PWR-V0); `docs/specs/new-e2e-test-acceptance-v0.md` (NEA-V0);
`docs/specs/browser-behavior-falsification-v0.md` (BBF-V0); the 2026-10-17 expert panel amendment
(`docs/build-log/2026-10-17-cce-expert-panel.md`), whose owner request was "I want a world class
solution that is a perfect additional and use case for corvint".

## Agent digest
- Claim: Reviewed E2E blocks compile into change-specific scenarios that must discriminate the change; hosted tests retire only after a shadow evaluation.
- Status: accepted (owner, 2026-10-22); experimental; CCE-V0-026..054 accepted under the standing auto-accept (2026-10-11). First slice V1-1665 implements CCE-V0-001, 002, 004..009 and 015 in `internal/changee2e` behind `corvint-test e2e` with focused tests and a live Playwright qualification (Node v22.23.2); CCE-V0-003 is partial (one hand-written declarations file). The 2026-10-17 panel amendment adds CCE-V0-016..025 and amends 003, 004, 009, 011 and 012. Implemented: the CCE-V0-021 environment block (V1-1681 serve part), the obligations soundness of CCE-V0-019/020 with the amended 004 base join (V1-1678), and (2026-10-22 lane) the amended 009 ladder, 016 `oracle_origin`, 017 head test tree, 018 structure controls, 024 `outcome-vacuous`, the offline fake runner and the 011/012 shadow report over a recorded window (CCE-V0-026..033; V1-1679, V1-1680, V1-1682, V1-1687). The 2026-10-22 lane 1022k implements the hosted-test prover CCE-V0-034..036 (offline and live), receipt-located assertions (PWP-V3-008), `cleanup-failed`, the 037 usefulness exclusion of `PRESERVES`, CCE-V0-010, 013 and the 014 byte cap (CCE-V0-039, 040), and the first V1 authoring slice (V1-1683): `e2e harvest`, the `.corvint/e2e-acceptances.json` ledger and the in-diff flag (CCE-V0-041..044), and the runtime-coverage reach source of V1-1685 (CCE-V0-045..047). Not implemented: CCE-V0-022/023/025 and the rest of 024; NOT_RUN: the 038 actualbudget rerun and any adopter window. Lane cce2-1022u attributes a located worker teardown failure as `teardown-failed` (CCE-V0-048, PWP-V3-009) and records how a hosted run's config differs from the adopter's (CCE-V0-049), binds harvested proposals to a map flow or screen (CCE-V0-050), lifts the edited-oracle cap only through a recorded independent acceptance (CCE-V0-051) proposes, never applies, a locator repair (CCE-V0-052), records the head pool's served paths per screen for `--coverage` (CCE-V0-053) and widens reach through the head tree's TJAA module graph behind `--module-graph` (CCE-V0-054). CCE-V0-086 (V1-1711) resolves a relative `--root`. CCE-V0-087 (V1-2427) attributes a control run's teardown failure like a side's; CCE-V0-088 (V1-2428) lets `e2e harvest --manifest` read a configured map path. The 2026-10-24 lane p1-cce proposes CCE-V0-100..104: offline replay of recorded side receipts and a live-required switch (V1-1687), `--ledger` bound to the impact relation (V1-1685) and path synthesis over proven ledger edges (V1-1684). Lane p1-cce-2 proposes CCE-V0-105..115: the rest of the V1-1684 block vocabulary (fixture outputs and data flow, sessions, `base.extend` teardown, role and label locators, accessible-name and count assertions, declared routes, `test.step` with trace, goldens at `corvint-test-e2e-compile/1`) and the V1-1685 API contract join with the no-`UNKNOWN`-to-`NONE` property. Lane V1-2620 proposes CCE-V0-116: the prove report keeps each lost run's concrete reason, bounded message and location. Everything ships as the `e2e` group of one experimental companion, `corvint-test`; V0 adds nothing to core `corvint`.
- Exists: `cmd/corvint-test` (`e2e catalog|harvest|obligations|compile|prove|shadow`) over `internal/changee2e`. It composes the existing AMAP-V0 map and scaffold, AMSP-V0 planner, AFU-V1 declared outcomes, ANL-V0 proven edges, RVN-V0 run verification, PWR-V0 run comparison and NEA-V0/BBF-V0 negative controls.
- Blocked on: the CCE-V0-038 external rerun (V1-1733); an adopter shadow window of at least 30 recorded entries (external; umbrella V1-1666); the rest of the companion fold (V1-1667, planned in `docs/specs/corvint-test-fold-v0.md`; groups 1 to 3 have moved and groups 4 to 6 remain).
- Read next: User and measurable job; Definitions; Requirements (016..033); Shadow evaluation and retirement; Beyond V0; Owner questions.

## User and measurable job

A team keeps a large hosted end-to-end suite. Most of its tests re-check behaviour a given change
cannot reach, while a change that does reach a screen is often covered only by a test written for
some other purpose. The owner's model is that every E2E test is assembled from the same few kinds
of parts: page objects, paths, inputs and outputs, results, outcomes and scripts. If Corvint holds
a revision-pinned catalog of those parts, then for each change it can compile the one scenario that
reaches the changed behaviour and asserts its declared outcome, run it locally or in CI, and prove
that the scenario actually detects the change.

The direction is augment-first. The parts that carry intent and are expensive to get right (page
objects, fixtures, input declarations and expected outcomes) stay hosted in the repository and are
reviewed like code. The scripts that string them together are generated on demand and are not
committed. Hosted tests keep running beside the generated scenarios until a shadow evaluation shows
that the generated scenarios catch what each hosted test catches; only then may Corvint propose a
hosted test for retirement, and only the owner retires it.

The primary job is a per-change proof for the agent or reviewer who made the change: "this change
reaches these declared outcomes, here is the generated scenario that checks each one, and here is
the evidence that the scenario would have caught the change going wrong". That proof is useful on
the first change a team runs it on. Retiring hosted tests is the later payoff (V2), and it is earned
only through the shadow gate; it is not the V0 value claim.

Measured jobs:

1. **Compile**: on a committed fixture, a diff that changes one screen's component yields exactly
   one compiled Playwright scenario. It reaches that screen over proven paths, calls only catalogued
   page-object methods and asserts the declared outcome with its declared matcher, locator and
   value. No `TODO` remains in the emitted body.
2. **Discriminate**: the compiled scenario passes at the head revision and fails at the base
   revision on the declared outcome's own assertion, and the BBF-V0 `wrong-locator` and
   `wrong-expected-value` controls each make it fail.
3. **Fail closed**: a diff whose reach is unknown, or that reaches an outcome with no catalogued
   locator, value, path or fixture, yields `INCOMPLETE` with each missing block named. It never
   yields an invented selector, path or expected value, and never narrows the hosted selection.
4. **Shadow**: over a frozen window of changes, the report compares compiled scenarios with the
   hosted affected selection on seeded regressions caught, real failures caught, escapes and
   runtime, and lists a hosted test as a retirement candidate only when the frozen gate holds.
5. **Useful per change**: a change is *useful* when its obligations do not read `UNKNOWN` and at
   least one obligation reads `DISCRIMINATES`, `REGRESSION_DETECTED` or `NEW_REACH` with
   `oracle_origin` `ADMITTED_AT_BASE`. `PRESERVES` never counts: it passes at base and head, so it
   cannot tell a refactor from a no-op diff (CCE-V0-037, V1-1713, the spike's F2 falsifier); a
   `PRESERVES` scenario still scores the seeded mutants it kills. The usefulness rate is useful changes over all changes
   run, with the denominator printed. The kill threshold is below 40% (Promotion and kill criteria).
6. **Time and cost**: every `prove` and `shadow` report gives time-to-verdict (wall clock from
   `compile` to the last verdict) and CPU seconds per change, on the same host as the hosted
   comparison, including repeats and controls.

## Verified current state

Checked against `origin/main` at `2c826212c74beb48507b60e7759fae80faaf2a40`.

- AMAP-V0 (`internal/appmap`, `corvint flows appmap`) compiles a revision-pinned screen graph that
  joins routes, flows and E2E specs and page objects through imports. Its `scaffold` projection
  (AMAP-V0-013) reuses callable page-object methods but leaves locator and outcome `TODO`s and never
  writes a file.
- AMSP-V0 (`internal/appmap/plan.go`) turns plain-language steps into a deterministic plan and a
  Playwright draft with one `TODO assert outcome <id>` line per declared outcome. Inferring outcome
  assertion code is an AMSP-V0 non-goal.
- AFU-V1 flow intents already declare outcomes as `outcome_id`, `behavior`, `matcher`, `locator` and
  `value`, plus variations with preconditions and observable facts. These are human-owned and can
  serve as the oracle.
- ANL-V0 keeps a navigation-edge ledger whose edges read `PROVEN` only from bound run receipts; AMAP
  entry rows read shortest proven paths from it (AMAP-V0-080..082).
- RVN-V0 binds passing Playwright receipts to map steps as `VERIFIED` at a revision,
  `UNVERIFIED_AT_HEAD` after a source change and `CONTRADICTED` on failure.
- PWR-V0 (`corvint runs stability|compare`) classifies per-test stability across repeats and
  behaviour parity between a baseline and a candidate run.
- NEA-V0 (`cmd/corvint-tests-accept`) repeats new Playwright tests and runs approved negative
  controls through BBF-V0, whose closed control vocabulary includes `wrong-locator` and
  `wrong-expected-value`.
- `corvint affected` and AFU-V1/ETS selection map a diff to affected tests and fail closed to the
  full set on unknown reach.
- No component holds typed input domains or fixture blocks; seed and setup selection is the open
  ticket V1-1587. No component composes a scenario from a diff, and none compares generated
  scenarios with a hosted suite.

## Definitions

- **Block**: one catalogued, revision-pinned part of an E2E test, identified by kind and a stable
  ID, with the Git blob and line span it came from. The closed V0 block kinds map the owner's
  vocabulary as follows.
  - `page-object`: a callable page-object method as AMAP-V0-013 defines callable, with its class,
    file and anchor.
  - `path`: a navigation edge or chain read `PROVEN` from the ANL-V0 ledger, or a direct entry URL
    from an AMAP entry row.
  - `input`: a declared input domain for a field or action: its name, the locator of the field and
    a closed set of named values, each tagged `valid` or `invalid`.
  - `fixture`: a declared setup or seed step that establishes a precondition, with the
    precondition IDs it satisfies and the cleanup it owns.
  - `outcome`: an AFU-V1 declared outcome (`behavior`, `matcher`, `locator`, `value`). This is the
    owner's "outputs" and "outcomes"; the matcher and value are the oracle.
  - `result`: a bound run receipt that verifies a block (RVN-V0 status, PWR-V0 stability). A result
    qualifies a block; it never creates one.
  - `script`: a compiled scenario, which is only ever an output.
- **Hosted block**: a block whose source is a tracked repository file a human reviews (page-object
  code, flow intents, input and fixture declarations). Corvint reads hosted blocks and may
  propose new ones as `INFERRED` text; it never admits or writes one.
- **Catalog**: the derived, immutable projection of every admitted block at one revision, with a
  content digest. It is derived state, not a database.
- **Obligation**: an outcome that a diff puts at risk, reached by joining changed source spans to
  screens, flows and outcomes through the existing impact and map relations. An obligation is
  `REACHED`, `UNKNOWN` (reach could not be bounded) or `NONE`.
- **Compiled scenario**: one generated Playwright spec file for one obligation, deterministic in
  (catalog digest, base, head, obligation ID, compiler version).
- **Discrimination verdict**: the result of running a compiled scenario against the head and base
  applications under negative controls: `DISCRIMINATES`, `NEW_REACH`, `PRESERVES`,
  `REGRESSION_DETECTED`, `SCENARIO_BROKEN`, `UNSTABLE`, `CONTROL_SURVIVED` or `INCONCLUSIVE`
  (CCE-V0-009). The panel amendment replaces the first slice's `FAILS_AT_HEAD` with
  `REGRESSION_DETECTED` and `SCENARIO_BROKEN`, and adds `NEW_REACH`.
- **Application revision** and **test revision**: a proof run executes the test tree of one
  revision (page objects, fixtures, the compiled scenario and its runner configuration) against the
  application tree of another. Both sides of a proof use the head test tree; only the application
  revision varies (CCE-V0-017).
- **Oracle origin**: whether a compiled scenario's outcome block was admitted unchanged at base
  (`ADMITTED_AT_BASE`), exists at base but the diff changed it (`CHANGED_IN_DIFF`), or is new in the
  diff (`NEW_IN_DIFF`) (CCE-V0-016). Only `ADMITTED_AT_BASE` outcomes are independent of the change
  under test.
- **Seeded regression**: an application-side mutant produced by a preregistered mechanical operator
  with a fixed seed and applied identically to the application tree under both the hosted and the
  compiled side. BBF-V0 controls stay the oracle-strength check on one scenario; they are not seeded
  regressions, because they edit the test, not the application.
- **Shadow window**: a frozen, ordered list of changes registered before the evaluation runs, over
  which compiled scenarios and the hosted affected selection both run. Its composition is set in
  Shadow evaluation and retirement.

## Binary boundary

Every CCE-V0 command is a subcommand of the `e2e` group in one experimental companion binary,
`cmd/corvint-test`: `corvint-test e2e catalog|harvest|obligations|compile|prove|shadow`. `corvint-test` is
named for every kind of test, not only E2E; this spec defines only the `e2e` group. Folding the
existing test companions (`corvint-test-runner`, `corvint-tests-accept`, `corvint-behavior-falsify`,
`corvint-playwright-minimize`) into it is governed by its own spec, `docs/specs/corvint-test-fold-v0.md`
(CTF-V0, V1-1667), under which the `runner`, `falsify` and `minimize` groups have moved. Two existing names collided with `corvint-test` and were resolved in the first slice
(V1-1665): the companion-release archive classifier now maps only `corvint-test-validity*` names to
`corvint-test-validity-mcp` (`internal/companionrelease/retained_test.go:142@9304a70e`), and core `corvint test` keeps its bare
`corvint-test:` diagnostics (`cmd/corvint/session_test_shards_unix_test.go:82@8b1f3cb8`) while the
companion prints `corvint-test e2e:`. V0 adds no command, flag, dependency or
default-path behaviour to core `corvint` (invariant 7), so the capability can be built, shipped,
killed or rolled back as one unit without touching the core binary. The companion reads the same
in-module relations core uses (AMAP-V0, ANL-V0, AFU-V1, RVN-V0, PWR-V0) as a library; it does not
shell out to `corvint` and does not duplicate those relations.

The read subcommands (`catalog`, `obligations`, `compile`, `shadow`) are read-only. Only `prove`
executes a browser, and only in a disposable workspace. Graduating the read subcommands into core
is a later, separate decision that needs its own owner acceptance and the promotion evidence below.

## Requirements

- `CCE-V0-001`: `corvint-test e2e catalog [--root PATH] --rev REV` MUST emit one deterministic
  `e2e-block-catalog/0` JSON document listing every admitted block of the seven kinds in the
  Definitions, each with its kind, stable ID, Git blob, line span and the source relation that
  admitted it, sorted by kind then ID, with the catalog content digest. It MUST be read-only and
  write only to stdout or an explicit `--out` file. Every `--out` path in this spec MUST resolve
  outside the repository root, and `.git/` and `.corvint/` are inside that root. Any other path MUST
  refuse with `e2e-out-not-allowed` before anything is written (invariant 4).
- `CCE-V0-002`: `page-object`, `path` and `outcome` blocks MUST come only from the existing AMAP-V0,
  ANL-V0 and AFU-V1 relations; a block those relations read `STALE`, `UNKNOWN`, `UNPROVEN` or
  `inferred` MUST be listed under `unadmitted` with its reason and MUST NOT be admitted.
- `CCE-V0-003`: `input` and `fixture` blocks MUST come only from a repository-declared, tracked
  `.corvint/e2e-blocks.json` (`e2e-block-declarations/0`), read at `--rev` through Git, whose
  schema is closed:
  - the top level holds only `schema`, `inputs` and `fixtures`, and optionally `environment`
    (CCE-V0-021) and `reach_exempt` (CCE-V0-020);
  - an input is `input_id`, `screen_id`, `locator` (an AFU-V1-026 role and name, or a test ID) and
    `values`; each value is `value_id`, `value` (a string) and `class` (`valid` or `invalid`);
  - a fixture is `fixture_id`, `satisfies` (AFU-V1 precondition IDs), `setup` and `cleanup`; each of
    `setup` and `cleanup` names a `page_object` and a `method` that must be a callable `page-object`
    block.

  The compiler applies an input value only as a Playwright `fill` on the input's own locator. A
  missing file means no input or fixture blocks, reported as the unknown `blocks-not-declared`. An
  unknown member, a wrong type, a duplicate ID or an unresolved reference MUST refuse with
  `e2e-blocks-invalid` and admit nothing from the file.
- `CCE-V0-004`: `corvint-test e2e obligations [--root PATH] --base SHA --head SHA` MUST map the diff to
  obligations through the AMAP-V0 impact relation over the application maps at both base and head,
  and MUST name in its document each relation it consulted and each it did not (the affected plan
  and the ANL-V0 ledger are not consulted until V1-1685 adds them; runtime coverage is consulted
  only when given, CCE-V0-045). It lists each reached outcome
  with the changed span and the chain that reaches it. When any changed span's reach cannot
  be bounded, the report MUST read `UNKNOWN` overall and name the spans; it MUST NOT read `NONE`.
  A compilation over an `UNKNOWN` report MUST read `INCOMPLETE` overall with those spans, even when
  every `REACHED` obligation compiled or none was reached.
- `CCE-V0-005`: `corvint-test e2e compile` MUST emit, for each `REACHED` obligation, at most one compiled
  scenario that: enters the obligation's screen over one admitted `path` block (shortest proven
  chain, then lexical order); satisfies each precondition with one admitted `fixture` block; drives
  actions only through admitted `page-object` methods and `input` values; and asserts the
  obligation's `outcome` with exactly its declared matcher, locator and value.
- `CCE-V0-006`: When any block a scenario needs is missing or unadmitted, the compiler MUST emit no
  scenario for that obligation and MUST report it `INCOMPLETE` with each missing block's kind and
  the reason. It MUST NOT emit a selector, URL, input value or expected value that is not the exact
  text of an admitted block.
- `CCE-V0-007`: Compilation MUST be deterministic: identical catalog digest, base, head, obligation
  and compiler version MUST produce byte-identical scenario files. Each file MUST carry a header
  comment with those five values and its own content digest. The digest is SHA-256 over the file's
  bytes with the digest value left empty while hashing (the ANL-V0-005 convention), and the catalog
  digest uses the same rule. Compiled files MUST be written only under an explicit `--out`
  directory, under the CCE-V0-001 `--out` rule.
- `CCE-V0-008`: Running and proving are separate and opt-in: only `corvint-test e2e prove` executes,
  never a read subcommand. `prove` MUST run each compiled scenario
  through the existing Playwright provider path at head with repeats (default 5, retries 0, one
  worker) and at base with the same settings, against a disposable workspace per BBF-V0's mutation
  boundary, and MUST bind every run to a PWP receipt.
- `CCE-V0-009`: The discrimination verdict MUST be the first of these that holds. "Head" and
  "base" name the application revision; the test revision is head on both sides (CCE-V0-017).
  1. `INCONCLUSIVE` when any required run produced no qualified receipt, when either pool is
     empty or any repeat was skipped or did not finish, or when a fixture cleanup failed (reason
     `cleanup-failed`; a cleanup failure never counts as an assertion result). Every later rung
     therefore sees a complete pool on each side.
  2. `UNSTABLE` when PWR-V0 classifies the head pool `INTERMITTENT`.
  3. `REGRESSION_DETECTED` when every head repeat fails at the outcome assertion's own location and
     every base repeat passes: the change broke a declared outcome.
  4. `SCENARIO_BROKEN` when any head repeat fails and 3 does not hold. From here on every head
     repeat passed.
  5. `CONTROL_SURVIVED` when any required control (CCE-V0-018) applied at head reads `survived`.
  6. `INCONCLUSIVE` when any required control reads anything other than `killed` under
     BBF-V0-005/006 with its criterion, test, product and environment identities matched (for
     example `invalid_control`, `infrastructure_failed` or `not_run`).
  7. `DISCRIMINATES` when every base repeat fails at the outcome assertion's own location with a
     value mismatch: the locator resolved and the matcher compared a different value.
  8. `NEW_REACH` when every base repeat fails at the outcome assertion's own location because the
     locator did not resolve: the outcome is new behaviour, not a changed value.
  9. `PRESERVES` when every base repeat passes.
  10. `INCONCLUSIVE` otherwise: mixed base results, mixed failure kinds, or a base failure anywhere
     but the outcome assertion.

  The assertion's location MUST be read from the receipt's error location (file and line of the
  emitted assertion), not from marker text in the message. Implemented (2026-10-22, lane 1022k):
  the PWP receipt carries the last attempt's `failureLocation` (PWP-V3-008, the innermost stack
  frame in the test file, else the runner's error location); a failure counts as at the assertion
  only when that location is the scenario's file and one of its assertion lines, and the proof note
  names the location (`at <file>:<line>`, or `receipt names no failure location`). A failure
  located inside the test file's own `finally` block reads `cleanup-failed`. A test-scoped fixture,
  `afterEach` or `afterAll` failure is a test failure located off the assertion; a worker-scoped
  fixture teardown failure is attributed by its location under CCE-V0-048.

  `PRESERVES` MUST be reported as a regression-preservation check only and MUST NOT be described as
  detecting the change. `NEW_REACH` MUST be described as checking new behaviour against its
  absence, not as detecting a changed value.
- `CCE-V0-010`: A compiled scenario MUST NOT read above `candidate` unless its verdict is
  `DISCRIMINATES`, `NEW_REACH` or `PRESERVES` and its `oracle_origin` is `ADMITTED_AT_BASE`; a passing compiled scenario MUST NOT narrow, skip or replace any
  hosted test's selection in V0.
- `CCE-V0-011`: `corvint-test e2e shadow --window FILE` MUST read a frozen shadow window (an ordered list
  of typed entries, digest-pinned before the first run, as Shadow evaluation and retirement defines
  the entry types and their revisions) and report, per entry and in total: obligations by state, scenarios by verdict and by `oracle_origin`, hosted affected
  tests run, seeded regressions (application-side mutants, Definitions) caught by each side, real
  hosted failures the compiled side missed (escapes), usefulness, and median and p95 wall-clock and
  CPU time of each side on the same host. The window MUST be refused unless its registration pins
  the mutation operator set, the seed, the catalog digest and the compiler version. Catches are
  counted per target identity (a seeded mutant's ID, or a real failure's hosted test and entry). A
  compiled scenario is *eligible* when its unmutated verdict is `DISCRIMINATES`, `NEW_REACH`,
  `REGRESSION_DETECTED` or `PRESERVES` and its `oracle_origin` is `ADMITTED_AT_BASE`. A seeded
  mutant is caught by the compiled side when at least one eligible scenario of that entry passes on
  the unmutated application and fails at its outcome assertion's location on the mutated one, in
  every repeat; a real failure is caught when an eligible scenario reads `REGRESSION_DETECTED` for
  it. Every target the hosted side caught and no eligible scenario caught is one miss and one escape,
  counted once however many scenarios were non-scoring. For an entry whose obligations read
  `UNKNOWN`, every target the hosted side caught there is a miss and an escape. Every rate MUST print
  its numerator and denominator and the one-sided 95% Clopper-Pearson bound.
- `CCE-V0-012`: A hosted test MAY be listed as a retirement candidate only when all of these hold
  over the shadow window:
  - every outcome it asserts is an admitted `outcome` block;
  - it ran in at least the frozen minimum number of window changes and caught at least the frozen
    minimum number of seeded regressions;
  - for every change in the window where it ran, the compiled scenarios caught every seeded
    regression it caught;
  - the compiled side had zero escapes against it;
  - the Holm-corrected one-sided 95% upper bound on the compiled miss rate against it, over the
    seeded regressions it caught, is at most the frozen bound;
  - the window size, composition and parity thresholds equal the frozen gate values in Shadow
    evaluation and retirement.

  A criterion whose denominator is zero MUST read failed, never vacuously held. The report MUST list
  each held criterion with its evidence, and each failed criterion for a test that is not a
  candidate.
- `CCE-V0-013`: Corvint MUST NOT delete, disable, skip or move a hosted test, and MUST NOT write a
  retirement into any configuration. A retirement candidate is a proposal for the owner to act on.
- `CCE-V0-014`: Every emitted document (`catalog`, `obligations`, `compile`, `prove`, `shadow`) MUST
  carry the input revisions and digests it was computed from and MUST stay within a byte cap
  (default 1 MiB), truncating to a deterministic prefix with an explicit `truncated` count rather
  than silently dropping entries. Verdicts, states, counts and denominators MUST be computed over
  the complete data before truncation, and truncation MUST NOT change them. A command that consumes
  another's document (`compile` reading a catalog or obligations, `shadow` reading proofs) MUST
  refuse a truncated input with `e2e-input-truncated`.
- `CCE-V0-015`: Every CCE-V0 command MUST be a subcommand of the `corvint-test e2e` group, as Binary
  boundary defines. V0 MUST NOT add a command, flag, import-time side effect or dependency to core
  `corvint`, and the companion MUST NOT invoke the `corvint` binary.

The 2026-10-17 expert panel amendment adds CCE-V0-016..025. Each closes a verified gap in the first
slice or the draft; the build-log entry gives the evidence.

- `CCE-V0-016`: Every compiled scenario MUST record `oracle_origin` by joining its outcome block at
  base and head: `ADMITTED_AT_BASE` when an admitted outcome with the same ID, matcher, locator and
  value exists at base; `CHANGED_IN_DIFF` when the ID exists at base with any of those different;
  `NEW_IN_DIFF` otherwise. Failure mode: a change edits both the product text and its declared
  outcome, and the scenario certifies itself. Only `ADMITTED_AT_BASE` scenarios count toward
  CCE-V0-010 and CCE-V0-011/012 until a human acceptance record exists (V1-1683).
- `CCE-V0-017`: `prove` MUST run the head test tree (the files of every admitted `page-object` and
  `fixture` block the scenario uses, their transitive relative imports, the compiled scenario, the
  runner configuration, and the package manifest and lockfile that resolve the runner's
  dependencies) against both the head and the base application trees. An import that does not
  resolve inside that tree or the resolved dependencies MUST make the proof `INCONCLUSIVE` with
  `test-tree-unresolved`. `prove` MUST record `application_revision`,
  `test_revision` and the test-tree digest for every run; the digest MUST be identical on both
  sides. A test-tree file that application code also imports MUST make the proof `INCONCLUSIVE`
  with `test-tree-overlaps-application`. Failure mode: the base run executes base page objects, so
  a verdict measures test drift instead of the change.
- `CCE-V0-018`: Besides the BBF-V0 `wrong-locator` and `wrong-expected-value` controls on the
  outcome assertion, `prove` MUST apply `omitted-event` to the step that is expected to produce the
  outcome and to each fixture setup the scenario uses. Each MUST read `killed` for a positive
  verdict. Failure mode: the outcome holds without the action, or the precondition is vacuous,
  and the template-only controls still read `killed`.
- `CCE-V0-019`: `obligations` MUST read `UNKNOWN`, never `NONE`, with the span named, when: a
  reached screen has no flow step that expects an outcome; the application map reports any unknown
  or unresolved step for a reached screen; the impact relation reports omitted or truncated
  entries; a screen, flow or outcome present at base is absent at head; or the number of reached
  obligations exceeds the limit (instead of refusing the document). `NONE` is legitimate only when
  every changed path matches a `reach_exempt` glob or the impact relation places it on no screen,
  with no map unknowns and no omissions.
- `CCE-V0-020`: Every `UNKNOWN` span MUST carry exactly one reason from the closed set
  `unplaced-path`, `screen-without-outcome`, `map-unknown`, `impact-truncated`,
  `removed-at-head`, `fan-out-over-limit` and `blocks-not-declared`; an unlisted reason is a
  defect. The optional `reach_exempt` member of `e2e-blocks.json` is a list of `{glob, reason}`
  entries read at base, so a diff cannot exempt itself; each exemption the report applies MUST cite
  its glob. A glob is a Go `path.Match` pattern over the repository-relative path (`*` does not
  cross `/`); an invalid or absolute pattern or an empty reason refuses `e2e-blocks-invalid`. The
  report lists each applied exemption under `exempt` as `{path, glob, reason}`, gives each unknown
  span an optional `detail` naming its cause (the impact reason, the map unknown, the removed
  entity, the screen or the dropped obligation), and when one span has several causes keeps the
  first of `removed-at-head`, `impact-truncated`, `fan-out-over-limit`, `map-unknown`,
  `screen-without-outcome`, `unplaced-path`. A map unknown counts for a reached screen when it
  names that screen, lies in a changed file, or names a step of a flow through that screen; when
  the report would otherwise read `NONE`, every map unknown counts. Any impact omission makes every
  changed path `impact-truncated`, and exemptions are not applied over a truncated impact set.
- `CCE-V0-021`: The optional `environment` member of `e2e-blocks.json` MUST hold only `serve` (an
  argv array, run without a shell), `base_url` (a loopback URL), `ready` (a path polled until it
  answers, with a bounded timeout) and `env` (variable names to pass through, never values). `prove`
  reads it at the test revision, runs `serve` inside each side's application tree and records its
  digest. Without it, `prove` serves the tree statically as the first slice does and records
  `environment: static`. A missing tool or a failed `ready` reads `INCONCLUSIVE`, never a verdict.
- `CCE-V0-022`: `prove` MUST pass the runner only an allowlisted environment (`PATH`, a workspace
  `HOME` and the declared `env` names), MUST abort and record every browser request to a
  non-loopback host, MUST retain receipt bytes under `--out` or report `NOT_RETAINED` with a reason,
  and MUST bind in its report the catalog, obligations and compile digests, the unknown spans, the
  overall obligation state and every scenario digest. When `prove` compiles for itself it MUST bind
  the same digests a separate `compile` would emit.
- `CCE-V0-023`: Every `corvint-test e2e` subcommand MUST exit by this closed table: `0` the
  document was emitted and no other code applies; `1` refusal or usage error (no document, or a
  refusal document); `2` `prove` found at least one `REGRESSION_DETECTED`; `3` the document was
  emitted but its signal is incomplete: obligations `UNKNOWN`, compile `INCOMPLETE`, a `prove`
  report whose bound obligations read `UNKNOWN` or whose bound compile reads `INCOMPLETE` (whatever
  its scenario verdicts), or any `prove` verdict `INCONCLUSIVE`, `UNSTABLE`, `CONTROL_SURVIVED` or
  `SCENARIO_BROKEN`. Code `2` wins over `3`. Failure mode: CI treats a broken or unproven scenario as green.
- `CCE-V0-024`: Compile hygiene: emitted golden bytes MUST be pinned to the compiler version, so a
  golden test fails when output bytes change without a version change; an outcome whose value is
  empty MUST be unadmitted with `outcome-vacuous`; the secret screen MUST run per block over input
  and outcome values and unadmit only the matching block with `secret-suspected`, rather than
  refusing the catalog.
- `CCE-V0-025`: Text read from the page or the runner (error messages, DOM snippets, console
  lines) MUST appear in a report only inside a bounded `untrusted_text` field, and never in a
  reason, code, summary or block. Failure mode: page-controlled text reaches an agent as
  instructions or forges a verdict explanation.

The 2026-10-22 feature lane (V1-1679, V1-1680, V1-1682, V1-1687) adds CCE-V0-026..033. They pin
readings the amended requirements leave open; each is accepted under the standing auto-accept of
2026-10-11.

- `CCE-V0-026`: The CCE-V0-009 failure kind of a failed assertion MUST be read from the runner's
  error message after removing terminal escape sequences: `locator-unresolved` when it reports
  that the element was not found, `value-mismatch` otherwise. Failure mode: a coloured message or
  a different matcher wording turns new behaviour into a value mismatch.
- `CCE-V0-027`: A `CHANGED_IN_DIFF` scenario whose verdict would be `REGRESSION_DETECTED`,
  `DISCRIMINATES`, `NEW_REACH` or `PRESERVES` MUST read `INCONCLUSIVE` with the note
  `oracle-self-certified`. A scenario MUST be marked scoring only when its verdict is
  `DISCRIMINATES`, `NEW_REACH` or `PRESERVES` and its origin is `ADMITTED_AT_BASE`; a
  `NEW_IN_DIFF` scenario keeps its verdict but never scores. Failure mode: a diff edits the
  outcome it is then judged by.
- `CCE-V0-028`: `prove` MUST reach the runner and the controls only through two injectable seams
  (one side run, one control set), so every ladder verdict, the test-tree overlay and the oracle
  cap are tested offline without Node or a browser. Failure mode: the ladder is covered only by
  live tests that no CI lane runs.
- `CCE-V0-029`: The `omitted-event` control for a step or a fixture setup is killed when the
  perturbed scenario fails or times out at any point, because removing the event is expected to
  break the outcome before or at the assertion. A perturbation line that is the outcome assertion
  itself MUST refuse `e2e-hook-event-not-found`. Failure mode: the control removes the assertion
  and reads killed for the wrong reason.
- `CCE-V0-030`: The test tree of CCE-V0-017 MUST be the compiled scenario, the generated runner
  configuration, the transitive closure of relative module specifiers (static `import`/`export ...
  from`, dynamic `import(...)` and `require(...)`, resolved as the path, then each TypeScript or
  JavaScript extension, then the TypeScript source of a `.js` specifier, then an `index` file), and
  the root package manifest and lockfiles; bare specifiers resolve through those manifests. A tree
  over 512 files reads `test-tree-unresolved`. An application file is any script outside the tree
  that is not test-like (a `.spec.` or `.test.` file, or a file under a conventional test
  directory); an application scan over 50,000 files reads `test-tree-unresolved`. Failure mode: a
  page object shared with application code is overlaid on the base side and changes the product
  under test.
- `CCE-V0-031`: `corvint-test e2e shadow --window FILE` MUST read one `e2e-shadow-window/0`
  document of recorded results (registration, gate, hosted tests and entries, each entry with its
  type, revisions, obligations, mutants, compiled scenarios with verdict, origin, killed mutants
  and real failures, and the hosted side's tests run, caught mutants, real failures and cost) and
  MUST NOT run either side. A missing operator set, seed, catalog digest or compiler version MUST
  refuse `e2e-shadow-unregistered`; an unknown member, a wrong schema, a duplicate entry or mutant
  ID, or a reference to an undeclared mutant, obligation or hosted test MUST refuse
  `e2e-shadow-window-invalid`. The `e2e-shadow/0` report is byte-deterministic. Failure mode: a
  shadow report scores results nobody registered.
- `CCE-V0-032`: Shadow readings: a compiled catch counts only from an eligible scenario
  (`DISCRIMINATES`, `NEW_REACH`, `REGRESSION_DETECTED` or `PRESERVES` with `ADMITTED_AT_BASE`), and
  an `UNKNOWN` obligation catches nothing; a miss is a hosted-caught mutant or a real failure the
  compiled side did not catch; an escape is a missed real failure only; a real failure is caught
  only by an eligible `REGRESSION_DETECTED` scenario listing it. The zero-escapes denominator is
  the `bug-fix` and `red-ci` entries the candidate ran in, and a zero denominator fails that
  criterion. The Holm family is every hosted test that caught at least one mutant. Bounds are exact
  Clopper-Pearson at 95%, a zero denominator reads upper 1 and lower 0, and every rate carries its
  numerator, denominator and side. The frozen gate is a compiled constant; a window whose gate
  differs fails `window-gate`. Failure mode: a vacuous denominator or a self-chosen family passes
  the gate.
- `CCE-V0-033`: The shadow report MUST state as limitations that recorded results are not
  re-verified, that `red-ci` completeness is assumed from the window, and that mutants of one entry
  are dependent (it also reports the entry-level miss rate). Failure mode: a passing report is read
  as independently re-run evidence.

The V1-1733 pivot (owner decision 2026-10-18; section Discrimination prover pivot) adds
CCE-V0-034..038, accepted under the standing auto-accept (2026-10-11). Lane 1022k implements
034..037; 038 is NOT_RUN (it needs an external checkout and host).

- `CCE-V0-034`: `prove` MUST accept a hosted Playwright test selected by file and optional title in
  place of a compiled scenario, run it with its test tree held at head against the base and head
  application trees, and report the CCE-V0-009 ladder. Its `oracle_origin` MUST come from joining
  the test file at base and head: unchanged reads `ADMITTED_AT_BASE`, changed `CHANGED_IN_DIFF`,
  absent `NEW_IN_DIFF`. Failure mode: a change that rewrites its own hosted assertion scores.
- `CCE-V0-035`: For a hosted test, a failure counts as at the outcome assertion only when the
  receipt's error location is one of the test's own assertion lines; otherwise the run reads
  `INCONCLUSIVE`. Failure mode: a setup failure at base reads `DISCRIMINATES`.
- `CCE-V0-036`: A behaviour-removal control (`omitted-event` on the action that precedes each
  assertion, CCE-V0-018/029) MUST be required for a positive verdict on a hosted test. The F1
  static-overlay falsifier of the spike MUST read `CONTROL_SURVIVED` or `INCONCLUSIVE`, never
  `DISCRIMINATES`.
- `CCE-V0-037`: A `PRESERVES` verdict on an outcome the diff does not change MUST NOT count as
  useful (V1-1713; the spike's F2 no-op falsifier).
- `CCE-V0-038`: The rerun on the four `actualbudget/actual` fixes that shipped hosted E2E tests
  (`3234be04d`, `c65884a27`, `eb28966d6`, `bc9bba67f`) MUST record per-change verdicts, minutes
  and usefulness in a build-log entry, pass or fail.

Lane 1022k (2026-10-22; V1-1733, V1-2378) adds CCE-V0-039 and 040, accepted under the standing
auto-accept of 2026-10-11, and pins these readings of 034..036:

- A hosted test is selected by `--hosted-test FILE` (a `.spec` or `.test` file with a TypeScript
  or JavaScript extension, present at head) and an optional `--hosted-title`; with no title the
  file must declare exactly one `test(...)`. It is run with `--grep` set to its escaped title.
  Its assertion lines are each line of the test body that starts with `expect(` or
  `await expect(`; the event line of each is the nearest preceding `await` line that is not an
  assertion. A missing file, no match or an assertion with no event refuses
  `e2e-hosted-test-invalid`, as do `--hosted-title` without `--hosted-test` and a hosted test
  together with compiled scenarios; several matching tests refuse `e2e-hosted-test-ambiguous`.
- The required controls of a hosted test are one `omitted-event` per event line
  (`control:omitted-event:step-N`); the template controls of CCE-V0-018 need an emitted assertion
  and do not apply. The hosted run uses the generated runner configuration, not the host's own
  configuration, projects or fixtures (a recorded proof limitation).

- `CCE-V0-039`: A test that times out MUST be read as that test's own outcome, not as lost
  infrastructure, when every other qualification of its receipt holds: an `omitted-event` control
  whose removed fixture step leaves an action waiting is killed (CCE-V0-029), and a timed-out side
  repeat reads a non-assertion failure. Failure mode: a load-bearing setup step reads
  `infrastructure_failed`, the proof reads `INCONCLUSIVE` and the control is never counted.
- `CCE-V0-040`: The CCE-V0-014 truncation MUST drop entries only from the end of one list per
  document (`blocks` of the catalog, `obligations`, the compile and prove `scenarios`, the shadow
  `entries`), keep the longest prefix whose encoding fits the cap, and set `truncated` to the
  number dropped; an untruncated document omits `truncated`. A document that exceeds the cap with
  that list empty MUST refuse `e2e-document-over-cap`. `shadow` refuses a window carrying a non-zero
  `truncated` with `e2e-input-truncated`; `compile` and `prove` recompute their inputs in process
  and read no emitted document. Failure mode: a capped report silently loses entries or changes a
  total.

Lane 1022k also adds CCE-V0-041..044, the first V1 authoring slice (V1-1683), accepted under the
standing auto-accept of 2026-10-11:

- `CCE-V0-041`: `corvint-test e2e harvest --rev R` MUST emit an `e2e-block-proposals/0` document
  (members `schema`, `harvester` = `corvint-test-e2e-harvest/0`, `revision`, `files`, `proposals`,
  `withheld`, optional `truncated`, `digest`) to stdout or to an `--out` file outside the
  repository (CCE-V0-001). It reads only the committed bytes of the tracked files at R whose names
  end in `.spec` or `.test` plus a TypeScript or JavaScript extension. It proposes an `outcome`
  block for each line of the literal form `[await ]expect(page.getByTestId('<id>')).<matcher>('<value>')[;]`
  with an admitted matcher (CCE-V0-005), and a `path` block (URL only, no screen) for each line
  `[await ]page.goto('/<url>')[;]`. Each proposal carries `provenance`
  `harvested:<blob>:<start>-<end>`, its blob and a span with the line's SHA-256, relation
  `harvested-proposal`, and the ID `<kind>:harvested:<first 16 hex of SHA-256 over kind, locator,
  matcher, value and URL>`, so moving a line keeps the ID and changing what it asserts or opens
  changes it. Repeats collapse to the first occurrence by path, then line; proposals are sorted by
  ID and the document is byte-identical across runs at the same revision. More than 2000 test
  files refuse `e2e-harvest-over-limit`. Page objects are not harvested: their callable methods
  are already admitted blocks (CCE-V0-001).
- `CCE-V0-042`: A line that names `expect(page.getByTestId(` or `page.goto(` but is not of a
  literal form above (a template literal, concatenation, variable, message argument or other
  matcher) MUST be listed under `withheld` with its path, line and reason `harvest-not-literal`;
  an empty value reads `outcome-vacuous` (CCE-V0-024) and a secret-shaped value (the CCE-V0-003
  screen) `harvest-secret-shaped`. A withheld line is never proposed and a secret-shaped value is
  never emitted.
- `CCE-V0-043`: The acceptance ledger `.corvint/e2e-acceptances.json` (closed schema
  `e2e-acceptances/0`: `acceptances`, each with a `proposal_id` holding `:harvested:` and a
  non-empty `reason`, no duplicates) MUST be read at the catalog revision; any schema error refuses
  `e2e-acceptances-invalid`. With the ledger present, the catalog admits a proposal as a block with
  relation `accepted-proposal`, its provenance, blob and span only when the ledger accepts its ID
  and the harvester proposes that ID at the same revision; an accepted ID not proposed there is
  unadmitted `acceptance-stale`. An unaccepted proposal never becomes a block, so `obligations`,
  `compile` and `prove` never read it. Without the ledger nothing is harvested and the catalog is
  unchanged. A harvested ID holds a second colon, which no declared or map-derived ID does, so an
  accepted proposal cannot shadow another block.
- `CCE-V0-044`: `obligations` and `compile` MUST list under `acceptances_in_diff` (omitted when
  empty) each accepted proposal admitted at head whose ID the base ledger does not accept, so an
  acceptance added by the change under test is visible to the reviewer and never trusted silently;
  a base ledger that does not validate accepts nothing. Not in this slice (V1 remainder):
  `CHANGED_IN_DIFF` and `NEW_IN_DIFF` oracle acceptance, the locator-repair loop, harvesting other
  assertion and navigation forms, and screen resolution of a harvested path.

Lane 1022k also adds CCE-V0-045..047, the runtime-coverage reach source of V1-1685, accepted under
the standing auto-accept of 2026-10-11:

- `CCE-V0-045`: `obligations`, `compile` and `prove` MUST accept `--coverage FILE`, a closed
  `e2e-runtime-coverage/0` document of at most 4 MiB (members `schema`, `revision`, `screens`, each
  with a head-map `screen` ID and the repository-relative `paths` observed executing while the
  application showed it). The `revision` MUST equal the head revision and each screen MUST be a
  head-map screen named once with clean relative paths; any other document, or an unreadable file,
  refuses `e2e-coverage-invalid`. A consulted document is named under `relations.consulted` as
  `runtime-coverage` with its SHA-256 as `coverage_digest`; without it `runtime-coverage` is listed
  under `relations.not_consulted` (CCE-V0-004). `prove --hosted-test` with `--coverage` refuses
  `e2e-hosted-test-invalid`. Recording the document is CCE-V0-053; a screen MAY also carry `pins`
  (`{path, blob}`, each naming one listed path once) and the document MAY carry `proof_digest`.
  A pin naming an unlisted or repeated path, or a blob the head revision does not hold for its
  path, refuses `e2e-coverage-invalid`.
- `CCE-V0-046`: A changed path that coverage lists under a screen MUST reach that screen as an
  impact hit with basis `runtime-coverage`, after impact's own hits (so an obligation impact reaches
  keeps impact's chain), under the same flow and `screen-without-outcome` rules as an impact hit.
- `CCE-V0-047`: Coverage MUST only widen reach. It never places a path for the unknown-span rules
  (CCE-V0-019/020), so every unknown span of the report without coverage stands and a report that
  reads `UNKNOWN` without coverage reads `UNKNOWN` with any valid coverage, never `NONE` or
  `REACHED`. Absence from coverage is never evidence that a path reaches nothing. Failure mode:
  coverage that names a changed path silently discharges an unknown span. Not in this slice
  (V1-1685 remainder): the TJAA module graph, API contract joins and the ANL-V0 ledger.

Lane cce2-1022u adds CCE-V0-048 (V1-2410), accepted under the standing auto-accept of 2026-10-11:

- `CCE-V0-048`: A run whose only failure is a run-level reporter error with a receipt location
  (PWP-V3-009) inside the staged test tree, outside `node_modules`, MUST read reason
  `teardown-failed at <tree-relative file>:<line>` and the scenario `INCONCLUSIVE` with that reason,
  never `run-without-qualified-receipt` and never an assertion result. An unlocated run-level error,
  or one located outside the tree or in `node_modules`, stays infrastructure. Failure mode: a
  worker-scoped fixture teardown that throws in the adopter's own fixture file is reported as an
  infrastructure fault, sending the adopter to the runner instead of the fixture. A BBF control run
  with such a failure still reads `infrastructure_failed`, with the teardown reason as its detail;
  CCE-V0-087 carries that detail into the control proof and the verdict.
- `CCE-V0-049`: (V1-2409) A hosted proof MUST carry `runner_config`: `applied` (the prove config
  name), the adopter's Playwright config found at head (the first default name, in Playwright's
  lookup order, from the hosted test's directory up to the repository root) with its SHA-256, and
  `deviations`, one `{key, adopter, prove}` entry per top-level or `use.` key whose effective value
  differs from the prove config's, where an unset adopter key takes Playwright's default. The
  adopter config is read as text and never executed; a value is recorded as written only when it is
  a number, a boolean or a short quoted string without credential characters, otherwise `<set>`.
  An absent, oversized (over 256 KiB) or unparsed config, or one with a spread or computed member,
  is named in `unread` and never refuses the proof. The prove config is still the one applied.
  Failure mode: a hosted verdict is read as the adopter's own suite outcome when its retries,
  projects, `globalSetup` or `storageState` were not applied.
- `CCE-V0-050`: (V1-2411) Every harvested proposal MUST carry `binding`, read from the application
  map (`appmap.json`) at the harvested revision: a path proposal binds to the `screen` the map
  resolved for the same navigation line of the same test file (basis `app-map-goto`); an outcome
  proposal binds to the one `flow` declaring an outcome on the same locator (basis
  `flow-outcome-locator`), else to the `screen` the map gives its test file (basis
  `test-file-screen`). Otherwise `binding` carries exactly one `unbound` reason from the closed set
  `app-map-unavailable`, `test-file-not-mapped`, `goto-unresolved`, `goto-not-mapped`,
  `flow-ambiguous`, `no-flow-or-screen`. A missing map never refuses the harvest, and the binding
  never changes a proposal ID or admits a proposal (CCE-V0-043). Failure mode: an accepted proposal
  is compiled against a flow it was never observed on. The site-knowledge IR (SKH-V0, not yet on
  this base) is not consulted; binding through it is the V1-2411 remainder.
- `CCE-V0-051`: (V1-2413) A `CHANGED_IN_DIFF` scenario's proof MUST carry `oracle_digest`
  (`sha256:` of the scenario's test text at head). `prove --oracle-acceptance FILE` reads a closed
  `e2e-oracle-acceptance/0` document `{schema, base, head, acceptances: [{assertion_id,
  oracle_digest, accepted_by, reason}]}` recorded outside the repository; a path inside the
  repository, an unreadable or over-1-MiB file, unknown members, a trailing value, another schema,
  `base` or `head` other than the resolved revisions, an empty field or a duplicate
  `assertion_id` refuses `e2e-oracle-acceptance-invalid` before any run. An entry naming the
  scenario's assertion lifts the CCE-V0-016 `oracle-self-certified` cap only when its
  `oracle_digest` equals the proof's and `accepted_by` is neither the head commit's author nor its
  committer (name or email, case-insensitive); the verdict then stands with the note
  `oracle-accepted by <acceptor>`. Every applied entry is recorded as `oracle_acceptance`
  `{document_digest, accepted_by, reason, status}`, where `status` is `accepted`,
  `oracle-digest-mismatch` or `acceptor-is-head-author`; the last two keep the cap and say so in
  the note. An accepted verdict never scores (CCE-V0-010 counts `ADMITTED_AT_BASE` only), and an
  entry for any other origin is ignored. Failure mode: the change that edits an outcome also
  certifies it, or an acceptance outlives an edit to the oracle it accepted. Identity is the
  recorded name only; the document carries no signature, so independence is a reviewable claim,
  not a cryptographic one.
- `CCE-V0-052`: (V1-2413) When every head repeat of a compiled (not hosted) scenario fails at the
  outcome assertion with `locator-unresolved`, the proof MUST carry `locator_repair`
  `{locator, proposed | unproposed, applied: false, evidence}`. The repair reads the zero-context
  base-to-head diff and pairs removed with added lines in order within each hunk; a pair whose
  removed line names the outcome locator as its `data-testid` and whose added line is identical
  except for that value is evidence `{path, base_line, head_line, candidate}`. One distinct
  candidate is `proposed`; several are `unproposed: repair-ambiguous`; none is
  `no-rename-in-diff` (`diff-unavailable` if the diff cannot be read). The proposal is never
  applied: the scenario, its verdict and its scoring are unchanged, and accepting it is a new
  compile. Failure mode: a renamed test id silently rewrites the oracle so that a regression reads
  as a pass.
- `CCE-V0-053`: (V1-2412) `prove --record-coverage` MUST wrap the head pool's in-process static
  server in a recorder of every request path it answers 200 or 304 (a directory as its
  `index.html`), attribute the paths to the screen the scenario's outcome assertion shows, drop
  test-tree and `node_modules` paths, and pin each remaining path to the blob the head revision
  holds for it. Each scenario proof then carries `recorded_coverage` `{screen, paths, pins,
  receipts}` (the head pool's qualified receipt digests) or `unrecorded` with one of
  `hosted-test-without-screen`, `declared-environment-serve`, `head-pool-unqualified`,
  `no-application-path-served` or `head-blobs-unreadable`, and an unrecorded scenario pins nothing.
  The base pool is never recorded. Prove writes `e2e-runtime-coverage.json` beside the proof, an
  `e2e-runtime-coverage/0` document at the head revision with the proof digest and each recorded
  screen's union of pinned paths, which `--coverage` (CCE-V0-045..047) consumes unchanged.
  Coverage is file-served, not line-executed: a served file counts as covered for its screen even
  if none of its code ran. Failure mode: coverage recorded at one revision widens reach at another,
  or a base pool's paths are attributed to the head.
- `CCE-V0-054`: (V1-1685) `obligations`, `compile` and `prove` MUST accept `--module-graph`, which
  builds the TJAA-V0 TypeScript/JavaScript module graph of the immutable head tree (never the
  worktree) and names `tjaa-module-graph` under `relations.consulted`, with `module_graph`
  `{digest, frontier}` or `{unavailable}` (`repository-unreadable`, `head-tree-unreadable`,
  `module-graph-unbuildable`); without the flag the relation is listed under
  `relations.not_consulted`. Each changed path the graph owns MUST reach every screen that places a
  file of a unit reaching it through imports (a router anchor, with its lineage descendants, a map
  test file or a flow anchor) as an impact hit with basis `tjaa-module-graph:<importer>`, after
  impact's and coverage's hits and never repeating one of theirs. Like coverage (CCE-V0-047), the
  graph only widens: it never places a path for the unknown-span rules, a hit on a screen without an
  outcome adds no `screen-without-outcome` span, and every unknown span of the report without the
  graph stands with its reason; the reached screens' map unknowns may join them (CCE-V0-019). The
  graph's frontier is reported, never treated as evidence of no reach. Failure mode: a changed
  helper module whose importer impact's reference hop cannot prove leaves every outcome of its
  importer's screens without an obligation.

The 2026-10-21 fix lane (V1-1711) adds CCE-V0-086; lane cce2-1022u uses 048..054; IDs 055..085 are
left free for parallel lanes.

- `CCE-V0-086`: A relative `--root`, including the default `.`, MUST resolve against the working
  directory to its absolute, symlink-resolved path before any Git or map read, for `catalog`,
  `obligations` and the commands built on them. Failure mode: the map reads run Git with the
  relative root applied twice and the command refuses `e2e-map-unavailable: evaluated revision is
  not a commit`, blaming the revision.

The 2026-10-23 flowdocs/appmap/CCE fix lane (V1-2427, V1-2428) adds CCE-V0-087 and 088, accepted
under the standing auto-accept of 2026-10-11:

- `CCE-V0-087`: A BBF control that reads `infrastructure_failed` and any of whose attempts' hook
  receipts records a CCE-V0-048 teardown (detail `teardown-failed at <tree-relative file>:<line>`)
  MUST keep each distinct such detail among its control-proof `reasons`, after BBF's own aggregate reasons, and
  MUST make the verdict `INCONCLUSIVE` with note `teardown-failed at <file>:<line> (control <id>)`,
  never `control-not-killed:<id>`. The BBF status stays `infrastructure_failed`; BBF's vocabulary is
  unchanged. A surviving control still reads `CONTROL_SURVIVED` first, since a survivor is evidence
  about the test whatever another control's teardown did; any other infrastructure detail keeps
  `control-not-killed:<id>`. Failure mode: a control whose perturbed run failed in the adopter's own
  fixture teardown reads as lost infrastructure, hiding the fixture defect behind the discrimination
  verdict.
- `CCE-V0-088`: `harvest` MUST read the application map from a configured repository-relative
  manifest (`HarvestOptions.Manifest`, the same field and `appmap.json` default as
  `CatalogOptions.Manifest`) to bind proposals (CCE-V0-050). Without a map at that path every
  proposal stays unbound with `app-map-unavailable`, as before. Failure mode: an adopter whose map is
  not `appmap.json` at the root gets every harvested proposal unbound although the catalog reads
  the same map. `corvint-test e2e harvest --manifest PATH` passes the path to `HarvestOptions`.

The 2026-10-24 CCE V1 lane (p1-cce: V1-1687, V1-1685, V1-1684) adds CCE-V0-100..104, proposed and
experimental under the standing auto-accept of 2026-10-11 until the owner accepts them. Each item's
rollback is to remove its code and tests; no stored state migrates, and an artifact written without
the new members reads as before.

- `CCE-V0-100`: The ordinary `go test` lane MUST replay the verdict ladder's
  `DISCRIMINATES`, `NEW_REACH`, `PRESERVES`, `REGRESSION_DETECTED` and `UNSTABLE` rungs from recorded
  side receipts, with no Node or browser. A recording is one `cce-recorded-side/0` file under
  `internal/changee2e/testdata/recorded/` (strict decoding: `schema`, `provenance` `live` or
  `synthetic`, `toolchain`, `compiler`, `scenario_sha256`, the absolute `tree` the pool ran in, and
  `receipt`, the canonical `corvint-playwright-external/3` document). The replay decodes the receipt
  with the provider's own `DecodeAttemptReceipt`, moves only the receipt's failure and
  infrastructure locations from the recorded tree (matched as a slash path, so a recording replays
  on any host platform) onto the staged one, qualifies the result with Prove's `qualify` (the
  admitted runtime and browser tuple, the qualified test ID and the attempt details), and runs
  the real staging, location and verdict code through the CCE-V0-028 seams; the BBF controls stay
  scripted. `scenario_sha256` pins the compiled scenario below its header (which names fixture commit
  revisions), and `compiler` MUST equal `CompilerVersion`. A missing recording reads
  `recording-missing`, a non-canonical receipt or unknown schema or provenance `recording-invalid`,
  and a recording of other scenario text `recording-stale`; each leaves its side unqualified and the
  verdict `INCONCLUSIVE`, never a synthesized outcome. Failure modes: replay trusts a recorded
  outcome the provider would not qualify; a stale recording reads as current; a ladder change goes
  untested in the ordinary lane because the live tests skip. Acceptance:
  `TestCCEV0100ReplayRecordedReceipts`, `TestCCEV0100ReplayRefusals`.
- `CCE-V0-101`: The live ladder MUST be able to refresh the recordings: with
  `CORVINT_CCE_RECORD_DIR` set and the live toolchain present, `TestCCEV0101LiveRecord` writes each
  qualified side of the rungs a live run reaches on the fixture (`DISCRIMINATES`, `PRESERVES`) as a
  `provenance: live` recording that replaces the synthetic one. `go test -cce-write-recordings`
  regenerates the synthetic set, whose receipts carry the live receipt's shape (the generated config
  and placement under a fixed synthetic tree, the declared application identity, the qualified
  runtime and browser tuple) and no host path. Without the variable the test skips. Failure mode:
  recordings drift from what a live pool emits and replay proves a ladder no real run can reach.
- `CCE-V0-102`: The live prove tests MUST fail, not skip, when `CORVINT_CCE_LIVE=required` and their
  toolchain is missing, and `make cce-live` runs them so. A CI job that sets the variable therefore
  cannot pass by skipping. Failure mode: a CI lane without Node or Playwright reports the live tests
  green. The CI job itself is `.github` work that needs the owner.
- `CCE-V0-103`: `obligations`, `compile` and `prove` MUST accept `--ledger FILE` (`DiffOptions.Ledger`),
  an `application-edge-ledger/0` document, and pass it to the AMAP/ANL impact relation instead of
  nil. A ledger of another application, map digest or revision than the head map, a trimmed ledger,
  or an unreadable or wrong-schema document refuses `e2e-ledger-invalid`. A bound ledger moves
  `anl-edge-ledger` from `not_consulted` to `consulted`, records the ledger's digest as
  `ledger_digest` (omitted without a ledger) and changes the impact digest, but never changes reach:
  the state, obligations, unknown spans and exemptions equal the report without it. A hosted-test
  `prove` refuses `--ledger` as it refuses `--coverage`. Failure mode: a ledger from another revision
  silently feeds reach, or a consulted ledger is not recorded.
- `CCE-V0-104`: With a bound ledger, a flow whose first screen has no admitted deep-link entry MUST
  compile over the ledger-bound AMAP entry projection's shortest `PROVEN` path (AMAP-V0-080) only:
  the path must start at an admitted `path:entry:` block; each hop is the ledger edge between its
  screens proven by the projection's test key (the lowest edge ID when several are); and the hop
  emits the admitted page-object method of that edge's crossing from-step, the mined, anchored
  statement a receipt saw cross the edge, before the flow's own steps. The compiled entry names
  `proven_path` and `proven_by`, and its `blocks` include the entry and each hop method. A hop
  abstains with one named path reason and nothing is generated for it: `proven-path-entry-not-admitted`,
  `proven-hop-edge-missing`, `proven-hop-evidence-inconsistent` (the crossing names no map flow, or
  its steps are not adjacent on the hop's screens), `proven-hop-precondition-not-set` (the crossing
  flow declares a precondition the compiled flow does not), `proven-hop-prefix-not-emitted` (the
  crossing step follows another step of its flow on the same screen, such as a fill before its
  submit, which only the crossing step would not carry), `proven-hop-step-is-fill`, or
  `proven-hop-` plus the CCE-V0-006 step reason. A screen the projection does not reach keeps its
  catalog reason, and without a ledger the output is unchanged. Precondition flows and session or
  parameter state are not synthesized. Failure modes: a path through an unproven, unverified or
  receipt-less edge; a generated navigation statement; a hop whose crossing ran under a fixture the
  scenario does not set up.

The 2026-10-24 CCE V1 lane p1-cce-2 (V1-1684 block vocabulary, V1-1685 API contract reach) adds
CCE-V0-105..115, proposed and experimental under the standing auto-accept of 2026-10-11 until the
owner accepts them. Every new declarations member is optional: a `.corvint/e2e-blocks.json` without
them reads exactly as before, and a binary without this slice refuses a file that uses them
(`e2e-blocks-invalid`, closed decoding), never silently ignores it. Compiled scenarios are never
committed, so no stored state migrates; rollback is to remove the members' code and tests and
restore `CompilerVersion` `corvint-test-e2e-compile/0`, after which a recording made under `/1`
refuses as CCE-V0-100 defines.

- `CCE-V0-105`: A declared fixture MAY name `outputs`, at most eight lower-camel identifiers, each
  once. Its setup method's return value is the fixture value: the compiled fixture throws
  `corvint-e2e fixture-output-missing NAME` before the test body runs when an output is undefined,
  and otherwise passes the value to the test. A name outside the identifier form, a repeat, an empty
  name or a ninth output refuses `e2e-blocks-invalid`. Failure mode: a later block reads an output
  the setup never produced and the scenario fails for a seeding reason that reads as behaviour.
  Acceptance: `TestCCEV0105FixtureOutputs`.
- `CCE-V0-106`: Data MUST flow between blocks only through a declared output: a route parameter
  bound to `fixture` and `output` renders `encodeURIComponent(String(fxN.output))` in the entry URL,
  where `fxN` is that fixture as the scenario sets it up. A scenario whose flow does not set the
  fixture's precondition misses its path block with `route-param-unbound:NAME`; a route naming an
  unadmitted fixture keeps its entry unadmitted with `route-fixture-unadmitted`, and one naming an
  output the fixture does not declare with `route-output-undeclared`. Failure mode: a URL reads a
  value no block in the scenario produces. Acceptance: `TestCCEV0106OutputDataFlow`.
- `CCE-V0-107`: A declared `session` (`session_id` in the `session:` namespace, non-empty
  `satisfies`, `storage_state_env`) satisfies a flow precondition no fixture satisfies. The compiled
  scenario runs under `test.use({ storageState: process.env.VAR })` and a file-level
  `test.skip(!process.env.VAR, 'corvint-e2e session-unset VAR')`, its blocks name the session, and
  the run environment declares `VAR` (its value recorded in the receipt) only when it is set, so a
  storage-state path never enters the scenario text. Two sessions in one scenario miss with
  `session-conflict`. A variable outside `^[A-Z_][A-Z0-9_]{0,63}$`, a reserved runner variable, a
  `CORVINT_` name, a repeated or non-`session:` ID and an empty `satisfies` refuse
  `e2e-blocks-invalid`. Failure modes: a signed-in scenario runs signed out and fails for that
  reason; a session path or secret is written into a generated file. Acceptance:
  `TestCCEV0107Session`.
- `CCE-V0-108`: Every fixture MUST compile to a Playwright `base.extend` fixture: setup before
  `use`, the declared cleanup method (callable, so with no parameters, reading its state from the
  page object) after it, so teardown runs whatever the test body does; for a setup with outputs the
  output check and `use` sit in a `try` whose `finally` runs cleanup, so a missing output still
  tears down. One test-scoped `po` fixture builds each page-object class once, and every fixture
  and the test body read their instances from it, so setup, actions and cleanup share one instance
  per class. A failure located at the
  cleanup call reads `cleanup-failed`, not an assertion. Only a setup without outputs is an
  omitted-event control: removing a setup whose outputs later blocks read fails by construction,
  which would not show the setup's effect is load-bearing, so that control is not generated and
  the prove limitations say so. Failure modes: a teardown is skipped after a failing step; a vacuous
  kill counts as control evidence. Acceptance: `TestCCEV0108Teardown`.
- `CCE-V0-109`: A declared locator (`block` an outcome or input ID; `by` `role` with a lower-case
  ARIA `role` and accessible `name`, or `label` with a `name` and no role) replaces the block's test
  ID: `page.getByRole(ROLE, { name: NAME, exact: true })` or `page.getByLabel(NAME, { exact: true })`.
  The wrong-locator control perturbs the accessible name in the declared form, and an unknown form
  refuses the perturbation. A locator for a block the catalog does not admit is withheld with
  `locator-block-unadmitted`; a malformed locator refuses `e2e-blocks-invalid` and a secret-shaped
  name `e2e-input-secret`. The typed locator is part of the oracle: `oracle_origin` (CCE-V0-016)
  compares the base declarations' locator for the outcome too, so adding, changing or removing one
  in the diff reads `CHANGED_IN_DIFF`. Failure modes: a role locator silently falls back to a test
  ID, or its control perturbs nothing; a diff rewrites an assertion's locator and keeps
  `ADMITTED_AT_BASE`. Acceptance: `TestCCEV0109TypedLocators`.
- `CCE-V0-110`: `toHaveAccessibleName` and `toHaveCount` MUST be admitted outcome matchers. A count
  is a positive integer below one million rendered unquoted; its wrong-expected-value control is
  the next integer. A zero count reads `outcome-vacuous` and a non-integer or negative count
  `outcome-not-compilable`. Failure mode: a count assertion compares a string, or a zero count
  passes on an empty page. Acceptance: `TestCCEV0110TypedAssertions`.
- `CCE-V0-111`: A declared `route` MAY bind every required URL parameter of an entry whose only
  catalog reason is `requires-parent-params`; each parameter is a constant `value` or a fixture
  output, its names equal the entry's in order-free comparison, and constants are URI-encoded.
  Without a route the entry keeps `requires-parent-params` and nothing is synthesized from an
  unproven edge (CCE-V0-104 remains the only path synthesis). An entry that is deep-linkable or not
  in the map reads `route-not-parameterised`, a guarded or otherwise unadmitted entry keeps its own
  reason, and a name mismatch reads `route-params-mismatch`; each replaces the entry's earlier reason
  rather than joining it. A parameter with both or neither binding, a non-identifier name or an
  over-long value refuses `e2e-blocks-invalid`, and a secret-shaped value `e2e-input-secret`.
  Failure mode: a guarded or unproven screen compiles as directly reachable. Acceptance:
  `TestCCEV0111ParameterisedRoutes`.
- `CCE-V0-112`: Every compiled action MUST be a `test.step` named by its block ID, the scenario MUST
  set `trace: 'retain-on-failure'`, and the compile entry MUST list each step's line and block
  (`steps`). Failure mode: a failing action cannot be traced to the block that emitted it.
  Acceptance: `TestCCEV0112StepsAndTrace`.
- `CCE-V0-113`: Each vocabulary addition MUST have a golden compile output under
  `internal/changee2e/testdata/golden/` (plain, fixture output, count, and session plus route plus
  role locator) whose `compiler` header equals `CompilerVersion` (`corvint-test-e2e-compile/1`),
  with only the revision and digest header lines normalised; `go test -cce-write-golden` regenerates
  them. A golden made by another compiler fails until regenerated, as a CCE-V0-100 recording does.
  Failure mode: a rendering change ships without a reviewed output diff. Acceptance:
  `TestCCEV0113GoldenCompile`.
- `CCE-V0-114`: `api_contracts` at base (each a `contract:` ID, one or more relative path globs and
  an absolute endpoint literal of at most 256 bytes without quotes or whitespace) MUST join a changed
  path matching a glob to each screen that places a head-tree file naming the endpoint (not followed
  by an identifier byte), with basis `api-contract:CONTRACT:FILE` in the chain and
  `e2e-blocks:api_contracts@base` among the consulted relations. The join only widens reach: hits
  are not counted as placed, a pair impact or another source already reached is not repeated, a hit
  on a screen without an outcome adds no span, a contract the diff itself declares joins nothing,
  and over 512 placed files the join adds nothing. A malformed contract refuses
  `e2e-blocks-invalid`. Failure modes: a backend change reads `NONE` because no file imports it; a
  diff widens or narrows its own reach. Acceptance: `TestCCEV0114APIContractWidensReach`.
- `CCE-V0-115`: A reach source MUST NOT turn `UNKNOWN` into `NONE`: with runtime coverage naming a
  changed path on every screen, the module graph and an API contract join all on, every unknown span
  of the report without them is still an unknown span (its one reason may be restated by the ranked
  CCE-V0-020 order, never dropped) and the state reads `UNKNOWN`. Failure mode: a widening source
  that also places a path removes the span that kept the report honest. Acceptance:
  `TestCCEV0115NoReachSourceTurnsUnknownToNone`, with `TestCCEV0046CoverageOnlyWidensReach` and
  `TestCCEV0054ModuleGraphWidensReach` for each source alone.

The 2026-10-24 CCE live-diagnosis lane (V1-2620) adds CCE-V0-116, proposed and experimental under
the standing auto-accept of 2026-10-11 until the owner accepts it. Rollback is to remove its code,
fields and tests; no stored state migrates, and a report or hook detail written without the fields
reads as before.

- `CCE-V0-116`: The prove report MUST keep, for each run that ends without a qualified receipt, the
  concrete cause, in additive fields only. An unqualified side's `head` or `base` proof carries
  `loss` and each BBF control attempt that reads `infrastructure_failed` is listed under the
  control's `lost_attempts` (attempt number and BBF's attempt reasons). Each loss holds the qualify
  reason; for a run-level reporter error its `error_location` (tree-relative `path:line`,
  `outside-test-tree`, or `unlocated`); and its `error_message` (the provider failure detail or the
  provider's run error), ANSI-stripped, with tree roots made relative and any other absolute path
  (POSIX, drive or `file:///` URL, spaces included) replaced by `<path>`, secret-screened, and
  bounded to 512 bytes. An attempt that kept no hook
  receipt carries `hook_process`, its hook process evidence. The control hook carries the location
  and message to the prover on the second and third lines of its infrastructure `detail`; a
  one-line detail, as hooks wrote before, reads as the reason alone; the reason stays whole and the
  location and message are cut so the detail stays within BBF's 4096-byte bound. No verdict, note, status,
  reason list or score reads these fields, and CCE-V0-087 attribution is unchanged. A live
  `internal/changee2e` test that fails on a verdict prints every loss. Failure mode: a CI control
  reads `infrastructure_failed` with only `one-or-more-attempts-lost-to-infrastructure`, so the
  cause of an `INCONCLUSIVE` live run cannot be told from the log (CI run 40129429349). Acceptance:
  `TestCCEV0116LostControlAttemptDetail`, `TestCCEV0116TeardownDetailCompatible`,
  `TestCCEV0116SideLoss`, `TestCCEV0116LossTextRedactsHostPaths`,
  `TestCCEV0116HookDetailWithinBBFBound`.

## Shadow evaluation and retirement

The frozen V0 gate values, which only an owner decision may change:

- the window holds at least 30 entries, chosen and digest-pinned before any compiled run. Each entry
  is one of three types, and merged changes alone would make the escape criterion vacuous:
  - `merged`: application base is the merge's first parent, application head is the merge, and the
    test revision is the head;
  - `bug-fix`: for a fix commit F whose parent P fails a hosted test present at P that passes at F,
    the application base is F, the application head is P (the reverse diff reintroduces the bug),
    the test revision is F on both sides and for both suites, and outcomes admitted at F count as
    `ADMITTED_AT_BASE`; the real failures are the hosted tests that fail at P and pass at F;
  - `red-ci`: a commit whose hosted CI failed, with the application base at its merge base with the
    main line and the test revision at the base, so a change cannot rewrite the oracle that judges
    it; the real failures are the hosted tests that fail at head and pass at base.

  At least 10 entries are `bug-fix`, and every `red-ci` commit in the period is included;
- seeded regressions are preregistered application-side mutants: the operator set, seed, catalog
  digest and compiler version are frozen at registration, and the same mutants apply to both sides;
- seeded-regression parity: per retirement candidate, the compiled side catches 100% of the seeded
  regressions the hosted test caught in the window, and in total at least 95% of all seeded
  regressions the hosted affected selection caught;
- escapes: zero real hosted failures the compiled side missed for that candidate;
- candidate evidence: the candidate ran in at least 5 window changes and caught at least 1 seeded
  regression;
- statistical bound (proposed, owner question 4): the sampling unit is a seeded mutant the hosted
  candidate caught, and the hypothesis family is every candidate considered in the window. For each
  candidate, H0 is "the compiled miss rate is above 5%", tested by the exact one-sided binomial
  test; Holm orders the m candidates by p-value and compares the k-th smallest with 0.05/(m-k+1).
  A candidate qualifies only when its hypothesis is rejected. With zero misses and no correction
  that needs at least 59 caught mutants (0.95^59 is about 0.049); the first Holm step for m
  candidates needs n with 0.95^n at most 0.05/m, so 72 for m = 2. Mutants of one entry are not
  independent, so the report also gives the bound with entries as the unit;
- a development window used to tune the compiler is never reused as the held-out shadow window.

A failed shadow evaluation stays recorded as failed; it is not renamed development progress.

## Non-goals and simpler baseline

- No model or LLM in the compile path. Composition is deterministic over admitted blocks. A local
  agent may author or propose hosted blocks, which humans review; it never writes a compiled
  scenario's oracle.
- No inferred expected values, locators or paths; the oracle is always a declared outcome.
- No deletion or disabling of hosted tests, and no automatic selection narrowing, in V0.
- Playwright only in V0; Nightwatch and other runners are later profiles.
- No visual or screenshot diffing, no cross-application data handoff (inherited from AMSP-V0), no
  learned ranking of paths or blocks.
- Simpler baseline: keep the hosted suite and run `corvint affected` selection only. Every shadow
  report states the compiled side's results against this baseline.

## Trust boundary, limits and failure modes

- The read subcommands (`catalog`, `obligations`, `compile`, `shadow` over existing receipts) are
  read-only and write only to stdout or an explicit `--out` path outside the repository root
  (CCE-V0-001; invariant 4). Only the opt-in `prove` subcommand executes browsers, and only in a
  marked disposable workspace (BBF-V0 boundary).
- Compiled scenarios are generated code run against the user's application. They contain only
  admitted block text, so they add no code path a hosted block does not already contain.
- Limits: at most 64 obligations per compile, 16 scenarios proven per run, 100 repeats per side,
  one worker; each limit refuses with a named code when exceeded rather than sampling, except the
  obligation limit, which under CCE-V0-019 reads `UNKNOWN` with `fan-out-over-limit` instead. The
  first slice's `e2e-obligations-over-limit` refusal is superseded (V1-1678) and no longer emitted.
- Fail-closed choices: unknown reach reads `UNKNOWN` (never `NONE`); a missing block reads
  `INCOMPLETE`; a missing receipt reads `INCONCLUSIVE`; none of these narrows hosted selection.
- Secrets: `input` and `outcome` values pass the existing secret screen per block; a matching block
  is unadmitted with `secret-suspected` (CCE-V0-024; the first slice refuses the whole catalog with
  `e2e-input-secret`), and PWP `/2` redaction applies to run evidence.
- Isolation: `prove` passes only an allowlisted environment and blocks non-loopback egress
  (CCE-V0-022); page-derived text stays in `untrusted_text` (CCE-V0-025).
- 2026-10-22 lane codes: refusals `e2e-shadow-unregistered`, `e2e-shadow-window-invalid` and
  `e2e-hook-event-not-found`; `INCONCLUSIVE` notes `test-tree-unresolved`,
  `test-tree-overlaps-application`, `oracle-self-certified` and
  `base-mixed-or-failed-off-assertion`; unadmitted-block reason `outcome-vacuous`.
- First-slice codes (V1-1665), each fail-closed:
  - Refusals: `e2e-root-required`, `e2e-revision-invalid`, `e2e-map-unavailable`,
    `e2e-impact-unavailable`, `e2e-repeats-over-limit`,
    `e2e-scenarios-required`, `e2e-scenarios-over-limit`, `e2e-scenario-invalid`,
    `e2e-toolchain-unavailable`, `e2e-tree-unavailable` and `e2e-out-unsafe` (a destination below
    `--out` is a link, leaves it, or repeats a placement).
  - Control hook refusals: `e2e-hook-definition-invalid`, `e2e-hook-toolchain-unavailable`,
    `e2e-hook-line-out-of-range` and `e2e-hook-assertion-not-found`.
  - Unadmitted-block reasons: `edge-unproven`, `method-not-callable`, `outcome-not-compilable` and
    `screen-unresolved`.
  - `INCOMPLETE` reasons: `no-admitted-fixture` and `no-flow-step-expects-outcome`.
  - Unknown-span reasons: the closed CCE-V0-020 set (V1-1678); the first slice emitted only
    `unplaced-path` (a changed path that impact neither places on a screen nor reports unknown).
  - Control `not_run` reasons: `verdict-decided-before-controls`, `control-workspace-unavailable`,
    `proof-repository-unavailable`, `plan-refused` and `execute-refused`.

## Deterministic acceptance and traceability

| Requirement | Planned evidence | Status |
|---|---|---|
| CCE-V0-001..003 | catalog golden over the AMAP-V0 fixture plus a declared `e2e-blocks.json` | PASS (first slice, one hand-written declarations file): `TestCCEV0001CatalogDeterministicAndBound`, `TestCCEV0002DeclarationsFailClosed`, `TestCCEV0001OutInsideRootRefuses`, `TestCCEV0001OutNestedSymlinkRefused`, `TestCCEV0001OutFileSymlinkRefused` |
| CCE-V0-004 | obligations golden for a one-screen diff, an unknown-reach diff and a no-UI diff | PASS: `TestCCEV0004OneScreenDiffReachesOneOutcome`, `TestCCEV0004UnboundedUnknownAndUnplacedPathUnknown`, `TestCCEV0004EarlierScreenReachesLaterStepOutcome`; the amended base-and-head map join (V1-1678) by `TestCCEV0019RemovedAtHeadUnknown`, and the report's `relations` member names the consulted relations (AMAP impact, the base and head maps, the base `reach_exempt`) and those not consulted (affected plan, ANL edge ledger) |
| CCE-V0-005..007 | compile golden, byte-identical rerun, `INCOMPLETE` cases per missing block kind | PASS (first slice; a missing fixture and unknown reach exercised): `TestCCEV0005CompileFromAdmittedBlocks`, `TestCCEV0006MissingBlocksIncomplete`, `TestCCEV0007PlacementsInjective` |
| CCE-V0-008..010 | live local Playwright run on the fixture app: head pass, base fail, two BBF controls | 008 and the pre-amendment 009 ladder PASS (the amended ladder is PASS offline, see the panel row; error-location matching is PARTIAL under the CCE-V0-009 note): `TestCCEV0008ProveLimitsRefuse`, `TestCCEV0008UntarSkipsLinks`, `TestCCEV0009VerdictOrder`, `TestCCEV0009AssertionDetector`, `TestCCEV0009HeadIntermittency`, `TestCCEV0009ControlPerturbation`; live (Playwright 1.63.0, Node v22.23.2, 5 repeats) `TestCCEV0009LiveDiscriminates` (head 5/5 pass, base 5/5 fail at the outcome assertion, both controls killed, qualified PWP receipts) and `TestCCEV0009LivePreserves`; the live tests skip without `CORVINT_PLAYWRIGHT_MODULES`. 010 PASS (lane 1022k): `TestCCEV0010NoSelectionFields`, `TestCCEV0013NoRepositoryWrites`, and scoring under CCE-V0-027 |
| CCE-V0-011..013 | shadow report over a synthetic window; adopter window `NOT_RUN` | 011/012 PASS over a synthetic window (see the panel row); adopter window `NOT_RUN`; 013 NOT_STARTED |
| CCE-V0-014 | byte-cap truncation tests per document | PASS (lane 1022k, V1-2378): `TestCCEV0014DocumentByteCap` (untruncated without `truncated`; a capped shadow report keeps a deterministic prefix, names the dropped count and keeps the totals; `e2e-document-over-cap`), `TestCCEV0014TruncatedInputRefused`, `TestCCEV0040CapListPrefix`; the same helper caps catalog, obligations, compile and prove |
| CCE-V0-015 | core baseline comparison before and after: `cmd/corvint` commands, flags and transitive imports unchanged and no CCE package in its import graph; no `init` side effect in any package both binaries import; every CCE-V0 command reachable as `corvint-test e2e <sub>`; a subprocess check that the companion never executes `corvint` | PASS: `TestCCEV0015SubcommandsReachable`, `TestCCEV0015CoreBoundary`, `TestCCEV0015NeverExecutesCorvint`; core tree byte-identical to the claim base outside docs, `go list -deps ./cmd/corvint` has no CCE package |

Panel amendment (2026-10-17). The planned evidence is the acceptance of the named ticket; each
row's status says what is delivered.

| Requirement | Planned evidence | Ticket | Status |
|---|---|---|---|
| CCE-V0-003 (`environment`) and 021 | declarations with the member; unknown members refuse; `serve` without a shell; failed `ready` reads `INCONCLUSIVE` | V1-1681 | PASS: `TestCCEV0021EnvironmentShape` (the closed shape; non-loopback, portless, `https` or path-carrying `base_url`, empty argv, relative `ready`, `NAME=value`, reserved names (`PATH`, `HOME`, the prove toolchain names, `CORVINT_BEHAVIOR_*`), a `serve[0]` escaping the tree and unknown members refuse `e2e-blocks-invalid`), `TestCCEV0021ServeTree` (argv run without a shell inside the tree, only `PATH`, `HOME` and declared names reach it, an already answering port refuses `port-in-use`, missing tool, a serve exiting before or after `ready` reads `serve-exited`, `ready` needs a 2xx answer within 120 s, the leader stays unreaped until its process group is killed and the port is released on stop, `environment-unavailable:*` reads `INCONCLUSIVE`); live (Playwright 1.63.0, Node v22.23.2) `TestCCEV0021LiveEnvironmentDiscriminates` (both sides and both BBF controls run against the declared serve; the proof records the environment digest) and the static tests now assert `environment: static`. `TestCCEV0021ServeCommandResolution` (a relative `serve[0]` resolves inside the tree, a symlink out refuses `serve-outside-tree`, a bare name is the executable Prove resolved once on its PATH), `TestCCEV0021ReadyRequires2xx`, `TestCCEV0021ControlRequestEnvironment` (the BBF hook receives only set declared names plus the resolved serve executable and PATH, so its serve matches the sides). Not detected (recorded as a proof limitation): a serve that binds another port, a detached child outliving the group |
| CCE-V0-003 (`reach_exempt`), 020 | declarations golden with the member | V1-1678 | PASS: `TestCCEV0020ReachExemptAndNone` (an exempt-only diff over a clean map reads `NONE` citing the base glob; map unknowns keep it `UNKNOWN`; a diff adding its own glob stays `UNKNOWN`), `TestCCEV0020ReachExemptInvalidRefuses` |
| CCE-V0-009 (amended), 017, 018 | offline fake-runner tests per ladder verdict; identical test-tree digest across sides; overlap reads `INCONCLUSIVE`; each structure control killed on the fixture | V1-1679, V1-1687 | PASS offline: `TestCCEV0028LadderOfflineFakeRunner` (every rung, the oracle cap, application and test revisions, identical digests), `TestCCEV0009VerdictOrder`, `TestCCEV0017TestTreeHeldAtHead`, `TestCCEV0017TestTreeRefusals`, `TestCCEV0018OmittedEventControls`; the earlier `project-location-unknown` was a missing headless shell 1248 in the shared browser cache, not a product fault; receipt error location PASS (lane 1022k, PWP-V3-008: `TestCCEV0009AssertionDetector`, `TestCCEV0026FailureKind`, the offline cleanup and page-object cases of `TestCCEV0028LadderOfflineFakeRunner`); live (lane 1022k: Playwright 1.64.0, Node v24.11.1, headless shell 1248) `TestCCEV0009LiveDiscriminates`, `TestCCEV0009LivePreserves`, `TestCCEV0021LiveEnvironmentDiscriminates` PASS; the CI lane for live tests is not delivered (`.github` needs the owner's ci-control-plane) |
| CCE-V0-011, 012 (amended) | synthetic window with each entry type: registration refusal, a mutant caught by one eligible scenario beside a broken one counts once as caught, an uncaught target counts once as a miss, bounds and Holm correction against a reference computation | V1-1682 | PASS (synthetic window): `TestCCEV0011ShadowRegistrationRefuses`, `TestCCEV0011ShadowMissesAndEscapes`, `TestCCEV0012ShadowBoundsAndHolm` (0.95^59 about 0.0485, Clopper-Pearson references, 72 versus 71 caught at m = 2, Holm step-down), `TestCCEV0011ShadowUsefulnessMinutesCost`, `TestCCEV0015SubcommandsReachable` (shadow); adopter window `NOT_RUN` |
| CCE-V0-016 | fixture per `oracle_origin` value; non-`ADMITTED_AT_BASE` never counts as a catch | V1-1680 | PASS: `TestCCEV0016OracleOrigin` (one fixture per value, field and encoded report), `TestCCEV0028LadderOfflineFakeRunner` (cap and non-scoring `NEW_IN_DIFF`), `TestCCEV0011ShadowMissesAndEscapes` (non-admitted catches never count) |
| CCE-V0-019, 020 | one fixture per `UNKNOWN` reason; `NONE` only under exemptions or no-screen placement; closed-enum test | V1-1678 | PASS: `TestCCEV0019ReachedScreenWithoutOutcomeUnknown` (F1), `TestCCEV0019MapUnknownOnReachedScreenUnknown` (F2), `TestCCEV0019ImpactTruncationUnknown` (F3, screens and edge-only omissions), `TestCCEV0019RemovedAtHeadUnknown` (F4, flow and outcome), `TestCCEV0019FanOutOverLimitUnknown` (F7), `TestCCEV0020ClosedUnknownReasonSet`; every fixture checks its reasons against the closed set. `blocks-not-declared` stays a catalog unknown and is never an obligations span; `NONE` arises only from exemptions (a non-UI path impact merely ignores stays `unplaced-path`) |
| CCE-V0-086 | relative-root catalog and obligations | V1-1711 | PASS: `TestCCEV0086RelativeRootResolves` |
| CCE-V0-087 | a control whose hook receipt records a located teardown failure reads `teardown-failed` in its reasons and the verdict note; other infrastructure stays `control-not-killed` | V1-2427 | PASS: `TestCCEV0087ControlTeardownAttribution` (BBF report fixture through `controlResults` and the offline ladder; failed with the reason carry or the verdict rung removed) |
| CCE-V0-088 | harvest binds through a non-default manifest path | V1-2428 | PASS (library and CLI): `TestCCEV0088HarvestConfiguredManifest` (map at `config/club.appmap.json` binds `book-tee-time`; the default name is absent and reads `app-map-unavailable`; failed with the hard-coded name restored); `TestCCEV0088HarvestManifestFlag` (`e2e harvest --manifest config/club.appmap.json` binds proposals, the default reads `app-map-unavailable`; failed before the flag was passed) |
| CCE-V0-100 | each recorded ladder rung replays offline; missing, non-canonical and stale recordings refuse | V1-1687 | PASS offline (darwin/arm64 and linux/WSL2): `TestCCEV0100ReplayRecordedReceipts` (five rungs from ten synthetic recordings through `DecodeAttemptReceipt`, `qualify` and the verdict code), `TestCCEV0100ReplayRefusals` (`recording-missing`, `recording-invalid` for a trailing byte and an unknown schema, `recording-invalid` for a relative tree, `recording-stale` for other scenario text, `repeat-count-mismatch` for a two-repeat replay, and a slash-path recording relocated under the host's staged tree with the recorded document's digest) |
| CCE-V0-101 | live run writes live recordings | V1-1687 | PARTIAL: `TestCCEV0101LiveRecord` written; SKIP here and NOT_RUN live (no qualified Playwright 1.64.0 tuple on this host; the Windows box's WSL has Node v22 and no `@playwright/test`), so every recording is `synthetic` |
| CCE-V0-102 | a required live run fails without its toolchain | V1-1687 | PASS: `TestCCEV0102LiveRequiredFails` (unset and `1` skip, `required` fails); `make cce-live` is NOT_RUN. CI job: NEEDS-OWNER (`.github`) |
| CCE-V0-103 | bound ledger consulted without changing reach; each binding mismatch refuses | V1-1685 | PASS: `TestCCEV0103LedgerBoundToImpact` (library and compile), `TestCCEV0103LedgerFlag` (CLI `obligations` and `compile`), `TestCCEV0034HostedTestFlag` (hosted `--ledger` refusal) |
| CCE-V0-104 | path synthesis over proven ledger edges only; every hop abstention named | V1-1684 | PASS: `TestCCEV0104ProvenPathSynthesis` (a tee-sheet flow compiles from `/#!/home` through `goToTeeSheets`; without a ledger or a set precondition it stays `INCOMPLETE`; the existing scenario is byte-identical; evidence, prefix, entry and edge abstentions) |
| CCE-V0-105 | fixture outputs checked and handed on; malformed outputs refuse | V1-1684 | PASS offline (darwin/arm64 and linux/WSL2): `TestCCEV0105FixtureOutputs` |
| CCE-V0-106 | entry URL reads a fixture output; unbound, unadmitted and undeclared bindings named | V1-1684 | PASS offline: `TestCCEV0106OutputDataFlow` |
| CCE-V0-107 | session storage state and skip; conflict; env declared only when set; malformed sessions refuse | V1-1684 | PASS offline: `TestCCEV0107Session`; live storage-state run NOT_RUN |
| CCE-V0-108 | `base.extend` teardown after `use`, in `finally` past an output check; shared `po` page objects; cleanup failure attributed; output setup not a control | V1-1684 | PASS offline: `TestCCEV0108Teardown`, `TestCCEV0018OmittedEventControls`; live teardown NOT_RUN |
| CCE-V0-109 | role and label locators and their wrong-locator controls; locator change reads `CHANGED_IN_DIFF`; unadmitted and malformed locators | V1-1684 | PASS offline: `TestCCEV0109TypedLocators` |
| CCE-V0-110 | accessible-name and count matchers; count control; vacuous and non-integer counts | V1-1684 | PASS offline: `TestCCEV0110TypedAssertions` |
| CCE-V0-111 | declared routes bind parameterised entries; no synthesis without a proven edge; each mismatch named | V1-1684 | PASS offline: `TestCCEV0111ParameterisedRoutes` |
| CCE-V0-112 | named `test.step` per action, trace on failure, `steps` in the entry | V1-1684 | PASS offline: `TestCCEV0112StepsAndTrace` |
| CCE-V0-113 | golden compile outputs pinned to `CompilerVersion` | V1-1684 | PASS offline: `TestCCEV0113GoldenCompile` (four goldens at `corvint-test-e2e-compile/1`); recordings regenerated at `/1` (`TestCCEV0100ReplayRecordedReceipts`) |
| CCE-V0-114 | base-declared API contract widens reach only | V1-1685 | PASS offline: `TestCCEV0114APIContractWidensReach` |
| CCE-V0-115 | no reach source, alone or combined, turns `UNKNOWN` into `NONE` | V1-1685 | PASS offline: `TestCCEV0115NoReachSourceTurnsUnknownToNone`, `TestCCEV0046CoverageOnlyWidensReach`, `TestCCEV0054ModuleGraphWidensReach` |
| CCE-V0-116 | each unqualified side and lost control attempt keeps its qualify reason, bounded secret-safe error message and location or `unlocated`; classification unchanged; live verdict failures print the losses | V1-2620 | PASS offline: `TestCCEV0116LostControlAttemptDetail` (unlocated run-level error on `control:omitted-event:setup-1` through the hook detail, a BBF report and the fake runner: verdict and reasons unchanged, no tree root, absolute path, secret or ANSI in the proof, failure output names each loss), `TestCCEV0116TeardownDetailCompatible`, `TestCCEV0116SideLoss`, `TestCCEV0116LossTextRedactsHostPaths` and `TestCCEV0116HookDetailWithinBBFBound` (each failed before the review fix) |
| CCE-V0-049 | hosted proof lists the adopter-config keys the prove config does not apply | V1-2409 | PASS: `TestCCEV0049HostedRunnerConfigDeviation` (an adopter config with `retries: 2`, `workers: 4`, `globalSetup`, `storageState`, a `baseURL` and projects; Playwright defaults when no config exists; a spread config read partially; failed with the proof wiring removed) |
| CCE-V0-051 | a CHANGED_IN_DIFF verdict lifts the cap only through an independent, digest-bound acceptance recorded outside the repository | V1-2413 | PASS: `TestCCEV0051OracleAcceptance` (independent acceptance lifts DISCRIMINATES unscored; head author by name or email, a stale digest and no document keep the cap; an admitted scenario ignores it; failed with the lift removed), `TestCCEV0051OracleAcceptanceRefusals`, `TestCCEV0051OracleAcceptanceFlag` |
| CCE-V0-052 | an unresolved outcome locator gets a proposed, never applied, data-testid rename with diff evidence | V1-2413 | PASS: `TestCCEV0052LocatorRepairProposal` (proposed `slot-state` with the line pair, verdict unchanged; value mismatch proposes nothing; ambiguous and no-rename; failed with the proposal removed) |
| CCE-V0-053 | the head pool's served application paths recorded per screen, pinned to head blobs, and consumed by `--coverage` | V1-2412 | PASS: `TestCCEV0053RecordingHandler` (200/304 recorded, directory as `index.html`, 404 not), `TestCCEV0053CoverageRecorderFeedsReach` (pins equal head blobs; test-tree and `node_modules` dropped; unrequested, hosted and unqualified pools unrecorded; the produced document widens reach through `runtime-coverage`; a stale pin refuses; fails with the head recorder removed or `checkPins` disabled), `TestCCEV0053RecordCoverageFlag`, live `TestCCEV0053LiveRecordsCoverage` (Playwright 1.64.0, Node 22.23.2: `index.html`, `app/routes.js`, `app/components/teesheet.html` pinned on `screen:admin:app.clubs.teesheets`) |
| CCE-V0-054 | the head tree's TJAA module graph widens reach from a changed module to its importers' screens and never removes or demotes an unknown span | V1-1685 (module graph; API joins and ANL edge-to-screen not delivered) | PASS: `TestCCEV0054ModuleGraphWidensReach` (a helper imported by the router anchor beside a comment the reference hop cannot prove reaches `obligation:book-tee-time/booked` with basis `tjaa-module-graph:app/routes.js`; every baseline unknown span stands with its reason; an unimported module reaches nothing; fails with the hits dropped), `TestCCEV0054ModuleGraphReadsHeadTree` (a worktree-only importer reaches nothing; fails when the graph is built from the worktree), `TestCCEV0054ModuleGraphFlag` |
| CCE-V0-050 | harvested proposals bound to a map flow or screen, or carrying a closed unbound reason | V1-2411 (partial: no site-knowledge IR on this base) | PASS: `TestCCEV0050HarvestBinding` (flow-outcome and goto bindings on the AMAP fixture, `no-flow-or-screen` and `goto-unresolved`, every proposal `app-map-unavailable` without a map; failed with the binding removed) |
| CCE-V0-048 | a located worker teardown error reads `teardown-failed` at its tree location; unlocated, outside-tree and `node_modules` errors stay infrastructure | V1-2410 | PASS: `TestCCEV0048TeardownAttribution`, the four teardown cases of `TestCCEV0028LadderOfflineFakeRunner` (both failed with the attribution removed), `TestPWPV3009RunLevelErrorLocation`; Playwright 1.64.0 observed live to report a worker fixture teardown as a located global error and test-scoped teardown on the test result (`docs/build-log/2026-10-22-cce2-1022u-cce-followups.md`) |
| CCE-V0-026..030 | failure kind; oracle cap and scoring; offline seams; omitted-event reading; test-tree resolution | V1-1679, V1-1680, V1-1687 | PASS: `TestCCEV0026FailureKind`, `TestCCEV0028LadderOfflineFakeRunner`, `TestCCEV0018OmittedEventControls`, `TestCCEV0017TestTreeRefusals` |
| CCE-V0-034..038 | hosted test held at head through the ladder; receipt-located assertions; required behaviour-removal control kills F1; no-op `PRESERVES` not useful; actualbudget rerun | V1-1733, V1-1713 | 034..037 PASS (lane 1022k): `TestCCEV0034HostedScenario`, `TestCCEV0034HostedTestFlag`, `TestCCEV0036HostedControls`, `TestCCEV0035HostedLadderOffline` (F1 reads `CONTROL_SURVIVED`, a base setup failure reads `INCONCLUSIVE`), `TestCCEV0037NoOpPreservesNotUseful`; live (Playwright 1.64.0, Node v24.11.1) `TestCCEV0034LiveHosted` (the booked test reads `DISCRIMINATES` with its base failure located at the assertion and its control killed; the F1 static overlay reads `CONTROL_SURVIVED`). 038 NOT_RUN (external host; command in `docs/build-log/2026-10-22-cce-1022k.md`) |
| CCE-V0-039, 040 | a timed-out omitted-event control is killed; truncation keeps a prefix | V1-1733, V1-2378 | PASS: 039 by the live `TestCCEV0009LivePreserves` and `TestCCEV0009LiveDiscriminates` (each read `INCONCLUSIVE control-not-killed:control:omitted-event:setup-1` before the fix); 040 by `TestCCEV0040CapListPrefix`, `TestCCEV0014DocumentByteCap` |
| CCE-V0-045..047 | coverage bound to head and the head map; a covered changed path reaches its screen's outcomes; coverage never removes an unknown span or turns UNKNOWN into NONE | V1-1685 (partial) | PASS: `TestCCEV0046CoverageOnlyWidensReach` (failed when coverage-placed paths dropped their unknown spans, and failed when coverage hits were dropped), `TestCCEV0045CoverageBoundToHead` (failed without the revision binding), `TestCCEV0045CoverageFlag`, `TestCCEV0034HostedTestFlag` (`--coverage` with `--hosted-test` refused) |
| CCE-V0-041..044 | harvest determinism and literal-only proposals; withheld reasons and secret screen; an unaccepted proposal never compiles; acceptance added in the diff is flagged | V1-1683 | PASS: `TestCCEV0041HarvestDeterministicLiteralOnly` (failed with the secret screen removed), `TestCCEV0043UnacceptedProposalNeverCompiles` (failed when every proposal was admitted, and failed with `acceptances_in_diff` unset), `TestCCEV0015SubcommandsReachable` (harvest reachable, `--out` inside the repository refused), `TestCCEV0013NoRepositoryWrites` |
| CCE-V0-031..033 | window format and refusals; shadow readings; limitations | V1-1682 | PASS: `TestCCEV0011ShadowRegistrationRefuses`, `TestCCEV0011ShadowMissesAndEscapes`, `TestCCEV0012ShadowBoundsAndHolm`, `TestCCEV0011ShadowUsefulnessMinutesCost` |
| CCE-V0-022, 023 | environment allowlist test; non-loopback request aborted and recorded; report digest changes with each bound input; one test per exit code, including a `prove` report with passing verdicts over `UNKNOWN` obligations exiting 3 | V1-1681 | NOT_STARTED |
| CCE-V0-024, 025 | golden bytes tied to the compiler version; `outcome-vacuous`; per-block `secret-suspected`; page text only in `untrusted_text` | V1-1679, V1-1681 | PARTIAL: `outcome-vacuous` by `TestCCEV0024OutcomeVacuous`; golden-version pinning, per-block `secret-suspected` and 025 NOT_STARTED |

Before more V0 build, spike V1-1677 replays 10 historical bug-fix commits of a real Playwright
repository through the slice and runs two oracle-review falsifiers (a static overlay that satisfies
a text matcher without the behaviour, and a base-only no-op seed), recording usefulness, time and
the continue, pivot or kill decision. V1-1687 adds an offline fake runner and a CI lane for the
live tests, which no CI lane runs today. Spike V1-1677 ran on 2026-10-18 and read 0 of 10
(Discrimination prover pivot below). The offline fake runner is delivered (CCE-V0-028); the CI lane
is not, because it needs a `.github` change under the owner's ci-control-plane.

The first slice (V1-1665) covers CCE-V0-001, 002, 004, 005, 006, 007, 008, 009 and 015 for one
obligation on the committed fixture. CCE-V0-003 there may use a single hand-written declarations
file. Everything else belongs to the umbrella (V1-1666).

## Rollout, rollback and compatibility

- Ship only as the `e2e` group of the experimental `corvint-test` companion; nothing changes in core `corvint` or for
  existing `affected`, `flows` or `test` users.
- Rollback: remove `cmd/corvint-test` (or only its `e2e` group once other groups exist), the CCE package and `.corvint/e2e-blocks.json` handling. No
  stored state migrates, because the catalog is derived and compiled scenarios are never committed.
- Drift: the catalog digest changes with any admitted block; a proven scenario whose catalog digest
  or anchors changed reads `UNVERIFIED_AT_HEAD` as RVN-V0 defines.

## Beyond V0

Recorded so V0 choices do not block them; each needs its own requirements before implementation.

- V1 authoring (V1-1683; the harvester, proposals document, acceptance ledger and in-diff flag are
  CCE-V0-041..044, the rest stays here): a deterministic harvester proposes blocks from existing specs and page
  objects with `harvested:<blob>:<span>` provenance; an `e2e-block-proposals/0` document written
  outside the repository; a human acceptance ledger `.corvint/e2e-acceptances.json`, which also
  accepts `CHANGED_IN_DIFF` and `NEW_IN_DIFF` oracles and flags an acceptance added in the diff
  under test; a bounded locator-repair loop (at most two attempts per obligation, never on a
  held-out window) that only proposes. A value observed at head is never an oracle.
- V1 vocabulary (V1-1684): fixture outputs and data flow, session blocks (environment-variable
  names only), teardown through test fixtures, parameterised routes, path synthesis over admitted
  edges, typed locators and assertions each with a control that kills it, and `test.step` with
  per-step provenance and a trace on failure.
- V1 reach (V1-1685; runtime coverage is CCE-V0-045..047 and 053, the TJAA module graph CCE-V0-054,
  the rest stays here): runtime coverage that may only widen reach and never produce `NONE`, API contract joins,
  and the ANL-V0 ledger in place of the first slice's nil.
- V1/V2 (V1-1686): per-hunk `CHECKED`, `PRESERVED_ONLY`, `UNCHECKED` or `UNKNOWN` roll-up, ARIA
  locator proposals, flake quarantine with retained evidence, read-only MCP tools over the
  documents, and snapshotting a proven scenario into the hosted suite. Retirement (V2) also needs a
  post-retirement canary and an un-retire trigger.

## Promotion and kill criteria

- Promote past experimental only on an adopter shadow window that meets the frozen gate.
- Kill or redesign when, on the first adopter shadow window, fewer than half of `REACHED`
  obligations compile without `INCOMPLETE`, or total seeded-regression parity is below 80%.
- Kill or pivot to a discrimination prover (CCE-V0-009 and 016..018 applied to existing hosted
  tests, without compilation) when the usefulness rate is below 40% on spike V1-1677 or the first
  adopter window, or when block authoring takes more minutes than the hosted runs it saves. Spike
  V1-1677 read 0 of 10; the owner pivoted on 2026-10-18 (V1-1733, CCE-V0-034..038).

### Discrimination prover pivot (V1-1733)

Authority: the owner's decision of 2026-10-18, after spike V1-1677 found 0 of 10 changes useful
(`docs/build-log/2026-10-18-cce-spike-v1-1677.md`): "Kill V0 compilation and pivot to the
discrimination prover over hosted tests." The 2026-10-22 acceptance moved the CCE tickets into 1.0, and
the owner answered question 9 YES on 2026-10-22: compilation resumes as a 1.0 route beside this
prover.

Intent: take an existing hosted Playwright test as given and prove whether it discriminates the
change, reusing the delivered prove machinery (the CCE-V0-009 ladder, the head test tree of
CCE-V0-017, the CCE-V0-018 controls, the CCE-V0-021 environment block, PWP receipts and the
CCE-V0-028 seams). Nothing is compiled.

The requirements are CCE-V0-034..038 in Requirements.

Non-goals: no compilation, no new oracle form, no deletion or disabling of hosted tests, and no
selection narrowing. Rollback: remove the hosted-test input from `prove`; compiled-scenario proofs
and the report schemas are unchanged.

## Owner questions

Answered 2026-10-24 (owner delegation): questions 1 to 8 are answered in
[`2026-10-24-owner-queue-decisions.md`](../build-log/2026-10-24-owner-queue-decisions.md) § V1-1666;
question 5 is deferred until the Playwright V0 route is delivered. The questions stay below for the record.

1. Is `.corvint/e2e-blocks.json` the right home for `input` and `fixture` declarations, or should
   they extend AFU-V1 flow intents?
2. Should `PRESERVES` (passes at base and head) count toward shadow parity for refactors, or only
   `DISCRIMINATES`?
   Partly settled by CCE-V0-037 (2026-10-22): `PRESERVES` never counts toward usefulness, and a
   `PRESERVES` scenario's killed seeded mutants still count toward parity; whether it should count
   toward parity for refactors beyond that stays open.
3. Should a team be able to snapshot a `DISCRIMINATES` scenario into its hosted suite, and if so is
   it then a hosted test with normal review?
4. Are 30 changes, 100% per-candidate parity, 95% total parity, zero escapes and the candidate
   minimums (5 changes run, 1 seeded regression caught) the right frozen gate values?
5. Should the first runner after Playwright be Nightwatch, given the existing Nightwatch work?
6. Is the reframing right: V0 sells a per-change proof to the agent and reviewer, and retirement
   is the V2 payoff?
7. Should a `CHANGED_IN_DIFF` or `NEW_IN_DIFF` oracle ever count before the V1 acceptance ledger
   exists, for example through a reviewer approving the PR?
8. Is a 5% Holm-corrected upper bound per candidate (at least 59 caught seeded regressions with zero
   misses) the right statistical gate, or is a looser bound acceptable for a first retirement?
9. Does the 2026-10-22 acceptance ("I want the CCE stuff in 1.0") resume compilation as a 1.0
   route, or does the 2026-10-18 kill stand with only the discrimination prover (V1-1733) in 1.0?
   Answered YES (owner, 2026-10-22; `docs/build-log/2026-10-22-cce-1022k.md`): the acceptance
   resumes the compilation work the 2026-10-18 kill stopped. The same decision accepts the V1-1667
   companion fold, which must still meet its own acceptance criteria (a fold spec naming every old
   and new invocation, unchanged profile IDs, byte-identical goldens before and after, a shim or
   structured refusal at each old name, and rollback).
