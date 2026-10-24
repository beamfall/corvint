# 2026-10-24 V1-1003: MCP impact budget, context summary and the evidence-ceiling count

Owner-held decision (docs/build-log/2026-10-24-owner-held-decisions.md, PR #1721) for V1-1003.
Requirements MCPV0-110..112 in `docs/specs/mcp-server-2026-07-28-v0.md`.

## Wire changes

- `corvint.impact` accepts optional `budgetBytes` (integer, 1216..1000000). When present the bridge
  compiles the receipt the CLI's `impact --budget-bytes` compiles (`contextindex.EvalImpactSyntax`)
  for the same index, paths and limit; when absent the unbudgeted receipt is unchanged (MCPV0-110).
- `corvint.context` accepts optional `summaryBytes` (integer, 1024..65536). When present the result
  is the CLI's `experimental-evidence-summary/0` projection of the canonical receipt plus LF, with
  an MCP continuation that names `packet_sha256` and calls `corvint.context` again without
  `summaryBytes` (MCPV0-111). The projection moved to `internal/contextsummary` so both surfaces
  share one implementation; the CLI keeps its continuation text and bytes.
- Explicit `null`, fractions, strings and out-of-range values are `invalid-arguments` for both.
- Impact results whose evidence the per-result ceiling of 10 cut now carry `evidence_truncated`,
  counted separately from the display cap's `evidence_omitted`, plus one coverage uncertainty line
  (MCPV0-112, amending MCPV0-031). This changes impact receipt bytes only when a result exceeds the
  ceiling; the analyzer schema input digest was repinned, the output schema and use-case receipts
  are unchanged.

## AC1 amendment

AC1 asked for a context packet byte budget. The decision instead exposes the existing CLI
`--summary-bytes` evidence summary; no new context packet budget exists. AC3 (`$ref` dedup of the
tool schemas) is rejected.

## tools/list bytes

| golden | before | after |
| --- | ---: | ---: |
| default | 4301 | 4367 |
| flows | 10208 | 10274 |
| task-review | 6016 | 6147 |
| task-review-lsp | 6296 | 6427 |
| total | 26821 | 27215 |

Rollback: remove the two arguments from the schemas and `validImpact`/`validContext`, and drop
`evidence_truncated` and its disclosure from `internal/contextindex`.
