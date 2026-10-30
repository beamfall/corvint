# Decision 0240 — docviews binds the admitted HDC plan

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

Decision 0231(j) left two shapes under one profile string: `internal/docviews` bound the experimental
`doccompiler.PatchPlan` as `corvint-human-documentation-plan/0`, while the admitted `HDCV0-027` wire
under that string is `AdmittedPlan`, and `VerifyAdmittedPlan` refuses `PatchPlan` bytes. This
decision supersedes 0231(j).

The call:

(a) Source. `docviews.Compile` and `docviews.Verify` take a `SourcePlan` of the index, the admitted
plan bytes, and the proposal patch. Both run `doccompiler.VerifyAdmittedPlan`; a missing index or any
refusal is `INVALID_PLAN`. The truth binds the SHA-256 of the exact plan bytes and the plan's
`source_identity.revision`, so `CompileOptions` no longer carries a revision. `Verify` also reports
`PLAN_BINDING_MISMATCH` when the truth's plan digest or revision differs (`CATN-V0-001`, `009`).

(b) Truth claim members. A truth claim copies the admitted clause: `id`, `state`, `kind`, `scope`,
`text`, `frontier` (only on `UNKNOWN`), and its anchors as `evidence` (`path`, `blob`, `start_line`,
`end_line`, `span_sha256`, `authority`, `reason`). The `PatchPlan`-era `status`, `uncertainty`, and
per-anchor `revision` members are gone. A `SUPPORTED` claim needs one anchor and a `CONFLICTED` claim
two at distinct `(path, blob, start_line, end_line)` (`CATN-V0-003`, `014`).

(c) Clean break. No compatibility reader exists, because no bundle has been released. The fixed
vectors in `internal/docviews/compile_test.go` were regenerated; no other file pins them.

(d) `PatchPlan` stays experimental under its own profile,
`corvint-human-documentation-experimental-patch-plan/0` (`doccompiler.ExperimentalPatchPlanProfile`).
It is not retired: `Plan` emits it and `build` and `NewReceipt` consume it. `internal/docmaintain`,
`cmd/corvint` docs verbs, and the MCP docs bridge do not use it. The conformance runner's own
`planProfile` constant describes an external candidate's observation shape and is unchanged.

Delivery: `CATN-V0-001`, `003`, `009`, and `014` amended; HDC spec notes on `PatchPlan` updated. No
requirement IDs added. `CATN` remains a proposed, experimental pure-core slice.

Rollback: revert the commit. That restores `PatchPlan` binding, its shared profile string, and the
earlier digest vectors; no persisted state depends on either wire.
