# 2026-10-08: OCM anchor token-boundary guidance (V1-0555)

## Intent

Ticket V1-0555 records workflow friction. The literal `t.Run("CVI-V0-002-strict-grammar", ...)`,
selected with its normalized fragment, was refused `claim-obligation-mismatch`. The literal
`t.Run("CVI-V0-002 strict grammar", ...)` linked. The refusal did not say that a hyphen next to
the ID continues the requirement token. The change adds proposed `OCM-V0-017` to
`docs/specs/ocm-v0-dogfood.md` and documents the token rule in `docs/DOGFOOD.md` §5. The number
was checked as free on `origin/main`, `origin/claude/batch-h-2026-10-08` and
`origin/claude/batch-i-2026-10-08`.

## Decisions

- **The matcher is unchanged.** `containsExactRequirement` still treats `A-Z`, `a-z`, `0-9`, `_`
  and `-` as token bytes. `CVI-V0-0021`, `XCVI-V0-002`, `_CVI-V0-002_` and
  `CVI-V0-002-strict-grammar` stay refused for `CVI-V0-002`.
- **Only the `ocm link` refusal gains the hint.** `withAnchorBoundaryHint` runs only after the
  candidate closure fails `claim-obligation-mismatch`. It re-reads the anchors of the selected
  claims and appends `requirementBoundaryHint`. The hint names the ID, the adjoining ASCII byte of
  the first occurrence and its side, the token rule, and a space-separated `t.Run` example. It
  echoes no other anchor bytes. The verifier's issue message (`ocm verify`/`status` stdout) and
  every conformance vector stay byte-identical, so the frozen `ocm/0.1-experimental` profile is
  untouched. If the ID does not occur in the anchor, or the anchors cannot be re-read, the
  original refusal is returned unchanged.
- **The stderr difference is recorded under DR-0031.** On that fixture the Python oracle exits 0
  with empty stderr, so there is no oracle stderr to match. The note says the recorded Go stderr
  digest predates the suffix. The adjudication and the promotion hold are unchanged.

## Evidence

- `internal/lrfrepo/ocm_anchor_test.go:TestOCMLinkExplainsRequirementTokenBoundary` covers four
  ID tokens that are adjoined or ambiguous. Each stays refused with the hint and leaves the map
  bytes unchanged. An absent ID keeps the exact original message. The space-separated ID shares
  the hyphenated spelling's normalized selector `test:TestAnchors/case:ocm-test-exact-anchor`, and
  only the space-separated one links. The verifier issue stays
  `{"code":"claim-obligation-mismatch","message":"claim anchor lacks the exact obligation ID"}`. A
  matcher truth table confirms the matcher did not change.
- `cmd/corvint/ocm_test.go:TestOCMLinkRejectsClaimWithoutExactObligationIDBeforePublication`
  asserts the CLI stderr for the `_TM-V0-008_` fixture carries the hint.
