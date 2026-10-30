# .NET affected-test selection V0

Owner: Russell Lewis
Date: 2026-08-29
Intent status: proposed
Delivery status: experimental
Authoritative inputs: `docs/specs/live-proof-carrying-verification-v0.md`, the operator's
2026-08-29 .NET framework requirements, `internal/liveverify/affected/unit.go`

## Agent digest
- Claim: Static .NET discovery conservatively maps changes to xUnit, NUnit, and MSTest selections while widening unresolved scope.
- Status: proposed/experimental
- Exists: the language-neutral affected-selection seam and this frozen adapter contract.
- Blocked on: complete static discovery, frontier handling, and independent runner/conformance qualification.
- Read next: User and measurable job; Verified current state; Requirements.

## User and measurable job

For a repository containing C# projects, Corvint should identify the runner-addressable xUnit, NUnit,
and MSTest checks affected by a changed `.cs` file, preserve project-reference witnesses, and expose
every static-discovery gap as unknown. The adapter succeeds experimentally when its supported method
inventory matches independent runner discovery on a real repository and the language-neutral
LPCV-V0-013/014/019 conformance suite remains byte-deterministic.

## Verified current state

The affected-selection seam admits exclusively owned repository paths and has no command/filter
field. A C# source file commonly contains several test methods, so method-per-unit and class-per-unit
models would attach one `.cs` path to several units and fail `ErrDuplicateOwner`. VSTest has no
source-file predicate. The smallest exact shape the seam can admit is therefore one test-source-file
unit whose identity contains the sorted OR of that file's statically proved method predicates.

## Requirements

- `DNAS-V0-001`: `Name` MUST be `dotnet`; `Owns` MUST accept `.cs` case-insensitively and MUST NOT
  claim `.feature`, `.yaml`, or `.yml`.
- `DNAS-V0-002`: Framework discovery MUST require both project evidence and matching source
  attributes. xUnit uses `xunit`/`xunit.v3` plus `Fact` or `Theory`; NUnit uses `NUnit` plus `Test`,
  `TestCase`, or `TestCaseSource`; MSTest uses `MSTest`, `MSTest.TestFramework`, or `MSTest.Sdk` plus
  `TestClass` and `TestMethod`/`DataTestMethod`. Repositories MAY contain several frameworks.
- `DNAS-V0-003`: One statically addressable test unit MUST own one `.cs` file and have identity
  `dotnet:<project.csproj>::<filter>`, where `<filter>` is the sorted unique `|` expression of exact
  `FullyQualifiedName=<namespace.class.method>` predicates. Its exact invocation is
  `dotnet test <project.csproj> --filter '<filter>'`. Parameter rows remain one method unit because
  static source cannot enumerate runtime data safely.
- `DNAS-V0-004`: One compilation unit `dotnet:<project.csproj>` MUST own the project's non-test C#
  files. Test-file units import it; it imports every test-file unit in the same project and all
  resolved repository `ProjectReference` compilation units. This conservative project cycle makes
  a changed shared test helper or fixture reach every same-project test. Unresolved membership or
  project references MUST raise a frontier. An observed `.fsproj` or `.vbproj` (any case) MUST raise
  `dotnet:project-reference-unresolved`, because its references into C# projects are not read.
- `DNAS-V0-005`: Aliased/custom attributes, inherited or generated tests, fixture-driven effects,
  reflection/data-source discovery, duplicate method identities, conditional/imported MSBuild
  configuration, unparseable source, and unsupported runner routing MUST raise a stable frontier and
  therefore `UNKNOWN` scope; they MUST NOT look like exclusions. A project with test evidence
  (`IsTestProject`, `Microsoft.NET.Test.Sdk`, or another test package) but no recognized
  xUnit/NUnit/MSTest framework package, such as one referencing only `xunit.core`, is unsupported
  runner routing and MUST raise `dotnet:test-runner-filter-unresolved`.
- `DNAS-V0-006`: `Microsoft.Playwright.*` and `Selenium.WebDriver` MUST use their detected
  xUnit/NUnit/MSTest host. A hostless E2E package is unknown. SpecFlow and Reqnroll package evidence
  MUST raise the Gherkin ownership frontier while `.feature` scenarios remain unclaimed.
- `DNAS-V0-007`: VSTest filter identities require `Microsoft.NET.Test.Sdk` and the matching VS test
  adapter. `Microsoft.Testing.Platform` or `MSTest.Sdk` routing remains unknown until a separately
  frozen framework-specific address contract exists.
- `DNAS-V0-008`: Units, paths, imports, predicates, and frontier reasons MUST be sorted and unique;
  fixed admissible inputs MUST reproduce the same graph and LPCV-V0-019 plan bytes.
- `DNAS-V0-009`: A new `.cs` path owned but not indexed MUST become `UNINDEXED_SOURCE_PATH`; project,
  feature, generator, and other unowned changes MUST preserve the language-neutral widening rules.
- `DNAS-V0-010`: Adding this explicit plugin MUST NOT change an existing CLI parity receipt byte or
  mutate `src/**`; no implicit global registration is allowed in this slice.

## Cross-language ownership decision required

Gherkin `.feature` files are runnable scenarios only after a host-specific generator binds them to
C# (Reqnroll/SpecFlow), Java, JavaScript, or another language. Maestro `.yaml`/`.yml` flows are also
runner inputs rather than source in one host language. Appium bodies keep their host extension, but
shared Appium configuration can have the same ambiguity.

If two current plugins put the same resource path in their units, graph construction fails with
`ErrDuplicateOwner`; if several plugins merely return true from `Owns`, the graph cannot record the
overlap and the first claimant found during unknown classification is accidental. Letting .NET claim
`.feature` would make host association, witnesses, eligibility, and exclusions depend on plugin
choice and would block another host adapter from admitting the same path.

A future resolution must specify exclusive versus shared resource ownership, canonical host/runner
identity, deterministic plugin-order-independent conflict behavior, how one dirty resource seeds all
bound scenario units, stable runner-addressable scenario IDs, and invalidation for feature/YAML,
step-definition, configuration, generator, and data changes. V0 does not widen `Language` or choose
that policy. A dirty unclaimed resource widens through `UNOWNED_DIRTY_PATH`; detected Reqnroll or
SpecFlow also raises `dotnet:gherkin-ownership-unresolved` before any dirty path exists.

## Non-goals and baseline

The baseline is `dotnet test <project>` or repository CI when any frontier is present. V0 does not
execute projects, evaluate MSBuild, discover runtime-generated rows, support GdUnit4/TUnit, infer
file dependencies from `using`, claim Gherkin/Maestro resources, or qualify any runtime/OS tuple.

## Trust, bounds, and failure behavior

The adapter reads repository files only through the bounded affected walker; it executes no build,
restore, package manager, or test runner. Whole-walk errors fail the observation. Per-file/project,
discovery, dependency, and runner uncertainty is fail-open for repository work and fail-closed for
narrow selection by entering `Result.Frontier`. Existing walker limits remain authoritative.

## Acceptance matrix

| Evidence | Required result |
|---|---|
| xUnit `Fact`/`Theory`, NUnit `Test`/`TestCase`, MSTest `TestMethod` | package-and-attribute detection plus exact FQN filter |
| mixed frameworks and Playwright/Selenium hosts | each detected host retained; no single-framework assumption |
| project-reference chain plus unrelated test project | complete selected closure and bounded exclusion |
| custom/dynamic/fixture/generated/imported config | named frontier and `UNKNOWN`, never silent omission |
| Reqnroll/SpecFlow `.feature` | no ownership claim plus Gherkin frontier |
| real NUnit repository | discovered supported FQN method set exactly equals independent runner discovery |
| language-neutral conformance and full repository gates | pass with deterministic graph/plan bytes |
| CLI parity | identical receipts using a freshly built, PATH-pinned `corvint` |

## Rollout, rollback, compatibility, and drift

The package is additive and explicitly composed. Rollback removes the package, its fixture case, and
this proposed spec without migrating state or changing existing graph semantics. Package names,
attribute conventions, VSTest/Microsoft.Testing.Platform filters, and C# syntax are drift surfaces;
new semantics require fixtures, a real-repository comparison, and a spec amendment before support.
Any confirmed omission disables bounded .NET selection until repaired and replayed.

## Traceability

| Requirement | Implementation/evidence | State |
|---|---|---:|
| `DNAS-V0-001..004`, `008..009` | `internal/liveverify/affected/dotnet/dotnet.go`; shared affected conformance | experimental |
| `DNAS-V0-002..007` | dotnet focused fixtures and frontier tests | experimental |
| `DNAS-V0-006` cross-language ownership | this spec; runtime scenario ownership decision remains open | partial/unknown |
| `DNAS-V0-010` compatibility | frozen `src/**` status and pinned 100-case parity run | experimental/pass |

## Unresolved decisions and promotion criteria

The operator must choose the shared-resource ownership model before scenario-level Reqnroll/SpecFlow
or Maestro support. Promotion additionally requires exact runtime/OS/framework tuples, clean and
dirty real-repository trials, dynamic/generator adversarial fixtures, and the applicable LPCV shadow
gate. This experimental adapter is not a generic .NET qualification claim.
