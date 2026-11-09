# Human Documentation Compiler downstream profile: Cross-Audience Truth Nondivergence V0

Owner: Russell Lewis
Drafted: 2026-08-30
Intent status: proposed
Delivery status: experimental pure-core slice
Status note: downstream profile of `human-documentation-compiler-v0.md`; frozen until HDC emits one
real implementation plan.
Implementation: `internal/docviews`
Authoritative inputs: `docs/PRODUCT.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`,
`docs/specs/human-documentation-compiler-v0.md`,
`docs/specs/deployment-neutral-index-platform-v0.md`

## Agent digest
- Claim: One evidence ledger must compile consistent expert, executive, and operator views without audience-specific factual divergence.
- Status: proposed/experimental pure-core slice
- Exists: `internal/docviews` deterministic compilation and verification.
- Blocked on: a real Human Documentation Compiler implementation plan and downstream qualification.
- Read next: Product claim and measurable job; Existing contracts and ownership; Requirements.

## Product claim and measurable job

Given one immutable Human Documentation Compiler plan and one disclosure scope, Corvint can compile
novice, operator, API, security, and executive projections that may differ only in claim order and
declared detail level. Every projection binds one shared truth corpus; it cannot copy, replace,
omit, add, promote, conceal, or independently stale a claim.

This is a narrow structural nondivergence claim. It is not a claim that the source plan is true,
complete, current, reviewed, safe to disclose, accessible when rendered, or accepted by a human.
It does not implement prose generation or a documentation surface.

The initial proof job is deterministic compilation and independent verification, against the exact
source HDC plan, of five projections over a mixed corpus containing `SUPPORTED`, `CONFLICTED`, and
`UNKNOWN` claims plus independent review-state, currency, and limitation metadata. Every audience
must reference every truth claim exactly once and bind the same truth and disclosure digests.

## Existing contracts and ownership

The Human Documentation Compiler remains the sole owner of claim/evidence validation, proposal
planning, patches, navigation, MkDocs execution, offline qualification, receipts, and repository
non-mutation. This profile consumes the admitted `corvint-human-documentation-plan/0` wire
(`doccompiler.AdmittedPlan`, `HDCV0-027`, decision 0231) through `VerifyAdmittedPlan` without
modifying it and does not add a method to `doccompiler.Compiler`. The experimental `PatchPlan` carries
a different profile and is not an input (decision 0240).

Clauses `HDCV0-023..032` govern stable claims and read-only planning. `HDCV0-041` governs deterministic
HDC bytes. `DNIP-DOC-012` owns generated/verified/reviewed lifecycle and stale currency semantics.
`LDG-V0-010` requires serving surfaces to expose the same corpus and trust metadata, but this slice
does not claim those surfaces exist.

Audience names are presentation intents, never identities, capabilities, access roles, or policy
decisions. Access and sensitive-content filtering occur before this compiler. All five projections
belong to one already-authorized disclosure scope.

## Closed model

Profiles are exactly:

- truth corpus: `corvint-cross-audience-truth/0`;
- audience view: `corvint-cross-audience-view/0`;
- bundle: `corvint-cross-audience-bundle/0`;
- verification report: `corvint-cross-audience-verification/0`.

Audience is exactly `NOVICE`, `OPERATOR`, `API`, `SECURITY`, or `EXECUTIVE`. Detail is exactly
`BRIEF`, `STANDARD`, or `FULL`. Detail is presentation metadata only and cannot change claim
membership or truth.

Each truth claim copies the admitted plan clause's exact ID, state, kind, scope, text, frontier, and
complete anchors (as `evidence`) into the truth corpus once. A `SUPPORTED` claim has at least one
anchor, an `UNKNOWN` claim carries its frontier and no other state does. Its overlay adds exactly one review state (`GENERATED`, `VERIFIED`, `REVIEWED`),
one currency (`CURRENT`, `STALE`, `UNKNOWN`), an explicit reason for non-current currency, and a
canonical limitations set. These fields do not change the HDC claim state.

An audience view contains only audience, ordered claim IDs, detail, disclosure policy/scope, profile,
and truth digest. It contains no claim text, state, kind, scope, frontier, evidence, review state, currency,
limitation, prose, visibility override, or authorization field. A consumer resolves every ID
against the bundled truth corpus.

The truth digest is:

```text
SHA-256("corvint-cross-audience-truth/0\0" || canonical-truth-json-without-LF)
```

Canonical JSON is exactly the `HDCV0-041` encoding emitted by `doccompiler`'s canonical emitter
(decision 0227): UTF-8, no insignificant whitespace, object members sorted by UTF-8 bytes, integer
numbers only, exactly the `HDCV0-041` string escape set (no HTML escaping of `<`, `>`, or `&`, and
U+2028 and U+2029 as literal UTF-8), contract-defined arrays, no maps, and exactly one trailing LF.
`canonical-truth-json-without-LF` is those truth-corpus bytes with that one trailing LF removed; the
bundle bytes keep it. The `HDCV0-041` 8 MiB bound governs an HDC plan or receipt only; this
profile's output bound is `CATN-V0-012`. Claims sort by ID; evidence and limitations sort by their
complete canonical key; views sort in the audience order above. Claim order inside a view is the explicitly supplied
projection order and remains digest-bound.

## Requirements

- `CATN-V0-001`: Compilation MUST bind one `corvint-human-documentation-plan/0` admitted plan that
  `doccompiler.VerifyAdmittedPlan` accepts against the supplied index and proposal patch, the SHA-256
  of those exact plan bytes, the plan's `source_identity.revision`, one disclosure-policy SHA-256, and
  one disclosure scope. A missing index, tampered plan or patch, or experimental `PatchPlan` bytes
  are `INVALID_PLAN` (decision 0240).
- `CATN-V0-002`: The compiler MUST reuse plan claims without extracting evidence, inferring
  authority, generating prose, or promoting claim state.
- `CATN-V0-003`: The truth corpus MUST store each claim ID exactly once and MUST preserve the plan's
  state, kind, scope, text, frontier, and complete anchor values. Evidence is ordered by the complete
  anchor key without removing a repeated anchor, so equal adjacent keys are canonical.
- `CATN-V0-004`: Every truth claim MUST have exactly one valid review state and currency overlay.
  `STALE` and `UNKNOWN` require a reason; `CURRENT` cannot carry one. Limitations remain visible.
- `CATN-V0-005`: A conforming bundle MUST contain exactly one projection for each of the five
  audiences in contract order.
- `CATN-V0-006`: Each projection MUST reference every truth claim exactly once. Missing, duplicate,
  or unknown claim IDs fail; the compiler never silently repairs a recipe.
- `CATN-V0-007`: Views MUST contain references and presentation metadata only. All views MUST bind
  the exact same truth digest, disclosure-policy digest, and disclosure scope.
- `CATN-V0-008`: Equal typed inputs MUST produce byte-identical truth digests, bundles, verification
  reports, and canonical bytes without mutating caller-owned values.
- `CATN-V0-009`: Independent verification MUST require the exact source HDC plan, recompute and
  re-verify it with `VerifyAdmittedPlan`, compare the bound plan digest and revision, compare every
  truth claim ID, state, kind, scope, text, frontier, and complete anchor value against that plan, recompute the truth digest, and reject invalid profiles/enums,
  noncanonical truth order, membership divergence, duplicate/missing audiences, lifecycle
  violations, and disclosure-binding divergence. Bundle self-consistency without source-plan
  preservation MUST NOT pass.
- `CATN-V0-010`: Audience MUST NOT grant access or select sensitive claims. Restricted material is
  removed or replaced by an authorized, equally visible gap before this profile receives it.
- `CATN-V0-011`: The production package MUST perform no filesystem, Git, subprocess, network,
  renderer, apply, publication, logging, telemetry, or repository operation.
- `CATN-V0-012`: V0 is bounded to 10,000 claims, 256 evidence anchors per claim, 64 limitations per
  claim, 64 KiB claim/reason text, 4 KiB per limitation, five views, and 64 MiB canonical output.
  `Verify` MUST measure the complete canonical bundle, including its trailing LF, and return
  `LIMIT_EXCEEDED` before PASS when it is larger than 64 MiB. Limit failure produces no partial
  bundle. `CanonicalBundle` MUST return `LIMIT_EXCEEDED` only when every reported divergence is
  `LIMIT_EXCEEDED`; any independently established non-limit divergence takes precedence as
  `NON_EQUIVALENT`. Neither failure returns canonical bytes or a digest.
- `CATN-V0-013`: This profile MUST NOT be presented as HDC delivery, audience usefulness,
  accessibility, disclosure correctness, or breakthrough evidence until the separate gates below
  pass.
- `CATN-V0-014`: A `CONFLICTED` claim MUST carry at least two evidence anchors with distinct
  `(path, blob, start_line, end_line)`, the profile's form of the `HDCV0-023` conflict rule
  (decision 0209). Admission already downgrades such a clause to `UNKNOWN`, so a plan claiming one
  does not verify. Compilation refuses fewer as
  `INVALID_PLAN` and verification reports `INVALID_TRUTH`. This profile does not re-qualify anchor
  authority, identity, range, or staleness; that validation stays with the Human Documentation Compiler.
  Evidence copies every anchor the verified plan carries, including an `HDCV0-023` disqualified
  anchor kept as review context on an `UNKNOWN` claim or beside a qualifying anchor; only the admitted
  claim state records qualification. Compilation MUST NOT refuse a verified plan for such an anchor,
  and checks an evidence member only as UTF-8 within the 64 KiB text bound (decision 0268).
- `CATN-V0-015`: The truth-digest input and the canonical bundle bytes MUST be exactly the
  `HDCV0-041` canonical encoding of the truth corpus and the bundle, produced by the
  `internal/doccompiler` canonical emitter rather than a second encoder; the truth digest hashes
  the domain prefix and the truth bytes without their one trailing LF (decision 0227).

## Failure model

Compilation uses one primary error code: `INVALID_PLAN`, `INVALID_INPUT`, `LIMIT_EXCEEDED`,
`NON_EQUIVALENT`, or `INTERNAL_ERROR`. Verification returns deterministic divergences, unique and
sorted by canonical audience rank (unscoped first, invalid audiences last), then audience spelling,
claim ID, and code, from:
`INVALID_PLAN`, `INVALID_PROFILE`, `INVALID_TRUTH`, `PLAN_BINDING_MISMATCH`,
`PLAN_CLAIM_MISMATCH`, `MISSING_PLAN_CLAIM`, `UNKNOWN_PLAN_CLAIM`,
`TRUTH_DIGEST_MISMATCH`, `MISSING_AUDIENCE`,
`DUPLICATE_AUDIENCE`, `INVALID_AUDIENCE`, `INVALID_DETAIL`, `POLICY_SCOPE_MISMATCH`,
`MISSING_CLAIM`, `DUPLICATE_CLAIM`, `UNKNOWN_CLAIM`, `INVALID_CURRENCY`,
`MISSING_CURRENCY_REASON`, `INVALID_REVIEW_STATE`, `NONCANONICAL_ORDER`, and `LIMIT_EXCEEDED`.

No failure may return a partial apparently usable bundle. Verification failure cannot be converted
to PASS by changing order, detail, or audience.

## Safety, privacy, and accessibility

- `SECURITY` is not a privileged view. A separate security authorization boundary must not be
  inferred from this audience label.
- Claim existence, IDs, ordering, limitations, and evidence paths can disclose sensitive facts.
  Only already-authorized structured claims enter a disclosure scope; hidden data cannot influence
  public ordering.
- V0 permits no audience-specific fluent text because it could introduce an unverified fact.
- A future renderer must expose literal state, currency, frontier, conflict, limitation, revision,
  and citation labels in every view. Color, icons, collapsed sections, hover, or ordering alone are
  insufficient. Keyboard navigation, semantic headings, stable anchors, and assistive-technology
  trials are separate promotion evidence.
- Translation is outside V0 because translation may change the proposition.

## Acceptance evidence

Focused tests MUST demonstrate mixed states/currencies, five exact permutations, caller-input
non-mutation, repeatable bytes, sorted unique divergences across distinct invalid audiences, empty
valid truth, tampered plan or patch, missing index, experimental `PatchPlan` bytes, missing/duplicate/unknown claims, missing/duplicate audience,
altered conflict/state/currency/limitation/evidence, `CONFLICTED` claims with zero, one, or co-located anchors, a disqualified review-context anchor on `UNKNOWN` and `SUPPORTED` claims, disclosure scope mismatch, unknown enums, noncanonical ordering, and boundary cases. A hostile fixture MUST
change a plan-derived truth field, recompute the truth digest and every view binding, and still fail
verification against the unchanged source plan. A second hostile fixture MUST construct an
otherwise consistent canonical bundle larger than 64 MiB without storing a persistent fixture and
prove that direct `Verify` returns `LIMIT_EXCEEDED`. A production-import audit
MUST show no route to filesystem, subprocess, network, or HDC execution functions.

The synthetic benchmark uses 1,000 mixed claims and all five audiences. It reports compilation and
verification allocation and latency only; it does not prove usefulness.

Product promotion requires a separately preregistered, sealed 100-module/50-change paired trial
against one generic-document baseline: zero accepted divergence mutation, byte-identical repeat,
100% disclosure-scope binding, at least 30% lower median time to a correct role-specific action, at
least 25% fewer wrong actions, no correctness or accessibility regression, and independent
replication. One hidden conflict, stale/unknown state, limitation, or unauthorized claim kills the
promotion claim.

## Rollout, non-goals, and rollback

P0 lands only the typed pure compiler, verifier, canonical bytes, focused tests, and synthetic
benchmark. It does not add CLI routing, HDC plan/receipt fields, Markdown, MkDocs, navigation,
website/editor serving, role-based access, localization, repository writes, publication, or product
promotion.

A later `/1` integration requires an accepted contract for paragraph identity, HDC lifecycle
metadata, renderer visibility, accessibility, and independent surface equivalence. It cannot silently
change this `/0` profile.

Rollback removes `internal/docviews` and stops producing new bundles. It does not modify HDC plans,
claims, documentation, Git state, or historical evidence. Failed benchmark and verification
artifacts remain failed evidence.

## Simpler baseline, compatibility, and drift

The simpler baseline is to present one validated HDC plan without compiling audience projections.
It provides no audience-specific ordering, but it also introduces no second representation that can
diverge. V0 is an isolated library profile: it changes no HDC, CEM, OCM, CLI, harness, repository,
or published-document wire. A change to the consumed HDC plan shape, the five closed audience
identities, canonical encoding, limit policy, or disclosure binding requires an explicit new profile
or an amendment to this proposed spec with regenerated vectors. Unknown fields and profile drift
fail closed rather than being ignored.

Decision 0227 amended the canonical encoding to `HDCV0-041` bytes on 2026-09-13 (`CATN-V0-015`).
It is a clean break with no compatibility reader: a truth digest or bundle digest computed earlier
over `encoding/json` bytes, which escaped U+2028 and U+2029, does not verify when its content holds
one of those code points; content without them keeps its bytes. No bundle has been released.

Decision 0240 moved the source plan from the experimental `PatchPlan` to the admitted plan on
2026-09-13 (`CATN-V0-001`, `003`, `009`, `014`). It is a clean break with no compatibility reader:
`Compile` and `Verify` take the plan bytes, proposal patch, and index; the truth claim members are
now state, kind, scope, frontier, and anchor evidence (status, uncertainty, and per-anchor revision
are gone); and the fixed truth and bundle digest vectors in `internal/docviews/compile_test.go` were
regenerated.

Decision 0268 stopped re-qualifying anchor identity on 2026-09-13 (`CATN-V0-014`). A verified plan
whose clause keeps a disqualified anchor (for example a `mutable` blob demoted to `UNKNOWN`) was
refused as `INVALID_PLAN`; it now compiles with the anchor copied and the admitted state unchanged.
No wire shape, digest domain, or fixed vector changes; every bundle that compiled before compiles to
the same bytes.

Unresolved decisions are the future paragraph-identity model, renderer visibility contract,
accessibility evidence, disclosure authorization boundary, and whether any audience projection is
useful in a sealed outcome trial. Until those decisions and the promotion gate above close, V0
remains experimental and unselectable.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| `CATN-V0-001..004` | `internal/docviews/compile.go`, `types.go` | plan binding, truth preservation (`TestCompilePreservesARepeatedPlanAnchor`), overlay, and canonical-input tests |
| `CATN-V0-005..009` | `internal/docviews/compile.go`, `verify.go` | five-view closure, hostile mutation, exact-source verification, determinism, and caller-nonmutation tests |
| `CATN-V0-010..011` | isolated package boundary | disclosure-scope assertions and production-import audit |
| `CATN-V0-012` | `internal/docviews/compile.go`, `verify.go` | direct compile/verify limit tests and canonical-byte boundary regression |
| `CATN-V0-013` | fixed experimental labels and package isolation | repository diff; outcome, accessibility, disclosure, and independent-replication gates `NOT_RUN` |
| `CATN-V0-014` | `internal/docviews/compile.go`, `verify.go` | `TestConflictedClaimRequiresTwoDistinctEvidenceLocations`, `TestCompileKeepsDisqualifiedAnchorsAsReviewContext` (decision 0268) |
| `CATN-V0-015` | `internal/docviews/compile.go`, `verify.go`, `internal/doccompiler/canonical.go` | `TestTruthDigestAndBundleHashHDCV0041BytesWithoutHTMLOrLineSeparatorEscapes` |
