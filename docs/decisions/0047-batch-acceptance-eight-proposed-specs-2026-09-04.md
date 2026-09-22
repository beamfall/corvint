# Decision 0047 — eight proposed specs are accepted as intent; delivery stays not-started

Date: 2026-09-04. Status: accepted. Authority: repository owner, verbatim instruction "accept all"
followed by the eight titles below (2026-09-04), given after a portfolio listing that separated
"proposed and not started" specs from the rest.

## What is accepted

| Spec | Requirement prefix | Previous intent |
|---|---|---|
| `applied-intelligence-breakthroughs-v0.md` | (none) | proposed |
| `human-documentation-compiler-v0.md` | `HDCV0` | proposed |
| `live-proof-carrying-verification-v0.md` | `LPCV-V0` | proposed |
| `observation-corpus-authority-v0.md` | `OCA-V0` | proposed |
| `proof-carrying-context-optimization-v0.md` | `PCCO-V0` | proposed |
| `semantic-escalation-gate-v0.md` | `SEG` | proposed |
| `change-frontier-profile-1.md` | `CF-V1` | proposed (not accepted) |
| `change-witness-relation-v0.md` | `CWR-V0` | proposed (not accepted) |

Acceptance is of intent only: each spec's numbered requirements, non-goals, failure modes, and
acceptance evidence are now the owner-approved contract an agent may build against. Every delivery
status stays `not-started`, no promotion or kill gate is run, and no product claim follows from this
record.

## What acceptance does not change

- **Dependencies stay where they are.** `docs/SPEC-DRIVEN-DEVELOPMENT.md` requires a spec whose
  dependency is unaccepted to report `NOT_RUN` until that dependency exists. Live Proof-Carrying
  Verification depends on Test Claim Qualification V0 (proposed); Human Documentation Compiler on
  Main Use-Case Conformance V0 (proposed); Proof-Carrying Context Optimization on Session Context
  Dividend V0 (deferred); `frontier/1` on Change Frontier V0 (proposed overall) and on the
  superseded Harness Authority Relation, whose authority is decision 0009; Change Witness Relation
  on CEM 0.2 canonical binding, Change Frontier V0, Lexical Relevance Floor V0, and OCM V0 (all
  proposed). Those gates remain `NOT_RUN`.
- **Owner inputs the specs name stay open.** Change Witness Relation still waits on the
  symbol-resolver boundary decision it names; Human Documentation Compiler on independent re-review
  and renderer qualification; Observation Corpus Authority on capture fixtures and sealed-corpus
  replay; Semantic Escalation Gate on provider-spy conformance and calibration; Applied Intelligence
  Breakthroughs on frozen external outcome trials.
- **`frontier/1` amends nothing.** Its own text says it describes rather than makes any change to
  `frontier/0`; acceptance of the profile does not accept Change Frontier V0 overall.
- Observation Corpus Authority's acceptance satisfies the "accepted observation authority" item
  Proof-Carrying Context Optimization and the Self-Observation Ledger list as a blocker; the other
  items on those lists stand.

## Rollback

Restore `Intent status: proposed` (or `proposed (not accepted)`) and the digest status line in each
of the eight specs, the header prose in `change-frontier-profile-1.md` and
`change-witness-relation-v0.md`, the eight rows in `docs/specs/README.md`, the eight `intent`
fields in `docs/specs/INDEX.json`, regenerate `docs/specs/REQUIREMENTS.tsv`, and remove this
record from `docs/decisions/README.md`. Nothing built depends on the accepted state yet.
