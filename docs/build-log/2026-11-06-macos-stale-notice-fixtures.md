# macOS-safe stale-notice summary fixtures

Greptile P1 identified that `TestDryRunLongStalePathsStayWithinSummaryBudget`
created paths beyond Darwin's 1024-byte absolute-path limit before reaching its
STR-V0-7313 assertions. The original test passed on Linux at
`c32b07c23003a4d9899d7de04f0bf72f18889a95` (rc 0), confirming why that run missed
this setup failure.

The filesystem case now caps full absolute fixture paths at 800 bytes, including
the temp root, and generated components at 200 bytes. It explicitly requires the
original notice to exceed its half-budget share while fitting in the total
budget; default and explicit summary forms retain the required sections within
4096 combined bytes. Full and JSON forms preserve the exact original notice.
`TestDryRunOversizedNoticeSharesSummaryBudget` supplies a greater-than-4096-byte
UTF-8 notice directly to the existing `summaryNoticeBudget` seam, then exercises
`testDryRun` with the reserved notice bytes and enough units to fill the remainder.
No production seam or behavior changed; STR-V0-7313 remains the governing intent.

Verification on Linux:

- `script/heavy.sh go test -count=1 -run '^TestDryRunLongStalePathsStayWithinSummaryBudget$' ./cmd/corvint`: pre-change rc 0.
- `script/heavy.sh go test -count=1 -run 'STRV0800|SummaryBudget|SummaryEverySection|DryRun' ./cmd/corvint`: rc 0.
- With `summaryNoticeBudget` changed to return the original oversized notice,
  `script/heavy.sh go test -count=1 -run 'TestDryRunLongStalePathsStayWithinSummaryBudget|TestDryRunOversizedNoticeSharesSummaryBudget' ./cmd/corvint`: rc 1; both tests fail (missing truncation, oversized notice and lost fixed sections).
- With `testDryRun` given `testSummaryLimit` instead of subtracting
  `options.summaryNoticeBytes`, the same mutant command returns rc 1: the new
  unit test observes 6124 combined bytes (4076 stdout + 2048 stderr).
- Each mutant was restored byte-for-byte immediately after its run; neither is
  part of the delivered diff. Changed Go source was formatted with `gofmt`.

Native pre-change context, query, impact and affected receipts were retained in
`/tmp/b1106a-fix2/`. Context returned READY with no critical missing governance,
but left STR-V0-7313 unresolved as a ticket-shaped ID; its original spec clause
was read directly. Query and impact succeeded. Initial dogfood refused stale
ignored OCM sidecars inherited from the sealed change; its prescribed archive
fix was applied. Its next run retained the inherited prechange-query STALE and
prechange-impact NOT_OBSERVED notes, then could not prepare an empty BASE..HEAD
diff. These limitations remain visible in the local reports. Final binding uses
the requested base and the session test runner intent only.

Darwin execution is NOT_OBSERVED on this Linux host. Retrieval, learning, profile,
benchmark and browser routes are inapplicable to this fixture-only change. No
repository-wide gate was requested; focused verification is the authorized gate.
Rollback is the test-only commit revert; product behavior is unchanged.
