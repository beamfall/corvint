# CI tools lane 1021i: record-job reuse trust, least-privilege rows, shadow-line logs, kept admin status

Lane `ci-tools-1021i`, base `8def072e08e9730a6102657c887655f62993b31d`. Requirements AFP-V0-170 and
AFP-V0-171, and the AFP-V0-024/046 amendments, are accepted under the owner's standing auto-accept
(2026-10-11). The `.github/` changes need the owner's `ci-control-plane` post on the PR head.

## V1-2271 (P1, security): the tested-tree record was forgeable

**Problem.** `tools/ci-reuse-plan` decided REUSE from the names of
`ci-tested-tree-<tree>-full-<shard>-of-<shards>` artifacts. Those artifacts were uploaded by the
`go-product-shard` jobs, which run the tested tree's own code, so any code in that tree could upload
all eight names. A main push or queue entry with the same tree would then skip the root race run.

**Decision.** This follows the pattern PR #1449 used for AFP-V0-163.

- A new `ci-tested-tree-record` job needs every shard (and `docs-plan` and `narrowing-verdict`).
  It runs only when all shards succeeded in full: no MET verdict, and neither DOCS nor REUSE.
- It runs no repository code: a checkout with `persist-credentials: false`, then git and printf.
- It prints one fixed line, `corvint-ci-tested-tree/1 tree= target= head= shards=`, to its own log.
- The `docs-plan` reuse step reads the jobs listing and that job's log through the jobs API
  (`actions: read`, so the job gains a least-privilege row). It no longer reads artifacts.
- The tool accepts the line only under these conditions:
  - exactly one record job belongs to the admitted run and its head, and it succeeded;
  - the log holds exactly one anchored line;
  - the line's tree, head and shard count match (and its target, for a merge-queue run).
- Every other case is FULL.
- The shards no longer upload a record. `records` in the decision keeps the old names, so the
  forward-row consumers are unchanged.

**Evidence.**

- `TestAFPV0170ReuseStepRefusesForgedArtifacts` runs the real reuse step script against a stub API
  and a real Git merge, for both `push` and `merge_group`.
  - Against the base `ci.yml` and `main.go` it fails: "forged artifacts with no record job decided
    REUSE", and the same for another tree.
  - With the fix it passes, and the genuine record decides REUSE.
- `TestAFPV0170OnlyTheRecordJobLogVouchesForTheTree` covers the tool's refusals.
- `TestAFPV0170RecordJobRunsNoRepositoryCode` pins the record job to the no-code allowlist.

**Transition.** A queue entry whose tool is built from the old `main` gets flags it does not
know, so it runs FULL (fail closed). The first push after the merge may also be FULL.

## V1-2270: ci-least-privilege-check failed on main

**Problem.** Reproduced on the base: the `ci-forward-bar.yml` workflow block, its `verdict` job and
`ci.yml` `narrowing-verdict` all lacked exception rows.

**Fix.** The rows were added, each naming the job, the permission and the justification. The check
and its test pass.

## V1-2269: ci-test-slices refused current shard logs

**Problem.** Reproduced with the shard logs of PR run 39635500953. The base binary refused them with
"line 939: damaged go test -json record: no Action" at the AFP-V0-047 shadow summary JSON line.

**Fix.** A complete JSON object that does not open with a `go test -json` key is skipped as another
step's output. A record key after other text is still refused.

**Evidence.**

- `TestAFPV0041ObserveSkipsTheShadowSummaryLine` fails before the fix.
- The new binary replays all live logs.

## V1-1847: control plane overwrote an admin success

**Fix.** Before posting `failure`, the `control-plane` job reads the head's statuses.

- If the newest `ci-control-plane` status is a success by a `User` creator, the job posts nothing.
- A new head, a bot success or a newer failure still gets `failure`.
- A failed listing stops the step with no post.

**Evidence.** `TestAFPV0171ControlPlaneKeepsAUserPostedSuccess` fails against the base workflow.

## Tests and gates

- Whole packages: `tools/ci-reuse-plan` and `tools/ci-test-slices`.
- Affected siblings: `tools/ci-forward-row`, `corvint-pr-tests`, `docs-ci-plan`,
  `test-read-scopes-check`, `fixture-maintenance-check`, `capabilities-check`,
  `cem-citation-baseline` and `cem-trial`.
  - `fixture-maintenance-check` first caught the new Git fixture committing without
    `maintenance.auto=false`; that is fixed.
- `go vet`, `gofmt -l` and `make windows-vet`.
- The doc gates, `ci-least-privilege-check` and its test, `host-package-versions-*`,
  `test-read-scopes-check`, `test-commands-check` and `analyzer-schema.sh check`.

## NOT_RUN

- `corvint affected` also selected `cmd/corvint`, `internal/**`, `conformance/**` and the script
  tests. They import no changed package, so they are left to batch CI.

- Hosted runs of the record job and of REUSE.
- A hosted control-plane rerun after an admin post.
- `actionlint`, which is not installed locally.
- `make gate`, which is not required.
- The tier 2 affected packages, left to batch CI.

## Rollback

Revert the commit. Reverting the record job, the reuse step and the tool separately leaves REUSE
unreachable (FULL), never wrongly reused.
