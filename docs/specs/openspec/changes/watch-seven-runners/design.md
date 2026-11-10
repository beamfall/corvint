# Design: watch-seven-runners

Status: proposed (2026-11-09). Requirement text lives in `specs/watch/spec.md`.

## Decisions

1. One running process answers for all seven runners, using each runner's contract implementation
   (change `runner-contract`) and the affected engine (G7, owner decision R7 (2026-11-09)).
2. Listing is the default; `--run` runs the tests. `--dry-run` is removed with no alias: no released
   version shipped `cmd/corvint/watch.go` (settled S2).
3. Tiers are `line | file | flow | full` with the `exact` flag (owner decision R16 (2026-11-09)).
   pytest answers carry `file` (owner decision R8 (2026-11-09)).
4. Answers name the runner instance keyed by root (owner decision R15 (2026-11-09), RNC-V1-014).
5. Watch reads the committed inventory and never refreshes it; absent or stale inventory answers
   `inventory-absent` or `inventory-stale` (owner decision R10 (2026-11-09), WCH-V1-014).
6. Kill switch: grammar `full:<runner>` from NMS-V1-008; watch is a CLI surface, so an unknown value
   exits 2 (owner decision R13 (2026-11-09)).
7. The `contextindex` snapshot is kept as a warm cache only if a measurement shows it is needed to
   meet the bar (G25 item 5).
8. Cypress is not a watch runner in 1.0 (owner decision R6 (2026-11-09)).

## Failure modes

- Event source fails or the filesystem has no events (for example `/mnt/c` under WSL): exit 2 with a
  typed error naming `--events-stdin` (keeps LPCV-V0-1087).
- Editors that save by atomic rename: the rename is treated as a change of the target path.
- kqueue file descriptor limit reached on macOS: typed error with the limit and the count, exit 2.
- A runner's state cannot be loaded or is stale: that runner's answer widens to its full command
  with a reason; other runners keep answering.
- Unsure answer: the full selection for the runner, with tier `full` and a reason; never an empty
  list presented as complete.
- Linux inotify watch limit (`fs.inotify.max_user_watches`) reached: typed error with the limit and
  the number of directories still unwatched, exit 2, naming `--events-stdin` (WCH-V1-012).
- HEAD moves while watch runs (branch switch, rebase, large checkout): the planning state no longer
  matches the diff basis; each runner answers its full command with reason `head-changed` until its
  state is reloaded (WCH-V1-010, WCH-V1-011).
- A burst of events from a tool (formatter, code generator, package install): coalesced into one
  answer; ignored paths produce no answer (WCH-V1-011).
- Inventory absent or stale: `inventory-absent` or `inventory-stale` (WCH-V1-014).
- Unknown `CORVINT_SELECTION` value: exit 2 at start (WCH-V1-009).

## Acceptance evidence

- Fixture tests per runner and requirement.
- The save-to-answer harness and its retained reports on both reference machines (WCH-V1-004,
  WCH-V1-005), with the machine, repo commit and Corvint version recorded.
- The snapshot decision record for WCH-V1-007, with both measurements if the snapshot is kept.
- NOT_RUN at the time of writing: everything.

## Rollback

No released version shipped `corvint watch`, so no script depends on the old run-by-default
behaviour or on `--dry-run`. To withdraw the change, revert its commits; the Go-only watch returns.
IDs are retired, not reused.

## Open questions for the owner

- Name the reference machines and the reference repo (the panel proposal is in `proposal.md`).
  Review T-m6 suggests one reference repo per runner, since no single repo of about 500k lines holds
  all seven runners; not decided.
- Accept that today's p50 evidence (about 700 ms, cold start) fails the 300 ms bar until a warm
  save-to-answer measurement exists, and that this may delay 1.0 (risk recorded with G7).
