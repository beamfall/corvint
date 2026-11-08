# JVM runner selection pieces (V1-5113)

Implementation base: `b33282354e0e28705502554bfee2da359e4ae851`. The owner also requires
the Corvint start/affected commands against `8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734`.
This is a build-only slice of the accepted runner-selection contract, RSP-V0-001 through
RSP-V0-005. Qualification and JVM test execution remain NOT_RUN.

## Delivered boundary

`internal/testrunner/registry/jvm_plugin.go` supplies a configured, compiled-in SPI factory
for the existing `gradle-junit`, `junit-platform` and `maven-surefire` runner declarations.
It reuses the existing native command builders and XML parsers, canonical inventory,
exclusion planning, reconciliation and evidence component types. The factory returns the
native invocation for core staging; SPI calls only interpret captured inputs or describe
commands. They launch no process, read no artifact path and cannot write the record store.

As with the existing script plugin factory, configured instances are separate from the
unconfigured legacy lookup. Existing manifest/lookup absence assertions and execution
wires are unchanged. The host must supply immutable, complete source-set observations,
verify tool pins, stage invocation files, capture process outcomes and artifacts, and
assemble core trace components. Presence is never qualification: even a session supplied
`qualified=true` stays FULL with the named `unqualified` qualification-piece reason.

The inventory adapter parses ConsoleLauncher flat discovery, requires its final native
counts, binds test classes to tracked source blobs and canonicalizes results. Gradle and
Surefire use their existing `class#method` identity; the console retains its native unique
ID. Only simple Jupiter methods have an unambiguous correspondence in this slice.
Dynamic/template/nested tests, unknown engines and unresolved classes refuse inventory
and therefore widen to FULL. Known empty Vintage and suite engine containers are allowed;
their tests remain unsupported. The fixed Gradle init script lists actual native `Test`
tasks per source set and their compile classpaths, with a bounded begin/end/count envelope.
Aggregate tasks are never inferred from a task's name and remain script-kind work.

Native filtering uses a reported FULL superset for every nonempty selected run set. This
avoids treating build-tool method filters as exact dynamic-test selectors. Missing or extra
terminal identities go through existing core reconciliation and its single FULL retry;
earlier failures are retained. Native suite aggregates are split without rewriting suite
bytes or relaxing either legacy parser. Process exit failure remains the executor's duty;
XML alone does not attest a successful process.

The evidence adapter reuses the JaCoCo XML decoder after a closed native-vocabulary check.
It preserves both executed and instrumented zero-hit lines, requires complete instrumented
and retired process-tree observations, unions descendants, and requires the identical union
on every identity attributed to a shared tree. Raw exec probes require a classfile-bound
XML export; unknown formats are incomplete. `loaded` is the core-resolved compile classpath,
not a source-file glob or observed hit set. Dialect regions must name the exact captured
source blob. Missing dialect output remains `absent:jvm-region-dialect-unavailable`.
This base has no Java region extractor; declaration extents and JaCoCo method start lines
are deliberately not substituted for executable-body boundaries. The parallel host work
must retain this limitation when supplying captures. No mutation harness or qualification
record is added here.

## Evidence and limits

Fixtures in `internal/testrunner/registry/jvm_plugin_test.go` include positive and negative
tests named for each of RSP-V0-001 through RSP-V0-005. They cover source-set enumeration,
determinism, truncation, unsupported discovery, terminal results, divergence/retry,
mandates and stale exclusions, FULL supersets, JaCoCo line states, immutable captures,
shared-tree unions, missing dialects, raw exec refusal, unknown XML, caps and lifecycle
failures. All are written and type-checked, NOT_RUN under the owner's build-only instruction.
Shared spec Traceability rows, Agent digest and status are unchanged; the integration
handback supplies the requirement-to-test mapping.

`GOTOOLCHAIN=local go vet ./internal/testrunner/registry` passed; `gofmt -l` on all four new
Go files produced no output. The orchestrator owns execution of the complete existing and
new registry tests and independent review. Retained runner qualification, live tools, the
repository gate, final CEM bind/check/seal and native ticket writes are NOT_RUN in this lane.
The explicitly permitted `make -s spec-requirements-check line-citations-check` passed.
Its implementation internally executed the whole `internal/specindex` package and selected
`internal/lrfrepo` enumeration tests through the existing local heavy wrapper; both passed.
Those helper executions do not verify the JVM implementation. The wrapper retained a
non-fatal `nice: setpriority: Operation not permitted` warning. Citation checking retained
180 skipped basename-only references; this change adds no moved existing line citations.

Corvint context and affected were used. The latter selected the registry package but retained
scope UNKNOWN: 534 selected units, 399 excluded, 98 unknown; its broader base includes
inherited lane work. It also reported a stale installed binary and four mutable new paths.
No narrower coverage claim follows from that plan. The required initial dogfood command
returned 2 because citation-plan, intent-scope and outcome inputs were absent; its generated
tracked CEM change was restored before implementation delivery. No final bind was attempted.
Official Core and Tasks update checks could not establish freshness because their version
smoke failed descendant/process-group cleanup admission. No update bypass was attempted.

Raw receipts are outside the repository at `~/lanes/rspt11jv-evidence/`:

| Artifact | Bytes | SHA-256 |
| --- | ---: | --- |
| `context.json` | 7352 | `f5a9cea6a0cb16d1c79e8c5fc4b04137680ea14c96f473999e952621d8eb0a1b` |
| `affected.json` | 241835 | `8d8e0614f04ca5ee6ca9f70dad1227c65e8500d79a350728006d4a13c7b0da8a` |
| `dogfood-start.log` | 2858 | `385ccb1df3befcb7fcc22f14f72b2d2602043259432cc90323725b941ba81816` |
| `update-check.json` | 465 | `bff4a3f9701477237d20d65c8c5c119ca2b619daedc7e0f41207ac728611cf72` |
| `update-tasks.json` | 475 | `a2580e2978049d33fbbc6554c3dbfe724f26c0848d4a95813dab825a06f90305` |

Console discovery layout was checked against the official
[JUnit 5.11.4 flat printer](https://github.com/junit-team/junit5/blob/r5.11.4/junit-platform-console/src/main/java/org/junit/platform/console/tasks/FlatPrintingListener.java)
and current official launcher documentation through Context7. The synthetic fixtures do
not constitute live-tool qualification. Rollback is reverting the new factory and adapters;
there is no stored-state migration or production exclusion change.
