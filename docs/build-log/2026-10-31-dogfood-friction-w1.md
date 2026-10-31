# 2026-10-31 dogfood-friction-w1 lane

Lane `dffric-w1`, base `3555875f94e9fd717ccb9f84d0aabf638adbe45d`: seven tickets on the dogfood
closeout tooling (`internal/dogfoodflow`, `script/dogfood-finish.sh`, the `Makefile`). The goal is
that a lane seals on its first closeout try and that its citation plan is mostly usable rows.

## Problem

- `make dogfood-closeout` with a plan but no `INTENTS` passed `dogfood-inputs-check`: the closeout
  did not set `LANE=1`, and the Go pre-flight only checked an intent manifest it was given. The full
  pass's `ocm-aggregate` then refused `missing-intent-scope` after the bind commit, which spent a try.
  Probe on the base: `make dogfood-inputs-check BASE=<base> CITATIONS=plan VERIFY=v OUTCOME=passed`
  printed `inputs-check PASS`.
- An intent over one OCM map's 256 obligations (`docs/specs/affected-plan-v0.md`) also passed the
  pre-flight. `lrfrepo.IntentObligationCounts` failed with `too-many-obligations`, so
  `precheckIntentObligations` skipped, and `ocm-prepare` refused only after the bind commit. Probe:
  `INTENTS` holding that one spec printed `inputs-check PASS` in 1.8 s.
- `propose-citations` printed each usable nearest-span row on stderr only (`proposed, not
  drafted`, `DCW-V0-846`), and its unknown rows did not say which file they stood for.
- A stacked or reused lane worktree holds its predecessor's OCM sidecars, written before the seal
  commit that is now BASE. `finish-lane` moves them aside itself (`DCW-V0-406`), but the pre-flight
  refused `reused-worktree-state`.
- A lane charter named a requirement-ID prefix block that was already in use, and nothing cheap
  checked it.

## Decisions

- **V1-3047, absent inputs (`DCW-V0-960`).** `dogfood-closeout` exports `LANE = 1` as a
  target-specific variable. Its recipe stays the four steps `DCW-V0-900` pins, so both sub-makes
  inherit `LANE`, and `LANE=` on the command line still wins. The Go pre-flight also refuses an
  absent manifest (`intent-scope-not-provided`), printed together with the `DCW-V0-800` refusal
  when both apply, so a direct `make dogfood-inputs-check` without `LANE=1` is covered too. A
  direct `dogfood finish-lane` was left as it is: about a dozen fixtures run it without a manifest
  through fake steps, and its check still refuses.
- **V1-3047, obligation bound (`DCW-V0-961`).** When the batch count fails with
  `too-many-obligations`, the precheck derives each intent alone and refuses, naming every intent
  over the per-map bound. It uses the same reason code `ocm-prepare` would print. Any other
  derivation error is still left to the steps.
- **V1-3047, verify syntax.** An unquoted `-run A|B` was already refused before any step, with
  its quoted form (`DCW-V0-403`). The probe shows `REFUSE unsupported-verify-syntax ... quote the
  -run alternation as one word`. This lane changed nothing there.
- **V1-3284, V1-3428 (`DCW-V0-962`).** The nearest stable line of the hunk's own file, on a span
  no other row cites, is now drafted on stdout as the cite row. Stderr marks it `drafted
  proposal:` with the row, and a `proposals=K` line follows the header. Two other designs were
  rejected:
  - A `#` comment or a relation prefix inside the plan. The four-field plan grammar has no comment
    form, and every plan reader (bind, refresh, stages, the precheck) would have to change.
  - An opt-in flag. Lanes already ignored the stderr hint, so the default must change.

  A data record, a one-line file and a hunk that defines a requirement (`- \`ID\`:`, same-change
  intent) get no proposal and stay unknown. The tier gate is unchanged: a same-path cite is
  counted under `tier-below-anchored-same-path`, and an anchoring gate still refuses it
  (`DCW-V0-388`). So the proposal lowers `unknownShare` but does not raise any anchoring claim.
- **V1-3270 (`DCW-V0-963`).** Every unknown row's note now begins `hunk edits PATH at BASE S,C;`.
  Cite rows keep the hunk path on the existing stderr mapping line. A proposal row's PATH is the
  hunk's own file, so most former unknown rows now name it in the row itself.
- **V1-3303 (`DCW-V0-964`).** The pre-flight prints `NOTE reused-worktree-state` when stale
  sidecars are the only stale state. The note names each sidecar, the move `finish-lane` makes
  and the manual `mv`, and the pre-flight continues. A stranded trace still refuses: its
  migration may rekey reachable traces, which `DCW-V0-406` leaves to the operator. A seal-time
  sweep of the sidecars was rejected, because `dogfood-check` and a reseal read them after the
  seal.
- **V1-3472 (`ASD-V0-960`).** `script/requirement-ids-free.sh PREFIX LO HI [REF]` and
  `make requirement-ids-free PREFIX= RANGE=lo..hi [REF=]` read only Git objects at REF (default
  `origin/main`) with one `git grep`. A neighbouring byte that is a letter or digit (`XDCW-V0-960`)
  or a four-digit tail does not count. The script lists each used ID's first location (exit 1),
  prints `free` (exit 0), or refuses its input (exit 2).
- **V1-3394, already delivered.** `ASD-V0-045` (V1-3119, in the base snapshot `9477995d7`): a
  missing or unreadable `CITATIONS` refusal names `corvint dogfood propose-citations BASE >
  plan.tsv` and the `dogfood-inputs-check` pre-check, and says nothing was built, prepared or
  bound. This happens before any build or batch-precheck step, so no try is spent. Tests:
  `script/lane-scripts_test.sh` (the `ASD-V0-045` case) and `script/dogfood-cache_test.sh` (the
  closeout case, `! rg -q batch-precheck`). An empty plan is refused in the pre-flight as
  `empty-citation-plan`, whose fix names the same command (probe on the base).

## Evidence

- New tests, each failing on the base code (checked by stashing the implementation file):
  - `TestDCWV0960InputsCheckRefusesAbsentIntentManifest`
  - `TestDCWV0961OverBoundSingleIntentRefusesBeforeAnyStep`
  - `TestDCWV0964InputsCheckNotesRetirableSidecars`
  - `TestProposeCitationsDCWV0962DraftsNearestStableLineAsProposal`, which replaces
    `...DCWV0439NeverDraftsModifiedFileLines`
  - `TestProposeCitationsDCWV0963UnknownRowsNameHunkPath`
  - the `script/dogfood-cache_test.sh` closeout case without `LANE`, which fails when the
    `Makefile` change is stashed
  - the `script/lane-scripts_test.sh` `requirement-ids-free` group
- Updated to the new contract: `DCWV0161`, `DCWV0150` (twice), `DCWV0393`, `DCWV0333` and
  `DCWV0385`. Each now admits only the hunk's own marked proposal on a modified file.
- Focused runs, all PASS:
  - `go test ./internal/dogfoodflow`, whole package. `corvint affected` reported `scope=UNKNOWN`
    with 94 units selected, so heavy.sh ran it with `HEAVY_WHOLE_REASON`.
  - `go test -run 'Dogfood|Propose|InputsCheck|FinishLane|DCWV0' ./cmd/corvint`
  - `make dogfood-cache-test`, `lane-scripts-test`, `dogfood-change-test`,
    `dogfood-bin-reuse-test` and `dogfood-bind-range-test`

## NOT_RUN

- `make gate` and the exhaustive `go test ./...`: not required for scoped issue work (AGENTS.md
  Verify). The rest of the `corvint affected` selection (94 units, `scope=UNKNOWN`, 86
  `LANGUAGE_FRONTIER` unknowns) was not run. The `cmd/corvint` tests outside the `-run` filter
  were not run either.
- Ticket acceptance criteria were not read: `corvint-tasks ticket show` refuses on this WSL host
  (no `.git/taskman` journal, and these tickets are newer than the committed `.taskman` snapshot).
  The work follows the charter's summaries, and the orchestrator was asked for the criteria.

## Rollback

Revert the lane's commits. Each requirement's own rollback is in the spec: the target-specific
`LANE`; the intent and per-intent refusals; the proposal branch of `propose`; the note prefix of
`unknownProposal`; the pre-flight note branch of `requireFreshWorktree`; and the script with its
target.

## Outcome

- V1-3047: delivered (`DCW-V0-960`, `DCW-V0-961`; the verify-syntax part was already covered by
  `DCW-V0-403`).
- V1-3394: already delivered (`ASD-V0-045`, V1-3119; evidence above).
- V1-3284, V1-3428: delivered (`DCW-V0-962`).
- V1-3270: delivered (`DCW-V0-963`).
- V1-3303: delivered (`DCW-V0-964`).
- V1-3472: delivered (`ASD-V0-960`).
- Dogfood: sealed once with `make dogfood-closeout` after this commit. The lane report gives the
  seal state and every `NOT_PRODUCED` reason.
