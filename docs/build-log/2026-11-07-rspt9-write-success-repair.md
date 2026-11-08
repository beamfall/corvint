# Write-success dependencies and registry declaration read scope

SPDX-License-Identifier: Apache-2.0

## Decision and scope

Self-review found that successful write-only opens and standalone truncation could
leave a present trace with no selection dependency on the existing path. Under
RSP-V0-007, RSP-V0-021 and RSP-V0-022, success observes the path even without a
subsequent data read. Retain that dependency by default.

The independence allowlist is unchanged and closed: only supported ordinary open
flags with successful O_CREAT|O_EXCL establish creation; prior tree-owned creation
(including its existing rename transfer) can suppress a later dependency. O_PATH
never establishes creation; unsupported flags mark components incomplete. Access
mode is no longer an exemption. Standalone truncate uses the same provenance check.
No wire shapes, failure codes, diagnostics, goldens or existing assertions changed.
Rollback is reverting this repair; doing so restores the reviewed soundness gap.

The registry's RSP-V0-050 declaration-schema regression reads the tracked
.github/cishards/test-commands.json. Its declared external read scope now names that
exact file, so the affected graph can resolve the literal read.

## Acceptance evidence

- TestRSPV0007WriteSuccessDependencies covers write-only open, append,
  open-with-truncate and standalone truncate with no subsequent read. Each retains
  the dependency and changes outcome after deletion. Prior exclusive creation
  remains independent. An unknown open flag leaves the component incomplete and
  retains the existing-path dependency, proving the fail-closed default.
- Whole internal/proctrace package: PASS (4.115s).
- Whole internal/testrunner/registry package, including
  TestRSPV0050ExistingDeclarationSchema: PASS (0.284s).
- Both packages were selected by affected against the requested base
  8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734. Tests ran through script/heavy.sh.
- Citation repin found no moved unchanged citation; line-citations-check passed.
- The external lane handback retains the declaration-only selection result and
  final binding/check outcome. Independent review belongs to the orchestrator;
  qualification and exhaustive gates are NOT_RUN in this repair scope.
