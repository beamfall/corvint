# Runner selection self-review repair

Repair starts at `9bbda0bbdc1bc78acf79bc2026f53ed12d46096b`; the lane's immutable
context, affected and CEM base remains `8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734`.
This entry records only the three requested findings and a blocking existing test.

- RSP-V0-007: `DropRead` refuses aliased reads even when creation/replacement of
  the resolved target is proven. Target provenance does not establish lexical
  symlink provenance. The unchanged `TestRSPV0007WriteProofSymlink` failed for
  both write kinds before the fix and passes afterward.
- RSP-V0-007/022: successful ordinary write-only opens retain their existence
  dependency. Exclusive creation and already-proven tree creation retain their
  existing exceptions. `TestRSPV0007WriteOnlyOpen` closes without reading, asserts
  a read dependency, deletes the file, and checks that the failing open and
  missing lookup are reached. Linux execution is NOT_RUN on this Mac; the whole
  Linux/amd64 package and tests compile locally.
- RSP-V0-002: Gradle discovery uses the selected native Test task with
  `--test-dry-run`, preserving its command, environment and fresh XML artifact
  declarations. Inventory accepts complete skipped native report identities,
  source bindings and counts; it rejects console scans, malformed reports,
  unknown classes, duplicates, non-skipped results and empty inventories.
  Existing inventory assertions are unchanged; Gradle inputs now use its native
  XML instead of unrelated console output. New fixtures retain both `p.Probe`
  and `p.SmokeTest`, and native tag/class/method-filter subsets.
- RSP-V0-005: the required whole registry run found the existing captured-input
  mutation assertion failing. JSON decoding into the populated input reused
  caller-owned slices/maps. Decode into an empty configuration to make the
  promised immutable snapshot. The existing assertion now passes unchanged.

## Decision and limits

Use Gradle's own dry-run discovery rather than recreating task semantics in
ConsoleLauncher. Gradle 8.3 introduced this mode; the official
[testing guide](https://docs.gradle.org/current/userguide/java_testing.html)
states that it generates reports showing selected tests, and the
[Test DSL](https://docs.gradle.org/current/dsl/org.gradle.api.tasks.testing.Test.html)
states that JUnit dry runs report skipped tests. Unsupported versions, failed
processes or unavailable fresh artifacts cannot supply inventory. The host must
capture every fresh suite after successful process retirement, as for execution.
This changes an internal artifact adapter, no JSON/wire shape or native parser.
ConsoleLauncher remains the existing path for the other runners. No golden,
frozen output, error/exit code, stderr contract or test assertion was changed.
Rollback is reverting this repair; no stored-state migration is involved.

Whole existing packages `internal/testevidence`, `internal/proctrace` and
`internal/testrunner/registry` pass through the existing local heavy wrapper,
without a test-name filter. Go is `go1.27.3`. Linux tracing execution and live
Gradle/JVM qualification remain NOT_RUN: this is a Mac and no Gradle launcher
is installed. The native dry-run fixtures are adapter tests, not live-tool
qualification. No qualification or exclusion authority is inferred.

Corvint context and fixed-base affected were used; affected selects all three
packages but retains UNKNOWN scope, stale-binary, inherited-base and build-variant
limits. Official update freshness is UNKNOWN because the installed updater's
version smoke cannot prove process cleanup in this sandbox. No verification
bypass or tool update was attempted. Account usage remains NOT_OBSERVED: no
account telemetry tool is exposed. Model controls were left unchanged; this is
bounded implementation work and independent review belongs to the orchestrator.

Pre-change capture was reused; no start capture was rerun. CEM rebind/check are
performed after the implementation commit; sealing, publication, Tasks writes
and model-backed review are excluded by the owner's repair instructions. Raw
receipts and checks remain outside Git under `~/lanes/rspt11jv-evidence/` and the
final outcome is in `~/lanes/rspt11jv.handback.md`.

`script/check-line-citations.sh --repin` found no moved unchanged citations;
`make line-citations-check` passed with the existing 180 skipped basename-only
references. `make changed-checks BASE=8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734`
passed citations but failed inherited read-scope/unbounded-reader declarations:
`internal/runnerselection` reaches an unbounded reader, several recorded units
are no longer unbounded, and unrelated benchmark/registry script fixtures lack
read declarations. These are outside the three-finding repair; no declaration
or assertion was loosened to hide them. The exact diagnostics remain in
`repair-changed-checks.log` outside the repository.
