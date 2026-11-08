# Qualification repair: retained plan bindings and bounded admission

Repair base: `fd95d92413829a09718c323ddbb00c5980d8ae40`.
Requirements: RSP-V0-034, RSP-V0-036, RSP-V0-050, RSP-V0-055.
Status: repairs and regression tests written; runtime execution deferred by the
owner's BUILD-ONLY instruction. No runner is qualified or enabled by this change.

## Verified findings and decisions

Replay head experiments previously carried only the head commit, so the retained
plan could be reused against a different first parent. The charter explicitly
authorizes adding `Experiment.Base`. The harness now passes the pinned merge base
to the driver, and retained replay checks it before using the observation's plan.
Missing pins refuse with `retained-replay-base-unpinned`; malformed or different
pins refuse with `retained-replay-base-mismatch`. Non-replay experiments omit the
new member. Existing CLI exit codes, stderr, command tree, evidence record shapes
and goldens are unchanged. This is a binding within a core-retained experiment,
not authentication of plugin assertions or proof of live collection.

The unchanged negative control now compares its effective selected identities
with the baseline as well as comparing FULL outcomes. A different selection
refuses with `negative-control-plan-differs`. FULL and an explicit inventory-sized
set are equivalent; ordering does not matter. Admission checks the retained
positive budget against the current trusted ceiling, checks nonnegative wall
time against that budget, and remains FULL with a bounded requalification job
on a failed check. An unset ceiling cannot admit an old passing record.

File-only capability now requires the declared `script` runner and a `script:`
test identity, matching the existing `ScriptTraceRecord` adapter boundary, as
well as the three complete empty `script-kind` markers. Other runners retain
native prerequisites. The existing union implementation already consults this
capability, so it now turns unbound markers into incomplete components and
collection readiness retains `collection-components-missing`. No record field
or selector-language special case is added. The existing positive fixture now
declares the script identity; every existing assertion is retained.

Item 1 is WRONG as proposed: `langplugin.SubPlugin.Paths` names the directories
that implement the mutation tool, not corpus edit roots. The original definition
in `internal/langplugin/langplugin.go` explicitly says this; the Go mutation
declaration points to `internal/liveverify/mutate` and separately declares `.go`
suffixes. Requiring corpus edits to fall under the tool's implementation paths
would refuse valid mutations. `TestRSPV0033PinnedDeclarationAndMutationPiece`
already exercises a corpus file outside the fixture provider's implementation
path. No new path authority or wire member is invented in this repair.

The optional quadratic byte-budget finding does not justify a simple change:
`FilePopulation` already performs a constant-time, overflow-safe preflight per
target before cloning. Its conservative quadratic byte estimate bounds the full
Before/After payloads of every enumerated mutant; removing the bound would admit
oversized materializations. `TestRSPV0033MalformedAndCappedPopulation` already
covers a 50,000-byte target refusing. A compact/lazy mutant representation would
change the retained population contract and is outside this repair.

## Acceptance evidence and limits

- RSP-V0-034: `TestRSPV0034ReplayPlanBaseBinding`,
  `TestRSPV0034CommandRefusesUnpinnedReplayPlan`, and
  `TestRSPV0034NegativeControlPlan`.
- RSP-V0-036: `TestRSPV0036QualificationBudgetCeiling`.
- RSP-V0-050/055: `TestRSPV0050SelectionCapabilityRunnerBinding`, alongside the
  unchanged positive assertions in `TestRSPV0050SelectionCapability`.

The tests are written, not executed. The lane handback retains the return codes
of formatting, package vet and the permitted spec/citation checks. Whole-package
runtime tests, independent review, live qualification, frozen evaluations and
CEM binding/checking/sealing remain NOT_RUN/NOT_PRODUCED for the host integration
round. The BUILD-ONLY and shared-CEM instructions supersede the earlier request
to run dogfood-change at start and bind at finish. Spec edits are limited to
Traceability rows; generated indexes and use-case receipts remain host-owned.

Self-development routes: original source and requirement inspection used;
`affected`, execution, mutation experiments and the dogfood binding workflow
deferred under the final check-only instruction. No retrieval, learning,
release, publication, or task-store mutation route applies to this repair.

Rollback: revert this repair commit. Replay inputs lacking a base remain unsafe
to use as evidence of qualification; reverting code does not validate them.
