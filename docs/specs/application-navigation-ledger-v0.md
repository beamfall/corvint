# Application Navigation Ledger V0

Owner: Russell Lewis
Date: 2026-10-10
Intent status: proposed
Delivery status: experimental
Authoritative inputs: owner requests V1-1132 (issue #774), V1-1167, V1-1133 (issue #775), V1-1134 (issue #776), V1-1135 (issue #777), V1-1137 (issue #779); `AGENTS.md`;
`docs/SPEC-DRIVEN-DEVELOPMENT.md`; AMAP-V0 and RVN-V0.

## Agent digest
- Claim: A revision-evaluated edge ledger reuses qualified receipts and fails closed on unproven assumptions.
- Status: proposed, pending owner acceptance (V1-1132); experimental.
- Exists: smallest single-map CLI slice; synthetic receipts only.
- Blocked on: owner acceptance and live adopter qualification.
- Read next: Requirements; Wire and limits; Traceability; Follow-ups.

## User job and current state

The adopter needs to reuse a previously proven navigation transition without launching a browser.
At base `55aa6bb9`, `internal/appmap/verify.go` verifies individual declared step/test bindings;
`types.go` stores ordered flow steps, stable screen/selector IDs, free-text actions and flow anchors.
No existing field reliably encodes click versus redirect versus form submission. Router templates
are URL templates, not component source files. This slice makes neither missing fact up.

## Requirements

- `ANL-V0-001`: Derive one edge per distinct (from screen ID, to screen ID, trigger, selector ID)
  from consecutive resolved flow steps. Use the destination step's exact action when it is
  `goto`, `click`, `redirect` or `form submit`; otherwise trigger is `UNKNOWN`. The selector ID
  belongs to that destination step. Retain same-screen edges, sort by content-derived edge ID,
  count unresolved pairs separately. An UNKNOWN trigger describes only declared adjacency, not
  an observed interaction. (proposed, pending owner acceptance (V1-1132))
- `ANL-V0-002`: Reuse RVN verification for each common test key bound to both endpoint steps,
  including sidecar declarations. Both endpoints must verify against the same receipt and app
  revision to prove a crossing. Report PROVEN, UNVERIFIED_AT_HEAD, CONTRADICTED or UNPROVEN;
  per-key evaluated unknowns and unplaced failures block proof; a key with no supplied receipt
  does not block another key's proof. Check both router lineages and the flow
  anchor at evaluated and receipt revisions; a changed flow selector or router URL template
  demotes placed evidence. Keep learned authority and receipt/test/flow/step references.
  (proposed, pending owner acceptance (V1-1132))
- `ANL-V0-003`: Deduplicate crossings by test key; choose the proven test with fewest declared
  flow steps, breaking ties by test key, then lexically. Print other proven test keys as redundant-crossers with
  a total count. Compute single-app proven/total coverage. A placed contradiction outranks a
  pass; unknown or stale evidence prevents promotion, and a skipped, unresolved-revision or
  otherwise inconclusive outcome for a proving test key blocks it even beside a fresh pass. (proposed, pending owner acceptance (V1-1132))
- `ANL-V0-004`: Accept a closed JSON sidecar with tests declaring `proves` and `assumes` edge
  IDs. Proves binds that test to both steps of each occurrence but never creates a passing
  outcome. Check the union of repeatable `--require` and all assumes against the full ledger;
  absent, unproven, contradicted and stale edges fail with exit 1 and a sorted unsatisfied list.
  Invalid inputs refuse with exit 2. (proposed, pending owner acceptance (V1-1132))
- `ANL-V0-005`: Emit deterministic `application-edge-ledger/0` JSON with evaluated revision,
  map digest, a digest of the complete ledger (digest field empty during hashing), and byte-capped
  projection with omitted counts. Trimming never changes coverage or assumption verdict. Use
  AMAP budget rules, safe bounded receipt/sidecar reads and no writes, browser or network.
  (proposed, pending owner acceptance (V1-1132))
- `ANL-V0-006`: `corvint flows appmap observe --map FILE --trace FILE|DIR [--revision REV]`
  reads closed `playwright-navigation-events/0` reporter JSON (`test_key`, ordered `events` of kind
  `navigate` or `redirect`, each with a URL and an optional triggering action with kind and selector;
  at most 64 files of 4 MiB and 4096 events each) and prints `application-navigation-observation/0`:
  per trace the ordered visits resolved through the map's URL lookup (hash prefix aware), and one
  observed edge per consecutive pair with trigger `goto`, `click`, `redirect`, `form submit` or
  `UNKNOWN`; `goto`, `click` or `form submit` (an action of kind `submit`) only when the event's
  action says so, so an event without action metadata or with another action, including a key press,
  reads `UNKNOWN`. A URL matching several screens reads UNKNOWN with sorted candidates; one matching none
  reads `unmapped-url`; an edge with an unresolved endpoint reads UNKNOWN. Observations are learned
  overlay facts carrying the trace digest and app revision and never add or change a static map
  node or edge. Playwright trace zips are refused in this slice.
  (proposed, pending owner acceptance (V1-1133))
- `ANL-V0-007`: Observation output MUST NOT contain URL userinfo, query values (including those
  inside a hash-route fragment) whose name is secret-shaped or whose value is opaque (24 or more
  token characters), or any action input value; such values print `REDACTED` or are dropped.
  (proposed, pending owner acceptance (V1-1133))
- `ANL-V0-008`: Observation output is deterministic with a digest of the complete observation,
  byte-capped by AMAP budget rules with omitted visit and edge counts that never alter the digest,
  read-only and offline. (proposed, pending owner acceptance (V1-1133))
- `ANL-V0-009`: `corvint test-plan prefixes --map FILE [--ledger FILE] [--facts FILE]` derives, per
  spec with a resolved test join, the ordered chain of the map's `test-sequence` edges and flags the
  spec when every edge of a chain of two or more edges is PROVEN in the `--ledger`
  (`application-edge-ledger/0`, matched on from and to screen) and the chain ends on a screen that is
  resolved, not abstract, has a template and whose lineage is FRESH at the evaluated revision. Each
  flag prints `{test, prefix_edges, proven_by, entry_template, saved_steps}` plus the entry screen,
  parameters, a `goto(template)` or session-fixture suggestion and the other flagged specs sharing the
  first edge. `saved_steps` is the edge count minus one. Without a ledger nothing flags.
  (proposed, pending owner acceptance (V1-1134))
- `ANL-V0-010`: A spec abstains with a reason, never flags, when its join is UNKNOWN
  (`join-unknown`), its chain is discontinuous (`sequence-discontinuous`), any prefix edge is declared an assertion target
  (`assertion-target`) or mutating (`state-change`) in the `--facts` sidecar
  (`test-prefix-facts/0`, one entry per spec path; a duplicate path is refused; keyed `FROM_SCREEN>TO_SCREEN` per spec path), has no declared read-only
  effect (`effects-unknown`), is not proven (`edge-not-proven`), or ends on an entry that is not
  deep-linkable (`entry-not-deep-linkable`) or not FRESH (`entry-template-not-fresh`), or saves
  nothing (`nothing-to-save`, a one-edge chain). (proposed, pending owner acceptance (V1-1134))
- `ANL-V0-011`: The report is deterministic with a digest of the complete report, byte-capped with
  omitted counts, learned authority, read-only and offline. A ledger not built from the map (app, map digest, evaluated revision) or trimmed to its byte cap is refused. With `--check` the command exits 1
  (`test-plan-mismatch`) when a flagged test is absent from `--baseline` or any flagged row was trimmed by the byte cap (an earlier report; any flag
  fails without one), and 2 on invalid input. (proposed, pending owner acceptance (V1-1134))
- `ANL-V0-012`: `test-plan page-state --input FILE` reads one `test-consolidation-input/0` document and
  groups variations that share app, screen, setup, user, org and the exact required fact set. A group
  has at least two variations (one visit can serve them), lists their IDs, specs and the union of
  assertions, and reports `visits_saved` as the variation count minus one. Groups may span specs.
  (proposed, pending owner acceptance (V1-1135))
- `ANL-V0-013`: A variation joins no group and is listed as blocked with a reason when any anchor is
  missing (`anchor-missing:<members>`), it is destructive or its destructive flag is unknown
  (`destructive`), it declares changes (`changes-state`, so a post-change state is never merged with
  a pre-change one). (proposed, pending owner acceptance (V1-1135))
- `ANL-V0-014`: The report (`test-page-state-groups/0`) is deterministic, sorted, independent of input
  order, carries the input digest and a digest of the report, is read-only and offline, and does not
  alter `consolidate` or `check`. Invalid input or use exits 2. (proposed, pending owner acceptance (V1-1135))

- `ANL-V0-015`: `flows appmap impact --map FILE --base COMMIT [--ledger FILE]` prints
  `application-map-impact/0` for the paths changed between `--base` and the evaluated revision. A
  changed router file (a screen's anchor, name or parent anchor) selects that screen and every
  descendant by lineage (`router-anchor`, `lineage:<screen>`); a changed attributed test file or flow
  anchor selects its screens (`test-attribution`, `test-file`, `flow-anchor`). The report lists the
  edges touching selected screens (with ledger status, `flipped` for `UNVERIFIED_AT_HEAD` and the
  cheapest re-proving test), the redundant crossing test keys and the attributed spec files; without
  a ledger the map's edges are listed as `NOT_IN_LEDGER`; a ledger not built from the map at the evaluated revision, or trimmed, is refused. (proposed, pending owner acceptance (V1-1137))
- `ANL-V0-016`: A changed UI path (html, script, style or template extension) that no map element
  places is listed as unknown (`unmapped-path`, or `manifest-changed` for the manifest) and forces the
  verdict `full-suite-required` for the app; otherwise the verdict is `targeted` or
  `no-navigation-impact`. The map binds no template or component files, so template changes are
  unknown until a component binding exists. The report is deterministic, byte-capped with omitted
  counts that never drop the verdict, digested, learned, read-only and offline; an unresolvable
  base or revision fails closed. (proposed, pending owner acceptance (V1-1137))
- `ANL-V0-031`: `corvint affected --base FULL_COMMIT_ID --appmap-impact REPORT` (repeatable up to 16,
  one report per app; refused without `--base` or with `--snapshot` or `--playwright-config`) reads
  each complete `application-map-impact/0` report (a bounded regular file, strictly decoded, with no
  omitted rows, a verdict that follows from its own lists and a matching digest; otherwise refused) whose
  `base` is `--base` and whose `head` is the receipt revision, and adds `advice.app_navigation`: per
  app, sorted, the learned authority, impact and map digests, `reported_verdict`, `screens` count,
  `spec_files` and the effective `verdict` with its `reasons`. The effective verdict is
  `full-suite-required` when the report lists unknown paths (`impact-unknown-paths`), the worktree
  has uncommitted paths the report cannot cover (`worktree-not-covered`) or the spec list exceeds 64
  (`spec-files-over-bound`); each such app also adds the `advice.unknown` line
  `APP_NAVIGATION_FULL_SUITE_REQUIRED: APP: REASONS`. The member is advisory: it never adds to or
  removes from `advice.checks`, the plan or the provider projection, so no selection is narrowed.
  This amends the AFP-V0-009 advice member set, also proposed. (proposed, pending owner acceptance (V1-1167))

## Wire and limits

`corvint flows appmap edges --map FILE [--receipt FILE]... [--revision REV]
[--declarations FILE] [--require EDGE_ID]... [--budget N | --full]`

Sidecar: `{"schema":"application-edge-declarations/0","tests":[{"test_key":"KEY",
"proves":["edge:HEX"],"assumes":["edge:HEX"]}]}`. At most 64 test declarations, 64 IDs per
list, 64 command requirements, 4096 adjacent pairs and 256 distinct test keys per pair.
Sidecars use the RVN regular-file reader capped at 4 MiB; receipts retain RVN's 16-file limit.
Default output cap is 6144 bytes. Counts cover complete lists even when rows are omitted.
Reused errors: `appmap-invalid-query`, `appmap-budget-too-small` and RVN receipt errors.
Ledger-internal verification reasons: `endpoint-unverified` (an endpoint lacks proof),
`crossing-evidence-mismatch` (endpoint proofs differ in receipt or revision), and
`flow-anchor-unverified` (flow freshness cannot support the receipt). These prevent PROVEN;
this slice reports the aggregate UNPROVEN state without projecting every rejected crossing.
Map/receipt digests are identity, not authenticity. Binding remains owner-labelled run evidence;
UNKNOWN trigger proof means the declared screen adjacency only. No claim of observed step ordering.

## Implementation plan and rollback

Add `internal/appmap/ledger.go`, a CLI file and focused tests; minimal dispatch/help and exit-code
hooks. Reuse the private RVN verifier and freshness cache. Run focused baseline, implementation
tests, vet and requested doc gates; independent review before handoff. Remove these additive hooks
and files to roll back; no store or map-format migration. Keep requirement IDs stable.

## Traceability

| Requirement | Implementation | Evidence |
| --- | --- | --- |
| ANL-V0-001 | `internal/appmap/ledger.go` | `TestANLV0001Identity` |
| ANL-V0-002 | `internal/appmap/ledger.go` | `TestANLV0002EvidenceAndDrift`, `TestANLV0002MixedEvidenceFailsClosed` |
| ANL-V0-003 | `internal/appmap/ledger.go` | `TestANLV0003Redundancy`, `TestANLV0003SameKeyUncertaintyBlocks` |
| ANL-V0-004 | `cmd/corvint/flows_appmap_edges.go` | `TestANLV0004EdgesCLI` |
| ANL-V0-005 | `internal/appmap/ledger.go` | `TestANLV0005Budgets` |
| ANL-V0-006 | `internal/appmap/observe.go`, `cmd/corvint/flows_appmap_observe.go` | `TestANLV0006Observe`, `TestANLV0006HashRouted`, `TestANLV0006ObserveCLI` |
| ANL-V0-007 | `internal/appmap/observe.go` | `TestANLV0007Redaction` |
| ANL-V0-008 | `internal/appmap/observe.go` | `TestANLV0008InputsAndBudget` |
| ANL-V0-009 | `internal/appmap/prefixes.go` | `TestANLV0009FlagsProvenPrefix` |
| ANL-V0-010 | `internal/appmap/prefixes.go` | `TestANLV0010NeverFlagsTargetsOrChanges`, `TestANLV0010RefusesDuplicateFactsAndForeignLedger` |
| ANL-V0-011 | `internal/appmap/prefixes.go`, `cmd/corvint/test_plan_prefixes.go` | `TestANLV0011StaleEntryAndBudget`, `TestANLV0011PrefixesCLI` |
| ANL-V0-012 | `internal/testplan/pagestate.go` | `TestANLV0012GroupsSharedPageState`, `TestANLV0012RequiredSetsStayDistinct` |
| ANL-V0-013 | `internal/testplan/pagestate.go` | `TestANLV0013BlocksAndSeparates` |
| ANL-V0-014 | `internal/testplan/pagestate.go`, `cmd/corvint/test_plan_pagestate.go` | `TestANLV0012GroupsSharedPageState`, `TestANLV0014PageStateCLI` |
| ANL-V0-015 | `internal/appmap/impact.go`, `cmd/corvint/flows_appmap_impact.go` | `TestANLV0015RouterChangeReachesDescendants`, `TestANLV0015ImpactCLI` |
| ANL-V0-016 | `internal/appmap/impact.go` | `TestANLV0016UnknownForcesFullSuite` |
| ANL-V0-031 | `internal/appmap/impact.go`, `cmd/corvint/affected_appmap.go` | `TestANLV0031AffectedRoutesImpactVerdict` |

## Follow-ups and promotion

Proposed follow-ups: typed trigger declarations and spec annotation parsing; component/template
file and page-object anchors absent from AMAP; multi-map per-app aggregation; costs beyond declared
step counts; test-plan integration; live adopter qualification. UNKNOWN trigger is not a typed
trigger acceptance claim. Promotion needs owner acceptance plus demonstrated useful reuse on an
owner-controlled suite; synthetic checks alone retain experimental status. Kill or revise the
projection if declared adjacency cannot safely support that suite. Live qualification is NOT_RUN.
