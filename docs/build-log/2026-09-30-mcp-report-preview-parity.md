# CI integration: readonly CEM report parity

Owner-requested PR #385 merge exposed two integration regressions at
`23799bb63832470014b96a4be57a7e564a335e76`. CI run `36713890494` failed the MCP
preview parity test and the spec-index header/README test. The MCP failure was
also reproduced locally before repair. Native bug tickets retain the original
failures, versions, expected behavior and focused observations.

## Existing authority and repair

`MCPV0-025` requires the readonly preview to render the CLI report's Markdown.
The published report appended the new CEM review projection, while the preview
still rendered only its old summary. The preview now computes the same
projection from the same verified inputs, preserves projection errors, applies
the report byte bound after both renderers, and includes the same record-set
digest and OCM validity condition. It publishes nothing. The existing bridge
parity test still verifies unchanged repository bytes and identical Markdown;
it additionally compares the record-set digest.

The QAT README row now starts with its existing registered claim. Its proposed
and experimental status, original-task binding, refusal and qualification limits
remain unchanged. This introduces no accepted intent or new requirement IDs.

## Evidence and limits

- Gate A: independent review PASS, no HIGH concerns.
- Focused race tests: `internal/mcp/bridge` PASS (21.630s),
  `internal/cem/workflow` PASS (96.933s).
- `internal/specindex` PASS (0.289s); focused vet PASS.
- Original failed and repaired logs are retained under
  `/private/tmp/corvint-wow-manager/` with the `mcp-preview` and `merge-ci-product`
  prefixes. Corvint context was READY with omissions retained; the affected plan
  retained its language-frontier unknowns.

Independent implementation review, the separate repair evidence binding and
required final CI are recorded by the task's private receipts and PR. The prior
six-workflow CEM remains bound to its original commit. This repair does not
promote any workflow or convert structural closure into semantic correctness.
Rollback is a revert of this repair; the original failures remain evidence.
