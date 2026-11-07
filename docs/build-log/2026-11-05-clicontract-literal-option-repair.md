# Literal option values in adapter help — 2026-11-05

Incoming head: `0e7c0b5f096368403668a7aa719a4152b755cad6`.
Base: `cdeed7ee6396a8d6bf6225eb5b2b4cb4d601b419`.
The latest independent review marks the recursive Tasks scopes, native Pulse
preamble refusal and catalog generation ordering fixed. Its remaining P2 is
the adapter's failure to consume authored literal option values.

## Decision and requirements

V1-2685 / CAP-V0-9300 already requires consuming valued options separately from
command words. Extend the existing synopsis parser to recognize a literal in
the value slot before a group boundary, another option or the synopsis end.
Preserve bracket and parenthesis boundaries so `[--json] COMMAND` does not
declare a value for a boolean flag. Retain placeholder and alternative-value
recognition. Trailing prose such as `--reset-slot-weights restores the default
order` does not declare an option value. This is a repair of the accepted
contract, with no new requirement, wire shape, field, error code, exit code,
native stderr, golden or core-freeze change.

`TestCLIProfile9300LiteralHelpValues` exercises `cem prepare --non-regular
unknown`, `--spec cem/1.0`, `--path-policy base`, and `context
--instruction-profile default`, including an option before the command. It also
checks bracketed boolean `--replace` and the two boolean eval flags mentioned
in help prose. The whole package fails before the fix on four literal-valued
cases with `unknown-verb`, then passes after the fix (0.230s). Existing tests
continue to cover valued/boolean preambles, inline values, consumed help tokens
and the `--` delimiter. No assertion was weakened or deleted.

The spec's existing test list now names this regression. No behavior outside
offline adapter help parsing is changed. V1-3736 / ACX-V0-9301 and V1-3738 /
CAP-V0-9302 retain their existing implementations and frozen native controls.

## Focused evidence and limits

The actual `corvint affected --base` receipt selects all checked Go packages,
with scope UNKNOWN: 247 selected and 92 unknown. The installed reader reports
`self-binary-stale`; no completeness claim follows from that receipt.
All Go tests use `script/heavy.sh`, one package per command. Whole touched
package tests include existing tests without a `-run` filter. Go is go1.27.3;
TMPDIR is `/home/russell/lanes/clicontract.tmp` (0700). Commit identity is scoped
only to commit/bind commands and is absent from tests and focused gates.

`make capability-catalog` runs after the existing ordering fix and produces
byte-identical committed snapshots and documentation. Whole
`tools/capabilities-check` passes (9.183s); `make -s capabilities-check` reports
54 commands, 57 binaries and 44 MCP tools. The existing shell regeneration
regression passes, including first-generation inclusion of a new binary.
Whole `tools/test-read-scopes-check` passes (2.885s), retaining recursive
production-source coverage and the unbounded-reader floor. Whole
`internal/cliapp` passes (0.002s). `make -s spec-requirements-check
line-citations-check` passes, including its existing specindex package and
lrfrepo obligation checks. Staged citation repin finds no moved unchanged
anchors. Whole process conformance passes; its run log and final CEM checking
are retained in the lane handback. No independent re-review is claimed here.

Raw evidence stays in
`/home/russell/lanes/clicontract-evidence/literal-help-repair/`.
The original pre-change capture is reused without recapture. Applicable
self-development routes are affected selection, offline CLI discovery and
the final CEM rebind/check. Learning, optional services and mutation routes do
not apply. Independent review, host integration, native Tasks completion,
publication and sealing remain orchestrator-owned. Broad selection, doc-gates,
selection-baseline, exhaustive tests/vet and Windows vet are NOT_RUN under the
owner's focused repair instructions. External interoperability, Darwin
qualification and token/cost savings remain NOT_OBSERVED.

| Artifact | SHA-256 |
|---|---|
| profile-before.log | `04c1c405465834e4546cdb85c23bb5e79ca551a754ee5c241eb2038c699df02a` |
| profile-after.log | `e0be4f9a40e0d2e206e7c9b66316c98949dcda44bfb6496a261019e7cbf9162a` |
| capabilities-package.log | `b11cd895dabfe2e440ec76cfc6ecfd1edf4ed2833d27533fd2815a925ccf3c53` |
| catalog-regression.log | `e6b6cb6f12681c4c5575f8b3735099ba1cce6e86f2fbbd9043b96b7e4fbba8c3` |

## Rollback and friction

Revert the synopsis-parser change and its new regression/spec test reference
to restore the incoming behavior. Existing frozen contracts remain intact;
rollback restores the demonstrated literal-value refusal.

1. The installed affected reader is older than this tree. Reproduce with the
   base command above; affected.stderr records the original d436dff76de1 build
   and 35 differing Go paths. Retain UNKNOWN rather than implying completeness.
2. Host load queues focused heavy commands for 60–81 seconds. Reproduce the
   capability or process command with load 45–51 and configured maximum 40.
   Logs preserve those waits; no lock directory or threshold override is used.
3. Citation checks skip 180 existing basename citations outside their scope.
   Reproduce with staged `script/check-line-citations.sh --repin` or the focused
   Make target. No exemption was added. The first repin also requested staging
   the edited spec; the required staged rerun found no moved anchors.
