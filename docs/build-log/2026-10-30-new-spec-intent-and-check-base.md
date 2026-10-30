# New-spec intent pre-flight and dogfood-check bound base

Lane friction-dogfood-w5, tickets `V1-3108` and `V1-3172`; immutable base
`c5d5f0829c28756ee7b202caf80a3f3fc37fa85b`. Requirements `DCW-V0-449` and `ASD-V0-085`.

## V1-3108: a cited companion-spec hunk failed only after the bind

A change that adds a companion spec (the route for a spec at the 256-obligation limit) and lists
it in `DOGFOOD_INTENTS_FILE` bound only when the citation plan left the new spec's hunk uncited.
A plan that cited it passed the cite step, spent the bind run, and then failed
`ocm-prepare-001 bootstrap-intent-not-unknown`.

Decision: `OCM-V0-009` stays as is. Each hunk of an intent absent at BASE must stay `unknown`, and
that is what lets the dogfood policy subtract it as a bootstrap unknown. The fix moves the
refusal to the point where it costs nothing. The prepare pre-flight (`dogfood inputs-check` and
the `finish-lane` pre-flight) reads the prepared map. It refuses each plan row that cites a hunk
on a base-absent intent, names the row, and prints a `fix:` line: delete the row, or keep it as
an `unknown<TAB>insufficient-evidence` row. The new check lives in
`internal/dogfoodflow/bootstrap_precheck.go`. It is reached through a one-line hook in
`precheckInputs`. A full or bind pass that still reaches the OCM refusal now prints the same fix
through the `fixHints` table.

Open follow-up: for a new file, the `propose-citations` drafter writes an unknown row whose note
asks the author to "name the BASE contract this new file implements". When the new file is a
bootstrap intent, that note steers the author toward the refused cite. The drafter has no intents
input today.

## V1-3172: dogfood-check needed BASE retyped

The post-commit `make dogfood-check` runs on the bind commit, whose committed CEM already records
its `baseRevision`. Without BASE, the Makefile now passes `--bound-base`. The script reads
`HEAD:.corvint/change.cem.json` and requires exactly one full-sha `baseRevision` that HEAD
contains. It prints which base it uses before any build. In every other case it refuses with the
existing `BASE is required` message and a `fix:` line. `ASD-V0-019` is amended explicitly: the
Makefile default `main` is still never used. The shell resolves the base before it builds the
base verifier, so the Go `dogfood check` contract (`--base` required) is unchanged.

## Evidence

- `TestDCWV0449InputsCheckRefusesCitedBootstrapIntentHunk` and
  `TestDCWV0449InputsCheckPassesUncitedBootstrapIntentHunk` (with
  `TestDCWV0379InputsCheck*` and `TestDCWV0380InputsCheckPasses*`): PASS through `script/heavy.sh`.
  On BASE, the refusal test fails because the pre-flight passed the plan.
- `script/dogfood-cache_test.sh`: PASS, covering the bound base, a non-ancestor base, no CEM, and
  the `ASD-V0-019` loop. On BASE, the Makefile refused every run without BASE.

Rollback: remove `bootstrap_precheck.go`, its hook and its `fixHints` row; restore the Makefile's
plain BASE argument and remove the script's `--bound-base` block.
