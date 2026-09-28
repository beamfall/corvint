# Git root and opaque submodule boundaries — issues 334 and 335

Owner request: fix GitHub issues #334 and #335. The owner explicitly selected opaque gitlinks:
recorded commit IDs, nested contents outside coverage, and safe checkout-commit observation.
This amends MCPV0-001 and EAF-V0-007; GENESIS-002 retains its explicit content-gap classification.
Base: `2e281305c49badc68d1a46035ca4ff966cbaff7c`, isolated branch `codex/git-root-fixes`.

The failing regressions reproduced directory-only MCP root admission and live-index gitlink
refusal. MCP now reuses reciprocal worktree validation, pins administrative identities, and
rechecks binding around calls. A same-inode pointer retarget cannot select another repository.
Opaque status reads bounded no-follow checkout HEAD/ref metadata, then lets Git format records
against an inert private checkout. Real nested Git configuration and file contents are never
used. Staged additions, removals and commit changes, checkout OID changes and type changes remain
observable. Gitlink mode disables rename detection, preserving source/destination paths as
add/delete. Unmerged gitlinks, non-NUL or pathspec status requests, unresolved HEADs and bounded
metadata refusals remain explicit. No CLI status verb was added; that was an optional suggestion.

Verification before final binding: failing-then-passing regressions, complete gitstatus and bridge
package tests, and added Genesis/SHA-256/drift tests passed. Candidate binaries replayed context,
index, init and MCP stdio on both primary and linked roots. Init is PARTIAL only for the explicit
`gitlink` content gap; declaring the prefix excluded yields COMPLETE. Final selected package tests
and vet are enrolled in the private dogfood plan and must pass before completion. Repository-wide
`make gate` is NOT_RUN under the owner's scoped-issue policy; no retrieval ranking changed.
Independent review found a Windows compilation gap and rejection of the inert `--ignored=no`
spelling used by affected/parent verification. Both were repaired, with the no-follow capture
kept platform-specific. Review findings and final gate results remain
in the task evidence directory and dogfood report.

Actual self-use: query, tracked-path impact, affected selection and the enrolled CEM/OCM workflow.
The first generic query returned irrelevant authority and was not retained; the specific
pre-change query/impact are retained under the worktree Git directory. Query omitted four rows;
impact omitted 57 rows, including direct callers. Source inspection and the selected kernel,
index, Genesis, MCP and authority tests address those paths; this is not exhaustive coverage.
The initial change pass, before any committed diff, reported CEM preparation NOT_PRODUCED
(`git-diff-failed`) and outcome NOT_PRODUCED (`outcome-input-not-provided`). These are retained.
Frozen retrieval evaluations, mutation, external providers, learning, console and publication are
not applicable to this compatibility repair. No source-ranking or learned-weight change, no
external outcome qualification, and no token-savings claim. Token and monetary cost are
NOT_OBSERVED. Usage baseline was 35% weekly. Private evidence/checkpoint:
`/private/tmp/corvint-git-root-fixes/`.

Rollback: revert the implementation/spec change together and invalidate the analyzer schema;
keep the failed reproductions and existing metadata nonexecution protections.
