# Benchmarks frozen LSP policy read scope (V1-0681)

At PR483 head `a72c32a400cff7586616d6b2d4d715fef5363e41`, hosted run
37031682341 job110919985906 failed `TestLSPQualificationFreezeRecord` because
`lspCheckFloors` could not read `docs/specs/lsp-qualification-evaluation-policy-v0.json`
under Landlock. The precise error was permission denied, not a digest mismatch.
The test, declaration, confiner and policy blobs are unchanged from public base
`1fda1b94984245d0cd0ac0a6d17cc72572ce6619`. This is a pre-existing declaration
omission; rerunning the unchanged shard is not evidence of a repair.

Add exactly that policy file to the `benchmarks` declaration in ascending order.
The frozen public corpus already resides inside the package subtree. Preserve
reader code, confiner code, policy bytes and all unrelated allowances. The
owner-directed AFP-V0-023 confinement contract in `affected-plan-v0.md` remains
proposed; this narrow repair does not promote a broader qualification profile.

The original hosted log, immutable base/head blob comparisons and ticket receipt
are retained privately. A genuine pre-edit enrollment at the clean public base
freezes the existing numbered AFP intent and three checks: focused documentation,
focused declaration tests, and actual Linux confinement qualification. The initial
empty-diff dogfood refusal remains retained; it is not a passing binding.

The private Linux recipe uses the existing Go 1.27.1 Linux arm64 image, offline
read-only source exported from the exact commit, an isolated unchanged wrapper,
an explicit Landlock ABI probe, the named frozen-record test under the wrapper
with race detection, and the unrelated-read denial invariant test. Skips cannot
qualify. This local tuple does not establish hosted Ubuntu amd64 results; those
remain required after publication. Each actual result retains its exact source,
image, recipe, argv, log and cleanup evidence. One actual INT and one TERM control
proved the owned Docker client joined and the unique container was absent before
the recipe was frozen; no unrelated process or container was retired.

Rollback removes this one allowance, restoring the demonstrated failure. Do not
broaden the whole spec directory, skip the test, or weaken confinement to pass CI.
V1-0681 stays open until required qualification and integration/native completion.
