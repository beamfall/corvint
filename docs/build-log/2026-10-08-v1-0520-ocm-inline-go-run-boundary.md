# 2026-10-08: OCM inline Go `t.Run` case anchor boundary (V1-0520)

## Intent

Ticket V1-0520 reported that a single-line `TestOCMLinkFixture` containing a literal
`t.Run("TM-V0-008 exact anchor", func...)` refused the normalized selector
`test:TestOCMLinkFixture/case:tm-v0-exact-anchor` with `claim-selector-out-of-range`. The same
source written over several lines linked. The ticket asked for the expected behaviour to be
decided, not inferred from formatting. This change adds proposed `OCM-V0-018` to
`docs/specs/ocm-v0-dogfood.md`.

## Reproduction (expected vs actual)

The fixtures were rebuilt on `origin/main` f33ea8ef with `enumerateClaims`, the extractor that
`ocm link --test-path` resolves selectors against:

| Source layout | Extracted selectors |
| --- | --- |
| `func TestOCMLinkFixture(t *testing.T) { t.Run("TM-V0-008 exact anchor", ...) }` | `test:TestOCMLinkFixture` only |
| header on one line, `t.Run(...)` on the next, `}` on its own line | test claim + `/case:tm-v0-exact-anchor` |
| header on one line, `t.Run(...) }` on the next | test claim + `/case:tm-v0-exact-anchor` |
| `func TestA(...) {}` then the inline `TestOCMLinkFixture` line | `test:TestA`, `test:TestOCMLinkFixture` only |

The ticket author expected the inline case to link. In fact it is not extracted, so the refusal
follows from the current contract.

## Findings

- Enumeration (`ocm_goindex.go` `indexCases`) derives a candidate's parent from the match offset,
  so it proposes the inline candidate. `enumerateClaims` then keeps only candidates the read-path
  verifier accepts. The verifier (`ocm.go` `extractableGoClaim`) derives the parent with
  `nearestTest(lineStart)`: the last `func Test...(` header that ends before the anchor's line.
  For an anchor on its parent's header line, that is no test, or an earlier one. The candidate is
  dropped, and link cannot record a claim the next verify would reject.
- The cause is therefore the verifier's line-based parent rule, not a selector-normalization
  defect. It is distinct from V1-0275.
- The real defect was the diagnostic. The `/case:` miss hint named only the supported anchor
  shapes, which describe the inline literal too, so the refusal looked like a missing capability.

## Decision

The boundary is stated, and the verifier is not widened. Accepting inline anchors means changing
the read-path parent rule. That would change which recorded claims verify, which is authority,
and it needs oracle/divergence adjudication. The frozen Python oracle is not in this snapshot to
compare against. `OCM-V0-018` makes the layout intentionally unsupported syntax. It appends
`, nor on its parent func header line` to every `/case:` miss hint. Extraction, selection and
verdicts are unchanged. Supporting the layout later is a separate capability: a verifier contract
change.

The hint already changes stderr under the open DR-0032 (no frozen corpus row discriminates). The
test-local message bound in `TestOCMClaimSelectorMiss` was raised from 320 to 360 bytes. The
normalized fragment is still bounded, and only the fixed shape list grew.

## Verification

- `GOMAXPROCS=3 go test -p 1 -count=1 -timeout 30m ./internal/lrfrepo`: ok. This includes the
  new `TestOCMInlineGoRunCaseBoundary`, which covers the two multi-line layouts (they link) and
  the two inline layouts (no case claim, the test claim is kept, and the miss names the boundary).
- `go test -run 'TestOCM.*(Claim|Selector|Link)' ./cmd/corvint`: ok.
- `go vet ./internal/lrfrepo`, `gofmt -l`: clean.
- Lane doc gates (spec-requirements through diagnostic-coverage) and `go test ./internal/specindex`: pass.

## NOT_RUN

- `make gate` and full `go test ./...`: per the owner's scoped-work preference.
- Comparison with the frozen Python oracle: not present in this repository snapshot.
- Dogfood CEM bind/seal: lane rules (no push or task-store mutation).

## Rollback

Revert the commit. This restores the previous hint text and the 320-byte test bound, and removes
`OCM-V0-018` and its traceability row. No wire, store or verdict state changes.
