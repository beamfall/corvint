---
name: corvint-evidence
description: Use bounded, revision-pinned Corvint evidence when investigating repository code, specifications, impact, or change frontier status.
---

# Corvint evidence

Use the installed `corvint` CLI only through Gemini CLI's normal tool permission flow. Use
`corvint query --task "..." --limit N` for project-operations orientation, repository, and
agent-tooling tasks (limit 1-50, default 10) and
`corvint impact PATH... --limit 10` for path impact: Go, plus Ruby, JavaScript and TypeScript under
the experimental `non-go-syntax-v0` profile. Other suffixes the index admits (for example `.py`,
`.md`) get positional evidence with missing-rule uncertainty, and unadmitted suffixes refuse with
`unsupported-impact-path-suffix`. Preserve receipt IDs and
exact revision identity. Preserve explicit unsupported results; never invoke a legacy runtime. Never treat evidence handles as proof the model read a result, tool observations
as proof a command passed, or FALLBACK stop advice as frontier authority. A missing or degraded Corvint
must remain visible but must not prevent unrelated coding.

`corvint help` lists every command; the catalog at
https://github.com/beamfall/corvint/blob/main/docs/CAPABILITIES.md gives each command, binary and MCP tool
with its maturity and refusals.
