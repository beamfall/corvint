# CEM 0.3 portable conformance packet

This Apache-2.0 packet is the proposed portable conformance packet for the `cem/0.3` profile
(`CEM-SM-011`..`CEM-SM-017`, decision 0521). Its status is `proposed`: it is not frozen, not owner
accepted as a stable profile, and not yet consumed by an independent external implementation.
It changes no `cem/0.1` or `cem/0.2` bytes. The 0.2 packet in `protocol/cem-0.2` stays frozen.

| File | Role |
|---|---|
| `ALGORITHMS.md` | Algorithm text: strict wire, sidecar distinction, structural predicates, witnesses, result stages. |
| `cem-0.3.schema.json` | Shape schema. Passing it is never sufficient; the algorithm text governs. |
| `OPERATION.json` | `verify-03` process ABI: argv, exit classes, stage order and code placement. |
| `WP1-OBSERVED.json` | The observed WP1 canonical-diff recipe the algorithm text names, SHA-256 `78df41faf9736520eb3b3724fc77abd88554ae243257d3b7c1d30e8f68e43630`. A byte copy of `protocol/cem-1.0/stable/inherited/WP1-OBSERVED.json`. |
| `vectors/manifest.json` | 221 cases, 3 repositories, 165 commits and the full pinned result for each case. |
| `vectors/maps/*.json` | One raw map per case, pinned by SHA-256 in `artifactSha256`. |

`vectors/manifest.json` is SHA-256
`9bb8393fba698fd5a65b502f49f5cf4bf4656bba09daa66ff817107224ca3931`, pinned separately by both
consumers. Each consumer must check the manifest digest and every artifact before execution.

## Reconstruction

Initialize each listed repository as an empty Git repository on `main` with its `objectFormat`
(two SHA-1 and one SHA-256). For each commit, write every file's exact bytes with
`hash-object -w --no-filters` and stage it with `update-index --cacheinfo mode,oid,path` into a
fresh temporary index. Bytes come from `content`, or from the pinned map named by `contentFile`.
A gitlink (`160000`) uses its literal `object`. Then run `write-tree` and a parentless
`commit-tree` with `message`. Use the manifest's author as author and committer, and its
timestamp for both dates. Require the exact `tree` and `revision`. Disable ambient Git
configuration, attributes, filters, hooks and line-ending conversion. No checkout is needed.
After every commit exists, apply the repository `envelope`: `object-alternates` writes
`.git/objects/info/alternates` naming an absent directory.

To run a case, give the case's raw map bytes to `verify-03` with `expectedBase` and
`targetArgument`, omitting an argument whose value is empty. Compare the exit code and the full
`result` object. `runtime.goVersion` is excluded from the comparison because it reports the
consumer's toolchain.

## Runtime gate

The 44 cases with `structuralRuntimeGated` claim a structural reason. Their pinned results are
generated on `qualifiedRuntime` `go1.27.1`, with `structuralQualification`
`EXPERIMENTAL_TUPLE_PENDING_FULL_CORPUS`, and hold, apart from `runtime.goVersion`, on every admitted
structural tuple: `go1.27.1`, and `go1.27.2` under decision 0522 (`CEM-V1-050`). On any other
runtime, a conforming consumer must instead
return UNSUPPORTED at stage `runtime`, code `unsupported-structural-runtime`, exit 2. It must not
fall back to a byte proof. Nonstructural cases keep their pinned result on every runtime.

## Coverage

| Category | Cases | Scope |
|---|---:|---|
| structural | 47 | Literal positives for every structural reason, declared overlaps, semantic near-misses and cross-class claims. |
| witness | 69 | Coverage and discriminates shape, byte and character boundaries (testRun, operator, description and detail, with two-byte runes), arithmetic, state rules, retention, and exclusion from hunk identity. |
| wire | 45 | Strict JSON lexemes, duplicate keys, depth 62..65, roots, key sets, spec dispatch and a map of exactly 4 MiB. |
| identity | 31 | SHA-1/SHA-256 baselines, patch digest, hunk and evidence identity, basis rules and tree modes. |
| sidecar | 15 | Base and target sidecar kinds, exact raw bytes, and a historical 0.2 sidecar. |
| arguments | 8 | Authority arguments, argument precedence, object alternates and the nonstructural runtime path. |
| drift | 6 | Stable, relocated, stale, ambiguous, deleted and mixed records. |

Every outcome, stage and code was declared by hand in the generator's case table. The generator
refuses to write a packet whose observed result differs from a declaration. It derives only Git
object IDs, patch digests, identities and the pinned result bytes.

## Native correspondence

Each case's `native` member is the outcome the native status verifier must reach: `valid`, `issue`
with the first issue code, or `error` with the error code. It is the projection of the portable
result, and the native consumer reads every case through the single-profile verifier
(`CEM-SM-018`). All 221 cases agree exactly; no case declares a divergence. Decision 0521 records
how the 17 earlier divergences were disposed of.

The portable repository envelope `primary-clean-config-bounded/1` is a declared implementation
ceiling, not a protocol rule. A repository whose `.git/config` carries another section is
UNSUPPORTED/repository `unsupported-repository-envelope` in `verify-03`. That refusal is tested
by the portable consumer only and is not a packet case. A map over 4 MiB is not a packet case
either (`NOT_PRODUCED`).

## Not established

This packet does not establish:
- independent external consumption (`CEM-V1-008`)
- Go 1.24 structural parity
- any device tuple other than the generating host
- frozen or stable status

The generator is `interop/cem01-go/packet03_test.go`. Run it with `CEM03_WRITE_PACKET=1` only for an
intended packet change. Without that variable it requires byte-identical output.
