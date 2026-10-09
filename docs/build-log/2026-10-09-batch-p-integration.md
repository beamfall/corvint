# Batch P integration (V1-1046, V1-1039, V1-0977, V1-0844)

Date: 2026-10-09

Batch P carries five lane branches onto main 29cc7fd4 (lanes cut at 02e84575; batch J merged in between):

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

Decision 0472 was merged here too, but the batch then reached 276 obligations across five intent
specs (agent leases 193, local completion policy 18, session context dividend 22, MCP server 34,
host lifecycle 9), over the change-evidence binder's 256 cap. Commit "batch P: defer decision 0472"
restores every path the decision 0472 merge changed to `29cc7fd4`, so the batch carries two intent
specs (43 obligations). Decision 0472 comes back in a follow-up PR that reverts that commit on top
of this batch, as V1-1028 did after batch H. Its lane head `df494abb` stays in this history.

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
