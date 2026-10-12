# V1-0632 C06..C08: stable TCQ/frontier, obligation sidecar and criterion experiment routes

Date: 2026-10-12. Base: `37ed36a46b40ba2499de14482df5d902098c77fc`. Branch:
`claude/v1-0632-closure-chain`. Scope: migration rows C06, C07 and C08 of
`protocol/cem-1.0/stable/MIGRATION.json` under decision 0525, with proposed requirements
`CEM-V1-120` to `CEM-V1-139` in their own section of `docs/specs/cem-stable-v1.md`
(`CEM-V1-140..159` belong to the parallel V1-1276 lane). No ticket-store writes, PRs,
CEM bind or seal.

## Decisions

- Each family follows the C05 pattern (`lrfrepo.VerifyStableUniverse`, `CEM-V1-051..059`):
  - The map is admitted only by the shipped `verify.Stable` `ACCEPT` over the exact bytes.
  - The repository, base, target and artifact root are all explicit, with no defaults.
  - The committed sidecar must be `EXACT`.
  - The map digest, base and patch are re-bound after acceptance.
- Every emitted document takes a new, distinct profile:
  - `tcq-stable/0`, with identity domain `corvint-tcq-stable/0`;
  - `external-frontier-obligations-stable/0`, with prefix `obligations-stable:sha256:`;
  - `cem-criterion-experiments-stable/0`.

  The frozen `tcq/0`, `frontier/0`, `external-frontier-obligations/0` and
  `cem-criterion-experiments/0` profiles and their bytes are unchanged, and no 0.x code is
  removed (V1-1278 owns removal).
- Fail-closed fork: no stable frontier document. `frontier/0` binds an `lrf/0` digest, and no
  stable LRF or frontier profile is accepted. `frontierrepo.RecomputeStable` therefore returns the
  verified universe, its obligations, and a `tcq-stable/0` result with
  `FrontierState: NOT_PRODUCED`. It never returns an `EMPTY` or closed frontier.
- Fail-closed fork: every stable route refuses an `ACCEPT`/`ABSENT` verifier result, i.e. a target
  that commits no sidecar.
- Fail-closed fork: the C08 stable admission does not read the map's criterion links. Criterion
  authority stays the native Tasks capture and verifier, exactly as in the frozen mode. The map only
  supplies the verified change and its hunk identities.
- C07 lists the map's declared criterion links in a separate `criteria` member with
  `NOT_OBSERVED` satisfaction. They are never associations and never close an obligation.
- C08 selects the mode from the request schema. The artifact root is an invocation input to every
  phase and never plan data. A frozen request with a root, or a stable request without one,
  refuses.
- `corvint obligations` takes four stable flags, all or nothing. `cmd/corvint/main.go` is
  unchanged, so no use-case receipt repin was needed.
- The `obligations` help text grew by 10 lines. The two
  `docs/specs/FRONTIER-DECISION-BRIEF-2026-08-29.md` citations into `cmd/corvint/help.go`
  were moved to lines 913-920 and 1057. Their content anchors (`@1713970e`, `@e432ec24`) are
  unchanged and still match, because the cited lines are byte-identical.
- `internal/cem/stablefixture` is a test-only helper (imported only from `_test.go` files). It
  converts a real prepared and cited `cem/0.2` change to a verified `cem/1.0` map with six
  artifacts, its sidecar committed at a new target. Its Git fixtures disable maintenance and
  auto-gc.
- `MIGRATION.json` is digest-pinned by `protocol/cem-1.0/stable/manifest.json` and was not
  edited. C06..C08 there still read `OPEN_NOT_IMPLEMENTED_BY_THIS_PROPOSAL`.
- The Agent digest line of the spec was not edited, to keep the parallel V1-1276 lane mergeable.
  The new section, the traceability table and `REQUIREMENTS.tsv` carry the IDs.

## Evidence

- Focused tests, all `go test -p 1 -count=1`, passed on this darwin host (go1.27.2), covering
  `internal/tcq`, `internal/frontierrepo`, `internal/extevidence`, `internal/criterionexperiment`,
  `cmd/corvint-cem-experiments` and `cmd/corvint`. See the spec traceability table for the
  per-requirement tests.
- The legacy suites in the same packages pass unchanged, including:
  - `TestHistoricalVerifyArtifactsAndSurvivors`;
  - the frozen Obligations tests;
  - the TCQ/frontier suites.
- `TestCEMV1136StableExperimentRunsAndVerifies` runs a real stable plan through `RunStable` and
  `VerifyStable`, which yield `VERIFIED_SATISFIED` with the native Tasks binary built from source
  in historical mode. A missing root and a tampered artifact then refuse.
- Corvint use: `corvint affected --base 37ed36a46` (Corvint 1.0.0-rc.3 build 456) selected 127
  units, 6 by direct source change; the advisory checks are `PLAN_ONLY`.

## Limits and NOT_RUN

- The stable TCQ route does not apply the frontier adapter's Python grammar-failure reason rewrite
  (TCQ-V0-047). Such a claim stays abstained with the generic `unsupported-anchor-profile` reason.
- No stable LRF evaluation and no stable frontier document exist. The C06 acceptance item "invalid
  generation/receipt cannot close" is covered only as far as no stable route closes anything.
- No CLI or MCP route exists for the stable TCQ/frontier upstream. The obligations stable mode and
  the experiments flag are the only CLI surfaces.
- C08:
  - planning is tested through `planWithCaptures`, which was split out of `planExperiment`
    with no behavior change, over the verified fixture captures; neither the stable nor the
    frozen tests exercise the live Tasks `capture` call;
  - the live exact-target Tasks gate (`gate`, live mode) over a stable plan is NOT_RUN;
  - the stale-generation and policy negatives over a stable plan are NOT_RUN beyond the shared
    frozen tests;
  - the stable CLI test covers dispatch and refusals only.
- Not run:
  - Linux, Windows and other structural runtime tuples;
  - the other 121 affected units (path-literal, dependency and binary-exec witnesses of additive
    files);
  - `make gate`.
- Stranger time is NOT_OBSERVED.

## Rollback

- Delete:
  - `internal/tcq/stable.go`;
  - `internal/frontierrepo/stable.go`;
  - `internal/extevidence/obligations_stable.go`;
  - `internal/criterionexperiment/stable.go`;
  - `internal/cem/stablefixture`;
  - their tests.
- Restore:
  - `parseArtifacts`;
  - the companion's single-schema checks and phase signatures;
  - `cmd/corvint/obligations.go` and its help text;
  - `cmd/corvint-cem-experiments/main.go`.
- Remove the `CEM-V1-120..139` section.

No 0.x profile, default, packet or stored evidence changes.
