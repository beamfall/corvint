---
name: optimizations
description: Performance and refactor improvements to apply
updated: 2026-09-19
---

# Optimizations

Performance wins and scoped refactors. Remove the entry when applied. Entries are dated, newest first, and kept to one short paragraph. The public tree starts this backlog empty.

<!--
### YYYY-MM-DD <area>: <one-line title>
One paragraph: what, where (file:line), why it matters, and what done looks like.
-->

### 2026-09-19 gate-affected: the rule (d) unresolved floor selects 103 packages on any dirty path
At `3f30a02`, `tools/gate-affected-select` marks 103 packages `unresolved` (`readers.go` `unresolved()`), so every non-empty change runs at least about 105 packages with `-p 1`. That is slower than `go test ./...` in parallel. 37 locate the root themselves (`runtime.Caller`/`os.Getwd`), and many others inherit it from a dependency, 10 of them via `cmd/corvint`. Candidate levers: move test root-location onto one shared helper whose result is bounded; or run the fast tier's `go test` without `-p 1` when the union is this wide. Done means a one-package change selects well under half the module, with rule (d) still fail-closed. Measurements are in the BUILD-LOG entry of 2026-09-19.

### 2026-09-19 gate-affected: token edges held only in `_test.go` files still propagate to importers
`link()` in `tools/gate-affected-select/readers.go` records a literal that names a package directory as an importer edge, whether or not the literal occurs in a test file. A test file is never imported, so its holder's importers cannot be reached through it. A throwaway variant that keeps the holder but stops the closure there moved a `cmd/corvint/go_only_cutover_test.go` selection from 119 to 114. The variant also changes which packages rule (d) marks unresolved by dependency, so it needs its own fixture test before adoption.
