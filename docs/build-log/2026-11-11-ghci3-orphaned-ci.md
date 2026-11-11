<!-- SPDX-License-Identifier: Apache-2.0 -->
# Lane ghci3: orphaned `.github` CI work for 1.0 (V1-5097, V1-5211, V1-5086, V1-5160; V1-5235)

Base: `b55635262788687c3eea9a08a94378e4ab8856a4` (origin/main, 2026-11-11). The lane rebuilt the
`.github` work of the never-opened batch branch `claude/batch-1109-gh` (26 commits, 3 merges of
`main`; bundle `b1109gh`) on `main`, by cherry-pick and adaptation, never by merging that branch.
It also carries lane ghci2's `.github` half (V1-5098, V1-5099, V1-5258, V1-5349), whose code and
spec half rides pull request 191 (`claude/batch-1111e`). Two branches, as ASD-V0-706 requires:

- `claude/ghci3`: `.github/**` only. It lands after pull request 191 (the ghci2 Go fixtures and the
  guard list) and after `claude/ghci3-code`.
- `claude/ghci3-code`: the spec amendment, the before-and-after CIN-V0-2320 binding and this entry.

## Inventory of `origin/main..claude/batch-1109-gh` (non-merge commits)

Checked against `origin/main` and pull request 191 (`origin/claude/batch-1111e`, `0e036a132`).

| Bundle commit | Subject (ticket) | Verdict | Where |
|---|---|---|---|
| `76821e6c0` | log the FULL reason and `full-reason.json` (V1-5196) | ALREADY_ON_MAIN | `b61e66768` (FULL reasons with codes, decision sidecar), `fed598947` |
| `7e9095d80` | confirm main-miss links by reruns (V1-5198) | ALREADY_ON_MAIN | `88510f617`, `0e1048201`, `50472354d`, `9b3e8b846` (`mainmiss-confirm.sh`, `main-miss.yml`; a different design than the bundle's `mainconfirm.sh`) |
| `6f168b2d1` | skip advisory macOS shards for Darwin-neutral PRs (V1-5197) | SUPERSEDED-for-1.0 | V1-5197 is post-1.0; dropped |
| `f853acbb6` | run testconfine's `main_test.go`, check dot-directory tests (V1-5160) | NEEDED | rebuilt as `dec57dc2b` (with the usage fix of `fa2581a5a`); CIN-V0-8728 |
| `e1a1f8e69` | shell-test selector from the trusted base, base doc gates (V1-5097) | SUPERSEDED | rebuilt to the OpenSpec design of pull request 191 as `8108eb3c0` (see V1-5097 below) |
| `7555c3a44` | read a full-reason-only narrowing artifact as full (V1-5196) | ALREADY_ON_MAIN | `fed598947` |
| `c5f9dddcc` | vscode-extension and cce-live suites from the Go closure (V1-5200) | cce-live half SUPERSEDED (decision 0593/0594: `internal/changee2e` is gone); vscode half NEEDED, not rebuilt | changes VSC-V0-080, which `TestVSCV0080ExtensionWorkflowRunsOnContractChanges` pins; its code and spec half is in no retained ref; V1-5200 is not this lane's |
| `b5425880c` | price an unpriced package conservatively (V1-5086) | NEEDED | rebuilt as `9f9d105c1`; AFP-V0-022 amended |
| `fa2581a5a` | Lane C fix round 1 | split: `dot-tests` usage NEEDED (folded into `dec57dc2b`); `contract.py`/`run.sh` SUPERSEDED with `e1a1f8e69`; `narrow.sh` code ordering SUPERSEDED (main codes FULL reasons in `b61e66768`, no such block) | |
| `4bcc6f060` | Lane C fix round 2 | SUPERSEDED with `e1a1f8e69` | |
| `4e134040a` | macOS advisory failures, shard result artifacts, proven PR base (V1-5212, V1-5210, V1-5211) | split: V1-5211 NEEDED, rebuilt as `8108eb3c0` (shell tests) and `5e67f9cb9` (web flows); V1-5212 macOS half SUPERSEDED-for-1.0; V1-5210 result-artifact half NEEDED, not rebuilt | AFP-V0-2832 to 2835 are defined in no retained ref; V1-5210 is not this lane's |
| `bac041c74` | honest rows for shadow FULL fallback; MISS-dominant dedupe (V1-5216, V1-5218) | SUPERSEDED | CIN-V0-7921 (`88510f617`): FULL rows are typed `ci-forward-FULL-...`, not `ABSTAINED`. The MISS-dominant dedupe (a later CLEAN re-run of a pair cannot replace its MISS) is not on main; it is an owner decision, reported, not rebuilt |
| `645590d3a` | the fourth pull request #163 INFRA key (CIN-V0-5219) | ALREADY_ON_MAIN | `infra-reclassified.txt` lists artifacts `13673798657` and `13684996630` |
| `7d6ce479b` | consumed `.github` inputs in the narrowing guard (V1-5202) | ALREADY_ON_MAIN | `b61e66768`, `90bafb074` (per-consumer guard, CIN-V0-8423) |
| `3b74a3dfd`, `9042cffa1`, `96f8a2c93`, `17efd35ad` | doc-gates contract step placement and variables (V1-5097) | SUPERSEDED | `contract.py` is not used; CIN-V0-8726 runs the base list instead |
| `34842fce4` | leave the cce-live job as on main | SUPERSEDED | net no-op against the bundle's own `c5f9dddcc` |
| `5c9c40db6` | confirm main misses with the shard's live-tier skip | ALREADY_ON_MAIN | `9b3e8b846` (live-tier parity in `mainmiss-confirm.sh`) |

Pull request 191 delivers the ghci2 code half (CIN-V0-8720 to 8723, the go-product fixtures, the
guard list) and the `trusted-shell-selection` proposal; nothing here duplicates it. The charter
named `0e17762f0` and `b39809d22` as V1-5211 parts; neither object exists in the repository, in
any of the 52 lane bundles or in any lane note, so nothing of them could be reused.

## V1-5097: trusted shell-test selection and the base doc-gate list (CIN-V0-8724 to 8727)

The bundle answered the `trusted-shell-selection` design's open questions the strict way (a
dropped step or gate failed the job unless `.github/*/reviewed` retired it; `contract.py` refused
changed doc-gate rules, `include`, `$(eval` and recipe variables). The design in pull request 191,
written later and matching the dispatch's expected answer, chooses FULL plus a warning and leaves
recipes to the candidate. The lane implemented the design's defaults and recorded both choices in
the spec intent, so the owner can still choose the strict answer:

- `.github/shell-tests/run.sh`: worktree of the event base, `go build -buildvcs=true` of the
  planner and selector, `affected --accept-stale-binary` admitting only no degradation or exactly
  `self-binary-stale`, selection from both `Makefile`s, union; a deleted base-selected step, a base
  that does not build, a refused or degraded plan or an unreadable list run every step with a
  warning. The design put the list reader in `ci.yml`; it lives in this script under `.github`,
  held by the same waiver, and the test sits in `.github/shell-tests/` so no guard-list change in
  `tools/ci-forward-row` is needed.
- `.github/doc-gates/base-gates.sh`: reads the base list with `awk`, the candidate's as `make`
  expands it (an override or `filter-out` later in the `Makefile` counts as leaving a gate out).
- The doc-gates step keeps the job's plain `needs.doc-gates-reuse.outputs.mode != 'REUSE'`
  condition, which AFP-V0-670 (`TestAFPV0670GatesWireExactTreeReuse`) requires of every step; on a
  push its `EVENT_BASE` is empty and the script skips.
- Real-repository smoke: on a merge-queue-shaped commit over `b55635262`, `run.sh` built both tools
  from the base, narrowed to 5 of 49 steps and named the event-base selector.

Fault injection, each against the real scripts (`trusted_selection_test.py`, 13 tests): building
the tools in the tested tree fails 2; dropping the union fails 1; planning from the raw event base
fails 2; admitting any degradation fails 2; reading the candidate gate list with `awk` fails 1;
skipping the base gates fails 2. The control run of the old step body misses the base-selected step.

## V1-5160: dot-directory tests (CIN-V0-8728)

All four `test-confine` copies (`ci.yml` and `linecov-miss.yml`, `main-miss.yml`,
`linecov-index.yml`) now include `cmd/main_test.go`: the linecov-miss copy must equal the shard's
(`TestLinecovMissConfirmationReproducesRecordedRunner`), and a `_test.go` file does not change the
built binary. Dropping the file from the go-static copy makes `check.py .` fail.

## V1-5211: the selection diff base (AFP-V0-2830, AFP-V0-2831)

Reproduced on retained evidence: run 42965249854 attempt 2 (job 130356086279, `shell-test-selected`)
logged `BASE_SHA: 02c66d0e263a5437f5c4745c64f3b4bd25188b4f` and `HEAD is now at e274c3480 Merge
02cf0213… into 993c60eba4d4…`; `02c66d0e2` is an ancestor of `993c60eba`, 4 commits and 2 files
behind. Run 42833209892 attempt 2 had equal values. The narrowing shadow already proves the base
(AFP-V0-800); the order planner (advisory), cadence classifier and corvint-pr-tests take base, head
and target and prove it in tools; `shell-test-selected` and `web-flows-plan` diffed the raw base.
Both now use the proven first parent on a pull request and run everything (or require the web-flows
consumer) when the proof is refused. No Go test was gated on this change on `main`, so nothing
needed un-gating; the tests are Python extract-and-run tests in the `.github` half. Removing the
web-flows proof block fails 2 of 4 tests in `.github/web-flows/plan_base_test.py`.

## V1-5086: shard pricing (partial)

`9f9d105c1` prices a package missing from the cost table at the table's 90th percentile for
placement and sizing (the AFP-V0-022 share keeps the median). Measured on run 42408095814 (FULL, 16
shards; package-level `Elapsed` summed from the shard job logs' `go test -json`): shards 0 to 10 ran
one unit each in 69 to 465 s; shards 11 to 15 summed 2963, 4055, 4483, 1946 and 1919 s; 458
packages, 18396 s in total; 165 packages (6405 s) were absent from the cost table, among them
`internal/tasks/cli` (1574 s, since deleted), `typedreach` (799 s), `criterionexperiment` (359 s)
and `corpusindex` (265 s); `conformance/cli-errors-v0` measured 262 s against 16 s in the table.
Remainder (NOT_DONE): refresh `package-costs.json` and `test-slices.json` from measured runs
including pull-request runs (AFP-V0-540 refreshes only from pushes); re-slice units above the
per-shard budget; size the FULL matrix too, which today stays 16 because `REUSE_SHARDS`, the
forward rows and the reuse records assume it (AFP-V0-2725); a FULL replay test of the measured
times (criterion 1) and a count-change test against the V1-4353 kill switch (criterion 4).

## V1-5235: layer-derived shard lists (NOT_DONE)

The layer manifest exists (`tools/import-ratchet/areas.json`, re-keyed to decision 0594's five
layers by V1-5233). A layer-level shard skip is not sound yet: `baseline.json` still admits 17
wrong-way imports, including docs into tests (`internal/doccorpus -> internal/jstestprovider`) and
shared into docs and tests (`internal/appflows -> internal/doccorpus`), so a docs-only change does
reach the shared and tests layers. Decision 0594 item 8 orders "enforce the import rule" before
"shard CI by layer". Remainder: empty the baseline (or derive layer dependents including its edges),
specify the layer-to-shard mapping against the 16-shard reuse and forward-row contracts, add the
drift check, and record one real docs-only pull request.

## Verification

`.github` head `dec57dc2b` on `claude/ghci3`. Combined tree (pull request 191 tip `0e036a132`, then
this code half, then the `.github` half): the nine `.github` Python test files pass;
`tools/ci-reuse-plan`, `tools/ci-forward-row`, `.github/cishards`, `tools/linecov`,
`tools/capabilities-check`, `internal/sessiontest`, `tools/corvint-pr-tests`, `tools/ci-shard-costs`,
`tools/ci-test-slices`, `.github/testconfine` and its `cmd` pass whole; `cmd/corvint` passes
`TestCINV07504*` and `TestAFPV0054*`. Main plus this code half alone: `tools/ci-reuse-plan` passes.

Recorded in the lane handback (`ghci3.handback.md`): every Python test of the `.github` half, the
Go packages that read `ci.yml` or import `.github/cishards`, the split check on each branch, the doc
gates. Hosted runs of every requirement here: NOT_RUN (not possible in a lane).
