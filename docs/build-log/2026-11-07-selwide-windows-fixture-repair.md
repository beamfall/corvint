# Selwide verifier repair 1: platform-scoped test fixture

The host verifier caught an undefined `fixtureHiddenConsumer` when Windows vet
compiled `package_context_test.go`. The fixture and its execution harness are
admitted only by `darwin || linux` in `main_test.go`; the new integration test
had been placed in an unconstrained file. This is a test build-boundary defect,
not a new selector soundness gap (V1-5018 / AFP-V0-9012, AFP-V0-054).

Move `TestRuntimeBuildContext_AFPV09012` unchanged to
`package_context_integration_test.go` with the same closed `darwin || linux`
platform allowlist as its fixture. Keep `TestRuntimeExclusionAllowlist_AFPV09012`
unconstrained, including its unknown-error case and all existing assertions.
Unknown platforms cannot admit a fixture-dependent test without its fixture.
The selector's closed exclusion allowlist and fail-closed behavior are unchanged.
No production behavior, frozen output, golden, or assertion changes.

Acceptance uses the whole `tools/corvint-pr-tests` package on Linux and the
unchanged `make windows-vet` target across both modules. `go list` confirms the
pure test remains selected on Windows and both tests remain selected on Darwin;
Darwin execution is NOT_RUN on this host. Independent review is host-owned.
Rollback is to revert this test-file split; that restores the observed Windows
compile failure and therefore cannot be promoted without another repair.

Raw logs are outside the repository under `~/lanes/selwide-evidence/` with prefix
`verifier-repair-1-`; final results and SHA-256 digests follow below.

Verification completed: whole-package Linux tests PASS; Windows vet PASS;
spec-requirements and line-citations checks PASS.

| Raw log suffix | Bytes | SHA-256 |
|---|---:|---|
| `package.log` | 599 | `993f6e590e875c44a93102c426bda74b06d67969ac508b88ce5ce4233c657756` |
| `windows-vet.log` | 673 | `2fd81550484f6dfff32b21f3906099b53dc318f54cf3b398b52e7be787efc624` |
| `doc-checks.log` | 1715 | `733ca85445dca97107bf50e0cac4f85da97d19d1a68de52387b9313c5cc6ceff` |
| `repin.log` | 190 | `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133` |
| `windows-files.log` | 42 | `5047c9d6e5148410790b0cb6ce61f3801b7037311054ae79683470452f463928` |
| `darwin-files.log` | 174 | `fade4a992804fa66fb62f50962c7e55d822846d6e5396897e1e4fd7214404c9f` |
