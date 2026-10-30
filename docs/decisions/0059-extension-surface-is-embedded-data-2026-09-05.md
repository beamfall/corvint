# Decision 0059 — The extension surface is embedded data, and its cost is a conformance row

Date: 2026-09-05. Status: accepted. Authority: repository owner, delegated call; the owner instructed on 2026-09-05 that open calls be resolved through the expert panel, and this record applies the panel memo `docs/reviews/panels/panel-4-plugin-wire.md` after independent review of its cited evidence.

## Context

Fourteen extension points exist and none admits a host or a language without a Go diff in this
repository (`docs/reviews/r9-plugin-strategy.md`). The only measured per-event extension cost is the
host adapter: one interpreter start plus a second `corvint` process, in three languages that each
restate the 8000-byte cap. No plugin surface has a latency bound any test enforces.

## Decision

1. Frozen as the plugin-facing contract: `corvint-harness-event/0` with its golden fixture, the
   `subset-of-recognised`/`refuse` degradation rule, the `Unit`/`Result.Frontier` data shape, the
   `go-live-*/0` provider wire, the CEM basis-relation set, snapshot-as-derived-state, and the
   `SOL-V0-007` ledger.
2. Every extension table is embedded in the binary. Corvint performs no runtime plugin discovery and
   reads no plugin file from the worktree; a worktree-readable table would be an unsigned authority
   input under invariant 3.
3. The host enum becomes an embedded host-schema table under the same degradation rule, so a fifth
   host is a data-plus-fixture diff.
4. An extension class may add at most 50 ms p95 and 100 ms max over the kernel for the same golden
   event, zero extra Corvint processes per event, and at most one host-forced runtime declared in
   `compatibility.json`, which must not restate any capped value. "Zero extra processes" as worded is
   rejected because OpenCode's plugin API is JavaScript.
5. A class without a passing `adapter-overhead` or `plugin-zero-cost` row in `conformance/perf-v0`
   is `FALLBACK`, never `FULL`; the manifest test rejects a plugin row with no bound.
6. A Go-native `hook` verb is accepted in principle and built only after item 5 exists and item 3 has
   landed; it must carry the `IDX-SNAP-V0-012` detached refresh and amend that clause.
7. A language manifest is accepted spec-first for declarative fields only (suffixes, tier, reason
   code, test patterns, keywords); import resolution stays Go; it is not built while any snapshot
   layout change is in flight.
8. `cem/0.3` and the DSSE attestation over the dogfood gate are backlog, not release-blocking:
   `CEM-CB-004` exact-spec dispatch makes a new profile purely additive, and commit b566241 closed
   the gate deadlock without a wire change. CEM profiles never negotiate subsets.
9. Snapshot layout work is DNIP P0-C format selection against the `DNIP-IDX-009` immutable-SQLite
   baseline under the invariant 7 gate; it is not a free-standing rewrite. Canonical sorted section
   encoding is in scope; a shared body store is out of scope. Overlapping the engine digest with the
   Git spawns (r2 O1) is unblocked and independent.

## Consequences

The host literals in `cmd/corvint/main.go` and `internal/gokernel/harness.go`, `AHI-011`, `AHI-015`
and the AHI platform table are amended with the table. Adapter deletion is deferred until the
overhead row can show the win.

## Rollback

Each item is additive and independently revertible: drop the perf rows and restore the enum literals
from the table's contents; no artifact, receipt, or cache migration.
