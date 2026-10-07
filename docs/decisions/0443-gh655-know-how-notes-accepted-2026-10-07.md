# Decision 0443 — issue 655 know-how note requirements accepted

Date: 2026-10-07. Status: accepted by the owner. On 2026-10-07 the owner replied "accepted" to the
orchestrator's report of the know-how note requirements and their fail-closed policy defaults.

## Context

Issue [655](https://github.com/beamfall/corvint/issues/655) asked for shared, anchored agent
know-how across tickets. The V1-0955 lane proposed `KHN-V0-001`..`007` in
`docs/specs/corvint-tasks-know-how-notes-v0.md` and an agent addendum to decision 0397 that lets the
know-how writer reuse `internal/secretscreen`. Codex reviewed the lane in three rounds until it
reported no finding. AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts, as written at branch `claude/gh655`:

| Item | Where | Ticket | Summary |
|---|---|---|---|
| `KHN-V0-001`..`007` | `corvint-tasks-know-how-notes-v0.md` | V1-0955 (issue 655) | Notes pinned to file blobs, receipt-backed add/supersede/retract with immutable history, write-time secret screen, read-time STALE/UNKNOWN freshness, 2 KiB claim delivery labelled untrusted, no authority |
| Resolved decisions | same spec | V1-0955 | The lane's fail-closed design choices |
| V1-0955 addendum | decision 0397 | V1-0955 | Two-file `internal/secretscreen` import edge for the know-how writer |

It also answers owner questions 1 to 4 and 7 by keeping the current behaviour: TEA-style ledger
history; no notes on COMPLETED home tickets; supersede and retract within the home ticket only; no
WORKER grant; bounds of 32 entries, 1024 text bytes, 4 anchors, 4 routes and a 2 KiB claim cap.
An identical retry replays only with the same `--commit` and `--issued-at`. Questions 5 and 6 stay
open as V1-0964 and question 8 as V1-0962.

## Limits

This decision settles intent only. Delivery stays experimental:

- The archive round trip, concurrency, redo, UNAVAILABLE-delivery and commit-race witnesses are
  not built (V1-0964).
- `make gate` and the full `go test ./...` were not run; a full `internal/tasks/store` run at the
  lane head timed out under host load (V1-0961).

## Rollback

Revert this decision and restore the "proposed, pending owner acceptance" markers in
`corvint-tasks-know-how-notes-v0.md`, `docs/specs/README.md`, `docs/specs/INDEX.json` and the
decision 0397 addendum, then regenerate `docs/specs/REQUIREMENTS.tsv`.
