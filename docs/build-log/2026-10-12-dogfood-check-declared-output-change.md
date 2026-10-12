# 2026-10-12 Dogfood check: declared `cem status` output change (V1-1279)

Branch `claude/v1-1279-dogfood`, based on 1f35e3da7. The requirement is `DCW-V0-047` in
`docs/specs/daily-change-evidence-workflow-v0.md`, accepted under the owner's standing auto-accept
(2026-10-11). Operator steps are in `docs/DOGFOOD.md` (Inputs, section 4, Fail-closed outcomes).

## Intent

`dogfood-check` requires the base verifier (built from `BASE`) and the tree verifier (built from the
change) to print byte-identical `cem status` output. A change whose purpose is to alter that
canonical output, such as an added member, therefore fails `verifier-disagreement` against its own
base and can never pass. V1-1265 hit this: it landed (beamfall/corvint#828) only by freezing the
default `cem status` output and keeping the new member out of it. This entry gives such a change
a declared, recorded path instead of a workaround.

## Decisions

- The declaration is `DOGFOOD_OUTPUT_CHANGE=REQUIREMENT-ID` for `dogfood-check` and
  `dogfood-seal`. It is admitted only when `docs/specs/REQUIREMENTS.tsv` at `HEAD` maps the ID to a
  spec and `BASE..HEAD` modifies that spec, so the change itself must carry the requirement that
  explains the new output. Unset or empty, the check is byte-for-byte unchanged.
- Refusals, all exit 2 and before any verifier runs: `output-change-requirement-unknown`,
  `output-change-not-in-diff`, `output-change-aggregate-unsupported`. The aggregate report has a
  closed schema with no member for the declaration, so the declaration is refused there rather than
  silently dropped or the schema widened.
- When admitted, only the `cem status` comparison changes: the base verifier still runs once (its
  stdout digest is recorded) and the tree verifier runs in both roles and must agree with itself.
  The OCM status comparison and `verifyAbstention` are unchanged.
- The SUMMARY line, the report's `dogfoodCheck` member (`baseIndependence`) and the seal's PASS line
  record `base-independence=NOT_PROVED base=sha256:B tree=sha256:T declared=ID`. `NOT_PROVED` is
  recorded even if the two outputs happen to agree: the declaration replaced the proof.
- This is the same limit `LCP-V0-014` already states for `dogfood finish` (one binary in all
  verifier roles shows self-consistency only); `dogfood finish` is not changed.
- V1-1265's history is not edited. V1-1276 (dogfood producer cutover) changes the canonical output
  and must cite `DCW-V0-047` in its `dogfood-check` and `dogfood-seal` runs instead of repeating
  the frozen-default workaround.

## Failure modes

- A declaration naming an unrelated requirement whose spec the change also edits is admitted; the
  rule binds the declaration to the diff, not to the meaning of the output change. Review of the
  cited requirement remains the control.
- An admitted change can no longer detect a regression the base verifier would have caught in the
  `cem status` bytes; the recorded base digest keeps the difference inspectable.
- In the `make dogfood-seal` path the PASS line is printed by `script/dogfood-seal.sh`, so the
  fields appear on the SUMMARY line of the check it runs, not on the shell's PASS line; the
  `corvint dogfood seal` subverb appends them to its own PASS line.

## Evidence

- Code: `internal/dogfoodflow/check.go` (`admitOutputChange`, `verifyStatus`, `baseIndependence`,
  `record`, `seal`); input wiring in `cmd/corvint/dogfood_flow.go`; help in `cmd/corvint/help.go`.
- Test: `TestCheckDCWV0047DeclaredOutputChange` in
  `internal/dogfoodflow/check_output_change_test.go` reproduces the bug (undeclared additive member
  fails `verifier-disagreement`), then passes when declared with base once / tree twice, checks the
  report member and the seal PASS line, an unstable tree verifier still fails, each refusal fires
  before any verifier runs, and agreeing verifiers without a declaration print no new field.
- Focused gates: `go test ./internal/dogfoodflow/`, `go test -run 'Dogfood|DCW' ./cmd/corvint/`,
  `go vet`, `gofmt`, `go build ./...` and the local doc gates.
- `NOT_OBSERVED`: a real output-changing lane run through `make dogfood-check` with the declaration
  (first expected in V1-1276), and the aggregate refusal in an enrolled aggregate check.

## Rollback

Revert this change: `DOGFOOD_OUTPUT_CHANGE` is ignored again and an output-changing lane returns to
`verifier-disagreement` or the frozen-default workaround. Reports and seals carrying
`baseIndependence` remain readable as recorded history; nothing else reads the member.
