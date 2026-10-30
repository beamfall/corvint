---
name: corvint-context
description: Gather revision-pinned Corvint evidence for a task before reading or editing code. Use at the start of a change, or when you need the governing spec clause, source or tests behind a feature.
license: Apache-2.0
---

# Corvint context

1. Call `corvint_context` with `{"task": "<the task in one sentence>"}`. It returns a packet handle
   and ranked evidence rows, each pinned to an immutable blob with its inclusion reason.
2. Open only the rows you need with `corvint_expand` (`handle`, `result`, `evidence`, and either
   `lines` such as `10:25` or a `requirement` ID). A stale packet refuses; ask for context again.
3. For a repository map, call `corvint_core_read` with `{"operation":"overview","input":{}}` or
   `{"operation":"features","input":{}}`. Both read a clean HEAD only and are heuristic guidance.
4. For a free-text lookup, use `{"operation":"query","input":{"task":"..."}}`.

Report unknowns, omissions and refusals as they appear in the receipt. Missing evidence is
uncertainty, never a reason to invent an answer. Project-owned specs outrank code and history.

Other Corvint capabilities: the catalog at
https://github.com/beamfall/corvint/blob/main/docs/CAPABILITIES.md gives each command, binary and
MCP tool with its maturity and refusals.
