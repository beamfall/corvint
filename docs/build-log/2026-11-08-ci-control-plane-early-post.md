# V1-5102 — Post the CI control-plane status on PR activity

Owner intent: the 2026-11-08 request authorizes this metadata-only workflow change and local
verification, with no Tasks commands, network writes, publication, arming or subagents.
Base and initial HEAD: `19c5a0111d667a11fd3a82ebcc8920f8a848260c` (`origin/main`), branch
`claude/v1-5102-control-plane`. The worktree was initially clean. One builder performed the
implementation and direct security review; independent agent review was excluded by request.
The inherited model/effort was retained for this bounded change; live routing controls and
account usage telemetry were unavailable, so no cost or spend-enforcement claim is made.

The root cause was the `workflow_run: completed` trigger on the whole `CI` workflow. The status
uses only GitHub metadata, yet waited for advisory/non-required jobs, including 55+ minute macOS
shards. The ticket reports historical avoidable delays of 60–140 minutes; those timings are
owner-supplied context, not a new hosted measurement.

The existing `control-plane` job now also starts on trusted `pull_request_target` events for
`main`: opened, synchronize, reopened and ready_for_review. It has no CI dependency, actions or
checkout, and explicitly holds only contents/read, pull-requests/read and statuses/write. Event
values enter the shell through environment variables. The completed-CI fallback uses the same
script and per-head concurrency group; merge-group and advisory artifact jobs remain unchanged.
Existing action SHA pins are unchanged. This avoids a second copy of the waiver implementation.

The PR files REST endpoint is addressed by PR number, not SHA. Reads before pagination and
immediately before the status POST bind inspection to the event head, an open PR, its base SHA
and changed-file count. The full JSON listing must match that count and remain below 3000 files;
changed and renamed-from `.github/` paths fail unless the latest authorized exact-head waiver
succeeds. Incomplete/malformed listings, the 3000-file boundary and API failures cannot be waived
into success. A moved/closed PR posts nothing. These fences are not an atomic API snapshot;
GitHub offers no transaction joining the files listing and status POST. The POST always targets
the captured head, never an automatically substituted newer head.

V1-5096 remains supported: an approver's `ci-control-plane-waiver` success on that exact head,
followed by a workflow_run rerun, produces the workflow's own `ci-control-plane` success.
Unauthorized, withdrawn, missing and wrong-context waivers remain rejected. GRG-V0-002's carry
rule belongs to a separate PR and removes the re-review need for base merges and evidence
commits. Automated Greptile re-review is out of scope here. Arming automation stays manual:
`GITHUB_TOKEN` cannot safely arm auto-merge on the owner's behalf. No arming was implemented.

Official GitHub documentation was checked through Context7 (`/github/docs`):
[trusted pull_request_target execution](https://github.com/github/docs/blob/main/content/actions/reference/security/securely-using-pull_request_target.md),
[event behavior](https://github.com/github/docs/blob/main/content/actions/reference/workflows-and-actions/events-that-trigger-workflows.md)
and [REST files listing limits](https://docs.github.com/en/rest/pulls/pulls#list-pull-requests-files).
The first Context7 attempt hit the sandbox's npm-cache write restriction; a disposable local
cache succeeded and was removed. No dependency or package files were changed.

## Verification and retained limits

- Fail-before proof: with only the new `TestV15102ControlPlaneStartsOnPRActivity` test added,
  `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run
  '^TestV15102ControlPlaneStartsOnPRActivity$' ./tools/ci-reuse-plan/` failed (exit 1): no
  `pull_request_target` trigger, independent job condition, minimal job permissions or event-head
  inputs existed. This ran before any workflow edit.
- Focused suite: `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run
  'Test(V15102|V15096|AFPV0079|AFPV0855)' ./tools/ci-reuse-plan/` passes. The tests execute the
  actual workflow shell with JSON API fixtures, prove a successful fixture before injected
  failures, and inspect call ordering to prove the failing path and last-minute revalidation
  were reached. Both the metadata count and returned-list cap are exercised, as are multi-page
  listings, renamed-from paths, moving heads/base/count, closed PRs, failed API calls and waivers.
  Existing cancelled-run, superseded-run, merge-group and waiver tests also pass.
- `GOTOOLCHAIN=local go vet ./tools/ci-reuse-plan/`,
  `actionlint .github/workflows/ci-control-plane.yml` and `git diff --check` pass.
  ShellCheck's existing ASCII folding/single-quoted jq warnings were resolved with jq's
  `ascii_downcase` and a localized explanation of jq's `$l` variable.
- Heavy commands explicitly set `HEAVY_REMOTE_HOST=` and
  `HEAVY_LOCAL_REASON='V1-5102 worktree-only verification; no network writes'` to avoid the host's
  configured remote offload. Go is `go1.27.3`. No repository-wide gate was requested or run.
- `AFP-V0-5102` was verified unused by a scoped `git grep` before definition. The owning
  `affected-plan-v0.md` is already at the generator's 256-obligation ceiling: adding this ID
  makes 257 and `make spec-requirements` refuses. A separate section outside `## Requirements`
  also refuses, correctly; no gate was weakened. A companion spec needs a scope expansion from
  the owner. Both `make spec-requirements` and `make spec-requirements-check` failed on
  that ceiling; the generated files remain unchanged. This local commit is not gate-complete
  and must not be treated as ready to land until the spec scope is resolved.
- Normal staging fails because the sandbox cannot create
  `/Users/russelllewis/projects/corvint/.git/worktrees/wt-5102/index.lock` (`Operation not
  permitted`). A private snapshot index/object directory was used only to exercise the spec
  generator; it does not count as staging in the worktree's real index. No protected Git
  metadata was overwritten. A subsequent normal `git add` and the requested plain-message
  `git commit` succeeded; the transient index-lock failure is no longer a blocker.
- `make changed-checks BASE=19c5a0111d667a11fd3a82ebcc8920f8a848260c` passes, including
  citation, unbounded-reader and test-read-scope checks. It retains the existing advisory
  that `internal/tasks/cli` is listed as unbounded although the scanner does not find it so;
  that declaration is outside this change. The merge-group, main-miss and forward-rows job
  blocks were also compared against the baseline and are byte-for-byte unchanged.
- Hosted early timing, fork event execution and live status writes are `NOT_OBSERVED`: the
  request forbids network writes. The fallback remains for suppressed events and reruns.

Corvint routes used: `context --task` found the owning spec and workflow but retained omitted
sources and no native ticket resolution; `affected --base` ran before tests and after the
meaningful diff. The latter reported `scope=UNKNOWN`, 271 selected, 634 excluded and 67
`LANGUAGE_FRONTIER` unknowns, so it does not prove exhaustive coverage. The installed Core is
`1.0.0-rc.3.dev`, build 136 at the baseline commit. `corvint-update check --component core
--allow-network` could not prove process-group quiescence under the sandbox and reported
freshness `UNKNOWN`; available releases were not established. Tasks/version/filing and CEM/OCM
writes were excluded by the explicit command/file scope. No release was installed.

Rollback is a revert of this scoped change, restoring the completed-CI-only path without
changing required checks, rulesets, action pins or the waiver approver list.
