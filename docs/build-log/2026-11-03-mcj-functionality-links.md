# MCJ slice 3a: functionality links, gap-filling and test-surface selection

Base: `bb729132b` (spec branch `claude/map-coverage-join-spec`). Ticket: V1-4386. Requirements:
MCJ-V0-020..024 in `docs/specs/map-coverage-join-v0.md`. Delivered as an experimental library,
`internal/funclink`. No plan, CLI or CI caller exists yet, so no change is selected by it today.

## Decisions

- **Authority is an offer mask.** Each code or test entry keeps the bitwise union of the
  authorities that offered it, and its authority is the highest bit. An offer only adds bits, so
  the result is the same in every offer order and a lower authority can never overwrite a higher
  one. A test checks all six orders of the three sources. A disagreement compares the set's
  highest authority present with each lower one present, and no entry is dropped for it.
- **Inferred AFU-V1 links are not read.** Only `basis: declared` links feed the declared tier;
  inferred links are proposals and would otherwise gain declared authority.
- **The AMAP-V0 impact report is the static route join.** `Map` has no structured
  route-to-component or handler edge. `ImpactReport.Screens[].Paths` places paths on screens,
  so each path joins its screen's code set as a whole file, at static authority.
- **Page-object surfaces use containment.** A change wholly inside a method, step helper or
  binding span selects only that surface's users. A change in the page object's span that no
  narrower span holds whole, including a block that straddles a method boundary, selects every
  test of the page object. Overlap alone would select every page-object test for every method
  edit; containment keeps the narrow case narrow and fails safe at the edges.
- **A surface with no extracted users adds nothing and is reported unresolved**, so the caller
  keeps its static plan for it rather than reading the silence as "no tests".
- **Shadow replacement records the fallback-miss check.** `ShadowReplacement` holds the proposed
  linked set, the fallback set and the omitted fallback tests. `FallbackMisses` intersects the
  omitted tests with the run's failures. Units whose code set is only observed never contribute,
  and an empty change is never eligible. The 1,000-change zero-miss bar needs CI shadow runs and
  stays `NOT_RUN`.
- **Test identities are opaque.** SKH-V0 test IDs, AFU-V1 test keys and `linecov` test IDs are
  separate spaces. The library does not normalise them, so one test named in two spaces counts as
  two entries until a caller joins them.

## Evidence

- `HEAVY_WHOLE_REASON="mcj funclink" bash script/heavy.sh env GOTOOLCHAIN=local go test -count=1
  -timeout 30m ./internal/funclink/... ./internal/appmap/...` on wsl-lanes (linux/amd64,
  go1.27.3): `internal/funclink` ok, `internal/appmap` ok, rc=0.
- The fault-injection test runs 2,000 seeded random changes (seed 20261103) and checks that
  `AddUncovered` and `SelectSurfaces` never drop a kept test.

## Not delivered

Project-config declared ownership is not read. The observed tier is an `ObservedSource`
interface with a test fixture only; it waits for the segment join (slices 1 and 3). Wiring into
`corvint affected` plan reasons and the CI shadow summary is later work.
