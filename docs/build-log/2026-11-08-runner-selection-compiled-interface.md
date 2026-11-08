# Runner selection compiled-in interface (V1-5105)

The accepted contract is decision 0592 and `runner-selection-plugins-v0.md`,
RSP-V0-001 through RSP-V0-004 and the compiled-in portion of RSP-V0-025.
The lane base is `f42df32d12bae40c3bfcc6bb8a48442966e4a71d`, containing the accepted
spec. The separately requested affected-plan base is
`8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734`.

## Implementation boundary

The three selection pieces are declared for each runner. Every current language
manifest and registry runner reports each piece absent with a runner-specific
reason. Existing Build/Parse dispatch, execution observations, command output,
and selection behavior remain on their existing path. No runner is promoted.

The compiled-in SPI describes commands and interprets core-captured inventory,
reports and evidence artifacts. Its command value carries Superset; its inventory
key carries checkout, runner, configuration digest, target OS and architecture.
The shared inventory and reconciliation values live in `internal/testrunner` so
both the language manifest and registry can use them without a dependency cycle.
These are in-process types, not amendments to existing JSON envelopes.

NativeEvidence contains only opaque executed, regions and loaded components;
this slice does not decode, admit or persist evidence records. Their record
schema and store admission remain owned by the evidence slice. No current
runner produces such records through this interface.

Core-provided fresh exclusions subtract from the head inventory, and mandates
add back required identities. An absent or invalid inventory produces FULL with
a reason. A reported native superset still owes every planned identity a terminal
result. Missing results trigger at most one FULL retry when allowed. Original
results are retained, a reported failure cannot turn into success on retry, and
divergence keeps the session FULL until the core supplies new qualification.
The existing production path does not use these helpers until a runner is enabled.

## Review finding retained

Self-review found that sharing the planned-test and returned-result slices with
a plugin could let it change what reconciliation checks or overwrite an earlier
attempt. The core now copies those slices. `TestRSPV0003RetainsCorePlanAndResults`
falsifies both cases; `TestRSPV0003RetryPreservesFailure` retains a native failure
when a missing-result retry passes. This is local review, not independent review.

## Self-use evidence and limits

The pre-change recorder retained context, query and impact receipts against the
accepted-spec base. It also reported a private self-observation refusal:
`observation-prohibited-content` (5 of 7 rows not recorded). Those missing rows
remain NOT_PRODUCED; the refusal was not bypassed.

The requested affected-plan invocation selected all three touched packages:
`internal/langplugin`, `internal/testrunner`, and `internal/testrunner/registry`.
It reported scope UNKNOWN, 415 selected units, 516 excluded units and 71
LANGUAGE_FRONTIER unknowns. These are selection observations, not a coverage claim.
The lane runs whole touched packages through `script/heavy.sh`, without a -run
filter, and retains raw command logs outside the repository. The handback records
final command results and digests.

Whole-repository tests, doc-gates, selection-baseline-check, real-runner
qualification, per-plugin recursive test sweeps and model-backed independent
review are NOT_RUN in this lane under the owner's scoped verification instruction.
The orchestrator owns independent review and wider verification. Darwin execution
is NOT_OBSERVED on this Linux host. No task-store writes, publication or seal are
part of this handback.

Rollback is removal of this inactive SPI and its declarations; existing execution
and selection remain available throughout, and there is no stored-data migration.
