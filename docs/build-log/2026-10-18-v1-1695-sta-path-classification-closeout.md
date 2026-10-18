# V1-1695: STA-V0 path classification closeout (2026-10-18)

V1-1695 taught the context index to classify OpenSpec and Spec Kit documents by pinned path in PR
#1094 (`docs/specs/spec-toolchain-adapters-v0.md`). It delivered `STA-V0-007` in
`internal/contextindex`, traced by `TestSTAV0007_ToolchainPathClassification`.

Its sealed binding (base `e1c349d21f`, seal `66545e6ee1`) left the touched hunks `cited`, so the
`touched-anchored` cem-verdict gate refused completion with `tier-below-anchored`. That map cannot be
rewritten. This entry is bound to `STA-V0-007`, and V1-1695 completes at its seal. V1-1760 tracks the
missing warning before seal.
