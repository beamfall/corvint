# Batch O integration (V1-1045 issue 701, V1-1037, V1-1041)

Date: 2026-10-09

Batch O carries three tickets onto main 986cec7c:

- V1-1045 (`corvint-tasks pool confirm-safe` held `taskman.prepare.lock` through the complete
  receipt audit, GitHub issue 701) from lane branch `claude/v1-1045-confirm-safe-checkpoint`
  (e6b3ddab). The owner accepted `CAL-V0-205` in chat on 2026-10-09; decision 0471 records it.
- V1-1037 (procgroup waitid stop/continue reports treated as exit on Darwin) and V1-1041
  (scheduler-dependent fail-open bound; groupreap race test that may not race) from lane branch
  `claude/v1-1037-procgroup-waitid` (cc1b1241, review fixes e60560cd and d4366347).

The lanes touch disjoint files and merged without conflicts.

Evidence:

- V1-1045:
  - `TestCALV0205_PoolConfirmSafeTakesWriterRoute` fails on the base and passes on the lane.
  - The focused pool/sweep/writer tests pass, and `go test ./internal/tasks/...` passes.
  - A synthetic measurement on 6,000 receipts gave 0.2 s on the fast route against 4.3 s on the
    full route.
  - The independent read-only review passed. Its one informational note is the accepted
    prefix-rewrite trust limit, recorded in decision 0471.
- V1-1037/V1-1041:
  - The new procgroup stop/continue test fails on the base.
  - The groupreap tests pass 30 times under `-race` and 10 times with `GOMAXPROCS=1`.
  - The fail-open matrix test passes.
  - The first independent review failed the race test because it proved no overlap and leaked on
    failure paths. The second review found a leak in the pipe setup. Both are fixed, in
    e60560cd and d4366347.
- On the batch head, the doc gates and the focused tests named in the bind plan are rerun.

Limits:

- Live qualification against the 21,100-receipt store is NOT_RUN.
- The procgroup and groupreap tests were run only on Darwin arm64.
- Two other waitid copies, in `internal/dashboard/repository` and `internal/testsupport`, still
  lack the stop-report check. They are filed as V1-1046.
