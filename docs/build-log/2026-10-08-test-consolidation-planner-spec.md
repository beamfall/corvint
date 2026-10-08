# Test consolidation planner proposed (TCN-V0)

Date: 2026-10-08
Task: V1-1023 / GitHub #681
Base: 2a93b5a182966f72836cd28bf569dc44c0f5220c

Issue 681 asks for a deterministic "minimum focused tests" preprocessing step: agents write one
end-to-end test per matrix row, and the owner reports 133 flows, 3,614 core rows and 6,355
`NEW TEST REQUIRED` proposals in the adopter workspace (figures from the issue, not reproduced).
This change adds only the proposed executable spec
`docs/specs/test-consolidation-planner-v0.md` (TCN-V0-001..012), its README and INDEX rows and the
regenerated `REQUIREMENTS.tsv`. No code, test or wire changes; intent stays `proposed` and delivery
`not-started` until the owner records an acceptance decision.

## Design decisions recorded in the spec

- The input is an explicit closed `test-consolidation-input/0` document (variation IDs with spec,
  app, route, screen, setup, user, org, action, assertion, requires/changes facts, destructive and
  declared witnesses), so the grouping rules are reviewable apart from derivation. Derivation from
  AFU-V1 intents plus AMAP-V0 maps is owner question 1.
- Order rule: B precedes A when A changes a fact key B requires, which makes the plan's step order
  independent of earlier steps passing. Contradictory requires split a context into first-fit
  compatible sets, and dependency cycles re-plan the greatest variation ID in a further set; a
  variation left alone reads `conflicting-state`. This is a conservative consolidation, not a
  guaranteed global minimum (exact minimisation is a set-partition problem). Destructive variations are always isolated
  (owner question 3 asks whether last-step placement is acceptable).
- Reuse reads original provider documents (what `corvint test-validity --receipt` accepts),
  projects them through the shared builder and requires associated, eligible, current and passed
  axes; every qualifying witness keeps its five-axis projection and the table shows its strength
  (LPCV-V0-047/048). The join is on exact test `id`
  because the document does not carry AFU-V1 test keys (owner question 5).
- Duplicates are conservative: same context, requires, changes and destructive flag as well as
  action and assertion, so no declared effect is discarded.
- Missing, unresolved or stale anchors abstain and make the plan `INCOMPLETE`; the pasted-table
  check exits non-zero for an incomplete plan as well as any byte difference.
- Surfaces: `corvint test-plan consolidate|check` as named by the issue (owner question 2 offers
  a `flows` subcommand instead of a new root verb) and an opt-in corpus MCP tool
  `corvint.consolidate_tests` gated on owner acceptance; `corvint-mcp` stays frozen.

## Independent review

Codex (`gpt-6-astra`, read-only) reviewed `2a93b5a1..be5ef690`: 0 P0, 2 P1, 6 P2. All were
accepted and fixed in the spec: witnesses now read original provider documents, not projected
output (P1); duplicate equality includes `changes` and `destructive` (P1); one reason precedence
for every test (P2); reuse keeps all five axes and shows strength (P2); several witnesses are all
kept and ordered (P2); the table header binds map digests and the evaluated revision (P2); the
"minimum" promise is replaced by first-fit compatible sets and an explicit non-minimality note
(P2); qualification uses AFU-V1 per-variation outcome evidence, because `corvint test-validity` is
test-level only (P2).

A second Codex pass confirmed those fixes and found 1 P1 and 2 P2, all fixed: witness freshness is
recomputed against the evaluated revision by the LPCV-V0-053 binding rule, never taken from the
receipt (P1, new reason `witness-unbound`); absent `requires`, `changes`, `destructive` or `org`
abstain instead of defaulting (P2); facts are parsed as `key=value` and a set naming one key twice
refuses (P2).

## Evidence

Doc gates and `go test ./internal/specindex` are the only executable checks for a spec-only
change; their results are recorded in the lane report. All TCN-V0 acceptance evidence is
`NOT_RUN` (no implementation). Owner-run adopter qualification is `NOT_RUN`.

## Non-goals and failure modes

No test generation, execution or repository write; no side-effect inference. The main labelled
fail-open risk is an undeclared side effect, which can make an order wrong; the plan stays
`authority: candidate` and the written test's test-validity run is the check.

## Rollback

Delete the spec, its README and INDEX rows and this entry, then regenerate `REQUIREMENTS.tsv`.
