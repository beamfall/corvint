# CEM stable foundation on the sealed integration base

This slice integrates the reviewed 44-path stable foundation as a separate change on top of the sealed runner-foundation branch (`b465bdcb`), because the combined change would exceed the 200 changed-path per-slice and trace budget (the base slice alone changes 202 paths from `993fe629`). It stays experimental: it does not change a default, promote CEM 1.0, or complete V1-0632.

## Source and authority

The source is the frozen composition packet `foundation-composition-a5326ba6` (closed manifest `1ac13d54f21d0f81e38f24504a714bcaebd44b7506d02950dd82b88078873377`, patch `88355aa64b074920183cabd1d1eb8b6ec37b5ef15ef772c62b7cc0ff97fcc269`): the public stable contract under `protocol/cem-1.0/stable/`, the repaired native verifier and wire types, the independently authored portable consumer in `interop/cem01-go`, and the optional typed producer. The patch applied to this base without conflict or fuzz. One line differs from the packet: the `docs/specs/README.md` row keeps the indexed claim as its first clause, which the spec-index test requires. The S0E repository envelope, default dispatch, the historical proof harness and the Tasks subtree are not part of this slice.

The packet's root Gate A review (`55ea4445…`) and independent composition review (`13f0e59a…`, PASS_BOUNDED) were issued against source commit `a5326ba6`. They are retained as evidence for the packet content, not for this base.

## Verification on this base

Observed on this base, retained outside the repository as session evidence:

- Focused `go test -count=1 -timeout 30m` passed for `internal/cem/{cli,gitauth,gitrun,patch,publish,sim,verify,wire,workflow}`, `internal/cemcandidate`, `cmd/corvint-cem-candidate`, `internal/specindex` and `cmd/corvint`; `go vet` on the same packages returned 0. After the dispatch repair below, `internal/cem/cli` and `cmd/corvint` were rerun.
- `interop/cem01-go` passed on Go 1.27.1 and Go 1.24.13.
- The specification checks first failed on 18 line citations displaced by the patch and passed after 14 pinned citations were repinned as pure line shifts: 13 `cmd/corvint/main.go` citations (+3) across four specifications and one `interop/cem01-go/cem.go` citation (+7) in decision 0098. Anchors are unchanged.
- The affected plan reports scope UNKNOWN with 183 selected units and 43 unknowns; 171 selected units were not run. The repository-wide gate is NOT_RUN and this focused evidence is not equivalent coverage.
- The change map is `cem/0.2`: it binds hunks to intent and carries no execution evidence. The change check passed as ready for review with test execution NOT_RUN, 0 of 47 declared requirements linked, and the pre-change query and impact receipts NOT_OBSERVED.

Independent review of the integrated slice found that stable dispatch matched `cem verify-stable` at any argument position, so another command carrying those two words as values was answered with a stable refusal envelope. Dispatch now matches only in command position after optional leading global options, with a regression test. Producer historical runner execution remains NOT_RERUN and native authority NOT_OBSERVED, as in the packet.

## Open qualification and rollback

Seven native S0E lifecycle and metadata blockers, the consumer rollout S1 to S9, Linux and cross-device runs, and independent external interoperability remain open. Rollback is a revert of this isolated slice.
