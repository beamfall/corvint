# Owner-only gate waivers (V1-5096)

The 2026-11-07 CI audit found that `ci-control-plane` and `greptile-gate` treated any
user-created `success` status as an owner waiver, so any repository writer could bypass two
required checks. The owner chose owner-only approval.

Decision: a waiver is now a separate status context (`ci-control-plane-waiver`,
`greptile-gate-waiver`) on the exact head, counted only when its creator is in `WAIVER_APPROVERS`
(step env in each default-branch workflow, currently `Russe11`, kept equal by a test). The newest
approver waiver decides; the workflow then posts the gate success itself. Other writers' statuses,
including hand-posted gate successes, are ignored by the workflows.

Limits: the ruleset still accepts the gate contexts from any writer until the owner binds them to
GitHub Actions; anything holding an approver's token can waive. Evidence: AFP-V0-171 and
GRG-V0-001 traceability rows.
