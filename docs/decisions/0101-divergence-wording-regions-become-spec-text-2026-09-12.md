# Decision 0101 — three divergence wording regions become owning-spec text

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls
and record them (orchestration thread, 2026-09-12).

Three `python-defect` entries in `conformance/divergence-register.md` pinned bytes that no accepted
clause stated. Reopening them as `spec-gap` would block `PASS` with no oracle left to discriminate
(decision 0088), so each gap closes by amending the owning accepted clause to state the Go behavior
already shipped. No production code or expected bytes change.

- `DR-0007` -> `CF-V0-032` (`docs/specs/change-frontier-v0.md`): on a surface with no
  caller-independent revision, each withheld-authority uncertainty entry is exactly
  `<reason>: no caller-independent revision is available at which to resolve the cited decision
  status, so accepted authority is withheld`, deduplicated, in byte order, ahead of every other
  `coverage.uncertainty` line.
- `DR-0026` -> `GPK-V0-052` (`docs/specs/go-production-kernel-migration-v0.md`): the disclosure is
  exactly `N test-path symbol candidates withheld from query ranking; the context verb serves test
  evidence`, absent at zero.
- `DR-0011` -> `GPK-V0-027`(c): a web specifier is read only from a `'`/`"` literal in live code in
  the three positions the lexer reads; comment, string, and template-literal bodies contribute no
  edge; `require(...)` is not recognized; an unterminated block comment or template literal records
  `WEB_SOURCE_UNPARSED`.

Each amendment adopts wording that users and the parity manifest already observe, so it asserts no
new behavior. The adjudications stay `python-defect` / known-divergent.

Rollback: revert this decision's commit. That removes the three amendment sentences, the register
closure notes, and the exact-wording assertion in `TestImpactWithholdsUnverifiableADRAuthority`, and
restores the fixes backlog entry; no runtime bytes change.
