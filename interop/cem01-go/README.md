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

`ci` is the portable CI verifier (`CEM-PILOT-020`..`023`): it derives the patch from two declared
commits, reads the map from the head tree, and prints one `cem-ci-report/0` line with exit 0 to 5.
It leaves `verify` unchanged. The pinned, network-denied runbook is `examples/cem/README.md`.

```console
$ go run . ci --repository /absolute/repository --base BASE_OID --head HEAD_OID \
    --map .corvint/change.cem.json
```

The implementation was produced from `interop/cem-0.1/ADAPTER.md`, `ALGORITHMS.md`,
`manifest.json`, and the digest-pinned public fixtures. The builder did not read `src/**`,
`tests/**`, Python source, or repository history. A separate adversarial reviewer initially found
eight contract defects; the repaired snapshot passed re-review with no remaining HIGH, MEDIUM, or
LOW findings.

This interoperability implementation is licensed under plain Apache-2.0 as listed in
`../../LICENSING.md`. Publication still requires the repository's release-integrity and
product-evidence gates.

### Experimental CEM 1.0 candidate

The additive `verify-candidate` command implements the public
[`protocol/cem-1.0/ALGORITHMS.md`](../../protocol/cem-1.0/ALGORITHMS.md)
contract. It does not change `verify` or `ci`, and historical interfaces still
refuse candidate maps. Building now requires Go 1.24 or later for rooted artifact
reads; the consumer remains standard-library-only.

```
cem01-go verify-candidate --repository /absolute/repo --map candidate.json \
  --expected-base FULL_BASE_OID --target FULL_TARGET_OID --artifacts /absolute/artifacts
```

Exit zero means canonical change integrity plus opaque referenced-byte integrity.
It never means a criterion passed or a native Tasks/runner record was authoritative.
Every output includes `REFERENCE_INTEGRITY_ONLY` and eight `NOT_OBSERVED` limits.
The packet deliberately includes an illustrative failed infrastructure receipt;
its intact bytes cannot become a semantic success claim.

The portable candidate path independently self-hashes commit, tree and required
blob objects, inventories both trees, derives the canonical diff, replays it into
verified target bytes, verifies evidence identity and drift, and checks artifact
bytes before and after Git verification. It keeps the candidate spec unchanged
when reusing format-neutral evidence and patch mechanics.

This reference path admits only primary SHA-1/SHA-256 repositories with a regular
`.git` directory, regular tracked files, and a narrow local configuration containing
only ordinary core repository settings and the object-format extension. Linked
worktrees, shallow repositories, alternates, grafts, info attributes, config
includes, remotes and other custom config sections, symlinks and gitlinks refuse.
Trees are limited to depth 32 and 4096 files; inherited object/patch/time limits
also apply. This is an operational portability profile, not qualification for all
native repository forms or a hostile same-user filesystem sandbox. Native runtime
schema validation, source attestation, 0.3 witness/structural capability migration,
independently authored external consumers and outcome evaluation remain outside
this candidate. Stable CEM 1.0 remains blocked on those qualifications.

`TestCandidateNormativePacket` reconstructs the new frozen SHA-1 and SHA-256
repositories and checks all 31 cases through the executable, separately from the
unchanged historical manifests. The packet and both consumers are Corvint-authored
reference portability evidence, not independent external interoperability evidence.
