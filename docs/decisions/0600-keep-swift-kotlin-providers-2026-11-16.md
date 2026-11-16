# Decision 0600: the Swift and Kotlin providers stay in 1.0

- Date: 2026-11-16
- Status: accepted as owner direction (owner chat, 2026-11-16, relayed by the orchestrator). No
  owner acceptance row is written by the branch that carries this file; the change
  `docs/specs/openspec/changes/source-removal/` stays `proposed` until the owner accepts it.
- Authority: owner direction, 2026-11-16, quoted below.
- Amends: decision 0596, the G26 line ("languages and analyzers beyond the seven runners are
  removed") and its accepted cost for beamfall-apple and beamfall-android;
  `docs/specs/PRODUCT-PLAN-1.0.md` (PROD-TEST-008, PROD-TEST-011 and the G26 row of "Removed from
  1.0"); change `source-removal` (batch B5, RMV-V1-014, RMV-V1-015 and the retirement table).
- Read with: decisions 0594 and 0596; `docs/specs/swift-affected-test-understanding-v0.md`
  (SATS-V0); `docs/specs/kotlin-jvm-affected-test-v0.md` (KAT-V0).

## Context

Decision 0596 accepted the 1.0 product plan. Its G26 removes every language provider beyond the
seven blocking runners (Go, Vitest, Jest, Playwright, pytest, RSpec, Minitest) and all 16
`corvint-analyzer-*` binaries. It recorded the cost: beamfall-apple (Swift) and beamfall-android
(Kotlin) run all their tests on every change until a provider is rebuilt after 1.0.

The owner reviewed the cuts on 2026-11-16 against the functionality wanted from 1.0. Two of the
owner's own product repositories are Swift and Kotlin, so G26 would take test selection away from
them on the day 1.0 ships.

## Owner direction

Owner, 2026-11-16, verbatim: "Reopen Swift/Kotlin".

The owner gave no other answer in that review, so every other cut of decision 0596 stands: G8
(CEM, OCM and dogfood), G22 (VS Code), G23 and R6, G24, G25, and the removal of the C-family, .NET,
Java and Rust providers.

## Evidence

Checked in code at `origin/main` `df315e350`:

- The Kotlin provider stands alone. `internal/liveverify/affected/java/kotlin.go` imports the
  Kotlin provider; the Kotlin provider does not import the Java one. Java can be removed and Kotlin
  kept.
- `internal/swiftimpact` imports `internal/liveverify/affected/swift` and is linked by blank import
  from `cmd/corvint/swift_impact.go` and `cmd/corvint-mcp/swift_impact.go`.
- `internal/testrunner/swiftselection` reads `internal/nativelinecov`. The other importers of
  `internal/nativelinecov` are the .NET, JVM and Rust registry files that B5 removes.
- `internal/langplugin/langplugin.go` declares, for Kotlin, the runners `android-junit`, `appium`,
  `appium-uiautomator2-wdio`, `espresso`, `kotest`, `kotlin-test`, `robolectric` and `uiautomator`;
  for Swift, `swift-testing`, `swift-xctest` and `xcode-xctest`. Each manifest also names one
  analyzer binary.

Not measured here: selection quality of either provider on beamfall-apple or beamfall-android. This
decision keeps what exists; it claims no accuracy figure.

## Decision

### Disposition

| Surface | Disposition | What stays | What goes |
| --- | --- | --- | --- |
| `internal/liveverify/affected/swift`, `internal/liveverify/affected/kotlin` | KEEP | the providers and their tests | nothing |
| `internal/swiftimpact`, `cmd/corvint/swift_impact.go`, `cmd/corvint-mcp/swift_impact.go` | KEEP | the package and both links | `cmd/corvint-test-validity-mcp/swift_impact.go` goes with its binary in B2, as before |
| Swift and Kotlin entries in `internal/langplugin/langplugin.go` and `internal/langplugin/decl/decl.go` | KEEP, reduced | provider, runner names, coverage, the paths that stay | the `Analyzers` entry and the analyzer paths |
| `internal/testrunner/swiftselection` | KEEP | the package | nothing |
| `internal/nativelinecov` | KEEP if reached (rule below) | the whole package | whatever only the removed .NET, JVM and Rust files reached |
| Swift and Kotlin runner names and argv in `internal/testrunner/platform`; `internal/testrunner/mobile` | KEEP as far as reached | what the Swift and Kotlin runners listed above need | runner names of removed languages |
| JVM registry files (`internal/testrunner/registry/jvm_*.go`) | KEEP only if reached | the parts a Kotlin runner needs, named in the B5 build log | the Java-only parts |
| Specs SATS-V0 and KAT-V0 | KEEP active | both specs | they leave the B5 retirement table |
| `corvint-analyzer-swift`, `corvint-analyzer-kotlin-android`, `internal/analyzerswift`, `internal/analyzerkotlinandroid`, specs SAC and KA-V0 | CUT (unchanged) | nothing | the binaries, packages and specs, in B5 |
| C-family, .NET, Java and Rust providers; the other 14 analyzers | CUT (unchanged) | nothing | as in decision 0596 G26 |

Rule for shared code: "reached" is tested, not judged. A package B5 named for removal stays whole
when `go list -deps` of a kept Swift or Kotlin package, or of `cmd/corvint` after the removal,
still lists it (transitive imports count). A registry `jvm_*.go` file stays when it declares a
symbol the kept Kotlin runner path references. The existing Swift and Kotlin selection tests must
pass after the removal, and the B5 build log records the `go list -deps` output or symbol
reference for each kept package or file. Anything with neither proof is removed.

### The runner list

PROD-TEST-011 and RNC-V1-004 (change `runner-contract`) name exactly seven blocking runners. This
decision does not add an eighth. The Swift and Kotlin runners are kept as additional, non-blocking
runners: selection and running keep working for them, and they run through the same compiled-in
runner contract (G23) as the seven. They are not part of the 1.0 acceptance evidence that
PROD-TEST-011 asks for per blocking runner, and a defect in them does not block the 1.0 release
unless it is a silent miss (a change that should run a test and runs none).

The `runner-contract` change needs a follow-up edit that states this beside RNC-V1-004. That edit
is not made here.

### Fail-closed rule

RMV-V1-014 and owner decision R32 are unchanged: a change no runner owns, or a removed or
unavailable runner, runs all tests with a named reason. They still gate B5, now for C-family, .NET,
Java and Rust. A `.swift` or `.kt` change is owned by its kept provider, so the removal no longer
widens it. The providers' own uncertainty rules are unchanged: SATS-V0-761 still widens
cross-repository dependencies to `UNKNOWN`, and KAT-V0-007 still returns `UNKNOWN_SCOPE` for an
unresolved discovery or build target.

### Accepted cost, replaced

The cost recorded in decision 0596 for beamfall-apple and beamfall-android no longer applies. The
new cost is that the Swift and Kotlin providers, `internal/swiftimpact` and their runner code stay
in the 1.0 source and must pass the 1.0 gates.

## Not changed

- Every other removal in decision 0596 and in change `source-removal`, and the batch order.
- The 2026-12-01 date for completing removals.
- The seven blocking runners and their acceptance evidence.
- `AGENTS.md` product invariants.

## Rollback

Revert the commit that carries this file and the plan edits. Nothing is migrated. No source was
removed or restored by this decision: B5 has not run, so both providers are still on `origin/main`.

## Non-goals

- No new Swift or Kotlin capability, and no accuracy claim for either provider.
- No return of any analyzer binary.
- No return of the Java, C-family, .NET or Rust providers.
- No `.github` change in the same branch.
- No release, tag or publication.
