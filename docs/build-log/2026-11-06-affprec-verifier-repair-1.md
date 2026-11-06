# affprec verifier repair 1: help ownership and driver-count conflict

Owner direction: repair only the two supplied affected-package failures, against
base c5379b4cf039fa8dd2df2aa4ee6e5b7b897cf514. Preserve every existing assertion
and the frozen contracts; reuse the existing pre-change capture.

## Ownership repair

V1-3992 / AFP-V0-9002 moved display-only help examples into two embedded text
files. The integrated AZU-V0-002 ownership ratchet requires their classification.
Declare each exact file in `.corvint/runtime-readers.json` under AFP-V0-602,
following the existing embedded catalog precedent. AFP-V0-600 embed edges,
enclosing-package selection, literal and declared readers, and unbounded readers
remain active. Do not classify them as documentation or exclude unbounded reads.
Rollback removes just these two entries and restores the reported ownership gap.

A current-tree binary's path-only plan classifies both files as
DECLARED_RUNTIME_READERS, selects Tasks CLI, and retains
shell:foreign-invocation-unmodelled. Raw plans and test output live outside the
repository at `/home/russell/lanes/affprec-evidence/verifier-repair-1-current/`.

## Precision acceptance conflict

The unchanged TestPrecisionReplay_AFPV09003 reports 285 selections for heavy.sh.
The retained plan contains 213 Make/CI drivers and 72 language units. The merged
base's `languages.All` includes the built-in driver claimant. AZU-V0-001 requires
driver units and their literal invocation edges; Makefile recipes invoke heavy.sh
through their admission wrappers. The old absolute threshold counts these added
driver units as well as the original language units. The count is therefore no
evidence of a newly unbounded Tasks scanner.

V1-4497 / AFP-V0-9003 qualification remains PARTIAL: the external integration of
driver units conflicts with the inherited below-86 all-unit assertion. Removing
driver selections would violate their contract; narrowing the assertion's domain
requires an explicit exception to the owner's no-assertion-weakening rule. Keep
the graph, the threshold, all assertions, and all true-reader controls intact.
V1-4143 and V1-4535 retain their prior measured evidence, but this live replay is
not a passing qualification for either. The other lane tickets are unchanged.

No JSON shape, wire field, error or exit contract, stderr text, committed golden,
core-freeze output, production Go source or test assertion changes. Independent
review, native ticket writes, publication and seal remain with orchestration.
The full affected-package rerun and final CEM outcome are recorded in the handback;
structural dogfood acceptance must not be represented as a package-test pass.
