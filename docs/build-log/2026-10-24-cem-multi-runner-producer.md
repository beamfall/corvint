# 2026-10-24 V1-2322 part 1: multi-profile runner receipts in the plain producer

On a WSL2 scratch clone of `origin/main` `025120e1a`, `corvint-cem-candidate assemble-stable`
with a `cem-stable-plain-assembly/1` request accepted exactly one `runnerPlan`/`runnerReceipt`.
Two receipts in one request were refused (`unknown field "runnerPlans"`, or `duplicate object key
"runnerPlan"`). A `go-test` receipt and a `node-test` receipt each gave native ACCEPT only in
separate maps. The reader schema already allows 64 `runnerReceipts`, so the limit sat in the
producer alone, and the spec stated it as a non-goal of `CEM-V1-240`.

The new requirements are `CEM-V1-573..578` in `docs/specs/cem-stable-v1.md`, section
"Multi-profile runner receipts in the plain producer". They are accepted under the owner's standing
auto-accept of lane requirements (2026-10-11) and the CEM-V1 auto-accept (2026-10-20). IDs 569..572
were reserved by open PR #1651 when these were numbered.

## Decisions

- **A `runners` member beside the single form, not a new profile.** The closed decoder accepts
  either the top-level pair or `runners`, never both (`runner-set-form`). The single form keeps its
  fixed artifact paths and labels, so its output is byte-identical (`CEM-V1-574`). The proof is the
  admitted single-form map digest recorded on the base commit, plus the existing opaque
  `neutral-criterion` byte comparison.
- **Each criterion names its receipts.** In the `runners` form each criterion carries
  `runnerReceiptSha256s`. Its link names exactly those receipts. Every pair receipt must cover at
  least one criterion (`runner-set-unused`), so the map has no unlinked receipt, which the reader
  would refuse anyway (`reference-coherence`).
- **Admission is required for `runners`.** An opaque multi-receipt map would let a caller pin any
  set of files as runner receipts. With admission, each pair passes `CEM-V1-281..285`, and the
  runner profile each receipt carries is known. That allows `runner-profile-conflict`: two receipts
  of the same runner covering one criterion are refused, while `go-test` plus `node-test` on one
  criterion is accepted.
- **The Tasks-bound producer is unchanged (`CEM-V1-578`).** The lane brief asked for both
  producers. Reading `cem-stable-assembly/1` shows it writes no map: its source check refuses every
  source, so it has no runner set to widen. A `runners` member there stays an unknown field.
- **Fixture under `interop/cem01-go/testdata/multi-runner/`, not under `protocol/cem-1.0/`.** No
  sealed packet or stable manifest entry changes, so there is no repin. The runner plans and
  receipts are synthetic documents in the closed `corvint-test-runner` shapes, built over the
  recipe repository's blobs. They were not produced by running `go test` or `node --test`.
- **The shared tamper matrix does not apply.** It covers patch and evidence classes. The
  six receipt tampers live in the case's `EXPECTED.json`, and both readers check them.

## Independent review

One Codex read-only round found three issues, all confirmed and fixed:
- The per-criterion receipt list allowed 64 digests, but the reader bounds each link array at 32,
  so 33 to 64 would fail in the encoder after admission. The bound is now 32.
- A `null` `runners` or `runnerReceiptSha256s` member decoded as absent, and an empty top-level
  pin in a `runners` request compared equal to no pin, so a request naming both forms got past
  `runner-set-form`. `PlainRequest` and `PlainCriterion` now record which members their JSON names.
  The single-form pins use `omitzero`, so a Go-built `runners` request still marshals without them.
  The closed decoder treats `omitzero` as optional, as it treats `omitempty`.
- `TestCEMV1575NamedMembersOfTheOtherFormAreRefused` covers these cases from request JSON. With the
  request-level presence checks and the 32 bound reverted, three of its four cases fail.

## Reader limit found

A reader does not parse runner documents. It checks that each receipt record's `planSha256` names a
pinned `runner-plan` artifact and that every artifact matches its digest. If a request swaps the
plans between two pairs, an unadmitted producer would record each receipt with the other pair's
plan digest, and that map would still be self-consistent for a reader. The producer's admission
refuses the swap as `runner-receipt-tampered`, which is one reason the `runners` form requires
admission. Swapping only the artifact files after production is caught by both readers
(`artifact-digest-mismatch`). The spec records this as a non-goal.

## Verification (Darwin arm64, go1.27.2)

`corvint affected --base 025120e1a7f14bce336ebe401ef6f2f99e8a3cb1` selected tier 0
`internal/cemcandidate` and `internal/cem/verify`, and tier 1 `cmd/corvint-cem-candidate`,
`internal/cem/cli` and `internal/cemchain`. All of them passed, as did `internal/specindex`,
`tools/test-read-scopes-check` and `interop/cem01-go` (`go test` and `go vet` from its own
directory). The 85 tier-2 packages were not run locally; CI runs them.

Dogfood: `NOT_PRODUCED`. The spec has more than 256 requirements, which blocks `dogfood-change`
(V1-2525).

## Remaining

The native Tasks lifecycle for a multi-runner map is later V1-2322 work: the `cem-verdict`
closeout over a two-profile map, and then the drift follow-up.
