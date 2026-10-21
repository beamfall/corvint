# Analyzer schema maintenance: one-command bump and base collision check (V1-2130, V1-1732)

Date: 2026-10-21. Base: `5cf75daf0` (`origin/main`). Requirements: `IDX-SNAP-V0-060`,
`IDX-SNAP-V0-061` (`docs/specs/index-snapshot-v0.md`), accepted under the owner's standing lane
auto-accept (2026-10-11).

## Decision

V1-2130 offered two shapes: bump only for changes that alter emitted index content, or one command
for all three steps. The lane chose the second, conservative shape. Deciding mechanically which
edits leave emitted content unchanged would weaken `IDX-SNAP-V0-017`, which over-invalidates on
purpose so a pack never answers with stale facts. The audit and its pins are unchanged.
`script/analyzer-schema.sh bump` now does the three manual steps: the number, both test pins, and
the use-case receipt repin. The digest is computed in Perl over the same inputs as the Go test.
The audited directory list is parsed from the test file, so the two cannot drift apart, and the
test still rejects a wrong pin.

The use-case receipts pin no `internal/contextindex` file. The repin that earlier lanes needed came
from `cmd/corvint/main.go` and similar subjects, not from the analyzer edit. The script runs the
repin anyway, because it is idempotent and reports `all N receipts current` when nothing changed.

V1-1732: the `.github/` workflow is owner-only, so the collision check reaches CI through an
existing CI target. `make decision-numbers-check` now depends on `make analyzer-schema-check`, and
CI's documentation step runs it. Residual: listing `analyzer-schema-check` by name in
`.github/workflows/ci.yml` needs the owner's ci-control-plane change.

The base comparison uses `origin/main`. CI checks out with `fetch-depth: 0`, so `origin/main`
resolves on pull_request, merge_group and push. When a branch's pins differ from its merge base
and from `origin/main`, its number must be greater than `origin/main`'s. That catches two branches
that both bumped to N with different digests: their numbers merge without a textual conflict, and
a conflict on the digest can be resolved by keeping either side.

## Measurement

Sample fix on this base: a comment-only append to `internal/contextindex/taskcontext.go`.

- Before: three manual edits (the schema constant, plus `auditedSchema` and `auditedSHA256` pasted
  from the failing test's message, which needs one compile and run of the test), then
  `script/repin-use-case-receipts.sh`.
- After: `script/analyzer-schema.sh bump` printed `corvint-analyzer/133 -> corvint-analyzer/134`
  and `use-case receipts: all 18 receipts current`, in 0.54 s wall, 0.09 s user (one run, quiet
  host). `TestAnalyzerSchemaInputs` then passed (`heavy.sh ... go test -run TestAnalyzerSchemaInputs
  ./internal/contextindex/`, 0.28 s). The edit was reverted.
- `script/analyzer-schema.sh check` on the unmodified base passed in 0.33 s wall, so the script's
  digest equals the committed pin `09ce7a24...`.

## Evidence

`script/analyzer-schema_test.sh` (`make analyzer-schema-test`, added to `shell-test` and
`GATE_STEPS`) covers six cases on a fixture repository: drift refusal naming the recovery, first
bump 7 -> 8, a kept number on a second edit of the same branch, a base that took the same number
(refused with `Rebump to corvint-analyzer/10`, then bumped past it), a branch with unchanged pins
behind a newer base (passes), and an unresolvable base (`NOT_RUN`, and `bump` refuses).

Rollback: remove the Makefile prerequisite and the two scripts.
