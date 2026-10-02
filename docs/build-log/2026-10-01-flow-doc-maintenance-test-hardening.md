# Flow-document maintenance tests kill the checks they name — 2026-10-01

V1-0465 follow-up to merged PR #465 (merge `b2d0c1b7`). Review rev-465-r1 found that the FDM-V0
tests did not exercise the checks they named; #465 merged before this repair was pushed, so it lands
separately on `origin/main` `1d55a8e1dd2c3b9907f1ef07beac8eac9ec75c84`. The evidence bound for the
unpushed repair branch is against the pre-merge base and is not reused.

The refusal cases in `TestFlowDocMaintenanceRefusals` previously applied the pre-tamper digest, so
every case was caught by the digest mismatch. Each case now re-previews after tampering, asserts its
specific refusal message on both preview and stale apply, and checks that the outputs are unchanged.
New cases cover untracked and status-hidden inputs and outputs, a valid receipt whose outputs are
stale, and a stale digest against a no-op state. Two package-private seams in `docs_maintain.go`
(`publishMaintenancePair`, `readRunEvidence`, declared at the end of the file so no cited line moves)
let `TestFlowDocMaintenanceDerivationRaces` race the staging, evidence-reread and HEAD rechecks.
Production calls the same functions as before. `TestMaintenancePairPostPublishRecheck` edits the
first output while the second publishes, and `TestFlowDocMaintenanceCLIGuards` covers both CLI flag
guards and the receipt-delivery report. The spec's acceptance table names the three new tests.

Each named check was deleted on this base and its named test run alone; all 15 mutations failed the
named test (no build failure), and each file was restored afterwards:

| Guard | Site | Named test | Result |
|---|---|---|---|
| receipt integrity/profile | `docs_maintain.go:256` | `TestFlowDocMaintenanceRefusals/forged-receipt` | FAIL |
| receipt output mismatch | `docs_maintain.go:260` | `TestFlowDocMaintenanceRefusals/stale-receipt` | FAIL |
| mixed destination existence | `docs_maintain.go:201` | `TestFlowDocMaintenanceRefusals/mixed` | FAIL |
| aliased destinations | `docs_maintain.go:204` | `TestFlowDocMaintenanceRefusals/alias` | FAIL |
| outputs match committed bytes | `docs_maintain.go:217` | `TestFlowDocMaintenanceRefusals/hidden-output` | FAIL |
| authored or invalid outputs | `docs_maintain.go:222` | `TestFlowDocMaintenanceRefusals/committed-output` | FAIL |
| dirty, untracked or deleted paths | `docs_maintain.go:341` | `TestFlowDocMaintenanceRefusals/{untracked-source,deleted-pair,dirty-output}` | FAIL (all three) |
| stale digest before no-op return | `docs_maintain.go:102` | `TestFlowDocMaintenanceRefusals/noop-stale-digest` | FAIL |
| inputs changed during staging | `docs_maintain.go:118` | `TestFlowDocMaintenanceDerivationRaces/inputs-changed-during-staging` | FAIL |
| evidence changed during read | `docs_maintain.go:290` | `TestFlowDocMaintenanceDerivationRaces/evidence-changed-during-read` | FAIL |
| HEAD moved during derivation | `docs_maintain.go:306` | `TestFlowDocMaintenanceDerivationRaces/head-moved-during-derivation` | FAIL |
| output changed after publication | `apply_pair.go:219` | `TestMaintenancePairPostPublishRecheck` | FAIL |
| maintain flag combination | `flows_docs.go:40` | `TestFlowDocMaintenanceCLIGuards` | FAIL |
| receipt-delivery failure report | `flows_docs.go:60` | `TestFlowDocMaintenanceCLIGuards` | FAIL |
| maintenance flags outside maintain | `flows_docs.go:65` | `TestFlowDocMaintenanceCLIGuards` | FAIL |

The pre-mutation digest check is kept: it is the only refusal before the no-op early return, so
without it a stale digest against a no-op state returns the previous receipt.

Untested: the post-publication parent recheck in `apply_pair.go` (the two `maintenance parent
changed after publication` returns before line 219) has no test that kills it, and the
receipt-integrity condition is mutation-checked as a whole, not per disjunct. Release-boundary
qualification, docs-MCP parity and concurrent-process qualification stay NOT_RUN as recorded for
#465; repository-wide `make gate` is NOT_RUN by the owner's scoped-work instruction. Rollback
reverts this change; production behaviour is unchanged by it.

## Correction to `2026-10-01-flow-doc-maintenance-port.md`

This entry corrects the merged #465 entry `docs/build-log/2026-10-01-flow-doc-maintenance-port.md`,
which stays unedited (decision 0423). Fidelity: that entry says the codex-branch files were "taken
byte-for-byte". They were copied from a local checkout of the codex branch; neither that branch nor
`44cf8ac5` is published on origin, so fidelity to the codex source cannot be verified from the
public repository. What is verifiable is the content of port commit `4c73e037` and the tests above.
touchPaths: the V1-0465 ticket's touchPaths name `2026-09-28` build-log paths from the codex branch;
the port recorded its evidence in the `2026-10-01` entry instead, so the delivered path differs from
the ticket's touchPaths.
