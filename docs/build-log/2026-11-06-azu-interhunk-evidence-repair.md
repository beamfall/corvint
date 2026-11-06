<!-- SPDX-License-Identifier: Apache-2.0 -->
# AZU replay inter-hunk evidence repair and unresolved-join blocker

Self-review identified two admission gaps. This round delivers only P2 under
V1-4692 / AZU-V0-009 and AZU-V0-8903. P1 under V1-4687 / AZU-V0-004 and
AZU-V0-8901 remains PARTIAL because the conservative repair conflicts with
existing assertions frozen by the owner's repair instructions. No assertion,
golden, wire shape, diagnostic or exit status was changed to admit it.

## P2: actual source mutation coordinates

Git's `diff.interHunkContext` setting can merge zero-context hunks and include
unchanged lines. The previous range-based parser admitted those coordinates as
mutation evidence and could report `OBSERVED_NO_MISS` for unchanged-line mutants.
Decision: explicitly pass `--inter-hunk-context=0` and independently parse actual
added/deleted records. Context advances both coordinates without marking either;
additions advance only merge coordinates, deletions only parent coordinates.
Existing Git path decoding and deleted-file handling remain in force.

`TestInterHunkContextRejectsUnchangedMutants_AZU_V0_8903` creates a real repository
with two edits separated by an unchanged line and configures inter-hunk context
20. It checks that the raw diff contains context, the canonical source diff
excludes it, and both inputs reject unchanged-line mutants on both sides with
`NOT_PRODUCED`. Actual edited coordinates remain present. The whole replay
package failed before repair (rc 1, 0.357 s) and passed afterward (rc 0, 0.346 s),
including every existing test. Existing missing-receipt and historical replay
qualification limits remain unchanged; no aggregate qualification is claimed.
Rollback must preserve actual-line coordinate admission and configuration
independence; removing only the command flag cannot replace the parser repair.

## P1: frozen assertion conflict, attempted repair reverted

The integrated `TestUnresolvedJoinTraversalRetainsUnknown_AZU_V0_8901` reproduced
the reported `filepath.Join("scratch", up, "run", name+".sh")`, with `up == ".."`
and `name == "run"`: BOUNDED, no unknowns, and excluded extra caller tests.
A conservative completeness veto for unlisted sources containing unresolved
join operands made the whole shell package pass (rc 0, 0.717 s) and preserved
known caller selections. It treated an unmatched directory pattern as absence
of positive reference evidence, never as proof of exclusion. Existing low-level
directory-recognition assertions were left intact.

That candidate failed the whole affected package (rc 1, 74.247 s). Existing
`TestRepositoryInboundCallers_AZU_V0_8901`,
`TestRepositoryShellForeignCallersComplete_TSP_V0_005`,
`TestSealCommitPlanIsBounded_V13898` and
`TestSealOnlyCEMDeltaStaysBounded_V13571` require the repository declaration to
close this uncertainty. Repository source has unresolved joins outside the
callee directory; their possible values have no admitted exclusion proof.
Broadly declaring those sources would require caller/mention classifications
that this round has not established. These bounded-plan expectations cannot
certify this conservative candidate.

Decision under the owner's explicit frozen-assertion rule: revert the attempted
P1 source/test edits, retain their patch and observed failures externally, and
report P1 PARTIAL rather than weaken assertions or claim a narrower prefix-only
fix proves exclusion. The reported completeness hole remains in the delivered
source. Required follow-up: owner-authorized reconciliation of the frozen
bounded/declaration expectations with conservative unresolved-operand uncertainty,
or separately qualified value/execution exclusion proofs. Neither is produced
by this round. The attempted regression is retained as evidence, not a passing
committed witness. No Tasks write or completion is claimed. After the rollback, the whole shell
package passed (rc 0, 0.702 s) and the whole affected package passed
(rc 0, 150.353 s), including the unchanged assertions that rejected the attempt.

## Evidence and routes

Raw artifacts remain under
`/home/russell/lanes/azu-new-evidence/self-review-traversal-context/`.

| Artifact | SHA-256 |
|---|---|
| `shell-before.log` | `ffb9f71b75a31fe11468db2cfa9bfb8c96f73c687d7c79d4b60960a2689af81e` |
| `shell-p1-attempt.log` | `7365d153c2b0de9ba6ec65cdbb280cecd49b21c84daa7e8a10977a67876205f8` |
| `affected-p1-attempt.log` | `5b398754c080b34a8aecc1b71884c4c97ebfd82399ede46532b19fc1728dd4f8` |
| `p1-attempt.patch` | `36a7a567eb8ebbd4835748eae19ccbff97f78855d0e6fea0807666c98c3fdec9` |
| `replay-before.log` | `7df9598c8bfb3090505ce0081fe60d999c327b37baa010363fc3fdf8d529d505` |
| `replay-final.log` | `537aebbd1e07e2598e476cf16570b257503ea78d3d49a70d65f0598cb4d83067` |

Self-development routes: reuse the existing pre-change capture; no replacement
capture. Used the exact-base affected plan: UNKNOWN, 186 selections, 86 unknowns;
shell, replay and affected packages selected, with stale installed-binary advice
retained. Used whole-package focused tests through the configured heavy wrapper
and the private lane TMPDIR. Documentation checks and the terminal CEM check are
retained in the external handback. CEM closure applies to delivered edits and
does not upgrade P1 or historical safety qualification. Independent review and
host verification are orchestrator-owned. Broad gates, publication, Tasks writes,
model-backed review and sealing are excluded by the owner.
