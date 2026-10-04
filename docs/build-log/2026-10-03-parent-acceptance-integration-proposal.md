# Parent acceptance integration after observer repair

Date: 2026-10-03. Human intent: existing GitHub #394 / V1-0541.
This entry records an integration proposal, not a successful campaign or completion.

The task-owned source and historical review record come from workflow base
`2a10b050d939d32cb79625b8afa6b95d5926518c` through
`e5d777cbd1202dbffb88ce114956b304035bd5c5`. The base itself is the task-owned
complete-attempt intent seed, parent `094700bfbc7b637bd2d6405cd82ab5508ac4aa20`.
Its PTF-V0-006/control-join amendment is carried forward with the implementation.
Integration targets main `dd8cc0ca918a80faaa041c98a783de11550b2bcc`, preserving
the merged V1-0689 observer implementation and NEA-V0-006/007 contract.
The original CEM binds the historical candidate and is not transplanted.

Read-only inspection of the original 7,331,840-byte report, SHA-256
`29c590e04863dc6d8f1ad26c852dac4608de97bd0f02ad78733c7cf73371fc0f`,
verified all 36 repeat/probe native receipt digests and identical carried projections.
All 32 baselines are ASSOCIATED, ELIGIBLE and CURRENT; 17 executions passed and 15 failed.
The failed rows trigger `provider-validity-incomplete` in the acceptance classifier.
This is expected negative classification under NEA-V0-004/PTF-V0-008, not missing
provider evidence. No additional validity repair is required or claimed.

The original N32 remains unqualified because the outer observer exhausted its bound.
Its marker SHA-256 `a86f8a032b611b2b933cbb3b63e78b75cde524fb83352f6380a90149ddff86f6`
and enrollment-state SHA-256
`f55c26e5acd25904d1ec492a83b5b65be57bdb8be9993b0a7cbdc100e2ed2fb1` remain immutable.
The preceding 2026-10-02 build-log entry is a historical protocol and review record;
its seven checks and old enrollment cannot authorize or qualify a new candidate.

A further campaign requires explicit owner disposition, admitted effects, independent
integration review, actual metadata checks, and a clean committed CEM binding before live
execution. Stable N2 supplies positive acceptance; nonasserting N2 measures survival;
one flaky N32 supplies mixed-repeat rejection and actual order/isolation observations.
Neither mixed outcomes nor a particular isolation pattern is guaranteed. Stop after a failed
prerequisite, retain failures, and never retry for a desired pattern. New frozen receipts
must reference the new candidate without modifying or relabeling the original enrollment.
Whole #394 remains open through qualification, integration and native completion.
Rollback reverts only the parent integration while retaining observer repair and all evidence.
