# Native release journal CI repair (issue 357)

PR 359's first shard failed two journal expectations after the CAL-V0-027
non-fixture staging admission change. The exact baseline at
`ecee5d1efc2a897feaaa8a66237567797e9ce362` reproduced both failures:
`TestTMV0008_AS07_JournalStageFixtureRestriction` expected UNSUPPORTED for
unlinked temporary stage bytes; `TestTMV0009_AS11_StageReceiptOnlyGenesisValidation/nonfixture`
expected UNSUPPORTED for fully validated receipt-only native genesis.
The accepted CAL-V0-027 shared staging requirement admits both identities;
restoring fixture-only production behavior would violate that requirement.

The repair changes only journal acceptance tests. Unlinked native stage
observation must succeed with a consistent, nonpending result and leave every
state and intent byte untouched. Native receipt-only genesis must return
REDO_PENDING with the existing full consistency and PRE_OR_POST assertions.
The paired native missing-blob case still requires JOURNAL_FORKED; malformed,
foreign-queue, invalid-genesis and other refusal cases remain intact. No writer,
execution authority, staging cleanup or production admission behavior changes.

Full journal and snapshot package tests passed (3.478s and 4.240s) with
`GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/tasks/journal ./internal/tasks/snapshot`.
The earlier sandbox cache-access failure is retained separately from that pass.
Evidence lives in `/tmp/corvint-native-release-357/repair-*`; the original CI
failure remains in `ci-shard0-direct.log`. Query and affected planning were used;
query retained three omitted ranked results and withheld test candidates.
The affected plan's repository-wide advice is not a full-gate result; scoped
repair policy selects the two related packages. Repository-wide CI rerun,
independent review, immutable check binding and seal remain delivery steps.
No ranking change needs frozen retrieval evaluation; providers, mutation and
new artifact qualification are outside this tests-only repair. Billed tokens
and cost remain NOT_OBSERVED. Rollback reverts these test changes and restores
the reproduced failures without altering production behavior.
