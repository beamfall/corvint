# Lane orchestration procedure in the repository: V1-1627..V1-1632

Human-owned intent: tickets V1-1627, V1-1628, V1-1629, V1-1630, V1-1631 and V1-1632 (P0 chores
from the 2026-10-17 lane cost review). New requirements `ASD-V0-013..018` in
`docs/specs/corvint-self-development-v0.md`, accepted under the owner's standing auto-accept of
lane requirements (2026-10-11). No `ASD-V0` ID above 012 existed on `origin/main`
(`2ab0e74421147cb16d2da183419e1ad2d991fa52`) or in the 14 open PRs.

Delivered: `docs/LANE-ORCHESTRATION.md` (the de facto orchestrator charter, generalised and linked
from `AGENT-ROUTES.md`, `DOGFOOD.md` and `SELF-DEVELOPMENT.md`); `script/heavy.sh`;
`script/lane-idle-check.sh`; `script/lane-clean.sh`; a missing-input refusal in
`script/dogfood-finish.sh` (`make dogfood-finish LANE=1`); `make lane-scripts-test`.

Decisions:

- Host pressure (V1-1627): the dispatcher throttle (`CAL-V0-068`, V1-0694) admits only workers that
  `corvint-tasks dispatch` launches; agent lanes are spawned by the orchestrator session, so its
  admission cannot see them. The procedure reuses the same inputs (load per CPU, Darwin
  memory-pressure level or Linux swap), classification and two-sample hysteresis, applied by the
  orchestrator at heartbeat. Lane thresholds (1.5 / 3.0 load per CPU) are stricter than the
  dispatcher example because each heavy lane already runs `-p 4` tests. Sample at authoring time:
  `vm.loadavg` 14.51 on 12 CPUs (1.2 per CPU), memory-pressure level 1.
- Semaphore (V1-1628): the prototype `/private/tmp/cem10-build/heavy.sh` used `mkdir` slots with a
  PID file and removed a dead owner's directory. Two waiters reclaiming the same dead slot could
  remove a slot a third process had just taken. `script/heavy.sh` uses `flock(2)` through perl
  instead, so the kernel releases a dead holder's slot and there is no reclaim step. Default
  `HEAVY_SLOTS` is CPUs/4 (3 here) and `HEAVY_P` CPUs/slots (4 here), the values the charter used by
  hand. The two forms do not see each other's slots, so a host cuts over all lanes at once.
- Missing inputs (V1-1629 AC2): `make dogfood-finish` defaults `BASE` to `main` (Makefile line 1),
  so its old `BASE is required` test never fired, and it stopped at the first missing input. The
  script now names every missing or unusable input in one refusal before any build. INTENTS,
  VERIFY and OUTCOME stay optional for `DCW-V0-060` callers; `LANE=1` requires them and a full
  40-hex BASE.
- Checkouts (V1-1632 AC1): the "plain clone, not a worktree" lane rule predates `DCW-V0-044`
  (2026-10-11, `TestDogfoodDCWV0044WorktreeLanesRunConcurrently`), which runs change, bind, check
  and seal concurrently in linked worktrees with per-worktree trace stores. Only borrowed-object
  clones (`--shared`, `--reference`) are refused (`unsupported-object-alternates`). The procedure
  therefore uses `git worktree add` from one shared plain clone, with a fresh plain clone as the
  fallback until a live lane qualifies `dogfood-finish` and `dogfood-batch-check` in a worktree.
  This lane itself used a plain clone, as its charter required.
- Idle replay (V1-1631 AC3): `script/lane-idle-check.sh --gaps` over this orchestrator session's
  721 subagent transcripts found 26 gaps over 30 minutes, the longest 976 min (16.3 h) from
  2026-10-15T14:30:49Z in `agent-a29be16da3943a9e4`, (inferred to be the parked docs lane the ticket cites), which the watchdog would have flagged at
  15:00:49Z (30 min in; within 40 min at a 10-minute heartbeat). Only that gap exceeded 15 h.

Evidence: `make lane-scripts-test` passes 6 groups (heavy.sh serialisation with `HEAVY_SLOTS=1`,
status and `-p` pass-through, dead-owner release; finish refusal; idle and gap detection; lane-clean
dry run and apply over clones and a linked worktree). Start-of-lane `make dogfood-change` rc=2
(no change yet, `outcome-input-not-provided`; 10 s).

NOT_PRODUCED, with the procedure in `docs/LANE-ORCHESTRATION.md` section 7: batch medians of load
and focused-test time under the cap (V1-1627 AC3); the 5-lane timing within 1.5x of quiet
(V1-1628 AC3); per-lane dogfood invocation counts (V1-1629 AC3); polling time and tool-timeout
counts (V1-1630 AC3); the one-off merged-lane cleanup with counts and free disk (V1-1632 AC3,
orchestrator-run). Live qualification of a dogfood finish in a linked worktree is NOT_RUN.
