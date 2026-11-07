# Provider authored help repair — 2026-11-05

Verifier repair 1, incoming head `c3b1f7898c47fa21be8cae2f78c34b6821b9ba67`,
base `cdeed7ee6396a8d6bf6225eb5b2b4cb4d601b419`. Owner scope is only the
reported `cmd/corvint-js-test-provider` package failure. Native ticket writes,
publication, sealing and independent review remain orchestrator-owned.

## Decision and requirement evidence

V1-2685 / CAP-V0-9300 / CAP-V0-973: shared exact-node help interception returned before the
provider's authored handler could append its flag-set detail. The unchanged
`TestSubcommandHelpPrintsUsage` fails for `negate -h`: the process exits 0 with
empty stderr, but stdout omits `Usage of negate:`. Add the provider to the existing
authored-test-help delegation branch. Its own command-tree handler still renders
the synopsis and appends its existing flag detail. Shared catalog and version
handling remain available. No new option, requirement, spec behavior, golden,
assertion, wire shape, code, stderr or exit contract changes.

`TestNativeProviderHelpDoesNotWrite` checks that delegation leaves both streams
untouched for negate and external-origins with -h, --help and -help. Existing
whole-provider tests exercise the actual process and authored flag help.
V1-3736 / ACX-V0-9301 compatibility remains covered by process conformance;
V1-3738 / CAP-V0-9302 requires no further implementation in this repair.

## Validation and limits

`corvint affected --base cdeed7ee6396a8d6bf6225eb5b2b4cb4d601b419` selects all
three checked packages, with UNKNOWN scope: 247 selected and 92 unknown. The
installed reader reports self-binary-stale; no coverage-completeness claim.
Go toolchain is go1.27.3. All Go tests run through script/heavy.sh, one whole
package per command without -run, using the configured shared semaphore and
TMPDIR=/home/russell/lanes/clicontract.tmp (0700). Commit identity is absent
from tests and scoped only to commit/bind commands.

Whole-provider regression reproduced before repair (1.675s), then PASS (1.715s).
Whole internal/cliapp PASS (0.002s); whole conformance/cli-errors-v0 PASS (79.453s).
Citation repin found no unchanged content that moved; make line-citations-check
PASS (180 existing basename citations skipped). Logs are retained under
`/home/russell/lanes/clicontract-evidence/verifier-repair-1/` and in the handback.
No raw logs are committed.

Self-development routes used: affected selection and the existing pre-change
capture's final CEM rebind/check. No pre-change recapture. Optional services,
learning, mutation and external provider qualification do not apply to this
offline help repair. Independent review is NOT_RUN by owner instruction;
the orchestrator supplies it. Broad selection, doc-gates, selection-baseline,
repository-wide tests/vet and Windows vet are NOT_RUN under this round's scope.
External interoperability, Darwin qualification and token/cost savings remain
NOT_OBSERVED. Final CEM/OCM omissions remain explicit in the handback.

## Evidence digests

| Artifact | SHA-256 |
|---|---|
| provider-before.log | `9c9f4a2ff779ccd24ef93810bf95208af5c5740902f53c404a561325246405bf` |
| provider-after.log | `60563d0cf2d798b0d6196ae94160fbf03a83f80479c8cb1f0c1968c5777351da` |
| cliapp-after.log | `37ebf0bca0732d0a7ef569d56d692a7d6732ed1f2661267b187e342def911876` |

## Rollback

Remove the provider from the shared authored-test-help branch and remove the new
delegation regression. This restores the host-reported failure; existing tests
and frozen contracts stay unchanged in either direction.
