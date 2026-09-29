# Fresh-agent checkpoint handoff pilot

Owner request: implement the OpenRig comparison's recovery qualification first (2026-09-29).
Ticket: V1-0473. Base: `85cb2889444ad41ac001876fca2b0000c76a09a7`, from `origin/main`.

The existing FPK-V0-020..026 wire already carries task, immutable handles, obligations and
historical verification; V1-0199 already delivered a separate dogfood handoff receipt. The useful
missing evidence was a fresh-session outcome, so this slice adds a repeatable public-CLI pilot
and its recipe rather than another writer, wire format or orchestration layer. OpenRig's explicit
restore outcomes and separation of observed state inspired the experiment; no source was copied.
Reference: `mvschwarz/openrig@d83bbebe97430c3f2777226fecc089d31e276aa8`,
`docs/as-built/architecture/lifecycle-snapshot-restore.md` and
`packages/daemon/src/domain/activity-taxonomy.ts`.

## Frozen experiment and outcome

[Recipe](../experiments/checkpoint-handoff-pilot.md) records the frozen task, exact outcome table,
operator prompts' scope, failure modes, non-goals and rollback. [Retained observations](../experiments/checkpoint-handoff-20260929.json)
contain the fixture/binary/checkpoint/receipt identities, raw deterministic receipts, independent
outcome results, repaired source/tests and verbatim agent reports. Local command artifacts remain
under `/tmp/corvint-recovery-handoff-20260929/`; these are diagnostics, not portable authority.

- Four CLI cases passed: unchanged, byte-identical repeat, committed code drift and deleted test.
  Every chosen handle verdict/drift flag and missing recovery row matched the expected case.
  Task/unknown/failed-approach prose, obligations and historical observations survived unchanged.
- Two fresh native agents, one checkpoint arm and one structured-notes arm, each repaired the
  same original fixture and passed eight evaluator-owned boundary cases. Both independently
  wrote tests that first exposed the defect. Neither saw the other arm or evaluator answers.
- Negative control failed on the original upper-bound bug, as expected. Existing-output refusal
  passed. SIGINT and SIGTERM retired the runner's live child and grandchild; initial `ps` inspection
  was sandbox-blocked and the same cleanup check passed with process-inspection permission.
- Both agents reported a default Go cache permission failure, then verified with their own
  temporary cache. Self-reported partial work intervals were 40 and 42 seconds; source read/view
  counts were four each. These intervals exclude different setup/reporting work and are not
  comparable full-task measurements. Billed tokens, cache use, cost and complete latency are
  `NOT_OBSERVED`. There is no savings claim.

This is one synthetic task and one session per arm on one host. The builder interruption was
represented by a committed unfinished fixture, not an actually killed process. Agent recovery
used unchanged evidence; drift was exercised separately by CLI cases. AT-06 remains experimental,
the separate AT-08 outcome comparison is not qualified, and SESSION-V0-001..016 stay deferred.

## Evidence routes and review

Used: original `query` and tracked-path `impact`, retained at the private Git evidence directory;
`affected` before checks; actual `prove --checkpoint` in the pilot; CEM/OCM and dogfood at closeout.
The pre-change query was a bounded governance lead in the clean worktree, not proof of behavior.
The mandatory start dogfood attempt reported no diff/CEM, missing intent scope and outcome input;
these pre-implementation failures remain in the local start log. Native host hooks ran in the
original chat checkout; qualification of hooks in the isolated worktree is `NOT_OBSERVED`.
Mutation testing, web flows, retrieval-learning evaluations and external-provider execution are
not applicable to this qualification-only change.

Gate A found no HIGH concern. Its two MED findings were adopted: freeze independent behavior
assertions and exact deterministic expectations. Both agent arms explicitly resume the unchanged
builder revision. Runtime selection was requested as Astra/medium for both fresh arms and the
independent reviewer because the task concerns evidence attribution; exact provider telemetry is
not independently observable. The parent runtime was not changed.

Focused checkpoint/handoff baseline passed (`go test -count=1 -timeout 30m ./cmd/corvint -run
'TestCheckpoint|TestDogfoodHandoff'`, 13.607s). Final checks and review are retained in the change's
bound reports. Independent review found two MED gaps: command artifacts lost on interruption
and missing exact dispatch prompts. Both were repaired: stdout/stderr, exit, elapsed time and
interruption reason now survive SIGINT/SIGTERM/timeout, and the retained result includes the
original dispatch messages with `fork_turns: none`. The timeout check shortened the deadline
through a test seam. A documentation check caught an accidental requirement-definition-shaped
prose line; it was reworded without changing a requirement. `make gate` is `NOT_RUN` under the owner's focused scoped-work policy.

The native claim was refused with `RESOURCE_COLLISION`: live attempt
`attempt:corvint:main:4df9cd37dec30e656bfce759dfc0aac4` holds `docs/build-log/` and
`docs/specs/`. This isolated operator-requested branch is not an admitted queue execution.
The task remains open until the verified branch is integrated and the native completion write is
available. The installed task runtime exposes no submit/gate-run/complete verbs; manual completion
is not used to bypass that gap. Revert the task-owned runner/docs to roll back; no provider config,
trust or service state was changed.
