---
name: bugs
description: Reproducible defects awaiting a fix
updated: 2026-09-20
---

# Bugs

Reproducible defects: something is broken, with a reproduction. Remove the entry once fixed. Entries are dated, newest first, and kept to one short paragraph. The public tree starts this backlog empty.

### 2026-09-20 local completion: verbose Go PASS lines are secret-screened

`dogfood verify` rejects a successful `go test -v` observation because
`internal/secretscreen` interprets Go's `--- PASS: TestName` marker as a credential-shaped `pass:`
assignment; issue #50 reproduced this as `log-secret-screened`. Fix the structural false positive
without weakening real `pass: value` detection, and cover retained verbose Go output end to end.

<!--
### YYYY-MM-DD <area>: <one-line title>
One paragraph: what, where (file:line), why it matters, and what done looks like.
-->
