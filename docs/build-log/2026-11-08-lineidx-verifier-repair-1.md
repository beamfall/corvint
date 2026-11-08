<!-- SPDX-License-Identifier: Apache-2.0 -->
# Lineidx verifier repair 1

The host verifier found an unowned qualification record (AZU-V0-002 / AZU-V0-8900)
and three unguarded executable fixture writes in the V1-4365 and V1-4366 tests.
The repair preserves the qualification record, wire contracts, goldens and assertions.

Decision: register only `.github/linecov/qualification.json` in the existing
`.corvint/test-selection.json` owners table (AFP-V0-2482), naming the ownership
completeness test and the workflow qualification test (GLS-V0-034 / GLS-V0-009).
This follows the adjacent CI qualification ownership declaration. The non-source
declaration rejects `.github/` paths; that prohibition stays intact. Ownership adds
test selection and preserves language-frontier uncertainty (AFP-V0-2488). The
allowlist added here is one exact path, not a directory or extension wildcard.

This finding is missing project registration, not an analysis soundness gap.
The unchanged `TestRepositoryOwnershipComplete_AZU_V0_8900` checks the reported
path as part of the complete tracked set and probes an unknown path. Existing
`TestDeclaredOwnerSelectsItsTests_AFPV02482` rejects unmatched paths and
`TestOwnerKeepsFrontierUnknowns_AFPV02488` retains frontier uncertainty.

All three executable writes in `TestLinecovMissConfirmationReproducesRecordedRunner`
(GLS-V0-046 / GLS-V0-009) and `TestWorkflowQualifiedInvocationMatchesWrapperRecord`
now use `execfile.WriteFile`. Its existing fork lock excludes concurrent forks
while a write descriptor is open. Fixture bytes, permissions and test assertions
are unchanged. No analysis allowlist or output contract was broadened.

Rollback is a revert of this repair; it restores the verifier failures, so it is
not a qualified delivery. Fresh production mutation qualification remains
NOT_PRODUCED; the source-stale qualification record still cannot enable narrowing.
Independent review and broad host gates belong to the orchestrator. No Tasks
writes, publication, new pre-change capture or seal were performed.

Verification: `corvint affected --base b75053a5b23cb0a4737a593d1fc1912aa3b79436`
selected all three checked packages; its installed binary reported stale source,
526 selected units and 79 unknowns. On Linux with go1.27.3, all existing tests ran
through `script/heavy.sh`, one whole package per command, with `-count=1 -timeout 30m`:

| Package | Result | Seconds |
| --- | --- | ---: |
| internal/liveverify/affected | PASS | 102.485 |
| tools/ci-reuse-plan | PASS | 14.037 |
| tools/linecov | PASS | 18.214 |

`make -s test-exec-writes-check line-citations-check` passed. Citation repinning
found no moved content. Raw logs stay in `~/lanes/lineidx-evidence/verifier-repair-1/`:

| Artifact | SHA-256 |
| --- | --- |
| affected-final.json | 3185741b45ad2500dd4bc0fc689ac147eb7b408bbcb3b9e0c0d9be81dfd24b33 |
| affected-test.log | 1a0c0a124fac27cd83dfc3af5d0182dfcf0d15866386e3ca472a93ceb50254ce |
| ci-reuse-plan-test.log | a86d5bf2f14b7a6e120f9cc56d941ac08596eb0d1ce5f4d11e8e045b4b52e3af |
| linecov-test.log | cd31ec085d2439e0a84d72083b7e58e7115626609c1e1d77ce02494d8474c864 |
| focused-checks.log | 855a4551b199f407e4e7cec705511746022a39aa0f70383fc13aac9acc748bba |
