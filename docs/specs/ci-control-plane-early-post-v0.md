# CI Control Plane Early Post V0

Owner: Russell Lewis
Date: 2026-11-08
Requirement prefix: `CCEP-V0`
Intent status: proposed (owner instruction 2026-11-08: ci related tickets should be fixed asap, always; V1-5102)
Delivery status: experimental

## Agent digest
- Claim: Post the required CI control-plane status on trusted PR activity without waiting for CI completion.
- Status: proposed (owner instruction 2026-11-08: ci related tickets should be fixed asap, always; V1-5102)/experimental
- Exists: `.github/workflows/ci-control-plane.yml` and `tools/ci-reuse-plan/control_plane_workflow_test.go`.
- Blocked on: hosted qualification; local tests use API fixtures and perform no network writes.
- Read next: Requirements; Acceptance evidence; Rollback.

## Requirements

- `CCEP-V0-001`: (proposed, owner instruction 2026-11-08; V1-5102; amends AFP-V0-016/079/855
  and the trigger restriction of decision 0320; preserves decision 0390's required checks)
  The trusted `.github/workflows/ci-control-plane.yml` SHALL start its metadata-only
  `control-plane` job on `pull_request_target` activity `opened`, `synchronize`, `reopened`
  and `ready_for_review` targeting `main`, without waiting for any CI job or conclusion. The
  job MUST never check out or execute PR code, use actions, or interpolate event values into
  `run:`; event inputs enter only through environment variables. Its only permissions SHALL
  be `contents: read`, `pull-requests: read` and `statuses: write`.

  The job SHALL select the event's exact PR head SHA, read the PR through REST, require that
  it remains open at that head before listing files, and read every files page including
  renamed-from paths. The REST files endpoint is PR-number-addressed, not SHA-addressed:
  the job SHALL fence that listing with head/base checks, require the returned file count to
  equal `changed_files`, require full 100-file nonfinal pages and the exact final-page
  count, reject empty or duplicate filenames, and refuse a count at or above 3000, missing
  file metadata, or any
  failed API call without posting any status. Missing inputs or malformed event identity,
  PR number, repository or head SHA MUST post nothing; only `workflow_run`
  may use the PR lookup fallback. A refused listing MUST NOT be overridden by
  a waiver. Immediately before posting on the captured head SHA, it SHALL re-read the PR and
  require the same open state, head SHA, base SHA and changed-file count. A mismatch posts
  nothing. This is revalidation, not an atomic GitHub snapshot or compare-and-swap guarantee.

  The existing `workflow_run` completed-CI path SHALL remain a fallback and rerun path,
  including AFP-V0-855's cancelled/superseded admission rule and the V1-5096/AFP-V0-171
  approver-only `ci-control-plane-waiver` lookup on the exact head. Both PR triggers SHALL
  share the `ci-control-plane-pull_request-<head sha>` concurrency group; the merge-group job
  and its status semantics SHALL remain unchanged. A clean complete listing posts success;
  a changed or renamed-from `.github/` path posts failure unless the newest authorized waiver
  on that head is success. No CI conclusion grants this status.

## Non-goals

Greptile re-review, auto-merge arming, ruleset changes, hosted writes and executing PR code.
This companion proposes amendments to AFP-V0-016/079/855 and the trigger restriction of
decision 0320; it preserves decision 0390's required checks and AFP-V0-171's waiver policy.
It does not add obligations to `affected-plan-v0.md`.

## Failure modes

API errors, missing inputs, incomplete/malformed pagination, duplicate filenames and moving
heads abstain. A write failure fails the job. A waiver cannot override unreadable file evidence.
GitHub offers no atomic transaction spanning PR revalidation and the status POST; the POST
always targets the captured head. Event suppression and runner latency can delay the early
path, so the completed-CI fallback remains.

## Acceptance evidence

| Requirement | Implementation | Evidence |
| --- | --- | --- |
| CCEP-V0-001 | `.github/workflows/ci-control-plane.yml` metadata-only job | `TestV15102ControlPlaneStartsOnPRActivity`, `TestV15102ControlPlaneMetadataBoundToHead`, `TestV15096ControlPlaneWaiverNeedsAnApprover`, `TestAFPV0079ControlPlaneReportsEveryCompletedRun`, `TestAFPV0079CancelledRunStillPostsTheStatus`, `TestAFPV0855SupersededCancelledRunStartsNoJob`; `actionlint`; `docs/build-log/2026-11-08-ci-control-plane-early-post.md`; hosted execution and timing NOT_OBSERVED |

The tests execute the actual workflow shell against API fixtures, proving a complete fixture
before injecting failures and asserting the API call trace and absence of status writes.
They cover both triggers, valid multiple pages, short pages, the 3000-file ceiling, missing
inputs/metadata, duplicate names, stale heads/base/count, renamed paths, API failures and
existing authorized/unauthorized waiver behavior.

## Rollback

Revert the PR trigger and shared metadata changes to restore the completed-CI-only path;
leave required checks, rulesets and waiver approvers intact. Revert this companion and its
registration with the implementation if the proposed amendment is withdrawn.
