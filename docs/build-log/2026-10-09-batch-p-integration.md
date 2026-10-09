# Batch P integration (V1-1046, V1-1039, V1-0977, V1-0844, decision 0472)

Date: 2026-10-09

Batch P carries five lane branches onto main 02e84575:

- V1-1046 (`claude/v1-1046-lane`, b056326d and review fix adb60e14): the Darwin dashboard process
  wait and the `internal/testsupport` pipe drain treated waitid stop/continue reports (si_code 4..6)
  as exits. Both now keep polling, and their cleanup signals only while the child is unreaped.
- V1-1039 (`claude/v1-1039-lane`, b3d64209): a regression test proves that the MCP CEM report
  refuses a missing cited blob with a valid map without a lazy fetch. No defect was found.
- V1-0977 (`claude/v1-0977-lane`, 5961288e): the last adopter-name mentions in `docs/BUILD-LOG.md`
  are redacted under a recorded privacy exception. Two residuals are intentional: a frozen `/1`
  wire tag (V1-0985) and one sealed CEM.
- V1-0844 (`claude/v1-0844-lane`, 3d76ba3b and review fix a1364d19): a deterministic subtest shows
  that an enrolled, incomplete Stop evaluation that decides `block` fails open with
  `dogfood-event-deadline` only through production deadline enforcement. A live HLQ diagnostic run
  and a 40-run timing probe are retained.
- Decision 0472 (`claude/owner-acceptances-0472`, df494abb): the owner accepted `CAL-V0-198..204`,
  `LCP-V0-018` and the V1-0500 outcome (`SESSION-V0-020..022`) in chat on 2026-10-09.

The lanes merged without conflicts.

Evidence:

- Each code lane had an independent read-only review. V1-1046 and V1-0844 failed their first review
  and passed after their fixes; V1-1039 and V1-0977 passed first time. The decision lane is
  docs-only and was read back before integration.
- Lane focused tests passed on each lane head; the bind plan reruns the doc gates, specindex, vet and
  the focused packages on the batch head.

Limits:

- Linux runtime for the waitid paths, live CAL qualification and real-host fail-open under load are
  NOT_RUN. `make gate` is NOT_RUN.
- V1-1036 (pre-reap group signals) failed its review and stays out of this batch. V1-1047 (four more
  post-reap signal sites) is still in progress.
