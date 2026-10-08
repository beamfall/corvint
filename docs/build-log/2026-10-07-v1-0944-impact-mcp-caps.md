# 2026-10-07: impact per-result caps, path impact byte budget, MCP text summary (V1-0944)

## Intent

The owner-requested token-usage audit (V1-0944) measured one-path CLI impact at about 31 KB.
Per-result `references` and `evidence` were unbounded, path impact refused `--budget-bytes`, and
MCP `corvint.impact` returned about 66 KB because the text block and `structuredContent` carried
the same payload. The new requirements, all proposed pending owner acceptance, are:

- `MCPV0-031`: at most four evidence rows and four references per impact result, with
  `evidence_omitted` / `references_omitted` counts.
- `MCPV0-032`: path impact honours `--budget-bytes` through the existing `query` packet compiler.
- `MCPV0-033`: the text block carries the `corvint-mcp-text-summary/0` projection when the
  receipt has a `results` list.

## Decisions

- The cap lives inside `contextindex.impact`, not in the CLI or MCP layer. This keeps
  `FPK-V0-010` (prove embeds the impact packet), witness and evaluation consumers on the same
  rows. Verification commands derive from the capped evidence.
- The summary keeps every field of the bridge object except the per-row body, so receipt
  binding, coverage, uncertainty and abstention stay visible to text-only clients. The
  terminator-collision refusal is still decided on the full object.
- The summary departs from the MCP SHOULD that structured results also serialize the JSON in
  text. This is recorded in `MCPV0-033` and left for the owner.
- The `GPK-V0` "only `--limit`" clause and `docs/DOGFOOD.md` now name range and untracked
  impact as the refusing forms.
- The analyzer audit schema moves to `corvint-analyzer/110` because `impact.go` is a pinned
  analyzer input.

## Measurements

These are root worktree measurements at base `0b5096ca`, with limit 10, comparing the base
binary with the candidate binary.

| Surface | Before | After |
|---|---:|---:|
| MCP `corvint.impact` frame, `internal/contextindex/impact.go` | 67,014 B (text 32,208, structured 31,921) | 23,598 B (text 6,031, structured 16,782) |
| MCP `tools/list` | 3,650 B | 3,650 B |
| CLI impact `internal/contextindex/impact.go` | 31,450 B | 16,311 B |
| CLI impact `internal/mcp/bridge/bridge.go` | 18,874 B | 13,011 B |
| CLI impact `cmd/corvint/main.go` | 24,094 B | 15,735 B |
| CLI impact `internal/contextindex/index.go` | 24,424 B | 18,634 B |
| CLI impact `impact.go --budget-bytes 8000` | refused | 7,319 B, `BUDGETED` |

## Limits

- Builder-level `MAX_EVIDENCE` truncation happens before the cap and remains uncounted.
- The `tools/list` snapshot sub-schema duplication (about 600 B) is not addressed.
- MCP exposes no `budgetBytes` argument yet.
- The VS Code extension validator mirrors the summary; its `tsc` typecheck and lint were
  `NOT_RUN` in this lane.

## Batch integration

V1-0944 and V1-0947 each bumped the analyzer schema to `corvint-analyzer/110` independently. Batch C
combines both changes as `corvint-analyzer/111`, with a new audited input SHA in
`internal/contextindex/analyzer_schema_test.go`.
