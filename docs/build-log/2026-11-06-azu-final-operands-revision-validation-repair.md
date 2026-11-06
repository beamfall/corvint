<!-- SPDX-License-Identifier: Apache-2.0 -->
# Final join operands and historical caller validation repair

This self-review repair addresses V1-4687 / AZU-V0-004, AZU-V0-8901 and
AZU-V0-8904. It retains the owner's frozen wire, diagnostic, golden and
existing-assertion constraints. The existing pre-change capture is reused;
there is no retroactive capture or new historical qualification claim.

## P1: PARTIAL, blocked by the frozen assertion

A Go caller can execute `run/run.sh` with
`exec.Command("sh", filepath.Join("elsewhere", name+".sh"))` when its runtime
argument is `../run/run`. An integrated shell/Go fixture with that argument
supplied through a function parameter reproduces BOUNDED selection with no
unknowns while excluding the undeclared caller's tests. A literal runtime
value inside the source fixture would instead select its package through a
path-literal read, so it is not the reproducer used for this finding.

The conservative candidate treats the unresolved final operand as traversable
unless the existing fixed final suffix excludes every shell suffix. It passes
the integrated regression's three variants (plain, parenthesized and nested
expression). However, the unchanged
`TestNormalizedJoinBounds_AZU_V0_8901` requires exclusion of
`filepath.Join(".", "elsewhere", name+".sh")`, which has the same traversal
possibility. The candidate fails that exact assertion. No sound suffix proof
can exclude a `.sh` suffix from all shell targets.

Decision: under the owner's explicit frozen-assertion repair rule, revert the
candidate and retain P1 as PARTIAL. Do not evade the assertion by using a
weaker matcher in tests or by treating identical joins differently in the
completeness check. The incoming defect remains open; resolving the conflicting
assertion requires owner/orchestrator authority outside this round. The failing
new regression and candidate patch stay in external evidence, not in a passing
committed test suite. No existing assertion or golden is changed.

Restored integrated reproduction log SHA-256:
`7bca5c89b56397429e4145cbe281165b98e66712722102987ea7efcb36188a51`.
Candidate assertion-conflict log SHA-256:
`5ca64a1505e3e16e503994af662bd1a3d24d854080ac04354d8a2bb2fb044fe1`.

## P2: canonical validation at both revisions

Decision: move the existing shell declaration decoder/validator into the
parent affected package, maintaining the import direction. Both immutable
snapshot reads and the shell reader call it. Alias the existing shell entry
type, profile and suffix list to the shared definitions. Keep the existing
JSON members, profile, limits and error strings. Current-tree completeness
and the graph's current-unit/caller comparison remain required in addition
to syntax validation. A historical declaration cannot close uncertainty merely
because its union of paths happens to resolve in the current graph.

`TestInboundRevisionRejectsInvalidSnapshots_AZU_V0_8904` checks nine invalid
entries at both snapshots: empty lists, unsorted/duplicate callers and mentions,
overlap, self-reference, oversized reason and control-bearing reason. Each
case requires the changed-declaration frontier to remain in the actual plan.
Before repair, twelve cases fail, including invalid base with valid HEAD.
After repair, the focused inbound-revision tests pass (0.024s), preserving the
existing valid old/new caller selection test. The whole shell package passes
(0.975s), including the existing exact 1024-byte reason acceptance and
1025-byte diagnostic checks. Whole touched-package and documentation results
are retained in the handback. Rollback restores the weaker independent
snapshot decoder and reintroduces the reproduced historical-validation gap.

Before-fix regression log SHA-256:
`6cf1028e80b937ab09a0c71205f28c36909a739147a9ab7b89171b2947a4bf9b`.
Focused passing log SHA-256:
`352b5c13f667cbb5accb0b69b5fafb96877cf187a6de8198d8a0aa2caecd5070`.

The required affected plan at base
`8fc0cf37642e7012f4c26671c9ed97a8706d6e09` selects both touched packages:
UNKNOWN, 399 selected units and 87 unknowns. Its stale installed-binary notice
is retained; this plan is selection advice, not current-source safety evidence.
Applicable self-development routes are affected selection and final CEM
binding/report inspection. Independent review, broader host checks and native
ticket completion remain orchestrator-owned. Raw evidence and binding inputs:
`/home/russell/lanes/azu-new-evidence/self-review-final-operands/`.
