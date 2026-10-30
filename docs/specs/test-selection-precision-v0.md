# Test Selection Precision V0

Owner: Russell Lewis
Date: 2026-10-30
Requirement prefix: `TSP-V0`
Intent status: experimental under the owner's standing lane auto-accept (2026-10-11; V1-3160); TSP-V0-001 amends AFP-V0-580
Delivery status: experimental
Authoritative inputs: native ticket V1-3160 (related V1-2763, V1-2896). These requirements narrow
when the [affected plan](affected-plan-v0.md) names a unit frontier; that spec's Requirements
section is at the 256-obligation OCM limit (`maxObligations`), so they live here.

## Agent digest
- Claim: `go:build-constraint-variants` is named only on a plan with a dirty path in the constrained package's own directory or an ancestor.
- Status: experimental under the owner's standing lane auto-accept (2026-10-11; V1-3160); TSP-V0-001 amends AFP-V0-580.
- Exists: `FrontierBearsOnOwnFiles` in `internal/liveverify/affected/unit.go`; `ownFilesBear` in `internal/liveverify/affected/select.go`; `observeDirectory` in `internal/liveverify/affected/golang/golang.go`; tests named in the traceability table.
- Blocked on: an owner decision for the nested-module and shell foreign-invocation frontiers (Open questions).
- Read next: Requirements; Failure modes; Open questions.

## Intent and scope

Intent (orchestrator lane request, 2026-10-30; V1-3160): the owner asked that every run select only
the minimum safe tests. At `fd51f53a2` every non-empty Corvint plan was `UNKNOWN`, and therefore
FULL in CI, because three global frontiers bore on it: `go:build-constraint-variants`,
`go:nested-module-frontier` and `shell:foreign-invocation-unmodelled`. Each frontier SHALL be scoped
to the changes it can actually bear on, and SHALL still make the plan `UNKNOWN` wherever it applies.

## Requirements

- **TSP-V0-001:** (experimental, amending AFP-V0-580, under the owner's standing lane auto-accept,
  2026-10-11; V1-3160) A unit MAY declare `frontierBears: OWN_FILES`. Its frontier SHALL then be
  named only on a plan with a dirty path whose directory is the unit's directory (the directory of
  its first source or test file) or an ancestor of it, whatever reached the unit. A unit with no source or test file
  SHALL bear as before. `OWN_FILES` SHALL apply in both the reached-unit and the narrowed-reader
  frontier loops of selection. The Go plugin SHALL declare `OWN_FILES` on every unit it gives
  `go:build-constraint-variants`. The model: a Go package's files all live in its directory, so a
  file is added, deleted, or gains or loses a constraint only by a dirty path there; the module
  and workspace manifests and the build configuration that choose the toolchain and tags (`go.mod`,
  `go.work`, `Makefile`) lie in an ancestor; every
  variant's imports are already graph edges, so a change elsewhere reaches every variant's
  dependents through the same union edges; and one test run compiles one host variant whichever
  packages it selects, so naming the frontier for a change outside the directory added
  unreached packages to the run without exercising any other variant.

## Non-goals

Evaluating build constraints for a host or running non-host variants; scoping the frontier to the
dirty file's own constraint (a plain file in the directory can be what a variant calls, which
AFP-V0-580's review withdrew a narrower scoping over); scoping `go:nested-module-frontier` or
`shell:foreign-invocation-unmodelled` (see Open questions).

## Failure modes

(1) A non-host variant that calls code in another package can break when that package changes,
and the plan is now `BOUNDED`; a FULL run on the same host compiles the same host variant, so it
would not have caught it either; the cross-compiling checks (`make windows-vet`, the CI Windows vet
step) remain the evidence for that variant. (2) A dirty path in a constrained package's directory
or an ancestor still names the frontier, including a non-Go file, an added variant and a deleted one;
a root-level change (`README.md`) therefore keeps it for every constrained package it reaches.
Build configuration outside an ancestor (`.github/**`, which forces FULL in CI; a script under
`script/` that passes `-tags`) is not modelled; no repository script passes `-tags` today. (3) A plan
from an older planner, or a unit graph cached before this change, carries no `OWN_FILES` and bears
as before; the graph cache is bound to the program identity, so a new binary rebuilds it.

## Open questions

These need an owner decision because no existing project-owned model bounds them; each frontier
stays as it was:

1. `go:nested-module-frontier` bears on every plan. The synthetic dependent-module unit for
   `tools/local-authority` inherits root locating (`LocatesRoot`) from
   `internal/authoritystore`, whose working-directory walk climbs to `.git`; a root locator's
   dependents are followed on every dirty path (AFP-V0-581 (1), AFP-V0-012 rule (d)). The module's
   tests cannot be selected (AFP-V0-582 non-goal) and CI never runs them, so the frontier is their
   only signal. Options: (a) a project declaration bounding a root locator's code-level reads (for
   example `internal/authoritystore` reads only `.git` metadata and an absolute system path);
   (b) read scopes for dependent-module keys, which `.github/testconfine` would not enforce;
   (c) accept AFP-V0-650's listed failure mode and stop naming it for root-module-only changes.
2. `shell:foreign-invocation-unmodelled` bears on every plan through `script/heavy.sh`, which
   locates the root (`git rev-parse --show-toplevel`) and sources a runtime file (`. "$w/meta.env"`),
   so it is an unbounded reader followed on every dirty path. Narrowing needs a model of what that
   dynamic source can read, or a model of the script's non-Go callers.

## Acceptance evidence and traceability

The tests below, failing on base `fd51f53a2`; the measured plans in build log
`docs/build-log/2026-10-30-v1-3160-frontier-scoping.md`.

| Requirement | Implementation | Evidence |
|---|---|---|
| TSP-V0-001 | `FrontierBearsOnOwnFiles` and `validFrontierBears` in `internal/liveverify/affected/unit.go`; `ownFilesBear` and its two call sites in `internal/liveverify/affected/select.go`; `observeDirectory` in `internal/liveverify/affected/golang/golang.go` | `TestBuildConstraintFrontierBearsOnOwnFiles_TSP_V0_001` (a dirty plain file, variant, absent variant or non-Go file in the directory, alone or with another path, and a root or ancestor file the package reads, bears and stays `UNKNOWN`; a dirty package the variant imports, a named data path and an unrelated package do not, the variant's tests stay selected, and the imported package's plan is `BOUNDED`), the narrowing, V1-0289 and V1-1818 cases in `internal/liveverify/affected/golang/golang_test.go` |

## Rollback

Remove `frontierBears = affected.FrontierBearsOnOwnFiles` in `observeDirectory`; the constant and
`ownFilesBear` are then inert. No stored state depends on the value.
