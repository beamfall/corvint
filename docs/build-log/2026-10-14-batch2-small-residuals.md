# 2026-10-14 batch 2: small residuals for V1-0494, V1-1287, V1-1312, V1-0150, V1-1296, V1-1300

Base `495eda08cf3dc4d8122b169a4cc16d9c4e599b19` (origin/main). The read-only audits of 2026-10-14
found each ticket delivered except for one small residual. This entry repairs five of them and
leaves V1-1296 open. No requirement ID was added. `corvint-tasks ticket show` could not run in the
lane clone because the clone has no journal, so the acceptance text comes from the ticket JSON the
audit saved that day. The orchestrator closes READY tickets against this change's merge.

## V1-0494: tasks workflow context regressions

- AC1 (reproduction): met earlier in `2026-09-29-tasks-context-identity.md`. On clean Core 6dc8ed1,
  the title-only query was OUT_OF_SCOPE and title plus canonical ID was READY.
- AC2 (contract and smallest fix): met by `internal/tasks/store/workflow.go:175-182` (5c3bd57638,
  merged c57a9863c) under CAL-V0-036 (`docs/specs/corvint-tasks-agent-leases-v0.md:750`).
- AC3 (enrichment bounds): met by the same lines. The query carries the title and the canonical ID
  only, never paths. Over-long queries are refused. The READY, fresh and exact-tree requirements
  are unchanged.
- AC4 (regressions): earlier evidence covered unsupported, out-of-scope, stale and wrong-tree
  packets (`workflow_context_test.go:116-150`). Absent source was covered only by the opt-in replay.
  This change adds two tests that run by default:
  - `TestWorkflowContextKeepsUncertainNonGoFrontier`: a READY, fresh, exact-tree packet with a Rust
    result and two uncertainty entries is returned byte-identical. The argv must be exactly
    `query --task "<title> <ID>" --budget-bytes 8000`.
  - `TestWorkflowContextRefusesIncompleteScopeAndAbsentSource`: refuses PARTIAL scope,
    AMBIGUOUS_BINDING, a not-ok packet, a packet with no revision, a directory with no repository
    and an absent revision. The test logs each refusal: `CONTEXT_UNAVAILABLE` for the first four
    and `UNSUPPORTED: git ...` for the absent cases.
  - Both are cited after CAL-V0-036 in the leases spec.
  - Results: `go test -race -run TestWorkflowContext ./internal/tasks/store/` and
    `go test ./internal/tasks/store/` both report ok.
- Verdict: READY_TO_CLOSE.

## V1-1287: the stable manifest check covers every entry

- AC1 (every entry matches its file): met. `go run ./tools/cem-stable-regen --check` reports PASS
  on the repository with the widened check.
- AC2 (the check covers every entry): met.
  - `manifestProblems` no longer takes an allow-list, and the stable manifest is checked like the
    packet and tuple manifests. Before this change only `RESULT.schema.json` was checked
    (`tools/cem-stable-regen/main.go:258-262` at base).
  - New test `TestCheckCoversEveryStableManifestEntry` drifts `PACKING.md` and
    `tamper-matrix/README.md` by one byte. It passes with the new code and fails against the
    schema-only check, which was verified by reverting the change locally.
  - The widened check exposed a second gap. The stable manifest also lists the
    `repository-envelope-packet/` files, but the regeneration did not re-bind those entries, so
    `TestRegenerateRealPacketCopy` failed its own check. The stable manifest step now re-binds
    each listed packet file it rewrites, plus the packet `manifest.json`.
  - Results: `go test -race ./tools/cem-stable-regen/` reports ok. `go vet` is clean.
- AC3 (repair recorded with the predecessor digests): met. See `docs/specs/cem-stable-v1.md`,
  "Stable manifest entry repair". It records c3eed8b8/302453 and 9c35b7a5/9705 from fc5588948,
  which af8f466dd re-bound. The CEM-V1-091 traceability row also cites the new test.
- No sealed byte changed. Rollback is to revert the two code hunks and the spec section.
- Verdict: READY_TO_CLOSE.

## V1-1312: DCW-V0-049 row cites the v1-1336 rerun

- AC1 (measuring at T' with `--targets`): met earlier. See `tools/cem-citation-baseline/main.go:26-30`,
  `TestBaselineDCWV0056TargetsVerifyAtTheNamedCommit`, and the equivalence in
  `2026-10-12-v1-1277-cem-migration.md:74-100`.
- AC2 (corpus rerun and baseline row):
  - The rerun is retained under `docs/build-log/evidence/v1-1336/` (#852, dc6811176). The summary
    reports 630 maps with 1 invalid. The rows contain 0 `excluded-artifact-mismatch` and 1
    `unsupported-resource-limit` (f0fda542). This was checked with grep over the jsonl.
  - The DCW-V0-049 row in `daily-change-evidence-workflow-v0.md` no longer says the migrated set
    is left for V1-1277. It now cites that rerun and records that the `chore` split is still empty.
- Verdict: READY_TO_CLOSE.

## V1-0150: linux amd64 lifecycle and hostile runs

- AC (linux amd64 runs on a real host or container): met by hosted release-gates run 38654510601.
  - The run is at 52ad86a368ae06520617b536b12f22d38102b362, an ancestor of base. The job is
    `linux-core`, id 116160978670, on ubuntu-24.04.
  - Architecture was checked with `gh run view 38654510601 --log --job 116160978670`. The log
    shows `platform Linux x86_64`, `GOHOSTARCH=amd64` and `os=linux/amd64`, which is an x64
    hosted runner.
  - Results:
    - Install lifecycle passed. The N-1 lifecycle passed with `packet=identical`.
    - The cli, codex and claude-code host lifecycles each passed 9/9.
    - The hostile regression matrix passed, with 27 of 30 rows PASS. The two case-fold rows were
      NOT_RUN and `memory-resident` was NOT_COVERED.
  - This is recorded in `stable-operations-v0.md` at the "Blocked on" line, in a 2026-10-14
    paragraph appended after Unresolved (end of file, so no cited line moves) and in the Unresolved
    text. The log is retained by GitHub, not in the repository.
- AC (whole-process memory has a test or a recorded reason): met by the recorded NOT_COVERED
  reason (`stable-operations-v0.md:223`).
- The ticket's `full-gate` was not run here, per the charter. The orchestrator runs it.
- Verdict: READY_TO_CLOSE once the orchestrator's full gate passes.

## V1-1296: dangling MIGRATION.json parentPlan

- AC1 (the pointer resolves or is removed, and a gate refuses dangling pointers): not met.
  - `protocol/cem-1.0/stable/MIGRATION.json:4` still names `../migration-plan.json` with a SHA256.
  - The file is byte-identical to `repository-envelope-packet/inherited/s0e/MIGRATION.json`. It is
    pinned by the stable manifest and by `INPUTS.json`.
  - `cem-stable-v1.md` (the MIGRATION.json notes near :1528 and :1684) says it "is not edited here"
    until that packet is regenerated.
  - `tools/cem-stable-regen` only re-binds additive member files. It has no mode that rewrites or
    re-binds a non-member stable file, and the charter forbids hand-editing.
  - The fix needs a spec decision on whether the inherited copy changes too, a new regen mode and a
    pointer gate. That is more than this batch's bound.
- Verdict: OPEN_RESIDUAL.

## V1-1300: decision 0158 developer-check row

- AC1: met. The decision 0158 row no longer lists `internal/dogfoodflow/check.go` as using
  ordinary Git status. A dated "Amendment 2026-10-14 (V1-1300)" records three things:
  - DCW-V0-045 moved the check to `gokernel.ProbeRepositoryContext`.
  - External excludes are disabled there, while `info/exclude` still applies.
  - Policy choice C is unchanged.
- AC2: met. The citation now points at the probe call `internal/dogfoodflow/check.go:251@e7b189a7`.
  `make line-citations-check` passes.
- Verdict: READY_TO_CLOSE.

## Verification and review

- The test units were selected with `corvint affected --base 495eda08c`. The focused tests and
  `-race` runs listed above pass. `go vet` is clean on both touched packages. `gofmt -l` is empty.
  The 12 CI doc gates pass after staging.
- Codex review: one read-only round over `495eda08c..986202db0` returned APPROVE with no confirmed
  findings. It reran the focused tests and checked the v1-1336 rows and the linux-core job log.
