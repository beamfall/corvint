# V1-0632 S4: portable stable routing

Date: 2026-10-11. Native work: V1-0632 slice S4, migration rows C04 and C19. Spec:
`docs/specs/cem-stable-v1.md`, section "Portable stable routing (V1-0632 S4)",
`CEM-V1-PSR-001..004` (proposed).

## Decisions

- The portable `cem/0.1` decoder peeks only at the exact `spec` member of a strictly parsed JSON
  object. The existing `unsupportedSpec` header check is case-insensitive, so the peek does not
  reuse it. Every other input keeps its earlier code.
- `ci` puts the routing hint in the existing `code` member (`use-verify-stable-canonical`). It does
  not add a member, so the fixed 14-member report and exit 4 still hold.
- The example routes to `verify-stable-canonical` instead of the public `verify-stable`. CI
  checkouts carry ordinary origin and user config, which only the canonical envelope admits.
- The stable branch is an opt-in `CEM_PROFILE` value of the existing script, not a new script and
  not a workflow change. This follows C19: an explicitly named stable recipe with no automatic
  cutover of the old default.
- The requirement IDs use the separate `CEM-V1-PSR-` family so they cannot collide with parallel
  V1-0632 slices that number `CEM-V1-0xx`.

## Evidence

- Failing before: `TestDefaultPathRefusesStableSpec` gave `version-shape`,
  `TestCIRoutesStableSpecToStableVerifier` sealed/minimal gave code `unsupported-profile`, and
  `TestExampleStableBranch` exited 4 instead of the direct verifier's exit. The control table
  `TestDefaultPathKeepsOtherProfileRefusals` passed both before and after.
- After: the four new tests pass. `TestExampleStableBranch` compares the script's stdout bytes and
  exit with a direct `verify-stable-canonical` run over the public S0e repository, for both the
  positive and a tampered artifact root. Because it compares rather than asserting ACCEPT, the test
  does not depend on the host Go runtime tuple.
- `make cem-recipes-test` passes, including the new `c-stable` case and the unchanged
  `c-unsupported` recovery.

## Limits

- On host go1.27.2 the stable branch returns `UNSUPPORTED` `unsupported-structural-runtime` for
  the Go-hunk fixtures. The example passes that through, and no end-to-end stable ACCEPT through the
  script was observed here.
- The four `TestStableCanonical*` failures (87 subtests) in the interop suite predate this change
  and have the same runtime-tuple cause. They belong to the Go pin move, not this slice.
- The GitHub workflow example `examples/cem/github-actions-portable.yml` is unchanged. It has no
  artifact-root input, so it cannot opt in yet.

## NOT_RUN

- `make gate` and the root `go test ./...`: not requested for scoped slice work.
- Independent review, CEM dogfood bind/seal and the native Tasks completion write belong to the
  integrating lane.
- `make cem-verify-pr-test`: the `cem/0.2` `verify-pr.sh` path is unchanged.
