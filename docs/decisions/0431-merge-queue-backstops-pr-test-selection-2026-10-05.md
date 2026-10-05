# Decision 0431 — A merge queue backstops PR test selection

Date: 2026-10-05. Status: accepted (owner answer 2026-10-05: all four open questions as
recommended). Owner request 2026-10-05: CI should "only run the tests that need running".
Tickets: V1-0616 (umbrella), `ci-shadow-historical-compat`,
V1-0760. Contract: `docs/specs/affected-plan-v0.md`. Amends `AFP-V0-013`, `AFP-V0-014`,
`AFP-V0-016`, `AFP-V0-017`, `AFP-V0-026`, the `AFP-V0-022` promotion sentence and the closing
sentence of the bounded documentation CI exception. Adds `AFP-V0-028` (queue-backstopped PR
narrowing), `AFP-V0-029` (forward qualification rows) and `AFP-V0-030` (kill switch). The IDs are
provisional (`AFP-V0-027` is the V1-0809 `ci:batched` constituent skip), and the spec amendment
assigns the final IDs. Amends decisions 0289 and 0320 where cited; keeps 0319 and 0390.

## Context

Measured 2026-10-05:

- PR CI takes 30–46 min. `go-product` waits for the slowest of 4 race shards
  (`go test -json -p 1 -count=1 -race -timeout 50m`). V1-0617 moves to 6 shards and refreshed
  costs, which replay at about 21 min
  (`docs/build-log/2026-10-05-ci-cancel-waste-shard-balance.md`); hosted numbers are pending.
- The last 100 CI runs (about 11 h) used 7063 runner-minutes. 42% of them were in cancelled runs.
- The PR narrowing machinery is merged but inert. `CORVINT_PR_TOOL_SOURCE`,
  `CORVINT_PR_QUALIFICATION_SOURCE` and `CORVINT_PR_QUALIFICATION_SHA256` are empty in
  `.github/workflows/ci.yml`.
- One qualification run has happened: `pr-tests-qualification.yml`, run 36897095402, on 2026-10-01.
  It failed row 1. The full historical suite took 2451.7 s and gave 226 package outcomes (192 pass,
  11 fail, 23 skip). The selection named 155 packages at scope `UNKNOWN` and omitted two failing
  packages, `internal/frontiernextrepo` and `internal/tracerecordrepo`.
  `docs/build-log/2026-10-01-ci-shadow-graft-advice.md` traces both failures to the harness
  environment: graft advice in Git output under `GIT_GRAFT_FILE=/dev/null`. The selector did not
  omit them because of the change. Promotion was correctly refused. The 200-row campaign (about
  136 runner-hours) never ran. A fresh campaign also needs a newly frozen identity.
- The advisory `AFP-V0-025` selected share on recent PRs is 43–47% of estimated suite time for
  single-feature PRs and 92.8% for batch PRs. Unbounded readers and `cmd/corvint` fan-in drive
  the batch figure (V1-0246, V1-0081, V1-0719).
- `ci.yml` already has the `merge_group` trigger, and `ci-control-plane.yml` already reports on
  queue commits (`AFP-V0-026`). The `main` ruleset has only `pull_request` and
  `required_status_checks` rules, with no `merge_queue` rule. V1-0760 is open: a `main` push
  should reuse the merge-queue run of the same commit.
- The owner integrates related task PRs into one batch PR and arms auto-merge on every PR (merge
  commits, required by the decision 0319 seal).
- Local scratch replay (installed `corvint` 1.0.0-rc.1, `corvint affected --base <first parent>`
  on the 7 newest first-parent merges of `main` up to `d524530f`): `plan.scope` was `UNKNOWN` in
  7 of 7 cases. Two kinds of entry caused this. The first is repository-wide `LANGUAGE_FRONTIER`
  entries (non-Go fixture languages, `go:build-constraint-variants`,
  `go:nested-module-frontier`). The second is the `UNOWNED_DIRTY_PATH` of the sealed CEM sidecar
  that every dogfooded change adds. This was not a hosted measurement.

The historical replay has spent its budget on harness drift, not on selector evidence. The
backstop it was meant to give can come more cheaply from running the full suite on the exact
commit that becomes `main`.

## Decision

1. **Queue on `main`.** The `main` ruleset gains a `merge_queue` rule with merge method `MERGE`,
   maximum group size 1 and the checks `go-product`, `doc-gates` and `ci-control-plane`. All
   three must report on queue commits (`AFP-V0-026`). Merge commits keep the bind commit that
   decision 0319 names sealed CEMs after. Group size 1 gives each queue run exactly one new PR on
   top of the previous entry, so each forward row (item 5) has one owner. Batch PRs already group
   the work. The queue's status-check timeout must exceed one full `CI` run.
2. **Full where it binds.** `merge_group`, `main` push and release runs stay FULL, unchanged:
   the DCI classifier, the driver, the order plan and `AFP-V0-024` reuse stay off for
   `merge_group`, as `AFP-V0-026` already requires. Static, vet, format, cross-build, interop,
   documentation and artifact checks stay full on every event, including `pull_request`.
3. **Narrowed PR race tests (`AFP-V0-028`).** On `pull_request`, `go-product` runs only the
   trusted selector's root packages, intersected with the existing complete-universe shard
   partition (`AFP-V0-022`). A shard with an empty intersection runs no Go test and keeps its
   audit. Every existing FULL fallback stays: missing or untrusted tools, planning or selector
   failure, a `FALLBACK` verdict, topology or partition drift, a dirty root module definition,
   and any module-level Go frontier. Three new FULL conditions are added:
   - the PR base is not `main`, because only `main` has the backstop;
   - the PR changes a path under `.github/`;
   - the branch rules for `main`, read through the API, do not include an active
     `merge_queue` rule, or cannot be read. Narrowing is valid only while the backstop exists.

   Only the package set may differ from the full branch. The narrowed invocation uses the
   full branch's argv, environment, `test-confine` wrapper (`AFP-V0-023`) and cache policy.
   The driver's closed replay environment existed to match the historical identity, and it
   carries known gaps (graft advice, missing `node` on PATH, no confinement), so it does not
   govern PR execution.
4. **Scope `UNKNOWN`.** A plan at scope `UNKNOWN` may narrow only when every `plan.unknown` entry
   is in a closed, reviewed allowlist of classes that the `AFP-V0-012` widening already covers:
   - `LANGUAGE_FRONTIER` with a non-`go:` detail, because no root Go race test discovers tests
     in those languages, and their fixture paths are still attributed by rules (b) to (d);
   - `go:build-constraint-variants` and `go:nested-module-frontier`, because the race job runs
     linux/amd64 root packages only, and the cross-build and nested interop jobs stay full;
   - `UNOWNED_DIRTY_PATH`, attributed by rules (b), (c) and (d);
   - `NO_SELECTABLE_TEST`, traversed to dependents (`AFP-V0-020`).

   Any other reason or detail, including one added later, runs FULL until the allowlist is
   amended by review. The strictly conservative rule, `UNKNOWN` always means FULL, was
   considered and rejected. It is safe, but on the replay above it never narrows, so it would
   deliver the queue's latency without any saving. The allowlist fails closed on anything new,
   and the queue catches whatever it lets through. If the owner prefers the strict rule, this
   decision reduces to items 1, 2 and 5 in shadow only.
5. **Trusted tools without the historical artifact.** The planner, selector and driver stay
   built outside the PR checkout from a reviewed `main` commit pinned in `ci.yml`. Changing that
   pin is a `.github/` change and needs the `AFP-V0-016` admin consent on the exact head. The
   SHA256 qualification artifact and the 200-row campaign stop being a precondition for PR
   narrowing. `AFP-V0-014`/`AFP-V0-017` and `pr-tests-qualification.yml` stay available as
   optional evidence, and nothing is deleted. Go version, Git and tool-digest drift still
   force FULL.
6. **Forward qualification rows (`AFP-V0-029`).** Every PR run that computes a trusted selection
   retains it, with the tool source, the plan and selection digests and the shard partition
   digest. Every `merge_group` shard retains its terminal `go test -json` package outcomes,
   bounded like the existing audit. A `workflow_run` job from the default branch's copy (as in
   `AFP-V0-016`) builds one row per queue run. That job holds only `actions: read` and
   `contents: read` and never executes repository code. The row compares S with F:
   - S is the selection of the latest successful PR run for the head that entered the queue;
   - F is every package whose outcome in the queue run is fail, timeout, build failure or
     missing.

   A **miss** is a package in F but not in S, for a PR whose PR-time run narrowed. A package in
   both F and S is recorded as `SELECTED_FAILED_IN_QUEUE`, which means base skew, flakiness or
   an environment difference, and is not a miss. Each failing package is rerun once on the same
   queue commit. A pass classifies it `FLAKY`, but the queue verdict stays failure. A row whose
   inputs are missing or unreadable is `INCOMPLETE`: it is neither clean nor a miss. The row is
   the `corvint-pr-forward-row/0` artifact, at most 64 KiB, named
   `ci-forward-<CLEAN|FLAKY|MISS|INCOMPLETE>-pr<N>-<queue sha>`, under the default artifact
   retention. The class is the worst in the row: `MISS` when any missed package failed its rerun,
   `FLAKY` when every missed package passed it, so the name alone carries what item 8 counts.
   Rows never feed ranking, selection or learning. Only item 7 reads them, and it can only
   widen to FULL.
7. **Shadow first.** Items 1, 2 and 6 go live with PR runs still FULL, while the driver computes
   and retains S before running the full shard. In shadow, a PR run produces its own row from
   the same commit's complete outcomes, so failing pushes give counterfactual evidence that a
   wholly green queue cannot. Narrowing (item 3) is enabled when 20 complete shadow rows exist,
   at least 5 of them with one or more failing packages, and none of them has a miss other than
   `FLAKY`.
8. **Kill switch (`AFP-V0-030`).** PR runs return to FULL, with the reason
   `forward-qualification-tripped`, when either holds in the latest 20 rows of the current epoch,
   counting `INCOMPLETE` rows in that window:
   - 2 or more `MISS` rows;
   - 3 or more `INCOMPLETE` rows.

   The PR job evaluates this from artifact names, which it lists as `AFP-V0-024` does, without
   downloading. A listing failure means FULL. Separately, a repository variable
   `CORVINT_PR_NARROWING=off` forces FULL at once. It is admin-settable and can only disable.
   A reviewer who classifies a single `MISS` row as `CHANGE_CAUSED` trips the switch by setting
   that variable; the classification is a human judgment that no artifact name can carry.
   Re-arming takes a reviewed `.github/` change that advances a `CORVINT_PR_FORWARD_EPOCH` pin,
   so that earlier rows leave the window, with a build-log entry that names the review. A miss
   can never reach `main`, because the queue fails and ejects the PR. The switch protects what
   a PR-time green means, and the queue's throughput.
9. **Batch PRs and V1-0760.** At a 92.8% share, narrowing saves little on batch PRs, and it adds
   no risk there. The gain is on single-feature PRs and on superseded pushes. A narrowed PR run
   never writes an `AFP-V0-024` tested-tree record. So until V1-0760 lets the `main` push reuse
   the queue run of the identical commit, every merge pays a narrowed PR run, a full queue run
   and a full `main` push run. V1-0760 is a prerequisite for item 3.

## Invariants

- **Invariant 2.** The selection remains a non-authoritative plan (`AFP-V0-004`). A narrowed PR
  run does not treat an exclusion as proof that omission is safe. It defers the excluded
  packages to the queue's full run, which must pass before the commit can become `main`. A
  narrowed green is reported in the `go-product` summary and the step summary as "selected
  packages passed: N of M; the full suite runs in the merge queue", never as "suite passed".
  Missing rows, tools, listings or rules produce `INCOMPLETE` or FULL, never a narrowed claim.
- **Invariant 3.** Project-owned authority still decides what reaches `main`: the ruleset's
  required checks run in full on the queue commit. Syntax-derived selection only sets the order
  and cost of PR-time feedback. The allowlist and kill switch are reviewed project records.
  Invariants 4, 5 and 7 are untouched: this is hosted CI, rows are not learning input, and the
  local product path does not change.

## Consequences and risks

- **Latency.** At least one full run (30–46 min today) is added between "ready" and merge. An
  ejection invalidates the speculative entries behind it, and they rerun in full.
- **Late failure.** A PR can be green on its selection and then fail in the queue. Auto-merge
  then leaves it out of the queue, so it needs a fix and re-arming. Reviewers may have
  approved code that fails later.
- **Cost.** Today every push runs a full suite, and 42% of minutes were cancelled. After this
  change, every push runs only its selection (43–47% of estimated suite time for single-feature
  PRs), and each queue entry adds one full run plus any ejection reruns. For a batch PR with one
  or two pushes, total runner-minutes are likely to rise until V1-0760 removes the `main`
  repeat. The net effect is not measured.
- **Flakes.** Flaky tests now fail queue entries and eject PRs, not just PR runs. `FLAKY`
  classification records them but does not fix them.
- **Tooling.** It is `NOT_VERIFIED` whether `gh pr merge --auto --merge` behaves the same under
  a queue rule, so the owner's arming scripts may need the method flag dropped. Stacked PRs (base
  not `main`) stay FULL.
- **GitHub behavior.** The following are assumed, not observed: the queue branch form,
  `workflow_run` delivery for `merge_group`, the branch-rules API, and `main` fast-forwarding to
  the tested queue commit.
- **Wrong if:** queue ejections caused by the change exceed 1 in 5 entries over 20 merges; the
  runner-minutes per merged PR or the median ready-to-merge time do not fall below the baseline
  over 20 merged PRs; the kill switch trips twice in 30 days; or most PRs fall back to FULL
  through the allowlist or the share.

## Non-goals

There is no narrowing of `main`, release, `merge_group`, static, vet, format, build, interop,
documentation or artifact checks. There is no JavaScript/E2E selection, no learned selection,
no relabelling of the 2026-10-01 rows and no deletion of the historical qualification code.
Speedup on batch PRs is not claimed.

## Failure modes

- Queue rule absent or the API unreadable: FULL.
- Selection artifact missing at queue time: an `INCOMPLETE` row.
- Allowlist miss or a new frontier: FULL.
- Queue status timeout shorter than the `CI` run: entries fail.
- Disabling the queue while narrowing is armed: blocked by the item 3 rules check.
- `ci-control-plane` status lingering on a queue commit: as `AFP-V0-026` already limits.

## Acceptance evidence

- **To accept:** the owner accepted this record on 2026-10-05. The amended requirement text is
  written by the spec amendment, which assigns the final IDs and is reviewed before item 1 lands.
- **To enable shadow:** a real PR merged through the queue with `go-product`, `doc-gates` and
  `ci-control-plane` reported on the queue commit. `main` equals that commit, and
  `dogfood-check` reads its seal.
- **To enable narrowing:** the item 7 bar is met, with retained rows, and V1-0760 is merged.
- **To promote** the profile from experimental: the first 20 narrowed PRs have complete forward
  rows and no trip. Median and p90 PR wall time, runner-minutes per merged PR and ready-to-merge
  time are measured over at least 20 merged PRs before and after. Every result retained in a
  build-log entry. A measurement that is not available stays `NOT_OBSERVED`.

## Rollback

- **R1:** set `CORVINT_PR_NARROWING=off`, or clear `CORVINT_PR_TOOL_SOURCE`. This restores today's
  full PR race runs. The queue keeps backstopping, and forward rows continue in shadow.
- **R2:** after R1, remove the `merge_queue` rule from the `main` ruleset. This restores the
  pre-decision ruleset (`pull_request` plus the three required checks). The `merge_group` trigger
  can stay, because it is inert without the rule.
- **R3:** revert the spec amendment and `AFP-V0-028` to `AFP-V0-030`. This restores the decision
  0289/0320 contract, in which the `AFP-V0-014` artifact is again required to narrow.

## Owner answers (2026-10-05)

The owner answered all four as recommended: "agree with all four, accept it".


1. **Scope `UNKNOWN`:** the item 4 allowlist, or always FULL? Recommended: the allowlist, because
   always-FULL never narrowed on the replay.
2. **Shadow bar:** 20 rows with at least 5 failing, or narrow immediately? Recommended: the bar,
   which costs nothing extra because PR runs are full today.
3. **Kill switch:** 2 `MISS` rows or 3 `INCOMPLETE` rows in the latest 20, plus the variable for one reviewer-confirmed `CHANGE_CAUSED` miss?
   Recommended: yes, as written.
4. **V1-0760 before narrowing,** and group size 1? Recommended: yes to both, so that no merge
   pays three suite runs and every row has one owner.
