# 2026-10-08: Completed stage observations bind their actual receipt classes (V1-0466)

## Intent

V1-0466 recorded that completed staging observation compared the descriptor operation directly
with the receipt kind, so an ordinary `MUTATE` stage with a `MUTATION` receipt refused, and that
the `StageLease` comment omitted the `GATE_RESULT` and `MANIFEST` kinds its writers emit.

## Finding on base

The closed mapping already landed with the V1-0449 integration in `0b93284b`
(`stageReceiptKind` in `internal/tasks/snapshot/stage_observation.go`), and the `StageLease`
comment already names all four lease kinds and treats FENCED as a `TRANSITION` refusal. The
mapping matches the writers on base `2a93b5a1`:

- `MUTATE`: `receiptKind` in `internal/tasks/transaction/model.go` emits `MUTATION`, `ARCHIVE`
  or `RESTORE`.
- `LEASE`: `lease_claim.go` `ADMIT`; `lease.go`, `lease_gate.go`, `supervisor.go`, pools and
  programs `TRANSITION`; `lease_gate.go` `GATE_RESULT` and `MANIFEST`; `recordFenced` in
  `lease.go` a `TRANSITION` with `REVISION_CONFLICT` and `FENCED`.
- `ESCALATION`: `lease_escalation.go` `TRANSITION`.
- `RELEASE`: release writes `RELEASE`; settled release reconciliation `RECONCILE` (`model.go`).
- `KEEP_JOURNAL`, `ADOPT_FILE`: `RECONCILE`; the remaining direct operations use their own name.

What remained open was acceptance criterion 2: actual published evidence did not cover
`ARCHIVE`, `RESTORE`, `RELEASE` or `RECONCILE`, and no actual stage was relabelled to prove that
cross-class pairs refuse.

## Change

- CAL-V0-027 amended (proposed) to state the closed map explicitly, with FENCED an outcome code
  rather than a receipt kind.
- `TestCALV0027_ActualCompletedStages` (`internal/tasks/store`) adds actual `ARCHIVE`, `RESTORE`,
  `RELEASE` and settled release `RECONCILE` stages. Every actual stage is then relabelled as each
  other stage class plus `UNKNOWN`; each relabel must refuse without changing the store, and where
  the closed layout admits the relabel (`ARCHIVE`/`RESTORE` as `ADOPT_FILE`, `IMPORT_APPLY`,
  `LEASE`; FENCED `TRANSITION` as `UNPAUSE`, `IMPORT_APPLY`) the refusal must be the receipt-kind
  binding. The test operator then removes the descriptor; observation still has no cleanup role.
- `TestCALV0027_CompletedStageReceiptKinds` (`internal/tasks/snapshot`) adds the direct,
  reconciliation and escalation aliases and further cross-class pairs. `FENCED` and unknown
  labels are refused by the receipt decoder before binding, and the mapping abstains on them.

No production code changed.

## Evidence

- Passing after: `go test -run TestCALV0027_ActualCompletedStages ./internal/tasks/store/` and
  `go test ./internal/tasks/snapshot/` pass.
- Failing before (mutants, reverted): restoring the pre-`0b93284b` comparison (`kind == operation`,
  reconciliation aliases only) fails every actual case, including the new `RELEASE-RECONCILE`
  case, with `JOURNAL_FORKED: staging: completed operation or timestamp differs`; widening
  `LEASE` to admit `ARCHIVE` fails `cross-class LEASE/ARCHIVE`.

## Non-goals and failure modes

- No new receipt kind, stage class or layout change; no recovery or cleanup authority.
- Relabels the layout refuses never reach the kind binding; that is a layout refusal, recorded
  as such, not kind-binding coverage.
- An unlisted future writer kind refuses as a fork until its class is added to the closed map.

## Rollback

Revert this commit; production behaviour is unchanged, so only the added tests and the spec
amendment are removed.

## NOT_RUN

Full `go test ./...` and `make gate` (lane policy); `internal/tasks/cli` and `internal/tasks/journal`
focused packages beyond the cited tests; live native release qualification.
