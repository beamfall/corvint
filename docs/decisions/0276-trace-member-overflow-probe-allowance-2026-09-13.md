# Decision 0276 — Trace-member overflow-probe allowance is keyed per member per store attempt

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`internal/dashboard/source` detects a source that grew past its `lstat` size by reading one byte past
the declared size. That overflow byte is charged against an allowance of two probes. Every trace
member read charged the same `(local-trace-v1, configuredOrdinal)` key, so the allowance was shared
by the whole store. A member that grew during its read on both store attempts spent both probes, the
next member read was refused as `IssueAggregateBudgetExceeded`, the store ended, and the scan failed
as fatal `DASHBOARD_RESOURCE_EXHAUSTED` with no snapshot, although the only defect was one member
that changed during its read. The spec allowed "one overflow byte per configured artifact per
attempt", which a store-wide key could not honour for a store of many members.

The call: the allowance is keyed per trace member per store attempt, as
`(configuredOrdinal, member name, store attempt)`. Each member acquisition attempt may spend at most
one overflow probe, two per member per store attempt, so an unstable member ends as its own
member-terminal `SOURCE_CHANGED_DURING_READ` and cannot refuse other members' reads. Singular
configured artifacts keep their `(adapterId, configuredOrdinal)` key and cap of two.

The invocation-wide worst case stays bounded by the existing caps: one store attempt reads at most
the 1,000-member cap, so it spends at most 1,000 x 2 = 2,000 overflow bytes; with two store attempts
per scan attempt and two scan attempts, a configured trace store spends at most 8,000 overflow bytes
per invocation. Overflow bytes are detection bytes and are never admitted as payload.

Rejected: raising the shared store cap (any fixed store-wide number still lets that many unstable
members refuse a stable one), and dropping the store attempt from the key (a member unstable on store
attempt 0 would have no probe left on the retry the store change itself triggers).

Consequence: no wire, snapshot, or issue-code change. The safe-acquisition bound paragraph
(`LOD-V0-019`) states the keying and the bound; `TestOverflowProbeAllowanceIsKeyedAndCappedAcrossInvocation`
asserts the per-key cap of two and that another member, another store attempt, and another store are
not refused, and `TestScanTraceStoreUnstableMemberDoesNotSpendOtherMembersOverflowProbes` drives the
store scan with a member that grows on both store attempts ahead of a stable member. No requirement
IDs added.

Rollback: revert the commit. That restores the store-wide key, the spec sentence, and the old test.
