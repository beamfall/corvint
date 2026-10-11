# V1-1230 Playwright run stability and V1-1229 run compare

Date: 2026-10-11. Base: `8b659142492966161ca82d1f5ff8f503fec1c504`.
Scope: new experimental `PWR-V0` requirements, proposed and pending owner acceptance; no decisions,
Core profile changes or ticket-store writes.

## Reader design

The two existing Playwright JSON parsers do not fit a run-set view. `internal/appflows` keys tests on
the spec title, and `internal/jstestprovider.ParsePlaywrightJSON` drops project, title path, worker
index and expected status. A new bounded reader in `internal/playwrightruns` therefore keeps the full
identity (project, file, title path). It reuses the shared `runhygiene.ScrubFailure` and
`secretscreen.Screen` discipline for first-error lines. Consolidating the parsers is left as a
follow-up.

Identity inputs are only what a run declares under `config.metadata.corvintRunIdentity`, plus the
Playwright version. Inferring them from Git or file timestamps would invent certainty that the report
does not carry, so an undeclared run abstains with `IDENTITY_INPUT_MISSING`.

## Evidence

The focused package and CLI tests pass on darwin/arm64, and `GOOS=windows go vet` compiles the
non-unix opener. Real-suite qualification against an adopter's reports is `NOT_RUN`. The JSON
reporter omits fixture timings, cleanup and passing `expect` calls, so these stay `NOT_OBSERVED`.
Root help needed 61 bytes trimmed from existing one-line descriptions to stay within the 4096-byte
CCF-V1-009 bound.

## Compare design (V1-1229)

`corvint runs compare` reuses the same reader and identity. Assertion origin comes only from the
error location the reporter gives: the test's own file, any other file, or none. The reporter cannot
tell a page object from a helper or a fixture, so all three are reported as one `SUPPORT` origin. A
support-only difference keeps the test `SAME` and is reported as a support difference. This lets a
page-object refactor reach `PARITY` while the difference stays visible. Undeclared identity inputs
do not block a verdict; they are reported as the unknown `IDENTITY_INPUTS_NOT_DECLARED`. Declared
config digests that differ give `INCOMPARABLE`. An unreadable report still emits a document
(`INCOMPARABLE`, `REPORT_UNREADABLE`) rather than refusing, so a pipeline always receives a verdict.

## Review round 1

The independent review found eight gaps, all fixed in one follow-up commit with focused tests.
A missing `projectName` is now refused. Backslash paths are normalized. Conflicting blob spellings
abstain. Matcher modifiers are kept. Within-side inputs are compared in an order-independent way.
`config.projects` is compared. An unreadable report no longer hides the other reasons. Emitted
identities and inputs are secret-screened, while matching still uses the raw values.

## Review round 2

Round 2 found four more gaps, all fixed with focused tests. A skipped result the test did not
expect (`expectedStatus` other than `skipped`) is now `notRun`. That is how Playwright records a
test it never started. An ambiguous identity in compare gets an `AMBIGUOUS` row with no
statistics, rather than a verdict built from an arbitrary observation. A side that lacks a test
reports N `absent`. Assertion files are compared raw and screened only on output.

## Review round 3

Round 3 found two gaps, both fixed with tests. The matcher regex now keeps the complete identifier,
so a custom `toHaveHTTPStatus2` no longer truncates to the same name as `toHaveHTTPStatus3`.
Conflicting blob spellings are kept beside the normalized inputs, so two runs with different
conflicts still give `INPUTS_DIFFER_WITHIN_SIDE`.
