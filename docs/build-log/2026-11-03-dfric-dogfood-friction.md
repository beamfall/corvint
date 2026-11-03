# Lane dfric: dogfood and lane-tooling friction

Date: 2026-11-03
Base: `14e055d69e3d18979f4eb5e33aae8eb4f16848fd`
Tickets: V1-4406, V1-4441, V1-4442, V1-4443, V1-4444, V1-4401, V1-4291, V1-4440

Eight frictions lanes hit in the change-evidence closeout and lane tooling, each fixed with a
regression. The specs are two companions, because the owning workflow spec holds 256 obligations:
`docs/specs/dogfood-lane-friction-v0.md` (`DCW-V0-2140..2146`) and
`docs/specs/corvint-tasks-lane-ticket-read-v0.md` (`TCP-V0-2230`, the block the charter assigned
to Tasks work). Both are standing acceptance under the owner's 2026-10-11 approval, not owner
read-back.

Decisions:

- V1-4406 (`DCW-V0-2140`). The static pre-check prefers the cached tree build, then a set
  `CORVINT_BIN`, then `PATH`; a set `CORVINT_BIN` without `--static` is a SKIP naming it, not a
  silent fall-back to an installed binary. The stale-binary refusal names a current-tree
  `CORVINT_BIN` always, unsetting it only when set, and `DOGFOOD_STATIC_PRECHECK=0` under
  `--static yes`.
- V1-4441 (`DCW-V0-2141`). A requirement row anchors a code hunk only when the hunk names the ID,
  so the route is two rows per hunk. DOGFOOD section 4 step 2 and the check's fix line say so.
  An explicit `propose-citations --refresh` drafts the anchoring row back beside carried rows that
  anchor nothing; finish-lane's automatic re-key (`DCW-V0-947`) sets `CarryOnly` so a bind never
  gains a row the operator did not review.
- V1-4442 (`DCW-V0-2142`). `make dogfood-retire-traces` is the one rebind step after a reset. It
  applies only a stranded-only dry-run plan with that plan's digest, as finish-lane already does
  (`DCW-V0-406`), and refuses a plan that also rekeys legacy traces. The pre-flight NOTE names it.
- V1-4443 (`DCW-V0-2143`). The expected first bind pass prints `PENDING commit-sidecar` and exits
  3 when every failing row is one the bind commit clears and the only change against HEAD is the
  sidecar; `make dogfood-change` maps 3 to 0. A link row whose map did not prepare because the
  replaced sidecar refused it counts with the OCM rows. finish-lane's nested passes keep exit 1
  and `bindAdmits`, so `dogfood-finish` and `dogfood-closeout` are unchanged. Callers checked:
  finish-lane (in-process, unchanged), `dogfood-closeout` (through finish-lane), the cmd/corvint
  and shell tests that ran a standalone first pass (updated to 3 where only the sidecar is open,
  kept at 1 for a genuine failure such as a missing plan). No lane script reads the code.
  V1-4250 could not be read on this host (V1-4440); nothing in the tree names it.
- V1-4444 (`DCW-V0-2144`). Clean means no change after the bind commit; the lane worktree
  qualifies. `dogfood check` already refused a dirty tree; its refusal now carries a fix line with
  the path count and the definition. AGENTS.md is a governing file and was left unchanged.
- V1-4401 (`DCW-V0-2145`). Generation lists a near-cap spec only when the staged section gains an
  ID since BASE (else HEAD); others are one NOTE count. Capacity mode still lists all.
- V1-4291 (`DCW-V0-2146`). Ranges take one to nine digits; an ID counts only in canonical `%03d`
  spelling; a bare run prints the usage.
- V1-4440 (`TCP-V0-2230`). Reproduced: `ticket show` refuses `MALFORMED` journal-absent for every
  lane ticket. Discovery is correct (common dir `/home/russell/projects/corvint/.git`); that
  primary checkout has no `.git/taskman` journal, and neither the committed snapshot nor
  `origin/main` holds the tickets; the installed binary and a tree build agree. The fix the lane
  can make without store state is the refusal: it now says the primary and all linked worktrees
  share the missing journal, so the `cd PRIMARY_WORKTREE` route cannot work here. Reading the
  tickets on this host still needs a journal or an exported snapshot, which is store state left
  to the orchestrator.

Evidence (focused, through `script/heavy.sh` where Go):

- `internal/dogfoodflow` whole: ok. `internal/tasks/cli` whole: ok.
- `cmd/corvint` `-run 'Dogfood|FinishLane|DCWV|StaleBinary|SelfBinary|OCMV002|TraceGitignore|Aggregate|CKCV|^TestUseCase|Help'`
  and `-run 'ACXV0858|TCBV00|LCPV0810|PortableRun|LocalCompletion|RequireClean'`: ok.
- `tools/flow-guide-check`: ok; `make flow-guide-check`: ok.
- `script/lane-scripts_test.sh` groups tools and admission, `script/gen-spec-requirements_test.sh`,
  `script/dogfood-change_test.sh`, `script/requirement-id-collisions_test.sh`,
  `script/runtime-environment_test.sh`, `script/dogfood-bin-reuse_test.sh`: ok.
- `script/dogfood-cache_test.sh` fails identically at the base (its closeout case meets
  `spec-capacity-check` refusing the fake BASE first); not caused by this change.

Independent review: `script/codex-review.sh` on the bind-ready head `7381f4d02` found two P2
issues, both fixed with regressions: the pending check honoured `core.excludesFile` although the
recorder does not, so a `.corvint/` file hidden only by a personal excludes file could be reported
PENDING (`TestChangeDCWV02143PendingOnlyForTheUncommittedSidecar`); and the static pre-check probed
a relative `CORVINT_BIN` from the caller's directory but ran it from the repository root
(`script/lane-scripts_test.sh` group 33). No other findings in the requested areas.

Limits: the whole `cmd/corvint` package and `make gate` were not run (focused tests per the
charter). Lane time saved is NOT_OBSERVED.
