# Test Consolidation Planner V0 — the minimum set of focused tests for a set of variations

Owner: Russell Lewis
Date: 2026-10-08
Requirement prefix: `TCN-V0`
Intent status: proposed
Delivery status: not-started
Authoritative inputs: owner request [issue 681](https://github.com/beamfall/corvint/issues/681)
(native ticket V1-1023), `AGENTS.md` invariants 1, 2, 4, 7 and 8,
`docs/SPEC-DRIVEN-DEVELOPMENT.md`, `docs/specs/application-map-v0.md` (AMAP-V0 screens, element
IDs and freshness), `docs/specs/application-map-scenario-planner-v0.md` (AMSP-V0 sessions and
setup scenarios), `docs/specs/application-flow-understanding-v1.md` (AFU-V1 flow, variation and
declared test-link vocabulary) and `docs/specs/live-proof-carrying-verification-v0.md`
(LPCV-V0-047..053, the `corvint-test-validity/0` document read as test witnesses).

## Agent digest
- Claim: A read-only planner could group variations sharing fixture state, screen and user into the fewest focused tests, as a fail-closed checkable table with reasons.
- Status: proposed/not-started (V1-1023; GitHub #681). Acceptance is human-owned: nothing here is accepted until the owner records a decision.
- Exists: nothing. AMAP-V0 knows screens and routes, AMSP-V0 knows sessions and setup scenarios, AFU-V1 knows variations and declared test keys, and `corvint test-validity` projects per-test execution; no surface turns a set of variations into a minimal test plan.
- Blocked on: owner acceptance of TCN-V0-001..012 and the owner questions; implementation; all acceptance evidence.
- Read next: Requirements; Grouping algorithm; Table and check; Owner questions.

## User and measurable job

An agent asked to cover a matrix of flow variations tends to write one end-to-end test per matrix
row. Each such test repeats data setup, sign-in and navigation, so authoring time and lane time
grow linearly with the number of rows. The owner reports, from the adopter workspace behind issues
657 and 660 (figures as stated in issue 681, not reproduced here): 133 flows, 3,614 core matrix
rows, and 6,355 matrix-row proposals of `NEW TEST REQUIRED`. The application map (AMAP-V0) and the
scenario planner (AMSP-V0) already know the shared setup, screen and route of each step, so the
grouping can be computed instead of judged per agent.

The job: given a set of variations with their anchors, and the existing test witnesses, produce
deterministically

1. the groups of variations that share a fixture state, a screen and a user, each to be written as
   the ordered steps of one test, in an order where no step depends on an earlier step passing;
2. the variations that need their own test, each with a closed reason code;
3. the variations an existing passing test already witnesses (reuse, no new test);
4. the variations that duplicate another (same action and assertion in the same context);
5. the variations the planner cannot place because an anchor is missing or stale (abstention);

and print it as a compact table that can be pasted into a plan and checked fail-closed later.

Measured outcome (proposed promotion metric): on a labelled variation set, the number of new tests
the plan proposes versus the one-test-per-row baseline, with every input variation accounted for
exactly once.

## Verified current state

At `2a93b5a1` (origin/main):

- AMAP-V0 (`internal/appmap`) compiles per-app maps whose screens have IDs `screen:<app>:<state>`,
  route templates, steps `step:<flow_id>/<step_id>` and Git-anchored freshness (FRESH, STALE,
  UNKNOWN). It serves single-flow projections.
- AMSP-V0 (`internal/appmap/plan.go`) composes one plan for a multi-step request: one session per
  app, one setup scenario per session, `stay`/`goto` navigation. It does not take a variation
  matrix and never groups variations across tests.
- AFU-V1 flow intents carry `variations` (`variation_id`, `preconditions`, `steps`, `outcomes`,
  `projects`) and declared, reviewed or inferred `test` links with test keys; `flows coverage`
  (AFU-V1-049..053) reports per-row PROVEN, MISSING_TEST and similar statuses. Nothing proposes how
  missing rows should be grouped into tests.
- `corvint test-validity` (LPCV-V0-051/053) emits `corvint-test-validity/0`: per test `id`,
  `name`, optional `project` and a five-axis projection (association, hygiene, freshness,
  execution, strength). It does not carry AFU-V1 test keys.
- No `test-plan` command, no `consolidate_tests` MCP tool and no `TCN` requirement exist.

## Definitions

- **Variation**: one input row, identified by a caller-stable `variation_id` (the AFU-V1 unit when
  the row comes from a flow intent).
- **Context**: the tuple `(spec, app, setup, screen, user, org)` of a variation. `spec` is the
  repository-relative test file the variation's test belongs in (for example AMSP-V0
  `draft.proposed_path` or the AMAP-V0 closest asserting spec); `setup` is the fixture state
  identifier (for example the AMSP-V0 session setup scenario path); `screen` is the AMAP-V0 screen
  element ID where the variation's own action starts; `user` is the acting role (the AFU-V1 actor);
  `org` is the tenant or organisation, empty when the application has one.
- **Fact**: one `key=value` string describing fixture state. `requires` are facts the setup must
  provide for the variation; `changes` are facts the variation leaves different from the setup.
- **Step**: one variation placed inside a test. A step starts on its context screen; returning to
  that screen after an earlier step is navigation, not a dependency.
- **Depends on**: step B depends on step A when A changes the key of a fact B requires. A plan
  order is **independent** when no step depends on an earlier step.
- **Witness**: an existing test, identified in a `corvint-test-validity/0` document, that the
  variation declares as its test and whose projection reads associated, eligible, current and
  passed.
- **Duplicate**: two variations with the same context, the same `requires`, the same ordered
  `action` and the same `assertion` set.

## Requirements

Every requirement below is proposed (V1-1023; GitHub #681); none is accepted.

- `TCN-V0-001`: The planner MUST be a read-only local projection. `corvint test-plan consolidate
  --input FILE [--tests FILE]... [--map FILE]... [--revision REV] [--max-steps N]
  [--format table|json]` MUST read only its named files and, with `--map`, Git at the evaluated
  revision; it MUST write only stdout (and a coded refusal on stderr), run no test or browser, open
  no network connection, persist no state and generate no test code. The default format is
  `table`. Exit 0 means a plan was produced, whether `COMPLETE` or `INCOMPLETE`; invalid arguments
  or input exit 2 with nothing on stdout. Plan `authority` MUST always be `candidate`.
  (proposed; V1-1023)
- `TCN-V0-002`: The input MUST be one closed `test-consolidation-input/0` JSON document of at most
  8 MiB with members `schema`, optional `revision` (the full object ID the anchors were read at) and
  `variations` (1..8192 entries). A variation is a closed object with `variation_id` and optional
  `flow_id`, and the anchors `spec`, `app`, `route`, `screen`, `setup`, `user`, `org`, `action`
  (1..64 ordered element IDs), `assertion` (1..64 element IDs, a set), `requires` and `changes`
  (0..64 facts each, sets), `destructive` (boolean) and `witnesses` (0..16 objects
  `{test_id, project?, basis}` with `basis` `declared` or `reviewed`). Identifiers MUST match
  `[A-Za-z0-9._:/@{}=+-]{1,256}`; `spec` MUST be a canonical repository-relative path with no
  empty, `.` or `..` segment and no whitespace or `|`. An unknown or duplicate member, trailing
  data, a repeated `variation_id`, an identifier outside the charset, a `basis` of `inferred`, or a
  bound overrun MUST refuse with `test-plan-invalid-input`. A member that is absent or `null` is a
  missing anchor (TCN-V0-003), not a refusal, except `variation_id`. Input order MUST NOT affect
  any output byte. (proposed; V1-1023)
- `TCN-V0-003`: A variation MUST be abstained, never grouped, isolated, reused or deduplicated,
  when any of `spec`, `app`, `screen`, `setup`, `user`, `action` or `assertion` is missing
  (reason `missing-anchor`, naming each missing member). With `--map` (1..8 AMAP-V0 maps, decoded
  and refused as `appmap-invalid-map`), the `screen` MUST be a screen of the map whose app equals
  the variation's `app` and, when `route` is present, its template MUST equal `route`; otherwise
  the variation is abstained `unresolved-screen` or `route-mismatch`. Each such screen's lineage
  freshness MUST be evaluated by the AMAP-V0 rule at `--revision` (default `HEAD`): STALE abstains
  `stale-anchor`, UNKNOWN abstains `anchor-unknown`. Without `--map` the plan MUST report
  `anchor_validation: NOT_RUN` and every variation's anchors as `caller` basis. A plan with any
  abstained variation reads `INCOMPLETE`; otherwise `COMPLETE`. (proposed; V1-1023)
- `TCN-V0-004`: Reuse MUST be decided only from `--tests` files (0..8), each one
  `corvint-test-validity/0` document read through the LPCV-V0-051 safe reader and bounds and
  re-projected, never trusting carried projections. A witness `{test_id, project}` matches a
  document test whose `id` equals `test_id` and, when `project` is given, whose project name equals
  it. A variation is `REUSED` only when at least one declared witness matches, every matching test
  across all documents reads association `ASSOCIATED`, hygiene `ELIGIBLE`, freshness `CURRENT` and
  execution `PASSED`, and the document is an `e2e` JavaScript document (a Go `tier:"preview"`
  document never witnesses). Otherwise each witness is reported with one of
  `witness-not-found`, `witness-not-passed`, `witness-stale`, `witness-not-associated`,
  `witness-ineligible`, `witness-conflicting` (matching tests disagree) or `witness-preview`, and
  the variation continues to planning. The strength axis MUST be carried unchanged onto the reuse
  row; reuse states that a passing witness exists, not that the test is adequate (LPCV-V0-048).
  (proposed; V1-1023)
- `TCN-V0-005`: Among non-abstained variations, duplicates MUST be classed by equal context, equal
  `requires`, equal ordered `action` and equal `assertion` set. Each class keeps one
  representative: the smallest `variation_id` (byte order) among members that are `REUSED`, else
  the smallest overall. Every other member reads `DUPLICATE` of that representative and needs no
  test of its own; a duplicate of a reused representative is reported as such. Variations whose
  action and assertion match but whose context or `requires` differ MUST NOT be classed as
  duplicates. (proposed; V1-1023)
- `TCN-V0-006`: The remaining variations (not abstained, reused or duplicate) MUST be partitioned
  by context. A variation MUST be placed in its own test with a closed reason, checked in this
  order: `destructive-change` when `destructive` is true; `conflicting-state` per TCN-V0-007;
  otherwise it joins its context's group. A test that ends with exactly one step reads
  `different-user-or-org` when another variation placed in a new test shares its `spec`, `app`, `setup` and
  `screen` but differs in `user` or `org`, and `unique-context` otherwise. A test with two or more
  steps carries no isolation reason (`-`). (proposed; V1-1023)
- `TCN-V0-007`: Within one context the order MUST be independent. Variations are visited in
  `variation_id` byte order; one whose `requires` assigns a different value to a key an earlier
  kept variation requires is isolated `conflicting-state`. Over the kept variations, B MUST precede
  A whenever A changes a key B requires (a variation changing a key it itself requires adds no
  edge). While this precedence graph has a cycle, the greatest `variation_id` in any strongly
  connected component of more than one node is isolated `conflicting-state`. The order is then the
  topological order that always takes the smallest available `variation_id`. More than 1,048,576
  precedence edges in one input MUST refuse with `test-plan-bound-exceeded`. (proposed; V1-1023)
- `TCN-V0-008`: A group's ordered steps MUST be cut into consecutive tests of at most `--max-steps`
  steps (2..32, default 8); every cut test keeps the independent order. Tests MUST be numbered
  `T001`, `T002`, ... after sorting by `spec`, then by the smallest `variation_id` they contain.
  The same input bytes, test-validity bytes, map bytes, revision and options MUST give the same
  output bytes on every run and host. (proposed; V1-1023)
- `TCN-V0-009`: `--format json` MUST print one closed `test-consolidation-plan/0` document: `schema`,
  `input_digest` (`sha256:` of the canonical input: variations sorted by ID, set members sorted,
  compact JSON), `tests_digest` (`sha256:` over the sorted digests of the `--tests` files, or
  `none`), `maps[]{app, map_digest}`, `evaluated_revision`, `anchor_validation`
  (`VALIDATED`|`NOT_RUN`), `authority` (`candidate`), `status`, `max_steps`, `counts{variations,
  new_tests, grouped, isolated, reused, duplicates, abstained, baseline_one_per_row}`, `tests[]{test,
  spec, context, steps[]{order, variation_id}, reason}`, `reused[]{variation_id, test_id, project,
  strength}`, `duplicates[]{variation_id, duplicate_of}`, `abstained[]{variation_id, reason,
  missing[]}`, `witness_rejections[]{variation_id, test_id, reason}` and `table_digest`. Every input
  variation MUST appear exactly once among test steps, `reused`, `duplicates` and `abstained`, and
  `baseline_one_per_row` equals the number of input variations. Output over 8 MiB MUST refuse with
  `test-plan-bound-exceeded`, never truncate. (proposed; V1-1023)
- `TCN-V0-010`: `--format table` MUST print exactly: one header line `<!-- corvint
  test-consolidation-table/0 input=<input_digest> tests=<tests_digest>
  anchors=<VALIDATED|NOT_RUN> max-steps=<N> status=<COMPLETE|INCOMPLETE> -->`, the line
  `| test | spec | variation ids | isolated reason |`, the line `| --- | --- | --- | --- |`, then
  one row per new test in test order (`| T001 | <spec> | <ids in step order joined by ", "> |
  <reason or -> |`), then one row per reused variation (`| REUSE <test_id>[@<project>] | <spec> |
  <id> | reused |`), per duplicate (`| DUPLICATE <representative id> | <spec> | <id> | duplicate |`)
  and per abstained variation (`| ABSTAIN | <spec or -> | <id> | <reason> |`), each block sorted by
  variation ID. Lines end with LF; no field is escaped because TCN-V0-002 excludes `|`, whitespace
  and control characters from every printed value. `table_digest` is the SHA-256 of these bytes.
  (proposed; V1-1023)
- `TCN-V0-011`: `corvint test-plan check --plan FILE --input FILE [--tests FILE]... [--map FILE]...
  [--revision REV] [--max-steps N]` MUST locate exactly one header line of TCN-V0-010 in FILE
  (at most 8 MiB; a Markdown plan may surround the table), take the table as the header and the
  following lines up to the first line that does not start with `|`, recompute the plan from the
  other arguments, and compare bytes after removing trailing spaces and tabs from each line. It
  MUST exit 0 only when the bytes match and the recomputed status is `COMPLETE`. It MUST exit 1
  with `test-plan-mismatch` (naming the first differing line, or a missing or extra variation ID)
  when they differ, with `test-plan-incomplete` when they match but the plan has abstained
  variations, and with `test-plan-header-missing` when FILE has no header or more than one; it
  exits 2 for invalid arguments or input. Its `max-steps` and the header's MUST agree, and an
  edited, reordered, added or dropped row MUST fail. (proposed; V1-1023)
- `TCN-V0-012`: When the owner accepts an MCP surface, it MUST be the read-only, idempotent,
  non-destructive, closed-world tool `corvint.consolidate_tests` on `corvint-corpus-mcp`, listed
  only when that server is started with `--consolidation`; `corvint-mcp` and its frozen MCP
  2026-07-28 V0 tool set MUST NOT change. The tool takes the input document inline (at most
  1 MiB), optional `tests` and `maps` as root-confined repository-relative paths (at most 8 each),
  optional `max_steps`, `format` and `plan` (a table string to check, at most 1 MiB), decoded
  strictly; it returns the CLI's exact bytes in the untrusted-data envelope with the plan object
  as `structuredContent`, and refuses with `test-plan-bound-exceeded` rather than exceed the
  1 MiB message cap. (proposed; V1-1023)

## Grouping algorithm (informative restatement of TCN-V0-003..008)

```text
validate input (002) -> abstain missing/unresolved/stale anchors (003)
  -> reuse from test-validity witnesses (004)
  -> duplicate classes, representative kept (005)
  -> isolate destructive (006)
  -> per context: conflicting requires, then cycle breaking (007)
  -> independent topological order, smallest id first (007)
  -> cut at max-steps, number T001.. (008) -> plan (009) -> table (010)
```

Worked example. Variations `V1..V5` share `(tests/e2e/teesheet.spec.ts, admin, scenarios/club.ts,
screen:admin:club.teesheet, club-admin, club-a)`. `V2` changes `slot.42` which `V4` requires, so
`V4` precedes `V2`; `V5` is destructive; `V3` has a passing witness; `V6` is `V1`'s duplicate.

```text
<!-- corvint test-consolidation-table/0 input=sha256:… tests=sha256:… anchors=VALIDATED max-steps=8 status=COMPLETE -->
| test | spec | variation ids | isolated reason |
| --- | --- | --- | --- |
| T001 | tests/e2e/teesheet.spec.ts | V1, V4, V2 | - |
| T002 | tests/e2e/teesheet.spec.ts | V5 | destructive-change |
| REUSE teesheet-book@chromium | tests/e2e/teesheet.spec.ts | V3 | reused |
| DUPLICATE V1 | tests/e2e/teesheet.spec.ts | V6 | duplicate |
```

Six rows of the baseline become two new tests.

## Input derivation (informative)

V0 takes an explicit input document so that the grouping rules can be reviewed and tested apart
from derivation. The intended sources are: `variation_id`, `flow_id`, `user` (actor), `requires`
(variation preconditions as facts) and `witnesses` (declared or reviewed `test` links) from AFU-V1
flow intents; `screen`, `route` and `action` element IDs from AMAP-V0 map steps; `setup` and `spec`
from an AMSP-V0 plan's session setup and draft path. `changes` and `destructive` have no compiled
source today and are caller-declared. A later requirement may derive the whole document from
`--flows DIR --map FILE...` (owner question 1).

## Non-goals and simpler baseline

- No test generation, drafting, editing or execution; AMSP-V0 `--draft` remains the drafting
  surface. No write to the repository, the task store or `.corvint/`.
- No inference of side effects: `changes` and `destructive` are declared, never guessed.
- No ranking, learning, network access, service or persisted state.
- No change to AFU-V1 coverage statuses, AMSP-V0 plans or the test-validity document.
- No claim that a reused or grouped test is adequate; strength stays as measured (LPCV-V0-048).
- Simpler baseline: one test per matrix row (`baseline_one_per_row`), or an agent grouping rows by
  judgement. The plan reports its count beside the baseline so the saving is visible.

## Trust boundary, limits and failure modes

- Input documents, test-validity documents and maps are untrusted data. Identifiers are matched,
  never executed; printed values are restricted to a charset that needs no escaping.
- Failure modes and their outcomes:
  - a missing anchor: `ABSTAIN missing-anchor`, plan `INCOMPLETE`, check exits 1;
  - a screen not in the map, a route mismatch, a STALE or UNKNOWN anchor: `ABSTAIN` with the
    reason;
  - an undeclared side effect (a `changes` fact the caller omitted): the order may be wrong. The
    planner cannot detect this; the plan's `anchor_validation` and `authority: candidate` keep it a
    proposal, and the follow-up run of the written test through `corvint test-validity` is the
    check (fail-open by necessity, labelled);
  - a step that fails at run time stops the later steps of a Playwright test, so their variations
    are unobserved in that run, though never wrongly passed; the planner caps steps per test
    (`--max-steps`) to bound this;
  - a failing, stale, ineligible or conflicting witness: no reuse, reason reported;
  - a test-validity document of an unrecognised kind or over its bound: refused as by LPCV-V0-051;
  - two requires on one key with different values: the later variation isolated
    `conflicting-state`;
  - a dependency cycle: its greatest ID isolated `conflicting-state`, repeated until acyclic;
  - an input, edge or output bound overrun: refused, never truncated;
  - a pasted table edited, reordered, extended or shortened: check exits 1 `test-plan-mismatch`;
  - a pasted table with no or two headers: check exits 1 `test-plan-header-missing`.
- Limits: 8 MiB input, 8192 variations (the AFU-V1-052 row bound), 64 action, assertion, requires
  and changes entries each, 16 witnesses per variation, 8 test-validity documents of 4 MiB, 8 maps,
  1,048,576 precedence edges, 8 MiB output.

### Proposed owned error codes

| Code | Meaning |
| --- | --- |
| `test-plan-invalid-input` | The input is not a closed bounded `test-consolidation-input/0` document (TCN-V0-002). |
| `test-plan-invalid-arguments` | A flag is unknown, repeated where single, out of range or combined illegally. |
| `test-plan-bound-exceeded` | The edge or output bound was exceeded (TCN-V0-007, TCN-V0-009, TCN-V0-012). |
| `test-plan-mismatch` | A checked table differs from the recomputed plan (TCN-V0-011). |
| `test-plan-incomplete` | A checked table matches but the plan has abstained variations (TCN-V0-011). |
| `test-plan-header-missing` | A checked file has no table header or more than one (TCN-V0-011). |

AMAP-V0 `appmap-invalid-map` and LPCV-V0-051 `invalid-test-validity-receipt` are reused for their
own inputs.

## Deterministic acceptance and traceability

All evidence is planned; none exists. Each row is `NOT_RUN` until implemented.

| Requirement | Planned evidence |
| --- | --- |
| TCN-V0-001 | CLI test: read-only (repository status and `.corvint/` unchanged), no network, exit codes, `authority: candidate`. |
| TCN-V0-002 | Decoder table test over every refusal and bound; permutation test proving input order does not change output bytes. |
| TCN-V0-003 | Fixture over the committed AMAP-V0 map: missing anchor, unknown screen, route mismatch, STALE and UNKNOWN freshness each abstain; `NOT_RUN` without `--map`. |
| TCN-V0-004 | Synthetic `corvint-test-validity/0` documents for each witness rejection reason, a conflicting pair across two documents, a Go preview document, and a reuse that keeps `NOT_MEASURED` strength. |
| TCN-V0-005 | Duplicate classes with and without a reused member; same action and assertion under a different user or `requires` stay distinct. |
| TCN-V0-006 | Destructive, conflicting, different-user-or-org and unique-context reasons on one fixture. |
| TCN-V0-007 | Precedence ordering, contradictory requires, a three-node cycle, the edge bound refusal. |
| TCN-V0-008 | `--max-steps` cuts preserving order; numbering; byte-identical output on repeated runs. |
| TCN-V0-009 | Golden JSON; every variation accounted for exactly once; output bound refusal. |
| TCN-V0-010 | Golden table, including the worked example above. |
| TCN-V0-011 | Check passes on the exact table inside surrounding Markdown; fails on an edited, reordered, added and dropped row, on a missing and a doubled header, on a max-steps disagreement and on an `INCOMPLETE` plan. |
| TCN-V0-012 | Only if accepted: MCP strict decoding, listing gated by `--consolidation`, byte parity with the CLI, `corvint-mcp` tool list unchanged. |

Owner-run qualification (solo-closable): run the planner on the adopter's labelled variation set
(issue 681 cites 3,614 core rows over 133 flows) and record new tests versus
`baseline_one_per_row`, abstentions by reason, and whether a sample of grouped tests, written and
run, passes with `corvint test-validity` showing every step's assertion executed. Until that run it
stays `NOT_RUN`.

## Rollout, rollback and compatibility

The slice is additive. Rollback removes the `test-plan` command and its help text, the planner
package and its tests, the optional `--consolidation` flag and tool of `corvint-corpus-mcp`, and
this spec's index rows. No stored state or wire of another spec changes, so nothing migrates. A
pasted table becomes unverifiable after rollback, which is the intended fail-closed outcome.

## Promotion and kill criteria (proposed)

- Promote to `implemented` when every TCN-V0 row above has passing executable evidence and one
  independent review is retained.
- Promote to `validated` only after the owner-run qualification shows fewer new tests than the
  baseline with every variation accounted for, and the sampled grouped tests pass without a step
  depending on another.
- Kill or rework if, on the owner's set, grouping saves less than 20 % of tests against the
  baseline, or sampled grouped tests fail because of an order dependency the declared facts did
  not reveal.

## Owner questions

1. Should V0 derive the input itself from `--flows DIR --map FILE...` (and an AMSP-V0 plan),
   instead of taking a caller-assembled document? Derivation removes agent judgement from the
   input; it needs a rule for a variation's start screen and for `changes`.
2. Should the surface be the new root verb `corvint test-plan` named in issue 681, or a subcommand
   of the existing `corvint flows` family (for example `flows consolidate`), which adds no root
   verb?
3. Should a destructive variation be allowed as the last step of a group instead of always being
   isolated?
4. Is the conservative duplicate rule (same context and requires, not only same action and
   assertion) the intended one?
5. How should witnesses join tests: on exact `corvint-test-validity/0` test `id` (V0), or through
   AFU-V1 test keys once test-validity carries them? Should AFU-V1-051 `PROVEN` coverage rows also
   count as witnesses?
6. Is the default of 8 steps per test, and the range 2..32, right for focused tests?
7. Should the MCP tool (TCN-V0-012) be built in V0 at all, or wait for CLI qualification?
