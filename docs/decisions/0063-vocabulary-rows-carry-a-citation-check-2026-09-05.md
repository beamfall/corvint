# Decision 0063 — Vocabulary rows carry a citation check, and a packet proven only by citations is `CITED`

Date: 2026-09-05. Status: accepted. Authority: repository owner, delegated call; the owner instructed on 2026-09-05 that open calls be resolved through the expert panel, and this record applies the panel memo `docs/reviews/panels/panel-6-proof.md` after independent review of its cited evidence.

## Context

`prove --task` returns zero verdicts on any vocabulary-only packet (`docs/reviews/r12-cold-start.md`
F6, confirmed on four cold repositories): `syntax` rows cite a blob and line Git can check, yet
`FPK-V0-003` assigns them `none`, so they are always `NOT_RUN`, and a stale citation can never fail.
`FPK-V0-004` defines `history-consistent` purely as a citation check for every authority that
carries it.

## Decision

1. `syntax` rows are assigned `history-consistent`; the language fallbacks in `falsifierFor` fall
   through to the authority table instead of returning `none`.
2. A new `FPK-V0-027`: an always-present `proof.claims` legend keyed by falsifier states what each
   verdict means; the `history-consistent` entry states that the cited blob and line still exist at
   the packet revision and that this is a check of the citation, not evidence that the row is
   relevant. The same sentence replaces the help text. A `READY` packet whose only falsifier-bearing
   rows are `syntax` with `history-consistent` reports `state: CITED`, so `proven_results` beside a
   `coverage.authoritative_results` of zero cannot read as invented certainty. The falsifier
   vocabulary stays at five; a per-row claim string is rejected as byte bloat that re-pins
   `FPK-V0-007` on every wording edit.
3. `FPK-V0-006` drift is tested in every non-checkpoint mode by one Go table test (four modes by two
   drift kinds, one CEM map-bytes kind, four controls); `prove` has no Python oracle, so no parity
   row is constructible. `compileCEMProof` gains the HEAD-tree re-read it lacks; with an explicit
   non-HEAD `--target` the tree half may refuse conservatively, stated in `FPK-V0-012`.
4. Every `Test…` token inside a spec's traceability table must resolve to a test function in the
   repository, enforced by `script/check-traceability-tests.sh` wired into `make gate`; a planned
   obligation is prose or a name followed by `(PLANNED)`, which the script counts and skips.
5. Evidence breakthroughs: replayable mutation witnesses (b2 B1) are accepted as a prototype in
   `proof.rows[].witness`, never in the oracle-pinned packet, with the falsifier that at least 19 of
   20 witnesses replay as kills on a second checkout; the offline cosign verification of the existing
   attestation envelope is accepted as a test; the attestation graph, reproduced receipts and the
   falsification ledger are backlog behind B1 and a supply-chain review; authorship-bound hunks are
   rejected as a build.

## Consequences

Vocabulary rows can fail on a stale citation; `proven_results` becomes non-zero on cold repositories
while `coverage.authoritative_results` stays zero; `proof.rows` bytes change and `FPK-V0-007`
re-pins; the query, impact, feature and harness wires are untouched.

## Rollback

Revert the table row, the fallback change and clause `FPK-V0-027`; the pre-change wire returns
exactly, since the packet is byte-unchanged throughout. The gate script and the drift test are
independently revertible.
