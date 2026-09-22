# LRF V0 spec-coverage audit

Method (`GPK-V0-034`): for each normative clause in
`docs/specs/lexical-relevance-floor-v0.md`, ask whether a corpus case exists that would **fail if
that clause were violated**. Pass counts do not answer this. A clause with no falsifying case is
`UNVERIFIED` no matter how green the suite is.

Corpus: `conformance/lrf-v0/cases.json` — 41 cases, 41 distinct `coverageId`s, 75 hunks (3 deletions).
The table below predates the eleven cases added on 2026-09-13; the executed per-clause mutations and
the clause parts still `UNVERIFIED` are in `docs/reviews/corpus-mutation-audit-2026-09-13.md`.

**Evidence labels.** `confirmed` = verified by running code. `inferred` = mapped from `coverageId`
names and corpus field probes, not executed per-clause. Inferred rows are hypotheses; promoting the
audit to a gate requires executing a falsifying mutation per clause.

| Clause | Subject | Falsifying case | Status | Evidence |
|---|---|---|---|---|
| `LRF-V0-001` | structural CEM/OCM validity | `failure.structural`, `context.02-cem`, `context.02-ocm`, `ocm.no-cem-witness` | covered | inferred |
| `LRF-V0-002` | frozen identifier algorithm, ID/stop-term redaction | `token.identifier-split`, `token.id-redaction`, `token.stop-terms`, `token.basename` | covered | inferred |
| `LRF-V0-003` | span ≤ 512 bytes **and** ≤ 20 LF lines | `cem.span-cap` | partial | inferred — one case for a two-part bound; the byte and line limits are not separately falsified |
| `LRF-V0-004` | basis external to both endpoints | `cem.self-reference`, `cem.deletion` | covered | inferred |
| `LRF-V0-005` | relation enum, independent basis evaluation | `cem.extra-basis`, `cem.candidate` | covered | inferred |
| `LRF-V0-006` | nonqualifying overlap (IDs, map paths, removed bytes, old rename basename) | `cem.no-overlap`, `ocm.no-overlap`, `ocm.nonmaterial` | partial | inferred — the clause names "old rename basename" explicitly; no case pairs a rename with an overlap through the old basename |
| `LRF-V0-007` | obligation hunk shares a requirement term | `ocm.no-links`, `ocm.candidate` | covered | inferred |
| `LRF-V0-008` | `lexically-proximate-candidate` preconditions | `ocm.candidate` | partial | inferred — one case for a multi-condition rule |
| `LRF-V0-009` | claim anchors are structural only and MUST NOT upgrade a result | `claim.no-upgrade` | covered | **confirmed** — two subcases differ ONLY in `claim_paths`; absent yields `lexically-proximate-candidate`, present yields `rejected`. Seeded regression: asserting an upgrade on the anchored subcase fails |
| `LRF-V0-010` | claim-ID reuse is non-gating; no change to candidate state or issue ordering | `claim.reuse-inert` | covered | **confirmed** — two subcases differ ONLY in whether both obligations name the same claim path; expected output is byte-identical. Seeded regression: a reuse-dependent result set fails |
| `LRF-V0-011` | candidate stays separate from execution observation | `execution.not-observed` | covered | **confirmed** — a `test-claim` relation basis yields the ordinary lexical disposition `cem-lexical-v0`. Seeded regression: execution vocabulary in a disposition fails |
| `LRF-V0-012` | canonical tuples, ordering, byte-identical output | `determinism.repeat`, `commitment.mutation` | covered | inferred |
| `LRF-V0-013` | additive preflight; MUST NOT rewrite CEM/OCM fields | `commitment.mutation` | partial | inferred — one case for a broad prohibition |
| `LRF-V0-014` | aggregate lexical-byte / term / edge bounds | `failure.bound` (`*.at-limit`, `*.plus-one`, `lexical-bytes.deletion-stem`, `terms.deletion-old-path`) | covered | **confirmed** — boundary probing is exact, and the deletion cells for BOTH the lexical-byte and subject-term bounds now exist (DR-0001, DR-0002), each proven live by a seeded-divergence regression |
| `LRF-V0-015` | study corpus subject/selection/adjudication sealing | — | out of scope | this corpus tests the CLI/library, not the 60-subject study protocol |
| `LRF-V0-016` | independent custodian seals results over 60 subjects | — | out of scope | as above; needs its own evidence trail |

## Summary

- 11 of 16 clauses have a falsifying case. Four are **confirmed** by execution rather than inferred:
  `LRF-V0-009`, `-010`, `-011`, and `-014`.
- 4 are **partial** — a single case standing in for a multi-part rule, so violating one part may
  still pass.
- **0 remain UNVERIFIED.** The three prohibitions that were the blind spot now each have a case whose
  result changes if the prohibition is violated, and each was proven live by a seeded regression that
  asserts the violation of its own clause. A corpus of positive results cannot demonstrate a
  prohibition; a differential pair can.
- 2 are out of scope for this corpus and need separate evidence.

The corpus stood at 30 cases when this summary was written. The remaining weakness is no longer coverage but depth: four
clauses are still `partial`, each carrying a single case for a multi-part rule, so violating one part
of one of those rules can still pass. `LRF-V0-015` and `-016` remain out of scope for this corpus.

## Next actions

1. ~~Add the `lexical-bytes.deletion-stem` sub-case (DR-0001)~~ — DONE (`5d61722`). The `ocm` lane
   turned out never to touch `conformance/lrf-v0/`, so the guard was blocking on a collision that did
   not exist. ~~Add `terms.deletion-old-path` (DR-0002)~~ — DONE (`2f66d22`).
2. ~~Author falsifying cases for `LRF-V0-009`, `-010`, `-011`~~ — DONE (`0f4f5d3`).
3. Split the partial rows so each part of a multi-part clause has its own falsifying case. Eight
   parts gained cases on 2026-09-13; the rest are listed in the mutation audit.
4. ~~Mechanize: perturb each spec-authored expectation and confirm at least one case turns red~~ —
   DONE 2026-09-13, `docs/reviews/corpus-mutation-audit-2026-09-13.md`.
