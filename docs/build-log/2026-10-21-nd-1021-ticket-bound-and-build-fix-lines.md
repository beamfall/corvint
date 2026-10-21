# 2026-10-21 lane nd-1021: plan-fixture ticket bound and build-failure fix lines

## Problem

- V1-0912: `corvint work plan-fixture` (internal/taskman) refused any capture or plan above 1,000
  tickets, entries or blockers, below the Tasks per-queue bound of 10,000. Decision 0548 directs
  raising the bound to 16,384, stated in the spec and in `--help`, tested on both sides.
- V1-1929: `dogfood-batch-check`, `dogfood-check` and `dogfood-finish` refused
  `current-tree-corvint-build-failed` / `base-corvint-build-failed` with no `fix:` line, unlike
  the other dogfood refusals; a batch of sealed constituents that do not compose had no named
  recovery.

## Decisions

- `NTP-V0-246`: one constant `maxTickets = 16384` bounds the capture page total, the queue ticket
  count, plan entries and per-entry blockers. The 16 MiB per-response and 32 MiB ticket aggregate
  byte caps still apply; above the bound capture and decode refuse, nothing is truncated.
  `resourceDeferred` stays at 1,000. `corvint work plan-fixture --help` states the bound.
- `DCW-V0-332`: one shared helper `dogfood_build_fix` in `script/dogfood-build-corvint.sh` prints
  the recovery after each build-failed refusal: a tree failure names the compile tail and the new
  `docs/DOGFOOD.md` "Non-composing constituents" paragraph (rebase the later constituent onto the
  batch head, fix, reseal with `BASE=<batch head>`, merge); a base failure says no change can be
  sealed on a non-building `BASE`. Exit and reason codes are unchanged.
- Both requirements are accepted under the owner's standing auto-accept of lane requirements
  (2026-10-11).

## Evidence

- `TestNTPV0246TicketBound` (internal/taskman) passes; with the entries bound at 1,000 it fails
  (`a plan of 16384 entries refused`).
- `TestNTPV0246PlanFixtureHelpStatesBound` (cmd/corvint) passes; against the base
  `taskman_fixture.go` it fails.
- `script/dogfood-cache_test.sh` passes; it now covers change, check and batch-check tree build
  failures and the check base build failure, asserting one `fix:` line each. With the batch-check
  helper call removed it fails.

## NOT_RUN

- `make gate` (owner standing preference: focused tests only).
- `dogfood-finish.sh` build failure is not exercised by the cache test (it needs a full lane
  input set); its two call sites are identical to the tested check ones.

## Rollback

Revert this change: the bound returns to 1,000 and the build-failed refusals lose their `fix:`
line. No artifact or format needs migrating.
