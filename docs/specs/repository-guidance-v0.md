# Repository guidance V0

Owner: Russell Lewis
Date: 2026-09-13
Requirement prefix: `RGV-V0`
Intent status: accepted (owner first-release instruction, 2026-09-13)
Delivery status: experimental

## Agent digest
- Claim: Three distinct advisory commands discover inferred features, compose a repository overview, and review an immutable range with local branch overlap hints.
- Status: accepted (owner first-release instruction, 2026-09-13); experimental.
- Exists: Genesis immutable inventory and affected-plan composition; guidance command implementation in this slice.
- Blocked on: integrated first-release qualification; no authority or coverage promotion.
- Read next: Requirements; Acceptance and rollback.

## Intent and non-goals

The owner requested the CodeSage-inspired feature-discovery, overview, and review ideas in the
first release. This is deterministic local composition, not model inference, embeddings, SCIP,
Serena integration, remote branch discovery, benchmark superiority, or accepted feature authority.
The existing explicit-ID `feature FEATURE_ID` command retains its grammar and semantics.

## Requirements

- **RGV-V0-001:** Separate experimental `features`, `overview`, and `review --base FULL_SHA [--max-refs N]` commands MUST emit canonical JSON. All successful receipts label authority inferred and mutates false. `features` omits overview-only languages/entrypoints/manifests/tests fields with an explicit omission explanation; `overview` includes them.
- **RGV-V0-002:** Read commands MUST capture clean HEAD, immutable commit/tree/blob inputs and recheck HEAD/status before emission. Dirty worktrees and drift MUST refuse with `unsupported-repository-guidance`; malformed arguments refuse without observation, trace, index or repository writes.
- **RGV-V0-003:** Discovery MUST reuse Genesis tracked inventory/classification. Candidates MUST carry original path/blob/line span and deterministic rule ID, merging duplicate labels with all bounded evidence. Inference MUST NOT synthesize authority, feature status or accepted IDs.
- **RGV-V0-004:** Bounded heuristics MUST cover literal feature/scenario markers, route/MCP registrations, cmd entrypoints and package/application manifests including literal bin/script declarations. Unsupported dynamic syntax, languages and exhausted source/candidate bounds MUST remain explicit unknowns/omissions.
- **RGV-V0-005:** Overview MUST compose exact commit/tree, detected languages, entrypoints/manifests, test conventions/runners, inferred features and index freshness/omissions. Optional index data MUST match the captured tree; missing/stale/unread index data MUST be explicit and MUST NOT trigger index building.
- **RGV-V0-006:** Suggested nextCalls MUST be bounded inert structured argv from a closed supported-command table. Repository strings MUST NOT supply arbitrary argv. Suggestions MUST NOT execute scripts, tests, commands or models.
- **RGV-V0-007:** Review MUST require a full immutable ancestor base and compose actual affected-plan/0 advice over immutable target sources plus changed inferred scope. Omitted sources MUST remain unknown; target-only changedFeatures MUST explicitly say deleted/base-only candidates are not discovered; guidance MUST NOT close CEM, OCM, frontier or test obligations.
- **RGV-V0-008:** Review MUST enumerate only sorted local refs/heads, capture the full bounded tip map and recheck it before emission. Ref drift MUST refuse. Default/max admission is 32 refs; caller may lower it to 1..32 with explicit omitted refs.
- **RGV-V0-009:** Review MUST explicitly skip current ref, ancestors of target, descendants of target and ancestry-stacked surviving tips. Remaining refs use merge-base(inputBase,tip)..tip and intersect the complete bounded path set with inputBase..target.
- **RGV-V0-010:** More than 256 paths/ref MUST produce UNKNOWN/incomplete, never a truncated negative. At most 64 overlapping path entries across all branch rows are emitted with explicit omissions (branch rows are already bounded by the ref cap). Ref enumeration, target paths, source blobs, candidates and output MUST have finite budgets.
- **RGV-V0-011:** Git subprocesses MUST use trust-isolated existing adapters, no replacement/graft-sensitive ancestry, a shared deadline and contained cleanup. Branch names are data, never shell fragments. Snapshot scratch materialization MUST be removed before successful return; cleanup failure MUST refuse success, identify the scratch path and preserve any original operation error; no child scripts execute.
- **RGV-V0-012:** Real committed fixtures MUST verify immutable evidence, deterministic bytes, overlap/disjoint/stacked branches, caps, drift/refusals and unchanged repository/ledger state. Promotion beyond experimental requires separately accepted evidence.

## Bounds and failure modes

One invocation has a 60-second global deadline, 2,048 contained Git operations, 10-second per-Git
limit, 64 MiB aggregate Git output, 8 MiB aggregate source blobs, 256 KiB per source, 4,096 inventory
entries, 256 candidates, 16 evidence rows/candidate, 1,024 local refs in the full map, 16,384 target
paths and 1 MiB final JSON. Overlarge immutable output or unsafe layout refuses; source omissions
remain unknown. Unsupported syntax has no completeness claim. Index fields are currently omitted
with state UNKNOWN/not-read. `unsupported-repository-guidance` includes dirty, drift, ancestry,
layout, source, deadline and output refusal reasons. Rollback removes the three command routes and
this advisory composition without changing explicit feature authority or existing gates.

Skip reasons include `current-ref`, `ancestor-of-target`, `descendant-of-target`,
`ancestry-stacked`, `ref-budget` and explicit UNKNOWN ancestry. The captured symbolic current ref
is also rechecked, including switches between refs sharing one commit. Overlap states are
`OVERLAP`, `OVERLAP_INCOMPLETE`, `DISJOINT`, and `UNKNOWN`.

## Acceptance and rollback

All guidance tests below live in `cmd/corvint/repository_guidance_test.go`.
Genesis Git containment retains the existing `TestDescendantCleanupOnCancellation` and
`TestDescendantCleanupOnTimeout` regressions in `internal/cem/gitrun/gitrun_unix_test.go`.

| Requirement | Test |
|---|---|
| RGV-V0-001 | TestRepositoryGuidanceCLIAndLiteralRegistration |
| RGV-V0-002 | TestRepositoryGuidanceRefusalsDoNotObserve; TestRepositoryGuidanceRefBoundsAndDrift |
| RGV-V0-003 | TestRepositoryGuidanceImmutableDiscovery |
| RGV-V0-004 | TestRepositoryGuidanceCLIAndLiteralRegistration; TestRepositoryGuidanceCandidateAndOverlapBounds |
| RGV-V0-005 | TestRepositoryGuidanceImmutableDiscovery |
| RGV-V0-006 | TestRepositoryGuidanceCLIAndLiteralRegistration |
| RGV-V0-007 | TestRepositoryGuidanceReviewBranches; TestRepositoryGuidanceDeletedFeatureIsExplicitUnknown |
| RGV-V0-008 | TestRepositoryGuidanceRefBoundsAndDrift; TestRepositoryGuidanceCandidateAndOverlapBounds |
| RGV-V0-009 | TestRepositoryGuidanceReviewBranches |
| RGV-V0-010 | TestRepositoryGuidanceCandidateAndOverlapBounds; TestRepositoryGuidanceRefBoundsAndDrift |
| RGV-V0-011 | TestRepositoryGuidanceAncestryTrustAndDeadline; TestRepositoryGuidanceCleanupFailureRefuses |
| RGV-V0-012 | TestRepositoryGuidanceImmutableDiscovery; TestRepositoryGuidanceRefusalsDoNotObserve |

Focused results and independent review are recorded in BUILD-LOG. Root owns final frozen full gate,
CEM/OCM and first-release integration; focused results alone do not assert promotion.
