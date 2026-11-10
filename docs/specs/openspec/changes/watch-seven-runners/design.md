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
9. Warm state (watch-flows verdicts, findings 3, 5, 7, 9 and 13; selection verdicts, finding 4).
   Watch keeps five things between saves: the package list, each runner's identity, a per-file blob
   table, the old-side blobs of the diff basis, and the evaluated flow links. WFF-V1-002 (change
   `watch-flows-hardening`) says when each is dropped; this change adds no second rule. Between
   saves only the changed paths are rescanned (WCH-V1-007).
10. Blobs are read through `BlobReader` and hashed in process; watch spawns no `git` process per
    save for them.
11. Flow link evaluation counts inside the Playwright save-to-answer budget (WCH-V1-004,
    WCH-V1-013); it is not timed apart.
12. A new save cancels an in-flight `--run`. The cancelled run is reported incomplete, never as
    passed.
13. Event registration is pruned by the ignore rules of WCH-V1-011: an ignored directory is not
    watched.
14. Changed paths travel on the event source channel, and `--events-stdin` accepts one path per
    line, so watch does not rediscover what changed.
15. Each answer carries `changed[]` and, per test, a `because` member, under a size cap. The cap's
    value is not set here; it is fixed with the answer profile.

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

- Decided, R42 (panel pick) A11: the owner's Mac and WSL2 (ext4) on the benchmark box, cores
  capped at 8; the seven pinned proof repositories, one per runner, sizes printed; Go also timed on
  the Corvint repository (WCH-V1-004).
- Settled (decided 2026-11-09 under R42, A15): a runner that misses the bar at the freeze blocks
  the release unless the owner recorded a demotion for it before the freeze; a demoted runner ships
  `watch` as preview with its measured p50 and p95; Go, Vitest and Jest cannot be demoted on this
  bar; fewer than four of seven passing means the release slips (WCH-V1-004, G10). Any demotion
  note is the owner's act.
- Accept that today's p50 evidence (about 700 ms, cold start) fails the 300 ms bar until a warm
  save-to-answer measurement exists, and that this may delay 1.0 (risk recorded with G7).
