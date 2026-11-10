---
name: corvint-lane-flow
description: The end-to-end flow for one Corvint build lane or change (orient, build, focused tests, doc gates, report) and the exact recovery for each common refusal. Use when starting, finishing or unblocking Corvint development work in this repository.
---

# Corvint lane flow

The canonical guide is section 13 of `docs/LANE-ORCHESTRATION.md` ("Lane flow guide and refusal
recovery map"); read it first, and the section it names for each step. `make flow-guide-check`
checks this skill and that section against the real CLI, Makefile and scripts (ASD-V0-996).

1. Orient: run `script/lane-corvint.sh` and use its worktree executable for reads; then `corvint index --if-stale`, `corvint affected --base FULL_SHA`, and
   `corvint context --task "..."`. Read tickets on any host, journal or not, with the
   dispatch's staged snapshot: `CORVINT_TASKS_SNAPSHOT=TICKET_DIR/queue-snapshot.json corvint-tasks
   ticket show <ID>`; screen lane branches with `corvint-tasks ticket delivery-screen <ID>...
   --lane-refs` (add `--unaudited` on a host without the Tasks journal). See section 12.
2. Build in your own worktree. When behavior changes, put the spec amendment in its own commit
   first, then `make spec-requirements`; check new IDs with `make requirement-ids-free`.
3. For an uncommitted unrelated-path control, use `corvint affected --worktree --path PATH`.
   For full foreground cmd/corvint coverage use `script/go-test-shard.sh ./cmd/corvint/ 1 8` and
   indexes 2 through 8 on unchanged inputs. Test through `script/heavy.sh`, one package per command, `set -o pipefail` when piped.
4. Before the final commit: `make doc-gates`, `make line-citations-check`, `gofmt -l`; repin with
   `make repin` and `script/repin-use-case-receipts.sh`.
5. Report: head SHA, one line per ticket, tests and results, and the Friction list.

A refusal names its recovery: read the `fix:` line it prints, or find its code in the guide's
refusal table. A refusal with neither is a defect: report it in the Friction list.
