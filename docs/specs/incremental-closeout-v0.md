# Incremental Closeout V0

Owner: Russell Lewis
Date: 2026-11-01
Requirement prefix: `ICL-V0`
Intent status: proposed
Delivery status: experimental
Authoritative inputs: ticket V1-1629 (lanes reran `dogfood-change` 4 to 19 times each; 855 runs cost
about 28 h where the protocol needs about 2 per lane); `AGENTS.md` invariants 2 and 4;
`docs/DOGFOOD.md` §4; the existing closeout contracts this slice refines: `DCW-V0-887` (pre-flight
receipt reused by the finish run), `DCW-V0-960` and `ASD-V0-015` (one refusal names every missing
input before any build), `DCW-V0-900` (closeout order), `ASD-V0-837` and V1-3550 (the first
batch-precheck leaves out the batch-drift go tests), `ASD-V0-042` (batch-precheck ledger reuse) and
`ASD-V0-050` (batch-drift selection against the affected plan).

## Agent digest
- Claim: The closeout re-runs only what a commit's content invalidates under each result's binding, prints every carry and decline, and refuses missing inputs first.
- Status: proposed/experimental
- Exists: preflight reuse in `internal/dogfoodflow/finishlane_reuse.go`; `@prebind` drift entries, the pre-bind record and the carry decision in `script/batch-precheck.sh`; the unadmitted input refusal in the `Makefile` `dogfood-inputs-check` and `dogfood-finish` recipes.
- Blocked on: owner acceptance; the per-lane `dogfood-change` median (V1-1629 AC3) needs the next batch's ledger.
- Read next: Requirements; Binding rules; Non-goals; Rollback.

## User and measurable job

A lane closes out once, after its last edit (`docs/DOGFOOD.md` §4). Each extra closeout pass costs
a full prepare, bind, check and seal. Before this slice, the closeout repeated work whose inputs had
not changed. Its two pre-flights each prepared the map. It also ran every batch-drift go test only
after the bind, so a drift failure that the bind could not affect surfaced only after a full bind and
cost a second closeout. The measurable job is the complete-run command count, repeated checks, and
wall time of one closeout on a representative scoped change, with every required gate kept.

## Requirements

- `ICL-V0-001`: A closeout step MAY skip work only when a recorded result binds every input
  that work reads. A content change MUST invalidate exactly the results whose binding includes the
  changed content. A result whose binding cannot be checked MUST run again, never carry. Each carry
  MUST print its source commit and what was carried. Each decline MUST print its reason.
- `ICL-V0-010`: A repeated `dogfood-inputs-check` whose pre-flight key equals the key of a passing
  receipt MUST replay that receipt's prepare lines, each marked `reused=inputs-check`, and keep the
  receipt for the finish run. The key is the `DCW-V0-887` key: BASE, target commit, target tree,
  outcome, stage count, the steps executable's SHA-256, and each input's path, presence and digest.
  Any other receipt is removed and the prepare runs. One closeout then runs `cem prepare` once,
  not twice.
- `ICL-V0-020`: A batch-drift entry marked `@prebind` is one the bind cannot reach: its package
  reads no `.corvint/` evidence path. Under `BATCH_PRECHECK_SKIP_DRIFT=1` (the closeout's first
  precheck), such entries MUST run, so their failure stops the closeout before any bind. The other
  selected entries are held for the post-seal precheck. `--list` names both rows. `--sync-check`
  accepts `@prebind` as a declared token.
- `ICL-V0-021`: When that first precheck passes on a pristine, clean tree with HEAD unchanged, it
  MUST write `<git-dir>/batch-precheck-prebind`. The record holds the format line, the head, a salt
  over the Go toolchain path and binary digest and the sorted `GO*`, `CGO_*` and `CORVINT_*`
  environment, and one line per entry that ran. A later precheck MUST carry a recorded entry only
  when all of the following hold:
  - the recorded head exists, is an ancestor of HEAD, and the tree is pristine;
  - the salt is equal;
  - every path changed since that head matches the evidence-path set (`.corvint/change.cem.json`,
    `.corvint/changes/<sha>.cem.json`, `.corvint/deferred-intents/**`);
  - `corvint affected --path` over those paths returns a valid `affected-plan/1` with Go `RUNNABLE`,
    every unknown is an accepted language frontier, and every unit maps;
  - that plan does not select the entry's package.

  Otherwise the entry MUST run again, and the precheck prints why.
- `ICL-V0-022`: A failing pre-bind run, a non-pristine or dirty tree, or a moved HEAD MUST leave no
  pre-bind record. A new first precheck MUST remove any earlier record before it runs.
- `ICL-V0-030`: `make dogfood-inputs-check` and `make dogfood-finish` MUST first run the
  `ASD-V0-015` input refusal outside `heavy.sh` admission (`DOGFOOD_FINISH_INPUTS_ONLY=1`, which
  exits after the refusal check). A missing input is then refused with exit 2, naming every missing
  input, without waiting on a held slot or host load. The admitted run that follows stays admitted
  (`ASD-V0-930`).

## Binding rules

| Evidence | Binds | Invalidated by |
| --- | --- | --- |
| Pre-flight receipt (ICL-V0-010) | BASE, target commit and tree, outcome, stages, executable digest, input digests | any change to these |
| Pre-bind drift pass (ICL-V0-021) | head ancestor, toolchain and environment salt, evidence-only diff, planner verdict per package | a non-evidence path, a changed salt, a planner unknown or a selected package |
| Post-seal drift, doc gates, tree verifier, check and seal | unchanged: they run every closeout | n/a (not carried by this slice) |

## Non-goals

- Carrying the tree verifier, check or seal across a fix commit. The verifier binds the whole tree
  and has no per-hunk scope in V0.
- Carrying doc-gate results. They need scopes from `tools/gate-ledger`, which this slice does not own.
- Changing citation-plan drafting (`propose-citations`) or CEM 2.0 intent.
- Lowering any gate, or treating a carried drift pass as a fresh run in reports. The carry row names it.

## Failure modes

- Planner unavailable, `jq` missing, a dirty worktree or a non-zero planner exit: the decline is
  printed and the entries run again.
- A planner unknown other than an accepted language frontier (for example `UNRESOLVED_READER`): run again.
- The planner selects a package through a broad reader (today `internal/tasks`, through the `..`
  literal in `boundary_test.go`): that entry runs again. This is correct but costs one run.
- A failing `@prebind` entry: the first precheck fails, no bind happens, and no record is written.
- Interruption: the record is written through a temporary file and a rename, only after a passing run.

## Acceptance evidence

- `script/batch-precheck_test.sh` group 15: pre-bind run, carry after an evidence-only bind
  commit, a selected package running again, `UNRESOLVED_READER`, an accepted TypeScript frontier, a
  planner failure, a changed environment salt, a dirty tree, a non-evidence commit, and a failing
  `@prebind` entry that leaves no record (ICL-V0-020..022).
- `cmd/corvint` `TestDogfoodInputsCheckICLV0010ReusesAMatchingPreflight` (ICL-V0-010).
- `script/lane-scripts_test.sh`: both targets refuse missing inputs without invoking the admission
  wrapper (ICL-V0-030), and the admitted run still waits behind a held slot (`ASD-V0-930`).
- `script/dogfood-cache_test.sh`: the closeout order with the skip-drift first precheck (`DCW-V0-900`).
- A replay over five real bind sequences and the raw costs:
  `docs/build-log/2026-11-01-icl-m15.md`.

## Rollback

Revert the slice's commits. To disable only the carry, remove the `@prebind` markers from
`DRIFT_TESTS`: every drift entry then runs after the seal, as before. Delete
`<git-dir>/batch-precheck-prebind` and `<git-dir>/corvint/inputs-check-pass` to drop any recorded
state. No stored format outside the lane git dir changes.
