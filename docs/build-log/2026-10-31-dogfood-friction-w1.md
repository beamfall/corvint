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

## Acceptance criteria received after the first seal

The ticket bodies arrived from the orchestrator after the first seal at `a7a0630b2`
(`~/lanes/dffric-acs.md`). Several criteria differed from the charter's summaries, so a stacked
second slice delivers them. That slice is described below under "Second slice"; the first-slice
decisions above it stand.

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
- **V1-3394, first slice: found partly delivered.** `ASD-V0-045` (V1-3119, in the base snapshot `9477995d7`): a
  missing or unreadable `CITATIONS` refusal names `corvint dogfood propose-citations BASE >
  plan.tsv` and the `dogfood-inputs-check` pre-check, and says nothing was built, prepared or
  bound. This happens before any build or batch-precheck step, so no try is spent. Tests:
  `script/lane-scripts_test.sh` (the `ASD-V0-045` case) and `script/dogfood-cache_test.sh` (the
  closeout case, `! rg -q batch-precheck`). An empty plan is refused in the pre-flight as
  `empty-citation-plan`, whose fix names the same command (probe on the base).

## Second slice (against the ticket acceptance criteria)

- **V1-3047 (`DCW-V0-965`).** The criteria ask that `inputs-check` refuse an over-bound aggregate
  and closeout's verify-syntax refusals with closeout's own code and fix line. Both were already
  refused before any step on the base: `OCM-V0-028` and `DCW-V0-380`/`403`. The reported
  484-obligation manifest is now admitted by both commands, because V1-2971 derived the aggregate
  bound as 16 x 256 = 4096. A parity test now pins it: for four inputs, the pre-flight and the
  closeout's `finish-lane` print identical `REFUSE` and `fix:` lines.
- **V1-3394 (`ASD-V0-961`).** This is no longer already-delivered: the criteria also require the
  record-only recipe. The missing-`CITATIONS` refusal now names it. `docs/DOGFOOD.md` gains a
  "Record-only closeout" paragraph with the whole command sequence; it uses `LANE_DIR`, not
  `GIT_DIR`, because Git itself reads `GIT_DIR`.
- **V1-3284, V1-3428 (`DCW-V0-966`, `DCW-V0-967`).** The root cause of the high unknown rates:
  `prepareAnchors` dropped every candidate line in a modified file, and a self-development change
  almost always modifies its governing spec. Changes:
  - Candidates from modified files are now kept. `stable` still drops every line a hunk edits or
    the change repeats, so `DCW-V0-393`'s stated intent (never bind the change's own text) holds.
  - The hunk's own file stays a proposal, never an anchor. A mode-only path (in git's name list
    but not in the patch) is still never cited.
  - An untouched file's candidate ranks first.
  - A requirement whose definition already exists at BASE was being classed as "added by this
    change". That was a real bug, and the likely cause of the ci-select-w1 repro. It is now
    classed as edited, and its unknown note names the edited definition and the
    `cite-span-not-stable` refusal it would cause.
  - A hunk naming or defining an added requirement cites the BASE definition of a requirement the
    entry refines, amends or names. Several hunks may share that span, an explicit exception to
    `DCW-V0-382`, because citing the refined requirement is the right evidence. Failing that, it
    cites the nearest stable entry or heading to where the entry is inserted, as a marked
    proposal with its reason.
  - A prior seal's CEM falls back to the `DCW-V0-385` definition line when the change edits the
    `docs/DOGFOOD.md` line. A `--refresh` row that kept it unknown is redrafted.

  Measured unknown rates (base binary, then this tree):
  - this lane's `3555875f9..cd644e29f`: 57/65 then 1/65;
  - the V1-3284 change `c5d5f0829..9ba44871f`: 140/146 then 14/146, with 100 rows marked as
    proposals.

  The old checkout's pre-flight refuses a newer binary, so that draft was not bind-checked. The
  regression fixture's draft is bind-checked instead, through real cem prepare and cite.
- **V1-3270 (`DCW-V0-963` amended).** The criteria ask for the new line range too. Every stderr
  mapping line and every unknown note now reads `PATH at BASE S,C HEAD S,C`.
- **V1-3303.** The criterion test was added: a real seal, sidecars dated before the seal commit, a
  stacked commit, then `inputs-check` and `finish-lane` bound against the seal. Both pass and seal
  with no manual cleanup.
- **V1-3472 (`ASD-V0-962`).** With no range, the check now refuses a prefix family that
  `REQUIREMENTS.tsv` already uses. It names the next free one: the last letter advances past
  families the index or any tracked file uses. `TCD-V0` gives `TCE-V0`.

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

- Second slice: the whole `internal/dogfoodflow` package and the `cmd/corvint` dogfood filter
  PASS. Also PASS: `TestDogfoodFinishLaneDCWV0964StackedFinishNeedsNoCleanup`,
  `TestDCWV0965InputsCheckRefusesWhatCloseoutRefuses`,
  `TestProposeCitationsDCWV0966BoundsTheUnknownRate`, `...DCWV0966CitesPriorSealInEveryRoute` and
  `...DCWV0967NamesAnEditedRequirementDefinition`, plus the five dogfood and lane shell suites.

## NOT_RUN

- `make gate` and the exhaustive `go test ./...`: not required for scoped issue work (AGENTS.md
  Verify). The rest of the `corvint affected` selection (94 units, `scope=UNKNOWN`, 86
  `LANGUAGE_FRONTIER` unknowns) was not run. The `cmd/corvint` tests outside the `-run` filter
  were not run either.
- At first the ticket acceptance criteria could not be read here: `corvint-tasks ticket show`
  refuses on this WSL host (no journal; V1-3474). The orchestrator later supplied them, and the
  second slice builds to them.

## Rollback

Revert the lane's commits. Each requirement's own rollback is in the spec: the target-specific
`LANE`; the intent and per-intent refusals; the proposal branch of `propose`; the note prefix of
`unknownProposal`; the pre-flight note branch of `requireFreshWorktree`; and the script with its
target.

## Outcome

- V1-3047: delivered (`DCW-V0-960`, `DCW-V0-961`, `DCW-V0-965`).
- V1-3394: delivered (`ASD-V0-961`, on top of `ASD-V0-045`).
- V1-3284, V1-3428: delivered (`DCW-V0-962`, `DCW-V0-966`, `DCW-V0-967`).
- V1-3270: delivered (`DCW-V0-963`).
- V1-3303: delivered (`DCW-V0-964`).
- V1-3472: delivered (`ASD-V0-960`, `ASD-V0-962`).
- Dogfood: the first slice sealed on its first closeout try (`a7a0630b2`, unknownShare 0/65). The
  second slice is resealed with the same BASE as the lane's second and last try. The lane report
  gives its seal state and every `NOT_PRODUCED` reason.
