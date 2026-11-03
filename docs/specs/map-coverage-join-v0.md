# Map Coverage Join V0

Owner: Russell Lewis
Date: 2026-11-03
Intent status: accepted
Delivery status: partial
Authoritative inputs: owner request of 2026-11-03 (tie line-level test selection to the site map,
page objects and the generated documentation across unit, integration and E2E tests, and stop
re-testing repeated E2E steps); `live-proof-carrying-verification-v0.md` (LPCV-V0-950..963);
`application-map-v0.md`; `site-knowledge-howto-v0.md`; `change-compiled-e2e-v0.md`;
`run-verified-navigation-v0.md`; `documentation-corpus-v1.md`; `AGENTS.md`.

## Agent digest
- Claim: Functionality links to its code and tests, so a change selects its feature's tests where line coverage is silent and shared E2E steps run once.
- Status: accepted / partial
- Exists: slice 3a as an experimental library, `internal/funclink` (V1-4386): functional units with code and test sets built from AFU-V1 declared links, the AMAP-V0 map and impact join, SKH-V0 page objects, steps and setup records, and an injected observed source; `functionality-link` gap-filling, test-side surface selection, the shadow fallback-replacement record and the gap report. It is not yet called by `corvint affected` or the CI selector, reads no project-config ownership, and has no observed source until the segment join (slices 1 and 3) exists. The rest of this join does not exist. Its inputs exist separately: the Go-only per-test `linecov` index (LPCV-V0-950..963, `internal/linecov`), the AMAP-V0 screen graph and overlay seam, the SKH-V0 page objects, method IDs and deduplicated step records (`SKRecord`, SKH-V0-022), RVN-V0 receipt verification and CCE-V0 obligations.
- Blocked on: a non-Go collector (LPCV-V0-955) for the observed links and segment requirements. Declared and static functionality links (MCJ-V0-020..024) need none. Tickets V1-4383..V1-4391.
- Read next: Requirements; Functionality links; Shared-step selection; Failure modes; Delivery slices; Owner questions.

## User job and verified baseline

The owner wants functionality linked to code and to tests, so that changing a feature selects
the tests that feature needs even where line coverage cannot say. That one link answers two
questions that today come from separate tools: "which tests can this changed line affect?" and
"which tests prove this feature works?". Line coverage has gaps: new code has no record yet, most
runner families have no collector (LPCV-V0-955), and an E2E consumer of a built binary has no
static edge to the code it drives, so the plan falls back to running everything. Both should hold at every
test level, unit, integration and E2E, down to the page-object method and the step. An E2E suite
repeats the same steps (sign in, open a page, fill a form) across many tests. When a change reaches
only those repeated steps, running every test that repeats them proves nothing more than running
one of them.

Baseline at `origin/main` 2e6a8a316, read from the specs and code named here:

- LPCV-V0-950..963 define a per-test record of executed coverage blocks. `internal/linecov` builds
  it for Go only, and it names tests, not steps. LPCV-V0-954 and LPCV-V0-955 require E2E and
  multi-runner collectors that are not built.
- AMAP-V0 joins routes, screens and flows to E2E tests through imports, not through execution.
- SKH-V0 extracts page objects (`SKPageObject`), methods with stable IDs (`SKMethod.ID`), test bodies
  in a closed step IR (`SKTest.Steps`) and deduplicated step sequences with their users
  (`SKRecord{ID, Steps, Usage, Tests}`), all anchored to blob-pinned spans (`appmap.Anchor`).
- RVN-V0 binds Playwright receipts to map steps; CCE-V0 compiles change-specific scenarios.
- DCP-V1 and `corvint.map_knowledge` serve the map and how-to knowledge as documentation. None of
  them reads coverage.

The simpler alternative, kept as the fallback throughout, is the existing static plan plus per-test
line selection: it never dedupes repeated steps and never tells the map which tests executed it.

## Definitions

- **Segment**: a contiguous part of one test run that the collector can bound: a Playwright
  `test.step`, one call of a page-object method, one setup unit (SKH setup record), or the whole
  test when no finer boundary exists.
- **Segment identity**: the SKH-V0 identity the segment executed, in this order of preference: the
  `SKRecord.ID` of the step sequence, the `SKMethod.ID` plus its argument template (`SKArg.Template`),
  the setup record ID, or the step title and anchor. Every identity is pinned to the commit of the
  record.
- **Reach**: the set of coverage blocks, product-side and test-side, one segment executed
  (LPCV-V0-950, LPCV-V0-954).
- **Functional unit**: a screen or flow (AMAP-V0), a page object or method (SKH-V0), or a setup
  record.
- **Level**: `unit`, `integration`, `e2e` or `unknown`, as MCJ-V0-007 assigns it.

## Requirements

### Records and join

- `MCJ-V0-001`: A coverage record (LPCV-V0-950) MAY carry an ordered list of segments, each with
  its segment identity, its reach and its elapsed time. A record without segment boundaries has one
  segment, the whole test, so every existing record stays valid and selects exactly as before.
- `MCJ-V0-002`: A segment identity binds only when the SKH-V0 model read at the record's commit
  holds that identity at the same anchor blob. Otherwise the segment is `unbound`: it keeps its
  reach for test-level selection and joins no functional unit.
- `MCJ-V0-003`: Segments are stored in the immutable linecov index as a new format version
  (`linecov-index/3`). A reader of `linecov-index/2` refuses it, and `linecov-index/3` reads a
  `/2` index as one segment per test. The index stays derived state with no database (invariant 7).
- `MCJ-V0-004`: For a change, the join maps each changed block to the segments whose reach holds
  it, and each segment to its test and functional unit. The plan reports, per test, the segments
  through which the change reaches it.

### Functionality links

- `MCJ-V0-020`: Each functional unit has a **code set** and a **test set**, every entry carrying its
  authority, highest first:
  1. declared: AFU-V1 `test` links and outcomes, AMAP-V0 declared flows, and ownership the project
     declares in its Corvint configuration;
  2. static: AMAP-V0 route-to-component and handler imports, and SKH-V0 page-object methods and the
     tests that call them (`SKTest.Steps`, `SKRecord.Tests`);
  3. observed: the product-side blocks reached by the unit's bound segments (MCJ-V0-002), and the
     tests owning those segments.
  A higher authority is never overwritten by a lower one; a disagreement is reported (MCJ-V0-011).
- `MCJ-V0-021`: When a changed block has no fresh record that covers it, the plan MUST add the test
  set of every unit whose code set holds that block, with reason `functionality-link` naming the
  unit and the authority. Uncovered blocks include new code, a `coverage-collector-missing` family,
  an incomplete record (MCJ-V0-015) and a file absent from the index. This rule only adds tests:
  it never removes a test that another rule keeps.
- `MCJ-V0-022`: Using a `functionality-link` set in place of a broader static fallback (for example
  a FULL run forced by an E2E consumer of a built binary) narrows the plan. It runs in shadow first
  under its own bar, and a unit whose code set is only observed never replaces a fallback. The
  shadow runs the full fallback; a **fallback miss** is a test the fallback ran that failed while
  the proposed linked set omitted it. Replacement is allowed only after zero fallback misses over
  at least 1,000 CI changes that took the fallback, plus the MCJ-V0-014 fault injection; a fallback
  miss is handled as MCJ-V0-018 describes, with the omitted test and the unit in the ticket. A
  changed block keeps the fallback, with its reason, when no unit links it (`unlinked`), only an
  observed code set reaches it (`observed-only`), or declared and static code sets hold only part
  of it (`partially-linked`, one entry per unheld part).
- `MCJ-V0-023`: A change to a unit's test-side surface selects that unit's affected tests even when
  no product block changed. Test-side surfaces are a page-object method, a selector binding, a step
  helper, a setup record or a flow declaration. The affected tests are the tests whose steps call
  that method or binding, or that use that record or flow, as SKH-V0 extracts them.
- `MCJ-V0-024`: The plan and the documentation list the gaps. **Untested functionality** is a unit
  with a code set and an empty test set. **Unmapped tests** are tests linked to no unit.
  **Unlinked code** is changed code in no unit's code set; it keeps the static plan. Each gap is
  reported as data that a `tests` ticket can cite.

### Shared-step selection

- `MCJ-V0-005`: Two segments of different tests are **equivalent** only when all of these hold at
  the index commit:
  1. their identities are equal and bound (MCJ-V0-002);
  2. their reaches are the same block set;
  3. the segments before them in each test have equal identities, or both start from the same
     bound setup record, so they run from the same state;
  4. the segments after them in each test have equal identities through the end of the test, so
     every check either test makes after the shared step is one the other makes too. A test with
     an assertion, expectation or step the other lacks after the shared step is not equivalent.
  Equivalence is never inferred from names or titles alone.
- `MCJ-V0-006`: When every segment through which a change reaches test T has an equivalent segment
  in a selected test R, the plan MAY exclude T as `covered-by-representative`, naming R and the
  segments. In each equivalence class it picks the representative with the smallest recorded
  elapsed time, breaking ties by test identity (LPCV-V0-022). A change that reaches any segment of
  T without an equivalent keeps T. A change to T's own test-side code that its representative does
  not share keeps T. Matching the changed segments alone never suffices: T is excluded only when
  condition 4 shows R makes every check T makes after the shared step, otherwise both run.
- `MCJ-V0-007`: Each record carries a level. A project-declared level wins: a runner project or
  config (for example a Playwright project), a build tag, or a path rule declared in the
  repository's Corvint configuration. Otherwise the runner family's default applies: browser E2E
  families (LPCV-V0-955) are `e2e`, and everything else is `unknown`. Level groups the plan report
  and the documentation; it never changes selection.
- `MCJ-V0-008`: Corvint MUST NOT skip, stub or reorder a step inside a running test to save time. It
  removes whole tests only. A setup substitution (replacing repeated navigation with a saved or
  API-made state) is emitted only as a reviewable suggestion naming the SKH setup record that proves
  the equivalent state; applying one is a test-source change made by the project.

### Map and documentation

- `MCJ-V0-009`: The AMAP-V0 overlay seam gains one fact kind, `coverage-reach`. For each functional
  unit it lists the tests, with level, whose bound segments executed it at the index commit. It reads
  `STALE` once any blob in those segments' reach has changed at head, and `UNKNOWN` when no record
  covers the unit.
- `MCJ-V0-010`: The DCP-V1 corpus, `corvint.map_knowledge` and how-to answers (SKH-V0) MUST show the
  `coverage-reach` tests for each unit they document, with the commit and the freshness. They label
  the tests as `executed`. A test counts as `proven` only through RVN-V0 verification or a CCE-V0
  discriminating scenario, because executing code is not asserting its outcome.
- `MCJ-V0-011`: For a change, the plan names the functional units it reaches ("affected
  functionality"). CCE-V0 obligation compilation and the PR review report read that list. Declared
  links (AFU-V1 `test` links, AMAP-V0 declared flows) outrank the coverage join when they disagree,
  and both are shown.

### Collectors

- `MCJ-V0-012`: Playwright is the first segment collector. It bounds `test.step` and page-object
  method calls through a fixture or reporter the project opts into. It joins browser coverage (V8
  precise coverage through the DevTools protocol, mapped through source maps) with server coverage
  (LPCV-V0-954). It runs in the external provider companion, never in the core binary (invariant 7,
  LPCV-V0-037).
- `MCJ-V0-013`: Ruby (RSpec, Capybara, SitePrism, which SKH-V0 already reads) and TypeScript unit
  runners follow, then the Beamfall adopter languages the owner names. Each collector is qualified
  separately, and a family without a qualified collector keeps the static plan
  (`coverage-collector-missing`, LPCV-V0-955).
- `MCJ-V0-014`: A collector qualifies only with a fault-injection fixture per segment kind, on the
  model of LPCV-V0-963. For each kind, a change reaching a non-shared segment must keep its test,
  and a change reaching only an equivalent segment must exclude all but the representative.

### Safety, evidence and qualification

- `MCJ-V0-015`: A segment record is incomplete when a process it started was not instrumented, a
  boundary was missing, or a source map did not resolve. Its test then keeps only its test-level
  record and takes part in no equivalence class.
- `MCJ-V0-016`: Every `covered-by-representative` exclusion in the plan lists the representative,
  the segment identities and the block-set equality, and the plan counts tests by level and by
  reason (extending LPCV-V0-961).
- `MCJ-V0-017`: Shared-step exclusion runs in shadow first: CI computes it, runs every test anyway,
  and records the result. It may exclude tests only after zero misses over at least 1,000 CI changes
  plus the MCJ-V0-014 fault injection, the same bar as LPCV shadow qualification. A **miss** is an
  excluded test that fails while its representative passes.
- `MCJ-V0-018`: A miss marks that equivalence class divergent in the next index until a blob in its
  reach changes, files a `bugs` ticket with the two test identities and the change, and blocks
  promotion until the cause is explained.
- `MCJ-V0-019`: The join and shared-step analysis MUST keep the LPCV-V0-958 plan within p95 ≤ 100 ms
  beyond the `git diff` and base-blob reads on the reference host, measured with segments present.


## Traceability

Slice 3a is experimental: focused tests in `internal/funclink/funclink_test.go`, no live run. The
library is not yet wired into the plan, the CLI or CI, so no plan carries its reasons.

| Requirement | Implementation | Evidence |
| --- | --- | --- |
| MCJ-V0-020 | `internal/funclink/funclink.go` (`Builder`, `Build`, authority mask), `sources.go` (`FromIntents`, `FromMap`, `FromImpact`, `FromSiteKnowledge`, `FromSetup`, `FromObserved` over the `ObservedSource` interface) | `TestMCJV0_020_UnitsCarryCodeAndTestSetsFromEverySource`, `TestMCJV0_020_LowerAuthorityNeverOverwritesHigher`, `TestMCJV0_020_ObservedSourceErrorAddsNothing`. Partial: project-config declared ownership is not read; the observed source is an interface with a test fixture only |
| MCJ-V0-021 | `select.go` `AddUncovered` | `TestMCJV0_021_AddsLinkedTestsForUncoveredBlocksOnly`, `TestMCJV0_021_023_FaultInjectionNeverDropsATest` (2,000 seeded random cases) |
| MCJ-V0-022 | `select.go` `ShadowReplace`, `FallbackMisses` (records the proposed linked set beside the fallback set and the omitted tests) | `TestMCJV0_022_ShadowReplacementRecordsTheFallbackMissCheck`. Shadow record only: no CI shadow run, so the 1,000-change zero-miss bar is `NOT_RUN` |
| MCJ-V0-023 | `select.go` `SelectSurfaces` | `TestMCJV0_023_PageObjectMethodChangeSelectsExactlyItsCallers`, `TestMCJV0_021_023_FaultInjectionNeverDropsATest` |
| MCJ-V0-024 | `select.go` `Gaps` | `TestMCJV0_024_GapReport` |

Test identities stay opaque: SKH-V0 test IDs, AFU-V1 test keys and `linecov` test IDs are separate
spaces, and joining them is the caller's.

## Failure modes

| Failure | Behaviour |
|---|---|
| SKH model unavailable or stale at the record commit | segments `unbound` (MCJ-V0-002); test-level selection only |
| Data-dependent step (same identity, different block sets) | not equivalent (MCJ-V0-005 condition 2); both tests kept |
| Same shared step, different later checks | not equivalent (MCJ-V0-005 condition 4); both tests kept |
| Linked set omits a test the fallback run fails | fallback miss; replacement stays in shadow (MCJ-V0-022) |
| Same step from a different prior state | not equivalent (condition 3); both tests kept |
| Uninstrumented browser, server or child process | record incomplete (MCJ-V0-015); test kept |
| Shadow miss | class divergent, ticket filed, promotion blocked (MCJ-V0-018) |
| Declared link contradicts coverage | declared link wins; both shown (MCJ-V0-011) |
| Changed code in no unit and no record | static plan unchanged; reported as unlinked code (MCJ-V0-024) |
| `linecov-index/3` read by an older reader | refused; CI runs its unnarrowed lists (LPCV-V0-951) |

## Non-goals

- No mid-test step skipping, stubbing or reordering (MCJ-V0-008).
- No new step language: segment identities come from SKH-V0 and the project's own test code.
- No embeddings, database, daemon or network dependency in the core binary (invariant 7).
- Coverage never counts as an assertion or as outcome proof (MCJ-V0-010).
- No change to LPCV-V0-950..963 selection when segments are absent, beyond the tests
  MCJ-V0-021 adds.

## Delivery slices

1. `linecov-index/3` segment format, Go one-segment-per-test reader, join library and plan reasons
   (MCJ-V0-001, 003, 004, 016).
2. Playwright segment collector with browser and server coverage (MCJ-V0-012, 015).
3. SKH identity binding and the `coverage-reach` overlay fact (MCJ-V0-002, 009).
3a. Functionality code and test sets, gap-filling selection, test-side surface selection and the
   gap report (MCJ-V0-020, 021, 023, 024); fallback replacement in shadow (MCJ-V0-022).
4. Shared-step equivalence and representative selection, shadow only (MCJ-V0-005, 006, 017).
5. Documentation and how-to `executed` / `proven` projection, and affected functionality for CCE
   and review (MCJ-V0-010, 011).
6. Level tagging plus the Ruby and TypeScript collectors (MCJ-V0-007, 013).
7. Fault-injection qualification, miss detector and plan-time measurement (MCJ-V0-014, 018, 019).
8. Setup substitution suggestions (MCJ-V0-008).

## Acceptance evidence

Each slice names focused tests by requirement ID, and slices 2, 4 and 7 name live runs. Slice 2
needs a Playwright run over a committed fixture app with a browser and a server. Slice 4 needs a CI
shadow summary. Slice 7 needs the fault-injection matrix and the measured plan-time p95. Promotion
from shadow (MCJ-V0-017) needs a recorded owner acceptance with the miss count, the change count and
the fault-injection result. Any run not performed stays `NOT_RUN`.

## Boundaries and rollback

The join only adds selection reasons and overlay facts. Removing it restores LPCV-V0-950..963
per-test selection unchanged. Rollback: unset the shared-step switch so exclusions stay
shadow-only, stop writing `linecov-index/3` (readers fall back to the last `/2` index), and drop the
`coverage-reach` overlay kind. No stored state outside the immutable index is created.

## Owner questions

Decided 2026-11-03 by the orchestrator under the owner's 2026-11-02 delegation; the owner may
revise any of them.

1. After Ruby and TypeScript (MCJ-V0-013), collectors follow the Beamfall adopter languages in
   order of their test count, taken from a census of the adopter repositories when slice 6 starts.
2. Shared-step exclusion (MCJ-V0-017) uses the LPCV bar, zero misses over at least 1,000 changes,
   counted only over changes that touch at least one segment-covered block, plus the
   per-segment-kind fault injection of MCJ-V0-014.
3. The representative is the cheapest test (MCJ-V0-006); CI time is the owner's stated goal, and
   documentation lists every executing test regardless of which one ran.
4. Playwright segment boundaries come from `test.step` and page-object calls without any
   test-source change; a project opt-in fixture is allowed only as a fallback, reported per run.
