# Decision 0284 — Dashboard overflow probes reset per whole-scan attempt

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-13).

`compileSnapshotWith` creates one source budget before its two-attempt whole-scan loop. The budget
kept logical reservations, physical bytes, and overflow-probe allowances in the same object. Decision
0276 separated trace-member probes by member and store attempt, but the key did not include the
outer whole-scan attempt. A member that spent both probes during the first scan therefore began the
repository-stability retry with no overflow allowance. Its next bounded read became fatal
`DASHBOARD_RESOURCE_EXHAUSTED`, although the retry exists because the first repository observation
changed and its output was discarded.

The call: each actual whole-scan attempt receives a fresh overflow-probe allowance backed by the
same invocation-wide logical reservation and physical-byte ledger. A repository-start failure that
runs no scan spends no attempt allowance. Starting a second scan resets no source reservation,
artifact count, logical byte, physical byte, deadline, process, or repository counter.

A singular configured artifact may spend two overflow bytes per whole-scan attempt, four over the
two-attempt invocation. A trace member may spend two per store attempt per whole-scan attempt. The
existing maxima therefore remain 2,000 overflow bytes per configured store attempt and 8,000 per
configured store per invocation. Overflow bytes remain detection bytes and are never admitted or
refunded as payload.

Rejected: retaining the invocation-wide allowance, because a discarded observation can disable the
only retry; and threading the outer attempt index into every source key, because an attempt-scoped
budget view establishes the same bound without coupling the source package to the CLI retry index.

Consequence: no wire, snapshot, issue-code, or requirement-ID change. `LOD-V0-019` distinguishes the
attempt-scoped probe allowance from the monotonic invocation ledger.
`TestCompileSnapshotRetriesWithAttemptScopedProbesAndCumulativeSourceLedger` drives both attempts
through `adapters.Scan`; `TestOverflowProbeAllowanceIsKeyedAndCappedPerWholeScanAttempt` proves that
the same key receives a fresh allowance while physical bytes remain cumulative.

Rollback: revert the commit. That restores an invocation-wide overflow-probe map and the retry
failure described above.
