## 2026-10-06 V1-0855: priority admission ranks waiting downstream claims first

Human-owned intent: owner request [issue 626](https://github.com/beamfall/corvint/issues/626),
ticket V1-0855. With `priorityAdmission` on, an implement-stage pooled claim for an equal-priority
ticket earlier in plan order took the only free member while a review claim for a ticket that gated
a phase had waited longer. CAL-V0-101 ranked competitors by plan order only.

Requirement: new `CAL-V0-105` (V1-0855 subsection of `## Requirements` in
`docs/specs/corvint-tasks-agent-leases-v0.md`); `CAL-V0-101` amended to rank by admission order.

### Change

- `internal/tasks/transaction/priority_admission.go`: a derived `admissionRank` (priority; at equal
  priority a ticket whose latest terminal generation at the current acceptance revision handed off
  to `review` or `integrate` first; downstream tickets by handoff `phaseSinceSeq`; then plan order).
  `admissionQueue` lists a pool's unblocked `OPEN` competitors in that order, reading a downstream
  competitor's blockers at its own stage; `priorityWaiting` is the queue prefix ranked ahead of the
  claimed ticket. The yield detail keeps its text and appends `; <ticketId> awaits <stage> since seq
  <seq>` when the ticket yielded to is downstream.
- `plan.go` `PriorityFirst`: the running plan-order waiting list is replaced by one precomputed
  queue per opted-in pool and a prefix lookup per entry, so plan, `claim --next`, explicit claims and
  `ticket show` claimability share one order. Pools without the flag build nothing.

### Decisions

- Downstream-first at equal priority is the derived stand-in for "waited longer": an implement
  ticket's wait is not recorded, and a review that open tickets depend on is downstream anyway. No
  priority inheritance from dependents and no `critical` field (owner question); a lower-priority
  review still yields to a higher-priority implement.
- `dispatch status` is unchanged: it reads dispatcher configuration and ledger, not admission. The
  decision is in the refusal detail, the plan blocker and the competitor's `nextStage`.
- The rank is read from records, so a claim's `--stage` does not change its own rank.
- Selection stays sound: a candidate at queue position p < free members is never deferred, because
  each earlier-in-plan selection ranked behind it already counts it as waiting.

### Evidence

`GOMAXPROCS=2 GOTOOLCHAIN=local go test -p 1 -count=1 -timeout 30m -run
'CALV0101|CALV0105|CALV0098|CALV0104|CALV0097|CALV0084|CALV0082|PriorityAdmission|Priority'
./internal/tasks/transaction ./internal/tasks/store ./internal/tasks/cli`: all `ok`. New
`TestCALV0105_WaitingReviewOutranksArrivingImplement` (the issue's order; flag off admits the
implement claim as before), `TestCALV0105_EarlierHandoffWinsAtEqualPriority`,
`TestCALV0105_AdmissionRank`; the 400-case `TestCALV0101_PlanClaimAndClaimNextAgree` now generates
handoffs and requires at least one yield to a downstream ticket; `TestCALV0101_FlagOffMatchesNMinusOne`
keeps its pinned pre-CAL-V0-101 digest. `go vet` (darwin, `GOOS=linux`, `GOOS=windows`) on the three
packages, `gofmt -l`, `make spec-requirements-check requirement-definitions-check
line-citations-check traceability-tests-check` and `go test -run TestAgentLeasesSpec
./internal/lrfrepo`: clean.

NOT_RUN: full package suites, `make gate`, live dispatcher or multi-agent qualification.

Rollback: revert the commit. No stored field or request preimage changes; admission returns to plan
order at the next read or claim.
