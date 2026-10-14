# 2026-10-13 V1-1348: `dogfood finish-lane` runs the bind loop end to end

Lane V1-1348, Corvint 1.0 scope `PRS-V1-018` (accepted, owner instruction 2026-10-13).
Requirements `DCW-V0-060..062` (`docs/specs/daily-change-evidence-workflow-v0.md`), accepted under
the owner's standing auto-accept (2026-10-11). Base d97f00b64 (origin/main after PRs #863, #865
and #866).

## Intent

Every lane ran the `docs/DOGFOOD.md` §4 bind loop by hand: a pass to learn hunk IDs, the plan, a
sidecar removal, a cited pass, the bind commit, a second pass, check and seal. Each step had its own
refusal and the recoveries lived in agent memory. One command now runs the loop and stops at the
first step that does not succeed with that step's own reasons and `fix:` lines.

## Decisions

- In process (`DCW-V0-060`): `internal/dogfoodflow/finishlane.go` `FinishLane` calls the same
  `change` run the `dogfood change` subverb uses, once per pass, and `Seal`, which runs the check.
  It holds one operation lock; the nested runs share it through the existing reentrant
  capability. Nothing shells out to make. `make dogfood-finish` delegates to
  `script/dogfood-finish.sh`, which builds the base and tree verifiers as `dogfood-seal` does and
  then `exec`s `corvint dogfood finish-lane` from the tree binary.
- Prepare is a pass without a plan. It stops only on a refusal (exit 2) or an unprepared map. The
  rows a planless pass cannot produce (`cem-cite` `citation-plan-not-provided`, `cem-status`
  `not-ready`, OCM rows over an invalid intent) are left to the bind, which reports them with the
  plan in place: the reference pass-0 log of lane V1-1278 failed those same rows on a lane that
  went on to seal.
- The bind pass may leave only `local-outcome` `record-index-failed`, the uncommitted sidecar the
  bind commit clears (`DCW-V0-015`). The rebind must exit 0.
- The bind commit runs `git commit` in the caller's environment for the sidecar path alone, as the
  recipe's commit does, so identity, signing and hooks are the caller's. With the same identity
  and dates the commits equal the recipe's byte for byte; the end-to-end test compares the seal
  commit and tree with a manual run in the same test.
- Resume (`DCW-V0-061`): a HEAD that is a one-parent commit titled `chore: bind change evidence`
  that changes only the sidecar resumes at rebind; a seal commit is `seal SKIP already-sealed`,
  exit 0. Before the bind commit the only uncommitted path admitted is the sidecar a stopped run
  left, compared by Core's hardened probe digest (`DCW-V0-045`), so no repository filter runs.
- One report (`DCW-V0-062`): every row a pass did not produce, prefixed by its step, stays in the
  `NOT_PRODUCED rows=N` block even when a later pass produced it.
- The root help's `dogfood` row keeps its four subverbs: adding `finish-lane` put the root help
  over its 4096-byte budget (`TestRootHelpIsACompactVerbIndex`). `corvint dogfood --help` lists it.

## Verification

- `GOTOOLCHAIN=local go test -count=1 ./internal/dogfoodflow/...`: PASS.
- `GOTOOLCHAIN=local go test -count=1 -run 'Dogfood|Finish|Help|Usage' ./cmd/corvint`: the five
  `TestDogfoodFinishLane*` tests PASS (end to end equal to the manual recipe; uniform-plan stop and
  corrected rerun; `invalid-requirements-section` stop; resume from the bind commit; refusals of a
  dirty worktree, HEAD at BASE, a missing plan and a positional BASE).
- `go vet`, `gofmt -l`, `script/repin-use-case-receipts.sh` (all 18 receipts current), and the
  local doc gates.

## Independent review

One Codex round (`codex exec -s read-only`, REQUEST_CHANGES, four findings):

- Fixed: a sealed HEAD returned `seal SKIP` before the base, plan and worktree were checked; it is
  now admitted as its bind commit is, and a dirty sealed HEAD is refused.
- Fixed: a seal whose commit a hook rejects leaves its staged rename, and the rerun refused it
  with a fix line that did not undo it. The fix line now names
  `git restore --source=HEAD --staged --worktree -- .corvint/change.cem.json
  .corvint/changes/BIND.cem.json`; `TestDogfoodFinishLaneDCWV0061ResumesAfterRejectedSeal`.
- Fixed: `make dogfood-finish` interpolated inputs into double quotes; each now reaches the wrapper
  single-quoted (`dogfood_finish_quote`).
- Not changed: prepare passes when the map is prepared even if the planless pass fails other
  rows. The bind is a full pass that reruns every step with the plan and stops with those rows'
  own reasons and `fix:` lines, so nothing is hidden; stopping prepare on them would stop lanes
  that seal, since a planless pass fails OCM rows on such lanes (V1-1278's pass-0 log).

## Own bind and seal, and gate rehearsal

This lane binds and seals itself with `make dogfood-finish`. This entry is part of the change the
seal binds, and the criterion-4 gate (V1-1283) needs the seal commit to be the branch head, so the
run's output and the `cem gate-verdict --predicate touched-anchored` rehearsal over
`docs/specs/README.md` and `docs/specs/REQUIREMENTS.tsv` cannot be written here. They are kept in
the pull request body and the lane report. The plan cites, for those two paths, BASE lines of
`AGENTS.md` that name them, the `path` anchor of `CEM-V1-081`.

## Limits

- The bind commit's title is the resume key: a bind commit made by hand under another title is
  read as part of the change.
- The bind commit uses the caller's Git configuration, so a rejecting hook or signing failure
  stops the run at `commit FAIL` with the sidecar uncommitted; the rerun continues from prepare.
- `script/dogfood-finish.sh` keeps the Corvint-only VERSION identity check and the two verifier
  builds of `script/dogfood-check.sh` (`DCW-V0-022`); the seal step is the in-process
  `dogfood seal` subverb, not `script/dogfood-seal.sh`.
- Rollback: revert the change; lanes run the per-step recipe, whose commits and maps are the same.
