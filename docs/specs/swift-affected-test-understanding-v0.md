# Swift affected-test understanding V0

Owner: Russell Lewis
Date: 2026-08-29
Intent status: proposed
Delivery status: experimental
Authoritative inputs: operator request dated 2026-08-29,
[`live-proof-carrying-verification-v0.md`](live-proof-carrying-verification-v0.md),
[`../SPEC-DRIVEN-DEVELOPMENT.md`](../SPEC-DRIVEN-DEVELOPMENT.md)

## Agent digest
- Claim: Static SwiftPM and Xcode analysis conservatively maps Swift changes to test targets while preserving scheme and runtime unknowns.
- Status: proposed/experimental
- Exists: `internal/liveverify/affected/swift` static SwiftPM/Xcode planning adapter and focused tests.
- Blocked on: runtime-provider qualification, scheme/runtime unknowns, and cross-language ownership.
- Read next: Verified current state; Cross-language ownership decision required; Traceability.

## User and measurable job

Given repository-relative Swift changes, Corvint should identify the statically reachable Swift test
targets without executing SwiftPM or Xcode. It must recognize XCTest, Swift Testing, and XCUITest;
preserve mixed-framework targets; and expose uncertainty whenever source text cannot establish the
target, dependency, discovery, scheme, or cross-language E2E boundary. This adapter is planning
evidence only. It does not qualify a runtime provider or authorize narrow test omission.

## Verified current state

`internal/liveverify/affected.Language` provides a namespace, source-path ownership predicate, and a
static unit graph. A unit contains paths and imports but no runner command, scheme, test plan,
destination, toolchain, or environment. `Graph` rejects a path assigned to two units. `Select`
already provides deterministic dependency witnesses and graph-bound exclusions, and any language
frontier makes scope `UNKNOWN`.

Before this slice, only the Go and Python adapters implemented the seam. Swift source, executable
`Package.swift` manifests, Xcode projects, shared schemes, XCTest/Swift Testing discovery, and
XCUITest app-host edges were absent.

## Unit and invocation contract

The unit is one SwiftPM target or Xcode native target. Target granularity is the smallest common
runner-addressable unit that does not assign one `.swift` file to several class/function units.
Test-declaring files are `Tests`; helper files compiled into a test target are `Sources`. Source
targets remain traversal-only units.

Stable identities are:

- `swift:spm:<package-directory>/<target>`;
- `swift:xcode:<project>.xcodeproj/<shared-scheme>/<target>` when a shared scheme is known;
- `swift:xcode:<project>.xcodeproj/<target>` with an explicit frontier when it is not.

The exact SwiftPM target invocation represented by the first identity is:

```console
swift test --package-path <package-directory> --enable-xctest --enable-swift-testing --filter '^<regex-escaped-target>\.'
```

The current Swift 6.3.3 runner was observed listing XCTest as
`Target.Case/testMethod` and Swift Testing as `Target.Suite/testFunction()`. The anchored target
filter executed both frameworks in one target.

The Xcode and XCUITest target invocation is:

```console
xcodebuild test -project <project>.xcodeproj -scheme <shared-scheme> -destination <qualified-destination> -only-testing:<target>
```

Repository-required `-testPlan` and environment belong in the runtime-provider plan. Because the
current `Unit` cannot carry them, the static adapter does not claim that an Xcode identity alone is
a complete executable plan. An absent shared scheme or XCUITest app-host edge is a frontier.

## Requirements

- `SATS-V0-001`: `Owns` MUST claim `.swift` and MUST NOT claim `.yaml`, `.yml`, `.feature`, or a
  non-Swift Appium test body.
- `SATS-V0-002`: A SwiftPM target declaration with literal name, path, sources, excludes, and local
  dependencies MUST produce one deterministic target unit. Executable/computed or plugin-generated
  target structure that static reading cannot close MUST add a frontier.
- `SATS-V0-003`: An Xcode `PBXNativeTarget` MUST derive membership through its `PBXGroup` and
  `sourceTree` hierarchy from its sources build phase, local dependencies from target dependencies,
  and test kind from product type. It MUST NOT guess ownership from a unique basename. Unresolved,
  synchronized, generated, or ambiguous project structure MUST add a frontier.
- `SATS-V0-004`: XCTest detection MUST recognize XCTest target membership, `XCTestCase` test
  declarations, performance methods, and runtime-discovery hooks. Performance tests remain in their
  ordinary unit-test target.
- `SATS-V0-005`: Swift Testing detection MUST recognize `import Testing` plus `@Test`, including
  top-level, implicit-suite, explicit `@Suite`, and parameterized declarations. `#expect` alone is
  not test-discovery evidence.
- `SATS-V0-006`: XCTest and Swift Testing files in one target MUST remain one mixed-framework target
  unit and MUST be runnable by the same target filter.
- `SATS-V0-007`: XCUITest MUST be identified by the UI-testing bundle product type and MUST remain a
  target distinct from unit-test bundles. A dependency suppresses the app-host frontier only when
  it resolves to an application product target, and that host MUST become an edge;
  `XCUIApplication` is supporting syntax, not target authority.
- `SATS-V0-008`: Literal target dependencies and static Swift imports, including attributed and
  `@testable` imports, MUST resolve to observed local target identities. Conditional compilation is
  retained conservatively and adds a frontier. Ambiguous module resolution adds all candidate edges
  and a frontier.
- `SATS-V0-009`: Unreadable source, nonliteral manifest fields, unparsed manifest/project structure,
  an owned source outside a target, overlapping target membership, unresolved shared/enabled scheme
  or app host, a test-target shell build phase, and reflection-driven or generated discovery MUST be
  explicit frontiers. They MUST NOT become silent exclusions.
- `SATS-V0-010`: Maestro and non-Swift Appium presence MAY be observed without owning their body
  paths. Their bodies remain outside the Swift graph and MUST add a cross-language frontier.
- `SATS-V0-011`: Unit, path, import, framework, and frontier ordering MUST be deterministic for fixed
  source text, preserving `LPCV-V0-013` witnesses and `LPCV-V0-019` canonical plans.
- `SATS-V0-012`: The adapter MUST remain a static reader. It MUST NOT invoke SwiftPM, Xcode, package
  resolution, a simulator, Appium, or Maestro.

## Detection and known unknowns

| Framework | Static authority | Runnable unit | Explicit unknown cases |
|---|---|---|---|
| XCTest | SwiftPM test target or Xcode unit-test product type; `import XCTest`, `XCTestCase`, `test…` support classification | test target | custom `defaultTestSuite`, `testInvocations`, `allTests`/`XCTMain`, runtime suites/selectors, generated declarations, unresolved inheritance |
| Swift Testing | test-target membership plus `import Testing` and `@Test`; `@Suite` is optional | test target | custom macro synthesis, runtime-computed parameter inventory, generated source, runner-module ambiguity |
| XCTest performance | XCTest target plus `measure`/metrics | containing XCTest target | configuration-conditioned methods or generated metrics |
| XCUITest | Xcode UI-testing bundle product type, scheme testable, and app-target dependency | distinct UI-test target | missing shared scheme, test plan, destination, app host, generated project membership |
| Appium | recognizable Appium dependency/configuration or Swift client import | host framework target only when Swift-owned and statically known | external JavaScript/Python/Java/etc. bodies and driver capabilities remain cross-language `UNKNOWN` |
| Maestro | YAML/YML flow in a Maestro path | not representable by the Swift plugin | every flow and `runFlow` edge remains cross-language `UNKNOWN` |

Parameterized Swift Testing cases and reflection-created XCTest cases are below the chosen target
granularity, so selecting their target still runs them. Their static case inventory and per-case
selectors remain unknown and cannot support individual-test exclusion.

## Cross-language ownership decision required

Maestro `.yaml`/`.yml` flows, Gherkin `.feature` files, and Appium bodies written in another host
language do not have one natural language owner. The Swift adapter therefore observes recognizable
harness presence but returns `Owns=false` and never places those paths in a Swift unit.

If two plugins place the same path in units, `Graph` fails with `ErrDuplicateOwner`. If several
plugins only return `Owns=true`, the current graph does not reject the overlap and a dirty unindexed
path collapses to a generic unknown without naming all claimants. If one plugin attaches the path,
it silently becomes the owner even when another runtime consumes it. None is a reviewed shared-file
contract.

A resolution must define: canonical artifact identity; exclusive versus shared ownership; repository
authority for matching file/config content; deterministic claimant priority or fan-out; one dirty
path reaching multiple provider units; cross-provider dependency edges; digest and ordering rules;
duplicate behavior; provider-specific witnesses, exclusions, and unknowns; and how a step binding in
one host language relates to a feature/flow. Until then these files stay unclaimed and widen scope.

Swift itself exposes a second seam limitation: one `.swift` file can be compiled into several
SwiftPM/Xcode targets, while `Graph` permits exactly one owning unit. The adapter deterministically
keeps one membership, reports `swift:overlapping-target-membership`, and leaves scope unknown. A
future multi-membership solution needs the same fan-out and witness semantics; this slice does not
widen `Language`, `Unit`, or `Graph`.

## Trust boundary, limits, and failure behavior

All reads stay beneath the absolute repository root and use the shared bounded source walker/reader.
The explicit hidden `.maestro` presence check is bounded by `affected.MaxWalkEntries`, follows no
symlink, and reads no flow content. Static false-positive import edges widen selection. A possible
miss, ambiguous mapping, or unsupported dynamic mechanism adds a fixed frontier and therefore
`UNKNOWN_SCOPE` under `LPCV-V0-016`.

The current core `Exclusion` shape lacks the provider and evidence-identity fields demanded by the
full text of `LPCV-V0-014`. The adapter cannot repair that without changing the frozen seam, so every
Swift result containing a test unit includes `swift:exclusion-evidence-unrepresentable`. Swift test
selection is always `UNKNOWN_SCOPE` even when its static graph is otherwise complete. The selected
targets remain useful planning evidence, but this slice makes no qualification, bounded-scope, or
safe-omission claim.

The shared walker suppresses directory-entry errors before a language adapter can observe them. The
permanent exclusion-contract frontier prevents those invisible entries from becoming a bounded Swift
plan, but a future core contract must expose walk gaps if it is to identify each unreadable path or
subtree precisely.

## Acceptance, rollout, and rollback

Acceptance requires deterministic fixture coverage for SwiftPM, XCTest, Swift Testing, mixed
targets, performance tests, Xcode unit/UI target separation, dependency closure, unknown cases,
cross-language non-ownership, and seam conformance; one real SwiftPM/Xcode repository comparison;
the repository Go/Python/interop gates; and byte-stable CLI parity using a path-proven candidate.

Rollout is opt-in through `swift.New()` behind the existing language interface, with the permanent
LPCV-V0-014 frontier above. No default provider,
registry, runtime execution, support claim, or receipt change is authorized. Rollback removes the
Swift package and its seam fixture/spec entries; the shared selection implementation and existing
Go/Python behavior remain unchanged.

## Traceability

| Requirements | Implementation | Evidence |
|---|---|---|
| `SATS-V0-001..003` | `internal/liveverify/affected/swift/swift.go` | Swift ownership, SwiftPM, and Xcode fixture tests |
| `SATS-V0-004..007` | framework classification and Xcode product/scheme parsing | mixed-framework, performance, and separate XCUITest fixtures; real Beamfall Apple observation |
| `SATS-V0-008..009` | import/dependency resolution and frontier constants | dependency-closure seam suite and unknown/overlap tests |
| `SATS-V0-010` | external-harness observation without ownership | Maestro/Appium non-ownership fixture |
| `SATS-V0-011` | canonical sorting plus unchanged `affected.Select` | shared determinism, witness, and exclusion conformance |
| `SATS-V0-012` | static `Units` implementation | source inspection; no process invocation in production package |
