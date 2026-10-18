# Change-Compiled E2E V0 — hosted blocks, generated scenarios, shadow-gated retirement

Owner: Russell Lewis
Date: 2026-10-17
Intent status: proposed
Delivery status: not-started
Authoritative inputs: owner direction of 2026-10-17 (this session; recorded in
`docs/build-log/2026-10-17-change-compiled-e2e-direction.md`): "Draft the spec as augment-first:
hosted blocks, generated scenarios, a shadow evaluation against the existing suite before any test
is retired"; `AGENTS.md`; `docs/SPEC-DRIVEN-DEVELOPMENT.md`; `docs/specs/application-map-v0.md`
(AMAP-V0); `docs/specs/application-map-scenario-planner-v0.md` (AMSP-V0);
`docs/specs/application-flow-understanding-v1.md` (AFU-V1 declared outcomes);
`docs/specs/application-navigation-ledger-v0.md` (ANL-V0 proven edges);
`docs/specs/run-verified-navigation-v0.md` (RVN-V0); `docs/specs/playwright-run-evidence-v0.md`
(PWR-V0); `docs/specs/new-e2e-test-acceptance-v0.md` (NEA-V0);
`docs/specs/browser-behavior-falsification-v0.md` (BBF-V0).

## Agent digest
- Claim: Reviewed E2E blocks compile into change-specific scenarios that must discriminate the change; hosted tests retire only after a shadow evaluation.
- Status: proposed; not-started. No requirement is implemented; every row of the traceability table reads `NOT_STARTED`.
- Exists: nothing of this capability. It composes the existing AMAP-V0 map and scaffold, AMSP-V0 planner, AFU-V1 declared outcomes, ANL-V0 proven edges, RVN-V0 run verification, PWR-V0 run comparison and NEA-V0/BBF-V0 negative controls.
- Blocked on: owner acceptance of this intent; first slice V1-1665 (umbrella V1-1666).
- Read next: Definitions; Requirements; Shadow evaluation and retirement; Owner questions.

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
- **Discrimination verdict**: the result of running a compiled scenario at head and base under
  negative controls: `DISCRIMINATES`, `PRESERVES`, `FAILS_AT_HEAD`, `UNSTABLE`, `CONTROL_SURVIVED` or
  `INCONCLUSIVE`.
- **Shadow window**: a frozen, ordered list of at least 30 merged changes, fixed before the
  evaluation runs, over which compiled scenarios and the hosted affected selection both run.

## Requirements

- `CCE-V0-001`: `corvint e2e catalog [--root PATH] --rev REV` MUST emit one deterministic
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
  - the top level holds only `schema`, `inputs` and `fixtures`;
  - an input is `input_id`, `screen_id`, `locator` (an AFU-V1-026 role and name, or a test ID) and
    `values`; each value is `value_id`, `value` (a string) and `class` (`valid` or `invalid`);
  - a fixture is `fixture_id`, `satisfies` (AFU-V1 precondition IDs), `setup` and `cleanup`; each of
    `setup` and `cleanup` names a `page_object` and a `method` that must be a callable `page-object`
    block.

  The compiler applies an input value only as a Playwright `fill` on the input's own locator. A
  missing file means no input or fixture blocks, reported as the unknown `blocks-not-declared`. An
  unknown member, a wrong type, a duplicate ID or an unresolved reference MUST refuse with
  `e2e-blocks-invalid` and admit nothing from the file.
- `CCE-V0-004`: `corvint e2e obligations [--root PATH] --base SHA --head SHA` MUST map the diff to
  obligations through the existing impact, affected-plan and map relations, listing each reached
  outcome with the changed span and the chain that reaches it. When any changed span's reach cannot
  be bounded, the report MUST read `UNKNOWN` overall and name the spans; it MUST NOT read `NONE`.
  A compilation over an `UNKNOWN` report MUST read `INCOMPLETE` overall with those spans, even when
  every `REACHED` obligation compiled or none was reached.
- `CCE-V0-005`: `corvint e2e compile` MUST emit, for each `REACHED` obligation, at most one compiled
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
- `CCE-V0-008`: Running and proving are separate, opt-in and executed by a companion
  (`corvint-e2e-prove`), never by a read command. The companion MUST run each compiled scenario
  through the existing Playwright provider path at head with repeats (default 5, retries 0, one
  worker) and at base with the same settings, against a disposable workspace per BBF-V0's mutation
  boundary, and MUST bind every run to a PWP receipt.
- `CCE-V0-009`: The discrimination verdict MUST be the first of these that holds:
  1. `INCONCLUSIVE` when any required run produced no qualified receipt.
  2. `UNSTABLE` when PWR-V0 classifies the head pool `INTERMITTENT`.
  3. `FAILS_AT_HEAD` when any head repeat fails.
  4. `CONTROL_SURVIVED` when the BBF-V0 `wrong-locator` or `wrong-expected-value` control, applied
     to the outcome assertion at head, reads `survived`.
  5. `INCONCLUSIVE` when either control reads anything other than `killed` under BBF-V0-005/006 with
     its criterion, test, product and environment identities matched (for example `invalid_control`,
     `infrastructure_failed` or `not_run`).
  6. `DISCRIMINATES` when every base repeat fails and PWR-V0 places every failure on the outcome
     assertion's own location.
  7. `PRESERVES` when every base repeat passes.
  8. `INCONCLUSIVE` otherwise: mixed base results, or a base failure anywhere but the outcome
     assertion.

  `PRESERVES` MUST be reported as a regression-preservation check only and MUST NOT be described as
  detecting the change.
- `CCE-V0-010`: A compiled scenario MUST NOT read above `candidate` unless its verdict is
  `DISCRIMINATES` or `PRESERVES`; a passing compiled scenario MUST NOT narrow, skip or replace any
  hosted test's selection in V0.
- `CCE-V0-011`: `corvint e2e shadow --window FILE` MUST read a frozen shadow window (an ordered list
  of merged-change base/head pairs, digest-pinned before the first run) and report, per change and
  in total: obligations by state, scenarios by verdict, hosted affected tests run, seeded
  regressions (BBF-V0 controls) caught by each side, real hosted failures the compiled side missed
  (escapes), and wall-clock runtime of each side. For a change whose obligations read `UNKNOWN`,
  the compiled side MUST be counted as missing every seeded regression and every real failure the
  hosted side caught in that change. Each counts as a miss in parity and as an escape.
- `CCE-V0-012`: A hosted test MAY be listed as a retirement candidate only when all of these hold
  over the shadow window:
  - every outcome it asserts is an admitted `outcome` block;
  - it ran in at least the frozen minimum number of window changes and caught at least the frozen
    minimum number of seeded regressions;
  - for every change in the window where it ran, the compiled scenarios caught every seeded
    regression it caught;
  - the compiled side had zero escapes against it;
  - the window size and parity thresholds equal the frozen gate values in Shadow evaluation and
    retirement.

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

## Shadow evaluation and retirement

The frozen V0 gate values, which only an owner decision may change:

- the window holds at least 30 merged changes, chosen and digest-pinned before any compiled run;
- seeded-regression parity: per retirement candidate, the compiled side catches 100% of the seeded
  regressions the hosted test caught in the window, and in total at least 95% of all seeded
  regressions the hosted affected selection caught;
- escapes: zero real hosted failures the compiled side missed for that candidate;
- candidate evidence: the candidate ran in at least 5 window changes and caught at least 1 seeded
  regression;
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

- Read commands (`catalog`, `obligations`, `compile`, `shadow` over existing receipts) are read-only
  and write only to stdout or an explicit `--out` path outside the repository root (CCE-V0-001;
  invariant 4). Only the opt-in companion executes browsers,
  and only in a marked disposable workspace (BBF-V0 boundary).
- Compiled scenarios are generated code run against the user's application. They contain only
  admitted block text, so they add no code path a hosted block does not already contain.
- Limits: at most 64 obligations per compile, 16 scenarios proven per run, 100 repeats per side,
  one worker; each limit refuses with a named code when exceeded rather than sampling.
- Fail-closed choices: unknown reach reads `UNKNOWN` (never `NONE`); a missing block reads
  `INCOMPLETE`; a missing receipt reads `INCONCLUSIVE`; none of these narrows hosted selection.
- Secrets: `input` values pass the existing secret screen; a value that matches is refused with
  `e2e-input-secret`, and PWP `/2` redaction applies to run evidence.

## Deterministic acceptance and traceability

| Requirement | Planned evidence | Status |
|---|---|---|
| CCE-V0-001..003 | catalog golden over the AMAP-V0 fixture plus a declared `e2e-blocks.json` | NOT_STARTED |
| CCE-V0-004 | obligations golden for a one-screen diff, an unknown-reach diff and a no-UI diff | NOT_STARTED |
| CCE-V0-005..007 | compile golden, byte-identical rerun, `INCOMPLETE` cases per missing block kind | NOT_STARTED |
| CCE-V0-008..010 | live local Playwright run on the fixture app: head pass, base fail, two BBF controls | NOT_STARTED |
| CCE-V0-011..013 | shadow report over a synthetic window; adopter window `NOT_RUN` | NOT_STARTED |
| CCE-V0-014 | byte-cap truncation tests per document | NOT_STARTED |

The first slice (V1-1665) covers CCE-V0-001, 002, 004, 005, 006, 007, 008 and 009 for one
obligation on the committed fixture. CCE-V0-003 there may use a single hand-written declarations
file. Everything else belongs to the umbrella (V1-1666).

## Rollout, rollback and compatibility

- Ship behind the experimental `corvint e2e` command group and a separate companion binary; nothing
  changes for existing `affected`, `flows` or `test` users.
- Rollback: remove the command group, the companion and `.corvint/e2e-blocks.json` handling. No
  stored state migrates, because the catalog is derived and compiled scenarios are never committed.
- Drift: the catalog digest changes with any admitted block; a proven scenario whose catalog digest
  or anchors changed reads `UNVERIFIED_AT_HEAD` as RVN-V0 defines.

## Promotion and kill criteria

- Promote past experimental only on an adopter shadow window that meets the frozen gate.
- Kill or redesign when, on the first adopter shadow window, fewer than half of `REACHED`
  obligations compile without `INCOMPLETE`, or total seeded-regression parity is below 80%.

## Owner questions

1. Is `.corvint/e2e-blocks.json` the right home for `input` and `fixture` declarations, or should
   they extend AFU-V1 flow intents?
2. Should `PRESERVES` (passes at base and head) count toward shadow parity for refactors, or only
   `DISCRIMINATES`?
3. Should a team be able to snapshot a `DISCRIMINATES` scenario into its hosted suite, and if so is
   it then a hosted test with normal review?
4. Are 30 changes, 100% per-candidate parity, 95% total parity, zero escapes and the candidate
   minimums (5 changes run, 1 seeded regression caught) the right frozen gate values?
5. Should the first runner after Playwright be Nightwatch, given the existing Nightwatch work?
