# Batch M integration (V1-1042, issue 696)

Date: 2026-10-08

Batch M carries V1-1042 (`corvint-tasks pool acquire` and early `pool release` for a live
external-agent attempt, GitHub issue 696) from lane branch `claude/v1-1042-pool-acquire`
(5f275bd4, review fix 30c7c840, based on 38fcf067) onto main ba5c0df3.

Integration decisions:

- Main took `CAL-V0-197` for V1-0827 (decision 0467) after the lane was cut, so the lane's proposed
  `CAL-V0-197..203` are renumbered `CAL-V0-198..204` in the spec, tests (`TestCALV0198_*` ..
  `TestCALV0204_*`), code comments, help text, `docs/TASKS-EXTERNAL-AGENTS.md` and the lane build
  log, in one commit before the merge. No accepted identifier changed.
- The spec header, status line, README row and `INDEX.json` delivery keep both entries; the
  traceability table lists `CAL-V0-196`, `CAL-V0-197` and `CAL-V0-198..204` as separate rows
  (main had joined the 196 and 197 rows on one line).

Evidence: lane focused suite `go test -p 1 ./internal/tasks/...` and vet passed on the lane head;
an independent read-only review of 38fcf067..30c7c840 reported no findings. After the merge the
doc gates, `go test ./internal/specindex`, build, vet and `go test -p 1 ./internal/tasks/...`
are rerun on the batch head.

Limits: CAL-V0-198..204 stay proposed (owner acceptance pending); live qualification NOT_RUN.
