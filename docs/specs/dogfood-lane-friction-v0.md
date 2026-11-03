# Dogfood Lane Friction V0

Owner: Russell Lewis
Date: 2026-11-03
Requirement prefix: `DCW-V0`
Intent status: accepted under the owner's standing auto-accept (2026-10-11)
Delivery status: implemented
Authoritative inputs: lane dfric charter (orchestrator, 2026-11-03) for V1-4406, V1-4441, V1-4442,
V1-4443, V1-4444, V1-4401 and V1-4291; `daily-change-evidence-workflow-v0.md` (`DCW-V0-015`,
`DCW-V0-112`, `DCW-V0-396`, `DCW-V0-406`, `DCW-V0-442`, `DCW-V0-731`, `DCW-V0-816`);
`spec-requirement-index-generator-v0.md` (`SRG-V0-740`, `SRG-V0-744`);
`corvint-self-development-v0.md` (`ASD-V0-960`, `ASD-V0-660`); `cem-stable-v1.md` (`CEM-V1-081`).

## Agent digest
- Claim: The change-evidence closeout and lane tooling name the real recovery for seven observed lane frictions.
- Status: accepted under the owner's standing auto-accept (2026-10-11) / implemented
- Exists: `script/dogfood-finish.sh`, `cmd/corvint/dogfood_flow.go`, `internal/dogfoodflow` (propose, check, change, reused worktree), `script/dogfood-retire-traces.sh`, `script/gen-spec-requirements.sh`, `script/requirement-ids-free.sh`.
- Blocked on: measured lane time saved remains NOT_OBSERVED.
- Read next: Requirements; Acceptance and evidence; Compatibility and rollback.

## User job and verified baseline

A lane closing out a change follows DOGFOOD section 4 and the lane flow guide. At base
`14e055d69e3d18979f4eb5e33aae8eb4f16848fd` lanes reported seven frictions: the static pre-check
ignored `CORVINT_BIN` and its stale-binary refusal said to unset a variable that was not set; a
requirement-only citation row left hunks `cited` with no documented two-row route; a reset of an
unpushed bind commit needed a manual `migrate-traces` dry run and apply; the expected first bind
pass ended `FAIL not-complete`; "clean worktree" was undefined; `make spec-requirements` printed
full-spec warnings for specs the change does not touch; and `make requirement-ids-free` refused a
four-digit range and printed `bad-prefix ''` when run bare. The owning workflow spec holds 256
indexed obligations at this base, so this companion holds the new obligations.

## Requirements

- `DCW-V0-2140`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4406) The static input pre-check of `make dogfood-finish` and `make dogfood-inputs-check` MUST run, in order of preference, the cached tree build of HEAD, else `CORVINT_BIN` when it is set, else the installed `corvint` on `PATH`, each only when it lists `--static`; a set `CORVINT_BIN` without `--static` is a `SKIP` naming it, never a fall-back to `PATH`. A dogfood flow's `self-binary-stale` refusal MUST name setting `CORVINT_BIN` to a build of this tree, MUST name unsetting `CORVINT_BIN` only when it is set, and under `--static yes` MUST also name `DOGFOOD_STATIC_PRECHECK=0`.
- `DCW-V0-2141`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4441) DOGFOOD section 4 MUST state that a hunk takes several citation rows and that the drafted anchoring row is kept beside the requirement row the hunk implements; the `tier-below-anchored` fix MUST name that route. An explicit `corvint dogfood propose-citations BASE --refresh PLAN` MUST keep every carried row and, for each carried hunk with at least one cite row and no carried span that earns a `CEM-V1-081` anchor, add the drafted anchoring row when one exists, naming each on a note and counting them as `anchors-restored=N`; a span it cannot read counts as anchoring. finish-lane's automatic re-key of a moved plan (`DCW-V0-947`) MUST carry rows only.
- `DCW-V0-2142`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4442) `make dogfood-retire-traces` MUST run `corvint migrate-traces --dry-run` with this tree's build (or `CORVINT_BIN`) and, when the plan holds stranded traces and no legacy rekey, apply it with that dry run's digest, printing one `RETIRE` line per trace and the rerun step; a plan that also rekeys legacy traces MUST be refused naming the manual dry run and apply, and an empty plan MUST report nothing stranded. The pre-flight's `NOTE stranded trace` line MUST name that target on a `fix:` clause.
- `DCW-V0-2143`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4443) A standalone `dogfood change` pass whose failing rows include the recorder's `record-index-failed`, are all rows the bind commit clears (`DCW-V0-015`, `DCW-V0-816`), and whose worktree's only change against HEAD is `.corvint/change.cem.json` MUST print `dogfood-change: PENDING commit-sidecar` with a `fix:` line naming "commit .corvint/change.cem.json, then rerun", in place of `FAIL not-complete`, keep the row and report lines, and exit 3; `make dogfood-change` MUST exit 0 for it. Every other incomplete pass MUST keep `FAIL not-complete` and exit 1, and finish-lane's in-process passes MUST keep their exit codes.
- `DCW-V0-2144`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4444) DOGFOOD section 4 and LANE-ORCHESTRATION section 13 MUST define the clean worktree of the post-bind rerun as one with no change after the bind commit (`git status --porcelain` lists nothing; ignored paths do not count), which the lane's own worktree meets. `dogfood check`'s `dirty-worktree` and `uncommitted-change-not-in-base-target` refusals MUST add a `fix:` line naming the dirty path count, that definition, and the rerun commands.
- `DCW-V0-2145`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4401) In generation modes, `script/gen-spec-requirements.sh` MUST print the per-spec near-cap warning only for a spec whose staged Requirements section gains an ID since `BASE` (or `SPEC_REQUIREMENTS_BASE`) when it names an available commit, else since `HEAD`; every other near-cap spec MUST be counted on one `spec-requirements: NOTE N spec(s)` line naming `make spec-capacity-check`. Over-cap refusals and the capacity mode (`SRG-V0-744`) are unchanged.
- `DCW-V0-2146`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4291) `script/requirement-ids-free.sh` and the block mode of `script/requirement-id-collisions.sh` MUST admit `LO` and `HI` of one to nine digits and count an ID only in its canonical spelling (at least three digits, `%03d`); the next-free-block search MUST cover the namespace of HI's width. A bare run, through the script or `make requirement-ids-free`, MUST refuse `bad-arguments` with the usage instead of `bad-prefix ''`.

## Non-goals, trust boundary and failure modes

No new evidence authority, citation rule, CEM or trace wire format, Tasks state or ranking input.
The restored anchoring row is a drafted proposal the operator reviews like any other draft; the
bind's own checks still decide admission. `dogfood-retire-traces` writes only through
`migrate-traces --apply`, which quarantines byte-preserved; it never deletes. A pending pass is
still incomplete: its report stays `"complete": false` and `dogfood check` still refuses it until
the rerun. A failure of the summary's base read falls back to HEAD and never blocks generation.
Missing measurement of lane time stays NOT_OBSERVED.

## Acceptance and evidence

| Requirement | Implementation | Deterministic evidence |
|---|---|---|
| `DCW-V0-2140` | `script/dogfood-finish.sh` static pre-check; `staleBinaryRecovery` in `cmd/corvint/dogfood_flow.go` | `script/lane-scripts_test.sh` group 33 (CORVINT_BIN preferred over PATH, a relative one resolved from the caller, no-static SKIP names it); `TestDogfoodFlowRefusesStaleBinary`, `TestStaleBinaryRecoveryNamesTheRealEscape` |
| `DCW-V0-2141` | `unanchoredCarried` and `draft` in `internal/dogfoodflow/propose.go`; `tierBelowAnchoredFix`; DOGFOOD section 4 step 2 | `TestProposeCitationsDCWV02141RefreshRestoresAnchoringRow` |
| `DCW-V0-2142` | `script/dogfood-retire-traces.sh`, `make dogfood-retire-traces`, `requireFreshWorktree` note | `script/lane-scripts_test.sh` group 63; `TestDCWV0844BindResetRerunNamesAndRetiresTheResetCase` note assertions in `internal/dogfoodflow/reused_worktree_reset_test.go` |
| `DCW-V0-2143` | `pendingSidecarCommit` and `reportFailures` in `internal/dogfoodflow/change.go`; `dogfood-change` Makefile rule | `TestChangeDCWV02143PendingOnlyForTheUncommittedSidecar`; `cmd/corvint` dogfood flow tests asserting exit 3 and `PENDING commit-sidecar` (`dogfood_flow_portable_test.go`, `dogfood_ocm_dcw_test.go`, `dogfood_batch_check_test.go`, `dogfood_worktree_lanes_test.go`, `dogfood_finish_lane_test.go`) |
| `DCW-V0-2144` | `requireCleanChange` in `internal/dogfoodflow/check.go`; DOGFOOD section 4; LANE-ORCHESTRATION section 13 | `TestCheckDCWV0045CleanProbeNeverRunsRepositoryFilters` dirty cases in `internal/dogfoodflow/check_status_test.go` |
| `DCW-V0-2145` | `script/gen-spec-requirements.sh` obligation block | `script/gen-spec-requirements_test.sh` case 50 |
| `DCW-V0-2146` | `script/requirement-ids-free.sh`, `script/requirement-id-collisions.sh` | `script/lane-scripts_test.sh` group 45 |

Results and limits are retained in `../build-log/2026-11-03-dfric-dogfood-friction.md`.

## Compatibility and rollback

Each requirement reverts independently: restore the static pre-check order and the stale-binary
suffix; drop `unanchoredCarried` and `CarryOnly`; delete the retire script, its target and the note
clause; return `pendingSidecarCommit` to false (exit 1 and `FAIL not-complete`) and drop the
Makefile mapping; drop the clean-worktree fix line and paragraphs; restore per-spec generation
warnings; and restore the three-digit range check. Callers that read exit 1 from a standalone
`dogfood change` first pass must accept 3 while this is in place; finish-lane is unaffected.

## Remaining decisions

No implementation decisions remain. Lane time saved is outside this deterministic gate.
