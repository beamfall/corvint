<!-- SPDX-License-Identifier: Apache-2.0 -->
# Additive CLI discovery and error profile

Owner intent: the 2026-11-05 clicontract repair charter requires V1-2685,
V1-3736 and V1-3738 without changing any frozen native error shape. Base is
`d436dff76de1b8e75c427069ae7f80003ae99bc2`; incoming repair head is
`4a980bde6658c4c301a494be544cf0dc6e051b74`.

## Decision

The frozen cases win. CCF-V1-006 requires a decision/divergence registration
for byte changes to parity-pinned output; additive fields alone would still
break exact native frames and closed Tasks decoding. No amendment, golden
rewrite or test-assertion relaxation is used here. Native analyzer mains and
their import closures remain unchanged. Existing native error serializers
remain authoritative.

`corvint-cli --binary BINARY ARGS...` is an explicitly selected discovery and
short CLI profile. It consumes the existing embedded, code-checked trees and
serves uniform offline help/catalogs and release versions, including the
analyzers and folded shims. Its unknown verbs and failed native short CLI
calls get a coded stderr envelope and exit 2; failed native stdout, stderr and
status are retained in details. Direct invocation retains its original
streams, bytes and exits. Native protocol execution is excluded from the
adapter. Short CLI execution is bounded to 30 seconds and 1 MiB per stream;
over-bound output cannot become successful truncated output.

CAP-V0-9300, ACX-V0-9301 and CAP-V0-9302 record this design fork under the
owner's explicit additive-mode authorization. They complete the previously
withdrawn help/error/version requirements through opt-in discovery, rather
than asserting that isolated native tools acquired a new CLI contract.
Rollback removes the adapter/catalog integration; no native state migration.

The restored analyzers also exposed a generator refusal: authored stdin
protocol usage may coexist with the exact native receipt controls
`--version` and `--descriptor`. The generator now retains those alternatives
and still refuses unknown options without authored usage. No analyzer source
or frozen source-size/digest ceiling changes.

## Evidence and limits

Raw artifacts are outside the repository at
`/home/russell/lanes/clicontract-evidence/repair-2/`.
The incoming whole conformance run failed twelve generated-help analyzer
subcases in 64.761 seconds; its native error table passed. This differs from
the charter's reported eight error cases, so both observations are retained.
Existing generated-help assertions are unchanged; the twelve frozen analyzer
invocations explicitly select the new profile. Additional process tests cover
every profile node and exact native analyzer frames, including specialized
profile identifiers and Swift's native exit 1.

The inherited pre-change capture is reused as directed. No change-start
capture, model-backed reviewer, Tasks mutation, push, PR or seal is performed.
Independent review and native ticket disposition remain orchestrator-owned.

Self-development routes: affected planning and final CEM report/check are used;
the pre-change context route reuses the retained capture. Ranking, learning,
browser/provider execution and external interoperability are inapplicable to
this CLI-only repair. Missing semantic/authority/runtime observations remain
NOT_OBSERVED, and guarding-test receipts remain NOT_PRODUCED unless the
native coordinator produces them. Structural CEM acceptance alone does not
establish semantic adequacy.

The incoming failure log is pinned by SHA-256
`4e8618ccf979531ba231dfe2b0fc4a84d07bb13b5ffb4c94af358c9a34971f29`
(`conformance-before.log`, twelve failures, 64.761 seconds). The first additive
conformance pass took 89.824 seconds; subsequent catalog flag-discovery repairs
require terminal replay rather than reusing that pass as current evidence.

The new not-found process check initially read this journal-absent checkout and
correctly received CTS-V0-821's NOT_OBSERVED result. Its assertions now execute
against the existing native Tasks fixture, under `internal/tasks/cli`, through
the same shared process oracle. No real queue is initialized or written. Keeping
the fixture on the Tasks side also preserves decision 0397's import boundary.
The adapter build's exact runtime source closure is declared in the existing
test-read-scopes format; planner code and safety checks are unchanged.

The drift gate caught a missing `--tree` control and binary-selector values
misclassified as subcommands. Parent-owned controls now inspect original argv
independently of the forwarded native suffix, and protocol classification is
separate from argv dispatch. Its generator test requires every control flag.
Adapter execution lives in `internal/cliprofile`, keeping native help import
closures free of its subprocess machinery. The new generator test's missing
root-directory setup was fixed without changing its assertions. Losing runs
remain retained alongside terminal artifacts and SHA256SUMS in the external
evidence directory; the handback records all terminal commands and outcomes.

## Terminal qualification

Implementation is `2046c94dac533af2e6e4a4b4c9038f74609997c7`. Subsequent
integration changes only read declarations, requirement/test references and
this evidence record. Terminal whole CLI conformance passes in 94.809 seconds:
native error rows, exact native analyzer frames, every profile node, catalogs,
versions, suggestions and message/code checks. SHA-256 of
`conformance-terminal.log` is
`4e1e2c09536f8d0c4a028e120276be2ce633a7809a83fae5628ee275102dd113`.

Whole `internal/cliapp`, `internal/cliprofile`, `internal/clitree`,
`internal/capcatalog`, `tools/cli-discovery-gen`, `tools/capabilities-check`
and `cmd/corvint-web-flows` pass. The generator package includes all-binary
installation, bounded rollback retention and release-line checks; its terminal
log SHA-256 is
`f5c5e2e77a40efaf7c05e2a6b7cd15a077561125b3b9e33199d196df588e33c3`.
The catalog gate observes 54 commands, 57 binaries and 44 MCP tools. Its whole
package log SHA-256 is
`f92bfd73394af4d8c4c63ad6138e13966081a66629bfa5e6e4a27844bdbf8186`.
Capabilities, read scopes and declaration compatibility pass together; log
SHA-256 is `68cc578a63025045fd7909dfc12abc2bf82d0ae4245c51ff4ae36aabcbfd46c1`.
Read declarations conservatively retain the existing native Tasks build closure
and tracked literals as well as the adapter's closure, as required by AFP-V0-023.
No reader, unit or uncertainty is dropped.

Whole Tasks CLI runs for 941.558 seconds and fails two existing tests:
`TestTMV0008_AS07_ReadsLeaveStoreByteIdentical/no_git` refuses archive staging
at `/home/russell/lanes/clicontract.tmp/heavy-run.zJXUyL` as not outside the
repository/state, and `TestCALV0302_CompactedStepsKeepTheirPostState` compares
`observedAt` values of `2026-11-05T15:29:34Z` and `2026-11-05T15:29:35Z`.
Their assertions and production behavior are unchanged. All new profile and
existing typed mutation/template acceptance tests pass in that whole run.
Tasks log SHA-256 is
`47c5aadd7aad6f4628823e10775944b8494850edf55172d3bed804eb274c4e51`.
The local aggregate outcome remains **failed**; host qualification of these
scratch/clock failures remains outstanding, rather than weakening either test.
Criterion delivery is MET for V1-2685 (CAP-V0-9300), V1-3736 (ACX-V0-9301)
and V1-3738 (CAP-V0-9302 and retained TCL-V0-7703). This is not native ticket
completion or an all-tests-passed claim.

Other friction: the shared heavy queue reached 485.8 seconds plus 20 seconds
of host-load admission for a subsecond check. Serial short checks under one
admission retain the same one-package commands. Commits return success while
Git reports the forbidden common path
`/home/russell/projects/corvint/.git/packed-refs.lock`; that path is not changed.
The required TMPDIR and configured `/home/russell/bench/rt/.locks` are retained.
