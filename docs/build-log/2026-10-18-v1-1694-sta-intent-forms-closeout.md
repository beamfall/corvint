# V1-1694: STA-V0 intent forms closeout (2026-10-18)

V1-1694 added the OpenSpec and Spec Kit OCM intent forms and fixtures in PR #1094
(`docs/specs/spec-toolchain-adapters-v0.md`). It delivered `STA-V0-002` to `STA-V0-006` and
`STA-V0-010`: the `openspec-spec`, `openspec-delta` and `speckit-spec` prepare forms, the
`nativeNames`, `excludedRequirements` and `nativeStatus` envelope members, the `--intent-form` help
listing, and the fixture projects under `interop/spec-toolchains/fixtures`.

Its sealed binding (base `e1c349d21f`, seal `39407f6691`) left the touched hunks `cited`, so the
`touched-anchored` cem-verdict gate refused completion with `tier-below-anchored`. That map cannot be
rewritten. This entry is bound to `STA-V0-002`, and V1-1694 completes at its seal. V1-1760 tracks the
missing warning before seal.
