# Corvint Tasks lane flow, wave 4 — V1-3569, V1-3463, V1-3576, V1-3331, V1-2909, V1-3577

Spec: `docs/specs/corvint-tasks-lane-flow-v0.md` (TLF-V0-880 to TLF-V0-889). Base origin/main
`8daec98ce4349c1583186692726d2c3a782ac647`.

## Delivered

- V1-3569 and V1-3463, progress phase (TLF-V0-880 to TLF-V0-883): `ticket progress` records an
  append-only phase entry (`PROGRESS_SET`, in the OPERATOR row of the default role matrix and
  admitted under an OPERATOR/ADMISSION barrier, which still refuses claims). The effective entry
  is the latest one that is not `CLEARED`, and only on a live (DRAFT, OPEN, HELD) ticket.
  `ticket search --phase` (with `NONE`), roadmap rows and `ticket show` carry it. `closeout-debt`
  reports an OPEN ticket whose effective head is a scanned commit as citation `progress-head`.
- V1-3576, effects proposals (TLF-V0-888, TLF-V0-889): `ticket effects-proposals` is a read. For
  each OPEN ticket with effects coverage `UNKNOWN`, it proposes existing repository paths named
  in the title, body and acceptance criteria, with the naming span. It is labelled "proposal:
  not authority", sets `mutationAuthority: false` and writes nothing.
- V1-3331, delivery-screen lane heads (TLF-V0-887): `ticket delivery-screen --lane-refs` also
  scans local branches and detached worktree heads. It reports `DELIVERED_ON_LANE` with
  `laneCommits`, and reports `IN_FLIGHT` for a progress phase past IN_PROGRESS.
- V1-2909, requirement-ID collisions (TLF-V0-886): `go run ./tools/requirement-id-collisions`
  compares requirement bullets in `docs/specs/*.md` across lane heads and the base. On the real
  repository it read 27 lane heads in 2.5 s, found 0 collisions and exited 0.

## Decisions

- `--lane-refs` counts only delivery references, meaning a delivery line or a ticket-scoped
  subject (TCD-V0-167). A plain citation on a lane is not delivery.
- `IN_FLIGHT` phases are DELIVERED_ON_BRANCH, IN_REVIEW, IN_MERGE_QUEUE and
  MERGED_AWAITING_CLOSEOUT. BLOCKED and IN_PROGRESS are not in flight.
- A lane head is a local branch or detached worktree head that the base does not contain and
  that shares a root commit with it. The primary checkout keeps about 1209 branches of the
  pre-snapshot private lineage, which has no merge base with origin/main (decision 0331).
  Without the root filter both reads would exceed the 256-head bound and refuse. With it,
  delivery-screen on the real repo reads 31 heads in 3.3 s. More than 256 heads is still refused
  `LIMIT_EXCEEDED` rather than cut.
- Each lane-head scan is bounded by `--limit` commits outside the screened ref. A truncated scan
  adds a warning.
- requirement-id-collisions treats amending an existing ID's text on one lane as an amendment,
  not a collision. It reports three collisions: an ID defined twice in one ref, an ID defined in
  a different file than on the base, and an ID new on two refs with different definitions.
- Merge point for the progress-head citation: `internal/tasks/cli/closeout_debt.go`. Other lanes
  touching closeout-debt rows should rebase onto this change.

## BLOCKED

- V1-3576 criterion 2, the operator batch set-effects pass. Applying the proposals is a store
  write, and lanes may not write the Tasks store. The operator runs it after reviewing the
  proposals.

## NOT DELIVERED

- V1-3577, `ticket split`, for budget. A split needs one atomic mutation that creates N children
  and retires the parent. The transaction model admits one post-state per mutation, so this is a
  model change, not a command. TLF-V0-884 and TLF-V0-885 stay "proposed, not implemented" in the
  spec.

## Friction

1. `script/heavy.sh` waited 120 s for host load on several runs (load 101 to 144 against
   `HEAVY_MAX_LOAD=10`). Repro: run any focused `go test` through heavy.sh while lanes are busy.
2. heavy.sh refuses whole-package runs without `-run`. That is policy, but there is no
   documented focused-run shortcut for a package's new tests.
3. `make spec-requirements` reports digest errors one at a time. Repro: introduce two malformed
   requirement bullets, then run it twice.
4. `corvint-tasks ticket show` nests the record under `items[0].record`, unlike search rows.
   Repro: compare `ticket show ID` with `ticket search` output shapes.
5. The closed `ticket progress` payload reader requires every nullable key to be present. Repro:
   omit `"pr":null`, and the command is refused MALFORMED.
6. `flags()` in `internal/tasks/cli/cli.go` has no boolean-flag support, so `--lane-refs` is
   stripped by hand before parsing.
7. delivery-screen has no `--json` flag. Its output is always JSON, so passing `--json` (as other
   corvint commands accept) is refused MALFORMED "unknown flag --json".
8. `make dogfood-change` retires 3 stale traces on every run.
9. The installed `corvint` reports DEGRADED `self-binary-stale`. The release check was not run
   in this lane: corvint 1.0.0-rc.3.dev build 34 (dbc17137), corvint-tasks
   0.0.0-tcp01-unverified+build.34.
10. The issue502 fixture has no regenerate flag.
11. The primary checkout's 1209 private-lineage local branches lie outside origin/main. Any tool
    listing local heads must filter by shared root.
12. The shared notes folder was empty at lane start, so no sibling hand-over existed.
