# Decision 0241 — the qualified lifecycle compact start carries the AHI-003 compaction codes

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`AHI-003` requires a host exposing post-compaction session start to rehydrate tracked dirty-path
impact, expose untracked paths as a count/digest, or name why it could not. The
`corvint-dogfood-event/0` adapters do this and append the block's `compaction-*` codes to
`degradations`. The protected `corvint-qualified-lifecycle/0` profile passed `rehydrate=false`
because `QLF-V0-005` and its wire table fixed `degradations` to `[]` on FULL and exactly
`["native-tuple-unqualified"]` on FALLBACK, and the renderer refused anything else.

The call: a qualified compact session start over a dirty worktree carries the same compaction block
under the prompt packet's `compaction` key. `degradations` is the support codes followed by exactly
the codes `gokernel.CompactionDegradations` derives from that block, in its order:
`compaction-dirty-set-over-budget`, `compaction-untracked-paths-not-rehydratable`,
`compaction-critical-evidence-overflow`. FULL has no support code; FALLBACK keeps
`native-tuple-unqualified` first. A compaction code does not change `support` or `qualification`:
it discloses a context limit, not a qualification fact. The renderer recomputes the expected codes
from the receipt's own block and refuses any other code, order, or count, and a `compaction` key on
any event other than session-start.

Rejected alternatives: carrying the block without the codes, which would hide an unresolved
untracked remainder that `AHI-003` requires to be named; and demoting a FULL compact start to
FALLBACK, which would make a context bound look like a lost native qualification.

Consequences: `QLF-V0-005`, the qualified wire table and `AHI-003` are amended.
`cmd/corvint/qualified_lifecycle.go` enables `rehydrate` and replaces the fixed degradation checks
with `qualifiedCompactionCodes`. `TestQualifiedLifecycleCompactSessionStartRehydratesDirtyPaths`
pins both support levels and three refused tamperings. The compaction codes were already listed in
`integrations/compatibility.json` `receiptDegradationPolicy.recognised`; no conformance fixture
pins the qualified wire. Native latency with the full-index impact build inside the 1600 ms
invocation deadline is not measured here; it stays under the `QLF-V0-010` native evidence.

Rollback: revert the commit. That restores `rehydrate=false`, the fixed closed degradation checks,
the unamended requirement texts, and the `docs/agent-memory/fixes.md` entry.
