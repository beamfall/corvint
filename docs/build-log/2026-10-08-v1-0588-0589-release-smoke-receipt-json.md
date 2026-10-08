# V1-0588 / V1-0589 release smoke corpus receipt and closed JSON

Base `ecfff8e063914c9e59c83d1efada9e9b1119a30e`. Owning contract: `PUB-V0-024` in
`docs/specs/public-release-v0.md` (installed Core discovery rows of the qualification receipt).

## V1-0588: corpus receipt version 2 (already fixed on base)

No code change. Commit `b4a0f8ea` ("fix: qualify corpus race checks and installed receipt
discovery", build-log `2026-10-01-corpus-ci-qualification.md`) already changed
`validateDocumentationCorpusSearch` in `internal/companionrelease/core_smoke.go` to accept
`corvint-corpus-receipt/1` or `corvint-corpus-receipt/2`, operation exactly `search` and at least
one result, and added `TestPUBV0024DocumentationCorpusSearchProfiles` with an unsupported `/3`
rejection control. `PUB-V0-024` already states that rule. Evidence on this base:

- `TestPUBV0024InstalledCoreDiscoveryWorkflows` builds `./cmd/corvint` from source into a test
  temporary directory and runs all five installed discovery steps, including the documentation
  corpus manifest/build/search CLI path; it passes (see focused tests below).
- The installed `corvint` on PATH (`Corvint 1.0.0-rc.2 (build 360)`) run on the exact minimal
  `docs/release.md` fixture through `docs corpus manifest`, `build` and `search --query
  ReleaseSmoke` returned schema `corvint-corpus-receipt/2`, operation `search`, one result.
- The `/3`, wrong-operation and empty-results controls still fail validation.

## V1-0589: trailing bytes after a smoke receipt

Failing before: `decodeClosedJSON` treated any error from the second `Decode` as end of input, so
a valid `/2` or `/1` search receipt followed by `{`, `}`, `x` and similar was accepted and the
profile predicates then passed. With the new regression rows added to
`TestPUBV0024DocumentationCorpusSearchProfiles` and the old helper, four rows failed:
`trailing_incomplete_object`, `legacy_trailing_incomplete_object`, `trailing_garbage`,
`trailing_close_brace`.

Repair: after the single `Decode`, `decodeClosedJSON` inspects the input from
`decoder.InputOffset()` and refuses any byte other than JSON whitespace (space, tab, CR, LF) with
"trailing bytes after JSON value", so a complete second value, garbage or an incomplete value all
fail without a second decode. (A first draft used `io.EOF` like `retained.go`; the added import
shifted the `core_smoke.go:52` citation in decision 0375, so the import-free form was kept.) The helper is shared
by every Core smoke JSON step (version, affected, test-validity, corpus search, work observe).
`PUB-V0-024` carries a proposed amendment citing V1-0589.

Passing after: all 15 rows pass, including the retained whitespace-only acceptance rows (`/2` and
`/1` with trailing `\n \t\r\n`), the complete-second-value and scalar rejection rows, and the
pre-existing profile, operation and empty-result controls.

## Non-goals, failure modes, rollback

Non-goals: no change to receipt profiles, the producer, archive or platform qualification, or
hosted CI; parser controls do not imply GH399 external or Linux installed qualification.
Failure mode: a future Core command that prints a diagnostic after its JSON on stdout would now
fail its smoke step; that is the intended closed-output contract. Rollback: revert this commit.

## Verification and NOT_RUN

Focused: `go test ./internal/companionrelease ./internal/releasecandidate ./internal/specindex`,
`go vet` on touched packages, gofmt and the lane doc gates. NOT_RUN: hosted CI, full
`go test ./...` and `make gate` (shared host; owner preference for scoped issue work), Linux
installed-platform qualification and retained candidate assembly.
