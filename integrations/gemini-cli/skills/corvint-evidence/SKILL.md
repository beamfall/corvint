---
name: corvint-evidence
description: Use bounded, revision-pinned Corvint evidence when investigating repository code, specifications, impact, or change frontier status.
---

# Corvint evidence

Use the installed `corvint` CLI only through Gemini CLI's normal tool permission flow. Use
`corvint query --task "..." --limit N` for project-operations orientation, repository, and
agent-tooling tasks (limit 1-50, default 10) and
`corvint impact PATH... --limit 10` only for supported Go-path impact. Preserve receipt IDs and
exact revision identity. Preserve explicit unsupported results; never invoke a legacy runtime. Never treat evidence handles as proof the model read a result, tool observations
as proof a command passed, or FALLBACK stop advice as frontier authority. A missing or degraded Corvint
must remain visible but must not prevent unrelated coding.
