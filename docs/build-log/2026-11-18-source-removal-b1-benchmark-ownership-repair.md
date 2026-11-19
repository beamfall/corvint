# Source-removal B1: retained benchmark ownership

V1-5694; Verifier repair 1 at parent `b7c4c2f4c8bec14e24cb569ce95a2c2c67403b98`.
Requirements: RMV-V1-007/031 and AZU-V0-002/8900/8926.

B1 removed `benchmarks/selfuse-batch/`, then removed its `benchmarks/**` ownership entry.
That left retained benchmark data unclaimed. Restore the directory declaration with the
existing repository ownership completeness test as its verification owner, following
AZU-V0-8900 and the adjacent retained-data declarations. This adds a real verification
owner without restoring a deleted package or changing the selection algorithm.

The class is stale project ownership after retirement of a test package, not an unsupported
analysis form. No recognition allowlist changes. The existing completeness test checks all
tracked paths and rejects a new path outside every declaration. The existing W-class reader
regression checks benchmark ownership and selection of the whole-tree readers. Both retain
their assertions. No golden, wire shape, error contract, or ownership baseline changes.

## Verification

`corvint affected --base 83d95f840c4506c43d0314b339686ba7273224d6 --format json`
selected the live repository package, declaration compatibility checker, and import ratchet.
The plan remains UNKNOWN: 344 selected, 344 excluded, 77 unknowns, 16 unresolved packages.
Unknown reasons: LANGUAGE_FRONTIER=58, UNINDEXED_SOURCE_PATH=5, UNOWNED_DIRTY_PATH=14.
Raw plan SHA-256: `65210529c1b65f6890da9668c66cab18e2623b40c048300cf39f8f5a67c93786`.

Whole existing packages ran separately through `script/heavy.sh go test -count=1 -timeout 30m`:

- `./internal/liveverify/affected/liverepo`: PASS, 143.411s; live tier enabled.
- `./tools/declaration-compat-check`: PASS, 0.298s.
- `./tools/import-ratchet`: PASS, 0.969s.

The ownership declaration tested has Git blob `fc1a4357f6ccde3b75e9d271ec0a544393fc7da4`.
Tests used the configured heavy semaphore, private lane TMPDIR, and no identity exports.
The live-package log SHA-256 is
`5725fb5988bde666920adf9db423f3f59d4c01b721dff28723868e5c09d895b3`.
Raw evidence stays in `/home/russell/lanes/sr-b13-evidence/verify1-ownership-*`.

Citation repin found no moved anchors. `make line-citations-check` passed with 184 existing
basename-only skips. `make dogfood-check` exited 2: no rule for that retired target.
No specification, Go source, or test changed in this repair, so their additional gates were
not repeated. The package runs above verify the final ownership declaration.

## Limits and rollback

This closes the reported benchmark ownership failures. V1-5694 remains PARTIAL for the prior
B1 reference-proof blockers; B2/B3 are unchanged. CEM binding is unavailable under decision
0594 and the retired `docs/DOGFOOD.md` contract. No seal, publication, task-store write,
model-backed review, or broad gate runs in this repair. The host owns independent review.
Revert this repair commit to restore the previous declaration, including its ownership gap.
