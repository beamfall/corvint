## 2026-10-05 V1-0787: stage-scoped execution prerequisites

Human-owned intent: owner request issue 585 and native ticket V1-0787, with owner decision D4. D4
makes the refusal a new closed code `PREREQUISITE_UNSATISFIED`. It also requires byte-sorted
canonical ordering and validation consistent with `dependencies`.

A dependency gates the whole ticket and takes part in cycles and completion. A prerequisite gates
claims and plans only for the stages it lists.

Requirement: `CAL-V0-099` in `docs/specs/corvint-tasks-agent-leases-v0.md`, in the "V1-0787
stage-scoped execution prerequisites amendment" section inside `## Requirements`, with TCP-00
amendment A19. CAL-V0-073..098 belong to other lanes.

### Change

- Record. The optional key `executionPrerequisites` has entries
  `[{gateId, obligation, stages, ticketId}]`.
  - It is a non-empty canonical-byte-sorted set of at most 64 entries. `stages` is a non-empty
    sorted set over `implement`, `review` and `integrate`. `gateId` is non-null iff `GATE_PASSED`.
  - It is same-queue, and never self-referencing. Each `(ticketId, obligation, gateId)` appears once.
  - The key is omitted when absent, so legacy bytes are unchanged.
  - It is not acceptance-relevant, and is excluded from cycles, completion and `requiredGates`.
  - It is listed in `wire.TicketRecordOptionalKeys`, which the native codec shares with Core.
- View (`internal/tasks/ticket/view.go`). `ticket.Context.Stage` scopes evaluation. An empty stage
  applies every entry (fail closed).
  - An unmet `COMPLETED` obligation or a missing ticket is a `PREREQUISITE_UNSATISFIED` blocker.
  - `GATE_PASSED` uses the gate oracle. `NOT_OBSERVED` is an unknown with the same code, and is
    always the result today.
  - Each detail names the prerequisite ticket and its stages. The next action is `wait-dependency`.
    The view echoes the set.
- Claim and plan. `leaseContext.eligibility` (claim and claim-next) and `claimBlockerObservations`
  (plan and recorded claimability) pass `--stage`. Show and blockers stay stageless.
- Writers.
  - REFINE sets the key, and clears it with `null`. An empty array is refused.
  - REFINE and ADOPT_FILE check that each referenced ticket exists (`DEPENDENCY_MISSING`) and that
    each gate is declared (`GATE_UNKNOWN`). ADOPT_FILE composes the key through REFINE.
  - Importer export items may carry the key, checked the same way. A re-import that omits it keeps
    the record's set.
  - CREATE does not accept the key.
  - `CanonicalPayload` now makes two set-sorting passes, so the outer set is ordered by the inner
    sorted stages.
- Core (`internal/taskman`). `decode.go` validates the key's shape and the native edge rules. The stageless planner reports
  `GATE_UNKNOWN` for a `GATE_PASSED` prerequisite, and `PREREQUISITE_UNSATISFIED` for a missing or
  uncompleted one.
- Wire. `PREREQUISITE_UNSATISFIED` is added to `wire.Codes`, which now holds 73 codes.
- The shared fixture `issue502-optional-keys-record.json` was regenerated with the key. The native
  and Core copies are identical.

### Evidence

Focused tests on the uncommitted tree all passed: `ticket`, `taskman`, `importer`, `mutation`,
`transaction`, `wire`, `intent`, `cli`, `lrfrepo` and `specindex`. `store` passed under
`-run 'CALV0007|CALV0008|CALV0017|Issue502|TMV0008|Blockers|Plan'`.

Other checks also passed: `gofmt`, `go vet` on the touched packages, `go build ./...`, and the
`windows/amd64` cross-build. With the docs staged, these `make -s` targets
passed: `spec-requirements-check`, `requirement-definitions-check`, `traceability-tests-check`,
`decision-numbers-check`, `line-citations-check`, `error-code-ownership-check`,
`unbounded-readers-check`, `use-case-receipts-check` (18 receipts current) and
`use-case-receipts-test`.

New tests are listed in the CAL-V0-099 evidence table of the spec. The `lrfrepo` test
`TestAgentLeasesSpecEnumeratesExecutionPrerequisites` checks that OCM enumerates CAL-V0-099.

The following were `NOT_RUN`:
- `make gate`;
- the full `store` package;
- live store qualification;
- independent review;
- dogfood bind and seal.

### Review repair

An independent Codex review of `4e5c53bf..7ce4ab24` returned `CHANGES_REQUIRED` with four P2
findings. All four are repaired in a follow-up commit, each with a regression test that failed
before the repair:

- Core accepted prerequisite sets that native Tasks refuses: duplicate `(ticketId, obligation,
  gateId)` edges with different stages, self and other-queue prerequisites, and ticket IDs that
  only had the `ticket:` prefix. Core now applies `ParseTicketID` and the native edge rules. The
  shared fixture `cal-v0-099-prerequisite-refusals.json` holds nine cases that both readers must
  refuse (`TestCALV0099_SharedRefusals`, `TestCALV0099_CoreSharedRefusals`). The native and Core
  copies are byte-identical.
- The documented rollback was wrong. Clearing the key keeps earlier ticket afterimages in the
  journal, which audit decodes strictly, so an older binary still refuses the store. The rollback
  now requires a compatible reader or a verified restore from a pre-change backup. It keeps a byte
  search that tells an operator whether the key was ever written.
- `claim-next` dropped the prerequisite identity when nothing was selectable. The plan entry now
  keeps the prerequisite details, and the refusal names each prerequisite and its stages
  (`TestCALV0099_ClaimNextRefusalNamesPrerequisite`, through `planClaimNext`).
- A ticket whose only issue was an unknown `GATE_PASSED` prerequisite reported `nextAction: admit`.
  Prerequisite unknowns now yield `wait-dependency` (`TestCALV0099_StageScopedView`).

### Open owner questions

- A stageless claim or plan applies every prerequisite (fail closed), rather than none.
- Without a production gate oracle, a `GATE_PASSED` prerequisite keeps its stages refused as
  `NOT_OBSERVED`. This matches dependencies.
- Self-reference is `MALFORMED`. Dependencies use `CYCLE`, but prerequisites have no cycle notion.
- CREATE does not accept the key, so it is set by refine.
- The 73-code count may need renumbering against other lanes that also add codes.
