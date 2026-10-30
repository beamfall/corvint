# Security PR CI repair: historical CEM proof and declared runtime tuple

The security remediation's CI exposed two concrete failures. The corpus-binding test
required historical Git objects absent from the fresh public repository, and the
companion-release tuple check retained Vitest 5.0.0 literals after the security patch
moved the provider and qualification tuple to 5.0.3. Neither failure authorizes a gate
skip, weaker evidence, or a new runtime qualification claim.

## Historical corpus binding

`TestCEMV1201CommittedCorpusBinding` fails identically on the unmodified public root
`9477995d776ddf88d7f6a82790dd9c22d51da765` in a `--no-local --single-branch` public-only
clone. Root947 deliberately retains eleven historical CEM fixtures under decision
0587, but not their original bind/migration history. The test source at root947 and
security candidate `8de25304257bf58cd6727a2122ada1a7671b4eb7` is identical. A workstation
with old objects masks the missing-proof failure; CI=true correctly refuses it
(V1-1334). Decisions 0331 and 0587 forbid claiming those records validate the new root
and allow a committed fixture for the historical reader.

The repair keeps CEM-V1-201/230/231's original comparisons. A test-only, non-thin Git
pack supplies precisely the original bind, path-tree, map/source and notice objects
for those eleven files. It imports only into a fresh temporary bare repository with
an empty template and no inherited Git repository/object/config injection. The loader
checks the independently pinned manifest, pack digest/size and exact OID/type/size
inventory, and verifies the original notice paths. Root947 supplies the immutable
historical allowlist; every retained name and byte must remain present, with matching
archive-pin bytes. All other maps resolve solely in the current repository. Missing
current proof still fails CI=true. Historical/current binding counts remain separate;
historical binding supplies no current-change coverage.

Fixture: 63 objects (14 commits, 31 trees, 18 blobs including six notice blobs),
542,370 uncompressed content bytes and 249,286 pack bytes. Pack SHA-256:
`f627783ae5f84cfc3c1dee442ccd07fd07d429ed95718ba75d97405d8286ee0d`.
Manifest SHA-256:
`a5d7fdef6e4884e45d125806ba2ce2387b22da7994f05cef9fb8517bbae71beb`.
Source archive pin: `beamfall/corvint-history` at
`1c4d8793cf71cbb01b8509b68a6cb1502a40a13e`. The selected commits' exact historical
LICENSE/LICENSING bytes accompany the metadata; predecessor notices are not silently
relabelled by the current root's license.

The fixture proves particular original commit:path and blob relationships. Parents
and off-path tree targets are deliberately absent; full connectivity, whole-history
validity and independently traversable ancestry are not claimed. The eleven binds'
ancestry to the archive pin was observed in the complete local source store during
construction and is retained separately as provenance. No unrelated source blobs,
refs, remotes or alternates are included. Tests require no network or complete archive.

## Executed verification and limits

On darwin/arm64 with Go 1.27.3 and Git 2.54.0, the corpus package and focused vet pass.
Positive initialization precedes corrupt/truncated/internal-checksum pack controls,
omitted-object and wrong-type/fixture-name/hash/blob controls. Changed/deleted
historical files refuse; valid current binding is exercised before a changed-byte
refusal. Unknown/current missing proof reaches the lookup failure, and a child test
binary confirms that CI=true exits unsuccessfully rather than skipping. Poisoned Git
environment variables neither supply foreign objects nor alter an isolated product
repository's object inventory. The initial positive trial exposed a working-directory
prefix error in root enumeration; `ls-tree --full-tree` corrected it before acceptance.

A repaired test binary compiled from the recorded uncommitted source was then run with
CI=true in a separate clean `--no-local --no-tags --single-branch` public-only clone at
8de2530. No source overlay or commit was made in that clone. The old 06cf00ac bind was
absent, no object alternates existed, all tests passed, and the clone's complete object
inventory and clean status were unchanged. It verified historical migrated=1/native=10
and current native=1. This is a source-hash/binary-bound isolation witness, not a claim
of final committed or hosted CI success. Parent closeout must rerun the corpus package
on the final seal, because sealing adds a current map, and repeat the ordinary test
command in a clean clone of that final head.

The tuple repair updates the four stale literals in the existing consistency test and
public-release spec to 5.0.3; Node/npm/Playwright/browser pins and assertions remain.
`TestV12690DeclaredRuntimeTupleDrift` reproduced the stale literal failure and passes
after the correction. Full real VSCode interactive qualification of the patched tuple
remains NOT_RUN; consistency tests and fixture unit tests do not substitute for it.

Independent implementation/fixture review, final exact-content checks, CEM/OCM closure,
CI integration and native ticket completion remain parent-owned. No repository-wide
`make gate` was run by this slice. Evidence is retained under
`/private/tmp/corvint-security-20261029/ci-repair/`, including `public-clone-result.json`,
its exact source hashes and command log, baseline/focused logs and fixture inventories.
Rollback reverts the task-owned test/fixture/spec changes; no product Git object store,
wire format, runtime behavior or persisted store is migrated.
