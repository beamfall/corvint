---
name: optimizations
description: Performance and refactor improvements to apply
updated: 2026-09-21
---

# Optimizations

Performance wins and scoped refactors. Remove the entry when applied. Entries are dated, newest first, and kept to one short paragraph. The public tree starts this backlog empty.

<!--
### YYYY-MM-DD <area>: <one-line title>
One paragraph: what, where (file:line), why it matters, and what done looks like.
-->

### 2026-09-21 contextindex: regexes compiled per call in eval queries
`internal/contextindex/eval_query.go:1329-1335@714c7d93`: `evalNegativeClaim` and `evalCapabilityOverride` call `regexp.MustCompile` on every invocation, and the former runs once per competitive record (`:1317-1320@96bc9a3c`). Hoist to package vars like every other regex in the package.

### 2026-09-21 contextindex: impact scans are quadratic over `index.Sources`
`internal/contextindex/impact.go:112-117@4e8273b7` and `:659@e0e5a847` iterate the whole `Sources` map per changed path (up to 100), and `samePackageReferences` (`:668-673@0f3507aa`) does `lines × names` `containsPythonWord` scans per sibling. `recordResult` (`:658-663@ed26f084`) scans all sources per ADR ID per ranked record. A directory→paths index built once per call, a tokenised name-set lookup, and a sorted path slice with `sort.SearchStrings` collapse these.

### 2026-09-21 contextindex: repeated splits and spawns in authority and range impact
`authority_trigger.go:235` re-splits the cited source per citation (cache lines per target path in `authorityTriggerRows`). `range_impact.go:372-379` is a linear scan called per Go path at `:494` and `:154` while a map is built later at `:196-199`; build it first. `range_impact.go:80-92` spawns `cat-file -t` and `rev-parse base^{tree}` that one `rev-parse --verify base^{commit} base^{tree}` covers; keep the deliberate closing repeat at `:343-351`.

### 2026-09-21 jstestprovider: reporter spawns `--version` per test and hashes config twice
`qualified-reporter.cjs:282,357` runs `spawnSync(executablePath, ['--version'])` on every `onTestEnd`; memoize per path as `bundledBrowsers` already does (`:14,:289,:304`). `:380-383` reads and hashes every cached module whole, then Go re-digests each in `bindQualifiedReport` (`external.go:429`); since Go re-verifies anyway, JS can emit paths only. `sensitive_input_boundary.go:111` runs one `ReplaceAll` per sensitive value per field; one `strings.NewReplacer` per receipt is a single pass (and fixes the ordering bug in `bugs.md`). `application_attestation.go:35,48,101` reads the executable three times; stream copy-and-hash once.

### 2026-09-21 workqueue: `candidateAdjacency` allocates and sorts to test overlap
`internal/workqueue/proposal.go:524@b925fd98` calls `intersection` (`:488@1db8b264`, alloc + map + sort) per candidate pair only to compare length to zero; a short-circuit set lookup is allocation-free and O(n²) pairs no longer each sort.

### 2026-09-21 extevidence: `strictMCP` parses each frame four times
`internal/extevidence/mcp.go:190@cfb13fd9` parses for uniqueness then decodes with `DisallowUnknownFields`, and `mcpResponse` (`:149-171@79af857b`) calls `mcpObject`→`strictMCP` then `strictMCP` again, for both `initial` and `result` in `mcpExchange`; up to 32 MiB of JSON scanning per 8 MiB frame. Parse once and reuse.
### 2026-09-19 gate-affected: the rule (d) unresolved floor selects 103 packages on any dirty path
At `3f30a02`, `tools/gate-affected-select` marks 103 packages `unresolved` (`readers.go` `unresolved()`), so every non-empty change runs at least about 105 packages with `-p 1`. That is slower than `go test ./...` in parallel. 37 locate the root themselves (`runtime.Caller`/`os.Getwd`), and many others inherit it from a dependency, 10 of them via `cmd/corvint`. Candidate levers: move test root-location onto one shared helper whose result is bounded; or run the fast tier's `go test` without `-p 1` when the union is this wide. Done means a one-package change selects well under half the module, with rule (d) still fail-closed. Measurements are in the BUILD-LOG entry of 2026-09-19.

### 2026-09-19 gate-affected: token edges held only in `_test.go` files still propagate to importers
`link()` in `tools/gate-affected-select/readers.go` records a literal that names a package directory as an importer edge, whether or not the literal occurs in a test file. A test file is never imported, so its holder's importers cannot be reached through it. A throwaway variant that keeps the holder but stops the closure there moved a `cmd/corvint/go_only_cutover_test.go` selection from 119 to 114. The variant also changes which packages rule (d) marks unresolved by dependency, so it needs its own fixture test before adoption.
