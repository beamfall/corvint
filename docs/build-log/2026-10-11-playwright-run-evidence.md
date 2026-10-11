# V1-1230 Playwright run stability

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
