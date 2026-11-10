# Design: `docs-mcp-hardening`

## Decisions

- A waiver changes what the page says, not only the report. The page and the sidecar print
  `WAIVED until DATE (now STATE)` for a waived claim. Generation and check both read the waiver file,
  so a regenerated page and a checked page agree (DMF-V1-001).
- `docs check` composes the existing checks; it adds no new analysis. It finds marker blocks by the
  existing marker pattern (`internal/docmaintain/render.go:23`), flow-claim pages by the flows
  manifest and citations by `internal/doccitations`. Each part reports `PASS`, `STALE` or
  `NOT_CHECKED` with a reason. Exit 1 when any part is `STALE`, 0 otherwise; a `NOT_CHECKED` part
  is visible in the report but does not fail the run (DMF-V1-002).
- `docs maintain --preview` keeps exit 0 for scripts that use it today. A new `--check` flag gives
  the failing exit code (DMF-V1-003).
- The citation check keeps reading the index. It adds a `worktree` member rather than reading the
  worktree, so its answer stays pinned to Git content (product invariant 1) (DMF-V1-004, 005).
- Repinning and move detection stay in `script/check-line-citations.sh` for now (finding 3, LATER).
- A flow-doc byte failure diffs the old and new sidecars to name claim IDs. Bytes stay the
  authority; the claim list is an explanation (DMF-V1-006).
- `docs flows generate` records its output directory in the manifest. `docs flows check` uses it by
  default; with no recorded directory and no flag, it reports `output: NOT_CHECKED` and
  `clean: false` (DMF-V1-007).
- Tool descriptions live next to the tool table rows owned by change `mcp-tool-table`, so one drift
  check covers both (DMF-V1-008).
- The 19 docs tools left after B2 are re-described, not pruned. A tool is removed before the
  freeze only on a fixture test showing its answer is a strict subset of another tool's for the
  same input; otherwise pruning waits for 1.1 with one release of notice. Reason: the verdicts
  found descriptions that read alike, not proven duplicate behaviour, and removing a tool breaks
  the agent packages that call it (decided 2026-11-09 under R42, B9).
- One error shape for every MCP tool: `code`, `message`, `retryable` and `next` (a command or
  argument change, or empty). Two new codes: `server-busy` (retryable) and `cancelled`. A child
  process failure keeps its last stderr line, as `affectedFailure`
  (`internal/mcp/toolgroup/affected.go:165-187`) already does (DMF-V1-010).
- A changed corpus artifact is verified with the same checks as the first open, then served. Each
  receipt names the artifact digest it used. A verification failure keeps serving nothing and says
  why; it never falls back to the old artifact silently (DMF-V1-011).
- `how` gap anchors moved to `how-rewrite` as HOW-V1-012 (R42 (panel pick) B1).
- `docs consume` keeps strict byte binding; its stale refusal names the commit or tree that moved
  (DMF-V1-013; R42 (panel pick) B5).

## Failure modes

- A waiver file that does not parse: generation and check fail with the file and line, rather than
  ignoring waivers.
- An expired waiver: the claim prints its real state and the check names the expired waiver.
- `docs check` in a repository with no markers, no manifest and no Markdown: every part is
  `NOT_CHECKED` with reason `none-found`, and the exit is 0.
- A dirty worktree: reports stay answers about the committed or indexed content and say `dirty`,
  with a count and the first paths; they never mix worktree and committed bytes.
- A sidecar missing on one side of a byte failure: the claim list is `UNKNOWN` and the byte failure
  still stands.
- A recorded output directory that no longer exists: `output: NOT_CHECKED`, reason `missing-dir`,
  `clean: false`.
- An artifact that is replaced while a call is reading it: the call finishes on the digest it
  opened; the next call opens the new one.
- An old MCP client that expects the old error object: the new members are added, existing members
  keep their meaning.

## Acceptance evidence

- Fixture tests for each scenario in `specs/docs-mcp-hardening/spec.md`, one test per requirement at
  least, run by the affected-plan selection for the touched packages.
- Re-recorded goldens for flow-doc pages, MCP `tools/list` and `how` gaps, with the diff reviewed.
- A live MCP session that rebuilds the corpus artifact mid-session and calls `docs_search` again
  without restarting (DMF-V1-011).
- A check that no two docs tools share a description (DMF-V1-008).
- `docs check` run on the Corvint repository itself, with its JSON report kept as evidence.

## Open questions

- Decided, R42 (panel pick) B2: `corvint.docs_recommend_tests` is removed; descriptions point to
  the testing server (DMF-V1-009).
- Decided, R42 (panel pick) B3: a rebuilt corpus is verified and reloaded, each answer names the
  digest, and a failed verification refuses (DMF-V1-011).
- Decided, R42 (panel pick) B1: DMF-V1-012 moved into `how-rewrite` as HOW-V1-012.
- Decided, R42 (panel pick) B4: finding 20 renames land before 1.0 with one-release aliases, inside
  the `flows-status-report` window (AFL-V1-008).
- Decided, R42 (panel pick) B5: finding 11 keeps strict byte binding for 1.0 with a clearer stale
  message (DMF-V1-013); cited-files mode is 1.x.
- Decided 2026-11-09 under R42, B9: the docs tool table is re-described, not pruned beyond B2
  (DMF-V1-008). Whether 19 tools is too large a surface is a 1.1 question.

## Rollback

Each requirement lands as its own commit and reverts on its own. Reverting restores today's
behaviour: waivers stay in the report only, `docs check` and `--check` disappear, error objects lose
the added members and the artifact pin returns. DMF-V1 IDs are retired, not reused.
