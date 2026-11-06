# Unresolved intermediate join traversal — independent review repair

The blocking review finding for V1-4687 / AZU-V0-004 / AZU-V0-8901 is
reproduced by `filepath.Join("scratch", up, "run", name+".sh")`, where
`up == ".."` and `name == "run"`. The normalized join matcher stopped at
`up` and treated the unrelated literal prefix as exclusion evidence. The
integrated graph then selected only the declared caller, excluded the
undeclared caller's tests and returned BOUNDED with no unknowns.

Decision: after consuming a literal prefix, an unresolved intermediate join
operand cannot be bounded by that prefix. Inspect operand boundaries within
the current call, respecting
quoted strings and nested expressions. Such an operand can traverse out of
any prefix, so conservatively retain foreign-invocation uncertainty. The
existing terminal-basename and fully literal join bounds remain unchanged.
No declaration format, diagnostic, error/exit code, golden or existing test
assertion changes. This repairs caller completeness; it does not infer a new
edge from the unresolved expression or qualify aggregate historical safety.
Rollback removes the intermediate-operand check and its helper, restoring the
reported unsafe certification; the regression would fail again. The scope is
the reviewed literal-prefix case; this does not promote support for every
join spelling or a wholly unresolved initial operand.

The first conservative candidate also widened joins whose first operand was
unresolved. Whole affected-package testing failed four unchanged repository
completeness/BOUNDED assertions (166.390s); the 24,851,611-byte failure log
stays outside the repository, SHA-256
`f1b9f5300b0efef61b71e84667f7c029ca9d2ab364e118307a58bcb83bedd370`.
Restricting this repair to a consumed literal prefix still exposed 19
previously unlisted sources, observed by the existing repository completeness
test (663 missing mention pairs across 35 scripts). Retain these conservative
matches and extend the existing project-owned declaration's `mentions` and
reasons after source inspection. Do not suppress the matches, change caller
edges, or rewrite a frozen assertion. The first declaration expansion exceeded the unchanged 64 KiB input bound
(87,626 bytes); a second whole affected-package run failed the same four
assertions (80.484s). No bound or assertion was changed.

Add a qualified exclusion proof: a fixed final suffix incompatible with every
supported shell suffix cannot name a script after an earlier traversal.
Partial suffixes, dynamic tails, cleaned dot/ancestor components, interpolation
markers and Windows case/trailing-dot variants retain uncertainty.
`TestTraversalJoinSuffixBounds_AZU_V0_8901` covers these boundaries independently
of the callee basename. A temporary diagnostic test replayed only the 663 added
pairs through the matcher, retaining 278 pairs across eight source files;
the diagnostic test was removed before final tests. The final declaration
keeps its schema, script keys and caller lists. Mention arrays use compact
formatting to stay within the unchanged byte bound: 63,583 bytes, at most 68
names per entry, longest reason 1013 bytes. No control limit was loosened.

Reviewed intermediate-join uses are:

- Fixture/object/data paths or generated test source:
  `cmd/corvint/work_production_fixture_test.go`,
  `conformance/host-lifecycle-v1/main_test.go`,
  `conformance/release-artifact-v0/archive_gate_test.go`,
  `internal/appmap/fixture_seed_test.go`,
  `internal/cem/verify/structural_test.go`,
  `internal/extevidence/lsp_test.go`, `internal/extevidence/selection_test.go`,
  `internal/testrunner/native/mtp_test.go`, `internal/witness/evidence_test.go`,
  and `tools/test-read-scopes-check/temp_root_test.go`.
- Read-only cache/proc records: `internal/depsource/depsource.go`,
  `internal/groupreap/argv_linux.go`, `internal/groupreap/proctable_linux.go`,
  `internal/tasks/store/lease_gate_load_trace_test.go`, and
  `internal/tasks/store/walk_cost_split_test.go`.
- Installation-path comparison: `internal/releasecandidate/version_test.go`.
- Text serialization: `script/check-ci-least-privilege.sh`,
  `script/gen-spec-requirements.sh`, and `tools/cem-citation-baseline/main.go`.

These reviewed constructions do not execute the declared repository scripts.
Only eight files still need new mentions after the suffix exclusion proof:
`cmd/corvint/work_production_fixture_test.go`,
`internal/appmap/fixture_seed_test.go`, `internal/extevidence/selection_test.go`,
`internal/releasecandidate/version_test.go`, `script/check-ci-least-privilege.sh`,
`script/gen-spec-requirements.sh`, `tools/cem-citation-baseline/main.go`, and
`tools/test-read-scopes-check/temp_root_test.go`.
Existing caller classifications stay intact. Retained-pairs evidence SHA-256:
`d30f9478b37980097e881735f6d124c858f3d36deb3ac323a61c64b4e5f69324`.

`TestTraversalJoinRetainsUnknown_AZU_V0_8901` is a committed integration
regression with a literal caller, an undeclared Go caller, and tests in both
packages. It exercises the reported `up`, a parenthesized operand and a nested
call. All three cases fail before the repair with the reported BOUNDED plan.
After the repair they retain UNKNOWN and the foreign-invocation frontier while
preserving the previously verified caller selection. Listing the traversal
caller restores completeness. The existing unrelated-directory assertions
also pass without modification.

Observed focused evidence, through `script/heavy.sh`, one package per command:

- Complete `internal/liveverify/affected/shell`: PASS, 0.850s. Log SHA-256:
  `58c37b5bdf337393b3f02b08c9f4d56f2ed6e38afc066e7b460835fbd51f5420`.
- Complete `tools/affected-zero-unknown-replay`: PASS, 0.430s. The previously
  accepted inter-hunk-context repair remains unchanged. Log SHA-256:
  `ff4d6c7e63bee6dc6092d0aa4974ad3448124232198601606e9868d8b451e029`.
- Before-fix integrated regression: FAIL as expected in all three cases.
  Log SHA-256:
  `6a3a9ab0fc7f0865b113eb1dd6ecc0a942897bd9484233bb744a0f7b0ccbca85`.

The required affected plan at base
`8fc0cf37642e7012f4c26671c9ed97a8706d6e09` selects the shell, affected
integration and replay packages: UNKNOWN, 184 selected units, 86 unknowns.
The installed binary's stale-source degradation stays visible. Applicable
self-development routes are affected selection and final CEM binding/report
inspection. Reuse the existing pre-change capture; no new capture is made.

Raw evidence, integration and documentation check results, final CEM binding
inputs and the review handback remain under
`/home/russell/lanes/azu-new-evidence/independent-review-traversal/` and
`/home/russell/lanes/azu-new.handback.md`. Independent review and the broad
host verifier remain orchestrator-owned. Publication, Tasks writes, broad
selection/gates and sealing are excluded by the repair instructions. Native
ticket completion and aggregate historical qualification remain unclaimed.
