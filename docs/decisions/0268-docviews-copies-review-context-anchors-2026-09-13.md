# Decision 0268 — docviews copies review-context anchors without re-qualifying them

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`doccompiler.AdmitClauses` keeps every anchor on an admitted clause (`internal/doccompiler/admission.go`,
`admitClause`). An anchor `HDCV0-023` disqualifies (mutable blob, path-only, line-only, reversed range,
missing member) therefore survives as review context, on a clause demoted to `UNKNOWN` or beside a
qualifying anchor on `SUPPORTED`, and `VerifyAdmittedPlan` accepts the plan. `docviews.Compile` then
refused the whole plan as `INVALID_PLAN`, because `evidenceValid` required a hex blob, a hex span digest,
and an ordered line range on every anchor. A scratch probe through `CompileAdmittedPlan`,
`VerifyAdmittedPlan`, and `Compile` reproduced it for a `mutable` blob demoted to `UNKNOWN`, a path-only
and a reversed anchor demoted to `UNKNOWN`, an input `UNKNOWN` clause with a `mutable` anchor, and a
`SUPPORTED` clause with one qualifying and one `mutable` anchor. `CATN-V0-014` already said the profile
does not re-qualify anchors; the identity check was that re-qualification.

The call: `internal/docviews` copies every anchor the verified plan carries and does not re-qualify its
identity, authority, range, or staleness. Qualification is recorded only by the admitted claim state,
which `VerifyAdmittedPlan` reproduces against the index. `evidenceValid` now checks only that each anchor
string member is UTF-8 within the 64 KiB text bound. The claim-shape checks stay: a `SUPPORTED` claim
needs evidence, a `CONFLICTED` claim two distinct locations, an `UNKNOWN` claim its frontier. So an
`UNKNOWN` claim with a retained anchor compiles as `UNKNOWN` with its frontier. The anchor is review
context and certifies nothing, and a correctly demoted clause no longer refuses the whole document.

Rejected: refusing disqualified anchors at admission. `HDCV0-023` explicitly lets `UNKNOWN` retain
anchors as review context, and dropping them would hide why a clause was demoted. Also rejected:
stripping them from the truth corpus. `CATN-V0-003` and `CATN-V0-009` require the complete anchor values
and compare them against the plan, so the corpus would stop preserving the plan. Also rejected:
widening the truth evidence shape with a per-anchor qualification flag or a separate review-context
array. That adds a wire member and a digest change. The flag would repeat a verdict only the HDC
admission against the index can establish, and the claim state already carries it.

Consequence: no wire shape, profile, digest domain, error code, or fixed vector changes; a bundle that
compiled before compiles to the same bytes. A verified plan carrying a retained disqualified anchor now
compiles instead of failing closed. A consumer must read claim state, not the presence of evidence, as
qualification; the truth corpus does not say which anchor of a `SUPPORTED` claim qualified. `CATN-V0-014`
is amended, the `HDCV0-023..026` delivery note in `human-documentation-compiler-v0.md` names the retained
anchors, and `TestCompileKeepsDisqualifiedAnchorsAsReviewContext` covers the demoted and the
side-by-side case. No `analyzerSchemaID` bump: `internal/docviews` is not under `internal/contextindex`.

Rollback: revert the commit. That restores the identity check in `evidenceValid`, the whole-plan
`INVALID_PLAN` refusal, the unamended `CATN-V0-014`, and the `docs/agent-memory/ideas.md` entry.
