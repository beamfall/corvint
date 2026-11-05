# clicontract verifier repair 1

Owner intent: repair only the six package failures and moved citation reported by
host verification of `4d86665ab127358062f6d0788369b29d425d1d7f`, against unchanged
base `d436dff76de1b8e75c427069ae7f80003ae99bc2`. Preserve frozen assertions,
source bounds, protocol bytes, error codes, streams, statuses and golden files.
Stop after checked change evidence; independent review and integration belong
to the orchestrator. No publication, sealing or native ticket writes are authorized.

## Decisions and requirement evidence

- V1-2685 / CAP-V0-7700: restore the twelve modified analyzer entrypoints exactly
  to BASE. Adding the shared CLI helper more than doubled several reachable
  production closures, failing ACP-009; JavaScript also exceeded its direct source
  ceiling, shader introduced forbidden ambient stderr, and structured-data left
  its pinned source digest. Frozen gates are unchanged. Process-level generated
  help for these twelve paths is PARTIAL, blocked by those existing compatibility
  bounds; offline Core trees remain available. Existing generated-help assertions
  are unchanged, and the full conformance package now retains twelve failing
  analyzer-help subcases. This failure is explicit, not evidence of delivery.
- V1-3736 / ACX-V0-7707 and ACP-011: restored analyzer mains preserve their original
  protocol dispatch, frames, statuses and stream ownership. No universal wrapping
  of analyzer errors or new native envelope is claimed.
- V1-3738 / CAP-V0-7702 and PUB-V0-001: Core keeps its literal release constant,
  as required by the existing public-release tuple test. Shared companion release
  metadata remains; the tuple test, rather than an alias in main.go, binds Core
  to VERSION. The explanatory comment preserves downstream citation positions.
- V1-2685 / CAP-V0-7700, AFP-V0-762: the web-flows observer embeds only its own
  generated help snapshot and uses the existing clitree help server. Its help and
  versions retain their shapes and bytes; it no longer imports the global catalog
  through cliapp. The existing generator writes the local copy alongside the
  global snapshot. Full runtime-reader replay now passes without suppressing the
  two Node consumers or changing selection semantics.
  Existing process conformance checks compare its machine help with the global
  snapshot; a standalone process parity check also passes.
- TSP-V0-951: declare the six actual CLI/help/framing/version package reads of
  the toolchain-receipt shell test. The package closure assertion is unchanged.
- DCG-V0-048: repin the single moved DOGFOOD citation using the existing script;
  its content anchor is unchanged. No cited prose or freeze contract is rewritten.

Rollback restores the incoming wrappers and literal alias, removes the observer's
local help asset/helper and generator copy step, and reverts the reader declaration
and citation repin. That rollback also restores the reported failing gates.

## Validation and limits

Raw evidence is retained outside the repository under
`/home/russell/lanes/clicontract-evidence/verifier-repair-1-current/`.
All Go test commands run through script/heavy.sh, one package per command, with
no -run filter and the required private TMPDIR. The affected plan selects every
executed package and remains UNKNOWN; current-tree selection reports 259 selected
and 91 unknown (80 LANGUAGE_FRONTIER, 5 UNOWNED_DIRTY_PATH, 4 NO_SELECTABLE_TEST,
2 UNSELECTED_BINARY_CONSUMER). This is advice, not a coverage-completeness claim.
Pre-change query/impact receipts are OBSERVED. The separate context packet retained
its two omitted candidates and lexical omissions. Other self-development routes
such as mutation, learning, provider execution and live qualification do not apply
to this scoped compatibility repair.

Independent review is NOT_RUN here, per the owner's orchestration boundary.
External interoperability, criterion adequacy and semantic acceptance remain
NOT_OBSERVED. Structural CEM/OCM closure does not establish them. Native ticket
completion remains with the orchestrator. The full selected runner and repository-wide
gate are NOT_RUN; whole-package focused validation follows the owner's explicit policy.

The twelve touched analyzer command packages, web-flows and the generator pass as
whole packages. The six originally failing packages also pass in full. Core was
run in full and retains 46 top-level failures (263.2s wrapper), including unsafe
profile/executable parents and parent-repository discovery: the mandated TMPDIR
is under `/home/russell/lanes`, mode 0775. No path guard or fixture assertion is
weakened. Host safe-scratch execution remains required. Full CLI conformance
retains exactly the twelve analyzer-help failures (66.5s wrapper); web-flows
process help has no failure, and its separate machine-help parity check passes.
The aggregate local outcome remains failed.

The initial full doc gate passed the repaired citation gate but found three stale
Core implementation receipts. Their original digest-only repin script refreshes
the three subjects and ledger (UCV0-004/005/018); schemas and attestations are
unchanged. Final receipt and documentation results are retained in the handback.
`make spec-requirements` reports no generated changes.

| Evidence | SHA-256 |
|---|---|
| internal_analyzercap.log | 4d01e8b13d23d31b25c8d691cacebffc11948d05d4b2674fbd66bb95251d7bd4 |
| internal_analyzerjs.log | 7d4a01ac4aa0dc2602bbed940ac5cb6dd86e9bb87d1d877c08d457f98538f28b |
| internal_analyzershader.log | 290e8fdbd8efe0350fa63db1037d65e885db7d7ec758bfa9d493065d8c2e90fb |
| internal_analyzerstructured.log | 5176c26456abcde750f42168c81b8f0727da0ef0d81824686fe114213613ca44 |
| internal_liveverify_affected.log | ada2afa5d7fde53b66200e8a92017e656051de8410e76ae542ea45491e11f835 |
| conformance_cli-errors-v0.log | 62efca1fd43044b710aeaffb510c0f8e0fa306233fe4b8fa6ff93e3b1c64009e |
| cmd_corvint.log | d352afa31a1349c3cd3ba4651a3a17554289fcf8a7a2fe288e33e257a361b1dc |
| web-flows-parity.log | f8e9d227492fee8034cfc84bcfc373da606b4a5b843fbeddad6e00d8d3500556 |
| affected-current.json | 51975e2346a50c15bd04a05f3f53e030ed46aab5b36243132dc66efa7a905a9c |

A repeat after the mechanical receipt edits exposed a live-dirty-worktree test
limit: `TestSelectionOnTheLiveDirtyWorktree` reports an unresolvable
`PATH_GLOB_READER` witness for internal/tasks/cli on a changed implementation
receipt (`Glob:receipts/*`). The earlier full package passed before receipts
were dirty. Keep that failure visible and repeat the unchanged full package at
the clean committed boundary; do not alter planner semantics or its assertion.
Receipt revision pins are refreshed after the source commit, so their immutable
revision contains the source bytes they name. The handback retains the terminal
clean-bound package result and the receipt/doc-gate results.
