# CEM 0.1 Go portability probe

Owner: Russell Lewis
Created: 2026-08-23
Intent status: proposed
Delivery status: experimental
Authoritative inputs: `docs/CHANGE-EVIDENCE-MAP.md`, `docs/specs/cem-pilot-kit.md`, and
`interop/cem-0.1/ADAPTER.md`, `ALGORITHMS.md`, and `manifest.json`

## Agent digest
- Claim: The Corvint-authored Go consumer tests CEM 0.1 portability but cannot satisfy independent interoperability or adoption gates.
- Status: proposed/experimental
- Exists: `interop/cem01-go`, passing the frozen 7/19/6 matrix and five producer maps.
- Blocked on: independent external adoption evidence.
- Read next: `cem-pilot-kit.md` and `cem-external-interop-v0.md`.

## User and job

A non-Python implementer must be able to build a CEM 0.1 consumer from the public contract and raw
fixtures alone, without importing, translating, linking, or invoking Corvint. This probe tests whether
the interchange contract is portable and sufficiently precise before asking an external project to
spend its time on it.

This is a Corvint-authored reference portability probe. It cannot fill the independent P1, C1, or C2
matrix rows and cannot establish external demand, semantic evidence quality, or product value.
It MUST NOT count toward CEM 0.1's independent-implementation gate because the same organization
authored both this probe and the reference implementation.

## Verified starting state

- `interop/cem-0.1` contains digest-pinned raw inputs, expected outcomes, algorithms, and an adapter
  process contract.
- Corvint's Python verifier and Corvint-authored conformance tests pass the frozen kit.
- The public kit contains no executable consumer or language-neutral runner.
- No independently authored producer or consumer has been demonstrated.

## Requirements

- `CEM-GO-001`: the probe MUST be a standalone Go command using only the Go standard library and
  the Git executable. It MUST NOT import, link, translate, copy, inspect, or subprocess Corvint
  implementation code. Its source lives outside the raw `interop/cem-0.1` kit.
- `CEM-GO-002`: it MUST implement the exact adapter invocation, JSON envelope, drift ordering, and
  exit-code contract in `ADAPTER.md` for `verify --repository DIR --map FILE --patch FILE
  [--target FULL_OID]`.
- `CEM-GO-003`: it MUST independently implement every normative consumer algorithm and resource
  bound in `ALGORITHMS.md`, including strict JSON, canonical identities, patch parsing and base
  simulation, SHA-1/SHA-256 Git objects, evidence and disposition checks, and same-path drift.
- `CEM-GO-004`: a process-boundary runner MUST digest-check every selected artifact, reconstruct the
  declared SHA-1 and SHA-256 repositories, and require the published result for all 7 valid, 19
  invalid, and 6 drift cases. It MUST also verify, but not produce, all 5 producer-job expected maps.
- `CEM-GO-005`: repository bytes and paths are untrusted. Reads and subprocess output MUST be
  bounded; Git invocation MUST disable ambient configuration, replacement objects, alternates,
  hooks, filters, text conversion, credentials, prompts, and lazy network fetches; unsupported
  operational conditions MUST exit 2 rather than accept or reject a conformance case.
- `CEM-GO-006`: the implementation matrix and result must label this probe as Corvint-authored
  reference portability evidence. P1, C1, and C2 MUST remain unclaimed.

## Non-goals and simpler baseline

The baseline is the existing raw kit plus Corvint's Python reference attestation. This slice does not
add a producer, library SDK, package distribution, CEM 0.2 support, report renderer, policy engine,
custom Git object reader, CI service, database, network service, or external-adoption claim.

## Failure, rollout, and rollback

Malformed or ambiguous inputs fail closed. Invocation, repository I/O, unavailable SHA-256 support,
or a documented lower operational bound exits unsupported. The probe remains local and
experimental until independent interoperability and publication gates pass.

Rollback deletes `interop/cem01-go` and its Corvint-authored matrix row; no map or repository data
migration is required. A discovered ambiguity is recorded against the public contract and repaired
there rather than hidden in consumer-specific behavior.

## Acceptance and kill criteria

Acceptance requires `gofmt`, `go vet ./...`, `go test ./...`, the complete process-boundary matrix,
the Corvint Python interop regressions, and an independent adversarial review. Exact case counts and
artifact digests come from the frozen manifest, not duplicated constants.

Kill or redesign the contract if a cold implementer needs Corvint source, cannot determine one
published result from the public artifacts, needs more than one engineer-day, or if the complete
runner cannot distinguish the Go consumer from a deliberately corrupted implementation. Passing
proves internal portability only; the independent-consumer and product-value gates remain open.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| CEM-GO-001..003 | `interop/cem01-go` | clean-room implementation record; Go unit tests and vet, including `TestSimilarityMetadataIsExact` (only the exact `similarity index N%` / `dissimilarity index N%` record is metadata) and `TestBinaryMarkerTextInHunkPayloadIsText` (a binary marker is a whole patch record, never prefixed hunk payload text); erratum 2 (decision 0192): `TestDissimilarityWithoutRenameParses`, `TestInconsistentNewPositionRejectedAtParse` |
| CEM-GO-004 | `interop/cem01-go` process-boundary runner | frozen 7/19/6 matrix plus five expected producer maps, including erratum-1 vector `deleted-symlink-changed` |
| CEM-GO-005 | bounded I/O and sanitized Git process boundary | hostile environment, resource, timeout, and repository tests; `TestEndToEndVerify` exits 2 (`object-alternates`) on a repository with `objects/info/alternates`, before any object read |
| CEM-GO-006 | `interop/cem-0.1/IMPLEMENTATIONS.md` | independent rows remain empty; review confirms non-claim |

## Open evidence

- the Corvint-authored clean-room Go consumer passes all 51 artifact digests, 7 valid cases, 19 invalid
  cases, 6 drift cases, and 5 producer expected maps across reconstructed SHA-1 and SHA-256
  repositories;
- `gofmt`, uncached Go tests, vet, Linux and Windows builds, Corvint's Python interop regressions, and
  repaired-snapshot adversarial review pass;
- the 2026-09-13 bug hunt repaired three probe defects found by differential testing against the
  Corvint verifier: loose similarity-index parsing, binary-marker substrings rejected inside hunk
  payload, and alternates not refused despite this spec's CEM-GO-005. Each repair follows the
  `ALGORITHMS.md` or CEM-GO-005 text, but its author had read Corvint's `internal/cem` parser, so
  those lines carry no clean-room provenance under CEM-GO-001;
- kit erratum 2 (decision 0192) changed the probe in two places. It accepts a `dissimilarity index`
  without a rename, and it rejects an inconsistent hunk new position at parse time. The same
  author also changed `internal/cem`, so these lines carry no clean-room provenance under
  CEM-GO-001 either;
- external authorship and adoption remain `NOT_RUN`;
- product value and token savings remain `NOT_RUN`.
