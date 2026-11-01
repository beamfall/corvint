# Beamfall Shadow Dogfood V1

Owner: Russell Lewis
Date: 2026-08-26
Intent status: superseded by `docs/specs/analyzer-capability-contract-v0.md`
Delivery status: superseded
Authoritative successor: `docs/specs/analyzer-capability-contract-v0.md`

## Agent digest
- Successor: `analyzer-capability-contract-v0.md`.
- Why: the fixture/receipt evidence is retained, while analyzer selection, launch, admission, and support authority moved to the successor contract.

## Scope

This private harness observes a selected external artifact. It has no Core
registry, selection, launch, admission, verifier, or support authority.

## Requirements

- `BSD-001`: `beamfall-shadow-dogfood/2` is a canonical, language-neutral
  outer manifest. It selects exactly one closed registry row: family, language,
  framework, toolchain, version, frozen-candidate family mapping, and input
  family. Atom-valid cross-products fail closed before the artifact opens.
- `BSD-002`: `corvint-analyzer-candidate/experimental` remains the exact frozen
  request and response grammar. Its request `family` is only `go`,
  `javascript-typescript`, `dotnet`, or `ruby`; no plugin, manifest, artifact,
  Core, or broad-matrix field enters it. A registry row without a mapping emits
  a bound `UNMAPPED_FROZEN_ENVELOPE` `NOT_RUN` receipt and starts nothing.
- `BSD-003`: Every run response and terminal receipt binds plugin ID, release,
  manifest digest, declared version, artifact digest, harness executable digest
  (never a running-Core claim),
  frozen request digest, response/stderr digests, run kind, and outcome.
- `BSD-004`: Manifest and artifact reads use descriptor-bound nofollow regular
  files. Artifact staging performs one context-checked bounded copy-and-hash
  pass, retains that staged descriptor for execution, and rejects links,
  drift, overflow, cancellation, or cleanup failure.
- `BSD-005`: A new private `0600` receipt is emitted for every outcome after a
  usable receipt path is supplied. It records bounded reason/outcome, wall/RSS,
  identities, process and directory cleanup state; cleanup errors are terminal.
- `BSD-006`: Literal external fixture artifacts and source inputs exist for Go;
  TS/JS/React; HTML/CSS; Swift/Objective-C/C/Metal; Kotlin/Java/JNI/GLSL; SQL;
  shell; Python tooling; structured/web assets; Ruby; and .NET. Ruby and .NET
  remain external-dogfood tuples. Fixtures are not support or analyzer claims.
- `BSD-007`: The one-pass artifact byte count and non-race allocation ceiling
  are causal regression checks. Wall time is diagnostic only because isolation
  has not been proven.

## Non-goals and gates

There is no runtime qualification, semantic PASS, network denial proof, or
load-sensitive latency claim. Full gate and integration are `NOT_RUN` by owner
direction; the separate runtime-parent repair remains unmerged and unrebasable.

| Requirement | Evidence |
|---|---|
| BSD-001, BSD-002 | `TestFrozenRequestIsByteExactAndHasOnlyFourFamilies`, `TestManifestRegistryRejectsCrossProductsAndNofollow` |
| BSD-003, BSD-005 | `TestBoundReceiptContainsEveryIdentityAndFailure`, `TestUnmappedTupleProducesBoundedNotRunReceipt`, `TestDeterministicReplayKeepsBoundIdentities` |
| BSD-004, BSD-007 | `TestArtifactPassCausalIORatchet`, `TestTimeoutReapsDescendantAndReceiptsCleanup` |
| BSD-006 | `TestMatrixFixturesAreLiteralDistinctExternalArtifacts` |
