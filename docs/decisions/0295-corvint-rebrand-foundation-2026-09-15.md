# Decision 0295 — Use Corvint as the prerelease product name

Date: 2026-09-15. Status: accepted. Authority: repository owner, who selected “I don't care about dotcom use Corvint.”

## Decision

The product display name is `Corvint`, the lowercase slug is `corvint`, and the planned source coordinate is `github.com/Beamfall/corvint`. The existing original remote `git@github.com:Beamfall/corvint.git` is evidence of organization ownership; this local change does not rename a remote repository or claim that the new coordinate already exists.

The first implementation slice changes repository-owned Go module declarations, active self-imports and operational module comparisons, native CLI version display, and the Makefile's main executable output. `cmd/corvint` remains temporarily to avoid mixing the source-directory migration into the foundation.

Existing versioned `corvint-*` wire/profile/schema identifiers, `.corvint/change.cem.json`, and the canonical `nextActions` literal `corvint` remain fixed under the CEM canonical-binding contract. Historical results, receipts, decisions, third-party and legal facts retain their original identity. State/env migration, plugins and editor IDs, package/publisher ownership, public URLs, visual assets, task-manager naming, and publication are later slices.

## Requirements

- **CRB-DEC-001:** Adopt the owner-approved `Corvint` name and the linked `corvint-rebrand-v0` contract, including its preserved compatibility and ownership boundaries.

## Consequences

Active Go source compiles under the new module coordinate and the locally built main executable identifies as Corvint. Residual Corvint strings remain expected where they encode frozen protocol/history or deferred migration work; a loose substring count cannot determine completion.

The module rename invalidates the current local analyzer source audit and derived index cache key. Their current-tree pins advance with the renamed source. Completed historical analyzer evidence retains its original source digest and qualification label; updating a current-tree maintenance pin does not relabel or rewrite that evidence. The isolated local-authority guest recipe remains a paired frozen old-module fixture while the host module and imports move to Corvint.

Current dogfood scripts validate `Corvint <VERSION>` for caller-supplied current binaries while retaining exact version equality and the independently built base verifier path. The Python analyzer's offline core-dependency guard follows the new module import and admits only Go's escaped `github.com/!beamfall/corvint/@v/` cache path for the main module.

Rollback reverts the module/import, version-display, and build-output foundation together. No protocol migration or historical rewrite is required.
