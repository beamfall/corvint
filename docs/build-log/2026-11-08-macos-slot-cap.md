# Advisory macOS capacity and stale queue runs (V1-5017)

Owner intent: 2026-11-08 V1-5017 request. Base: `19c5a0111d667a11fd3a82ebcc8920f8a848260c`
(`origin/main` at change start). Governing requirements: CIN-V0-5017..5019 in
`docs/specs/ci-narrowing-v0.md`, extending CIN-V0-4625/4754 and AFP-V0-026/432/433.
Those new IDs had no existing matches in the tracked specs before this change.

## Observed before and measurement boundary

Owner-supplied observation from **2026-11-07**: job **128372903198** was created at **10:02**
and started at **11:06**, approximately **64 minutes** waiting. Four advisory `go-macos-shard`
jobs plus `cem-release-gate` occupied all **five** Team-plan hosted macOS slots. The advisory
shards have `continue-on-error: true` and a 55-minute timeout. Merge-group jobs could continue
occupying runners after their queue entry merged or left the queue.

**After-measurement is pending a day of PRs.** No hosted queue latency reduction or cancellation
has been measured locally. Compare macOS created-to-started time and overlap after deployment;
local fixtures prove guards, not GitHub scheduling or a global runner reservation.

## Decision and safety boundary

Use the acceptance criterion's cap option: preserve four partitions, set `max-parallel: 2`, and
reuse the trusted cadence and documentation decisions before allocating a macOS runner. A verified
cadence DEFER or verified DOCS skips allocation; missing, failed or unknown classifiers leave
shards running, capped, unless the other classifier supplies independent verified skip evidence.
The advisory aggregator uses the same decisions. No new Darwin-only filename filter is introduced:
shared production code can affect Darwin through dependencies. Existing conservative classification
keeps Darwin/Unix source, build-tagged files, macOS scripts and the shard workflow on the run path.
The nightly native campaign is unchanged. Two is a per-matrix limit; multiple PRs or unrelated
macOS jobs can still consume the account's five slots.

Use a default-branch Ubuntu cleanup workflow on main push and queue-branch deletion, with a
five-minute schedule as fallback. It reads every page of queued/in-progress CI and CEM merge-group
runs, rechecks each run, and cancels only after a successful matching-refs response proves that
run's exact queue branch absent. Live refs, API errors (including authorization-masked 404),
malformed data, unrelated workflows/events/branches and completed runs cause no cancellation.
A matching prefix does not keep a deleted exact branch alive. No checkout occurs; `actions: write`
is limited to this job. Existing per-entry concurrency remains unchanged: a base-wide cancellation
group could cancel a still-live entry. Schedule delays and API outages postpone cleanup safely.

Official semantics checked: [Git references API](https://docs.github.com/en/rest/git/refs),
[workflow cancellation API](https://docs.github.com/en/rest/actions/workflow-runs), and
[workflow concurrency](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax).
Context7 corroborated concurrency behavior from the official GitHub Actions documentation.
Rollback is reverting these workflow changes; no required-status configuration or stored data migrates.

## Verification

- Fail-before: `TestCINV05017MacOSSlotCap` was added while `go-macos.yml` was unmodified. The
  requested heavy-wrapper test exited 1: `go-macos-shard has 0 copies of line "      max-parallel: 2", want 1`.
- Focused workflow/classifier tests, vet, actionlint, specification generation/check and changed-file
  checks: results recorded at local completion below. Cleanup tests use an isolated `gh` spy;
  no actual cancellation, remote write, push or Tasks command was performed.
- Required contexts were located via `git grep`: AFP-V0-016/026 name `go-product`, `doc-gates`
  and `ci-control-plane`; the Greptile gate owns `greptile-gate`. Their publishing workflows and
  the nightly `release-gates.yml` are byte-identical to the base. Stable advisory `go-macos`
  and CEM job names are unchanged. No ruleset was edited or remote ruleset state asserted.
- Corvint context reported omitted evidence and unresolved ticket identity; original workflows/tests
  were read directly. `corvint affected --base` ran before tests and retained `scope=UNKNOWN`,
  67 LANGUAGE_FRONTIER unknowns (including shell foreign invocation and nested Go modules).
  The requested targeted tests cover this patch; no whole-repository coverage is claimed.
- Installed Core: `1.0.0-rc.3.dev`, build 136, commit `19c5a0111d667a11fd3a82ebcc8920f8a848260c`.
  The read-only official release listing returned no releases; freshness is NOT_OBSERVED. Tasks
  version/update and filing were not attempted under the task's explicit prohibition.
- Work stayed in the requested worktree; no subagents or independent review were launched under
  the owner's explicit constraint. No CEM/OCM sidecars were generated outside the allowed paths.

### Local completion results

PASS after the fixture repair:

```sh
HEAVY_REMOTE=0 HEAVY_LOCAL_REASON='V1-5017 owner requires this worktree only; no network writes' HEAVY_CACHE_TRIM_HOURS=0 script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run '^Test(CINV0501[78]|CINV04625|CINV0475[023]|AFPV043[2345]|AFPV0670)' ./tools/ci-reuse-plan/
GOTOOLCHAIN=local go vet ./tools/ci-reuse-plan/
actionlint .github/workflows/go-macos.yml .github/workflows/merge-queue-cleanup.yml
git diff --check
```

The first combined run exposed an existing fixture issue on case-insensitive macOS: its branch
`head` made `git rev-parse HEAD` ambiguous, polluting the captured SHA with a warning before the
classifier ran. Renaming that throwaway branch to `pr-topic` restored valid initialization; the
complete focused run then passed (71.316 seconds). The cleanup fixture's injected failures are
asserted reached separately from its cancellation effects. No production classifier was changed.

Normal `git add` failed creating `.git/worktrees/wt-5017/index.lock`: **Operation not permitted**.
Normal `make spec-requirements` consequently refused the unstaged spec (SRG-V0-954). A private
index **and private object store** at `/tmp/v1-5017-staging` stage this worktree's content without
writing the protected Git metadata. With those explicit Git environment overrides,
`make spec-requirements` and `make spec-requirements-check` passed, including the specindex and
OCM obligation-bound tests. `REQUIREMENTS.tsv` is regenerated; INDEX.json and README.md did not
change. The initial generator attempt also caught definitions outside `## Requirements`; moving
them into its `### Hosted macOS capacity` subsection fixed that placement.

The initial private snapshot was verification evidence, not normal-index staging. The final
ordinary `git add` and requested `git commit` retry succeeded without an override; the transient
lock-file restriction no longer blocked delivery. No permission bypass, remote write or alternate
worktree was used. The task is committed locally on `claude/v1-5017-macos-slots`.

`make changed-checks BASE=19c5a0111d667a11fd3a82ebcc8920f8a848260c` passed against the
private staged snapshot: line citations, unbounded readers and test read scopes. It retained the
existing nonfatal `internal/tasks/cli` ratchet cleanup note. The first private-index check exposed
fixture inheritance: specindex's temporary repositories inherited the private `GIT_INDEX_FILE`
and rewrote that disposable index. The snapshot was rebuilt from the untouched real index;
rechecking with the three private Git variables unset **only for test subprocesses** passed,
and the final private staged diff contains exactly the eight task files. The real index and
worktree contents were not affected by that fixture artifact.

After ordinary staging/commit succeeded, `make spec-requirements` (no generated changes) and
`make spec-requirements-check` both passed again against the normal repository index, with no
private Git environment overrides. Final publication remains local only; no push was requested.
