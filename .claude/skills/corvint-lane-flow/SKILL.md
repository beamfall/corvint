---
name: corvint-lane-flow
description: The end-to-end flow for one Corvint build lane or change (orient, build, focused tests, doc gates, dogfood inputs, report) and the exact recovery for each common refusal. Use when starting, finishing or unblocking Corvint development work in this repository.
---

# Corvint lane flow

The canonical guide is section 13 of `docs/LANE-ORCHESTRATION.md` ("Lane flow guide and refusal
recovery map"); read it first, and the section it names for each step. `make flow-guide-check`
checks this skill and that section against the real CLI, Makefile and scripts (ASD-V0-996).

1. Orient: `corvint index --if-stale`, `corvint affected --base FULL_SHA`, and
   `corvint context --task "..."`. Record the pre-change receipts once, before the first edit:
   `make dogfood-change BASE=FULL_SHA`.
2. Build in your own worktree. When behavior changes, put the spec amendment in its own commit
   first, then `make spec-requirements`; check new IDs with `make requirement-ids-free`.
3. Test through `script/heavy.sh`, one package per command, `set -o pipefail` when piped.
4. Before the final commit: `make doc-gates`, `make line-citations-check`, `gofmt -l`; repin with
   `make repin` and `script/repin-use-case-receipts.sh`.
5. Dogfood inputs: `corvint dogfood propose-citations BASE`, review it into `CITATIONS`, write
   `INTENTS` and `VERIFY`, then `make dogfood-inputs-check BASE=.. CITATIONS=.. INTENTS=..
   VERIFY=.. OUTCOME=passed`.
6. Report: head SHA, one line per ticket, tests and results, and the Friction list.

A refusal names its recovery: read the `fix:` line it prints, or find its code in the guide's
refusal table. A refusal with neither is a defect: report it in the Friction list.
