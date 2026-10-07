# Run-Verified Navigation V0 — Playwright receipts bound to application-map steps

Owner: Russell Lewis
Date: 2026-10-07
Intent status: proposed
Delivery status: experimental
Authoritative inputs: owner request [issue 658](https://github.com/beamfall/corvint/issues/658)
(native ticket V1-0957), `AGENTS.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`,
`docs/specs/application-map-v0.md` (AMAP-V0 map, freshness, projections and overlay seam),
`docs/specs/playwright-external-provider-v0.md` (PWP receipts, test identity and qualification),
`docs/specs/observation-corpus-authority-v0.md` (OCA-V0-004 authority limit) and
`docs/specs/application-flow-understanding-v1.md` (AFU-V1 declared `test` links), plus the
orchestrator note of 2026-10-07 on [issue 660](https://github.com/beamfall/corvint/issues/660)
(V1-0959, scenario planner reads per-step status and defaults to `unverified`).

## Agent digest
- Claim: Playwright receipts bound to map steps read VERIFIED at their app revision, UNVERIFIED_AT_HEAD after a source change, CONTRADICTED on failure.
- Status: proposed (pending owner acceptance; V1-0957); experimental. RVN-V0-001..008 are implemented in `internal/appmap/verify.go` and `corvint flows appmap screen|flow --receipt --bind` over the committed AMAP fixture with synthetic PWP receipts; no live Playwright run or adopter qualification.
- Exists: `Step.tests` from declared AFU-V1 `test` links, per-step `verification` on the `screen` and `flow` projections, `appmap.VerifySteps` for in-process callers (V1-0959), three owned refusal codes.
- Blocked on: owner acceptance; AMAP-V0 (V1-0956) acceptance; live receipt qualification.
- Read next: Requirements; Status lattice; Failure modes; Owner questions.

## User and measurable job

An agent repairing or extending an E2E path needs to know which navigation steps a real browser run
has actually exercised at a known application revision, which a run has refuted, and which have
changed since the run. AMAP-V0 answers only "these anchors are unchanged since the map revision";
it never says a step works. Measured jobs, each over the AMAP fixture with synthetic qualified PWP
receipts:

1. A passing receipt outcome whose test ID is bound to a step prints that step `VERIFIED` at the
   receipt's application revision with a receipt reference.
2. Editing the step's router state after that revision prints `UNVERIFIED_AT_HEAD`.
3. A failing bound outcome prints `CONTRADICTED`.
4. An unknown, unresolvable or non-commit application revision never prints `VERIFIED`.

## Verified current state

At `43c65094` (gh657 lane head): AMAP-V0 compiles steps with router-state anchors and computes
FRESH/STALE/UNKNOWN per call; `internal/appmap/overlay.go` defines a learned-fact seam that names
issue 658 but implements no verification, and AMAP-V0 lists "No run verification" as a non-goal.
PWP already defines qualified receipts (`corvint-playwright-external/0..3`), a content-derived test
ID per outcome (`TestOutcome.id`, a digest over the run identity, test-file digests, project,
anchor and full title), `QualifiedReceiptBindingReady` and `QualifiedApplicationRevision`. AFU-V1
intents already carry `links` with `basis` `declared` or `inferred` and a `test` target. No
component joined them.

## Definitions

- **Bound key**: a test key attached to a step, either a declared AFU-V1 link or a `--bind`.
- **Outcome**: one `TestOutcome` of a supplied receipt whose `id` equals a bound key.
- **App revision**: `jstestprovider.QualifiedApplicationRevision` of the outcome: the attested
  repository revision for `/1`, or `/2` with attestation (`app_identity: attested`), else the
  receipt's declared application identity (`app_identity: declared`).
- **App-source anchors**: the router-state anchors of the step's screen and every ancestor screen
  (the AMAP-V0 lineage). The flow intent and the test source are test-side: a changed test file
  already changes the PWP test ID.
- **Placed**: an outcome whose app revision is a full commit OID that resolves to itself under
  `--root`, and whose app-source anchors are all FRESH at that revision.

## Requirements

Every requirement below is (proposed, pending owner acceptance; V1-0957).

- `RVN-V0-001`: `corvint flows appmap screen` and `flow` MUST accept repeatable
  `--receipt FILE` (at most 16) and `--bind STEP_ID=TEST_KEY` (at most 64) and nothing else new.
  Each receipt MUST be a regular file of at most 4 MiB, opened without following a final symlink
  or blocking on a FIFO, with the same identity, size and modification time before, at open and
  after the read, that
  decodes through `testvaliditydoc.Decode` as canonical bytes and projects to a Playwright external
  provider document (any PWP profile); otherwise refuse with `appmap-verify-invalid-receipt`.
  `--bind` without `--receipt`, or too many inputs, MUST refuse with `appmap-invalid-query`. With no
  `--receipt` the projection MUST be byte-identical to AMAP-V0 output. Identical receipts are
  deduplicated by SHA-256. (proposed, pending owner acceptance; V1-0957)
- `RVN-V0-002`: A step's bound keys MUST be the union of `Step.tests` (compiled into the map from
  the flow intent's `basis: declared` links whose target type is `test`; an `inferred` link never
  binds) and its `--bind` keys. A `--bind` whose step ID does not start with `step:` or whose key
  is empty, over 256 bytes or contains space or control characters MUST refuse with
  `appmap-invalid-query`; a step ID absent from the map MUST refuse with
  `appmap-verify-unknown-step`; a key equal to no outcome ID of any supplied receipt MUST refuse
  with `appmap-verify-test-absent`. A receipt outcome binds a step only when its PWP test ID equals
  a bound key; titles, files and URLs never bind. (proposed, pending owner acceptance; V1-0957)
- `RVN-V0-003`: An outcome MUST count as a pass only when `QualifiedReceiptBindingReady` holds and
  its state is `passed`, and as a failure only when it is ready and its state is `failed` or
  `timedOut`; every other outcome is `inconclusive-outcome`. A pass or failure MUST be placed before
  it can verify or contradict: an app revision that is not a full lowercase commit OID resolving to
  itself under `--root` reads `app-revision-unresolved`; an app-source anchor STALE at that revision
  reads `anchor-differs-at-app-revision`; freshness UNKNOWN at a resolved revision reads
  `freshness-unknown`. An unknown or unresolvable revision MUST never yield `VERIFIED` or
  `CONTRADICTED`, and a failure with an unresolved revision (`unplaced-failure`) MUST block
  `VERIFIED`. (proposed, pending owner acceptance; V1-0957)
- `RVN-V0-004`: Placed evidence MUST be compared with the projection's evaluated revision
  (`--revision`, default HEAD): any app-source anchor STALE there MUST yield `UNVERIFIED_AT_HEAD`
  citing the placed evidence and its revision; any UNKNOWN there MUST yield `unverified` with
  `freshness-unknown`. Recompiling the map at a later revision pins the new source, so an older
  receipt then reads `anchor-differs-at-app-revision`. (proposed, pending owner acceptance; V1-0957)
- `RVN-V0-005`: The status MUST be exactly one of `VERIFIED`, `UNVERIFIED_AT_HEAD`,
  `CONTRADICTED` or `unverified`, decided in this order: no bound outcome (`no-binding` or
  `no-receipt-outcome`); evaluated freshness UNKNOWN; evaluated STALE (`UNVERIFIED_AT_HEAD`); a
  placed failure (`CONTRADICTED`, which outranks any pass); a placed pass with no unplaced failure
  (`VERIFIED`); else `unverified` with the first reason of `freshness-unknown`, `unplaced-failure`,
  `app-revision-unresolved`, `anchor-differs-at-app-revision`, `inconclusive-outcome`. A step whose
  AMAP status is not resolved reads `unverified` with `step-unresolved`. The cited evidence is the
  least by (revision, receipt SHA-256, test key, project). (proposed, pending owner acceptance;
  V1-0957)
- `RVN-V0-006`: When receipts are supplied, every step item of the `screen` and `flow` projections
  MUST carry `verification{status, revision?, receipt{sha256, test_key, project, profile}?,
  app_identity?, selector_evidence?, reason?, outcomes, authority}` inside the existing AMAP-V0
  budgets, refusing only with `appmap-budget-too-small`; trimmed steps drop their verification with
  them. A step's status MUST depend only on its own app-source anchors, never on another anchor of
  the projection. Verification MUST be computed per call, persist nothing, and leave the repository and
  worktree unchanged. In-process callers MUST obtain the same statuses from
  `appmap.VerifySteps(ctx, map, flow, Options)` keyed by step ID. (proposed, pending owner
  acceptance; V1-0957)
- `RVN-V0-007`: `verification.authority` MUST always be `learned`. A run status MUST NOT change any
  other projection field, the selector's static strength (`selector_evidence` is a separate
  `run-verified` or `run-contradicted` label), freshness, candidate-research or intent status, and
  MUST NOT feed ranking, evidence admission or authority: no ranking or authority package imports
  `internal/appmap`. Per OCA-V0-004 a `VERIFIED` step is owner-labelled run evidence that one bound
  test passed against one app revision, not a truth claim about unobserved behavior. (proposed,
  pending owner acceptance; V1-0957)
- `RVN-V0-008`: Verification MUST read at most 16 receipts of at most 4 MiB each and evaluate
  freshness once per distinct (revision, router lineage) per call (one tree read plus the bounded
  AMAP-V0 blob reads each), with no background work, network, browser execution or cache file. (proposed, pending
  owner acceptance; V1-0957)

## Status lattice

| Status | Meaning | Cites receipt |
| --- | --- | --- |
| `VERIFIED` | A placed, qualified passing outcome; no placed or unplaced failure; app-source anchors FRESH at the evaluated revision. | yes, with `revision` |
| `UNVERIFIED_AT_HEAD` | Placed evidence exists, but an app-source anchor changed between the map revision and the evaluated revision. | yes |
| `CONTRADICTED` | A placed, qualified failing outcome; anchors FRESH at the evaluated revision. | yes |
| `unverified` | Anything else; `reason` names why. Also the consumer default when no `verification` is printed. | no |

Reasons: `step-unresolved`, `no-binding`, `no-receipt-outcome`, `freshness-unknown`,
`unplaced-failure`, `app-revision-unresolved`, `anchor-differs-at-app-revision`,
`inconclusive-outcome`. Selector evidence labels: `run-verified`, `run-contradicted`.

## Wire contract

`corvint flows appmap screen|flow --map FILE --screen|--flow VALUE [--budget N | --full]
[--revision REV] [--receipt FILE]... [--bind STEP_ID=TEST_KEY]...`. Paths are relative to the
working directory, like `--map`. `find`, `scaffold` and `build` are unchanged. The map artifact
`application-map/0` gains an optional `tests` array on steps (omitted when empty), so maps without
declared test links are byte-identical. Go callers: `appmap.LoadVerification(map, receipts, binds)`
then `Options.Verification`; statuses are `appmap.Verified`, `UnverifiedAtHead`, `Contradicted`
and `Unverified` on `appmap.StepVerification.Status`.

### Owned error codes

| Code | Meaning |
| --- | --- |
| `appmap-verify-invalid-receipt` | A `--receipt` is not a regular file within 4 MiB, is a symlink or FIFO, changed while opened or read, is not canonical, or is not a PWP Playwright external receipt. |
| `appmap-verify-unknown-step` | A `--bind` names a step ID absent from the map. |
| `appmap-verify-test-absent` | A `--bind` key equals no outcome ID in any supplied receipt. |

`appmap-invalid-query` (AMAP-V0) is reused for a malformed `--bind`, `--bind` without `--receipt`,
or too many inputs.

## Non-goals and simpler baseline

- No browser execution, receipt production or receipt storage; receipts are explicit inputs.
- No ledger or history: a status is recomputed on every call and never remembered.
- No per-screen, per-method or per-selector status beyond the step's `selector_evidence` label.
- No `flows navigate` or MCP surface; no TypeScript alias resolution (V1-0958); no scenario
  planning (V1-0959 reads these statuses).
- No recency rule: a newer pass does not clear an older placed failure.
- Simpler baseline: "the test passed in CI". It names no app revision, does not notice a later
  router change, and does not say which navigation step the test exercised.

## Trust boundary, limits and failure modes

- Receipts are untrusted: decoded only through the canonical PWP document contract and bounded
  before decode. A declared app identity is the runner's claim; it is labelled `declared` and can
  only place evidence when it resolves to an exact commit in `--root`.
- A receipt for a different repository whose OID does not exist locally reads
  `app-revision-unresolved`, never `VERIFIED`.
- A step whose router state is unresolved or unanchored reads `step-unresolved` or
  `freshness-unknown`.
- Component templates, page objects and backend code are not app-source anchors in V0, so a change
  confined to them does not demote `VERIFIED` (owner question 3).
- A bound test that exercises a step only incidentally still verifies it; binding quality is the
  author's declaration (owner question 1).

## Deterministic acceptance and traceability

| Requirement | Evidence |
| --- | --- |
| RVN-V0-001 | `TestRVNV0002DeclaredBindings`, `TestRVNV0006BudgetsAndReadOnly`, `TestRVNV0FlowsAppmapVerificationCLI` |
| RVN-V0-002 | `TestRVNV0002PassingReceiptVerifiesStep`, `TestRVNV0002DeclaredBindings`, `TestRVNV0002InferredLinkDoesNotBind`, `TestRVNV0FlowsAppmapVerificationCLI` |
| RVN-V0-003 | `TestRVNV0003UnresolvedRevisionNeverVerifies`, `TestRVNV0003FailingReceiptContradicts` |
| RVN-V0-004 | `TestRVNV0004SourceChangeUnverifiesAtHead` |
| RVN-V0-005 | `TestRVNV0002PassingReceiptVerifiesStep`, `TestRVNV0003FailingReceiptContradicts`, `TestRVNV0004SourceChangeUnverifiesAtHead` |
| RVN-V0-006 | `TestRVNV0006BudgetsAndReadOnly`, `TestRVNV0006StatusIgnoresUnrelatedAnchors`, `TestRVNV0002PassingReceiptVerifiesStep`, `TestRVNV0FlowsAppmapVerificationCLI` |
| RVN-V0-007 | `TestRVNV0007AuthorityLimit`, `TestRVNV0002PassingReceiptVerifiesStep` |
| RVN-V0-008 | `TestRVNV0002DeclaredBindings`, `TestRVNV0006BudgetsAndReadOnly` |

Not run: a live Playwright run producing a receipt against a real application; adopter-scale
qualification.

## Rollout, rollback and compatibility

Experimental and opt-in: nothing changes without `--receipt`. Rollback is reverting
`internal/appmap/verify.go`, the `verification` field, the `--receipt`/`--bind` flags and the
`Step.tests` derivation; maps that carry `tests` still decode only if the field is kept, so a
rollback also rebuilds maps. No stored state needs migration.

## Owner questions

1. Should a declared (unattested) app identity be allowed to place evidence, or only attested `/1`
   and `/2` receipts? V0 allows declared and labels it.
2. Should a newer placed pass supersede an older placed failure? V0 keeps `CONTRADICTED`.
3. Should component templates, page objects or the flow intent count as app-source anchors? V0
   uses router-state lineage only, to avoid demoting every receipt when an intent gains a link.
4. Should verification also attach as an overlay `Fact` (AMAP-V0-014) rather than a typed field?
   V0 uses a typed field so V1-0959 can read it without parsing text.
5. Should `flows navigate` and MCP tools expose the same status?
