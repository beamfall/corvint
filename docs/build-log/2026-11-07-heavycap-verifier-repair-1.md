# Heavycap verifier repair 1

Date: 2026-11-07. Base: `cb403735903b1e54a8469373489960c0b2b89d99`.
Repair input: host verifier failure at `d530095f6477198d1a990f8237bd0fed2fac1a3f`.

The host verifier found two integration omissions in the heavycap change. Under TSP-V0-005,
the heavy runtime fixture tests are callers of the runner, while the selection replay source's
runtime-cap comment is only a mention. Both now appear in the existing declaration, with their
classification explained. Its schema and the check's completeness assertions are unchanged.

Under ASD-V0-929, the two replay executable fixtures (V1-4722 / DTL-V0-017) and the processor-count
fixture (V1-5040 / ASD-V0-2477) now use the existing `execfile.WriteFile` helper. Holding the fork
lock during each executable write prevents concurrent fixture processes from inheriting its open
write descriptor. Fixture bytes, permissions and test assertions are unchanged. The shared test
file also retains the V1-5007 / ASD-V0-2478 coverage without alteration. V1-4944 / MCJ-V0-025 code
and evidence are unchanged by this repair.

The repair adds no functionality or requirement definition and changes no wire contract, golden,
error code or core-freeze output. Rollback is a scoped revert of these declaration and fixture
writer edits; that would restore the verifier failures. The original V1-5007 external pre-commit
integration blocker remains PARTIAL. Raw repair logs are retained outside the repository under
`/home/russell/lanes/heavycap-evidence/`; the repair handback records commands, results and digests.

Independent review and broad host verification remain assigned to the orchestrator. No new
pre-change capture, Tasks write, publication or seal is part of this repair.
