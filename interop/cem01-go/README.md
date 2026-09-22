# CEM 0.1 Go portability probe

This directory contains a Corvint-authored, clean-room Go consumer for the public CEM 0.1
interchange contract. It uses only the Go standard library and the Git executable. It does not
import, link, translate, inspect, or subprocess Corvint's Python implementation.

The probe answers one question: can the published adapter, algorithms, manifest, and raw fixtures
produce the same deterministic decisions in a second runtime? It is reference portability evidence,
not an independently authored implementation and not a P1, C1, or C2 matrix claim.

## Verify

```console
$ gofmt -d *.go
$ go test -count=1 ./...
$ go vet ./...
```

The process-boundary test checks all 51 manifest artifact digests, reconstructs the exact SHA-1 and
SHA-256 repositories, and requires the published outcomes for 7 valid cases, 19 invalid cases, 6
drift cases covering all 5 drift states, and all 5 producer expected maps. Focused tests cover strict
JSON and mathematical wire integers, Unicode and canonical identities, bounded regular inputs, patch
edge cases, empty blobs, overlapping drift, Git isolation, deadline enforcement, and batched
capacity.

`TestPortableProfileCompatibility` additionally consumes the candidate raw packet in
`protocol/cem-0.2` through this consumer's process ABI: it rejects 0.2 and checks separately pinned
0.1 equivalents, including ordered mixed drift, unknowns and evidence reuse across hunks. Repeated
basis pairs remain invalid within one hunk. This supplement changes neither the historical frozen
matrix nor the consumer's 0.1-only profile; it does not establish independent 0.2 interoperability.

## Run

```console
$ go run . verify --repository /absolute/repository \
    --map /absolute/change.cem.json --patch /absolute/change.patch
$ go run . verify --repository /absolute/repository \
    --map /absolute/change.cem.json --patch /absolute/change.patch \
    --target FULL_COMMIT_OID
```

Exit 0 means accepted, exit 1 means invalid or unsafe drift, and exit 2 means invocation or
operational failure. Standard output is exactly one JSON protocol object; standard error is empty.
Verification proves structural integrity and drift status, never semantic support or correctness.

The implementation was produced from `interop/cem-0.1/ADAPTER.md`, `ALGORITHMS.md`,
`manifest.json`, and the digest-pinned public fixtures. The builder did not read `src/**`,
`tests/**`, Python source, or repository history. A separate adversarial reviewer initially found
eight contract defects; the repaired snapshot passed re-review with no remaining HIGH, MEDIUM, or
LOW findings.

This interoperability implementation is licensed under plain Apache-2.0 as listed in
`../../LICENSING.md`. Publication still requires the repository's release-integrity and
product-evidence gates.
