# Selection-baseline floor re-recorded at 14 under the non-code-map rules

Date: 2026-11-12. Batch PR 197 (`claude/batch-1111a`). Ticket V1-5424.

## Decision

Owner decision (2026-11-12, in session): re-record `floor.narrowMerged` in
`benchmarks/selection-baseline/changes.json` from 33 to 14. `misses` stays 0.

## Evidence

The batch delivers the non-code map (NCM-V1-001..006, owner decisions R31 and R32, 2026-11-09).
Under it, every Corvint configuration file under `.corvint/` runs every test, and `Makefile`,
lockfiles and `.github/**` never narrow (NCM-V1-005). Replaying the fixed change set on the batch
head gives `narrow=14/43 merged (rate 0.326) misses=0`, down from 36/43 on origin/main
(`docs/build-log/2026-11-11-affstatus.md`).

The 19 changes that no longer narrow, by first full-run reason:

- `corvint-config .corvint/test-read-scopes.json`: 8
- `corvint-config .corvint/test-read-scope-literals.json`: 3
- `never-narrowed-path Makefile`: 4
- `.corvint/shell-foreign-callers.json`, `.corvint/documentation-paths.json`,
  `.corvint/cem-lineage.tsv`: 1 each
- two `package-lock.json` files and `.github/workflows/release-gates.yml`: 1 each

No narrowed change misses a failed package (`misses=0`). The per-change `recorded` fields of each
moved change now carry the batch replay's CI scope and verdict (TSP-V0-710).

A sealed CEM under `.corvint/changes/` named by a full commit is change evidence, not configuration
(`2026-11-12-ncm-sealed-cem-evidence.md`). That rule recovered 13 of the 32 changes that first
moved.

## Follow-up

Precision lost here is tracked as V1-5465 (NEED-1.0). The candidate fix is narrowing an edit to
`test-read-scopes.json` or its literals file to the packages whose entries changed.

Rollback: restore `floor.narrowMerged` to 33 and the `recorded` fields from the parent commit.
