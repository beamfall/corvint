# Decision 0436 — Owner accepts the settlements of nine disputed panel findings

Date: 2026-10-06. Status: accepted by the owner on 2026-10-06 ("approval covered the V1-0334–0344
settlement too"). Tickets: V1-0334, V1-0335, V1-0336, V1-0337, V1-0339, V1-0340, V1-0342, V1-0343
and V1-0344.

## Context

The pre-1.0 expert panel left these findings disputed: both verifiers confirmed the code facts but
split on whether each was a defect or an accepted choice. Each ticket's acceptance criterion is "the
dispute is settled by an owner-accepted spec amendment or decision, and any resulting fix lands
with a regression test". The fixes and amendments listed below landed on `main` before this decision.
None of them had recorded owner acceptance. V1-0338 and V1-0341 were already completed and are not
part of this decision.

## Decision

The owner accepts each settlement as it landed:

| Ticket | Panel | Commit | Settlement | Regression tests |
|---|---|---|---|---|
| V1-0334 | D1 | `9cc47ec5` | The in-repo second consumer is scoped to cem/0.1 in the product spec's Core item 2 and in the README. | Documentation scope only: no behaviour changed. |
| V1-0335 | D2 | `c9b5eed2` | `cem cover`, `discriminate` and structural `mark` never write a cem/0.3 map over a cem/0.2 input or the Core sidecar. | `internal/cem/workflow/{cover,discriminate,workflow}_test.go` |
| V1-0336 | D3 | `2e4a264e` | The stripped repository and CEM read refusal codes are emitted (decision 0398). | `cmd/corvint/{cem,main,dogfood_flow_portable}_test.go`, `conformance/cli-parity-v0/runner_test.go`, `internal/cem/cli/cli_test.go` |
| V1-0337 | D4 | `ab6c752c` | Every frozen Core mode is compared with a structural golden. | `cmd/corvint/core_freeze_test.go` |
| V1-0339 | D6 | `93db88dc` | The aggregate source bound refusal is language-neutral and names the total and the remedy. | `internal/contextindex/{analyzer_schema,git,query}_test.go`, `cmd/corvint/prove_checkpoint_test.go` |
| V1-0340 | D7 | `ec50af2d` | An unowned dirty path selects its package and importers, as gate rules (a) and (b) do. | `internal/liveverify/affected/golang/golang_test.go`, `cmd/corvint/usecase_hostile_change_consequence_test.go` |
| V1-0342 | D11 | `753b0314` | `affected` takes AGENTS.md checks only under an exact Verify heading, strips shell comments, and makes launch commands advisory. | `cmd/corvint/affected_test.go` |
| V1-0343 | D12 | `e345beb9` | The test linker refuses a link from one plain-word mention, and a lexical-only test row is conditional. | `internal/contextindex/{taskcontext_testlink,analyzer_schema}_test.go` |
| V1-0344 | D14 | `90f70843` | The README's abstention claim is scoped to what the status table supports. | Documentation scope only: no behaviour changed. |

## Rollback

Revert this file and its index row, and reopen any ticket completed on it. Reopening a dispute needs
a new spec amendment or decision. The landed fixes are reverted by their own commits, not by this
decision.

## What this does not claim

This decision does not re-run or re-review the landed changes. It records owner acceptance of
settlements whose code, specs and tests are already on `main`.
