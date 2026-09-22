---
name: questions
description: Open questions awaiting a human answer
updated: 2026-09-21
---

# Questions

Ambiguities that only a human can resolve: intent, product calls, "is this on purpose?". Remove the entry when answered. Entries are dated, newest first, and kept to one short paragraph. The public tree starts this backlog empty.

<!--
### YYYY-MM-DD <area>: <one-line title>
One paragraph: what, where (file:line), why it matters, and what done looks like.
-->

### 2026-09-21 DCP-V1-032: should a prior bundle with dangling criteria disqualify a delta?
`validatePreviousBehaviorAdapterResult` rejects a `--previous` bundle whose test names a criterion absent from the variation inventory, while the forward pass keeps that criterion and reports `undocumented-tested-behavior`. The spec says the prior "must pass semantic-link validation before it can affect a delta" but does not say whether reported findings count as failing it. Which half is intended: the validator tolerates what the emitter emits, or the emitter prunes and the spec says so?
