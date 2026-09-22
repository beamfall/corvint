---
name: fixes
description: Non-defect fixes: tech debt, code quality, misleading text
updated: 2026-09-21
---

# Fixes

Non-defect fixes: tech-debt patches, code-quality adjustments, misleading comments or text. Remove the entry when done. Entries are dated, newest first, and kept to one short paragraph. The public tree starts this backlog empty.

<!--
### YYYY-MM-DD <area>: <one-line title>
One paragraph: what, where (file:line), why it matters, and what done looks like.
-->

### 2026-09-21 cmd/corvint: `CPUPROFILE` makes two read commands write a file
`cmd/corvint/taskcontext.go:86@aa1f0538` and `main.go:1142` call `os.Create(profilePath)` when `CPUPROFILE` is set, from the documented read-only `context` command and the generic harness-event path. Off by default and operator-pointed, so not an invariant-4 violation, but undocumented. Done: document the env knob beside the invariant, or move profiling behind an explicit flag.
