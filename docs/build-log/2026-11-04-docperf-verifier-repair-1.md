# Docperf verifier repair 1: pin the frozen fixture's identity

## Finding and decision

The host verifier at `f48990f50ff5579ef3e2c27bfb16aaf63e639c91` still failed
`TestContextDefaultWireIsTheGolden` and
`TestContextLSPOffKeepsTheGoldenAndOnDegrades`. The returned commit was
`05d3142624a106eafdac3a4f644b6f44eacaf3eb`; the unchanged golden requires
`9ab69ef47d196e1a757a3de3319527eb3686ff8b`. The previous repair's external
environment change did not cover the host verifier's exported Git identity.
The diagnosis and exact reconstruction remain in
[the earlier repair record](2026-11-04-docperf-repair.md).

Decision: pin the existing fixture identity (`t <t@x>`) for both author and
committer in `gitFixture`'s subprocess environment, alongside its existing
pinned dates. Explicit Git identity variables override the fixture's
`user.name` and `user.email` configuration. Pinning them at the subprocess
boundary also leaves parallel tests and the lane's commit identity independent.
No assertions, fixture files, goldens, product behavior, wire contracts, error
codes, stderr or core-freeze output change. TCP-V0-024's existing default-wire
compatibility assertion and TCP-V0-150's commit identity remain the governing
contracts. No spec amendment or new requirement is needed for this fixture
repair. Rollback removes only the identity pin and restores the environment
dependence; it does not change the golden.

## Verification and limits

Pre-change `make dogfood-change` ran with the required Russell Lewis identity.
It retained context, but returned missing citation/intent/outcome inputs; those
are supplied at final binding. Query and impact receipt binding remained
`NOT_OBSERVED` (`agent-receipt-binding-mismatch`). The context route was used
for the two failures and TCP-V0-024/TCP-V0-150. The changed-tree affected plan
selects `cmd/corvint` with `DIRECT_TEST_CHANGE`: 118 units selected, 504
excluded, with `LANGUAGE_FRONTIER` unknowns for build-constraint variants and
unmodelled shell invocation. Retrieval/learning evaluations, provider routes,
mutation and performance requalification do not apply to a fixture-only repair.

With all four Russell identity variables exported, the exact failing command
`script/heavy.sh go test -count=1 -run 'Freeze|GPKV0002|Golden' ./cmd/corvint/`
passes (2.460 s package time). Existing byte-for-byte assertions remain intact.
`script/heavy.sh --admit-only make windows-vet` also passes with the writable
cache (58.6 s wrapper time); this is type-checking evidence, not Windows runtime
qualification. The freeze log SHA-256 is
`72e3e1890450642b6d9d65fc0cfa1c5c1510301c3f25ef241e4e6438f5dc2bc7`;
the Windows vet log SHA-256 is
`83b1c61a5fac811596b7c0b1d0ff32d37a2347f3b661a5d4fa3dca29f1a6b2f9`.

The required whole-package command,
`script/heavy.sh go test -count=1 -timeout 30m ./cmd/corvint/`, ran without a
filter and failed (507.561 s package, 514.8 s wrapper; 47 top-level failed tests).
It used the exported Russell identity as the freeze check did. Retain this as
a failed check, not equivalent whole-package coverage. Many failures explicitly
name the mandated scratch environment: `/home/russell/lanes` is mode 0775, so
work-source and executable checks refuse its descendants; `/home/russell/lanes/.git`
also makes nominal non-repository fixtures and profile outputs lie inside a Git
worktree. The whole run additionally finds identity-sensitive genesis receipt
goldens. A focused diagnostic with identity variables unset runs
`TestCoreVerbsEmitTheFrozenProfiles` successfully, but
`TestMER099SourceRootUnavailableOutsideGit` still fails a candidate-digest
check; its cause remains unassessed in this repair. The full failed-test names
and output are retained, rather than claiming every failure shares one cause.
No unrelated fixture, product security check, shared directory mode, enclosing
Git metadata, assertion or golden is changed. A host-side whole-package rerun
in admitted scratch conditions is still owed. Whole CLI log SHA-256:
`a4a63c7f156acc520d663b4d558153dc824815607698f81d2a673606727d8159`.

The host verifier also passed whole-package tests for `internal/contextindex`
(95.473 s), `internal/doccorpus` (156.382 s) and `internal/mcp/corpusbridge`
(40.780 s), plus doc-gates and Windows vet. Those source trees are unchanged
by this repair; their evidence is retained rather than rerun.

Raw logs and inputs are outside the repository under
`/home/russell/lanes/docperf-evidence/verifier-repair-1/`. The host verifier log
SHA-256 is `1be83773a10d223e2289428ed99931956223622a070d2023d009640d8e1e2b6c`.
The handback's "Verifier repair 1" section retains the whole CLI package and
Windows vet results, final binding/check outcome, evidence digests and friction.
The original per-ticket performance qualifications and native completion limits
remain open. Independent review is orchestrator-owned. No model-backed review,
Tasks write, publication, background job or seal is authorized for this round.
