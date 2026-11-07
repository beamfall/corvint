# clicontract verifier repair 1

Owner intent: repair only the four checks listed in Verifier repair 1, against
`d436dff76de1b8e75c427069ae7f80003ae99bc2`, preserving native wire contracts,
goldens and existing assertions. The orchestrator retains independent review,
integration and ticket completion. No publication or seal is authorized.

## Decisions and delivered repair

- CAP-V0-7700 (V1-2685): the isolated Work production seed now copies the local
  dependency closure's compiler-reported `EmbedFiles`. The new offline catalog
  dependency needs its Markdown guide and nested tree JSON files in addition to
  Go sources. Existing Work assertions remain intact.
- CAP-V0-7706 (V1-3277): render the catalog's relative Markdown link with
  `path.Join`. Its bytes remain unchanged; the emitted link is not a runtime
  filesystem read. The unbounded-reader checker needs no exemption or new scope
  declaration. Its check and the full `internal/clitree` suite pass.
- CIN-V0-2304 / ASD-V0-036: retain CI's inline checker byte-for-byte and
  recognize its exact normalized body in the local workflow mirror. The mirror
  runs its source-owned canonical body only after equality succeeds. A changed
  build, appended shell syntax or an unlisted script remains unknown. Keep CI's
  trusted-base build, separate PR-head checkout, merge-base binding and failure
  propagation inside the protected control-plane boundary. A Makefile wrapper
  would leave that boundary; extracting a script would break the unchanged
  `TestCINV02304BatchPullRequestNeedsSealedEvidence` inline-step assertion.
  Neither alternative is retained, and no `.github/` changes remain. New shell
  witnesses check exact matching, rejection of body drift and appended syntax,
  and retention of the next workflow command.
- UCV0-018: regenerate the three stale implementation receipt pins and ledger
  with `script/repin-use-case-receipts.sh`. Their shapes and result fields remain
  unchanged. The receipt check and full `conformance/use-cases-v0` tests pass.

## Frozen compatibility boundary

The eight failing shipped-help cases require replacing native help/refusal
bytes on four isolated analyzers and four folded shims. The owner's hard rule
forbids that change. Retain the assertions and the scope trim's native entrypoints;
V1-2685 remains PARTIAL rather than claiming universal shipped-help acceptance.
V1-3738 and V1-3736 keep their previously recorded partial dispositions.

The host Core goldens also inherited repository author/committer variables,
overriding the fixtures' configured identities and changing their commit IDs.
The fixture helpers now isolate those four variables inside their disposable
Git processes, retaining their originally configured identities. All repository
commits and dogfood binds use the required exported Russell Lewis identity.
No golden or frozen assertion is edited. An initial whole-package run also
confirmed the diagnosis by unsetting those variables in its test process;
the final rerun keeps the seal identity exported.

## Evidence and limits

Raw evidence is outside the repository at
`/home/russell/lanes/clicontract-evidence/verifier-repair-1/`; the handback retains
the final commands, results and digests. The initial affected plan ran before
tests. Corvint context and dogfood's pre-change query/impact routes were used;
the installed affected binary reported its stale BASE build explicitly.
Optional LSP, ranking/learning evaluations, mutation, live platform qualification
and new capability generation are not applicable to these repairs. Existing
catalog outputs are preserved. The orchestrator owns independent review; no
model-backed reviewer is run here. CEM structural acceptance does not turn a
retained failed test into product acceptance. Rollback is to revert this repair
commit, restoring the previous implementation and receipt pins together.

Whole-package results retained from this round (all Go tests use the heavy
wrapper, one selected package per command, without `-run`):

| Artifact | Result | SHA-256 |
| --- | --- | --- |
| `affected.json` | rc=0; 256 selected, 90 unknown; scope UNKNOWN | `d4bd5c0203605f5935686c30c8fe5361ca0cf91503321b12aab48cfcfcca8ed1` |
| `clitree.log` | rc=0; full existing package | `7f0f7c651fab3d5a7e15f6c5e16b872fbbd12f3251943821c93bf6632d593869` |
| `use-cases.log` | rc=0; full existing package | `fcfee9d486aeda114e4f5bbcc0de910026724ec0abb001fca21f142a2c77cb5e` |
| `batch-precheck-checked.log` | rc=0; complete existing shell suite plus new drift witnesses | `c4e5fc52282e3354e95ac77db6ca84d16eff365ea6e0bf6bb18111c7ab1a94f5` |
| `frozen-identity.log` | rc=0; the three reported golden tests with the seal identity exported | `7f10514891715f74fa05dbca9a4625d7840a00b7cdef42d955e5dc70be6534f9` |
| `doc-gates-checked.log` | rc=0; full doc gates, 367.5s wrapper run | `d9e1333b18541eaa0ab12e1f61d31abeee2e7d4ca332fd373c5f77ba7a6fd01f` |
| `cli-errors.log` | rc=1; 8 frozen help subcases | `6070d5af0f1bc20d71410523f48c8985980e1605c5c8b12db1635682661fdb0b` |
| `corvint.log` | rc=1; 46 top-level failures; 577.0s wrapper run | `a1be62b52a443aae7aebcfc6b82a7a506f8de2d94a9d228c9827f61328b88829` |
| `corvint-checked.log` | rc=1; final full package with exported seal identity; 46 top-level failures; 488.375s package / 504.7s wrapper | `0cedd02b8d2ef103cca3985729a576940b74ae7498a8e968f8b10cf77ee59c5f` |

The focused identity replay covers `TestContextLSPOffKeepsTheGoldenAndOnDegrades`,
`TestContextDefaultWireIsTheGolden` (TCP-V0-150) and
`TestCoreVerbsEmitTheFrozenProfiles` (CCF-V1-002). It supplements the required
whole-package rerun; it does not replace it. The initial `corvint.log` is the
environment diagnosis before fixture identity isolation; the final whole-package
result is retained separately as `corvint-checked.log` in the handback.

The Core failures report the mandated scratch path's group-writable
`/home/russell/lanes` ancestor or discovery of that parent as a repository.
No missing embed-file compilation failure or default-wire/Core profile golden
drift remains in this run. Work source and execution qualification still fail
before their parity assertions, so these observations do not establish Work
acceptance. Do not weaken the source, path, profile or executable checks; rerun
the full package on the host with its allowed safe scratch environment.
