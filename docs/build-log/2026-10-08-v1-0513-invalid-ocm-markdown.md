# V1-0513: invalid OCM verdict in the Markdown proof report

## Intent

V1-0513 reported that an earlier unreleased candidate (da02db8c) rendered only the OCM `reason` in
the Markdown report, so a valid CEM could appear alongside a hidden native OCM invalid state and
verification issues. Expected: explicit validity, state and escaped issues, no inferred ready result,
and JSON/Markdown record digest parity.

## Findings

The defect is already fixed on `origin/main` (f33ea8ef): commit 73321199 ("Protect OCM inputs and
expose invalid review verdicts") renders `OCM validity`, `OCM state` and each `OCM issue` code and
message through `mdreport.CodeSpan` in `renderReviewProjection`. No production change was made.
The existing native test covered only wrong-base Markdown for the first issue code, without digest
parity, messages, malformed OCM inputs or a check against an inferred ready result.

## Change

- Proposed `CEM-PILOT-032` (not accepted) in `docs/specs/cem-pilot-kit.md`, with a traceability row.
- `TestReviewProjectionNativeInvalidOCMMarkdownParity`: native wrong-base, noncanonical and
  malformed canonical OCM inputs each keep `ok:false`, `ocmValid:false`, state `invalid`, every
  escaped issue line and the JSON `recordSetSha256` in Markdown, with no ready state, valid verdict
  or obligation join; a non-JSON OCM is refused without a published report.
- The native producer fixture moved into the `nativeOCMRepo` helper, shared with the existing test.

## Verification

- `go test -run TestReviewProjection ./internal/cem/workflow`: pass.
- Mutation check: disabling the issue rendering fails the wrong-base and noncanonical cases.
- `go vet`, `gofmt` on the touched package; spec/doc gates and `internal/specindex`.

## NOT_RUN

`make gate`, full `go test ./...`, dogfood CEM bind/seal (lane rules), live OCM qualification.

## Rollback

Revert the commit; it adds a test, a helper, a proposed requirement and this entry only.
