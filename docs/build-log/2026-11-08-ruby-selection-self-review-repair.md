# Ruby selection self-review repair

Owner request: repair the four rspt11rb self-review findings only (V1-5113).
Requirements: RSP-V0-002, RSP-V0-003, RSP-V0-005, RSP-V0-012(g), RSP-V0-048.

Directory observations include entry types. The selector now retains a parent listing
for every mode or link transition, through either lexical or resolved identity. It does
not guess which particular transition is invisible. The existing closed allowlist is
fully decoded normalized relative paths in known repositories and modify/add/delete
changes (rename expands to delete/add); unknown forms retain `ancestry-unproven`.
`TestRSPV0012EntryTypeListing` covers type transitions and an unknown change kind.

The shipped Minitest reporter (also Rails-test) preserves Minitest::Skip messages;
Test::Unit preserves omission/pending messages in the existing Message member. No new
report fields, terminal states or reconciliation policy are introduced. The live
`TestRSPV0003RubyReporterSkipReasonsLive` exercises their actual output through inventory
and terminal reconciliation. Configuration identity excludes the fresh ReportDir while
retaining the request's runner/configuration pins; fresh staging works under the same
inventory key and changed configuration pins are still refused.

`rubylinecov.ReadSelectionArtifacts` is the artifact-only boundary: it reads the complete
core-captured suite directory and optional isolated directories, never executes Ruby or
writes state. It shares Collect's isolation comparison rather than accepting a caller's
isolation assertion. Missing/divergent isolation stays incomplete. The closed artifact
vocabulary is regular suite.jsonl and child-*.jsonl files, existing collector event kinds,
and passed/failed/pending terminal states. Unknown artifacts/events/states fail closed.
Bounds are 4096 isolated directories, 4096 files per directory, 16 MiB per file and
128 MiB total bytes. Core capture provenance and immutable source pinning remain required.
The decoder regressions cover missing/divergent/duplicate isolation and unknown forms;
the registry live fixture follows Command → artifacts → exported decoder → Evidence.

No activation, qualification, frozen golden, global absent-piece assertion or wire change.
The prior global activation blocker remains. Rollback is this repair's implementation
commit; without the selector repair, retain full selection for mode/link transitions.
Independent review and repository-wide gates remain assigned to the host orchestrator.

Verification: all existing tests of the four touched packages ran without a name
filter, through script/heavy.sh, selected by affected against the requested base
8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734. Package results: registry PASS (1.593s),
rubylinecov PASS (4.047s), dynamic PASS (0.704s), runnerselection PASS (0.015s).
The lane privately staged Ubuntu Ruby 3.2.3, Minitest 5.22.2 and Test::Unit 3.6.2;
Rails-test exercises its Minitest execution profile, not a Rails application corpus.
RSpec live qualification and full Rails qualification remain NOT_RUN. No host install
or lock-directory override was used. The affected binary reported self-binary-stale
and scope UNKNOWN (115 unknowns); its four touched Go packages were selected.

The first live skip test also tried discovery and observed TypeError from Minitest's
random-order runnable_methods with an uninitialized seed. This is outside the four
repair findings; the reporter regression uses the fixture's declared inventory and
actual reporter messages. Discovery is not claimed repaired. Native ticket writes
and independent reviewer invocation are forbidden in this lane; the host receives
this finding and owns those remaining actions.
