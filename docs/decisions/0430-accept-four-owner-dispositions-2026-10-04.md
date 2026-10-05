# Decision 0430: accept four pending owner dispositions

Date: 2026-10-04. Status: accepted (owner answer 2026-10-04: "I want you to accept them").
Tickets: V1-0711, V1-0742, V1-0743, V1-0745.

## Context

Four delivered changes left a disposition that their contracts reserve for the owner. The agent
listed the four and the owner accepted all of them.

## Decision

The owner accepted, as written:

1. **V1-0711, replaced deadline criterion** (`AHI-044`, `docs/specs/agent-harness-integration-v0.md`).
   The ticket asked for a case where the host deadline is shorter than the adapter's budget. That case
   is replaced, not met. No budget the adapter derives can meet a kill earlier than the declared one.
   `TestAHI017AdapterHostKillMatchesDeclaredHooks` keeps the shipped timeouts equal to the declared
   kill table, and the two delay cases force each adapter onto its own deadline first. The inferred
   consequence of a host that kills earlier anyway is accepted: scratch, temporary files and Git
   children can remain. V1-0734 tracks children that outlive the adapter.
2. **V1-0742, sealed harnesses** (`docs/build-log/2026-10-04-v1-0742-harness-read-bounds.md`).
   Acceptance criterion 1's bounded `agent_cost` reader is applied to
   `benchmarks/daily-loop-v0/harness.py` and `benchmarks/untouched-repository-v1/harness.py` only
   in each harness's next numbered preregistration. Their sealed harness digests and committed
   preregistrations stay unchanged until then, and no seal is edited in place. A follow-up ticket
   tracks that step, so V1-0742 may close with criterion 1 met for daily-loop-v1 only.
3. **V1-0743, `DCW-V0-032`** (`docs/specs/daily-change-evidence-workflow-v0.md`). The requirement
   text is accepted. It amends the accepted `DCW-V0-019` empty-plan sentence for `dogfood change`
   only.
4. **V1-0745, `LTPM-V0-015` and `LTPM-V0-016`** (`docs/specs/local-trace-producer-migration-v0.md`,
   decision 0429). The requirement text is accepted.

## Consequences

The requirement labels and the decision 0429 status now read accepted. Each ticket still closes only
after its acceptance evidence is retained and the native completion write succeeds. Acceptance
covers these dispositions only. It does not accept other proposed requirements those tickets
touched, such as the V1-0742 amendments in `docs/specs/cem-reviewer-trial-v0.md`. Rollback: revert
this record and the label changes. The requirements then read proposed again, and the four tickets
return to waiting on the owner.
