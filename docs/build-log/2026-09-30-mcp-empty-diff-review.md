# MCP empty-diff review repair — 2026-09-30

## Observed failure and cause

CI run 36726094223 at d7749792 passed documentation, interop, artifact integrity
and product shards 0, 1 and 2. Shard 3 failed only
`TestTaskReviewCEMReportNeverRunsPlantedGit`. A local reproduction confirmed that
no planted Git marker was created: the new review projection rejected an empty
canonical base-to-target diff with `binary-patch`, which the bridge sanitized
as `internal-error`. The deliberately invalid fixture still requires a report.

## Contract and bounded repair

MCPV0-025 requires CLI/preview parity, no publication and start-time Git pinning;
CEM-PILOT-006 and CEM-PILOT-028 require the complete derived-hunk denominator
and explicit surplus map hunks. A successfully derived canonical diff of exactly
zero bytes now has an empty review denominator. Verification remains false,
surplus map hunks remain visible, and the exact empty-patch and record digests
remain bound. Shared parsing, verification, legacy input, Git pinning and error
handling are unchanged. This is a repair of the experimental projection.

## Acceptance evidence and rollback

The new focused workflow regression first failed with `binary-patch: patch is
empty`. The actual MCP process test first failed after an explicit absent-marker
check. Gate A admitted the canonical-only exception. Focused package/process
checks, independent source review, final change evidence and required CI are
retained in the manager directory and owning native ticket V1-0529 as they finish.
No passing final CI, merge or native completion is inferred from these local
checks. Revert the bounded projection exception to roll back; historical CEMs
and traces remain intact. The six workflows remain proposed/experimental.
