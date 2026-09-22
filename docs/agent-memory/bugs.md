---
name: bugs
description: Reproducible defects awaiting a fix
updated: 2026-09-21
---

# Bugs

Reproducible defects: something is broken, with a reproduction. Remove the entry once fixed. Entries are dated, newest first, and kept to one short paragraph. The public tree starts this backlog empty.

<!--
### YYYY-MM-DD <area>: <one-line title>
One paragraph: what, where (file:line), why it matters, and what done looks like.
-->

### 2026-09-21 doccorpus: a bundle carrying a real reconciliation finding is refused as `--previous`
`internal/doccorpus/behavior_adapter.go:297-311@b43f74a1` makes `invalid("test variation claim")` / `invalid("orphan claim")` fatal for a prior result, while forward emission at `:1160-1166@110e8943` deliberately retains such criteria and reports `undocumented-tested-behavior` / `missing-reverse-link`. Run 1 exits 0 with the diagnostic; run 2 with `--previous run1.json` exits 2, so the DCP-V1-032 delta is unusable in exactly the case DCP-V1-031 exists to report. Owner call recorded in `questions.md`. Done: either spec says dangling criteria disqualify a prior and the emitter prunes them, or the validator tolerates what the emitter emits.
