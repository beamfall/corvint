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
