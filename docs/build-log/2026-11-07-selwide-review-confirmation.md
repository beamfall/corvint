# Selwide review confirmation and closed declaration regression

The repeated independent findings concern V1-5018 / AFP-V0-9012,
AZU-V0-005 and AZU-V0-9921. At this round's starting HEAD `f6d0e9b4a`,
`ce34ea127` already fixes both reported defects; `7fac46521` additionally
preserves hidden tests through workflow execution and FULL fallback. Keep
those implementations and frozen contracts unchanged. This round extends
acceptance evidence, not the declared behavior or wire format.

P1's class is loss of indexed packages between selection and execution:
`./...` is not a complete hidden-package universe. Discovery supplies explicit
hidden package arguments and the Go tool validates them. Execution admits
packages from that observed universe and the validated shard placement;
unindexable claimed source retains `UNINDEXED_SOURCE_PATH` and FULL selection.
The existing `TestHiddenGoSelection_AFPV09012` includes a new `.unmodelled`
source-kind fault to exercise that fail-closed default. The sharded regression
executes both the reported hidden helper and its importer. The FULL regression
requires the hidden test's failure with passing visible consumers, under
qualification failure, planner fallback and identity drift. The CI sidecar
regression covers narrowing, placement disagreement and FULL execution.

P2's class is treating declared output as source before validation. The
acceptance predicate exempts hidden source candidates only when a declaration
matches the closed `corvint-non-source-paths/0` profile: known members, validated
canonical paths, bounded ordered entries and printable nonempty reasons. All
other declaration forms retain source claims. The reported invalid UTF-8 Go
filename is already covered on disk and immutable sources. Extend
`TestHiddenUnrepresentableNames_AZU_V0_005` with an unknown entry member:
`DeclaredNonSourceHolds` must exempt nothing and both Go observers must still
refuse the invalid filename with `ErrWalkUnrepresentable`. This directly tests
the observer's ordering, independently of the graph's declaration refusal.
No new bad-form list or syntax heuristic is introduced.

V1-4941 / AFP-V0-9013 and V1-4710 / AFP-V0-9014 remain unchanged. Rollback
removes only this round's added regression and evidence; it does not revert
the already accepted repair. No golden, existing assertion, diagnostic, exit
code or output shape changes.

Verification uses the existing pre-change capture, the required affected read,
whole package tests through heavy.sh, and final CEM rebind/check. The affected
receipt selects all four relevant packages (426 selected units, 71 unknowns,
scope UNKNOWN); its installed planner reports a stale binary, so it is advice,
not current-tree qualification. Driver tests build the current planner and
selector. Independent review, host qualification and native ticket closeout
remain orchestrator-owned. No Tasks writes, publication or seal. Raw evidence
is retained under `/home/russell/lanes/selwide-evidence/`.

## Verification evidence

| Artifact | SHA-256 | Result |
|---|---|---|
| `review-3-affected.json` | `c2b135eee3de616e9e50451f97054e7bc9927481117aeb2c83d7bdae1164296d` | UNKNOWN retained |
| `review-3-affected-whole.log` | `9c53b91f1a271879c56aaeb660f6094582972ccbdc4533de7b7300ff0658af6a` | ok  	github.com/Beamfall/corvint/internal/liveverify/affected	112.991s |
| `review-3-golang-whole.log` | `50b647b56cb6adcffbee2fec5ff158f4e4b78ed32de09c3c163231bf5d4df46b` | ok  	github.com/Beamfall/corvint/internal/liveverify/affected/golang	9.510s |
| `review-3-forward-whole.log` | `3905c053b4f42c893bd1994f0c7b49677c53cdb6911f408ecef5d58f7ba7eafb` | ok  	github.com/Beamfall/corvint/tools/ci-forward-row	51.930s |
| `review-3-citations-check.log` | `855a4551b199f407e4e7cec705511746022a39aa0f70383fc13aac9acc748bba` | PASS |
| `review-3-driver-whole.log` | `84a82460f9610ebc2b1418137857066d64e031d2feafb73641a35cfddac0f587` | ok  	github.com/Beamfall/corvint/tools/corvint-pr-tests	132.800s |

All four whole-package invocations passed with no Git identity exports.
Citation repinning moved none; line-citations-check passed. Shared heavy
admission and low-disk advisories remain in the logs; no overrides were used.
