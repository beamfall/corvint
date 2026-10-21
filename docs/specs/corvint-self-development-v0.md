# Corvint Self-Development V0

Owner: Russell Lewis
Date: 2026-09-08
Intent status: accepted direction (owner requested Corvint fully involved in its future development, 2026-09-08); ASD-V0-013..020 accepted under the owner's standing auto-accept (2026-10-11) (V1-1627..V1-1632, V1-1755, V1-1910)
Delivery status: experimental
Authoritative inputs: `AGENTS.md`, `docs/DOGFOOD.md`, `docs/LANE-ORCHESTRATION.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`,
`docs/specs/local-completion-policy-v0.md`, `docs/specs/go-production-kernel-migration-v0.md`

## Agent digest
- Claim: Corvint development selects applicable existing feature routes, preserves real use evidence, and finishes through the existing local completion policy.
- Status: accepted direction (owner requested Corvint fully involved in its future development, 2026-09-08); ASD-V0-013..020 accepted under the owner's standing auto-accept (2026-10-11) (V1-1627..V1-1632, V1-1755, V1-1910)/experimental
- Exists: `docs/SELF-DEVELOPMENT.md`, thin Codex/Claude skill routes and native feature/eval help.
- Blocked on: final exact-target checks, independent review and enrolled strict finish; formal FULL and protected-runner authority remain separate.
- Read next: Requirements; Acceptance and evidence; Failure and rollback.

## User job and current boundary

An agent developing Corvint should use the applicable existing retrieval, impact, evidence, review,
verification and learning features rather than relying only on automatic prompt context. The
simpler baseline remains the existing dogfood workflow with task-specific manual CLI selection.
This slice makes that selection discoverable; it does not create another planner or require every
command on every task. Generic host skills use the shared guide only for Corvint or explicit local
adoption; unrelated repositories keep their own workflow.

The qualified prior source `7d1297bae60081c3023571683c2a4fe4dbcc29ab` already supplies native local
completion, CEM/OCM binding and the feature families. Help lacked feature/eval topics despite their
production dispatchers, and prove prose contradicted its separate committed-range mode. The
current change repairs discovery and records actual route use, without adding language/platform,
profile, execution or closing-authority semantics. Compatibility remains FALLBACK.

## Requirements

- `ASD-V0-001`: One shared repository-owned stage guide maps every inventoried family to an applicable trigger, exact admitted surface, existing authority, retained evidence and explicit exclusion. This is practical self-use coverage, never formal FULL.
- `ASD-V0-002`: Both generic host skills use the shared guide only when repository-owned instructions opt in and the repository-owned guide exists. Corvint self-development opts in explicitly; unrelated repositories retain existing fallback without looking for Corvint-only documentation. Preserve native identity and explicit Claude key/root rules. Read-only questions select relevant read routes without requiring the full self-use loop.
- `ASD-V0-003`: A real source-change slice captures original task context, appropriate impact and dirty affected advice, then challenges relevant evidence with prove. Source reads and chosen tests follow the returned evidence or retain an explicit reason for widening.
- `ASD-V0-004`: Pre-acknowledgment review inspects relevant bound frontier/0 output alongside existing CEM/OCM reports. Valid open frontier remains advisory and cannot satisfy, block or upgrade formal authority.
- `ASD-V0-005`: Reuse existing native receipts and LCP selected-check logs/report-set bindings. Considered/used/unavailable/deferred/not-applicable are caller review prose in the existing task evidence record, with original artifact pointers and bounded concrete reasons; do not extend strict LCP/receipt schemas or add a ledger.
- `ASD-V0-006`: Used means an actual retained command result with root/revision/profile/exit identity; a suggested argv, fixture test or generic check pass is not evidence of actual task use. Existing unknowns, omission counts and failed attempts remain visible.
- `ASD-V0-007`: Optional execution, migration, signing and service routes obey their own permission and containment contracts. No inferred authority bundle, blind-v4 access, new network service, automatic mutation campaign or detached refresh.
- `ASD-V0-008`: Learning closure reuses the explicit outcome; no duplicate record. A learned-mechanism/scoring change runs LTA registered two-arm evaluation; routine explicit record does not need per-row admission evaluation.
- `ASD-V0-009`: Repair feature/eval discovery and contradictory prove usage prose against existing dispatch/profile tests. Help and skills may expose implemented candidate slices but cannot broaden platform/language/revision admission.
- `ASD-V0-010`: Reuse existing relevant CLI profile tests and documentation/package/spec validators for guidance and help changes. Add a focused semantic regression only for an evidenced uncovered misroute or public CLI behavior change; do not add string-presence or implementation-mirroring tests.
- `ASD-V0-011`: Demonstrate applicable routes on the actual enrolled implementation task with original CLI outputs, native host/session identity, selected check observations, independent review and strict finish. Record unobserved Claude/Codex use as NOT_OBSERVED; future host guidance does not claim guaranteed automatic adoption or require an unrelated model trial before guidance can land.
- `ASD-V0-012`: A future feature change updates the shared guide/status and its relevant route witness in the same change. Reviewer intent audit checks missed applicable families; no nominal percentage based on executed CLI count. Rollback removes routing additions without weakening existing dogfood completion.
- `ASD-V0-013`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1627) Lane admission caps concurrently running heavy lanes (lanes that run Go builds or tests) by the host-pressure signal of `CAL-V0-068` (one-minute load per CPU plus the Darwin memory-pressure level or Linux swap), sampled by the orchestrator at each heartbeat with two-sample hysteresis: at most 4 calm, 3 high and 1 critical on the reference 12-core host. An unreadable sample keeps the current level. Pressure holds only new heavy launches; running lanes are not stopped for it and light lanes are not counted. The procedure states why the dispatcher's own admission does not apply (agent lanes are not dispatcher workers).
- `ASD-V0-014`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1628) `script/heavy.sh` admits each heavy Go command through a host-wide counting lock with a configurable slot count (`HEAVY_SLOTS`, default CPUs/4), lock directory (`HEAVY_LOCK_DIR`) and Go parallelism bound (`HEAVY_P` exported as `GOFLAGS=-p=N` unless `-p` is already set). The command inherits the slot lock and runs in its own process group: the slot stays held until every process that kept the lock has exited, even when the wrapper is killed, is released when they exit for any reason, and INT/TERM/HUP are forwarded to the whole group; the wrapper returns the command's exit status, and an invalid limit is refused with exit 2. Lane charters run every heavy Go command through it.
- `ASD-V0-015`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1629) A lane fixes BASE, citation plan, intents, verify file and outcome before work starts and runs the post-commit finish (`make dogfood-finish`, `DCW-V0-060`) once after its last edit, at most twice. `script/dogfood-finish.sh` refuses before any build, in one line, naming every missing input, every named input file that is not a readable regular file, and an invalid outcome; with `LANE=1` it also requires INTENTS, VERIFY, OUTCOME and a full 40-hex BASE.
- `ASD-V0-016`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1630) Lanes run no sleep or wait loops and no background jobs; CI and merge-queue waits belong to the orchestrator. `docs/LANE-ORCHESTRATION.md` lists the commands known to approach the 600 s tool limit with their phases and the bound a lane applies to each.
- `ASD-V0-017`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1631) The orchestrator's heartbeat runs the read-only `script/lane-idle-check.sh` over each running lane's transcript or output and flags a lane idle for more than 30 minutes; last activity comes from RFC 3339 transcript timestamps (Z or numeric offset), or the modification time of a file with no timestamp key, and a transcript that cannot be read or holds an unparseable timestamp makes the lane UNKNOWN, flagged like idle (also in `--gaps`), never a silent modification-time fallback; a flagged lane is messaged once, then stopped or redispatched with its claim released and the reason retained. `--gaps` replays transcripts and reports when each idle gap would have been flagged.
- `ASD-V0-018`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1632) Lanes use one checkout each, created once from a shared plain clone with `git worktree add` (a fresh plain clone is the fallback) and reused for every check of the lane. `script/lane-clean.sh` is dry-run by default and with `--apply` removes only checkouts under the given root whose HEAD and local branches are ancestors of the reference ref, whose status is clean, with no stash, no registered linked worktree (for a clone), and not listed as active; any Git inspection that fails keeps the checkout, and a root that does not resolve to a directory other than `/` is refused; it reports counts and free disk before and after.
- `ASD-V0-019`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1755) The dogfood make targets (`dogfood-change`, `dogfood-check`, `dogfood-seal`, `dogfood-finish`, `dogfood-batch-check`, `dogfood-bind-range`) never use the Makefile's file default `BASE ?= main`: a BASE that does not come from the command line or the environment is treated as missing, and the target refuses with exit 2 and a message naming BASE before any script runs (`dogfood-finish` names it in its one-line missing-input refusal). Non-dogfood targets keep the default.
- `ASD-V0-020`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1910) `script/check-goos-filenames.sh` (`make goos-filenames-check`, a `make gate` step) fails, naming each file and a rename such as `_jsts.go`, when a tracked Go file (test or not) under `cmd/` or `internal/` has a filename suffix that Go reads as a GOOS/GOARCH build constraint (Go's known lists; the first name element is ignored, as Go does) for a platform other than darwin, linux, windows, amd64 or arm64, unless the path is listed in `script/goos-filename-allowlist.txt`.

## Non-goals and trust boundary

No feature-count target, universal runner, strict LCP schema change, new ledger/database, service,
protected principal, signing key or automatic command fanout. No sealed evaluation input, transcript
collection, silent trace migration, fabricated provider attachment, authority promotion or guaranteed
future model adoption. Existing root/key, full-frame/privacy, optional execution and cleanup contracts
remain controlling. Feature routes preserve failed, unknown, withheld and unavailable observations.
The lane procedure (`ASD-V0-013..020`) is operator guidance and helper scripts: it adds no Tasks
store state, no daemon, no automatic lane stop and no product command, and its scripts are never
inputs to ranking, evidence or authority.

## Acceptance and evidence

| Requirement | Implementation and actual evidence |
|---|---|
| ASD-V0-001, ASD-V0-002, ASD-V0-007, ASD-V0-008, ASD-V0-012 | Shared guide, both installed-package source skills and scoped independent review. Ordinary repository behavior and existing local policy remain unchanged. |
| ASD-V0-003, ASD-V0-004, ASD-V0-005, ASD-V0-006, ASD-V0-011 | Original task context/impact/affected/prove outputs, bound frontier and CEM/OCM reports, existing frozen LCP check observations and final strict finish. Before final binding, missing reports remain NOT_PRODUCED. These are task observations, not native-host conformance. |
| ASD-V0-009, ASD-V0-010 | Existing `TestHelpInvocationsAreDeterministicAndDoNotInspectRootOrStdin` gains production-dispatch cases for feature/eval help with an invalid root and forbidden stdin. Existing feature/eval/profile tests retain admission boundaries; canonical documentation/package/spec validators cover integration. |
| ASD-V0-019 | `Makefile` `DOGFOOD_BASE`; `make lane-scripts-test`: each dogfood target refuses without BASE, passes an explicit or environment BASE to its script, and `dogfood-finish LANE=1` lists BASE among the missing inputs. |
| ASD-V0-020 | `script/check-goos-filenames.sh`; `make goos-filenames-test` (`script/check-goos-filenames_test.sh`): on a temporary Git fixture `_js.go`, `_wasm_test.go`, `_js_wasm.go` and `_plan9_386.go` are refused with the file and a suggested name, while darwin/linux/windows/arm64 suffixes, `validity_js_flow.go`, a bare `js.go`, files outside `cmd/` and `internal/` and allowlisted paths pass. |
| ASD-V0-013, ASD-V0-016 | `docs/LANE-ORCHESTRATION.md` sections 1 and 4. The before/after batch measurements (median load per CPU and focused-test time with the cap; polling time and tool-timeout counts) are NOT_PRODUCED until the next batch; the procedure is in section 7. |
| ASD-V0-014, ASD-V0-015, ASD-V0-017, ASD-V0-018 | `script/heavy.sh`, `script/dogfood-finish.sh`, `script/lane-idle-check.sh`, `script/lane-clean.sh`; `make lane-scripts-test` (`script/lane-scripts_test.sh`): two concurrent `HEAVY_SLOTS=1` commands never overlap, status and `-p` pass through, a killed owner frees its slot; the finish refusal names every missing input before any build; idle and gap detection on fixed timestamps; only merged, clean, inactive clones and worktrees are removed and only with `--apply`. The 5-lane timing, dogfood-run counts and one-off cleanup counts are NOT_PRODUCED here (section 7). |

Validation proceeds with focused existing discovery/profile tests and document validators, then one
coordinated canonical gate on the final source+CEM target, independent semantic review and exact
keyed finish. No string-presence-only suite is added to count guide prose. Guidance alone does not
justify an unrelated model trial. Record actual optional-route exclusions in caller task evidence;
passing a fixture does not imply all features were useful or used.

The original pre-change query and range receipts are retained outside tracked source, with their
omissions and zero-change OUT_OF_SCOPE result. Initial coordination reports missing citations,
intent scope and outcome plus its unchanged-base preparation failure; these are not erased or
presented as a finished change. Actual route/use and final qualification pointers belong in the
same change's BUILD-LOG evidence record.

## Failure and rollback

A missing guide in an unrelated repository is not an error. Unsupported command admission retains
its typed gap and uses original source/repository tools; it never calls a legacy runtime to invent
success. A valid open frontier is advisory, while malformed input remains a failure. Missing local
completion evidence blocks that enrolled gate under its existing policy, not under a new rule here.

Lane procedure failures are refusals, never silent success: `heavy.sh` refuses an invalid slot or
parallelism value and an unusable lock directory (exit 2); `dogfood-finish.sh` refuses missing or
unusable inputs before building; `lane-idle-check.sh` refuses a missing path and reports a lane with
no readable activity as `UNKNOWN` (flagged); `lane-clean.sh` refuses an unavailable reference ref
and keeps any checkout it cannot read, with the reason. A stale shared repository makes merged
checkouts look unmerged (kept), never the reverse.

Roll back `ASD-V0-013..020` by reverting `docs/LANE-ORCHESTRATION.md`, the three lane scripts, the
`DOGFOOD_BASE` Makefile refusal, the GOOS filename check, the
`dogfood-finish` input refusal and `lane-scripts-test` together; lanes then return to charter-only
procedure. Rollback the guide, links, skill routing and help fixes together; retain existing local completion,
old outcomes and failed evidence. Future feature changes update their guide route and relevant
witness in the same change. Delivery may become implemented only after the in-scope checks, real
task evidence and independent review pass. Usefulness, token savings, formal FULL and protected
runner acceptance have separate gates and are not claimed by this status.
