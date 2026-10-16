# CI least-privilege check covers every workflow (V1-1469, ARTIFACT-V0-008)

Date: 2026-10-16. Ticket: V1-1469. Requirement: `ARTIFACT-V0-008` in
`docs/specs/release-artifact-integrity-v0.md`, amended; the disposition was owner delegated to the
orchestrator (2026-10-16), and its decision record is written by the orchestrator-dispositions lane.

## Problem

`script/check-ci-least-privilege.sh` read only `.github/workflows/ci.yml`. The one job holding
`issues: write` (`release-gates.yml` job `nightly-failure-signal`, SOP-V0-013) and the
`ci-control-plane.yml` status writer were outside any least-privilege check; only
`TestSOPV0013WorkflowBinding` pinned the former.

## Contract

- The check reads every tracked `.github/workflows/*.yml` (and `*.yaml`) from the Git index.
- Workflow-level and job-level `permissions:` blocks must be exactly `contents: read` (job-level
  `{}` stays allowed) unless `script/ci-least-privilege-exceptions.tsv` declares that block
  (`workflow` or `job:<id>`) with its exact permission set and a reason. A block that differs from
  its declared set fails, and a row that matches no block fails as stale.
- Declared exceptions: `release-gates.yml` `job:nightly-failure-signal`
  (`actions:read contents:read issues:write`), `ci-control-plane.yml` `workflow`
  (`contents:read pull-requests:read statuses:write`) and `ci-control-plane.yml`
  `job:forward-rows` (`actions:read contents:read`). No workflow file changed.
- The persist-credentials and full-SHA pin checks now also apply to every workflow; all passed
  without change.

## Evidence

- Before the exception list, the widened check failed on exactly those three blocks.
- Mutations staged on the real tree: workflow-level `issues: write` in `release-gates.yml` fails
  (`release-gates.yml:35`); an undeclared job-level `contents: write` on `intake-containment.yml`
  `containment` fails. Both restored; the check then passes.
- `script/check-ci-least-privilege_test.sh`: 12 cases pass (7 new: a non-`ci.yml` workflow-level
  write, an undeclared job-level write, a job widened past its exception, a stale row, and the
  three review findings below).

## Independent review

One Codex round (gpt-6-astra, read-only) found three parser bypasses, all reproduced and fixed
with a regression case each: a blank line ended the workflow-level block early, so a later
`issues: write` passed (pre-existing); a quoted `"permissions":` key was ignored (pre-existing);
and a quoted job id kept the previous job's name, so it could consume that job's exception (new).
The spec now states that the check is line-oriented and fails closed on blocks it cannot name.

## Rollback

Revert the change; the check returns to `ci.yml` only and the exception file is unused.
