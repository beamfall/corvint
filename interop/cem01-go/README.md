# CEM 1.0 Go portable consumer

This directory contains a Corvint-authored, clean-room Go consumer for the public CEM 1.0 stable
contract in [`protocol/cem-1.0/stable`](../../protocol/cem-1.0/stable). It uses only the Go standard
library and the Git executable, and it does not import or link Corvint's product code. It is
reference portability evidence, not an independently authored implementation.

## Verify

```console
$ gofmt -d *.go
$ go test -count=1 ./...
$ go vet ./...
```

## Run

```console
$ go run . verify-stable-canonical --repository /absolute/repository \
    --map /absolute/change.cem.json --expected-base FULL_BASE_OID --target FULL_TARGET_OID \
    --artifacts /absolute/artifacts
```

`verify-stable-canonical` and `verify-stable` verify a `cem/1.0` map and print one
`cem-stable-verification/1` result. Verification proves change integrity and drift status, never
semantic support or correctness.

## Verdict ledger reader (experimental)

`cem01-go read-verdict-ledger --repository /absolute/repository --commit REV` reads the note of
REV under `refs/notes/corvint-verdicts`, the ledger the native `cem anchor --artifacts` writes. On
exit 0 it prints one JSON line with the members and values of the native `cem provenance
--verdicts` `verdicts` member: the same closed state and, only when the state is `verified`, the
same records. A refusal exits 2 with `{"ok":false,"code":CODE}`, using the native codes
(`invalid-arguments`, `repository-object-unavailable`). It writes nothing, carries no authority and
is not read by any verify route (`CEM-V1-754`, V1-2023).

## Retired cem/0.x routes

Decision 0525 retired the `cem/0.1` default `verify` route, `verify-03` and `ci` (V1-1278 S3,
`CEM-V1-222`). Any other invocation prints one JSON line and a fix line on standard error: a map
whose `spec` is a `cem/0.x` string is refused with the closed code `unsupported-spec` (exit 1), and
anything else is `invocation` (exit 2). The stable commands refuse a `cem/0.x` map with
`unsupported-spec`. Git history keeps the retired packets, routes and their evidence.

`testdata/inherited-vectors` keeps the inherited `cem/0.1` patch, base-tree and hunk-identity
vectors, moved unchanged from the retired CEM 0.1 interop packet, because the patch parser, LF
tokenizer and replay algorithms that `cem/1.0` restates are still tested against them. They are
algorithm vectors only; nothing admits a `cem/0.1` map.

This interoperability implementation is licensed under plain Apache-2.0 as listed in
`../../LICENSING.md`.

## Experimental CEM 1.0 candidate

The additive `verify-candidate` command implements the public
[`protocol/cem-1.0/ALGORITHMS.md`](../../protocol/cem-1.0/ALGORITHMS.md)
contract. Building requires Go 1.24 or later for rooted artifact reads; the consumer
remains standard-library-only.

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
repositories and checks all 31 cases through the executable. The packet and both consumers are Corvint-authored
reference portability evidence, not independent external interoperability evidence.
