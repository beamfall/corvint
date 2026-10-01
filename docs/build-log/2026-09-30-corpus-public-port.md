# Public corpus query and parity port

Date: 2026-09-30
Intent: owner issue #399, native V1-0546; public-port V1-0551 and retained defect V1-0548.
Owning contract: `docs/specs/documentation-corpus-v1.md`, DCP-V1-038 through DCP-V1-042.

## Public boundary and decision

The candidate starts at public commit `25971bda1ca1664d8a546d751cb2bb9bbd454daf`
(tree `b25933041dce55c2f7212ff77c943e5b837601ff`). Only the 21 corpus implementation,
test and owning-spec paths were imported from the earlier unpublished prototype. Its private
ancestry, generated requirements inventory and CEMs were not imported. The old source,
failed review and satisfied binding remain preserved separately. All existing imported paths
matched the public base before application; the exact patch passed a dry-run and public compilation.

V1-0548 is a prototype defect: this public base has no indexed query parity package or command.
A scratch reproduction and the public-port regression both observed one identical weak gap becoming
two as one missing and two extra. The repair gives every weak row a canonical-content digest and
occurrence number, including a singleton. The same transition now has zero missing and one extra;
the reverse has one missing and zero extra. Strong IDs retain changed-content diagnostics and
reject duplicates. Weak content changes are missing/extra because those records lack a strong ID.
Full receipt comparison remains strict and order-sensitive; diagnostic record keys are independent
of result ordering. This change does not authorize retiring a prior server.

## Requirement witnesses

| Requirement | Focused witness |
|---|---|
| DCP-V1-038 | `TestCorpusTypedQueryConformance`, `TestDocsCorpusTypedCLIInvocation` |
| DCP-V1-039 | `TestCorpusRetirementBindings` |
| DCP-V1-040 | `TestIndexedCorpusReproductionAndProvenance`, `TestIndexedCorpusCapacityQualification` |
| DCP-V1-041 | `TestSwitchParityWeakMultiplicity`, `TestSwitchParityStableTypedIdentities`, `TestSwitchParityRepeatedStaleJourneys`, `TestIndexedCorpusSwitchParity` |
| DCP-V1-042 | `TestHostedCorpusHTTPConformance`, `TestHostedCorpusConcurrencyAndCleanup` |

The new multiplicity matrix covers addition/removal, empty input, self comparison, permutation,
mixed weak contents, weak content changes and strong content changes. Duplicate top-level IDs,
dependency IDs and observation IDs remain refused. The before-fix failure is retained outside
tracked source. Gate A found no HIGH or MED concerns in this bounded public port and repair.
Public compile, the repaired parity vectors, typed and legacy CLI invocation tests, and vet passed.
The complete changed-package suite passed, including doccorpus, corpusbridge, corpusindex,
corpusserve and the corpus command packages. Independent review of the public source and spec
returned PASS with no actionable correctness findings; no public-slice repair cycle was needed.
Shared binding and integration remain outside that source review.

## Qualification, evidence and rollback

The existing corpus profiles remain proposed/experimental. Local JSON-over-HTTP tests do not
qualify remote hosting, TLS, authentication or MCP Streamable HTTP. Imported trust and retirement
policy remain caller declarations; absent evidence cannot establish authority. No new hosted
campaign, provider support, broad corpus redesign or repository-wide gate is included.

Fresh public context and affected plans were retained before edits. The new worktree has its own
local enrollment; the old leaf's satisfaction is not reused as public proof. Shared requirements
and CEM paths were explicitly excluded from source admission while another task holds them, so
initial tracked dogfood-change and final registry/CEM/OCM/native gates remain deferred until
compatible shared admission. Original pre-change packets and that delayed-binding reason remain
visible. Publication, integration and native ticket completion are not established by this entry.

Rollback is to leave this isolated public candidate unpromoted or revert its task-only source.
The original prototype and its sealed branch remain recoverable. No live connector writes occur.
