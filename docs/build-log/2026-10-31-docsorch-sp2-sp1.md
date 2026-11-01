# Lane docsorch-sp2: lane flow docs, sealed-leaf integration, SSH-host procedures

## Problem

Twelve documentation-leaning tickets: agents rediscover lane recoveries from memory notes (V1-3239),
sealed-leaf integration is undocumented (V1-0527), SSH lane hosts lack inputs and a closeout
channel (V1-3252, V1-3262, V1-3620, V1-3610), and several small doc defects.

## Decisions

- Lane flow guide and refusal map live in `docs/LANE-ORCHESTRATION.md` sections 13 to 15, not a new
  file, so existing routes and the charter's section references keep working. Requirement
  `ASD-V0-990` (block 990 to 994) pins the input-name agreement with a shell-test group.
- V1-3258: the docs were right for the dogfood verbs and the Makefile right for `inputs-check`;
  resolved by documenting the one-for-one mapping, not by adding `DOGFOOD_*` aliases to the Makefile
  (scripts are another lane's). The pin test is group V1-3258 in `script/lane-scripts_test.sh`.
- V1-3610: by design no seeded sibling hand-over exists; stated in section 13 (AC1 second branch).
- V1-3620/V1-3262/V1-3252: remote lanes use staged files plus `~/lane-reports/<lane>/` fetched by
  ssh; the existing `CORVINT_TASKS_SNAPSHOT` route (section 12) answers V1-3262 AC2.
- V1-0527: the existing `NOTE sealed-cem-merged` text contradicts the ticket's older reproductions;
  not reproduced here, recorded as such in DOGFOOD.md.

## Evidence

- `HEAVY_MAX_LOAD=100000 script/lane-scripts_test.sh`: 36 groups passed (includes the new group).
- Read-only ssh to the Unraid host showed `~/lanes/heavy.sh` already in the delegating form.
- `make spec-requirements`, `make line-citations-check`: see the lane report.

## Outcome

- Delivers V1-3239 docs part (flow guide, refusal table). AC2 to AC4 NOT DELIVERED (Go drift test,
  refusal audit and check; outside docs scope), V1-3239 stays open.
- Delivers V1-0527 (DOGFOOD.md "Integrating sealed branches").
- Delivers V1-3605 AC2 (doc states real behavior and a host check). Host step NOT DELIVERED.
- Delivers V1-3610 (AC1 states none by design, AC2 in section 13).
- V1-3620: AC1 delivered (named channel); AC2 NOT DELIVERED (needs live SSH lane and orchestrator).
- V1-2563: AC2 delivered (declared difference and a check procedure); AC1 hosted-runner observation
  NOT DELIVERED (needs a `.github/` step run on a hosted runner).
- Delivers V1-3252 (route named), V1-3253 (rule and charter line), V1-3259 (both ACs),
  V1-3258 (AC1 and AC2). V1-3262: AC1 delivered as a convention (dispatch export path); AC2 satisfied
  by the existing snapshot route, but the `journal-absent` refusal wording naming the snapshot was
  not checked or changed (internal/tasks belongs to the tasks lanes).
- V1-0822: NOT DELIVERED. A spike needing a labelled session corpus for precision and recall and
  new `docs/specs/` machinery; no labelled data exists in the repo. Definitions would be: re-reads of
  one file, reads outside the planned set (unplanned-reads ledger), retried commands, reverted edits,
  time to first relevant read; NOT_OBSERVED on missing data; evaluation commands only.
- Dogfood: not sealed. NOT_PRODUCED: prechange receipts only query and impact (docs-only; `dogfood-change`
  not run before edits because no bindable change existed yet).

## NOT_RUN

`make doc-gates` (exceeds the window), Go tests, `make windows-vet` (no Go file changed).

## Rollback

Revert the commits of this lane; the docs and the one test group are independent of runtime code.

## Friction

1. `corvint dogfood propose-citations --help` prints only "Usage:" and no flags (repro: run it);
   the DOGFOOD.md route is the only reference. Confirmed.
2. A ticket cites `ASD-V0-920` as printed by heavy.sh, but the ID is in no tracked spec on the base
   (`git grep ASD-V0-920` empty): the host-line work is unmerged. Confirmed.
3. `make dogfood-change` cannot run before the first edit (it needs a committed change and all
   inputs), contradicting the charter's "run once before the first edit"; this lane produced the
   query and impact receipts by hand. Confirmed.
4. Ticket V1-0527 body still describes `sealed-cem-in-change` for unmodified sealed leaves, while
   DOGFOOD.md says `NOTE sealed-cem-merged`; stale ticket or regression, not reproduced. Suspected.
