# Decision 0092 — three uncovered Go-only observables become spec-owned, not go-defects

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls
and record them (orchestration thread, 2026-09-12).

A wire-variant audit found three Go-only observable changes on parity-covered commands. No frozen
parity case exercises them, and no spec required them. That is the same shape as the `baseSpan`
member that `0091-cem-status-basespan-is-a-go-defect-2026-09-12.md` removed. Each is adjudicated
here. Two clauses apply to all three:

- `GPK-V0-002` (`docs/specs/go-production-kernel-migration-v0.md`) makes a field, normalization,
  or error-text change "a wire change owned by its existing spec, not a migration exception".
  Amending the owning spec is therefore the sanctioned route. `GPK-V0-003`'s ban on a `go` wire
  variant forbids an unowned divergence, not a spec-owned one.
- Decision 0088 and `GOC-V0-001..003` (`docs/specs/go-only-cutover-v0.md`) retired the Python
  oracle. The live cross-check can no longer reach these paths, and `GPK-V0-033`/`GOC-V0-002`
  forbid transcribing expectations from the candidate. Each kept behavior is therefore pinned by an
  independent spec assertion (`GOC-V0-003`), not by a re-captured parity case.

`0091-cem-status-basespan-is-a-go-defect-2026-09-12.md` differs on one point. It removed `baseSpan`
because that member broke a frozen parity expectation (`cem-status-bound`) and had no owning spec
text. None of the three changes below alters any frozen case's bytes. `TestGPKV0002ManifestReplay`
passes on the base of this change.

## Calls

1. **`90c94ea0` writer secret screen: keep.** Commit `90c94ea0` added four shapes to
   `internal/secretscreen.Pattern`: a whole quoted bare-assignment value, the bare `pass` stem,
   `whsec_`/`hf_`/`dop_v1_`/`xapp-` and Slack webhook URLs, and an AWS key ID with its adjacent
   secret. `record` now refuses these inputs, and `query` history learning drops them. Admitting
   them is a secret-exposure failure under AGENTS.md invariant 5 ("secret-screened") and under the
   go-only-cutover failure model ("secret exposure ... blocks completion"). No clause requires the
   oracle's narrower screen, and this decision does not revert a security improvement. The shapes
   become numbered requirement `LTA-V0-004` in `docs/specs/learned-trace-admission-v0.md`.
   `StoredV1Pattern` and the dashboard readers are unchanged. The Python-parity follow-up is closed
   because no Python matcher remains to port to (`GOC-V0-001`).
2. **`2b91d358` `cem prepare` recovery line: keep.** `CEM-PILOT-002` (`docs/specs/cem-pilot-kit.md`)
   requires the refusal of an outdated or invalid map, and that refusal and its exit status are
   unchanged. The appended line names the `--replace` recovery that clause already prescribes. It
   carries no source content, and a genuinely absent or unreadable map keeps the fixed
   `cannot read CEM map` text, so frozen case `cem-unreadable-map` is unchanged. The line becomes
   `CEM-PILOT-018`.
3. **`144140a6` invalid top-level command choice list: keep.** The oracle's argparse rule lists
   every registered top-level command in declaration order. Its 13 entries were that rule applied
   to its own surface. Each verb added since then is owned by its own spec. Freezing the list at 13
   would make the refusal understate the verbs that `GPK-V0-001`'s "help" and "validation"
   compatibility surface actually dispatches. The oracle's 13 stay as the leading run, and no frozen
   case uses an invalid top-level verb. The rule becomes accepted amendment `GPK-V0-059`.

A future change to any of these three observables is a change to its numbered requirement and
must amend that requirement in the same commit.

## Rollback

Revert this decision's commit. That removes `LTA-V0-004`, `CEM-PILOT-018`, `GPK-V0-059`, and their
traceability rows and focused test assertions, and restores the fixes.md parity entry. The three behaviors then return to
unowned status under `GPK-V0-002`. Reverting a behavior itself is a separate code change:

- `90c94ea0`: revert only its `secretscreen.go` hunk, plus a matching analyzer schema pin bump.
- `2b91d358`: drop the `GuidanceOf` append in `emitCEMError`.
- `144140a6`: truncate `topLevelCommands` to the oracle's 13 entries.

None of these reverts rewrites stored rows, maps, or receipts.
