<!-- SPDX-License-Identifier: Apache-2.0 -->
# AZU selection soundness repair

The independent review of `16c97eaa4e35ef6b630917391c51ad23d63ab662`
found three unsafe omissions. This repair implements the existing AZU-V0-001,
AZU-V0-004 and AZU-V0-8901 safety intent without changing declaration shapes,
error codes, diagnostics, committed goldens or the default driver registration.

Root shell callees no longer bypass constructed-caller checking. A root callee
has no directory literal that can exclude a computed basename, so joins,
quoted concatenations and formatted strings conservatively count as potential
callers. Missing declarations retain both foreign-callers-invalid and
foreign-invocation uncertainty while preserving previously verified callers.
`TestRootConstructedForeignCallers_AZU_V0_8901` reproduced the missing uncertainty
before the fix for a Go path join and two concatenations, and passes afterward.

Go package commands except `run` default to the current package when no package
operand appears. Track operands independently of flags and application arguments,
then resolve that package after `cd` or `go -C`. Unknown directories retain the
existing all-owner path import. `TestDriverImplicitGoPackage_AZU_V0_001` failed
before the fix for seven command forms, losing the make target, its dependent
target and both dependent jobs. It passes afterward. The directory scanner test
also covers all supported default-package commands, flags and unknown directories.

Design decision: reject flow mappings instead of expanding the YAML subset in
this repair. This includes mappings inside flow sequences. Merely returning the
parser error proved insufficient: the fallback unit's frontier did not bear
when its missing edge prevented participation. Reach unparsable driver fallback
units for dirty paths not covered by the existing project no-unbounded-reader
proof (AFP-V0-772), keeping UNKNOWN rather than certifying an
incomplete plan. `TestDriverFlowMappingKeepsUncertainty_AZU_V0_001` reproduced
three omissions before the fix and passes after both changes. Flow mapping
execution-edge inference remains disabled; its safe fallback is retained.

Rollback is to disable the experimental driver and retain the uncertain/full
plan; do not restore the unsafe omission rules. Aggregate AZU-V0-009 historical
replay/full-suite/mutation qualification and AZU-V0-002 promotion retain their
existing external blockers. This round changes neither replay admission nor
ownership declarations. The host orchestrator owns independent review and broad
qualification. Focused and whole-package results are retained in the lane handback.

Raw logs are outside the repository at
`/home/russell/lanes/azu-new-evidence/`. Focused runs returned 1 before the fixes
and 0 after them; these digests bind those observations:

| Log | SHA-256 |
|---|---|
| `review-round-before-shell.log` | `cefd18de8262354deb4aa879ad9e0dcb716f59cdba6d2e4a551b04473b33d130` |
| `review-round-before-driver.log` | `182c90fc57e3842074ec83360805a5d19f3bd10de7f6876ea547678987b24cb7` |
| `review-round-after-shell.log` | `f57666ddfcc00a9bc3d50eb2fa01b88e6d9b1d7463d79c941dfe0c1b16282534` |
| `review-round-after-driver-final.log` | `99c100ce4bf2ffaaee36970e7ef0122c854835e625b1322035a12b3984f04d91` |

The first whole affected-package run returned 1 (208.644s): the broader fallback
broke `TestSealCommitPlanIsBounded_V13898` and
`TestSealOnlyCEMDeltaStaysBounded_V13571`. Reuse `unboundedDirty` to honor the
existing project-owned AFP-V0-772 proof for data with no unbounded readers.
Computed invocation handling is unchanged; literal readers of excluded data
still reach the unparsable unit and retain uncertainty. The new
`TestDriverUnparsableReaderExclusions_AZU_V0_001` covers an absent proof, an
accepted proof, a mixed code/data change and a literal reader. Existing test
assertions remain intact. The handback retains the repeated whole-package result.

Initial whole-package failure log SHA-256:
`dce03e22de712112776a99cc1931ea6a5de67ca82e930fede2d846bed72a90a2`.
