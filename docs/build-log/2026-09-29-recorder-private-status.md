# Fresh recorder status admission — V1-0469 / issue 343

The issue reproduction failed three times at a stable commit/tree: the writer's own untracked
`.trace-operation.lock` changed the final raw Git status digest. Fixtures that committed an
ignore rule hid the defect. A disposable-only exclude control recorded twice with one row and
one trace ID. The new compiled-CLI regression reproduced the failure in main and linked
worktrees before the repair, without any trace ignore rule.

The repair interprets source cleanliness only within the explicit recorder. A bounded raw-status
expansion must hash-match the existing index/observation; it cannot refresh that authority.
Only exact untracked trace artifact paths are eligible. Read-only descriptor checks enforce
private directory/member modes, regular single-link files, bounded size, and an empty lock.
Canonical rows still pass existing store validation. Every tracked artifact is refused before
recovery or append, including an otherwise clean tracked target. Real commit/tree/source changes
remain refusals. Read/query adapters retain their exact status comparison.

Independent plan review found that existing lock opening does not validate its bytes and lone
staging recovery can precede later canonical rejection. The revised plan therefore refuses all
unignored pre-existing staging before recovery. Only the final append check receives its exact
already-created, descriptor-pinned temporary; earlier callbacks cannot authorize staging.
Unignored interrupted recovery is still unsupported. Ignored-store recovery is unchanged and
this repair does not claim a new whole-store recovery atomicity guarantee.

Rollback is a revert of this source change; no existing stores move and no ignore configuration is
written. Invalid-input and unsafe-artifact fixtures assert unchanged bytes, modes and entries.
The affected trace packages and compiled fresh-repository CLI regression passed during building.
Final selected-check and independent source-review evidence is retained in the task's private
recorder evidence directory; this entry is not a final gate, promotion, merge or ticket receipt.
Repository-wide gate and learned-ranking frozen evaluation are NOT_RUN for this scoped recorder
repair; ranking and learning behavior are unchanged. CEM/OCM uncertainty and exact postcommit
binding remain visible in the dogfood reports. Billed tokens, cache use and total cost are
NOT_OBSERVED; no savings claim is made.
