# 2026-10-16 V1-1517: dated note on the CAL-V0-120 TTL failure mode

Lane V1-1517 (DOC). `docs/specs/corvint-tasks-agent-leases-v0.md` CAL-V0-120 "Failure modes" still
said a heartbeat TTL change fences live evidence handoffs `STALE_POLICY`. CAL-V0-123 (decision 0439)
makes a `holderLiveness`-only change handoff-compatible. PR #990 (V1-1511) corrected
`docs/TASKS-EXTERNAL-AGENTS.md` only and did not touch this sentence.

## Decision

Add a dated note after the CAL-V0-120 failure-mode paragraph. It points to CAL-V0-123 for the
handoff reading and to `completionBlocker` in `internal/tasks/transaction/lease_gate.go`, which still
refuses `STALE_POLICY` for an attempt claimed under an earlier policy. Accepted requirement text is
unchanged, no requirement ID is added, and older build-log entries are left as historical.

## Evidence

Diff is the note only (5 lines). Doc-gates run locally; see the PR.
