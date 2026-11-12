# Repository guidance V0

Owner: Russell Lewis
Date: 2026-09-13
Requirement prefix: `RGV-V0`
Intent status: accepted (owner first-release instruction, 2026-09-13); RGV-V0-015 proposed (V1-0995); RGV-V0-050 accepted under the owner's standing auto-accept (2026-10-11; V1-2053)
Delivery status: experimental

## Agent digest
- Claim: One advisory command, `review`, gives affected advice and local branch overlap hints for an immutable range (`features` and `overview` cut, decision 0597).
- Status: accepted (owner first-release instruction, 2026-09-13); RGV-V0-015 proposed (V1-0995); RGV-V0-050 accepted under the owner's standing auto-accept (2026-10-11; V1-2053); experimental.
- Exists: Genesis immutable inventory and affected-plan composition; guidance command implementation in this slice.
- Blocked on: integrated first-release qualification; no authority or coverage promotion.
- Read next: Requirements; Acceptance and rollback.

## Intent and non-goals

The owner requested the CodeSage-inspired feature-discovery, overview, and review ideas in the
first release. This is deterministic local composition, not model inference, embeddings, SCIP,
Serena integration, remote branch discovery, benchmark superiority, or accepted feature authority.
The existing explicit-ID `feature FEATURE_ID` command retains its grammar and semantics.

## Requirements

- **RGV-V0-001:** The experimental `review --base FULL_SHA [--max-refs N]` command MUST emit canonical JSON with authority labelled inferred and `mutates` false. The `features` and `overview` commands MUST NOT exist: a call to either gets the usage refusal and exit 2. The review document MUST NOT carry a `features` or `changedFeatures` member, languages, entrypoints, manifests or `nextCalls`. (amended 2026-11-11, decision 0597)
- **RGV-V0-002:** Read commands MUST capture clean HEAD, immutable commit/tree/blob inputs and recheck HEAD/status before emission. Dirty worktrees and drift MUST refuse with `unsupported-repository-guidance`; malformed arguments refuse without observation, trace, index or repository writes.
- **RGV-V0-003:** Retired 2026-11-11 (decision 0597): the inferred feature list is cut; review still reads the Genesis inventory for RGV-V0-013, and feature maps come from flows (decision 0596 G25). The ID stays for traceability.
- **RGV-V0-004:** Retired 2026-11-11 (decision 0597): regex feature discovery is cut. The ID stays for traceability.
- **RGV-V0-005:** Retired 2026-11-11 (decision 0597): `overview` is cut; `corvint affected` and `corvint impact` answer change questions. The ID stays for traceability.
- **RGV-V0-006:** Retired 2026-11-11 (decision 0597): only `features` and `overview` emitted `nextCalls`. The ID stays for traceability.
- **RGV-V0-007:** Review MUST require a full immutable ancestor base and compose actual affected-plan advice over immutable target sources. Omitted sources MUST remain unknown, and guidance MUST NOT close test obligations. (amended 2026-11-11, decision 0597)
- **RGV-V0-008:** Review MUST enumerate only sorted local refs/heads, capture the full bounded tip map and recheck it before emission. Ref drift MUST refuse. Default/max admission is 32 refs; caller may lower it to 1..32 with explicit omitted refs.
- **RGV-V0-009:** Review MUST explicitly skip current ref, ancestors of target, descendants of target and ancestry-stacked surviving tips. Remaining refs use merge-base(inputBase,tip)..tip and intersect the complete bounded path set with inputBase..target.
- **RGV-V0-010:** More than 256 paths/ref MUST produce UNKNOWN/incomplete, never a truncated negative. At most 64 overlapping path entries across all branch rows are emitted with explicit omissions (branch rows are already bounded by the ref cap). Ref enumeration, target paths, source blobs, candidates and output MUST have finite budgets.
- **RGV-V0-011:** Git subprocesses MUST use trust-isolated existing adapters, no replacement/graft-sensitive ancestry, a shared deadline and contained cleanup. Branch names are data, never shell fragments. Snapshot scratch materialization MUST be removed before successful return; cleanup failure MUST refuse success, identify the scratch path and preserve any original operation error; no child scripts execute.
- **RGV-V0-012:** Real committed fixtures MUST verify immutable evidence, deterministic bytes, overlap/disjoint/stacked branches, caps, drift/refusals and unchanged repository/ledger state. Promotion beyond experimental requires separately accepted evidence.
- **RGV-V0-013:** Review MUST NOT plan affected advice over an incomplete guidance inventory. When the inventory cap omits entries or any tracked source is not admitted as snapshot bytes, the embedded `review.affected` MUST be the standalone `affected --base FULL_SHA` receipt for the same root and base, planned over the clean worktree at the captured revision and byte-equal to that command's receipt; a revision other than the captured one refuses as drift, and a receipt-level unknown MUST name the substitution and the equivalent command. A complete inventory keeps the immutable snapshot plan unchanged. (Proposed, experimental; V1-5012, 2026-11-11; applies AFP-V0-9730 to review.) Because `review.affected` is this binary's plan and must equal the default `corvint affected --base FULL_SHA` document, review MUST apply the AFP-V0-846 judgement of its own binary before planning: a `self-binary-stale` binary refuses with exit 2, the error code `self-binary-stale`, the AFP-V0-846 detail and a `fix:` clause naming the rebuild (`go install ./cmd/corvint`) and the uninstalled build (`go run ./cmd/corvint review --base FULL_SHA`), writing no receipt. Review accepts no `--accept-stale-binary`: the commands its `affected` and `affectedFull` must equal refuse without it. A `self-binary-unverified` binary still reviews: `review.affected` and the planned receipt the `affectedFull` digest covers carry the `degradations` and `degradationDetails` members that command gives them, and one receipt-level `unknown` line, `review.affected was planned by a self-binary-unverified binary: ` plus the ACX-V0-750 notice, keeps it visible. A fresh binary, or one Corvint cannot judge, is unchanged. Rollback: drop the judgement in `reviewGuidance` (`cmd/corvint/repository_guidance.go`).
- **RGV-V0-014:** Review MUST emit `NO_REPOSITORY_GATE_DECLARED` only from a complete read of the repository declarations: either the complete immutable snapshot or the standalone affected path's direct worktree read. An empty selection or a gate-absence claim MUST NOT derive from a capped inventory.
- **RGV-V0-015:** (proposed (V1-0995)) Review MUST NOT embed the full `affected-plan/0` receipt. `review.affected` MUST be the AFP-V0-035/036 `affected-plan/1` projection of the receipt RGV-V0-007/013 planned, byte-equal to the default `corvint affected --base FULL_SHA` document over the same clean worktree. `review.affectedFull` MUST carry `argv` `["corvint","affected","--base",FULL_SHA,"--full"]` and `digest`, `affected-plan:sha256:` plus the lowercase hex SHA-256 of the canonical planned `affected-plan/0` receipt, which is that command's stdout without its trailing newline at the captured revision; a different digest means the worktree plan differs from the one review composed. Falsifier: a review document that carries `plan.selected[].tests` or the `plan.excluded` array, or whose `affectedFull` digest does not match the stdout of its `argv`. Rollback: embed the `affected-plan/0` receipt again and drop `affectedFull`.
- **RGV-V0-050:** (accepted under the owner's standing auto-accept, 2026-10-11; V1-2053) The RGV-V0-002 dirty-worktree refusal MUST name the dirty paths: the total count and, in `git status --porcelain=v1` order, the status code and quoted path of at most the first 8 entries, with `(+N more)` for the rest. When any entry is untracked it MUST also state that Corvint runs Git without global configuration (decision 0158), so a path ignored only by a global excludes file (`core.excludesFile` or `~/.config/git/ignore`) is untracked here, and name the fix: a tracked `.gitignore` or `.git/info/exclude`. The refusal code, its trigger and the no-observation rule are unchanged. Non-goal: honouring global excludes, and the `MIXED` worktree state of the MCP binding, whose dirty set lives in the context index. Rollback: restore the fixed refusal text.
- **RGV-V0-955:** Retired 2026-11-11 (decision 0597): `overview` and `features`, the only commands it governed, are cut; `review` still refuses every dirty state under RGV-V0-002. The ID stays for traceability.
- **RGV-V0-956:** Retired 2026-11-11 (decision 0597): `features` and `feature` are cut. The ID stays for traceability.

## Bounds and failure modes

One invocation has a 60-second global deadline, 2,048 contained Git operations, 10-second per-Git
limit, 64 MiB aggregate Git output, 8 MiB aggregate source blobs, 256 KiB per source, 4,096 inventory
entries, 256 candidates, 16 evidence rows/candidate, 1,024 local refs in the full map, 16,384 target
paths and 1 MiB final JSON. Overlarge immutable output or unsafe layout refuses; source omissions
remain unknown. These bounds limit discovery; review's affected section plans through the standalone
affected path whenever they omit inventory (RGV-V0-013), so its receipt also counts toward the 1 MiB output bound. Unsupported syntax has no completeness claim. Overview reads only an existing native snapshot and reports MATCHING with the captured commit/tree,
or UNKNOWN with the native miss reason and an index omission. A missing matching filename cannot
distinguish absent data, another tree or an unsupported engine; that combined reason stays explicit.
Corrupt or unsupported bytes, unsafe layouts and unreadable stores remain UNKNOWN. No index is built.

Markers use whole-line `feature:` or `scenario:` syntax, optionally in `//`, `#`, `--`, `/* */`,
`*` or `<!-- -->` comments. Labels start with a letter or number and contain at most 120 characters
from letters, numbers, spaces, underscore, dot, slash, parentheses and hyphen (also at most 120 bytes).
Go markers come only from comment tokens; string literals cannot become markers. JSON/JSONL markers
are excluded. Other formats retain this bounded heuristic, with no syntax-completeness claim.
C++ suffixes include `.cpp`, `.cc`, `.cxx` and `.hpp`; `.h` is an explicit ambiguous-language omission. `unsupported-repository-guidance` includes dirty, drift, ancestry,
layout, source, deadline and output refusal reasons. Rollback removes the `review` route (the
`features` and `overview` routes were cut by decision 0597) and this advisory composition without changing explicit feature authority or existing gates.

Skip reasons include `current-ref`, `ancestor-of-target`, `descendant-of-target`,
`ancestry-stacked`, `ref-budget` and explicit UNKNOWN ancestry. The captured symbolic current ref
is also rechecked, including switches between refs sharing one commit. Overlap states are
`OVERLAP`, `OVERLAP_INCOMPLETE`, `DISJOINT`, and `UNKNOWN`.

## Acceptance and rollback

All guidance tests below live in `cmd/corvint/repository_guidance_test.go`, except RGV-V0-050's in `internal/genesis/guidance_test.go`.
Genesis Git containment retains the existing `TestDescendantCleanupOnCancellation` and
`TestDescendantCleanupOnTimeout` regressions in `internal/gitrun/gitrun_unix_test.go`.

| Requirement | Test |
|---|---|
| RGV-V0-001 | TestRepositoryGuidanceRemovedVerbsRefuse; TestRepositoryGuidanceReviewCarriesNoInferredFeatures |
| RGV-V0-002 | TestRepositoryGuidanceRefusalsDoNotObserve; TestRepositoryGuidanceRefBoundsAndDrift |
| RGV-V0-003 | retired (decision 0597) |
| RGV-V0-004 | retired (decision 0597) |
| RGV-V0-005 | retired (decision 0597) |
| RGV-V0-006 | retired (decision 0597) |
| RGV-V0-007 | TestRepositoryGuidanceReviewBranches; TestRepositoryGuidanceDeletedPathReachesAffected |
| RGV-V0-008 | TestRepositoryGuidanceRefBoundsAndDrift; TestRepositoryGuidanceCandidateAndOverlapBounds |
| RGV-V0-009 | TestRepositoryGuidanceReviewBranches |
| RGV-V0-010 | TestRepositoryGuidanceCandidateAndOverlapBounds; TestRepositoryGuidanceRefBoundsAndDrift |
| RGV-V0-011 | TestRepositoryGuidanceAncestryTrustAndDeadline; TestRepositoryGuidanceCleanupFailureRefuses |
| RGV-V0-012 | TestRepositoryGuidanceRefusalsDoNotObserve; TestRepositoryGuidanceReviewBranches |
| RGV-V0-013 | TestRepositoryGuidanceReviewAffectedInventoryCompleteness; TestRepositoryGuidanceReviewBranches; the V1-5012 stale-binary clause: TestGuidanceReviewJudgesSelfBinaryFreshness_V15012 in `cmd/corvint/repository_guidance_stale_test.go` (stale refused with exit 2, the code, the fix: line and no opt-in, stdout empty; unverified and fresh reviews still equal the default affected document and the `--full` digest; it fails on the exit-0 review) |
| RGV-V0-014 | TestRepositoryGuidanceReviewAffectedInventoryCompleteness |
| RGV-V0-015 | TestRepositoryGuidanceReviewBranches; TestRepositoryGuidanceReviewAffectedInventoryCompleteness (both through `assertReviewAffectedResolves`, which runs the `affectedFull` argv and checks its digest) |
| RGV-V0-050 | TestGuidanceDirtyRefusalNamesPathsAndGlobalExcludes (a path ignored only by `~/.config/git/ignore`, a tracked modification without the hint, and the 8-path bound) |
| RGV-V0-955 | retired (decision 0597) |
| RGV-V0-956 | retired (decision 0597) |

Focused results and independent review are recorded in BUILD-LOG. Root owns final frozen full gate,
CEM/OCM and first-release integration; focused results alone do not assert promotion.

## Index observation labels

Overview index observations use `captured-tree` as the matching reason when both index commit
and tree equal the captured repository revision and tree.
An index-probe read error is retained as `snapshot-read-failed`; it leaves index state unknown and
does not authorize a rebuild or imply freshness.
