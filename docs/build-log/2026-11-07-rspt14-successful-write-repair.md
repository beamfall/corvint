# Successful mutation dependencies and retained read-surface blocker

Date: 2026-11-07. Self-review repair of lane rspt14, based on
`9761ea1f459c0d3b3c963953e24a76eaeda16370`; repair starts at
`4851c81ee42c52a3eaee038268a9856a281c597b`.

The first P1 closes dependency loss from successful mutation operations, rather than
adding a syscall exception. Under RSP-V0-007/021, a successful open retains its path
regardless of access mode, unless the supported flag model proves exclusive creation.
The common write recorder retains dependencies for every mutation by default. Its
closed ownership allowlist is proven `create` or `replace`; unknown write kinds cannot
establish ownership even with a claimed proof. Previously proven tree-owned paths
retain their existing suppression. Unsupported open forms remain incomplete under
RSP-V0-022. No wire, error code, existing assertion or golden changes.

`TestRSPV0021CapturedWriteSuccessDependency` covers seven native probes without
subsequent reads: write-only, append, truncate and nonexclusive-create opens, creat,
truncate, and an unknown open flag. Complete captures exclude an unrelated edit but
select fixture deletion and rename by RSP-V0-012(f). Renaming the fixture changes the
actual success assertion for operations that cannot create it. Unknown flags retain
selection even for an unrelated edit. `TestRSPV0007WriteDependencyDefault` covers
creation proof, later tree-owned truncation and an unknown write kind claiming proof.
The new capture regression fails on all six understood forms with pre-repair sources.
The complete repaired runnerselection package passes, including the unknown-form case.

The full proctrace package fails seven existing top-level tests (15 failure lines
including subtests), all with `undeclared-outside-read`. A Go overlay containing the
original HEAD versions of the two changed implementation files, and omitting only the
new test, reproduces exactly that failing test set. The earlier PT_INTERP walker
records the host's `/lib64` symlink as an undeclared outside read. Existing assertions
are preserved. This is an inherited, separately scoped integration failure, not a
successful package gate; the repair charter permits only its two listed findings.

V1-5116 / RSP-V0-051 remains PARTIAL. Reinspection confirms that `linecov.Index`
contains records but no stored runner inventory or admitted qualification/carry-forward
artifact; its gob encoder serializes that struct as `linecov-index/3`.
`testrunner.RunnerInventory` explicitly is not a wire format. `runnerselection.Runner`
accepts caller-supplied admission, and `affectedRunnerShadow` supplies declarations
only. No persisted-context reader or admission producer is present. The named
`internal/runnerqualification` integration is absent in this checkout. The repair's
frozen-contract rule forbids inventing the missing storage fields/shape. A record
cannot establish its own expected identity or admission, and an always-UNKNOWN command
would not deliver the requested positive end-to-end surface. No such placeholder is
shipped. The external prerequisite is an approved persisted-context/admission contract
and its producer/reader integration. Command registration, query goldens, negative
cases, MCP/feature-route exposure, AFU projection and no-write acceptance evidence
remain NOT_PRODUCED; no completion or production narrowing is claimed.

The existing pre-change capture is reused per charter. The applicable affected route
was used; the installed binary reports itself stale. Its plan selects both touched
packages and retains scope UNKNOWN (95 unknowns: 74 LANGUAGE_FRONTIER,
20 UNOWNED_DIRTY_PATH, one NO_SELECTABLE_TEST). Whole-repository gates, independent
review and other runtime qualification belong to the orchestrator. No runtime,
provider, publication or task-store write route was exercised. Rollback is reverting
this repair; it restores the known successful-operation dependency omission.

The required spec requirements and line-citation checks pass. The repin command
found no moved citation. Go reports version go1.27.3. Final CEM binding/check results
and any containment refusal are retained in the external handback; the local outcome
is blocked, not passed. No seal is requested.

Raw evidence lives outside Git under `/home/russell/lanes/rspt14-evidence/self-review/`.
SHA-256 digests bind the observations above:

| Artifact | SHA-256 | Result |
| --- | --- | --- |
| affected.json | 9e5c2f43d7d0d3536cd553e653961300691b94582f4a72bfceb9503b67afaeef | Both packages selected; UNKNOWN |
| proctrace-test.log | f77dc0ea01e82f23ef4f98d7151f5f727ff6f539bfea3dc3089705d690bd41ad | Full package FAIL |
| proctrace-before-test.log | 8d0fb9b29717598133cb786bf8890555f44f6d90ab7afa7be9f567169443c317 | Same inherited failures |
| runnerselection-test.log | 7d71daa7965090fbb84877d06db19c8fe484b3f006cd4897aeb57c3f1fc57022 | Full package PASS |
| runnerselection-before-regression.log | 41aa229fc27bebaaf7103c7eb33578fc4cdb36c32378805257db174837111967 | Six positive-form regressions FAIL before repair |
