# Decision 0422: owner answers A1 to A6 for 1.0.0-rc.1

Date: 2026-09-26. Status: accepted (owner answer 2026-09-26: "A1 yes … A6 go-chi/chi").
Tickets: V1-0018, V1-0019, V1-0263, V1-0284, V1-0286, V1-0350, V1-0379.

## Context

On 2026-09-26 the agent listed ten owner decisions on the path to `1.0.0-rc.1` (A1 to A10), each
with a recommendation. Four of the decision 0421 blockers had merged fixes that waited only on
spec acceptance, and the readiness record and the untouched-repository validation waited on owner
choices. The owner answered in the form the list suggested, "A1 yes … A6 go-chi/chi". That answer is
recorded as yes to the recommendation for A1 to A5, and `go-chi/chi` for A6.

## Decision

1. **A1, Core compatibility freeze.** `core-compatibility-freeze-v1.md` is accepted as a whole.
   Every clause it marked proposed is accepted:
   - the three CCF-V1-004 refusal exemptions (panel blocker B5);
   - the V1-0284 map-first and promisor classification;
   - the decision 0398 codes, per-mode goldens and register rows;
   - the V1-0350 register rows;
   - the N-1 replay, `make core-n1-replay`, at runbook step 8.

   The V1-0340 removal of the `plan.excluded[].reason` value
   `UNINDEXED_DIRTY_GO_PATH_MAY_BE_DELETED_OR_RENAMED`, which 0.8.1 writes, becomes part of the
   baseline.
2. **A2.** `GPK-V0-070` is accepted: markers come only from comments and relate to the change
   (V1-0263, panel blocker B4).
3. **A3.** The V1-0286 amendments are accepted: the `build-cost.json` record of
   `IDX-SNAP-V0-012`, and the snapshot miss in `agent-harness-integration-v0.md` that reports stale
   without building when the recorded cost cannot fit the deadline.
4. **A4, readiness record.** `stable-readiness-record-v1.md` is accepted, and so is the
   SRR-V1-012 command shape: a new binary, `cmd/corvint-readiness-record`, with two modes.
   - Build mode takes `-candidate DIR -source-root DIR -evidence-file TSV [-store-release ID
     -store-candidate-sha256 HEX] -output FILE` and does not replace an existing output file.
   - Verify mode takes `-verify FILE -candidate DIR -evidence-file TSV`.

   `cmd/corvint-release-candidate` stays unchanged. The implementation and its tests land
   separately, and no release uses the record before they do.
5. **A5, licensing.** `cmd/corvint/frontier.go`, `cmd/corvint/frontier_adapters.go` and
   `cmd/corvint/frontier_test.go` keep their Apache-2.0 notices as an owner-approved exception to
   the path map. Its own change names the three files in `LICENSING.md` and amends decision 0002
   (V1-0379). Nothing is relicensed, and no notice changes.
6. **A6.** The untouched public repository for V1-0019 (`PRS-V1-008`) is
   `github.com/go-chi/chi`. The commit and the cases are frozen before any execution.

## Non-goals

- This decision completes no ticket and changes no dependency. Those store mutations stay
  owner-run.
- It changes no delivery status, and it is not qualification, a candidate or promotion.
- It does not decide A7 to A10:
  - A7, the BUILD-LOG layout (V1-0358);
  - A8, the uncommitted 2026-09-26 audit output;
  - A9, GitHub issue comments;
  - A10, freezing the `corvint-tasks` repository (V1-0318).

## Rollback

Revert this change. The four specs return to their proposed markers, and the SRR-V1-012 shape
returns to a proposal. Reopen any ticket that was completed on the strength of this decision:
V1-0263, V1-0284, V1-0286 or V1-0350.
